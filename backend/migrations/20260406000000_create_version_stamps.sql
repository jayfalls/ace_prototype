-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS version_stamps (
    key         TEXT PRIMARY KEY,
    version     TEXT NOT NULL,
    source_hash TEXT,
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_by  TEXT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS version_stamps;
-- +goose StatementEnd
