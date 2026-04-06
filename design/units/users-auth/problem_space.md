# Problem Space

## Initial Discovery

### Core Questions

**Q: What problem are we trying to solve?**
A: The ACE Framework currently has no authentication or authorization system. All API endpoints are public — there is no concept of a user, no login, no access control, and no way to isolate one user's agents from another's. This is acceptable for a prototype but blocks every future feature: agents must belong to users, costs must be attributed to accounts, resource sharing requires identity, and billing requires accounts. This unit establishes the complete identity and access management foundation — user accounts, authentication flows, authorization model, and the middleware that enforces access control across all services. Without this unit, the system cannot support multi-user SaaS deployments, cannot protect agent data, and cannot implement billing or auditing.

The specific problems being solved are:
1. **No user identity** — There is no way to create, verify, or manage user accounts
2. **No authentication** — Any client can call any endpoint; no login, no tokens, no session management
3. **No authorization** — No role-based access control, no resource-level permissions, no way to share agents between users
4. **No auth event emission** — Auth operations (login, logout, password change, token revocation) produce no observable events for downstream consumption. The audit trail infrastructure that consumes these events belongs to the separate Auditing unit, but this unit must emit the events that feed it.
5. **Multi-user SaaS impossible** — Without identity and access control, the system cannot scale beyond a single-user hobbyist deployment

**Q: Who are the users?**
A:
- **End users (hobbyists)** — Single-user deployments, want simple email/password signup and login, no complexity
- **End users (SaaS customers)** — Multi-user deployments, want SSO (Google/GitHub), team collaboration, agent sharing
- **Administrators** — Platform operators who manage users, roles, billing, and system-wide settings
- **Backend services** (consuming auth middleware and shared packages) — API handlers that need to identify the authenticated user and enforce access control
- **Developers** — Need clear patterns for adding protected endpoints, integrating SSO, and extending RBAC
- **Future services** — Any service that needs to verify identity or check permissions will depend on the auth primitives established here

**Q: What are the success criteria?**
A:
1. Users can register with email/password and verify their email address
2. Users can authenticate via Google OAuth and GitHub OAuth
3. Authenticated requests carry verifiable identity (JWT) through the middleware stack
4. RBAC enforces role-based access: admin, user, viewer roles with distinct permissions
5. Resource-level authorization allows users to share agents with other users (e.g., as viewers)
6. Password reset flow works end-to-end with secure token generation and expiration
7. All protected endpoints return 401 for unauthenticated requests and 403 for unauthorized requests
8. The auth system works identically in single-user and multi-user deployment modes
9. Auth middleware emits auth-specific events via `shared/messaging` for audit logging and trace correlation
10. OpenAPI `BearerAuth` security scheme is enforced on all protected endpoints

**Q: What constraints exist (budget, timeline, tech stack)?**
A:
- **Tech Stack**: Go backend (Chi router, SQLC, PostgreSQL), SvelteKit/TypeScript frontend, NATS messaging, Valkey caching, OpenTelemetry observability
- **Handler → Service → Repository pattern** — All auth logic follows the established layered architecture
- **SQLC for database access** — All queries are type-safe, no raw SQL in Go code
- **Goose Go functions for migrations** — No SQL migration files, all migrations are Go functions
- **No `interface{}` or `any`** — Explicit types throughout
- **No else chains** — Early returns only
- **NATS for inter-service communication** — Auth events (login, logout, password change) flow through NATS for cache invalidation and downstream service coordination
- **Valkey for caching** — Token revocation state, rate limiting, and permission caching use Valkey via `shared/caching` (exact strategies to be determined in research phase)
- **Auth events are separate from UsageEvents** — Auth events (login, logout, password change, token revocation, role change) use raw NATS messaging via `shared/messaging` with subject pattern `ace.auth.%s.event`. They are NOT UsageEvents — UsageEvent (in `shared/telemetry`) is designed for LLM calls, memory reads, and tool executions with AgentID/CycleID/SessionID fields. Auth events have different semantics and schema.
- **OpenTelemetry integration** — All auth operations emit trace spans with userId correlation for observability
- **Existing response package** — `response.Unauthorized()` and `response.Forbidden()` helpers already exist and must be used
- **OpenAPI already documents BearerAuth** — Security scheme is defined as `BearerAuth` with `scheme: bearer` and `bearerFormat: JWT`
- **All operations go through the Makefile** — No direct docker/go/npm commands
- **Pre-commit hooks are mandatory** — All code passes quality gates

## Iterative Exploration

### Authentication Architecture

#### 1. Multi-user SaaS Model
**Q: How should the system handle both single-user and multi-user deployments?**
A: **Unified user model with deployment-mode awareness** — The same database schema and authentication flows work for both single-user (hobbyist Docker Compose) and multi-user (Kubernetes SaaS) deployments. In single-user mode, the first user created automatically becomes an admin and no further registration is allowed (or registration is disabled via config). In multi-user mode, registration is open and users manage their own accounts. The deployment mode is determined by environment configuration, not by code branching.

#### 2. Email + Password + SSO
**Q: What authentication methods should be supported?**
A: **Email/password and SSO (Google + GitHub) with a unified user model** — A single `users` table stores all user accounts regardless of authentication method. Email/password users have a password hash; SSO users have OAuth provider records linked to their account. A user can have both email/password and SSO linked to the same account. Email verification is mandatory for email/password accounts. Password reset uses secure, time-limited tokens sent via email.

#### 3. RBAC + Resource-level Authorization
**Q: What authorization model should be used?**
A: **RBAC with resource-level permissions** — System-level roles (admin, user, viewer) define what a user can do globally. Resource-level permissions define what a user can do with specific resources (e.g., "user A can view agent B"). The authorization model supports:
- **System roles**: `admin` (full access), `user` (manage own resources), `viewer` (read-only access to shared resources)
- **Resource sharing**: Users can share agents, tools, skills, and configurations with other users at specific permission levels
- **Ownership**: Every resource has an `owner_id` referencing the user who created it
- **Permission checks**: Middleware and service-layer checks enforce both role and resource-level permissions

#### 4. SSO Providers: Google + GitHub
**Q: Which OAuth providers should be supported initially?**
A: **Google and GitHub** — These cover the vast majority of developer and general user SSO needs. The implementation should be structured to allow additional OIDC-generic providers later, but the initial scope is Google and GitHub only. Each provider stores its own OAuth token and refresh token for potential API access on behalf of the user.

#### 5. JWT Token Strategy
**Q: What JWT strategy should be used?**
A: **Deferred to research phase** — The specific JWT implementation (access token duration, refresh token rotation, token storage, revocation strategy) requires research into best practices for Go + SvelteKit + NATS architectures. The decision must balance security, usability, and the distributed nature of the system. Key considerations include:
- Short-lived access tokens (5-15 minutes) with refresh token rotation
- Token revocation via Valkey blacklist for immediate logout and security events
- Refresh token storage strategy (httpOnly cookies vs. secure storage)
- Token refresh flow for SvelteKit frontend
- Token validation middleware for Chi router

### Integration with Existing Architecture

#### 6. Relationship to Existing API
**Q: How does auth integrate with the existing public API endpoints?**
A: **Auth middleware wraps protected routes; public endpoints remain unprotected** — The existing `/health/*`, `/examples/*`, and `/metrics` endpoints remain public (no auth required). All new endpoints that access user-specific resources require authentication. The auth middleware extracts and validates the JWT from the `Authorization: Bearer <token>` header, attaches the user identity to the request context, and allows the handler to proceed. The `response.Unauthorized()` and `response.Forbidden()` helpers handle error responses.

#### 7. Relationship to shared/messaging and NATS
**Q: How does auth integrate with the observability and event pipeline?**
A: **Auth events are separate from UsageEvents** — Auth operations (login, logout, password change, token revocation, role changes) emit auth-specific events via `shared/messaging` using NATS subject pattern `ace.auth.%s.event` (e.g., `ace.auth.login.event`, `ace.auth.logout.event`, `ace.auth.password_change.event`). These events are separate from UsageEvents which are designed for LLM calls, memory reads, and tool executions with AgentID/CycleID/SessionID fields. Auth events have their own schema and semantics.

The auth events are consumed by:
- **Auditing unit** — Audit log ingestion for compliance and security tracking
- **Cache invalidation services** — Session and permission cache updates
- **Session tracking services** — Login activity dashboards

Trace spans carry `userId` for correlation across the observability pipeline. Auth failures are tracked as security-relevant events with elevated log levels.

#### 8. Relationship to shared/caching
**Q: How does auth use the caching layer?**
A: **Valkey for token blacklists, rate limiting, and session state** — The `shared/caching` package provides primitives for:
- **Token blacklist**: Revoked tokens are cached in Valkey with TTL matching the original token's remaining lifetime
- **Rate limiting**: Login attempt rate limiting uses Valkey counters with sliding windows
- **Session state**: Active session metadata (last activity, device info) cached for dashboard display
- **Permission caching**: Resource-level permissions cached to avoid repeated database queries on every request

#### 9. Relationship to NATS
**Q: How does auth communicate with other services via NATS?**
A: **Auth events published as NATS messages for downstream consumption** — When a user logs in, changes their password, or has their role modified, an event is published to NATS so other services can react:
- **Login event**: Services can invalidate stale caches, update session tracking
- **Password change event**: All active sessions for the user are invalidated
- **Role change event**: Permission caches are invalidated across services
- **Account deletion event**: All user resources are marked for cleanup
The auth service publishes these events via `shared/messaging` using subject pattern `ace.auth.<event_type>.event`; other services subscribe as needed. The message envelope follows `shared/messaging` patterns with `userId` in the headers.

### Security Considerations

#### 10. Password Security
**Q: What password hashing algorithm should be used?**
A: **To be determined in research phase** — Argon2id is the leading candidate, providing resistance against GPU-based attacks and side-channel attacks. The research phase must confirm the optimal algorithm (Argon2id vs bcrypt vs scrypt), select a Go library, and determine configurable parameters (memory, iterations, parallelism). Passwords must meet minimum complexity requirements (configurable, with sensible defaults).

#### 11. Rate Limiting
**Q: How should authentication endpoints be rate-limited?**
A: **To be determined in research phase** — Login attempts must be rate-limited per email address and per IP address, with configurable thresholds for temporary account lockout. Options to evaluate include sliding window counters in Valkey, token bucket algorithms, and fixed-window approaches. The research phase must select the strategy and confirm it integrates with the existing `shared/caching` primitives.

#### 12. Email Verification and Password Reset
**Q: How should email verification and password reset work?**
A: **Secure token-based flows with expiration** — Email verification tokens and password reset tokens must be cryptographically random, single-use, and expire after a configurable duration. The research phase must evaluate token storage strategies (e.g., storing a hash of the token in PostgreSQL rather than the raw token to prevent token theft from database compromise). The exact token generation method, storage pattern, and expiration defaults will be determined during research.

## Functional Requirements

The following functional requirements will feed directly into the BSD. Each is numbered for traceability.

- **FR-1**: The system SHALL allow user registration via email and password
- **FR-2**: The system SHALL allow user login via email and password
- **FR-3**: The system SHALL support OAuth/SSO login via Google and GitHub
- **FR-4**: The system SHALL issue JWT access tokens for authenticated sessions (token strategy to be determined in research phase)
- **FR-5**: The system SHALL enforce RBAC with system-level roles (admin, user, viewer) and resource-level authorization
- **FR-6**: The system SHALL allow users to share resources (agents, tools, skills, configurations) with other users at specific permission levels
- **FR-7**: The system SHALL provide password reset via email with secure, time-limited tokens
- **FR-8**: The system SHALL verify email addresses during registration for email/password accounts
- **FR-9**: The system SHALL support both multi-user SaaS and single-user deployment modes through configuration only, with no code branching between modes
- **FR-10**: The system SHALL emit auth events (login, logout, password change, token revocation, role change) via `shared/messaging` using NATS subject pattern `ace.auth.<event_type>.event` for downstream consumption by the Auditing unit and other services. These are NOT UsageEvents — auth events have separate schema and semantics.
- **FR-11**: The system SHALL return 401 for unauthenticated requests to protected endpoints and 403 for authenticated requests with insufficient permissions
- **FR-12**: The system SHALL rate-limit authentication endpoints to prevent brute-force attacks
- **FR-13**: The system SHALL allow a user to link multiple OAuth providers (Google, GitHub) to a single account
- **FR-14**: The system SHALL allow users to unlink OAuth providers from their account

## Key Insights

1. **Auth is foundational infrastructure** — Like messaging, telemetry, and caching, authentication and authorization must be established before any feature service that needs identity is built. Retrofitting auth across services is exponentially more costly than building it first.

2. **Unified user model simplifies everything** — A single `users` table with linked OAuth provider records avoids the complexity of separate user models for email/password vs SSO. This is the pattern used by Supabase, Clerk, and Auth0.

3. **Deployment-mode awareness is critical** — The system must work seamlessly for a single hobbyist running Docker Compose and for a SaaS platform with thousands of users. The same codebase, same database schema, same API — only configuration differs.

4. **Resource ownership is the foundation of authorization** — Every resource (agent, tool, skill, configuration) must have an `owner_id`. Without this, resource-level permissions are impossible. The database design must enforce ownership from the start.

5. **Token revocation must be immediate** — When a user logs out, changes their password, or is deactivated, their tokens must be invalidated immediately. The research phase must evaluate revocation strategies (e.g., Valkey-backed blacklist, short token lifetimes, or hybrid approaches) that avoid per-request database queries.

6. **Auth events feed the audit pipeline** — This unit emits auth-specific events (login, logout, password change, token revocation, role change) via `shared/messaging` using NATS subject pattern `ace.auth.<event_type>.event`. These events are separate from UsageEvents which are designed for LLM calls, memory reads, and tool executions with AgentID/CycleID/SessionID fields. The audit trail infrastructure — audit log tables, compliance reporting, query patterns, and retention policies — belongs to the separate Auditing unit. This unit's responsibility is event emission only, not audit storage or querying.

7. **SSO is table stakes for SaaS** — Google and GitHub OAuth cover the majority of developer and general user sign-in needs. The implementation should be structured for extensibility to additional OIDC providers.

8. **The JWT strategy is the hardest technical decision** — Token duration, refresh rotation, storage, and revocation strategy have significant security and usability implications. This requires careful research and cannot be deferred indefinitely.

## Non-Goals

The following are explicitly out of scope for this unit:

- **MFA (Multi-Factor Authentication)** — TOTP, SMS-based 2FA, and hardware key support are deferred to a future unit
- **Email delivery infrastructure** — Selection and integration of an email service provider (SES, SendGrid, etc.) is deferred to the research phase; this unit defines the email event interface but does not implement delivery
- **OIDC-generic provider support** — Only Google and GitHub OAuth are in scope. A generic OIDC provider adapter may be added in a future unit
- **General auditing infrastructure** — Audit log tables, compliance reporting, query patterns, and retention policies belong to the separate Auditing unit. This unit only emits auth events via `shared/messaging` with NATS subject pattern `ace.auth.<event_type>.event`
- **Billing integration** — Payment processing, subscription management, and invoicing are deferred to a future unit
- **General security hardening** — CSP, HSTS, certificate management, and TLS configuration belong to the separate Security unit
- **Admin dashboard UI** — User management UI for administrators is deferred to the Frontend Design unit

## Dependencies Identified

- **PostgreSQL** — User accounts, OAuth provider records, roles, permissions, password reset tokens, email verification tokens
- **Valkey** — Token blacklists, rate limiting, session state, permission caching (via `shared/caching`)
- **NATS** — Auth event broadcasting for downstream service coordination via `shared/messaging` with subject pattern `ace.auth.<event_type>.event`
- **shared/messaging** — NATS message envelopes for auth event publishing (NOT shared/telemetry/UsageEvent)
- **shared/caching** — Cache primitives for token blacklists and permission caching
- **response package** — `response.Unauthorized()` and `response.Forbidden()` for error responses
- **OpenAPI BearerAuth** — Security scheme already documented; middleware must enforce it

## Assumptions Made

1. The `users` table will include: `id`, `email`, `password_hash` (nullable for SSO-only users), `email_verified`, `status` (active, suspended, pending_verification), `deleted_at` (TIMESTAMPTZ for soft deletes, following existing database conventions), `created_at`, `updated_at`
2. OAuth provider records will be stored in a separate `oauth_providers` table linked to `users` by `user_id`
3. Roles will be stored as an enum or small lookup table with system-level roles: `admin`, `user`, `viewer`
4. Resource-level permissions will use a junction table (e.g., `resource_permissions`) linking users to resources with permission levels
5. JWT token strategy (lifetime, refresh rotation, storage mechanism) will be determined during the research phase — see Open Questions
6. Email delivery will use a configurable provider interface (specific provider deferred to research phase)
7. The SvelteKit frontend will handle OAuth redirects and token storage (exact mechanism deferred to research phase)
8. Password complexity requirements will be configurable with sensible defaults
9. Rate limiting strategy will be determined during the research phase — see Open Questions
10. All auth-related database queries will use SQLC for type safety
11. All auth migrations will use Goose Go functions

## Open Questions (For Research)

1. **JWT token strategy**: Optimal access token duration, refresh token rotation mechanism, token storage (httpOnly cookies vs. secure storage), revocation strategy, and whether refresh tokens are stored in PostgreSQL (persistence) and/or Valkey (revocation)
2. **Email delivery**: Which SMTP provider or email service to use? How to handle email templates and localization?
3. **Session management**: Should sessions be stateless (JWT-only) or stateful (server-side session records)? Hybrid approach?
4. **CSRF protection**: What CSRF strategy for the SvelteKit frontend? Double-submit cookie? SameSite cookies?
5. **OAuth state parameter**: How to securely generate and validate OAuth state parameters to prevent CSRF attacks?
6. **Password reset token delivery**: Email-only, or should we support SMS/2FA in the future?
7. **Multi-factor authentication**: Should MFA be in scope for this unit or deferred to a future unit? (Currently assumed out of scope — confirm.)
8. **Auth event schema**: What fields should auth events (login, logout, password change, etc.) include? What is the exact message envelope format for `ace.auth.<event_type>.event`?
9. **Deployment-mode auto-provisioning**: In single-user mode, should the first user be auto-created, or should an admin seed script handle it?
10. **Token refresh endpoint**: Should token refresh be a dedicated endpoint, or should it be handled via cookie-based automatic refresh?
11. **Password hashing algorithm**: Argon2id is the leading candidate — confirm optimal algorithm, Go library selection, and default parameters (memory, iterations, parallelism)
12. **Rate limiting strategy**: Evaluate sliding window counters, token bucket, and fixed-window approaches for login attempt rate limiting

## Next Steps

1. Proceed to BSD (Business Specification Document) with the problem space clarified
2. Research phase should evaluate:
   - JWT token strategy (duration, rotation, storage, revocation)
   - Email delivery options and template management
   - Session management patterns (stateless vs. stateful vs. hybrid)
   - CSRF protection strategies for SvelteKit
   - OAuth state parameter generation and validation
   - MFA scope decision (this unit vs. future unit)
3. Design the database schema for users, OAuth providers, roles, and permissions
4. Design the auth middleware stack for Chi router
5. Define the shared auth package interfaces (if any) for transport-agnostic auth primitives
6. Define NATS event shapes for auth event broadcasting (subject pattern: `ace.auth.<event_type>.event`)
7. Define the API contract for all auth endpoints (register, login, logout, refresh, password reset, email verify, OAuth callback)