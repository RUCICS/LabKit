BEGIN;

ALTER TABLE submissions
    ADD COLUMN quota_state TEXT NOT NULL DEFAULT 'pending'
        CHECK (quota_state IN ('pending', 'charged', 'free'));

COMMIT;
