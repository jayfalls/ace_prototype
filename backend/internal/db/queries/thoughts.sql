-- name: CreateThought :one
INSERT INTO thoughts (
    session_id,
    layer,
    content,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4
) RETURNING *;

-- name: GetThoughtByID :one
SELECT * FROM thoughts WHERE id = $1;

-- name: ListThoughtsBySession :many
SELECT * FROM thoughts 
WHERE session_id = $1 
ORDER BY created_at ASC 
LIMIT $2 OFFSET $3;

-- name: ListThoughtsByLayer :many
SELECT * FROM thoughts 
WHERE session_id = $1 AND layer = $2
ORDER BY created_at ASC 
LIMIT $3 OFFSET $4;

-- name: DeleteThought :exec
DELETE FROM thoughts WHERE id = $1;
