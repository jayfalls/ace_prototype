# Functional Specification Document

## Overview
Define the data model, API structure, and type-safe database access for the ACE Framework MVP.

## Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Database | PostgreSQL | Primary data store for all entities |
| SQL Generator | SQLC | Type-safe SQL at compile time, no ORM |
| API | Gin | Fast HTTP framework with WebSocket support |
| Auth | JWT | Stateless token-based authentication |
| Real-time | WebSocket | Thought streaming, real-time updates |

## Data Model

### Core Entities

#### Agent
```sql
-- Represents an autonomous cognitive entity
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    config JSONB DEFAULT '{}',
    status TEXT DEFAULT 'idle', -- idle, running, paused
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Memory
```sql
-- Long-term memory storage with tree structure
CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id),
    parent_id UUID REFERENCES memories(id), -- tree structure
    content TEXT NOT NULL,
    summary TEXT, -- summarized version
    tags JSONB DEFAULT '[]',
    memory_type TEXT NOT NULL, -- experience, fact, pattern
    importance INTEGER DEFAULT 5, -- 1-10
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Session
```sql
-- User-agent interaction sessions
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id),
    user_id UUID NOT NULL,
    status TEXT DEFAULT 'active', -- active, completed
    context JSONB DEFAULT '{}',
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP
);
```

#### Thought
```sql
-- Individual thought records for debugging/traceability
CREATE TABLE thoughts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES sessions(id),
    agent_id UUID REFERENCES agents(id),
    layer INTEGER NOT NULL, -- 1-6
    cycle INTEGER NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### User
```sql
-- User accounts
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    name TEXT,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### LLMProvider
```sql
-- LLM provider configurations (OpenAI, Anthropic, Ollama, etc.)
CREATE TABLE llm_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL, -- openai, anthropic, ollama
    api_key TEXT, -- encrypted
    base_url TEXT, -- for custom endpoints
    default_model TEXT,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### LLMAttachment
```sql
-- Which LLM is attached to which layer/component
CREATE TABLE llm_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id),
    provider_id UUID REFERENCES llm_providers(id),
    target_type TEXT NOT NULL, -- layer, global_loop, task_prosecution
    target_id TEXT NOT NULL, -- layer number or loop name
    model TEXT NOT NULL,
    config JSONB DEFAULT '{}', -- temperature, max_tokens, etc.
    priority INTEGER DEFAULT 0, -- which attachment to use first
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### AgentSetting
```sql
-- Agent-specific settings
CREATE TABLE agent_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id),
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### SystemSetting
```sql
-- Global system settings
CREATE TABLE system_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT UNIQUE NOT NULL,
    value JSONB NOT NULL,
    description TEXT,
    is_secret BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Tool Sources

Tools are dynamically loaded from multiple sources, not stored in DB:
- **Hardcoded Tools**: Built-in tools (filesystem, HTTP, etc.)
- **MCP Servers**: Model Context Protocol servers
- **Anthropic Skills**: Anthropic skill definitions

### AgentToolWhitelist
```sql
-- Whitelist of tools available to specific agents
-- Tools are loaded dynamically from sources (hardcoded, MCP, skills)
-- This table stores which tools are enabled for each agent
CREATE TABLE agent_tool_whitelist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id),
    tool_source TEXT NOT NULL, -- hardcoded, mcp, skill
    tool_name TEXT NOT NULL, -- tool identifier from source
    enabled BOOLEAN DEFAULT TRUE,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(agent_id, tool_source, tool_name)
);
```

## API Structure

### REST Endpoints

#### Agents
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/agents | List all agents |
| POST | /api/agents | Create new agent |
| GET | /api/agents/:id | Get agent details |
| PUT | /api/agents/:id | Update agent |
| DELETE | /api/agents/:id | Delete agent |
| POST | /api/agents/:id/start | Start agent |
| POST | /api/agents/:id/stop | Stop agent |

#### Memories
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/agents/:id/memories | List agent memories |
| POST | /api/agents/:id/memories | Create memory |
| GET | /api/agents/:id/memories/:mem_id | Get memory |
| PUT | /api/agents/:id/memories/:mem_id | Update memory |
| DELETE | /api/agents/:id/memories/:mem_id | Delete memory |
| GET | /api/agents/:id/memories/search | Search memories by tags |

#### Sessions
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/sessions | List sessions |
| POST | /api/sessions | Create session |
| GET | /api/sessions/:id | Get session |
| POST | /api/sessions/:id/end | End session |

#### Thoughts
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/sessions/:id/thoughts | Get thought trace |
| GET | /api/agents/:id/thoughts | Get agent thoughts |

#### Users
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/auth/register | Register user |
| POST | /api/auth/login | Login user |
| GET | /api/users/me | Get current user |

#### LLM Providers
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/llm-providers | List LLM providers |
| POST | /api/llm-providers | Create provider |
| GET | /api/llm-providers/:id | Get provider |
| PUT | /api/llm-providers/:id | Update provider |
| DELETE | /api/llm-providers/:id | Delete provider |

#### LLM Attachments
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/agents/:id/llm-attachments | List agent's LLM attachments |
| POST | /api/agents/:id/llm-attachments | Create attachment |
| GET | /api/agents/:id/llm-attachments/:att_id | Get attachment |
| PUT | /api/agents/:id/llm-attachments/:att_id | Update attachment |
| DELETE | /api/agents/:id/llm-attachments/:att_id | Delete attachment |

#### Settings
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/agents/:id/settings | Get agent settings |
| PUT | /api/agents/:id/settings | Update agent settings |
| GET | /api/settings | Get system settings (admin) |
| PUT | /api/settings | Update system settings (admin) |

#### Tools (Whitelist)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/agents/:id/tools | List agent's tool whitelist |
| PUT | /api/agents/:id/tools | Update agent's tool whitelist |
| GET | /api/tools/sources | Get available tool sources (hardcoded, MCP, skills) |

### WebSocket Messages

#### Client → Server
| Event | Payload | Description |
|-------|---------|-------------|
| `agent.message` | `{ agent_id, message }` | Send message to agent |
| `agent.subscribe` | `{ agent_id }` | Subscribe to agent updates |

#### Server → Client
| Event | Payload | Description |
|-------|---------|-------------|
| `thought.update` | `{ session_id, layer, content }` | Real-time thought |
| `thought.complete` | `{ session_id, final }` | Thought complete |
| `agent.status` | `{ agent_id, status }` | Agent status change |

## Type Safety (SQLC)

### Query Organization
```
sqlc.yaml
db/
  query/
    agents.sql
    memories.sql
    sessions.sql
    thoughts.sql
    users.sql
  schema/
    001_initial.sql
  models/
    *.sql.go (generated)
```

### Generated Types
- `Agent` - Agent model
- `Memory` - Memory model  
- `Session` - Session model
- `Thought` - Thought model
- `User` - User model

## Authentication

### JWT Flow
1. User logs in → receives JWT token
2. Token includes: `user_id`, `exp`, `roles`
3. Middleware validates token on protected routes
4. Token refresh before expiration

### Protected Routes
- `/api/agents/*` - Requires auth
- `/api/sessions/*` - Requires auth
- `/api/memories/*` - Requires auth
- `/ws/*` - Requires auth

## Data Validation

### Request Validation
- All endpoints validate input JSON
- UUID format validation
- String length limits
- JSONB schema validation for config fields

### Sanitization
- SQL injection prevention via SQLC (parameterized queries)
- XSS prevention on user inputs
- Rate limiting (future)

## Out of Scope
- Frontend implementation (separate unit)
- Deployment configuration (deployment unit)
- LLM provider integrations (separate unit)
- Detailed monitoring setup (monitoring unit)
