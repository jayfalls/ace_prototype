# Users-Auth Unit Architecture

## Overview

This document describes how the **users-auth unit integrates into the ACE Framework as authentication and authorization middleware embedded within the API service**. Auth is not a separate microservice for MVP—it runs within the `services/api` service, providing JWT validation, RBAC, and session management as part of the API's internal middleware and service layers.

**Unit**: `users-auth`
**Status**: Architecture documentation
**Implementation**: Embedded in `services/api/internal/` (NOT a separate service)

---

## 1. Auth Integration Overview

### 1.1 Position in System Architecture

Auth operates as middleware within the API service, not as a separate service. This approach simplifies MVP deployment and reduces network latency for token validation.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          ACE Framework                                  │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────────────┐  │
│  │  Frontend   │      │    API       │      │  Cognitive Engine   │  │
│  │ SvelteKit   │◄────►│     Go      │◄────►│       Go           │  │
│  │   (Web UI)  │      │    (Gin)    │      │   (6 ACE Layers)  │  │
│  └──────┬───────┘      └──────┬───────┘      └──────────┬───────────┘  │
│         │                    │               │              │              │
│         │              ┌──────┴───────┐             │              │
│         │              │              │             │              │
│         │         ┌────▼────┐   ┌─────▼────────────┐              │
│         │         │   Auth   │   │    WebSocket   │              │
│         │         │ Middleware│   │    Handler    │              │
│         │         │   (JWT)  │   └───────────────┘              │
│         │         └────┬─────┘                                  │
│         │              │                                          │
│         │         ┌────▼────┐    ┌───────────────────────────┐    │
│         │         │  Auth   │    │   Shared Services     │    │
│         │         │ Service │    │ - shared/messaging │    │
│         │         └────┬────┘    │ - shared/caching  │    │
│         │              │          │ - shared/telemetry │    │
│         └─────────────┼──────────┴───────────────────────────┘    │
│                       │                                          │
│                       ▼                                          │
│                  ┌────────────┐                                   │
│                  │ PostgreSQL │                                   │
│                  │  (Ace DB) │                                   │
│                  └────────────┘                                   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Request Flow with Auth

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Authenticated Request Flow                           │
│                                                                          │
│  1. REQUEST                     ┌─────────────────────┐                │
│     ─────────                  │  Authorization:     │                │
│     GET /api/agents          │  Bearer <jwt>       │                │
│                              └─────────┬───────────┘                │
│                                      │                           │
│                                      ▼                           │
│  2. MIDDLEWARE STACK                                                      │
│     ┌─────────────────────────────────────────────────────────────┐      │
│     │ TraceMiddleware (OTel)                                      │      │
│     │   - Extract W3C trace context                             │      │
│     └────────────────────────┬──────────────────────────────────┘      │
│                              │                                         │
│                              ▼                                         │
│     ┌─────────────────────────────────────────────────────────────┐    │
│     │ RateLimitMiddleware                                          │    │
│     │   - Per-IP rate limiting                                    │    │
│     └────────────────────────┬──────────────────────────────────────┘    │
│                              │                                         │
│                              ▼                                         │
│     ┌─────────────────────────────────────────────────────────────┐    │
│     │ AuthMiddleware ─────────────────────────────────────────────── │    │
│     │   - Extract JWT from Bearer token                            │    │
│     │   - Validate signature (RS256)                             │    │
│     │   - Check token blacklist (Valkey)                         │    │
│     │   - Attach user to context                                │    │
│     │       │                                                   │    │
│     │       ▼                                                   │    │
│     │ RBACMiddleware (optional)                                  │    │
│     │   - Check user role against required role                 │    │
│     └────────────────────��───┬──────────────────────────────────────┘    │
│                              │                                          │
│                              ▼                                          │
│  3. HANDLER                                                         │
│     ┌────────────────────────────────────────────────────────┐            │
│     │ AgentHandler.List                                       │            │
│     │   - GetUserFromContext (user ID from JWT subject)       │            │
│     │   - Query agents owned by user                      │            │
│     └────────────────────────────────────────────────────┘            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Component Architecture

### 2.1 System Components

Auth is **embedded within the API service** (`services/api`), not a separate service. The following shows the API service with auth as an internal component:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    API SERVER (services/api)                            │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  HTTP Handlers                                                   │     │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐  │     │
│  │  │AuthHandler │  │ AgentHandler│  │WebSocket   │  │ Health  │  │     │
│  │  │  (auth/)   │  │             │  │ Handler    │  │Handler  │  │     │
│  │  └──────┬──────┘  └──────┬──────┘  └─────┬─────┘  └─────┬─────┘  │     │
│  └─────────┼────────────────┼───────────────┼──────────────┼────────┘     │
│            │                │               │              │              │
│  ┌─────────▼────────────────▼───────────────▼──────────────▼────────────┐  │
│  │                   Auth Middleware (embedded)                       │   │
│  │                                                                      │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │             │   │
│  │  │ JWT Validator │  │   RBAC     │  │ Rate Limit  │  │             │   │
│  │  │   (RS256)   │  │  Checker    │  │  (Valkey)  │  │             │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │             │   │
│  └──────────────────────────┬──────────────────────────┘              │   │
│                               │                                         │   │
│  ┌───────────────────────────▼───────────────────────────────────────┐  │
│  │                    Auth Services (embedded)                         │  │
│  │                                                                      │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐│   │
│  │  │  Password   │  │   Token     │  │ Magic Link │  │ Permission ││   │
│  │  │  Service    │  │  Service    │  │  Service   │  │  Service   ││   │
│  │  │ (Argon2id)  │  │  (JWT RS256)│  │             │  │   (RBAC)   ││   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘│   │
│  └──────────────────────────┬───────────────────────────────────────────────┘  │
│                               │                                          │
│  ┌───────────────────────────▼───────────────────────────────────────┐  │
│  │                  Repository Layer (SQLC)                          │  │
│  │                                                                      │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐│  │
│  │  │UserRepository│ │SessionRepo  │  │ TokenRepo   │  │PermRepository││  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘│  │
│  └─────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
           │                    │              │
           ▼                    ▼              ▼
    ┌──────────┐        ┌──────────┐    ┌──────────┐
    │PostgreSQL│        │  Valkey  │    │   NATS   │
    │ (ace_db) │        │(ace_valkey)│    │(ace_broker)│
    └──────────┘        └──────────┘    └──────────┘

NOTE: Auth runs inside services/api as middleware + services, NOT as a separate service.
┌─────────────────────────────────────────────────────────────────────────┐
│                         API Server (services/api)                       │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │
│  │ AuthHandler │  │ AgentHandler│  │WebSocket   │  │  Health    │   │
│  └──────┬──────┘  └──────┬──────┘  └─────┬─────┘  └─────┬─────┘   │
│         │                │               │              │            │
│  ┌──────▼────────────────▼───────────────▼──────────────▼────────────┐   │
│  │                   Auth Middleware                                │            │
│  │                                                      │            │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │            │
│  │  │ JWT Validator │  │  RBAC      │  │ Rate Limit │  │            │
│  │  │   (RS256)   │  │  Checker  │  │  (Valkey) │  │            │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │            │
│  └──────────────────────────┬───────────────────────────────┘            │
│                               │                                          │
│  ┌───────────────────────────▼───────────────────────────────────────┐  │
│  │                    Auth Service                                     │  │
│  │                                                                      │  │
│  │  ┌─────────────┐  ┌────────��─��──┐  ┌─────────────┐  ┌─────────────┐│  │
│  │  │  Password  │  │   Token    │  │ Magic Link │  │ Permission ││  │
│  │  │  Service   │  │  Service   │  │  Service   │  │  Service   ││  │
│  │  │ (Argon2id) │  │  (JWT RS256)│  │            │  │   (RBAC)  ││  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘│  │
│  └──────────────────────────┬───────────────────────────────────────────────┘  │
│                               │                                          │
│  ┌───────────────────────────▼───────────────────────────────────────┐  │
│  │                  Repository Layer (SQLC)                          │  │
│  │                                                                      │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐│  │
│  │  │UserRepository│ │SessionRepo │  │ TokenRepo  │  │PermRepository││  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘│  │
│  └─────────────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────��─────────────────────────────────────┘
           │                    │              │
           ▼                    ▼              ▼
    ┌──────────┐        ┌──────────┐    ┌──────────┐
    │PostgreSQL│        │  Valkey  │    │   NATS   │
    │(ace_db) │        │(ace_valkey)│    │(ace_broker)│
    └──────────┘        └──────────┘    └──────────┘
```

### 2.2 Component Responsibilities

> All locations are relative to `services/api/internal/` — auth is embedded within the API service, not a separate service.

| Component | Responsibility | Location (relative to `internal/`) |
|-----------|---------------|-------------------------------------|
| **AuthHandler** | HTTP endpoints for auth operations | `handler/auth/` |
| **AuthMiddleware** | JWT validation, RBAC enforcement | `middleware/auth.go` |
| **PasswordService** | Argon2id hashing and verification | `service/password.go` |
| **TokenService** | JWT generation and validation (RS256) | `service/token.go` |
| **MagicLinkService** | Magic link token generation and verification | `service/magic_link.go` |
| **PermissionService** | RBAC and resource permissions | `service/permission.go` |
| **EventService** | NATS event publishing | `service/event.go` |
| **UserRepository** | User CRUD via SQLC | `repository/user.go` |
| **SessionRepository** | Session CRUD | `repository/session.go` |
| **TokenRepository** | Auth token storage | `repository/auth_token.go` |
| **PermissionRepository** | Resource permission lookups | `repository/permission.go` |

---

## 3. Data Flow Diagrams

### 3.1 User Registration Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    User Registration Flow                               │
│                                                                          │
│  client                                                           │
│     │                                                              │
│     │ POST /auth/register                                            │
│     │ { "email": "user@example.com", "password": "..." }          │
│     ▼                                                              │
│ ┌────────────────┐                                                │
│ │ AuthMiddleware │                                                │
│ │   (passthru)  │                                                │
│ └───────┬────────┘                                                │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────┐                            │
│ │ Register Handler                  │                            │
│ │ 1. Validate email format           │                            │
│ │ 2. Validate password strength     │                            │
│ │ 3. Check email not exists         │                            │
│ └───────┬────────────────────────────┘                            │
│         │                                                         │
│         ▼                                                         ��
│ ┌────────────────────────────────────────────────────┐            │
│ │ PasswordService.HashPassword                        │            │
│ │   - Generate 16-byte salt                          │            │
│ │   - Argon2id(password, salt, 64MB, 3 iter, 4x)  │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ UserRepository.Create                              │            │
│ │   - INSERT INTO users (email, password_hash, ...) │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ MagicLinkService.Generate                         │            │
│ │   - Generate 32-byte cryptographic token         │            │
│ │   - Store SHA256(token) in auth_tokens           │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ EventService.PublishUserRegistered                 │            │
│ │   - Publish to ace.auth.user_registered.event   │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│    201 Created                                                   │
│    { "success": true, "data": { "user_id": "...", "message": ...} │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Login Flow (Password)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Login Flow (Password)                                │
│                                                                          │
│  client                                                           │
│     │                                                              │
│     │ POST /auth/login                                               │
│     │ { "email": "user@example.com", "password": "..." }              │
│     ▼                                                              │
│ ┌────────────────┐                                                │
│ │ RateLimiter    │ ── (per IP, reject if exceeded)               │
│ └───────┬────────┘                                                │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────┐                            │
│ │ Login Handler                      │                            │
│ │ 1. Lookup user by email          │                            │
│ │ 2. Check account not locked   │                            │
│ └───────┬──────────────────────────┘                            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ PasswordService.Check                              │            │
│ │   - Verify Argon2id hash                         │            │
│ │   - On failure: increment failed attempts         │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ TokenService.IssueTokens                        │            │
│ │   - Generate JWT (RS256, 15 min)               │            │
│ │   - Generate refresh token (UUID)             │            │
│ │   - Hash and store refresh token in DB          │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ SessionRepository.Create                         │            │
│ │   - Store refresh token hash                 │            │
│ │   - Track device, IP, user agent           │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ EventService.PublishLogin                      │            │
│ │   - Publish to ace.auth.login.event              │            │
│ └────────────────────────────────────────────┬───────┘            │
│         │                                             │
│         ▼                                             │
│    200 OK                                                      │
│    { "success": true, "data": {                                  │
│      "access_token": "eyJ...",                                   │
│      "refresh_token": "rt_...",                                 │
│      "token_type": "Bearer",                                    │
│      "expires_in": 900                                         │
│    }}                                                          │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.3 Protected Request Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                 Protected Request Flow                                 │
│                                                                          │
│  client                                                           │
│     │                                                              │
│     │ GET /api/agents                                              │
│     │ Authorization: Bearer eyJhbGciOiJSUzI1NiIs...                │
│     ▼                                                              │
│ ┌─────────────���─���┐                                                │
│ │ TraceMiddleware│  (extracts trace context)                      │
│ └───────┬────────┘                                                │
│         │                                                         │
│ ┌───────▼────────┐                                                │
│ │ RateLimiter   │  (100 req/min per IP)                          │
│ └───────┬────────┘                                                │
│         │                                                         │
│ ┌───────▼────────────────────────┐                                 │
│ │ AuthMiddleware              │                                 │
│ │                           │                                 │
│ │ 1. Extract Bearer token   │                                 │
│ │ 2. Parse JWT header     │                                 │
│ │ 3. Validate RS256 sig  │                                 │
│ │ 4. Check expiry      │                                 │
│ │ 5. Validate claims   │                                 │
│ │     - iss: "ace-auth"  │                                 │
│ │     - aud: "ace-api"  │                                 │
│ │ 6. Check blacklist │ ─── (Valkey: token:revoked:{jti})         │
│ │ 7. Attach user to ctx│     (subject, email, role)                    │
│ └───────┬────────────────────────┘                                 │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────┐                 │
│ │ [Optional] RBACMiddleware                  │                 │
│ │   - Get user role from context         │                 │
│ │   - Check role in required list   │                 │
│ └───────┬────────────────────────────────┘                         │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────┐                 │
│ │ AgentHandler.List                    │                 │
│ │   - userID := user.ID (from ctx) │                 │
│ │   - Query agents WHERE user_id = ?│                 │
│ └───────┬────────────────────────────────┘                         │
│         │                                                         │
│         ▼                                                         │
│    200 OK                                                      │
│    { "success": true, "data": { "agents": [...] }                 │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.4 Token Refresh Flow

```
┌─────────────────────────────────────────────────���─���─────────────────────┐
│                    Token Refresh Flow                                   │
│                                                                          │
│  client                                                           │
│     │                                                              │
│     │ POST /auth/refresh                                            │
│     │ { "refresh_token": "rt_550e8400-e29b-..." }                  │
│     │                                                              │
│     │ (or Cookie: HttpOnly refresh_token)                            │
│     ▼                                                              │
│ ┌────────────────┐                                                │
│ │ RateLimiter    │                                                 │
│ └───────┬────────┘                                                │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────┐                            │
│ │ Refresh Handler                    │                            │
│ │ 1. Extract refresh token         │                            │
│ │ 2. Lookup session by token  │                            │
│ │ 3. Validate session exists │                            │
│ │ 4. Validate not expired    │                            │
│ │ 5. Validate not revoked   │                            │
│ └───────┬──────────────────────────┘                            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ SessionRepository.Revoke                         │            │
│ │   - Mark old session as revoked                 │            │
│ │   - Add to Valkey blacklist                   │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ TokenService.IssueTokens                        │            │
│ │   - Generate new JWT (RS256, 15 min)        │            │
│ │   - Generate new refresh token (UUID)       │            │
│ │   - Store hash in DB                      │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ SessionRepository.Create                         │            │
│ │   - Store new refresh token hash             │            │
│ └──────���┬���──────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│    200 OK                                                      │
│    { "success": true, "data": {                                  │
│      "access_token": "eyJ...",                                   │
│      "refresh_token": "rt_...",                                 │
│      "token_type": "Bearer",                                    │
│      "expires_in": 900                                         │
│    }}                                                          │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.5 Logout Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Logout Flow                                     │
│                                                                          │
│  client                                                           │
│     │                                                              │
│     │ POST /auth/logout                                             │
│     │ Authorization: Bearer <jwt>                               │
│     ▼                                                              │
│ ┌────────────────┐                                                │
│ │ AuthMiddleware │  (validates JWT)                               │
│ └───────┬────────┘                                                │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────┐                            │
│ │ Get JWT claims from context         │                            │
│ │   - jti (token ID)           │                            │
│ │   - sub (user ID)            │                            │
│ └───────┬──────────────────────────┘                            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ Valkey Blacklist                        │            │
│ │   - SET token:revoked:{jti} "1"       │            │
│ │   - EXPIRE token:revoked:{jti} 900    │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ TokenRepository.RevokeByJTI                 │            │
│ │   - Mark all user tokens as revoked         │            │
│ └───────┬───────────────────────────────────────────┘            │
│         │                                                         │
│         ▼                                                         │
│ ┌────────────────────────────────────────────────────┐            │
│ │ EventService.PublishLogout                    │            │
│ │   - Publish to ace.auth.logout.event            │            │
│ └────────────────────────────────────────────┬───────────┘            │
│         │                                             │
│         ▼                                             │
│    200 OK                                                      │
│    { "success": true, "message": "Logged out successfully" }    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Package Structure

### 4.1 Directory Layout

Auth is implemented as part of `services/api/internal/`, not as a separate service. The following shows all auth-related packages within the API service:

```
services/api/
├── cmd/
│   └── main.go                     # Entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Config struct with AuthConfig fields
│   ├── handler/
│   │   ├── auth/                   # Auth handlers (to be created)
│   │   │   ├── register.go         # POST /auth/register
│   │   │   ├── login.go            # POST /auth/login
│   │   │   ├── logout.go           # POST /auth/logout
│   │   │   ├── refresh.go          # POST /auth/refresh
│   │   │   ├── magic_link.go       # POST /auth/magic-link/*
│   │   │   └── password.go         # POST /auth/password/*
│   │   ├── admin/                  # Admin handlers (to be created)
│   │   │   └── users.go            # GET/PUT/POST /admin/users/*
│   │   └── health.go               # Health endpoints
│   ├── middleware/
│   │   ├── auth.go                 # JWT validation (to be created)
│   │   ├── rbac.go                 # Role checking (to be created)
│   │   └── rate_limit.go           # Rate limiting (to be created)
│   ├── model/
│   │   ├── user.go                 # User model (to be created)
│   │   ├── session.go              # Session model (to be created)
│   │   ├── auth_token.go           # Auth token model (to be created)
│   │   ├── token_claims.go         # JWT claims model (to be created)
│   │   └── errors.go               # Auth-specific errors
│   ├── repository/
│   │   ├── db.go                   # Database connection
│   │   ├── user.go                 # User queries (SQLC)
│   │   ├── session.go              # Session queries (SQLC)
│   │   ├── auth_token.go           # Auth token queries (SQLC)
│   │   └── permission.go           # Permission queries (SQLC)
│   └── service/
│       ├── password.go             # Argon2id hashing (to be created)
│       ├── token.go                # JWT RS256 (to be created)
│       ├── auth.go                 # Auth orchestration (to be created)
│       ├── magic_link.go           # Magic link tokens (to be created)
│       ├── permission.go           # RBAC permissions (to be created)
│       └── event.go                # NATS event publishing (to be created)
├── migrations/
│   └── *_create_auth_tables.go     # Goose migrations (to be created)
├── sqlc.yaml
└── sql/
    └── query.sql                   # SQLC queries
```

> **Note**: All auth code lives in `services/api/internal/`, not in a separate `services/auth/` directory.

### 4.2 Package Descriptions

| Package | Description | Location | Key Types/Functions |
|---------|-------------|-----------|---------------------|
| `handler/auth` | HTTP handlers for auth endpoints | `internal/handler/auth/` | `Register`, `Login`, `Logout`, `Refresh`, `MagicLink`, `Password` |
| `middleware/auth` | JWT validation middleware | `internal/middleware/auth.go` | `RequireAuth`, `GetUserFromContext` |
| `middleware/rbac` | Role-based middleware | `internal/middleware/rbac.go` | `RequireRole` |
| `middleware/rate_limit` | Rate limiting | `internal/middleware/rate_limit.go` | `NewRateLimiter`, `Limit` |
| `model/user` | User domain model | `internal/model/user.go` | `User`, `UserRole`, `UserStatus` |
| `model/session` | Session model | `internal/model/session.go` | `Session` |
| `model/token_claims` | JWT claims | `internal/model/token_claims.go` | `TokenClaims` |
| `repository/user` | User database queries | `internal/repository/user.go` | `CreateUser`, `GetUserByEmail`, `GetUserByID` |
| `repository/session` | Session database queries | `internal/repository/session.go` | `CreateSession`, `GetSession`, `RevokeSession` |
| `service/password` | Argon2id password service | `internal/service/password.go` | `HashPassword`, `VerifyPassword` |
| `service/token` | JWT token service | `internal/service/token.go` | `IssueTokens`, `ValidateAccessToken` |
| `service/auth` | Auth orchestration | `internal/service/auth.go` | `Login`, `Register`, `Logout` |
| `service/magic_link` | Magic link tokens | `internal/service/magic_link.go` | `Generate`, `Verify` |
| `service/permission` | RBAC service | `internal/service/permission.go` | `CheckPermission`, `GrantPermission` |
| `service/event` | Event publishing | `internal/service/event.go` | `PublishLogin`, `PublishLogout` |

> All paths are relative to `services/api/internal/`.

---

## 5. Integration Points

### 5.1 Frontend Integration

| Aspect | Integration |
|--------|------------|
| **Token storage** | httpOnly `Authorization` header with Bearer token |
| **Refresh** | Automatic via cookie or explicit POST `/auth/refresh` |
| **Magic links** | URL: `https://app.example.com/auth/verify?token=...` |
| **Logout** | POST `/auth/logout` with Bearer token |

**Cookie configuration:**
```go
http.SetCookie(w, &http.Cookie{
    Name:     "refresh_token",
    Value:    refreshToken,
    HttpOnly: true,
    Secure:   true,           // HTTPS only
    SameSite: http.SameSiteLax,
    MaxAge:   7 * 24 * 60 * 60,  // 7 days
    Path:     "/",
})
```

### 5.2 Cognitive Engine Integration

The cognitive engine receives user context from JWT claims embedded in the NATS message envelope:

```go
// JWT claims include:
// - sub: user ID
// - email: user email
// - role: user role (admin, user, viewer)

type TokenClaims struct {
    Subject string    `json:"sub"`
    Email   string    `json:"email"`
    Role   string    `json:"role"`
    // ... standard claims
}
```

For cognitive engine operations, the `agentId` is distinct from `userId`. The cognitive engine filters agents by `userId` from claims for ownership validation.

### 5.3 Valkey Integration

| Key Pattern | Purpose | TTL |
|------------|---------|-----|
| `token:revoked:{jti}` | Token blacklist | Token remaining lifetime |
| `ratelimit:auth:login:{ip}` | Login rate limit per IP | Sliding window |
| `ratelimit:auth:login:{email}` | Login rate limit per email | Sliding window |
| `perm:{userId}:{resourceType}:{resourceId}` | Permission cache | 5 minutes |

### 5.4 NATS Integration

Auth events published via `shared/messaging`:

| Event Type | Subject | Consumer |
|-----------|---------|----------|
| Login | `ace.auth.login.event` | Observability |
| Logout | `ace.auth.logout.event` | Observability |
| Failed Login | `ace.auth.failed_login.event` | Security |
| Password Change | `ace.auth.password_change.event` | Audit |
| Role Change | `ace.auth.role_change.event` | Audit |
| Account Suspended | `ace.auth.account_suspended.event` | Audit |
| Account Deleted | `ace.auth.account_deleted.event` | Audit |
| Token Revoked | `ace.auth.token_revoked.event` | Audit |

---

## 6. Deployment Modes

### 6.1 Single-User Mode

For hobbyist Docker Compose deployments (N=1):

- No open registration (`/auth/register` returns 404)
- First user created via seed script gets admin role
- Environment variables: `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD`

**Seed script behavior:**
```go
func runSeed(ctx context.Context, db *pgxpool.Pool) error {
    // Check if any users exist
    var count int
    db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
    
    if count > 0 {
        return nil  // Seed already run
    }
    
    // Create admin user
    _, err := db.Exec(ctx, `
        INSERT INTO users (email, password_hash, role, status)
        VALUES ($1, $2, 'admin', 'active')
    `, adminEmail, hashedPassword)
    
    return err
}
```

### 6.2 Multi-User Mode

For production deployments:

- Open registration via `/auth/register`
- First user gets admin role via seed (or self-promotion)
- Admin can disable registration

**Configuration:**
```go
type AuthConfig struct {
    // ... other fields
    DeploymentMode string `env:"DEPLOYMENT_MODE" default:"multi"` // "single" or "multi"
}
```

---

## 7. Security Considerations

### 7.1 Token Security

| Aspect | Implementation |
|--------|---------------|
| **Algorithm** | RS256 (RSA) everywhere |
| **Access token TTL** | 15 minutes |
| **Refresh token rotation** | Each refresh creates new token, revokes old |
| **Token revocation** | Valkey blacklist for immediate logout |
| **Key storage** | Environment variables (PEM format) |

### 7.2 Password Security

| Aspect | Implementation |
|--------|---------------|
| **Algorithm** | Argon2id |
| **Memory** | 64 MB |
| **Iterations** | 3 |
| **Parallelism** | 4 (match CPU cores) |
| **Salt** | 16 bytes per password |
| **Output** | 32 bytes |

### 7.3 Rate Limiting

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/auth/login` | 5 attempts | 15 min per IP |
| `/auth/register` | 3 attempts | 15 min per IP |
| `/auth/password/reset` | 3 attempts | 1 hour per email |
| General API | 100 requests | 1 minute per IP |

### 7.4 Account Lockout

- After 5 failed login attempts: 15-minute lockout
- Lockout tracked in Valkey: `lockout:{email}`
- Password reset clears failed attempt count

---

## 8. Summary

The users-auth unit integrates into the ACE Framework as authentication middleware within the API service. Key design decisions:

1. **Embedded, not separate**: Auth runs in the API service for MVP simplicity
2. **RS256 everywhere**: JWT signing uses RSA, same in dev and prod
3. **Argon2id**: Modern password hashing with 64MB memory
4. **Refresh token rotation**: Each refresh invalidates old token
5. **Hybrid session state**: Stateless JWT + stateful refresh tokens
6. **Valkey integration**: Token blacklist, rate limiting, permission cache
7. **NATS events**: Auth operations publish to `ace.auth.*.event`

This architecture enables:
- Fast token validation (<5ms latency)
- Immediate token revocation
- Rate limiting on auth endpoints
- Audit trail via NATS events
- Horizontal scaling (stateless JWT validation)