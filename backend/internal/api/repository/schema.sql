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
-- users table (user accounts with authentication credentials and roles)
-- =============================================================================
CREATE TABLE IF NOT EXISTS users (
    id               TEXT PRIMARY KEY,
    email            TEXT NOT NULL UNIQUE,
    password_hash    TEXT NOT NULL,
    role             TEXT NOT NULL DEFAULT 'user',
    status           TEXT NOT NULL DEFAULT 'pending',
    suspended_at     TEXT,
    suspended_reason TEXT,
    deleted_at       TEXT,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

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
-- auth_tokens table (auth tokens for magic links and password reset)
-- =============================================================================
CREATE TABLE IF NOT EXISTS auth_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    token_type  TEXT NOT NULL,
    token_hash  TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    used_at     TEXT,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_token_hash ON auth_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires_at ON auth_tokens(expires_at);

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
