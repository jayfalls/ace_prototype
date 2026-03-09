-- name: GetAgentSetting :one
SELECT * FROM agent_settings WHERE agent_id = $1 AND key = $2;

-- name: ListAgentSettings :many
SELECT * FROM agent_settings WHERE agent_id = $1;

-- name: UpsertAgentSetting :one
INSERT INTO agent_settings (agent_id, key, value)
VALUES ($1, $2, $3)
ON CONFLICT (agent_id, key) 
DO UPDATE SET value = $3, updated_at = NOW()
RETURNING *;

-- name: DeleteAgentSetting :exec
DELETE FROM agent_settings WHERE agent_id = $1 AND key = $2;

-- name: GetSystemSetting :one
SELECT * FROM system_settings WHERE key = $1;

-- name: ListSystemSettings :many
SELECT * FROM system_settings;

-- name: UpsertSystemSetting :one
INSERT INTO system_settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) 
DO UPDATE SET value = $2, updated_at = NOW()
RETURNING *;

-- name: DeleteSystemSetting :exec
DELETE FROM system_settings WHERE key = $1;
