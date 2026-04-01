package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/google/uuid"
)

func TestRunBuildsAndRunsLoop(t *testing.T) {
	t.Setenv("LABKIT_WORKER_BOOTSTRAP_ONLY", "true")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := false
	ran := false

	previousNewLoop := newLoop
	t.Cleanup(func() {
		newLoop = previousNewLoop
	})

	newLoop = func(cfg loopConfig) loopRunner {
		called = true
		if cfg.workerID != "worker-test" {
			t.Fatalf("workerID = %q, want worker-test", cfg.workerID)
		}
		if cfg.queue == nil {
			t.Fatal("queue = nil, want bootstrap queue")
		}
		if cfg.handler == nil {
			t.Fatal("handler = nil, want bootstrap handler")
		}
		return loopRunnerFunc(func(ctx context.Context) error {
			ran = true
			cancel()
			<-ctx.Done()
			return nil
		})
	}

	if err := run(ctx, "worker-test"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !called {
		t.Fatal("newLoop was not called")
	}
	if !ran {
		t.Fatal("loop Run was not called")
	}
}

func TestRunUsesHostnameWhenWorkerIDUnset(t *testing.T) {
	t.Setenv("LABKIT_WORKER_BOOTSTRAP_ONLY", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	previousNewLoop := newLoop
	t.Cleanup(func() {
		newLoop = previousNewLoop
	})

	newLoop = func(cfg loopConfig) loopRunner {
		host, err := os.Hostname()
		if err != nil {
			t.Fatalf("Hostname() error = %v", err)
		}
		if cfg.workerID != host {
			t.Fatalf("workerID = %q, want hostname %q", cfg.workerID, host)
		}
		return loopRunnerFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
	}

	if err := run(ctx, ""); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunDelegatesToDevWorkerWithRunOnceOption(t *testing.T) {
	t.Setenv("LABKIT_WORKER_BOOTSTRAP_ONLY", "false")
	t.Setenv("LABKIT_DEV_MODE", "true")
	t.Setenv("LABKIT_WORKER_DEV_FAKE_EVALUATION", "true")
	t.Setenv("LABKIT_WORKER_RUN_ONCE", "true")

	previousRunDevWorker := runDevWorker
	t.Cleanup(func() {
		runDevWorker = previousRunDevWorker
	})

	called := false
	runDevWorker = func(_ context.Context, opts devWorkerOptions) error {
		called = true
		if opts.workerID != "worker-test" {
			t.Fatalf("workerID = %q, want worker-test", opts.workerID)
		}
		if !opts.runOnce {
			t.Fatal("runOnce = false, want true")
		}
		return nil
	}

	if err := run(context.Background(), "worker-test"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !called {
		t.Fatal("runDevWorker was not called")
	}
}

func TestRunDelegatesToRealWorkerByDefault(t *testing.T) {
	t.Setenv("LABKIT_WORKER_BOOTSTRAP_ONLY", "false")
	t.Setenv("LABKIT_DEV_MODE", "false")
	t.Setenv("LABKIT_WORKER_RUN_ONCE", "true")
	t.Setenv("LABKIT_WORKER_WORKSPACE_ROOT", "/tmp/labkit-worker")
	t.Setenv("LABKIT_WORKER_MEMORY_LIMIT", "768m")
	t.Setenv("LABKIT_WORKER_CPU_LIMIT", "1.5")

	previousRunRealWorker := runRealWorker
	t.Cleanup(func() {
		runRealWorker = previousRunRealWorker
	})

	called := false
	runRealWorker = func(_ context.Context, opts realWorkerOptions) error {
		called = true
		if opts.workerID != "worker-test" {
			t.Fatalf("workerID = %q, want worker-test", opts.workerID)
		}
		if !opts.runOnce {
			t.Fatal("runOnce = false, want true")
		}
		if opts.workspaceRoot != "/tmp/labkit-worker" {
			t.Fatalf("workspaceRoot = %q", opts.workspaceRoot)
		}
		if opts.memoryLimit != "768m" {
			t.Fatalf("memoryLimit = %q", opts.memoryLimit)
		}
		if opts.cpuLimit != "1.5" {
			t.Fatalf("cpuLimit = %q", opts.cpuLimit)
		}
		return nil
	}

	if err := run(context.Background(), "worker-test"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !called {
		t.Fatal("runRealWorker was not called")
	}
}

func TestRunOneShotProcessesSingleQueuedJobAndReturns(t *testing.T) {
	queue := &fakeOneShotQueue{
		job: &sqlc.EvaluationJobs{ID: uuid.MustParse("11111111-1111-7111-8111-111111111111")},
	}
	handler := &fakeOneShotHandler{}

	if err := runOneShot(context.Background(), queue, "worker-test", handler); err != nil {
		t.Fatalf("runOneShot() error = %v", err)
	}
	if queue.claimCalls != 1 {
		t.Fatalf("claim calls = %d, want 1", queue.claimCalls)
	}
	if handler.handleCalls != 1 {
		t.Fatalf("handle calls = %d, want 1", handler.handleCalls)
	}
}

func TestRunOneShotReturnsWhenQueueIsEmpty(t *testing.T) {
	queue := &fakeOneShotQueue{}
	handler := &fakeOneShotHandler{}

	if err := runOneShot(context.Background(), queue, "worker-test", handler); err != nil {
		t.Fatalf("runOneShot() error = %v", err)
	}
	if handler.handleCalls != 0 {
		t.Fatalf("handle calls = %d, want 0", handler.handleCalls)
	}
}

func TestRunOneShotPropagatesHandlerError(t *testing.T) {
	queue := &fakeOneShotQueue{
		job: &sqlc.EvaluationJobs{ID: uuid.MustParse("22222222-2222-7222-8222-222222222222")},
	}
	handler := &fakeOneShotHandler{err: errors.New("boom")}

	err := runOneShot(context.Background(), queue, "worker-test", handler)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("runOneShot() error = %v, want boom", err)
	}
}

type fakeOneShotQueue struct {
	job        *sqlc.EvaluationJobs
	err        error
	claimCalls int
}

func (q *fakeOneShotQueue) Claim(context.Context, string) (*sqlc.EvaluationJobs, error) {
	q.claimCalls++
	if q.err != nil {
		return nil, q.err
	}
	return q.job, nil
}

type fakeOneShotHandler struct {
	err         error
	handleCalls int
}

func (h *fakeOneShotHandler) Handle(context.Context, *sqlc.EvaluationJobs) error {
	h.handleCalls++
	return h.err
}
