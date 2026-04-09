# User Stories: Users-Auth Unit

<!--
Intent: Define user-facing behavior in executable format that drives implementation and testing.
Scope: All user interactions for authentication, authorization, and account management.
Used by: AI agents to generate acceptance tests and ensure features meet user expectations.
Unit: users-auth
-->

---

## Table of Contents

1. [Authentication Flows](#1-authentication-flows)
   - [Registration Flow](#11-registration-flow)
   - [Login Flow](#12-login-flow)
   - [SSO OAuth Flow](#13-sso-oauth-flow)
   - [Password Reset Flow](#14-password-reset-flow)
   - [Email Verification Flow](#15-email-verification-flow)
   - [Token Refresh Flow](#16-token-refresh-flow)
   - [Logout Flow](#17-logout-flow)
2. [Authorization Flows](#2-authorization-flows)
3. [Admin Flows](#3-admin-flows)
4. [User Stories (Detailed)](#4-user-stories-detailed)
5. [Acceptance Criteria Mapping](#5-acceptance-criteria-mapping)

---

## 1. Authentication Flows

### 1.1 Registration Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        EMAIL/PASSWORD REGISTRATION FLOW                       │
└─────────────────────────────────────────────────────────────────────────────┘

  User                      Frontend                      Backend
   │                            │                            │
   │  1. Fill registration form │                            │
   │────────────────────────────>│                            │
   │                            │  2. POST /auth/register    │
   │                            │  {email, password}         │
   │                            │───────────────────────────>│
   │                            │                            │
   │                            │  3. Validate input         │
   │                            │  - Email format            │
   │                            │  - Password complexity     │
   │                            │  - Duplicate check         │
   │                            │                            │
   │                            │  4. Hash password (Argon2id)│
   │                            │                            │
   │                            │  5. Create user record     │
   │                            │  status: "pending_verification"│
   │                            │                            │
   │                            │  6. Generate email token   │
   │                            │  - UUID                    │
   │                            │  - Hash stored in DB       │
   │                            │  - Expiry: 24 hours        │
   │                            │                            │
   │                            │  7. Publish event          │
   │                            │  ace.auth.user_registered.event│
   │                            │                            │
   │                            │  8. Send verification email │
   │                            │  (via email service)       │
   │                            │                            │
   │  9. Success response       │                            │
   │  {message: "Check email"}  │                            │
   │<────────────────────────────│                            │
   │                            │                            │
   │                            │                            ▼
```

**API Request/Response:**

```http
POST /auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "SecureP@ss123"
}
```

```json
{
  "success": true,
  "message": "Registration successful. Please check your email to verify your account.",
  "user_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Error Responses:**

| Status | Error Code | Description |
|--------|------------|-------------|
| 400 | `INVALID_EMAIL` | Email format is invalid |
| 400 | `WEAK_PASSWORD` | Password does not meet complexity requirements |
| 409 | `EMAIL_EXISTS` | Email already registered |
| 429 | `RATE_LIMITED` | Too many registration attempts |
| 500 | `INTERNAL_ERROR` | Server error |

---

### 1.2 Login Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           EMAIL/PASSWORD LOGIN FLOW                         │
└─────────────────────────────────────────────────────────────────────────────┘

  User                      Frontend                      Backend
   │                            │                            │
   │  1. Fill login form        │                            │
   │  {email, password}         │                            │
   │────────────────────────────>│                            │
   │                            │  2. Check rate limit       │
   │                            │  - Per IP: 10/min          │
   │                            │  - Per email: 5/5min       │
   │                            │                            │
   │  3. POST /auth/login       │                            │
   │  {email, password}         │                            │
   │                            │───────────────────────────>│
   │                            │                            │
   │                            │  4. Find user by email     │
   │                            │                            │
   │                            │  5. Check account status   │
   │                            │  - active: proceed         │
   │                            │  - suspended: reject       │
   │                            │  - pending_verification:   │
   │                            │    reject                  │
   │                            │                            │
   │                            │  6. Verify password        │
   │                            │  - Argon2id compare        │
   │                            │                            │
   │                            │  7. Reset failed attempts │
   │                            │                            │
   │                            │  8. Generate tokens       │
   │                            │  - Access token (5-15 min) │
   │                            │  - Refresh token (7 days)  │
   │                            │                            │
   │                            │  9. Store session          │
   │                            │  - Valkey for distributed  │
   │                            │                            │
   │                            │  10. Publish event         │
   │                            │  ace.auth.login.event      │
   │                            │  {user_id, ip, user_agent} │
   │                            │                            │
   │  11. Return tokens         │                            │
   │  {access_token,           │                            │
   │   refresh_token,          │                            │
   │   expires_in}              │                            │
   │<────────────────────────────│                            │
   │                            │                            │
```

**Failed Login Flow (Brute Force Protection):**

```
   │                            │                            │
   │                            │  ❌ Invalid credentials   │
   │                            │                            │
   │                            │  1. Increment failed      │
   │                            │     attempts counter       │
   │                            │  - Valkey: failed:{email} │
   │                            │                            │
   │                            │  2. Check threshold       │
   │                            │  - If >= 5 attempts:      │
   │                            │    lock account 15min     │
   │                            │                            │
   │                            │  3. Publish event         │
   │                            │  ace.auth.failed_login.event│
   │                            │                            │
   │  4. Error response         │                            │
   │  {error: "invalid_creds"}  │                            │
   │  OR                        │                            │
   │  {error: "account_locked", │
   │   locked_until: "..."}     │                            │
   │<────────────────────────────│                            │
```

**API Request/Response:**

```http
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "SecureP@ss123"
}
```

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "rt_550e8400-e29b-41d4-a716-446655440001",
  "token_type": "Bearer",
  "expires_in": 900
}
```

---

### 1.3 SSO OAuth Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           GOOGLE OAUTH FLOW                                  │
└─────────────────────────────────────────────────────────────────────────────┘

  User                      Frontend                      Backend
   │                            │                            │
   │  1. Click "Login with    │                            │
   │     Google"               │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  2. Generate state token  │
   │                            │  - UUID + CSRF protection  │
   │                            │  - Store in session/Valkey │
   │                            │                            │
   │  3. Redirect to Google     │                            │
   │  /auth/oauth/google       │                            │
   │<────────────────────────────│                            │
   │                            │                            │
   │  4. Google Login Page      │                            │
   │  ─────────────────────────│                            │
   │  User authenticates       │                            │
   │  with Google              │                            │
   │  ─────────────────────────│                            │
   │                            │                            │
   │  5. Callback to           │                            │
   │  /auth/oauth/google/      │                            │
   │  callback?code=xxx       │                            │
   │  &state=yyy               │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  6. Validate state         │
   │                            │  - Prevent CSRF           │
   │                            │                            │
   │                            │  7. Exchange code for      │
   │                            │     tokens (Google API)    │
   │                            │                            │
   │                            │  8. Get user info          │
   │                            │  (email, name, picture)    │
   │                            │                            │
   │                            │  9. Check if user exists   │
   │                            │  by Google ID or email     │
   │                            │                            │
   │                            │  10. If new user:          │
   │                            │  - Create user account     │
   │                            │  - Link oauth_provider     │
   │                            │  - status: "active"       │
   │                            │    (email pre-verified)    │
   │                            │                            │
   │                            │  11. If existing user:    │
   │                            │  - Link oauth_provider     │
   │                            │    if not already linked   │
   │                            │                            │
   │                            │  12. Generate tokens       │
   │                            │  - Access + Refresh       │
   │                            │                            │
   │                            │  13. Publish event         │
   │                            │  ace.auth.login.event      │
   │                            │  {provider: "google"}     │
   │                            │                            │
   │  14. Redirect to app       │                            │
   │  with tokens in URL or     │
   │  set-cookie                │
   │<────────────────────────────│                            │
   │                            │                            │
```

**GitHub OAuth Flow:** Identical structure, substituting `google` with `github`.

**State Parameter Security:**

```
┌────────────────────────────────────────────────┐
│            STATE PARAMETER STRUCTURE          │
├────────────────────────────────────────────────┤
│  {                                            │
│    "csrf_token": "uuid",                      │
│    "redirect_url": "/dashboard",              │
│    "timestamp": 1712600000,                  │
│    "signature": "HMAC-SHA256(...)"            │
│  }                                            │
│  Base64URL encoded and passed to OAuth provider│
│                                                │
│  Validation:                                   │
│  1. Decode state                              │
│  2. Verify signature                          │
│  3. Check timestamp (max 10 minutes)          │
│  4. Verify CSRF token matches stored value   │
└────────────────────────────────────────────────┘
```

---

### 1.4 Password Reset Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          PASSWORD RESET FLOW                                 │
└─────────────────────────────────────────────────────────────────────────────┘

  User                      Frontend                      Backend
   │                            │                            │
   │  1. Click "Forgot         │                            │
   │     Password"              │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │  2. Enter email            │                            │
   │  {email}                   │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  3. Check rate limit       │
   │                            │  - 3 requests/hour/email   │
   │                            │                            │
   │                            │  4. Find user by email     │
   │                            │  - Always return success   │
   │                            │    (prevents enumeration)  │
   │                            │                            │
   │                            │  5. If user exists:        │
   │                            │  - Generate reset token    │
   │                            │  - Hash stored in DB       │
   │                            │  - Expiry: 1 hour          │
   │                            │  - Single-use flag        │
   │                            │                            │
   │                            │  6. Send reset email       │
   │                            │  (via email service)       │
   │                            │                            │
   │  7. "Check your email"     │                            │
   │<────────────────────────────│                            │
   │                            │                            │
   │  8. Click reset link       │                            │
   │  /auth/password/reset/     │                            │
   │  confirm?token=xxx         │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  9. Validate token         │
   │                            │  - Check hash match        │
   │                            │  - Check expiry           │
   │                            │  - Check not used         │
   │                            │                            │
   │  10. Show new password     │                            │
   │  form                      │                            │
   │<────────────────────────────│                            │
   │                            │                            │
   │  11. Enter new password    │                            │
   │  {password}                │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  12. Validate password     │
   │                            │                            │
   │                            │  13. Update password_hash  │
   │                            │  - Argon2id               │
   │                            │                            │
   │                            │  14. Mark token as used   │
   │                            │                            │
   │                            │  15. Invalidate all        │
   │                            │      sessions              │
   │                            │  - Valkey: revoke all     │
   │                            │    user's refresh tokens   │
   │                            │                            │
   │                            │  16. Publish event         │
   │                            │  ace.auth.password_change │
   │                            │  .event                    │
   │                            │                            │
   │  17. Success response      │                            │
   │  {message: "Password reset│                            │
   │   successful"}             │                            │
   │<────────────────────────────│                            │
   │                            │                            │
```

**API Requests/Responses:**

```http
POST /auth/password/reset/request
Content-Type: application/json

{
  "email": "user@example.com"
}
```

```json
{
  "success": true,
  "message": "If an account exists with this email, a password reset link has been sent."
}
```

```http
POST /auth/password/reset/confirm
Content-Type: application/json

{
  "token": "550e8400-e29b-41d4-a716-446655440002",
  "password": "NewSecureP@ss456"
}
```

```json
{
  "success": true,
  "message": "Password has been reset. Please log in with your new password."
}
```

---

### 1.5 Email Verification Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         EMAIL VERIFICATION FLOW                              │
└─────────────────────────────────────────────────────────────────────────────┘

  User                      Frontend                      Backend
   │                            │                            │
   │  1. Click verification     │                            │
   │  link in email             │                            │
   │  /auth/email/verify?       │                            │
   │  token=xxx                 │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  2. Validate token          │
   │                            │  - Check hash match        │
   │                            │  - Check expiry           │
   │                            │  - Check not used         │
   │                            │                            │
   │                            │  3. Update user status     │
   │                            │  - email_verified: true    │
   │                            │  - status: "active"        │
   │                            │                            │
   │                            │  4. Mark token as used     │
   │                            │                            │
   │                            │  5. Publish event          │
   │                            │  ace.auth.email_verified   │
   │                            │  .event                    │
   │                            │                            │
   │  6. Success page           │                            │
   │  "Email verified!"         │                            │
   │<────────────────────────────│                            │
   │                            │                            │
```

**API Request/Response:**

```http
POST /auth/email/verify
Content-Type: application/json

{
  "token": "550e8400-e29b-41d4-a716-446655440003"
}
```

```json
{
  "success": true,
  "message": "Email verified successfully. Your account is now active.",
  "redirect_url": "/login"
}
```

---

### 1.6 Token Refresh Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          TOKEN REFRESH FLOW                                  │
└─────────────────────────────────────────────────────────────────────────────┘

  User                      Frontend                      Backend
   │                            │                            │
   │  1. API request with       │                            │
   │     expired access token    │                            │
   │────────────────────────────>│                            │
   │                            │  2. Detect expired token   │
   │                            │                            │
   │  3. POST /auth/refresh     │                            │
   │  {refresh_token}           │                            │
   │  OR                        │                            │
   │  Cookie: refresh_token     │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  4. Validate refresh token │
   │                            │  - Check not revoked      │
   │                            │  - Check not expired      │
   │                            │                            │
   │                            │  5. Check token in Valkey  │
   │                            │  - Not blacklisted        │
   │                            │                            │
   │                            │  6. Rotate refresh token  │
   │                            │  - Revoke old token       │
   │                            │  - Issue new refresh token│
   │                            │                            │
   │                            │  7. Issue new access token │
   │                            │                            │
   │  8. Return new tokens     │                            │
   │  {access_token,            │                            │
   │   refresh_token,          │                            │
   │   expires_in}              │                            │
   │<────────────────────────────│                            │
   │                            │                            │
   │  9. Retry original request │                            │
   │     with new access token  │                            │
   │────────────────────────────>│                            │
   │                            │                            │
```

**Token Rotation Diagram:**

```
┌─────────────────────────────────────────────────────────────┐
│                    TOKEN ROTATION STRATEGY                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   BEFORE REFRESH:                                           │
│   ┌─────────────┐                                           │
│   │ Access Token│ (expires at T+5min)                       │
│   ├─────────────┤                                           │
│   │Refresh Token│ (stored in Valkey, expires at T+7days)   │
│   └─────────────┘                                           │
│                                                              │
│   AFTER REFRESH:                                            │
│   ┌─────────────┐                                           │
│   │ New Access │ (new expiry T+5min)                        │
│   ├─────────────┤                                           │
│   │ Old Refresh│ ──► BLACKLISTED in Valkey                 │
│   ├─────────────┤                                           │
│   │ New Refresh│ (new, stored in Valkey)                    │
│   └─────────────┘                                           │
│                                                              │
│   Security:                                                 │
│   - Stolen refresh token cannot be reused                   │
│   - Each use rotates to a new token                        │
│   - Compromised old token immediately blacklisted          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**API Request/Response:**

```http
POST /auth/refresh
Content-Type: application/json
Cookie: refresh_token=rt_550e8400-e29b-41d4-a716-446655440001

{
  "refresh_token": "rt_550e8400-e29b-41d4-a716-446655440001"
}
```

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "rt_660e8400-e29b-41d4-a716-446655440001",
  "token_type": "Bearer",
  "expires_in": 900
}
```

---

### 1.7 Logout Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             LOGOUT FLOW                                     │
└─────────────────────────────────────────────────────────────────────────────┘

  User                      Frontend                      Backend
   │                            │                            │
   │  1. Click "Logout"         │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  2. POST /auth/logout      │
   │                            │  Authorization: Bearer xxx  │
   │                            │───────────────────────────>│
   │                            │                            │
   │                            │  3. Extract session ID      │
   │                            │     from token             │
   │                            │                            │
   │                            │  4. Revoke refresh token   │
   │                            │  - Add to Valkey blacklist │
   │                            │  - TTL = remaining token   │
   │                            │    lifetime               │
   │                            │                            │
   │                            │  5. Publish event          │
   │                            │  ace.auth.logout.event    │
   │                            │  {session_id, user_id}    │
   │                            │                            │
   │  6. Clear local tokens     │                            │
   │  - Remove from storage     │                            │
   │  - Clear cookies           │                            │
   │                            │                            │
   │  7. Success response       │                            │
   │<────────────────────────────│                            │
   │                            │                            │
   │  8. Redirect to login      │                            │
   │  /login                    │                            │
   │<────────────────────────────│                            │
   │                            │                            │
```

**Single Logout (Invalidate All Sessions):**

```
   User                      Frontend                      Backend
   │                            │                            │
   │                            │  On password change:      │
   │                            │  ace.auth.password_change │
   │                            │  .event                   │
   │                            │───────────────────────────>│
   │                            │                            │
   │                            │  1. Find all user sessions│
   │                            │  - Query Valkey by user_id │
   │                            │                            │
   │                            │  2. Blacklist all tokens   │
   │                            │  - Add all refresh tokens │
   │                            │    to revocation list    │
   │                            │                            │
   │                            │  3. Clear session cache   │
   │                            │  - Valkey DEL by pattern  │
   │                            │    "session:{user_id}:*"  │
   │                            │                            │
```

**API Request/Response:**

```http
POST /auth/logout
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
```

```json
{
  "success": true,
  "message": "Logged out successfully."
}
```

---

## 2. Authorization Flows

### 2.1 Role-Based Access Control Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      ROLE-BASED ACCESS CONTROL FLOW                         │
└─────────────────────────────────────────────────────────────────────────────┘

  Request                      Auth                     Service
   │                      Middleware                    │
   │                            │                        │
   │  1. Protected request      │                        │
   │  Authorization: Bearer xxx │                        │
   │────────────────────────────>│                        │
   │                            │                        │
   │                            │  2. Extract & validate │
   │                            │     JWT token          │
   │                            │  - Signature verify   │
   │                            │  - Expiry check       │
   │                            │  - Not revoked        │
   │                            │                        │
   │                            │  3. Get user role      │
   │                            │  - From token claims   │
   │                            │  - Cache in Valkey     │
   │                            │                        │
   │                            │  4. Check RBAC rules   │
   │                            │                        │
   │                            │  ┌──────────────────┐  │
   │                            │  │ Role Permissions │  │
   │                            │  ├──────────────────┤  │
   │                            │  │ admin: *         │  │
   │                            │  │ user: /api/users/│  │
   │                            │  │       {own}      │  │
   │                            │  │ viewer: /api/*  │  │
   │                            │  │       {read}     │  │
   │                            │  └──────────────────┘  │
   │                            │                        │
   │                            │  5. If authorized:    │
   │                            │     Attach user context│
   │                            │     to request         │
   │                            │────────────────────────>│
   │                            │                        │
   │                            │  6. Process request    │
   │                            │<────────────────────────│
   │                            │                        │
   │  7. Response              │                        │
   │<────────────────────────────│                        │
   │                            │                        │
```

**RBAC Permission Matrix:**

| Action | admin | user | viewer |
|--------|-------|------|--------|
| Read own resources | ✓ | ✓ | ✗ |
| Write own resources | ✓ | ✓ | ✗ |
| Delete own resources | ✓ | ✓ | ✗ |
| Read shared resources | ✓ | ✓ | ✓ |
| Manage users | ✓ | ✗ | ✗ |
| Assign roles | ✓ | ✗ | ✗ |
| System configuration | ✓ | ✗ | ✗ |
| View all agents | ✓ | ✗ | ✗ |

### 2.2 Resource-Level Authorization Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                   RESOURCE-LEVEL AUTHORIZATION FLOW                         │
└─────────────────────────────────────────────────────────────────────────────┘

  Request                      Auth                     Service
   │                      Middleware                    │
   │                            │                        │
   │  1. Request resource       │                        │
   │  GET /api/agents/{id}     │                        │
   │  Authorization: Bearer xxx│                        │
   │────────────────────────────>│                        │
   │                            │                        │
   │                            │  2. Authenticate        │
   │                            │  (see RBAC flow)       │
   │                            │                        │
   │                            │  3. Extract resource   │
   │                            │     ID from URL        │
   │                            │                        │
   │                            │  4. Get resource owner │
   │                            │  - Query DB or cache   │
   │                            │                        │
   │                            │  5. Check permissions  │
   │                            │                        │
   │                            │  ┌──────────────────┐  │
   │                            │  │ Permission Check │  │
   │                            │  ├──────────────────┤  │
   │                            │  │ IF owner_id ==   │  │
   │                            │  │    current_user │  │
   │                            │  │ THEN: GRANT     │  │
   │                            │  │                 │  │
   │                            │  │ ELSE IF EXISTS  │  │
   │                            │  │ permission in   │  │
   │                            │  │ resource_perm   │  │
   │                            │  │ THEN: GRANT     │  │
   │                            │  │                 │  │
   │                            │  │ ELSE: DENY      │  │
   │                            │  └──────────────────┘  │
   │                            │                        │
   │                            │  6. Cache permission  │
   │                            │  - Valkey TTL: 5min   │
   │                            │                        │
   │  7. If DENIED:             │                        │
   │  HTTP 403 Forbidden        │                        │
   │<────────────────────────────│                        │
   │                            │                        │
   │  8. If GRANTED:            │                        │
   │  Process request           │                        │
   │────────────────────────────>│                        │
   │                            │                        │
```

**Permission Levels:**

| Level | Read | Execute | Write | Delete | Share |
|-------|------|---------|-------|--------|-------|
| `view` | ✓ | ✗ | ✗ | ✗ | ✗ |
| `use` | ✓ | ✓ | ✗ | ✗ | ✗ |
| `admin` | ✓ | ✓ | ✓ | ✓ | ✓ |

### 2.3 Permission Check Flow

```
┌─────────────────────────────────────────────────────────────┐
│                  PERMISSION CHECK SEQUENCE                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────┐    ┌──────────┐    ┌──────────────────────┐   │
│  │ Request │───>│  Check   │───>│ Check Valkey Cache  │   │
│  │  with   │    │  RBAC    │    │ "perm:{user}:{res}" │   │
│  │ Bearer  │    │  (Role)  │    └─────────┬──────────┘   │
│  └─────────┘    └──────────┘              │               │
│       │                                  │ Cache Hit      │
│       │                            ┌──────┴──────────┐     │
│       │                            │                  │     │
│       │                     Cache  │            No Cache│    │
│       │                       Hit  │                  │     │
│       │                            ▼                  ▼     │
│       │                     ┌──────────┐      ┌──────────┐ │
│       │                     │ Use Cached│      │  Query   │ │
│       │                     │ Permission│      │ Database │ │
│       │                     └──────────┘      └────┬─────┘ │
│       │                                              │       │
│       │                                              │       │
│       │                            ┌─────────────────┘       │
│       │                            │                         │
│       │                            ▼                         │
│       │                     ┌──────────┐                    │
│       │                     │  Update  │                    │
│       │                     │  Cache   │                    │
│       │                     │(Valkey)  │                    │
│       │                     └────┬─────┘                    │
│       │                          │                          │
│       └──────────────────────────┼──────────────────────────┤
│                                   │                          │
│                                   ▼                          │
│                            ┌──────────────┐                 │
│                            │ Return Result│                 │
│                            │ (Grant/Deny) │                 │
│                            └──────────────┘                 │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Admin Flows

### 3.1 User Suspension Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         USER SUSPENSION FLOW                                  │
└─────────────────────────────────────────────────────────────────────────────┘

  Admin                     Frontend                     Backend
   │                            │                            │
   │  1. View user list        │                            │
   │  GET /admin/users         │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │  2. Select user to         │                            │
   │  suspend                   │                            │
   │  POST /admin/users/:id/   │                            │
   │  suspend                   │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  3. Verify admin role      │
   │                            │                            │
   │                            │  4. Update user status     │
   │                            │  - status: "suspended"     │
   │                            │  - suspended_at: now()    │
   │                            │  - suspended_reason: "..." │
   │                            │                            │
   │                            │  5. Invalidate all tokens  │
   │                            │  - Blacklist all sessions  │
   │                            │  - Valkey: add all to     │
   │                            │    revocation list        │
   │                            │                            │
   │                            │  6. Publish event          │
   │                            │  ace.auth.account_suspend  │
   │                            │  .event                    │
   │                            │                            │
   │  7. Success response       │                            │
   │  {message: "User suspended│                            │
   │   successfully"}           │                            │
   │<────────────────────────────│                            │
   │                            │                            │
   │  8. User cannot login      │                            │
   │  - HTTP 403: "Account      │                            │
   │    suspended"              │                            │
   │                            │                            │
```

**API Request/Response:**

```http
POST /auth/admin/users/550e8400-e29b-41d4-a716-446655440000/suspend
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "reason": "Violation of terms of service"
}
```

```json
{
  "success": true,
  "message": "User has been suspended. All active sessions have been invalidated.",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "suspended_at": "2026-04-09T12:00:00Z"
}
```

### 3.2 User Deletion Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          USER DELETION FLOW                                  │
└─────────────────────────────────────────────────────────────────────────────┘

  Admin                     Frontend                     Backend
   │                            │                            │
   │  1. Select user to         │                            │
   │  delete                    │                            │
   │  POST /admin/users/:id/   │                            │
   │  delete                   │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  2. Verify admin role      │
   │                            │                            │
   │                            │  3. Soft delete user       │
   │                            │  - deleted_at: now()       │
   │                            │  - email: anonymized       │
   │                            │    (delete_{id}@deleted)   │
   │                            │  - name: "Deleted User"    │
   │                            │                            │
   │                            │  4. Revoke all sessions    │
   │                            │  - Same as suspension      │
   │                            │                            │
   │                            │  5. Mark resources for      │
   │                            │     cleanup                │
   │                            │  - Set orphaned flag       │
   │                            │  - Schedule for deletion   │
   │                            │                            │
   │                            │  6. Publish event          │
   │                            │  ace.auth.account_deleted │
   │                            │  .event                    │
   │                            │                            │
   │  7. Success response       │                            │
   │  {message: "User deleted"} │                            │
   │<────────────────────────────│                            │
   │                            │                            │
```

### 3.3 Role Assignment Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          ROLE ASSIGNMENT FLOW                                │
└─────────────────────────────────────────────────────────────────────────────┘

  Admin                     Frontend                     Backend
   │                            │                            │
   │  1. View user roles        │                            │
   │  GET /admin/users/:id      │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │  2. Update role            │                            │
   │  PUT /admin/users/:id/role │                            │
   │  {role: "admin"}           │                            │
   │────────────────────────────>│                            │
   │                            │                            │
   │                            │  3. Verify admin role       │
   │                            │                            │
   │                            │  4. Validate role          │
   │                            │  - Must be valid enum      │
   │                            │  - admin can't demote self │
   │                            │                            │
   │                            │  5. Update user role        │
   │                            │  - roles: ["admin"]        │
   │                            │                            │
   │                            │  6. Invalidate permission   │
   │                            │     cache                  │
   │                            │  - Valkey DEL             │
   │                            │    "perm:{user_id}:*"     │
   │                            │                            │
   │                            │  7. Publish event          │
   │                            │  ace.auth.role_change     │
   │                            │  .event                    │
   │                            │                            │
   │  8. Success response       │                            │
   │  {message: "Role updated"} │                            │
   │<────────────────────────────│                            │
   │                            │                            │
```

**API Request/Response:**

```http
PUT /auth/admin/users/550e8400-e29b-41d4-a716-446655440000/role
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "role": "admin"
}
```

```json
{
  "success": true,
  "message": "User role updated successfully.",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "new_role": "admin",
  "updated_at": "2026-04-09T12:00:00Z"
}
```

---

## 4. User Stories (Detailed)

### US-1: User Registration with Email/Password

```gherkin
Feature: Email/Password Registration

  Scenario: Successful registration with valid credentials
    Given the user is on the registration page
    And no account exists with email "newuser@example.com"
    When the user submits registration with:
      | email | password |
      | newuser@example.com | SecureP@ss123 |
    Then the system creates a new user account with status "pending_verification"
    And sends a verification email to "newuser@example.com"
    And returns success response with user_id
    And publishes "ace.auth.user_registered.event"

  Scenario: Registration fails with weak password
    Given the user is on the registration page
    When the user submits registration with:
      | email | password |
      | user@example.com | weak |
    Then the system returns HTTP 400 with error "WEAK_PASSWORD"
    And no user account is created

  Scenario: Registration fails with duplicate email
    Given an account exists with email "existing@example.com"
    And the user is on the registration page
    When the user submits registration with:
      | email | password |
      | existing@example.com | SecureP@ss123 |
    Then the system returns HTTP 409 with error "EMAIL_EXISTS"
    And no duplicate account is created

  Scenario: Registration rate limited
    Given the user has attempted 10 registrations in the last minute
    And the user is on the registration page
    When the user submits registration
    Then the system returns HTTP 429 with error "RATE_LIMITED"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-1 |
| **Title** | User Registration with Email/Password |
| **User Persona** | New user (Hobbyist, SaaS User) |
| **User Goal** | Create an account to access the platform |
| **User Benefit** | Establish identity to save and access personal data |
| **Pre-conditions** | User is not logged in; registration is enabled |
| **Main Flow** | 1. User fills registration form → 2. System validates input → 3. Hash password → 4. Create user record → 5. Generate verification token → 6. Send verification email → 7. Return success |
| **Alternative Flows** | Invalid email format; weak password; duplicate email; rate limited |
| **Post-conditions** | User record created with pending_verification status; verification email sent |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-2: User Login with Email/Password

```gherkin
Feature: Email/Password Login

  Background:
    Given a verified user exists with email "user@example.com" and password "CorrectP@ss123"

  Scenario: Successful login with correct credentials
    Given the user is on the login page
    And no active session exists
    When the user submits login with:
      | email | password |
      | user@example.com | CorrectP@ss123 |
    Then the system returns HTTP 200 with access_token and refresh_token
    And resets failed login attempts for this user
    And publishes "ace.auth.login.event" to NATS

  Scenario: Failed login with incorrect password
    Given the user is on the login page
    When the user submits login with:
      | email | password |
      | user@example.com | WrongP@ss123 |
    Then the system returns HTTP 401 with error "INVALID_CREDENTIALS"
    And increments failed login counter
    And publishes "ace.auth.failed_login.event" to NATS

  Scenario: Account locked after 5 failed attempts
    Given the user has 4 failed login attempts
    When the user submits login with wrong password
    Then the system returns HTTP 429 with error "ACCOUNT_LOCKED"
    And locks the account for 15 minutes
    And publishes "ace.auth.account_locked.event" to NATS

  Scenario: Login rejected for unverified email
    Given a user exists with email "unverified@example.com" and status "pending_verification"
    When the user submits login with correct credentials
    Then the system returns HTTP 403 with error "EMAIL_NOT_VERIFIED"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-2 |
| **Title** | User Login with Email/Password |
| **User Persona** | Registered user (Hobbyist, SaaS User) |
| **User Goal** | Access my account securely |
| **User Benefit** | Access saved work, get personalized experience |
| **Pre-conditions** | User has registered and verified email |
| **Main Flow** | 1. User enters credentials → 2. Rate limit check → 3. Validate credentials → 4. Generate tokens → 5. Store session → 6. Publish event → 7. Return tokens |
| **Alternative Flows** | Invalid credentials; account locked; email not verified |
| **Post-conditions** | Valid JWT access token and refresh token issued |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-3: SSO Login via Google

```gherkin
Feature: Google OAuth Login

  Scenario: New user logs in with Google for the first time
    Given the user clicks "Login with Google"
    When the OAuth flow completes with Google user "user@gmail.com"
    Then the system creates a new user with email "user@gmail.com"
    And links Google OAuth provider to the account
    And sets user status to "active" (email pre-verified by Google)
    And returns tokens
    And publishes "ace.auth.login.event" with provider "google"

  Scenario: Existing email/password user links Google on login
    Given a user exists with email "user@example.com" and password
    And the user completes Google OAuth with email "user@example.com"
    When the OAuth flow completes
    Then the system links Google OAuth provider to existing account
    And returns tokens
    And does NOT create a new user

  Scenario: Google login blocked due to OAuth state mismatch (CSRF)
    Given the user initiates Google OAuth
    When the OAuth callback has an invalid state parameter
    Then the system returns HTTP 400 with error "INVALID_OAUTH_STATE"
    And does not authenticate the user
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-3 |
| **Title** | SSO Login via Google |
| **User Persona** | SaaS User, Developer |
| **User Goal** | Log in without creating a new password |
| **User Benefit** | Faster signup, no new credentials to remember |
| **Pre-conditions** | Google OAuth configured; user has Google account |
| **Main Flow** | 1. User clicks Google login → 2. Generate CSRF state → 3. Redirect to Google → 4. User authenticates → 5. OAuth callback → 6. Validate state → 7. Exchange code → 8. Create/link user → 9. Generate tokens |
| **Alternative Flows** | CSRF attack detected; email already exists with different auth |
| **Post-conditions** | User authenticated; Google provider linked to account |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-4: SSO Login via GitHub

```gherkin
Feature: GitHub OAuth Login

  Scenario: Developer logs in with GitHub for the first time
    Given the user clicks "Login with GitHub"
    When the OAuth flow completes with GitHub user
    Then the system creates a new user
    And links GitHub OAuth provider to the account
    And returns tokens
    And publishes "ace.auth.login.event" with provider "github"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-4 |
| **Title** | SSO Login via GitHub |
| **User Persona** | Developer User |
| **User Goal** | Log in using existing GitHub identity |
| **User Benefit** | Single sign-on; potential GitHub API integration |
| **Pre-conditions** | GitHub OAuth configured; user has GitHub account |
| **Main Flow** | Same as Google OAuth (US-3) |
| **Alternative Flows** | Same as Google OAuth (US-3) |
| **Post-conditions** | User authenticated; GitHub provider linked |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-5: Link Multiple OAuth Providers

```gherkin
Feature: Multiple OAuth Provider Linking

  Background:
    Given a user is logged in with email "user@example.com"
    And the user has linked Google OAuth to their account

  Scenario: Link GitHub to existing account
    Given the user is on account settings page
    And GitHub OAuth is not linked
    When the user initiates GitHub OAuth linking
    And completes the OAuth flow
    Then GitHub OAuth is linked to the account
    And the user can now log in with either Google or GitHub

  Scenario: Unlink OAuth provider
    Given the user is on account settings page
    And Google OAuth is linked
    And the account has at least one other auth method (password)
    When the user unlinks Google OAuth
    Then Google OAuth is unlinked from the account
    And the user cannot login with Google anymore

  Scenario: Cannot unlink last auth method
    Given the user is on account settings page
    And Google OAuth is the only linked auth method
    And the account has no password
    When the user attempts to unlink Google OAuth
    Then the system returns HTTP 400 with error "CANNOT_REMOVE_LAST_AUTH_METHOD"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-5 |
| **Title** | Link Multiple OAuth Providers |
| **User Persona** | SaaS User |
| **User Goal** | Use multiple login methods interchangeably |
| **User Benefit** | Flexibility; backup login method |
| **Pre-conditions** | User is authenticated |
| **Main Flow** | 1. User goes to settings → 2. Initiates OAuth linking → 3. Completes OAuth → 4. Provider linked |
| **Alternative Flows** | Attempting to remove last auth method |
| **Post-conditions** | Multiple OAuth providers linked |
| **Priority** | Should have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-6: User Logout

```gherkin
Feature: User Logout

  Scenario: Successful logout
    Given a user is logged in with valid tokens
    When the user clicks "Logout"
    Then the system invalidates the current session's refresh token
    And publishes "ace.auth.logout.event" to NATS
    And returns success response
    And clears the frontend session

  Scenario: Global logout (all devices)
    Given a user is logged in
    When the user changes their password
    Then all refresh tokens for the user are invalidated
    And the user must re-authenticate on all devices
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-6 |
| **Title** | User Logout |
| **User Persona** | Any authenticated user |
| **User Goal** | Secure my account on shared devices |
| **User Benefit** | Prevents unauthorized access from current device |
| **Pre-conditions** | User is authenticated with valid tokens |
| **Main Flow** | 1. User clicks logout → 2. Backend revokes refresh token → 3. Publish event → 4. Return success |
| **Alternative Flows** | Password change triggers global logout |
| **Post-conditions** | Current session tokens invalidated |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-7: Password Reset

```gherkin
Feature: Password Reset

  Scenario: Request password reset for existing user
    Given a user exists with email "user@example.com"
    And the user is on the password reset page
    When the user submits email "user@example.com"
    Then the system always returns success (prevents email enumeration)
    And if user exists, generates a reset token
    And sends reset email with the token

  Scenario: Reset password with valid token
    Given a user has a valid password reset token
    When the user submits new password "NewSecureP@ss456" with the token
    Then the system updates the password hash
    And invalidates the reset token
    And invalidates all user sessions
    And publishes "ace.auth.password_change.event"

  Scenario: Reset password with expired token
    Given a user has an expired password reset token
    When the user submits new password with the token
    Then the system returns HTTP 400 with error "TOKEN_EXPIRED"
    And does not update password

  Scenario: Reset password rate limited
    Given the user has requested 3 password resets in the last hour for email "user@example.com"
    When the user submits another password reset request
    Then the system returns HTTP 429 with error "RATE_LIMITED"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-7 |
| **Title** | Password Reset |
| **User Persona** | User who forgot password |
| **User Goal** | Regain access to my account |
| **User Benefit** | Recover account without losing access |
| **Pre-conditions** | User account exists with valid email |
| **Main Flow** | 1. User submits email → 2. System generates reset token → 3. Email sent → 4. User clicks link → 5. Submits new password → 6. Password updated → 7. Sessions invalidated |
| **Alternative Flows** | Expired token; rate limited; invalid token |
| **Post-conditions** | New password set; all sessions invalidated |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-8: Email Verification

```gherkin
Feature: Email Verification

  Scenario: Verify email with valid token
    Given a user exists with status "pending_verification"
    And the user has a valid email verification token
    When the user clicks the verification link
    Then the system updates user status to "active"
    And marks email as verified
    And invalidates the verification token
    And publishes "ace.auth.email_verified.event"

  Scenario: Verify email with expired token
    Given a user has an expired verification token
    When the user clicks the verification link
    Then the system returns HTTP 400 with error "TOKEN_EXPIRED"
    And does not update user status
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-8 |
| **Title** | Email Verification |
| **User Persona** | New registered user |
| **User Goal** | Confirm my email address |
| **User Benefit** | Unlocks full account features; enables password reset |
| **Pre-conditions** | User registered but not verified |
| **Main Flow** | 1. User clicks verification link → 2. Token validated → 3. Status updated to active |
| **Alternative Flows** | Expired token; already verified |
| **Post-conditions** | Email verified; account active |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-9: Role-Based Access Control

```gherkin
Feature: Role-Based Access Control

  Scenario: Admin can access user management
    Given a user is logged in with role "admin"
    When the user requests GET /admin/users
    Then the system returns HTTP 200 with user list

  Scenario: Regular user cannot access admin endpoints
    Given a user is logged in with role "user"
    When the user requests GET /admin/users
    Then the system returns HTTP 403 with error "INSUFFICIENT_PERMISSIONS"

  Scenario: Viewer can read shared resources
    Given a user is logged in with role "viewer"
    And a resource exists that is shared with this user with permission "view"
    When the user requests GET /api/resources/{id}
    Then the system returns HTTP 200 with resource data

  Scenario: Viewer cannot modify shared resources
    Given a user is logged in with role "viewer"
    And a resource exists that is shared with this user with permission "view"
    When the user requests PUT /api/resources/{id}
    Then the system returns HTTP 403 with error "INSUFFICIENT_PERMISSIONS"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-9 |
| **Title** | Role-Based Access Control |
| **User Persona** | Platform Admin, Team Admin, User, Viewer |
| **User Goal** | Control what users can do on the platform |
| **User Benefit** | Security; appropriate access levels |
| **Pre-conditions** | User authenticated |
| **Main Flow** | 1. Request received → 2. Auth middleware extracts role → 3. Check role permissions → 4. Grant/deny |
| **Alternative Flows** | Insufficient permissions |
| **Post-conditions** | Request proceeds or 403 returned |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-10: Resource-Level Authorization

```gherkin
Feature: Resource-Level Authorization

  Background:
    Given user "owner@example.com" owns an agent
    And user "viewer@example.com" is logged in

  Scenario: Owner has full access to their resource
    Given "owner@example.com" is logged in
    And requests access to their own agent
    Then the system grants access with all permissions

  Scenario: Shared user can view shared resource
    Given "viewer@example.com" has "view" permission on the agent
    When "viewer@example.com" requests GET /api/agents/{id}
    Then the system returns HTTP 200 with agent data

  Scenario: User without permission is denied access
    Given "viewer@example.com" has no permission on the agent
    When "viewer@example.com" requests GET /api/agents/{id}
    Then the system returns HTTP 403 with error "RESOURCE_ACCESS_DENIED"

  Scenario: User with "use" permission can execute but not modify
    Given "user@example.com" has "use" permission on the agent
    When "user@example.com" requests POST /api/agents/{id}/execute
    Then the system returns HTTP 200
    But when "user@example.com" requests PUT /api/agents/{id}
    Then the system returns HTTP 403
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-10 |
| **Title** | Resource-Level Authorization |
| **User Persona** | Any user who owns or shares resources |
| **User Goal** | Control who can access my agents |
| **User Benefit** | Collaboration while protecting sensitive work |
| **Pre-conditions** | User authenticated; resource exists |
| **Main Flow** | 1. Request resource → 2. Auth check → 3. Check ownership or permissions → 4. Grant/deny |
| **Alternative Flows** | Resource not found; permission denied |
| **Post-conditions** | Resource access granted or 403 returned |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-11: View Shared Resources

```gherkin
Feature: View Shared Resources

  Scenario: Viewer lists shared resources
    Given a user with role "viewer" is logged in
    And resources are shared with this user
    When the user requests GET /api/resources/shared
    Then the system returns HTTP 200 with list of shared resources
    And each resource includes shared_by information

  Scenario: Viewer can read but not modify shared content
    Given a user with role "viewer" is logged in
    And a document is shared with this user
    When the user views the document
    Then the system returns HTTP 200 with document content
    But when the user attempts to edit the document
    Then the system returns HTTP 403
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-11 |
| **Title** | View Shared Resources |
| **User Persona** | Viewer role, Team Member |
| **User Goal** | Review shared work without modifying it |
| **User Benefit** | Collaboration; read-only access to team assets |
| **Pre-conditions** | User has viewer role or resource shared with view permission |
| **Main Flow** | 1. List shared resources → 2. View individual resources |
| **Alternative Flows** | No shared resources; permission insufficient |
| **Post-conditions** | Resources displayed (read-only) |
| **Priority** | Should have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-12: Unauthenticated Access Rejection

```gherkin
Feature: Unauthenticated Access Rejection

  Scenario: Missing token returns 401
    Given a user is not authenticated
    When the user requests a protected endpoint without token
    Then the system returns HTTP 401 with error "UNAUTHORIZED"
    And includes WWW-Authenticate header

  Scenario: Invalid token returns 401
    Given a user is not authenticated
    When the user requests a protected endpoint with invalid token
    Then the system returns HTTP 401 with error "INVALID_TOKEN"

  Scenario: Expired token returns 401
    Given a user has an expired access token
    When the user requests a protected endpoint
    Then the system returns HTTP 401 with error "TOKEN_EXPIRED"

  Scenario: Valid token but insufficient permissions returns 403
    Given a user is authenticated with valid token
    But the user lacks required permissions for the endpoint
    When the user requests the protected endpoint
    Then the system returns HTTP 403 with error "INSUFFICIENT_PERMISSIONS"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-12 |
| **Title** | Unauthenticated Access Rejection |
| **User Persona** | All users (security requirement) |
| **User Goal** | Prevent unauthorized access to protected data |
| **User Benefit** | Security; data protection |
| **Pre-conditions** | Request to protected endpoint |
| **Main Flow** | 1. Request received → 2. Extract token → 3. Validate → 4. Return 401/403 or proceed |
| **Alternative Flows** | Missing token; invalid token; expired token; insufficient permissions |
| **Post-conditions** | Request rejected with appropriate error code |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-13: Single-User Mode

```gherkin
Feature: Single-User Mode

  Scenario: First user becomes admin automatically
    Given the system is configured in single-user mode
    And no users exist
    When a new user registers
    Then the user is assigned role "admin"
    And registration may be disabled via configuration

  Scenario: Second registration blocked in single-user mode
    Given the system is configured in single-user mode
    And one user already exists
    When another user attempts to register
    Then the system returns HTTP 403 with error "REGISTRATION_DISABLED"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-13 |
| **Title** | Single-User Mode |
| **User Persona** | Hobbyist User |
| **User Goal** | Use system without complex setup |
| **User Benefit** | Out-of-the-box experience; admin access |
| **Pre-conditions** | DEPLOYMENT_MODE=single |
| **Main Flow** | 1. First user registers → 2. Automatically becomes admin |
| **Alternative Flows** | Registration disabled after first user |
| **Post-conditions** | First user has admin role |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-14: Multi-User Mode

```gherkin
Feature: Multi-User Mode

  Scenario: Open registration in multi-user mode
    Given the system is configured in multi-user mode
    When any new user registers
    Then the user account is created
    And assigned role "user"
    And registration remains open for others

  Scenario: Users are isolated by default
    Given user A and user B exist
    And user A creates a private resource
    When user B attempts to access user A's resource
    Then the system returns HTTP 403
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-14 |
| **Title** | Multi-User Mode |
| **User Persona** | SaaS Operator |
| **User Goal** | Support multiple independent users |
| **User Benefit** | SaaS deployment capability |
| **Pre-conditions** | DEPLOYMENT_MODE=multi |
| **Main Flow** | 1. Users register → 2. Users create isolated resources |
| **Alternative Flows** | Resource sharing required for cross-user access |
| **Post-conditions** | Users isolated; sharing opt-in |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-15: Change Password

```gherkin
Feature: Change Password

  Background:
    Given a user is logged in with email "user@example.com"
    And the user's current password is "OldP@ss123"

  Scenario: Successful password change
    When the user submits password change with:
      | current_password | new_password |
      | OldP@ss123 | NewP@ss456 |
    Then the system validates current password
    And updates password hash to new password
    And invalidates all existing sessions
    And publishes "ace.auth.password_change.event"

  Scenario: Password change with incorrect current password
    When the user submits password change with:
      | current_password | new_password |
      | WrongP@ss | NewP@ss456 |
    Then the system returns HTTP 400 with error "INVALID_CURRENT_PASSWORD"
    And does not change password

  Scenario: Password change with weak new password
    When the user submits password change with:
      | current_password | new_password |
      | OldP@ss123 | weak |
    Then the system returns HTTP 400 with error "WEAK_PASSWORD"
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-15 |
| **Title** | Change Password |
| **User Persona** | Authenticated user |
| **User Goal** | Maintain account security |
| **User Benefit** | Proactive security; all devices re-authenticated |
| **Pre-conditions** | User authenticated with valid session |
| **Main Flow** | 1. User submits current + new password → 2. Validate current → 3. Update hash → 4. Invalidate sessions → 5. Publish event |
| **Alternative Flows** | Wrong current password; weak new password |
| **Post-conditions** | Password changed; all sessions invalidated |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-16: Account Deletion

```gherkin
Feature: Account Deletion

  Background:
    Given a user is logged in
    And the user owns several resources

  Scenario: User initiates self-deletion
    When the user submits account deletion request
    Then the system soft-deletes the user account
    And sets deleted_at timestamp
    And anonymizes email (user_{id}@deleted.local)
    And invalidates all sessions immediately
    And publishes "ace.auth.account_deleted.event"
    And marks all user resources for cleanup

  Scenario: Admin deletes user account
    Given an admin is logged in
    When the admin requests deletion of user {user_id}
    Then the system soft-deletes the target user
    And invalidates all their sessions
    And publishes "ace.auth.account_deleted.event"
    And returns success response
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-16 |
| **Title** | Account Deletion |
| **User Persona** | Any user, Admin |
| **User Goal** | Remove my data from the platform |
| **User Benefit** | GDPR compliance; privacy control |
| **Pre-conditions** | User authenticated (or admin authenticated for others) |
| **Main Flow** | 1. Deletion request → 2. Soft delete user → 3. Anonymize data → 4. Invalidate sessions → 5. Publish event → 6. Schedule cleanup |
| **Alternative Flows** | Admin deleting another user |
| **Post-conditions** | User soft-deleted; sessions revoked |
| **Priority** | Must have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

### US-16a: Account Suspension

```gherkin
Feature: Account Suspension

  Background:
    Given an admin is logged in
    And a user exists with email "user@example.com"

  Scenario: Admin suspends user account
    When the admin submits suspension of user "user@example.com"
    And provides reason "Violation of terms"
    Then the system updates user status to "suspended"
    And sets suspended_at timestamp
    And records suspension reason
    And invalidates all user sessions
    And publishes "ace.auth.account_suspended.event"

  Scenario: Suspended user cannot log in
    Given user "user@example.com" is suspended
    When the user attempts to log in
    Then the system returns HTTP 403 with error "ACCOUNT_SUSPENDED"
    And includes suspension information

  Scenario: Admin reverses suspension
    Given user "user@example.com" is suspended
    When the admin submits account restoration for "user@example.com"
    Then the system updates user status to "active"
    And publishes "ace.auth.account_restored.event"
    And the user can log in again
```

**Story Details:**
| Field | Value |
|-------|-------|
| **ID** | US-16a |
| **Title** | Account Suspension |
| **User Persona** | Platform Admin |
| **User Goal** | Temporarily revoke access for policy violations |
| **User Benefit** | Security enforcement; compliance |
| **Pre-conditions** | Admin authenticated |
| **Main Flow** | 1. Admin requests suspension → 2. Update status → 3. Invalidate sessions → 4. Publish event |
| **Alternative Flows** | User login attempts blocked; admin can restore |
| **Post-conditions** | User suspended; sessions invalidated |
| **Priority** | Should have |
| **Acceptance Criteria** | See Gherkin scenarios above |

---

## 5. Acceptance Criteria Mapping

| Story ID | Story Title | Acceptance Criteria | Test Priority | BSD Reference |
|----------|-------------|---------------------|---------------|---------------|
| US-1 | User Registration with Email/Password | Registration works with valid input; weak passwords rejected; duplicate emails handled; rate limiting active | Must have | AC-1 |
| US-2 | User Login with Email/Password | Successful login returns tokens; failed logins tracked; account lockout works; events published | Must have | AC-2 |
| US-3 | SSO Login via Google | OAuth flow completes; new users auto-registered; existing users can link; CSRF prevented | Must have | AC-3 |
| US-4 | SSO Login via GitHub | OAuth flow completes; new users auto-registered; existing users can link; CSRF prevented | Must have | AC-4 |
| US-5 | Link Multiple OAuth Providers | Users can link/unlink providers; at least one auth method required | Should have | AC-5 |
| US-6 | User Logout | Session invalidated; event published; all devices logged out on password change | Must have | AC-6 |
| US-7 | Password Reset | Reset email sent; valid tokens work; expired tokens rejected; rate limited | Must have | AC-7 |
| US-8 | Email Verification | Valid tokens activate account; expired tokens rejected | Must have | AC-8 |
| US-9 | Role-Based Access Control | Admin can access admin endpoints; regular users cannot; viewers have limited access | Must have | AC-9 |
| US-10 | Resource-Level Authorization | Owner has full access; shared users have appropriate permissions; unauthorized access blocked | Must have | AC-10 |
| US-11 | View Shared Resources | Viewers can list and read shared resources; cannot modify | Should have | AC-11 |
| US-12 | Unauthenticated Access Rejection | Missing token returns 401; invalid token returns 401; insufficient permissions returns 403 | Must have | AC-12 |
| US-13 | Single-User Mode | First user becomes admin; registration disabled after first user | Must have | AC-13 |
| US-14 | Multi-User Mode | Open registration; users isolated by default | Must have | AC-14 |
| US-15 | Change Password | Password changes successfully; all sessions invalidated; weak passwords rejected | Must have | AC-15 |
| US-16 | Account Deletion | Soft delete works; data anonymized; sessions revoked; events published | Must have | AC-16 |
| US-16a | Account Suspension | Admin can suspend users; suspended users cannot login; admin can restore | Should have | AC-16a |

---

## Appendix: API Response Format

### Success Response

```json
{
  "success": true,
  "data": { ... },
  "message": "Operation completed successfully"
}
```

### Error Response

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": { ... }
  }
}
```

### Pagination Response

```json
{
  "success": true,
  "data": [ ... ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

---

**Document Version:** 1.0  
**Unit:** users-auth  
**Status:** Draft  
**Created:** 2026-04-09
