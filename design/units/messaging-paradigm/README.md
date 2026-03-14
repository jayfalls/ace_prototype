# Messaging Paradigm

## Status: Problem Space In Review

## Overview

Establishes the communication contract for how all services in the ACE Framework communicate with each other. This foundational unit defines message formats, NATS subject naming conventions, and the shared messaging wrapper that every future service will depend on.

## Documents

| Document | Status |
|----------|--------|
| [Problem Space](problem_space.md) | In Review |
| [BSD](bsd.md) | TODO |
| [User Stories](user_stories.md) | TODO |
| [Research](research.md) | TODO |
| [FSD](fsd.md) | TODO |
| [Architecture](architecture.md) | TODO |
| [Implementation](implementation.md) | TODO |

## Key Decisions

- Message envelope fields in NATS headers for efficient routing/filtering
- Subject format: `ace.<domain>.<agentId>.<subsystem>.<action>`
- agentId mandatory on all cognitive messages
- System messages: `ace.system.<subsystem>.<action>`
- JSON for payloads (operational simplicity)
- JetStream for persistence and durability
- Shared wrapper handles connection, reconnection, drain, health checks

## Dependencies

- **Before**: Core API
- **After**: Observability, Auth, all feature services

This unit must be completed before any service beyond the API is built.
