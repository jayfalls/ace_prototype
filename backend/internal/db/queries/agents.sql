-- name: CreateAgent :one
INSERT INTO agents (
    owner_id,
    name,
    description,
    config,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
) RETURNING *;

-- name: GetAgentByID :one
SELECT * FROM agents WHERE id = $1;

-- name: ListAgentsByOwner :many
SELECT * FROM agents 
WHERE owner_id = $1 
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- name: ListAgentsByStatus :many
SELECT * FROM agents 
WHERE owner_id = $1 AND status = $2
ORDER BY created_at DESC 
LIMIT $3 OFFSET $4;

-- name: UpdateAgent :one
UPDATE agents SET
    name = $2,
    description = $3,
    config = $4,
    status = $5,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = $1;
