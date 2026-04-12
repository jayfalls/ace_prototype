-- Schema definitions for sqlc query validation.
-- These tables are created via Goose migrations in the migrations/ directory.
-- This file is used by sqlc for schema inference only.

-- =============================================================================
-- version_stamps table (cache invalidation by version)
-- =============================================================================
CREATE TABLE version_stamps (
    key VARCHAR(512) PRIMARY KEY,
    version VARCHAR(255) NOT NULL,
    source_hash VARCHAR(64),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255)
);

-- =============================================================================
-- users table (user accounts with authentication credentials and roles)
-- =============================================================================
CREATE TABLE users (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email            VARCHAR(255) NOT NULL UNIQUE,
    password_hash    VARCHAR(255) NOT NULL,
    role             VARCHAR(20) NOT NULL DEFAULT 'user' 
                     CHECK (role IN ('admin', 'user', 'viewer')),
    status           VARCHAR(30) NOT NULL DEFAULT 'pending' 
                     CHECK (status IN ('pending', 'active', 'suspended')),
    suspended_at    TIMESTAMPTZ,
    suspended_reason VARCHAR(255),
    deleted_at      TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status) WHERE deleted_at IS NULL;

-- =============================================================================
-- sessions table (active user sessions with refresh tokens)
-- =============================================================================
CREATE TABLE sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) NOT NULL,
    user_agent          VARCHAR(512),
    ip_address          INET,
    last_used_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- =============================================================================
-- auth_tokens table (auth tokens for magic links and password reset)
-- =============================================================================
CREATE TABLE auth_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_type  VARCHAR(30) NOT NULL 
                CHECK (token_type IN ('login', 'verification', 'password_reset')),
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens(token_hash);
CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens(expires_at);

-- =============================================================================
-- resource_permissions table (resource-level permissions for fine-grained access control)
-- =============================================================================
CREATE TABLE resource_permissions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_type   VARCHAR(50) NOT NULL,
    resource_id     UUID NOT NULL,
    permission_level VARCHAR(20) NOT NULL 
                    CHECK (permission_level IN ('view', 'use', 'admin')),
    granted_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(user_id, resource_type, resource_id)
);

CREATE INDEX idx_resource_permissions_user_id ON resource_permissions(user_id);
CREATE INDEX idx_resource_permissions_resource ON resource_permissions(resource_type, resource_id);
