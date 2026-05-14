BEGIN;

CREATE TABLE user_lab_bonus_quota (
    user_id BIGINT NOT NULL REFERENCES users(id),
    lab_id TEXT NOT NULL REFERENCES labs(id),
    remaining INTEGER NOT NULL DEFAULT 0 CHECK (remaining >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, lab_id)
);

ALTER TABLE submissions
    DROP CONSTRAINT IF EXISTS submissions_quota_state_check;

ALTER TABLE submissions
    ADD CONSTRAINT submissions_quota_state_check
    CHECK (quota_state IN ('pending', 'charged', 'free', 'admin_reset', 'bonus'));

COMMIT;
