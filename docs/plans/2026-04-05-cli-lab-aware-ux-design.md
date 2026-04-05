# CLI Lab-Aware UX Design

## Context

The CLI already has a richer submit live view and a reusable single-line spinner, but the rest of the command surface still has a few rough edges:

- `board` and `history` can block on network fetches with no feedback.
- Some CLI copy still feels platform-generic even when the command is clearly scoped to a single lab.
- Empty and confirmation states leave a little too much interpretation to the user.

The goal is to improve perceived responsiveness and context clarity without changing protocol-level identifiers or turning the CLI into a per-lab binary.

## UX Boundary

This change splits copy into two categories.

### Platform semantics: keep `LabKit`

These remain unchanged because they describe the shared platform rather than one lab:

- binary / root command name: `labkit`
- config directory and env vars: `.labkit`, `LABKIT_*`
- request headers and API contract: `X-LabKit-*`, `/api/...`
- web platform entry points such as `LabKit web`
- account / auth flow messaging that is not lab-specific

### Lab semantics: personalize by active lab

These should use the current lab manifest where it improves clarity:

- leaderboard and history headings
- submission headings and follow-up hints
- nickname / track success confirmations
- lab-specific empty states

The source of truth is `manifest.lab.name`, falling back to the lab ID and finally to existing generic copy if needed.

## Proposed Changes

### 1. Loading feedback for fetch-then-render commands

Use the existing line spinner for commands that may wait on one or more network round trips before rendering output:

- `board`
- `history`
- `history <submission-id>`
- `nick`
- `track`
- `keys`
- `revoke`

Spinner copy should stay generic and stable:

- `Loading leaderboard…`
- `Loading history…`
- `Loading submission…`
- `Updating nickname…`
- `Updating track…`
- `Loading keys…`
- `Revoking key…`

`web` already uses a spinner and should only be aligned stylistically if needed, not rebranded around the lab.

### 2. Lab-aware headings and confirmations

Introduce a small CLI UX context helper that derives a human-facing lab display name from the active lab manifest. Use it only for commands that already fetch the manifest as part of their normal flow.

Examples:

- `Leaderboard · Sorting · 42 participants`
- `Submission history · Malloc Lab`
- `Nickname updated · Sorting · Cat`
- `Track set · Sorting · latency`

This keeps the platform identity intact while making the current lab explicit.

### 3. Better empty and next-step states

When a table would otherwise render with no rows, show a direct empty-state sentence plus a next-step hint.

Examples:

- leaderboard empty: no ranked submissions yet, suggest `submit`
- history empty: no submissions yet, suggest `submit`

This is especially useful in a fresh lab where the current table framing can feel like a failed render rather than an empty result.

## Implementation Notes

- Avoid adding another configuration layer. Lab-aware copy should be derived from the manifest already fetched by the command.
- Keep spinner handling centralized so commands do not duplicate start/stop/error cleanup logic.
- Preserve non-TTY behavior: the spinner should degrade to a single plain text line and never corrupt output.
- Do not rename `LabKit web` or auth/platform copy.

## Testing

Add CLI tests that cover:

- `board` emits a loading line before the final render on non-TTY output
- `history` emits a loading line before the final render on non-TTY output
- headings include the active lab name where intended
- empty leaderboard / empty history render explicit next-step messaging
- success confirmations for `nick` and `track` include the lab name

## Recommendation

Implement the change as a focused CLI polish pass inside `apps/cli/internal/commands`, reusing the existing spinner rather than introducing a separate progress abstraction.
