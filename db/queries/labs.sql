-- name: CreateLab :one
INSERT INTO labs (id, name, manifest, manifest_updated_at)
VALUES ($1, $2, $3, NOW())
RETURNING *;

-- name: GetLab :one
SELECT *
FROM labs
WHERE id = $1
LIMIT 1;

-- name: ListLabs :many
SELECT *
FROM labs
ORDER BY created_at DESC;

-- name: UpdateLabManifest :exec
UPDATE labs
SET name = $2,
    manifest = $3,
    manifest_updated_at = NOW()
WHERE id = $1;
