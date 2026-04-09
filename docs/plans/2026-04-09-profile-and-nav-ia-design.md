# Profile And Navigation IA Design

**Date:** 2026-04-09

## Goal

Align the web app information architecture with the actual product shape:

- Remove fake top-level navigation.
- Move personal capabilities into a dedicated `Profile` surface.
- Make nickname a global user attribute.
- Keep track selection lab-scoped.

## Problem

The current app shell and personal flows are structurally inconsistent:

- The top bar mixes real destinations with context-dependent links.
- `History` appears only for some routes, which makes the primary nav unstable.
- Nickname editing lives inside the leaderboard, which conflates user identity with a specific lab context.
- The backend currently stores nickname in `lab_profiles`, so the product behaves like nickname is per-lab even though the expected user mental model is global identity.
- Devices, recent activity, and profile settings are split across unrelated surfaces instead of living under one personal entry point.

The result is functional but immature. The product feels like a set of pages rather than a coherent application.

## Product Principles

1. The app should have one honest primary space: labs.
2. Utility surfaces are not top-level content areas.
3. Personal identity is global unless there is a strong reason to scope it.
4. Lab-specific configuration should stay attached to the lab context.
5. Navigation should stay stable across routes.

## Chosen Approach

Use a minimal global shell:

- Left side: `LabKit` brand, linked to the lab catalog.
- Right side: utility entries `Admin` and `Profile`.
- No persistent top-level nav beyond the brand entry.

Within this structure:

- `Profile` becomes the single personal hub.
- `Admin` remains a separate tool entry, not a primary content section.
- Lab-specific pages expose their own contextual actions such as board and history.

This is intentionally simpler than the current shell. The app does not yet have enough parallel product areas to justify full primary navigation.

## Rejected Alternatives

### Keep `Labs / Profile / Admin` as top-level nav

Rejected because `Profile` and `Admin` are utility destinations, not peer product spaces. This creates fake IA weight.

### Keep `Devices` as top-level nav and add `Profile`

Rejected because it still splits personal features across multiple global entry points.

### Leave nickname editing on the leaderboard

Rejected because the leaderboard is a lab-specific performance view, not a global identity surface.

## Information Architecture

### Global shell

- Brand click returns to `/`.
- `Admin` appears in the header only when the session is authorized for admin access.
- `Profile` appears in the header when a browser session exists.
- The current lab phase badge appears only when the route belongs to a specific lab context.

### Top-level destinations

- `/` -> lab catalog
- `/profile` -> personal hub
- `/admin` -> admin console

### Lab-context destinations

These are not part of global navigation:

- `/labs/:labID` -> leaderboard or lab landing
- `/labs/:labID/history` -> current user's submission history for that lab
- future lab-local routes such as submit or docs should follow the same pattern

## Profile Surface

`Profile` owns user-level identity and personal operational data.

### Sections

#### Identity

- Global nickname
- Future user-level metadata if needed, such as email, student id, or role

#### Devices

- Registered keys
- Device name
- Public key
- Creation time
- Future revoke/delete actions

#### Activity

- Recent submissions across labs
- Recent labs the user interacted with
- Direct links back to lab board/history

### Non-goals

- `Profile` does not own lab-specific track selection.
- `Profile` does not replace lab-local history views.

## Data Model Boundaries

### Global user profile

Move nickname to a user-level profile model:

- one nickname per user
- leaderboard rows read nickname from user profile
- profile editing updates the global value

### Lab profile

Keep lab-scoped profile data only for settings that are inherently lab-specific:

- selected track
- future lab-local preferences if needed

This turns the model into:

- `user profile`: identity
- `lab profile`: lab participation configuration

## API Changes

### New or reshaped user-level profile API

- `GET /api/profile`
- `PUT /api/profile`

Expected responsibilities:

- return global nickname
- return devices/keys summary or enough data for the page
- optionally return recent activity summary if that is efficient to serve directly

### Existing lab-scoped API

- keep `PUT /api/labs/{labID}/track`
- stop using `PUT /api/labs/{labID}/nickname` from the web UI

### Compatibility

The old lab-scoped nickname endpoint can remain temporarily for compatibility, but it becomes deprecated immediately after the new profile flow ships.

## Frontend Changes

### App shell

- remove the current primary nav links
- render brand-only left side
- render utility actions on the right
- stop conditionally injecting `History` into the global header

### Leaderboard

- remove inline nickname editing
- keep lab-relevant user state such as:
  - current track
  - quota
  - my row highlight
  - link to lab history
- if an identity affordance is needed, use a link to `Profile`, not an embedded editor

### Devices page

- fold current `/devices` content into `/profile`
- keep `/devices` as a redirect for compatibility during migration

### History

- keep lab history as a lab-context page
- surface access via the relevant lab views and from recent activity in `Profile`

## Migration Strategy

### Phase 1: front-end IA correction

- ship the new shell and `Profile` page
- merge devices into profile
- remove global `History` nav
- remove leaderboard nickname editor

This step improves product coherence before the full backend data migration is complete.

### Phase 2: backend semantic split

- add user-level profile storage and API
- update leaderboard and personal surfaces to read nickname from the new source
- keep track in the lab profile path

### Phase 3: data backfill

Backfill global nickname from existing lab-scoped data.

Recommended default rule:

- for each user, take the most recent non-empty lab nickname
- if none exists, fall back to the system default or empty value

### Phase 4: compatibility cleanup

- redirect or retire `/devices`
- remove unused frontend calls to lab nickname APIs
- remove deprecated server code after confirming no remaining callers

## Risks

### Identity migration ambiguity

Some users may have used different nicknames across labs. The migration rule must be explicit and one-way.

### Partial rollout mismatch

If the frontend moves before the backend profile API exists, temporary composition logic will be needed to keep the page useful.

### Admin visibility

The shell needs a reliable way to know whether `Admin` should render. If the current session API does not expose this cleanly, a lightweight capability check will be required.

## Success Criteria

- The top bar no longer changes shape based on whether a lab history route is available.
- The leaderboard no longer edits global identity data.
- Users can manage nickname and keys from a single `Profile` page.
- Track selection remains available without implying that it is a global identity field.
- The navigation model matches the actual product shape and feels stable.
