BEGIN;

ALTER TABLE user_keys
    ADD COLUMN revoked_at TIMESTAMPTZ;

CREATE INDEX idx_user_keys_active_by_user
    ON user_keys (user_id, created_at DESC)
    WHERE revoked_at IS NULL;

COMMIT;
