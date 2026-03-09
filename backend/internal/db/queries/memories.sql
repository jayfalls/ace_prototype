-- name: CreateMemory :one
INSERT INTO memories (
    owner_id,
    agent_id,
    content,
    memory_type,
    parent_id,
    tags,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
) RETURNING *;

-- name: GetMemoryByID :one
SELECT * FROM memories WHERE id = $1;

-- name: ListMemoriesByOwner :many
SELECT * FROM memories 
WHERE owner_id = $1 
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- name: ListMemoriesByType :many
SELECT * FROM memories 
WHERE owner_id = $1 AND memory_type = $2
ORDER BY created_at DESC 
LIMIT $3 OFFSET $4;

-- name: ListMemoriesByAgent :many
SELECT * FROM memories 
WHERE owner_id = $1 AND agent_id = $2
ORDER BY created_at DESC 
LIMIT $3 OFFSET $4;

-- name: SearchMemories :many
SELECT * FROM memories 
WHERE owner_id = $1 AND (
    content ILIKE '%' || $2 || '%' OR
    $2 = ANY(tags)
)
ORDER BY created_at DESC 
LIMIT $3 OFFSET $4;

-- name: UpdateMemory :one
UPDATE memories SET
    content = $2,
    memory_type = $3,
    parent_id = $4,
    tags = $5,
    metadata = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteMemory :exec
DELETE FROM memories WHERE id = $1;
