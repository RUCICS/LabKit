#!/bin/sh
set -eu

export PGPASSWORD="${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
POSTGRES_HOST="${POSTGRES_HOST:-postgres}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
LABKIT_MIGRATIONS_DIR="${LABKIT_MIGRATIONS_DIR:-/migrations}"

until pg_isready -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; do
  sleep 1
done

if [ "$#" -gt 0 ]; then
  exec labkit-migrate "$@"
fi

exec labkit-migrate up
