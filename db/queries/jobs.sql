-- name: CreateEvaluationJob :one
INSERT INTO evaluation_jobs (submission_id)
VALUES ($1)
RETURNING *;

-- name: GetNextQueuedEvaluationJob :one
SELECT *
FROM evaluation_jobs
WHERE status = 'queued' AND available_at <= NOW()
ORDER BY available_at, created_at
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: MarkEvaluationJobRunning :exec
UPDATE evaluation_jobs
SET status = 'running',
    attempts = attempts + 1,
    worker_id = $2,
    started_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: MarkEvaluationJobDone :execrows
UPDATE evaluation_jobs
SET status = $2,
    last_error = $4,
    finished_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND status = 'running'
  AND worker_id = $3;

-- name: RequeueEvaluationJob :execrows
UPDATE evaluation_jobs
SET status = 'queued',
    available_at = $3,
    worker_id = NULL,
    started_at = NULL,
    finished_at = NULL,
    last_error = $4,
    updated_at = NOW()
WHERE id = $1
  AND status = 'running'
  AND worker_id = $2;

-- name: ListRecentEvaluationJobsByLab :many
SELECT
    ej.id,
    ej.submission_id,
    s.user_id,
    s.lab_id,
    ej.status,
    ej.attempts,
    ej.available_at,
    ej.worker_id,
    ej.last_error,
    ej.started_at,
    ej.finished_at,
    ej.created_at,
    ej.updated_at
FROM evaluation_jobs ej
JOIN submissions s ON s.id = ej.submission_id
WHERE s.lab_id = $1
ORDER BY ej.updated_at DESC, ej.created_at DESC
LIMIT $2;
