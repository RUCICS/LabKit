BEGIN;

DROP INDEX IF EXISTS idx_user_keys_active_by_user;

ALTER TABLE user_keys
    DROP COLUMN IF EXISTS revoked_at;

COMMIT;
