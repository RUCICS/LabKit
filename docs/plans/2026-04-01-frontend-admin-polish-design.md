# Frontend Admin Polish Design

## Goal

Turn the current Vue UI from a thin demo shell into a product-facing interface that supports real local operation:

- student-facing pages should feel like a finished product rather than a prototype
- admin pages should complete the existing backend workflow instead of stopping at read-only queue inspection
- local users should be able to understand the CLI workflow quickly from the repository docs

## Product Direction

### Visual language

- Use a light, restrained visual system rather than a dark “engineering demo” aesthetic.
- Remove explanatory marketing copy and replace it with concise product UI labels.
- Increase information density and hierarchy with clearer navigation, quieter surfaces, and stronger table/card structure.

### Student-facing web

- Home should primarily show available labs and direct paths into each board.
- Leaderboard should emphasize rank, metric selection, and freshness rather than long descriptive text.
- Profile and auth confirmation should read like operational product screens, not implementation notes.

### Admin web

- `/admin` should act as a real operations entry point.
- Admin needs to support:
  - register a new lab by posting a manifest
  - update an existing lab manifest
  - open queue status
  - export grades
  - trigger re-evaluation
- Admin token handling should stay browser-session scoped and continue to avoid leaking the token in links.

## Implementation Scope

- Keep the existing API contracts and worker/runtime behavior.
- Focus changes in the Vue app, shared styles, deploy-time SPA routing, and README usage guidance.
- Prefer small shared helpers for admin fetch/auth flows rather than adding a heavy client abstraction layer.
