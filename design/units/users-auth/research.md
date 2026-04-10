# Users-Auth Unit Research

## Overview

This document provides technical research and recommendations for the authentication and authorization system (users-auth unit) of the ACE Framework. It addresses the open questions identified in the problem_space.md with actionable recommendations based on current best practices.

**Assumptions:**
- Go backend using Chi router
- SvelteKit frontend
- PostgreSQL database with SQLC
- Valkey for caching/rate limiting
- NATS for event broadcasting
- Go standard library patterns preferred

---

## 1. JWT Token Strategy

### 1.1 Access Token Duration

| Duration | Pros | Cons | Recommendation |
|----------|------|------|----------------|
| **5 minutes** | Minimal exposure if compromised | Frequent refresh, higher latency | High-security applications |
| **15 minutes** | Good balance of security/usability | Standard industry practice | **Recommended for most applications** |
| 30 minutes | Less refresh overhead | Longer exposure window | Low-security internal tools |

**Recommendation:** Use **15 minutes** for access tokens. This is the industry standard (OAuth 2.0 best practice) and balances security with user experience.

### 1.2 Refresh Token Rotation

**Recommendation:** Implement **refresh token rotation** with reuse detection.

Refresh token rotation means:
1. Each time a refresh token is used, issue a new refresh token
2. The old refresh token is immediately invalidated
3. If a stolen refresh token is used after rotation, detect reuse and revoke all sessions

**Why rotation matters:**
- Without rotation, a stolen refresh token is valid until expiry (days/weeks)
- With rotation, a stolen token is only valid for one use
- Reuse detection converts token theft into a one-time attack window

**Implementation approach:**
- Store refresh token hash in PostgreSQL (for persistence across restarts)
- Store revocation state in Valkey (for fast lookup)
- On refresh: validate token → mark old as used → issue new pair

### 1.3 Token Storage

| Storage | Security | Usability | Recommendation |
|---------|----------|----------|----------------|
| **httpOnly cookie** | XSS cannot read, CSRF mitigated with SameSite | Automatic on refresh | **Recommended for web apps** |
| localStorage | XSS can steal tokens | Simple to implement | Not recommended |
| Memory (JS variable) | XSS cannot persist | Requires re-auth on refresh | Good for SPAs, complex |
| Authorization header | Manual management | Standard API pattern | Good for API-only access |

**Recommendation:** Use **httpOnly cookies with SameSite=Lax** for the web frontend. The access token is read-only by the server; the refresh token is in an httpOnly cookie that JavaScript cannot access.

**Cookie configuration:**
```go
httpOnly: true
secure: true  // HTTPS only in production
sameSite: "lax"  // Send on same-site requests and top-level navigations
maxAge: 7 * 24 * 60 * 60  // 7 days for refresh token
```

### 1.4 Token Refresh Endpoint

**Recommendation:** Support **both approaches** for different client types:

| Client Type | Approach | Endpoint | Mechanism |
|-------------|----------|----------|-----------|
| **Browser clients** | Cookie-based automatic | Automatic via httpOnly cookie | Cookie sent automatically, server validates and issues new tokens |
| **API clients** | Dedicated endpoint | `POST /auth/refresh` | Explicit token refresh with request body |

**Dedicated refresh endpoint for API clients:**
```
POST /auth/refresh
Content-Type: application/json

{
    "refresh_token": "eyJhbG..."
}

Response:
{
    "success": true,
    "data": {
        "access_token": "eyJhbG...",
        "refresh_token": "eyJhbG...",  // New rotated token
        "expires_in": 900  // 15 minutes
    }
}
```

**Why both approaches:**
- Browser clients: httpOnly cookies are secure and automatic; no JavaScript needed
- API clients (mobile apps, third-party integrations): Need explicit token management
- The refresh token is still rotated in both cases

### 1.5 Token Revocation Strategy

| Strategy | Mechanism | Use Case |
|----------|-----------|----------|
| **Short-lived tokens (15min)** | No revocation needed, token auto-expires | Simple deployments |
| **Valkey blacklist** | Add jti to denylist with TTL | Immediate revocation needed |
| **Token versioning** | Increment version in DB, invalidate old | Force re-auth on security events |

**Recommendation:** Use a **hybrid approach**:
1. Access tokens are short-lived (15 minutes) — no revocation needed for normal cases
2. Valkey blacklist for immediate revocation (logout, password change, security events)
3. Store revoked token's jti in Valkey with TTL matching remaining token lifetime

**Implementation:**
```
Key: "token:revoked:{jti}"
Value: "1"
TTL: remaining token lifetime (max 15 minutes)
```

### 1.6 RS256 for JWT Signing

**Algorithm:** Use **RS256 (RSA)** for all environments — development and production alike.

| Algorithm | Key Management | Use Case | Recommendation |
|-----------|---------------|----------|----------------|
| **RS256 (RSA)** | Private key signs, public key verifies | All environments | **Required — no exceptions** |
| HS256 (HMAC) | Shared secret, same key signs/verifies | Legacy compatibility | Not used |

**Rationale for RS256 everywhere:**
- The API service signs tokens; other services (cognitive engine, future microservices) can verify with the public key
- Compromised public key cannot forge tokens (only verify)
- Industry standard for OAuth/OIDC
- Dev and prod must be identical to avoid configuration drift

**Key storage:**
- Private key: Environment variable `AUTH_JWT_PRIVATE_KEY`
- Public key: Environment variable `AUTH_JWT_PUBLIC_KEY`
- Keys should be in PEM format (PKCS#8 for private, SPKI for public)
- Public key can also be exposed via JWKS endpoint `/auth/.well-known/jwks.json`

**Environment configuration:**
```bash
# RS256 key pair (all environments)
AUTH_JWT_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqh...\n-----END PRIVATE KEY-----"
AUTH_JWT_PUBLIC_KEY="-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqh...\n-----END PUBLIC KEY-----"
```

**Configuration pattern (matching existing config.go):**
```go
type AuthConfig struct {
    JWTPrivateKey string // PEM-encoded private key
    JWTPublicKey  string // PEM-encoded public key
    JWTAccessTTL time.Duration // 15 minutes
    JWTRefreshTTL time.Duration // 7 days
}
```

---

## 2. Password Hashing

### 2.1 Algorithm Comparison

| Algorithm | Memory Hardness | GPU Resistance | Recommendation |
|-----------|----------------|---------------|----------------|
| **Argon2id** | 64MB+ configurable | Excellent | **Recommended — modern standard** |
| bcrypt | ~4KB fixed | Limited | Legacy systems, simple deployments |
| scrypt | Configurable | Good | Alternative to Argon2id |
| PBKDF2 | Low | Poor | Legacy compatibility only |

**Recommendation:** Use **Argon2id** — winner of the Password Hashing Competition (2015), recommended by OWASP, resistance to both side-channel and GPU-based attacks.

### 2.2 Go Library Selection

| Library | Maintenance | API Style | Recommendation |
|---------|-------------|-----------|----------------|
| **golang.org/x/crypto/argon2** | Official Go team, stdlib | Low-level | **Required — standard library** |
| alexedwards/argon2id | Active | bcrypt-like | Alternative if bcrypt-like API needed |
| andskur/argon2-hashing | Active | bcrypt-like | Alternative |

**Recommendation:** Use **`golang.org/x/crypto/argon2`** — it's the standard library, maintained by the Go team, no external dependencies.

**Why not alexedwards/argon2id:**
- Adds external dependency for convenience
- Standard library provides all needed functionality
- Consistency with Go ecosystem (no third-party deps for crypto)

### 2.3 Argon2id Parameters

| Parameter | OWASP Minimum | Recommended | Notes |
|-----------|---------------|-------------|-------|
| Memory | 19 MB (19456 KB) | 64 MB (65536 KB) | Higher = more GPU resistance |
| Iterations | 2 | 3 | Time cost |
| Parallelism | 1 | 4 | Match CPU cores |
| Salt length | 16 bytes | 16 bytes | Included in hash output |
| Key length | 32 bytes | 32 bytes | Output hash length |

**Recommendation parameters for production:**
```go
import "golang.org/x/crypto/argon2"

// Parameters for production
const (
    Argon2Memory     = 64 * 1024 // 64 MB
    Argon2Iterations = 3
    Argon2Parallelism = 4
    Argon2SaltLength = 16
    Argon2KeyLength  = 32
)

// HashPassword hashes a password using Argon2id
func HashPassword(password string, salt []byte) ([]byte, error) {
    return argon2.IDKey(
        []byte(password),
        salt,
        Argon2Iterations,
        Argon2Memory,
        Argon2Parallelism,
        Argon2KeyLength,
    )
}

// VerifyPassword compares a password with a hash using constant-time comparison
func VerifyPassword(password, hash []byte, salt []byte) bool {
    expected := argon2.IDKey(password, salt, Argon2Iterations, Argon2Memory, Argon2Parallelism, Argon2KeyLength)
    return subtle.ConstantTimeCompare(hash, expected) == 1
}
```

**Note:** Parameters should be tuned to target ~500ms hashing time on production hardware. Start with these values and adjust based on benchmarks.

---

## 3. Magic Link Token Approach

### 3.1 Overview

Instead of OAuth providers or complex email services, this system uses **magic link tokens** for authentication:
- Tokens are generated, stored in the database, and sent to the user's email
- If SMTP is configured, emails are sent automatically
- If SMTP is not configured, tokens are logged to the console (dev mode) or returned in the API response

This approach works **completely offline** with no external dependencies for core authentication.

### 3.2 Token Flow

**Registration / Login:**
1. User submits email address
2. System generates a cryptographically secure token (32 bytes, URL-safe)
3. Token is stored in database with 15-minute expiry
4. If SMTP configured: email sent with magic link
5. If SMTP not configured: token logged to console / returned in API response
6. User clicks link / submits token
7. Token validated, user authenticated, tokens issued

**Password Reset:**
1. User submits email address
2. System generates reset token (same pattern)
3. Token sent via email or shown in console/dev response
4. User clicks link with token
5. Token validated, user can set new password
6. Old refresh tokens are revoked (security measure)

### 3.3 Token Storage Schema

```sql
CREATE TABLE auth_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,  -- SHA256 of token
    token_type VARCHAR(20) NOT NULL,   -- 'magic_link', 'password_reset', 'email_verify'
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,           -- NULL until consumed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_tokens_hash ON auth_tokens(token_hash);
CREATE INDEX idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX idx_auth_tokens_expires ON auth_tokens(expires_at);
```

### 3.4 Implementation Pattern

```go
// GenerateMagicLinkToken creates a magic link token and returns the token + URL
func (s *AuthService) GenerateMagicLinkToken(ctx context.Context, email string) (string, string, error) {
    // Generate token
    rawToken := make([]byte, 32)
    if _, err := rand.Read(rawToken); err != nil {
        return "", "", fmt.Errorf("generate token: %w", err)
    }
    token := base64.URLEncoding.EncodeToString(rawToken)
    tokenHash := sha256.Sum256(rawToken)

    // Store token
    expiresAt := time.Now().Add(15 * time.Minute)
    _, err := s.db.Exec(ctx, `
        INSERT INTO auth_tokens (user_id, token_hash, token_type, expires_at)
        VALUES ($1, $2, 'magic_link', $3)
    `, userID, hex.EncodeToString(tokenHash[:]), expiresAt)
    if err != nil {
        return "", "", fmt.Errorf("store token: %w", err)
    }

    // Build magic link URL
    magicURL := fmt.Sprintf("%s/auth/verify?token=%s", s.config.BaseURL, token)

    // Send via email or log
    if s.config.SMTPEnabled {
        go s.sendEmail(email, "Your Magic Link", magicURL)
    } else {
        // Dev mode: log to console
        log.Printf("[DEV] Magic link for %s: %s", email, magicURL)
    }

    return token, magicURL, nil
}
```

### 3.5 SMTP Integration (Optional)

SMTP is optional — the system works without it. When configured, use the standard library `net/smtp`:

```go
// SMTPConfig is optional and only needed if email delivery is required
type SMTPConfig struct {
    Enabled  bool
    Host     string
    Port     int
    Username string
    Password string
    From     string
}

func (s *AuthService) sendEmail(to, subject, body string) error {
    if !s.smtp.Enabled {
        return nil
    }

    addr := fmt.Sprintf("%s:%d", s.smtp.Host, s.smtp.Port)
    auth := smtp.PlainAuth("", s.smtp.Username, s.smtp.Password, s.smtp.Host)

    msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body)
    return smtp.SendMail(addr, auth, s.smtp.From, []string{to}, []byte(msg))
}
```

### 3.6 Why This Approach

| Aspect | Magic Link Tokens | OAuth Providers |
|--------|------------------|-----------------|
| External dependencies | None (SMTP optional) | Google, GitHub APIs required |
| Account linking | Email is the identity | Provider-specific accounts |
| Offline deployment | Works fully offline | Requires internet for OAuth |
| User friction | Click link, done | Redirect, approve, done |
| Setup complexity | Zero (works without SMTP) | OAuth app registration |
| Security | Token-based, time-limited | Provider-secured |

---

## 4. CSRF Protection

### 4.1 SvelteKit CSRF Handling

SvelteKit has built-in CSRF protection enabled by default. The key decisions are:

| Setting | When to Use | Recommendation |
|---------|-------------|----------------|
| **checkOrigin: true** | All deployments (default) | Keep enabled |
| **trustedOrigins** | Cross-origin form submissions | Add for local dev + production domains |

**Recommendation:** Keep SvelteKit's built-in CSRF protection enabled. Add trusted origins for development:

```ts
// svelte.config.ts
export default {
    kit: {
        csrf: {
            checkOrigin: true,
            trustedOrigins: [
                'http://localhost:5173',  // Local dev
                'https://app.example.com' // Production
            ]
        }
    }
}
```

### 4.2 SameSite Cookie Configuration

| Value | Behavior | Use Case |
|-------|----------|----------|
| **SameSite=Lax** | Sent on same-site + top-level GET navigations | **Recommended for auth cookies** |
| SameSite=Strict | Sent on same-site only | High-security, breaks legitimate cross-site navigation |
| SameSite=None | Sent in all contexts | Requires Secure (HTTPS) |

**Recommendation:** Use `SameSite=Lax` for refresh tokens. This provides CSRF protection while allowing legitimate scenarios like clicking links from email.

### 4.3 Magic Link Security

**Token security requirements:**
- Minimum 32 bytes of cryptographically secure random data
- URL-safe encoding (base64)
- Single-use: consumed on first successful verification
- Time-limited: 15-minute expiry
- Stored as SHA256 hash in database (not plaintext)

**Rate limiting magic link requests:**
- 3 requests per email address per 15 minutes
- 5 requests per IP address per 15 minutes
- Prevents token enumeration and spam

---

## 5. Rate Limiting

### 5.1 Algorithm Comparison

| Algorithm | Accuracy | Memory | Burst Handling | Recommendation |
|-----------|----------|--------|----------------|----------------|
| Fixed Window | Approximate | Low | 2x burst at boundaries | Simple cases only |
| **Sliding Window Counter** | Near-exact | Low | Smoothed | **Recommended for general use** |
| Sliding Window Log | Exact | O(n) | No burst | High-value APIs |
| Token Bucket | Exact | Low | Controlled bursts | API with bursty traffic |

**Recommendation:** Use **sliding window counter** because:
- Near-exact accuracy (blends two fixed windows)
- Low memory (2 keys per client)
- Smoothed boundaries (no boundary burst)
- Best balance for most API rate limiting

### 5.2 Valkey Implementation

**Note:** Valkey is a Redis fork with Redis-compatible protocol and commands. All rate limiting patterns use standard Redis commands via the valkey-cli or any Redis/Valkey client.

**Recommendation:** Implement rate limiting using Valkey with Lua scripts for atomicity.

```lua
-- Sliding window counter Lua script
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])  -- Window in seconds
local limit = tonumber(ARGV[3])

-- Get current and previous windows
local current = now - (now % window)
local previous = current - window

-- Get counts
local current_count = tonumber(redis.call('GET', key .. ':' .. current) or '0')
local previous_count = tonumber(redis.call('GET', key .. ':' .. previous) or '0')

-- Calculate weighted count
local elapsed = now - previous
local weight = 1 - (elapsed / window)
local weighted_count = previous_count * weight + current_count

if weighted_count < limit then
    redis.call('INCR', key .. ':' .. current)
    redis.call('EXPIRE', key .. ':' .. current, window * 2)
    return {1, limit - weighted_count - 1}  -- allowed, remaining
else
    return {0, 0}  -- denied, remaining
end
```

### 5.3 Rate Limit Tiers

| Endpoint | Limit | Window | Reason |
|----------|-------|--------|--------|
| `/auth/login` | 5 attempts | 15 minutes | Brute force protection |
| `/auth/register` | 3 attempts | 15 minutes | Registration spam |
| `/auth/password-reset` | 3 attempts | 15 minutes | Token enumeration |
| `/auth/refresh` | 10 attempts | 15 minutes | Token theft detection |
| General API | 100 requests | 1 minute | Fair usage |

**Key approach:** Implement **per-IP** + **per-Email** rate limiting for auth endpoints.

---

## 6. Session Management

### 6.1 Stateless vs Stateful vs Hybrid

| Approach | Token Storage | Revocation | Scalability | Recommendation |
|----------|--------------|------------|-------------|----------------|
| **Stateless (JWT-only)** | None (token is session) | Valkey blacklist | Excellent | Simple, fast verification |
| Stateful (server-side) | Database/Valkey | Immediate | Requires session store | Maximum control |
| **Hybrid** | JWT + refresh token | Both mechanisms | Balanced | **Recommended** |

**Recommendation:** Use **hybrid approach** — stateless access tokens with stateful refresh tokens.

- **Access token**: JWT with claims, no server storage needed
- **Refresh token**: Stored in database, rotated on use, revocable

### 6.2 Session Storage in Valkey

**Note:** Valkey uses Redis-compatible commands. All patterns described use standard Redis commands (SET, GET, DEL, KEYS, etc.) via the valkey-cli or any Redis/Valkey client library.

**Recommendation:** Store refresh tokens as SHA256 hashes (not the raw token).

```go
// Store refresh token
tokenHash := sha256.Sum256([]byte(refreshToken))
key := fmt.Sprintf("session:refresh:%x", tokenHash)
valkey.Set(ctx, key, userID, 7*24*time.Hour) // 7 days TTL

// Revoke all sessions for user
pattern := "session:refresh:*"
keys, _ := valkey.Keys(ctx, pattern).Result()
for _, k := range keys {
    valkey.Del(ctx, k)
}
```

**Why hash the token:**
- Even if Valkey is compromised, tokens cannot be used directly
- One-way hash, no way to derive original token
- Consistent length for key operations

### 6.3 Session Data Schema

**Refresh token table (PostgreSQL):**
```sql
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA256 of token
    device_id  VARCHAR(255),                  -- Optional device fingerprint
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
```

---

## 7. Auth Event Schema

### 7.1 NATS Subject Pattern

Following the existing `shared/messaging` patterns, auth events use:

```
ace.auth.{event_type}.event
```

### 7.2 Event Types and Schemas

```go
// Auth event types
const (
    SubjectAuthLogin          = "ace.auth.login.event"
    SubjectAuthLogout         = "ace.auth.logout.event"
    SubjectAuthPasswordChange = "ace.auth.password_change.event"
    SubjectAuthTokenRevoke    = "ace.auth.token_revoke.event"
    SubjectAuthRoleChange     = "ace.auth.role_change.event"
    SubjectAuthAccountDelete  = "ace.auth.account_delete.event"
)

// AuthEvent represents the base auth event structure
type AuthEvent struct {
    EventType   string    `json:"event_type"`   // login, logout, password_change, etc.
    UserID      string    `json:"user_id"`
    SessionID   string    `json:"session_id,omitempty"`
    IPAddress   string    `json:"ip_address,omitempty"`
    UserAgent   string    `json:"user_agent,omitempty"`
    Timestamp   time.Time `json:"timestamp"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}
```

### 7.3 Example Event Payloads

**Login event:**
```json
{
    "event_type": "login",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "session_id": "session-123",
    "ip_address": "192.168.1.1",
    "user_agent": "Mozilla/5.0...",
    "timestamp": "2026-04-09T10:30:00Z",
    "metadata": {
        "method": "magic_link"
    }
}
```

**Password change event:**
```json
{
    "event_type": "password_change",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "session_id": "session-123",
    "timestamp": "2026-04-09T10:30:00Z",
    "metadata": {
        "sessions_revoked": "5"
    }
}
```

---

## 8. Deployment Mode Auto-Provisioning

### Single-User Mode Initialization

In single-user mode (Docker Compose hobbyist deployment), the system must handle first-time user creation.

**Recommendation:** Use a **seed script on first startup** that checks if users exist.

```go
// cmd/seed/main.go
import (
    "crypto/rand"
    "encoding/hex"

    "golang.org/x/crypto/argon2"
    "github.com/jackc/pgx/v5/pgxpool"
)

func runSeed(ctx context.Context, db *pgxpool.Pool) error {
    // Check if any users exist
    var count int
    err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
    if err != nil {
        return fmt.Errorf("check users: %w", err)
    }

    if count > 0 {
        log.Println("Users exist, skipping seed")
        return nil
    }

    // Create first admin user
    adminEmail := os.Getenv("SEED_ADMIN_EMAIL")
    adminPassword := os.Getenv("SEED_ADMIN_PASSWORD")
    if adminEmail == "" || adminPassword == "" {
        return fmt.Errorf("SEED_ADMIN_EMAIL and SEED_ADMIN_PASSWORD required for first user")
    }

    // Hash password using Argon2id (standard library)
    salt := make([]byte, 16)
    if _, err := rand.Read(salt); err != nil {
        return fmt.Errorf("generate salt: %w", err)
    }
    hash := argon2.IDKey([]byte(adminPassword), salt, 3, 64*1024, 4, 32)

    _, err = db.Exec(ctx, `
        INSERT INTO users (email, password_hash, role, email_verified, status)
        VALUES ($1, $2, 'admin', true, 'active')
    `, adminEmail, hash)
    if err != nil {
        return fmt.Errorf("create admin: %w", err)
    }

    log.Printf("Created admin user: %s", adminEmail)
    return nil
}
```

**Key considerations:**
- Run seed script **after** migrations on container startup
- Use environment variables for credentials (`SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD`)
- In multi-user mode, disable auto-provisioning via config
- Log the creation event via NATS for audit trail

**Configuration:**
```go
type AuthConfig struct {
    // ...
    DeploymentMode       string // "single" or "multi"
    SeedAdminEmail      string
    SeedAdminPassword   string
}
```

---

## 9. Non-Scope Clarifications

### MFA (Multi-Factor Authentication)

**MFA is explicitly out of scope for this unit** and deferred to future work. This includes:
- TOTP (Authenticator apps)
- SMS-based 2FA
- Hardware keys (WebAuthn/FIDO2)
- Backup codes

The authentication system is designed to be MFA-compatible at a later stage without requiring architectural changes.

### OAuth / SSO Providers

**OAuth providers (Google, GitHub, etc.) are explicitly out of scope.** This unit uses magic link tokens only.

Rationale:
- No external API dependencies for authentication
- Works completely offline (no Google/GitHub API calls)
- Simpler deployment (no OAuth app registration)
- Magic links work for any email provider

---

## 10. Deployment Simplicity

This authentication system is designed for **easy local deployment** with zero external service dependencies for core functionality.

### What Works Without Internet

| Feature | Offline Support | Notes |
|---------|---------------|-------|
| User registration | ✅ | Magic link token generated |
| User login | ✅ | Magic link token generated |
| Password reset | ✅ | Token generated |
| JWT authentication | ✅ | No external calls |
| Token refresh | ✅ | No external calls |
| Rate limiting | ✅ | Uses local Valkey |
| Session revocation | ✅ | Uses local Valkey |

### When SMTP is NOT Configured

In development or offline mode, magic links are logged to the console:

```
[DEV] Magic link for user@example.com: https://localhost:5173/auth/verify?token=abc123...
[DEV] Or use token directly: abc123...
```

Users can:
1. Check server logs for the magic link URL
2. Copy the token and paste it into the UI
3. Use the development "copy token" button in the frontend

### Required External Services

| Service | Required | Purpose | Can Be Local |
|---------|----------|---------|--------------|
| PostgreSQL | ✅ Yes | User data, tokens | Docker Compose |
| Valkey | ✅ Yes | Rate limiting, blacklist | Docker Compose |
| NATS | ✅ Yes | Auth events | Docker Compose |
| SMTP Server | ❌ No | Email delivery | Optional / Mailhog |
| OAuth Providers | ❌ No | Authentication | Not used |

### Docker Compose Example

The system runs fully with Docker Compose using local services:

```yaml
services:
  api:
    environment:
      - DATABASE_URL=postgres://user:pass@postgres:5432/ace
      - VALKEY_URL=valkey:6379
      - NATS_URL=nats://nats:4222
      # SMTP optional - without it, magic links go to logs
      # - SMTP_HOST=mailhog
      # - SMTP_PORT=1025
```

This means **single-user hobbyist deployments work out of the box** without any external accounts or API keys.

---

## 10. Summary Table of Recommendations

| Category | Decision | Recommendation | Library/Pattern |
|----------|----------|----------------|-----------------|
| **JWT Access Token** | Duration | 15 minutes | - |
| **JWT Algorithm** | Signing | RS256 everywhere | golang-jwt/jwt |
| **JWT Key Storage** | Environment | `AUTH_JWT_PRIVATE_KEY`, `AUTH_JWT_PUBLIC_KEY` | PEM format |
| **Refresh Token** | Rotation | Rotate on every use | PostgreSQL + Valkey |
| **Token Storage** | Client-side | httpOnly cookies | SameSite=Lax, Secure |
| **Token Refresh** | Endpoint | Dedicated `/auth/refresh` + cookie-based | Both approaches |
| **Token Revocation** | Strategy | Valkey blacklist | TTL = remaining token life |
| **Password Hashing** | Algorithm | Argon2id | golang.org/x/crypto/argon2 |
| **Hash Parameters** | Config | 64MB, 3 iterations | runtime.NumCPU() parallelism |
| **Auth Method** | Approach | Magic link tokens | No external dependencies |
| **Magic Link** | Storage | SHA256 hash in DB | 15-minute expiry |
| **CSRF Protection** | Strategy | SvelteKit built-in + SameSite | checkOrigin: true |
| **Rate Limiting** | Algorithm | Sliding window counter | Valkey Lua scripts |
| **Session Storage** | Approach | Hybrid (JWT + DB refresh) | PostgreSQL + Valkey |
| **Auto-Provisioning** | Single-user | Seed script on first startup | Checks if users exist |
| **MFA** | Scope | Explicitly out of scope | Deferred to future work |
| **OAuth/SSO** | Scope | Explicitly out of scope | Magic links only |

---

## 11. Package Recommendations

### Go Dependencies

```go
// JWT handling
github.com/golang-jwt/jwt/v5

// Password hashing (standard library)
golang.org/x/crypto/argon2  // Standard library, no external deps

// Validation
github.com/go-playground/validator/v10

// UUID generation
github.com/google/uuid

// SMTP (optional - standard library used if configured)
net/smtp  // Standard library, optional
```

**No external dependencies required for core authentication.** The system uses:
- Standard library `net/smtp` for optional email
- Standard library `golang.org/x/crypto/argon2` for password hashing

### SvelteKit Dependencies

```bash
# Form handling (already in SvelteKit)
# No additional packages needed for CSRF

# For JWT handling in frontend (if needed)
jose  # For JWT parsing/validation in edge functions
```

---

## 12. References

- [RFC 9106: Argon2id](https://www.rfc-editor.org/rfc/rfc9106)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [JWT Best Practices (IETF)](https://www.rfc-editor.org/rfc/rfc8725)
- [OAuth 2.0 Security Best Current Practice](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics)
- [Redis Rate Limiting Guide](https://redis.io/learn/howtos/ratelimiting)
- [SvelteKit Form Actions CSRF](https://kit.svelte.dev/docs/form-actions#form-without-an-action)
