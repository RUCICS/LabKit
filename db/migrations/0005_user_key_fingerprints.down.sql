BEGIN;

DROP INDEX IF EXISTS idx_user_keys_active_fingerprint;

ALTER TABLE user_keys
    DROP COLUMN IF EXISTS fingerprint;

COMMIT;
