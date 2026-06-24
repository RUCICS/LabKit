package grade

import (
	"context"

	dbpkg "labkit.local/packages/go/db"
	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the DB surface used by the grade service.
type Repository interface {
	UpsertFinalGrade(context.Context, sqlc.UpsertFinalGradeParams) (sqlc.FinalGrades, error)
	GetFinalGrade(context.Context, sqlc.GetFinalGradeParams) (sqlc.FinalGrades, error)
	PublishFinalGrades(context.Context, string) (int64, error)
	DeleteFinalGradesByLab(context.Context, string) (int64, error)
}

type repo struct {
	store *dbpkg.Store
}

// NewRepository builds a grade Repository backed by the shared sqlc store.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repo{store: dbpkg.New(pool)}
}

func (r *repo) UpsertFinalGrade(ctx context.Context, arg sqlc.UpsertFinalGradeParams) (sqlc.FinalGrades, error) {
	return r.store.UpsertFinalGrade(ctx, arg)
}

func (r *repo) GetFinalGrade(ctx context.Context, arg sqlc.GetFinalGradeParams) (sqlc.FinalGrades, error) {
	return r.store.GetFinalGrade(ctx, arg)
}

func (r *repo) PublishFinalGrades(ctx context.Context, labID string) (int64, error) {
	return r.store.PublishFinalGrades(ctx, labID)
}

func (r *repo) DeleteFinalGradesByLab(ctx context.Context, labID string) (int64, error) {
	return r.store.DeleteFinalGradesByLab(ctx, labID)
}
