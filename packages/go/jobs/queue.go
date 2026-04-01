package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"labkit.local/packages/go/db/sqlc"
)

const (
	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusError   = "error"
)

var (
	ErrInvalidTerminalStatus = errors.New("jobs: invalid terminal status")
	ErrInvalidWorkerID       = errors.New("jobs: invalid worker id")
	ErrTransitionNotAllowed  = errors.New("jobs: transition not allowed")
)

// Queue exposes the minimal PostgreSQL-backed job queue primitives.
type Queue struct {
	pool *pgxpool.Pool
}

// New constructs a queue backed by the supplied PostgreSQL pool.
func New(pool *pgxpool.Pool) *Queue {
	return &Queue{pool: pool}
}

// Enqueue inserts a new evaluation job for a submission.
func (q *Queue) Enqueue(ctx context.Context, submissionID uuid.UUID) (sqlc.EvaluationJobs, error) {
	return sqlc.New(q.pool).CreateEvaluationJob(ctx, submissionID)
}

// Acknowledge marks a claimed job as terminal.
func (q *Queue) Acknowledge(ctx context.Context, jobID uuid.UUID, workerID, status, lastError string) error {
	if err := validateWorkerID(workerID); err != nil {
		return err
	}
	if !isTerminalStatus(status) {
		return fmt.Errorf("%w: %s", ErrInvalidTerminalStatus, status)
	}

	rows, err := sqlc.New(q.pool).MarkEvaluationJobDone(ctx, sqlc.MarkEvaluationJobDoneParams{
		ID:        jobID,
		Status:    status,
		WorkerID:  textValue(workerID),
		LastError: textValue(lastError),
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("%w: acknowledge job %s for worker %q", ErrTransitionNotAllowed, jobID, workerID)
	}
	return nil
}

// Retry requeues a claimed job for later processing.
func (q *Queue) Retry(ctx context.Context, jobID uuid.UUID, workerID string, availableAt time.Time, lastError string) error {
	if err := validateWorkerID(workerID); err != nil {
		return err
	}

	rows, err := sqlc.New(q.pool).RequeueEvaluationJob(ctx, sqlc.RequeueEvaluationJobParams{
		ID:          jobID,
		WorkerID:    textValue(workerID),
		AvailableAt: timestamptzValue(availableAt),
		LastError:   textValue(lastError),
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("%w: retry job %s for worker %q", ErrTransitionNotAllowed, jobID, workerID)
	}
	return nil
}

func validateWorkerID(workerID string) error {
	if strings.TrimSpace(workerID) == "" {
		return ErrInvalidWorkerID
	}
	return nil
}

func isTerminalStatus(status string) bool {
	return status == StatusDone || status == StatusError
}

func textValue(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func timestamptzValue(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: v, Valid: true}
}
