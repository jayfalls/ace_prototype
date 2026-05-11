-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS providers (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL UNIQUE,
    provider_type       TEXT NOT NULL,
    base_url            TEXT NOT NULL,
    encrypted_api_key   BLOB NOT NULL,
    api_key_nonce       BLOB NOT NULL,
    encrypted_dek       BLOB NOT NULL,
    dek_nonce           BLOB NOT NULL,
    encryption_version  INTEGER NOT NULL DEFAULT 1,
    config_json         TEXT NOT NULL DEFAULT '{}',
    is_enabled          INTEGER NOT NULL DEFAULT 1,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_providers_type ON providers(provider_type);
CREATE INDEX IF NOT EXISTS idx_providers_enabled ON providers(is_enabled);

CREATE TABLE IF NOT EXISTS provider_models (
    id                  TEXT PRIMARY KEY,
    provider_id         TEXT NOT NULL,
    model_id            TEXT NOT NULL,
    display_name        TEXT NOT NULL,
    context_limit       INTEGER,
    features_json       TEXT NOT NULL DEFAULT '{}',
    pricing_json        TEXT NOT NULL DEFAULT '{}',
    parameters_json     TEXT NOT NULL DEFAULT '{}',
    is_user_edited      INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);
CREATE INDEX IF NOT EXISTS idx_provider_models_provider_id ON provider_models(provider_id);

CREATE TABLE IF NOT EXISTS provider_groups (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    strategy        TEXT NOT NULL,
    config_json     TEXT NOT NULL DEFAULT '{}',
    is_default      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_provider_groups_default ON provider_groups(is_default);

CREATE TABLE IF NOT EXISTS provider_group_members (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    priority    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (group_id) REFERENCES provider_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    UNIQUE(group_id, provider_id)
);
CREATE INDEX IF NOT EXISTS idx_provider_group_members_group_id ON provider_group_members(group_id);
CREATE INDEX IF NOT EXISTS idx_provider_group_members_priority ON provider_group_members(group_id, priority);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS provider_group_members;
DROP TABLE IF EXISTS provider_models;
DROP TABLE IF EXISTS provider_groups;
DROP TABLE IF EXISTS providers;
-- +goose StatementEnd
