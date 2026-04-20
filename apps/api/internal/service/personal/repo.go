package personal

import (
	"context"
	"errors"
	"strings"

	"labkit.local/packages/go/auth"
	dbpkg "labkit.local/packages/go/db"
	"labkit.local/packages/go/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func (r *repo) ListSubmissionsByUserLab(ctx context.Context, arg sqlc.ListSubmissionsByUserLabParams) ([]sqlc.Submissions, error) {
	return r.store.ListSubmissionsByUserLab(ctx, arg)
}

func (r *repo) GetUserProfileByID(ctx context.Context, userID int64) (sqlc.Users, error) {
	return r.store.GetUserProfileByID(ctx, userID)
}

func (r *repo) UpdateUserNickname(ctx context.Context, arg sqlc.UpdateUserNicknameParams) (sqlc.Users, error) {
	return r.store.UpdateUserNickname(ctx, arg)
}

func (r *repo) ListRecentSubmissionsByUser(ctx context.Context, arg sqlc.ListRecentSubmissionsByUserParams) ([]sqlc.Submissions, error) {
	return r.store.ListRecentSubmissionsByUser(ctx, arg)
}

func (r *repo) GetSubmission(ctx context.Context, id uuid.UUID) (sqlc.Submissions, error) {
	return r.store.GetSubmission(ctx, id)
}

func (r *repo) ListScoresBySubmission(ctx context.Context, submissionID uuid.UUID) ([]sqlc.Scores, error) {
	return r.store.ListScoresBySubmission(ctx, submissionID)
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

func (r *repo) ReserveNonce(ctx context.Context, nonce string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `INSERT INTO used_nonces (nonce) VALUES ($1) ON CONFLICT DO NOTHING`, nonce)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *repo) UpsertLabProfileNickname(ctx context.Context, arg sqlc.UpsertLabProfileNicknameParams) (sqlc.LabProfiles, error) {
	return r.store.UpsertLabProfileNickname(ctx, arg)
}

func (r *repo) UpsertLabProfileTrack(ctx context.Context, arg sqlc.UpsertLabProfileTrackParams) (sqlc.LabProfiles, error) {
	return r.store.UpsertLabProfileTrack(ctx, arg)
}

func (r *repo) ListUserKeys(ctx context.Context, userID int64) ([]sqlc.UserKeys, error) {
	return r.store.ListUserKeys(ctx, userID)
}

func (r *repo) DeleteUserKey(ctx context.Context, arg sqlc.DeleteUserKeyParams) error {
	return r.store.DeleteUserKey(ctx, arg)
}
