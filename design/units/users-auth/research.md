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

### 1.6 RS256 vs HS256 for JWT Signing

| Algorithm | Key Management | Use Case | Recommendation |
|-----------|---------------|----------|----------------|
| **RS256 (RSA)** | Private key signs, public key verifies | Multiple services, microservices | **Recommended for production** |
| HS256 (HMAC) | Shared secret, same key signs/verifies | Single service, simple deployments | Acceptable for local development only |

**Recommendation:** 
- **Production**: Use **RS256** because:
  - The API service signs tokens; other services (cognitive engine, future microservices) can verify with the public key
  - Compromised public key cannot forge tokens (only verify)
  - Industry standard for OAuth/OIDC
- **Development**: HS256 is acceptable for local development where simplicity is preferred

**Key storage (RS256):**
- Private key: Environment variable `AUTH_JWT_PRIVATE_KEY` or secrets manager (Vault, AWS Secrets Manager)
- Public key: Environment variable `AUTH_JWT_PUBLIC_KEY` or derived from private key
- Keys should be in PEM format (PKCS#8 for private, SPKI for public)
- Public key can also be exposed via JWKS endpoint `/auth/.well-known/jwks.json`

**Example environment configuration:**
```bash
# RS256 key pair (production)
AUTH_JWT_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqh...\n-----END PRIVATE KEY-----"
AUTH_JWT_PUBLIC_KEY="-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqh...\n-----END PUBLIC KEY-----"

# HS256 secret (development only)
AUTH_JWT_SECRET="your-256-bit-secret-for-development-only"
```

**Configuration pattern (matching existing config.go):**
```go
type AuthConfig struct {
    JWTAlgorithm   string // "RS256" or "HS256"
    JWTPrivateKey  string // PEM-encoded private key (RS256) or secret (HS256)
    JWTPublicKey   string // PEM-encoded public key (RS256) or empty (HS256)
    JWTAccessTTL  time.Duration // 15 minutes
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

| Library | Stars | Maintenance | API Style | Recommendation |
|---------|-------|-------------|-----------|---------------|
| **alexedwards/argon2id** | 626 | Active (Oct 2025) | bcrypt-like | **Recommended** |
| golang.org/x/crypto/argon2 | Standard lib | Official Go team | Low-level | Use directly if needed |
| sixcolors/argon2id | 3 | Active (Mar 2026) | bcrypt-like | Newer, less battle-tested |
| andskur/argon2-hashing | 25 | Active (May 2025) | bcrypt-like | Alternative |

**Recommendation:** Use **`github.com/alexedwards/argon2id`** because:
- 626 stars, widely used (~370 public GitHub files)
- Simple bcrypt-like API: `GenerateFromPassword`, `ComparePasswordAndHash`
- Active maintenance (Oct 2025)
- Constant-time comparison built-in

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
params := &argon2id.Params{
    Memory:      64 * 1024,  // 64 MB
    Iterations:  3,
    Parallelism: uint8(runtime.NumCPU()),
    SaltLength:  16,
    KeyLength:   32,
}
```

**Note:** Parameters should be tuned to target ~500ms hashing time on production hardware. Start with these values and adjust based on benchmarks.

---

## 3. Email Delivery

### 3.1 SMTP/Email Service Comparison

| Provider | Free Tier | Cost (100k/month) | Go SDK | Recommendation |
|----------|-----------|-------------------|--------|----------------|
| **Amazon SES** | 62k/year | ~$10 | aws-sdk-go-v2 | **Best value, AWS native** |
| SendGrid | 100/day | ~$60 | sendgrid-go | Easy setup, good docs |
| Mailgun | 5k/month | ~$75 | mailgun-go | Developer-friendly, good APIs |
| Postmark | 100/month | ~$100 | postmark | Best deliverability, transactional |

**Recommendation:** Use **Amazon SES** because:
- Lowest cost at scale ($0.10 per 1000 emails)
- Native AWS integration (same VPC/network as other services)
- Reliable deliverability (80-90% inbox placement)
- Official Go SDK (aws-sdk-go-v2/services/ses)

**For development:** Use **Mailtrap** (mailtrap.io) — catches all emails, no accidental sends.

### 3.2 Email Template Approach

**Recommendation:** Store templates in the codebase (Go html/template or text/template).

```go
// templates/email.go
package templates

var EmailVerification = template.Must(template.New("email-verification").Parse(`
<!DOCTYPE html>
<html>
<body>
<p>Click the link to verify your email:</p>
<a href="{{.VerifyURL}}">{{.VerifyURL}}</a>
<p>This link expires in 24 hours.</p>
</body>
</html>
`))
```

**Why not external services:**
- Keep email logic self-contained
- No external dependency for email rendering
- Full control over content/branding
- Can switch providers without changing templates

### 3.3 Email Service Interface

Define an interface for email delivery to allow provider swapping:

```go
// EmailService defines the email delivery interface
type EmailService interface {
    SendVerificationEmail(ctx context.Context, to string, token string) error
    SendPasswordResetEmail(ctx context.Context, to string, token string) error
    SendWelcomeEmail(ctx context.Context, to string, name string) error
}

// SESEmailService implements EmailService using AWS SES
type SESEmailService struct {
    client *ses.Client
    sender string
}
```

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

### 4.3 OAuth State Parameter Best Practices

**Recommendation:** Store OAuth state in a **signed cookie** (not localStorage) with 10-minute TTL.

```go
// Generate state
state := base64.URLEncoding.EncodeToString(crypto_rand.Bytes(32))

// Store in signed cookie
cookie := &http.Cookie{
    Name:     "oauth_state",
    Value:    state,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteLaxMode,
    MaxAge:   600, // 10 minutes
    Path:     "/auth/callback",
}

// Validate on callback: compare cookie value with query parameter
```

**Why signed cookie (not just httpOnly):**
- The state needs to be readable by JavaScript to include in the callback URL
- Signed with a secret to prevent forgery
- Short TTL limits attack window

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
        "method": "password",
        "provider": "email"
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

    hash, err := argon2id.GenerateFromPassword(adminPassword, argon2id.DefaultParams)
    if err != nil {
        return fmt.Errorf("hash password: %w", err)
    }

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

---

## 10. Summary Table of Recommendations

| Category | Decision | Recommendation | Library/Pattern |
|----------|----------|----------------|-----------------|
| **JWT Access Token** | Duration | 15 minutes | - |
| **JWT Algorithm** | Signing | RS256 (production), HS256 (dev) | golang-jwt/jwt |
| **JWT Key Storage** | Environment | `AUTH_JWT_PRIVATE_KEY`, `AUTH_JWT_PUBLIC_KEY` | PEM format |
| **Refresh Token** | Rotation | Rotate on every use | PostgreSQL + Valkey |
| **Token Storage** | Client-side | httpOnly cookies | SameSite=Lax, Secure |
| **Token Refresh** | Endpoint | Dedicated `/auth/refresh` + cookie-based | Both approaches |
| **Token Revocation** | Strategy | Valkey blacklist | TTL = remaining token life |
| **Password Hashing** | Algorithm | Argon2id | alexedwards/argon2id |
| **Hash Parameters** | Config | 64MB, 3 iterations | runtime.NumCPU() parallelism |
| **Email Provider** | Service | Amazon SES | aws-sdk-go-v2 |
| **Email Templates** | Storage | Go templates in codebase | html/template |
| **CSRF Protection** | Strategy | SvelteKit built-in + SameSite | checkOrigin: true |
| **OAuth State** | Storage | Signed httpOnly cookie | 10-minute TTL |
| **Rate Limiting** | Algorithm | Sliding window counter | Valkey Lua scripts |
| **Session Storage** | Approach | Hybrid (JWT + DB refresh) | PostgreSQL + Valkey |
| **Auto-Provisioning** | Single-user | Seed script on first startup | Checks if users exist |
| **MFA** | Scope | Explicitly out of scope | Deferred to future work |

---

## 11. Package Recommendations

### Go Dependencies

```go
// JWT handling
github.com/golang-jwt/jwt/v5

// Password hashing
github.com/alexedwards/argon2id

// OAuth2 client (for Google/GitHub)
github.com/go-oauth2/oauth2/v4

// AWS SDK (for SES)
github.com/aws/aws-sdk-go-v2
github.com/aws/aws-sdk-go-v2/service/ses

// Validation
github.com/go-playground/validator/v10

// UUID generation
github.com/google/uuid

// Crypto (for token hashing)
golang.org/x/crypto/bcrypt  // backup option
```

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
