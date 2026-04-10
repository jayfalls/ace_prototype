# Security Considerations: Users-Auth Unit

<!--
Intent: Define security requirements, threat modeling, and controls for the users-auth unit.
Scope: Authentication, authorization, data protection, input validation, and compliance.
Used by: AI agents implementing the auth system.
-->

---

## 1. Security Overview

The users-auth unit implements a **defense-in-depth** security model with multiple layers of protection:

- **Zero Trust**: All requests authenticated by default; no implicit trust
- **Minimum Privilege**: Users receive lowest role necessary; escalate only when needed
- **Secure by Design**: Security controls built into core architecture, not added retroactively

This security model protects:
- User credentials and authentication flows
- Session management and token validation
- Authorization and access control
- Sensitive user data (PII, passwords, tokens)

---

## 2. Authentication

| Method | Description | Implementation |
|--------|-------------|----------------|
| **Email/Password** | Primary authentication for registered users | Argon2id hashing (64MB memory, 3 iterations), rate limited (5 attempts/15min), 15min lockout after threshold |
| **Magic Link** | Passwordless login via email | Token stored as SHA256 hash in DB, single-use, time-limited (15-min expiry) |
| **JWT Access Token** | Session token for API requests | RS256 signed, 15-min expiry (configurable), contains user claims |
| **Refresh Token** | Long-lived token for session renewal | Opaque UUID, rotated on each use, 7-day expiry, Valkey blacklist for revocation |

### 2.1 Token Security

**JWT Access Token:**
- Signed with RS256 (asymmetric) — private key signs, public key verifies
- Contains: `iss` (issuer), `sub` (user ID), `aud` (audience), `exp` (expiry), `iat` (issued), `jti` (unique ID), `role`, `email`
- Public key exposed via JWKS endpoint at `/auth/.well-known/jwks.json`
- Signature verified on every protected request

**Refresh Token:**
- Opaque UUID v4, never exposed as JWT
- Stored as SHA256 hash in PostgreSQL `sessions` table
- Rotated on each refresh (old token invalidated, new token issued)
- Revocation supported via Valkey blacklist: `token:revoked:{jti}`

### 2.2 Password Security

- **Algorithm**: Argon2id (winner of Password Hashing Competition, OWASP recommended)
- **Parameters**: 64MB memory, 3 iterations, 4-way parallelism (matching CPU cores)
- **Salt**: 16 bytes cryptographically random, unique per password
- **Key Length**: 32 bytes output
- **Verification**: Constant-time comparison to prevent timing attacks

---

## 3. Authorization

### 3.1 Role-Based Access Control (RBAC)

| Resource | Permission | Access Control |
|----------|------------|----------------|
| **User profile** | read/write | Owner only — verified via `user_id` in JWT claims |
| **Admin endpoints** (`/admin/*`) | admin role | RBAC middleware enforces `role == admin` |
| **User management** | admin role | RBAC middleware enforces `role == admin` |
| **Session management** | owner/admin | Verify session.user_id matches JWT subject or role is admin |
| **Resource permissions** | view/use/admin | PermissionService checks permission table |

### 3.2 RBAC Roles

| Role | Description | Permissions |
|------|------------|------------|
| **admin** | Platform/system administrator | Full access to all operations, user management, role assignment, audit access |
| **user** | Standard authenticated user | Own resources, limited system access, can share resources |
| **viewer** | Read-only access | View shared resources, no modifications, no own resource creation |

### 3.3 Resource-Level Permissions

| Permission Level | Capabilities |
|---------------|------------|
| **view** | Read-only access to resource |
| **use** | Execute/use the resource |
| **admin** | Full control including sharing and deletion |

---

## 4. Data Protection

### 4.1 Sensitive Data Classification

| Data | Classification | Protection |
|------|---------------|------------|
| **Password hash** | Secret | Argon2id (64MB memory), salt unique per user, never stored plaintext |
| **Refresh token** | Secret | SHA256 hashed in PostgreSQL, SHA256 hashed in Valkey cache |
| **JWT private key** | Secret | File permissions 0600, environment variable or secrets manager |
| **User email** | PII | Not logged, encrypted at rest in PostgreSQL |
| **Magic link token** | Secret | SHA256 hashed in database, single-use |
| **Session data** | Confidential | Valkey with TTL, encrypted connection |

### 4.2 Encryption

**At Rest:**
- PostgreSQL: Tablespace encryption if available; otherwise relies on filesystem-level encryption
- Valkey: In-memory only (no persistence by design)

**In Transit:**
- TLS 1.2 minimum required for all connections
- TLS 1.3 preferred for client connections
- HSTS header enforced on all auth endpoints
- Secure cookies: `httpOnly=true`, `secure=true`, `sameSite=lax`

---

## 5. Input Validation

| Input | Validation Rules | Error Response |
|-------|-----------------|----------------|
| **Email** | RFC 5322 format, max 255 chars, lowercase normalized | 400 Invalid email format |
| **Password** | 8+ chars, upper+lower+number (symbol configurable) | 400 Password does not meet requirements |
| **Magic link token** | UUID format, 32 hex chars | 400 Invalid token format |
| **JWT** | RS256 signature valid, not expired, issuer validated | 401 Token invalid/expired |
| **Refresh token** | UUID format, exists in DB, not revoked | 401 INVALID_REFRESH_TOKEN |
| **Role** | Must be admin/user/viewer | 400 Invalid role |
| **Permission level** | Must be view/use/admin | 400 Invalid permission |

### 5.1 Validation Implementation

```go
// Email validation (RFC 5322 subset)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Password requirements
- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter  
- At least one number
- Optional: at least one symbol

// Role validation (enum)
var validRoles = map[string]bool{
    "admin":  true,
    "user":   true,
    "viewer": true,
}
```

---

## 6. Threat Modeling

### 6.1 STRIDE Analysis

| Threat | Description | Mitigation |
|--------|-------------|------------|
| **Spoofing** | Attacker poses as valid user with stolen credentials | Argon2id password hashing (GPU-resistant), rate limiting, account lockout after 5 failures |
| **Tampering** | Attacker modifies token claims | RS256 signatures verify token integrity, DB constraints prevent injection |
| **Repudiation** | User denies performing action | NATS events with full audit trail (ace.auth.*.event), structured logging |
| **Information Disclosure** | Sensitive data leaked via logs/errors | No PII in logs, httpOnly cookies, input sanitization |
| **Denial of Service** | Attacker exhausts resources | Rate limiting per IP + per-email, sliding window algorithm |
| **Elevation of Privilege** | Attacker escalates from user to admin | RBAC middleware + DB CHECK constraints, explicit role checks |

### 6.2 Attack Surfaces

| Endpoint | Attack Vector | Protection |
|----------|------------|------------|
| `/auth/register` | Registration spam | Rate limit: 3 requests/15min per IP |
| `/auth/login` | Brute force credential attack | Rate limit: 5 attempts/15min per IP + email; 15min lockout after 5 failures |
| `/auth/magic-link/request` | Token enumeration | Rate limit: 3 requests/15min per email |
| `/auth/password/reset/request` | Token enumeration | Rate limit: 3 requests/hour per email |
| `/auth/refresh` | Token theft/replay | Rotation required, Valkey blacklist check |
| **JWT validation bypass** | Expired or manipulated token | Signature verification, expiry check, blacklist check |

### 6.3 Rate Limiting Tiers

| Endpoint | Per-IP Limit | Per-Email Limit | Lockout Threshold |
|----------|-------------|---------------|----------------|
| `/auth/login` | 5/15min | 5/15min | 5 failures → 15min |
| `/auth/register` | 3/15min | — | — |
| `/auth/magic-link/request` | 5/15min | 3/15min | — |
| `/auth/password/reset/request` | 3/15min | 3/hour | — |
| `/auth/refresh` | 10/15min | — | — |

---

## 7. Security Controls

| Control | Type | Implementation |
|---------|------|----------------|
| **Password hashing** | Preventative | Argon2id with 64MB memory, 3 iterations |
| **Rate limiting** | Preventative | Valkey sliding window counter (Lua script atomicity) |
| **Account lockout** | Preventative | 5 failed attempts → 15min lockout (Valkey key with TTL) |
| **Token blacklist** | Preventative | Valkey key `token:revoked:{jti}` with TTL matching remaining token lifetime |
| **RBAC enforcement** | Preventative | Middleware checks role in JWT; DB CHECK constraint on role column |
| **Audit logging** | Detective | NATS events (ace.auth.login.event, ace.auth.failed_login.event, etc.) |
| **Input validation** | Preventative | Sanitization + type checking + length limits |
| **CSRF protection** | Preventative | SameSite cookies, SvelteKit built-in origin checking |
| **Token rotation** | Preventative | Refresh token invalidated on each use, new token issued |

### 7.1 Audit Event Types

| Event | Subject | Purpose |
|-------|---------|---------|
| `ace.auth.login.event` | Login success | Audit trail |
| `ace.auth.failed_login.event` | Login failure | Security monitoring |
| `ace.auth.account_locked.event` | Lockout trigger | Security alerting |
| `ace.auth.logout.event` | User logout | Session tracking |
| `ace.auth.password_change.event` | Password update | Security audit |
| `ace.auth.role_change.event` | Admin role change | Audit trail |
| `ace.auth.token_revoked.event` | Token/session revocation | Security audit |

---

## 8. Security Testing Checklist

### 8.1 Unit Tests

- [ ] Unit tests for password hashing with Argon2id
- [ ] Unit tests for password verification (correct/incorrect)
- [ ] Unit tests for JWT generation (RS256 signing)
- [ ] Unit tests for JWT validation (valid/expired/invalid signature)
- [ ] Unit tests for RBAC middleware (admin/user/viewer permissions)
- [ ] Unit tests for rate limiting logic (sliding window algorithm)
- [ ] Unit tests for email validation (valid/invalid formats)
- [ ] Unit tests for password validation (meeting/not meeting requirements)
- [ ] Unit tests for magic link token validation
- [ ] Unit tests for token blacklist checking

### 8.2 Integration Tests

- [ ] Integration test: Login flow (register → login → access token)
- [ ] Integration test: Magic link flow (request → verify → login)
- [ ] Integration test: Token refresh flow
- [ ] Integration test: Lockout mechanism after 5 failures
- [ ] Integration test: Rate limiting enforcement
- [ ] Integration test: Admin endpoints blocked for non-admin users
- [ ] Integration test: Password change invalidates sessions
- [ ] Integration test: Session revocation

### 8.3 Penetration Testing

- [ ] Brute force attack: Login endpoint throttled
- [ ] Credential stuffing: Multiple email addresses blocked
- [ ] Token manipulation: Modified JWT rejected
- [ ] Privilege escalation: User cannot access admin endpoints
- [ ] Token enumeration: Magic link endpoints return consistent responses
- [ ] Password reset enumeration: Generic responses (no email existence disclosure)
- [ ] Session fixation: New tokens issued on login, old invalidated

---

## 9. Compliance

### 9.1 GDPR Requirements

| Requirement | Implementation |
|-------------|----------------|
| **Consent** | User explicitly agrees to terms during registration |
| **Data minimization** | Only necessary fields collected (email, password hash) |
| **Right to access** | User can request account data export |
| **Right to deletion** | Soft delete (account marked deleted_at) with scheduled purge |
| **Data portability** | User can export account data in JSON format |
| **Storage limitation** | Passwords never stored, only hashes retained |

### 9.2 OWASP Alignment

- **Password Storage**: Argon2id (OWASP recommended)
- **Authentication**: Multi-factor auth ready (deferred to future)
- **Access Control**: RBAC with explicit role enforcement
- **Input Validation**: Whitelist approach, reject invalid

---

## 10. Incident Response

### 10.1 Detection and Alerting

| Incident | Detection | Response |
|----------|----------|---------|
| **Credential stuffing** | Multiple failed logins from different IPs | IP block, alert on threshold |
| **Account lockout cascade** | High lockout rate | Review rate limits, possible attack |
| **Token theft suspected** | Refresh from unexpected IP | Invalidate all user tokens, force re-login |
| **Brute force attack** | Login rate limit exceeded | IP block, alert |

### 10.2 Recovery Procedures

1. **Compromised credentials**: Password reset required, all sessions invalidated
2. **Token theft**: All tokens for user added to blacklist via Valkey
3. **Account compromise**: Admin can suspend account, revoke all sessions
4. **Security event**: NATS event published to `ace.auth.*.event` for SIEM integration

### 10.3 Monitoring Alerts

- [ ] High rate of failed login attempts (>10/minute per IP)
- [ ] Account lockout rate spike (>5% of users locked)
- [ ] Unusual token refresh failures
- [ ] JWT validation failures
- [ ] Rate limit rejections

---

## Appendix: Security Configuration

### Required Environment Variables

```bash
# JWT (RS256)
AUTH_JWT_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----"
AUTH_JWT_PUBLIC_KEY="-----BEGIN PUBLIC KEY-----"
AUTH_ACCESS_TOKEN_TTL="15m"
AUTH_REFRESH_TOKEN_TTL="168h"

# Security
AUTH_PASSWORD_MIN_LENGTH="8"
AUTH_PASSWORD_REQUIRE_UPPER="true"
AUTH_PASSWORD_REQUIRE_LOWER="true"
AUTH_PASSWORD_REQUIRE_NUMBER="true"
AUTH_LOCKOUT_THRESHOLD="5"
AUTH_LOCKOUT_DURATION="15m"

# Rate limiting
AUTH_LOGIN_RATE_LIMIT_PER_IP="5"
AUTH_LOGIN_RATE_LIMIT_PER_EMAIL="5"
AUTH_PASSWORD_RESET_RATE_LIMIT="3"

# Deployment
AUTH_DEPLOYMENT_MODE="multi"
```

### Security Boundaries

- No user data logged (including email addresses)
- Passwords never logged in any context
- JWT claims audited, tokens not logged
- IP addresses logged only in NATS events for audit trail
- Error messages sanitized — no internal details exposed

---

**Document Version:** 1.0  
**Unit:** users-auth  
**Status:** Draft  
**Created:** 2026-04-10