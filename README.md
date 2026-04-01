# LabKit

LabKit is a single-course ICS lab leaderboard platform with a Go API, Go CLI, Go worker, Vue web app, PostgreSQL, and a real Docker Compose local stack.

## Repository Overview

- `apps/api`: HTTP API for device authorization, labs, submissions, leaderboard, history, keys, and admin endpoints.
- `apps/worker`: background worker for evaluator jobs. The default worker path now runs the real Docker evaluator runtime; the old dev-fake path remains only as an explicit opt-in debug mode.
- `apps/cli`: Cobra-based CLI for `auth`, `submit`, `board`, `history`, `nick`, `track`, `keys`, and `revoke`.
- `apps/web`: Vue 3 + Vite UI for lab list, leaderboard, auth confirmation, key inventory, and admin lab/queue views.
- `packages/go/*`: shared Go packages for manifests, auth/signing, DB access, job queue primitives, evaluator parsing, and domain types.
- `db/`: PostgreSQL migrations and sqlc query definitions.
- `deploy/`: Docker Compose deployment stack, image build files, migration helper, and Caddy reverse-proxy config.
- `scripts/`: local dev helpers, smoke tests, and the happy-path e2e script.

## Stack

- Go `1.26` with toolchain `go1.26.1`
- PostgreSQL `18`
- Vue `3`, Vite `6`, TypeScript, Pinia, Vitest
- Docker Compose + Caddy for local orchestration
- Bash, `curl`, and `python3` for smoke/e2e scripts

## Local Quickstart

Prerequisites:

- Go `1.26.1`
- Node `22+` and `npm`
- Docker with Compose support
- `bash`, `curl`, and `python3`

Start the local stack:

```bash
cp deploy/.env.example deploy/.env
cd apps/web && npm ci && cd ../..
bash scripts/dev-up.sh
```

Default local entry points:

- Web: `http://localhost:8080/`
- Admin: `http://localhost:8080/admin?token=dev-admin-token`
- Health: `http://localhost:8080/healthz`

The default examples above follow [deploy/.env.example](/home/starrydream/ICS2/LabKit/deploy/.env.example). If your local [deploy/.env](/home/starrydream/ICS2/LabKit/deploy/.env) uses a different `LABKIT_HTTP_PORT` such as `8083`, use that port consistently for web, admin, CLI, and auth commands.

Stop the stack:

```bash
bash scripts/dev-down.sh
```

## Production Deploy

For a real domain and school OAuth callback, start from [deploy/.env.prod.example](/home/starrydream/ICS2/LabKit/deploy/.env.prod.example):

```bash
cp deploy/.env.prod.example deploy/.env
```

Then set at least:

- `LABKIT_SITE_ADDRESS` to your public host, for example `lab.ics.astralis.icu`
- `LABKIT_HTTP_PORT=80`
- `LABKIT_HTTPS_PORT=443`
- `LABKIT_OAUTH_CLIENT_ID` and `LABKIT_OAUTH_CLIENT_SECRET` to the values issued by the school
- `LABKIT_OAUTH_REDIRECT_URL=https://<your-host>/api/device/verify`

The current production OAuth wiring expects:

- authorize URL: `https://cas.ruc.edu.cn/cas/oauth2.0/authorize`
- token URL: `https://cas.ruc.edu.cn/cas/oauth2.0/accessToken`
- profile URL: `https://cas.ruc.edu.cn/cas/oauth2.0/user/profiles`

The school-side OAuth application must register the same HTTPS callback URL. Caddy now supports both HTTP and HTTPS in Compose and will terminate TLS directly when `LABKIT_SITE_ADDRESS` is set to your real domain.

## Local Admin Flow

1. Open `http://localhost:8080/admin?token=dev-admin-token`
2. Paste a lab manifest into the manifest editor
3. Use `Register lab` to create a new lab
4. Open `Queue` for that lab to inspect jobs, export grades, or trigger re-evaluation

The admin SPA now covers:

- lab registration
- lab manifest updates
- queue inspection
- grade export
- re-evaluation

## CLI Quickstart

Build or run the CLI directly:

```bash
go run ./apps/cli/cmd/labkit --help
```

Build portable binaries:

```bash
bash scripts/build-cli.sh
```

Or build a narrower target set:

```bash
bash scripts/build-cli.sh linux/amd64 darwin/arm64
```

Common commands:

```bash
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 auth --no-encrypt
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 keys
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 revoke 1

go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 --lab sorting board
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 --lab sorting submit main.c README.md
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 --lab sorting history
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 --lab sorting nick alice
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 --lab sorting track runtime_ms
```

CLI config now uses TOML:

- global config: `~/.config/labkit/config.toml`
- project config: nearest `.labkit/config.toml`, searched upward from the current working directory

Global auth state is stored as a per-server keyring:

```toml
# ~/.config/labkit/config.toml
default_server_url = "http://localhost:8080"

[servers."http://localhost:8080"]
key_path = "/home/user/.config/labkit/id_ed25519"
key_fingerprint = "SHA256:..."
encrypted = false
```

Recommended local project config:

```toml
# .labkit/config.toml
server_url = "http://localhost:8080"
lab = "sorting"
```

With that file in the repo, you can run the lab-scoped commands without repeating `--server-url` and `--lab`:

```bash
go run ./apps/cli/cmd/labkit auth
go run ./apps/cli/cmd/labkit board
go run ./apps/cli/cmd/labkit submit main.c README.md
go run ./apps/cli/cmd/labkit history
go run ./apps/cli/cmd/labkit nick alice
go run ./apps/cli/cmd/labkit track runtime_ms
```

Override priority is:

- CLI flags
- environment variables
- local `.labkit/config.toml`
- global `~/.config/labkit/config.toml`
- built-in defaults

`labkit auth` now:

- reuses the existing local key for the same server by default
- rotates that server key when you pass `--rotate-key`
- asks whether to encrypt the private key in an interactive terminal
- requires `--encrypt` or `--no-encrypt` in non-interactive usage

Signed CLI requests now use `X-LabKit-Key-Fingerprint` instead of `X-LabKit-Key-ID`.

Important local auth note:

- `labkit auth` expects the OAuth device flow.
- For pure local development without a real OAuth provider, the repo still relies on the dev-only `/api/dev/device/bind` shortcut used by tests and e2e.
- To expose that shortcut, set `LABKIT_DEV_MODE=true` in `deploy/.env` before starting the stack.

Current local auth workaround:

```bash
# terminal A
go run ./apps/cli/cmd/labkit --server-url http://localhost:8080 auth --no-encrypt

# terminal B: fetch the latest device_code created by terminal A
docker compose -f deploy/docker-compose.yml --env-file deploy/.env exec -T postgres \
  psql -U labkit -d labkit -Atc \
  "select device_code from device_auth_requests order by created_at desc limit 1;"

# terminal B: approve that device through the dev bind endpoint
curl -X POST http://localhost:8080/api/dev/device/bind \
  -H 'Content-Type: application/json' \
  -d '{"device_code":"<device_code>","student_id":"2026001","device_name":"local-dev"}'
```

After the bind succeeds, the `labkit auth` process in terminal A will finish and persist the per-server keyring entry.

Further references:

- [本地认证与 Admin 鉴权说明](/home/starrydream/ICS2/LabKit/docs/reference/local-auth.md)
- [Lab 编写与 Evaluator 协议说明](/home/starrydream/ICS2/LabKit/docs/reference/lab-authoring.md)
- [示例 manifest](/home/starrydream/ICS2/LabKit/examples/labs/local-smoke.lab.toml)
- [示例 evaluator](/home/starrydream/ICS2/LabKit/examples/evaluator/local-smoke/evaluator.py)

## Verification

- `go test ./...`
- `cd apps/web && npm test`
- `cd apps/web && npm run build`
- `bash db/migrations/schema_smoke_test.sh`
- `bash scripts/deploy_smoke_test.sh`
- `bash scripts/e2e-happy-path.sh`

## Architecture Notes

- The API is the integration point for browser and CLI clients.
- `deploy/docker-compose.yml` now builds and runs the real API, worker, web, migration, PostgreSQL, and Caddy services.
- The worker default path uses the real Docker evaluator runtime; `LABKIT_WORKER_DEV_FAKE_EVALUATION=true` remains available only for explicit debugging.
- The frontend is intentionally thin and currently focuses on public board visibility, auth confirmation, and admin surfaces rather than full student self-service.

## Verification Snapshot

Current baseline:

- [x] `go test ./...`
- [x] `cd apps/web && npm test`
- [x] `cd apps/web && npm run build`
- [x] `bash db/migrations/schema_smoke_test.sh`
- [x] `bash scripts/deploy_smoke_test.sh`
- [x] `bash scripts/e2e-happy-path.sh`
