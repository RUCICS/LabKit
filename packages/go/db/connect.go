package db

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResolvePoolConfigFromEnv resolves a pgx pool config from environment
// variables. DATABASE_URL takes precedence; otherwise the config is built from
// split POSTGRES_* variables so passwords with reserved URL characters are
// encoded safely.
func ResolvePoolConfigFromEnv(lookup func(string) string) (*pgxpool.Config, error) {
	if lookup == nil {
		return nil, fmt.Errorf("environment lookup is required")
	}

	databaseURL := strings.TrimSpace(lookup("DATABASE_URL"))
	if databaseURL != "" {
		return pgxpool.ParseConfig(databaseURL)
	}

	host := strings.TrimSpace(lookup("POSTGRES_HOST"))
	if host == "" {
		return nil, fmt.Errorf("DATABASE_URL or POSTGRES_HOST is required")
	}

	portText := strings.TrimSpace(lookup("POSTGRES_PORT"))
	if portText == "" {
		portText = "5432"
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_PORT %q: %w", portText, err)
	}

	database := strings.TrimSpace(lookup("POSTGRES_DB"))
	if database == "" {
		return nil, fmt.Errorf("POSTGRES_DB is required when DATABASE_URL is unset")
	}

	user := strings.TrimSpace(lookup("POSTGRES_USER"))
	if user == "" {
		return nil, fmt.Errorf("POSTGRES_USER is required when DATABASE_URL is unset")
	}

	sslMode := strings.TrimSpace(lookup("POSTGRES_SSLMODE"))
	if sslMode == "" {
		sslMode = "disable"
	}

	connectionURL := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, lookup("POSTGRES_PASSWORD")),
		Host:     net.JoinHostPort(host, portText),
		Path:     "/" + database,
		RawQuery: url.Values{"sslmode": []string{sslMode}}.Encode(),
	}).String()

	cfg, err := pgxpool.ParseConfig(connectionURL)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.Port = uint16(port)
	return cfg, nil
}
