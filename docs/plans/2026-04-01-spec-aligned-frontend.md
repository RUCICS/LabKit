# Spec-Aligned Frontend Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rework the Vue UI to match the design spec's dark, information-dense monitoring aesthetic while preserving current routes and admin functionality.

**Architecture:** Keep the current Vue route structure and API usage. Replace the global shell, tokens, and component styling first, then adapt student/admin pages to the spec's layout, typography, and data hierarchy.

**Tech Stack:** Vue 3, Vue Router, TypeScript, Vitest, Vite, existing REST API

---

### Task 1: Lock current behavior before visual refactor

**Files:**
- Modify: `apps/web/src/app.smoke.test.ts`
- Modify: `apps/web/src/views/*.test.ts` as needed

1. Update or add assertions only where route-visible labels must intentionally change.
2. Keep the admin action tests and leaderboard behavior tests as the safety net.
3. Run the relevant web tests to confirm the suite is green before visual refactor work.

### Task 2: Replace global tokens and shell with the spec system

**Files:**
- Modify: `apps/web/src/styles/main.css`
- Modify: `apps/web/src/main.ts`
- Modify: `apps/web/index.html`

1. Reintroduce the dark-only palette, typography, spacing, border, and accent tokens from the spec.
2. Add the fixed atmosphere layers the spec calls for: grid texture and top glow scaffolding.
3. Replace the current light shell with the spec-aligned top navigation and status treatment.
4. Keep the app responsive at the spec's two breakpoints.

### Task 3: Rebuild leaderboard components around the spec

**Files:**
- Modify: `apps/web/src/views/LeaderboardView.vue`
- Modify: `apps/web/src/components/board/LeaderboardMetricTabs.vue`
- Modify: `apps/web/src/components/board/LeaderboardTable.vue`
- Modify: `apps/web/src/components/board/LeaderboardEmptyState.vue`
- Modify: `apps/web/src/components/board/LeaderboardHiddenState.vue`

1. Add the stat-card-like header summary and stronger leaderboard chrome.
2. Restyle tabs, table, row states, selected metric emphasis, and empty/hidden states to the spec.
3. Preserve all tested metric switching and hidden-state behavior.

### Task 4: Align lab list, auth, profile, and admin pages to the spec

**Files:**
- Modify: `apps/web/src/views/LabListView.vue`
- Modify: `apps/web/src/views/ProfileView.vue`
- Modify: `apps/web/src/views/AuthConfirmView.vue`
- Modify: `apps/web/src/views/AdminLabsView.vue`
- Modify: `apps/web/src/views/AdminQueueView.vue`

1. Convert each page to the spec's darker, denser surface treatment.
2. Tighten copy so the pages feel operational and product-grade.
3. Keep the existing admin registration/update/export/reeval workflows intact.

### Task 5: Verify and document local operation

**Files:**
- Modify: `README.md` if the local auth explanation needs tightening after implementation

1. Run `cd apps/web && npm test`
2. Run `cd apps/web && npm run build`
3. Run `bash scripts/deploy_smoke_test.sh`
4. Rebuild the local stack and verify `/`, `/admin`, `/admin/labs`, and `/healthz`
