# Problem Space

## Initial Discovery

### Core Questions

**Q: What problem are we trying to solve?**
A: The ACE Framework is a distributed multi-service system with autonomous agents executing cognitive cycles, making LLM calls, querying memory, reading tools/skills definitions, and accessing shared resources continuously. Without a shared caching foundation established before feature services are built, each service will cache data independently — inventing its own cache key conventions, its own TTL strategies, its own invalidation logic, and its own consistency guarantees. The result is a system that is technically caching in isolation but impossible to reason about as a whole: you cannot guarantee data freshness across services if every service speaks a different caching dialect.

This unit establishes the shared caching primitives in `shared/caching` before any feature service that needs caching is built. Like `shared/messaging` established NATS communication contracts and `shared/telemetry` established observability contracts, this unit establishes the contracts for how every service stores, retrieves, invalidates, and observes cached data. Every subsequent service inherits these primitives rather than inventing its own.

The specific problems being solved are:
1. **Reducing latency for frequently accessed data** — Memory tiers, tool definitions, skill configurations, and cognitive layer context are accessed repeatedly and must not incur full round-trip delays every time.
2. **Reducing LLM token costs** — Cacheable prompt completions, embedding results, and retrieved context blocks should be reused when semantically identical rather than re-invoking the LLM.
3. **Improving cognitive engine performance** — The 6 cognitive layers process continuously, and per-layer caching of intermediate results (decision trees, alignment evaluations, strategy computations) prevents redundant work across iterations.

**Q: Who are the users?**
A:
- **Backend services** (consuming `shared/caching` package) — cognitive engine, memory manager, tool executor, API
- **Frontend** (SvelteKit app consuming browser-side caching module for UI data)
- **Operators/Devs** (observing cache hit rates, eviction patterns, invalidation chains via Grafana)
- **End users** (experiencing faster agent responses, lower costs, smoother interaction)

**Q: What are the success criteria?**
A:
1. Every subsequent service that needs caching uses `shared/caching` — no custom cache implementations
2. Cache operations are observable: hit/miss rates, eviction counts, invalidation chains, and latency are tracked as UsageEvents and exposed via the observability pipeline
3. Cache invalidation is consistent: when data changes in one service, all dependent caches across services are invalidated within defined consistency windows
4. The library uses Valkey as the sole cache backend across all environments, including local development via Docker Compose
5. Stampede protection prevents thundering herd scenarios when popular cache keys expire
6. Cache warming strategies can be configured per-use-case to pre-populate critical data on service startup

**Q: What constraints exist (budget, timeline, tech stack)?**
A:
- **Tech Stack**: Go backend, SvelteKit/TypeScript frontend, PostgreSQL, NATS, existing shared packages (`shared/messaging`, `shared/telemetry`)
- **Transport-agnostic**: `shared/caching` must follow the same pattern as `shared/messaging` and `shared/telemetry` — no imports of `net/http`, NATS client, or any specific transport. HTTP/NATS adapters live in service-internal layers.
- **PostgreSQL for shared state**: Cache metadata, warming schedules, and version stamps can leverage existing PostgreSQL. Actual cached data lives in the cache backend.
- **Mandatory integration with shared/telemetry**: Every cache operation emits UsageEvents with detailed traces. Observability is not optional — it drives product features first, then reliability.
- **agentId everywhere**: All cache keys, traces, and UsageEvents must carry agentId where applicable. Multi-agent cache isolation is a first-class concern.

## Iterative Exploration

### Scope and Architecture

#### 1. Scope: Design Document AND Shared Library
**Q: Should this be a design document, a shared library, or both?**
A: **Both** — a design specification AND a shared library (`shared/caching`), similar to how `shared/messaging` and `shared/telemetry` work. The design document defines the patterns, principles, and guidelines. The shared library provides the reusable Go primitives that all services import. Bespoke shared modules for specific use cases (e.g., a memory-specific caching layer) are also acceptable when the shared primitives do not fit cleanly.

#### 2. Cache Tiers and Locations
**Q: Should the caching system be tied to specific backends or infrastructure?**
A: **Valkey-only** — Valkey is the sole cache backend, running in all environments (local development via Docker Compose, staging, production). The shared library defines the interface and patterns; the backend is always Valkey. Specific configuration (memory limits, eviction policies) is tuned per deployment.

#### 3. Invalidation Strategy
**Q: What cache invalidation approach should be used?**
A: **Support every possible invalidation paradigm** — TTL-based, event-driven, versioned, and hybrid — with solid principles and guidelines. The shared library provides primitives for each approach:
- **TTL-based**: Standard expiration, sliding expiration, stale-while-revalidate
- **Event-driven**: NATS-based invalidation broadcasts when data changes
- **Versioned**: Content-hash or counter-based versioning that detects staleness without explicit invalidation
- **Hybrid**: Combinations (e.g., TTL as safety net + event-driven for speed)

The best approach is determined on a per-feature basis. The library provides the building blocks; the service implements the specific strategy.

#### 4. Observability Integration
**Q: How should caching integrate with observability?**
A: **Everything must be observed** — the more detailed the tracking the better. Core tenet: observability drives product features first, then reliability. All cache operations must be tracked as UsageEvents with detailed traces. This includes:
- Hit/miss rates per namespace, key pattern, and agent
- Eviction counts and reasons
- Invalidation chain tracing (which event invalidated which keys across which services)
- Cache warming progress and success/failure
- Latency breakdowns (backend fetch time vs cache read time)
- Token cost savings from cache hits (estimated LLM cost avoided)

This data powers product features: a cache efficiency dashboard for operators, cost savings indicators for end users, and debugging tools for "why did my agent re-fetch this data?"

#### 5. Shared Library Primitives
**Q: What primitives should be in the shared library?**
A: The research/technical phase should determine the full API surface, but reasonable patterns include:
- **Core Operations**: `Get`, `Set`, `Delete` — basic CRUD
- **Cache-Aside Pattern**: `GetOrFetch` — retrieve from cache or execute fetch function, populate cache, return result
- **Invalidation**: by key, by pattern (glob/regex), by tag, by namespace
- **Bulk Operations**: `GetMany`, `SetMany`, `DeleteMany` — for batch processing
- **TTL Management**: set per-key TTL, sliding expiration, stale-while-revalidate
- **Cache Warming**: pre-populate cache on startup or on schedule, with progress tracking
- **Metrics/Stats**: hit rates, sizes, eviction stats per namespace or globally
- **Namespaces**: logical isolation by service, agent, or feature

#### 6. Testing Strategy
**Q: What testing approach is needed?**
A: **All of the following** — every feature must be rock solid:
- **Unit tests** for cache operations (Valkey mock and test instance)
- **Integration tests** for cross-service invalidation (cache in service A invalidated by event from service B via NATS)
- **Load tests** for stampede scenarios (simultaneous expiry of popular keys)
- **Consistency tests** for distributed invalidation (verify eventual consistency windows)
- The testing phase should also define testing patterns/guidelines for future features that use caching

### Integration with Existing Shared Packages

#### 7. Relationship to shared/messaging
**Q: How does cache invalidation coordinate with NATS?**
A: Cache invalidation events should flow through NATS using `shared/messaging` patterns. When service A updates data, it publishes an invalidation message; services B and C subscribe and clear relevant cache entries. The `shared/caching` library should provide optional NATS integration helpers (adapters in service-internal code, not in the shared package itself) that wire up invalidation subscriptions. The shared package defines the invalidation event shape; the service wires it to NATS subjects.

#### 8. Relationship to shared/telemetry
**Q: How does cache observability integrate with the telemetry pipeline?**
A: Every cache operation that crosses a latency threshold or represents a state change (hit, miss, set, delete, invalidate, evict) emits a UsageEvent via `shared/telemetry`. A new operation type (`OperationTypeCacheHit`, `OperationTypeCacheMiss`, `OperationTypeCacheWrite`, `OperationTypeCacheInvalidate`) and resource type (`ResourceTypeCache`) may need to be added to the telemetry package. Trace spans for cache operations carry agentId, namespace, and key pattern as attributes.

#### 9. Relationship to Memory Architecture
**Q: How does caching relate to the 4-tier memory system?**
A: The memory unit has its own memory tiers (L1-L4). Caching operates at a different layer — it caches the results of fetching from memory, not the memory tiers themselves. For example, an L4 query result that is frequently accessed can be cached to avoid repeated PostgreSQL tree traversals. The memory manager and the caching system are complementary: memory manages the cognitive state; caching optimizes the retrieval of that state.

## Key Insights

1. **Caching is foundational infrastructure, not a feature** — like messaging and telemetry, it must be established as a shared primitive before any service that needs it is built. Retrofitting caching patterns across services is as costly as retrofitting observability or messaging.

2. **Observability is the primary driver** — cache operations must be observable not just for reliability monitoring but for product features (cost savings dashboards, efficiency indicators). This mirrors the observability unit's principle of "observability drives product features first, then reliability."

3. **Valkey runs everywhere** — Valkey is the sole cache backend, running in all environments including local development via Docker Compose. One client, one backend, all environments — no abstraction layer needed.

4. **Invalidation is the hard problem** — supporting TTL-based, event-driven, versioned, and hybrid invalidation as first-class primitives (not afterthoughts) is the key differentiator. Most caching failures come from invalidation bugs, not from caching bugs.

5. **Cross-service invalidation requires NATS coordination** — the messaging paradigm is already established; cache invalidation events should flow through NATS subjects using existing `shared/messaging` patterns. This ensures independent service deployability.

6. **agentId threading through cache keys enables multi-agent isolation** — in a swarm of 1000+ agents, each agent's cache must be isolated. Without agentId in cache keys, agents pollute each other's caches and attribution breaks.

7. **Stampede protection is non-negotiable** — the cognitive engine processes continuously, and when a popular cache key expires, hundreds of concurrent loops could all attempt to recompute simultaneously. Single-flight/coalescing patterns must be built into the library.

8. **The shared library must be transport-agnostic** — following the established pattern from `shared/messaging` and `shared/telemetry`, the caching package must not import NATS, HTTP, or any transport. NATS integration adapters live in service-internal code.

## Dependencies Identified

- **Valkey client (valkey-go)** — sole cache backend client (research phase)
- **shared/messaging** — for NATS-based cache invalidation event flow (integration pattern, not package dependency)
- **shared/telemetry** — for UsageEvent emission, trace spans, and metrics on all cache operations
- **PostgreSQL** — for cache metadata, warming schedules, version stamps (queries via SQLC)

## Assumptions Made

1. The `shared/caching` package will be imported by all backend services that need caching (cognitive engine, memory manager, tool executor, API)
2. The frontend will have a separate browser-side caching module (not Go) — designed but implementation deferred to the frontend unit
3. NATS-based invalidation will use dedicated subjects (e.g., `ace.cache.%s.invalidate`) — subject constants defined in the caching package or a caching-specific adapter
4. Valkey is the chosen cache backend for all environments including local development
5. Cache key conventions will be standardized: `{namespace}:{agentId}:{entityType}:{entityId}:{version}`
6. Stale-while-revalidate pattern will be the default for most use cases — serve stale data immediately, refresh in background
7. Cache warming will be configurable per-namespace with optional NATS-triggered warming events

## Open Questions (For Research)

1. **Valkey configuration**: Optimal memory limits, eviction policies, and cluster sizing for ACE workloads
2. **Cross-service invalidation protocol**: Should invalidation be key-based, tag-based, or namespace-based? What consistency guarantees can we provide across NATS?
3. **Stampede protection implementation**: Single-flight (request coalescing) should be built into the library — but should it be optional or mandatory per operation?
4. **Cache key serialization**: How to handle complex keys efficiently? Hash-based? Struct-based? Template-based?
5. **Memory overhead estimation**: What is the expected cache size per agent? Per service? Per pod? This determines backend sizing requirements
6. **Cold start warming**: How aggressive should cache warming be on service startup? Full pre-population vs lazy loading?

## Next Steps

1. Proceed to BSD (Business Specification Document) with the problem space clarified
2. Research phase should evaluate:
   - Valkey configuration and cluster sizing for ACE workloads
   - Cross-service invalidation patterns over NATS
   - Stampede protection strategies (single-flight, request coalescing)
   - Cache key design conventions
3. Design the `shared/caching` package API surface
4. Define UsageEvent integration points with `shared/telemetry`
5. Define NATS integration patterns with `shared/messaging`
6. Define testing strategy covering unit, integration, load, and consistency tests
