-- name: CreateUser :one
-- Creates a new user account.
INSERT INTO users (
    id,
    email,
    password_hash,
    role,
    status,
    created_at,
    updated_at
)
VALUES (
    gen_random_uuid(),
    $1,
    $2,
    COALESCE($3, 'user'::VARCHAR),
    COALESCE($4, 'pending'::VARCHAR),
    NOW(),
    NOW()
)
RETURNING
    id,
    email,
    password_hash,
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
    password_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at
FROM users
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetUserByEmail :one
-- Gets a user by email, excluding soft-deleted users.
SELECT
    id,
    email,
    password_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at
FROM users
WHERE email = $1
  AND deleted_at IS NULL;

-- name: UpdateUser :one
-- Updates user fields (email, password_hash, role, status).
UPDATE users
SET
    email = COALESCE($2, email),
    password_hash = COALESCE($3, password_hash),
    role = COALESCE($4, role),
    status = COALESCE($5, status),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    password_hash,
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
    deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    password_hash,
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
    password_hash,
    role,
    status,
    suspended_at,
    suspended_reason,
    deleted_at,
    created_at,
    updated_at
FROM users
WHERE deleted_at IS NULL
  AND ($1::VARCHAR IS NULL OR status = $1::VARCHAR)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUsersCount :one
-- Counts users with optional status filter.
SELECT COUNT(*) AS count
FROM users
WHERE deleted_at IS NULL
  AND ($1::VARCHAR IS NULL OR status = $1::VARCHAR);

-- name: UpdateUserRole :one
-- Updates only the role field for a user.
UPDATE users
SET
    role = $2,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    password_hash,
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
    status = 'suspended'::VARCHAR,
    suspended_at = NOW(),
    suspended_reason = $2,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING
    id,
    email,
    password_hash,
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
    status = 'active'::VARCHAR,
    suspended_at = NULL,
    suspended_reason = NULL,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
  AND status = 'suspended'::VARCHAR
RETURNING
    id,
    email,
    password_hash,
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
  AND status != 'suspended'::VARCHAR;
