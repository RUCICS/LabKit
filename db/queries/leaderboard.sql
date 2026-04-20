-- name: UpsertLeaderboardEntry :one
INSERT INTO leaderboard (user_id, lab_id, submission_id, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (user_id, lab_id)
DO UPDATE SET submission_id = EXCLUDED.submission_id,
              updated_at = NOW()
RETURNING *;

-- name: GetLeaderboardEntry :one
SELECT *
FROM leaderboard
WHERE user_id = $1 AND lab_id = $2
LIMIT 1;

-- name: ListLeaderboardByLab :many
SELECT *
FROM leaderboard
WHERE lab_id = $1
ORDER BY updated_at DESC;

-- name: ListLeaderboardByLabAndMetricAsc :many
SELECT
    lb.user_id,
    u.nickname,
    lb.lab_id,
    lb.submission_id,
    lb.updated_at,
    s.metric_id,
    s.value
FROM leaderboard lb
JOIN users u ON u.id = lb.user_id
JOIN scores s ON s.submission_id = lb.submission_id
WHERE lb.lab_id = $1 AND s.metric_id = $2
ORDER BY s.value ASC, lb.updated_at ASC;

-- name: ListLeaderboardByLabAndMetricDesc :many
SELECT
    lb.user_id,
    u.nickname,
    lb.lab_id,
    lb.submission_id,
    lb.updated_at,
    s.metric_id,
    s.value
FROM leaderboard lb
JOIN users u ON u.id = lb.user_id
JOIN scores s ON s.submission_id = lb.submission_id
WHERE lb.lab_id = $1 AND s.metric_id = $2
ORDER BY s.value DESC, lb.updated_at DESC;

-- name: ListScoresByLab :many
SELECT
    s.submission_id,
    s.metric_id,
    s.value
FROM leaderboard lb
JOIN scores s ON s.submission_id = lb.submission_id
WHERE lb.lab_id = $1
ORDER BY s.submission_id, s.metric_id;
