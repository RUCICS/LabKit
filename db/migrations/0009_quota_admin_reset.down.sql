BEGIN;

UPDATE submissions SET quota_state = 'free' WHERE quota_state = 'admin_reset';

ALTER TABLE submissions
    DROP CONSTRAINT IF EXISTS submissions_quota_state_check;

ALTER TABLE submissions
    ADD CONSTRAINT submissions_quota_state_check
    CHECK (quota_state IN ('pending', 'charged', 'free'));

COMMIT;
