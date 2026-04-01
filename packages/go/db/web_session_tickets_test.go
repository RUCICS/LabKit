package db

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5"
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

func TestWebSessionTicketLifecycle(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t, ctx)
	resetTestDatabase(t, ctx, pool)

	now := time.Now().UTC()
	expiresAt := now.Add(time.Minute).Truncate(time.Microsecond)
	ticketHash := "ticket-hash-123"

	row := pool.QueryRow(ctx, `
		INSERT INTO web_session_tickets (ticket_hash, user_id, key_id, redirect_path, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ticket_hash, user_id, key_id, redirect_path, expires_at
	`, ticketHash, int64(1), int64(1), "/auth/confirm", expiresAt)

	var createdHash, createdPath string
	var createdUserID, createdKeyID int64
	var createdExpiresAt time.Time
	if err := row.Scan(&createdHash, &createdUserID, &createdKeyID, &createdPath, &createdExpiresAt); err != nil {
		t.Fatalf("insert ticket error = %v", err)
	}
	if createdHash != ticketHash {
		t.Fatalf("ticket_hash = %q, want %q", createdHash, ticketHash)
	}
	if createdUserID != 1 || createdKeyID != 1 {
		t.Fatalf("ticket ownership = (%d, %d), want (1, 1)", createdUserID, createdKeyID)
	}
	if createdPath != "/auth/confirm" {
		t.Fatalf("redirect_path = %q, want %q", createdPath, "/auth/confirm")
	}
	if !createdExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v, want %v", createdExpiresAt, expiresAt)
	}

	consumeRow := pool.QueryRow(ctx, `
		DELETE FROM web_session_tickets
		WHERE ticket_hash = $1
		  AND expires_at > NOW()
		RETURNING user_id, key_id, redirect_path, expires_at
	`, ticketHash)

	var consumedUserID, consumedKeyID int64
	var consumedPath string
	var consumedExpiresAt time.Time
	if err := consumeRow.Scan(&consumedUserID, &consumedKeyID, &consumedPath, &consumedExpiresAt); err != nil {
		t.Fatalf("consume ticket error = %v", err)
	}
	if consumedUserID != 1 || consumedKeyID != 1 {
		t.Fatalf("consumed ownership = (%d, %d), want (1, 1)", consumedUserID, consumedKeyID)
	}
	if consumedPath != "/auth/confirm" {
		t.Fatalf("consumed redirect_path = %q, want %q", consumedPath, "/auth/confirm")
	}
	if !consumedExpiresAt.Equal(expiresAt) {
		t.Fatalf("consumed expires_at = %v, want %v", consumedExpiresAt, expiresAt)
	}

	replayRow := pool.QueryRow(ctx, `
		DELETE FROM web_session_tickets
		WHERE ticket_hash = $1
		  AND expires_at > NOW()
		RETURNING user_id
	`, ticketHash)
	if err := replayRow.Scan(new(int64)); !errorsIsNoRows(err) {
		t.Fatalf("replay consume error = %v, want pgx.ErrNoRows", err)
	}

	expiredHash := "ticket-hash-456"
	_, err := pool.Exec(ctx, `
		INSERT INTO web_session_tickets (ticket_hash, user_id, key_id, redirect_path, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		`, expiredHash, int64(1), int64(1), "/auth/confirm", now.Add(-time.Minute).Truncate(time.Microsecond))
	if err != nil {
		t.Fatalf("insert expired ticket error = %v", err)
	}

	expiredRow := pool.QueryRow(ctx, `
		DELETE FROM web_session_tickets
		WHERE ticket_hash = $1
		  AND expires_at > NOW()
		RETURNING user_id
	`, expiredHash)
	if err := expiredRow.Scan(new(int64)); !errorsIsNoRows(err) {
		t.Fatalf("consume expired ticket error = %v, want pgx.ErrNoRows", err)
	}
}

func TestCleanupExpiredWebSessionTickets(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t, ctx)
	resetTestDatabase(t, ctx, pool)

	store := New(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiredHash := "ticket-expired"
	activeHash := "ticket-active"

	seedTicket := func(ticketHash string, expiresAt time.Time) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO web_session_tickets (ticket_hash, user_id, key_id, redirect_path, expires_at)
			VALUES ($1, $2, $3, $4, $5)
		`, ticketHash, int64(1), int64(1), "/auth/confirm", expiresAt); err != nil {
			t.Fatalf("seed ticket %q error = %v", ticketHash, err)
		}
	}

	seedTicket(expiredHash, now.Add(-time.Minute))
	seedTicket(activeHash, now.Add(time.Minute))

	if err := store.CleanupExpiredWebSessionTickets(ctx); err != nil {
		t.Fatalf("CleanupExpiredWebSessionTickets() error = %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM web_session_tickets WHERE ticket_hash = $1`, expiredHash).Scan(&count); err != nil {
		t.Fatalf("count expired ticket error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expired ticket rows = %d, want 0", count)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM web_session_tickets WHERE ticket_hash = $1`, activeHash).Scan(&count); err != nil {
		t.Fatalf("count active ticket error = %v", err)
	}
	if count != 1 {
		t.Fatalf("active ticket rows = %d, want 1", count)
	}
}

func TestDeletingUserKeyCascadesWebSessionTickets(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t, ctx)
	resetTestDatabase(t, ctx, pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := pool.Exec(ctx, `
		INSERT INTO web_session_tickets (ticket_hash, user_id, key_id, redirect_path, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, "ticket-cascade", int64(1), int64(1), "/auth/confirm", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("seed ticket error = %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM user_keys WHERE id = $1`, int64(1)); err != nil {
		t.Fatalf("delete user key error = %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM web_session_tickets WHERE key_id = $1`, int64(1)).Scan(&count); err != nil {
		t.Fatalf("count tickets after key delete error = %v", err)
	}
	if count != 0 {
		t.Fatalf("tickets remaining after key delete = %d, want 0", count)
	}
}

func TestMain(m *testing.M) {
	code := func() int {
		port, err := freePort()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pick postgres port: %v\n", err)
			return 1
		}

		runtimeDir, err := os.MkdirTemp("", "labkit-db-pg-runtime-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create runtime dir: %v\n", err)
			return 1
		}
		defer os.RemoveAll(runtimeDir)

		dataDir, err := os.MkdirTemp("", "labkit-db-pg-data-*")
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

	stmt := `
DO $$
BEGIN
  IF to_regclass('public.web_session_tickets') IS NOT NULL THEN
    EXECUTE 'TRUNCATE TABLE web_session_tickets, user_keys, users, labs RESTART IDENTITY CASCADE';
  ELSE
    EXECUTE 'TRUNCATE TABLE user_keys, users, labs RESTART IDENTITY CASCADE';
  END IF;
END $$;`
	if _, err := pool.Exec(ctx, stmt); err != nil {
		t.Fatalf("reset database error = %v", err)
	}

	statements := []string{
		"INSERT INTO labs (id, name, manifest) VALUES ('lab-1', 'Lab 1', '{}'::jsonb)",
		"INSERT INTO users (student_id) VALUES ('s1234567')",
		"INSERT INTO user_keys (user_id, public_key, device_name) VALUES (1, 'pk-1', 'device-1')",
	}
	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed statement %q error = %v", stmt, err)
		}
	}
}

func applyMigrations(ctx context.Context) error {
	root := repoRoot()
	migrationPaths, err := filepath.Glob(filepath.Join(root, "db", "migrations", "*up.sql"))
	if err != nil {
		return err
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

	for _, path := range migrationPaths {
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

func errorsIsNoRows(err error) bool {
	return err != nil && err == pgx.ErrNoRows
}
