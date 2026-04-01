# CLI UX Redesign — Design Spec

**Date:** 2026-04-01  
**Scope:** `apps/cli/` — all user-facing output and live rendering  
**Goal:** Transform the current naive output into a modern, visually polished CLI experience without introducing interactive TUI (no Bubbletea). Pure Lipgloss + ANSI rendering.

---

## 1. Visual Language

### Color Palette (Tokyo Night)

All colors are Lipgloss `Color()` values using 256-color or true-color terminal codes.

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| Primary blue | `#7aa2f7` | blue | Titles, active/in-progress states, spinner |
| Success green | `#9ece6a` | green | Passed, your leaderboard row, completed stages |
| Warning yellow | `#e0af68` | yellow | Gold medal, warn-level scores, queued |
| Error red | `#f7768e` | red | Failed, error states |
| Muted gray | `#565f89` | gray | Timestamps, secondary labels, inactive stages |
| Text | `#c0caf5` | light | Default body text |
| Subtle bg | `#1f2335` | dark | Panel/row highlight background |

### Typography Rules

- **Titles**: bold + primary blue. Format: `● Verb  noun` (in-progress) or `✓ Verb  noun` (done) or `✗ Verb  noun` (error)
- **Section headers**: bold, left-aligned, followed by single blank line
- **Secondary info**: muted gray, smaller conceptually (timestamps, IDs, counts)
- **Timestamps**: always relative (`2h ago`, `1d ago`), never raw ISO
- **Scores**: colored by value — green if at/above the metric's threshold (from manifest), yellow if below, muted if unavailable (`—`). If the manifest defines no threshold for a metric, always render in default text color.
- **Status badges**: inline colored text — `PASSED` green, `FAILED` red, `QUEUED` yellow, `RUNNING` blue

### Spacing

- One blank line between the title line and body content
- Two spaces indent for body fields
- Separator lines use `─` (U+2500), full terminal width or content width
- Vertical grouping accent: `╷ … ╵` left-border block for result cards

---

## 2. `labkit submit`

### Phase 1 — In Progress (TTY mode, in-place rewrite)

Single-region live renderer that rewrites 4 lines in place using ANSI cursor-up + clear-line.

```
● Submitting  matrix-mul
                                               (blank line)
  ◐ running tests                              4.2s
  ████████████░░░░░░░░  60%
  ✓ packed   ✓ queued   ● running   ○ scoring
```

- Spinner cycles through `◐ ◓ ◑ ◒` at 120ms
- Progress bar width adapts to terminal width; uses `█` for filled, `░` for empty. Percentage shown is stage-based: `completedStages / totalStages`. Stages are: packed, queued, running, scoring (4 total).
- Stage indicator at bottom: completed stages green `✓`, active blue `●`, pending muted `○`
- Time counter updates in place, right-aligned
- On non-TTY (pipe/redirect): emit simple one-line status updates per phase change, no animation

### Phase 2 — Completed (static, stays in terminal)

After polling resolves, clear the live region and print static summary:

```
✓ Submitted  matrix-mul

  ✓ packed        0.3s
  ✓ queued        0.1s
  ✓ running       4.2s
  ✓ scoring       0.4s

  ╷
  │  PASSED   5.0s total
  │  4f3a9b2c
  ╵

  Scores
  correctness    100%
  performance     85.2%
  overall         92.6%
```

- Stage list uses green `✓` with per-stage elapsed time
- Result card uses `╷ … ╵` vertical accent, title colored by outcome (green/red)
- Score values colored: green if good, yellow if below threshold, muted if `—`

### Failure Output

```
✗ Submitted  matrix-mul

  ✗ running       2.1s   ← red, stopped here

  ╷
  │  FAILED   2.4s total
  │  4f3a9b2c
  │
  │  test_case_3: expected 42, got 17
  │  test_case_7: time limit exceeded
  ╵
```

- Failure details rendered inside the `╷ … ╵` block, indented
- Stage that failed shown in red

### `--detach` / `--no-wait` flags

Print one-line confirmation and exit immediately:

```
● Submitted  matrix-mul  (detached)
  id    4f3a9b2c
```

---

## 3. `labkit board`

### Layout

```
Leaderboard  matrix-mul · sorted by score · 24 participants

  #    NICKNAME              VALUE    UPDATED
  ────────────────────────────────────────────
  🥇   alice        ██████   95.5%    2h ago
  🥈   bob          █████░   92.0%    5h ago
  🥉   charlie      █████░   90.1%    6h ago
  →    you (huanc)  █████░   88.3%    1h ago   ← green highlighted row
       dave         ████░░   85.1%    3h ago
       eve          ████░░   81.0%    1d ago
```

### Background Fill Bar (key design element)

Each row has a background color fill proportional to its score relative to the top score:
- Fill width = `(score / maxScore) * rowWidth` characters. If maxScore is 0 or all scores are equal, fill all rows to 100% width.
- Fill color: gold tint for rank 1, blue tint for ranks 2–3, accent tint for others, green tint for your row
- Background opacity is subtle (dark tint), text remains legible on top
- Implementation: render fixed-width row string, split at fill point, set `Background()` on filled segment

### Your Row

- Rank column shows `→` instead of number
- All text colored green
- Row background uses green tint instead of blue

### Medal Logic

- Rank 1: `🥇` (gold text)
- Rank 2: `🥈` (silver text)  
- Rank 3: `🥉` (bronze text)
- Other ranks: plain number, muted gray

### Multi-metric boards (`--by` flag)

Show metric tabs above the table:

```
  score  |  correctness  |  performance
  ─────────────────────────────────────
```

Active tab underlined/bold; others muted. Not interactive — user passes `--by score` to switch.

---

## 4. `labkit history`

```
Submission history  matrix-mul

  ID          STATUS    SCORE    SUBMITTED
  ──────────────────────────────────────────
  4f3a9b2c   PASSED    92.6%    1h ago
  a1b2c3d4   FAILED      —      3h ago
  8e9f0a1b   PASSED    88.3%    1d ago

  3 submissions
```

- Status colored: PASSED green, FAILED red, QUEUED yellow, RUNNING blue
- Score `—` in muted gray when not available
- Footer shows total count

---

## 5. `labkit auth`

### Authorization Flow

```
● Authorizing  labkit.example.com

  Open this URL in your browser:
  https://labkit.example.com/device

  Enter code:  ABCD-1234            ← bold, easy to read

  ◐ Waiting for authorization…
```

- Spinner animates while polling
- URL on its own line, full and copyable

### Completion

```
✓ Authorized  labkit.example.com

  key       abc12345
  device    laptop
  server    labkit.example.com
```

---

## 6. `labkit keys` and `labkit revoke`

### keys

```
Bound keys  2 total

  ID          DEVICE      CREATED
  ───────────────────────────────
  abc12345    laptop      2d ago
  def67890    desktop     5d ago
```

### revoke

Success:
```
✓ Revoked  key abc12345
```

---

## 7. `labkit nick` and `labkit track`

Short confirmations only:

```
✓ Nickname updated  huanc
```

```
✓ Track set  baseline
```

---

## 8. `labkit web`

```
● Opening  labkit.example.com/profile

  ✓ Launched browser
```

If browser cannot be opened (non-TTY):

```
● Web session  labkit.example.com/profile

  Open this URL to continue:
  https://labkit.example.com/profile?ticket=…
```

---

## 9. Error Handling

All command-level errors use a consistent format:

```
✗ Error  <command> failed

  <human-readable explanation>
  <suggested remediation if applicable>
```

Examples:

```
✗ Error  submit failed

  The server returned 401 Unauthorized.
  Run `labkit auth` to re-authorize this device.
```

```
✗ Error  board unavailable

  The leaderboard is hidden by the instructor.
```

Network/connection errors:
```
✗ Error  cannot reach server

  labkit.example.com is not responding.
  Check your network connection or try again later.
```

---

## 10. Implementation Notes

### What changes in `ui/`

- **`theme.go`**: Add full Tokyo Night palette as named constants. Add `RelativeTime(t time.Time) string` helper.
- **`table.go`**: Extend `CompactTable` with optional separator line rendering and status-colored cells.
- **`submit_live.go`**: Replace single-line spinner with 4-line region renderer (cursor-up rewrite). Add progress bar and stage indicator row.
- **New `ui/bgbar.go`**: `BgFillRow` — renders a fixed-width string with proportional background color fill. Used by leaderboard rows.
- **New `ui/result.go`**: `ResultBlock` — renders the `╷ … ╵` vertical accent card for submission results.
- **`styles.go`**: Add `ErrorLine`, `RelativeTimestamp`, `ScoreValue(v float32, threshold float32)` helpers.

### What changes in commands

- **`lab_commands.go`**: Update `runBoard` to use `BgFillRow` per row, medal logic, background fill calculation. Update `runHistory` to use colored status and relative timestamps. Update `renderSubmissionFinal` to use `ResultBlock`.
- **`auth.go`**: Update auth flow output to new title/field format.
- **`keys.go`**: Update table output with relative timestamps.

### No new dependencies required

All effects are achievable with existing Lipgloss (`Background()`, `Foreground()`, `Bold()`, `Width()`) and ANSI escape codes already in use. No Bubbletea needed.

---

## 11. Out of Scope

- Interactive menus or prompts (no Bubbletea)
- Pagination of board/history results
- Color theme configuration (Tokyo Night is the single theme)
- Animations in non-TTY environments
