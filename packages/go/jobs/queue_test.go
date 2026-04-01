package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"labkit.local/packages/go/jobs"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	testDBUser     = "postgres"
	testDBPassword = "postgres"
	testDBName     = "labkit"
)

var (
	testPostgres    *embeddedpostgres.EmbeddedPostgres
	testDatabaseURL string
)

func TestQueueEnqueueJob(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)

	job, err := queue.Enqueue(tctx, submissionID)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if job.SubmissionID != submissionID {
		t.Fatalf("Enqueue() submission_id = %v, want %v", job.SubmissionID, submissionID)
	}
	if job.Status != jobs.StatusQueued {
		t.Fatalf("Enqueue() status = %q, want %q", job.Status, jobs.StatusQueued)
	}
	if job.Attempts != 0 {
		t.Fatalf("Enqueue() attempts = %d, want 0", job.Attempts)
	}
}

func TestQueueClaimSingleWorker(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	enqueued, err := queue.Enqueue(tctx, submissionID)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if job == nil {
		t.Fatal("Claim() returned nil job")
	}
	if job.ID != enqueued.ID {
		t.Fatalf("Claim() job id = %v, want %v", job.ID, enqueued.ID)
	}
	if job.Status != jobs.StatusRunning {
		t.Fatalf("Claim() status = %q, want %q", job.Status, jobs.StatusRunning)
	}
	if job.Attempts != 1 {
		t.Fatalf("Claim() attempts = %d, want 1", job.Attempts)
	}
	if !job.WorkerID.Valid || job.WorkerID.String != "worker-a" {
		t.Fatalf("Claim() worker_id = %#v, want worker-a", job.WorkerID)
	}
}

func TestQueueClaimRejectsEmptyWorkerID(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	if _, err := queue.Enqueue(tctx, submissionID); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	if _, err := queue.Claim(tctx, "   "); !errors.Is(err, jobs.ErrInvalidWorkerID) {
		t.Fatalf("Claim() error = %v, want ErrInvalidWorkerID", err)
	}
}

func TestQueueClaimSkipsLockedRowsAcrossWorkers(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	firstSubmission := seedSubmission(t, tctx, pool)
	secondSubmission := seedSubmission(t, tctx, pool)

	firstJob, err := queue.Enqueue(tctx, firstSubmission)
	if err != nil {
		t.Fatalf("Enqueue(first) error = %v", err)
	}
	secondJob, err := queue.Enqueue(tctx, secondSubmission)
	if err != nil {
		t.Fatalf("Enqueue(second) error = %v", err)
	}

	lockerConn, err := pool.Acquire(tctx)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer lockerConn.Release()

	lockTx, err := lockerConn.Begin(tctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() {
		_ = lockTx.Rollback(tctx)
	}()

	var lockedID uuid.UUID
	row := lockTx.QueryRow(
		tctx,
		`SELECT id
		FROM evaluation_jobs
		WHERE status = 'queued' AND available_at <= NOW()
		ORDER BY available_at, created_at
		LIMIT 1
		FOR UPDATE`,
	)
	if err := row.Scan(&lockedID); err != nil {
		t.Fatalf("locking query error = %v", err)
	}
	if lockedID != firstJob.ID {
		t.Fatalf("locked job id = %v, want %v", lockedID, firstJob.ID)
	}

	claimed, err := queue.Claim(tctx, "worker-b")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if claimed == nil {
		t.Fatal("Claim() returned nil job")
	}
	if claimed.ID != secondJob.ID {
		t.Fatalf("Claim() job id = %v, want %v", claimed.ID, secondJob.ID)
	}
	if claimed.ID == firstJob.ID {
		t.Fatalf("Claim() picked locked job %v", claimed.ID)
	}
}

func TestQueueRetryAfterFailureSchedulesFutureClaim(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	_, err := queue.Enqueue(tctx, submissionID)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if job == nil {
		t.Fatal("Claim() returned nil job")
	}

	retryAt := time.Now().Add(2 * time.Minute).UTC()
	if err := queue.Retry(tctx, job.ID, "worker-a", retryAt, "temporary failure"); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	next, err := queue.Claim(tctx, "worker-b")
	if err != nil {
		t.Fatalf("Claim() before retry window error = %v", err)
	}
	if next != nil {
		t.Fatalf("Claim() before retry window = %#v, want nil", next)
	}

	if _, err := pool.Exec(tctx, `UPDATE evaluation_jobs SET available_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, job.ID); err != nil {
		t.Fatalf("force available_at into past error = %v", err)
	}

	retried, err := queue.Claim(tctx, "worker-c")
	if err != nil {
		t.Fatalf("Claim() after retry window error = %v", err)
	}
	if retried == nil {
		t.Fatal("Claim() after retry window returned nil job")
	}
	if retried.ID != job.ID {
		t.Fatalf("Claim() retried id = %v, want %v", retried.ID, job.ID)
	}
	if retried.Attempts != 2 {
		t.Fatalf("Claim() retried attempts = %d, want 2", retried.Attempts)
	}
}

func TestQueueAcknowledgeMarksClaimedJobTerminal(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	_, err := queue.Enqueue(tctx, submissionID)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	if err := queue.Acknowledge(tctx, job.ID, "worker-a", jobs.StatusDone, ""); err != nil {
		t.Fatalf("Acknowledge() error = %v", err)
	}

	row := pool.QueryRow(tctx, `SELECT status, worker_id, finished_at FROM evaluation_jobs WHERE id = $1`, job.ID)
	var status string
	var workerID *string
	var finishedAt time.Time
	if err := row.Scan(&status, &workerID, &finishedAt); err != nil {
		t.Fatalf("scan acknowledged job error = %v", err)
	}
	if status != jobs.StatusDone {
		t.Fatalf("status = %q, want %q", status, jobs.StatusDone)
	}
	if workerID == nil || *workerID != "worker-a" {
		t.Fatalf("worker_id = %v, want worker-a", workerID)
	}
	if finishedAt.IsZero() {
		t.Fatal("finished_at was not set")
	}
}

func TestQueueAcknowledgeRejectsNonTerminalStatus(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	_, err := queue.Enqueue(tctx, submissionID)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	if err := queue.Acknowledge(tctx, job.ID, "worker-a", jobs.StatusRunning, ""); !errors.Is(err, jobs.ErrInvalidTerminalStatus) {
		t.Fatalf("Acknowledge() error = %v, want ErrInvalidTerminalStatus", err)
	}
}

func TestQueueAcknowledgeRejectsEmptyWorkerID(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	if _, err := queue.Enqueue(tctx, submissionID); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	if err := queue.Acknowledge(tctx, job.ID, "", jobs.StatusDone, ""); !errors.Is(err, jobs.ErrInvalidWorkerID) {
		t.Fatalf("Acknowledge() error = %v, want ErrInvalidWorkerID", err)
	}
}

func TestQueueRejectsStaleWorkerTransitionsAfterRetryAndReclaim(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	_, err := queue.Enqueue(tctx, submissionID)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim(worker-a) error = %v", err)
	}
	if err := queue.Retry(tctx, job.ID, "worker-a", time.Now().Add(-time.Minute).UTC(), "retry now"); err != nil {
		t.Fatalf("Retry(worker-a) error = %v", err)
	}

	reclaimed, err := queue.Claim(tctx, "worker-b")
	if err != nil {
		t.Fatalf("Claim(worker-b) error = %v", err)
	}
	if reclaimed == nil {
		t.Fatal("Claim(worker-b) returned nil job")
	}

	if err := queue.Acknowledge(tctx, job.ID, "worker-a", jobs.StatusError, "stale"); !errors.Is(err, jobs.ErrTransitionNotAllowed) {
		t.Fatalf("Acknowledge() error = %v, want ErrTransitionNotAllowed", err)
	}
	if err := queue.Retry(tctx, job.ID, "worker-a", time.Now().Add(time.Minute).UTC(), "stale"); !errors.Is(err, jobs.ErrTransitionNotAllowed) {
		t.Fatalf("Retry() error = %v, want ErrTransitionNotAllowed", err)
	}
}

func TestQueueRetryRejectsEmptyWorkerID(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	if _, err := queue.Enqueue(tctx, submissionID); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	if err := queue.Retry(tctx, job.ID, "   ", time.Now().Add(time.Minute).UTC(), "temporary failure"); !errors.Is(err, jobs.ErrInvalidWorkerID) {
		t.Fatalf("Retry() error = %v, want ErrInvalidWorkerID", err)
	}
}

func TestQueueRetryClearsRunningMetadata(t *testing.T) {
	tctx := context.Background()
	pool := openTestPool(t, tctx)
	resetTestDatabase(t, tctx, pool)

	queue := jobs.New(pool)
	submissionID := seedSubmission(t, tctx, pool)
	_, err := queue.Enqueue(tctx, submissionID)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	job, err := queue.Claim(tctx, "worker-a")
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	retryAt := time.Now().Add(time.Minute).UTC()
	if err := queue.Retry(tctx, job.ID, "worker-a", retryAt, "temporary failure"); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	row := pool.QueryRow(tctx, `SELECT status, worker_id, started_at, last_error, available_at FROM evaluation_jobs WHERE id = $1`, job.ID)
	var status string
	var workerID *string
	var startedAt *time.Time
	var lastError *string
	var availableAt time.Time
	if err := row.Scan(&status, &workerID, &startedAt, &lastError, &availableAt); err != nil {
		t.Fatalf("scan retried job error = %v", err)
	}
	if status != jobs.StatusQueued {
		t.Fatalf("status = %q, want %q", status, jobs.StatusQueued)
	}
	if workerID != nil {
		t.Fatalf("worker_id = %v, want nil", workerID)
	}
	if startedAt != nil {
		t.Fatalf("started_at = %v, want nil", startedAt)
	}
	if lastError == nil || *lastError != "temporary failure" {
		t.Fatalf("last_error = %v, want temporary failure", lastError)
	}
	if availableAt.Before(retryAt.Add(-time.Second)) || availableAt.After(retryAt.Add(time.Second)) {
		t.Fatalf("available_at = %v, want near %v", availableAt, retryAt)
	}
}

func TestMain(m *testing.M) {
	code := func() int {
		port, err := freePort()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pick postgres port: %v\n", err)
			return 1
		}

		runtimeDir, err := os.MkdirTemp("", "labkit-jobs-pg-runtime-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create runtime dir: %v\n", err)
			return 1
		}
		defer os.RemoveAll(runtimeDir)

		dataDir, err := os.MkdirTemp("", "labkit-jobs-pg-data-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create data dir: %v\n", err)
			return 1
		}
		defer os.RemoveAll(dataDir)

		testPostgres = embeddedpostgres.NewDatabase(
			embeddedpostgres.DefaultConfig().
				Username(testDBUser).
				Password(testDBPassword).
				Database(testDBName).
				Port(uint32(port)).
				Version(embeddedpostgres.V16).
				RuntimePath(runtimeDir).
				DataPath(dataDir),
		)
		if err := testPostgres.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "start embedded postgres: %v\n", err)
			return 1
		}
		defer func() {
			_ = testPostgres.Stop()
		}()

		testDatabaseURL = fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable", testDBUser, testDBPassword, port, testDBName)

		if err := applyMigrations(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "apply migrations: %v\n", err)
			return 1
		}

		return m.Run()
	}()

	os.Exit(code)
}

func openTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func resetTestDatabase(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	statements := []string{
		"TRUNCATE TABLE evaluation_jobs, leaderboard, scores, submissions, lab_profiles, user_keys, users, labs RESTART IDENTITY CASCADE",
		"INSERT INTO labs (id, name, manifest) VALUES ('lab-1', 'Lab 1', '{}'::jsonb)",
		"INSERT INTO users (student_id) VALUES ('s1234567')",
		"INSERT INTO user_keys (user_id, public_key, device_name) VALUES (1, 'pk-1', 'device-1')",
	}

	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("reset statement %q error = %v", stmt, err)
		}
	}
}

func seedSubmission(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	submissionID := uuid.New()
	_, err := pool.Exec(
		ctx,
		`INSERT INTO submissions (
			id, user_id, lab_id, key_id, artifact_key, content_hash, status
		) VALUES ($1, 1, 'lab-1', 1, 'artifact', 'hash', 'queued')`,
		submissionID,
	)
	if err != nil {
		t.Fatalf("insert submission error = %v", err)
	}
	return submissionID
}

func applyMigrations(ctx context.Context) error {
	root := repoRoot()
	migrations := []string{
		filepath.Join(root, "db/migrations/0001_init.up.sql"),
		filepath.Join(root, "db/migrations/0002_uuidv7_and_jobs.up.sql"),
	}

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE OR REPLACE FUNCTION uuidv7() RETURNS uuid
		LANGUAGE sql
		AS $$ SELECT gen_random_uuid(); $$;
	`); err != nil {
		return err
	}

	for _, path := range migrations {
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", path, err)
		}
	}

	return nil
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %T", listener.Addr())
	}
	return addr.Port, nil
}
