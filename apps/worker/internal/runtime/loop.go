package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/jobs"
)

const defaultPollInterval = time.Second

type JobClaimer interface {
	Claim(ctx context.Context, workerID string) (*sqlc.EvaluationJobs, error)
}

type JobHandler interface {
	Handle(ctx context.Context, job *sqlc.EvaluationJobs) error
}

type JobHandlerFunc func(ctx context.Context, job *sqlc.EvaluationJobs) error

func (f JobHandlerFunc) Handle(ctx context.Context, job *sqlc.EvaluationJobs) error {
	return f(ctx, job)
}

type Loop struct {
	Queue        JobClaimer
	WorkerID     string
	PollInterval time.Duration
	Handler      JobHandler
}

func (l Loop) Run(ctx context.Context) error {
	if strings.TrimSpace(l.WorkerID) == "" {
		return jobs.ErrInvalidWorkerID
	}
	if l.Queue == nil {
		return fmt.Errorf("runtime: queue is required")
	}
	if l.Handler == nil {
		return fmt.Errorf("runtime: handler is required")
	}

	pollInterval := l.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}

		job, err := l.Queue.Claim(ctx, l.WorkerID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		if job != nil {
			if err := l.Handler.Handle(ctx, job); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
			timer.Reset(0)
			continue
		}

		timer.Reset(pollInterval)
	}
}
