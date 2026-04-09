# Business Specification Document: Users-Auth Unit

## Overview

This document defines the business requirements for the **Users-Auth** unit, which establishes the complete identity and access management foundation for the ACE Framework. The unit enables user account management, authentication flows (email/password and OAuth/SSO), JWT-based session management, role-based access control (RBAC), and resource-level authorization.

The ACE Framework currently has no authentication or authorization system—all API endpoints are public with no concept of user identity, access control, or resource isolation. This unit resolves these foundational gaps, enabling multi-user SaaS deployments, cost attribution, resource sharing, billing, and audit trails.

---

## 1. Problem Statement

The ACE Framework lacks the ability to:
1. **Identify users** — No mechanism to create, verify, or manage user accounts
2. **Authenticate requests** — Any client can call any endpoint without credentials
3. **Authorize access** — No role-based or resource-level permissions exist
4. **Emit auth events** — Auth operations produce no observable events for auditing
5. **Scale beyond single-user** — No multi-user deployment capability

Without this unit, the system cannot support multi-user SaaS, protect agent data, or implement billing or auditing.

---

## 2. User Personas

| Persona | Description | Primary Goals |
|---------|-------------|---------------|
| **Hobbyist User** | Single-user deployment, technical user | Simple signup/login, email verification, password management |
| **SaaS User** | Multi-user organization member | SSO login (Google/GitHub), team collaboration, agent sharing |
| **Team Admin** | Manages team members and shared resources | User role management, resource sharing permissions, team oversight |
| **Platform Admin** | Platform operator managing the entire system | System-wide user management, role assignment, billing oversight |
| **API Developer** | Integrates with ACE API programmatically | Clear auth patterns, token management, API key support for services |
| **Frontend Developer** | Builds UI for authentication flows | OAuth redirect handling, token storage, secure session management |

---

## 3. User Stories

### Authentication & Registration

**US-1: User Registration with Email/Password**
> As a **new user**, I want to register with my email and password, so that I can create an account to access the platform.

**Acceptance Criteria:**
- [ ] User can submit registration form with email and password
- [ ] Password must meet minimum complexity requirements (configurable, default: 8+ chars, mixed case, number)
- [ ] System sends verification email to the provided address
- [ ] Email verification token expires after configurable duration (default: 24 hours)
- [ ] User account is in "pending_verification" status until email is verified
- [ ] Duplicate email registration returns appropriate error
- [ ] Password is hashed using secure algorithm (Argon2id preferred)

**US-2: User Login with Email/Password**
> As a **registered user**, I want to log in with my email and password, so that I can access my account securely.

**Acceptance Criteria:**
- [ ] User can submit login form with email and password
- [ ] Successful login returns JWT access token and refresh token
- [ ] Failed login after 5 attempts triggers temporary lockout (15 minutes)
- [ ] Login emits `ace.auth.login.event` via NATS
- [ ] Failed login attempt emits `ace.auth.failed_login.event` via NATS
- [ ] Login from new device sends notification (if email verified)
- [ ] Rate limiting prevents brute-force attacks (10 attempts per minute per IP)

**US-3: SSO Login via Google**
> As a **SaaS user**, I want to log in with my Google account, so that I can access the platform without creating a new password.

**Acceptance Criteria:**
- [ ] User can initiate login via Google OAuth flow
- [ ] OAuth state parameter is validated to prevent CSRF attacks
- [ ] New users are automatically registered upon first Google login
- [ ] Existing email/password users can link Google OAuth to their account
- [ ] Google OAuth tokens are stored for potential API access
- [ ] Login emits `ace.auth.login.event` via NATS

**US-4: SSO Login via GitHub**
> As a **developer user**, I want to log in with my GitHub account, so that I can access the platform using my existing GitHub identity.

**Acceptance Criteria:**
- [ ] User can initiate login via GitHub OAuth flow
- [ ] OAuth state parameter is validated to prevent CSRF attacks
- [ ] New users are automatically registered upon first GitHub login
- [ ] Existing email/password users can link GitHub OAuth to their account
- [ ] GitHub OAuth tokens are stored for potential API access
- [ ] Login emits `ace.auth.login.event` via NATS

**US-5: Link Multiple OAuth Providers**
> As a **user**, I want to link both Google and GitHub to my account, so that I can log in with either provider interchangeably.

**Acceptance Criteria:**
- [ ] User can link additional OAuth provider to existing authenticated session
- [ ] Linking requires re-authentication with the new provider
- [ ] Both providers can be used to log into the same account
- [ ] User can unlink OAuth providers (must retain at least one auth method)

**US-6: User Logout**
> As a **logged-in user**, I want to log out, so that I can secure my account on shared devices.

**Acceptance Criteria:**
- [ ] Logout invalidates the current session's tokens immediately
- [ ] Refresh tokens are added to revocation list in Valkey
- [ ] Logout emits `ace.auth.logout.event` via NATS
- [ ] User is redirected to login page after logout

**US-7: Password Reset**
> As a **user who forgot my password**, I want to reset my password via email, so that I can regain access to my account.

**Acceptance Criteria:**
- [ ] User can request password reset by providing their email
- [ ] System sends email with secure, single-use reset token
- [ ] Reset token expires after configurable duration (default: 1 hour)
- [ ] User can set new password using valid reset token
- [ ] Successful reset invalidates all existing sessions and emits `ace.auth.password_change.event`
- [ ] Rate limiting prevents reset request spam (3 requests per hour per email)

**US-8: Email Verification**
> As a **new user**, I want to verify my email address, so that I can confirm my identity and unlock full account features.

**Acceptance Criteria:**
- [ ] User receives verification email with unique token upon registration
- [ ] Clicking verification link activates the account
- [ ] Verified email allows password reset and SSO linking
- [ ] Unverified accounts have limited functionality until verification

### Authorization & Access Control

**US-9: Role-Based Access Control**
> As a **platform admin**, I want to assign system roles to users, so that I can control what each user can do across the platform.

**Acceptance Criteria:**
- [ ] System roles: `admin` (full access), `user` (manage own resources), `viewer` (read-only shared resources)
- [ ] Admin can assign roles to users
- [ ] Users can view their own role
- [ ] Role changes emit `ace.auth.role_change.event` via NATS
- [ ] Permission denied returns HTTP 403

**US-10: Resource-Level Authorization**
> As a **user**, I want to control who can access my agents, so that I can collaborate while protecting sensitive work.

**Acceptance Criteria:**
- [ ] Users can share agents, tools, skills, and configurations with other users
- [ ] Permission levels: `view` (read-only), `use` (execute), `admin` (full control)
- [ ] Resource owner retains full permissions
- [ ] Permission checks happen on every protected endpoint
- [ ] Unauthorized access to resources returns HTTP 403

**US-11: View Shared Resources**
> As a **viewer**, I want to view agents shared with me, so that I can review work without modifying it.

**Acceptance Criteria:**
- [ ] Users with `view` permission can list and read shared resources
- [ ] Users with `view` permission cannot create, modify, or delete
- [ ] Viewer role can access shared resources but not system administration

**US-12: Unauthenticated Access Rejection**
> As a **security-conscious operator**, I want protected endpoints to reject unauthenticated requests, so that unauthorized users cannot access sensitive data.

**Acceptance Criteria:**
- [ ] Protected endpoints return HTTP 401 for missing/invalid JWT
- [ ] Protected endpoints return HTTP 403 for valid JWT but insufficient permissions
- [ ] `response.Unauthorized()` and `response.Forbidden()` helpers are used consistently
- [ ] OpenAPI `BearerAuth` security scheme is enforced

### Deployment Modes

**US-13: Single-User Mode**
> As a **hobbyist user**, I want the system to work out-of-the-box for a single user, so that I don't need complex setup.

**Acceptance Criteria:**
- [ ] First registered user automatically becomes admin
- [ ] Registration can be disabled via configuration
- [ ] No code branching between single-user and multi-user modes
- [ ] All features work identically in single-user mode

**US-14: Multi-User Mode**
> As a **SaaS operator**, I want the system to support multiple independent users, so that I can offer the platform as a service.

**Acceptance Criteria:**
- [ ] Open registration allowed in multi-user mode
- [ ] Users are isolated from each other's resources by default
- [ ] Resource sharing is opt-in and explicit
- [ ] System scales to thousands of concurrent users

### Account Management

**US-15: Change Password**
> As a **logged-in user**, I want to change my password, so that I can maintain account security.

**Acceptance Criteria:**
- [ ] User can change password by providing current and new password
- [ ] New password must meet complexity requirements
- [ ] Password change invalidates all existing sessions
- [ ] Emits `ace.auth.password_change.event` via NATS

**US-16: Account Deletion**
> As a **user**, I want to delete my account, so that I can remove my data from the platform.

**Acceptance Criteria:**
- [ ] User can request account deletion from settings
- [ ] Account is soft-deleted (marked with `deleted_at` timestamp)
- [ ] All active sessions are invalidated immediately
- [ ] Emits `ace.auth.account_deleted.event` via NATS
- [ ] Scheduled background job purges data after retention period

**US-16a: Account Suspension**
> As a **platform admin**, I want to suspend user accounts, so that I can temporarily revoke access for security violations or policy breaches.

**Acceptance Criteria:**
- [ ] Admin can suspend any user account via `/admin/users/:id/suspend`
- [ ] Suspended accounts cannot log in or access protected endpoints
- [ ] Suspended status returns HTTP 403 with appropriate error message
- [ ] Account suspension emits `ace.auth.account_suspended.event` via NATS
- [ ] Admin can reverse suspension to restore account access
- [ ] User's existing tokens are invalidated upon suspension

---

## 4. Business Rules

The following rules are **invariants** that must always hold true in the system:

### Authentication Rules

| ID | Rule |
|----|------|
| **BR-1** | All passwords MUST be hashed before storage using a secure algorithm (Argon2id preferred) |
| **BR-2** | JWT access tokens MUST be validated on every protected request |
| **BR-3** | Revoked tokens MUST be rejected immediately upon validation |
| **BR-4** | Rate limiting MUST be enforced on all authentication endpoints |
| **BR-5** | Email verification tokens MUST be single-use and time-limited |
| **BR-6** | Password reset tokens MUST be single-use and time-limited |
| **BR-7** | OAuth state parameters MUST be validated to prevent CSRF attacks |
| **BR-8** | Failed login attempts MUST trigger temporary account lockout after threshold |

### Authorization Rules

| ID | Rule |
|----|------|
| **BR-9** | Every protected resource MUST verify the user's identity before granting access |
| **BR-10** | Every protected resource MUST verify the user's permissions for the requested action |
| **BR-11** | Resource owners MUST always have full permissions to their own resources |
| **BR-12** | Users MUST have at least one authentication method (email/password or OAuth) |
| **BR-13** | Admin role MUST be required for system-wide operations (user management, role assignment) |

### Data Integrity Rules

| ID | Rule |
|----|------|
| **BR-14** | Email addresses MUST be unique across all users |
| **BR-15** | User accounts MUST be soft-deleted (never hard-deleted) to preserve audit trail |
| **BR-16** | All auth events MUST be emitted via NATS for downstream consumption |
| **BR-17** | Every resource MUST have an `owner_id` referencing the creating user |

### Deployment Rules

| ID | Rule |
|----|------|
| **BR-18** | Single-user mode MUST automatically assign admin role to first user |
| **BR-19** | Deployment mode MUST be determined by configuration only, not code |
| **BR-20** | The same database schema MUST work for both deployment modes |

---

## 5. Functional Requirements

Each functional requirement maps to a specific user story and problem_space FR.

| ID | Requirement | User Story | Problem Space |
|----|-------------|------------|---------------|
| **FR-1** | The system SHALL allow user registration via email and password | US-1 | FR-1 |
| **FR-2** | The system SHALL allow user login via email and password | US-2 | FR-2 |
| **FR-3** | The system SHALL support OAuth/SSO login via Google and GitHub | US-3, US-4 | FR-3 |
| **FR-4** | The system SHALL issue JWT access tokens for authenticated sessions with configurable lifetime (default 5-15 minutes) | US-2 | FR-4 |
| **FR-5** | The system SHALL enforce RBAC with system-level roles (admin, user, viewer) | US-9 | FR-5 |
| **FR-6** | The system SHALL allow resource-level authorization with permission levels | US-10 | FR-5, FR-6 |
| **FR-7** | The system SHALL provide password reset via email with secure, time-limited tokens | US-7 | FR-7 |
| **FR-8** | The system SHALL verify email addresses during registration | US-1, US-8 | FR-8 |
| **FR-9** | The system SHALL support both multi-user SaaS and single-user deployment modes | US-13, US-14 | FR-9 |
| **FR-10** | The system SHALL emit auth events via NATS with subject pattern `ace.auth.<event_type>.event` | US-2, US-6, US-9, US-15, US-16 | FR-10 |
| **FR-11** | The system SHALL return 401 for unauthenticated requests and 403 for unauthorized requests | US-12 | FR-11 |
| **FR-12** | The system SHALL rate-limit authentication endpoints | US-2, US-7 | FR-12 |
| **FR-13** | The system SHALL allow linking multiple OAuth providers to a single account | US-5 | FR-13 |
| **FR-14** | The system SHALL allow unlinking OAuth providers from an account | US-5 | FR-14 |
| **FR-15** | The system SHALL invalidate all sessions upon password change | US-15 | FR-10 |
| **FR-16** | The system SHALL support soft deletion of user accounts | US-16 | - |
| **FR-17** | The system SHALL cache permission lookups in Valkey for performance | US-10 | - |

### API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/auth/register` | POST | Public | Register new user with email/password |
| `/auth/login` | POST | Public | Login with email/password |
| `/auth/logout` | POST | Required | Logout and invalidate tokens |
| `/auth/refresh` | POST | Public | Refresh access token |
| `/auth/password/reset/request` | POST | Public | Request password reset email |
| `/auth/password/reset/confirm` | POST | Public | Reset password with token |
| `/auth/email/verify` | POST | Public | Verify email address |
| `/auth/oauth/google` | GET | Public | Initiate Google OAuth flow |
| `/auth/oauth/google/callback` | GET | Public | Google OAuth callback |
| `/auth/oauth/github` | GET | Public | Initiate GitHub OAuth flow |
| `/auth/oauth/github/callback` | GET | Public | GitHub OAuth callback |
| `/auth/oauth/link` | POST | Required | Link OAuth provider to account |
| `/auth/oauth/unlink` | POST | Required | Unlink OAuth provider from account |
| `/auth/password/change` | POST | Required | Change password (logged in) |
| `/auth/me` | GET | Required | Get current user profile |
| `/auth/me/sessions` | GET | Required | List active sessions |
| `/auth/me/sessions/:id` | DELETE | Required | Revoke specific session |
| `/admin/users` | GET | Admin | List all users |
| `/admin/users/:id` | GET | Admin | Get user details |
| `/admin/users/:id/role` | PUT | Admin | Update user role |
| `/admin/users/:id/suspend` | POST | Admin | Suspend user account |
| `/admin/users/:id/delete` | POST | Admin | Soft-delete user account |

---

## 6. Non-Functional Requirements

### Performance

| Requirement | Target | Measurement |
|-------------|--------|-------------|
| **NFR-1** | Login response time | < 200ms (excluding network latency) |
| **NFR-2** | Token validation latency | < 5ms per request |
| **NFR-3** | Auth event publishing | < 50ms (async, non-blocking) |
| **NFR-4** | Permission cache hit ratio | > 95% for active users |
| **NFR-5** | Concurrent authenticated users | Support 10,000+ concurrent sessions |

### Security

| Requirement | Target | Measurement |
|-------------|--------|-------------|
| **NFR-6** | Password hashing | Argon2id with memory ≥ 64MB, iterations ≥ 3 |
| **NFR-7** | Access token lifetime | 5-15 minutes (configurable) |
| **NFR-8** | Refresh token rotation | Required on every refresh |
| **NFR-9** | Token revocation | Immediate (Valkey-backed blacklist) |
| **NFR-10** | Rate limit (login) | 10 attempts/minute per IP |
| **NFR-11** | Rate limit (password reset) | 3 requests/hour per email |
| **NFR-12** | Account lockout | 15 minutes after 5 failed attempts |
| **NFR-13** | Password minimum length | 8 characters |
| **NFR-14** | CSRF protection | Validated OAuth state parameter |
| **NFR-15** | JWT signature algorithm | RS256 (asymmetric) for production |

### Scalability

| Requirement | Target | Measurement |
|-------------|--------|-------------|
| **NFR-16** | Database queries per request | ≤ 3 (with caching) |
| **NFR-17** | Horizontal scaling | Stateless auth middleware supports multiple instances |
| **NFR-18** | Session storage | Valkey-backed for distributed deployments |

### Observability

| Requirement | Target | Measurement |
|-------------|--------|-------------|
| **NFR-19** | Trace correlation | All auth operations emit trace spans with `userId` |
| **NFR-20** | Auth event logging | All auth events published to NATS |
| **NFR-21** | Failed auth logging | Elevated log level for security events |

### Reliability

| Requirement | Target | Measurement |
|-------------|--------|-------------|
| **NFR-22** | Uptime | 99.9% for auth endpoints |
| **NFR-23** | Graceful degradation | Public endpoints accessible when NATS unavailable |
| **NFR-24** | Data durability | All auth data persisted in PostgreSQL |

---

## 7. Out of Scope

The following items are explicitly **not** included in this unit:

| Item | Reason for Exclusion |
|------|---------------------|
| **MFA (Multi-Factor Authentication)** | TOTP, SMS, hardware keys deferred to future unit |
| **Email delivery infrastructure** | SMTP/email service selection deferred to research phase; interface defined only |
| **OIDC-generic provider support** | Only Google and GitHub in scope initially |
| **General auditing infrastructure** | Audit log tables, compliance reporting belong to Auditing unit; this unit only emits events |
| **Billing integration** | Payment processing, subscriptions, invoicing deferred to future unit |
| **Security hardening** | CSP, HSTS, certificate management belong to Security unit |
| **Admin dashboard UI** | User management UI deferred to Frontend Design unit |
| **API keys for services** | Service-to-service auth deferred to future unit |
| **Session management dashboard** | Session listing/management UI deferred to future unit |
| **Password complexity configuration UI** | Configuration via environment variables only |

---

## 8. Dependencies

### External Services

| Dependency | Purpose | Required |
|------------|---------|----------|
| **PostgreSQL** | User accounts, OAuth provider records, roles, permissions, tokens | Yes |
| **Valkey** | Token blacklists, rate limiting, session state, permission caching | Yes |
| **NATS** | Auth event broadcasting via `shared/messaging` | Yes |
| **Google OAuth** | SSO authentication | Yes (for Google login) |
| **GitHub OAuth** | SSO authentication | Yes (for GitHub login) |
| **Email Service** | Transactional email (verification, password reset) | Yes (interface only; provider deferred) |

### Internal Packages

| Package | Purpose | Required |
|---------|---------|----------|
| **shared/messaging** | NATS message envelopes for auth events | Yes |
| **shared/caching** | Cache primitives for token blacklists, rate limiting | Yes |
| **response** | `Unauthorized()` and `Forbidden()` helpers | Yes |
| **OpenAPI BearerAuth** | Security scheme already defined | Yes |

### Platform Requirements

| Requirement | Description |
|-------------|-------------|
| Go 1.21+ | Backend runtime |
| Chi router | HTTP routing |
| SQLC | Type-safe database queries |
| Goose | Database migrations |
| SvelteKit | Frontend framework |

---

## 9. Assumptions

The following assumptions are made for this unit:

| ID | Assumption |
|----|------------|
| **A-1** | PostgreSQL is available and accessible for user data storage |
| **A-2** | Valkey is available for caching and token revocation |
| **A-3** | NATS is available for event publishing |
| **A-4** | OAuth credentials (Google, GitHub) are obtained during deployment |
| **A-5** | Email service credentials are configured for transactional emails |
| **A-6** | The `users` table schema includes: `id`, `email`, `password_hash`, `email_verified`, `status`, `deleted_at`, `created_at`, `updated_at` |
| **A-7** | OAuth provider records are stored in a separate `oauth_providers` table |
| **A-8** | Roles are stored as an enum with values: `admin`, `user`, `viewer` |
| **A-9** | Resource-level permissions use a junction table `resource_permissions` |
| **A-10** | All database queries use SQLC for type safety |
| **A-11** | All migrations use Goose Go functions |
| **A-12** | The SvelteKit frontend handles OAuth redirects and token storage |
| **A-13** | Deployment mode is determined by environment variable `DEPLOYMENT_MODE` (single/multi) |
| **A-14** | No `any` or `interface{}` types in auth code—explicit types throughout |
| **A-15** | No else chains—early returns only for error handling |

---

## 10. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| **SM-1: Registration completion rate** | > 80% of users complete email verification | Count: verified accounts / total registrations |
| **SM-2: Login success rate** | > 99% of valid login attempts succeed | Count: successful logins / total login attempts |
| **SM-3: Auth event delivery** | > 99.9% of events published to NATS | Count: published events / emitted events |
| **SM-4: Token validation latency** | < 5ms p99 | OpenTelemetry trace histogram |
| **SM-5: Auth endpoint availability** | 99.9% uptime | External monitoring |
| **SM-6: Failed auth rate** | < 1% of requests are auth failures | Count: 401/403 responses / total protected requests |
| **SM-7: Session revocation effectiveness** | 100% of revoked tokens rejected | Security audit |
| **SM-8: Rate limit effectiveness** | > 99% of brute-force attacks blocked | Security audit |
| **SM-9: Deployment mode flexibility** | Both modes work with same codebase | Integration tests |
| **SM-10: OAuth linking adoption** | > 30% of users link at least one OAuth provider | Count: users with OAuth / total users |

---

## Appendix: Auth Event Schema

Auth events are published via NATS with subject pattern `ace.auth.<event_type>.event`:

### Event Types

| Event | Subject | Trigger |
|-------|---------|---------|
| Login | `ace.auth.login.event` | Successful authentication |
| Logout | `ace.auth.logout.event` | User logout |
| Password Change | `ace.auth.password_change.event` | Password updated |
| Token Revocation | `ace.auth.token_revocation.event` | Session/token invalidated |
| Role Change | `ace.auth.role_change.event` | User role modified |
| Account Deleted | `ace.auth.account_deleted.event` | User account soft-deleted |
| Account Suspended | `ace.auth.account_suspended.event` | User account suspended |
| Failed Login | `ace.auth.failed_login.event` | Invalid credentials attempt |

### Message Envelope (via shared/messaging)

```json
{
  "event_id": "uuid",
  "event_type": "login|logout|...",
  "user_id": "uuid",
  "timestamp": "ISO8601",
  "metadata": {
    "ip_address": "string",
    "user_agent": "string",
    "provider": "google|github|email",
    "session_id": "uuid",
    ...
  }
}
```

---

## Appendix: Database Schema Summary

### Core Tables

| Table | Purpose |
|-------|---------|
| `users` | User accounts with email, password_hash, status, roles |
| `oauth_providers` | Linked OAuth accounts (Google, GitHub) |
| `email_verification_tokens` | Email verification tokens |
| `password_reset_tokens` | Password reset tokens |
| `sessions` | Active user sessions (if stateful) |
| `roles` | System roles (admin, user, viewer) |
| `resource_permissions` | Resource-level access grants |

### Indexes

| Table | Index | Purpose |
|-------|-------|---------|
| `users` | `email` (unique) | Login lookup |
| `users` | `status` | Admin queries |
| `oauth_providers` | `(user_id, provider)` | OAuth linking |
| `oauth_providers` | `provider_id` | OAuth login |
| `password_reset_tokens` | `token_hash` | Reset validation |
| `sessions` | `user_id` | Session listing |

---

**Document Version:** 1.0  
**Unit:** users-auth  
**Status:** Draft  
**Created:** 2026-04-09
