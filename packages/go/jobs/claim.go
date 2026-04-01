package jobs

import (
	"context"
	"errors"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Claim moves the next available queued job into running state.
func (q *Queue) Claim(ctx context.Context, workerID string) (*sqlc.EvaluationJobs, error) {
	if err := validateWorkerID(workerID); err != nil {
		return nil, err
	}

	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := sqlc.New(tx)
	job, err := queries.GetNextQueuedEvaluationJob(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if err := queries.MarkEvaluationJobRunning(ctx, sqlc.MarkEvaluationJobRunningParams{
		ID:       job.ID,
		WorkerID: textValue(workerID),
	}); err != nil {
		return nil, err
	}

	job.Status = StatusRunning
	job.Attempts++
	job.WorkerID = pgtype.Text{String: workerID, Valid: true}
	now := timestamptzValue(time.Now().UTC())
	job.StartedAt = now
	job.UpdatedAt = now

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &job, nil
}
