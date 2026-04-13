# Research: Frontend Design

## Overview

ACE's frontend is a SvelteKit SPA served from the Go binary's embedded assets. The backend exposes a REST API with JWT auth, and the frontend must present the agent management interface, chat interface, cognitive layer inspector, and observability dashboards. This document evaluates the technical choices for each frontend concern.

---

## 1. SvelteKit Rendering Mode

### Options Evaluated

| Mode | SSR | Hydration | API Dependency | Auth Complexity | Build Output |
|------|-----|-----------|----------------|-----------------|--------------|
| **SPA (adapter-static)** | No | No | External API | Client-only JWT | Static files |
| **SSR (adapter-node)** | Yes | Yes | Server-side load | Cookie-based session | Node server |
| **Hybrid SSR + CSR** | Partial | Selective | Both | Mixed | Static + server routes |

### Analysis

- **SPA mode**: The backend serves static files. The Go binary already has a `SPAHandler` config in the router. No Node.js server needed. Auth tokens stored client-side (localStorage or memory). API calls from the browser carry JWT via `Authorization: Bearer` header. This matches the existing architecture: single binary, embedded NATS, embedded SQLite.

- **SSR mode**: Would require running a Node.js server alongside the Go binary, breaking the single-binary deployment model. SvelteKit `load` functions would call the Go API, but you'd need cookie-based auth or have the Node server proxy JWTs. Significantly more complex operational model.

- **Hybrid SSR + CSR**: Prerender public pages, hydrate dynamic pages client-side. Adds complexity (`export const ssr = false` per route) for marginal SEO benefit (ACE is an authenticated dashboard, not a public website).

### Recommendation

**SPA mode with `adapter-static`** (already configured in `svelte.config.js`). Reasons:

1. Single-binary deployment is a core architectural constraint. The Go server embeds and serves the frontend build output.
2. ACE is an authenticated dashboard application — SEO is irrelevant.
3. The backend already validates JWTs on every request — the frontend's job is token storage and API calls, not server-side rendering.
4. Hydration mismatch bugs are eliminated entirely when there's no SSR.
5. Simplifies auth: client stores JWT, sends it with every API request. No cookie set/parsing logic needed in SvelteKit.

Trade-off: initial page load is slower (entire JS bundle must download before anything renders). Mitigated by code splitting (SvelteKit does this automatically) and aggressive prefetching via `data-sveltekit-preload-data="hover"` (already set in `app.html`).

---

## 2. State Management

### Options Evaluated

| Approach | Boilerplate | Reactivity | Svelte 5 Native | Testability | Cross-Component |
|----------|-------------|------------|-----------------|-------------|-----------------|
| **Svelte 5 Runes (classes)** | Low | Built-in | Yes | High | Via exported instances |
| **Svelte stores (writable/derived)** | Medium | Built-in | Legacy | Medium | Via imports |
| **External library (Redux-like)** | High | External | No | Low | Via subscriptions |
| **URL state (search params)** | Low | Built-in | Yes | High | Via URL |

### Analysis

Svelte 5 runes (`$state`, `$derived`, `$effect`) replace the old reactive declarations and store patterns. The idiomatic Svelte 5 approach is **class-based stores** using runes:

```typescript
// $lib/stores/auth.svelte.ts
export class AuthStore {
  user = $state<User | null>(null);
  accessToken = $state<string>('');
  isAuthenticated = $derived(this.user !== null);

  async login(email: string, password: string) { ... }
  async logout() { ... }
}
export const authStore = new AuthStore();
```

This pattern provides:
- Zero external dependencies (no Redux, no Zustand, no Jotai)
- Svelte compiler generates fine-grained reactive updates
- Classes are testable in isolation (pure TypeScript)
- `$derived` eliminates manual subscription chains
- State ownership is explicit — you import the instance

Svelte's old `writable`/`derived` stores still work but are the legacy pattern. Runes are the future.

### Recommendation

**Svelte 5 rune-based classes in `.svelte.ts` modules**. Each domain gets its own store file. No external state management library.

Store structure:
- `$lib/stores/auth.svelte.ts` — auth state, tokens, user profile
- `$lib/stores/agents.svelte.ts` — agent list, selected agent, agent state
- `$lib/stores/ui.svelte.ts` — sidebar state, theme, notifications

Rules:
- **Props for component-local state** — form inputs, toggle states, temporary values
- **Stores for cross-component state** — auth, agents, UI shell
- **URL params for shareable state** — selected agent ID, page number, filters

---

## 3. Authentication Flow

### Options Evaluated

| Strategy | Token Storage | CSRF Protection | Logout | SSE Support | XHR Complexity |
|----------|---------------|-----------------|--------|-------------|----------------|
| **JWT in localStorage** | localStorage | N/A (custom header) | Delete token | Header-based auth | Medium |
| **JWT in memory + refresh in cookie** | Memory + HttpOnly cookie | Built-in (SameSite) | Clear memory + call logout | Cookie-based | Low |
| **HttpOnly cookie only** | HttpOnly cookie | CSRF token needed | Clear cookie | Cookie-based | Low |

### Analysis

ACE's backend uses `Authorization: Bearer <token>` for auth (see `auth_middleware.go`). The backend does NOT set cookies — it returns JSON with `access_token` and `refresh_token` in the response body. This means:

1. **localStorage approach**: Simplest. Store both tokens in localStorage. Send access token in Authorization header. Vulnerable to XSS (but ACE controls all scripts — no third-party JS injection in a single-binary deploy).

2. **In-memory + refresh cookie**: More secure against XSS. Access token in JS memory (lost on page refresh). Refresh token would require backend change to set an HttpOnly cookie — currently the backend returns it as JSON.

3. **HttpOnly cookie only**: Would require significant backend changes (the current API returns tokens in JSON, not as Set-Cookie headers). Not backwards-compatible.

### Recommendation

**JWT in memory with localStorage persistence**. Pattern:

1. Login → backend returns `{ access_token, refresh_token, user, expires_in }`
2. Store `access_token` and `refresh_token` in localStorage
3. Load from localStorage into in-memory store on app init
4. Attach `Authorization: Bearer <access_token>` to all API requests via a fetch wrapper
5. On 401 response, attempt refresh via `/auth/refresh`, retry original request
6. On refresh failure, clear storage, redirect to login
7. On explicit logout, call `/auth/logout` (invalidates session), clear storage

This works because:
- ACE is a single-binary deployment with no third-party scripts — XSS risk is minimal
- The backend already expects `Authorization: Bearer` header
- localStorage survives page refreshes (in-memory alone would lose auth on reload)
- The refresh token allows silent token renewal without user interaction

### Token Refresh Strategy

Implement an automatic refresh via an API client wrapper (`$lib/api/client.ts`):

```
Request interceptors:
1. Attach Authorization header from auth store
2. On 401 → attempt refresh via /auth/refresh
3. If refresh succeeds → retry original request with new token
4. If refresh fails → clear auth store, redirect to /login

Timing:
- Check token expiry before each request
- If token expires in <30s, proactively refresh
- Refresh token rotation: backend returns new refresh token on each refresh
```

---

## 4. API Client Layer

### Options Evaluated

| Approach | Type Safety | Error Handling | Auth Integration | Boilerplate |
|----------|-------------|----------------|------------------|-------------|
| **Typed fetch wrapper** | Manual | Centralized | Built-in | Medium |
| **OpenAPI codegen** | Auto-generated | Per-endpoint | Manual | Low |
| **tRPC** | End-to-end | Built-in | Via middleware | Low |
| **Raw fetch calls** | None | Ad-hoc | Per-call | High |

### Analysis

- **Typed fetch wrapper**: Define API functions in `$lib/api/` with full TypeScript types. Centralized error handling, auth token injection, and refresh logic. Most flexible, most maintainable.

- **OpenAPI codegen**: The backend has Swagger/OpenAPI docs. Could auto-generate TypeScript clients. Risk: generated code can be verbose, hard to customize, and the backend's OpenAPI spec is manually maintained (not always in sync).

- **tRPC**: Requires a Node.js server-side runtime. Incompatible with our SPA + Go backend architecture. tRPC needs a TypeScript server.

- **Raw fetch**: No structure, no type safety, duplicated auth logic.recipes for bugs.

### Recommendation

**Custom typed fetch wrapper** at `$lib/api/client.ts` with domain-specific modules:

```
$lib/api/
├── client.ts          # Base fetch wrapper (auth, refresh, error handling)
├── auth.ts            # Login, logout, refresh, me
├── agents.ts          # Agent CRUD, state management
├── sessions.ts        # Session management
├── admin.ts           # Admin endpoints
├── telemetry.ts       # Telemetry data
└── types.ts           # Shared API types (match backend models)
```

The `client.ts` wrapper handles:
- Base URL configuration (from env: `VITE_API_URL`)
- Auth token injection (from auth store)
- Automatic 401 → refresh → retry
- Error normalization (backend returns `{ success, error: { code, message } }`)
- Request/response type generics

---

## 5. Component Library & Styling

### Options Evaluated

| Library | Svelte 5 Support | Headless | Tailwind | Bundle Impact | Style Control |
|---------|-----------------|----------|----------|---------------|---------------|
| **Shadcn-Svelte** | Yes (runes) | Yes (Bits UI) | Yes | Minimal (copy-paste) | Full |
| **Skeleton v3** | Yes | Yes | Yes | Moderate | Theme system |
| **DaisyUI** | Svelte wrapper | No | Yes | Low | CSS variables |
| **Flowbite-Svelte** | Partial | No | Yes | Moderate | Tailwind classes |
| **Custom (Bits UI + Tailwind)** | Yes | Yes | Yes | Minimal | Full |

### Analysis

- **Shadcn-Svelte**: The rising standard for Svelte UI. Copy-paste model — components live in your codebase, no dependency lock-in. Built on Bits UI (accessible headless primitives) + Tailwind. Complete Svelte 5 runes support. Widely adopted pattern from React world.

- **Skeleton v3**: Full design system with built-in theming. Good for rapid prototyping. Has a Pro tier. Opinionated — harder to customize to a non-Skeleton aesthetic. Svelte-first.

- **DaisyUI**: CSS plugin, not a component library. Semantic class names (`btn`, `card`). Works with Tailwind but doesn't provide Svelte component wrappers with proper event handling. The Svelte wrapper (DaisyUI Svelte) is a third-party effort.

- **Custom (Bits UI + Tailwind)**: Maximum control, minimum dependency. Build components on top of Bits UI's accessible primitives. More initial work but no vendored component library constraints.

### Recommendation

**Custom component library built on Bits UI + Tailwind CSS v4**. Adopt the shadcn-svelte pattern (copy-paste components into `$lib/components/ui/`) but write our own to match ACE's design language.

Reasons:
1. ACE needs a bespoke design language (agent management, cognitive layer inspector, cognitive cycle debug views) — no off-the-shelf library has these
2. Bits UI handles accessibility (ARIA, keyboard navigation, focus management) — we don't reinvent that
3. Tailwind v4 for utility-first styling — consistent with the Go ecosystem's preference for convention over configuration
4. Copy-paste model means we own the code and can modify freely — no dependency upgrade pain
5. Shadcn-svelte's CLI can bootstrap common components (Button, Dialog, Input, etc.) which we then customize

Component structure:
```
$lib/components/
├── ui/                 # Primitives (Button, Input, Dialog, etc.)
│   ├── button/
│   ├── input/
│   ├── dialog/
│   ├── select/
│   └── ...
├── layout/             # App shell components
│   ├── Sidebar.svelte
│   ├── Header.svelte
│   └── AppShell.svelte
├── auth/               # Auth-related components
│   ├── LoginForm.svelte
│   └── AuthGuard.svelte
├── agents/             # Agent management components
│   ├── AgentCard.svelte
│   ├── AgentList.svelte
│   └── AgentDetail.svelte
└── chat/               # Chat interface components
    ├── MessageList.svelte
    ├── MessageInput.svelte
    └── ChatPanel.svelte
```

---

## 6. Real-Time Communication

### Options Evaluated

| Approach | Direction | Complexity | Browser Support | Reconnection | ACE Fit |
|----------|-----------|------------|-----------------|--------------|---------|
| **SSE** | Server→Client | Low | Native | Automatic | Good |
| **WebSocket** | Bidirectional | High | Native | Manual | Overkill |
| **Polling** | Server→Client | Lowest | Universal | N/A | Fallback |
| **NATS WS** | Bidirectional | Highest | Requires WS support | Manual | Best |

### Analysis

ACE's cognitive engine communicates via NATS internally. The frontend needs to receive real-time updates about:
- Agent state changes (idle → processing → error)
- Cognitive cycle progress (layer processing updates)
- Chat messages (agent responses streaming)
- Usage/cost events

For the frontend, the data flow is primarily **server → client**. The client doesn't need to push arbitrary data to the server — it makes REST API calls for commands (create agent, send message, etc.) and receives streaming updates for observations.

**SSE (Server-Sent Events)**:
- Perfect for server→client streaming
- Native browser support (EventSource API)
- Automatic reconnection built into the protocol
- HTTP-based (works with existing auth middleware — JWT in query param for SSE connections)
- Scales well for moderate connection counts
- SvelteKit can serve SSE endpoints via `+server.ts` GET handlers returning ReadableStream

**WebSocket**:
- Bidirectional, but ACE doesn't need client→server push
- Requires separate WS server or upgrade handling in Go
- Connection state management is more complex
- JWT auth over WS requires first message or query param

**NATS WebSocket**:
- ACE already uses NATS internally
- NATS supports WebSocket connections (`nats.ws`)
- Would allow frontend to subscribe directly to NATS subjects
- Requires exposing NATS WS endpoint and managing client subscriptions
- More complex auth (need to restrict subjects per user/role)

### Recommendation

**SSE for real-time updates in Phase 1, NATS WebSocket for Phase 2**.

Phase 1 (initial build):
- Go backend exposes `/api/events` SSE endpoint
- Frontend connects with JWT token (query param for EventSource, or fetch-based SSE with Authorization header)
- Backend bridges NATS messages to SSE events per user/agent
- SvelteKit client uses a typed SSE client module

Phase 2 (advanced):
- Expose NATS WebSocket endpoint for direct subscription
- Frontend subscribes to specific subjects based on user's agents
- More granular control over which events the client receives
- Reduces backend SSE bridging code

SSE client pattern:
```typescript
// $lib/api/stream.ts
export class EventStream {
  private source: EventSource | null = null;
  
  connect(token: string) {
    this.source = new EventSource(`/api/events?token=${token}`);
    this.source.onmessage = (event) => { ... };
  }
  
  disconnect() {
    this.source?.close();
  }
}
```

---

## 7. Routing & Code Organization

### Recommended File Structure

```
frontend/src/
├── app.html
├── app.d.ts
├── routes/
│   ├── +layout.svelte              # App shell (sidebar, header)
│   ├── +page.svelte                # Dashboard / home
│   ├── (auth)/                     # Auth group (no layout wrap)
│   │   ├── login/+page.svelte
│   │   └── register/+page.svelte
│   ├── (app)/                      # Authenticated app group
│   │   ├── +layout.svelte          # Auth guard + app shell
│   │   ├── +layout.ts              # ssr = false
│   │   ├── agents/
│   │   │   ├── +page.svelte        # Agent list
│   │   │   └── [id]/+page.svelte   # Agent detail
│   │   ├── chat/
│   │   │   └── [agentId]/+page.svelte  # Chat with agent
│   │   ├── inspector/
│   │   │   └── [agentId]/+page.svelte  # Cognitive layer inspector
│   │   └── settings/+page.svelte
│   └── (public)/                   # Public pages (if any)
├── lib/
│   ├── api/                        # API client layer
│   │   ├── client.ts               # Base fetch wrapper
│   │   ├── auth.ts
│   │   ├── agents.ts
│   │   ├── sessions.ts
│   │   ├── admin.ts
│   │   ├── telemetry.ts
│   │   └── types.ts                # Shared API types
│   ├── stores/                     # Svelte 5 rune stores
│   │   ├── auth.svelte.ts
│   │   ├── agents.svelte.ts
│   │   ├── ui.svelte.ts
│   │   └── notifications.svelte.ts
│   ├── components/                 # UI components
│   │   ├── ui/                     # Primitive components
│   │   ├── layout/                 # App shell components
│   │   ├── auth/                   # Auth components
│   │   ├── agents/                # Agent components
│   │   └── chat/                  # Chat components
│   ├── telemetry/                  # OpenTelemetry (exists)
│   └── utils/                      # Shared utilities
│       ├── formatter.ts
│       └── constants.ts
└── test/
```

Key conventions:
- `(auth)` group: unauthenticated pages, no sidebar
- `(app)` group: authenticated pages, with `ssr = false` in layout
- `(public)` group: public pages like landing (if needed later)
- `$lib/api/`: all backend communication
- `$lib/stores/`: `.svelte.ts` rune stores only
- `$lib/components/ui/`: primitive, headless-based components

---

## 8. Form Handling & Validation

### Options Evaluated

| Library | Svelte 5 Support | Validation | Type Safety | Bundle Size |
|---------|-----------------|------------|-------------|-------------|
| **Superforms** | Yes (SvelteKit actions) | Zod/Valibot | End-to-end | Moderate |
| **Svelte 5 native** | Native | Manual | Manual | Zero |
| **Felte** | Yes | Yup/Zod/Custom | Partial | Small |

### Analysis

- **Superforms**: SvelteKit-native form library with server-side validation via form actions. Excellent type inference. Tightly integrated with SvelteKit's `+page.server.ts` and form actions. However, ACE uses SPA mode (`ssr = false`), which means no SvelteKit server-side form actions. Superforms would need to be used in client-only mode, losing its server-side validation advantage.

- **Svelte 5 native**: Use `$state` for form fields, write validation functions manually. Zero dependencies. Most flexible. Requires more boilerplate for each form.

- **Felte**: Lightweight, framework-agnostic form handler. Works with any validation library. Good Svelte integration.

### Recommendation

**Custom form composable using Svelte 5 runes** with Zod for validation. Pattern:

```typescript
// $lib/utils/form.svelte.ts
export function useForm<T extends Record<string, unknown>>(
  initialValues: T, 
  schema: ZodSchema<T>
) {
  const values = $state(initialValues);
  const errors = $state<Partial<Record<keyof T, string>>>({});
  const touched = $state<Partial<Record<keyof T, boolean>>>({});
  
  const isValid = $derived(Object.keys(errors).length === 0);
  const isDirty = $derived(/* ... */);
  
  function validate() { /* zod validate */ }
  function reset() { /* reset to initial */ }
  function handleSubmit(fn: (values: T) => Promise<void>) { /* ... */ }
  
  return { values, errors, touched, isValid, isDirty, validate, reset, handleSubmit };
}
```

Why custom over Superforms:
1. SPA mode means no SvelteKit form actions
2. ACE's auth API expects JSON POST, not form-encoded data
3. Validation happens via Zod schemas that can be shared with types
4. Full control over error display, loading states, and submission flow

---

## 9. Testing Strategy

### Options Evaluated

| Tool | Scope | Svelte 5 Support | Speed | Ecosystem |
|------|-------|-----------------|-------|-----------|
| **Vitest** | Unit/Integration | Yes | Fast | Excellent |
| **Testing Library** | Component | Yes (v5) | Medium | Good |
| **Playwright** | E2E | Framework-agnostic | Slow | Excellent |
| **Storybook** | Visual | Partial | N/A | Good |

### Recommendation

**Vitest + Testing Library + Playwright**, layered approach:

1. **Unit tests** (Vitest): Stores, utils, API client functions
2. **Component tests** (Vitest + Testing Library): Individual components with `render()` and `fireEvent()`
3. **E2E tests** (Playwright): Critical paths (login, agent creation, chat interaction)

Current setup already uses Vitest (see `vitest.config.ts`). Extend it.

Test file convention: co-located (`Component.test.ts` next to `Component.svelte`).

---

## 10. Accessibility & Internationalization

### Accessibility (a11y)

Bits UI components ship with ARIA attributes and keyboard navigation. Apply these rules:
- All interactive elements must be reachable by keyboard
- Use semantic HTML (`<button>`, `<nav>`, `<main>`, `<aside>`)
- ARIA labels on icon-only buttons
- Focus management in modals and route changes
- Color contrast ratios meet WCAG 2.1 AA

### Internationalization (i18n)

ACE's initial audience is English-speaking developers. i18n is **not in scope for Phase 1**. Structure strings to make future i18n extraction easy (no inline strings in component markup, use a constants file or `$lib/utils/strings.ts`). Revisit when multi-language support is a product requirement.

---

## 11. Key Architectural Decisions Summary

| Decision | Choice | Rationale |
|----------|-------|-----------|
| Rendering mode | SPA (adapter-static) | Single-binary deploy, no Node.js server, no SSR bugs |
| State management | Svelte 5 runes (class stores) | Zero deps, compiler-optimized, idiomatic, testable |
| Auth storage | JWT in localStorage → in-memory | Matches backend Bearer token auth, simple refresh flow |
| API client | Custom typed fetch wrapper | Full control, matches backend JSON envelope, centralized auth |
| Component library | Custom on Bits UI + Tailwind v4 | Bespoke UI needs, accessibility, ownership, no lock-in |
| Real-time | SSE (Phase 1), NATS WS (Phase 2) | Server→client fits SSE, upgrade path to direct NATS |
| Form handling | Custom runes + Zod | SPA mode negates Superforms advantage, API expects JSON |
| Testing | Vitest + Testing Library + Playwright | Layered, matches existing setup |
| i18n | Deferred | Not Phase 1, but string-friendly structure required |
| CSS | Tailwind CSS v4 | Utility-first, matches component copy-paste model, fast iteration |

---

## 12. Trade-offs & Risks

### Accepted Trade-offs

1. **SPA over SSR**: Faster initial development, simpler deployment, but slower initial page load. Acceptable for authenticated dashboard app.

2. **localStorage for JWT**: Vulnerable to XSS in theory, but ACE controls all scripts and deploys as single binary. The alternative (in-memory + refresh cookies) requires backend changes we can defer.

3. **Custom components over library**: More upfront work, but zero dependency lock-in and total design control. Bits UI gives accessibility primitives for free.

4. **SSE over WebSocket**: Simpler, fits the server→client pattern. Cannot push client→server via SSE, but REST API calls handle that direction.

### Risks

1. **Bundle size**: Custom components + Tailwind can bloat if not tree-shaken. Mitigate with Tailwind purging and component-level imports.

2. **Svelte 5 runes maturity**: Runes are stable but ecosystem (component libraries, tooling) is still catching up. Mitigate by using Bits UI (already runes-native) and writing custom components (no legacy dependency).

3. **SSE connection limits**: Browsers limit ~6 SSE connections per domain. ACE needs at most 1-2 connections per user (agent state + chat stream). Not a practical concern.

4. **Token refresh race conditions**: Multiple concurrent requests hitting 401 simultaneously could trigger multiple refreshes. Mitigate with a refresh mutex (single in-flight refresh promise, all queued requests await it).

5. **Type sync between frontend and backend**: API types are manually maintained. Future: generate TypeScript types from backend OpenAPI spec as a build step.