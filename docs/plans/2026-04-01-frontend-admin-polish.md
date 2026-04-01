# Frontend Admin Polish Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the Vue admin workflow and restyle the UI so the local product feels like a real release rather than a prototype.

**Architecture:** Keep the current API surface intact. Add missing admin interactions in the Vue app, centralize minimal admin token request helpers, then restyle the global shell and page layouts around a lighter product UI.

**Tech Stack:** Vue 3, Vue Router, TypeScript, Vitest, Vite, existing Go API endpoints

---

### Task 1: Cover missing admin workflow behavior with tests

**Files:**
- Modify: `apps/web/src/views/AdminLabsView.test.ts`
- Create or Modify: `apps/web/src/views/AdminQueueView.test.ts`

1. Add a failing test for creating a lab from a pasted manifest.
2. Add a failing test for updating an existing lab manifest.
3. Add a failing test for triggering re-evaluation from the admin queue view.
4. Add a failing test for grade export link/token behavior if needed.
5. Run the focused tests and confirm they fail for the intended missing behavior.

### Task 2: Implement admin workflow completion

**Files:**
- Modify: `apps/web/src/views/AdminLabsView.vue`
- Modify: `apps/web/src/views/AdminQueueView.vue`
- Create: `apps/web/src/lib/admin.ts`

1. Add a shared helper for reading the admin token and building authorized admin requests.
2. Implement a manifest editor flow in the admin labs view for register and update.
3. Implement success/error state handling that fits a production control surface.
4. Add queue-page actions for re-evaluate and grade export.
5. Re-run the focused admin tests and make them pass.

### Task 3: Redesign the product shell and student-facing views

**Files:**
- Modify: `apps/web/src/main.ts`
- Modify: `apps/web/src/styles/main.css`
- Modify: `apps/web/src/views/LabListView.vue`
- Modify: `apps/web/src/views/LeaderboardView.vue`
- Modify: `apps/web/src/views/ProfileView.vue`
- Modify: `apps/web/src/views/AuthConfirmView.vue`
- Modify: `apps/web/src/views/AdminLabsView.vue`
- Modify: `apps/web/src/views/AdminQueueView.vue`

1. Replace the current dark glass shell with a light product layout and tighter navigation.
2. Remove prototype-style descriptive copy and replace it with concise product labels.
3. Restructure student pages around lists, status, and leaderboard interaction.
4. Restructure admin pages into a cleaner operations console.
5. Keep existing tests green; add or update text expectations only where behavior meaningfully changed.

### Task 4: Document real local usage

**Files:**
- Modify: `README.md`
- Modify: `deploy/.env.example`

1. Add a concrete CLI quickstart using the current command surface.
2. Clarify the local auth limitation and dev-mode shortcut expectations.
3. Clarify which web surfaces are available locally and which routes to open.

### Task 5: Verify end-to-end

**Files:**
- No code changes required unless verification fails

1. Run `cd apps/web && npm test`
2. Run `cd apps/web && npm run build`
3. Run `bash scripts/deploy_smoke_test.sh`
4. Rebuild the local `web` stack with compose.
5. Verify `/`, `/admin`, `/admin/labs`, and `/healthz` through the configured local port.
