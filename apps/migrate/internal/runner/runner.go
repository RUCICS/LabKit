package runner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const versionTableName = "schema_migrations"

type Config struct {
	DatabaseURL   string
	MigrationsDir string
}

type Runner struct {
	cfg Config
}

func New(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

func (r *Runner) Up(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, r.cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	hasVersionTable, err := r.hasVersionTable(ctx, pool)
	if err != nil {
		return err
	}
	if !hasVersionTable {
		hasUserTables, err := r.hasUserTables(ctx, pool)
		if err != nil {
			return err
		}
		if hasUserTables {
			return fmt.Errorf("database has existing tables but no %s table; run baseline --version 7 once before up", versionTableName)
		}
	}

	if err := ensureUUIDSupport(ctx, pool); err != nil {
		return err
	}

	m, err := r.newMigrator()
	if err != nil {
		return err
	}
	defer closeMigrator(m)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func (r *Runner) Baseline(ctx context.Context, version uint) error {
	pool, err := pgxpool.New(ctx, r.cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	hasVersionTable, err := r.hasVersionTable(ctx, pool)
	if err != nil {
		return err
	}
	if hasVersionTable {
		return fmt.Errorf("%s table already exists; baseline is only for unmanaged legacy databases", versionTableName)
	}

	hasUserTables, err := r.hasUserTables(ctx, pool)
	if err != nil {
		return err
	}
	if !hasUserTables {
		return fmt.Errorf("cannot baseline a clean database; run up instead")
	}

	m, err := r.newMigrator()
	if err != nil {
		return err
	}
	defer closeMigrator(m)

	if err := m.Force(int(version)); err != nil {
		return fmt.Errorf("set baseline version: %w", err)
	}
	return nil
}

func (r *Runner) Version(ctx context.Context) (uint, bool, error) {
	pool, err := pgxpool.New(ctx, r.cfg.DatabaseURL)
	if err != nil {
		return 0, false, fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	hasVersionTable, err := r.hasVersionTable(ctx, pool)
	if err != nil {
		return 0, false, err
	}
	if !hasVersionTable {
		return 0, false, nil
	}

	m, err := r.newMigrator()
	if err != nil {
		return 0, false, err
	}
	defer closeMigrator(m)

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("read migration version: %w", err)
	}
	return version, dirty, nil
}

func (r *Runner) hasVersionTable(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return false, fmt.Errorf("check %s table: %w", versionTableName, err)
	}
	return exists, nil
}

func (r *Runner) hasUserTables(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_type = 'BASE TABLE'
			  AND table_name <> $1
		)
	`, versionTableName).Scan(&exists); err != nil {
		return false, fmt.Errorf("check existing public tables: %w", err)
	}
	return exists, nil
}

func (r *Runner) newMigrator() (*migrate.Migrate, error) {
	db, err := sql.Open("pgx", r.cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database/sql connection: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: versionTableName,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+r.cfg.MigrationsDir, "postgres", driver)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create migrator: %w", err)
	}
	return m, nil
}

func ensureUUIDSupport(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`); err != nil {
		return fmt.Errorf("ensure pgcrypto extension: %w", err)
	}
	if _, err := pool.Exec(ctx, `
DO $$
BEGIN
  IF to_regprocedure('uuidv7()') IS NULL THEN
    EXECUTE 'CREATE FUNCTION uuidv7() RETURNS uuid LANGUAGE sql AS ''SELECT gen_random_uuid();''';
  END IF;
END $$;
`); err != nil {
		return fmt.Errorf("ensure uuidv7() helper: %w", err)
	}
	return nil
}

func closeMigrator(m *migrate.Migrate) {
	sourceErr, databaseErr := m.Close()
	if databaseErr != nil {
		return
	}
	if sourceErr != nil {
		return
	}
}
