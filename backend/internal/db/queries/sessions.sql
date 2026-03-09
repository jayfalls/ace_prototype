-- name: CreateSession :one
INSERT INTO sessions (
    agent_id,
    owner_id,
    status,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4
) RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: ListSessionsByAgent :many
SELECT * FROM sessions 
WHERE agent_id = $1 
ORDER BY started_at DESC 
LIMIT $2 OFFSET $3;

-- name: ListSessionsByOwner :many
SELECT * FROM sessions 
WHERE owner_id = $1 
ORDER BY started_at DESC 
LIMIT $2 OFFSET $3;

-- name: EndSession :one
UPDATE sessions SET
    status = $2,
    ended_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;
