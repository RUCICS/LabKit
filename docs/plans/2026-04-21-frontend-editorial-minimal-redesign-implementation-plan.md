# Frontend Editorial Minimal Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign LabKit’s Home/Profile/Admin pages into a cohesive Editorial Minimal console with consistent hierarchy, row-first scanning, and improved interaction states—without breaking existing functionality.

**Architecture:** Keep the existing Vue 3 + Vite setup and dark-only token system. Introduce a small set of shared layout primitives and row/list components, then refactor each target view (Home/Profile/Admin Labs/Admin Queue) to use them. Validate with existing Vitest suites and add/adjust tests only where behavior/markup changes.

**Tech Stack:** Vue 3 (`<script setup>`), Vue Router, scoped CSS + global tokens in `apps/web/src/styles/main.css`, Vitest.

---

### Task 1: Create the isolated workspace baseline

**Files:**
- Modify: none
- Test: none

**Step 1: Ensure dependencies installed**

Run:

```bash
cd apps/web
pnpm install
```

Expected: success.

**Step 2: Run baseline tests**

Run:

```bash
cd apps/web
pnpm test
```

Expected: all tests PASS.

**Step 3: Commit nothing**

No commit for baseline.

---

### Task 2: Bring the approved design doc into this branch

**Files:**
- Create: `docs/plans/2026-04-21-frontend-editorial-minimal-redesign-design.md`

**Step 1: Add the design doc**

Copy the approved content from the main workspace version into this branch.

**Step 2: Verify it’s tracked**

Run:

```bash
git status --porcelain
```

Expected: the design doc is listed as untracked.

**Step 3: Commit**

Run:

```bash
git add docs/plans/2026-04-21-frontend-editorial-minimal-redesign-design.md
git commit -m "docs: add frontend editorial redesign spec"
```

Expected: commit succeeds.

---

### Task 3: Establish shared “console” primitives (layout + sections + rows)

**Files:**
- Modify: `apps/web/src/styles/main.css`
- Create: `apps/web/src/components/chrome/PageTitleBlock.vue`
- Create: `apps/web/src/components/chrome/SectionHeader.vue`
- Create: `apps/web/src/components/chrome/RowLink.vue`
- Create: `apps/web/src/components/chrome/SlimStatusBar.vue`
- Test: `apps/web/src/app.smoke.test.ts` (adjust expectations if needed)

**Step 1: Write failing/adjusted smoke assertions (if needed)**

Update smoke test only if markup changes; keep it minimal.

**Step 2: Add minimal components**

- `PageTitleBlock`: renders H1 + meta slot, standardized spacing/typography
- `SectionHeader`: renders H2 + meta slot
- `RowLink`: provides consistent row affordances (hover/focus/active)
- `SlimStatusBar`: consistent inline success/error messaging

**Step 3: Add minimal global CSS**

- Add row styling utilities (background shift, border/rail)
- Ensure focus-visible is consistent
- Keep tokens unchanged unless strictly needed

**Step 4: Run tests**

Run:

```bash
cd apps/web
pnpm test
```

Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/styles/main.css apps/web/src/components/chrome
git commit -m "feat(ui): add shared console layout primitives"
```

---

### Task 4: Refactor Home (`LabListView`) into a single-column directory list

**Files:**
- Modify: `apps/web/src/views/LabListView.vue`
- (Optional) Create: `apps/web/src/components/labs/LabRow.vue`
- Test: `apps/web/src/views/LabListView.test.ts`

**Step 1: Update/extend the existing test**

Assert:
- list uses row links (click target is the whole row)
- key fields render (name/id/status/close date)

**Step 2: Implement view refactor**

- Replace `.lab-grid` card layout with vertical rows
- Keep the same data fetch and navigation path (`/labs/:id/board`)
- Keep metrics display compact with overflow handling (`+N`)

**Step 3: Run tests**

```bash
cd apps/web
pnpm test -- --run src/views/LabListView.test.ts
pnpm test
```

Expected: PASS.

**Step 4: Commit**

```bash
git add apps/web/src/views/LabListView.vue apps/web/src/views/LabListView.test.ts apps/web/src/components/labs
git commit -m "refactor(home): convert labs grid to directory rows"
```

---

### Task 5: Refactor Profile into a cohesive “personal console”

**Files:**
- Modify: `apps/web/src/views/ProfileView.vue`
- Modify (if necessary): `apps/web/src/views/ProfileView.test.ts`
- (Optional) Create: `apps/web/src/components/profile/DeviceKeyRow.vue`

**Step 1: Update tests to match new hierarchy**

Ensure:
- exactly one page `H1` (“Profile”) is present
- sections (“Identity”, “Devices”, “Activity”) exist as H2
- devices list is row-based (not card-based)

**Step 2: Implement view refactor**

- Introduce `PageTitleBlock`
- Use row list for keys and activity
- Add “Copy key” affordance (minimal, no new deps)
- Keep API calls intact (`/api/profile`, `/api/keys`, PUT `/api/profile`)

**Step 3: Run tests**

```bash
cd apps/web
pnpm test -- --run src/views/ProfileView.test.ts
pnpm test
```

Expected: PASS.

**Step 4: Commit**

```bash
git add apps/web/src/views/ProfileView.vue apps/web/src/views/ProfileView.test.ts apps/web/src/components/profile
git commit -m "refactor(profile): reshape into single console layout"
```

---

### Task 6: Refactor Admin Labs into rows + improved editor ergonomics

**Files:**
- Modify: `apps/web/src/views/AdminLabsView.vue`
- Modify: `apps/web/src/views/AdminLabsView.test.ts`

**Step 1: Update tests**

Assert:
- catalog items render as rows
- editor controls still work (mode switch, required validations remain)

**Step 2: Implement refactor**

- Replace catalog cards with rows
- Standardize headers using shared primitives
- Keep editor logic unchanged; adjust layout only

**Step 3: Run tests**

```bash
cd apps/web
pnpm test -- --run src/views/AdminLabsView.test.ts
pnpm test
```

Expected: PASS.

**Step 4: Commit**

```bash
git add apps/web/src/views/AdminLabsView.vue apps/web/src/views/AdminLabsView.test.ts
git commit -m "refactor(admin): make labs catalog row-first"
```

---

### Task 7: Refactor Admin Queue into row/table scanning + collapsible errors

**Files:**
- Modify: `apps/web/src/views/AdminQueueView.vue`
- Modify: `apps/web/src/views/AdminQueueView.test.ts`

**Step 1: Update tests**

Assert:
- summary stats still show
- jobs render in a dense list/table structure
- error details are hidden by default and can be expanded

**Step 2: Implement refactor**

- Convert job cards into rows/table-like grid
- Move `last_error` into a `<details>` (native, no deps) or controlled expand UI

**Step 3: Run tests**

```bash
cd apps/web
pnpm test -- --run src/views/AdminQueueView.test.ts
pnpm test
```

Expected: PASS.

**Step 4: Commit**

```bash
git add apps/web/src/views/AdminQueueView.vue apps/web/src/views/AdminQueueView.test.ts
git commit -m "refactor(admin): make queue view scannable with collapsible errors"
```

---

### Task 8: Visual QA (manual) + lint check + full test pass

**Files:**
- Modify: as-needed

**Step 1: Run dev server**

```bash
cd apps/web
pnpm dev
```

Manually verify:
- Home: directory list rows are clearly clickable
- Profile: only one H1; rows scan well; long keys do not explode layout
- Admin: labs list scans; editor is usable; queue scans; errors expand/collapse
- Responsive: 375px / 768px / 1024px

**Step 2: Run tests**

```bash
cd apps/web
pnpm test
```

Expected: PASS.

**Step 3: Commit (only if fixes were needed)**

```bash
git add -A
git commit -m "fix(ui): polish console layout and interactions"
```

