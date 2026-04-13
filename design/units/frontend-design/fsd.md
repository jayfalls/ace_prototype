# FSD: Frontend Design

[unit: frontend-design]

## 1. File Tree Structure

```
frontend/src/
├── app.html                              # HTML shell with theme pre-load script
├── app.css                               # Tailwind v4 directives + base theme tokens
├── app.d.ts                              # Global type declarations
│
├── routes/
│   ├── +layout.svelte                    # Root layout (theme init, auth store init)
│   ├── +page.svelte                       # Root redirect (/ → /login or /dashboard)
│   ├── (auth)/
│   │   ├── +layout.svelte                # Centered card layout, no sidebar, auth guard redirect
│   │   ├── login/+page.svelte            # Login form page
│   │   ├── register/+page.svelte         # Registration form page
│   │   ├── forgot-password/+page.svelte   # Password reset request page
│   │   ├── reset-password/+page.svelte   # Password reset confirm page (token from URL)
│   │   └── magic-link/+page.svelte       # Magic link verification page (token from URL)
│   ├── (app)/
│   │   ├── +layout.ts                    # Export: `export const ssr = false`
│   │   ├── +layout.svelte                # App shell layout (auth guard, sidebar, header)
│   │   ├── +page.svelte                  # Dashboard overview
│   │   ├── profile/+page.svelte          # User profile & session management
│   │   ├── admin/
│   │   │   └── users/
│   │   │       ├── +page.svelte          # User list (paginated table)
│   │   │       └── [id]/+page.svelte     # User detail (suspend, restore, role change)
│   │   ├── telemetry/
│   │   │   ├── +page.svelte              # Telemetry overview (health cards)
│   │   │   ├── spans/+page.svelte        # Span browser (filterable table)
│   │   │   ├── metrics/+page.svelte      # Metrics dashboard (aggregated view)
│   │   │   └── usage/+page.svelte        # Usage & cost analysis (filterable table)
│   │   └── settings/+page.svelte         # App settings (theme selector, preferences)
│   └── (errors)/
│       ├── +layout.svelte                 # Error page layout (centered, minimal)
│       └── +page.svelte                   # 404 fallback
│
├── lib/
│   ├── api/
│   │   ├── client.ts                     # Base fetch wrapper (auth headers, refresh, error normalization)
│   │   ├── types.ts                      # TypeScript interfaces matching backend models
│   │   ├── auth.ts                       # login, register, logout, refresh, me
│   │   ├── sessions.ts                   # listSessions, revokeSession
│   │   ├── admin.ts                      # listUsers, getUser, updateRole, suspendUser, restoreUser
│   │   └── telemetry.ts                  # spans, metrics, usage, health
│   │
│   ├── stores/
│   │   ├── auth.svelte.ts                # AuthStore: user, tokens, isAuthenticated, login/logout/refresh
│   │   ├── ui.svelte.ts                  # UIStore: theme, sidebarCollapsed, notifications
│   │   └── notifications.svelte.ts       # NotificationStore: toasts, alerts
│   │
│   ├── components/
│   │   ├── ui/                           # Primitive components (Bits UI + Tailwind)
│   │   │   ├── button/
│   │   │   │   ├── Button.svelte
│   │   │   │   └── index.ts
│   │   │   ├── input/
│   │   │   │   ├── Input.svelte
│   │   │   │   └── index.ts
│   │   │   ├── dialog/
│   │   │   │   ├── Dialog.svelte
│   │   │   │   ├── DialogContent.svelte
│   │   │   │   ├── DialogOverlay.svelte
│   │   │   │   ├── DialogTrigger.svelte
│   │   │   │   └── index.ts
│   │   │   ├── select/
│   │   │   │   ├── Select.svelte
│   │   │   │   └── index.ts
│   │   │   ├── table/
│   │   │   │   ├── Table.svelte
│   │   │   │   ├── TableRow.svelte
│   │   │   │   ├── TableHeader.svelte
│   │   │   │   ├── TableCell.svelte
│   │   │   │   └── index.ts
│   │   │   ├── badge/
│   │   │   │   ├── Badge.svelte
│   │   │   │   └── index.ts
│   │   │   ├── avatar/
│   │   │   │   ├── Avatar.svelte
│   │   │   │   └── index.ts
│   │   │   ├── dropdown-menu/
│   │   │   │   ├── DropdownMenu.svelte
│   │   │   │   ├── DropdownMenuContent.svelte
│   │   │   │   ├── DropdownMenuItem.svelte
│   │   │   │   ├── DropdownMenuTrigger.svelte
│   │   │   │   └── index.ts
│   │   │   ├── toast/
│   │   │   │   ├── Toast.svelte
│   │   │   │   ├── Toaster.svelte
│   │   │   │   └── index.ts
│   │   │   ├── card/
│   │   │   │   ├── Card.svelte
│   │   │   │   ├── CardHeader.svelte
│   │   │   │   ├── CardContent.svelte
│   │   │   │   ├── CardFooter.svelte
│   │   │   │   └── index.ts
│   │   │   ├── tabs/
│   │   │   │   ├── Tabs.svelte
│   │   │   │   ├── TabsList.svelte
│   │   │   │   ├── TabsTrigger.svelte
│   │   │   │   ├── TabsContent.svelte
│   │   │   │   └── index.ts
│   │   │   ├── skeleton/
│   │   │   │   ├── Skeleton.svelte
│   │   │   │   └── index.ts
│   │   │   └── separator/
│   │   │       ├── Separator.svelte
│   │   │       └── index.ts
│   │   ├── layout/
│   │   │   ├── AppShell.svelte          # Sidebar + Header + Content slot
│   │   │   ├── Sidebar.svelte           # Collapsible nav sidebar
│   │   │   ├── Header.svelte            # Breadcrumbs, user menu, mobile toggle
│   │   │   ├── NavItem.svelte           # Sidebar nav link (icon + label + badge)
│   │   │   ├── UserMenu.svelte          # Dropdown: profile, sessions, logout
│   │   │   └── Breadcrumbs.svelte       # Auto-generated from route
│   │   ├── auth/
│   │   │   ├── LoginForm.svelte          # Email + password + submit + links
│   │   │   ├── RegisterForm.svelte       # Email + password + confirm + submit
│   │   │   ├── ForgotPasswordForm.svelte # Email + submit
│   │   │   ├── ResetPasswordForm.svelte  # New password + confirm + submit
│   │   │   └── MagicLinkVerifier.svelte  # Token verification + result display
│   │   ├── admin/
│   │   │   ├── UserTable.svelte          # Paginated user list with status filter
│   │   │   ├── UserDetailCard.svelte     # User info card with actions
│   │   │   ├── SuspendDialog.svelte      # Confirmation dialog with reason input
│   │   │   └── RoleSelect.svelte         # Role dropdown (admin/user/viewer)
│   │   ├── telemetry/
│   │   │   ├── HealthCards.svelte        # System health status cards
│   │   │   ├── SpansTable.svelte         # Filterable span list with detail expand
│   │   │   ├── MetricsList.svelte        # Aggregated metric cards/list
│   │   │   └── UsageTable.svelte         # Usage events table with cost summary
│   │   └── shared/
│   │       ├── DataState.svelte          # Loading/Error/Empty state wrapper
│   │       ├── Pagination.svelte         # Page number navigation
│   │       ├── ConfirmDialog.svelte       # Confirmation dialog with message
│   │       └── SearchInput.svelte         # Debounced search input
│   │
│   ├── themes/
│   │   ├── one-dark.css                  # One Dark preset (dark variant)
│   │   ├── one-light.css                 # One Light preset (light variant)
│   │   ├── catppuccin-mocha.css          # Catppuccin Mocha preset (dark)
│   │   ├── catppuccin-latte.css           # Catppuccin Latte preset (light)
│   │   ├── nord.css                       # Nord preset (dark)
│   │   ├── monokai.css                   # Monokai preset (dark)
│   │   └── index.ts                      # Theme registry, type definitions, apply/persist logic
│   │
│   ├── validation/
│   │   └── schemas.ts                    # Zod validation schemas for all forms
│   │
│   ├── utils/
│   │   ├── cn.ts                          # clsx + tailwind-merge class merger
│   │   ├── form.svelte.ts                # useForm composable (runes + Zod)
│   │   ├── formatter.ts                   # Date, duration, cost formatting utilities
│   │   └── constants.ts                   # Route paths, breakpoints, defaults
│   │
│   └── telemetry/                         # (existing) OpenTelemetry client
│       ├── index.ts
│       ├── error.ts
│       ├── metrics.ts
│       └── trace.ts
│
└── test/
    ├── setup.ts                           # Vitest setup (jsdom, globals)
    └── integration/
        ├── auth.test.ts                  # Login/logout/refresh flow integration tests
        └── admin.test.ts                 # Admin user management flow integration tests
```

---

## 2. Component Inventory

### 2.1 UI Primitives (`$lib/components/ui/`)

Every primitive wraps Bits UI for accessibility and adds Tailwind styling. Each follows the pattern: directory → component file → barrel export (`index.ts`). Props include `class` for extension.

| Component | Bits UI Base | Key Props | Events | Notes |
|-----------|-------------|-----------|--------|-------|
| **Button** | N/A (native) | `variant`, `size`, `disabled`, `loading`, `type` | `click` | `variant`: primary, secondary, destructive, ghost, outline. `size`: sm, md, lg. Loading shows spinner and disables. |
| **Input** | N/A (native) | `type`, `placeholder`, `value`, `disabled`, `error`, `id` | `input`, `blur`, `focus` | Wraps native `<input>` with label, error message slot, and focus ring styling |
| **Dialog** | Bits UI Dialog | `open` (bindable), `class` | N/A | Composite: Dialog, DialogContent, DialogOverlay, DialogTrigger. Focus trap, Escape dismiss, portal rendering |
| **Select** | Bits UI Select | `value` (bindable), `options`, `placeholder`, `disabled` | `change` | Dropdown with keyboard navigation. Option type: `{ value: string, label: string }` |
| **Table** | N/A (native) | Composable sub-components | N/A | Table, TableRow, TableHeader, TableCell. Responsive: horizontal scroll on mobile |
| **Badge** | N/A (native) | `variant`, `size` | N/A | `variant`: default, success, warning, error, info. Roles statuses |
| **Avatar** | Bits UI Avatar | `src`, `alt`, `fallback`, `size` | N/A | Fallback shows initials. `size`: sm, md, lg |
| **DropdownMenu** | Bits UI Dropdown | N/A | N/A | Composite: DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger. Arrow key nav |
| **Toast** | N/A (custom) | `variant`, `title`, `description`, `duration` | `dismiss` | Primitive. Toast container managed by NotificationStore. `variant`: success, error, warning, info |
| **Toaster** | N/A (custom) | N/A | N/A | Renders toast stack with auto-dismiss. Reads from NotificationStore |
| **Card** | N/A (native) | N/A | N/A | Composite: Card, CardHeader, CardContent, CardFooter. Layout primitive |
| **Tabs** | Bits UI Tabs | `value` (bindable) | `change` | Composite: Tabs, TabsList, TabsTrigger, TabsContent |
| **Skeleton** | N/A (native) | `variant`, `class` | N/A | `variant`: text, circle, rect, table. Pulse animation |
| **Separator** | Bits UI Separator | `orientation`, `decorative` | N/A | Horizontal or vertical divider |

### 2.2 Layout Components (`$lib/components/layout/`)

| Component | Purpose | Key Props | State Sources | Notes |
|-----------|---------|-----------|---------------|-------|
| **AppShell** | Authenticated layout frame: sidebar + header + content | N/A | `authStore.user`, `uiStore.sidebarCollapsed` | Renders Sidebar, Header, `<main>` with `<slot>`. Skip-to-content link. ARIA landmarks |
| **Sidebar** | Collapsible navigation sidebar | N/A | `uiStore.sidebarCollapsed` | 256px expanded, 64px collapsed. Logo, NavItems, settings at bottom. Overlay mode on mobile |
| **Header** | Top bar with breadcrumbs, user menu, mobile toggle | N/A | `authStore.user` | Hamburger on mobile, breadcrumb path, UserMenu dropdown |
| **NavItem** | Individual sidebar navigation item | `icon`, `label`, `href`, `badge?` | N/A | Uses `aria-current="page"` on active route. Icon from lucide-svelte |
| **UserMenu** | User avatar dropdown menu | N/A | `authStore.user` | Profile link, sessions link, logout action. Uses DropdownMenu primitive |
| **Breadcrumbs** | Auto-generated breadcrumb from current route | N/A | `$page.url` | Parses route segments to labels. Last segment is current page (non-link) |

### 2.3 Auth Components (`$lib/components/auth/`)

| Component | Purpose | Form Fields | API Call | Validation Schema |
|-----------|---------|-------------|----------|-------------------|
| **LoginForm** | Email + password login | email, password | `authApi.login()` | `loginSchema` |
| **RegisterForm** | New account creation | email, password, confirmPassword | `authApi.register()` | `registerSchema` |
| **ForgotPasswordForm** | Request password reset email | email | `authApi.resetPasswordRequest()` | `forgotPasswordSchema` |
| **ResetPasswordForm** | Confirm password reset | newPassword, confirmPassword | `authApi.resetPasswordConfirm(token)` | `resetPasswordSchema` |
| **MagicLinkVerifier** | Verify magic link token from URL | N/A | `authApi.magicLinkVerify(token)` | N/A (no form) |

### 2.4 Admin Components (`$lib/components/admin/`)

| Component | Purpose | Key Props | Actions | Notes |
|-----------|---------|-----------|---------|-------|
| **UserTable** | Paginated user list with filters | `users: UserListItem[]`, `total`, `page`, `limit`, `onPageChange`, `onStatusFilter` | View, Suspend, Restore | Responsive: card layout on mobile |
| **UserDetailCard** | Full user detail display | `user: AdminUserResponse` | Change Role, Suspend, Restore | Shows suspended_at and suspended_reason if present |
| **SuspendDialog** | Confirmation dialog with reason input | `open`, `userId`, `onConfirm` | Confirm suspension | Uses Dialog primitive. Optional reason textarea, max 500 chars |
| **RoleSelect** | Role selection dropdown | `value`, `onchange` | Select role | Options: admin, user, viewer. Admin-only action |

### 2.5 Telemetry Components (`$lib/components/telemetry/`)

| Component | Purpose | Key Props | Data Source | Notes |
|-----------|---------|-----------|-------------|-------|
| **HealthCards** | System health status grid | N/A (fetches own data) | `telemetryApi.health()` | Cards: database, messaging (NATS), cache, telemetry. Status indicator + counts |
| **SpansTable** | Filterable span browser | `spans`, `total`, `filters`, `onFilterChange`, `onPageChange` | `telemetryApi.spans()` | Filters: service, operation, status, time range. Expandable rows for attributes |
| **MetricsList** | Aggregated metrics display | `metrics`, `total` | `telemetryApi.metrics()` | Filter: service, name, time window. Value + timestamp + labels |
| **UsageTable** | Usage events with cost | `events`, `total`, `filters` | `telemetryApi.usage()` | Filters: agent_id, event_type, resource_type, time range. Cost summary footer |

### 2.6 Shared Components (`$lib/components/shared/`)

| Component | Purpose | Key Props | Notes |
|-----------|---------|-----------|-------|
| **DataState** | Loading/Error/Empty wrapper | `loading`, `error`, `empty`, `onRetry` | Renders slot on success. Shows Skeleton on loading, error message on error, empty state on empty |
| **Pagination** | Page navigation | `page`, `total`, `limit`, `onPageChange` | Shows page numbers, prev/next, total count |
| **ConfirmDialog** | Confirmation before destructive action | `open`, `title`, `message`, `onConfirm`, `variant` | Uses Dialog primitive. `variant`: danger (destructive actions) |
| **SearchInput** | Debounced search input | `value`, `placeholder`, `onSearch`, `debounceMs` | Emits search after configurable debounce delay |

---

## 3. Page Specifications

### 3.1 Auth Pages

#### `/login` — Login Page
- **Layout**: `(auth)` group — centered card, no sidebar
- **Auth guard**: Redirects to `/` if already authenticated
- **Component**: `LoginForm`
- **State**: `{ email: '', password: '' }`
- **Flow**: Submit → `authStore.login(email, password)` → on success: `goto('/')`, on 401: show "Invalid email or password"
- **Links**: "Forgot password?" → `/forgot-password`, "Create account" → `/register`, "Sign in with magic link" → `/magic-link`

#### `/register` — Registration Page
- **Layout**: `(auth)` group
- **Auth guard**: Redirects to `/` if already authenticated
- **Component**: `RegisterForm`
- **State**: `{ email: '', password: '', confirmPassword: '' }`
- **Flow**: Submit → `authApi.register(email, password)` → on 201: auto-login, `goto('/')`, on 409: show "Account with this email already exists"
- **Links**: "Already have an account?" → `/login`

#### `/forgot-password` — Password Reset Request
- **Layout**: `(auth)` group
- **Auth guard**: Redirects to `/` if already authenticated
- **Component**: `ForgotPasswordForm`
- **State**: `{ email: '' }`
- **Flow**: Submit → `authApi.resetPasswordRequest(email)` → always show "If an account exists, a reset link has been sent" (prevents email enumeration)
- **Links**: "Back to login" → `/login`

#### `/reset-password` — Password Reset Confirm
- **Layout**: `(auth)` group
- **Auth guard**: No redirect (token-based, user may not be logged in)
- **Component**: `ResetPasswordForm`
- **State**: `{ newPassword: '', confirmPassword: '' }`, token extracted from `URL.searchParams.get('token')`
- **Flow**: Submit → `authApi.resetPasswordConfirm(token, newPassword)` → on success: auto-login with returned tokens, `goto('/')`, on 400: show "This reset link has expired or is invalid"

#### `/magic-link` — Magic Link Verification
- **Layout**: `(auth)` group
- **Auth guard**: No redirect (token-based)
- **Component**: `MagicLinkVerifier`
- **State**: No form — triggers verification immediately on mount
- **Flow**: On mount → `authApi.magicLinkVerify(token)` → on success: store tokens, `goto('/')`, on failure: show "This link has expired or is invalid" with link to `/login`

### 3.2 Dashboard Page

#### `/` — Dashboard Overview
- **Layout**: `(app)` group — sidebar + header
- **Auth guard**: Redirects to `/login` if not authenticated
- **Components**: `HealthCards`, quick-action buttons
- **Data**: `GET /telemetry/health` and `GET /health/ready`
- **Content**:
  - Welcome message: "Welcome back, {user.email}"
  - System health cards: database status, NATS status, cache status, telemetry counts
  - Quick actions: "View Telemetry" button, "Manage Users" button (admin only), "View Profile" button
- **State machine**: `Initial → Loading → Success | Error`

### 3.3 Profile Page

#### `/profile` — User Profile & Sessions
- **Layout**: `(app)` group
- **Auth guard**: Yes
- **Data sources**: `GET /auth/me` (user profile), `GET /auth/me/sessions` (session list)
- **Content**:
  - User profile card: email, role, status, created_at
  - Active sessions table: user_agent (parsed), ip_address, last_used_at, created_at, expires_at, "Revoke" button per row
  - "Sign out all other sessions" button
  - "Change password" link → `/forgot-password`
- **Actions**: Revoke session → `DELETE /auth/me/sessions/{id}` with optimistic update

### 3.4 Admin Pages

#### `/admin/users` — User List
- **Layout**: `(app)` group, admin-only
- **Auth guard**: Redirects non-admin to 403 page
- **Components**: `UserTable`, `SearchInput`
- **Data**: `GET /admin/users?page={page}&limit={limit}&status={status}`
- **State**: `{ page: 1, limit: 20, statusFilter: 'all', searchQuery: '' }`
- **Content**:
  - Status filter: All, Active, Pending, Suspended (dropdown)
  - User table columns: Email, Role, Status, Created, Actions
  - Actions per row: View → `/admin/users/{id}`, Suspend (for active), Restore (for suspended)
  - Pagination controls
- **State machine**: `Initial → Loading → Success | Error`

#### `/admin/users/[id]` — User Detail
- **Layout**: `(app)` group, admin-only
- **Auth guard**: Redirects non-admin to 403 page
- **Components**: `UserDetailCard`, `SuspendDialog`, `RoleSelect`
- **Data**: `GET /admin/users/{id}`
- **Content**:
  - User info: id, email, role, status, created_at, updated_at
  - If suspended: `suspended_at`, `suspended_reason`
  - Role change dropdown: admin/user/viewer → `PUT /admin/users/{id}/role`
  - Suspend button → `SuspendDialog` → `POST /admin/users/{id}/suspend { reason }`
  - Restore button → `POST /admin/users/{id}/restore`
  - "Back to users" link
- **Actions**: Role change (optimistic), Suspend (with confirmation dialog), Restore (with confirmation)

### 3.5 Telemetry Pages

#### `/telemetry` — Overview
- **Layout**: `(app)` group
- **Auth guard**: Yes (all authenticated users)
- **Components**: `HealthCards`
- **Data**: `GET /telemetry/health`
- **Content**: Health cards showing database, messaging, cache, telemetry subsystem status. Counts: spans last hour, metrics last hour. Quick links to sub-pages

#### `/telemetry/spans` — Span Browser
- **Layout**: `(app)` group
- **Components**: `SpansTable`, `DataState`, `Pagination`
- **Data**: `GET /telemetry/spans?service=&operation=&status=&start_time=&end_time=&limit=50&offset=0`
- **Filters**: service (text), operation (text), status (ok/error dropdown), time range (start_time, end_time as RFC3339)
- **State**: `{ service: '', operation: '', status: '', timeRange: '24h', offset: 0, limit: 50 }`
- **Content**: Table with columns: Trace ID, Span ID, Operation, Service, Duration (ms), Status, Timestamp. Expandable rows showing attributes JSON

#### `/telemetry/metrics` — Metrics Dashboard
- **Layout**: `(app)` group
- **Components**: `MetricsList`, `DataState`
- **Data**: `GET /telemetry/metrics?name=&window=1h&limit=50`
- **Filters**: name (text), window (5m/15m/1h/6h/24h dropdown)
- **State**: `{ name: '', window: '1h', limit: 50 }`
- **Content**: Metric cards showing name, value, type, timestamp, labels

#### `/telemetry/usage` — Usage & Cost
- **Layout**: `(app)` group
- **Components**: `UsageTable`, `DataState`, `Pagination`
- **Data**: `GET /telemetry/usage?agent_id=&event_type=&from=&to=&limit=100&offset=0`
- **Filters**: agent_id (text), event_type (dropdown: llm_call, memory_read, memory_write, tool_execute, db_query), resource_type (dropdown: api, memory, tool, database, messaging), from/to (time range)
- **State**: `{ agentId: '', eventType: '', resourceType: '', from: '7d ago', to: 'now', limit: 100, offset: 0 }`
- **Content**: Table columns: Agent ID, Operation, Resource, Duration (ms), Cost (USD), Timestamp. Summary row showing total cost in period

### 3.6 Settings Page

#### `/settings` — App Settings
- **Layout**: `(app)` group
- **Auth guard**: Yes
- **Data**: None (local state only)
- **Content**:
  - Theme selector: grid of theme preset cards (One Dark, One Light, Catppuccin Mocha, Catppuccin Latte, Nord, Monokai). Active theme highlighted.
  - Dark/light mode toggle: switch between dark and light variants of current preset
  - Sidebar default: expanded/collapsed preference (persisted in localStorage)
- **State**: All from `uiStore` — `theme`, `mode`, `sidebarCollapsed`

### 3.7 Error Page

#### `(errors)/+page.svelte` — 404 Fallback
- **Layout**: `(errors)` group — centered message
- **Content**: "Page not found" message with link back to `/`
- **Auth guard**: No

---

## 4. API Client Structure

### 4.1 Base Client (`$lib/api/client.ts`)

The `APIClient` class is the single entry point for all backend communication. It handles auth token injection, proactive/reactive refresh, and error normalization.

```typescript
class APIClient {
  private baseUrl: string;
  private refreshPromise: Promise<void> | null = null;

  constructor() {
    this.baseUrl = import.meta.env.VITE_API_URL || '';
  }

  // Core typed request — all domain modules call this
  async request<T>(options: RequestOptions): Promise<T>;

  // Private: token management
  private getAccessToken(): string | null;
  private setTokens(access: string, refresh: string, expiresIn: number): void;
  private clearTokens(): void;

  // Private: proactive refresh (<30s before expiry)
  private async ensureValidToken(): Promise<void>;

  // Private: reactive refresh (on 401 response)
  private async handleUnauthorized(): Promise<void>;

  // Private: error code → user message mapping
  private normalizeError(response: Response): APIError;
}

export const apiClient = new APIClient();
```

**Request lifecycle:**
1. `ensureValidToken()` — if `accessToken` expires in <30s, refresh first
2. Attach `Authorization: Bearer <accessToken>` header
3. Send request
4. On 401 → `handleUnauthorized()` (refresh mutex) → retry original
5. On success → unwrap `{ success, data }` envelope, return typed `T`
6. On error → throw `APIError` with code + message + field details

**Refresh mutex:** Single in-flight refresh promise. Concurrent 401s await the same promise. On refresh failure: clear auth store, redirect to `/login`.

### 4.2 Request/Response Types (`$lib/api/types.ts`)

All TypeScript interfaces mirroring the backend Go structs and JSON envelope:

```typescript
// --- API Envelope ---
interface APIEnvelope<T> {
  success: boolean;
  data?: T;
  error?: APIError;
}

interface APIError {
  code: string;
  message: string;
  details?: FieldError[];
}

interface FieldError {
  field: string;
  message: string;
}

// --- Pagination ---
interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}

// --- Auth ---
interface LoginRequest {
  email: string;
  password: string;
}

interface RegisterRequest {
  email: string;
  password: string;
}

interface TokenResponse {
  access_token: string;
  refresh_token: string;
  user: User;
  expires_in: number;
}

interface RefreshRequest {
  refresh_token: string;
}

interface ResetPasswordRequest {
  token: string;
  new_password: string;
}

interface MagicLinkVerifyRequest {
  token: string;
}

// --- User (matches backend model.User) ---
type UserRole = 'admin' | 'user' | 'viewer';
type UserStatus = 'pending' | 'active' | 'suspended';

interface User {
  id: string;
  email: string;
  role: UserRole;
  status: UserStatus;
  suspended_at?: string;
  suspended_reason?: string;
  created_at: string;
  updated_at: string;
}

// --- User List Item (matches backend UserListItem) ---
interface UserListItem {
  id: string;
  email: string;
  role: UserRole;
  status: UserStatus;
  created_at: string;
  updated_at: string;
}

// --- Admin User Response (matches backend AdminUserResponse) ---
interface AdminUserResponse {
  id: string;
  email: string;
  role: UserRole;
  status: UserStatus;
  suspended_at?: string;
  suspended_reason?: string;
  created_at: string;
  updated_at: string;
}

// --- Sessions (matches backend SessionResponse) ---
interface Session {
  id: string;
  user_id: string;
  user_agent?: string;
  ip_address?: string;
  last_used_at: string;
  expires_at: string;
  created_at: string;
}

interface SessionsListResponse {
  sessions: Session[];
  total: number;
  page: number;
  limit: number;
}

// --- Admin (matches backend UsersListResponse) ---
interface UsersListResponse {
  users: UserListItem[];
  total: number;
  page: number;
  limit: number;
}

// --- Telemetry: Spans (matches backend SpanResponse) ---
interface Span {
  trace_id: string;
  span_id: string;
  operation: string;
  service: string;
  start_time: string;
  end_time: string;
  duration_ms: number;
  status: string;
  attributes?: Record<string, unknown>;
}

interface SpansResponse {
  spans: Span[];
  total: number;
  limit: number;
  offset: number;
}

// --- Telemetry: Metrics (matches backend MetricResponse) ---
interface Metric {
  name: string;
  type: string;
  labels?: Record<string, string>;
  value: number;
  timestamp: string;
  window?: string;
}

interface MetricsResponse {
  metrics: Metric[];
  total: number;
  limit: number;
}

// --- Telemetry: Usage (matches backend UsageEventResponse) ---
interface UsageEvent {
  id: string;
  agent_id: string;
  session_id: string;
  event_type: string;
  model?: string;
  input_tokens?: number;
  output_tokens?: number;
  cost_usd?: number;
  duration_ms?: number;
  timestamp: string;
}

interface UsageResponse {
  events: UsageEvent[];
  total: number;
  limit: number;
  offset: number;
}

// --- Telemetry: Health (matches backend TelemetryHealthResponse) ---
type HealthStatus = 'healthy' | 'degraded' | 'error';

interface SubsystemCheck {
  status: string;
  mode?: string;
  path?: string;
  size_bytes?: number;
  connections?: number;
  max_cost_bytes?: number;
  current_cost_bytes?: number;
  hit_rate?: number;
  spans_last_hour?: number;
  metrics_last_hour?: number;
  error?: string;
}

interface TelemetryHealthResponse {
  status: HealthStatus;
  checks: Record<string, SubsystemCheck>;
}

// --- Health (matches backend HealthStatus) ---
interface SystemHealthCheck {
  status: string;
  reason?: string;
}

interface SystemHealthResponse {
  status: string;
  checks: Record<string, SystemHealthCheck>;
}
```

### 4.3 Domain Modules

#### `$lib/api/auth.ts`

```typescript
export async function login(email: string, password: string): Promise<TokenResponse>;
export async function register(email: string, password: string): Promise<TokenResponse>;
export async function logout(sessionId: string): Promise<void>;
export async function refresh(refreshToken: string): Promise<TokenResponse>;
export async function me(): Promise<User>;
export async function resetPasswordRequest(email: string): Promise<void>;
export async function resetPasswordConfirm(token: string, newPassword: string): Promise<TokenResponse>;
export async function magicLinkRequest(email: string): Promise<void>;
export async function magicLinkVerify(token: string): Promise<TokenResponse>;
```

| Function | HTTP | Endpoint | Auth | Response Type |
|----------|------|----------|------|----------------|
| `login` | POST | `/auth/login` | No | `TokenResponse` |
| `register` | POST | `/auth/register` | No | `TokenResponse` |
| `logout` | POST | `/auth/logout` | Yes | `void` |
| `refresh` | POST | `/auth/refresh` | No | `TokenResponse` |
| `me` | GET | `/auth/me` | Yes | `User` |
| `resetPasswordRequest` | POST | `/auth/password/reset/request` | No | `void` |
| `resetPasswordConfirm` | POST | `/auth/password/reset/confirm` | No | `TokenResponse` |
| `magicLinkRequest` | POST | `/auth/magic-link/request` | No | `void` |
| `magicLinkVerify` | POST | `/auth/magic-link/verify` | No | `TokenResponse` |

#### `$lib/api/sessions.ts`

```typescript
export async function listSessions(page?: number, limit?: number): Promise<SessionsListResponse>;
export async function revokeSession(sessionId: string): Promise<void>;
```

| Function | HTTP | Endpoint | Auth | Response Type |
|----------|------|----------|------|----------------|
| `listSessions` | GET | `/auth/me/sessions?page={p}&limit={l}` | Yes | `SessionsListResponse` |
| `revokeSession` | DELETE | `/auth/me/sessions/{id}` | Yes | `void` |

#### `$lib/api/admin.ts`

```typescript
export async function listUsers(page?: number, limit?: number, status?: string): Promise<UsersListResponse>;
export async function getUser(id: string): Promise<AdminUserResponse>;
export async function updateUserRole(id: string, role: UserRole): Promise<AdminUserResponse>;
export async function suspendUser(id: string, reason: string): Promise<AdminUserResponse>;
export async function restoreUser(id: string): Promise<AdminUserResponse>;
```

| Function | HTTP | Endpoint | Auth | Role | Response Type |
|----------|------|----------|------|------|----------------|
| `listUsers` | GET | `/admin/users?page={p}&limit={l}&status={s}` | Yes | Admin | `UsersListResponse` |
| `getUser` | GET | `/admin/users/{id}` | Yes | Admin | `AdminUserResponse` |
| `updateUserRole` | PUT | `/admin/users/{id}/role` | Yes | Admin | `AdminUserResponse` |
| `suspendUser` | POST | `/admin/users/{id}/suspend` | Yes | Admin | `AdminUserResponse` |
| `restoreUser` | POST | `/admin/users/{id}/restore` | Yes | Admin | `AdminUserResponse` |

#### `$lib/api/telemetry.ts`

```typescript
export async function spans(params?: SpanQueryParams): Promise<SpansResponse>;
export async function metrics(params?: MetricQueryParams): Promise<MetricsResponse>;
export async function usage(params?: UsageQueryParams): Promise<UsageResponse>;
export async function health(): Promise<TelemetryHealthResponse>;
```

| Function | HTTP | Endpoint | Auth | Query Params | Response Type |
|----------|------|----------|------|-------------|----------------|
| `spans` | GET | `/telemetry/spans` | Yes | service, operation, status, start_time, end_time, limit, offset | `SpansResponse` |
| `metrics` | GET | `/telemetry/metrics` | Yes | name, window (5m/15m/1h/6h/24h), limit | `MetricsResponse` |
| `usage` | GET | `/telemetry/usage` | Yes | agent_id, event_type, from, to, limit, offset | `UsageResponse` |
| `health` | GET | `/telemetry/health` | Yes | None | `TelemetryHealthResponse` |

**Query param interfaces:**

```typescript
interface SpanQueryParams {
  service?: string;
  operation?: string;
  status?: string;
  start_time?: string; // RFC3339
  end_time?: string;   // RFC3339
  limit?: number;      // default 50, max 1000
  offset?: number;     // default 0
}

interface MetricQueryParams {
  name?: string;
  window?: '5m' | '15m' | '1h' | '6h' | '24h'; // default 1h
  limit?: number; // default 50, max 200
}

interface UsageQueryParams {
  agent_id?: string;
  event_type?: string;
  from?: string;    // RFC3339
  to?: string;      // RFC3339
  limit?: number;   // default 100, max 500
  offset?: number;  // default 0
}
```

### 4.4 Error Handling Map

Every API response flows through `client.ts`'s error normalization:

| HTTP Status | Error Code | Action |
|-------------|-----------|--------|
| 401 | `unauthorized` | Trigger refresh → retry. On refresh failure → clear auth, redirect `/login` |
| 403 | `forbidden` | Show "You do not have permission" message, redirect to `/` |
| 404 | `not_found` | Show 404 page |
| 400 | `validation_error` | Map `details` field errors to form fields |
| 400 | `invalid_request` | Show toast with error message |
| 409 | `user_already_exists` | Show "An account with this email already exists" |
| 429 | `rate_limit_exceeded` | Show "Too many requests. Please wait." toast |
| 500 | `internal_error` | Show "Something went wrong. Please try again." toast |
| Network error | N/A | Show "Unable to connect. Please check your connection." toast |