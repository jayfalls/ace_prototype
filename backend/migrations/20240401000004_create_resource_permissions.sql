-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS resource_permissions (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL,
    resource_type    TEXT NOT NULL,
    resource_id      TEXT NOT NULL,
    permission_level TEXT NOT NULL,
    granted_by       TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_resource_permissions_user_id ON resource_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_resource_permissions_resource ON resource_permissions(resource_type, resource_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resource_permissions_user_id;
DROP INDEX IF EXISTS idx_resource_permissions_resource;
DROP TABLE IF EXISTS resource_permissions;
-- +goose StatementEnd
