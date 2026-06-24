BEGIN;

-- final_grades is no longer tied to any single lab's grading scheme. Drop the
-- CoLab-specific breakdown columns and store an ordered, free-form breakdown as
-- JSON ([{label, value}]). total becomes an optional display string (the
-- headline), so a lab can grade with whatever columns make sense for it.
ALTER TABLE final_grades
    DROP COLUMN IF EXISTS track,
    DROP COLUMN IF EXISTS ratio,
    DROP COLUMN IF EXISTS perf_score,
    DROP COLUMN IF EXISTS percentile,
    DROP COLUMN IF EXISTS board_score,
    ADD COLUMN IF NOT EXISTS items JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE final_grades
    ALTER COLUMN total DROP NOT NULL,
    ALTER COLUMN total TYPE TEXT USING total::text;

COMMIT;
