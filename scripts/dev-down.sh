#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_DIR="$ROOT_DIR/deploy"
ENV_FILE="$DEPLOY_DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  ENV_FILE="$DEPLOY_DIR/.env.example"
fi

docker compose \
  -f "$DEPLOY_DIR/docker-compose.yml" \
  --env-file "$ENV_FILE" \
  down --remove-orphans
