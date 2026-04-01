#!/bin/sh
set -eu

export PGPASSWORD="${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
POSTGRES_HOST="${POSTGRES_HOST:-postgres}"

until pg_isready -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; do
  sleep 1
done

if [ "$(psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT to_regclass('public.users') IS NOT NULL")" != "t" ]; then
  psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < /migrations/0001_init.up.sql
fi

if [ "$(psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT to_regclass('public.evaluation_jobs') IS NOT NULL")" != "t" ]; then
  psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < /migrations/0002_uuidv7_and_jobs.up.sql
fi

if [ "$(psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT to_regclass('public.web_session_tickets') IS NOT NULL")" != "t" ]; then
  psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < /migrations/0003_web_session_tickets.up.sql
fi

if [ "$(psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'user_keys' AND column_name = 'revoked_at')")" != "t" ]; then
  psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < /migrations/0004_user_keys_soft_delete.up.sql
fi
