BEGIN;

ALTER TABLE submissions
    ALTER COLUMN id SET DEFAULT uuidv7();

CREATE TABLE evaluation_jobs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    submission_id UUID NOT NULL UNIQUE REFERENCES submissions(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'done', 'error')),
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    worker_id TEXT,
    last_error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_evaluation_jobs_status_available_at
    ON evaluation_jobs (status, available_at);

COMMIT;
