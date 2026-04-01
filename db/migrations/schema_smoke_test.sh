#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MIG_DIR="$ROOT_DIR/db/migrations"
CONTAINER_NAME="labkit-schema-smoke-$$"
DB_NAME="labkit"
DB_USER="postgres"
DB_PASSWORD="postgres"

cleanup() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

require_migration() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "missing migration: $path" >&2
    exit 1
  fi
}

require_migration "$MIG_DIR/0001_init.up.sql"
require_migration "$MIG_DIR/0002_uuidv7_and_jobs.up.sql"
require_migration "$MIG_DIR/0003_web_session_tickets.up.sql"

docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d \
  --name "$CONTAINER_NAME" \
  -e POSTGRES_DB="$DB_NAME" \
  -e POSTGRES_PASSWORD="$DB_PASSWORD" \
  postgres:18 >/dev/null

until docker exec "$CONTAINER_NAME" pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; do
  sleep 1
done

apply_migration() {
  local path="$1"
  docker exec -i "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 < "$path" >/dev/null
}

apply_migration "$MIG_DIR/0001_init.up.sql"
apply_migration "$MIG_DIR/0002_uuidv7_and_jobs.up.sql"
apply_migration "$MIG_DIR/0003_web_session_tickets.up.sql"

assert_exists() {
  local sql="$1"
  local expected="$2"
  local actual
  actual="$(docker exec -i "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -Atqc "$sql")"
  if [[ "$actual" != "$expected" ]]; then
    echo "assertion failed: expected '$expected', got '$actual'" >&2
    exit 1
  fi
}

assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'labs';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'users';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'user_keys';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'device_auth_requests';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'lab_profiles';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'submissions';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'scores';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'leaderboard';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'used_nonces';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'evaluation_jobs';" "1"
assert_exists "select count(*) from information_schema.tables where table_schema = 'public' and table_name = 'web_session_tickets';" "1"
assert_exists "select count(*) from pg_indexes where schemaname = 'public' and indexname = 'idx_user_code_pending';" "1"
assert_exists "select count(*) from pg_indexes where schemaname = 'public' and indexname = 'idx_submissions_lab_user_created_at';" "1"
assert_exists "select count(*) from pg_indexes where schemaname = 'public' and indexname = 'idx_submissions_lab_status_created_at';" "1"
assert_exists "select count(*) from pg_indexes where schemaname = 'public' and indexname = 'idx_leaderboard_lab_updated_at';" "1"
assert_exists "select count(*) from pg_indexes where schemaname = 'public' and indexname = 'idx_evaluation_jobs_status_available_at';" "1"
assert_exists "select count(*) from pg_indexes where schemaname = 'public' and indexname = 'idx_web_session_tickets_expires_at';" "1"
assert_exists "select count(*) from information_schema.columns where table_schema = 'public' and table_name = 'submissions' and column_name = 'id' and column_default like '%uuidv7()%';" "1"
assert_exists "select count(*) from information_schema.columns where table_schema = 'public' and table_name = 'evaluation_jobs' and column_name = 'id' and column_default like '%uuidv7()%';" "1"
assert_exists "select count(*) from pg_constraint where conrelid = 'submissions'::regclass and contype = 'c' and pg_get_constraintdef(oid) like '%status%' and pg_get_constraintdef(oid) like '%queued%' and pg_get_constraintdef(oid) like '%running%' and pg_get_constraintdef(oid) like '%done%' and pg_get_constraintdef(oid) like '%timeout%' and pg_get_constraintdef(oid) like '%error%';" "1"
assert_exists "select count(*) from pg_constraint where conrelid = 'submissions'::regclass and contype = 'c' and pg_get_constraintdef(oid) like '%verdict%' and pg_get_constraintdef(oid) like '%build_failed%' and pg_get_constraintdef(oid) like '%rejected%' and pg_get_constraintdef(oid) like '%scored%' and pg_get_constraintdef(oid) like '%error%';" "1"
assert_exists "select count(*) from pg_constraint where conrelid = 'device_auth_requests'::regclass and contype = 'c' and pg_get_constraintdef(oid) like '%status%' and pg_get_constraintdef(oid) like '%pending%' and pg_get_constraintdef(oid) like '%approved%' and pg_get_constraintdef(oid) like '%expired%';" "1"
assert_exists "select count(*) from pg_constraint where conrelid = 'evaluation_jobs'::regclass and contype = 'c' and pg_get_constraintdef(oid) like '%status%' and pg_get_constraintdef(oid) like '%queued%' and pg_get_constraintdef(oid) like '%running%' and pg_get_constraintdef(oid) like '%done%' and pg_get_constraintdef(oid) like '%error%';" "1"
assert_exists "select count(*) from pg_constraint where conrelid = 'web_session_tickets'::regclass and contype = 'f';" "2"

echo "schema smoke test passed"
