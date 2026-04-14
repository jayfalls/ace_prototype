# Authentication & Authorization (Users & Auth Unit)

**Unit**: users-auth  
**Status**: ✅ Complete  
**Implementation**: 17 micro-PRs

---

## Overview

This document describes the authentication and authorization system implemented in the ACE Framework. The system provides JWT-based authentication with support for username/PIN login, session management, and role-based access control.

---

## Architecture

### Technology Stack

- **PIN Hashing**: Argon2id (64MB memory, 3 iterations, 4 parallelism)
- **Token Signing**: RS256 (RSA 2048-bit keys)
- **Access Token TTL**: 15 minutes (configurable)
- **Refresh Token TTL**: 7 days (configurable)
- **PIN Length**: 4-6 digits

### Database Schema

| Table | Purpose |
|-------|---------|
| `users` | User accounts with username, pin_hash, role, status |
| `sessions` | Active user sessions with refresh token hashes |
| `resource_permissions` | Resource-level permissions (view/use/admin) |

### Role-Based Access Control

| Role | Permissions |
|------|-------------|
| `admin` | Full system access, user management |
| `user` | Standard access to own resources |
| `viewer` | Read-only access |

### Permission Levels

| Level | Description |
|-------|-------------|
| `view` | Read access to resource |
| `use` | View + execute/action on resource |
| `view` | Full control including management |

---

## API Endpoints

### Authentication

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/register` | Create new user account (username + PIN) |
| POST | `/auth/login` | Login with username + PIN |
| POST | `/auth/logout` | Invalidate session |
| POST | `/auth/refresh` | Rotate refresh token |

### Session Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/me` | Get current user profile |
| GET | `/auth/me/sessions` | List user's active sessions |
| DELETE | `/auth/me/sessions/:id` | Revoke specific session |

### Admin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/users` | List all users (paginated) |
| GET | `/admin/users/:id` | Get user details |
| PUT | `/admin/users/:id/role` | Update user role |
| POST | `/admin/users/:id/suspend` | Suspend user account |
| POST | `/admin/users/:id/restore` | Restore suspended user |

---

## Middleware

| Middleware | Purpose |
|------------|---------|
| `RequireAuth` | Validates Bearer JWT, attaches user to context |
| `RequireRole` | Checks user has required role |
| `RateLimiter` | Per-IP and per-email rate limiting |

### Middleware Order

```
Recovery → Logger → CORS → RateLimit → Auth → Handler
```

---

## PIN Requirements

| Requirement | Default |
|-------------|---------|
| Minimum length | 4 digits |
| Maximum length | 6 digits |
| Numeric only | Required |

---

## Events

| Event | Subject |
|-------|---------|
| Login | `ace.auth.login.event` |
| Logout | `ace.auth.logout.event` |
| Failed Login | `ace.auth.failed_login.event` |
| Password Change | `ace.auth.password_change.event` |
| Role Change | `ace.auth.role_change.event` |
| Account Suspended | `ace.auth.suspended.event` |
| Account Deleted | `ace.auth.deleted.event` |

*Events are published internally via the event bus.*

---

## Configuration

Environment variables for auth (see `backend/services/api/internal/config/config.go`):

```bash
# JWT
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
JWT_AUDIENCE=ace-api
JWT_ISSUER=ace-auth

# Rate Limiting
RATE_LIMIT_PER_IP=100
RATE_LIMIT_WINDOW=1m
LOGIN_LOCKOUT_THRESHOLD=5
LOGIN_LOCKOUT_DURATION=15m

# PIN
PIN_MIN_LENGTH=4
PIN_MAX_LENGTH=6

# Deployment
DEPLOYMENT_MODE=single
BASE_URL=http://localhost:3000
```

---

## Implementation Files

### Models
- `backend/services/api/internal/model/user.go`
- `backend/services/api/internal/model/session.go`
- `backend/services/api/internal/model/auth_token.go`
- `backend/services/api/internal/model/permission.go`
- `backend/services/api/internal/model/token_claims.go`
- `backend/services/api/internal/model/errors.go`
- `backend/services/api/internal/model/events.go`

### Services
- `backend/services/api/internal/service/token_service.go`
- `backend/services/api/internal/service/auth_service.go`
- `backend/services/api/internal/service/permission_service.go`
- `backend/services/api/internal/service/event_service.go`

### Handlers
- `backend/services/api/internal/handler/auth_handler.go`
- `backend/services/api/internal/handler/session_handler.go`
- `backend/services/api/internal/handler/admin_handler.go`

### Middleware
- `backend/services/api/internal/middleware/auth_middleware.go`
- `backend/services/api/internal/middleware/rbac_middleware.go`
- `backend/services/api/internal/middleware/rate_limit_middleware.go`

### Database
- `backend/migrations/20240401000001_create_version_stamps.sql`
- `backend/migrations/20240401000002_create_users.sql`
- `backend/migrations/20240401000003_create_sessions.sql`
- `backend/migrations/20240401000004_create_resource_permissions.sql`
- `backend/migrations/20240401000005_create_ott_spans.sql`
- `backend/migrations/20240401000006_create_ott_metrics.sql`
- `backend/migrations/20240401000007_create_usage_events.sql`

---

## Unit Test Coverage

- Token service tests
- Auth service tests
- Permission service tests
- Event service tests
- Handler tests (auth, session, admin)
- Middleware tests (auth, RBAC, rate limiting)

---

## Related Documentation

- [design/units/users-auth/problem_space.md](../design/units/users-auth/problem_space.md)
- [design/units/users-auth/bsd.md](../design/units/users-auth/bsd.md)
- [design/units/users-auth/fsd.md](../design/units/users-auth/fsd.md)
- [design/units/users-auth/implementation.md](../design/units/users-auth/implementation.md)
- [design/units/users-auth/security.md](../design/units/users-auth/security.md)
- [design/units/users-auth/research.md](../design/units/users-auth/research.md)