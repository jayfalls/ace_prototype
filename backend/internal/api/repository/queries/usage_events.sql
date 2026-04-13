-- name: ListUsageEvents :many
-- Lists usage events with optional filters and pagination.
SELECT
    id,
    agent_id,
    session_id,
    event_type,
    model,
    input_tokens,
    output_tokens,
    cost_usd,
    duration_ms,
    metadata,
    created_at
FROM usage_events
WHERE (? = '' OR agent_id = ?)
  AND (? = '' OR event_type = ?)
  AND (? = '' OR created_at >= ?)
  AND (? = '' OR created_at <= ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListUsageEventsCount :one
-- Counts usage events matching the filter criteria.
SELECT COUNT(*) AS count
FROM usage_events
WHERE (? = '' OR agent_id = ?)
  AND (? = '' OR event_type = ?)
  AND (? = '' OR created_at >= ?)
  AND (? = '' OR created_at <= ?);

-- name: GetUsageEventByID :one
-- Gets a single usage event by ID.
SELECT
    id,
    agent_id,
    session_id,
    event_type,
    model,
    input_tokens,
    output_tokens,
    cost_usd,
    duration_ms,
    metadata,
    created_at
FROM usage_events
WHERE id = ?;

-- name: GetUsageEventsByAgentID :many
-- Gets all usage events for a given agent ID.
SELECT
    id,
    agent_id,
    session_id,
    event_type,
    model,
    input_tokens,
    output_tokens,
    cost_usd,
    duration_ms,
    metadata,
    created_at
FROM usage_events
WHERE agent_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetUsageEventsBySessionID :many
-- Gets all usage events for a given session ID.
SELECT
    id,
    agent_id,
    session_id,
    event_type,
    model,
    input_tokens,
    output_tokens,
    cost_usd,
    duration_ms,
    metadata,
    created_at
FROM usage_events
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountUsageEventsLastHour :one
-- Counts usage events created in the last hour.
SELECT COUNT(*) AS count
FROM usage_events
WHERE created_at >= datetime('now', '-1 hour');

-- name: SumUsageCosts :one
-- Sums total usage costs for a given agent and time range.
SELECT 
    COALESCE(SUM(cost_usd), 0) as total_cost,
    COALESCE(SUM(input_tokens), 0) as total_input_tokens,
    COALESCE(SUM(output_tokens), 0) as total_output_tokens,
    COUNT(*) as event_count
FROM usage_events
WHERE agent_id = ?
  AND (? = '' OR event_type = ?)
  AND created_at >= ?
  AND created_at <= ?;
