BEGIN;

ALTER TABLE user_keys
    ADD COLUMN fingerprint TEXT;

CREATE UNIQUE INDEX idx_user_keys_active_fingerprint
    ON user_keys (fingerprint)
    WHERE revoked_at IS NULL
      AND fingerprint IS NOT NULL;

COMMIT;
