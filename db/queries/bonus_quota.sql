-- name: GetBonusQuota :one
SELECT remaining
FROM user_lab_bonus_quota
WHERE user_id = $1 AND lab_id = $2;

-- name: ConsumeBonusQuota :one
UPDATE user_lab_bonus_quota
SET remaining = remaining - 1,
    updated_at = NOW()
WHERE user_id = $1
  AND lab_id = $2
  AND remaining > 0
RETURNING remaining;

-- name: AdjustBonusQuotaForLab :execrows
INSERT INTO user_lab_bonus_quota (user_id, lab_id, remaining, updated_at)
SELECT s.user_id, sqlc.arg(lab_id)::text, GREATEST(0, sqlc.arg(delta)::int), NOW()
FROM (SELECT DISTINCT user_id FROM submissions WHERE lab_id = sqlc.arg(lab_id)::text) AS s
ON CONFLICT (user_id, lab_id)
DO UPDATE SET
    remaining = GREATEST(0, user_lab_bonus_quota.remaining + sqlc.arg(delta)::int),
    updated_at = NOW();

-- name: ResetBonusQuotaForLab :execrows
UPDATE user_lab_bonus_quota
SET remaining = 0,
    updated_at = NOW()
WHERE lab_id = $1
  AND remaining > 0;

-- name: RefundBonusSubmission :execrows
WITH flipped AS (
    UPDATE submissions
    SET quota_state = 'free'
    WHERE id = $1 AND quota_state = 'bonus'
    RETURNING user_id, lab_id
)
UPDATE user_lab_bonus_quota ulbq
SET remaining = remaining + 1,
    updated_at = NOW()
FROM flipped
WHERE ulbq.user_id = flipped.user_id
  AND ulbq.lab_id = flipped.lab_id;

-- name: CountLabParticipants :one
SELECT COUNT(DISTINCT user_id)
FROM submissions
WHERE lab_id = $1;
