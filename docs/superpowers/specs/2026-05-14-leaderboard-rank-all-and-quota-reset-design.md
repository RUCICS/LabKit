# Design: Leaderboard `rank_all` Config + Admin Quota Reset

**Date:** 2026-05-14

---

## Overview

Two independent features:

1. **`rank_all` config** — make the leaderboard ranking mode configurable. Default: all users across every track appear with a real rank number. Optional old behavior (only track-matching users get a rank) retained via `rank_all = false`.
2. **Admin quota reset** — admin can zero out all users' daily quota usage for a given lab via a web panel button, by marking today's charged submissions as a new `admin_reset` state.

---

## Feature 1: Leaderboard `rank_all`

### Background

When a lab has `[board] pick = true`, users select a track (one of the metric IDs). Currently:
- The backend assigns sequential ranks 1–N to **everyone** based on score sort.
- The CLI and web **override** this with their own `nextRank` counter that only increments for track-matching users, so non-track users show "—" and the rank numbers skip those slots.

This is inconsistent: the backend and client compute ranks independently and will diverge. The fix is to make the backend the **single source of truth** for rank assignment.

### New Behavior

| `rank_all` value | Who gets a rank | Visual de-emphasis |
|---|---|---|
| `true` (default) | Everyone | Non-track users shown at reduced opacity |
| `false` | Only track-matching users | Non-track users shown at reduced opacity, rank = `null` |

When `pick = false` (no track mode), `rank_all` is irrelevant — all users are always ranked.

### Manifest Change

**File:** `packages/go/manifest/manifest.go`

```go
type BoardSection struct {
    RankBy  string `toml:"rank_by"`
    Pick    bool   `toml:"pick"`
    RankAll *bool  `toml:"rank_all"` // nil → default true when pick=true
}

// IsRankAll returns true if all users should receive a rank number.
// When not explicitly set, defaults to true.
func (b BoardSection) IsRankAll() bool {
    return b.RankAll == nil || *b.RankAll
}
```

In `normalize()`, add:

```go
if m.Board.Pick && m.Board.RankAll == nil {
    t := true
    m.Board.RankAll = &t
}
```

### Backend: `GetBoard()` Change

**File:** `apps/api/internal/service/leaderboard/service.go`

Add `RankAll bool` to the `Board` response struct:

```go
type Board struct {
    LabID          string                      `json:"lab_id"`
    SelectedMetric string                      `json:"selected_metric"`
    RankAll        bool                        `json:"rank_all"`
    Metrics        []BoardMetric               `json:"metrics"`
    Rows           []BoardRow                  `json:"rows"`
    Quota          *submissionsvc.QuotaSummary `json:"quota,omitempty"`
}
```

Change the rank-assignment loop (currently lines 186–189) to conditionally skip non-track users when `rank_all = false`:

```go
rankAll := parsed.Board.IsRankAll()
board.RankAll = rankAll
nextRank := 1
for i := range rows {
    trackMatches := !parsed.Board.Pick ||
        strings.EqualFold(rows[i].row.Track, board.SelectedMetric)
    if rankAll || trackMatches {
        rows[i].row.Rank = nextRank
        nextRank++
    }
    // Rank stays 0 (zero value) for non-participants when rank_all=false
    board.Rows = append(board.Rows, rows[i].row)
}
```

`BoardRow.Rank` stays `int`; a value of `0` means "no rank" (non-participant in `rank_all=false` mode).

### Web Change

**File:** `apps/web/src/components/board/LeaderboardTable.vue`

Add `rankAll` prop:

```typescript
const props = defineProps<{
  rows: LeaderboardRow[];
  metrics: LeaderboardMetric[];
  selectedMetricId: string;
  rankAll: boolean;
  closeAt?: string;
  apiHint: string;
  metricUnits?: Record<string, string>;
}>();
```

Replace the `nextRank` counter in `rowsWithDisplayRank` with:

```typescript
const rowsWithDisplayRank = computed(() =>
  props.rows.map((row) => {
    const participates = !isTrackBased.value || !!row.track && row.track === props.selectedMetricId;
    const displayRank = row.rank > 0 ? row.rank : null;
    return { row, participates, displayRank };
  })
);
```

`participates` now only drives the visual de-emphasis class (`board-table__row--unranked`), not rank numbering. The `row.rank` from the API is authoritative.

**File:** `apps/web/src/components/board/types.ts`

Add `rank_all` to `LeaderboardBoard`:

```typescript
export interface LeaderboardBoard {
  lab_id: string;
  selected_metric: string;
  rank_all: boolean;
  metrics: LeaderboardMetric[];
  rows: LeaderboardRow[];
  quota?: QuotaSummary;
}
```

**File:** `apps/web/src/views/LeaderboardView.vue` — add `:rank-all="board.rank_all"` to the `<LeaderboardTable>` usage at line 263.

### CLI Change

**File:** `apps/cli/internal/commands/lab_commands.go`

Remove the `displayRanks` / `nextRank` block (lines ~1161–1171). Instead, use `row.Rank` directly from the API response. Display `"—"` when `row.Rank == 0`.

The `boardRowParticipatesInSelectedTrack` helper stays, but it only determines the visual style (muted rendering), not the rank number.

---

## Feature 2: Admin Quota Reset

### Background

Quota is enforced by counting submissions with `quota_state IN ('pending', 'charged')` within today's time window. There is no stored "remaining quota" field — the reset is achieved by changing the state of today's submissions so they no longer count.

### New `quota_state` Value: `admin_reset`

Semantically identical to `free` (excluded from quota counting) but distinguishable in logs and audits.

### DB Migration

New file: `db/migrations/0007_quota_admin_reset.up.sql`

```sql
ALTER TABLE submissions
    DROP CONSTRAINT IF EXISTS submissions_quota_state_check;

ALTER TABLE submissions
    ADD CONSTRAINT submissions_quota_state_check
    CHECK (quota_state IN ('pending', 'charged', 'free', 'admin_reset'));
```

Down migration (`0007_quota_admin_reset.down.sql`):

```sql
-- Re-label any admin_reset rows as free before dropping the value
UPDATE submissions SET quota_state = 'free' WHERE quota_state = 'admin_reset';

ALTER TABLE submissions
    DROP CONSTRAINT IF EXISTS submissions_quota_state_check;

ALTER TABLE submissions
    ADD CONSTRAINT submissions_quota_state_check
    CHECK (quota_state IN ('pending', 'charged', 'free'));
```

### New SQL Query

**File:** `db/queries/submissions.sql`

```sql
-- name: AdminResetLabQuotaToday :execrows
UPDATE submissions
SET quota_state = 'admin_reset'
WHERE lab_id = $1
  AND created_at >= $2
  AND quota_state IN ('pending', 'charged');
```

Returns the number of rows affected (useful for the response body).

### Admin Service

**File:** `apps/api/internal/service/admin/service.go`

Add to `Repository` interface:

```go
AdminResetLabQuotaToday(ctx context.Context, labID string, windowStart time.Time) (int64, error)
```

Add `quotaLocation` to `Service` (reuse `submissionsvc.DefaultQuotaLocation()`):

```go
type Service struct {
    repo          Repository
    now           func() time.Time
    quotaLocation *time.Location
}
```

Add method:

```go
type ResetLabQuotaResult struct {
    RowsAffected int64 `json:"rows_affected"`
}

func (s *Service) ResetLabQuota(ctx context.Context, labID string) (ResetLabQuotaResult, error) {
    windowStart, _ := submissionsvc.QuotaWindowForTime(s.nowUTC(), s.quotaLocationOrDefault())
    n, err := s.repo.AdminResetLabQuotaToday(ctx, labID, windowStart)
    if err != nil {
        return ResetLabQuotaResult{}, err
    }
    return ResetLabQuotaResult{RowsAffected: n}, nil
}
```

**File:** `apps/api/internal/service/admin/repo.go`

Add to `repo` struct:

```go
func (r *repo) AdminResetLabQuotaToday(ctx context.Context, labID string, windowStart time.Time) (int64, error) {
    return r.store.AdminResetLabQuotaToday(ctx, sqlc.AdminResetLabQuotaTodayParams{
        LabID:       labID,
        CreatedAt:   pgtype.Timestamptz{Time: windowStart, Valid: true},
    })
}
```

### HTTP Layer

**File:** `apps/api/internal/http/admin_handler.go`

Add to `AdminService` interface:

```go
ResetLabQuota(ctx context.Context, labID string) (adminsvc.ResetLabQuotaResult, error)
```

Add handler:

```go
func (h *AdminHandler) ResetLabQuota(w http.ResponseWriter, r *http.Request) {
    labID := r.PathValue("labID")
    result, err := h.Service.ResetLabQuota(r.Context(), labID)
    if err != nil {
        h.writeError(w, r, err)
        return
    }
    middleware.WriteJSON(w, http.StatusOK, result)
}
```

**File:** `apps/api/internal/http/router.go`

Add route alongside existing admin routes:

```go
mux.Handle("POST "+apiPrefix+"/admin/labs/{labID}/quota/reset",
    adminGuard(http.HandlerFunc(adminHandler.ResetLabQuota)))
```

### Web UI

**File:** `apps/web/src/views/AdminLabsView.vue`

Add a "Reset Quota" button to each lab row's actions area, next to the existing Edit/Queue buttons. The button:

1. Opens a browser `confirm()` dialog ("Reset daily quota for all users in this lab?")
2. On confirmation, POSTs to `/api/admin/labs/{labID}/quota/reset`
3. Shows a brief success message (e.g., `{rows_affected} submissions reset`) or an error toast on failure

The fetch call uses the existing admin token auth pattern already used by other admin API calls in the view.

---

## Out of Scope

- CLI admin command for quota reset (not requested)
- Per-user quota reset (only lab-wide reset is needed)
- Quota reset history/audit log

---

## Files Changed Summary

| File | Change |
|---|---|
| `packages/go/manifest/manifest.go` | Add `RankAll *bool` to `BoardSection`, `IsRankAll()` helper, normalize default |
| `apps/api/internal/service/leaderboard/service.go` | Add `RankAll bool` to `Board` struct; fix rank loop to use `IsRankAll()` |
| `apps/web/src/components/board/types.ts` | Add `rank_all` to `LeaderboardBoard` |
| `apps/web/src/components/board/LeaderboardTable.vue` | Add `rankAll` prop; use `row.rank` directly, drop `nextRank` counter |
| `apps/web/src/views/LeaderboardView.vue` | Pass `:rank-all="board.rank_all"` to `<LeaderboardTable>` |
| `apps/cli/internal/commands/lab_commands.go` | Use `row.Rank` from API; `boardRowParticipatesInSelectedTrack` drives style only |
| `db/migrations/0007_quota_admin_reset.up.sql` | Add `admin_reset` to CHECK constraint |
| `db/migrations/0007_quota_admin_reset.down.sql` | Revert constraint, re-label rows |
| `db/queries/submissions.sql` | Add `AdminResetLabQuotaToday` query |
| `apps/api/internal/service/admin/service.go` | Add `ResetLabQuota`, `ResetLabQuotaResult`, `quotaLocation` |
| `apps/api/internal/service/admin/repo.go` | Implement `AdminResetLabQuotaToday` on `repo` and `Repository` interface |
| `apps/api/internal/http/admin_handler.go` | Add `ResetLabQuota` to interface and handler |
| `apps/api/internal/http/router.go` | Register `POST /admin/labs/{labID}/quota/reset` |
| `apps/web/src/views/AdminLabsView.vue` | Add "Reset Quota" button with confirm dialog |
