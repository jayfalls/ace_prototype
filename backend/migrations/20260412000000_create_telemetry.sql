-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ott_spans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trace_id TEXT NOT NULL,
    span_id TEXT NOT NULL,
    parent_span_id TEXT,
    operation_name TEXT NOT NULL,
    service_name TEXT NOT NULL,
    start_time TEXT NOT NULL,
    end_time TEXT NOT NULL,
    duration_ms INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'ok',
    attributes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ott_spans_trace_id ON ott_spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_ott_spans_service ON ott_spans(service_name);
CREATE INDEX IF NOT EXISTS idx_ott_spans_created_at ON ott_spans(created_at);

CREATE TABLE IF NOT EXISTS ott_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'counter',
    labels TEXT,
    value REAL NOT NULL,
    timestamp TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ott_metrics_name ON ott_metrics(name);
CREATE INDEX IF NOT EXISTS idx_ott_metrics_created_at ON ott_metrics(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_ott_metrics_name;
DROP INDEX IF EXISTS idx_ott_metrics_created_at;
DROP TABLE IF EXISTS ott_metrics;
DROP INDEX IF EXISTS idx_ott_spans_trace_id;
DROP INDEX IF EXISTS idx_ott_spans_service;
DROP INDEX IF EXISTS idx_ott_spans_created_at;
DROP TABLE IF EXISTS ott_spans;
-- +goose StatementEnd
