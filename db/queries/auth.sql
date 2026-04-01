-- name: CreateUser :one
INSERT INTO users (student_id)
VALUES ($1)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByStudentID :one
SELECT *
FROM users
WHERE student_id = $1
LIMIT 1;

-- name: GetUserKeyByID :one
SELECT *
FROM user_keys
WHERE id = $1
  AND revoked_at IS NULL
LIMIT 1;

-- name: GetUserKeyByFingerprint :one
SELECT *
FROM user_keys
WHERE fingerprint = $1
  AND revoked_at IS NULL
LIMIT 1;

-- name: CreateUserKey :one
INSERT INTO user_keys (user_id, public_key, fingerprint, device_name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUserKeyFingerprint :exec
UPDATE user_keys
SET fingerprint = $2
WHERE id = $1
  AND fingerprint IS NULL;

-- name: ListUserKeys :many
SELECT *
FROM user_keys
WHERE user_id = $1
  AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: ListActiveUserKeys :many
SELECT *
FROM user_keys
WHERE revoked_at IS NULL
ORDER BY created_at DESC;

-- name: DeleteUserKey :exec
UPDATE user_keys
SET revoked_at = NOW()
WHERE id = $1
  AND user_id = $2
  AND revoked_at IS NULL;

-- name: CreateDeviceAuthRequest :one
INSERT INTO device_auth_requests (device_code, user_code, public_key, student_id, oauth_state, status, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetDeviceAuthRequestByDeviceCode :one
SELECT *
FROM device_auth_requests
WHERE device_code = $1
LIMIT 1;

-- name: GetPendingDeviceAuthRequestByUserCode :one
SELECT *
FROM device_auth_requests
WHERE user_code = $1 AND status = 'pending'
LIMIT 1;

-- name: GetPendingDeviceAuthRequestByOAuthState :one
SELECT *
FROM device_auth_requests
WHERE oauth_state = $1 AND status = 'pending'
LIMIT 1;

-- name: CompleteDeviceAuthRequest :one
WITH completed_request AS (
    UPDATE device_auth_requests
    SET student_id = $3,
        status = 'approved'
    WHERE device_code = $1
      AND oauth_state = $2
      AND status = 'pending'
      AND expires_at > NOW()
    RETURNING public_key, student_id
),
upsert_user AS (
    INSERT INTO users (student_id)
    SELECT $3
    FROM completed_request
    ON CONFLICT (student_id) DO UPDATE
        SET student_id = EXCLUDED.student_id
    RETURNING id, student_id
),
inserted_key AS (
    INSERT INTO user_keys (user_id, public_key, fingerprint, device_name)
    SELECT upsert_user.id, completed_request.public_key, $4, $5
    FROM upsert_user, completed_request
    RETURNING id
)
SELECT upsert_user.id AS user_id,
       inserted_key.id AS user_key_id,
       upsert_user.student_id
FROM upsert_user, inserted_key;

-- name: ExpireDeviceAuthRequests :exec
UPDATE device_auth_requests
SET status = 'expired'
WHERE status = 'pending'
  AND expires_at < NOW();
