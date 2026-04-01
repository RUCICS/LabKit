BEGIN;

CREATE TABLE web_session_tickets (
    ticket_hash TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    key_id BIGINT NOT NULL REFERENCES user_keys(id) ON DELETE CASCADE,
    redirect_path TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_web_session_tickets_expires_at
    ON web_session_tickets (expires_at);

COMMIT;
