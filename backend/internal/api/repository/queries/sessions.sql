-- name: CreateSession :one
-- Creates a new session for a user.
INSERT INTO sessions (
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at;

-- name: GetSessionByID :one
-- Gets a session by ID.
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE id = ?;

-- name: GetSessionByUserID :many
-- Gets all sessions for a user.
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE user_id = ?
  AND expires_at > ?
ORDER BY last_used_at DESC;

-- name: GetSessionByRefreshTokenHash :one
-- Gets a session by refresh token hash.
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE refresh_token_hash = ?
  AND expires_at > ?;

-- name: UpdateSessionLastUsed :one
-- Updates the last_used_at timestamp for a session.
UPDATE sessions
SET last_used_at = ?
WHERE id = ?
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at;

-- name: UpdateSessionRefreshTokenHash :one
-- Updates the refresh_token_hash for a session.
UPDATE sessions
SET refresh_token_hash = ?
WHERE id = ?
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at;

-- name: DeleteSession :exec
-- Deletes a specific session by ID.
DELETE FROM sessions
WHERE id = ?;

-- name: DeleteSessionByRefreshTokenHash :exec
-- Deletes a session by refresh token hash.
DELETE FROM sessions
WHERE refresh_token_hash = ?;

-- name: DeleteAllSessionsByUserID :exec
-- Deletes all sessions for a user (used when revoking all sessions).
DELETE FROM sessions
WHERE user_id = ?;

-- name: DeleteExpiredSessions :exec
-- Deletes all expired sessions (cleanup job).
DELETE FROM sessions
WHERE expires_at < ?;

-- name: CountSessionsByUserID :one
-- Counts active sessions for a user.
SELECT COUNT(*) AS count
FROM sessions
WHERE user_id = ?
  AND expires_at > ?;

-- name: GetSessionByIDAndUserID :one
-- Gets a session by ID and user ID (for verification).
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE id = ?
  AND user_id = ?
  AND expires_at > ?;
