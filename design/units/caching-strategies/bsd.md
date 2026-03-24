# Business Specification Document

## Feature Name
Caching Strategies

## Problem Statement
The ACE Framework is a distributed multi-service system where autonomous agents execute cognitive cycles continuously — making LLM calls, querying memory, reading tool/skill definitions, and accessing shared resources. Without a shared caching foundation, each service will cache data independently, inventing its own cache key conventions, TTL strategies, invalidation logic, and consistency guarantees. The result is a system that is impossible to reason about as a whole: data freshness cannot be guaranteed across services when every service speaks a different caching dialect.

This fragmentation creates three concrete problems:
1. **Excessive latency** — Memory tiers, tool definitions, skill configurations, and cognitive layer context are accessed repeatedly and incur full round-trip delays on every access.
2. **Unnecessary LLM costs** — Semantically identical prompt completions, embedding results, and retrieved context blocks are re-invoked rather than reused.
3. **Redundant cognitive engine work** — The 6 cognitive layers process continuously, and without per-layer caching, intermediate results (decision trees, alignment evaluations, strategy computations) are recomputed across iterations.

## Solution
Establish `shared/caching` as a foundational infrastructure package — analogous to `shared/messaging` and `shared/telemetry` — before any feature service that needs caching is built. The unit delivers both a design specification (patterns, principles, guidelines) and a shared Go library providing reusable caching primitives.

The shared library defines interfaces and patterns, not implementations. Valkey is the sole cache backend and runs in all environments including local development via Docker Compose. Every service that needs caching imports `shared/caching` and inherits consistent cache key conventions, invalidation strategies, observability integration, and stampede protection — rather than inventing its own.

## In Scope
- **Shared caching library (`shared/caching`)** — Transport-agnostic Go package with core operations (`Get`, `Set`, `Delete`), cache-aside pattern (`GetOrFetch`), bulk operations, TTL management, and namespace isolation
- **Frontend caching module** — SvelteKit/browser-side caching module for UI data, API response caching, and client-side state management
- **Backend tool selection** — Research and evaluation of cache backends (Valkey vs alternatives) with documented recommendation
- **Invalidation primitives** — First-class support for TTL-based, event-driven, versioned, and hybrid invalidation strategies as composable building blocks
- **Stampede protection** — Built-in single-flight/request coalescing to prevent thundering herd when popular cache keys expire
- **Cache warming** — Configurable per-namespace pre-population on service startup with optional NATS-triggered warming events
- **Observability integration** — Mandatory UsageEvent emission on all cache operations (hits, misses, writes, invalidations, evictions) via `shared/telemetry`; new operation types and resource types as needed
- **Multi-agent cache isolation** — agentId threaded through all cache keys, traces, and UsageEvents for attribution and swarm-scale isolation
- **Standardized cache key conventions** — `{namespace}:{agentId}:{entityType}:{entityId}:{version}` format
- **Design specification** — Patterns, principles, and guidelines document for caching across the ACE Framework
- **Testing strategy** — Unit tests, integration tests for cross-service invalidation, load tests for stampede scenarios, consistency tests for distributed invalidation

## Out of Scope
- **Production deployment and sizing** — Cache cluster provisioning, capacity planning, and operational runbooks are deferred to the production deployment unit
- **Cache implementation inside cognitive layers** — The cognitive engine unit will consume `shared/caching` primitives; this unit only provides them
- **Custom NATS subjects for cache invalidation** — Subject definition and subscription wiring live in service-internal adapter code, not in the shared package (transport-agnostic constraint)

## Value Proposition
Building `shared/caching` as foundational infrastructure delivers four compounding benefits:

1. **Consistency** — Every service speaks the same caching dialect. Cache key conventions, invalidation semantics, and observability contracts are uniform. Engineers onboard once and reuse everywhere.
2. **Reliability** — Stampede protection and consistent invalidation prevent the class of caching bugs that plague distributed systems (thundering herd, stale data, cache poisoning).
3. **Observability** — Cache operations are first-class telemetry data powering product features (cost savings dashboards, efficiency indicators) and operational insight (hit rates, eviction patterns, invalidation chains). This mirrors the observability unit's principle that observability drives product features first.
4. **Cost reduction** — Reusing LLM completions, embedding results, and retrieved context blocks directly reduces token spend. Cache hit rate telemetry makes savings measurable and attributable per agent.

Establishing this foundation early prevents the exponential cost of retrofitting caching patterns across services later — the same rationale that drove `shared/messaging` and `shared/telemetry` to be built before the services that consume them.

## Success Criteria
| Criterion | Metric | Target |
|-----------|--------|--------|
| Library adoption | % of backend services using `shared/caching` for all cache operations | 100% — no custom cache implementations |
| Observability coverage | % of cache operations emitting UsageEvents with agentId, namespace, and key pattern | 100% — every Get, Set, Delete, Invalidate, Evict |
| Invalidation consistency | Time from source data change to dependent cache clearance across services | Within defined consistency window per namespace |
| Stampede protection | Concurrent duplicate fetches for the same expired key | 1 maximum (single-flight coalescing) |
| Cache warming | Critical namespaces populated within target time of service startup | Configurable per namespace, default < 5s |
| Cost savings | Estimated LLM cost avoided via cache hits per agent per day | Measurable and attributed via UsageEvents |
