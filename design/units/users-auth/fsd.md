# Functional Specification Document: Users-Auth Unit

<!--
Intent: Define the technical implementation details for the users-auth unit.
Scope: Complete technical blueprint for authentication, authorization, token management, and account operations.
Used by: AI agents implementing the auth system.

References:
- BSD: design/units/users-auth/bsd.md
- User Stories: design/units/users-auth/user_stories.md
- Problem Space: design/units/users-auth/problem_space.md
- ACE Framework: design/README.md
-->

---

## 1. System Overview

### 1.1 Purpose

The **users-auth** unit establishes the complete identity and access management foundation for the ACE Framework. It enables:

- User account management (registration, login, deletion)
- Authentication flows (email/password and OAuth/SSO via Google and GitHub)
- JWT-based session management with refresh token rotation
- Role-based access control (RBAC) with system-level roles
- Resource-level authorization for sharing agents, tools, and configurations
- Auth event emission via NATS for downstream consumption

### 1.2 Architecture Summary

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT LAYER                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                         │
│  │  Browser    │  │  Mobile    │  │  API Client │                         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                         │
└─────────┼────────────────┼────────────────┼────────────────────────────────┘
          │                │                │
          │   Bearer JWT   │                │
          ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API GATEWAY LAYER                                 │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                    Auth Middleware Stack                               │  │
│  │  1. TraceMiddleware (OTel)  →  2. RateLimitMiddleware  →  3. AuthMW   │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AUTH SERVICE (services/auth)                        │
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐          │
│  │  Handler Layer  │  │  Service Layer   │  │ Repository Layer│          │
│  │                 │  │                 │  │                 │          │
│  │ AuthHandler     │  │ AuthService     │  │ UserRepository  │          │
│  │ OAuthHandler   │  │ TokenService    │  │ SessionRepo     │          │
│  │ AdminHandler   │  │ OAuthService    │  │ PermissionRepo  │          │
│  │ SessionHandler │  │ PermissionSvc   │  │ TokenRepo       │          │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘          │
│           │                     │                      │                    │
│           └────────────────────┼──────────────────────┘                    │
│                                │                                             │
│                                ▼                                             │
│                    ┌─────────────────────┐                                  │
│                    │  shared/messaging   │  (NATS publisher)               │
│                    └─────────────────────┘                                  │
│                                                                             │
│                                │                                             │
│                                ▼                                             │
│                    ┌─────────────────────┐                                  │
│                    │    shared/caching   │  (Valkey backend)                │
│                    └─────────────────────┘                                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
          ┌─────────────────────────┼─────────────────────────┐
          │                         │                         │
          ▼                         ▼                         ▼
┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐
│    PostgreSQL    │   │     Valkey       │   │      NATS        │
│                  │   │                  │   │                  │
│ • users          │   │ • Token blacklist│   │ ace.auth.*.event │
│ • oauth_providers│   │ • Rate limits    │   │                  │
│ • sessions       │   │ • Permission cache│   │                  │
│ • roles          │   │ • OAuth state    │   │                  │
│ • permissions    │   │ • Session state   │   │                  │
│ • reset_tokens   │   │                  │   │                  │
│ • verify_tokens  │   │                  │   │                  │
└──────────────────┘   └──────────────────┘   └──────────────────┘
```

### 1.3 Package Layout

```
services/auth/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point
├── internal/
│   ├── handler/
│   │   ├── auth_handler.go         # Registration, login, logout
│   │   ├── oauth_handler.go       # OAuth flows
│   │   ├── password_handler.go    # Password reset, change
│   │   ├── session_handler.go     # Session management
│   │   └── admin_handler.go       # Admin user management
│   ├── service/
│   │   ├── auth_service.go        # Core auth logic
│   │   ├── token_service.go       # JWT generation, validation
│   │   ├── oauth_service.go       # OAuth provider logic
│   │   ├── password_service.go    # Password hashing, validation
│   │   ├── permission_service.go  # RBAC, resource permissions
│   │   └── email_service.go      # Email sending (interface)
│   ├── repository/
│   │   ├── user_repository.go    # User CRUD via SQLC
│   │   ├── session_repository.go  # Session CRUD
│   │   ├── token_repository.go    # Token storage
│   │   └── permission_repository.go # Permission lookups
│   ├── middleware/
│   │   ├── auth_middleware.go     # JWT validation
│   │   ├── rbac_middleware.go    # Role checking
│   │   └── rate_limit_middleware.go # Rate limiting
│   └── model/
│       ├── user.go                # User domain model
│       ├── session.go             # Session model
│       ├── token.go               # Token claims model
│       └── errors.go              # Auth-specific errors
├── migrations/
│   └── 2024XXXXXXX_*.go          # Goose migrations
├── sqlc/
│   ├── query.sql                  # SQLC queries
│   └── models.sql                 # SQLC models
├── config/
│   └── config.go                  # Config struct
└── Makefile
```

---

## 2. Technical Requirements

### 2.1 Functional Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| FR-1 | User registration via email/password | Must | Email verification required |
| FR-2 | User login via email/password | Must | JWT + refresh tokens |
| FR-3 | OAuth login via Google | Must | State parameter validation |
| FR-4 | OAuth login via GitHub | Must | State parameter validation |
| FR-5 | JWT access tokens with configurable lifetime | Must | Default 5-15 minutes |
| FR-6 | Refresh token rotation | Must | Each refresh invalidates old token |
| FR-7 | Token revocation via Valkey blacklist | Must | Immediate invalidation |
| FR-8 | RBAC with admin, user, viewer roles | Must | Role-based middleware |
| FR-9 | Resource-level permissions | Must | view, use, admin levels |
| FR-10 | Password reset via email tokens | Must | Single-use, time-limited |
| FR-11 | Email verification | Must | Single-use token |
| FR-12 | Rate limiting on auth endpoints | Must | Per-IP and per-email |
| FR-13 | Account lockout after failed attempts | Must | 5 attempts → 15 min lockout |
| FR-14 | Auth event emission via NATS | Must | All auth operations |
| FR-15 | Single-user mode auto-admin | Must | First user = admin |
| FR-16 | Multi-user mode open registration | Must | Deployment config |
| FR-17 | Account suspension | Should | Admin action |
| FR-18 | OAuth provider linking | Should | Multiple providers per user |

### 2.2 Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1 | Login response time | < 200ms |
| NFR-2 | Token validation latency | < 5ms |
| NFR-3 | Password hashing | Argon2id, memory ≥ 64MB |
| NFR-4 | Access token lifetime | 5-15 minutes (configurable) |
| NFR-5 | Refresh token lifetime | 7 days |
| NFR-6 | Rate limit (login) | 10 attempts/minute per IP |
| NFR-7 | Rate limit (password reset) | 3 requests/hour per email |
| NFR-8 | JWT algorithm | RS256 (production), HS256 (dev) |
| NFR-9 | Auth event delivery | > 99.9% to NATS |
| NFR-10 | Horizontal scaling | Stateless middleware |

---

## 3. Data Models

### 3.1 Database Schema

#### E-R Diagram

```
┌─────────────────┐       ┌─────────────────────┐
│     roles       │       │       users         │
├─────────────────┤       ├─────────────────────┤
│ id (PK)         │       │ id (PK)             │
│ name            │◄──────│ role_id (FK)        │
│ permissions     │       │ email (UNIQUE)      │
│ created_at      │       │ password_hash       │
└─────────────────┘       │ email_verified     │
                           │ status             │
                           │ suspended_at       │
                           │ suspended_reason   │
                           │ deleted_at        │
                           │ created_at        │
                           │ updated_at        │
                           └─────────┬──────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              │                      │                      │
              ▼                      ▼                      ▼
┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐
│  oauth_providers    │  │      sessions        │  │  resource_permissions│
├─────────────────────┤  ├─────────────────────┤  ├─────────────────────┤
│ id (PK)             │  │ id (PK)             │  │ id (PK)             │
│ user_id (FK)        │  │ user_id (FK)        │  │ user_id (FK)        │
│ provider            │  │ refresh_token_hash  │  │ resource_type       │
│ provider_user_id   │  │ user_agent          │  │ resource_id         │
│ access_token_enc   │  │ ip_address          │  │ permission_level    │
│ refresh_token_enc  │  │ last_used_at        │  │ granted_by (FK)     │
│ expires_at         │  │ expires_at          │  │ created_at          │
│ created_at         │  │ created_at          │  └─────────────────────┘
└─────────────────────┘  └─────────────────────┘
              │
              ▼
┌─────────────────────────┐  ┌─────────────────────────┐
│ email_verification_tokens│  │  password_reset_tokens  │
├─────────────────────────┤  ├─────────────────────────┤
│ id (PK)                 │  │ id (PK)                 │
│ user_id (FK)            │  │ user_id (FK)            │
│ token_hash              │  │ token_hash             │
│ expires_at              │  │ expires_at             │
│ used_at                 │  │ used_at                │
│ created_at              │  │ created_at             │
└─────────────────────────┘  └─────────────────────────┘
```

#### SQL Schema (Goose Migration)

```go
// migrations/20240401000001_create_auth_tables.go
package migrations

import (
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(upCreateAuthTables, downCreateAuthTables)
}

func upCreateAuthTables(tx *sql.Tx) error {
    // Create users table (using VARCHAR + CHECK constraints instead of ENUM for PostgreSQL compatibility)
    _, err := tx.Exec(`
        CREATE TABLE users (
            id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
            email           VARCHAR(255) NOT NULL,
            password_hash   VARCHAR(255),
            email_verified  BOOLEAN     NOT NULL DEFAULT FALSE,
            role            VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'viewer')),
            status          VARCHAR(30) NOT NULL DEFAULT 'pending_verification' CHECK (status IN ('pending_verification', 'active', 'suspended')),
            suspended_at    TIMESTAMPTZ,
            suspended_reason TEXT,
            deleted_at      TIMESTAMPTZ,
            created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
            updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
            CONSTRAINT users_email_unique UNIQUE (email)
        );
    `)
    if err != nil {
        return err
    }

    // Create oauth_providers table
    _, err = tx.Exec(`
        CREATE TABLE oauth_providers (
            id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            provider          VARCHAR(50) NOT NULL,  -- 'google', 'github'
            provider_user_id  VARCHAR(255) NOT NULL, -- OAuth subject ID
            access_token_enc  TEXT,
            refresh_token_enc TEXT,
            expires_at        TIMESTAMPTZ,
            created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT oauth_provider_unique UNIQUE (user_id, provider),
            CONSTRAINT oauth_provider_id_unique UNIQUE (provider, provider_user_id)
        );
    `)
    if err != nil {
        return err
    }

    // Create sessions table
    _, err = tx.Exec(`
        CREATE TABLE sessions (
            id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            refresh_token_hash VARCHAR(255) NOT NULL,
            user_agent        TEXT,
            ip_address        INET,
            last_used_at      TIMESTAMPTZ,
            expires_at        TIMESTAMPTZ NOT NULL,
            created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Create email_verification_tokens table
    _, err = tx.Exec(`
        CREATE TABLE email_verification_tokens (
            id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token_hash  VARCHAR(255) NOT NULL,
            expires_at  TIMESTAMPTZ NOT NULL,
            used_at     TIMESTAMPTZ,
            created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Create password_reset_tokens table
    _, err = tx.Exec(`
        CREATE TABLE password_reset_tokens (
            id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token_hash  VARCHAR(255) NOT NULL,
            expires_at  TIMESTAMPTZ NOT NULL,
            used_at     TIMESTAMPTZ,
            created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Create resource_permissions table
    _, err = tx.Exec(`
        CREATE TABLE resource_permissions (
            id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            resource_type     VARCHAR(50) NOT NULL,  -- 'agent', 'tool', 'skill', 'config'
            resource_id       UUID        NOT NULL,
            permission_level   VARCHAR(20) NOT NULL, -- 'view', 'use', 'admin'
            granted_by        UUID        REFERENCES users(id),
            created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT resource_permission_unique UNIQUE (user_id, resource_type, resource_id)
        );
    `)
    if err != nil {
        return err
    }

    // Create indexes
    _, err = tx.Exec(`
        CREATE INDEX idx_users_email ON users(email);
        CREATE INDEX idx_users_status ON users(status);
        CREATE INDEX idx_oauth_providers_user_id ON oauth_providers(user_id);
        CREATE INDEX idx_oauth_providers_provider_user_id ON oauth_providers(provider, provider_user_id);
        CREATE INDEX idx_sessions_user_id ON sessions(user_id);
        CREATE INDEX idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);
        CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
        CREATE INDEX idx_resource_permissions_user_id ON resource_permissions(user_id);
        CREATE INDEX idx_resource_permissions_resource ON resource_permissions(resource_type, resource_id);
    `)
    if err != nil {
        return err
    }

    // Create updated_at trigger
    _, err = tx.Exec(`
        CREATE TRIGGER set_users_updated_at
            BEFORE UPDATE ON users
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    `)

    return err
}

func downCreateAuthTables(tx *sql.Tx) error {
    _, err := tx.Exec(`
        DROP TABLE IF EXISTS resource_permissions;
        DROP TABLE IF EXISTS password_reset_tokens;
        DROP TABLE IF EXISTS email_verification_tokens;
        DROP TABLE IF EXISTS sessions;
        DROP TABLE IF EXISTS oauth_providers;
        DROP TABLE IF EXISTS users;
    `)
    return err
}
```

### 3.2 Domain Models (Go)

```go
// internal/model/user.go
package model

import (
    "time"

    "github.com/google/uuid"
)

type UserRole string

const (
    RoleAdmin  UserRole = "admin"
    RoleUser  UserRole = "user"
    RoleViewer UserRole = "viewer"
)

type UserStatus string

const (
    StatusPendingVerification UserStatus = "pending_verification"
    StatusActive             UserStatus = "active"
    StatusSuspended          UserStatus = "suspended"
)

type User struct {
    ID            uuid.UUID  `json:"id"`
    Email         string     `json:"email"`
    PasswordHash  *string    `json:"-"` // Never exposed in JSON
    EmailVerified bool       `json:"email_verified"`
    Role          UserRole   `json:"role"`
    Status        UserStatus `json:"status"`
    SuspendedAt   *time.Time `json:"suspended_at,omitempty"`
    SuspendedReason *string  `json:"suspended_reason,omitempty"`
    DeletedAt     *time.Time `json:"deleted_at,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
}

type OAuthProvider struct {
    ID               uuid.UUID  `json:"id"`
    UserID           uuid.UUID  `json:"user_id"`
    Provider         string     `json:"provider"` // "google", "github"
    ProviderUserID   string     `json:"provider_user_id"`
    AccessTokenEnc   string     `json:"-"`
    RefreshTokenEnc  *string    `json:"-"`
    ExpiresAt        *time.Time `json:"expires_at,omitempty"`
    CreatedAt        time.Time  `json:"created_at"`
}

type Session struct {
    ID                uuid.UUID  `json:"id"`
    UserID            uuid.UUID  `json:"user_id"`
    RefreshTokenHash  string     `json:"-"`
    UserAgent         string     `json:"user_agent,omitempty"`
    IPAddress         string     `json:"ip_address,omitempty"`
    LastUsedAt        *time.Time `json:"last_used_at,omitempty"`
    ExpiresAt         time.Time  `json:"expires_at"`
    CreatedAt         time.Time  `json:"created_at"`
}

type PermissionLevel string

const (
    PermissionView  PermissionLevel = "view"
    PermissionUse   PermissionLevel = "use"
    PermissionAdmin PermissionLevel = "admin"
)

type ResourcePermission struct {
    ID              uuid.UUID       `json:"id"`
    UserID          uuid.UUID       `json:"user_id"`
    ResourceType    string          `json:"resource_type"`
    ResourceID      uuid.UUID       `json:"resource_id"`
    PermissionLevel PermissionLevel  `json:"permission_level"`
    GrantedBy       *uuid.UUID      `json:"granted_by,omitempty"`
    CreatedAt       time.Time       `json:"created_at"`
}

type EmailVerificationToken struct {
    ID         uuid.UUID  `json:"id"`
    UserID     uuid.UUID  `json:"user_id"`
    TokenHash  string     `json:"-"`
    ExpiresAt  time.Time  `json:"expires_at"`
    UsedAt     *time.Time `json:"used_at,omitempty"`
    CreatedAt  time.Time  `json:"created_at"`
}

type PasswordResetToken struct {
    ID         uuid.UUID  `json:"id"`
    UserID     uuid.UUID  `json:"user_id"`
    TokenHash  string     `json:"-"`
    ExpiresAt  time.Time  `json:"expires_at"`
    UsedAt     *time.Time `json:"used_at,omitempty"`
    CreatedAt  time.Time  `json:"created_at"`
}
```

---

## 4. API Specification

### 4.1 Authentication Endpoints

#### POST /auth/register

Register a new user with email and password.

**Security:** Public

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecureP@ss123"
}
```

**Response (201 Created):**
```json
{
  "success": true,
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "message": "Registration successful. Please check your email to verify your account."
  }
}
```

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | INVALID_EMAIL | Email format invalid |
| 400 | WEAK_PASSWORD | Password doesn't meet complexity |
| 409 | EMAIL_EXISTS | Email already registered |
| 429 | RATE_LIMITED | Too many attempts |

---

#### POST /auth/login

Authenticate with email and password.

**Security:** Public

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecureP@ss123"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "rt_550e8400-e29b-41d4-a716-446655440001",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | INVALID_REQUEST | Missing fields |
| 401 | INVALID_CREDENTIALS | Wrong email/password |
| 403 | EMAIL_NOT_VERIFIED | Email not verified |
| 403 | ACCOUNT_SUSPENDED | Account suspended |
| 429 | ACCOUNT_LOCKED | Too many failed attempts |
| 429 | RATE_LIMITED | Too many attempts |

---

#### POST /auth/logout

Invalidate current session.

**Security:** Bearer Token Required

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Logged out successfully."
}
```

---

#### POST /auth/refresh

Refresh access token using refresh token.

**Security:** Public (refresh token in body or cookie)

**Request:**
```json
{
  "refresh_token": "rt_550e8400-e29b-41d4-a716-446655440001"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "rt_660e8400-e29b-41d4-a716-446655440001",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 401 | INVALID_REFRESH_TOKEN | Token invalid or revoked |
| 401 | REFRESH_TOKEN_EXPIRED | Token expired |

---

### 4.2 Password Management Endpoints

#### POST /auth/password/reset/request

Request password reset email.

**Security:** Public

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "If an account exists with this email, a password reset link has been sent."
}
```

> Note: Always returns success to prevent email enumeration attacks.

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 429 | RATE_LIMITED | Too many reset requests |

---

#### POST /auth/password/reset/confirm

Reset password using token.

**Security:** Public (token in body)

**Request:**
```json
{
  "token": "550e8400-e29b-41d4-a716-446655440002",
  "password": "NewSecureP@ss456"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Password has been reset. Please log in with your new password."
}
```

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | INVALID_TOKEN | Token invalid |
| 400 | TOKEN_EXPIRED | Token expired |
| 400 | TOKEN_USED | Token already used |
| 400 | WEAK_PASSWORD | Password doesn't meet requirements |

---

#### POST /auth/password/change

Change password while logged in.

**Security:** Bearer Token Required

**Request:**
```json
{
  "current_password": "OldP@ss123",
  "new_password": "NewSecureP@ss456"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Password changed successfully. All other sessions have been invalidated."
}
```

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | INVALID_CURRENT_PASSWORD | Current password wrong |
| 400 | WEAK_PASSWORD | New password doesn't meet requirements |

---

### 4.3 Email Verification Endpoints

#### POST /auth/email/verify

Verify email address with token.

**Security:** Public (token in body)

**Request:**
```json
{
  "token": "550e8400-e29b-41d4-a716-446655440003"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Email verified successfully. Your account is now active."
}
```

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | INVALID_TOKEN | Token invalid |
| 400 | TOKEN_EXPIRED | Token expired |
| 400 | TOKEN_USED | Token already used |

---

### 4.4 OAuth Endpoints

#### GET /auth/oauth/google

Initiate Google OAuth flow.

**Security:** Public

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| redirect_url | string | No | URL to redirect after OAuth |

**Response:** 302 Redirect to Google OAuth

---

#### GET /auth/oauth/google/callback

Google OAuth callback.

**Security:** Public

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| code | string | OAuth authorization code |
| state | string | CSRF state parameter |

**Response:** 302 Redirect to frontend with tokens in URL fragment

---

#### GET /auth/oauth/github

Initiate GitHub OAuth flow.

**Security:** Public

**Response:** 302 Redirect to GitHub OAuth

---

#### GET /auth/oauth/github/callback

GitHub OAuth callback.

**Security:** Public

**Response:** 302 Redirect to frontend with tokens in URL fragment

---

#### POST /auth/oauth/link

Link OAuth provider to existing account.

**Security:** Bearer Token Required

**Request:**
```json
{
  "provider": "google"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "OAuth provider linked successfully."
}
```

---

#### POST /auth/oauth/unlink

Unlink OAuth provider from account.

**Security:** Bearer Token Required

**Request:**
```json
{
  "provider": "google"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "OAuth provider unlinked successfully."
}
```

**Error Responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | CANNOT_REMOVE_LAST_AUTH_METHOD | No other auth method available |

---

### 4.5 Session Management Endpoints

#### GET /auth/me

Get current user profile.

**Security:** Bearer Token Required

**Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "email_verified": true,
    "role": "user",
    "status": "active",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

---

#### GET /auth/me/sessions

List active sessions.

**Security:** Bearer Token Required

**Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "sessions": [
      {
        "id": "session-uuid",
        "user_agent": "Mozilla/5.0...",
        "ip_address": "192.168.1.1",
        "last_used_at": "2024-04-09T12:00:00Z",
        "created_at": "2024-04-01T10:00:00Z"
      }
    ]
  }
}
```

---

#### DELETE /auth/me/sessions/:id

Revoke specific session.

**Security:** Bearer Token Required

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Session revoked successfully."
}
```

---

### 4.6 Admin Endpoints

#### GET /admin/users

List all users (paginated).

**Security:** Bearer Token Required (Admin role)

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| page | int | 1 | Page number |
| per_page | int | 20 | Items per page |
| status | string | all | Filter by status |

**Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "users": [
      {
        "id": "uuid",
        "email": "user@example.com",
        "role": "user",
        "status": "active",
        "created_at": "2024-01-15T10:30:00Z"
      }
    ]
  },
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

---

#### GET /admin/users/:id

Get user details.

**Security:** Bearer Token Required (Admin role)

**Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "email": "user@example.com",
    "email_verified": true,
    "role": "user",
    "status": "active",
    "oauth_providers": ["google"],
    "sessions_count": 2,
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

---

#### PUT /admin/users/:id/role

Update user role.

**Security:** Bearer Token Required (Admin role)

**Request:**
```json
{
  "role": "admin"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User role updated successfully.",
  "data": {
    "user_id": "uuid",
    "new_role": "admin"
  }
}
```

---

#### POST /admin/users/:id/suspend

Suspend user account.

**Security:** Bearer Token Required (Admin role)

**Request:**
```json
{
  "reason": "Violation of terms of service"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User has been suspended. All active sessions have been invalidated."
}
```

---

#### POST /admin/users/:id/restore

Restore a suspended user account.

**Security:** Bearer Token Required (Admin role)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User account has been restored successfully.",
  "data": {
    "user_id": "uuid",
    "status": "active"
  }
}
```

---

#### POST /admin/users/:id/delete

Soft-delete user account.

**Security:** Bearer Token Required (Admin role)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User account has been deleted."
}
```

---

## 5. Authentication Flow

### 5.1 JWT Token Strategy

#### Token Structure

**Access Token Claims:**
```json
{
  "iss": "ace-auth",
  "sub": "user-uuid",
  "aud": ["ace-api"],
  "exp": 1712600000,
  "iat": 1712599400,
  "jti": "token-uuid",
  "role": "user",
  "email": "user@example.com"
}
```

**Refresh Token:**
- Opaque token (UUID v4)
- Stored as hash in PostgreSQL `sessions` table
- Revoked via Valkey blacklist

#### Token Lifecycle

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           TOKEN LIFECYCLE                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. LOGIN                                                                    │
│  ┌──────────┐    ┌─────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Validate │───>│ Hash & Store│───>│ Create JWT   │───>│ Return tokens│   │
│  │ password │    │ refresh token│   │ access token │    │              │   │
│  └──────────┘    └─────────────┘    └──────────────┘    └──────────────┘   │
│                                                                              │
│  2. ACCESS TOKEN USE                                                        │
│  ┌──────────┐    ┌─────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Extract  │───>│ Validate    │───>│ Check        │───>│ Process      │   │
│  │ Bearer   │    │ signature   │    │ not revoked  │    │ request      │   │
│  └──────────┘    └─────────────┘    └──────────────┘    └──────────────┘   │
│                                               │                             │
│                                               │ Cache miss → DB check       │
│                                                                              │
│  3. TOKEN REFRESH                                                           │
│  ┌──────────┐    ┌─────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Validate │───>│ Revoke old  │───>│ Store new    │───>│ Return new   │   │
│  │ refresh  │    │ refresh     │    │ refresh      │    │ tokens       │   │
│  │ token    │    │ (Valkey)   │    │ hash         │    │              │   │
│  └──────────┘    └─────────────┘    └──────────────┘    └──────────────┘   │
│                                                                              │
│  4. LOGOUT                                                                  │
│  ┌──────────┐    ┌─────────────┐    ┌──────────────┐                        │
│  │ Extract  │───>│ Revoke      │───>│ Publish      │                        │
│  │ session  │    │ refresh     │    │ logout event │                        │
│  └──────────┘    └─────────────┘    └──────────────┘                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Token Configuration

```go
// config/config.go
type Config struct {
    Auth AuthConfig `envprefix:"AUTH_"`
}

type AuthConfig struct {
    // JWT Configuration
    AccessTokenTTL  time.Duration `env:"ACCESS_TOKEN_TTL" envdefault:"15m"`
    RefreshTokenTTL time.Duration `env:"REFRESH_TOKEN_TTL" envdefault:"168h"` // 7 days
    
    // JWT Signing
    JWTAlgorithm    string `env:"JWT_ALGORITHM" envdefault:"RS256"`
    JWTPrivateKey   string `env:"JWT_PRIVATE_KEY"` // PEM encoded for RS256
    JWTPublicKey    string `env:"JWT_PUBLIC_KEY"`  // PEM encoded for RS256
    JWTSecret       string `env:"JWT_SECRET"`      // For HS256 (development only)
    
    // Rate Limiting
    LoginRateLimitPerIP     int `env:"LOGIN_RATE_LIMIT_PER_IP" envdefault:"10"`      // per minute
    LoginRateLimitPerEmail  int `env:"LOGIN_RATE_LIMIT_PER_EMAIL" envdefault:"5"`   // per 5 minutes
    PasswordResetRateLimit  int `env:"PASSWORD_RESET_RATE_LIMIT" envdefault:"3"`     // per hour
    LockoutThreshold       int `env:"LOCKOUT_THRESHOLD" envdefault:"5"`
    LockoutDuration        time.Duration `env:"LOCKOUT_DURATION" envdefault:"15m"`
    
    // Password
    PasswordMinLength    int `env:"PASSWORD_MIN_LENGTH" envdefault:"8"`
    PasswordRequireUpper bool `env:"PASSWORD_REQUIRE_UPPER" envdefault:"true"`
    PasswordRequireLower bool `env:"PASSWORD_REQUIRE_LOWER" envdefault:"true"`
    PasswordRequireNumber bool `env:"PASSWORD_REQUIRE_NUMBER" envdefault:"true"`
    PasswordRequireSymbol bool `env:"PASSWORD_REQUIRE_SYMBOL" envdefault:"false"`
    
    // Tokens
    EmailTokenTTL     time.Duration `env:"EMAIL_TOKEN_TTL" envdefault:"24h"`
    ResetTokenTTL     time.Duration `env:"RESET_TOKEN_TTL" envdefault:"1h"`
    
    // Deployment
    DeploymentMode string `env:"DEPLOYMENT_MODE" envdefault:"multi"` // "single" or "multi"
    
    // OAuth
    GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
    GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
    GitHubClientID     string `env:"GITHUB_CLIENT_ID"`
    GitHubClientSecret string `env:"GITHUB_CLIENT_SECRET"`
}
```

---

### 5.2 Middleware Implementation

```go
// internal/middleware/auth_middleware.go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/google/uuid"
    "github.com/go-chi/chi/v5"

    "ace/services/auth/internal/model"
    "ace/services/auth/internal/service"
    "ace/shared/caching"
    "ace/shared/messaging"
    "ace/shared/telemetry"
)

type contextKey string

const (
    UserContextKey   contextKey = "user"
    TokenContextKey  contextKey = "token"
)

type AuthMiddleware struct {
    tokenService *service.TokenService
    cache        *caching.Cache
}

func NewAuthMiddleware(tokenSvc *service.TokenService, cache *caching.Cache) *AuthMiddleware {
    return &AuthMiddleware{
        tokenService: tokenSvc,
        cache:        cache,
    }
}

// RequireAuth validates JWT and attaches user to context
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        // Extract Bearer token
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            response.Unauthorized(w, "UNAUTHORIZED", "Missing authorization header")
            return
        }

        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
            response.Unauthorized(w, "INVALID_TOKEN", "Invalid authorization header format")
            return
        }
        tokenString := parts[1]

        // Validate JWT
        claims, err := m.tokenService.ValidateAccessToken(ctx, tokenString)
        if err != nil {
            switch {
            case errors.Is(err, service.ErrTokenExpired):
                response.Unauthorized(w, "TOKEN_EXPIRED", "Access token has expired")
            case errors.Is(err, service.ErrTokenRevoked):
                response.Unauthorized(w, "TOKEN_REVOKED", "Access token has been revoked")
            case errors.Is(err, service.ErrTokenInvalid):
                response.Unauthorized(w, "INVALID_TOKEN", "Access token is invalid")
            default:
                response.Unauthorized(w, "UNAUTHORIZED", "Authentication failed")
            }
            return
        }

        // Check token blacklist
        revoked, err := m.cache.Get(ctx, "blacklist:"+claims.JTI)
        if err == nil && revoked != nil {
            response.Unauthorized(w, "TOKEN_REVOKED", "Access token has been revoked")
            return
        }

        // Attach claims to context
        ctx = context.WithValue(ctx, TokenContextKey, claims)
        ctx = context.WithValue(ctx, UserContextKey, &model.User{
            ID:    claims.Subject,
            Email: claims.Email,
            Role:  claims.Role,
        })

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// RequireRole checks if user has required role
func RequireRole(roles ...model.UserRole) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, ok := r.Context().Value(UserContextKey).(*model.User)
            if !ok {
                response.Unauthorized(w, "UNAUTHORIZED", "User not authenticated")
                return
            }

            for _, role := range roles {
                if user.Role == role {
                    next.ServeHTTP(w, r)
                    return
                }
            }

            response.Forbidden(w, "INSUFFICIENT_PERMISSIONS", "You don't have permission to access this resource")
        })
    }
}

// GetUserFromContext extracts user from request context
func GetUserFromContext(ctx context.Context) *model.User {
    user, _ := ctx.Value(UserContextKey).(*model.User)
    return user
}

// GetTokenClaimsFromContext extracts token claims from request context
func GetTokenClaimsFromContext(ctx context.Context) *model.TokenClaims {
    claims, _ := ctx.Value(TokenContextKey).(*model.TokenClaims)
    return claims
}
```

---

## 6. Authorization Model

### 6.1 RBAC Permissions Matrix

| Permission | admin | user | viewer |
|------------|-------|------|--------|
| **Users** ||||
| Read own profile | ✓ | ✓ | ✗ |
| Update own profile | ✓ | ✓ | ✗ |
| Delete own account | ✓ | ✓ | ✗ |
| List all users | ✓ | ✗ | ✗ |
| Update user roles | ✓ | ✗ | ✗ |
| Suspend users | ✓ | ✗ | ✗ |
| Delete users | ✓ | ✗ | ✗ |
| **Sessions** ||||
| View own sessions | ✓ | ✓ | ✗ |
| Revoke own sessions | ✓ | ✓ | ✗ |
| View all sessions | ✓ | ✗ | ✗ |
| Revoke any session | ✓ | ✗ | ✗ |
| **Resources** ||||
| Create own resources | ✓ | ✓ | ✗ |
| Read own resources | ✓ | ✓ | ✗ |
| Update own resources | ✓ | ✓ | ✗ |
| Delete own resources | ✓ | ✓ | ✗ |
| Read shared resources | ✓ | ✓ | ✓ |
| Execute shared resources | ✓ | ✓ | ✗ |
| Manage shared permissions | ✓ | ✓ | ✗ |

### 6.2 Resource-Level Permissions

```go
// internal/service/permission_service.go
package service

import (
    "context"
    "errors"

    "github.com/google/uuid"

    "ace/services/auth/internal/model"
    "ace/services/auth/internal/repository"
    "ace/shared/caching"
)

var (
    ErrResourceAccessDenied = errors.New("resource access denied")
    ErrResourceNotFound    = errors.New("resource not found")
)

type PermissionService struct {
    permissionRepo *repository.PermissionRepository
    cache          *caching.Cache
}

func NewPermissionService(repo *repository.PermissionRepository, cache *caching.Cache) *PermissionService {
    return &PermissionService{
        permissionRepo: repo,
        cache:          cache,
    }
}

// CheckResourcePermission verifies if user can access a resource
func (s *PermissionService) CheckResourcePermission(
    ctx context.Context,
    userID uuid.UUID,
    resourceType string,
    resourceID uuid.UUID,
    requiredLevel model.PermissionLevel,
) error {
    // Build cache key
    cacheKey := "perm:" + userID.String() + ":" + resourceType + ":" + resourceID.String()

    // Check cache first
    cached, err := s.cache.Get(ctx, cacheKey)
    if err == nil && cached != nil {
        // Parse cached permission and check if sufficient
        cachedLevel := model.PermissionLevel(cached)
        if s.hasPermission(cachedLevel, requiredLevel) {
            return nil
        }
        return ErrResourceAccessDenied
    }

    // Query database
    permission, err := s.permissionRepo.GetResourcePermission(ctx, userID, resourceType, resourceID)
    if err != nil {
        if errors.Is(err, repository.ErrPermissionNotFound) {
            return ErrResourceAccessDenied
        }
        return err
    }

    // Check permission level
    if !s.hasPermission(permission.PermissionLevel, requiredLevel) {
        return ErrResourceAccessDenied
    }

    // Cache result
    s.cache.Set(ctx, cacheKey, []byte(permission.PermissionLevel), caching.WithTTL(5*time.Minute))

    return nil
}

// hasPermission checks if granted level satisfies required level
func (s *PermissionService) hasPermission(granted, required model.PermissionLevel) bool {
    // Permission hierarchy: admin > use > view
    levelValues := map[model.PermissionLevel]int{
        model.PermissionView:  1,
        model.PermissionUse:   2,
        model.PermissionAdmin: 3,
    }

    return levelValues[granted] >= levelValues[required]
}

// GrantPermission grants a permission to a user
func (s *PermissionService) GrantPermission(
    ctx context.Context,
    userID uuid.UUID,
    resourceType string,
    resourceID uuid.UUID,
    permissionLevel model.PermissionLevel,
    grantedBy uuid.UUID,
) error {
    permission := &model.ResourcePermission{
        ID:              uuid.New(),
        UserID:          userID,
        ResourceType:    resourceType,
        ResourceID:      resourceID,
        PermissionLevel: permissionLevel,
        GrantedBy:       &grantedBy,
    }

    err := s.permissionRepo.UpsertResourcePermission(ctx, permission)
    if err != nil {
        return err
    }

    // Invalidate cache
    cacheKey := "perm:" + userID.String() + ":" + resourceType + ":" + resourceID.String()
    s.cache.Delete(ctx, cacheKey)

    return nil
}

// RevokePermission removes a permission
func (s *PermissionService) RevokePermission(
    ctx context.Context,
    userID uuid.UUID,
    resourceType string,
    resourceID uuid.UUID,
) error {
    err := s.permissionRepo.DeleteResourcePermission(ctx, userID, resourceType, resourceID)
    if err != nil {
        return err
    }

    // Invalidate cache
    cacheKey := "perm:" + userID.String() + ":" + resourceType + ":" + resourceID.String()
    s.cache.Delete(ctx, cacheKey)

    return nil
}
```

---

## 7. Event Schema

### 7.1 NATS Auth Events

Auth events use the subject pattern `ace.auth.<event_type>.event` and are published via `shared/messaging`.

#### Event Envelope

```json
{
  "event_id": "uuid",
  "event_type": "login",
  "user_id": "uuid",
  "timestamp": "2024-04-09T12:00:00Z",
  "metadata": {
    "ip_address": "192.168.1.1",
    "user_agent": "Mozilla/5.0...",
    "provider": "email|google|github",
    "session_id": "uuid",
    "role": "user",
    "reason": "optional for suspend/delete"
  }
}
```

#### Event Types

| Event Type | Subject | Trigger |
|------------|---------|---------|
| login | `ace.auth.login.event` | Successful authentication |
| logout | `ace.auth.logout.event` | User logout |
| failed_login | `ace.auth.failed_login.event` | Invalid credentials |
| account_locked | `ace.auth.account_locked.event` | Account locked after failures |
| password_change | `ace.auth.password_change.event` | Password updated |
| email_verified | `ace.auth.email_verified.event` | Email verification completed |
| user_registered | `ace.auth.user_registered.event` | New user registration |
| role_change | `ace.auth.role_change.event` | User role modified |
| account_suspended | `ace.auth.account_suspended.event` | User account suspended |
| account_restored | `ace.auth.account_restored.event` | User account restored |
| account_deleted | `ace.auth.account_deleted.event` | User account soft-deleted |
| oauth_linked | `ace.auth.oauth_linked.event` | OAuth provider linked |
| oauth_unlinked | `ace.auth.oauth_unlinked.event` | OAuth provider unlinked |
| token_revoked | `ace.auth.token_revoked.event` | Token/session revoked |

#### Go Event Types

```go
// internal/model/events.go
package model

import (
    "time"

    "github.com/google/uuid"
)

type AuthEventType string

const (
    EventLogin           AuthEventType = "login"
    EventLogout          AuthEventType = "logout"
    EventFailedLogin     AuthEventType = "failed_login"
    EventAccountLocked   AuthEventType = "account_locked"
    EventPasswordChange  AuthEventType = "password_change"
    EventEmailVerified   AuthEventType = "email_verified"
    EventUserRegistered  AuthEventType = "user_registered"
    EventRoleChange      AuthEventType = "role_change"
    EventAccountSuspended AuthEventType = "account_suspended"
    EventAccountRestored AuthEventType = "account_restored"
    EventAccountDeleted  AuthEventType = "account_deleted"
    EventOAuthLinked     AuthEventType = "oauth_linked"
    EventOAuthUnlinked   AuthEventType = "oauth_unlinked"
    EventTokenRevoked    AuthEventType = "token_revoked"
)

type AuthEvent struct {
    EventID   uuid.UUID              `json:"event_id"`
    EventType AuthEventType          `json:"event_type"`
    UserID    uuid.UUID              `json:"user_id"`
    Timestamp time.Time              `json:"timestamp"`
    Metadata  AuthEventMetadata      `json:"metadata"`
}

type AuthEventMetadata struct {
    IPAddress   string  `json:"ip_address,omitempty"`
    UserAgent   string  `json:"user_agent,omitempty"`
    Provider    string  `json:"provider,omitempty"`
    SessionID   string  `json:"session_id,omitempty"`
    Role        string  `json:"role,omitempty"`
    Reason      string  `json:"reason,omitempty"`
    OAuthProvider string `json:"oauth_provider,omitempty"`
    Attempts    int     `json:"attempts,omitempty"`
}
```

#### Event Publisher

```go
// internal/service/event_service.go
package service

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/google/uuid"

    "ace/services/auth/internal/model"
    "ace/shared/messaging"
)

type EventService struct {
    messagingClient *messaging.Client
}

func NewEventService(client *messaging.Client) *EventService {
    return &EventService{messagingClient: client}
}

func (s *EventService) PublishAuthEvent(ctx context.Context, event *model.AuthEvent) error {
    subject := fmt.Sprintf("ace.auth.%s.event", event.EventType)

    payload, err := json.Marshal(event)
    if err != nil {
        return err
    }

    return s.messagingClient.Publish(
        ctx,
        subject,
        event.EventID.String(),  // correlationID
        event.UserID.String(),  // agentID (used for user correlation)
        "",                     // cycleID (not applicable for auth)
        "auth-service",
        payload,
    )
}

// Convenience methods for common events

func (s *EventService) PublishLoginEvent(ctx context.Context, userID uuid.UUID, metadata model.AuthEventMetadata) error {
    return s.PublishAuthEvent(ctx, &model.AuthEvent{
        EventID:   uuid.New(),
        EventType: model.EventLogin,
        UserID:    userID,
        Timestamp: time.Now(),
        Metadata:  metadata,
    })
}

func (s *EventService) PublishLogoutEvent(ctx context.Context, userID uuid.UUID, metadata model.AuthEventMetadata) error {
    return s.PublishAuthEvent(ctx, &model.AuthEvent{
        EventID:   uuid.New(),
        EventType: model.EventLogout,
        UserID:    userID,
        Timestamp: time.Now(),
        Metadata:  metadata,
    })
}

func (s *EventService) PublishPasswordChangeEvent(ctx context.Context, userID uuid.UUID, metadata model.AuthEventMetadata) error {
    return s.PublishAuthEvent(ctx, &model.AuthEvent{
        EventID:   uuid.New(),
        EventType: model.EventPasswordChange,
        UserID:    userID,
        Timestamp: time.Now(),
        Metadata:  metadata,
    })
}

func (s *EventService) PublishRoleChangeEvent(ctx context.Context, userID uuid.UUID, newRole string, metadata model.AuthEventMetadata) error {
    metadata.Role = newRole
    return s.PublishAuthEvent(ctx, &model.AuthEvent{
        EventID:   uuid.New(),
        EventType: model.EventRoleChange,
        UserID:    userID,
        Timestamp: time.Now(),
        Metadata:  metadata,
    })
}
```

---

## 8. Error Handling

### 8.1 Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| UNAUTHORIZED | 401 | Missing or invalid authentication |
| INVALID_TOKEN | 401 | JWT is malformed or invalid |
| TOKEN_EXPIRED | 401 | Access token has expired |
| TOKEN_REVOKED | 401 | Token has been revoked |
| INVALID_REFRESH_TOKEN | 401 | Refresh token invalid or revoked |
| REFRESH_TOKEN_EXPIRED | 401 | Refresh token has expired |
| INVALID_CREDENTIALS | 401 | Email/password mismatch |
| INSUFFICIENT_PERMISSIONS | 403 | Valid auth but insufficient role |
| EMAIL_NOT_VERIFIED | 403 | Email not yet verified |
| ACCOUNT_SUSPENDED | 403 | Account is suspended |
| ACCOUNT_DELETED | 403 | Account has been deleted |
| RESOURCE_ACCESS_DENIED | 403 | No permission for resource |
| REGISTRATION_DISABLED | 403 | Registration disabled in single-user mode |
| INVALID_EMAIL | 400 | Email format invalid |
| WEAK_PASSWORD | 400 | Password doesn't meet requirements |
| INVALID_CURRENT_PASSWORD | 400 | Current password is wrong |
| INVALID_TOKEN | 400 | Token invalid or malformed |
| TOKEN_EXPIRED | 400 | Token has expired |
| TOKEN_USED | 400 | Token already consumed |
| CANNOT_REMOVE_LAST_AUTH_METHOD | 400 | Cannot unlink only auth method |
| EMAIL_EXISTS | 409 | Email already registered |
| USER_NOT_FOUND | 404 | User does not exist |
| RESOURCE_NOT_FOUND | 404 | Resource does not exist |
| SESSION_NOT_FOUND | 404 | Session does not exist |
| ACCOUNT_LOCKED | 429 | Account temporarily locked |
| RATE_LIMITED | 429 | Too many requests |
| INTERNAL_ERROR | 500 | Server error |

### 8.2 Error Response Format

```go
// internal/model/errors.go
package model

import "errors"

var (
    ErrUnauthorized           = errors.New("unauthorized")
    ErrInvalidToken          = errors.New("invalid token")
    ErrTokenExpired          = errors.New("token expired")
    ErrTokenRevoked          = errors.New("token revoked")
    ErrInvalidCredentials    = errors.New("invalid credentials")
    ErrInsufficientPermissions = errors.New("insufficient permissions")
    ErrEmailNotVerified      = errors.New("email not verified")
    ErrAccountSuspended      = errors.New("account suspended")
    ErrAccountDeleted        = errors.New("account deleted")
    ErrResourceAccessDenied  = errors.New("resource access denied")
    ErrRegistrationDisabled   = errors.New("registration disabled")
    ErrInvalidEmail          = errors.New("invalid email")
    ErrWeakPassword          = errors.New("weak password")
    ErrEmailExists           = errors.New("email already exists")
    ErrUserNotFound          = errors.New("user not found")
    ErrAccountLocked         = errors.New("account locked")
    ErrRateLimited           = errors.New("rate limited")
)

// ErrorResponse represents the standard error response
type ErrorResponse struct {
    Success bool       `json:"success"`
    Error   ErrorInfo  `json:"error"`
}

type ErrorInfo struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Details map[string]interface{}  `json:"details,omitempty"`
}
```

---

## 9. Security Considerations

### 9.1 Password Security

```go
// internal/service/password_service.go
package service

import (
    "errors"
    "unicode"

    "github.com/alexedwards/argon2id"
)

const (
    Argon2Memory      = 64 * 1024 // 64MB
    Argon2Iterations  = 3
    Argon2Parallelism = 4
    Argon2KeyLength   = 32
)

type PasswordService struct {
    minLength     int
    requireUpper  bool
    requireLower  bool
    requireNumber bool
    requireSymbol bool
}

func NewPasswordService(cfg config.PasswordConfig) *PasswordService {
    return &PasswordService{
        minLength:     cfg.MinLength,
        requireUpper:  cfg.RequireUpper,
        requireLower:  cfg.RequireLower,
        requireNumber: cfg.RequireNumber,
        requireSymbol: cfg.RequireSymbol,
    }
}

// HashPassword creates an Argon2id hash of the password
func (s *PasswordService) HashPassword(password string) (string, error) {
    params := &argon2id.Params{
        Memory:      Argon2Memory,
        Iterations:  Argon2Iterations,
        Parallelism: Argon2Parallelism,
        KeyLength:   Argon2KeyLength,
    }

    hash, err := argon2id.CreateHash(password, params)
    if err != nil {
        return "", err
    }

    return hash, nil
}

// VerifyPassword checks if the password matches the hash
func (s *PasswordService) VerifyPassword(password, hash string) bool {
    _, err := argon2id.VerifyPassword(password, hash)
    return err == nil
}

// ValidatePassword checks password complexity requirements
func (s *PasswordService) ValidatePassword(password string) error {
    if len(password) < s.minLength {
        return ErrWeakPassword
    }

    var hasUpper, hasLower, hasNumber, hasSymbol bool

    for _, char := range password {
        switch {
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsNumber(char):
            hasNumber = true
        case unicode.IsPunct(char) || unicode.IsSymbol(char):
            hasSymbol = true
        }
    }

    if s.requireUpper && !hasUpper {
        return errors.New("password must contain at least one uppercase letter")
    }
    if s.requireLower && !hasLower {
        return errors.New("password must contain at least one lowercase letter")
    }
    if s.requireNumber && !hasNumber {
        return errors.New("password must contain at least one number")
    }
    if s.requireSymbol && !hasSymbol {
        return errors.New("password must contain at least one symbol")
    }

    return nil
}
```

### 9.2 Rate Limiting

```go
// internal/middleware/rate_limit_middleware.go
package middleware

import (
    "net/http"
    "strconv"
    "time"
    "unsafe"

    "ace/shared/caching"
    "ace/shared/response"
)

type RateLimiter struct {
    cache         *caching.Cache
    limits        map[string]*RateLimitConfig
}

type RateLimitConfig struct {
    MaxRequests int
    Window      time.Duration
}

func NewRateLimiter(cache *caching.Cache) *RateLimiter {
    return &RateLimiter{
        cache: cache,
        limits: map[string]*RateLimitConfig{
            "login": {
                MaxRequests: 10,
                Window:      time.Minute,
            },
            "password_reset": {
                MaxRequests: 3,
                Window:      time.Hour,
            },
            "register": {
                MaxRequests: 5,
                Window:      time.Minute,
            },
        },
    }
}

func (r *RateLimiter) Limit(limitType string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            ctx := req.Context()
            cfg := r.limits[limitType]
            if cfg == nil {
                next.ServeHTTP(w, req)
                return
            }

            // Get identifier (IP or email)
            identifier := r.getIdentifier(req)
            key := "ratelimit:" + limitType + ":" + identifier

            // Use INCRBY with pipeline pattern to avoid race conditions
            // Increment counter atomically
            count, err := r.cache.Increment(ctx, key, cfg.Window)
            if err != nil {
                // If cache fails, allow the request but log
                next.ServeHTTP(w, req)
                return
            }

            // Set TTL on first request using atomic compare-and-swap pattern
            // Only set TTL if count is 1 (first increment)
            if count == 1 {
                r.cache.Expire(ctx, key, cfg.Window)
            }

            // Check limit
            if count > int64(cfg.MaxRequests) {
                response.TooManyRequests(w, "RATE_LIMITED", "Too many requests. Please try again later.")
                return
            }

            // Add rate limit headers
            w.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.MaxRequests))
            remaining := int64(cfg.MaxRequests) - count
            if remaining < 0 {
                remaining = 0
            }
            w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

            next.ServeHTTP(w, req)
        })
    }
}

func (r *RateLimiter) getIdentifier(req *http.Request) string {
    // Try email first (for email-based rate limits)
    if email := req.Header.Get("X-RateLimit-Email"); email != "" {
        return email
    }

    // Fall back to IP
    return req.RemoteAddr
}
```

### 9.3 CSRF Protection for OAuth

```go
// internal/service/oauth_state_service.go
package service

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "strconv"
    "time"

    "github.com/google/uuid"

    "ace/shared/caching"
)

const (
    OAuthStateTTL = 10 * time.Minute
)

type OAuthState struct {
    CSRFToken   string `json:"csrf_token"`
    RedirectURL string `json:"redirect_url"`
    Timestamp   int64  `json:"timestamp"`
    Signature   string `json:"signature"`
}

type OAuthStateService struct {
    cache   *caching.Cache
    secrets map[string]string // provider -> secret
}

func NewOAuthStateService(cache *caching.Cache, secrets map[string]string) *OAuthStateService {
    return &OAuthStateService{
        cache:   cache,
        secrets: secrets,
    }
}

// GenerateState creates a new OAuth state parameter
func (s *OAuthStateService) GenerateState(provider, redirectURL string) (string, error) {
    csrfToken := uuid.New().String()
    timestamp := time.Now().Unix()

    state := OAuthState{
        CSRFToken:   csrfToken,
        RedirectURL: redirectURL,
        Timestamp:   timestamp,
    }

    // Sign the state
    sigInput := csrfToken + redirectURL + strconv.FormatInt(timestamp, 10)
    state.Signature = s.sign(provider, sigInput)

    // Serialize and encode
    data, err := json.Marshal(state)
    if err != nil {
        return "", err
    }

    stateString := base64.URLEncoding.EncodeToString(data)

    // Store in cache for validation
    cacheKey := "oauth_state:" + csrfToken
    s.cache.Set(context.Background(), cacheKey, []byte(stateString), caching.WithTTL(OAuthStateTTL))

    return stateString, nil
}

// ValidateState validates an OAuth state parameter
func (s *OAuthStateService) ValidateState(provider, stateString string) (*OAuthState, error) {
    // Decode
    data, err := base64.URLEncoding.DecodeString(stateString)
    if err != nil {
        return nil, errors.New("invalid state encoding")
    }

    var state OAuthState
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, errors.New("invalid state format")
    }

    // Check signature
    sigInput := state.CSRFToken + state.RedirectURL + strconv.FormatInt(state.Timestamp, 10)
    expectedSig := s.sign(provider, sigInput)
    if state.Signature != expectedSig {
        return nil, errors.New("invalid state signature")
    }

    // Check timestamp (max 10 minutes)
    if time.Now().Unix()-state.Timestamp > 600 {
        return nil, errors.New("state expired")
    }

    // Check cache for CSRF token
    cacheKey := "oauth_state:" + state.CSRFToken
    cached, err := s.cache.Get(context.Background(), cacheKey)
    if err != nil || cached == nil {
        return nil, errors.New("state already used")
    }

    // Delete from cache (single-use)
    s.cache.Delete(context.Background(), cacheKey)

    return &state, nil
}

func (s *OAuthStateService) sign(provider, data string) string {
    secret := s.secrets[provider]
    if secret == "" {
        secret = s.secrets["default"]
    }

    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(data))
    return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
```

---

## 10. Configuration

### 10.1 Environment Variables

```bash
# =============================================================================
# AUTH SERVICE CONFIGURATION
# =============================================================================

# JWT Configuration
AUTH_ACCESS_TOKEN_TTL=15m
AUTH_REFRESH_TOKEN_TTL=168h
AUTH_JWT_ALGORITHM=RS256
AUTH_JWT_PRIVATE_KEY_PATH=/secrets/jwt-private.pem
AUTH_JWT_PUBLIC_KEY_PATH=/secrets/jwt-public.pem
AUTH_JWT_SECRET=development-secret-only  # Only for HS256 dev mode

# Rate Limiting
AUTH_LOGIN_RATE_LIMIT_PER_IP=10
AUTH_LOGIN_RATE_LIMIT_PER_EMAIL=5
AUTH_PASSWORD_RESET_RATE_LIMIT=3
AUTH_LOCKOUT_THRESHOLD=5
AUTH_LOCKOUT_DURATION=15m

# Password Requirements
AUTH_PASSWORD_MIN_LENGTH=8
AUTH_PASSWORD_REQUIRE_UPPER=true
AUTH_PASSWORD_REQUIRE_LOWER=true
AUTH_PASSWORD_REQUIRE_NUMBER=true
AUTH_PASSWORD_REQUIRE_SYMBOL=false

# Token TTLs
AUTH_EMAIL_TOKEN_TTL=24h
AUTH_RESET_TOKEN_TTL=1h

# Deployment Mode
AUTH_DEPLOYMENT_MODE=multi  # "single" or "multi"

# OAuth Providers
AUTH_GOOGLE_CLIENT_ID=your-google-client-id
AUTH_GOOGLE_CLIENT_SECRET=your-google-client-secret
AUTH_GITHUB_CLIENT_ID=your-github-client-id
AUTH_GITHUB_CLIENT_SECRET=your-github-client-secret

# OAuth Secrets (for state signing)
AUTH_OAUTH_SECRET_DEFAULT=your-oauth-state-secret

# Database
DATABASE_URL=postgres://ace:password@postgres:5432/ace?sslmode=disable

# Valkey
VALKEY_URL=valkey://valkey:6379

# NATS
NATS_URL=nats://nats:4222

# Service
SERVICE_NAME=auth
SERVICE_PORT=8081
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

### 10.2 Configuration Struct

```go
// config/config.go
package config

import (
    "os"
    "time"

    "github.com/knadh/koanf/parsers/toml"
    "github.com/knadh/koanf/providers/env"
    "github.com/knadh/koanf/providers/file"
    "github.com/knadh/koanf/v2"
)

type Config struct {
    Database DatabaseConfig `koanf:"database"`
    Valkey   ValkeyConfig   `koanf:"valkey"`
    NATS     NATSConfig     `koanf:"nats"`
    Service  ServiceConfig  `koanf:"service"`
    Auth     AuthConfig     `koanf:"auth"`
    OAuth    OAuthConfig    `koanf:"oauth"`
}

type DatabaseConfig struct {
    URL string `koanf:"url" env:"DATABASE_URL" envdefault:"postgres://localhost:5432/ace"`
}

type ValkeyConfig struct {
    URL string `koanf:"url" env:"VALKEY_URL" envdefault:"valkey://localhost:6379"`
}

type NATSConfig struct {
    URL string `koanf:"url" env:"NATS_URL" envdefault:"nats://localhost:4222"`
}

type ServiceConfig struct {
    Name string `koanf:"name" env:"SERVICE_NAME" envdefault:"auth"`
    Port int    `koanf:"port" env:"SERVICE_PORT" envdefault:"8081"`
}

type AuthConfig struct {
    AccessTokenTTL   time.Duration `koanf:"access_token_ttl" env:"AUTH_ACCESS_TOKEN_TTL" envdefault:"15m"`
    RefreshTokenTTL  time.Duration `koanf:"refresh_token_ttl" env:"AUTH_REFRESH_TOKEN_TTL" envdefault:"168h"`
    JWTAlgorithm     string        `koanf:"jwt_algorithm" env:"AUTH_JWT_ALGORITHM" envdefault:"RS256"`
    JWTPrivateKey    string        `koanf:"jwt_private_key" env:"AUTH_JWT_PRIVATE_KEY" envdefault:""`
    JWTPublicKey     string        `koanf:"jwt_public_key" env:"AUTH_JWT_PUBLIC_KEY" envdefault:""`
    JWTSecret        string        `koanf:"jwt_secret" env:"AUTH_JWT_SECRET" envdefault:""`

    LoginRateLimitPerIP    int `koanf:"login_rate_limit_per_ip" env:"AUTH_LOGIN_RATE_LIMIT_PER_IP" envdefault:"10"`
    LoginRateLimitPerEmail int `koanf:"login_rate_limit_per_email" env:"AUTH_LOGIN_RATE_LIMIT_PER_EMAIL" envdefault:"5"`
    PasswordResetRateLimit int `koanf:"password_reset_rate_limit" env:"AUTH_PASSWORD_RESET_RATE_LIMIT" envdefault:"3"`
    LockoutThreshold       int `koanf:"lockout_threshold" env:"AUTH_LOCKOUT_THRESHOLD" envdefault:"5"`
    LockoutDuration        time.Duration `koanf:"lockout_duration" env:"AUTH_LOCKOUT_DURATION" envdefault:"15m"`

    PasswordMinLength     int  `koanf:"password_min_length" env:"AUTH_PASSWORD_MIN_LENGTH" envdefault:"8"`
    PasswordRequireUpper  bool `koanf:"password_require_upper" env:"AUTH_PASSWORD_REQUIRE_UPPER" envdefault:"true"`
    PasswordRequireLower  bool `koanf:"password_require_lower" env:"AUTH_PASSWORD_REQUIRE_LOWER" envdefault:"true"`
    PasswordRequireNumber bool `koanf:"password_require_number" env:"AUTH_PASSWORD_REQUIRE_NUMBER" envdefault:"true"`
    PasswordRequireSymbol bool `koanf:"password_require_symbol" env:"AUTH_PASSWORD_REQUIRE_SYMBOL" envdefault:"false"`

    EmailTokenTTL time.Duration `koanf:"email_token_ttl" env:"AUTH_EMAIL_TOKEN_TTL" envdefault:"24h"`
    ResetTokenTTL time.Duration `koanf:"reset_token_ttl" env:"AUTH_RESET_TOKEN_TTL" envdefault:"1h"`

    DeploymentMode string `koanf:"deployment_mode" env:"AUTH_DEPLOYMENT_MODE" envdefault:"multi"`
}

type OAuthConfig struct {
    GoogleClientID     string `koanf:"google_client_id" env:"AUTH_GOOGLE_CLIENT_ID" envdefault:""`
    GoogleClientSecret string `koanf:"google_client_secret" env:"AUTH_GOOGLE_CLIENT_SECRET" envdefault:""`
    GitHubClientID     string `koanf:"github_client_id" env:"AUTH_GITHUB_CLIENT_ID" envdefault:""`
    GitHubClientSecret string `koanf:"github_client_secret" env:"AUTH_GITHUB_CLIENT_SECRET" envdefault:""`
}

func Load() (*Config, error) {
    k := koanf.New(".")

    // Load from TOML file if exists
    if err := k.Load(file.Provider("config.toml"), toml.Parser()); err != nil {
        // Ignore if file doesn't exist
    }

    // Load from environment variables
    if err := k.Load(env.Provider("", ".", func(s string) string {
        return s
    }), nil); err != nil {
        return nil, err
    }

    var cfg Config
    if err := k.Unmarshal("", &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

---

## 11. Testing Requirements

### 11.1 Unit Tests

| Component | Test Coverage | Key Tests |
|-----------|---------------|-----------|
| Password hashing | >95% | Hash/verify, timing attack resistance |
| JWT validation | >95% | Valid/invalid/expired tokens, signature verification |
| Rate limiting | >90% | Per-IP, per-email, sliding window |
| Permission checking | >95% | Owner access, shared access, no access |
| OAuth state | >95% | Generate, validate, CSRF prevention |

### 11.2 Integration Tests

| Test | Description |
|------|-------------|
| Full login flow | Register → Email verify → Login → Token refresh → Logout |
| OAuth flow | Initiate → Callback → Link account |
| Password reset | Request → Email → Reset → Login with new password |
| Admin operations | Create user → Suspend → Restore → Delete |
| Rate limit | Exceed limit → Verify 429 response |
| Token revocation | Login multiple devices → Logout one → Verify others work |

---

## 12. Edge Cases

| Scenario | Expected Behavior | Handling |
|----------|------------------|----------|
| Login during lockout | Return 429 with lockout expiry | Check lockout before password verify |
| Verify already-used token | Return 400 TOKEN_USED | Check used_at before accepting |
| Reset with deleted user email | Silently succeed | Always return success to prevent enumeration |
| Link already-linked OAuth | Return 409 CONFLICT | Check existing provider before linking |
| Unlink last auth method | Return 400 CANNOT_REMOVE_LAST_AUTH_METHOD | Verify other methods exist |
| Access after account deleted | Return 403 ACCOUNT_DELETED | Check deleted_at on every request |
| Concurrent token refresh | Only first succeeds | Use database transaction with row lock |
| Valkey unavailable | Allow request (graceful degradation) | Log error, skip cache operations |
| NATS unavailable | Allow request (graceful degradation) | Log error, async retry event publishing |

---

## Appendix A: SQLC Query Interface

```sql
-- sqlc/query.sql
-- name: GetUserByEmail :one
SELECT id, email, password_hash, email_verified, role, status, 
       suspended_at, suspended_reason, deleted_at, created_at, updated_at
FROM users
WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT id, email, password_hash, email_verified, role, status,
       suspended_at, suspended_reason, deleted_at, created_at, updated_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, status)
VALUES ($1, $2, $3, $4)
RETURNING id, email, email_verified, role, status, created_at, updated_at;

-- name: UpdateUserStatus :exec
UPDATE users 
SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: GetOAuthProvider :one
SELECT id, user_id, provider, provider_user_id, created_at
FROM oauth_providers
WHERE provider = $1 AND provider_user_id = $2;

-- name: LinkOAuthProvider :exec
INSERT INTO oauth_providers (user_id, provider, provider_user_id, access_token_enc, refresh_token_enc, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, provider) DO UPDATE
SET provider_user_id = $3, access_token_enc = $4, refresh_token_enc = $5, expires_at = $6;

-- name: CreateSession :one
INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at;

-- name: GetSessionByTokenHash :one
SELECT id, user_id, expires_at, created_at
FROM sessions
WHERE refresh_token_hash = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteAllUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: CreatePermission :exec
INSERT INTO resource_permissions (user_id, resource_type, resource_id, permission_level, granted_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, resource_type, resource_id) DO UPDATE
SET permission_level = $4, granted_by = $5;

-- name: GetPermission :one
SELECT id, user_id, resource_type, resource_id, permission_level, granted_by, created_at
FROM resource_permissions
WHERE user_id = $1 AND resource_type = $2 AND resource_id = $3;

-- name: DeletePermission :exec
DELETE FROM resource_permissions
WHERE user_id = $1 AND resource_type = $2 AND resource_id = $3;
```

---

**Document Version:** 1.0  
**Unit:** users-auth  
**Status:** Draft  
**Created:** 2026-04-09
