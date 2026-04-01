package leaderboard

import (
	"context"

	dbpkg "labkit.local/packages/go/db"
	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

type repo struct {
	store *dbpkg.Store
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repo{store: dbpkg.New(pool)}
}

func (r *repo) GetLab(ctx context.Context, labID string) (sqlc.Labs, error) {
	return r.store.GetLab(ctx, labID)
}

func (r *repo) ListLeaderboardByLabAndMetricAsc(ctx context.Context, arg sqlc.ListLeaderboardByLabAndMetricAscParams) ([]sqlc.ListLeaderboardByLabAndMetricAscRow, error) {
	return r.store.ListLeaderboardByLabAndMetricAsc(ctx, arg)
}

func (r *repo) ListLeaderboardByLabAndMetricDesc(ctx context.Context, arg sqlc.ListLeaderboardByLabAndMetricDescParams) ([]sqlc.ListLeaderboardByLabAndMetricDescRow, error) {
	return r.store.ListLeaderboardByLabAndMetricDesc(ctx, arg)
}

func (r *repo) ListScoresByLab(ctx context.Context, labID string) ([]sqlc.Scores, error) {
	return r.store.ListScoresByLab(ctx, labID)
}

func (r *repo) ListLabProfilesByLab(ctx context.Context, labID string) ([]sqlc.LabProfiles, error) {
	return r.store.ListLabProfilesByLab(ctx, labID)
}
