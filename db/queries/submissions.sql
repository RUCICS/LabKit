-- name: CreateSubmission :one
INSERT INTO submissions (
    user_id, lab_id, key_id, artifact_key, content_hash, status,
    verdict, message, detail, image_digest, started_at, finished_at
)
VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: GetSubmission :one
SELECT *
FROM submissions
WHERE id = $1
LIMIT 1;

-- name: ListSubmissionsByUserLab :many
SELECT *
FROM submissions
WHERE user_id = $1 AND lab_id = $2
ORDER BY created_at DESC;

-- name: GetLatestScoredSubmissionByUserLab :one
SELECT *
FROM submissions
WHERE user_id = $1
  AND lab_id = $2
  AND verdict = 'scored'
ORDER BY finished_at DESC NULLS LAST, created_at DESC
LIMIT 1;

-- name: UpdateSubmissionResult :exec
UPDATE submissions
SET status = $2,
    verdict = $3,
    message = $4,
    detail = $5,
    image_digest = $6,
    started_at = $7,
    finished_at = $8
WHERE id = $1;

-- name: UpdateSubmissionRunning :exec
UPDATE submissions
SET status = $2,
    started_at = $3
WHERE id = $1;

-- name: CreateScore :exec
INSERT INTO scores (submission_id, metric_id, value)
VALUES ($1, $2, $3)
ON CONFLICT (submission_id, metric_id)
DO UPDATE SET value = EXCLUDED.value;

-- name: ListScoresBySubmission :many
SELECT *
FROM scores
WHERE submission_id = $1
ORDER BY metric_id;
