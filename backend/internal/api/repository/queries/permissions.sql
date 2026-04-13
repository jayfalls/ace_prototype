-- name: CreatePermission :one
-- Creates a new resource permission.
INSERT INTO resource_permissions (
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at;

-- name: UpsertPermission :one
-- Creates or updates a resource permission (idempotent).
INSERT INTO resource_permissions (
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, resource_type, resource_id) DO UPDATE SET
    permission_level = excluded.permission_level,
    granted_by = excluded.granted_by
RETURNING
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at;

-- name: GetPermission :one
-- Gets a permission by user, resource type, and resource ID.
SELECT
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
FROM resource_permissions
WHERE user_id = ?
  AND resource_type = ?
  AND resource_id = ?;

-- name: GetPermissionByID :one
-- Gets a permission by its ID.
SELECT
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
FROM resource_permissions
WHERE id = ?;

-- name: DeletePermission :exec
-- Deletes a permission by user, resource type, and resource ID.
DELETE FROM resource_permissions
WHERE user_id = ?
  AND resource_type = ?
  AND resource_id = ?;

-- name: DeletePermissionByID :exec
-- Deletes a permission by its ID.
DELETE FROM resource_permissions
WHERE id = ?;

-- name: ListPermissionsByUser :many
-- Lists all permissions for a user.
SELECT
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
FROM resource_permissions
WHERE user_id = ?
ORDER BY resource_type, created_at DESC;

-- name: ListPermissionsByUserAndType :many
-- Lists all permissions for a user of a specific resource type.
SELECT
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
FROM resource_permissions
WHERE user_id = ?
  AND resource_type = ?
ORDER BY created_at DESC;

-- name: ListPermissionsByResource :many
-- Lists all permissions for a specific resource.
SELECT
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
FROM resource_permissions
WHERE resource_type = ?
  AND resource_id = ?;

-- name: ListPermissionsByResourceType :many
-- Lists all permissions for a specific resource type.
SELECT
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at
FROM resource_permissions
WHERE resource_type = ?
ORDER BY created_at DESC;

-- name: UpdatePermission :one
-- Updates the permission level for an existing permission.
UPDATE resource_permissions
SET permission_level = ?
WHERE user_id = ?
  AND resource_type = ?
  AND resource_id = ?
RETURNING
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at;

-- name: UpdatePermissionByID :one
-- Updates the permission level by permission ID.
UPDATE resource_permissions
SET permission_level = ?
WHERE id = ?
RETURNING
    id,
    user_id,
    resource_type,
    resource_id,
    permission_level,
    granted_by,
    created_at;

-- name: CheckPermissionExists :one
-- Checks if a permission exists for a user on a resource.
SELECT EXISTS(
    SELECT 1 FROM resource_permissions
    WHERE user_id = ?
      AND resource_type = ?
      AND resource_id = ?
) AS permission_exists;

-- name: CountPermissionsByUser :one
-- Counts all permissions for a user.
SELECT COUNT(*) AS count
FROM resource_permissions
WHERE user_id = ?;

-- name: CountPermissionsByResource :one
-- Counts all permissions for a specific resource.
SELECT COUNT(*) AS count
FROM resource_permissions
WHERE resource_type = ?
  AND resource_id = ?;

-- name: DeleteAllPermissionsByUser :exec
-- Deletes all permissions for a user (e.g., when user is deleted).
DELETE FROM resource_permissions
WHERE user_id = ?;

-- name: DeleteAllPermissionsByResource :exec
-- Deletes all permissions for a resource (e.g., when resource is deleted).
DELETE FROM resource_permissions
WHERE resource_type = ?
  AND resource_id = ?;

-- name: GetUsersWithResourceAccess :many
-- Gets all users with any permission on a resource.
SELECT DISTINCT
    rp.user_id,
    rp.resource_type,
    rp.resource_id,
    rp.permission_level,
    rp.created_at
FROM resource_permissions rp
WHERE rp.resource_type = ?
  AND rp.resource_id = ?
ORDER BY rp.created_at DESC;
