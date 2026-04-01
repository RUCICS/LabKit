# Device Auth Web Surface Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move the device authorization UI to `/auth/device`, keep `/api/device/verify` as a redirect/callback-only endpoint, and restyle the auth surfaces to match the LabKit web design language.

**Architecture:** The web app owns all user-visible auth pages. The API remains responsible for provider redirects, OAuth state validation, callback completion, and browser-session issuance. This keeps OAuth protocol details out of the SPA while eliminating API-rendered HTML pages.

**Tech Stack:** Go HTTP handlers and router tests, Vue 3 + vue-router views, Vitest, existing LabKit design tokens in `apps/web/src/styles/main.css`.

---

### Task 1: Lock API route behavior with tests

**Files:**
- Modify: `apps/api/internal/http/auth_router_test.go`
- Modify: `apps/api/internal/http/device_verify_handler.go`

**Step 1: Write the failing tests**

- Change the device authorize response expectation from `/api/device/verify` to `/auth/device`
- Add a test that `GET /api/device/verify` without query params no longer returns an HTML form

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/http -run 'TestRouterWiresDevice' -count=1`

Expected: FAIL because the current verification URL and handler behavior still point to the API-rendered page.

**Step 3: Write minimal implementation**

- Return `/auth/device` from device authorization creation
- Remove inline HTML rendering from `DeviceVerifyHandler`
- Keep `user_code` redirect and `code/state` callback branches intact

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/http -run 'TestRouterWiresDevice' -count=1`

Expected: PASS

**Step 5: Commit**

```bash
git add apps/api/internal/http/auth_router_test.go apps/api/internal/http/device_verify_handler.go
git commit -m "refactor(auth): move device auth UI off api route"
```

### Task 2: Add the `/auth/device` route and view tests

**Files:**
- Modify: `apps/web/src/router.ts`
- Modify: `apps/web/src/router.test.ts`
- Create: `apps/web/src/views/DeviceAuthView.vue`
- Create: `apps/web/src/views/DeviceAuthView.test.ts`

**Step 1: Write the failing tests**

- Router test for `/auth/device`
- View test that:
  - renders the page shell
  - reads `user_code` from the query string if present
  - redirects the browser to `/api/device/verify?user_code=...` on submit
  - shows validation feedback for empty input

**Step 2: Run test to verify it fails**

Run: `npm --prefix apps/web test -- DeviceAuthView router`

Expected: FAIL because the route and view do not exist yet.

**Step 3: Write minimal implementation**

- Register `/auth/device` in the router
- Implement a simple Vue view with query-backed form state and submit redirect

**Step 4: Run test to verify it passes**

Run: `npm --prefix apps/web test -- DeviceAuthView router`

Expected: PASS

**Step 5: Commit**

```bash
git add apps/web/src/router.ts apps/web/src/router.test.ts apps/web/src/views/DeviceAuthView.vue apps/web/src/views/DeviceAuthView.test.ts
git commit -m "feat(web): add device auth route"
```

### Task 3: Polish `/auth/device` into the real product surface

**Files:**
- Modify: `apps/web/src/views/DeviceAuthView.vue`
- Modify: `apps/web/src/styles/main.css`

**Step 1: Write the failing test**

- Add assertions for the final structure:
  - branded auth surface
  - segmented code input
  - helper text / status region
  - disabled submit state while redirecting

**Step 2: Run test to verify it fails**

Run: `npm --prefix apps/web test -- DeviceAuthView`

Expected: FAIL because the current minimal view lacks the final structure.

**Step 3: Write minimal implementation**

- Build the final layout using existing design tokens
- Adapt the reference HTML into Vue-friendly markup
- Keep the interaction TTY-simple: no fake async state, just crisp client-side validation and redirect transition

**Step 4: Run test to verify it passes**

Run: `npm --prefix apps/web test -- DeviceAuthView`

Expected: PASS

**Step 5: Commit**

```bash
git add apps/web/src/views/DeviceAuthView.vue apps/web/src/styles/main.css
git commit -m "feat(web): restyle device auth surface"
```

### Task 4: Bring `/auth/confirm` into the same auth surface system

**Files:**
- Modify: `apps/web/src/views/AuthConfirmView.vue`
- Create or Modify tests: `apps/web/src/views/AuthConfirmView.test.ts`

**Step 1: Write the failing test**

- Assert the confirm page uses the shared auth layout and renders both success and missing-parameter states clearly

**Step 2: Run test to verify it fails**

Run: `npm --prefix apps/web test -- AuthConfirmView`

Expected: FAIL because the current page is still the older minimal panel.

**Step 3: Write minimal implementation**

- Rework the view so it visually matches the device auth page
- Preserve the current query-driven semantics

**Step 4: Run test to verify it passes**

Run: `npm --prefix apps/web test -- AuthConfirmView`

Expected: PASS

**Step 5: Commit**

```bash
git add apps/web/src/views/AuthConfirmView.vue apps/web/src/views/AuthConfirmView.test.ts
git commit -m "feat(web): align auth confirmation surface"
```

### Task 5: Verify end-to-end behavior and docs

**Files:**
- Modify if needed: `README.md`
- Modify if needed: `docs/reference/local-auth.md`

**Step 1: Run focused verification**

Run:

```bash
go test ./apps/api/internal/http -count=1
npm --prefix apps/web test
npm --prefix apps/web run build
```

Expected: all green

**Step 2: Update docs if route text changed**

- Replace any user-facing references that still tell people to open `/api/device/verify`

**Step 3: Re-run verification**

Run:

```bash
go test ./apps/api/... -count=1
npm --prefix apps/web test
npm --prefix apps/web run build
```

Expected: all green

**Step 4: Commit**

```bash
git add README.md docs/reference/local-auth.md apps/api apps/web
git commit -m "docs(auth): document device auth web route"
```
