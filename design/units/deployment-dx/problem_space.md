# Problem Space: Deployment & Developer Experience

**Unit:** deployment-dx  
**Status:** Discovery  
**Date:** 2026-04-12

---

## 1. Problem Statement

ACE currently operates as a complex multi-service architecture requiring Docker/Podman orchestration with 10+ containers (PostgreSQL, NATS, Valkey, Prometheus, Grafana, Loki, Tempo, OTEL Collector, API, Frontend). This creates significant friction for developers and end users:

- **Developer Onboarding**: New contributors need Docker knowledge and 30+ second cold starts
- **Resource Footprint**: Multi-gigabyte memory/disk requirements
- **Deployment Complexity**: Cannot simply "download and run"
- **Testing Friction**: Tests require container orchestration
- **Git Hook Latency**: Pre-commit hooks take too long

The core conflict: External dependencies provide battle-tested reliability but create operational complexity that hinders adoption. The goal is a single-binary deployment model that maintains functionality while removing friction.

---

## 2. Constraints

### Technical Constraints
- Must remain cross-platform (Linux primary, macOS secondary)
- Single binary deployment model - no external daemons required for default case
- Preserve existing API contracts and interface boundaries
- Go 1.26 baseline
- SvelteKit frontend (currently uses adapter-node)

### Operational Constraints
- Hot reload must be preserved or improved for development
- Optional enterprise hooks must remain available (external DB, messaging, observability)
- Configuration must distinguish dev vs prod environments simply
- No Windows support required for initial release

### UX Constraints
- Installation must work with single curl command
- Binary must be runnable as `ace` command after install
- Data directory must follow platform conventions
- Tests must complete fast enough for git hooks

---

## 3. Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Installation steps | 10+ (Docker, compose, env setup) | 1 (`curl \| sh`) |
| Cold start time | 30+ seconds | <5 seconds |
| Binary count | 10+ containers | 1 binary |
| Git hook duration | >60 seconds | <30 seconds |
| Disk footprint | 2GB+ | <200MB |
| Memory footprint | 1GB+ | <200MB |

---

## 4. Scope Definition

### In Scope
- Consolidate multi-module Go workspace to single binary
- Replace external PostgreSQL with embedded database
- Replace external NATS with internal messaging
- Replace external Valkey with internal caching
- Replace external observability stack with custom telemetry
- Embed frontend assets in Go binary
- Simplify Makefile to `make ace`, `make test`
- Create curl-based installation script
- Update git hook to call `make test` efficiently
- Remove devops/ folder and container orchestration
- Remove changelogs/ folder
- Simplify environment configuration

### Out of Scope
- Windows support (Unix/Linux only)
- Backwards compatibility with existing Docker deployments
- Migration tools from existing PostgreSQL databases (fresh start)

---

## 5. Key Questions for Research Phase

The following areas require independent research and evaluation:

1. **Embedded Database**: What options exist for Go-embedded databases? Evaluate all candidates for ACID compliance, performance, migration path from PostgreSQL, SQL compatibility.

2. **Internal Messaging**: What patterns/libraries exist for in-process messaging? Evaluate durability needs, pub/sub capabilities, request-reply patterns.

3. **Internal Caching**: What Go caching solutions exist? Evaluate TTL support, eviction policies, memory limits, performance characteristics.

4. **Installation Standards**: What are industry-standard practices for CLI tool installation? Evaluate directory conventions (XDG vs alternatives), PATH modification, security considerations.

5. **Data Directory Standards**: Where should application data be stored? Evaluate platform conventions, permissions, backup considerations.

6. **Frontend Embedding**: What patterns exist for embedding SPAs in Go binaries? Evaluate build tag approaches, development vs production workflows, SPA routing.

7. **Custom Telemetry Architecture**: How to build internal telemetry without external collectors? Evaluate storage, querying, optional export to enterprise systems.

8. **Build System Consolidation**: How to consolidate go.work multi-module structure? Evaluate single module vs keeping shared packages, import paths.

9. **Test Optimization**: How to speed up test execution? Evaluate parallelization, test categorization, integration vs unit test separation.

10. **Git Hook Efficiency**: How to integrate doc generation into test flow without slowing hooks? Evaluate make target organization, conditional execution.

---

## 6. Risk Analysis

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Embedded database performance doesn't match PostgreSQL | Medium | High | Configuration flag to use external PostgreSQL |
| Single binary becomes too large | Medium | Medium | Compression, optional components |
| Loss of observability UX (Grafana dashboards) | High | Medium | Build simple debug endpoints, future dashboard |
| Migration complexity from current architecture | Medium | High | Incremental slices, preserve interfaces |
| `curl \| sh` security concerns | Low | Medium | Checksum verification, standalone verification script |
| Tests remain slow despite optimization | Medium | High | Categorize tests (unit/integration), parallel execution |

---

## 7. Next Steps

After this document is merged, the Architect will research all available options for the key questions above and produce a **Research Document** with findings and recommendations.
