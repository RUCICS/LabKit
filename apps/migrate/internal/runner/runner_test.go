package runner

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	testDBUser     = "postgres"
	testDBPassword = "postgres"
	testDBName     = "labkit"
)

func TestUpAppliesAllMigrationsOnCleanDatabase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env := openTestEnv(t)

	r := New(Config{
		DatabaseURL:   env.databaseURL,
		MigrationsDir: migrationsDir(t),
	})

	if err := r.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	version, dirty, err := r.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if got, want := version, uint(7); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
	if dirty {
		t.Fatal("dirty = true, want false")
	}
	assertTableExists(t, ctx, env.pool, "labs")
	assertTableExists(t, ctx, env.pool, "evaluation_jobs")
	assertTableExists(t, ctx, env.pool, "web_session_tickets")
}

func TestUpRejectsLegacyDatabaseWithoutVersionTable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env := openTestEnv(t)
	applyLegacyMigrations(t, ctx, env.pool)

	r := New(Config{
		DatabaseURL:   env.databaseURL,
		MigrationsDir: migrationsDir(t),
	})

	err := r.Up(ctx)
	if err == nil {
		t.Fatal("Up() error = nil, want baseline guidance")
	}
	if !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("Up() error = %v, want baseline guidance", err)
	}
	assertVersionTableAbsent(t, ctx, env.pool)
}

func TestBaselineMarksLegacyDatabaseWithoutReplayingMigrations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env := openTestEnv(t)
	applyLegacyMigrations(t, ctx, env.pool)

	r := New(Config{
		DatabaseURL:   env.databaseURL,
		MigrationsDir: migrationsDir(t),
	})

	if err := r.Baseline(ctx, 7); err != nil {
		t.Fatalf("Baseline() error = %v", err)
	}

	version, dirty, err := r.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if got, want := version, uint(7); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
	if dirty {
		t.Fatal("dirty = true, want false")
	}
	if err := r.Up(ctx); err != nil {
		t.Fatalf("Up() after Baseline() error = %v", err)
	}
}

func TestBaselineRejectsCleanDatabase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env := openTestEnv(t)

	r := New(Config{
		DatabaseURL:   env.databaseURL,
		MigrationsDir: migrationsDir(t),
	})

	err := r.Baseline(ctx, 7)
	if err == nil {
		t.Fatal("Baseline() error = nil, want rejection")
	}
	if !strings.Contains(err.Error(), "clean") {
		t.Fatalf("Baseline() error = %v, want clean database message", err)
	}
}

func TestVersionDoesNotCreateVersionTableOnCleanDatabase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env := openTestEnv(t)

	r := New(Config{
		DatabaseURL:   env.databaseURL,
		MigrationsDir: migrationsDir(t),
	})

	version, dirty, err := r.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version != 0 || dirty {
		t.Fatalf("Version() = (%d, %t), want (0, false)", version, dirty)
	}
	assertVersionTableAbsent(t, ctx, env.pool)
}

func TestVersionDoesNotCreateVersionTableOnLegacyDatabase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env := openTestEnv(t)
	applyLegacyMigrations(t, ctx, env.pool)

	r := New(Config{
		DatabaseURL:   env.databaseURL,
		MigrationsDir: migrationsDir(t),
	})

	version, dirty, err := r.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version != 0 || dirty {
		t.Fatalf("Version() = (%d, %t), want (0, false)", version, dirty)
	}
	assertVersionTableAbsent(t, ctx, env.pool)
}

type testEnv struct {
	databaseURL string
	pool        *pgxpool.Pool
}

func openTestEnv(t *testing.T) testEnv {
	t.Helper()

	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort() error = %v", err)
	}

	runtimeDir, err := os.MkdirTemp("", "labkit-migrate-pg-runtime-*")
	if err != nil {
		t.Fatalf("MkdirTemp(runtime) error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(runtimeDir) })

	dataDir, err := os.MkdirTemp("", "labkit-migrate-pg-data-*")
	if err != nil {
		t.Fatalf("MkdirTemp(data) error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

	db := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Username(testDBUser).
			Password(testDBPassword).
			Database(testDBName).
			Port(uint32(port)).
			Version(embeddedpostgres.V16).
			RuntimePath(runtimeDir).
			DataPath(dataDir),
	)
	if err := db.Start(); err != nil {
		t.Fatalf("embedded postgres start error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Stop()
	})

	databaseURL := fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable", testDBUser, testDBPassword, port, testDBName)
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(pool.Close)

	return testEnv{
		databaseURL: databaseURL,
		pool:        pool,
	}
}

func applyLegacyMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE OR REPLACE FUNCTION uuidv7() RETURNS uuid
		LANGUAGE sql
		AS $$ SELECT gen_random_uuid(); $$;
	`); err != nil {
		t.Fatalf("bootstrap uuidv7() error = %v", err)
	}

	paths, err := filepath.Glob(filepath.Join(migrationsDir(t), "*.up.sql"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply legacy migration %q error = %v", path, err)
		}
	}
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", "..", "db", "migrations"))
}

func assertTableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, table).Scan(&exists); err != nil {
		t.Fatalf("table existence query for %q error = %v", table, err)
	}
	if !exists {
		t.Fatalf("table %q does not exist", table)
	}
}

func assertVersionTableAbsent(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("schema_migrations existence query error = %v", err)
	}
	if exists {
		t.Fatal("schema_migrations table exists, want absent")
	}
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
