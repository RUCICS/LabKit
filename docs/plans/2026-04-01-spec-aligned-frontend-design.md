# Spec-Aligned Frontend Design

## Goal

Rebuild the current Vue frontend to match `docs/reference/labkit-design-spec.md` rather than the lighter product shell that is in the repo now.

## Direction

### Visual language

- Follow the spec exactly: dark-only theme, IBM Plex Sans + JetBrains Mono feel, dense information hierarchy, terminal warmth without terminal cosplay.
- Make data the visual center of the product. Navigation, cards, and chrome should recede behind rankings, metrics, statuses, and submission states.
- Remove the current light SaaS look entirely.

### Student-facing pages

- The leaderboard page becomes the primary expression of the product: strong top nav, stat cards, track tabs, denser table, track indicator, footer metadata.
- The lab list becomes a darker catalog of operational lab cards rather than a generic card grid.
- Auth confirm and profile pages inherit the same system-monitor aesthetic rather than standalone marketing-like layouts.

### Admin-facing pages

- Keep the workflow completion added in this branch, but reskin it into the same dark control-panel language.
- Admin pages should feel like operations tooling, not separate product marketing pages.

## Constraints

- Do not change the API contract for leaderboard/admin flows in this round.
- Preserve the existing Vue routes and current admin functionality.
- Keep the app accessible and maintain the current tested behaviors while changing visual hierarchy and copy where needed.
