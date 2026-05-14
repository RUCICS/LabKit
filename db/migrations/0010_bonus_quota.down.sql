BEGIN;

ALTER TABLE submissions
    DROP CONSTRAINT IF EXISTS submissions_quota_state_check;

ALTER TABLE submissions
    ADD CONSTRAINT submissions_quota_state_check
    CHECK (quota_state IN ('pending', 'charged', 'free', 'admin_reset'));

DROP TABLE IF EXISTS user_lab_bonus_quota;

COMMIT;
