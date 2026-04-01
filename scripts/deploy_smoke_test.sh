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

require_output_grep() {
  local pattern="$1"
  local output="$2"
  if ! grep -Eq "$pattern" <<<"$output"; then
    echo "missing expected content in compose runtime config: $pattern" >&2
    exit 1
  fi
}

get_env_value() {
  local key="$1"
  local path="$2"
  local line
  line="$(grep -E "^${key}=" "$path" | head -n 1)"
  printf '%s' "${line#*=}"
}

require_auth_template() {
  local path="$1"
  local provider

  require_grep '^LABKIT_AUTH_PROVIDER=' "$path"
  require_grep '^LABKIT_OAUTH_CLIENT_ID=' "$path"
  require_grep '^LABKIT_OAUTH_CLIENT_SECRET=' "$path"
  require_grep '^LABKIT_OAUTH_REDIRECT_URL=' "$path"
  require_grep '^LABKIT_OAUTH_AUTHORIZE_URL=' "$path"
  require_grep '^LABKIT_OAUTH_TOKEN_URL=' "$path"
  require_grep '^LABKIT_OAUTH_DEVICE_AUTH_TTL=' "$path"

  provider="$(get_env_value LABKIT_AUTH_PROVIDER "$path")"
  case "$provider" in
    cas_ruc)
      require_grep '^LABKIT_OAUTH_PROFILE_URL=' "$path"
      ;;
    school_devcenter)
      require_grep '^LABKIT_OAUTH_USER_URL=' "$path"
      require_grep '^LABKIT_OAUTH_PROFILE_URL=' "$path"
      require_grep '^LABKIT_OAUTH_SCOPE=' "$path"
      ;;
    *)
      echo "unsupported auth provider in $path: $provider" >&2
      exit 1
      ;;
    esac
}

make_school_devcenter_env_file() {
  local path
  path="$(mktemp)"
  sed \
    -e 's/^LABKIT_AUTH_PROVIDER=.*/LABKIT_AUTH_PROVIDER=school_devcenter/' \
    -e 's|^LABKIT_OAUTH_REDIRECT_URL=.*|LABKIT_OAUTH_REDIRECT_URL=https://lab.ics.astralis.icu/api/device/verify|' \
    -e 's|^LABKIT_OAUTH_AUTHORIZE_URL=.*|LABKIT_OAUTH_AUTHORIZE_URL=https://school.example.edu/oauth2/authorize|' \
    -e 's|^LABKIT_OAUTH_TOKEN_URL=.*|LABKIT_OAUTH_TOKEN_URL=https://school.example.edu/oauth2/token|' \
    -e 's|^LABKIT_OAUTH_PROFILE_URL=.*|LABKIT_OAUTH_PROFILE_URL=https://school.example.edu/apis/oauth2/v1/profile|' \
    "$DEPLOY_DIR/.env.prod.example" > "$path"
  printf '%s\n' 'LABKIT_OAUTH_USER_URL=https://school.example.edu/apis/oauth2/v1/user' >> "$path"
  printf '%s\n' 'LABKIT_OAUTH_SCOPE=profile' >> "$path"
  printf '%s\n' "$path"
}

require_file "$DEPLOY_DIR/docker-compose.yml"
require_file "$DEPLOY_DIR/caddy/Caddyfile"
require_file "$DEPLOY_DIR/.env.example"
require_file "$DEPLOY_DIR/.env.prod.example"
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
require_grep '^LABKIT_HTTPS_PORT=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_SITE_ADDRESS=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_ADMIN_TOKEN=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_ARTIFACT_ROOT=' "$DEPLOY_DIR/.env.example"
require_auth_template "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_WORKER_ID=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_WORKER_MEMORY_LIMIT=' "$DEPLOY_DIR/.env.example"
require_grep '^LABKIT_SITE_ADDRESS=' "$DEPLOY_DIR/.env.prod.example"
require_grep '^LABKIT_HTTPS_PORT=' "$DEPLOY_DIR/.env.prod.example"
require_auth_template "$DEPLOY_DIR/.env.prod.example"
require_grep '^LABKIT_OAUTH_REDIRECT_URL=https://lab\.ics\.astralis\.icu/api/device/verify$' "$DEPLOY_DIR/.env.prod.example"
require_grep '^LABKIT_OAUTH_AUTHORIZE_URL=https://cas\.ruc\.edu\.cn/cas/oauth2\.0/authorize$' "$DEPLOY_DIR/.env.prod.example"
require_grep '^LABKIT_OAUTH_TOKEN_URL=https://cas\.ruc\.edu\.cn/cas/oauth2\.0/accessToken$' "$DEPLOY_DIR/.env.prod.example"
require_grep '^LABKIT_OAUTH_PROFILE_URL=https://cas\.ruc\.edu\.cn/cas/oauth2\.0/user/profiles$' "$DEPLOY_DIR/.env.prod.example"
require_grep 'reverse_proxy api:8080' "$DEPLOY_DIR/caddy/Caddyfile"
require_grep 'reverse_proxy web:80' "$DEPLOY_DIR/caddy/Caddyfile"
require_grep 'handle /auth/session\*' "$DEPLOY_DIR/caddy/Caddyfile"
require_grep '\{\$LABKIT_SITE_ADDRESS::80\}' "$DEPLOY_DIR/caddy/Caddyfile"
require_grep 'LABKIT_OAUTH_DEVICE_AUTH_TTL' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'LABKIT_HTTPS_PORT' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'LABKIT_SITE_ADDRESS' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'LABKIT_AUTH_PROVIDER' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'LABKIT_OAUTH_USER_URL' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'LABKIT_OAUTH_SCOPE' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'caddy-data' "$DEPLOY_DIR/docker-compose.yml"
require_grep 'caddy-config' "$DEPLOY_DIR/docker-compose.yml"
require_fixed '/migrations/0005_user_key_fingerprints.up.sql' "$DEPLOY_DIR/migrate.sh"
require_grep '<title>LabKit</title>' "$ROOT_DIR/apps/web/index.html"
require_absent 'python:3.12-alpine' "$DEPLOY_DIR/docker-compose.yml"
require_absent 'tail -f /dev/null' "$DEPLOY_DIR/docker-compose.yml"
require_absent 'python3 -m http.server' "$DEPLOY_DIR/docker-compose.yml"

compose_default_config="$(docker compose -f "$DEPLOY_DIR/docker-compose.yml" --env-file "$DEPLOY_DIR/.env.example" config)"
require_output_grep 'LABKIT_AUTH_PROVIDER: "?cas_ruc"?' "$compose_default_config"
require_output_grep 'LABKIT_OAUTH_USER_URL: ""' "$compose_default_config"
require_output_grep 'LABKIT_OAUTH_SCOPE: ""' "$compose_default_config"
require_output_grep 'LABKIT_OAUTH_PROFILE_URL: "?https://example\.invalid/oauth/profile"?' "$compose_default_config"

school_env_file="$(make_school_devcenter_env_file)"
trap 'rm -f "$school_env_file"' EXIT
compose_school_config="$(docker compose -f "$DEPLOY_DIR/docker-compose.yml" --env-file "$school_env_file" config)"
require_output_grep 'LABKIT_AUTH_PROVIDER: "?school_devcenter"?' "$compose_school_config"
require_output_grep 'LABKIT_OAUTH_USER_URL: "?https://school\.example\.edu/apis/oauth2/v1/user"?' "$compose_school_config"
require_output_grep 'LABKIT_OAUTH_SCOPE: "?profile"?' "$compose_school_config"
require_output_grep 'LABKIT_OAUTH_AUTHORIZE_URL: "?https://school\.example\.edu/oauth2/authorize"?' "$compose_school_config"
require_output_grep 'LABKIT_OAUTH_TOKEN_URL: "?https://school\.example\.edu/oauth2/token"?' "$compose_school_config"
require_output_grep 'LABKIT_OAUTH_PROFILE_URL: "?https://school\.example\.edu/apis/oauth2/v1/profile"?' "$compose_school_config"

docker compose -f "$DEPLOY_DIR/docker-compose.yml" --env-file "$DEPLOY_DIR/.env.prod.example" config >/dev/null

echo "deploy smoke test passed"
