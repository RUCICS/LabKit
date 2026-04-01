# CLI Loading Polish Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Polish leaderboard rank presentation and loading feedback so LabKit CLI waiting states feel consistent, animated, and product-grade in TTY mode while remaining clean in non-interactive output.

**Architecture:** Add a shared command-layer spinner for single-line waiting states and update the submit live renderer to share animation timing while keeping its specialized multi-line layout. Refine leaderboard row layout to separate the current-user marker from the rank symbol so medals and alignment survive all combinations.

**Tech Stack:** Go, Cobra, Lip Gloss, ANSI terminal rendering, Go tests.

---

### Task 1: Lock Down Leaderboard Rank Behavior

**Files:**
- Modify: `apps/cli/internal/commands/submit_test.go`
- Modify: `apps/cli/internal/commands/lab_commands.go`

**Step 1: Write the failing test**

- Add a test for a current-user leaderboard row ranked in the top three.
- Assert the plain output still contains both the arrow marker and the medal.
- Assert adjacent rows remain aligned in plain-text form.

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run TestRenderBoardKeepsMedalForCurrentUserTopThree -count=1`

**Step 3: Write minimal implementation**

- Split the current-user marker into its own fixed-width column.
- Keep rank rendering independent so medals are preserved.
- Adjust row width/header construction to maintain alignment.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'TestRenderBoard(ShowsMedalsAndBgFill|HighlightsCurrentUserRow|KeepsMedalForCurrentUserTopThree)' -count=1`

**Step 5: Commit**

```bash
git add apps/cli/internal/commands/lab_commands.go apps/cli/internal/commands/submit_test.go
git commit -m "fix(cli): preserve leaderboard medals for current user"
```

### Task 2: Add A Shared TTY Spinner Primitive

**Files:**
- Create: `apps/cli/internal/commands/spinner.go`
- Create: `apps/cli/internal/commands/spinner_test.go`
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/web.go`

**Step 1: Write the failing test**

- Add tests for spinner rendering in TTY and non-TTY modes.
- Add a command-level auth test that expects animated waiting text to be produced through the shared spinner path without raw control-code regressions in non-TTY mode.

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'TestSpinner|TestAuthCommand' -count=1`

**Step 3: Write minimal implementation**

- Implement a reusable spinner type with start/update/stop behavior.
- Make it no-op/static when `deps.IsTTY()` is false.
- Use it in auth polling and browser session setup where waits are user-visible.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'TestSpinner|TestAuthCommand' -count=1`

**Step 5: Commit**

```bash
git add apps/cli/internal/commands/spinner.go apps/cli/internal/commands/spinner_test.go apps/cli/internal/commands/auth.go apps/cli/internal/commands/web.go
git commit -m "feat(cli): add shared spinner for waiting states"
```

### Task 3: Upgrade Submit Live Motion

**Files:**
- Modify: `apps/cli/internal/commands/submit_live.go`
- Modify: `apps/cli/internal/commands/submit_live_test.go`

**Step 1: Write the failing test**

- Add tests that assert the progress bar uses high-resolution Unicode block glyphs when supported.
- Add a test that verifies the rendered bar changes as the frame index advances even when status is unchanged.
- Add a fallback-oriented test for environments where Unicode block width is not reliable.

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'TestFormatSubmitLiveBlock|TestSubmitLiveRenderer' -count=1`

**Step 3: Write minimal implementation**

- Replace the bar renderer with a high-resolution Unicode block renderer using left-block glyphs for partial cells.
- Add a subtle blue-green pulse highlight that moves across the filled region by brightness only.
- Preserve a simpler fallback renderer for terminals that cannot render the Unicode block glyphs cleanly.
- Reuse the shared spinner frame set for line-one motion.

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'TestFormatSubmitLiveBlock|TestSubmitLiveRenderer' -count=1`

**Step 5: Commit**

```bash
git add apps/cli/internal/commands/submit_live.go apps/cli/internal/commands/submit_live_test.go
git commit -m "feat(cli): polish submit live progress animation"
```

### Task 4: Run Full CLI Verification

**Files:**
- Verify only

**Step 1: Run targeted package tests**

Run: `go test ./apps/cli/internal/commands ./apps/cli/internal/ui -count=1`

**Step 2: Run full CLI tests**

Run: `go test ./apps/cli/... -count=1`

**Step 3: Inspect terminal build**

Run: `CGO_ENABLED=0 go build ./apps/cli/cmd/labkit`

**Step 4: Commit**

```bash
git add .
git commit -m "test(cli): verify loading polish changes"
```
