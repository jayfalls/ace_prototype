-- Schema definitions for sqlc query validation.
-- These tables are created via Goose migrations in the migrations/ directory.
-- This file is used by sqlc for schema inference only.

-- =============================================================================
-- version_stamps table (cache invalidation by version)
-- =============================================================================
CREATE TABLE IF NOT EXISTS version_stamps (
    key TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    source_hash TEXT,
    updated_at TEXT NOT NULL,
    updated_by TEXT
);

-- =============================================================================
-- users table (user accounts with PIN-based authentication)
-- =============================================================================
CREATE TABLE IF NOT EXISTS users (
    id               TEXT PRIMARY KEY,
    username         TEXT NOT NULL UNIQUE,
    password_hash    TEXT NOT NULL,
    pin_hash         TEXT,
    role             TEXT NOT NULL DEFAULT 'user',
    status           TEXT NOT NULL DEFAULT 'pending',
    suspended_at     TEXT,
    suspended_reason TEXT,
    deleted_at       TEXT,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username IS NOT NULL;

-- =============================================================================
-- sessions table (active user sessions with refresh tokens)
-- =============================================================================
CREATE TABLE IF NOT EXISTS sessions (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL,
    refresh_token_hash  TEXT NOT NULL,
    user_agent          TEXT,
    ip_address          TEXT,
    last_used_at        TEXT NOT NULL,
    expires_at          TEXT NOT NULL,
    created_at          TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- =============================================================================
-- resource_permissions table (resource-level permissions for fine-grained access control)
-- =============================================================================
CREATE TABLE IF NOT EXISTS resource_permissions (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL,
    resource_type    TEXT NOT NULL,
    resource_id      TEXT NOT NULL,
    permission_level TEXT NOT NULL,
    granted_by       TEXT,
    created_at       TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_resource_permissions_user_id ON resource_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_resource_permissions_resource ON resource_permissions(resource_type, resource_id);

-- =============================================================================
-- ott_spans table (OpenTelemetry trace spans)
-- =============================================================================
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
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ott_spans_trace_id ON ott_spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_ott_spans_service ON ott_spans(service_name);
CREATE INDEX IF NOT EXISTS idx_ott_spans_created_at ON ott_spans(created_at);

-- =============================================================================
-- ott_metrics table (OpenTelemetry metrics)
-- =============================================================================
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

-- =============================================================================
-- providers table (provider configurations with encrypted API keys)
-- =============================================================================
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

-- =============================================================================
-- provider_models table (per-provider model metadata and capabilities)
-- =============================================================================
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

-- =============================================================================
-- provider_groups table (groups of providers for routing/load balancing)
-- =============================================================================
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

-- =============================================================================
-- provider_group_members table (membership of providers in groups)
-- =============================================================================
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

-- =============================================================================
-- usage_events table (cost attribution data)
-- =============================================================================
CREATE TABLE IF NOT EXISTS usage_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    model TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cost_usd REAL,
    duration_ms INTEGER,
    metadata TEXT,
    provider_id TEXT,
    provider_group_id TEXT,
    cached_tokens INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_events_agent_id ON usage_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_event_type ON usage_events(event_type);
CREATE INDEX IF NOT EXISTS idx_usage_events_created_at ON usage_events(created_at);
CREATE INDEX IF NOT EXISTS idx_usage_events_provider_id ON usage_events(provider_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_provider_group_id ON usage_events(provider_group_id);
