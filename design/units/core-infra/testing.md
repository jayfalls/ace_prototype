# Test Plan

<!--
Intent: Define the testing strategy and specific test cases for the feature.
Scope: Unit tests, integration tests, E2E tests, and their coverage targets.
Used by: AI agents to write comprehensive tests that validate the feature.
-->

## Overview

The core-infra unit implements foundational services for the ACE Framework MVP, including user authentication, agent management, session tracking, thought recording, memory storage, and LLM provider configuration. Comprehensive testing is essential to ensure reliability and security.

## Test Strategy

### Testing Pyramid

```
        /\
       /  \      E2E Tests (Few - ~5)
      /----\     - Critical user flows (auth, agent lifecycle)
     /      \
    /--------\  Integration Tests (Some - ~20)
   /          \ - API endpoints, DB interactions
  /------------\ Unit Tests (Many - ~50+)
 /              \ - Individual functions/methods
```

### Test Priorities

| Priority | Coverage Target | Description |
|----------|----------------|-------------|
| Must | 100% | Authentication, authorization, critical data operations |
| Should | 80% | CRUD operations, WebSocket, settings |
| Could | 60% | Edge cases, error handling |

## Unit Tests

### Test File Structure (Backend)

```
backend/
├── internal/
│   ├── handlers/
│   │   └── handlers_test.go
│   ├── services/
│   │   └── services_test.go
│   ├── models/
│   │   └── models_test.go
│   └── middleware/
│       └── middleware_test.go
```

### Test Cases - Authentication

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| auth | HashPassword | Hash password, verify correct | Success, no error |
| auth | HashPassword | Hash password, verify wrong | Error returned |
| auth | GenerateToken | Generate JWT token | Token string returned |
| auth | ValidateToken | Validate valid token | Claims returned |
| auth | ValidateToken | Validate expired token | Error returned |
| auth | ValidateToken | Validate invalid token | Error returned |

### Test Cases - User Service

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| users | CreateUser | Valid input | User created, ID returned |
| users | CreateUser | Duplicate email | Error returned |
| users | CreateUser | Invalid email format | Validation error |
| users | GetUserByID | Valid ID | User returned |
| users | GetUserByID | Invalid ID | Error returned |
| users | GetUserByEmail | Valid email | User returned |
| users | UpdateUser | Valid update | User updated |
| users | DeleteUser | Valid ID | User deleted |

### Test Cases - Agent Service

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| agents | CreateAgent | Valid input | Agent created |
| agents | CreateAgent | Duplicate name | Error returned |
| agents | ListAgents | User has agents | List returned |
| agents | ListAgents | User has no agents | Empty list |
| agents | GetAgent | Valid ID, owner | Agent returned |
| agents | GetAgent | Not owner | Error returned |
| agents | UpdateAgent | Valid update | Agent updated |
| agents | DeleteAgent | Valid ID | Agent deleted |

### Test Cases - Session Service

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| sessions | CreateSession | Agent exists | Session created |
| sessions | CreateSession | Agent inactive | Error returned |
| sessions | EndSession | Active session | Session ended |
| sessions | ListSessions | Agent has sessions | List returned |

### Test Cases - Thoughts

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| thoughts | CreateThought | Valid input | Thought created |
| thoughts | ListThoughts | Session has thoughts | List returned |
| thoughts | ListThoughts | No thoughts | Empty list |

### Test Cases - Memories

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| memories | CreateMemory | Valid input | Memory created |
| memories | ListMemories | Filter by type | Filtered list |
| memories | SearchMemories | Search query | Results returned |
| memories | UpdateMemory | Valid update | Memory updated |
| memories | DeleteMemory | Valid ID | Memory deleted |

### Test Cases - LLM Providers

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| llm | CreateProvider | Valid input | Provider created |
| llm | GetProvider | Valid ID | Provider returned |
| llm | UpdateProvider | Valid update | Provider updated |
| llm | DeleteProvider | Valid ID | Provider deleted |

### Test Cases - Settings

| Module | Function | Test Case | Expected Result |
|--------|----------|-----------|-----------------|
| settings | GetSetting | Setting exists | Value returned |
| settings | GetSetting | Setting not exists | Default returned |
| settings | SetSetting | Valid input | Setting saved |
| settings | DeleteSetting | Valid key | Setting deleted |

### Mock Dependencies

| Dependency | Mock Library | Rationale |
|------------|--------------|-----------|
| Database | sqlmock | Test SQL queries without DB |
| JWT | Custom mock | Test token generation/validation |
| Password hashing | Custom mock | Test hashing without bcrypt cost |

## Integration Tests

### Test Cases

| Scenario | Preconditions | Test Steps | Expected Result |
|----------|---------------|------------|-----------------|
| User registration | Clean DB | POST /users/register | 201, user data |
| User login | User exists | POST /users/login | 200, tokens |
| Create agent | User logged in | POST /agents | 201, agent data |
| List agents | Agents exist | GET /agents | 200, list |
| Get agent | Agent exists | GET /agents/:id | 200, agent |
| Update agent | Agent exists | PUT /agents/:id | 200, updated |
| Delete agent | Agent exists | DELETE /agents/:id | 204 |
| Start session | Agent exists | POST /sessions | 201, session |
| End session | Session active | DELETE /sessions/:id | 200 |
| Record thought | Session active | POST /thoughts | 201 |
| Create memory | User logged in | POST /memories | 201 |
| Search memories | Memories exist | GET /memories?search= | 200, results |
| Create LLM provider | User logged in | POST /llm-providers | 201 |
| WebSocket connect | Valid token | WS /ws?token=... | Connected |

### Test Fixtures

| Fixture | Purpose | Setup |
|---------|---------|-------|
| DB | Test database | PostgreSQL test container |
| TestUser | Authenticated user | Create test user, return token |
| TestAgent | Test agent | Create agent for testing |
| TestSession | Active session | Start session for testing |

## E2E Tests

### Critical User Flows

| Flow | Priority | Test Steps |
|------|----------|------------|
| User registration and login | Must | Register → Login → Verify token |
| Agent CRUD lifecycle | Must | Create → Read → Update → Delete |
| Real-time chat | Must | Connect WS → Send message → Receive response |
| Memory persistence | Must | Create memory → Search → Retrieve |

### Test Data

| Data | Setup | Cleanup |
|------|-------|---------|
| Test users | Create before test | Delete after test |
| Test agents | Create before test | Delete after test |
| Test sessions | Create before test | End after test |

## Test Coverage

### Coverage Targets

| Metric | Target | Minimum |
|--------|--------|---------|
| Line Coverage | 80% | 70% |
| Branch Coverage | 70% | 60% |
| Function Coverage | 90% | 80% |

### Coverage Exclusions

| File/Module | Reason |
|--------------|--------|
| migrations/ | Database migrations |
| main.go | Entry point |
| config/ | Configuration loading |

## Test Execution

### Running Tests (Backend)

```bash
# Unit tests
go test ./internal/... -v -cover

# Integration tests  
go test ./internal/integration/... -v -tags=integration

# All tests with coverage
go test ./... -v -coverprofile=coverage.out

# View coverage
go tool cover -html=coverage.out
```

### Running Tests (Frontend)

```bash
# Unit tests
npm test

# Unit tests with coverage
npm test -- --coverage

# E2E tests
npm run test:e2e
```

### CI/CD Integration

- **Trigger**: On every PR and push to mvp, main
- **Required**: All tests must pass
- **Coverage Gate**: 70% minimum

## Test Maintenance

- Review tests quarterly
- Remove obsolete tests
- Update test data as features evolve
- Add tests for bug fixes
