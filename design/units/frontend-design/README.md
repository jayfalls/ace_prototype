# Frontend Design

**Status**: Implementation  
**Unit ID**: frontend-design

## Overview

Building the complete frontend design system, component library, and all pages for the ACE Framework. This unit covers design system architecture, theme engine with multiple presets, atomic component library, responsive navigation, and integration with all existing backend APIs.

## Documents

| Document | Status | Description |
|----------|--------|-------------|
| [Problem Space](problem_space.md) | ✅ Complete | Core conflict, constraints, and success metrics |
| [Research](research.md) | ✅ Complete | Technology research and options analysis |
| [BSD](bsd.md) | ✅ Complete | Business Specification Document |
| [Architecture](architecture.md) | ✅ Complete | Technical architecture and patterns |
| [FSD](fsd.md) | ✅ Complete | File-level specification |
| [Implementation Plan](implementation_plan.md) | ✅ Complete | Vertical slices and execution order |

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

## Key Decisions

See [research.md](research.md) and [bsd.md](bsd.md) for details:
- **Component Library**: Custom on Bits UI + Tailwind v4
- **State Management**: Svelte 5 rune-based classes
- **API Client**: Custom typed fetch wrapper
- **Form Handling**: Custom runes composables + Zod
- **Rendering**: SPA with `adapter-static`

## Success Criteria

- [ ] Lighthouse score ≥ 90 (all metrics)
- [ ] Bundle size < 200KB gzipped
- [ ] All 5 page categories functional
- [ ] Theme switching with 5+ presets
- [ ] Full API integration
- [ ] WCAG 2.1 AA accessibility compliance
