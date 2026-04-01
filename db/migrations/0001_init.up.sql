BEGIN;

CREATE TABLE labs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    manifest JSONB NOT NULL,
    manifest_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    student_id TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    public_key TEXT NOT NULL UNIQUE,
    device_name TEXT NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE device_auth_requests (
    id BIGSERIAL PRIMARY KEY,
    device_code TEXT NOT NULL UNIQUE,
    user_code TEXT NOT NULL,
    public_key TEXT NOT NULL,
    student_id TEXT,
    oauth_state TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'expired')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_user_code_pending
    ON device_auth_requests (user_code)
    WHERE status = 'pending';

CREATE TABLE lab_profiles (
    user_id BIGINT NOT NULL REFERENCES users(id),
    lab_id TEXT NOT NULL REFERENCES labs(id),
    nickname TEXT NOT NULL DEFAULT '匿名',
    track TEXT,
    PRIMARY KEY (user_id, lab_id)
);

CREATE TABLE submissions (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    lab_id TEXT NOT NULL REFERENCES labs(id),
    key_id BIGINT NOT NULL REFERENCES user_keys(id),
    artifact_key TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    status TEXT NOT NULL
        CHECK (status IN ('queued', 'running', 'done', 'timeout', 'error')),
    verdict TEXT
        CHECK (verdict IS NULL OR verdict IN ('build_failed', 'rejected', 'scored', 'error')),
    message TEXT,
    detail JSONB,
    image_digest TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_submissions_lab_user_created_at
    ON submissions (lab_id, user_id, created_at DESC);

CREATE INDEX idx_submissions_lab_status_created_at
    ON submissions (lab_id, status, created_at DESC);

CREATE TABLE scores (
    submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    metric_id TEXT NOT NULL,
    value REAL NOT NULL,
    PRIMARY KEY (submission_id, metric_id)
);

CREATE INDEX idx_scores_metric_id
    ON scores (metric_id);

CREATE TABLE leaderboard (
    user_id BIGINT NOT NULL REFERENCES users(id),
    lab_id TEXT NOT NULL REFERENCES labs(id),
    submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, lab_id)
);

CREATE INDEX idx_leaderboard_lab_updated_at
    ON leaderboard (lab_id, updated_at DESC);

CREATE TABLE used_nonces (
    nonce TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
