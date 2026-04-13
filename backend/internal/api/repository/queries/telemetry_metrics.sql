-- name: ListMetrics :many
-- Lists metrics with optional filters and pagination.
SELECT
    id,
    name,
    type,
    labels,
    value,
    timestamp,
    created_at
FROM ott_metrics
WHERE (? = '' OR name = ?)
  AND (? = '' OR timestamp >= ?)
  AND (? = '' OR timestamp <= ?)
ORDER BY timestamp DESC
LIMIT ? OFFSET ?;

-- name: ListMetricsCount :one
-- Counts metrics matching the filter criteria.
SELECT COUNT(*) AS count
FROM ott_metrics
WHERE (? = '' OR name = ?)
  AND (? = '' OR timestamp >= ?)
  AND (? = '' OR timestamp <= ?);

-- name: GetMetricByID :one
-- Gets a single metric by ID.
SELECT
    id,
    name,
    type,
    labels,
    value,
    timestamp,
    created_at
FROM ott_metrics
WHERE id = ?;

-- name: GetMetricsByName :many
-- Gets all metrics with a given name.
SELECT
    id,
    name,
    type,
    labels,
    value,
    timestamp,
    created_at
FROM ott_metrics
WHERE name = ?
ORDER BY timestamp DESC
LIMIT ?;

-- name: CountMetricsLastHour :one
-- Counts metrics created in the last hour.
SELECT COUNT(*) AS count
FROM ott_metrics
WHERE created_at >= datetime('now', '-1 hour');

-- name: AggregateMetrics :many
-- Aggregates metrics by name and labels within a time window.
SELECT
    name,
    type,
    labels,
    AVG(value) as value,
    COUNT(*) as sample_count,
    MIN(timestamp) as window_start,
    MAX(timestamp) as window_end
FROM ott_metrics
WHERE (? = '' OR name = ?)
  AND timestamp >= ?
  AND timestamp <= ?
GROUP BY name, labels
ORDER BY name ASC
LIMIT ?;
