BEGIN;

ALTER TABLE device_auth_requests
    ADD COLUMN device_name TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE user_keys
    DROP CONSTRAINT user_keys_public_key_key;

CREATE UNIQUE INDEX idx_user_keys_active_public_key
    ON user_keys (public_key)
    WHERE revoked_at IS NULL;

COMMIT;
