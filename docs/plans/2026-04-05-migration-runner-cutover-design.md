# Migration Runner Cutover Design

## Goal

Replace the current hand-maintained schema-sniffing shell migration flow with a repo-owned Go migration runner built on `golang-migrate`, while preserving a safe one-time cutover path for existing environments that are already manually verified to be at schema version `0007`.

## Context

Today [migrate.sh](/home/starrydream/ICS2/LabKit/deploy/migrate.sh) conditionally applies individual SQL files by checking whether certain tables or columns exist. That approach was acceptable during early schema evolution, but it has clear limits:

- each new migration requires bespoke shell logic
- there is no version table or dirty-state tracking
- error recovery and concurrent deployment behavior are tool-less
- clean environments and long-lived environments are managed through different implicit heuristics

The team has confirmed that production is already at `0007`, and accepts a one-time cutover that requires old environments to be manually confirmed before baseline.

## Decision

Use `golang-migrate` as the migration engine, embedded inside a small Go runner owned by this repository.

### Why `golang-migrate`

- It matches the existing `db/migrations/NNNN_name.up.sql` / `.down.sql` layout.
- It provides a standard version table and dirty-state semantics.
- It supports both library and CLI usage, so the repo can own a stable interface while delegating execution semantics to a mature tool.

### Why a repo-owned runner instead of invoking the CLI directly

- Keeps deployment behavior in repo code rather than in image-specific packaging.
- Makes cutover policy explicit and testable.
- Avoids depending on an external binary being installed in every image.
- Lets us keep `deploy/migrate.sh` as a thin entrypoint wrapper.

## Scope Boundary

### In scope

- Introduce a Go migration runner binary.
- Use `golang-migrate` against the existing `db/migrations` directory.
- Add explicit cutover behavior for old environments already at `0007`.
- Simplify `deploy/migrate.sh` into a thin wrapper over the runner.

### Out of scope

- Backfilling or auto-detecting arbitrary pre-`0007` schema states.
- Converting to a schema-diff workflow such as Atlas.
- Rewriting historical migrations.
- Supporting mixed "old shell + new version table" migration ownership indefinitely.

## Environment Model

There are only three supported database states after the cutover:

### 1. Clean environment

The database has no application tables and no migration version table.

Expected behavior:
- run `up`
- migrations apply from `0001` through latest

### 2. Existing pre-managed environment

The database already contains LabKit schema objects, has no migration version table, and has been manually confirmed to be equivalent to `0007`.

Expected behavior:
- run `baseline --version 7` once
- this records version ownership but does not execute migration SQL
- all later runs use normal `up`

### 3. Managed environment

The migration version table already exists.

Expected behavior:
- run normal `up`

## Runner Behavior

Create a binary such as `labkit-migrate` with these commands:

- `version`
- `baseline --version <n>`
- `up`

### `version`

Reports:
- whether the migration version table exists
- current version
- dirty state if present

### `baseline --version 7`

Rules:
- only allowed when the version table does not yet exist
- intended only for manually verified old environments
- creates or initializes the migration metadata at version `7`
- does not execute SQL files
- fails if the database appears empty, to prevent accidentally baselining a new environment

### `up`

Rules:
- if the version table exists, run normal `golang-migrate up`
- if the version table does not exist and the database is empty, run normal `up` from `0001`
- if the version table does not exist and the database is not empty, fail with a clear message:
  "database has existing schema but is not baselined; manually confirm schema is at 0007, then run baseline --version 7"

This is the critical safety gate that prevents a live old database from being misclassified as a clean one.

## Database State Detection

The runner needs a small amount of local introspection:

- check whether the migration version table exists
- check whether the database already contains LabKit-owned tables, for example `users`, `labs`, or `submissions`

This detection is only for routing behavior during cutover, not for deciding which migrations to apply.

## Deployment Design

Keep [migrate.sh](/home/starrydream/ICS2/LabKit/deploy/migrate.sh), but reduce it to:

- wait for Postgres readiness
- assemble connection env
- invoke the runner

Recommended default deployment command:

- `labkit-migrate up`

Operational cutover steps for an old environment:

1. Manually confirm the database schema is already at `0007`.
2. Run `labkit-migrate baseline --version 7` once.
3. Switch the environment to the new default path: `labkit-migrate up`.

For new environments:

1. Start from an empty database.
2. Run `labkit-migrate up`.

## Failure Semantics

- If `golang-migrate` reports dirty state, the runner should surface that clearly and exit non-zero.
- If an existing un-managed database is detected during `up`, the runner must fail closed instead of guessing.
- If `baseline` is invoked against an empty database or a DB that already has a version table, it must fail.

## Migration Ownership After Cutover

After this change:

- historical shell checks are removed
- `golang-migrate` becomes the single owner of migration order and state
- every future schema change must be added as a new numbered SQL migration

## Recommendation

Implement the runner as a small Go binary in this repository, keep the cutover intentionally strict, and treat `baseline --version 7` as a one-time operational step for already-verified legacy environments.
