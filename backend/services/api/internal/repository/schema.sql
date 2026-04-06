-- Schema for version_stamps table (cache invalidation by version).
-- This file is used by sqlc for schema inference.
-- The actual migration is in 20260406000000_create_version_stamps.go (Goose).

CREATE TABLE version_stamps (
    key VARCHAR(512) PRIMARY KEY,
    version VARCHAR(255) NOT NULL,
    source_hash VARCHAR(64),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255)
);
