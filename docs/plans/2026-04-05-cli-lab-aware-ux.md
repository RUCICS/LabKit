# CLI Lab-Aware UX Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve CLI responsiveness and clarity by adding loading feedback, lab-aware copy for lab-scoped commands, and clearer empty states without changing protocol-level LabKit identifiers.

**Architecture:** Add a thin lab UX context/helper in the CLI command layer, plus a small reusable loading wrapper around the existing line spinner. Commands that already fetch manifest data will use that context to render headings and confirmations. Platform-level copy remains unchanged.

**Tech Stack:** Go, Cobra, existing CLI spinner and rendering helpers, Go test

---

### Task 1: Document the lab-aware UX boundary in code

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/auth.go`
- Test: `apps/cli/internal/commands/submit_test.go`

**Step 1: Write the failing test**

Add tests that expect:

- lab-specific headings in `board` and `history`
- lab-specific success confirmations in `nick` and `track`
- platform copy like `LabKit web` remains unchanged elsewhere

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'Test(Board|History|Nick|Track)'`

Expected: FAIL because current output does not include the lab-aware wording.

**Step 3: Write minimal implementation**

Add a helper that derives a lab display name from `manifest.lab.name` with safe fallbacks, then thread it into the relevant renderers and success paths.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'Test(Board|History|Nick|Track)'`

Expected: PASS

### Task 2: Add loading wrappers for fetch-then-render commands

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/auth.go`
- Test: `apps/cli/internal/commands/submit_test.go`
- Test: `apps/cli/internal/commands/auth_test.go`

**Step 1: Write the failing test**

Add tests for non-TTY output that assert:

- `board` prints `Loading leaderboard…`
- `history` prints `Loading history…`
- `history <submission-id>` prints `Loading submission…`
- `keys` prints `Loading keys…`
- `revoke` prints `Revoking key…`

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'Test(Board|History|Keys|Revoke)'`

Expected: FAIL because the loading messages are not emitted yet.

**Step 3: Write minimal implementation**

Add a helper that starts/stops `lineSpinner` around a function body and use it in the listed commands, making sure error paths stop cleanly.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'Test(Board|History|Keys|Revoke)'`

Expected: PASS

### Task 3: Improve empty states

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Test: `apps/cli/internal/commands/submit_test.go`

**Step 1: Write the failing test**

Add tests asserting that empty leaderboard and empty history responses render a clear message and a next-step hint.

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'Test(Board|History).*Empty'`

Expected: FAIL because current output only shows table scaffolding.

**Step 3: Write minimal implementation**

Render an explicit empty-state message before the quota section, with submit/history guidance only where appropriate.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'Test(Board|History).*Empty'`

Expected: PASS

### Task 4: Run focused verification

**Files:**
- Test: `apps/cli/internal/commands/auth_test.go`
- Test: `apps/cli/internal/commands/submit_test.go`

**Step 1: Run focused package tests**

Run: `go test ./apps/cli/internal/commands`

Expected: PASS

**Step 2: Run CLI package tests**

Run: `go test ./apps/cli/...`

Expected: PASS

**Step 3: Review output for regressions**

Confirm that:

- spinner lines do not remain in final non-TTY output beyond the expected single loading line
- `LabKit web` wording is unchanged
- command names and protocol labels remain untouched
