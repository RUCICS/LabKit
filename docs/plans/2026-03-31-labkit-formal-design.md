# LabKit Formal Product Design

## Context

This document captures the agreed formal-product design for LabKit and annotates it with the current repository reality as of 2026-04-01. It is still the high-level target for a single-instance ICS platform, but the notes below distinguish implemented behavior from the original release intent.

## Implementation Snapshot

- The repo now contains the four planned apps: API, worker, CLI, and web.
- The checked-in compose stack now builds and runs the real API, worker, web, migration, PostgreSQL, and Caddy services.
- The source-run dev flow remains available for direct local debugging, but it is no longer the only real execution path.
- Dev-only support remains available for targeted local debugging:
  - `LABKIT_DEV_MODE=true` enables the `/api/dev/device/bind` shortcut for local device binding.
  - `LABKIT_WORKER_DEV_FAKE_EVALUATION=true` plus `LABKIT_WORKER_RUN_ONCE=true` enables a one-shot fake-evaluation worker flow when explicitly requested.
- Current verification status on 2026-04-01:
  - `go test ./...`, `cd apps/web && npm test`, `bash db/migrations/schema_smoke_test.sh`, `bash scripts/deploy_smoke_test.sh`, and `bash scripts/e2e-happy-path.sh` pass.

## Product Scope

LabKit is a leaderboard-oriented lab platform for ICS labs with numeric metrics, ranking, and repeated submissions. It is explicitly not a pass/fail autograder replacement.

Deployment target:
- Single ICS course
- Single long-lived deployment
- Single-machine production environment

Core product principles:
- All lab-specific behavior must be expressed through the manifest and evaluator protocol
- Platform code should not need per-lab customization
- OAuth is used only to establish identity
- Day-to-day CLI actions use Ed25519 request signing
- The platform stores artifacts and evaluation provenance for audit and re-evaluation

## Technical Baseline

- Backend: Go
- Database: PostgreSQL 18
- Frontend: Vue 3 + Vite + TypeScript
- CLI: Go + Cobra
- Deployment target: Docker Compose on a single Linux host
- Artifact storage: local filesystem directory
- Evaluation execution: Docker containers started by a dedicated worker

Current implementation caveat:
- The real Docker evaluator path is now wired by default. The dev-fake worker path remains only as an opt-in debug path.

Important database choice:
- Use PostgreSQL 18 instead of SQLite
- Use UUIDv7 for externally visible primary identifiers such as submissions and jobs

## System Architecture

The system is a modular monolith at the product level, with one additional worker process for evaluation execution.

Runtime components:
- `labkit-api`: HTTP API service
- `labkit-worker`: background worker for evaluator jobs
- `labkit-web`: Vue frontend
- `postgres`: primary database
- `artifacts/`: persistent submission archive directory

Responsibilities:
- API accepts requests, validates auth and manifest rules, persists submissions, exposes board/history/admin endpoints
- Compose now starts the same core services used by local verification: postgres, migrate, API, worker, web, and Caddy
- Worker verification now uses the real Docker evaluator runtime by default; dev fake evaluation is opt-in only
- Frontend provides public leaderboard pages, OAuth confirmation UI, key inventory, and admin pages for lab registration and queue inspection

Explicit non-goals:
- No microservice split
- No multi-tenant abstraction
- No web-first submission workflow

## Authentication Model

Chosen model:
- School OAuth handles identity confirmation
- Device authorization binds `student_id <-> public_key`
- CLI stores local Ed25519 private key
- All regular authenticated CLI requests are signed with Ed25519

Why:
- Avoid storing school tokens on student machines
- Enable long-lived CLI usage after initial device binding
- Support multiple devices per student cleanly

OAuth integration notes:
- Browser redirects to school CAS OAuth authorization endpoint
- Server exchanges authorization code for access token
- Server fetches user profile and extracts student identity
- OAuth callback must validate `state`

School platform request signing from `sign.md` is not reused for student CLI signing. It applies only when LabKit server calls school open-platform APIs in the future.

## Core Data Model

Primary entities:
- `users`
- `user_keys`
- `device_auth_requests`
- `labs`
- `lab_profiles`
- `submissions`
- `scores`
- `leaderboard`
- `used_nonces`
- `evaluation_jobs`

Important design adjustments from the original draft:
- `submissions.id` should be UUIDv7, not integer
- Introduce `evaluation_jobs` as a first-class queue table instead of overloading `submissions.status`

Entity semantics:
- `submissions`: student submission records and evaluation outcome
- `evaluation_jobs`: execution lifecycle for running evaluator work
- `scores`: structured metric rows
- `leaderboard`: materialized pointer to the latest scored submission per user and lab

## Backend Module Boundaries

Backend modules:
- `auth`
- `labs`
- `submissions`
- `evaluator`
- `leaderboard`
- `admin`

Module responsibilities:
- `auth`: OAuth device flow, key binding, signature verification, nonce checks, key revoke
- `labs`: manifest parsing, validation, registration, public lab info, schedule window checks
- `submissions`: upload validation, artifact archive, submission creation, history/detail queries
- `evaluator`: Docker execution, protocol parsing, result validation, score persistence, leaderboard refresh
- `leaderboard`: board queries, track switching, nickname update, export support
- `admin`: lab management, reeval, system monitoring

## Main Data Flows

### 1. Device Binding

- CLI generates local Ed25519 key pair
- CLI requests device authorization
- Server creates pending authorization with `device_code`, `user_code`, and OAuth `state`
- Browser completes OAuth login
- Server exchanges `code`, fetches user profile, binds student identity to public key
- CLI polls until authorization succeeds

### 2. Submission Intake

- CLI fetches public manifest and validates files locally
- CLI signs payload containing lab ID, timestamp, nonce, and content hash
- API verifies signature, timestamp window, nonce, schedule, and quota policy
- Uploaded files are archived to artifact storage
- API creates `submissions` and `evaluation_jobs` atomically
- API returns `queued`

### 3. Evaluation

- Worker claims queued job using PostgreSQL locking
- Worker restores files from artifact archive to a temporary directory
- Worker runs evaluator container with network disabled and resource limits
- Worker reads stdout last line JSON
- Worker validates verdict, scores, and detail payload
- Worker updates submission outcome, scores, and leaderboard in one transaction

### 4. Query Paths

- Submission history/detail reads from `submissions` and `scores`
- Board reads from `leaderboard`, `lab_profiles`, and `scores`
- Board never scans all submission history dynamically

## Leaderboard Rules

Chosen rule:
- The leaderboard always uses the latest `verdict = scored` submission

Not chosen:
- Historical best score leaderboard
- Mixed ranking semantics across different labs

Track behavior:
- If `board.pick = true`, students may declare a track
- Declared track affects default board view and exported grading view
- Board ordering itself is always controlled by a single selected metric view

## Frontend Scope

Student-facing pages:
- Lab list / home
- Public leaderboard page
- Device authorization confirmation page
- Personal page for key inventory

Admin pages:
- Lab management
- Evaluation queue and failure monitoring
- Export grades and trigger re-evaluation

Explicitly excluded from the first formal release:
- Web-based primary submission flow
- Online code editor
- Rich profile/social systems
- Complex RBAC
- Real-time websockets

## Repository Structure

Recommended monorepo layout:

```text
LabKit/
  apps/
    api/
    worker/
    cli/
    web/
  packages/
    go/
      manifest/
      auth/
      db/
      jobs/
      evaluator/
      labkit/
  db/
    migrations/
    queries/
  deploy/
    docker-compose.yml
    caddy/
    postgres/
  docs/
    plans/
  scripts/
```

Guidelines:
- Keep deployable apps separate
- Keep shared Go code in focused domain packages
- Keep DB migrations and sqlc queries outside app-specific folders
- Avoid generic directories such as `common` or `utils` unless a package has a precise domain

## Queue and Worker Strategy

Chosen strategy:
- PostgreSQL-backed job queue
- No Redis in the first formal release

Worker behavior:
- Claim jobs with `FOR UPDATE SKIP LOCKED`
- Record attempts and failure reasons
- Support crash recovery and retries

Why:
- Single-machine deployment does not justify another infrastructure dependency
- PostgreSQL is sufficient and operationally simpler here

## Testing Strategy

Must-have automated tests:
- Manifest parsing and validation
- Ed25519 signing and replay protection
- Evaluator protocol validation
- Leaderboard semantics
- Submission-to-evaluation integration paths

Recommended but secondary early tests:
- OAuth callback integration
- CLI command tests
- Frontend page tests
- Admin API integration tests

Quality rule:
- Behavior-changing bug fixes must include regression tests

## Error Model and Observability

Error classes:
- `user_error`
- `system_error`
- `evaluator_error`
- `admin_error`

Operational goals:
- Every failed submission must be diagnosable
- Logs must be structured JSON
- Logs should carry `request_id`, `submission_id`, `job_id`, `lab_id`, and `user_id` when available

Required admin visibility:
- Queued jobs
- Running jobs
- Recent failures
- Recent timeouts
- Per-lab submission activity

Initial observability stance:
- Strong structured logs and admin pages first
- Do not require a full metrics stack on day one

## Verification Reality

The current repository supports the full verification checklist from Task 26, and the Docker-backed commands now pass in this workspace:

- `bash db/migrations/schema_smoke_test.sh`
- `bash scripts/e2e-happy-path.sh`

The earlier controller note about Docker daemon access was a transient environment issue, not a repository defect.

## First Formal Release Scope

Must ship:
- Manifest parsing and validation
- OAuth device binding
- Ed25519 request signing
- CLI auth and submission workflow
- Artifact persistence
- Docker evaluator execution
- Strict evaluator output validation
- Public leaderboard
- History, nickname, track, keys, revoke
- Admin lab management, export, reeval, queue visibility
- Audit metadata such as content hash and image digest

Must not expand into:
- Multi-course multi-tenant support
- Object-storage abstraction plugins
- Web-primary submission experience
- Cluster scheduling
- Complex organization/permission systems
- Built-in plagiarism platform

## Outcome

This design defines LabKit as a single-course platform with clear auth boundaries, auditable evaluation, manifest-driven customization, and a restrained operational scope. The current repo now covers the planned app surfaces, real compose deployment path, and real evaluator runtime, while still keeping a small set of explicit dev-only shortcuts for local debugging.
