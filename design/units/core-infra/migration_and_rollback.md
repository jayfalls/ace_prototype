# Migration and Rollback

<!--
Intent: Define database schema changes and rollback procedures.
Scope: All migrations needed, rollback strategies, and data migration scripts.
Used by: AI agents to safely modify the database schema and recover from failures.
-->

## Overview

This document defines the database schema migrations for the ACE Framework MVP core-infra unit. All migrations use golang-migrate for version control and include proper rollback procedures.

## Migrations

### Migration 1: Create Users Table
**Direction**: UP
**Description**: Create the users table for authentication

```sql
-- Up Migration
-- Create users table

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(30) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
```

```sql
-- Down Migration
DROP TABLE users;
```

### Migration 2: Create Agents Table
**Direction**: UP
**Description**: Create the agents table

```sql
-- Up Migration
-- Create agents table

CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    config JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'inactive',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_agents_owner_id ON agents(owner_id);
CREATE INDEX idx_agents_status ON agents(status);
```

```sql
-- Down Migration
DROP TABLE agents;
```

### Migration 3: Create Sessions Table
**Direction**: UP
**Description**: Create the sessions table

```sql
-- Up Migration
-- Create sessions table

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP,
    metadata JSONB DEFAULT '{}'
);

-- Add indexes
CREATE INDEX idx_sessions_agent_id ON sessions(agent_id);
CREATE INDEX idx_sessions_owner_id ON sessions(owner_id);
CREATE INDEX idx_sessions_status ON sessions(status);
```

```sql
-- Down Migration
DROP TABLE sessions;
```

### Migration 4: Create Thoughts Table
**Direction**: UP
**Description**: Create the thoughts table for tracking agent reasoning

```sql
-- Up Migration
-- Create thoughts table

CREATE TABLE thoughts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    layer VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_thoughts_session_id ON thoughts(session_id);
CREATE INDEX idx_thoughts_layer ON thoughts(layer);
CREATE INDEX idx_thoughts_created_at ON thoughts(created_at);
```

```sql
-- Down Migration
DROP TABLE thoughts;
```

### Migration 5: Create Memories Table
**Direction**: UP
**Description**: Create the memories table for long-term storage

```sql
-- Up Migration
-- Create memories table

CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    memory_type VARCHAR(50) NOT NULL,
    parent_id UUID REFERENCES memories(id) ON DELETE SET NULL,
    tags TEXT[],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_memories_owner_id ON memories(owner_id);
CREATE INDEX idx_memories_agent_id ON memories(agent_id);
CREATE INDEX idx_memories_parent_id ON memories(parent_id);
CREATE INDEX idx_memories_memory_type ON memories(memory_type);
CREATE INDEX idx_memories_tags ON memories USING GIN(tags);
```

```sql
-- Down Migration
DROP TABLE memories;
```

### Migration 6: Create LLM Providers Table
**Direction**: UP
**Description**: Create the llm_providers table

```sql
-- Up Migration
-- Create llm_providers table

CREATE TABLE llm_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    api_key_encrypted VARCHAR(512),
    base_url VARCHAR(512),
    model VARCHAR(100),
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_llm_providers_owner_id ON llm_providers(owner_id);
CREATE INDEX idx_llm_providers_provider_type ON llm_providers(provider_type);
```

```sql
-- Down Migration
DROP TABLE llm_providers;
```

### Migration 7: Create LLM Attachments Table
**Direction**: UP
**Description**: Link LLM providers to agents/layers

```sql
-- Up Migration
-- Create llm_attachments table

CREATE TABLE llm_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES llm_providers(id) ON DELETE CASCADE,
    layer VARCHAR(50) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_llm_attachments_agent_id ON llm_attachments(agent_id);
CREATE INDEX idx_llm_attachments_provider_id ON llm_attachments(provider_id);
CREATE INDEX idx_llm_attachments_layer ON llm_attachments(layer);
```

```sql
-- Down Migration
DROP TABLE llm_attachments;
```

### Migration 8: Create Settings Tables
**Direction**: UP
**Description**: Create agent_settings and system_settings tables

```sql
-- Up Migration
-- Create agent_settings table

CREATE TABLE agent_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(agent_id, key)
);

CREATE INDEX idx_agent_settings_agent_id ON agent_settings(agent_id);

-- Create system_settings table

CREATE TABLE system_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(100) UNIQUE NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_system_settings_key ON system_settings(key);
```

```sql
-- Down Migration
DROP TABLE agent_settings;
DROP TABLE system_settings;
```

### Migration 9: Create Tool Whitelist Table
**Direction**: UP
**Description**: Create the agent_tool_whitelists table

```sql
-- Up Migration
-- Create agent_tool_whitelists table

CREATE TABLE agent_tool_whitelists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tool_name VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(agent_id, tool_name)
);

-- Add indexes
CREATE INDEX idx_tool_whitelists_agent_id ON agent_tool_whitelists(agent_id);
```

```sql
-- Down Migration
DROP TABLE agent_tool_whitelists;
```

## Data Migration

No initial data migrations required for MVP. All tables start empty.

## Rollback Strategy

### Primary Rollback

| Step | Action | Command |
|------|--------|---------|
| 1 | Rollback last migration | `make migrate-down` |
| 2 | Verify schema | `make migrate-status` |
| 3 | Run tests | `make test` |

### Automatic Rollback
- **Tool**: golang-migrate
- **Command**: `make migrate-down` or `migrate -path db/migrations -database "postgres://..." down`

### Manual Rollback Procedures
If automatic rollback fails:
1. Connect to database
2. Run down migration SQL manually
3. Verify table state with `\dt`

## Pre-Migration Checklist
- [x] Backup database (pg_dump)
- [ ] Test migration on staging
- [x] Verify sufficient disk space
- [x] Check for locks
- [ ] Notify users of downtime (if applicable)

## Post-Migration Checklist
- [x] Verify data integrity
- [ ] Run application tests
- [x] Check logs for errors
- [x] Verify performance
- [x] Update documentation

## Migration Dependencies

| Migration | Depends On |
|-----------|-----------|
| agents | users |
| sessions | agents, users |
| thoughts | sessions |
| memories | users, agents |
| llm_providers | users |
| llm_attachments | agents, llm_providers |
| agent_settings | agents |
| agent_tool_whitelists | agents |

## Rollback Dependencies

| Migration | Must Be Rolled Back With |
|-----------|-------------------------|
| agents | sessions, memories, llm_attachments, agent_settings, agent_tool_whitelists |
| users | All tables (cascade) |
| sessions | thoughts |
| llm_providers | llm_attachments |
