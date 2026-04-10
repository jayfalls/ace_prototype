# Implementation Plan: Users-Auth Unit

## Overview

This document breaks down the implementation of the users-auth unit into micro-PRs. Each PR represents the smallest divisible unit of work that can be independently implemented, tested, and merged.

## Prerequisites

Before starting implementation, ensure you have read:
- `design/README.md` - ACE Framework architecture
- `design/units/users-auth/` - All design documents:
  - `problem_space.md` - Problem definition
  - `bsd.md` - Business specification
  - `user_stories.md` - User-facing behavior
  - `fsd.md` - Technical implementation details
  - `research.md` - Technical research and recommendations

## Architecture Pattern

This implementation follows the established pattern:
```
Handler → Service → Repository
```

- **Handler**: HTTP request/response only, no business logic
- **Service**: Business logic, orchestrates repositories
- **Repository**: Database queries via SQLC

## Dependencies on Existing Shared Modules

- `ace/shared/caching` - Valkey operations (token blacklist, rate limiting, permission cache)
- `ace/shared/messaging` - NATS event publishing

---

## Phase 1: Foundation

### PR 1: Database Migration
**Goal**: Create all auth-related database tables using Goose Go migrations

**Files to create/modify**:
- `backend/services/auth/migrations/20240401000001_create_auth_tables.go` - Main auth tables migration
- `backend/services/auth/migrations/20240401000002_add_auth_indexes.go` - Additional indexes

**Dependencies**: None (first PR)

**Acceptance Criteria**:
- [ ] Users table created with: id, email, password_hash, role, status, suspended_at, suspended_reason, deleted_at, created_at, updated_at
- [ ] Sessions table created with: id, user_id, refresh_token_hash, user_agent, ip_address, last_used_at, expires_at, created_at
- [ ] AuthTokens table created with: id, user_id, token_type, token_hash, expires_at, used_at, created_at
- [ ] ResourcePermissions table created with: id, user_id, resource_type, resource_id, permission_level, granted_by, created_at
- [ ] All indexes created for query performance
- [ ] Migration can be rolled back successfully

**Testing**:
- Run migration against test database
- Verify tables exist with correct columns
- Test rollback functionality

---

### PR 2: Domain Models
**Goal**: Create Go domain models that mirror database tables

**Files to create/modify**:
- `backend/services/auth/internal/model/user.go` - User model with roles and status enums
- `backend/services/auth/internal/model/session.go` - Session model
- `backend/services/auth/internal/model/auth_token.go` - Auth token model (magic links, verification)
- `backend/services/auth/internal/model/permission.go` - Resource permission model
- `backend/services/auth/internal/model/token_claims.go` - JWT claims model
- `backend/services/auth/internal/model/errors.go` - Auth-specific errors

**Dependencies**: PR 1 (migration)

**Acceptance Criteria**:
- [ ] User model has all fields with correct types and JSON tags
- [ ] UserRole enum: admin, user, viewer
- [ ] UserStatus enum: pending, active, suspended
- [ ] PermissionLevel enum: view, use, admin
- [ ] AuthTokenType enum: login, verification, password_reset
- [ ] TokenClaims struct matches JWT payload (iss, sub, aud, exp, iat, jti, role, email)
- [ ] Custom errors defined: ErrInvalidCredentials, ErrTokenExpired, etc.

**Testing**:
- Unit tests for model serialization
- Verify JSON tags work correctly

---

### PR 3: SQLC Queries
**Goal**: Create type-safe database queries using SQLC

**Files to create/modify**:
- `backend/services/auth/sqlc.yaml` - SQLC configuration
- `backend/services/auth/internal/repository/queries/users.sql` - User CRUD queries
- `backend/services/auth/internal/repository/queries/sessions.sql` - Session queries
- `backend/services/auth/internal/repository/queries/auth_tokens.sql` - Auth token queries
- `backend/services/auth/internal/repository/queries/permissions.sql` - Permission queries

**Dependencies**: PR 1 (tables exist)

**Acceptance Criteria**:
- [ ] SQLC generates Go code from queries
- [ ] User queries: Create, GetByID, GetByEmail, Update, List, SoftDelete
- [ ] Session queries: Create, GetByID, GetByUserID, Delete, DeleteAllByUserID
- [ ] AuthToken queries: Create, GetByHash, MarkAsUsed, DeleteExpired
- [ ] Permission queries: Create, Get, Delete, ListByUser, ListByResource
- [ ] Generated code compiles without errors

**Testing**:
- Run `sqlc generate`
- Verify generated files in `internal/repository/generated/`

---

## Phase 2: Core Services

### PR 4: Auth Service (Password Hashing)
**Goal**: Implement password hashing using Argon2id

**Files to create/modify**:
- `backend/services/auth/internal/service/password_service.go` - Password hashing/verification

**Dependencies**: PR 2 (models)

**Acceptance Criteria**:
- [ ] HashPassword uses golang.org/x/crypto/argon2 with parameters: 64MB memory, 3 iterations, 4 parallelism
- [ ] VerifyPassword uses constant-time comparison
- [ ] Password complexity validation (min 8 chars, upper, lower, number)
- [ ] Password strength checking returns helpful error messages

**Testing**:
- Unit tests for hash/verify correctness
- Performance test (~500ms hashing time)

---

### PR 5: Token Service (JWT)
**Goal**: Implement JWT generation and validation with RS256

**Files to create/modify**:
- `backend/services/auth/internal/service/token_service.go` - JWT service

**Dependencies**: PR 2 (models)

**Acceptance Criteria**:
- [ ] GenerateAccessToken creates RS256 signed JWT with correct claims
- [ ] GenerateRefreshToken creates opaque UUID token
- [ ] ValidateAccessToken verifies signature and expiry
- [ ] Token claims include: iss, sub, aud, exp, iat, jti, role, email
- [ ] Access token TTL configurable (default 15 minutes)
- [ ] Refresh token TTL configurable (default 7 days)
- [ ] RS256 used in all environments (dev and prod identical)

**Testing**:
- Unit tests for token generation/validation
- Verify RS256 signature (not HS256)

---

### PR 6: Auth Service (Core Logic)
**Goal**: Implement core authentication business logic

**Files to create/modify**:
- `backend/services/auth/internal/service/auth_service.go` - Main auth logic

**Dependencies**: PR 3 (SQLC), PR 4 (password), PR 5 (tokens)

**Acceptance Criteria**:
- [ ] Register creates user with hashed password
- [ ] Login validates credentials and returns tokens
- [ ] Single-user mode: first user becomes admin
- [ ] Multi-user mode: open registration allowed
- [ ] Account status check (active, pending, suspended)
- [ ] Failed login attempt tracking
- [ ] Account lockout after 5 failed attempts (15 min)

**Testing**:
- Integration tests with test database

---

### PR 7: Magic Link Service
**Goal**: Implement magic link token generation and verification

**Files to create/modify**:
- `backend/services/auth/internal/service/magic_link_service.go` - Magic link handling

**Dependencies**: PR 3 (SQLC), PR 5 (token service)

**Acceptance Criteria**:
- [ ] GenerateLoginToken creates 32-byte cryptographically secure token
- [ ] Token stored as SHA256 hash (not plaintext)
- [ ] Token expiry configurable (default 15 minutes for login, 1 hour for password reset)
- [ ] Single-use tokens (marked as used after verification)
- [ ] GeneratePasswordResetToken creates token for password reset flow
- [ ] Token validation checks: hash match, expiry, not used

**Testing**:
- Unit tests for token generation
- Integration tests for verify flow

---

### PR 8: Permission Service
**Goal**: Implement RBAC and resource-level permissions

**Files to create/modify**:
- `backend/services/auth/internal/service/permission_service.go` - Permission checks
- `backend/services/auth/internal/service/rate_limit_service.go` - Rate limiting

**Dependencies**: PR 3 (SQLC), PR 2 (models)

**Acceptance Criteria**:
- [ ] CheckResourcePermission verifies user can access resource
- [ ] Permission hierarchy: admin > use > view
- [ ] Permission caching in Valkey (5 minute TTL)
- [ ] GrantPermission adds resource permission
- [ ] RevokePermission removes resource permission
- [ ] Rate limit checking for login endpoint (10/min per IP, 5/5min per email)
- [ ] Rate limit checking for password reset (3/hour per email)

**Testing**:
- Unit tests for permission logic
- Integration tests with Valkey

---

## Phase 3: HTTP Handlers

### PR 9: Auth Handlers
**Goal**: Implement HTTP handlers for auth endpoints

**Files to create/modify**:
- `backend/services/auth/internal/handler/auth_handler.go` - Register, login, logout
- `backend/services/auth/internal/handler/password_handler.go` - Password reset, change
- `backend/services/auth/internal/handler/magic_link_handler.go` - Magic link request/verify

**Dependencies**: PR 6 (auth service), PR 7 (magic link)

**Acceptance Criteria**:
- [ ] POST /auth/register - Creates user, returns tokens
- [ ] POST /auth/login - Validates credentials, returns tokens
- [ ] POST /auth/logout - Invalidates session
- [ ] POST /auth/refresh - Rotates refresh token, returns new tokens
- [ ] POST /auth/password/reset/request - Sends reset token (or logs in dev mode)
- [ ] POST /auth/password/reset/confirm - Resets password with token
- [ ] POST /auth/password/change - Changes password while logged in
- [ ] POST /auth/magic-link/request - Requests magic link
- [ ] POST /auth/magic-link/verify - Verifies magic link, returns tokens
- [ ] All handlers use response helpers from core-api pattern

**Testing**:
- Handler unit tests
- Integration tests with test database

---

### PR 10: Session Handlers
**Goal**: Implement session management endpoints

**Files to create/modify**:
- `backend/services/auth/internal/handler/session_handler.go` - Session management

**Dependencies**: PR 6 (auth service), PR 9 (auth handlers)

**Acceptance Criteria**:
- [ ] GET /auth/me - Returns current user profile
- [ ] GET /auth/me/sessions - Lists user's active sessions
- [ ] DELETE /auth/me/sessions/:id - Revokes specific session

**Testing**:
- Handler tests

---

### PR 11: Admin Handlers
**Goal**: Implement admin-only user management endpoints

**Files to create/modify**:
- `backend/services/auth/internal/handler/admin_handler.go` - Admin user management

**Dependencies**: PR 9 (auth handlers)

**Acceptance Criteria**:
- [ ] GET /admin/users - Lists all users (paginated)
- [ ] GET /admin/users/:id - Gets user details
- [ ] PUT /admin/users/:id/role - Updates user role
- [ ] POST /admin/users/:id/suspend - Suspends user, revokes sessions
- [ ] POST /admin/users/:id/restore - Restores suspended user
- [ ] POST /admin/users/:id/delete - Soft-deletes user

**Testing**:
- Handler tests
- Verify admin-only access

---

## Phase 4: Middleware

### PR 12: Auth Middleware
**Goal**: Implement JWT validation middleware

**Files to create/modify**:
- `backend/services/auth/internal/middleware/auth_middleware.go` - JWT validation
- `backend/services/auth/internal/middleware/rbac_middleware.go` - Role checking

**Dependencies**: PR 5 (token service), PR 11 (admin handlers)

**Acceptance Criteria**:
- [ ] RequireAuth middleware validates Bearer JWT
- [ ] Attaches user and claims to request context
- [ ] Checks token blacklist in Valkey
- [ ] RequireRole middleware checks user role
- [ ] Context helpers: GetUserFromContext, GetTokenClaimsFromContext

**Testing**:
- Middleware unit tests
- Integration tests with valid/invalid tokens

---

### PR 13: Rate Limiting Middleware
**Goal**: Implement rate limiting middleware

**Files to create/modify**:
- `backend/services/auth/internal/middleware/rate_limit_middleware.go` - Rate limiting

**Dependencies**: PR 8 (permission service), PR 12 (auth middleware)

**Acceptance Criteria**:
- [ ] Sliding window counter algorithm
- [ ] Per-IP rate limiting
- [ ] Per-email rate limiting for auth endpoints
- [ ] Rate limit headers in responses (X-RateLimit-Limit, X-RateLimit-Remaining)
- [ ] 429 response when rate limited

**Testing**:
- Middleware tests with mock Valkey

---

## Phase 5: Events

### PR 14: Auth Event Publishing
**Goal**: Implement NATS event publishing for auth operations

**Files to create/modify**:
- `backend/services/auth/internal/service/event_service.go` - Event publishing
- `backend/services/auth/internal/model/events.go` - Event types

**Dependencies**: PR 9 (auth handlers), PR 10 (session handlers)

**Acceptance Criteria**:
- [ ] PublishLoginEvent sends login event to NATS
- [ ] PublishLogoutEvent sends logout event
- [ ] PublishFailedLoginEvent sends failed login attempt
- [ ] PublishPasswordChangeEvent sends password change event
- [ ] PublishRoleChangeEvent sends role change event
- [ ] PublishAccountSuspendedEvent sends suspension event
- [ ] PublishAccountDeletedEvent sends deletion event
- [ ] Event schema matches fsd.md specification

**Testing**:
- Unit tests with mock NATS client

---

## Phase 6: Configuration

### PR 15: Auth Configuration
**Goal**: Complete configuration loading for auth service

**Files to create/modify**:
- `backend/services/auth/internal/config/config.go` - Auth config struct

**Dependencies**: PR 1 (foundation)

**Acceptance Criteria**:
- [ ] JWT config: AccessTokenTTL, RefreshTokenTTL, PrivateKey, PublicKey
- [ ] Rate limit config: LoginRateLimitPerIP, LoginRateLimitPerEmail, LockoutThreshold, LockoutDuration
- [ ] Password config: MinLength, RequireUpper, RequireLower, RequireNumber, RequireSymbol
- [ ] Token config: EmailTokenTTL, ResetTokenTTL
- [ ] Deployment config: DeploymentMode (single/multi), BaseURL, SMTP config
- [ ] All config from environment variables

**Testing**:
- Config validation tests

---

### PR 16: Server Setup
**Goal**: Wire up HTTP server with all middleware and handlers

**Files to create/modify**:
- `backend/services/auth/cmd/main.go` - Server entry point
- `backend/services/auth/internal/router/router.go` - Route configuration

**Dependencies**: PR 12 (middleware), PR 9-11 (handlers)

**Acceptance Criteria**:
- [ ] Server starts on configured port
- [ ] All routes registered correctly
- [ ] Middleware stack: Recovery → Logger → CORS → RateLimit → Auth → Handler
- [ ] Graceful shutdown works

**Testing**:
- Server starts and responds to requests

---

## Phase 7: Testing

### PR 17: Unit Tests
**Goal**: Comprehensive unit tests for all services

**Files to create/modify**:
- `backend/services/auth/internal/service/*_test.go` - Service unit tests
- `backend/services/auth/internal/handler/*_test.go` - Handler unit tests
- `backend/services/auth/internal/middleware/*_test.go` - Middleware unit tests

**Dependencies**: PRs 4-16 (all implementation)

**Acceptance Criteria**:
- [ ] Password service tests
- [ ] Token service tests
- [ ] Auth service tests (mock repository)
- [ ] Magic link service tests
- [ ] Permission service tests
- [ ] Handler tests with mock services
- [ ] Middleware tests with mock dependencies

**Coverage Target**: 80% for new code

---

### PR 18: Integration Tests
**Goal**: End-to-end tests for authentication flows

**Files to create/modify**:
- `backend/services/auth/internal/integration/auth_test.go` - Auth flow tests
- `backend/services/auth/internal/integration/admin_test.go` - Admin flow tests

**Dependencies**: PR 17 (unit tests)

**Acceptance Criteria**:
- [ ] Registration flow works end-to-end
- [ ] Login/logout flow works
- [ ] Token refresh works
- [ ] Password reset flow works
- [ ] Magic link login works
- [ ] Admin user management works

**Testing**:
- Uses test database with migrations applied

---

## Implementation Order Summary

| PR | Name | Files | Dependencies |
|----|------|-------|--------------|
| 1 | Database Migration | migrations/*.go | None |
| 2 | Domain Models | internal/model/*.go | 1 |
| 3 | SQLC Queries | sqlc.yaml, queries/*.sql | 1 |
| 4 | Password Service | internal/service/password_service.go | 2 |
| 5 | Token Service | internal/service/token_service.go | 2 |
| 6 | Auth Service | internal/service/auth_service.go | 3, 4, 5 |
| 7 | Magic Link Service | internal/service/magic_link_service.go | 3, 5 |
| 8 | Permission Service | internal/service/permission_service.go | 3, 2 |
| 9 | Auth Handlers | internal/handler/auth_handler.go, *_handler.go | 6, 7 |
| 10 | Session Handlers | internal/handler/session_handler.go | 6, 9 |
| 11 | Admin Handlers | internal/handler/admin_handler.go | 9 |
| 12 | Auth Middleware | internal/middleware/auth_middleware.go | 5, 11 |
| 13 | Rate Limit Middleware | internal/middleware/rate_limit_middleware.go | 8, 12 |
| 14 | Event Publishing | internal/service/event_service.go | 9, 10 |
| 15 | Auth Configuration | internal/config/config.go | 1 |
| 16 | Server Setup | cmd/main.go, internal/router/router.go | 12, 9-11 |
| 17 | Unit Tests | */*_test.go | 4-16 |
| 18 | Integration Tests | internal/integration/*_test.go | 17 |

---

## Implementation Notes

### Module Structure
The auth service is a separate Go module:
```
backend/services/auth/
├── cmd/server/main.go
├── internal/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── middleware/
│   ├── model/
│   └── config/
├── migrations/
├── sqlc/
└── go.mod
```

### Workspace Integration
Add auth module to existing go.work:
```go
go 1.26

use (
    "./services/api"
    "./shared"
    "./services/auth"  // New
)
```

### Error Handling Pattern
- Services return errors with context
- Handlers log and convert to HTTP responses
- Use response helpers from core-api: `response.Success()`, `response.Error()`

### Testing Strategy
- Use interfaces for dependencies (enables mocking)
- Keep tests adjacent to code: `service_test.go`
- Integration tests use testcontainers or test database

### Single-User Mode
In single-user mode (first deployment):
- First registered user automatically gets admin role
- Registration remains open
- Config: `AUTH_DEPLOYMENT_MODE=single`

### Dev Mode
Without SMTP configured:
- Magic links logged to console: `[DEV] Magic link for user@example.com: ...`
- Password reset tokens logged similarly
- Allows full offline development

---

## Workflow

1. Create branch from `main` for each PR
2. Implement one PR at a time
3. Ensure all acceptance criteria are met
4. Create PR with links to related issues
5. After merge, move to next PR

## Future Extensions

After initial implementation:
- MFA support (TOTP, WebAuthn)
- OAuth providers (Google, GitHub)
- Session device management
- Login history / audit log