package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"labkit.local/apps/worker/internal/runtime"
	"labkit.local/apps/worker/internal/service/evaluation"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/jobs"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv("LABKIT_WORKER_ID")); err != nil {
		log.Fatal(err)
	}
}

type loopRunner interface {
	Run(ctx context.Context) error
}

type loopRunnerFunc func(ctx context.Context) error

func (f loopRunnerFunc) Run(ctx context.Context) error {
	return f(ctx)
}

type loopConfig struct {
	workerID     string
	pollInterval time.Duration
	queue        runtime.JobClaimer
	handler      runtime.JobHandler
}

type devWorkerOptions struct {
	workerID     string
	pollInterval time.Duration
	runOnce      bool
}

type realWorkerOptions struct {
	workerID      string
	pollInterval  time.Duration
	runOnce       bool
	workspaceRoot string
	memoryLimit   string
	cpuLimit      string
}

var newLoop = func(cfg loopConfig) loopRunner {
	return runtime.Loop{
		Queue:        cfg.queue,
		WorkerID:     cfg.workerID,
		PollInterval: cfg.pollInterval,
		Handler:      cfg.handler,
	}
}

var runDevWorker = defaultRunDevWorker
var runRealWorker = defaultRunRealWorker

func run(ctx context.Context, workerID string) error {
	resolvedWorkerID, err := resolveWorkerID(workerID)
	if err != nil {
		return err
	}

	pollInterval, err := resolvePollInterval()
	if err != nil {
		return err
	}

	if os.Getenv("LABKIT_WORKER_BOOTSTRAP_ONLY") == "true" {
		log.Printf("labkit worker bootstrap ready for worker %q; bootstrap-only polling mode is explicitly enabled", resolvedWorkerID)

		loop := newLoop(loopConfig{
			workerID:     resolvedWorkerID,
			pollInterval: pollInterval,
			queue:        bootstrapQueue{},
			handler:      bootstrapHandler{},
		})
		return loop.Run(ctx)
	}

	runOnce := strings.EqualFold(strings.TrimSpace(os.Getenv("LABKIT_WORKER_RUN_ONCE")), "true")
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LABKIT_DEV_MODE")), "true") && strings.EqualFold(strings.TrimSpace(os.Getenv("LABKIT_WORKER_DEV_FAKE_EVALUATION")), "true") {
		return runDevWorker(ctx, devWorkerOptions{
			workerID:     resolvedWorkerID,
			pollInterval: pollInterval,
			runOnce:      runOnce,
		})
	}

	return runRealWorker(ctx, realWorkerOptions{
		workerID:      resolvedWorkerID,
		pollInterval:  pollInterval,
		runOnce:       runOnce,
		workspaceRoot: strings.TrimSpace(os.Getenv("LABKIT_WORKER_WORKSPACE_ROOT")),
		memoryLimit:   strings.TrimSpace(os.Getenv("LABKIT_WORKER_MEMORY_LIMIT")),
		cpuLimit:      strings.TrimSpace(os.Getenv("LABKIT_WORKER_CPU_LIMIT")),
	})
}

func defaultRunDevWorker(ctx context.Context, opts devWorkerOptions) error {
	if strings.TrimSpace(opts.workerID) == "" {
		return fmt.Errorf("worker id is required")
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	artifactRoot := strings.TrimSpace(os.Getenv("LABKIT_ARTIFACT_ROOT"))
	if artifactRoot == "" {
		artifactRoot = "artifacts"
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	queue := jobs.New(pool)
	handler := &devFakeHandler{
		workerID:     opts.workerID,
		queue:        queue,
		queries:      sqlc.New(pool),
		persister:    evaluation.NewService(evaluation.NewRepository(pool)),
		artifactRoot: artifactRoot,
		now:          time.Now,
	}

	if opts.runOnce {
		log.Printf("labkit worker dev fake evaluation processing one job for worker %q", opts.workerID)
		return runOneShot(ctx, queue, opts.workerID, handler)
	}

	log.Printf("labkit worker dev fake evaluation ready for worker %q", opts.workerID)

	loop := newLoop(loopConfig{
		workerID:     opts.workerID,
		pollInterval: opts.pollInterval,
		queue:        queue,
		handler:      handler,
	})
	return loop.Run(ctx)
}

func defaultRunRealWorker(ctx context.Context, opts realWorkerOptions) error {
	if strings.TrimSpace(opts.workerID) == "" {
		return fmt.Errorf("worker id is required")
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	artifactRoot := strings.TrimSpace(os.Getenv("LABKIT_ARTIFACT_ROOT"))
	if artifactRoot == "" {
		artifactRoot = "artifacts"
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	queue := jobs.New(pool)
	handler := &runtimeHandler{
		workerID:     opts.workerID,
		queue:        queue,
		queries:      sqlc.New(pool),
		persister:    evaluation.NewService(evaluation.NewRepository(pool)),
		artifactRoot: artifactRoot,
		now:          time.Now,
		runner: runtime.DockerRunner{
			Memory: opts.memoryLimit,
			CPUs:   opts.cpuLimit,
			Workspaces: runtime.TempDirManager{
				Root: opts.workspaceRoot,
			},
		},
		imageInspector: dockerImageInspector{},
	}

	if opts.runOnce {
		log.Printf("labkit worker processing one job for worker %q", opts.workerID)
		return runOneShot(ctx, queue, opts.workerID, handler)
	}

	log.Printf("labkit worker ready for worker %q", opts.workerID)

	loop := newLoop(loopConfig{
		workerID:     opts.workerID,
		pollInterval: opts.pollInterval,
		queue:        queue,
		handler:      handler,
	})
	return loop.Run(ctx)
}

func runOneShot(ctx context.Context, queue runtime.JobClaimer, workerID string, handler runtime.JobHandler) error {
	if strings.TrimSpace(workerID) == "" {
		return jobs.ErrInvalidWorkerID
	}
	if queue == nil {
		return fmt.Errorf("runtime: queue is required")
	}
	if handler == nil {
		return fmt.Errorf("runtime: handler is required")
	}

	job, err := queue.Claim(ctx, workerID)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}
	return handler.Handle(ctx, job)
}

func resolveWorkerID(workerID string) (string, error) {
	if workerID != "" {
		return workerID, nil
	}

	host, err := os.Hostname()
	if err != nil || host == "" {
		return "", fmt.Errorf("LABKIT_WORKER_ID is required")
	}
	return host, nil
}

func resolvePollInterval() (time.Duration, error) {
	raw := os.Getenv("LABKIT_WORKER_POLL_INTERVAL_MS")
	if raw == "" {
		return time.Second, nil
	}

	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		return 0, fmt.Errorf("invalid LABKIT_WORKER_POLL_INTERVAL_MS: %q", raw)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

type bootstrapQueue struct{}

func (bootstrapQueue) Claim(context.Context, string) (*sqlc.EvaluationJobs, error) {
	return nil, nil
}

type bootstrapHandler struct{}

func (bootstrapHandler) Handle(context.Context, *sqlc.EvaluationJobs) error {
	return nil
}
