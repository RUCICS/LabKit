#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_DIR="$ROOT_DIR/deploy"

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "missing file: $path" >&2
    exit 1
  fi
}

require_grep() {
  local pattern="$1"
  local path="$2"
  if ! grep -Eq "$pattern" "$path"; then
    echo "missing expected content in $path: $pattern" >&2
    exit 1
  fi
}

require_fixed() {
  local needle="$1"
  local path="$2"
  if ! grep -Fq "$needle" "$path"; then
    echo "missing expected content in $path: $needle" >&2
    exit 1
  fi
}

require_absent() {
  local pattern="$1"
  local path="$2"
  if grep -Eq "$pattern" "$path"; then
    echo "unexpected content in $path: $pattern" >&2
    exit 1
  fi
}

require_file "$DEPLOY_DIR/docker-compose.yml"
require_file "$DEPLOY_DIR/caddy/Caddyfile"
require_file "$DEPLOY_DIR/.env.example"
require_file "$DEPLOY_DIR/api.Dockerfile"
require_file "$DEPLOY_DIR/worker.Dockerfile"
require_file "$DEPLOY_DIR/web.Dockerfile"
require_file "$DEPLOY_DIR/nginx/default.conf"
require_file "$DEPLOY_DIR/migrate.sh"
require_file "$ROOT_DIR/scripts/dev-up.sh"
require_file "$ROOT_DIR/scripts/dev-down.sh"
require_file "$ROOT_DIR/apps/web/index.html"
test -x "$ROOT_DIR/scripts/dev-up.sh"
test -x "$ROOT_DIR/scripts/dev-down.sh"
test -x "$ROOT_DIR/scripts/deploy_smoke_test.sh"

require_grep '^services:' "$DEPLOY_DIR/docker-compose.yml"
require_grep '^[[:space:]]+postgres:' "$DEPLOY_DIR/docker-compose.yml"
require_grep '^[[:space:]]+migrate:' "$DEPLOY_DIR/docker-compose.yml"
require_grep '^[[:space:]]+api:' "$DEPLOY_DIR/docker-compose.yml"
require_grep '^[[:space:]]+worker:' "$DEPLOY_DIR/docker-compose.yml"
require_grep '^[[:space:]]+web:' "$DEPLOY_DIR/docker-compose.yml"
require_grep '^[[:space:]]+caddy:' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'dockerfile: deploy/api.Dockerfile' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'dockerfile: deploy/worker.Dockerfile' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'dockerfile: deploy/web.Dockerfile' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'service_completed_successfully' "$DEPLOY_DIR/docker-compose.yml"
require_grep '/var/run/docker.sock:/var/run/docker.sock' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'artifacts-data' "$DEPLOY_DIR/docker-compose.yml"
require_fixed 'COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf' "$DEPLOY_DIR/web.Dockerfile"
require_fixed 'try_files $uri $uri/ /index.html;' "$DEPLOY_DIR/nginx/default.conf"
require_grep '^POSTGRES_DB=' "$DEPLOY_DIR/.env.example"
require_grep '^POSTGRES_USER=' "$DEPLOY_DIR/.env.example"
require_grep '^POSTGRES_PASSWORD=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_HTTP_PORT=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_ADMIN_TOKEN=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_ARTIFACT_ROOT=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_OAUTH_CLIENT_ID=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_OAUTH_CLIENT_SECRET=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_OAUTH_REDIRECT_URL=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_OAUTH_AUTHORIZE_URL=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_OAUTH_TOKEN_URL=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_OAUTH_PROFILE_URL=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_WORKER_ID=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_WORKER_MEMORY_LIMIT=' "$DEPLOY_DIR/.env.example"
require_grep 'reverse_proxy api:8080' "$DEPLOY_DIR/caddy/Caddyfile"
require_grep 'reverse_proxy web:80' "$DEPLOY_DIR/caddy/Caddyfile"
require_grep 'handle /auth/session\*' "$DEPLOY_DIR/caddy/Caddyfile"
require_grep '<title>LabKit</title>' "$ROOT_DIR/apps/web/index.html"
require_absent 'python:3.12-alpine' "$DEPLOY_DIR/docker-compose.yml"
require_absent 'tail -f /dev/null' "$DEPLOY_DIR/docker-compose.yml"
require_absent 'python3 -m http.server' "$DEPLOY_DIR/docker-compose.yml"

docker compose -f "$DEPLOY_DIR/docker-compose.yml" --env-file "$DEPLOY_DIR/.env.example" config >/dev/null

echo "deploy smoke test passed"
