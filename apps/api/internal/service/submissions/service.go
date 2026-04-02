package submissions

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"labkit.local/apps/api/internal/storage"
	"labkit.local/packages/go/auth"
	dbpkg "labkit.local/packages/go/db"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/ssh"
)

var (
	ErrLabNotFound        = errors.New("lab not found")
	ErrMissingFiles       = errors.New("missing required files")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrLabClosed          = errors.New("lab is closed")
	ErrSubmissionKey      = errors.New("submission key not found")
	ErrInvalidSubmission  = errors.New("invalid submission")
	ErrSubmissionTooLarge = errors.New("submission exceeds manifest max_size")
	ErrDailyQuotaExceeded = errors.New("daily quota exceeded")
)

type UploadFile struct {
	Name    string
	Content []byte
}

type SubmitInput struct {
	LabID          string
	KeyFingerprint string
	Timestamp      time.Time
	Nonce          string
	Signature      []byte
	Files          []UploadFile
}

type Submission struct {
	ID          uuid.UUID     `json:"id"`
	Status      string        `json:"status"`
	ArtifactKey string        `json:"artifact_key"`
	ContentHash string        `json:"content_hash"`
	Quota       *QuotaSummary `json:"quota,omitempty"`
}

type ArtifactStore interface {
	Save(context.Context, string, []byte) error
	Delete(context.Context, string) error
}

type Repository interface {
	GetLab(context.Context, string) (sqlc.Labs, error)
	GetUserKeyByFingerprint(context.Context, string) (sqlc.UserKeys, error)
	ListUserKeys(context.Context, int64) ([]sqlc.UserKeys, error)
	ReserveNonce(context.Context, string) (bool, error)
	CountSubmissionQuotaUsage(context.Context, int64, string, time.Time, time.Time) (int64, error)
	GetLatestSubmissionByUserLab(context.Context, int64, string) (sqlc.Submissions, error)
	BeginTx(context.Context) (Tx, error)
}

type Tx interface {
	LockSubmissionQuota(context.Context, int64, string, time.Time) error
	CountSubmissionQuotaUsage(context.Context, int64, string, time.Time, time.Time) (int64, error)
	ReserveNonce(context.Context, string) (bool, error)
	CreateSubmission(context.Context, sqlc.CreateSubmissionParams) (sqlc.Submissions, error)
	CreateEvaluationJob(context.Context, uuid.UUID) (sqlc.EvaluationJobs, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type Service struct {
	repo          Repository
	artifacts     ArtifactStore
	now           func() time.Time
	verifier      auth.Verifier
	quotaLocation *time.Location
}

type dailyQuotaExceededError struct {
	daily    int
	used     int
	location *time.Location
}

func (e *dailyQuotaExceededError) Error() string {
	return fmt.Sprintf(
		"Daily submission limit reached (%d/%d). Quota resets at 00:00 %s.",
		e.used,
		e.daily,
		quotaLocationName(e.location),
	)
}

func (e *dailyQuotaExceededError) Is(target error) bool {
	return target == ErrDailyQuotaExceeded
}

func NewService(repo Repository, artifacts ArtifactStore) *Service {
	return &Service{
		repo:          repo,
		artifacts:     artifacts,
		now:           time.Now,
		verifier:      auth.Verifier{Now: time.Now},
		quotaLocation: defaultQuotaLocation(),
	}
}

func (s *Service) SetQuotaLocation(location *time.Location) {
	if location == nil {
		return
	}
	s.quotaLocation = location
}

func (s *Service) Intake(ctx context.Context, input SubmitInput) (result Submission, err error) {
	labRow, err := s.repo.GetLab(ctx, input.LabID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, ErrLabNotFound
		}
		return Submission{}, err
	}
	manifestData, err := parseManifest(labRow.Manifest)
	if err != nil {
		return Submission{}, err
	}
	if !labOpen(manifestData.Schedule, s.nowUTC()) {
		return Submission{}, ErrLabClosed
	}
	if err := validateFiles(manifestData.Submit.Files, manifestData.Submit.MaxSize, input.Files); err != nil {
		return Submission{}, err
	}

	key, err := s.repo.GetUserKeyByFingerprint(ctx, input.KeyFingerprint)
	if err != nil {
		return Submission{}, err
	}
	publicKey, err := parseAuthorizedKey(key.PublicKey)
	if err != nil {
		return Submission{}, fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	fileNames := make([]string, 0, len(input.Files))
	artifactFiles := make([]storage.ArtifactFile, 0, len(input.Files))
	for _, file := range input.Files {
		fileNames = append(fileNames, file.Name)
		artifactFiles = append(artifactFiles, storage.ArtifactFile{
			Name:    file.Name,
			Content: append([]byte(nil), file.Content...),
		})
	}
	archive, contentHash, err := storage.Archive(artifactFiles)
	if err != nil {
		return Submission{}, fmt.Errorf("archive submission: %w", err)
	}

	verifier := s.verifier
	verifier.Now = s.nowUTC
	payload := auth.NewPayload(input.LabID, input.Timestamp, input.Nonce, fileNames).WithContentHash(contentHash)
	if err := verifier.VerifySignature(publicKey, payload, input.Signature); err != nil {
		if errors.Is(err, auth.ErrInvalidSignature) {
			return Submission{}, ErrInvalidSignature
		}
		return Submission{}, err
	}

	artifactKey := buildArtifactKey(input.LabID, key.UserID, fileNames, contentHash)
	savedArtifact := false
	committed := false
	defer func() {
		if !savedArtifact || committed {
			return
		}
		if deleteErr := s.artifacts.Delete(ctx, artifactKey); deleteErr != nil {
			err = errors.Join(err, deleteErr)
		}
	}()

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return Submission{}, err
	}
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	windowStart, windowEnd := quotaWindowForTime(s.nowUTC(), s.quotaLocationOrDefault())
	if err := tx.LockSubmissionQuota(ctx, key.UserID, input.LabID, windowStart); err != nil {
		return Submission{}, err
	}
	usedBefore, err := s.enforceDailyQuota(ctx, tx, manifestData, key.UserID, input.LabID, windowStart, windowEnd)
	if err != nil {
		return Submission{}, err
	}
	if err := s.artifacts.Save(ctx, artifactKey, archive); err != nil {
		return Submission{}, fmt.Errorf("persist artifact: %w", err)
	}
	savedArtifact = true
	if reserved, err := tx.ReserveNonce(ctx, input.Nonce); err != nil {
		return Submission{}, err
	} else if !reserved {
		return Submission{}, auth.ErrNonceReplayed
	}

	submissionRow, err := tx.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		UserID:      key.UserID,
		LabID:       input.LabID,
		KeyID:       key.ID,
		ArtifactKey: artifactKey,
		ContentHash: contentHash,
		Status:      "queued",
	})
	if err != nil {
		return Submission{}, err
	}
	if _, err := tx.CreateEvaluationJob(ctx, submissionRow.ID); err != nil {
		return Submission{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, err
	}
	committed = true
	savedArtifact = false

	return Submission{
		ID:          submissionRow.ID,
		Status:      submissionRow.Status,
		ArtifactKey: submissionRow.ArtifactKey,
		ContentHash: submissionRow.ContentHash,
		Quota:       BuildQuotaSummary(manifestData, int(usedBefore)+1, s.quotaLocationOrDefault()),
	}, nil
}

func (s *Service) GetSubmitPrecheck(ctx context.Context, userID int64, labID string) (SubmitPrecheck, error) {
	if s == nil || s.repo == nil {
		return SubmitPrecheck{}, fmt.Errorf("submissions service unavailable")
	}

	labRow, err := s.repo.GetLab(ctx, labID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmitPrecheck{}, ErrLabNotFound
		}
		return SubmitPrecheck{}, err
	}
	manifestData, err := parseManifest(labRow.Manifest)
	if err != nil {
		return SubmitPrecheck{}, err
	}

	windowStart, windowEnd := QuotaWindowForTime(s.nowUTC(), s.quotaLocationOrDefault())
	used, err := s.repo.CountSubmissionQuotaUsage(ctx, userID, labID, windowStart, windowEnd)
	if err != nil {
		return SubmitPrecheck{}, err
	}

	precheck := SubmitPrecheck{
		Quota: BuildQuotaSummary(manifestData, int(used), s.quotaLocationOrDefault()),
	}
	latest, err := s.repo.GetLatestSubmissionByUserLab(ctx, userID, labID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return precheck, nil
		}
		return SubmitPrecheck{}, err
	}
	precheck.LatestSubmission = LatestSubmissionHintFromRow(latest)
	return precheck, nil
}

func (s *Service) nowUTC() time.Time {
	now := s.now
	if now == nil {
		now = time.Now
	}
	return now().UTC()
}

func (s *Service) enforceDailyQuota(ctx context.Context, tx Tx, manifestData *manifest.Manifest, userID int64, labID string, windowStart, windowEnd time.Time) (int64, error) {
	if manifestData == nil || manifestData.Quota.Daily <= 0 {
		return 0, nil
	}

	used, err := tx.CountSubmissionQuotaUsage(ctx, userID, labID, windowStart, windowEnd)
	if err != nil {
		return 0, err
	}
	if used < int64(manifestData.Quota.Daily) {
		return used, nil
	}
	return used, NewDailyQuotaExceededError(manifestData.Quota.Daily, int(used), s.quotaLocationOrDefault())
}

func (s *Service) quotaLocationOrDefault() *time.Location {
	if s.quotaLocation != nil {
		return s.quotaLocation
	}
	return defaultQuotaLocation()
}

func quotaWindowForTime(now time.Time, location *time.Location) (time.Time, time.Time) {
	localNow := now.In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	return start, start.AddDate(0, 0, 1)
}

func quotaLocationName(location *time.Location) string {
	if location == nil || strings.TrimSpace(location.String()) == "" {
		return "Asia/Shanghai"
	}
	return location.String()
}

func submissionQuotaLockKey(userID int64, labID string, dayStart time.Time) int64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%d|%s|%s", userID, labID, dayStart.Format(time.RFC3339))
	return int64(hash.Sum64())
}

func NewDailyQuotaExceededError(daily, used int, location *time.Location) error {
	return &dailyQuotaExceededError{
		daily:    daily,
		used:     used,
		location: location,
	}
}

func defaultQuotaLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		return location
	}
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

func validateFiles(expected []string, maxSize string, files []UploadFile) error {
	if len(files) == 0 {
		return ErrMissingFiles
	}
	limit, err := parseMaxSize(maxSize)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSubmission, err)
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, name := range expected {
		expectedSet[name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		name := file.Name
		if strings.TrimSpace(name) != name {
			return ErrInvalidSubmission
		}
		if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
			return ErrInvalidSubmission
		}
		if _, ok := expectedSet[name]; !ok {
			return ErrInvalidSubmission
		}
		if _, ok := seen[name]; ok {
			return ErrInvalidSubmission
		}
		if int64(len(file.Content)) > limit {
			return ErrSubmissionTooLarge
		}
		seen[name] = struct{}{}
	}
	for _, required := range expected {
		if _, ok := seen[required]; !ok {
			return ErrMissingFiles
		}
	}
	return nil
}

func parseMaxSize(raw string) (int64, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return 0, errors.New("max size is required")
	}

	units := []struct {
		suffix string
		scale  int64
	}{
		{suffix: "GB", scale: 1000 * 1000 * 1000},
		{suffix: "MB", scale: 1000 * 1000},
		{suffix: "KB", scale: 1000},
		{suffix: "B", scale: 1},
	}
	for _, unit := range units {
		suffix := unit.suffix
		if !strings.HasSuffix(s, suffix) {
			continue
		}
		valuePart := strings.TrimSpace(strings.TrimSuffix(s, suffix))
		if valuePart == "" {
			return 0, fmt.Errorf("invalid max size %q", raw)
		}
		value, err := strconv.ParseFloat(valuePart, 64)
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("invalid max size %q", raw)
		}
		size := value * float64(unit.scale)
		if size > math.MaxInt64 {
			return 0, fmt.Errorf("invalid max size %q", raw)
		}
		return int64(size), nil
	}
	return 0, fmt.Errorf("unsupported max size unit in %q", raw)
}

func parseManifest(raw []byte) (*manifest.Manifest, error) {
	var parsed manifest.Manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func labOpen(schedule manifest.ScheduleSection, now time.Time) bool {
	if !schedule.Open.IsZero() && now.Before(schedule.Open.UTC()) {
		return false
	}
	if !schedule.Close.IsZero() && now.After(schedule.Close.UTC()) {
		return false
	}
	return true
}

func parseAuthorizedKey(raw string) (ed25519.PublicKey, error) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(raw)))
	if err != nil {
		return nil, err
	}
	cryptoKey, ok := publicKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, errors.New("public key does not expose crypto key")
	}
	edKey, ok := cryptoKey.CryptoPublicKey().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ed25519")
	}
	return edKey, nil
}

func buildArtifactKey(labID string, userID int64, fileNames []string, contentHash string) string {
	names := append([]string(nil), fileNames...)
	slices.Sort(names)
	fingerprint := sha256.Sum256([]byte(strings.Join(names, "\n")))
	return fmt.Sprintf("%s/%d/%s-%s.tar.gz", labID, userID, hex.EncodeToString(fingerprint[:8]), contentHash[:16])
}

type repo struct {
	store *dbpkg.Store
	pool  *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repo{
		store: dbpkg.New(pool),
		pool:  pool,
	}
}

func (r *repo) GetLab(ctx context.Context, labID string) (sqlc.Labs, error) {
	return r.store.GetLab(ctx, labID)
}

func (r *repo) GetUserKeyByFingerprint(ctx context.Context, fingerprint string) (sqlc.UserKeys, error) {
	fingerprintText := pgtype.Text{String: fingerprint, Valid: strings.TrimSpace(fingerprint) != ""}
	row, err := r.store.GetUserKeyByFingerprint(ctx, fingerprintText)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.UserKeys{}, err
	}
	rows, err := r.store.ListActiveUserKeys(ctx)
	if err != nil {
		return sqlc.UserKeys{}, err
	}
	for _, row := range rows {
		if row.Fingerprint.Valid && strings.TrimSpace(row.Fingerprint.String) != "" {
			continue
		}
		publicKey, err := parseAuthorizedKey(row.PublicKey)
		if err != nil {
			continue
		}
		if auth.PublicKeyFingerprint(publicKey) == fingerprint {
			_ = r.store.UpdateUserKeyFingerprint(ctx, sqlc.UpdateUserKeyFingerprintParams{
				ID:          row.ID,
				Fingerprint: fingerprintText,
			})
			return row, nil
		}
	}
	return sqlc.UserKeys{}, pgx.ErrNoRows
}

func (r *repo) ListUserKeys(ctx context.Context, userID int64) ([]sqlc.UserKeys, error) {
	return r.store.ListUserKeys(ctx, userID)
}

func (r *repo) ReserveNonce(ctx context.Context, nonce string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `INSERT INTO used_nonces (nonce) VALUES ($1) ON CONFLICT DO NOTHING`, nonce)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *repo) CountSubmissionQuotaUsage(ctx context.Context, userID int64, labID string, start, end time.Time) (int64, error) {
	return r.store.CountSubmissionQuotaUsage(ctx, sqlc.CountSubmissionQuotaUsageParams{
		UserID:      userID,
		LabID:       labID,
		CreatedAt:   pgtype.Timestamptz{Time: start, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: end, Valid: true},
	})
}

func (r *repo) GetLatestSubmissionByUserLab(ctx context.Context, userID int64, labID string) (sqlc.Submissions, error) {
	return r.store.GetLatestSubmissionByUserLab(ctx, sqlc.GetLatestSubmissionByUserLabParams{
		UserID: userID,
		LabID:  labID,
	})
}

func (r *repo) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &storeTx{tx: tx, store: dbpkg.New(tx)}, nil
}

type storeTx struct {
	tx    pgx.Tx
	store *dbpkg.Store
}

func (tx *storeTx) LockSubmissionQuota(ctx context.Context, userID int64, labID string, dayStart time.Time) error {
	_, err := tx.tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, submissionQuotaLockKey(userID, labID, dayStart))
	return err
}

func (tx *storeTx) CountSubmissionQuotaUsage(ctx context.Context, userID int64, labID string, start, end time.Time) (int64, error) {
	return tx.store.CountSubmissionQuotaUsage(ctx, sqlc.CountSubmissionQuotaUsageParams{
		UserID:      userID,
		LabID:       labID,
		CreatedAt:   pgtype.Timestamptz{Time: start, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: end, Valid: true},
	})
}

func (tx *storeTx) ReserveNonce(ctx context.Context, nonce string) (bool, error) {
	tag, err := tx.tx.Exec(ctx, `INSERT INTO used_nonces (nonce) VALUES ($1) ON CONFLICT DO NOTHING`, nonce)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (tx *storeTx) CreateSubmission(ctx context.Context, arg sqlc.CreateSubmissionParams) (sqlc.Submissions, error) {
	return tx.store.CreateSubmission(ctx, arg)
}

func (tx *storeTx) CreateEvaluationJob(ctx context.Context, submissionID uuid.UUID) (sqlc.EvaluationJobs, error) {
	return tx.store.CreateEvaluationJob(ctx, submissionID)
}

func (tx *storeTx) Commit(ctx context.Context) error {
	return tx.tx.Commit(ctx)
}

func (tx *storeTx) Rollback(ctx context.Context) error {
	return tx.tx.Rollback(ctx)
}
