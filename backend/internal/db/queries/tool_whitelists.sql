-- name: GetToolWhitelist :one
SELECT * FROM agent_tool_whitelists WHERE agent_id = $1 AND tool_name = $2;

-- name: ListToolWhitelists :many
SELECT * FROM agent_tool_whitelists WHERE agent_id = $1;

-- name: UpsertToolWhitelist :one
INSERT INTO agent_tool_whitelists (agent_id, tool_name, enabled, config)
VALUES ($1, $2, $3, $4)
ON CONFLICT (agent_id, tool_name) 
DO UPDATE SET enabled = $3, config = $4, updated_at = NOW()
RETURNING *;

-- name: DeleteToolWhitelist :exec
DELETE FROM agent_tool_whitelists WHERE agent_id = $1 AND tool_name = $2;
