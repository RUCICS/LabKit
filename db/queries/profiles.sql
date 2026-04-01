-- name: UpsertLabProfileNickname :one
INSERT INTO lab_profiles (user_id, lab_id, nickname)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, lab_id)
DO UPDATE SET nickname = EXCLUDED.nickname
RETURNING user_id, lab_id, nickname, track;

-- name: UpsertLabProfileTrack :one
INSERT INTO lab_profiles (user_id, lab_id, nickname, track)
VALUES ($1, $2, '匿名', $3)
ON CONFLICT (user_id, lab_id)
DO UPDATE SET track = EXCLUDED.track
RETURNING user_id, lab_id, nickname, track;

-- name: ListLabProfilesByLab :many
SELECT user_id, lab_id, nickname, track
FROM lab_profiles
WHERE lab_id = $1
ORDER BY user_id;
