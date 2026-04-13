# Frontend Design

**Status**: Discovery  
**Unit ID**: frontend-design

## Overview

Building the complete frontend design system, component library, and all pages for the ACE Framework. This unit covers design system architecture, theme engine with multiple presets, atomic component library, responsive navigation, and integration with all existing backend APIs.

## Documents

| Document | Status | Description |
|----------|--------|-------------|
| [Problem Space](problem_space.md) | ✅ Complete | Core conflict, constraints, and success metrics |
| [BSD](BSD.md) | 🔄 Pending | Bounded Specification Document - research-driven decisions |
| [Architecture](ARCHITECTURE.md) | ⏳ Pending | Technical architecture and patterns |
| [FSD](FSD.md) | ⏳ Pending | File-level specification |
| [Implementation Plan](IMPLEMENTATION_PLAN.md) | ⏳ Pending | Vertical slices and execution order |

## Quick Links

- **Backend APIs**: `/backend/internal/api/handler/`
- **Frontend Source**: `/frontend/src/`
- **OpenAPI Spec**: `/backend/docs/swagger.json`

## Scope Summary

### Included
- Design system with CSS tokens and theme engine
- Component library (atomic design methodology)
- Responsive layout shell with collapsible sidebar
- All page implementations:
  - Authentication (login, register, password reset, magic link)
  - Dashboard
  - User profile and session management
  - Admin panel (user management)
  - Telemetry inspector
- API client generation from OpenAPI
- State management architecture

### Excluded (Future Units)
- Real-time WebSocket updates (next unit)
- Advanced data visualization
- Internationalization (i18n)
- PWA/offline capabilities

## Key Decisions (TBD via Research)

1. Component library choice (shadcn-svelte vs alternatives)
2. Theme engine implementation approach
3. State management pattern (Svelte 5 runes vs stores)
4. API client generation tool
5. Form handling library

## Success Criteria

- [ ] Lighthouse score ≥ 90 (all metrics)
- [ ] Bundle size < 200KB gzipped
- [ ] All 5 page categories functional
- [ ] Theme switching with 5+ presets
- [ ] Full API integration
- [ ] WCAG 2.1 AA accessibility compliance
