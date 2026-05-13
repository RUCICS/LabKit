# Admin Panel Redesign + Reeval Quota Fix

**Date:** 2026-05-13  
**Status:** Approved

## Overview

Two related improvements:

1. **Backend fix**: Admin-triggered reevaluation must not deduct users' daily submission quota.
2. **Frontend redesign**: Replace the current admin panel (raw TOML textarea, URL-only auth) with a proper UI — sidebar navigation, form-based manifest editor, and a login page.

---

## Problem 1: Reeval Deducts User Quota

### Root Cause

`admin/service.go Reevaluate()` creates new submissions via `tx.CreateSubmission()`. The SQL query (`CreateSubmission`) does not include `quota_state` in its `INSERT`, so new rows default to `'pending'`. The quota counter (`CountSubmissionQuotaUsage`) counts rows where `quota_state IN ('pending', 'charged')`, so admin-created reeval submissions are indistinguishable from user-initiated ones and count against each user's daily limit.

### Fix

Add a new SQL query `CreateFreeSubmission` that inserts with `quota_state = 'free'`. Use this query in the admin `Reevaluate` path instead of `CreateSubmission`. Normal user submissions continue using `CreateSubmission` (defaulting to `'pending'`).

**Changes required:**
- `db/queries/submissions.sql` — add `CreateFreeSubmission` query
- `packages/go/db/sqlc/submissions.sql.go` — regenerate via `sqlc generate`
- `apps/api/internal/service/admin/service.go` — update `Tx` interface and `Reevaluate()` to use `CreateFreeSubmission`
- `apps/api/internal/service/admin/repo.go` (or inline in service) — implement `CreateFreeSubmission` in the admin tx

No database migration needed — `quota_state` column already exists with the `'free'` value as a valid option.

---

## Problem 2: Admin Panel UX

### Current Pain Points

- Manifest editor is a raw TOML textarea; clicking "Edit" leaves it blank — admin must retype the full manifest from scratch.
- Authentication only via `?token=xxx` URL query parameter; no UI to enter or update the token without knowing the URL trick.
- Queue view has no back navigation to the Labs list.
- Queue appears as a top-level navigation item alongside Labs (wrong hierarchy — Queue is per-lab).

### Design Decisions (from brainstorming)

| Decision | Choice |
|---|---|
| Overall layout | Left sidebar (extensible top-level entries) |
| Edit flow | Right-side drawer, slides in on "Edit" |
| Form style | Flat grouped fields (Lab / Schedule / Quota / Files / Metrics / Board) |
| Authentication | Full-screen login page + "remember on this device" toggle |
| Queue placement | Row-level button on each lab, NOT a sidebar nav item |

---

## Architecture

### Frontend (Vue 3 SPA — `apps/web`)

#### New / changed views

| File | Change |
|---|---|
| `src/views/AdminLoginView.vue` | **New** — full-screen token entry form |
| `src/views/AdminLabsView.vue` | **Rewrite** — sidebar layout, lab list with Edit/Queue/Grades buttons |
| `src/views/AdminQueueView.vue` | **Update** — add breadcrumb nav (← Labs), remove from sidebar |
| `src/components/admin/AdminShell.vue` | **New** — shared sidebar wrapper (Labs nav item + token status slot) |
| `src/components/admin/LabEditDrawer.vue` | **New** — right-side drawer with flat grouped form |
| `src/lib/admin.ts` | **Update** — add `rememberToken(token)` (localStorage) vs `sessionToken(token)` (sessionStorage) |
| `src/router.ts` | **Update** — add `/admin/login` route; add navigation guard that redirects to login when no token |

#### Sidebar navigation

The sidebar renders top-level admin sections. Initially only "Labs". Future sections (e.g. Settings, Users) can be added here. Queue and Grades are **not** sidebar items.

```
ADMIN
  🧪 Labs          ← only item for now
  [future items]

  ─────────────
  TOKEN
  ● active  ✎     ← inline token edit on click
```

#### Authentication flow

1. User visits any `/admin/*` route.
2. Router guard checks `readAdminToken()`. If empty → redirect to `/admin/login`.
3. Login page: password-type input for token + "Remember on this device" toggle.
   - Toggle **off** (default): `sessionStorage` — clears when tab closes.
   - Toggle **on**: `localStorage` — persists across browser restarts. Warning shown.
4. On submit: store token, redirect to `/admin/labs`.
5. If any API call returns 401: clear stored token, redirect to `/admin/login`.
6. Token status in sidebar bottom: shows "● active". Click `✎` opens inline input to replace token (without going back to login page).
7. URL `?token=xxx` parameter continues to work as a shortcut (existing behavior preserved).

#### LabEditDrawer form sections

The drawer fetches full lab manifest via a new admin API endpoint on open, then renders:

| Section | Fields |
|---|---|
| **LAB** | Name (editable), ID (read-only), Eval Image, Timeout |
| **SCHEDULE** | Visible (datetime), Open (datetime), Close (datetime, optional) |
| **QUOTA & FILES** | Daily limit (number), Max size (text), Submit files (tag pills, add/remove) |
| **METRICS** | List of metrics, each: ID (read-only), Name, Sort (asc/desc select), Unit |
| **BOARD** | Rank by (select from metric IDs), Pick track (toggle) |

On "Save": submits JSON to `PUT /api/admin/labs/{labID}`. Backend accepts both JSON and TOML (see below).

#### Queue view changes

- Add breadcrumb: `← Labs / {labID} / Queue`
- `← Labs` link navigates to `/admin/labs`
- Remove Queue from sidebar entirely

---

### Backend (`apps/api`)

#### New SQL query

```sql
-- name: CreateFreeSubmission :one
INSERT INTO submissions (
    user_id, lab_id, key_id, artifact_key, content_hash, status,
    verdict, message, detail, image_digest, started_at, finished_at,
    quota_state
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'free')
RETURNING *;
```

#### New admin API endpoint — GET manifest

`GET /api/admin/labs/{labID}` (admin-guarded) returns the full manifest including `eval.image`:

```json
{
  "id": "sorting",
  "name": "Sorting Lab",
  "manifest": { ...full Manifest fields including eval.image... }
}
```

This is separate from the public `GET /api/labs/{labID}` which redacts `eval.image`.

#### Accept JSON manifests in RegisterLab / UpdateLab

`manifest.Parse()` currently only accepts TOML. Extend it to also accept JSON:

```go
func Parse(data []byte) (*Manifest, error) {
    // try JSON first (starts with '{')
    // fall back to TOML
}
```

This allows the drawer form to submit the manifest as JSON (serialized from the form state) while keeping backward compatibility with existing TOML-based tooling (CLI, scripts).

#### Admin service — Reevaluate

Replace `tx.CreateSubmission` with `tx.CreateFreeSubmission` in the reeval loop. The `Tx` interface in `admin/service.go` gains `CreateFreeSubmission`.

---

## Data Flow: Edit Lab

```
User clicks Edit on lab row
  → LabEditDrawer opens
  → GET /api/admin/labs/{labID}   (new endpoint, returns full manifest)
  → form pre-populated with all fields
User edits fields, clicks Save
  → PUT /api/admin/labs/{labID}   (body: JSON manifest)
  → backend: manifest.Parse() accepts JSON → validates → stores
  → drawer closes, labs list refreshes
```

## Data Flow: Reeval

```
Admin clicks Reevaluate
  → POST /api/admin/labs/{labID}/reeval
  → for each leaderboard entry:
      CreateFreeSubmission(quota_state='free')   ← NEW
      CreateEvaluationJob(submissionID)
  → returns { jobs_created: N }
User's daily quota counter: unchanged (free submissions excluded)
```

---

## Out of Scope

- Metrics add/remove in the form (metrics are structurally immutable post-creation per `sameStructure()` check — form shows existing metrics as read-only ID with editable Name/Unit).
- Lab ID change (structurally immutable).
- Pagination of the labs list or queue.
- Dark/light theme toggle.

---

## Error Handling

- Drawer: if GET manifest fails, show error state with retry button (don't open blank).
- Save: show inline error in drawer footer on API error; keep drawer open.
- Login: show error message if 401 returned from authenticate attempt.
- Any admin API returning 401 mid-session: clear token, redirect to login, show "Session expired" message.
