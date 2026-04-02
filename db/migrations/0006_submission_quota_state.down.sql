BEGIN;

ALTER TABLE submissions
    DROP COLUMN quota_state;

COMMIT;
