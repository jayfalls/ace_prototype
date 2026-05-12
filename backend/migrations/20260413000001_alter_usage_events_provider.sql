-- +goose Up
-- +goose StatementBegin
ALTER TABLE usage_events ADD COLUMN provider_id TEXT;
ALTER TABLE usage_events ADD COLUMN provider_group_id TEXT;
ALTER TABLE usage_events ADD COLUMN cached_tokens INTEGER DEFAULT 0;
ALTER TABLE usage_events ADD COLUMN retry_count INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_usage_events_provider_id ON usage_events(provider_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_provider_group_id ON usage_events(provider_group_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite does not support DROP COLUMN; indexes are dropped.
DROP INDEX IF EXISTS idx_usage_events_provider_id;
DROP INDEX IF EXISTS idx_usage_events_provider_group_id;
-- +goose StatementEnd
