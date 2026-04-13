# Architecture: Frontend Design

[unit: frontend-design]

## 1. Architectural Overview

The ACE frontend is a **SvelteKit SPA** served as static assets from the Go binary. It consumes the backend REST API via a typed fetch wrapper with JWT auth. The architecture follows a strict unidirectional data flow: API client → rune stores → Svelte components.

```
┌──────────────────────────────────────────────────────────────────┐
│                        Browser / DOM                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │  Auth    │  │  Agents  │  │   UI     │  │  Notifications   │  │
│  │  Store   │  │  Store   │  │  Store   │  │  Store           │  │
│  │ (runes)  │  │ (runes)  │  │ (runes)  │  │  (runes)         │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘  │
│       │              │              │                  │            │
│  ┌────▼──────────────▼──────────────▼──────────────────▼─────────┐ │
│  │                    Svelte Components                          │ │
│  │  Pages → Organisms → Molecules → Atoms                      │ │
│  └────────────────────────┬──────────────────────────────────────┘ │
│                           │                                       │
│  ┌────────────────────────▼──────────────────────────────────────┐ │
│  │                    API Client Layer                           │ │
│  │  client.ts → auth.ts → agents.ts → sessions.ts → ...         │ │
│  └────────────────────────┬──────────────────────────────────────┘ │
│                           │                                       │
└───────────────────────────┼───────────────────────────────────────┘
                            │  HTTP (Bearer JWT)
                            ▼
                ┌──────────────────────┐
                │   Go Backend API     │
                │   (Chi Router)       │
                └──────────────────────┘
```

### Key Constraints

1. **Single-binary deployment**: Frontend builds to static files embedded in Go binary via `embed.FS`. No Node.js server in production.
2. **SPA mode only**: `adapter-static` with `fallback: 'index.html'`. No SSR, no hydration mismatches.
3. **Svelte 5 runes exclusively**: `$state`, `$derived`, `$effect`, `$props`. No legacy `writable`/`readable` stores.
4. **Backend owns auth**: JWT Bearer tokens. Frontend stores, refreshes, and attaches them. No server-side sessions in SvelteKit.

---

## 2. Layer Architecture

The frontend has four layers, each with a single responsibility. Data flows down (stores → components), events flow up (components → stores → API calls). No component makes direct API calls — stores orchestrate all async operations.

### Layer 1: API Client (`$lib/api/`)

Type-safe HTTP wrapper. Single entry point for all backend communication. Handles auth token injection, refresh logic, and error normalization.

```
$lib/api/
├── client.ts          # Base fetch wrapper: auth headers, refresh, error unwrap
├── types.ts           # TypeScript interfaces matching backend models
├── auth.ts            # login, register, logout, refresh, me
├── sessions.ts        # listSessions, revokeSession
├── admin.ts           # listUsers, getUser, updateRole, suspend, restore
└── telemetry.ts       # spans, metrics, usage, health
```

**Client behavior:**
- `client.ts` exports a `fetchApi<T>(method, path, body?)` function
- Attaches `Authorization: Bearer <token>` from auth store on every request
- On 401: attempt refresh → retry original request → on failure, clear auth, redirect to `/login`
- Refresh mutex: single in-flight refresh promise. Concurrent 401s await the same refresh
- Proactive refresh: if token expires in <30s, refresh before making the request
- All responses unwrapped from the `{ success, data, error }` envelope

### Layer 2: Stores (`$lib/stores/`)

Svelte 5 rune-based class stores. Each domain has one store file. Stores are the single source of truth for their domain's state. Components read from stores; stores read from the API client.

```
$lib/stores/
├── auth.svelte.ts     # AuthStore: user, tokens, isAuthenticated, login/logout
├── ui.svelte.ts       # UIStore: sidebar state, theme, active notifications
└── notifications.svelte.ts  # NotificationStore: toasts, alerts
```

**Store rules:**
- Stores are classes with `$state` properties for reactive data and `$derived` for computed values
- Stores are instantiated once and exported as singletons
- Stores call API client methods; components never call API client directly
- Stores handle loading/error states for their domain
- Auth store loads tokens from localStorage on construction, persists on mutation

### Layer 3: Components (`$lib/components/`)

Svelte components following atomic design. Components own no data-fetching logic — they receive data via props or read from stores.

```
$lib/components/
├── ui/                # Primitives (Button, Input, Dialog, etc.)
│   ├── button/
│   ├── input/
│   ├── dialog/
│   ├── select/
│   ├── table/
│   ├── badge/
│   ├── avatar/
│   ├── dropdown-menu/
│   ├── toast/
│   ├── card/
│   ├── tabs/
│   └── skeleton/
├── layout/            # App shell components
│   ├── AppShell.svelte
│   ├── Sidebar.svelte
│   ├── Header.svelte
│   └── MobileNav.svelte
├── auth/              # Auth page components
│   ├── LoginForm.svelte
│   ├── RegisterForm.svelte
│   ├── ForgotPasswordForm.svelte
│   ├── ResetPasswordForm.svelte
│   └── MagicLinkVerifier.svelte
├── admin/             # Admin page components
│   ├── UserTable.svelte
│   ├── UserDetailCard.svelte
│   ├── SuspendDialog.svelte
│   └── RoleSelect.svelte
└── telemetry/         # Telemetry page components
    ├── HealthCards.svelte
    ├── SpansTable.svelte
    ├── MetricsList.svelte
    └── UsageTable.svelte
```

### Layer 4: Pages (`src/routes/`)

SvelteKit route pages. Thin orchestration layer that wires stores to components. Pages handle navigation and route params.

```
src/routes/
├── +layout.svelte              # Root layout (theme class, global providers)
├── (auth)/                     # Unauthenticated route group
│   ├── +layout.svelte          # Auth layout (centered card, no sidebar)
│   ├── login/+page.svelte
│   ├── register/+page.svelte
│   ├── forgot-password/+page.svelte
│   ├── reset-password/+page.svelte
│   └── magic-link/+page.svelte
├── (app)/                      # Authenticated route group
│   ├── +layout.svelte          # App layout (sidebar + header + content)
│   ├── +layout.ts              # ssr = false
│   ├── +page.svelte            # Dashboard
│   ├── profile/+page.svelte
│   ├── admin/
│   │   └── users/
│   │       ├── +page.svelte    # User list
│   │       └── [id]/+page.svelte # User detail
│   ├── telemetry/
│   │   ├── +page.svelte        # Overview
│   │   ├── spans/+page.svelte
│   │   ├── metrics/+page.svelte
│   │   └── usage/+page.svelte
│   └── settings/+page.svelte
└── (errors)/
    ├── +layout.svelte
    ├── 404/+page.svelte
    └── 500/+page.svelte
```

---

## 3. Auth Architecture

### 3.1 Token Flow

```
Login ──POST /auth/login──> { access_token, refresh_token, user, expires_in }
  │
  ├── Store access_token + refresh_token in localStorage
  ├── Load tokens into AuthStore (in-memory $state)
  └── Redirect to /

Every API Request ──> client.ts attaches Authorization: Bearer <access_token>

Token Expired (<30s) ──> Proactive refresh:
  ├── POST /auth/refresh { refresh_token }
  ├── New tokens stored in localStorage + AuthStore
  └── Original request proceeds with new token

Token Expired (401 response) ──> Reactive refresh:
  ├── Refresh mutex: single in-flight refresh promise
  ├── Concurrent 401s await same promise
  ├── Success → retry original request with new token
  └── Failure → clear storage, redirect to /login

Logout ──POST /auth/logout──> invalidate session server-side
  │
  ├── Clear localStorage (access_token, refresh_token, ace-theme)
  ├── Reset AuthStore to defaults
  └── Redirect to /login
```

### 3.2 Auth Store

```typescript
// $lib/stores/auth.svelte.ts
export class AuthStore {
  user = $state<User | null>(null);
  accessToken = $state<string>('');
  refreshToken = $state<string>('');
  expiresAt = $state<number>(0);
  isLoading = $state<boolean>(false);
  error = $state<string | null>(null);

  isAuthenticated = $derived(this.user !== null && this.accessToken !== '');

  // Initialization: load from localStorage, validate expiry
  init(): void;

  // Auth operations: delegate to $lib/api/auth.ts
  async login(email: string, password: string): Promise<void>;
  async register(email: string, password: string): Promise<void>;
  async logout(): Promise<void>;
  async refreshTokens(): Promise<void>;

  // Proactive refresh: check expiry before each API call
  ensureValidToken(): Promise<void>;

  // Clear all state
  clear(): void;
}

export const authStore = new AuthStore();
```

### 3.3 Auth Guard

The `(app)/+layout.svelte` layout acts as the auth guard:

```svelte
<script>
  import { authStore } from '$lib/stores/auth.svelte';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  let initialized = $state(false);

  onMount(async () => {
    authStore.init();
    if (!authStore.isAuthenticated) {
      goto('/login');
    }
    initialized = true;
  });
</script>

{#if initialized && authStore.isAuthenticated}
  <AppShell>
    <slot />
  </AppShell>
{/if}
```

The `(auth)/+layout.svelte` layout redirects authenticated users away from auth pages:

```svelte
<script>
  import { authStore } from '$lib/stores/auth.svelte';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  onMount(() => {
    authStore.init();
    if (authStore.isAuthenticated) {
      goto('/');
    }
  });
</script>

<div class="auth-layout">
  <slot />
</div>
```

---

## 4. Routing Architecture

### 4.1 Route Groups

SvelteKit route groups `(auth)`, `(app)`, and `(errors)` provide separate layouts without affecting URL paths.

| Group | Layout | Auth | Sidebar |
|-------|--------|------|---------|
| `(auth)` | Centered card, no sidebar | No | No |
| `(app)` | Full app shell with sidebar | Yes | Yes (collapsible) |
| `(errors)` | Minimal, centered message | No | No |

### 4.2 Navigation Guard

Navigation protection is handled by layout components, not a central router guard. Each route group's `+layout.svelte` checks `authStore.isAuthenticated` and redirects accordingly. This keeps auth logic co-located with layout concerns.

### 4.3 Role-Based Visibility

Admin-only routes (`/admin/*`) are guarded by checking `authStore.user.role === 'admin'` in the `(app)/+layout.svelte`. If a non-admin user navigates to `/admin/users`, they see a 403 page. Role-based visibility in the sidebar (hiding admin links for non-admins) is cosmetic — the backend enforces real RBAC.

---

## 5. Theme Engine Architecture

### 5.1 CSS Custom Properties

Themes are implemented as CSS custom property sets applied to the `<html>` element via a class. The theme engine lives in `$lib/stores/ui.svelte.ts` and persists to `localStorage` under key `ace-theme`.

**Token hierarchy:**

```
:root (light mode defaults)
├── --color-background
├── --color-foreground
├── --color-card / --color-card-foreground
├── --color-primary / --color-primary-foreground
├── --color-secondary / --color-secondary-foreground
├── --color-muted / --color-muted-foreground
├── --color-accent / --color-accent-foreground
├── --color-destructive / --color-destructive-foreground
├── --color-border
├── --color-input
├── --color-ring
├── --radius-sm / --radius-md / --radius-lg
└── --font-sans / --font-mono

.dark (dark mode overrides — same tokens, different values)
```

Each theme preset maps to a combination of light/dark custom property files:

```
$lib/themes/
├── one-dark.css       # One Dark (dark variant)
├── one-light.css      # One Light (light variant)
├── catppuccin-mocha.css
├── catppuccin-latte.css
├── nord.css
├── monokai.css
└── index.ts            # Theme registry, type definitions
```

### 5.2 Theme Switching Logic

```typescript
// $lib/stores/ui.svelte.ts
export class UIStore {
  theme = $state<string>('one-dark');   // Current preset name
  mode = $state<'dark' | 'light'>('dark');  // Current mode

  // Derived: the actual CSS class to apply
  themeClass = $derived(`${this.theme}-${this.mode}`);

  setTheme(preset: string): void {
    this.theme = preset;
    this.persist();
    this.apply();
  }

  toggleMode(): void {
    this.mode = this.mode === 'dark' ? 'light' : 'dark';
    this.persist();
    this.apply();
  }

  private apply(): void {
    // Remove all theme classes from <html>
    // Add current themeClass
    // Update chart/text colors as needed
  }

  private persist(): void {
    localStorage.setItem('ace-theme', JSON.stringify({
      theme: this.theme,
      mode: this.mode
    }));
  }
}
```

Theme application happens in `+layout.svelte` root, which applies the theme class before first paint via a `<script>` tag in `app.html` that reads `localStorage` synchronously.

### 5.3 Static CSS Approach

Each theme file imports Tailwind directives and maps theme tokens to CSS custom properties:

```css
/* $lib/themes/one-dark.css */
@layer base {
  .one-dark {
    --color-background: 40 40 40;
    --color-foreground: 220 220 210;
    /* ... all tokens ... */
  }
}
```

Tailwind v4's CSS-first configuration (`@theme` directive) maps these tokens to utility classes. Components use Tailwind's semantic classes (`bg-background`, `text-foreground`, `border-border`) which resolve to the current theme's custom properties. Theme switching is a class change on `<html>` — zero JavaScript style computation.

---

## 6. Component Architecture

### 6.1 Atomic Design Hierarchy

```
Atoms ──> Molecules ──> Organisms ──> Templates ──> Pages

Atom:      Button, Input, Badge, Avatar, Skeleton
Molecule:  FormField (Label + Input + Error), SearchBar, UserRow, Toast
Organism:  LoginForm, UserTable, Sidebar, Header, HealthCards
Template:  AuthLayout, AppLayout
Page:      /login, /admin/users, /telemetry/spans
```

**Rules:**
- Atoms never import stores. They receive all data via props and emit events.
- Molecules may combine atoms and read from stores for simple lookups (e.g., theme selector).
- Organisms compose molecules and atoms, handle local interaction patterns.
- Pages orchestrate organisms and stores. Pages are thin — they wire data flow, not UI logic.

### 6.2 UI Primitive Pattern (shadcn-svelte)

Each UI primitive in `$lib/components/ui/` is a shadcn-svelte component with Tailwind styling. The pattern follows shadcn-svelte's copy-paste-and-own model:

```svelte
<!-- $lib/components/ui/dialog/Dialog.svelte -->
<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog';
  import { cn } from '$lib/utils/cn';

  type Props = {
    open?: boolean;
    class?: string;
  };

  let { open = $bindable(false), class: className, children }: Props = $props();
</script>

<Dialog.Root bind:open>
  <Dialog.Portal>
    <Dialog.Overlay class={cn('fixed inset-0 bg-black/80', className)} />
    <Dialog.Content class="fixed left-1/2 top-1/2 ...">
      {@render children()}
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>
```

**Key principle:** UI primitives own only presentation. They accept `class` for extension. They emit events via callbacks. shadcn-svelte provides accessible components (ARIA, focus trap, keyboard navigation) with Tailwind for visual styling.

### 6.3 Component File Structure

Each component lives in its own directory with an `index.ts` barrel export:

```
$lib/components/ui/button/
├── Button.svelte
├── index.ts          # export { default as Button } from './Button.svelte'
└── index.test.ts     # co-located test
```

### 6.4 `cn()` Utility

A `cn()` utility (uses `clsx` + `tailwind-merge`) merges Tailwind classes without conflicts:

```typescript
// $lib/utils/cn.ts
import { clsx } from 'clsx';
import type { ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(...inputs));
}
```

### 6.5 Form Composable

A reusable form composable built on Svelte 5 runes + Zod:

```typescript
// $lib/utils/form.svelte.ts
import { ZodSchema } from 'zod';

export function useForm<T extends Record<string, unknown>>(
  initialValues: T,
  schema: ZodSchema<T>
) {
  const values = $state(initialValues);
  const errors = $state<Partial<Record<keyof T, string>>>({});
  const touched = $state<Partial<Record<keyof T, boolean>>>({});
  const isSubmitting = $state(false);

  const isValid = $derived(Object.keys(errors).every(k => !errors[k]));
  const isDirty = $derived(
    JSON.stringify(values) !== JSON.stringify(initialValues)
  );

  function validate(): boolean { /* Zod parse, set errors */ }
  function reset(): void { /* reset to initialValues */ }
  async function handleSubmit(fn: (values: T) => Promise<void>): Promise<void> {
    if (!validate()) return;
    isSubmitting = true;
    try { await fn(values); } finally { isSubmitting = false; }
  }

  return { values, errors, touched, isValid, isDirty, isSubmitting, validate, reset, handleSubmit };
}
```

Used in every form page:
```svelte
<script>
  import { useForm } from '$lib/utils/form.svelte';
  import { loginSchema } from '$lib/validation/schemas';
  import { authStore } from '$lib/stores/auth.svelte';

  const form = useForm({ email: '', password: '' }, loginSchema);
</script>

<form on:submit|preventDefault={() => form.handleSubmit(authStore.login)}>
  <FormField label="Email" bind:value={form.values.email} error={form.errors.email} />
  <FormField label="Password" type="password" bind:value={form.values.password} error={form.errors.password} />
  <Button type="submit" disabled={!form.isValid || form.isSubmitting}>
    {form.isSubmitting ? 'Signing in...' : 'Sign in'}
  </Button>
</form>
```

---

## 7. API Client Architecture

### 7.1 Base Client

```typescript
// $lib/api/client.ts
interface RequestOptions {
  method: 'GET' | 'POST' | 'PUT' | 'DELETE';
  path: string;
  body?: unknown;
  params?: Record<string, string>;
  headers?: Record<string, string>;
}

interface APIEnvelope<T> {
  success: boolean;
  data?: T;
  error?: APIError;
}

class APIClient {
  private baseUrl: string;
  private refreshPromise: Promise<void> | null = null;

  constructor() {
    this.baseUrl = import.meta.env.VITE_API_URL || '';
  }

  // Core request method — all API calls go through this
  async request<T>(options: RequestOptions): Promise<T>;

  // Token management
  private getAccessToken(): string | null;
  private setTokens(access: string, refresh: string, expiresIn: number): void;
  private clearTokens(): void;

  // Proactive refresh — called before each request
  private async ensureValidToken(): Promise<void>;

  // Reactive refresh — called on 401
  private async handleUnauthorized(): Promise<void>;

  // Error normalization
  private normalizeError(response: Response): APIError;
}

export const apiClient = new APIClient();
```

### 7.2 Domain Modules

Each domain module wraps `apiClient` with typed request/response:

```typescript
// $lib/api/auth.ts
export async function login(email: string, password: string): Promise<TokenResponse> {
  return apiClient.request<TokenResponse>({
    method: 'POST',
    path: '/auth/login',
    body: { email, password }
  });
}

// $lib/api/sessions.ts
export async function listSessions(page: number, limit: number): Promise<PaginatedResponse<Session>> {
  return apiClient.request<PaginatedResponse<Session>>({
    method: 'GET',
    path: '/auth/me/sessions',
    params: { page: String(page), limit: String(limit) }
  });
}
```

### 7.3 TypeScript Types

All types live in `$lib/api/types.ts` and mirror the backend's JSON response shapes:

```typescript
// Matching backend/response.go APIResponse envelope
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

// Matching backend/model/user.go
interface User {
  id: string;
  email: string;
  role: 'admin' | 'user' | 'viewer';
  status: 'pending' | 'active' | 'suspended';
  suspended_at?: string;
  suspended_reason?: string;
  created_at: string;
  updated_at: string;
}

interface TokenResponse {
  access_token: string;
  refresh_token: string;
  user: User;
  expires_in: number;
}

interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}

// ... sessions, admin, telemetry types
```

### 7.4 Error Handling Strategy

```
API Response
  ├── success: true → unwrap data, return typed value
  ├── success: false, error.code:
  │   ├── "unauthorized"   → trigger refresh → retry → redirect to /login
  │   ├── "forbidden"      → show 403 message
  │   ├── "not_found"      → show 404 page
  │   ├── "validation_error" → map field errors to form
  │   ├── "rate_limit_exceeded" → show toast "Too many requests"
  │   └── "internal_error" → show generic error toast
  └── Network error → show "Unable to connect" toast
```

---

## 8. State Management Patterns

### 8.1 State Location Rules

| State Type | Location | Example |
|-----------|----------|---------|
| Auth state (user, tokens) | `auth.svelte.ts` store | User identity, JWTs |
| Global UI state (sidebar, theme) | `ui.svelte.ts` store | Sidebar collapsed, theme preset |
| Notification toasts | `notifications.svelte.ts` store | Error toasts, success messages |
| Page-specific data | Component `$state` | Form inputs, filter selections |
| URL state | `$page.url.searchParams` | Page number, active filter |
| Derived data | `$derived` in component or store | Filtered lists, computed totals |

### 8.2 Data Fetching Pattern

Pages use a standard data-loading lifecycle:

```svelte
<script>
  import { adminApi } from '$lib/api/admin';

  let users = $state<UserListItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  async function loadUsers(page = 1) {
    loading = true;
    error = null;
    try {
      const response = await adminApi.listUsers(page);
      users = response.users;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => loadUsers());
</script>

{#if loading}
  <Skeleton type="table" />
{:else if error}
  <ErrorState {error} on:retry={() => loadUsers()} />
{:else}
  <UserTable {users} on:page-change={(e) => loadUsers(e.detail)} />
{/if}
```

### 8.3 Optimistic Updates

For destructive or high-frequency actions (suspend user, revoke session), update the store optimistically and revert on error:

```typescript
async function suspendUser(id: string, reason: string) {
  const previous = this.users.find(u => u.id === id);
  this.updateUserStatus(id, 'suspended'); // Optimistic
  try {
    await adminApi.suspendUser(id, reason);
  } catch {
    this.updateUserStatus(id, previous.status); // Revert
    notifications.add('Failed to suspend user', 'error');
  }
}
```

---

## 9. Layout Architecture

### 9.1 App Shell

The authenticated app shell consists of three persistent regions:

```
┌────────────────────────────────────────────────────────────┐
│ ┌──────────┐ ┌──────────────────────────────────────────┐  │
│ │          │ │  Header                                   │  │
│ │          │ │  [Hamburger] [Breadcrumbs] [User Menu]   │  │
│ │          │ ├──────────────────────────────────────────┤  │
│ │ Sidebar  │ │                                          │  │
│ │          │ │  Content (scrollable)                     │  │
│ │ [Logo]   │ │                                          │  │
│ │ Dash     │ │  ┌──────────────────────────────────┐   │  │
│ │ Agents   │ │  │  Page-specific content             │   │  │
│ │ Telemetry│ │  │                                      │   │  │
│ │ Admin*   │ │  │                                      │   │  │
│ │          │ │  │                                      │   │  │
│ │ -------- │ │  │                                      │   │  │
│ │ Settings │ │  │                                      │   │  │
│ │ [Avatar] │ │  └──────────────────────────────────┘   │  │
│ └──────────┘ └──────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘

Mobile (<768px):
┌────────────────────────────────────────────┐
│  [≡] [Breadcrumbs]              [User]     │
│────────────────────────────────────────────│
│                                            │
│  Content (full width)                      │
│                                            │
└────────────────────────────────────────────┘
(Sidebar slides in as overlay on hamburger click)
```

### 9.2 Responsive Behavior

| Breakpoint | Width | Sidebar State | Toggle |
|-----------|-------|---------------|--------|
| Mobile | <768px | Hidden (overlay) | Hamburger menu in header |
| Tablet | 768–1024px | Collapsed (64px, icons only) | Click logo area or toggle button |
| Desktop | >1024px | Expanded (256px, icons + text) | Toggle button collapses to 64px |

Sidebar state persists in `ui.svelte.ts` store and syncs to `localStorage` key `ace-sidebar-collapsed`. On mobile, sidebar auto-closes on route navigation.

### 9.3 Layout Components

```
$lib/components/layout/
├── AppShell.svelte       # Sidebar + Header + Content slot
├── Sidebar.svelte       # Collapsible sidebar with nav items
├── Header.svelte         # Breadcrumbs, user menu, mobile toggle
├── NavItem.svelte        # Individual sidebar nav link (icon + text)
├── UserMenu.svelte       # Dropdown: profile, sessions, logout
└── Breadcrumbs.svelte    # Auto-generated from route path
```

`AppShell.svelte` reads `authStore.user` and `uiStore.sidebarCollapsed`. It renders the `(app)` layout structure and provides a `<slot />` for page content.

### 9.4 Auth Layout

The `(auth)` layout is a centered card with no sidebar:

```
┌────────────────────────────────────────────┐
│                                            │
│          ┌────────────────────┐            │
│          │   ACE Logo         │            │
│          │                    │            │
│          │   Form Card        │            │
│          │   (Login/Register) │            │
│          │                    │            │
│          │   Links            │            │
│          └────────────────────┘            │
│                                            │
└────────────────────────────────────────────┘
```

---

## 10. Accessibility Architecture

### 10.1 Structural Requirements

- Skip-to-content link as first focusable element in `(app)/+layout.svelte`
- Landmark roles: `<nav role="navigation">` for sidebar, `<main role="main">` for content, `<header role="banner">` for header
- Heading hierarchy: each page has exactly one `<h1>`, sub-sections use `<h2>` / `<h3>` in order
- All forms have `<label>` elements associated with inputs via `for`/`id` or wrapping

### 10.2 Interactive Requirements

- Focus-visible ring on all interactive elements (`focus-visible:ring-2 focus-visible:ring-ring`)
- Focus trap in all Dialog/Modal components (Bits UI handles this)
- `Escape` key closes modals, dropdowns, and the mobile sidebar
- Dropdown menus have arrow key navigation (Bits UI handles this)
- Loading states announced via `aria-live="polite"` regions
- Page title updates on route change via SvelteKit's `$page.url`

### 10.3 Color Contrast

All theme presets must meet WCAG 2.1 AA contrast ratios:
- Normal text: ≥ 4.5:1 against background
- Large text (≥18pt or 14pt bold): ≥ 3:1 against background
- UI components and graphical objects: ≥ 3:1 against background

Each theme preset is tested against these ratios before inclusion.

---

## 11. Testing Architecture

### 11.1 Test Layers

| Layer | Tool | Scope | Location |
|-------|------|-------|----------|
| Unit | Vitest | Stores, utils, API client | Co-located `.test.ts` |
| Component | Vitest + Testing Library | Individual components | Co-located `.test.ts` |
| Integration | Vitest | Form flows, auth flows | `src/test/integration/` |

E2E testing (Playwright) is a separate unit and not in scope.

### 11.2 Store Testing

Stores are plain TypeScript classes — they can be tested without rendering any Svelte components:

```typescript
// $lib/stores/auth.svelte.test.ts
import { authStore } from './auth.svelte';
import { vi } from 'vitest';
import * as authApi from '$lib/api/auth';

vi.mock('$lib/api/auth');

test('login stores tokens and user', async () => {
  const mockResponse = { access_token: 'at', refresh_token: 'rt', user: { id: '1', email: 'a@b.c', role: 'user', status: 'active' }, expires_in: 3600 };
  vi.mocked(authApi.login).mockResolvedValue(mockResponse);
  
  await authStore.login('a@b.c', 'password');
  expect(authStore.isAuthenticated).toBe(true);
  expect(authStore.user?.email).toBe('a@b.c');
});
```

### 11.3 Component Testing

Use Testing Library's `render` and `screen` for component tests. Focus on user-visible behavior, not implementation details:

```typescript
// $lib/components/ui/button/Button.test.ts
import { render, screen } from '@testing-library/svelte';
import Button from './Button.svelte';

test('renders with text and handles click', async () => {
  const { component } = render(Button, { props: { children: 'Click me' } });
  expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument();
});
```

---

## 12. Dependency Management

### 12.1 Production Dependencies

| Package | Purpose | Justification |
|---------|---------|---------------|
| `shadcn-svelte` | UI component library | Copy-paste-and-own components, accessibility built-in, Tailwind styling |
| `tailwindcss` v3 | Utility CSS | Design system tokens, zero runtime CSS-in-JS, shadcn requires Tailwind v3 |
| `clsx` | Conditional class names | Tiny (228B), used by `cn()` utility |
| `tailwind-merge` | Tailwind class conflict resolution | Resolves conflicting Tailwind classes in `cn()` |
| `zod` | Schema validation | Client-side form validation, matches server constraints |
| `lucide-svelte` | Icon library | Consistent icon set, tree-shakeable, matches shadcn ecosystem |

### 12.2 Development Dependencies

| Package | Purpose |
|---------|---------|
| `@sveltejs/kit` v2 | Framework |
| `svelte` v5 | Compiler |
| `vite` v6 | Build tool |
| `vitest` v2 | Testing |
| `@testing-library/svelte` v5 | Component testing |
| `jsdom` | DOM environment for tests |

### 12.3 Excluded Dependencies

| Package | Reason for Exclusion |
|---------|---------------------|
| `@sveltejs/adapter-node` | We use `adapter-static` — no Node server |
| `superforms` | Requires SvelteKit server-side form actions, which we don't have in SPA mode |
| `@tanstack/svelte-query` | Premature optimization — our data fetching is simple REST + our own loading states. Add if caching/stale-while-revalidate becomes a pain point |
| `svelte-routing` | SvelteKit has built-in routing |
| `axios` | Native `fetch` is sufficient; our `APIClient` wrapper handles auth, refresh, and error normalization |
| Any CSS-in-JS library | Tailwind v4 handles all styling |

---

## 13. Build & Deployment

### 13.1 Build Pipeline

```
npm run build
  ├── SvelteKit compiles (adapter-static)
  ├── Tailwind v4 processes CSS
  ├── Vite bundles JS (tree-shaken, code-split per route)
  ├── Output: static files in /build
  └── Go binary embeds /build via embed.FS
```

### 13.2 Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `VITE_API_URL` | Backend API base URL | `` (same origin) |
| `VITE_OTEL_ENDPOINT` | OpenTelemetry trace endpoint | `` (disabled) |

`VITE_API_URL` is empty by default, meaning the frontend makes API calls to the same origin that serves it. In development, Vite's proxy configuration in `vite.config.ts` forwards `/api` requests to the Go backend at `localhost:8080`.

### 13.3 Development Proxy

```typescript
// vite.config.ts (addition)
export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
});
```

This allows the frontend dev server (`localhost:5173`) to call the backend API (`localhost:8080`) without CORS issues during development. In production, both are served from the same Go binary.

---

## 14. Complete Directory Structure

```
frontend/src/
├── app.html                          # HTML shell (theme pre-load script)
├── app.css                           # Tailwind v4 imports + theme token definitions
├── app.d.ts                          # Global type declarations
│
├── routes/
│   ├── +layout.svelte                # Root layout (theme init, providers)
│   ├── +page.svelte                  # Redirect to / or /login
│   ├── (auth)/
│   │   ├── +layout.svelte            # Centered card layout, no auth guard
│   │   ├── login/+page.svelte
│   │   ├── register/+page.svelte
│   │   ├── forgot-password/+page.svelte
│   │   ├── reset-password/+page.svelte
│   │   └── magic-link/+page.svelte
│   ├── (app)/
│   │   ├── +layout.ts                # ssr = false
│   │   ├── +layout.svelte            # App shell (sidebar + header + content)
│   │   ├── +page.svelte              # Dashboard
│   │   ├── profile/+page.svelte
│   │   ├── admin/
│   │   │   └── users/
│   │   │       ├── +page.svelte      # User list
│   │   │       └── [id]/+page.svelte # User detail
│   │   ├── telemetry/
│   │   │   ├── +page.svelte          # Overview
│   │   │   ├── spans/+page.svelte
│   │   │   ├── metrics/+page.svelte
│   │   │   └── usage/+page.svelte
│   │   └── settings/+page.svelte
│   └── (errors)/
│       ├── +layout.svelte
│       └── +page.svelte               # 404 fallback
│
├── lib/
│   ├── api/
│   │   ├── client.ts                 # Base fetch wrapper (auth, refresh, errors)
│   │   ├── types.ts                  # API response types (mirrors backend)
│   │   ├── auth.ts                   # Login, register, logout, refresh, me
│   │   ├── sessions.ts              # List, revoke
│   │   ├── admin.ts                  # ListUsers, GetUser, Suspend, Restore, UpdateRole
│   │   └── telemetry.ts             # Spans, metrics, usage, health
│   │
│   ├── stores/
│   │   ├── auth.svelte.ts            # AuthStore: user, tokens, isAuthenticated
│   │   ├── ui.svelte.ts              # UIStore: theme, sidebar state
│   │   └── notifications.svelte.ts   # NotificationStore: toasts
│   │
│   ├── components/
│   │   ├── ui/                       # Primitive components (Bits UI + Tailwind)
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
│   │   │   │   ├── TableCell.svelte
│   │   │   │   └── index.ts
│   │   │   ├── badge/
│   │   │   ├── avatar/
│   │   │   ├── dropdown-menu/
│   │   │   ├── toast/
│   │   │   ├── card/
│   │   │   ├── tabs/
│   │   │   ├── skeleton/
│   │   │   └── separator/
│   │   ├── layout/
│   │   │   ├── AppShell.svelte
│   │   │   ├── Sidebar.svelte
│   │   │   ├── Header.svelte
│   │   │   ├── NavItem.svelte
│   │   │   ├── UserMenu.svelte
│   │   │   └── Breadcrumbs.svelte
│   │   ├── auth/
│   │   │   ├── LoginForm.svelte
│   │   │   ├── RegisterForm.svelte
│   │   │   ├── ForgotPasswordForm.svelte
│   │   │   ├── ResetPasswordForm.svelte
│   │   │   └── MagicLinkVerifier.svelte
│   │   ├── admin/
│   │   │   ├── UserTable.svelte
│   │   │   ├── UserDetailCard.svelte
│   │   │   ├── SuspendDialog.svelte
│   │   │   └── RoleSelect.svelte
│   │   ├── telemetry/
│   │   │   ├── HealthCards.svelte
│   │   │   ├── SpansTable.svelte
│   │   │   ├── MetricsList.svelte
│   │   │   └── UsageTable.svelte
│   │   └── shared/
│   │       ├── DataState.svelte       # Loading/Error/Empty wrapper
│   │       ├── Pagination.svelte
│   │       ├── ConfirmDialog.svelte
│   │       └── SearchInput.svelte
│   │
│   ├── themes/
│   │   ├── one-dark.css
│   │   ├── one-light.css
│   │   ├── catppuccin-mocha.css
│   │   ├── catppuccin-latte.css
│   │   ├── nord.css
│   │   ├── monokai.css
│   │   └── index.ts
│   │
│   ├── validation/
│   │   └── schemas.ts                # Zod schemas for all forms
│   │
│   ├── utils/
│   │   ├── cn.ts                      # clsx + tailwind-merge
│   │   ├── form.svelte.ts             # useForm composable
│   │   ├── formatter.ts               # Date, duration, cost formatters
│   │   └── constants.ts               # Route paths, breakpoints, defaults
│   │
│   └── telemetry/
│       ├── index.ts                   # (existing)
│       ├── error.ts                   # (existing)
│       ├── metrics.ts                 # (existing)
│       └── trace.ts                   # (existing)
│
└── test/
    ├── setup.ts                       # Vitest setup (jsdom, globals)
    └── integration/
        ├── auth.test.ts               # Login/logout flow
        └── admin.test.ts              # Admin user management flow
```

---

## 15. Architectural Decisions Record

| # | Decision | Choice | Alternatives Considered | Rationale |
|---|----------|-------|------------------------|-----------|
| ADR-1 | Rendering mode | SPA (adapter-static) | SSR, Hybrid SSR | Single-binary deployment constraint; no Node server; auth dashboard doesn't need SEO |
| ADR-2 | State management | Svelte 5 rune class stores | Svelte stores, tRPC, external libs | Zero external deps; compiler-optimized reactivity; testable pure TS classes |
| ADR-3 | Auth token storage | localStorage + in-memory | HttpOnly cookies, memory-only | Backend returns JSON tokens; localStorage survives refresh; XSS risk minimal (single-binary, no 3P scripts) |
| ADR-4 | Component library | shadcn-svelte + Tailwind v3 | Bits UI, Skeleton, DaisyUI | shadcn-svelte provides accessible components with copy-paste model; no runtime dependency; easy customization; matches design system |
| ADR-5 | CSS framework | Tailwind CSS v3 | CSS Modules, Styled-components, UnoCSS | Utility-first matches component model; shadcn requires v3; tree-shaking removes unused utilities |
| ADR-6 | API client | Custom typed fetch wrapper | OpenAPI codegen, tRPC, raw fetch | Full control over auth/refresh/error logic; matches backend JSON envelope; no codegen drift risk |
| ADR-7 | Form handling | Custom useForm + Zod | Superforms, Felte | SPA mode negates Superforms advantage; API expects JSON POST, not form-encoded; full control |
| ADR-8 | Theme engine | CSS custom properties + class switching | CSS-in-JS, Tailwind plugin, PostCSS | Zero runtime overhead; class swap is instant; each theme is a static CSS file; dark/light is a mode toggle |
| ADR-9 | Icon library | Lucide Svelte | Heroicons, Font Awesome, Phosphor | Tree-shakeable; consistent with shadcn ecosystem; Svelte-native components |
| ADR-10 | Real-time updates | Deferred (SSE Phase 2) | WebSocket, NATS WS, Polling | Not in scope for Phase 1; design API client to be extensible for event stream subscription |