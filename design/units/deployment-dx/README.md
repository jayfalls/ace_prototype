# Deployment & Developer Experience Unit

**Status:** Design  
**Goal:** Simplify deployment, development workflow, and end-user experience

## Overview

This unit transforms ACE from a complex multi-container orchestration into a simple, single-binary deployment model while maintaining extensibility for enterprise observability.

## Key Objectives

1. **Single Go Binary**: Consolidate all backend services into one binary
2. **Simplified Make Commands**: `make ace` (build binary with embedded frontend), `make test` (all tests)
3. **Internal Systems**: Replace external dependencies with embedded/internal alternatives
4. **Curl Install**: Single-command installation (`curl | sh`) with `ace` command
5. **Embedded Frontend**: Bundle frontend into the Go binary
6. **Custom Telemetry**: Replace OTEL collectors with custom telemetry system

## Clarifications

### Database Strategy
Research required on Go embedded database options (SQLite vs alternatives).

### Messaging Replacement
Architect to research and recommend after problem space analysis.

### Cache Replacement
Architect to research and recommend (in-memory with TTL vs alternatives).

### Observability/Telemetry
Custom-built telemetry system that:
- Provides core tracing for usage tracking, cost analysis, self-improvement loops
- Can optionally hook into enterprise observability stacks
- Replaces external collectors (Prometheus, Grafana, Loki, Tempo, OTEL)

### Installation Path
Research industry standards for CLI tool installation.

### Data Directory
Research industry standards for application data storage.

### Frontend Strategy
Embed built frontend assets into Go binary using embed package.

## Dependencies to Remove

- Docker/Podman containers (dev/prod)
- PostgreSQL container → embedded DB
- NATS server → internal messaging
- Valkey/Redis → internal cache
- Prometheus/Grafana/Loki/Tempo → custom telemetry
- OTEL Collector → custom telemetry
- changelogs/ folder

## Design Documents

- [Problem Space](problem_space.md) - Problem mapping, constraints, success metrics
- [Research](research.md) - Technology evaluation and options analysis (Architect)
- [BSD](bsd.md) - Business/System Design
- [FSD](fsd.md) - Functional Specification
- [Architecture](architecture.md) - Technical architecture
- [Implementation Plan](implementation.md) - Vertical slices

## Progress

| Document | Status |
|----------|--------|
| Problem Space | Complete |
| Research | Complete |
| BSD | Complete |
| FSD | Complete |
| Architecture | Complete |
| Implementation Plan | Complete |

## Vertical Slices

13 slices defined in [implementation.md](implementation.md). Key ordering:

1. **Module consolidation** (high risk — all imports depend on this)
2. **XDG paths + config + CLI** (foundation for subsystem wiring)
3. **Embedded database** (high risk — data layer)
4. **Embedded NATS** (medium risk — well-documented)
5. **In-process cache** (medium risk — new CacheBackend impl)
6. **Custom telemetry** (medium risk — OTel SDK customization)
7. **Git hook + test optimization** (low risk — independent)
8. **App lifecycle + server** (high risk — integration point)
9. **Frontend embedding** (medium risk — SPA routing + build tags)
10. **Inspector API** (low risk — CRUD endpoints)
11. **Build system** (low risk — tooling)
12. **Install scripts** (low risk — shell scripting)
13. **Cleanup** (low risk — deletions only)
