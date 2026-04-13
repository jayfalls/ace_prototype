# Implementation Plan: Frontend Design

[unit: frontend-design]

## Execution Strategy

Each slice is a vertical, testable increment. A slice establishes one complete user-facing capability end-to-end: utility/config → API client → store → component → page → test. No horizontal "build all X" phases.

Dependencies between slices are explicit. A slice that depends on a prior slice lists it.

---

## Slice 1: Project Scaffolding & Tailwind v4 + Theme Engine

- **Backend:** N/A
- **Frontend:**
  - Install production deps: `bits-ui`, `tailwindcss` v4, `clsx`, `tailwind-merge`, `zod`, `lucide-svelte`
  - Install dev deps: `@tailwindcss/vite`
  - Configure `vite.config.ts` with Tailwind v4 plugin and `/api` + `/health` proxy to `localhost:8080`
  - Create `app.css` with `@import 'tailwindcss'` and `:root` base tokens (all 18 CSS custom properties)
  - Update `app.html` with FOUC-prevention script (read `ace-ui` from localStorage, apply theme class)
  - Create `$lib/utils/cn.ts` (clsx + tailwind-merge)
  - Create `$lib/utils/constants.ts` (ROUTES, BREAKPOINTS, SIDEBAR, PAGINATION, AUTH, THEME constants)
  - Create `$lib/themes/index.ts` (theme registry, ThemePreset type, getThemeClass function)
  - Create all 6 theme preset CSS files (`one-dark.css`, `one-light.css`, `catppuccin-mocha.css`, `catppuccin-latte.css`, `nord.css`, `monokai.css`) with full token sets
  - Create `$lib/stores/ui.svelte.ts` (UIStore with theme, mode, sidebarCollapsed, persist, apply, init)
  - Update root `+layout.svelte` to call `uiStore.init()` on mount
  - Configure `vitest.config.ts` with jsdom environment and setup file path
  - Create `src/test/setup.ts` (jsdom globals)
- **Test:**
  - `ui.svelte.test.ts`: theme toggle, persist to localStorage, apply class to document, defaults on empty storage
  - `cn.test.ts`: merge conflicts, conditional classes, empty input
  - `constants.test.ts`: verify ROUTES keys exist, numeric breakpoint values

---

## Slice 2: API Client & Auth Store

- **Backend:** N/A (consumes existing backend API)
- **Frontend:**
  - Create `$lib/api/types.ts` (all TypeScript interfaces: APIEnvelope, APIError, FieldError, User, TokenResponse, PaginatedResponse, Session, SessionsListResponse, UserListItem, AdminUserResponse, UsersListResponse, Span, SpansResponse, Metric, MetricsResponse, UsageEvent, UsageResponse, TelemetryHealthResponse, SystemHealthResponse, query param interfaces)
  - Create `$lib/api/client.ts` (APIClient class: request<T>, token management, proactive refresh, reactive refresh mutex, error normalization, envelope unwrapping)
  - Create `$lib/api/auth.ts` (login, register, logout, refresh, me, resetPasswordRequest, resetPasswordConfirm, magicLinkRequest, magicLinkVerify)
  - Create `$lib/api/sessions.ts` (listSessions, revokeSession)
  - Create `$lib/stores/auth.svelte.ts` (AuthStore: login, register, logout, refreshTokens, ensureValidToken, init, clear, localStorage persistence)
  - Create `$lib/stores/notifications.svelte.ts` (NotificationStore: add, dismiss, success, error, warning, info, auto-dismiss)
- **Test:**
  - `client.test.ts`: token injection on requests, 401 triggers refresh, refresh mutex blocks concurrent refreshes, error code mapping, envelope unwrapping
  - `auth.test.ts`: each auth API function calls correct endpoint with correct payload
  - `auth.svelte.test.ts`: login stores tokens and user, logout clears storage, refresh updates tokens, init restores from localStorage, clear resets state
  - `notifications.svelte.test.ts`: add creates toast, dismiss removes toast, auto-dismiss fires after duration, shorthand methods set correct variant

---

## Slice 3: Validation & Form Utilities

- **Backend:** N/A
- **Frontend:**
  - Install `zod` (already included in Slice 1 deps)
  - Create `$lib/validation/schemas.ts` (loginSchema, registerSchema, forgotPasswordSchema, resetPasswordSchema, suspendUserSchema, updateUserRoleSchema)
  - Create `$lib/utils/form.svelte.ts` (useForm composable: values, errors, touched, isSubmitting, isValid, isDirty, validate, validateField, reset, handleSubmit, setFieldErrors)
  - Create `$lib/utils/formatter.ts` (formatDate, formatDateTime, formatRelativeTime, formatDuration, formatCost, formatNumber, parseUserAgent, roleBadgeVariant, statusBadgeVariant)
- **Test:**
  - `schemas.test.ts`: valid inputs pass, invalid inputs fail with correct messages, edge cases (empty strings, short passwords, mismatched confirm)
  - `form.svelte.test.ts`: validate sets errors on invalid input, handleSubmit calls submit function on valid input, handleSubmit does not call on invalid, setFieldErrors maps API errors, reset clears state
  - `formatter.test.ts`: all format functions produce expected output for known inputs, edge cases (null, zero, negative)

---

## Slice 4: UI Primitives — Atoms

- **Backend:** N/A
- **Frontend:**
  - Create `$lib/components/ui/button/Button.svelte` + `index.ts` (variants: primary, secondary, destructive, ghost, outline; sizes: sm, md, lg; loading state with spinner; disabled state)
  - Create `$lib/components/ui/input/Input.svelte` + `index.ts` (type, placeholder, value, disabled, error, id, label; blur validation trigger)
  - Create `$lib/components/ui/badge/Badge.svelte` + `index.ts` (variants: default, success, warning, error, info; sizes: sm, md)
  - Create `$lib/components/ui/skeleton/Skeleton.svelte` + `index.ts` (variants: text, circle, rect, table; pulse animation)
  - Create `$lib/components/ui/separator/Separator.svelte` + `index.ts` (orientation: horizontal/vertical; decorative prop)
  - Create `$lib/components/ui/avatar/Avatar.svelte` + `index.ts` (src, alt, fallback initials, sizes: sm, md, lg)
- **Test:**
  - `Button.test.ts`: renders text, shows spinner on loading, disables on loading, emits click, applies variant classes
  - `Input.test.ts`: renders label, shows error message, binds value, disables
  - `Badge.test.ts`: renders text, applies variant class
  - `Skeleton.test.ts`: renders with variant class, shows pulse animation

---

## Slice 5: UI Primitives — Composites (Dialog, Select, Dropdown, Toast, Tabs, Card, Table)

- **Backend:** N/A
- **Frontend:**
  - Create `$lib/components/ui/dialog/` (Dialog.svelte, DialogContent.svelte, DialogOverlay.svelte, DialogTrigger.svelte, index.ts) — Bits UI Dialog wrapper with focus trap, Escape dismiss, overlay
  - Create `$lib/components/ui/select/Select.svelte` + `index.ts` — Bits UI Select with keyboard navigation and value binding
  - Create `$lib/components/ui/dropdown-menu/` (DropdownMenu.svelte, DropdownMenuContent.svelte, DropdownMenuItem.svelte, DropdownMenuTrigger.svelte, index.ts) — Bits UI Dropdown with arrow key nav
  - Create `$lib/components/ui/toast/Toast.svelte`, `Toaster.svelte` + `index.ts` — Toast rendering, auto-dismiss, stacked positioning, reads from notificationStore
  - Create `$lib/components/ui/tabs/` (Tabs.svelte, TabsList.svelte, TabsTrigger.svelte, TabsContent.svelte, index.ts) — Bits UI Tabs with keyboard navigation
  - Create `$lib/components/ui/card/` (Card.svelte, CardHeader.svelte, CardContent.svelte, CardFooter.svelte, index.ts) — Layout primitive
  - Create `$lib/components/ui/table/` (Table.svelte, TableRow.svelte, TableHeader.svelte, TableCell.svelte, index.ts) — Responsive table with horizontal scroll on mobile
- **Test:**
  - `Dialog.test.ts`: opens on trigger click, closes on Escape, closes on overlay click, focus trapped inside
  - `Select.test.ts`: renders options, selects value, keyboard navigation
  - `Toast.test.ts`: renders toast with title, auto-dismisses, dismiss on click, stacks multiple toasts
  - `Card.test.ts`: renders header, content, footer slots
  - `Table.test.ts`: renders rows and cells, applies responsive wrapper

---

## Slice 6: Layout Shell — AppShell, Sidebar, Header, Navigation

- **Backend:** N/A
- **Frontend:**
  - Create `$lib/components/layout/NavItem.svelte` (icon + label + optional badge, aria-current on active route, lucide-svelte icons)
  - Create `$lib/components/layout/Sidebar.svelte` (collapsible: 256px expanded, 64px collapsed; logo area, nav items, settings at bottom; mobile overlay mode; reads/writes uiStore.sidebarCollapsed)
  - Create `$lib/components/layout/Header.svelte` (breadcrumbs from route, mobile hamburger toggle, UserMenu dropdown)
  - Create `$lib/components/layout/UserMenu.svelte` (avatar, dropdown: profile link, sessions link, logout; uses DropdownMenu primitive, reads authStore.user)
  - Create `$lib/components/layout/Breadcrumbs.svelte` (auto-generated from $page.url, last segment non-link)
  - Create `$lib/components/layout/AppShell.svelte` (Sidebar + Header + `<main role="main">` content slot, skip-to-content link, ARIA landmarks)
  - Create `(auth)/+layout.svelte` (centered card layout, no sidebar, redirects authenticated users to `/`)
  - Create `(app)/+layout.ts` (`export const ssr = false; export const prerender = false;`)
  - Create `(app)/+layout.svelte` (auth guard: calls authStore.init(), redirects unauthenticated to `/login`, renders AppShell with slot)
  - Create `(errors)/+layout.svelte` (centered message layout)
- **Test:**
  - `Sidebar.test.ts`: renders nav items with correct links, collapses on toggle, shows overlay on mobile, hides admin links for non-admin
  - `Header.test.ts`: renders breadcrumbs, renders user menu, shows hamburger on mobile
  - `AppShell.test.ts`: renders sidebar and content slot, skip-to-content link exists

---

## Slice 7: Auth Pages — Login, Register, Forgot/Reset Password, Magic Link

- **Backend:** N/A (consumes existing auth endpoints)
- **Frontend:**
  - Create `$lib/components/auth/LoginForm.svelte` (email + password + submit, uses useForm + loginSchema, calls authStore.login, links to forgot-password, register, magic-link)
  - Create `$lib/components/auth/RegisterForm.svelte` (email + password + confirm + submit, uses useForm + registerSchema, calls authStore.register)
  - Create `$lib/components/auth/ForgotPasswordForm.svelte` (email + submit, calls authApi.resetPasswordRequest, always shows success message per BSD)
  - Create `$lib/components/auth/ResetPasswordForm.svelte` (new password + confirm + submit, token from URL params, calls authApi.resetPasswordConfirm)
  - Create `$lib/components/auth/MagicLinkVerifier.svelte` (no form, calls authApi.magicLinkVerify on mount, shows loading/success/error states)
  - Create route pages:
    - `(auth)/login/+page.svelte`
    - `(auth)/register/+page.svelte`
    - `(auth)/forgot-password/+page.svelte`
    - `(auth)/reset-password/+page.svelte`
    - `(auth)/magic-link/+page.svelte`
  - Update root `+page.svelte` to redirect to `/` or `/login` based on auth state
- **Test:**
  - `LoginForm.test.ts`: renders email and password fields, shows validation errors on blur, submits on valid input, shows error toast on 401, redirects on success
  - `RegisterForm.test.ts`: renders all fields, validates password match, shows conflict error on 409
  - `ForgotPasswordForm.test.ts`: always shows "reset link sent" message regardless of response
  - `ResetPasswordForm.test.ts`: extracts token from URL, validates passwords, auto-logins on success
  - `MagicLinkVerifier.test.ts`: calls verify on mount, shows spinner, redirects on success, shows error with retry link on failure

---

## Slice 8: Dashboard & Profile Pages

- **Backend:** N/A (consumes existing `/telemetry/health`, `/health/ready`, `/auth/me`, `/auth/me/sessions`)
- **Frontend:**
  - Create `$lib/api/telemetry.ts` (health function — spans, metrics, usage added in later slices)
  - Create `$lib/api/sessions.ts` (listSessions, revokeSession — if not already in Slice 2)
  - Create `$lib/components/telemetry/HealthCards.svelte` (3-4 cards: database, NATS/messaging, cache status; shows status indicator + counts from `/telemetry/health`)
  - Create `$lib/components/shared/DataState.svelte` (loading/error/empty wrapper: renders Skeleton on loading, error message on error, empty state on empty, slot on success)
  - Create `(app)/+page.svelte` — Dashboard: welcome message, HealthCards, quick-action buttons ("View Telemetry", "Manage Users" for admin, "View Profile")
  - Create `(app)/profile/+page.svelte` — Profile: user info card (email, role, status, created_at), session table with revoke buttons, "Sign out all other sessions" button, "Change password" link
  - Create `$lib/components/shared/Pagination.svelte` (page numbers, prev/next, total count display)
  - Create `$lib/components/shared/SearchInput.svelte` (debounced input, emits search after configurable delay)
- **Test:**
  - `HealthCards.test.ts`: renders health status cards, shows error state when API fails, shows loading skeletons
  - `DataState.test.ts`: renders slot on success, skeleton on loading, error message on error, empty message on empty
  - Dashboard page: shows welcome message, renders health cards, quick-action buttons
  - Profile page: renders user info, session list, revoke button calls API

---

## Slice 9: Admin — User List & User Detail Pages

- **Backend:** N/A (consumes existing `/admin/users` endpoints)
- **Frontend:**
  - Create `$lib/api/admin.ts` (listUsers, getUser, updateUserRole, suspendUser, restoreUser — if not already in Slice 2)
  - Create `$lib/components/admin/UserTable.svelte` (paginated user list with status filter dropdown, columns: Email, Role, Status, Created, Actions; responsive card layout on mobile)
  - Create `$lib/components/admin/UserDetailCard.svelte` (full user info, role select, suspend/restore buttons)
  - Create `$lib/components/admin/SuspendDialog.svelte` (confirmation dialog with optional reason textarea, max 500 chars, uses Dialog primitive)
  - Create `$lib/components/admin/RoleSelect.svelte` (role dropdown: admin/user/viewer, uses Select primitive)
  - Create `$lib/components/shared/ConfirmDialog.svelte` (generic confirm dialog with title, message, onConfirm, variant)
  - Create `(app)/admin/users/+page.svelte` — User list: SearchInput + status filter + UserTable + Pagination
  - Create `(app)/admin/users/[id]/+page.svelte` — User detail: UserDetailCard with suspend/restore/role change, back link
- **Test:**
  - `UserTable.test.ts`: renders user rows, filters by status, paginates
  - `SuspendDialog.test.ts`: opens on trigger, requires confirmation, passes reason to onConfirm
  - `RoleSelect.test.ts`: renders role options, emits change event
  - Admin user list page: renders users, filter works, pagination works
  - Admin user detail page: renders user info, suspend triggers dialog, role change calls API

---

## Slice 10: Telemetry Pages — Spans, Metrics, Usage

- **Backend:** N/A (consumes existing telemetry endpoints)
- **Frontend:**
  - Create `$lib/api/telemetry.ts` (add spans, metrics, usage functions alongside existing health)
  - Create `$lib/components/telemetry/SpansTable.svelte` (filterable span list: service, operation, status, time range; expandable rows for attributes; pagination)
  - Create `$lib/components/telemetry/MetricsList.svelte` (metric cards/list with name, value, type, timestamp; filter by name and window)
  - Create `$lib/components/telemetry/UsageTable.svelte` (usage events table: agent_id, operation, resource, duration, cost, timestamp; filter controls; cost summary footer; pagination)
  - Create `(app)/telemetry/+page.svelte` — Overview: HealthCards + nav links to sub-pages
  - Create `(app)/telemetry/spans/+page.svelte` — Spans: DataState + SpansTable with filters
  - Create `(app)/telemetry/metrics/+page.svelte` — Metrics: DataState + MetricsList with filters
  - Create `(app)/telemetry/usage/+page.svelte` — Usage: DataState + UsageTable with filters + cost summary
- **Test:**
  - `SpansTable.test.ts`: renders span rows, filters apply, expand shows attributes
  - `MetricsList.test.ts`: renders metric cards, filter controls work
  - `UsageTable.test.ts`: renders usage rows, cost summary footer, filters work
  - Telemetry overview page: renders health cards and nav links

---

## Slice 11: Settings & Error Pages

- **Backend:** N/A
- **Frontend:**
  - Create `(app)/settings/+page.svelte` — Settings page: theme selector grid (6 presets with visual preview), dark/light mode toggle, sidebar default preference
  - Create `(errors)/+page.svelte` — 404 page: "Page not found" message with link to `/`
  - Create `(errors)/+layout.svelte` — Minimal centered layout for error pages
  - Update root `+layout.svelte` to render `<Toaster />` from notificationStore for global toasts
  - Wire up theme selector to `uiStore.setTheme()` and mode toggle to `uiStore.toggleMode()`
- **Test:**
  - Settings page: theme selector applies theme on click, mode toggle switches dark/light, preferences persist on reload
  - 404 page: shows "Page not found" with home link
  - Toaster: renders toast notifications from notificationStore

---

## Slice 12: Integration Tests & Polish

- **Backend:** N/A
- **Frontend:**
  - Create `src/test/integration/auth.test.ts` — End-to-end auth flow: login → redirect to dashboard → logout → redirect to login. Token refresh on expiry. Invalid credentials error display.
  - Create `src/test/integration/admin.test.ts` — Admin flow: list users → navigate to user detail → change role → suspend user → restore user. Role-guard redirects non-admin.
  - Accessibility audit: verify focus management, ARIA landmarks, heading hierarchy, color contrast in all theme presets
  - Responsive audit: verify mobile overlay sidebar, collapsed tablet sidebar, expanded desktop sidebar at all breakpoints
  - Bundle size check: `npm run build` and verify gzipped bundle < 200KB
  - Lighthouse audit: verify all metrics ≥ 90
- **Test:**
  - Integration auth test passes: login/logout/refresh flow works end-to-end
  - Integration admin test passes: CRUD operations + role guard works
  - Bundle size under 200KB gzipped
  - Lighthouse score ≥ 90 on all metrics

---

## Dependency Graph

```
Slice 1 (Scaffolding + Theme)
  └─→ Slice 2 (API Client + Auth Store)
       └─→ Slice 3 (Validation + Form Utils)
            ├─→ Slice 4 (UI Atoms)
            │    └─→ Slice 5 (UI Composites)
            │         └─→ Slice 6 (Layout Shell)
            │              ├─→ Slice 7 (Auth Pages)
            │              └─→ Slice 8 (Dashboard + Profile)
            │                   └─→ Slice 9 (Admin Pages)
            │                        └─→ Slice 10 (Telemetry Pages)
            │                             └─→ Slice 11 (Settings + Error Pages)
            │                                  └─→ Slice 12 (Integration Tests + Polish)
            └─→ Slice 8 (depends on DataState from template, which uses atoms)
```

Slice 3 is independent of Slices 4-5 but must exist before auth pages (Slice 7) use `useForm`.

Slice 6 (Layout) depends on composites (Slice 5) for Dialog/Dropdown in UserMenu, and atoms (Slice 4) for Badge in NavItem.

---

## Notable Cross-Cutting Concerns (applied in every slice)

- **Accessibility:** Skip-to-content link, ARIA landmarks, focus-visible, heading hierarchy, form labels, `aria-live` for loading states
- **Responsive:** Every page tested at mobile (<768px), tablet (768–1024px), desktop (>1024px)
- **Theme:** All components use CSS custom property tokens (bg-background, text-foreground, etc.), never hardcoded colors
- **Error handling:** All API calls follow DataState pattern (loading → success | error)
- **No `any`:** Explicit types throughout, no `interface{}`
- **No else:** Early returns only