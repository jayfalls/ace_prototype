-- name: ListSpans :many
-- Lists trace spans with optional filters and pagination.
SELECT
    id,
    trace_id,
    span_id,
    parent_span_id,
    operation_name,
    service_name,
    start_time,
    end_time,
    duration_ms,
    status,
    attributes,
    created_at
FROM ott_spans
WHERE (? = '' OR service_name = ?)
  AND (? = '' OR operation_name = ?)
  AND (? = '' OR start_time >= ?)
  AND (? = '' OR end_time <= ?)
ORDER BY start_time DESC
LIMIT ? OFFSET ?;

-- name: ListSpansCount :one
-- Counts spans matching the filter criteria.
SELECT COUNT(*) AS count
FROM ott_spans
WHERE (? = '' OR service_name = ?)
  AND (? = '' OR operation_name = ?)
  AND (? = '' OR start_time >= ?)
  AND (? = '' OR end_time <= ?);

-- name: GetSpanByID :one
-- Gets a single span by ID.
SELECT
    id,
    trace_id,
    span_id,
    parent_span_id,
    operation_name,
    service_name,
    start_time,
    end_time,
    duration_ms,
    status,
    attributes,
    created_at
FROM ott_spans
WHERE id = ?;

-- name: GetSpanByTraceID :many
-- Gets all spans for a given trace ID.
SELECT
    id,
    trace_id,
    span_id,
    parent_span_id,
    operation_name,
    service_name,
    start_time,
    end_time,
    duration_ms,
    status,
    attributes,
    created_at
FROM ott_spans
WHERE trace_id = ?
ORDER BY start_time ASC;

-- name: CountSpansLastHour :one
-- Counts spans created in the last hour.
SELECT COUNT(*) AS count
FROM ott_spans
WHERE created_at >= datetime('now', '-1 hour');
