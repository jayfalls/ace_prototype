# API Specification

<!--
Intent: Define all API endpoints exposed by the feature.
Scope: REST/GraphQL endpoints, request/response schemas, authentication, and error codes.
Used by: AI agents to implement and consume the API correctly.
-->

## Overview

The core-infra API provides RESTful endpoints for the ACE Framework MVP, including user authentication, agent management, session tracking, thought recording, memory storage, and LLM provider configuration.

## Authentication

| Method | Header | Description |
|--------|--------|-------------|
| Bearer Token | Authorization: Bearer \<token\> | JWT token authentication |
| WebSocket | Query: token=\<token\> | JWT for WebSocket upgrade |

## Base URL

```
Production: https://api.ace-framework.io/v1
Staging: https://api-staging.ace-framework.io/v1
Development: http://localhost:8080/v1
```

## Endpoints

### Resource: Users

#### POST /api/v1/users/register
Register a new user.

**Request Body**
```json
{
  "email": "user@example.com",
  "username": "johndoe",
  "password": "SecurePassword123!"
}
```

**Response 201**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "username": "johndoe",
    "role": "user",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### POST /api/v1/users/login
Authenticate user and receive tokens.

**Request Body**
```json
{
  "email": "user@example.com",
  "password": "SecurePassword123!"
}
```

**Response 200**
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1...",
    "refresh_token": "eyJhbGciOiJIUzI1...",
    "expires_in": 900
  }
}
```

#### POST /api/v1/users/refresh
Refresh access token.

**Request Body**
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1..."
}
```

**Response 200**
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1...",
    "expires_in": 900
  }
}
```

#### GET /api/v1/users/me
Get current user profile.

**Response 200**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "username": "johndoe",
    "role": "user",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### PUT /api/v1/users/me
Update current user profile.

**Request Body**
```json
{
  "email": "newemail@example.com",
  "username": "newusername"
}
```

#### DELETE /api/v1/users/me
Delete current user account.

**Response 204**: No content

---

### Resource: Agents

#### GET /api/v1/agents
List user's agents.

**Query Parameters**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| page | integer | No | 1 | Page number |
| limit | integer | No | 20 | Items per page (max 100) |
| status | string | No | - | Filter by status |

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440001",
      "name": "My Agent",
      "description": "A helpful assistant",
      "status": "inactive",
      "config": {},
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 10,
    "total_pages": 1
  }
}
```

#### POST /api/v1/agents
Create a new agent.

**Request Body**
```json
{
  "name": "My Agent",
  "description": "A helpful assistant",
  "config": {
    "temperature": 0.7,
    "max_tokens": 2048
  }
}
```

**Response 201**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "owner_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "My Agent",
    "description": "A helpful assistant",
    "status": "inactive",
    "config": {
      "temperature": 0.7,
      "max_tokens": 2048
    },
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

#### GET /api/v1/agents/{id}
Get agent by ID.

**Response 200**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "owner_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "My Agent",
    "description": "A helpful assistant",
    "status": "inactive",
    "config": {},
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

#### PUT /api/v1/agents/{id}
Update agent.

**Request Body**
```json
{
  "name": "Updated Name",
  "description": "Updated description",
  "config": {
    "temperature": 0.8
  }
}
```

#### DELETE /api/v1/agents/{id}
Delete agent.

**Response 204**: No content

---

### Resource: Sessions

#### GET /api/v1/agents/{agent_id}/sessions
List sessions for an agent.

**Query Parameters**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| page | integer | No | 1 | Page number |
| limit | integer | No | 20 | Items per page |
| status | string | No | - | Filter by status |

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440002",
      "agent_id": "550e8400-e29b-41d4-a716-446655440001",
      "status": "active",
      "started_at": "2024-01-01T00:00:00Z",
      "ended_at": null,
      "metadata": {}
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 5,
    "total_pages": 1
  }
}
```

#### POST /api/v1/agents/{agent_id}/sessions
Start a new session.

**Response 201**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "agent_id": "550e8400-e29b-41d4-a716-446655440001",
    "owner_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "active",
    "started_at": "2024-01-01T00:00:00Z",
    "ended_at": null,
    "metadata": {}
  }
}
```

#### GET /api/v1/sessions/{id}
Get session by ID.

**Response 200**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "agent_id": "550e8400-e29b-41d4-a716-446655440001",
    "owner_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "active",
    "started_at": "2024-01-01T00:00:00Z",
    "ended_at": null,
    "metadata": {}
  }
}
```

#### DELETE /api/v1/sessions/{id}
End a session.

**Request Body**
```json
{
  "status": "completed"
}
```

---

### Resource: Thoughts

#### GET /api/v1/sessions/{session_id}/thoughts
List thoughts for a session.

**Query Parameters**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| layer | string | No | - | Filter by layer |
| page | integer | No | 1 | Page number |
| limit | integer | No | 100 | Items per page |

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440003",
      "session_id": "550e8400-e29b-41d4-a716-446655440002",
      "layer": "perception",
      "content": "Processing user input...",
      "metadata": {},
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 100,
    "total": 50,
    "total_pages": 1
  }
}
```

#### POST /api/v1/sessions/{session_id}/thoughts
Record a thought (internal API).

**Request Body**
```json
{
  "layer": "perception",
  "content": "Processing user input...",
  "metadata": {
    "tokens_used": 100
  }
}
```

---

### Resource: Memories

#### GET /api/v1/memories
List user's memories.

**Query Parameters**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| page | integer | No | 1 | Page number |
| limit | integer | No | 20 | Items per page |
| type | string | No | - | Filter by memory_type |
| tags | string | No | - | Filter by tags (comma-separated) |
| search | string | No | - | Search in content |

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440004",
      "owner_id": "550e8400-e29b-41d4-a716-446655440000",
      "agent_id": "550e8400-e29b-41d4-a716-446655440001",
      "content": "User prefers detailed responses",
      "memory_type": "preference",
      "parent_id": null,
      "tags": ["preference", "user"],
      "metadata": {},
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

#### POST /api/v1/memories
Create a memory.

**Request Body**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440001",
  "content": "User prefers detailed responses",
  "memory_type": "preference",
  "parent_id": null,
  "tags": ["preference", "user"]
}
```

#### GET /api/v1/memories/{id}
Get memory by ID.

#### PUT /api/v1/memories/{id}
Update memory.

**Request Body**
```json
{
  "content": "Updated content",
  "tags": ["updated", "tags"]
}
```

#### DELETE /api/v1/memories/{id}
Delete memory.

**Response 204**: No content

---

### Resource: LLM Providers

#### GET /api/v1/llm-providers
List user's LLM providers.

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440005",
      "owner_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "OpenAI",
      "provider_type": "openai",
      "api_key_encrypted": "***",
      "base_url": "https://api.openai.com/v1",
      "model": "gpt-4",
      "config": {},
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### POST /api/v1/llm-providers
Create an LLM provider.

**Request Body**
```json
{
  "name": "OpenAI",
  "provider_type": "openai",
  "api_key": "sk-...",
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4",
  "config": {
    "organization": "org-..."
  }
}
```

#### GET /api/v1/llm-providers/{id}
Get LLM provider by ID.

#### PUT /api/v1/llm-providers/{id}
Update LLM provider.

**Request Body**
```json
{
  "name": "Updated OpenAI",
  "model": "gpt-4-turbo"
}
```

#### DELETE /api/v1/llm-providers/{id}
Delete LLM provider.

**Response 204**: No content

---

### Resource: LLM Attachments

#### GET /api/v1/agents/{agent_id}/llm-attachments
List LLM attachments for an agent.

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440006",
      "agent_id": "550e8400-e29b-41d4-a716-446655440001",
      "provider_id": "550e8400-e29b-41d4-a716-446655440005",
      "layer": "cognition",
      "priority": 1,
      "config": {},
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### POST /api/v1/agents/{agent_id}/llm-attachments
Attach an LLM provider to an agent.

**Request Body**
```json
{
  "provider_id": "550e8400-e29b-41d4-a716-446655440005",
  "layer": "cognition",
  "priority": 1
}
```

#### DELETE /api/v1/llm-attachments/{id}
Remove LLM attachment.

---

### Resource: Settings

#### GET /api/v1/agents/{agent_id}/settings
List settings for an agent.

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440007",
      "agent_id": "550e8400-e29b-41d4-a716-446655440001",
      "key": "temperature",
      "value": "0.7",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### PUT /api/v1/agents/{agent_id}/settings
Upsert agent settings.

**Request Body**
```json
{
  "temperature": "0.8",
  "max_tokens": "4096"
}
```

#### GET /api/v1/system-settings
List system settings (admin only).

#### PUT /api/v1/system-settings
Update system settings (admin only).

---

### Resource: Tool Whitelists

#### GET /api/v1/agents/{agent_id}/tools
List tool whitelist for an agent.

**Response 200**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440008",
      "agent_id": "550e8400-e29b-41d4-a716-446655440001",
      "tool_name": "web_search",
      "enabled": true,
      "config": {},
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### PUT /api/v1/agents/{agent_id}/tools
Update tool whitelist.

**Request Body**
```json
{
  "tools": [
    {
      "tool_name": "web_search",
      "enabled": true
    },
    {
      "tool_name": "code_executor",
      "enabled": false
    }
  ]
}
```

---

### Resource: WebSocket

#### GET /api/v1/ws
WebSocket endpoint for real-time communication.

**Query Parameters**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| token | string | Yes | JWT authentication token |
| session_id | string | Yes | Session ID to connect to |

**WebSocket Messages (Client → Server)**
```json
{
  "type": "message",
  "content": "Hello agent!"
}
```

```json
{
  "type": "thought",
  "layer": "perception",
  "content": "Processing input..."
}
```

**WebSocket Messages (Server → Client)**
```json
{
  "type": "message",
  "content": "Hello! How can I help?",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

```json
{
  "type": "thought",
  "layer": "cognition",
  "content": "Thinking about response...",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

```json
{
  "type": "error",
  "message": "Error description"
}
```

---

### Resource: Health

#### GET /health
Health check endpoint.

**Response 200**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

#### GET /health/ready
Readiness check endpoint.

**Response 200**
```json
{
  "ready": true,
  "checks": {
    "database": true,
    "migrations": true
  }
}
```

---

## Error Responses

| Status Code | Code | Description |
|-------------|------|-------------|
| 400 | VALIDATION_ERROR | Invalid request body or parameters |
| 401 | UNAUTHORIZED | Missing or invalid token |
| 403 | FORBIDDEN | Insufficient permissions |
| 404 | NOT_FOUND | Resource not found |
| 409 | CONFLICT | Resource already exists |
| 422 | VALIDATION_ERROR | Business logic validation failed |
| 429 | RATE_LIMITED | Too many requests |
| 500 | INTERNAL_ERROR | Server error |

**Error Response Format**
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Email is required",
    "details": [
      {
        "field": "email",
        "message": "Email is required"
      }
    ]
  }
}
```

## Rate Limiting

- **Limit**: 100 requests per minute per user
- **Headers**: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset

## Versioning

- **Current Version**: v1
- **Version in URL**: Yes (/v1/)
- **Deprecation Policy**: 6 months notice
