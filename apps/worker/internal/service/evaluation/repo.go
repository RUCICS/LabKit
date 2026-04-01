package evaluation

import (
	"context"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repo struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repo{pool: pool}
}

func (r *repo) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &storeTx{tx: tx, queries: sqlc.New(tx)}, nil
}

type storeTx struct {
	tx      pgx.Tx
	queries *sqlc.Queries
}

func (tx *storeTx) UpdateSubmissionResult(ctx context.Context, arg sqlc.UpdateSubmissionResultParams) error {
	return tx.queries.UpdateSubmissionResult(ctx, arg)
}

func (tx *storeTx) CreateScore(ctx context.Context, arg sqlc.CreateScoreParams) error {
	return tx.queries.CreateScore(ctx, arg)
}

func (tx *storeTx) UpsertLeaderboardEntry(ctx context.Context, arg sqlc.UpsertLeaderboardEntryParams) (sqlc.Leaderboard, error) {
	return tx.queries.UpsertLeaderboardEntry(ctx, arg)
}

func (tx *storeTx) Commit(ctx context.Context) error {
	return tx.tx.Commit(ctx)
}

func (tx *storeTx) Rollback(ctx context.Context) error {
	return tx.tx.Rollback(ctx)
}
