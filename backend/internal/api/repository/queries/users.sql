-- name: CreateUser :one
-- Creates a new user account.
INSERT INTO users (
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    created_at,
    updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at;

-- name: GetUserByID :one
-- Gets a user by ID, excluding soft-deleted users.
SELECT
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at
FROM users
WHERE id = ?
  AND deleted_at IS NULL;

-- name: GetUserByEmail :one
-- Gets a user by email, excluding soft-deleted users.
SELECT
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at
FROM users
WHERE email = ?
  AND deleted_at IS NULL;

-- name: GetUserByUsername :one
-- Gets a user by username, excluding soft-deleted users.
SELECT
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at
FROM users
WHERE username = ?
  AND deleted_at IS NULL;

-- name: UpdateUser :one
-- Updates user fields (email, username, password_hash, pin_hash, role, status).
UPDATE users
SET
    email = COALESCE(?, email),
    username = COALESCE(?, username),
    password_hash = COALESCE(?, password_hash),
    pin_hash = COALESCE(?, pin_hash),
    role = COALESCE(?, role),
    status = COALESCE(?, status),
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at;

-- name: SoftDeleteUser :one
-- Soft-deletes a user by setting deleted_at timestamp.
UPDATE users
SET
    deleted_at = ?,
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at;

-- name: ListUsers :many
-- Lists users with optional status filter and pagination.
SELECT
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at
FROM users
WHERE deleted_at IS NULL
  AND (? IS NULL OR status = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListUsersCount :one
-- Counts users with optional status filter.
SELECT COUNT(*) AS count
FROM users
WHERE deleted_at IS NULL
  AND (? IS NULL OR status = ?);

-- name: UpdateUserRole :one
-- Updates only the role field for a user.
UPDATE users
SET
    role = ?,
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at;

-- name: SuspendUser :one
-- Suspends a user account with reason.
UPDATE users
SET
    status = 'suspended',
    suspended_at = ?,
    suspended_reason = ?,
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at;

-- name: RestoreUser :one
-- Restores a suspended user account.
UPDATE users
SET
    status = 'active',
    suspended_at = NULL,
    suspended_reason = NULL,
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL
  AND status = 'suspended'
RETURNING
    id,
    email,
    username,
    password_hash,
    pin_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at;

-- name: CountUsers :one
-- Counts total active (non-deleted) users.
SELECT COUNT(*) AS count
FROM users
WHERE deleted_at IS NULL;

-- name: CountUsersByStatus :one
-- Counts users by status (excluding deleted).
SELECT status, COUNT(*) AS count
FROM users
WHERE deleted_at IS NULL
GROUP BY status;

-- name: GetActiveUserCount :one
-- Gets the count of non-deleted, non-suspended users.
SELECT COUNT(*) AS count
FROM users
WHERE deleted_at IS NULL
  AND status != 'suspended';
