-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ott_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'counter',
    labels TEXT,
    value REAL NOT NULL,
    timestamp TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ott_metrics_name ON ott_metrics(name);
CREATE INDEX IF NOT EXISTS idx_ott_metrics_created_at ON ott_metrics(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ott_metrics;
-- +goose StatementEnd
