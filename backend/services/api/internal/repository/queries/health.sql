-- name: GetLatestHealthCheck :one
-- Get the most recent health check record
SELECT
    id,
    status,
    message,
    checked_at,
    created_at
FROM health_check
ORDER BY checked_at DESC
LIMIT 1;

-- name: CreateHealthCheck :one
-- Insert a new health check record
INSERT INTO health_check (status, message, checked_at)
VALUES ($1, $2, NOW())
RETURNING id, status, message, checked_at, created_at;

-- name: ListHealthChecks :many
-- List health check records with optional limit
SELECT
    id,
    status,
    message,
    checked_at,
    created_at
FROM health_check
ORDER BY checked_at DESC
LIMIT $1;
