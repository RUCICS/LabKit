# CLI Loading Polish Design

## Goal

Refine the LabKit CLI waiting experience so it behaves like a polished product rather than a collection of one-off renderers. This includes fixing leaderboard rank presentation for the current user, upgrading submit progress visuals, and introducing a real spinner primitive for long waits.

## Approach

Use a shared TTY-aware loading primitive instead of scattering ad hoc symbols across commands. The shared spinner should only animate in interactive terminals and should degrade to static text in redirected or non-interactive output. The submit live renderer remains specialized, but it should consume the same frame vocabulary and visual timing so motion feels coherent across commands.

The leaderboard should preserve medal semantics independently from the "current user" marker. A current-user row in the top three must still show its medal, with the user marker rendered as a separate aligned prefix. Rank alignment should be stable across medal, numeric rank, and current-user combinations.

## Interaction Rules

- Only animate in TTY mode.
- Only use spinners for flows where the user is genuinely waiting, such as auth polling, submit polling, and browser-session setup.
- Keep short synchronous confirmations static.
- Preserve clean plain-text output when ANSI is stripped.

## Rendering Details

### Leaderboard

- Reserve a fixed-width marker column for the current-user arrow.
- Reserve a fixed-width rank column for medals or rank numbers.
- Keep medals for ranks 1-3 even when the row is the current user.
- Continue green highlighting for the current-user row without breaking table alignment.

### Shared Spinner

- Introduce a reusable spinner helper with a frame set, tick interval, start/update/stop behavior, and TTY fallback.
- Support a single-line waiting message that can be updated in place when interactive.

### Submit Live Renderer

- Replace the flat filled bar with a high-resolution Unicode block bar so the leading edge can move in sub-cell increments.
- Keep the palette constrained to a continuous blue-to-green gradient.
- Add a subtle pulse that sweeps through the filled portion by brightness only, without changing glyph shape aggressively.
- Reuse the shared spinner frame vocabulary so motion across commands feels related.
- Automatically fall back to a simpler bar when the terminal cannot render the Unicode block glyphs reliably.
- Keep stage labels and elapsed time readable even when ANSI is stripped.

## Testing

- Add leaderboard coverage for a current-user row in the top three and verify medal retention plus alignment markers.
- Add shared spinner tests covering non-TTY fallback and TTY frame updates.
- Extend submit live tests to validate high-resolution Unicode block rendering, fallback behavior, and regressions in plain-text readability.
- Add command-level tests for auth waiting output to ensure the new spinner path does not regress non-interactive output.
