package personal

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	submissionsvc "labkit.local/apps/api/internal/service/submissions"
	"labkit.local/packages/go/auth"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/ssh"
)

var (
	ErrLabNotFound            = errors.New("lab not found")
	ErrSubmissionNotFound     = errors.New("submission not found")
	ErrInvalidNickname        = errors.New("invalid nickname")
	ErrInvalidTrack           = errors.New("invalid track")
	ErrTrackSelectionDisabled = errors.New("track selection is disabled")
	ErrUnauthorized           = errors.New("unauthorized")
)

type Repository interface {
	GetLab(context.Context, string) (sqlc.Labs, error)
	ListSubmissionsByUserLab(context.Context, sqlc.ListSubmissionsByUserLabParams) ([]sqlc.Submissions, error)
	GetSubmission(context.Context, uuid.UUID) (sqlc.Submissions, error)
	ListScoresBySubmission(context.Context, uuid.UUID) ([]sqlc.Scores, error)
	GetUserKeyByFingerprint(context.Context, string) (sqlc.UserKeys, error)
	ReserveNonce(context.Context, string) (bool, error)
	UpsertLabProfileNickname(context.Context, sqlc.UpsertLabProfileNicknameParams) (sqlc.LabProfiles, error)
	UpsertLabProfileTrack(context.Context, sqlc.UpsertLabProfileTrackParams) (sqlc.LabProfiles, error)
	ListUserKeys(context.Context, int64) ([]sqlc.UserKeys, error)
	DeleteUserKey(context.Context, sqlc.DeleteUserKeyParams) error
}

type Service struct {
	repo          Repository
	quotaLocation *time.Location
	now           func() time.Time
}

type AuthInput struct {
	KeyFingerprint string
	Method         string
	Path           string
	Timestamp      time.Time
	Nonce          string
	Signature      []byte
	Body           []byte
}

type AuthenticatedUser struct {
	UserID int64
	KeyID  int64
}

type HistoryItem struct {
	ID         uuid.UUID  `json:"id"`
	Status     string     `json:"status"`
	Verdict    string     `json:"verdict,omitempty"`
	Message    string     `json:"message,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type Score struct {
	MetricID string  `json:"metric_id"`
	Value    float32 `json:"value"`
}

type SubmissionDetail struct {
	HistoryItem
	LabID       string                      `json:"lab_id"`
	KeyID       int64                       `json:"key_id"`
	ArtifactKey string                      `json:"artifact_key"`
	ContentHash string                      `json:"content_hash"`
	QuotaState  string                      `json:"quota_state,omitempty"`
	Detail      json.RawMessage             `json:"detail,omitempty"`
	Scores      []Score                     `json:"scores,omitempty"`
	Quota       *submissionsvc.QuotaSummary `json:"quota,omitempty"`
}

type HistoryResponse struct {
	Submissions []HistoryItem               `json:"submissions"`
	Quota       *submissionsvc.QuotaSummary `json:"quota,omitempty"`
}

type Profile struct {
	LabID    string `json:"lab_id"`
	Nickname string `json:"nickname"`
	Track    string `json:"track,omitempty"`
	Pick     bool   `json:"pick"`
}

type Key struct {
	ID         int64     `json:"id"`
	PublicKey  string    `json:"public_key"`
	DeviceName string    `json:"device_name"`
	CreatedAt  time.Time `json:"created_at"`
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, quotaLocation: submissionsvc.DefaultQuotaLocation(), now: time.Now}
}

func (s *Service) SetQuotaLocation(location *time.Location) {
	if location != nil {
		s.quotaLocation = location
	}
}

func (s *Service) SetNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *Service) Authenticate(ctx context.Context, in AuthInput) (AuthenticatedUser, error) {
	if s == nil || s.repo == nil {
		return AuthenticatedUser{}, fmt.Errorf("personal service unavailable")
	}
	key, err := s.repo.GetUserKeyByFingerprint(ctx, in.KeyFingerprint)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthenticatedUser{}, ErrUnauthorized
		}
		return AuthenticatedUser{}, err
	}
	publicKey, err := parseAuthorizedKey(key.PublicKey)
	if err != nil {
		return AuthenticatedUser{}, ErrUnauthorized
	}
	payload := auth.NewPayload(strings.ToUpper(strings.TrimSpace(in.Method))+" "+strings.TrimSpace(in.Path), in.Timestamp, in.Nonce, nil).
		WithContentHash(bodyHash(in.Body))
	verifier := auth.Verifier{Now: time.Now, Nonces: s.repo}
	if err := verifier.Verify(ctx, publicKey, payload, in.Signature); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidSignature), errors.Is(err, auth.ErrExpiredTimestamp), errors.Is(err, auth.ErrNonceReplayed), errors.Is(err, auth.ErrNonceStoreRequired):
			return AuthenticatedUser{}, ErrUnauthorized
		default:
			return AuthenticatedUser{}, err
		}
	}
	return AuthenticatedUser{UserID: key.UserID, KeyID: key.ID}, nil
}

func (s *Service) ListSubmissionHistory(ctx context.Context, userID int64, labID string) (HistoryResponse, error) {
	if s == nil || s.repo == nil {
		return HistoryResponse{}, fmt.Errorf("personal service unavailable")
	}
	if strings.TrimSpace(labID) == "" {
		return HistoryResponse{}, ErrLabNotFound
	}
	lab, err := s.repo.GetLab(ctx, labID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HistoryResponse{}, ErrLabNotFound
		}
		return HistoryResponse{}, err
	}
	parsed, err := manifestFromRow(lab.Manifest)
	if err != nil {
		return HistoryResponse{}, err
	}
	rows, err := s.repo.ListSubmissionsByUserLab(ctx, sqlc.ListSubmissionsByUserLabParams{
		UserID: userID,
		LabID:  labID,
	})
	if err != nil {
		return HistoryResponse{}, err
	}
	items := make([]HistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, historyItemFromSubmission(row))
	}
	windowStart, windowEnd := submissionsvc.QuotaWindowForTime(s.nowUTC(), s.quotaLocationOrDefault())
	return HistoryResponse{
		Submissions: items,
		Quota:       submissionsvc.BuildQuotaSummary(parsed, submissionsvc.CountQuotaUsage(rows, windowStart, windowEnd), s.quotaLocationOrDefault()),
	}, nil
}

func (s *Service) GetSubmissionDetail(ctx context.Context, userID int64, labID string, submissionID uuid.UUID) (SubmissionDetail, error) {
	if s == nil || s.repo == nil {
		return SubmissionDetail{}, fmt.Errorf("personal service unavailable")
	}
	if strings.TrimSpace(labID) == "" {
		return SubmissionDetail{}, ErrLabNotFound
	}
	lab, err := s.repo.GetLab(ctx, labID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionDetail{}, ErrLabNotFound
		}
		return SubmissionDetail{}, err
	}
	parsed, err := manifestFromRow(lab.Manifest)
	if err != nil {
		return SubmissionDetail{}, err
	}
	row, err := s.repo.GetSubmission(ctx, submissionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionDetail{}, ErrSubmissionNotFound
		}
		return SubmissionDetail{}, err
	}
	if row.UserID != userID || row.LabID != labID {
		return SubmissionDetail{}, ErrSubmissionNotFound
	}
	scores, err := s.repo.ListScoresBySubmission(ctx, submissionID)
	if err != nil {
		return SubmissionDetail{}, err
	}
	rows, err := s.repo.ListSubmissionsByUserLab(ctx, sqlc.ListSubmissionsByUserLabParams{
		UserID: userID,
		LabID:  labID,
	})
	if err != nil {
		return SubmissionDetail{}, err
	}
	windowStart, windowEnd := submissionsvc.QuotaWindowForTime(s.nowUTC(), s.quotaLocationOrDefault())
	detail := submissionDetailFromRow(row, scores)
	detail.Quota = submissionsvc.BuildQuotaSummary(parsed, submissionsvc.CountQuotaUsage(rows, windowStart, windowEnd), s.quotaLocationOrDefault())
	return detail, nil
}

func (s *Service) UpdateNickname(ctx context.Context, userID int64, labID, nickname string) (Profile, error) {
	if s == nil || s.repo == nil {
		return Profile{}, fmt.Errorf("personal service unavailable")
	}
	if strings.TrimSpace(labID) == "" {
		return Profile{}, ErrLabNotFound
	}
	lab, err := s.repo.GetLab(ctx, labID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrLabNotFound
		}
		return Profile{}, err
	}
	name := strings.TrimSpace(nickname)
	if name == "" {
		return Profile{}, ErrInvalidNickname
	}
	row, err := s.repo.UpsertLabProfileNickname(ctx, sqlc.UpsertLabProfileNicknameParams{
		UserID:   userID,
		LabID:    labID,
		Nickname: name,
	})
	if err != nil {
		return Profile{}, err
	}
	return profileFromRow(row, lab)
}

func (s *Service) UpdateTrack(ctx context.Context, userID int64, labID, track string) (Profile, error) {
	if s == nil || s.repo == nil {
		return Profile{}, fmt.Errorf("personal service unavailable")
	}
	if strings.TrimSpace(labID) == "" {
		return Profile{}, ErrLabNotFound
	}
	lab, err := s.repo.GetLab(ctx, labID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrLabNotFound
		}
		return Profile{}, err
	}
	parsed, err := manifestFromRow(lab.Manifest)
	if err != nil {
		return Profile{}, err
	}
	if !parsed.Board.Pick {
		return Profile{}, ErrTrackSelectionDisabled
	}
	track = strings.TrimSpace(track)
	if track == "" {
		return Profile{}, ErrInvalidTrack
	}
	if !trackExists(parsed, track) {
		return Profile{}, ErrInvalidTrack
	}
	row, err := s.repo.UpsertLabProfileTrack(ctx, sqlc.UpsertLabProfileTrackParams{
		UserID: userID,
		LabID:  labID,
		Track:  pgtype.Text{String: track, Valid: true},
	})
	if err != nil {
		return Profile{}, err
	}
	return profileFromRow(row, lab)
}

func (s *Service) ListKeys(ctx context.Context, userID int64) ([]Key, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("personal service unavailable")
	}
	rows, err := s.repo.ListUserKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	keys := make([]Key, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, keyFromRow(row))
	}
	return keys, nil
}

func (s *Service) RevokeKey(ctx context.Context, userID, keyID int64) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("personal service unavailable")
	}
	return s.repo.DeleteUserKey(ctx, sqlc.DeleteUserKeyParams{
		ID:     keyID,
		UserID: userID,
	})
}

func historyItemFromSubmission(row sqlc.Submissions) HistoryItem {
	item := HistoryItem{
		ID:        row.ID,
		Status:    row.Status,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.Verdict.Valid {
		item.Verdict = row.Verdict.String
	}
	if row.Message.Valid {
		item.Message = row.Message.String
	}
	if row.FinishedAt.Valid {
		finished := row.FinishedAt.Time
		item.FinishedAt = &finished
	}
	return item
}

func submissionDetailFromRow(row sqlc.Submissions, scores []sqlc.Scores) SubmissionDetail {
	detail := SubmissionDetail{
		HistoryItem: historyItemFromSubmission(row),
		LabID:       row.LabID,
		KeyID:       row.KeyID,
		ArtifactKey: row.ArtifactKey,
		ContentHash: row.ContentHash,
		QuotaState:  row.QuotaState,
	}
	if len(row.Detail) > 0 {
		detail.Detail = append(json.RawMessage(nil), row.Detail...)
	}
	if len(scores) > 0 {
		detail.Scores = make([]Score, 0, len(scores))
		for _, score := range scores {
			detail.Scores = append(detail.Scores, Score{
				MetricID: score.MetricID,
				Value:    score.Value,
			})
		}
	}
	return detail
}

func (s *Service) quotaLocationOrDefault() *time.Location {
	if s != nil && s.quotaLocation != nil {
		return s.quotaLocation
	}
	return submissionsvc.DefaultQuotaLocation()
}

func (s *Service) nowUTC() time.Time {
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	return now().UTC()
}

func profileFromRow(row sqlc.LabProfiles, lab sqlc.Labs) (Profile, error) {
	parsed, err := manifestFromRow(lab.Manifest)
	if err != nil {
		return Profile{}, err
	}
	return Profile{
		LabID:    row.LabID,
		Nickname: row.Nickname,
		Track:    textValue(row.Track),
		Pick:     parsed.Board.Pick,
	}, nil
}

func keyFromRow(row sqlc.UserKeys) Key {
	return Key{
		ID:         row.ID,
		PublicKey:  row.PublicKey,
		DeviceName: row.DeviceName,
		CreatedAt:  row.CreatedAt.Time,
	}
}

func manifestFromRow(raw []byte) (*manifest.Manifest, error) {
	var parsed manifest.Manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func trackExists(m *manifest.Manifest, track string) bool {
	for _, metric := range m.Metrics {
		if metric.ID == track {
			return true
		}
	}
	return false
}

func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
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

func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
