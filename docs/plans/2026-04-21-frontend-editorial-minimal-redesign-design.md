---
title: "LabKit Frontend Editorial Minimal Redesign"
date: 2026-04-21
status: approved
owner: starrydream + agent
scope:
  - home (labs)
  - profile
  - admin (labs, queue)
non_goals:
  - rebrand away from dark-only tokens
  - framework/styling migration
  - adding new product features beyond UX/UI restructure
---

## 1. Background

Current frontend already has a strong dark-only token base (OLED-ish) and a clear “mono + instrument” vibe, but the pages still feel “weird” because:

- Layout primitives and hierarchy aren’t consistent across pages.
- Too many “card-like panels” compete at the same elevation level, creating a stitched-together feel.
- Click affordances and scanning rhythm vary between Home/Profile/Admin.

This plan redesigns the frontend into an **Editorial Minimal console**: calm, precise, content-led, with consistent typography/spacing and predictable interactive states.

## 2. Goals

- Make Home/Profile/Admin feel like one product (same shell, same hierarchy rules).
- Improve information architecture: clear primary line, secondary meta, and actions.
- Improve scanning speed: prefer rows/tables over card walls where appropriate.
- Improve interaction quality: consistent hover/active/focus, fewer scattered micro-buttons.
- Keep changes incremental and reviewable; do not break functionality.

## 3. Non-goals

- Switching to light mode.
- Introducing marketing-landing patterns (hero/bento/scroll-journey).
- Replacing existing fonts/tokens unless needed for consistency.
- Adding new data endpoints or admin features (beyond UI refinements).

## 4. Global Design Direction

### 4.1 Style: Editorial Minimal (Console)

- **Dark-only**, low-noise surfaces, high readability.
- Use typography + spacing to create hierarchy; reduce reliance on heavy borders/shadows.
- “Data is the interface” remains true, especially for admin/queue and leaderboard.

### 4.2 Global Layout Rules (Hard constraints)

- **Single App Shell + Single Page Shell** across all pages.
- **Max 2 surface layers**:
  1) `PageShell` (page background container)
  2) Optional `Section` (only when necessary)
- Avoid nesting “cards inside cards” with the same border/background treatment.

### 4.3 Typography & Hierarchy (Global)

- Exactly one page-level `H1`.
- Section headings use `H2` plus small meta; avoid repeating “page-sized” headings per section.
- Labels/meta: mono, uppercase, tracking; keep consistent size and color across pages.
- Body copy: sans; keep line width constrained (around existing `--page-shell__lede` behavior).

### 4.4 Interaction Baseline (Global)

- All clickable rows/cards must have:
  - Hover: subtle background shift + border/rail emphasis (no layout shift)
  - Active: tiny press feedback
  - Focus: visible `--focus-ring`
- Prefer **entire row clickable**; keep actions as secondary affordances.

## 5. Page Specs

### 5.1 Home (`/`) — Labs Index (Option A confirmed: single-column directory list)

**Intent:** A scannable catalog. Click a lab → go to board. No “selected preview” step.

#### Information Architecture

- Title block:
  - `H1`: “Labs”
  - Meta: count + optional refresh timestamp
- Content:
  - A single vertical list (rows) replacing the card grid.

#### Lab Row Layout (Row-as-Link)

Each row is a `RouterLink` wrapping the row:

- Left: `lab.name` (primary), below: `lab.id` + context tags (`course · semester`) (secondary)
- Middle (optional, de-emphasized): `ranked by <metric>`
- Right: `StatusBadge` + `CLOSES 06/01` (mono, aligned)
- Metrics: compact chips/dots line, show up to 3 + `+N` overflow indicator.

#### States

- Loading: skeleton rows (6–8) instead of a single sentence.
- Error: slim error bar under title with retry.
- Empty: composed empty state (short, direct copy).

#### Acceptance Criteria

- Home reads as a catalog; scanning works at a glance.
- Clicking a lab row goes directly to board.
- Hover/focus states clearly show clickability without shifting layout.

### 5.2 Profile (`/profile`) — Personal Console

**Intent:** One coherent console page, not three competing panels.

#### Information Architecture

- Title block:
  - `H1`: “Profile”
  - Meta: signed-in identity (`student_id`, `user_id` where available)
- Sections:
  1) Identity (nickname update)
  2) Devices (keys)
  3) Activity (recent submissions)

#### Identity

- Single-line settings row: Nickname input + Save.
- Feedback is a slim status line below (success/error).

#### Devices

- Replace “card list” with **rows**.
- Each row shows `device_name`, `Key <id>`, `Created <date>`.
- `public_key` is not always fully expanded:
  - Provide “Copy” as the primary action.
  - Optionally expand per-row for full key display.

#### Activity

- Rows with `lab_id`, `status`, `Submitted <time>`.
- Board/History actions are compact and consistent with global button/link styling.

#### Acceptance Criteria

- Exactly one `H1` on the page.
- Devices and Activity scan like a console (rows), not a card wall.
- Copy key is easy; long key text doesn’t dominate the layout.

### 5.3 Admin Labs (`/admin/labs`) — Catalog + Editor

**Intent:** Clear “list + editor” relationship. Reduce card feel; improve editing ergonomics.

#### Layout

- Keep 2-column desktop layout; stack on smaller breakpoints.
- Left: lab catalog as rows (scannable, consistent).
- Right: editor panel with stable header and strong textarea focus states.

#### Catalog

- Row shows: name/id/phase/metrics count.
- Actions: `Edit` (primary) + `Queue` (secondary) aligned consistently.
- “New” action stays in header as the clear entry.

#### Editor

- Header: title + short explanation + inline status.
- Controls: compact (Action/Lab ID) with consistent labels.
- Textarea: mono, strong focus ring, tabular nums.

#### Acceptance Criteria

- Catalog reading speed improves (rows).
- Editor feels like a dedicated workspace; status doesn’t jump around.

### 5.4 Admin Queue (`/admin/labs/:labID/queue`) — Queue Monitor

**Intent:** A monitor page. Rows/tables beat cards for scanning.

#### Structure

- Title block: “Queue” + lab id + meta.
- Actions row: reevaluate (primary) + export (secondary).
- Jobs list: shift from card stack to **table-like rows** (or dense row list).

#### Error Display

- `last_error` is collapsible per job (default collapsed) to avoid huge vertical expansion.

#### Acceptance Criteria

- Users can scan status quickly: totals, running/queued, recent updates.
- Error details are accessible without dominating the page.

## 6. Implementation Notes (Constraints)

- Keep existing token system in `apps/web/src/styles/main.css`.
- Prefer refactors that reduce duplication of per-view scoped CSS.
- Avoid adding new dependencies unless necessary.
- Ensure keyboard navigation remains strong (focus-visible everywhere interactive).

## 7. Open Questions

- None. Direction confirmed: Editorial Minimal + Home Option A (single-column directory list).

