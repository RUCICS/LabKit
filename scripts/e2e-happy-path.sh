#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_DIR="$ROOT_DIR/deploy"
TESTDATA_DIR="$ROOT_DIR/scripts/testdata"
ENV_FILE="$DEPLOY_DIR/.env"
if [[ ! -f "$ENV_FILE" ]]; then
  ENV_FILE="$DEPLOY_DIR/.env.example"
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

API_PORT="${LABKIT_E2E_API_PORT:-18080}"
BASE_URL="${LABKIT_E2E_BASE_URL:-http://127.0.0.1:${API_PORT}}"
LAB_ID="${LABKIT_E2E_LAB_ID:-e2e-happy-path}"
STUDENT_ID="${LABKIT_E2E_STUDENT_ID:-2026e2e}"
ADMIN_TOKEN="${LABKIT_ADMIN_TOKEN:-dev-admin-token}"
POSTGRES_USER="${POSTGRES_USER:-labkit}"
POSTGRES_DB="${POSTGRES_DB:-labkit}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-labkit}"
KEEP_STACK="${LABKIT_E2E_KEEP_STACK:-false}"
TMP_DIR="$(mktemp -d)"
ARTIFACT_ROOT="${LABKIT_E2E_ARTIFACT_ROOT:-$TMP_DIR/labkit-e2e-artifacts}"
CONTAINER_ARTIFACT_ROOT="/var/lib/labkit/artifacts"
COMPOSE_PROJECT_NAME="${LABKIT_E2E_COMPOSE_PROJECT:-labkit-e2e-${USER:-user}-$$}"
NETWORK_NAME="${COMPOSE_PROJECT_NAME}_default"
API_CONTAINER="${LABKIT_E2E_API_CONTAINER:-${COMPOSE_PROJECT_NAME}-api}"
API_IMAGE="${LABKIT_E2E_API_IMAGE:-labkit-e2e-api:${COMPOSE_PROJECT_NAME}}"
WORKER_IMAGE="${LABKIT_E2E_WORKER_IMAGE:-labkit-e2e-worker:${COMPOSE_PROJECT_NAME}}"
EVALUATOR_IMAGE="${LABKIT_E2E_EVALUATOR_IMAGE:-labkit-e2e-evaluator:${COMPOSE_PROJECT_NAME}}"

export LABKIT_DEV_MODE="${LABKIT_DEV_MODE:-true}"
export COMPOSE_PROJECT_NAME
mkdir -p "$ARTIFACT_ROOT"

cleanup() {
  rm -rf "$TMP_DIR"
  if [[ "$KEEP_STACK" != "true" ]]; then
    docker rm -f "$API_CONTAINER" >/dev/null 2>&1 || true
    docker compose -p "$COMPOSE_PROJECT_NAME" -f "$DEPLOY_DIR/docker-compose.yml" --env-file "$ENV_FILE" down --remove-orphans -v >/dev/null 2>&1 || true
    docker rmi -f "$API_IMAGE" "$WORKER_IMAGE" "$EVALUATOR_IMAGE" >/dev/null 2>&1 || true
    rm -rf "$ARTIFACT_ROOT" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

require_cmd() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "missing required command: $name" >&2
    exit 1
  fi
}

compose() {
  docker compose -p "$COMPOSE_PROJECT_NAME" -f "$DEPLOY_DIR/docker-compose.yml" --env-file "$ENV_FILE" "$@"
}

json_get() {
  local path="$1"
  python3 -c '
import json
import sys

segments = [part for part in sys.argv[1].split(".") if part]
value = json.load(sys.stdin)
for segment in segments:
    if isinstance(value, list):
        value = value[int(segment)]
    else:
        value = value[segment]
if isinstance(value, bool):
    print("true" if value else "false")
elif value is None:
    print("")
else:
    print(value)
' "$path"
}

wait_for_http() {
  local url="$1"
  for _ in $(seq 1 90); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url" >&2
  exit 1
}

wait_for_postgres() {
  for _ in $(seq 1 60); do
    if compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for postgres" >&2
  exit 1
}

apply_migrations() {
  compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < "$ROOT_DIR/db/migrations/0001_init.up.sql"
  compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < "$ROOT_DIR/db/migrations/0002_uuidv7_and_jobs.up.sql"
  compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < "$ROOT_DIR/db/migrations/0003_web_session_tickets.up.sql"
  compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 < "$ROOT_DIR/db/migrations/0004_user_keys_soft_delete.up.sql"
}

build_runtime_images() {
  docker build -f "$DEPLOY_DIR/api.Dockerfile" -t "$API_IMAGE" "$ROOT_DIR" >/dev/null
  docker build -f "$DEPLOY_DIR/worker.Dockerfile" -t "$WORKER_IMAGE" "$ROOT_DIR" >/dev/null
  docker build -f "$TESTDATA_DIR/evaluator/Dockerfile" -t "$EVALUATOR_IMAGE" "$TESTDATA_DIR/evaluator" >/dev/null
}

render_manifest() {
  local output="$1"
  sed "s|__E2E_EVAL_IMAGE__|$EVALUATOR_IMAGE|g" "$TESTDATA_DIR/e2e-happy-path.lab.toml" >"$output"
}

generate_keypair() {
  local key_path="$1"
  local pub_path="$2"
  local helper="$TMP_DIR/e2e-keygen.go"
  cat >"$helper" <<'EOF'
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: e2e-keygen <private-key-path> <public-key-path>")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[1], pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		panic(err)
	}
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[2], ssh.MarshalAuthorizedKey(sshKey), 0o644); err != nil {
		panic(err)
	}
	fmt.Print(string(ssh.MarshalAuthorizedKey(sshKey)))
}
EOF
  go run "$helper" "$key_path" "$pub_path" >/dev/null
}

sign_submission() {
  local key_path="$1"
  local lab_id="$2"
  local timestamp="$3"
  local nonce="$4"
  shift 4
  local helper_dir
  helper_dir="$(mktemp -d "$ROOT_DIR/apps/api/.e2e-sign-helper.XXXXXX")"
  cat >"$helper_dir/main.go" <<'EOF'
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"labkit.local/apps/api/internal/storage"
	"labkit.local/packages/go/auth"
)

func main() {
	if len(os.Args) < 6 {
		panic("usage: signer <private-key-path> <lab-id> <timestamp> <nonce> <file> [<file>...]")
	}
	privateKey := readPrivateKey(os.Args[1])
	timestamp, err := time.Parse(time.RFC3339Nano, os.Args[3])
	if err != nil {
		panic(err)
	}
	files := make([]storage.ArtifactFile, 0, len(os.Args)-5)
	fileNames := make([]string, 0, len(os.Args)-5)
	for _, path := range os.Args[5:] {
		content, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		name := filepath.Base(path)
		files = append(files, storage.ArtifactFile{Name: name, Content: content})
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	_, contentHash, err := storage.Archive(files)
	if err != nil {
		panic(err)
	}
	payload := auth.NewPayload(os.Args[2], timestamp, os.Args[4], fileNames).WithContentHash(contentHash)
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		panic(err)
	}
	signature := ed25519.Sign(privateKey, signingBytes)
	fmt.Printf("%s\n%s\n", contentHash, base64.StdEncoding.EncodeToString(signature))
}

func readPrivateKey(path string) ed25519.PrivateKey {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		panic("invalid private key")
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	privateKey, ok := value.(ed25519.PrivateKey)
	if !ok {
		panic("private key is not ed25519")
	}
	return privateKey
}
EOF

  local out
  if ! out="$(go run "$helper_dir" "$key_path" "$lab_id" "$timestamp" "$nonce" "$@")"; then
    rm -rf "$helper_dir"
    return 1
  fi
  rm -rf "$helper_dir"
  printf '%s' "$out"
}

require_cmd curl
require_cmd docker
require_cmd go
require_cmd python3
if ! docker info >/dev/null 2>&1; then
  echo "docker daemon is not available" >&2
  exit 1
fi

echo "==> booting services"
compose up -d postgres
wait_for_postgres

echo "==> applying migrations"
apply_migrations

echo "==> building runtime images"
build_runtime_images

echo "==> starting api"
docker run -d --rm \
  --name "$API_CONTAINER" \
  --network "$NETWORK_NAME" \
  --user "$(id -u):$(id -g)" \
  -p "${API_PORT}:8080" \
  -v "$ARTIFACT_ROOT:$CONTAINER_ARTIFACT_ROOT" \
  -e POSTGRES_HOST="postgres" \
  -e POSTGRES_PORT="5432" \
  -e POSTGRES_DB="$POSTGRES_DB" \
  -e POSTGRES_USER="$POSTGRES_USER" \
  -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  -e POSTGRES_SSLMODE="disable" \
  -e LABKIT_ADMIN_TOKEN="$ADMIN_TOKEN" \
  -e LABKIT_ARTIFACT_ROOT="$CONTAINER_ARTIFACT_ROOT" \
  -e LABKIT_DEV_MODE=true \
  -e LABKIT_OAUTH_CLIENT_ID="dev-client-id" \
  -e LABKIT_OAUTH_CLIENT_SECRET="dev-client-secret" \
  -e LABKIT_OAUTH_REDIRECT_URL="${BASE_URL}/api/device/verify" \
  -e LABKIT_OAUTH_AUTHORIZE_URL="https://dev.invalid/oauth/authorize" \
  -e LABKIT_OAUTH_TOKEN_URL="https://dev.invalid/oauth/token" \
  -e LABKIT_OAUTH_PROFILE_URL="https://dev.invalid/oauth/profile" \
  "$API_IMAGE" >/dev/null

echo "==> waiting for API"
wait_for_http "$BASE_URL/healthz"

echo "==> registering test lab"
MANIFEST_FILE="$TMP_DIR/e2e-happy-path.lab.toml"
render_manifest "$MANIFEST_FILE"
curl -fsS \
  -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/toml" \
  --data-binary @"$MANIFEST_FILE" \
  "$BASE_URL/api/admin/labs" >/dev/null

echo "==> generating submission keypair"
PRIVATE_KEY="$TMP_DIR/id_ed25519.pem"
PUBLIC_KEY_FILE="$TMP_DIR/id_ed25519.pub"
generate_keypair "$PRIVATE_KEY" "$PUBLIC_KEY_FILE"
PUBLIC_KEY="$(tr -d '\n' < "$PUBLIC_KEY_FILE")"

echo "==> creating device authorization request"
AUTHORIZE_RESPONSE="$TMP_DIR/authorize.json"
curl -fsS \
  -X POST \
  -H "Content-Type: application/json" \
  -d "{\"public_key\":$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$PUBLIC_KEY")}" \
  "$BASE_URL/api/device/authorize" >"$AUTHORIZE_RESPONSE"
DEVICE_CODE="$(json_get device_code < "$AUTHORIZE_RESPONSE")"

echo "==> completing dev-only device binding shortcut"
BIND_RESPONSE="$TMP_DIR/bind.json"
curl -fsS \
  -X POST \
  -H "Content-Type: application/json" \
  -d "{\"device_code\":$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$DEVICE_CODE"),\"student_id\":$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$STUDENT_ID"),\"device_name\":\"e2e-script\"}" \
  "$BASE_URL/api/dev/device/bind" >"$BIND_RESPONSE"

echo "==> polling approved device authorization"
POLL_RESPONSE="$TMP_DIR/poll.json"
curl -fsS -X POST "$BASE_URL/api/device/poll?device_code=$DEVICE_CODE" >"$POLL_RESPONSE"
KEY_ID="$(json_get key_id < "$POLL_RESPONSE")"

echo "==> signing and submitting artifact"
TIMESTAMP="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
NONCE="$(python3 -c 'import uuid; print(uuid.uuid4().hex)')"
SIGN_OUTPUT="$(sign_submission "$PRIVATE_KEY" "$LAB_ID" "$TIMESTAMP" "$NONCE" "$TESTDATA_DIR/e2e-submission.json")"
CONTENT_HASH="$(printf '%s\n' "$SIGN_OUTPUT" | sed -n '1p')"
SIGNATURE="$(printf '%s\n' "$SIGN_OUTPUT" | sed -n '2p')"
SUBMIT_RESPONSE="$TMP_DIR/submit.json"
curl -fsS \
  -X POST \
  -H "X-LabKit-Key-ID: $KEY_ID" \
  -H "X-LabKit-Timestamp: $TIMESTAMP" \
  -H "X-LabKit-Nonce: $NONCE" \
  -H "X-LabKit-Signature: $SIGNATURE" \
  -F "files=@$TESTDATA_DIR/e2e-submission.json;type=application/json" \
  "$BASE_URL/api/labs/$LAB_ID/submit" >"$SUBMIT_RESPONSE"

ACTUAL_HASH="$(json_get content_hash < "$SUBMIT_RESPONSE")"
if [[ "$ACTUAL_HASH" != "$CONTENT_HASH" ]]; then
  echo "content hash mismatch: expected $CONTENT_HASH, got $ACTUAL_HASH" >&2
  exit 1
fi

echo "==> running worker"
docker run --rm \
  --network "$NETWORK_NAME" \
  --user "$(id -u):$(id -g)" \
  --group-add "$(stat -c '%g' /var/run/docker.sock)" \
  -v "$ARTIFACT_ROOT:$CONTAINER_ARTIFACT_ROOT" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e POSTGRES_HOST="postgres" \
  -e POSTGRES_PORT="5432" \
  -e POSTGRES_DB="$POSTGRES_DB" \
  -e POSTGRES_USER="$POSTGRES_USER" \
  -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  -e POSTGRES_SSLMODE="disable" \
  -e LABKIT_ARTIFACT_ROOT="$CONTAINER_ARTIFACT_ROOT" \
  -e LABKIT_WORKER_RUN_ONCE=true \
  -e LABKIT_WORKER_ID="e2e-worker" \
  "$WORKER_IMAGE"

echo "==> verifying leaderboard"
BOARD_RESPONSE="$TMP_DIR/board.json"
curl -fsS "$BASE_URL/api/labs/$LAB_ID/board" >"$BOARD_RESPONSE"
RANK="$(json_get rows.0.rank < "$BOARD_RESPONSE")"
NICKNAME="$(json_get rows.0.nickname < "$BOARD_RESPONSE")"
METRIC_ID="$(json_get rows.0.scores.0.metric_id < "$BOARD_RESPONSE")"
METRIC_VALUE="$(json_get rows.0.scores.0.value < "$BOARD_RESPONSE")"

if [[ "$RANK" != "1" ]]; then
  echo "unexpected rank: $RANK" >&2
  exit 1
fi
if [[ "$NICKNAME" != "匿名" ]]; then
  echo "unexpected nickname: $NICKNAME" >&2
  exit 1
fi
if [[ "$METRIC_ID" != "score" ]]; then
  echo "unexpected metric id: $METRIC_ID" >&2
  exit 1
fi
if [[ "$METRIC_VALUE" != "97.5" ]]; then
  echo "unexpected metric value: $METRIC_VALUE" >&2
  exit 1
fi

echo "happy path verification passed"
