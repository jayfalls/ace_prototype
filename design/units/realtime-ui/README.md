# Real-time UI Updates & Retry Mechanisms

Status: Complete

## Overview

This unit delivers a rock-solid real-time user experience where all data is always live through a seamless combination of WebSockets and polling. The system ensures users never need to refresh and automatically rectifies state when network or performance issues stabilize.

## Problem Space

See [problem_space.md](./problem_space.md)

## Documents

- [Problem Space](./problem_space.md) - Core conflict, constraints, and success metrics
- [Research](./research.md) - Comparative analysis of WebSocket libraries, transport strategies, NATS bridging, reconnection patterns, and polling fallback
- [Architecture](./architecture.md) - System design: Hub+Client+TopicReg backend bridge, RealtimeManager frontend, message types, auth flow, reconnection, and observability
- [Implementation Plan](./implementation_plan.md) - 11 vertical slices: message types → TopicReg → Hub/Client → WS handler → RealtimeManager → reconnect/polling → UI components → store integration → observability → end-to-end → integration tests

## Implementation

- 11 slices completed, 331 tests total
- Backend: `backend/internal/api/realtime/` (Hub, Client, TopicReg, SeqBuffer, Handler, Config)
- Frontend: `frontend/src/lib/realtime/` (RealtimeManager, Connection, Reconnect, Polling, Topics)
- Stores: `frontend/src/lib/stores/agents.svelte.ts`, `frontend/src/lib/stores/usage.svelte.ts`
- UI: `frontend/src/lib/components/realtime/` (ConnectionIndicator, LiveBadge)
- Integration: `backend/internal/api/realtime/integration_test.go`, `frontend/src/test/integration/realtime.test.ts`

## Related Units

- [Frontend Design](../frontend-design/README.md) - SvelteKit foundation
- [Messaging Paradigm](../messaging-paradigm/README.md) - NATS communication patterns
- [Observability](../observability/README.md) - Connection health and sync status
