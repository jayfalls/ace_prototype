# Testing Strategy

## Test Types

### Unit Tests
**Backend (Go)**
- Handler functions: 80% target
- Store methods: 100% target
- Auth functions: 100% target

**Frontend (TypeScript/Svelte)**
- Utility functions
- Component logic

### Integration Tests
- API endpoint tests with test client
- Auth flow tests
- WebSocket connection tests

### E2E Tests (Playwright)
- Login/Register flow
- Agent CRUD flow
- Chat message flow
- Memory operations

## Backend Tests

### Auth Tests
```go
// TestRegister_success
// TestRegister_duplicateEmail
// TestLogin_success
// TestLogin_wrongPassword
// TestTokenRefresh
// TestValidateToken
```

### Agent Tests
```go
// TestCreateAgent
// TestListAgents
// TestGetAgent
// TestUpdateAgent
// TestDeleteAgent
// TestStartAgent
// TestStopAgent
```

### Memory Tests
```go
// TestCreateMemory
// TestListMemories
// TestUpdateMemory
// TestDeleteMemory
// TestSearchMemories
```

### Tools Tests
```go
// TestListAvailableTools
// TestAddToolWhitelist
// TestRemoveToolWhitelist
```

## Frontend Tests

### Page Tests
- Login page renders correctly
- Register form validation
- Agent list renders
- Chat messages display
- Memory tree renders
- Settings form works

## Running Tests

### Backend
```bash
cd backend
go test ./... -v
```

### Frontend
```bash
cd frontend
npm test
# or
npx vitest run
```

### E2E
```bash
npx playwright test
```

## Coverage Targets
- Backend handlers: 80%
- Backend stores: 90%
- Auth: 100%
- Frontend components: 70%
