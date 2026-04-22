# Frontend Design System

**Unit**: frontend-design  
**Status**: ✅ Complete  
**Stack**: SvelteKit + Svelte 5 + Tailwind CSS v4 + shadcn-svelte

---

## Overview

The ACE Framework frontend is a modern, themeable, responsive SPA built with SvelteKit. It features an OS-style authentication system, 45 theme presets, and a complete component library based on shadcn-svelte.

---

## Architecture

### Technology Stack

| Technology | Purpose |
|------------|---------|
| **SvelteKit** | Full-stack framework with SPA adapter |
| **Svelte 5** | Reactive UI with runes ($state, $effect, $derived) |
| **Tailwind CSS v4** | Utility-first styling |
| **shadcn-svelte** | Accessible component primitives |
| **Zod** | Schema validation |
| **Lucide** | Icon library |

### Project Structure

```
frontend/src/
├── lib/
│   ├── api/           # API client and types
│   ├── realtime/      # Real-time: manager, connection, reconnect, polling, topics
│   ├── components/
│   │   ├── layout/    # Sidebar, AppShell, NavItem
│   │   ├── telemetry/ # HealthCards
│   │   ├── shared/    # DataState, Pagination
│   │   └── ui/        # shadcn components
│   ├── stores/        # Svelte 5 rune-based stores (auth, agents, usage, notifications, ui)
│   ├── themes/        # 45 theme definitions
│   ├── utils/         # Helpers, formatters, constants
│   └── validation/    # Zod schemas
├── routes/
│   ├── (app)/         # Authenticated routes
│   │   ├── +page.svelte (Dashboard)
│   │   ├── profile/
│   │   ├── sessions/
│   │   ├── admin/
│   │   ├── telemetry/
│   │   └── settings/
│   ├── (auth)/        # Unauthenticated routes
│   │   ├── login/
│   │   ├── login/[username]/
│   │   ├── setup/
│   │   └── register/
│   └── (errors)/      # Error pages
│       └── 404/
└── test/              # Unit and integration tests
```

---

## Authentication

### OS-Style Login Flow

1. **User List** (`/login`): Shows all users as selectable avatars (like Windows/Mac login)
2. **PIN Entry** (`/login/[username]`): Enter 4-6 digit PIN for selected user
3. **Setup** (`/setup`): First user registration (becomes admin automatically)
4. **Register** (`/register`): Additional user registration (requires existing admin)

### Auth Store

```typescript
// lib/stores/auth.svelte.ts
authStore.login(username, pin)     // Login with PIN
authStore.register(username, pin)  // Register new user
authStore.logout()                  // Clear session
authStore.refreshTokens()           // Rotate tokens
```

---

## Theme System

### 45 Theme Presets

Dark themes: One Dark, Nord, Catppuccin Mocha, Monokai, Gruvbox Dark, Tokyo Night, Dracula, Ayu Dark, Everforest, Kanagawa, Rose Pine, Solarized Dark, Night Owl, Palenight, GitHub Dark, Cobalt2, Material, Shades of Purple, Synthwave '84, Vercel, Zenburn, Flexoki, Mercury, Osaka Jade, Cursor, Lucent Orng, Orng, Aura, OpenCode, One Dark Pro, AMOLED, Carbonfox, Vesper, Matrix, and more.

Light themes: One Light, Catppuccin Latte, Gruvbox Light, Ayu Light, Solarized Light, and all dark themes have light variants.

### CSS Variables

All themes use CSS custom properties:
```css
--background: 220 20% 10%;
--foreground: 220 10% 92%;
--primary: 262 83% 58%;
/* ... 18 total tokens */
```

### Theme Store

```typescript
uiStore.theme          // Current theme name
uiStore.mode           // 'dark' | 'light'
uiStore.setTheme(name) // Switch theme
uiStore.toggleMode()   // Toggle dark/light
```

---

## Layout System

### Sidebar

- **Expanded**: 256px width with labels
- **Collapsed**: 64px width, icons only
- **Bottom Controls**: Collapse toggle, Settings, User avatar (stacked vertically)
- **Responsive**: Mobile overlay drawer

### Navigation Items

| Route | Icon | Description |
|-------|------|-------------|
| `/` | LayoutDashboard | Dashboard |
| `/agents` | Bot | Agents (disabled) |
| `/chat` | MessageSquare | Chat (disabled) |
| `/memory` | HardDrive | Memory (disabled) |
| `/telemetry` | Activity | Telemetry |
| `/admin/users` | Shield | Admin (admin only) |
| `/settings` | Settings | Settings |

---

## Components

### shadcn-svelte Components

Button, Card, Input, Badge, Select, Dialog, Tabs, Table, Avatar, Skeleton, Separator, Switch, Toast, Sheet, Dropdown Menu

### Custom Components

| Component | Purpose |
|-----------|---------|
| `AppShell` | Layout wrapper with sidebar + main content |
| `Sidebar` | Collapsible navigation (includes ConnectionIndicator) |
| `NavItem` | Navigation link with icon |
| `HealthCards` | System health status display (real-time via WebSocket) |
| `DataState` | Loading/error/empty state wrapper |
| `Toaster` | Global toast notifications (includes connection status toasts) |
| `ConnectionIndicator` | Real-time connection status badge |
| `LiveBadge` | Pulsing green dot for active connections |

---

## API Client

### Features

- Token management with automatic refresh
- Request/response interceptors
- Error normalization
- Envelope unwrapping ( `{success, data}` → `data` )

### Usage

```typescript
import { apiClient } from '$lib/api/client';
import { login } from '$lib/api/auth';

// Automatic token injection
const tokens = await login(username, pin);

// Automatic refresh on 401
const user = await getCurrentUser();
```

---

## Forms

### useForm Composable

```typescript
import { useForm } from '$lib/utils/form.svelte';
import { loginSchema } from '$lib/validation/schemas';

const form = useForm({
  schema: loginSchema,
  initialValues: { username: '', pin: '' },
  onSubmit: async (values) => {
    await authStore.login(values.username, values.pin);
  }
});
```

---

## Testing

### Test Coverage

- **331 tests** across 30 test files (includes realtime unit tests)
- Unit tests for stores, API, utilities
- Integration tests for auth flow
- Component tests for layout

### Run Tests

```bash
make test
```

---

## Environment

### Development

```bash
cd frontend
npm run dev        # Vite dev server on :5173
npm run test:run   # Run vitest
npm run check      # svelte-check
```

### Production Build

```bash
cd frontend
npm run build      # Static adapter output
```

---

## Related Documentation

- [design/units/frontend-design/problem_space.md](../design/units/frontend-design/problem_space.md)
- [design/units/frontend-design/research.md](../design/units/frontend-design/research.md)
- [design/units/frontend-design/bsd.md](../design/units/frontend-design/bsd.md)
- [design/units/frontend-design/architecture.md](../design/units/frontend-design/architecture.md)
- [design/units/frontend-design/fsd.md](../design/units/frontend-design/fsd.md)
- [design/units/frontend-design/implementation_plan.md](../design/units/frontend-design/implementation_plan.md)
- [realtime.md](./realtime.md) — Real-time UI updates, WebSocket/polling, reconnection
