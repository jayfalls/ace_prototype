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
VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $3,
    $4,
    NOW(),
    $5,
    NOW()
)
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address::VARCHAR,
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
    ip_address::VARCHAR,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE id = $1;

-- name: GetSessionByUserID :many
-- Gets all sessions for a user.
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address::VARCHAR,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE user_id = $1
  AND expires_at > NOW()
ORDER BY last_used_at DESC;

-- name: GetSessionByRefreshTokenHash :one
-- Gets a session by refresh token hash.
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address::VARCHAR,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE refresh_token_hash = $1
  AND expires_at > NOW();

-- name: UpdateSessionLastUsed :one
-- Updates the last_used_at timestamp for a session.
UPDATE sessions
SET last_used_at = NOW()
WHERE id = $1
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address::VARCHAR,
    last_used_at,
    expires_at,
    created_at;

-- name: DeleteSession :exec
-- Deletes a specific session by ID.
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteSessionByRefreshTokenHash :exec
-- Deletes a session by refresh token hash.
DELETE FROM sessions
WHERE refresh_token_hash = $1;

-- name: DeleteAllSessionsByUserID :exec
-- Deletes all sessions for a user (used when revoking all sessions).
DELETE FROM sessions
WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
-- Deletes all expired sessions (cleanup job).
DELETE FROM sessions
WHERE expires_at < NOW();

-- name: CountSessionsByUserID :one
-- Counts active sessions for a user.
SELECT COUNT(*) AS count
FROM sessions
WHERE user_id = $1
  AND expires_at > NOW();

-- name: GetSessionByIDAndUserID :one
-- Gets a session by ID and user ID (for verification).
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address::VARCHAR,
    last_used_at,
    expires_at,
    created_at
FROM sessions
WHERE id = $1
  AND user_id = $2
  AND expires_at > NOW();
