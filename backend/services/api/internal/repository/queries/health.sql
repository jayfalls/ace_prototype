-- name: GetLatestHealthCheck :one
-- Get the most recent health check record
SELECT id, db, err, created
FROM health_check
ORDER BY created DESC
LIMIT 1;

-- name: CreateHealthCheck :one
-- Insert a new health check record
INSERT INTO health_check (db, err, created)
VALUES ($1, $2, NOW())
RETURNING id, db, err, created;

-- name: ListHealthChecks :many
-- List health check records with optional limit
SELECT id, db, err, created
FROM health_check
ORDER BY created DESC
LIMIT $1;
