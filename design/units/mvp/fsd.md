# Functional Specification Document

## Overview
Complete functional specification for ACE Framework MVP implementation.

## API Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/v1/auth/register | Register new user |
| POST | /api/v1/auth/login | Login, returns JWT |
| POST | /api/v1/auth/refresh | Refresh JWT token |
| GET | /api/v1/auth/me | Get current user |

### Agents
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/agents | List all agents |
| POST | /api/v1/agents | Create agent |
| GET | /api/v1/agents/:id | Get agent details |
| PUT | /api/v1/agents/:id | Update agent |
| DELETE | /api/v1/agents/:id | Delete agent |
| POST | /api/v1/agents/:id/start | Start agent |
| POST | /api/v1/agents/:id/stop | Stop agent |

### Sessions
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/sessions | List sessions |
| POST | /api/v1/sessions | Create session |
| GET | /api/v1/sessions/:id | Get session |
| DELETE | /api/v1/sessions/:id | Delete session |

### Messages (Chat)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/sessions/:id/messages | List messages |
| POST | /api/v1/sessions/:id/messages | Send message |

### Thoughts (Visualizations)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/thoughts | List thoughts (filterable) |
| POST | /api/v1/thoughts/simulate | Simulate thought cycle |

### Memories
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/agents/:id/memories | List memories |
| POST | /api/v1/agents/:id/memories | Create memory |
| GET | /api/v1/agents/:id/memories/:memory_id | Get memory |
| PUT | /api/v1/agents/:id/memories/:memory_id | Update memory |
| DELETE | /api/v1/agents/:id/memories/:memory_id | Delete memory |
| GET | /api/v1/agents/:id/memories/search | Search memories |

### Providers
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/providers | List providers |
| POST | /api/v1/providers | Create provider |
| DELETE | /api/v1/providers/:id | Delete provider |

### Settings
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/agents/:id/settings | Get settings |
| PUT | /api/v1/agents/:id/settings | Update settings |

### Tools
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/v1/tools | List available tools |
| GET | /api/v1/agents/:id/tools | List agent tools |
| POST | /api/v1/agents/:id/tools | Add tool to whitelist |
| DELETE | /api/v1/agents/:id/tools/:tool_id | Remove tool |

### WebSocket
| Path | Description |
|------|-------------|
| /ws/agents/:id | Real-time thought stream |

## Data Models

### User
```go
type User struct {
    ID           string `json:"id"`
    Email        string `json:"email"`
    Name         string `json:"name"`
    PasswordHash string `json:"-"`
    CreatedAt    string `json:"created_at"`
    UpdatedAt    string `json:"updated_at"`
}
```

### Agent
```go
type Agent struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Status      string `json:"status"` // stopped, running, error
    Config      map[string]interface{} `json:"config"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
}
```

### Session
```go
type Session struct {
    ID        string `json:"id"`
    AgentID   string `json:"agent_id"`
    UserID    string `json:"user_id"`
    Status    string `json:"status"` // active, closed
    Context   map[string]interface{} `json:"context"`
    CreatedAt string `json:"created_at"`
}
```

### Message
```go
type Message struct {
    ID        string `json:"id"`
    SessionID string `json:"session_id"`
    Role      string `json:"role"` // user, assistant
    Content   string `json:"content"`
    CreatedAt string `json:"created_at"`
}
```

### Thought
```go
type Thought struct {
    ID        string `json:"id"`
    SessionID string `json:"session_id"`
    Layer     string `json:"layer"` // perception, reasoning, action, reflection
    Cycle     int    `json:"cycle"`
    Content   string `json:"content"`
    Metadata  map[string]interface{} `json:"metadata"`
    CreatedAt string `json:"created_at"`
}
```

### Memory
```go
type Memory struct {
    ID          string `json:"id"`
    AgentID     string `json:"agent_id"`
    ParentID    string `json:"parent_id,omitempty"`
    Content     string `json:"content"`
    Tags        []string `json:"tags"`
    MemoryType  string `json:"memory_type"` // long_term, medium_term, short_term
    Importance  int     `json:"importance"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
}
```

### Provider
```go
type Provider struct {
    ID           string `json:"id"`
    OwnerID      string `json:"owner_id"`
    Name         string `json:"name"`
    ProviderType string `json:"provider_type"` // openai, anthropic, google, azure
    APIKey       string `json:"api_key"`
    BaseURL      string `json:"base_url"`
    Model        string `json:"model"`
    Config       map[string]interface{} `json:"config"`
    CreatedAt    string `json:"created_at"`
    UpdatedAt    string `json:"updated_at"`
}
```

### AgentTool
```go
type AgentTool struct {
    ID          string `json:"id"`
    AgentID     string `json:"agent_id"`
    ToolSource  string `json:"tool_source"`
    ToolName    string `json:"tool_name"`
    Enabled     bool   `json:"enabled"`
    CreatedAt   string `json:"created_at"`
}
```

## Authentication Flow
1. User registers with email/password
2. User logs in, receives JWT (1hr expiry)
3. All API requests include Bearer token
4. Token refresh before expiration

## WebSocket Protocol
- Client connects to /ws/agents/:id
- Server sends thought updates in JSON format
- Client can send control messages (start/stop stream)
- Heartbeat every 30s

## Frontend Pages

| Page | Route | Description |
|------|-------|-------------|
| Login | /login | User authentication |
| Register | /register | New user signup |
| Agents | / | Agent list and management |
| Chat | /chat | Agent interaction |
| Visualizations | /visualizations | Thought visualization |
| Memory | /memory | Memory browser |
| Settings | /settings | Configuration |
