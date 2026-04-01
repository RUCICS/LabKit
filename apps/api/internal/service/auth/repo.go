package auth

import (
	"context"

	dbpkg "labkit.local/packages/go/db"
	"labkit.local/packages/go/db/sqlc"

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

func (r *repo) CreateDeviceAuthRequest(ctx context.Context, arg sqlc.CreateDeviceAuthRequestParams) (sqlc.DeviceAuthRequests, error) {
	return r.store.CreateDeviceAuthRequest(ctx, arg)
}

func (r *repo) GetDeviceAuthRequestByDeviceCode(ctx context.Context, deviceCode string) (sqlc.DeviceAuthRequests, error) {
	return r.store.GetDeviceAuthRequestByDeviceCode(ctx, deviceCode)
}

func (r *repo) GetPendingDeviceAuthRequestByUserCode(ctx context.Context, userCode string) (sqlc.DeviceAuthRequests, error) {
	return r.store.GetPendingDeviceAuthRequestByUserCode(ctx, userCode)
}

func (r *repo) GetPendingDeviceAuthRequestByOAuthState(ctx context.Context, state string) (sqlc.DeviceAuthRequests, error) {
	return r.store.GetPendingDeviceAuthRequestByOAuthState(ctx, pgtype.Text{String: state, Valid: state != ""})
}

func (r *repo) CompleteDeviceAuthRequest(ctx context.Context, arg sqlc.CompleteDeviceAuthRequestParams) (sqlc.CompleteDeviceAuthRequestRow, error) {
	return r.store.CompleteDeviceAuthRequest(ctx, arg)
}

func (r *repo) GetUserKeyByPublicKey(ctx context.Context, publicKey string) (sqlc.UserKeys, error) {
	var row sqlc.UserKeys
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, public_key, fingerprint, device_name, created_at, revoked_at
		FROM user_keys
		WHERE public_key = $1
		  AND revoked_at IS NULL
		LIMIT 1
	`, publicKey).Scan(&row.ID, &row.UserID, &row.PublicKey, &row.Fingerprint, &row.DeviceName, &row.CreatedAt, &row.RevokedAt)
	return row, err
}
