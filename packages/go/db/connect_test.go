package db

import (
	"strings"
	"testing"
)

func TestResolvePoolConfigFromEnvBuildsConfigFromSplitPostgresEnv(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"POSTGRES_HOST":     "postgres",
		"POSTGRES_PORT":     "5432",
		"POSTGRES_DB":       "labkit",
		"POSTGRES_USER":     "labkit",
		"POSTGRES_PASSWORD": "G/29UewN(1Bo[H6X@foo:bar?baz&qux=#",
		"POSTGRES_SSLMODE":  "disable",
	}

	cfg, err := ResolvePoolConfigFromEnv(envLookup(env))
	if err != nil {
		t.Fatalf("ResolvePoolConfigFromEnv() error = %v", err)
	}

	if got, want := cfg.ConnConfig.Host, "postgres"; got != want {
		t.Fatalf("Host = %q, want %q", got, want)
	}
	if got, want := cfg.ConnConfig.Port, uint16(5432); got != want {
		t.Fatalf("Port = %d, want %d", got, want)
	}
	if got, want := cfg.ConnConfig.Database, "labkit"; got != want {
		t.Fatalf("Database = %q, want %q", got, want)
	}
	if got, want := cfg.ConnConfig.User, "labkit"; got != want {
		t.Fatalf("User = %q, want %q", got, want)
	}
	if got, want := cfg.ConnConfig.Password, env["POSTGRES_PASSWORD"]; got != want {
		t.Fatalf("Password = %q, want %q", got, want)
	}
}

func TestResolvePoolConfigFromEnvPrefersDatabaseURL(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"DATABASE_URL":      "postgres://url-user:url-pass@db.internal:5433/url-db?sslmode=disable",
		"POSTGRES_HOST":     "postgres",
		"POSTGRES_PORT":     "5432",
		"POSTGRES_DB":       "labkit",
		"POSTGRES_USER":     "labkit",
		"POSTGRES_PASSWORD": "ignored",
	}

	cfg, err := ResolvePoolConfigFromEnv(envLookup(env))
	if err != nil {
		t.Fatalf("ResolvePoolConfigFromEnv() error = %v", err)
	}

	if got, want := cfg.ConnConfig.Host, "db.internal"; got != want {
		t.Fatalf("Host = %q, want %q", got, want)
	}
	if got, want := cfg.ConnConfig.Port, uint16(5433); got != want {
		t.Fatalf("Port = %d, want %d", got, want)
	}
	if got, want := cfg.ConnConfig.Database, "url-db"; got != want {
		t.Fatalf("Database = %q, want %q", got, want)
	}
	if got, want := cfg.ConnConfig.User, "url-user"; got != want {
		t.Fatalf("User = %q, want %q", got, want)
	}
	if got, want := cfg.ConnConfig.Password, "url-pass"; got != want {
		t.Fatalf("Password = %q, want %q", got, want)
	}
}

func TestResolvePoolConfigFromEnvRejectsInvalidPort(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"POSTGRES_HOST": "postgres",
		"POSTGRES_PORT": "bad-port",
		"POSTGRES_DB":   "labkit",
		"POSTGRES_USER": "labkit",
	}

	_, err := ResolvePoolConfigFromEnv(envLookup(env))
	if err == nil {
		t.Fatal("ResolvePoolConfigFromEnv() error = nil, want invalid port error")
	}
	if !strings.Contains(err.Error(), "POSTGRES_PORT") {
		t.Fatalf("error = %v, want POSTGRES_PORT message", err)
	}
}

func envLookup(env map[string]string) func(string) string {
	return func(key string) string {
		return env[key]
	}
}
