BEGIN;

DROP INDEX IF EXISTS idx_user_keys_active_public_key;

ALTER TABLE user_keys
    ADD CONSTRAINT user_keys_public_key_key UNIQUE (public_key);

ALTER TABLE device_auth_requests
    DROP COLUMN device_name;

COMMIT;
