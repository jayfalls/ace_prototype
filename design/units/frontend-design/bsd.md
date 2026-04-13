# BSD: Frontend Design

[unit: frontend-design]

## 1. Business Domain

ACE's frontend is a single-page application served from the Go binary's embedded assets. It provides an authenticated dashboard for managing autonomous agents, inspecting their cognitive layers, and monitoring system telemetry. The backend exposes a REST API with JWT auth; the frontend's job is to present all of this coherently.

---

## 2. User Roles & Permissions

| Role | Capabilities |
|------|-------------|
| **admin** | Full system access: user management (list, view, suspend, restore, role change), all telemetry, all sessions, own profile |
| **user** | Own profile, own sessions, own agents (future), all telemetry (read-only) |
| **viewer** | Read-only: view telemetry, view own profile. Cannot create agents, cannot manage users |

Role enforcement is server-side (RBAC middleware). The frontend derives permissions from the JWT claims to show/hide UI elements — this is cosmetic only, not a security boundary.

---

## 3. Business Rules

### 3.1 Authentication

- **Registration**: `POST /auth/register` → `{ email, password }` → `{ access_token, refresh_token, user, expires_in }`. Password minimum 8 characters.
- **Login**: `POST /auth/login` → `{ email, password }` → `{ access_token, refresh_token, user, expires_in }`.
- **Token refresh**: `POST /auth/refresh` → `{ refresh_token }` → `{ access_token, refresh_token, expires_in }`. Proactive refresh when token expires in <30s. Refresh mutex prevents concurrent refresh races.
- **Logout**: `POST /auth/logout` → `{ session_id }` → `{ message }`. Invalidates session server-side, clears client storage.
- **Password reset**: Two-step: `POST /auth/password/reset/request` → `{ email }` → email sent. Then `POST /auth/password/reset/confirm` → `{ token, new_password }` → new tokens.
- **Magic link**: Two-step: `POST /auth/magic-link/request` → `{ email }` → email sent. Then `POST /auth/magic-link/verify` → `{ token }` → `{ access_token, refresh_token, user, expires_in }`.

### 3.2 Session Management

- **List sessions**: `GET /auth/me/sessions?page=1&limit=20` → paginated session list with IP, user agent, created_at, last_used_at, expires_at.
- **Revoke session**: `DELETE /auth/me/sessions/{id}` → terminates a specific session. Used for "sign out other devices".
- **Current user**: `GET /auth/me` → returns authenticated user profile (id, email, role, status, created_at, updated_at).

### 3.3 Admin — User Management

- **List users**: `GET /admin/users?page=1&limit=20&status=active` → paginated user list. Filters: status.
- **Get user**: `GET /admin/users/{id}` → full user detail including suspended_at, suspended_reason.
- **Update role**: `PUT /admin/users/{id}/role` → `{ role: "admin" | "user" | "viewer" }`. Admin-only.
- **Suspend user**: `POST /admin/users/{id}/suspend` → `{ reason }`. Sets status to `suspended`, records reason and timestamp.
- **Restore user**: `POST /admin/users/{id}/restore` → sets status back to `active`, clears suspension fields.

### 3.4 Telemetry

- **Spans**: `GET /telemetry/spans` → paginated list of OTel spans. Filters: service, operation, status, time range. Returns trace_id, span_id, operation, service, duration, status.
- **Metrics**: `GET /telemetry/metrics` → aggregated metrics. Filters: service, name, window (1h, 6h, 24h, 7d). Returns metric name, value, timestamp, dimensions.
- **Usage**: `GET /telemetry/usage` → usage events (LLM calls, memory reads, tool executions). Filters: agent_id, operation_type, resource_type, time range. Returns cost, duration, metadata per event.
- **Health**: `GET /telemetry/health` → system health (spans last hour, metrics last hour, usage last hour counts).

### 3.5 Health

- **Liveness**: `GET /health/live` → `{ status: "ok" }`. No auth required.
- **Readiness**: `GET /health/ready` → `{ status, checks: { database, nats, cache } }`. No auth required.

---

## 4. State Transitions

### 4.1 Auth State Machine

```
[Unauthenticated] ──login/register/magic-link──> [Authenticated]
[Authenticated] ──token expired──> [Refreshing]
[Refreshing] ──success──> [Authenticated]
[Refreshing] ──failure──> [Unauthenticated] (redirect to /login)
[Authenticated] ──logout──> [Unauthenticated] (clear storage)
[Authenticated] ──401 response──> [Refreshing] ──failure──> [Unauthenticated]
```

Rules:
- On app load, check localStorage for tokens. If present and not expired, transition to Authenticated.
- If present but expired, attempt refresh. Success → Authenticated. Failure → Unauthenticated.
- Proactive refresh: if token expires within 30 seconds, refresh before the next API call.
- On logout, call `/auth/logout`, clear localStorage, reset auth store, redirect to `/login`.

### 4.2 User Status Transitions (Admin-only)

```
[pending] ──(first login)──> [active]
[active] ──suspend──> [suspended]
[suspended] ──restore──> [active]
```

A suspended user cannot authenticate (server rejects with 403). The frontend does not need to handle this client-side beyond displaying "Account suspended" on login failure.

### 4.3 UI Navigation States

```
[Loading] ──auth resolved──> [Authenticated Shell] or [Login Page]
[Authenticated Shell] ──sidebar collapse──> [Collapsed Sidebar]
[Authenticated Shell] ──sidebar expand──> [Expanded Sidebar]
[Authenticated Shell] ──route change──> [Page Transition] ──data loaded──> [Page Ready]
```

The sidebar starts collapsed on mobile, expanded on desktop. User preference persists in localStorage.

### 4.4 Page Data States

Every page that fetches data follows this pattern:

```
[Initial] ──fetch triggered──> [Loading]
[Loading] ──success──> [Success] (data rendered)
[Loading] ──error──> [Error] (retry button shown)
[Success] ──refetch──> [Loading] (show stale data with loading indicator)
[Error] ──retry──> [Loading]
```

---

## 5. Theme System

### 5.1 Theme Engine

Themes are CSS custom property sets scoped to a root `<html>` class. One active theme at a time. Theme selection persists across sessions (localStorage key: `ace-theme`).

**Theme structure:**
Each theme defines a complete set of CSS custom properties:
- `--color-background`, `--color-foreground` — base surfaces
- `--color-card`, `--color-card-foreground` — card surfaces
- `--color-primary`, `--color-primary-foreground` — primary actions
- `--color-secondary`, `--color-secondary-foreground` — secondary actions
- `--color-muted`, `--color-muted-foreground` — subdued elements
- `--color-accent`, `--color-accent-foreground` — highlights
- `--color-destructive`, `--color-destructive-foreground` — errors, dangers
- `--color-border`, `--color-input`, `--color-ring` — form elements
- `--radius-sm`, `--radius-md`, `--radius-lg` — border radii
- `--font-sans`, `--font-mono` — font families

**Dark/light mode:**
Each theme preset has a dark variant and a light variant. The toggle switches between variants of the same preset (e.g., from "One Dark" to "One Light"). If the user changes the preset, the mode (dark/light) is preserved.

### 5.2 Preset Themes (Phase 1)

| Preset | Style | Description |
|--------|-------|-------------|
| **One Dark** | Dark | Atom One Dark-inspired. Warm tones, high contrast. Default theme. |
| **One Light** | Light | Atom One Light-inspired. Clean, warm whites. |
| **Catppuccin Mocha** | Dark | Pastel dark theme. Soft contrast, cozy feel. |
| **Catppuccin Latte** | Light | Pastel light theme. Warm tones. |
| **Nord** | Dark | Arctic blue-grey palette. Cool, calm. |
| **Monokai** | Dark | Vibrant syntax highlighter palette. High contrast pops of color. |

### 5.3 Theme Switching Logic

1. On app init, read `ace-theme` from localStorage. If missing, default to "one-dark".
2. If the chosen theme is a dark variant, set `class="dark"` on `<html>` and apply the dark custom properties.
3. If the chosen theme is a light variant, remove `class="dark"` and apply the light custom properties.
4. Theme selector in settings shows current preset. Selecting a new preset applies it immediately and persists.
5. Dark/light toggle flips between the dark and light variants of the current preset.

---

## 6. Navigation & Layout

### 6.1 App Shell

The app shell is a fixed layout with:
- **Sidebar** (left): Collapsible, 256px expanded, 64px collapsed. Contains logo, primary navigation items, and settings/user at bottom.
- **Header** (top): Shows breadcrumbs, page title, and user avatar/menu.
- **Content** (center): Scrollable main content area.

On mobile (<768px), the sidebar becomes an overlay that slides in from the left. A hamburger menu button in the header toggles it. Clicking outside or navigating closes it.

### 6.2 Route Structure

| Route | Page | Auth | Role |
|-------|------|------|------|
| `/login` | Login form | No | Any |
| `/register` | Registration form | No | Any |
| `/forgot-password` | Password reset request | No | Any |
| `/reset-password` | Password reset confirm (token in URL) | No | Any |
| `/magic-link` | Magic link verification (token in URL) | No | Any |
| `/` | Dashboard overview | Yes | Any |
| `/profile` | User profile & session management | Yes | Any |
| `/admin/users` | User list (paginated) | Yes | Admin |
| `/admin/users/[id]` | User detail | Yes | Admin |
| `/telemetry` | Telemetry overview | Yes | Any |
| `/telemetry/spans` | Spans browser | Yes | Any |
| `/telemetry/metrics` | Metrics dashboard | Yes | Any |
| `/telemetry/usage` | Usage & cost analysis | Yes | Any |
| `/settings` | App settings (theme, preferences) | Yes | Any |

### 6.3 Navigation Items

Sidebar primary navigation:
- **Dashboard** (icon: LayoutDashboard) → `/`
- **Agents** (icon: Bot) → `/agents` (placeholder, future unit)
- **Telemetry** (icon: Activity) → `/telemetry`
- **Admin** (icon: Shield) → `/admin/users` (admin role only)

Bottom sidebar:
- **Settings** (icon: Settings) → `/settings`
- **User avatar** → profile dropdown with profile link, session management, logout

---

## 7. Page Specifications

### 7.1 Login Page (`/login`)

**Purpose**: Authenticate existing users.

**State**: Email input, password input, submit button, "Forgot password?" link, "Register" link, "Sign in with magic link" link.

**Business rules**:
- Email and password are required. Email must be valid format. Password minimum 8 characters (client-side; server enforces as well).
- On success, store tokens, redirect to `/`.
- On 401, show "Invalid email or password" error.
- On network error, show "Unable to connect. Please try again."
- If already authenticated, redirect to `/`.

### 7.2 Register Page (`/register`)

**Purpose**: Create a new account.

**State**: Email input, password input, confirm password input, submit button, "Already have an account?" link.

**Business rules**:
- Email, password, confirm password required.
- Password minimum 8 characters.
- Password must match confirm password (client-side validation).
- On 201, auto-login (store tokens), redirect to `/`.
- On 409, show "An account with this email already exists."
- On 400, show validation errors from server response.

### 7.3 Forgot Password Page (`/forgot-password`)

**Purpose**: Request password reset email.

**State**: Email input, submit button, "Back to login" link.

**Business rules**:
- Always show "If an account exists with this email, a reset link has been sent." This prevents email enumeration.
- On success, show confirmation message with "Check your email" guidance.
- On error, still show the same confirmation message (server does not reveal whether the email exists).

### 7.4 Reset Password Page (`/reset-password`)

**Purpose**: Set new password after clicking reset link from email.

**State**: New password input, confirm password input, submit button. Token extracted from URL query param.

**Business rules**:
- Token comes from URL: `/reset-password?token=xxx`.
- Password minimum 8 characters. Must match confirm password.
- On success, auto-login with new tokens (server returns TokenResponse), redirect to `/`.
- On 400, show "This reset link has expired or is invalid."

### 7.5 Magic Link Page (`/magic-link`)

**Purpose**: Verify magic link token from email.

**State**: Loading spinner while verifying token, success/error message. Token extracted from URL.

**Business rules**:
- Token from URL: `/magic-link?token=xxx`.
- On page load, immediately call `POST /auth/magic-link/verify` with the token.
- On success, store tokens, redirect to `/`.
- On failure, show "This link has expired or is invalid" with a "Request a new link" button.

### 7.6 Dashboard (`/`)

**Purpose**: Overview of the ACE system. Landing page after login.

**State**: System health status, recent activity summary, quick actions.

**Content**:
- System health card: database, NATS, cache status from `GET /health/ready`.
- Quick stats: spans in last hour, metrics in last hour, usage events in last hour (from `GET /telemetry/health`).
- Welcome message with user name.
- Quick-action buttons: "View Telemetry", "Manage Users" (admin only), "View Profile".

### 7.7 Profile Page (`/profile`)

**Purpose**: View and manage user profile and sessions.

**State**: User info card, session list with revoke buttons, logout-all button.

**Content**:
- User profile card: email, role, status, created_at (from `GET /auth/me`).
- Active sessions table: device info (user agent parsed), IP address, last used, created, expires. Each row has a "Revoke" button.
- "Sign out all other sessions" button that revokes all sessions except current.
- "Change password" link → `/forgot-password` (reuses the password reset flow).

### 7.8 Admin — User List (`/admin/users`)

**Purpose**: List and filter all users. Admin-only.

**State**: Paginated user table, status filter, search input.

**Content**:
- Table columns: Email, Role, Status, Created, Actions.
- Status filter dropdown: All, Active, Pending, Suspended.
- Pagination controls (page number, items per page).
- Actions per row: View detail, Suspend (for active), Restore (for suspended), Change role.

### 7.9 Admin — User Detail (`/admin/users/[id]`)

**Purpose**: View and manage a single user. Admin-only.

**State**: User detail card, action buttons.

**Content**:
- User info: id, email, role, status, created_at, updated_at.
- If suspended: show suspended_at, suspended_reason.
- Action buttons: "Change Role" (dropdown with admin/user/viewer), "Suspend" (with reason input), "Restore" (if suspended).
- Confirmation dialog for destructive actions (suspend).
- "Back to users" link.

### 7.10 Telemetry Overview (`/telemetry`)

**Purpose**: High-level telemetry summary.

**State**: Health metrics cards, navigation to sub-pages.

**Content**:
- Cards showing: spans last hour, metrics last hour, usage events last hour (from `GET /telemetry/health`).
- Quick links: "View Spans", "View Metrics", "View Usage".

### 7.11 Telemetry — Spans (`/telemetry/spans`)

**Purpose**: Browse and filter OpenTelemetry spans.

**State**: Filterable span list/table.

**Content**:
- Filters: service, operation name, status (ok/error), time range.
- Table columns: Trace ID, Span ID, Operation, Service, Duration, Status, Timestamp.
- Click row → expand for span details (attributes, events).
- Pagination.

### 7.12 Telemetry — Metrics (`/telemetry/metrics`)

**Purpose**: View aggregated telemetry metrics.

**State**: Metric list with aggregation options.

**Content**:
- Filters: service, metric name, time window (1h, 6h, 24h, 7d).
- Metric cards/list showing: name, value, timestamp, dimensions.
- Pagination.

### 7.13 Telemetry — Usage (`/telemetry/usage`)

**Purpose**: Analyze resource usage and costs.

**State**: Usage event list with filters.

**Content**:
- Filters: agent_id, operation_type (llm_call, memory_read, memory_write, tool_execute, db_query), resource_type (api, memory, tool, database, messaging), time range.
- Table columns: Agent ID, Operation, Resource, Duration, Cost, Timestamp.
- Cost summary (total cost in period).
- Pagination.

---

## 8. Error Handling Rules

### 8.1 API Error Envelope

All backend errors follow the format:
```json
{
  "success": false,
  "error": {
    "code": "string",
    "message": "Human-readable message",
    "details": [{ "field": "email", "message": "Invalid format" }]
  }
}
```

The frontend maps error codes to user-facing messages:
- `unauthorized` → redirect to login
- `forbidden` → show "You do not have permission" message
- `not_found` → show 404 page
- `validation_error` → show field-level errors on form
- `rate_limit_exceeded` → show "Too many requests. Please wait."
- `internal_error` → show generic "Something went wrong. Please try again."

### 8.2 Network Errors

- Network failures (no response) show: "Unable to connect to the server. Please check your connection."
- Timeouts (request takes >10s) show: "The server is taking too long to respond."

### 8.3 Rate Limiting

The backend applies rate limiting (see `rate_limit_middleware.go`). On 429:
- Show "Too many requests. Please wait a moment and try again."
- Exponential backoff on retries for the same endpoint (1s, 2s, 4s, max 30s).

---

## 9. Client-Side Validation Rules

All form validation uses Zod schemas that match server constraints:

| Field | Rules |
|-------|-------|
| **Email** | Required, valid email format |
| **Password** | Required, minimum 8 characters |
| **Confirm Password** | Must match password field |
| **Suspension Reason** | Optional, max 500 characters |
| **Role Selection** | One of: admin, user, viewer |

Validation fires on blur (not on input). Submit validates all fields. Server validation errors override client validation (server is authoritative).

---

## 10. Responsive Breakpoints

| Breakpoint | Width | Sidebar Behavior |
|------------|-------|------------------|
| Mobile | <768px | Overlay sidebar, hamburger toggle |
| Tablet | 768px–1024px | Collapsed sidebar (icons only), toggleable |
| Desktop | >1024px | Expanded sidebar (default), toggleable |

All pages must be functional on mobile. Tables use horizontal scroll or card view on mobile. Admin user table switches to card layout on mobile.

---

## 11. Accessibility Requirements (WCAG 2.1 AA)

- All interactive elements reachable via keyboard (Tab, Enter, Space, Escape)
- All icons have `aria-label` attributes
- Focus trap in modals and dialogs
- Focus visible on all interactive elements (ring indicator)
- Skip-to-content link on app shell
- Proper heading hierarchy (h1 → h2 → h3, no skipped levels)
- ARIA roles on sidebar (`role="navigation"`), main content (`role="main"`), header (`role="banner"`)
- Color contrast ratio ≥ 4.5:1 for normal text, ≥ 3:1 for large text
- Form inputs have associated `<label>` elements
- Error messages associated with inputs via `aria-describedby`
- Loading states announced via `aria-live="polite"`
- Page titles change on route navigation

---

## 12. Out of Scope

These items are explicitly excluded from this unit and will be addressed in future units:

- **Real-time updates (WebSocket/SSE)**: Static data fetching only. Real-time subscriptions are a separate unit.
- **Agent management pages**: Agent CRUD, cognitive layer inspector, chat interface. These depend on agent endpoints not yet in the backend.
- **Advanced data visualization**: Charts, graphs, and complex visualizations. Basic tables and lists only.
- **Internationalization (i18n)**: English only. Structure strings for future extraction.
- **PWA/offline**: No service worker, no offline mode.
- **End-to-end testing**: Playwright tests are a separate unit.
- **OAuth/SSO**: Not yet in the backend. Only email/password and magic link auth.