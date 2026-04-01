package runtime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"labkit.local/packages/go/db/sqlc"
)

func TestLoopTreatsHandlerCancellationAsGracefulStop(t *testing.T) {
	t.Parallel()

	loop := Loop{
		Queue: &stubQueue{
			job: &sqlc.EvaluationJobs{ID: uuid.New()},
		},
		WorkerID: "worker-a",
		Handler: JobHandlerFunc(func(context.Context, *sqlc.EvaluationJobs) error {
			return context.Canceled
		}),
	}

	err := loop.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

type stubQueue struct {
	job *sqlc.EvaluationJobs
}

func (q *stubQueue) Claim(context.Context, string) (*sqlc.EvaluationJobs, error) {
	job := q.job
	q.job = nil
	return job, nil
}
