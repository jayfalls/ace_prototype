-- name: CreateAuthToken :one
-- Creates a new auth token (for magic links, verification, password reset).
INSERT INTO auth_tokens (
    id,
    user_id,
    token_type,
    token_hash,
    expires_at,
    created_at
)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING
    id,
    user_id,
    token_type,
    token_hash,
    expires_at,
    used_at,
    created_at;

-- name: GetAuthTokenByHash :one
-- Gets an auth token by hash, checking it hasn't been used.
-- Returns only unused tokens that haven't expired.
SELECT
    id,
    user_id,
    token_type,
    token_hash,
    expires_at,
    used_at,
    created_at
FROM auth_tokens
WHERE token_hash = ?
  AND used_at IS NULL
  AND expires_at > ?;

-- name: GetAuthTokenByID :one
-- Gets an auth token by ID.
SELECT
    id,
    user_id,
    token_type,
    token_hash,
    expires_at,
    used_at,
    created_at
FROM auth_tokens
WHERE id = ?;

-- name: MarkAuthTokenUsed :one
-- Marks an auth token as used by setting used_at timestamp.
UPDATE auth_tokens
SET used_at = ?
WHERE id = ?
  AND used_at IS NULL
  AND expires_at > ?
RETURNING
    id,
    user_id,
    token_type,
    token_hash,
    expires_at,
    used_at,
    created_at;

-- name: MarkAuthTokenUsedByHash :one
-- Marks an auth token as used by hash.
UPDATE auth_tokens
SET used_at = ?
WHERE token_hash = ?
  AND used_at IS NULL
  AND expires_at > ?
RETURNING
    id,
    user_id,
    token_type,
    token_hash,
    expires_at,
    used_at,
    created_at;

-- name: DeleteExpiredAuthTokens :exec
-- Deletes all expired auth tokens (cleanup job).
DELETE FROM auth_tokens
WHERE expires_at < ?;

-- name: DeleteAuthTokenByHash :exec
-- Deletes an auth token by hash (e.g., when user cancels action).
DELETE FROM auth_tokens
WHERE token_hash = ?;

-- name: DeleteAuthTokenByID :exec
-- Deletes an auth token by ID.
DELETE FROM auth_tokens
WHERE id = ?;

-- name: DeleteAuthTokensByUserID :exec
-- Deletes all auth tokens for a user (e.g., when user is deleted).
DELETE FROM auth_tokens
WHERE user_id = ?;

-- name: CountUnusedAuthTokensByUserID :one
-- Counts unused auth tokens for a user (for rate limiting).
SELECT COUNT(*) AS count
FROM auth_tokens
WHERE user_id = ?
  AND used_at IS NULL
  AND expires_at > ?;

-- name: CountAuthTokensByTypeAndUser :one
-- Counts auth tokens of a specific type for a user.
SELECT COUNT(*) AS count
FROM auth_tokens
WHERE user_id = ?
  AND token_type = ?
  AND used_at IS NULL
  AND expires_at > ?;
