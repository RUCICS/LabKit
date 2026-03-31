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
test -f deploy/docker-compose.yml
test -f .gitignore
test -f README.md
