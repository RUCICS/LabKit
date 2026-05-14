# Bonus Submission Quota

Bonus quota is a per-`(user, lab)` counter that students can spend **after**
their daily quota is exhausted, with no automatic reset. It is intended for
one-off staff-issued compensation (e.g. extending submission capacity after
an outage).

## Data model

- `user_lab_bonus_quota (user_id, lab_id, remaining, updated_at)` —
  added in migration `0010`. `remaining` defaults to 0; an absent row is
  treated as 0, so no historical user is affected by deployment.
- Submissions consume bonus quota via a single `UPDATE … remaining = remaining
  - 1 … RETURNING remaining` inside the same transaction that already takes the
  per-user-per-lab advisory lock.
- Bonus-funded submissions are stored with `submissions.quota_state = 'bonus'`
  (new value, added to the existing CHECK constraint in migration `0010`).
  `CountSubmissionQuotaUsage` keeps its existing `WHERE quota_state IN
  ('pending', 'charged')` filter, so `'bonus'` rows never count toward the
  daily window.
- Workers preserve `quota_state = 'bonus'` end-to-end the same way they
  preserve `'free'` — they will not flip it to `'charged'`.

## API

All endpoints require the admin bearer token (`LABKIT_ADMIN_TOKEN`).

- `POST /api/admin/labs/{labID}/quota/bonus`
  body: `{ "delta": <int>, "dry_run": <bool, optional> }`
  - Adds `delta` to every participant's `remaining`. Negative `delta` removes
    quota with a floor at zero. `delta = 0` is rejected.
  - Participants = `SELECT DISTINCT user_id FROM submissions WHERE lab_id = $1`.
    Students who never submitted are excluded (compensation only makes sense
    for users who showed up).
  - `dry_run: true` returns the participant count without writing.
- `POST /api/admin/labs/{labID}/quota/bonus/reset`
  body: `{ "dry_run": <bool, optional> }`
  - Zeroes out every row for the lab.

The response shape:

```json
{
  "lab_id": "colab-2026-p2",
  "delta": 5,
  "users_affected": 42,
  "lab_participants": 42,
  "dry_run": false
}
```

## CLI wrapper (production-safe)

`scripts/admin-bonus-quota.sh` shows the affected user count, prompts for
confirmation, and then posts the mutation.

```bash
export LABKIT_API_URL=https://<prod-host>
export LABKIT_ADMIN_TOKEN=<prod-token>

# Grant +5 bonus submissions to every participant of colab-2026-p2
scripts/admin-bonus-quota.sh --project colab-2026-p2 --add 5

# Revoke 2 bonus submissions (floors at 0)
scripts/admin-bonus-quota.sh --project colab-2026-p2 --add -2

# Reset everyone to 0
scripts/admin-bonus-quota.sh --project colab-2026-p2 --reset
```

`ASSUME_YES=1` skips the prompt for non-interactive use.

## Verification on a dev environment

1. Apply migration `0010` (the migrate binary runs all `*.up.sql` in order).
2. Pick a test user whose daily quota is already exhausted, or seed one:

   ```sql
   -- Fast-forward today's daily window for user 123, lab colab-2026-p2,
   -- daily quota = 3
   INSERT INTO submissions (id, user_id, lab_id, key_id, artifact_key,
                            content_hash, status, quota_state, created_at)
   SELECT gen_random_uuid(), 123, 'colab-2026-p2', k.id,
          'seed/'||g, 'seedhash'||g, 'charged', 'charged', NOW()
   FROM user_keys k, generate_series(1, 3) g
   WHERE k.user_id = 123 LIMIT 3;
   ```

3. Grant the user some bonus credit:

   ```bash
   LABKIT_ADMIN_TOKEN=dev-admin-token \
     scripts/admin-bonus-quota.sh --project colab-2026-p2 --add 5
   ```

4. As that user, hit `GET /api/labs/colab-2026-p2/submit/precheck`:
   `quota.left = 0`, `quota.bonus.remaining = 5`. Submit one more file.
   The intake transaction:
   - Sees daily exhausted.
   - Calls `ConsumeBonusQuota`, returns `remaining = 4`.
   - Creates the submission with `quota_state = 'bonus'`.
   - Returns `result.quota.bonus.remaining = 4`.
5. Inspect:

   ```sql
   SELECT remaining FROM user_lab_bonus_quota WHERE user_id = 123;
   SELECT quota_state FROM submissions WHERE user_id = 123 ORDER BY created_at DESC LIMIT 1;
   ```

   You should see `remaining = 4` and the latest submission's
   `quota_state = 'bonus'`.

6. To roll back the grant: `scripts/admin-bonus-quota.sh --project
   colab-2026-p2 --add -5` (or `--reset`).

## Refund on system errors

A bonus-funded submission that resolves to a no-charge verdict (the same
verdicts that would map a daily-funded submission to `quota_state = 'free'`
— i.e. `VerdictError`, or anything listed in `manifest.quota.free`) is
**automatically refunded**:

- The bonus credit is added back to `user_lab_bonus_quota.remaining`.
- The submission's `quota_state` is flipped from `'bonus'` to `'free'`.

The two writes happen in a single CTE inside the worker's Persist
transaction. Idempotency is enforced by the `WHERE quota_state = 'bonus'`
guard in that CTE — a retried Persist (worker crash before job
acknowledgement, or a stale duplicate handoff) finds `quota_state` already
`'free'` and no-ops, so each bonus credit is refunded at most once.

Symmetry with daily quota: a daily-funded `VerdictError` submission is
recorded with `quota_state = 'free'` and does not count toward today's
window. The bonus equivalent recovers the credit instead — for daily quota
the "refund" is implicit because consumption is counted at read time; for
bonus the credit is materialised, so we have to add it back explicitly.

Scored submissions (`VerdictScored`) and charge-eligible failures
(`VerdictRejected`/`VerdictBuildFailed` when not declared free) keep
`quota_state = 'bonus'`; the credit stays consumed.
