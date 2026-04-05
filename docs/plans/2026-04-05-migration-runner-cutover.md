# Migration Runner Cutover Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the legacy schema-sniffing migration shell logic with a Go-based `golang-migrate` runner that supports one-time baseline to version `0007` for manually verified old environments and normal `up` for clean or already-managed environments.

**Architecture:** Add a repo-owned Go binary that wraps `golang-migrate` library calls plus a small amount of database-state detection. Keep `deploy/migrate.sh` as a thin shell entrypoint. Fail closed for ambiguous states. Clean databases run full `up`; old pre-managed databases must be explicitly baselined to `7` once.

**Tech Stack:** Go, `golang-migrate`, PostgreSQL, existing `db/migrations` SQL files, shell entrypoint

---

### Task 1: Add design-level safety tests for state classification

**Files:**
- Create: `apps/migrate/internal/runner/runner_test.go`
- Modify: `apps/migrate/internal/runner/runner.go`

**Step 1: Write the failing test**

Add tests for:

- empty DB + no version table => `up` is allowed
- non-empty DB + no version table => `up` fails with baseline guidance
- version table present => `up` delegates to migrate
- `baseline 7` fails on empty DB
- `baseline 7` fails when version table already exists

**Step 2: Run test to verify it fails**

Run: `go test ./apps/migrate/...`

Expected: FAIL because the runner does not exist yet.

**Step 3: Write minimal implementation**

Implement state classification and command dispatch logic with small interfaces around DB inspection and migrate engine calls.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/migrate/...`

Expected: PASS

### Task 2: Implement the Go migration runner

**Files:**
- Create: `apps/migrate/cmd/labkit-migrate/main.go`
- Create: `apps/migrate/internal/runner/runner.go`
- Create: `apps/migrate/go.mod`
- Optionally create: `apps/migrate/internal/runner/postgres.go`

**Step 1: Write the failing test**

Add command-level tests or focused unit tests that verify:

- `version`
- `baseline --version 7`
- `up`

parse correctly and route to the right runner behavior.

**Step 2: Run test to verify it fails**

Run: `go test ./apps/migrate/...`

Expected: FAIL until command wiring exists.

**Step 3: Write minimal implementation**

Build the smallest runner that:

- reads connection info from env / database URL
- finds the migrations directory
- inspects DB state
- calls `golang-migrate`

**Step 4: Run test to verify it passes**

Run: `go test ./apps/migrate/...`

Expected: PASS

### Task 3: Replace shell migration ownership with the runner

**Files:**
- Modify: `deploy/migrate.sh`
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/api.Dockerfile` or add a dedicated migration image build path

**Step 1: Write the failing test**

Add a deploy-oriented test or snapshot/command test for the shell script behavior if practical. If not practical, add a documented manual verification checklist in the plan and keep code changes minimal and explicit.

**Step 2: Run test to verify it fails**

Run the most focused available verification for the script or build path.

Expected: FAIL or no-op until the runner is wired in.

**Step 3: Write minimal implementation**

Update deployment so the migration container invokes:

- `labkit-migrate up`

and no longer manually applies individual SQL files.

**Step 4: Run test to verify it passes**

Run the same verification used above, plus build the relevant image or command path.

### Task 4: Add operational cutover safeguards and docs

**Files:**
- Modify: `docs/reference/local-auth.md` if migration instructions appear there
- Create or modify: `docs/reference/deploy.md` or a dedicated migration ops doc

**Step 1: Write the failing test**

If docs tests exist, update them; otherwise define a manual checklist:

- old DB must be manually verified at `0007`
- run `baseline --version 7` once
- all future deploys use `up`
- clean DBs never use baseline

**Step 2: Run verification**

Check links, commands, and examples for consistency.

**Step 3: Write minimal documentation**

Document:

- first-time cutover for old environments
- setup for fresh environments
- what to do on dirty-state failure

**Step 4: Run verification**

Re-check docs references and command examples.

### Task 5: Fresh verification

**Files:**
- Test: `apps/migrate/...`
- Test: deployment build path

**Step 1: Run migration runner tests**

Run: `go test ./apps/migrate/...`

Expected: PASS

**Step 2: Run broader regression checks**

Run: `go test ./apps/api/... ./packages/go/db`

Expected: PASS

**Step 3: Verify deployment integration**

Run the most direct command available to prove the migration image or command path builds cleanly.

Expected: success with no ambiguous cutover behavior.

Plan complete and saved to `docs/plans/2026-04-05-migration-runner-cutover.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?  
