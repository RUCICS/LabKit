package admin

import (
	"context"

	dbpkg "labkit.local/packages/go/db"
	"labkit.local/packages/go/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (r *repo) GetSubmission(ctx context.Context, id uuid.UUID) (sqlc.Submissions, error) {
	return r.store.GetSubmission(ctx, id)
}

func (r *repo) GetUserByID(ctx context.Context, id int64) (sqlc.Users, error) {
	return r.store.GetUserByID(ctx, id)
}

func (r *repo) ListLeaderboardByLab(ctx context.Context, labID string) ([]sqlc.Leaderboard, error) {
	return r.store.ListLeaderboardByLab(ctx, labID)
}

func (r *repo) ListLabProfilesByLab(ctx context.Context, labID string) ([]sqlc.LabProfiles, error) {
	return r.store.ListLabProfilesByLab(ctx, labID)
}

func (r *repo) ListScoresByLab(ctx context.Context, labID string) ([]sqlc.Scores, error) {
	return r.store.ListScoresByLab(ctx, labID)
}

func (r *repo) ListRecentEvaluationJobsByLab(ctx context.Context, arg sqlc.ListRecentEvaluationJobsByLabParams) ([]sqlc.ListRecentEvaluationJobsByLabRow, error) {
	return r.store.ListRecentEvaluationJobsByLab(ctx, arg)
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

func (tx *storeTx) ListLeaderboardByLab(ctx context.Context, labID string) ([]sqlc.Leaderboard, error) {
	return tx.store.ListLeaderboardByLab(ctx, labID)
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
