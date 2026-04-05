package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"labkit.local/apps/migrate/internal/cli"
	"labkit.local/apps/migrate/internal/runner"
	dbpkg "labkit.local/packages/go/db"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:]))
}

func run(ctx context.Context, args []string) int {
	cfg, err := dbpkg.ResolvePoolConfigFromEnv(os.Getenv)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "resolve database config: %v\n", err)
		return 1
	}

	migrationsDir, err := resolveMigrationsDir()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "resolve migrations dir: %v\n", err)
		return 1
	}

	engine := runner.New(runner.Config{
		DatabaseURL:   cfg.ConnString(),
		MigrationsDir: migrationsDir,
	})
	if err := cli.Execute(ctx, os.Stdout, os.Stderr, engine, args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "labkit-migrate: %v\n", err)
		return 1
	}
	return 0
}

func resolveMigrationsDir() (string, error) {
	candidates := []string{
		strings.TrimSpace(os.Getenv("LABKIT_MIGRATIONS_DIR")),
		strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")),
		"/migrations",
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "db", "migrations"),
			filepath.Join(wd, "..", "..", "..", "db", "migrations"),
		)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no migrations directory found; set LABKIT_MIGRATIONS_DIR")
}
