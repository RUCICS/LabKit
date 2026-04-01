# LabKit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the first production-quality version of LabKit as a single-instance ICS leaderboard platform with Go API, Go worker, Go CLI, Vue frontend, PostgreSQL 18, OAuth device binding, Ed25519 request signing, Docker-based evaluator execution, and manifest-driven lab behavior.

**Architecture:** Use a monorepo with deployable apps for API, worker, CLI, and web, plus shared Go domain packages and PostgreSQL-backed job queue semantics. Keep evaluation execution in a dedicated worker process and refresh leaderboard state transactionally with scoring results.

**Tech Stack:** Go 1.26 + toolchain go1.26.1, chi, pgx, sqlc, golang-migrate, PostgreSQL 18, Cobra, Vue 3, Vite, Pinia, TypeScript, Docker Compose

---

## Completion Snapshot

As of 2026-04-01, Tasks 1-25 are materially present in the repository. Task 26 is the documentation and final verification pass, plus any narrow integration fixes needed to make the required verification commands reflect reality.

Key as-built notes:

- The repository remains a multi-module Go workspace. Task 26 adds a root-level verification harness so the required `go test ./...` command is meaningful from repo root.
- The checked-in compose stack now builds and runs the real API, worker, web, migration, PostgreSQL, and Caddy services.
- The source-run dev flow remains available for direct local debugging.
- Dev-only support is still available when explicitly enabled:
  - `LABKIT_DEV_MODE=true` enables `/api/dev/device/bind`
  - `LABKIT_WORKER_DEV_FAKE_EVALUATION=true` and `LABKIT_WORKER_RUN_ONCE=true` enable one-shot fake evaluation
- The worker verification path now uses the real Docker evaluation runtime by default.

## Failing Verification Checklist

Task 26 verification pass on 2026-04-01:

- [x] `go test ./...`
- [x] `cd apps/web && npm test`
- [x] `bash db/migrations/schema_smoke_test.sh`
- [x] `bash scripts/deploy_smoke_test.sh`
- [x] `bash scripts/e2e-happy-path.sh`

## Current Verification Status

Status after the Task 26 pass:

- [x] `go test ./...`
- [x] `cd apps/web && npm test`
- [x] `bash db/migrations/schema_smoke_test.sh`
- [x] `bash scripts/deploy_smoke_test.sh`
- [x] `bash scripts/e2e-happy-path.sh`

### Task 1: Bootstrap Repository Structure

**Files:**
- Create: `apps/api/`
- Create: `apps/worker/`
- Create: `apps/cli/`
- Create: `apps/web/`
- Create: `packages/go/manifest/`
- Create: `packages/go/auth/`
- Create: `packages/go/db/`
- Create: `packages/go/jobs/`
- Create: `packages/go/evaluator/`
- Create: `packages/go/labkit/`
- Create: `db/migrations/`
- Create: `db/queries/`
- Create: `deploy/docker-compose.yml`
- Create: `.gitignore`
- Create: `README.md`

**Step 1: Write the failing structure check**

Create a lightweight verification script or test that asserts required top-level directories and files exist.

**Step 2: Run structure check to verify it fails**

Run: `test -d apps/api && test -d apps/worker && test -f deploy/docker-compose.yml`
Expected: fail because the repository has not been scaffolded yet.

**Step 3: Create the minimal repository skeleton**

Add the top-level folders, placeholder files, root Go workspace configuration, and root README.

**Step 4: Run structure check to verify it passes**

Run: `test -d apps/api && test -d apps/worker && test -f deploy/docker-compose.yml`
Expected: success.

**Step 5: Commit**

```bash
git add .
git commit -m "chore: scaffold labkit monorepo"
```

### Task 2: Establish Go Workspace and Shared Domain Types

**Files:**
- Create: `go.work`
- Create: `apps/api/go.mod`
- Create: `apps/worker/go.mod`
- Create: `apps/cli/go.mod`
- Create: `packages/go/labkit/go.mod`
- Create: `packages/go/labkit/types.go`
- Create: `packages/go/labkit/errors.go`
- Test: `packages/go/labkit/types_test.go`

**Step 1: Write the failing tests**

Add tests that assert core domain types exist for labs, metrics, submissions, verdicts, and common error classification.

**Step 2: Run test to verify it fails**

Run: `go test ./packages/go/labkit/...`
Expected: fail because the shared package does not exist yet.

**Step 3: Write minimal implementation**

Add the shared module, define domain enums and structs, and add typed error helpers for user/system/evaluator/admin error classes.

**Step 4: Run test to verify it passes**

Run: `go test ./packages/go/labkit/...`
Expected: pass.

**Step 5: Commit**

```bash
git add go.work apps/api/go.mod apps/worker/go.mod apps/cli/go.mod packages/go/labkit
git commit -m "feat: add shared go domain package"
```

### Task 3: Implement Manifest Schema and Validation

**Files:**
- Create: `packages/go/manifest/go.mod`
- Create: `packages/go/manifest/manifest.go`
- Create: `packages/go/manifest/validate.go`
- Create: `packages/go/manifest/public.go`
- Test: `packages/go/manifest/manifest_test.go`

**Step 1: Write the failing tests**

Cover:
- valid single-metric lab
- valid multi-metric lab
- duplicate metric IDs
- invalid `board.rank_by`
- invalid schedule ordering
- unsupported metric sort direction
- missing required sections

**Step 2: Run test to verify it fails**

Run: `go test ./packages/go/manifest/...`
Expected: fail because manifest parsing and validation are not implemented.

**Step 3: Write minimal implementation**

Implement TOML parsing, schema structs, validation rules, and public-manifest projection for unauthenticated clients.

**Step 4: Run test to verify it passes**

Run: `go test ./packages/go/manifest/...`
Expected: pass.

**Step 5: Commit**

```bash
git add packages/go/manifest
git commit -m "feat: add manifest parser and validator"
```

### Task 4: Design Database Schema and Migrations

**Files:**
- Create: `db/migrations/0001_init.up.sql`
- Create: `db/migrations/0001_init.down.sql`
- Create: `db/migrations/0002_uuidv7_and_jobs.up.sql`
- Create: `db/migrations/0002_uuidv7_and_jobs.down.sql`
- Create: `db/queries/labs.sql`
- Create: `db/queries/auth.sql`
- Create: `db/queries/submissions.sql`
- Create: `db/queries/leaderboard.sql`
- Create: `db/queries/jobs.sql`
- Test: `db/migrations/schema_smoke_test.sh`

**Step 1: Write the failing schema smoke test**

Create a script that boots PostgreSQL 18, applies migrations, and verifies the expected tables and indexes exist.

**Step 2: Run test to verify it fails**

Run: `bash db/migrations/schema_smoke_test.sh`
Expected: fail because migrations do not exist yet.

**Step 3: Write minimal implementation**

Add schema migrations for all core tables, UUIDv7 IDs, indexes, constraints, and queue support via `evaluation_jobs`.

**Step 4: Run test to verify it passes**

Run: `bash db/migrations/schema_smoke_test.sh`
Expected: pass.

**Step 5: Commit**

```bash
git add db/migrations db/queries
git commit -m "feat: add initial database schema"
```

### Task 5: Generate Database Access Layer

**Files:**
- Create: `sqlc.yaml`
- Create: `packages/go/db/go.mod`
- Create: `packages/go/db/sqlc/`
- Create: `packages/go/db/store.go`
- Test: `packages/go/db/store_test.go`

**Step 1: Write the failing tests**

Add tests that expect typed query helpers for labs, auth requests, submissions, scores, leaderboard, and jobs.

**Step 2: Run test to verify it fails**

Run: `go test ./packages/go/db/...`
Expected: fail because sqlc output and store helpers do not exist.

**Step 3: Write minimal implementation**

Configure sqlc, generate PGX-backed query code, and wrap transaction helpers for critical flows.

**Step 4: Run test to verify it passes**

Run: `go test ./packages/go/db/...`
Expected: pass.

**Step 5: Commit**

```bash
git add sqlc.yaml packages/go/db
git commit -m "feat: add typed database access layer"
```

### Task 6: Implement Ed25519 Signing and Replay Protection

**Files:**
- Create: `packages/go/auth/go.mod`
- Create: `packages/go/auth/signature.go`
- Create: `packages/go/auth/nonce.go`
- Create: `packages/go/auth/payload.go`
- Test: `packages/go/auth/signature_test.go`

**Step 1: Write the failing tests**

Cover:
- valid signature verification
- invalid signature rejection
- expired timestamp rejection
- reused nonce rejection
- payload canonicalization stability

**Step 2: Run test to verify it fails**

Run: `go test ./packages/go/auth/...`
Expected: fail because the auth package does not exist yet.

**Step 3: Write minimal implementation**

Implement request payload canonicalization, Ed25519 verification, timestamp window checks, and nonce storage helpers.

**Step 4: Run test to verify it passes**

Run: `go test ./packages/go/auth/...`
Expected: pass.

**Step 5: Commit**

```bash
git add packages/go/auth
git commit -m "feat: add request signing and replay protection"
```

### Task 7: Implement OAuth Device Authorization Flow

**Files:**
- Modify: `db/queries/auth.sql`
- Create: `apps/api/internal/config/oauth.go`
- Create: `apps/api/internal/service/auth/device_flow.go`
- Create: `apps/api/internal/http/auth_handler.go`
- Create: `apps/api/internal/http/oauth_callback_handler.go`
- Test: `apps/api/internal/service/auth/device_flow_test.go`

**Step 1: Write the failing tests**

Cover:
- create device authorization request
- poll pending request
- callback rejects invalid state
- callback exchanges code and binds public key to user

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/auth/...`
Expected: fail because the API auth flow is not implemented.

**Step 3: Write minimal implementation**

Add handlers and service logic for device authorize, poll, and OAuth callback processing against the school CAS endpoints.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/auth/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/api/internal db/queries/auth.sql
git commit -m "feat: add oauth device authorization flow"
```

### Task 8: Build API Bootstrap, Middleware, and Health Endpoints

**Files:**
- Create: `apps/api/cmd/labkit-api/main.go`
- Create: `apps/api/internal/http/router.go`
- Create: `apps/api/internal/http/middleware/request_id.go`
- Create: `apps/api/internal/http/middleware/error_response.go`
- Create: `apps/api/internal/http/health_handler.go`
- Test: `apps/api/internal/http/router_test.go`

**Step 1: Write the failing tests**

Add tests for:
- health endpoint response
- request ID middleware
- structured error serialization

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/http/...`
Expected: fail because router and middleware are not wired.

**Step 3: Write minimal implementation**

Create API bootstrap, router wiring, middleware stack, health endpoints, and shared error handling.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/http/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/api/cmd apps/api/internal/http
git commit -m "feat: bootstrap api service"
```

### Task 9: Add Lab Registration and Public Lab APIs

**Files:**
- Create: `apps/api/internal/service/labs/service.go`
- Create: `apps/api/internal/http/labs_handler.go`
- Test: `apps/api/internal/service/labs/service_test.go`

**Step 1: Write the failing tests**

Cover:
- admin lab registration with valid manifest
- reject structural updates
- expose public lab info only
- schedule visibility behavior

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/labs/...`
Expected: fail because lab services are not implemented.

**Step 3: Write minimal implementation**

Implement admin create/update, manifest validation, and public GET endpoints for lab listing and detail.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/labs/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/api/internal/service/labs apps/api/internal/http/labs_handler.go
git commit -m "feat: add lab registration and public lab apis"
```

### Task 10: Implement Submission Intake and Artifact Persistence

**Files:**
- Create: `apps/api/internal/service/submissions/service.go`
- Create: `apps/api/internal/http/submissions_handler.go`
- Create: `apps/api/internal/storage/artifacts.go`
- Test: `apps/api/internal/service/submissions/service_test.go`

**Step 1: Write the failing tests**

Cover:
- reject missing files
- reject invalid signature
- reject closed lab
- accept valid submission and persist artifact
- create submission and evaluation job atomically

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/submissions/...`
Expected: fail because submission flow is not implemented.

**Step 3: Write minimal implementation**

Implement multipart upload handling, manifest-based validation, archive creation, artifact persistence, and transactionally create submission and queued job.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/submissions/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/api/internal/service/submissions apps/api/internal/http/submissions_handler.go apps/api/internal/storage
git commit -m "feat: add submission intake and artifact storage"
```

### Task 11: Implement Evaluator Protocol Validation

**Files:**
- Create: `packages/go/evaluator/go.mod`
- Create: `packages/go/evaluator/result.go`
- Create: `packages/go/evaluator/validate.go`
- Test: `packages/go/evaluator/validate_test.go`

**Step 1: Write the failing tests**

Cover:
- valid scored result
- missing scores for declared metrics
- extra metric keys
- invalid detail format
- invalid verdict
- last-line extraction behavior

**Step 2: Run test to verify it fails**

Run: `go test ./packages/go/evaluator/...`
Expected: fail because evaluator validation is not implemented.

**Step 3: Write minimal implementation**

Implement result parsing, protocol validation, and manifest-aware score validation helpers.

**Step 4: Run test to verify it passes**

Run: `go test ./packages/go/evaluator/...`
Expected: pass.

**Step 5: Commit**

```bash
git add packages/go/evaluator
git commit -m "feat: add evaluator protocol validation"
```

### Task 12: Implement PostgreSQL Job Queue Primitives

**Files:**
- Create: `packages/go/jobs/go.mod`
- Create: `packages/go/jobs/queue.go`
- Create: `packages/go/jobs/claim.go`
- Test: `packages/go/jobs/queue_test.go`

**Step 1: Write the failing tests**

Cover:
- enqueue job
- single-worker claim
- multi-worker skip-locked behavior
- retry scheduling after failure

**Step 2: Run test to verify it fails**

Run: `go test ./packages/go/jobs/...`
Expected: fail because queue helpers do not exist.

**Step 3: Write minimal implementation**

Add helper functions and repository methods for enqueueing, claiming, acknowledging, and retrying jobs.

**Step 4: Run test to verify it passes**

Run: `go test ./packages/go/jobs/...`
Expected: pass.

**Step 5: Commit**

```bash
git add packages/go/jobs
git commit -m "feat: add postgres job queue primitives"
```

### Task 13: Build Worker Bootstrap and Docker Runner

**Files:**
- Create: `apps/worker/cmd/labkit-worker/main.go`
- Create: `apps/worker/internal/runtime/loop.go`
- Create: `apps/worker/internal/runtime/docker_runner.go`
- Create: `apps/worker/internal/runtime/tempdir.go`
- Test: `apps/worker/internal/runtime/docker_runner_test.go`

**Step 1: Write the failing tests**

Cover:
- runner builds correct docker invocation
- timeout is handled
- non-zero exit maps to evaluator error
- stdout last-line extraction is stable

**Step 2: Run test to verify it fails**

Run: `go test ./apps/worker/internal/runtime/...`
Expected: fail because worker runtime is not implemented.

**Step 3: Write minimal implementation**

Implement worker bootstrap, polling loop, temp workspace management, and Docker execution wrapper with required constraints.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/worker/internal/runtime/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/worker/cmd apps/worker/internal/runtime
git commit -m "feat: add worker runtime and docker runner"
```

### Task 14: Persist Evaluation Results and Refresh Leaderboard

**Files:**
- Create: `apps/worker/internal/service/evaluation/service.go`
- Create: `apps/worker/internal/service/evaluation/leaderboard.go`
- Test: `apps/worker/internal/service/evaluation/service_test.go`

**Step 1: Write the failing tests**

Cover:
- scored submission writes scores
- scored submission refreshes leaderboard
- build_failed preserves no scores
- invalid evaluator output becomes error and no quota charge semantics hook
- latest scored submission replaces old leaderboard pointer

**Step 2: Run test to verify it fails**

Run: `go test ./apps/worker/internal/service/evaluation/...`
Expected: fail because evaluation persistence is not implemented.

**Step 3: Write minimal implementation**

Implement transactional result persistence, score upsert, submission updates, and leaderboard refresh logic.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/worker/internal/service/evaluation/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/worker/internal/service/evaluation
git commit -m "feat: persist evaluation results and refresh leaderboard"
```

### Task 15: Expose History, Submission Detail, Nickname, Track, and Keys APIs

**Files:**
- Create: `apps/api/internal/http/history_handler.go`
- Create: `apps/api/internal/http/profile_handler.go`
- Create: `apps/api/internal/http/keys_handler.go`
- Test: `apps/api/internal/http/profile_handler_test.go`

**Step 1: Write the failing tests**

Cover:
- list submission history
- fetch submission detail with scores
- update nickname
- update track only when `pick = true`
- list and revoke keys

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/http/...`
Expected: fail because these authenticated handlers do not exist.

**Step 3: Write minimal implementation**

Implement authenticated API handlers and services for personal history, profile actions, and key management.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/http/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/api/internal/http apps/api/internal/service
git commit -m "feat: add user history and profile apis"
```

### Task 16: Expose Public Leaderboard API

**Files:**
- Create: `apps/api/internal/service/leaderboard/service.go`
- Create: `apps/api/internal/http/leaderboard_handler.go`
- Test: `apps/api/internal/service/leaderboard/service_test.go`

**Step 1: Write the failing tests**

Cover:
- board ordered by default metric
- board ordered by selected metric
- `asc` and `desc` handling
- track column behavior when `pick = true`
- hidden board before `visible`

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/leaderboard/...`
Expected: fail because leaderboard querying is not implemented.

**Step 3: Write minimal implementation**

Implement manifest-aware leaderboard query generation and public board handler responses.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/leaderboard/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/api/internal/service/leaderboard apps/api/internal/http/leaderboard_handler.go
git commit -m "feat: add public leaderboard api"
```

### Task 17: Implement Admin Export, Re-evaluation, and Queue Visibility

**Files:**
- Create: `apps/api/internal/http/admin_handler.go`
- Create: `apps/api/internal/service/admin/service.go`
- Test: `apps/api/internal/service/admin/service_test.go`

**Step 1: Write the failing tests**

Cover:
- export grades for latest leaderboard state
- trigger reeval creates fresh jobs
- recent queue status API
- reject structural lab update

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/admin/...`
Expected: fail because admin workflows are not implemented.

**Step 3: Write minimal implementation**

Implement admin export, re-evaluation orchestration, and queue monitoring endpoints.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/admin/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/api/internal/service/admin apps/api/internal/http/admin_handler.go
git commit -m "feat: add admin export and reeval apis"
```

### Task 18: Build CLI Skeleton and Config Storage

**Files:**
- Create: `apps/cli/cmd/labkit/main.go`
- Create: `apps/cli/internal/config/config.go`
- Create: `apps/cli/internal/http/client.go`
- Create: `apps/cli/internal/output/table.go`
- Test: `apps/cli/internal/config/config_test.go`

**Step 1: Write the failing tests**

Cover:
- config directory resolution
- key path resolution
- config read/write

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/config/...`
Expected: fail because CLI config does not exist.

**Step 3: Write minimal implementation**

Scaffold Cobra root command, local config persistence, HTTP client wrapper, and output formatting helpers.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/config/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/cli/cmd apps/cli/internal
git commit -m "feat: bootstrap cli application"
```

### Task 19: Implement CLI Auth and Key Management Commands

**Files:**
- Create: `apps/cli/internal/commands/auth.go`
- Create: `apps/cli/internal/commands/keys.go`
- Create: `apps/cli/internal/crypto/ed25519.go`
- Test: `apps/cli/internal/commands/auth_test.go`

**Step 1: Write the failing tests**

Cover:
- keypair generation
- start device flow
- poll until approved
- list keys
- revoke key

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: fail because auth commands are not implemented.

**Step 3: Write minimal implementation**

Implement local key generation, device auth flow, poll handling, and key management commands.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/cli/internal/commands apps/cli/internal/crypto
git commit -m "feat: add cli auth and key commands"
```

### Task 20: Implement CLI Submit, Board, History, Track, and Nick Commands

**Files:**
- Modify: `apps/cli/internal/commands/`
- Test: `apps/cli/internal/commands/submit_test.go`

**Step 1: Write the failing tests**

Cover:
- manifest-driven file validation
- signed submit request
- board display by metric
- history rendering
- nickname update
- track update

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: fail because core CLI commands are incomplete.

**Step 3: Write minimal implementation**

Implement the user-facing CLI commands and manifest-driven output formatting.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/cli/internal/commands
git commit -m "feat: add cli submission and board commands"
```

### Task 21: Build Vue Frontend Foundation

**Files:**
- Create: `apps/web/package.json`
- Create: `apps/web/vite.config.ts`
- Create: `apps/web/src/main.ts`
- Create: `apps/web/src/router.ts`
- Create: `apps/web/src/stores/`
- Create: `apps/web/src/styles/`
- Test: `apps/web/src/app.smoke.test.ts`

**Step 1: Write the failing tests**

Add a simple smoke test asserting the app bootstraps and router renders a page shell.

**Step 2: Run test to verify it fails**

Run: `cd apps/web && npm test`
Expected: fail because the frontend app is not scaffolded.

**Step 3: Write minimal implementation**

Create the Vue app, router, store foundation, shared styles, and test runner setup.

**Step 4: Run test to verify it passes**

Run: `cd apps/web && npm test`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/web
git commit -m "feat: bootstrap vue frontend"
```

### Task 22: Implement Public Leaderboard UI

**Files:**
- Create: `apps/web/src/views/LabListView.vue`
- Create: `apps/web/src/views/LeaderboardView.vue`
- Create: `apps/web/src/components/board/`
- Test: `apps/web/src/views/LeaderboardView.test.ts`

**Step 1: Write the failing tests**

Cover:
- render leaderboard rows
- metric switch behavior
- empty board state
- hidden board state before visibility

**Step 2: Run test to verify it fails**

Run: `cd apps/web && npm test -- LeaderboardView`
Expected: fail because leaderboard UI is not implemented.

**Step 3: Write minimal implementation**

Build the public leaderboard experience, metric tabs, lab list, and empty/hidden states.

**Step 4: Run test to verify it passes**

Run: `cd apps/web && npm test -- LeaderboardView`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/web/src/views apps/web/src/components/board
git commit -m "feat: add public leaderboard ui"
```

### Task 23: Implement OAuth Confirmation, Personal, and Admin UI

**Files:**
- Create: `apps/web/src/views/AuthConfirmView.vue`
- Create: `apps/web/src/views/ProfileView.vue`
- Create: `apps/web/src/views/AdminLabsView.vue`
- Create: `apps/web/src/views/AdminQueueView.vue`
- Test: `apps/web/src/views/ProfileView.test.ts`

**Step 1: Write the failing tests**

Cover:
- auth confirmation success state
- profile key list rendering
- admin queue status rendering

**Step 2: Run test to verify it fails**

Run: `cd apps/web && npm test -- ProfileView`
Expected: fail because these views do not exist.

**Step 3: Write minimal implementation**

Implement the browser confirmation page, self-service profile page, and focused admin pages.

**Step 4: Run test to verify it passes**

Run: `cd apps/web && npm test -- ProfileView`
Expected: pass.

**Step 5: Commit**

```bash
git add apps/web/src/views
git commit -m "feat: add auth profile and admin ui"
```

### Task 24: Add Local Development and Deployment Configuration

**Files:**
- Create: `deploy/docker-compose.yml`
- Create: `deploy/caddy/Caddyfile`
- Create: `deploy/.env.example`
- Create: `scripts/dev-up.sh`
- Create: `scripts/dev-down.sh`
- Test: `scripts/deploy_smoke_test.sh`

**Step 1: Write the failing test**

Create a smoke script that validates compose config and required environment variables.

**Step 2: Run test to verify it fails**

Run: `bash scripts/deploy_smoke_test.sh`
Expected: fail because deploy files do not exist.

**Step 3: Write minimal implementation**

Add compose definitions for API, worker, web, and PostgreSQL, plus reverse proxy config and development helper scripts.

**Step 4: Run test to verify it passes**

Run: `bash scripts/deploy_smoke_test.sh`
Expected: pass.

**Step 5: Commit**

```bash
git add deploy scripts
git commit -m "chore: add local deployment setup"
```

### Task 25: Add End-to-End Happy Path Verification

**Files:**
- Create: `scripts/e2e-happy-path.sh`
- Create: `scripts/testdata/`
- Test: `scripts/e2e-happy-path.sh`

**Step 1: Write the failing end-to-end script**

Script the minimal flow:
- boot services
- register a test lab
- perform a fake device binding shortcut in dev mode
- submit a fake artifact
- run worker
- verify leaderboard output

**Step 2: Run test to verify it fails**

Run: `bash scripts/e2e-happy-path.sh`
Expected: fail because the system is not fully implemented.

**Step 3: Write minimal implementation**

Add dev-only support and test data needed to make the happy path executable.

**Step 4: Run test to verify it passes**

Run: `bash scripts/e2e-happy-path.sh`
Expected: pass.

**Step 5: Commit**

```bash
git add scripts
git commit -m "test: add end-to-end happy path verification"
```

### Task 26: Final Verification and Documentation Pass

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-03-31-labkit-formal-design.md`
- Modify: `docs/plans/2026-03-31-labkit-implementation-plan.md`

**Step 1: Write the failing verification checklist**

Document the commands that must pass before completion and record the initial failing pass.

**Step 2: Run verification suite**

Run:
- `go test ./...`
- `cd apps/web && npm test`
- `bash db/migrations/schema_smoke_test.sh`
- `bash scripts/deploy_smoke_test.sh`
- `bash scripts/e2e-happy-path.sh`

Expected: all pass.

**Step 3: Fix remaining issues**

Address any final integration gaps uncovered by the verification suite. In the current repo snapshot, that included adding a root workspace verifier so `go test ./...` works from repo root.

**Step 4: Update docs**

Refresh the README with local setup, architecture overview, key operational commands, current verification status, and environment caveats.

**Step 5: Commit**

```bash
git add README.md docs/plans
git commit -m "docs: finalize setup and implementation guidance"
```
