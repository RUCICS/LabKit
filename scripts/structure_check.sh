#!/usr/bin/env bash
set -euo pipefail

test -d apps/api
test -d apps/worker
test -d apps/cli
test -d apps/web
test -d packages/go/manifest
test -d packages/go/auth
test -d packages/go/db
test -d packages/go/jobs
test -d packages/go/evaluator
test -d packages/go/labkit
test -d db/migrations
test -d db/queries
test -f go.work
test -f apps/api/go.mod
test -f apps/worker/go.mod
test -f apps/cli/go.mod
test -f packages/go/manifest/go.mod
test -f packages/go/auth/go.mod
test -f packages/go/db/go.mod
test -f packages/go/jobs/go.mod
test -f packages/go/evaluator/go.mod
test -f packages/go/labkit/go.mod
test -f deploy/docker-compose.yml
test -f .gitignore
test -f README.md
