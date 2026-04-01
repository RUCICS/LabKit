package submissions

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/ssh"
)

var (
	ErrMissingFiles       = errors.New("missing required files")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrLabClosed          = errors.New("lab is closed")
	ErrSubmissionKey      = errors.New("submission key not found")
	ErrInvalidSubmission  = errors.New("invalid submission")
	ErrSubmissionTooLarge = errors.New("submission exceeds manifest max_size")
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
	ID          uuid.UUID `json:"id"`
	Status      string    `json:"status"`
	ArtifactKey string    `json:"artifact_key"`
	ContentHash string    `json:"content_hash"`
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
	BeginTx(context.Context) (Tx, error)
}

type Tx interface {
	CreateSubmission(context.Context, sqlc.CreateSubmissionParams) (sqlc.Submissions, error)
	CreateEvaluationJob(context.Context, uuid.UUID) (sqlc.EvaluationJobs, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type Service struct {
	repo      Repository
	artifacts ArtifactStore
	now       func() time.Time
	verifier  auth.Verifier
}

func NewService(repo Repository, artifacts ArtifactStore) *Service {
	return &Service{
		repo:      repo,
		artifacts: artifacts,
		now:       time.Now,
		verifier:  auth.Verifier{Now: time.Now},
	}
}

func (s *Service) Intake(ctx context.Context, input SubmitInput) (result Submission, err error) {
	labRow, err := s.repo.GetLab(ctx, input.LabID)
	if err != nil {
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
	verifier.Nonces = s.repo
	payload := auth.NewPayload(input.LabID, input.Timestamp, input.Nonce, fileNames).WithContentHash(contentHash)
	if err := verifier.Verify(ctx, publicKey, payload, input.Signature); err != nil {
		if errors.Is(err, auth.ErrInvalidSignature) {
			return Submission{}, ErrInvalidSignature
		}
		return Submission{}, err
	}

	artifactKey := buildArtifactKey(input.LabID, key.UserID, fileNames, contentHash)
	if err := s.artifacts.Save(ctx, artifactKey, archive); err != nil {
		return Submission{}, fmt.Errorf("persist artifact: %w", err)
	}
	savedArtifact := true
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
	}, nil
}

func (s *Service) nowUTC() time.Time {
	now := s.now
	if now == nil {
		now = time.Now
	}
	return now().UTC()
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
	row, err := r.store.GetUserKeyByFingerprint(ctx, fingerprint)
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
				Fingerprint: fingerprint,
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
