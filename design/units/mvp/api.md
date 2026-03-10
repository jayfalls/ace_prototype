# API Reference

## Authentication Endpoints

### POST /api/v1/auth/register
Register a new user.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123",
  "name": "John Doe"
}
```

**Response (201):**
```json
{
  "data": {
    "id": "user-uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### POST /api/v1/auth/login
Login and receive JWT token.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response (200):**
```json
{
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600
  }
}
```

### POST /api/v1/auth/refresh
Refresh JWT token.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600
  }
}
```

### GET /api/v1/auth/me
Get current user info.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "data": {
    "id": "user-uuid",
    "email": "user@example.com",
    "name": "John Doe"
  }
}
```

## Agent Endpoints

### GET /api/v1/agents
List all agents for current user.

**Headers:** `Authorization: Bearer <token>`

**Response (200):**
```json
{
  "data": [
    {
      "id": "agent-uuid",
      "name": "Assistant",
      "description": "A helpful assistant",
      "status": "running",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

### POST /api/v1/agents
Create a new agent.

**Headers:** `Authorization: Bearer <token>`

**Request:**
```json
{
  "name": "My Agent",
  "description": "Agent description"
}
```

**Response (201):**
```json
{
  "data": {
    "id": "agent-uuid",
    "name": "My Agent",
    "description": "Agent description",
    "status": "stopped",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### GET /api/v1/agents/:id
Get agent details.

**Response (200):**
```json
{
  "data": {
    "id": "agent-uuid",
    "name": "My Agent",
    "description": "Agent description",
    "status": "running",
    "config": {},
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

### PUT /api/v1/agents/:id
Update agent.

**Request:**
```json
{
  "name": "Updated Name",
  "description": "Updated description"
}
```

### DELETE /api/v1/agents/:id
Delete agent.

**Response (204):** No content

### POST /api/v1/agents/:id/start
Start agent.

**Response (200):**
```json
{
  "data": {
    "session_id": "session-uuid",
    "status": "running"
  }
}
```

### POST /api/v1/agents/:id/stop
Stop agent.

**Response (200):**
```json
{
  "data": {
    "status": "stopped"
  }
}
```

## Session Endpoints

### GET /api/v1/sessions
List sessions.

### POST /api/v1/sessions
Create session.

### GET /api/v1/sessions/:id
Get session.

### DELETE /api/v1/sessions/:id
Delete session.

## Message Endpoints

### GET /api/v1/sessions/:id/messages
List messages for session.

### POST /api/v1/sessions/:id/messages
Send message.

**Request:**
```json
{
  "content": "Hello!"
}
```

**Response (201):**
```json
{
  "data": {
    "id": "message-uuid",
    "session_id": "session-uuid",
    "role": "user",
    "content": "Hello!",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

## Thought Endpoints

### GET /api/v1/thoughts
List thoughts (query: session_id).

### POST /api/v1/thoughts/simulate
Simulate thought cycle.

**Request:**
```json
{
  "session_id": "session-uuid"
}
```

## Memory Endpoints

### GET /api/v1/agents/:id/memories
List memories for agent.

### POST /api/v1/agents/:id/memories
Create memory.

**Request:**
```json
{
  "content": "Important information",
  "tags": ["important", "research"],
  "memory_type": "long_term",
  "importance": 8
}
```

### GET /api/v1/agents/:id/memories/:memory_id
Get memory.

### PUT /api/v1/agents/:id/memories/:memory_id
Update memory.

### DELETE /api/v1/agents/:id/memories/:memory_id
Delete memory.

### GET /api/v1/agents/:id/memories/search
Search memories.

**Query params:** q (search query), tags (comma-separated)

## Provider Endpoints

### GET /api/v1/providers
List providers.

### POST /api/v1/providers
Create provider.

### DELETE /api/v1/providers/:id
Delete provider.

## Tool Endpoints

### GET /api/v1/tools
List available tools.

### GET /api/v1/agents/:id/tools
List agent tools.

### POST /api/v1/agents/:id/tools
Add tool to whitelist.

### DELETE /api/v1/agents/:id/tools/:tool_id
Remove tool.

## Settings Endpoints

### GET /api/v1/agents/:id/settings
Get agent settings.

### PUT /api/v1/agents/:id/settings
Update agent settings.

**Request:**
```json
{
  "settings": [
    {"key": "max_tokens", "value": "2048"},
    {"key": "temperature", "value": "0.7"}
  ]
}
```

## WebSocket

### GET /ws/agents/:id
WebSocket connection for real-time thoughts.

**Query param:** token (JWT)

**Messages from server:**
```json
{
  "type": "thought",
  "data": {
    "id": "thought-uuid",
    "layer": "perception",
    "content": "Processing input"
  }
}
```
