# Security Considerations

<!--
Intent: Define security requirements, threat modeling, and controls for the feature.
Scope: Authentication, authorization, data protection, input validation, and compliance.
Used by: AI agents to ensure the feature is built securely.
-->

## Security Overview

The core-infra unit implements foundational security controls for the ACE Framework MVP, including JWT-based authentication, role-based authorization, input validation, and encrypted data storage.

## Authentication

| Method | Description | Implementation |
|--------|-------------|----------------|
| JWT (JSON Web Tokens) | Stateless authentication for API access | Token issued on login, validated on each request |
| Password Hashing | Secure password storage using bcrypt | bcrypt cost factor 12 |
| WebSocket Auth | Real-time connection authentication | JWT passed as query param on upgrade |

## Authorization

| Resource | Permission | Access Control |
|----------|------------|----------------|
| Users | read (self), update (self), delete (self) | User can only modify own data |
| Agents | CRUD (owner only) | User can only access own agents |
| Sessions | read/write (owner only) | Session linked to user via owner_id |
| Thoughts | read (owner only) | Thoughts linked to sessions owned by user |
| Memories | read/write (owner only) | Memories have owner_id field |
| LLMProviders | CRUD (owner only) | Provider linked to user |
| Settings | read/write (owner for agent, admin for system) | owner_id check or admin role |
| ToolWhitelist | read/write (owner only) | Tool whitelist per agent |

## Data Protection

### Sensitive Data

| Data | Classification | Protection |
|------|---------------|------------|
| User passwords | Secret | bcrypt hashed, never stored in plain text |
| JWT tokens | Secret | Short expiry (15 min access, 7 days refresh) |
| API keys (LLM providers) | Secret | Encrypted at rest in database |
| LLM provider credentials | Secret | Encrypted storage |

### Encryption
- **At Rest**: PostgreSQL data encrypted at rest (AES-256)
- **In Transit**: TLS 1.3 for all HTTP and WebSocket connections

## Input Validation

| Input | Validation Rules | Error Response |
|-------|-----------------|----------------|
| Email | RFC 5322 format, max 255 chars | 400 Bad Request |
| Password | Min 8, max 72 chars, complexity requirements | 400 Bad Request |
| Username | Alphanumeric + underscore, 3-30 chars | 400 Bad Request |
| Agent name | Max 100 chars, no special chars | 400 Bad Request |
| API keys | Min 10 chars, no whitespace | 400 Bad Request |
| JSON payloads | Max 1MB, parseable | 400 Bad Request |

## Threat Modeling

### STRIDE Analysis

| Threat | Description | Mitigation |
|--------|-------------|------------|
| Spoofing | Impersonating users | JWT validation with signature verification |
| Tampering | Modifying data in transit | TLS 1.3, checksums on payloads |
| Repudiation | Denying actions taken | Audit logging for all operations |
| Information Disclosure | Exposing sensitive data | Encryption at rest, role-based access |
| Denial of Service | Crashing the service | Rate limiting, request size limits |
| Elevation of Privilege | Gaining admin access | Role checks, principle of least privilege |

### Attack Surfaces
- REST API endpoints (Gin handlers)
- WebSocket connections
- Database queries (SQL injection)
- Authentication endpoints
- File uploads (if any)

## Security Controls

| Control | Type | Implementation |
|---------|------|----------------|
| Rate limiting | Preventative | Token bucket algorithm, 100 req/min per user |
| Input sanitization | Preventative | Go-Playground validator library |
| SQL injection prevention | Preventative | SQLC parameterized queries |
| XSS protection | Preventative | Content-Type: application/json, CSP headers |
| CSRF protection | Preventative | SameSite cookies for JWT |
| Audit logging | Detective | Structured JSON logs with request IDs |

## Security Testing
- [x] Input validation unit tests
- [ ] Authentication flow integration tests
- [ ] Penetration testing checklist
- [ ] Vulnerability scanning (OWASP ZAP)
- [ ] JWT token validation tests
- [ ] Rate limiting tests

## Compliance
- GDPR: User data can be exported and deleted on request
- SOC2: Audit logging, encryption, access controls

## Incident Response
1. Identify compromised credentials - revoke tokens, force password reset
2. Isolate affected systems - disable affected user accounts
3. Preserve evidence - logs, database state
4. Notify affected users per GDPR requirements
5. Root cause analysis and remediation
