-- name: GetVersionStamp :one
SELECT key, version, source_hash, updated_at, updated_by
FROM version_stamps
WHERE key = $1;

-- name: UpsertVersionStamp :exec
INSERT INTO version_stamps (key, version, source_hash, updated_at, updated_by)
VALUES ($1, $2, $3, NOW(), $4)
ON CONFLICT (key) DO UPDATE SET
    version = EXCLUDED.version,
    source_hash = EXCLUDED.source_hash,
    updated_at = NOW(),
    updated_by = EXCLUDED.updated_by;

-- name: DeleteVersionStamp :exec
DELETE FROM version_stamps WHERE key = $1;
