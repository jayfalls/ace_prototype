# Architecture

## System Overview
```
┌─────────────────────────────────────────────────────────────────┐
│                        ACE MVP Architecture                      │
│                                                                  │
│  ┌──────────────┐     ┌──────────────┐     ┌───────────────┐  │
│  │   Frontend   │     │  API Server  │     │ In-Memory DB  │  │
│  │  SvelteKit   │◄───►│     Go       │◄───►│   (MVP)       │  │
│  │   :3001      │     │    :8080      │     │   (Hash)      │  │
│  └──────┬───────┘     └──────┬───────┘     └───────────────┘  │
│         │                    │                                   │
│         │              ┌─────┴─────┐                            │
│         │              │   Auth    │                            │
│         │              │   JWT     │                            │
│         │              └───────────┘                            │
│         │                                                          │
│         │              ┌───────────┐                             │
│         └─────────────►│ WebSocket │◄─── Real-time updates      │
│                        │   :8082   │                            │
│                        └───────────┘                             │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### Frontend (SvelteKit)
- **Port**: 3001
- **State Management**: Svelte stores
- **API Client**: TypeScript client in lib/api.ts

### Backend (Go + Gin)
- **Port**: 8080
- **Framework**: Gin
- **Auth**: JWT middleware

### In-Memory Storage
- **Agents**: map[string]*Agent
- **Sessions**: map[string]*Session
- **Memories**: map[string]*Memory
- **Providers**: map[string]*Provider

## API Structure
```
/api/v1
├── /auth          # Authentication
│   ├── POST /register
│   ├── POST /login
│   ├── POST /refresh
│   └── GET /me
├── /agents        # Agent management
│   ├── GET /
│   ├── POST /
│   ├── GET /:id
│   ├── PUT /:id
│   ├── DELETE /:id
│   ├── POST /:id/start
│   └── POST /:id/stop
├── /sessions      # Session management
│   ├── GET /
│   ├── POST /
│   ├── GET /:id
│   └── DELETE /:id
├── /thoughts      # Thought visualization
│   ├── GET /
│   └── POST /simulate
├── /memories     # Memory management
│   ├── GET /agents/:id
│   ├── POST /agents/:id
│   ├── GET /agents/:id/:mid
│   ├── PUT /agents/:id/:mid
│   ├── DELETE /agents/:id/:mid
│   └── GET /agents/:id/search
├── /providers    # LLM Providers
│   ├── GET /
│   ├── POST /
│   └── DELETE /:id
├── /tools        # Tool management
│   ├── GET /
│   ├── GET /agents/:id
│   ├── POST /agents/:id
│   └── DELETE /agents/:id/:tid
└── /settings     # Agent settings
    ├── GET /agents/:id
    └── PUT /agents/:id
```

## Data Flow

### Agent Start Flow
1. Frontend calls POST /api/v1/agents/:id/start
2. Backend validates JWT
3. Creates session if none exists
4. Updates agent status to "running"
5. Returns session info

### Chat Flow
1. User types message
2. Frontend POST /api/v1/sessions/:id/messages
3. Backend creates message record
4. Simulates agent response
5. Creates thought records
6. Returns response + thoughts

### WebSocket Flow
1. Frontend connects to /ws/agents/:id
2. Server validates session
3. On thought update, broadcast to WS
4. Frontend updates UI in real-time
