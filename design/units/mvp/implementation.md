# Implementation Plan

## Phase 1: Foundation (Completed)
- [x] Project setup (Go backend, SvelteKit frontend)
- [x] Basic API structure with Gin
- [x] Agent CRUD endpoints
- [x] Basic frontend layout

## Phase 2: Core Features (Completed)
- [x] Session management
- [x] Chat API and UI
- [x] Thought visualization
- [x] Settings page

## Phase 3: Missing APIs (To Implement)
- [ ] Memories API (CRUD + search)
- [ ] Tools API (whitelist)
- [ ] JWT Authentication (register/login/refresh)
- [ ] WebSocket for real-time updates

## Phase 4: Frontend Pages (To Implement)
- [ ] Login page
- [ ] Register page
- [ ] Memory browser page

## Phase 5: Testing (To Implement)
- [ ] Unit tests for handlers
- [ ] Integration tests for API
- [ ] Frontend component tests

## Backend Implementation Details

### Memories Handler
```go
// MemoryStore in-memory implementation
type MemoryStore struct {
    mu       sync.RWMutex
    memories map[string]*Memory
    byAgent  map[string][]string // agentID -> []memoryID
}

// Methods needed:
// - CreateMemory(agentID, parentID, content, tags, memoryType, importance) (*Memory, error)
// - GetMemory(id) (*Memory, error)
// - UpdateMemory(id, content, tags, importance) (*Memory, error)  
// - DeleteMemory(id) error
// - ListMemoriesByAgent(agentID) ([]*Memory, error)
// - SearchMemories(agentID, query, tags) ([]*Memory, error)
```

### Tools Handler
```go
// ToolStore in-memory implementation
type ToolStore struct {
    mu    sync.RWMutex
    tools map[string]*AgentTool
}

// Methods needed:
// - ListAvailableTools() ([]Tool, error)
// - GetAgentTools(agentID) ([]*AgentTool, error)
// - AddToolToWhitelist(agentID, toolSource, toolName) (*AgentTool, error)
// - RemoveToolFromWhitelist(agentID, toolID) error
```

### Auth Handler
```go
// Auth features:
// - Register(email, password, name) (*User, error)  
// - Login(email, password) (string, error) // returns JWT
// - RefreshToken(tokenString) (string, error)
// - ValidateToken(tokenString) (*Claims, error)

// JWT Claims:
// - UserID, Email, Exp
```

### WebSocket
```go
// WebSocket handler:
// - Handle connection /ws/agents/:id
// - Validate session
// - Subscribe to thought updates
// - Broadcast to connected clients
// - Heartbeat ping/pong
```

## Frontend Implementation Details

### Auth Pages
- /login - Login form
- /register - Registration form  
- Use JWT in localStorage
- Auto-refresh before expiry

### Memory Page
- Tree view of memories
- Search bar with filters
- Create/edit/delete modals

### WebSocket Client
- Connect on visualizations page
- Reconnect on disconnect
- Update thought store in real-time
