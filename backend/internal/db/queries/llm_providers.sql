-- name: CreateLLMProvider :one
INSERT INTO llm_providers (
    owner_id,
    name,
    provider_type,
    api_key_encrypted,
    base_url,
    model,
    config
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
) RETURNING *;

-- name: GetLLMProviderByID :one
SELECT * FROM llm_providers WHERE id = $1;

-- name: ListLLMProvidersByOwner :many
SELECT * FROM llm_providers 
WHERE owner_id = $1 
ORDER BY created_at DESC;

-- name: UpdateLLMProvider :one
UPDATE llm_providers SET
    name = $2,
    provider_type = $3,
    api_key_encrypted = $4,
    base_url = $5,
    model = $6,
    config = $7,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteLLMProvider :exec
DELETE FROM llm_providers WHERE id = $1;

-- name: CreateLLMAttachment :one
INSERT INTO llm_attachments (
    agent_id,
    provider_id,
    layer,
    priority,
    config
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
) RETURNING *;

-- name: ListLLMAttachmentsByAgent :many
SELECT * FROM llm_attachments 
WHERE agent_id = $1 
ORDER BY priority DESC;

-- name: DeleteLLMAttachment :exec
DELETE FROM llm_attachments WHERE id = $1;
