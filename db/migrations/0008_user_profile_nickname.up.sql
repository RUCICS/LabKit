BEGIN;

ALTER TABLE users
    ADD COLUMN nickname TEXT NOT NULL DEFAULT '匿名';

WITH normalized_nicknames AS (
    SELECT
        lp.user_id,
        BTRIM(lp.nickname) AS nickname
    FROM lab_profiles lp
    WHERE BTRIM(lp.nickname) <> ''
),
unambiguous_nicknames AS (
    SELECT
        nn.user_id,
        MIN(nn.nickname) AS nickname
    FROM normalized_nicknames nn
    GROUP BY nn.user_id
    HAVING COUNT(DISTINCT nn.nickname) = 1
)
UPDATE users u
SET nickname = unambiguous_nicknames.nickname
FROM unambiguous_nicknames
WHERE u.id = unambiguous_nicknames.user_id;

COMMIT;
