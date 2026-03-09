# Mockups and Designs

<!--
Intent: Define the visual design and UX for the feature.
Scope: UI mockups, wireframes, visual specifications, and design assets.
Used by: AI agents to implement the UI exactly as designed.
-->

## Overview

The core-infra unit is a backend-only unit that provides API services. UI mockups and frontend designs are out of scope for this unit and will be covered in subsequent frontend-focused units.

## Note

This is a **backend-only** unit. The following frontend elements will be designed in future units:

- User interface mockups
- Wireframes for authentication pages
- Agent management UI
- Chat interface design
- Memory browser interface

## Future UI Units

| Unit | Description |
|------|-------------|
| frontend-auth | Login, registration, password reset pages |
| frontend-agents | Agent CRUD UI |
| frontend-chat | Real-time chat interface |
| frontend-dashboard | Main dashboard with agent management |

## Backend API Documentation

For the API design, see [api.md](./api.md).

## Screen Designs (Future)

### Authentication Screens (frontend-auth unit)

- Login page
- Registration page
- Password reset flow

### Agent Management (frontend-agents unit)

- Agent list page
- Agent creation/edit modal
- Agent detail view

### Chat Interface (frontend-chat unit)

- Chat window
- Thought trace panel
- Session history

### Memory Browser (future unit)

- Memory list view
- Search interface
- Memory detail view

## Visual Design System (Future)

When frontend work begins, the design system will include:

- **Color Palette**: Primary, secondary, accent colors
- **Typography**: Font families, sizes, weights
- **Components**: Buttons, inputs, cards, modals
- **Spacing**: Consistent spacing scale
- **Accessibility**: WCAG 2.1 AA compliance

## Design Resources

| Resource | Link | Description |
|----------|------|-------------|
| Figma | TBD | Design files |
| Component Library | TBD | Reusable components |
| Icons | TBD | Icon set |

## Implementation Notes

When implementing frontend in future units:

1. Use consistent component library
2. Follow the design tokens defined
3. Ensure responsive design
4. Test accessibility
5. Support internationalization
