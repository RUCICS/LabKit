-- name: CreateWebSessionTicket :one
INSERT INTO web_session_tickets (ticket_hash, user_id, key_id, redirect_path, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: AcquireWebSessionTicketCreateLock :exec
SELECT pg_advisory_xact_lock(424242424242);

-- name: CountActiveWebSessionTickets :one
SELECT count(*)::bigint
FROM web_session_tickets
WHERE expires_at > NOW();

-- name: CleanupExpiredWebSessionTickets :exec
DELETE FROM web_session_tickets
WHERE expires_at <= NOW();

-- name: ConsumeWebSessionTicket :one
DELETE FROM web_session_tickets
WHERE ticket_hash = $1
  AND expires_at > NOW()
RETURNING *;
