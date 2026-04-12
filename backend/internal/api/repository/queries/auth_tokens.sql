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
VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $3,
    $4,
    NOW()
)
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
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > NOW();

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
WHERE id = $1;

-- name: MarkAuthTokenUsed :one
-- Marks an auth token as used by setting used_at timestamp.
UPDATE auth_tokens
SET used_at = NOW()
WHERE id = $1
  AND used_at IS NULL
  AND expires_at > NOW()
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
SET used_at = NOW()
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > NOW()
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
WHERE expires_at < NOW();

-- name: DeleteAuthTokenByHash :exec
-- Deletes an auth token by hash (e.g., when user cancels action).
DELETE FROM auth_tokens
WHERE token_hash = $1;

-- name: DeleteAuthTokenByID :exec
-- Deletes an auth token by ID.
DELETE FROM auth_tokens
WHERE id = $1;

-- name: DeleteAuthTokensByUserID :exec
-- Deletes all auth tokens for a user (e.g., when user is deleted).
DELETE FROM auth_tokens
WHERE user_id = $1;

-- name: CountUnusedAuthTokensByUserID :one
-- Counts unused auth tokens for a user (for rate limiting).
SELECT COUNT(*) AS count
FROM auth_tokens
WHERE user_id = $1
  AND used_at IS NULL
  AND expires_at > NOW();

-- name: CountAuthTokensByTypeAndUser :one
-- Counts auth tokens of a specific type for a user.
SELECT COUNT(*) AS count
FROM auth_tokens
WHERE user_id = $1
  AND token_type = $2
  AND used_at IS NULL
  AND expires_at > NOW();
