-- name: GetVersionStamp :one
SELECT key, version, source_hash, updated_at, updated_by
FROM version_stamps
WHERE key = ?;

-- name: UpsertVersionStamp :exec
INSERT INTO version_stamps (key, version, source_hash, updated_at, updated_by)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
    version = excluded.version,
    source_hash = excluded.source_hash,
    updated_at = excluded.updated_at,
    updated_by = excluded.updated_by;

-- name: DeleteVersionStamp :exec
DELETE FROM version_stamps WHERE key = ?;
