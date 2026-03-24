# Research Document

## Topic
Caching strategies for the ACE Framework: distributed cache backends, in-memory cache libraries, cross-service invalidation patterns, and stampede protection mechanisms.

## Industry Standards

The caching landscape in 2025-2026 is shaped by two major shifts:
1. **The Redis license change** (March 2024): Redis moved from BSD to dual RSALv2/SSPLv1, prompting the Valkey fork under the Linux Foundation. In May 2025, Redis added AGPLv3 as a third license option, but community momentum had shifted.
2. **Multi-threaded in-memory stores**: Dragonfly and Valkey 8.0+ both push beyond Redis's single-threaded model, delivering 2-5x throughput improvements.

For in-memory Go caching, the field has evolved from simple map+mutex approaches to sophisticated concurrent structures with TinyLFU admission policies (Otter, Ristretto) and integrated stampede protection (sturdyc).

**Leading companies' patterns**: AWS and Google Cloud have adopted Valkey as their default managed cache. Most Go microservice architectures use either a singleflight wrapper around cache-aside or a library with built-in stampede protection. Cross-service invalidation universally uses some form of pub/sub (NATS, Redis Pub/Sub, or Kafka).

---

## Alternative Approaches Evaluated

### Category 1: Distributed Cache Backends

#### Approach 1A: Valkey (Primary Recommendation)

**Description**: Linux Foundation-backed fork of Redis 7.2.4, BSD 3-clause licensed. Supported by AWS, Google Cloud, Oracle, Alibaba, Ericsson, and Snap. Now at version 8.1+, with meaningful architectural divergence from Redis.

**Pros**:
- BSD 3-clause license — no copyleft restrictions, no licensing friction
- 37% higher SET throughput and 16% higher GET throughput vs Redis 8.0 on Graviton instances
- Enhanced I/O threading in 8.0+ (configurable via `io-threads`)
- AWS ElastiCache and GCP Cloud Memorystore default for new instances
- Per-slot metrics for granular observability
- 50% faster cluster failover detection
- Drop-in Redis replacement — same RESP protocol, 90%+ command compatibility
- Active community: 50+ contributing companies under Linux Foundation governance

**Cons**:
- Lacks Redis 8's vector sets and integrated Redis Query Engine (search/vector)
- No bundled modules equivalent to Redis Stack (RedisJSON, RedisTimeSeries)
- Client library ecosystem still catching up in some languages (though Go clients work fine)
- Experimental RDMA support — not production-ready

**Use Cases**: Standard caching workloads (session storage, rate limiting, pub/sub, sorted sets, key-value). Production deployments on AWS or GCP. Teams wanting clear open-source licensing without AGPL copyleft concerns.

#### Approach 1B: Redis 8.x

**Description**: The original in-memory data store, now under tri-license (RSALv2/SSPLv1/AGPLv3). Redis 8.0 integrated the former Redis Stack modules into core.

**Pros**:
- Most mature ecosystem and battle-tested at scale
- Redis 8.0 bundles RedisJSON, RediSearch, RedisTimeSeries, RedisBloom into core
- Vector set support for AI/ML workloads
- Salvatore Sanfilippo (antirez) returned to Redis Ltd in Nov 2024
- AGPLv3 is OSI-approved open source

**Cons**:
- AGPLv3 copyleft: if you modify and offer as a service, must release modifications
- SSPLv1 restricts cloud providers — still not OSI-approved
- Three-license model creates legal review friction
- Single-threaded command execution (I/O threads help but don't fully close the gap)
- AWS and GCP charge 20-33% more for Redis vs Valkey instances
- Community trust damaged by 2024 license change

**Use Cases**: Teams needing Redis Stack modules (vector search, JSON, time series). Azure deployments (Azure Cache for Redis still favors Redis). Organizations comfortable with AGPLv3 copyleft obligations.

#### Approach 1C: Memcached

**Description**: Simple distributed memory object caching system. Multi-threaded. No persistence. No complex data structures.

**Pros**:
- Extremely low operational complexity
- Very high throughput for simple GET/SET workloads
- Multi-threaded by design
- Zero licensing concerns (BSD)
- Mature, well-understood

**Cons**:
- No persistence — all data lost on restart
- No complex data structures (no sorted sets, hashes, streams, pub/sub)
- No built-in clustering or replication
- 1MB value size limit by default
- No Lua scripting or server-side computation

**Use Cases**: Pure caching layer where persistence and data structures don't matter. Supplement to a primary database. High-throughput, simple key-value workloads.

#### Approach 1D: PostgreSQL-Backed Cache

**Description**: Using PostgreSQL (with UNLOGGED tables or materialized views) as a cache layer. Version stamps for invalidation stored alongside.

**Pros**:
- Strong consistency guarantees (ACID)
- No additional infrastructure — reuses existing PostgreSQL
- Version stamps queried via SQLC with type safety
- Familiar operational model for the team
- Supports complex queries on cached data

**Cons**:
- Significantly slower than in-memory stores (~5-15ms vs <1ms)
- Not suitable for high-throughput caching (>10K ops/sec per connection)
- Disk I/O overhead even with UNLOGGED tables
- Doesn't scale horizontally for caching workloads

**Use Cases**: Version stamp storage (as specified in FSD). Metadata store for cache warming schedules and statistics. Low-throughput, consistency-critical caching where durability matters more than latency.

#### Approach 1E: Dragonfly (Honorable Mention)

**Description**: Ground-up multi-threaded, Redis-compatible in-memory data store. Shared-nothing architecture. Source-available (BSL 1.1, Apache 2.0 after 2029).

**Pros**:
- Highest single-node throughput: 2-5M RPS on 32-core vs ~500K RPS for Valkey/Redis
- 38% less memory usage per item than Redis
- No `fork()` for snapshots — avoids copy-on-write memory spikes
- Drop-in Redis replacement for most commands
- Linear scaling with CPU cores

**Cons**:
- Source-available license (BSL 1.1) — not OSI-approved open source until 2029
- Smaller ecosystem and community than Redis/Valkey
- Less battle-tested in production at scale
- No managed service from major cloud providers (as of early 2026)

**Use Cases**: High-throughput, self-hosted workloads where single-node performance is critical. Teams that can accept source-available licensing.

---

### Category 2: Distributed Cache Backend Comparison Matrix

| Criteria | Valkey | Redis 8.x | Memcached | PostgreSQL | Dragonfly |
|----------|--------|-----------|-----------|------------|-----------|
| **License** | BSD 3-clause | AGPLv3/RSALv2/SSPLv1 | BSD 3-clause | PostgreSQL License | BSL 1.1 (→ Apache 2029) |
| **Throughput (16 vCPU)** | ~700K RPS | ~500K RPS | ~800K RPS | ~15K RPS | ~2.5M RPS |
| **Persistence** | Yes (RDB/AOF) | Yes (RDB/AOF) | No | Yes (WAL) | Yes (custom) |
| **Data structures** | Rich (strings, hashes, lists, sets, sorted sets, streams, pub/sub) | Richest (adds vector sets, JSON, search) | Simple (key-value only) | Full SQL | Rich (Redis-compatible) |
| **Clustering** | Yes (Redis Cluster compatible) | Yes (Redis Cluster) | No (client-side sharding) | N/A (single logical) | Yes (multi-node) |
| **Managed service** | AWS, GCP default | AWS, Azure, GCP | AWS, GCP | AWS RDS, Cloud SQL | None (self-hosted) |
| **Operational complexity** | Medium | Medium | Low | Low (already deployed) | Medium |
| **Go client quality** | go-redis compatible | go-redis (official) | memcache client | pgx (excellent) | go-redis compatible |
| **Community momentum** | Very high (50+ companies) | Moderate | Stable/declining | N/A | Growing |

---

### Category 3: In-Memory Go Cache Libraries

#### Approach 2A: Otter v2 (Primary Recommendation)

**Description**: Modern in-memory cache library using W-TinyLFU admission policy. Written by the author who documented the evolution of Go caching libraries. Apache 2.0 licensed. Latest: v2.3.0 (December 2025).

**Pros**:
- Highest hit rates across all workload types via adaptive W-TinyLFU
- Best throughput under high contention (Go port of Caffeine benchmarks)
- Lowest memory overhead across all cache capacities
- Built-in TTL support (expire-after-write, expire-after-access)
- Size-based eviction with configurable capacity
- Optional features: loaders, writers, removal listeners, stats
- Active maintenance — 14 releases, latest Dec 2025
- Requires Go 1.24+
- Recommended by r/golang community

**Cons**:
- Relatively newer library (created Aug 2023)
- Smaller production track record than ristretto/bigcache
- No built-in stampede protection (must add singleflight)

**Use Cases**: Default in-memory backend for all workloads. Best general-purpose choice for new projects.

#### Approach 2B: Ristretto (Dgraph)

**Description**: High-performance cache with TinyLFU admission policy. Developed by Dgraph Labs. Used in Dgraph's production systems.

**Pros**:
- Battle-tested in production (Dgraph database)
- TinyLFU admission for high hit rates
- Cost-based API for memory management
- TTL support with per-item configuration
- Good documentation and large user base

**Cons**:
- Higher memory overhead than Otter (more allocations per operation)
- Lower throughput than Otter in modern benchmarks
- `interface{}` based API (no generics)
- Development has slowed — less active than Otter

**Use Cases**: Production systems needing proven stability. Teams already familiar with the Dgraph ecosystem.

#### Approach 2C: BigCache (Allegro)

**Description**: Efficient cache for gigabytes of data. Separates values from hashmap to minimize GC pressure. Used by Allegro (large Polish e-commerce platform).

**Pros**:
- Excellent GC performance for large caches (gigabytes)
- Sharding by FNV hash reduces lock contention
- Configurable bucket count
- Battle-tested at Allegro scale
- Active maintenance

**Cons**:
- No built-in TTL support (must implement expiration manually)
- No eviction policy beyond capacity (no LRU/LFU)
- Lower hit rates than TinyLFU-based caches
- Byte slice keys less convenient than string keys
- Higher per-operation latency than Otter in benchmarks

**Use Cases**: Very large caches (multi-GB) where GC pressure is the primary concern. Append-heavy workloads where entries don't need TTL-based expiration.

#### Approach 2D: FreeCache

**Description**: Cache that reduces GC overhead by storing data in ring buffer segments.

**Pros**:
- Zero GC overhead (ring buffer storage)
- Built-in TTL support
- High throughput for simple operations
- BSD licensed

**Cons**:
- Fixed memory allocation (ring buffer must be sized upfront)
- No generics support
- Less flexible API
- Lower hit rates than TinyLFU caches

**Use Cases**: Memory-constrained environments. Workloads with predictable data sizes.

#### Approach 2E: go-cache (patrickmn)

**Description**: Simple in-memory key-value store with expiration. Similar to Memcached but in-process.

**Pros**:
- Extremely simple API
- Built-in per-item TTL
- Auto-cleanup of expired items
- Well-known, widely used
- Good for small-scale applications

**Cons**:
- Uses single RWMutex — poor concurrent performance at scale
- No eviction policy (items expire or stay forever)
- No generics (interface{} values)
- Not suitable for high-throughput workloads
- Maintenance status unclear (last significant update 2020)

**Use Cases**: Development/testing, small-scale applications, prototyping. Not recommended for production at scale.

#### Approach 2F: sturdyc

**Description**: Caching library focused on stampede protection, request coalescing, and batch endpoint caching. Uses xxhash for key distribution.

**Pros**:
- Built-in stampede protection (in-flight tracking per key)
- Request coalescing and refresh coalescing
- Caching non-existent records
- Batch endpoint caching with cache key permutations
- Sharded writes with xxhash distribution
- Asynchronous refresh support
- Reduces data source load by 90%+ in production

**Cons**:
- More complex API than simple caches
- Newer library (less production track record)
- Designed for specific use case (upstream data source caching)

**Use Cases**: Services that cache data from upstream APIs or databases with expensive fetch operations. Workloads needing built-in stampede protection without wrapping in singleflight.

#### Approach 2G: Other Libraries Evaluated

| Library | Status | Notes |
|---------|--------|-------|
| **groupcache** | Archived/unmaintained | Google's original. No TTL. Various community forks exist but none have strong adoption. Designed for immutable data. |
| **theine** | Active | Another W-TinyLFU cache. Good performance but Otter has surpassed it in features and benchmarks. |
| **ccache** | Semi-active | LRU cache with TTL. Simple but not as performant as modern alternatives. |
| **fastcache** | Semi-active | Append-only log design. Good for specific workloads but limited features. |
| **sync.Map** | Standard library | No TTL, no eviction. Only suitable for specific patterns (keys rarely change). |

---

### Category 4: In-Memory Cache Libraries Comparison Matrix

| Criteria | Otter v2 | Ristretto | BigCache | FreeCache | go-cache | sturdyc |
|----------|----------|-----------|----------|-----------|----------|---------|
| **Hit rate (W-TinyLFU)** | Best | Good (TinyLFU) | N/A (no admission) | N/A | N/A | N/A |
| **Throughput** | Best | Good | Good | Good | Poor at scale | Good |
| **Memory overhead** | Lowest | Medium | Low | Very low | Medium | Medium |
| **GC pressure** | Low | Medium | Very low | Zero | Medium | Low |
| **TTL support** | Yes | Yes | No | Yes | Yes | Yes |
| **Eviction policy** | W-TinyLFU | TinyLFU | Capacity only | Capacity only | TTL only | Configurable |
| **Stampede protection** | No | No | No | No | No | Built-in |
| **Generics** | Yes | No | No | No | No | Yes |
| **Maintenance (2025-2026)** | Very active | Slowing | Active | Semi-active | Inactive | Active |
| **Production track record** | Growing | Strong (Dgraph) | Strong (Allegro) | Moderate | Widespread | Growing |
| **Go version required** | 1.24+ | 1.18+ | 1.16+ | 1.16+ | 1.16+ | 1.21+ |

---

### Category 5: Cross-Service Invalidation Patterns

#### Approach 3A: NATS-Based Invalidation (Primary Recommendation)

**Description**: Use NATS pub/sub (or JetStream for durability) to broadcast invalidation events across services. Aligns with ACE's existing NATS messaging architecture (`shared/messaging`).

**Pattern**:
```
Service A (data change) → Publish InvalidationEvent → NATS subject: ace.cache.{namespace}.invalidate
                                                      → Service B (subscriber): receives, deletes cache entry
                                                      → Service C (subscriber): receives, deletes cache entry
```

**Pros**:
- Already integrated with ACE's messaging infrastructure (`shared/messaging`)
- Subject wildcard routing (`ace.cache.*.invalidate`) enables flexible subscriptions
- JetStream option for at-least-once delivery if needed
- CorrelationID support for invalidation chain tracing (already in FSD)
- Low latency (< 1ms publish, fan-out is NATS core strength)
- No additional infrastructure beyond existing NATS broker

**Cons**:
- Fire-and-forget with NATS core (at-most-once) — subscribers down during publish miss events
- JetStream adds latency and complexity
- Requires careful subject design to avoid over-invalidation

**Use Cases**: Primary invalidation mechanism for ACE services. Real-time consistency across distributed services.

#### Approach 3B: Redis Pub/Sub for Invalidation

**Description**: Use Redis's built-in Pub/Sub for invalidation broadcasting.

**Pros**:
- No additional infrastructure if Redis is already deployed
- Simple API (PUBLISH/SUBSCRIBE)
- Pattern-based subscriptions (PSUBSCRIBE)
- Very low latency

**Cons**:
- Fire-and-forget — no message durability (missed messages are lost)
- No consumer groups or competing consumers
- Doesn't fit ACE's architecture (services communicate via NATS, not Redis)
- No built-in correlation ID or envelope support

**Use Cases**: Simpler systems without NATS. When Redis is the only infrastructure available.

#### Approach 3C: Version Stamp Approach

**Description**: Store version stamps in PostgreSQL. On cache read, compare cached version against current version in DB. Mismatch triggers refresh.

**Pros**:
- No pub/sub infrastructure needed for basic staleness detection
- Strong consistency — version always checked against source of truth
- Works even if event delivery fails
- Natural fit with SQLC for type-safe queries
- Bulk invalidation via version bump (increment version counter = invalidate all related keys)

**Cons**:
- Adds a DB query to every cache read (version check)
- Higher latency than event-driven (DB round-trip vs pub/sub)
- Not suitable for high-throughput read paths
- Doesn't notify other services — they detect staleness on next read

**Use Cases**: Data that changes infrequently but where consistency is critical. Metadata and configuration caches. As a safety net alongside event-driven invalidation.

#### Approach 3D: Hybrid Invalidation (Event + TTL Safety Net)

**Description**: Event-driven invalidation as primary mechanism, with TTL as fallback for missed events.

**Pros**:
- Best of both worlds: fast invalidation via events + guaranteed eventual consistency via TTL
- Resilient to event delivery failures
- TTL catches any events that were dropped
- Configurable per namespace (different TTLs for different data types)

**Cons**:
- More complex to implement and reason about
- Two mechanisms to test and monitor
- TTL may cause unnecessary refreshes if events are working correctly

**Use Cases**: Production systems needing both speed and reliability. Recommended default for ACE namespaces.

---

### Category 6: Cross-Service Invalidation Comparison Matrix

| Criteria | NATS-Based | Redis Pub/Sub | Version Stamp | Hybrid (Event + TTL) |
|----------|-----------|---------------|---------------|----------------------|
| **Delivery guarantee** | At-most-once (core) / At-least-once (JetStream) | At-most-once | Strong (DB check) | At-most-once + TTL fallback |
| **Latency** | < 1ms | < 1ms | 5-15ms (DB query) | < 1ms primary |
| **Infrastructure** | Existing NATS | Redis | PostgreSQL (existing) | NATS + TTL |
| **Resilience** | Moderate (core) / High (JetStream) | Low | High | High |
| **Complexity** | Low | Low | Medium | Medium |
| **ACE alignment** | Perfect | Poor | Good | Perfect |
| **Observability** | CorrelationID, UsageEvents | Limited | DB audit trail | Both |

---

### Category 7: Stampede Protection

#### Approach 4A: golang.org/x/sync/singleflight (Primary Recommendation)

**Description**: Go team's official package for duplicate function call suppression. Ensures only one in-flight execution per key at a time.

**Pros**:
- Official Go team package — zero external dependencies beyond `x/sync`
- Battle-tested in production across the Go ecosystem
- Dead simple API: `group.Do(key, func() (interface{}, error))`
- `DoChan` variant for async usage
- `shared` flag indicates if result was from another caller
- Works perfectly with cache-aside pattern (GetOrFetch)
- Well-documented, widely understood

**Cons**:
- Global lock on the internal map — can become a bottleneck at extreme scale
- No generics (returns `interface{}`)
- No context support — can't cancel waiting goroutines when the leader is cancelled
- No timeout support built-in
- Single group = single lock; sharding requires manual implementation

**Use Cases**: Default stampede protection for all cache-aside operations. The FSD already specifies this interface.

#### Approach 4B: resenje.org/singleflight

**Description**: Enhanced alternative to `x/sync/singleflight` with generics and context-aware cancellation.

**Pros**:
- Generic types: `Group[K comparable, V any]`
- Context-aware cancellation
- Better ergonomics for modern Go (1.18+)
- Comparable performance to `x/sync/singleflight`
- Active maintenance (v0.4.3, Sep 2024)

**Cons**:
- External dependency (not Go team maintained)
- Less battle-tested than `x/sync/singleflight`
- Smaller community

**Use Cases**: Teams wanting type safety and context support. Alternative if `x/sync/singleflight`'s limitations become problematic.

#### Approach 4C: Built-in (sturdyc)

**Description**: sturdyc provides built-in stampede protection via in-flight tracking per key, combined with refresh coalescing and asynchronous refresh.

**Pros**:
- Zero additional code needed — stampede protection is part of the cache API
- Refresh coalescing reduces upstream load by 90%+
- Asynchronous refresh for non-blocking reads
- Handles batch endpoints with per-record cache keys
- Caches non-existent records to prevent repeated misses

**Cons**:
- Couples stampede protection to cache implementation
- Less flexibility than standalone singleflight
- Designed for specific upstream-caching pattern

**Use Cases**: When stampede protection should be the cache library's responsibility, not the caller's. Upstream API caching with batch support.

#### Approach 4D: Sharded/Multiflight

**Description**: Sharding the singleflight key space across multiple groups to reduce lock contention at extreme scale.

**Pros**:
- Eliminates single-lock bottleneck
- Linear scaling with shard count
- Can be built on top of `x/sync/singleflight`

**Cons**:
- More complex to implement
- Requires consistent key-to-shard mapping
- Overkill for most workloads

**Use Cases**: Extreme scale (>100K concurrent goroutines hitting singleflight). Premature optimization for most teams.

---

### Category 8: Stampede Protection Comparison Matrix

| Criteria | x/sync/singleflight | resenje.org/singleflight | sturdyc (built-in) | Sharded multiflight |
|----------|---------------------|--------------------------|--------------------|--------------------|
| **Type safety** | No (interface{}) | Yes (generics) | Yes (generics) | Depends on base |
| **Context support** | No | Yes | Yes | Depends on base |
| **External dependency** | Go team (x/sync) | resenje.org | viccon/sturdyc | Custom |
| **Performance at scale** | Good (global lock) | Good | Good (sharded writes) | Best |
| **Integration with cache** | Manual wrapping | Manual wrapping | Automatic | Manual wrapping |
| **Production track record** | Excellent | Limited | Growing | Custom |
| **Complexity** | Very low | Low | Medium | High |

---

## Recommended Architecture

### Distributed Cache Backend

| Environment | Recommendation | Rationale |
|-------------|---------------|-----------|
| **Development** | In-memory (Otter v2) | Zero infrastructure, fast iteration, matches FSD default |
| **Production (primary)** | **Valkey** | Best performance, cleanest licensing, cloud-provider alignment |
| **Production (metadata)** | PostgreSQL | Version stamps, warming schedules, cache statistics (via SQLC) |
| **Future consideration** | Dragonfly | If single-node throughput becomes the bottleneck |

**Rationale for Valkey over Redis**: The license change created real operational risk. Valkey delivers equal or better performance with BSD licensing. AWS and GCP have made it their default — following the cloud providers' lead reduces vendor friction. Redis's vector search and Redis Stack modules are not needed for caching workloads.

### In-Memory Cache Library

| Use Case | Recommendation | Rationale |
|----------|---------------|-----------|
| **Default in-memory backend** | **Otter v2** | Best hit rates, throughput, and memory efficiency. Modern API with generics. |
| **Fallback/alternative** | Ristretto | Proven in production if Otter's newer codebase is a concern |
| **Specialized: stampede + batching** | sturdyc | If upstream API caching with built-in coalescing is needed |

**Rationale for Otter v2**: It's the evolution of the Go caching ecosystem. The author documented the progression from early caches through ristretto to otter, incorporating lessons learned. W-TinyLFU provides the best hit rates, and throughput benchmarks consistently show it as the fastest option for most workloads.

### Cross-Service Invalidation

| Layer | Recommendation | Rationale |
|-------|---------------|-----------|
| **Primary** | **NATS-based invalidation** | Aligns with ACE's `shared/messaging` infrastructure. Low latency. |
| **Safety net** | TTL-based expiration | Catches any missed events. Configurable per namespace. |
| **Version detection** | PostgreSQL version stamps | For versioned invalidation strategy (FSD requirement) |

**Rationale for Hybrid (NATS + TTL)**: Event-driven gives us fast, precise invalidation. TTL ensures eventual consistency even if NATS messages are lost. This dual-layer approach is the industry standard for distributed cache invalidation.

### Stampede Protection

| Use Case | Recommendation | Rationale |
|----------|---------------|-----------|
| **Default** | **golang.org/x/sync/singleflight** | Official Go package, battle-tested, simple API |
| **Alternative** | resenje.org/singleflight | If generics and context support are needed |

**Rationale for singleflight**: It's the standard solution in the Go ecosystem. The FSD already specifies its interface. The global lock concern only matters at extreme scale (>100K concurrent), which is unlikely for individual cache keys.

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| **Valkey lacks Redis Stack modules** | ACE caching doesn't need vector search/JSON/TimeSeries. If needed later, Dragonfly or Redis under AGPLv3 is an option. |
| **Otter v2 is newer (less battle-tested)** | Ristretto is available as a drop-in alternative. The `CacheBackend` interface abstracts the library choice. |
| **NATS at-most-once delivery may miss invalidations** | TTL safety net catches missed events. Hybrid strategy (event + TTL) provides dual-layer protection. |
| **singleflight global lock at extreme scale** | Shard the key space across multiple singleflight groups if profiling shows contention. Not needed at typical scale. |
| **Valkey/Redis divergence over time** | The `CacheBackend` interface isolates implementation. Protocol compatibility is ~90% and stable. |
| **Version stamp DB queries add latency** | Only used for versioned invalidation strategy (not all namespaces). Can be cached locally with short TTL. |

---

## References

- [Redis vs Valkey in 2026: What the License Fork Actually Changed](https://dev.to/synsun/redis-vs-valkey-in-2026-what-the-license-fork-actually-changed-1kni) — DEV Community, March 2026
- [Redis 8.0 and Valkey 8: Feature Comparison and Migration Guide](https://zeonedge.com/tk/blog/redis-8-valkey-8-migration-guide-comparison) — ZeonEdge, March 2026
- [Redis vs Valkey vs Dragonfly 2026: Full Comparison](https://devtoolswatch.com/en/redis-vs-valkey-vs-dragonfly-2026) — DevTools Research, Feb 2026
- [Redis vs Valkey: A Deep Dive for Enterprise Architects](https://andrewbaker.ninja/2026/01/04/redis-vs-valkey-a-deep-dive-for-enterprise-architects/) — Andrew Baker, Jan 2026
- [Redis 8.0 vs Valkey 8.1: A Deep Technical Comparison](https://dragonflydb.io/blog/redis-8-0-vs-valkey-8-1-a-technical-comparison) — Dragonfly Blog, Aug 2025
- [The Evolution of Caching Libraries in Go](https://maypok86.github.io/otter/) — Reddit r/golang / maypok86
- [Otter v2 Documentation](https://maypok86.github.io/otter/) — Official
- [Go Cache Benchmark](https://github.com/xeoncross/go-cache-benchmark) — GitHub
- [sturdyc Documentation](https://pkg.go.dev/github.com/viccon/sturdyc) — pkg.go.dev
- [Cache Invalidation Strategies That Actually Work in Production](https://cachee.ai/blog/posts/2025-12-20-cache-invalidation-strategies-that-actually-work.html) — Cachee.ai, Dec 2025
- [Pub/Sub Messaging Patterns: Redis, NATS, and When to Use What](https://dev.to/young_gao/pubsub-messaging-patterns-redis-nats-and-when-to-use-what-2el2) — DEV Community
- [Singleflight in Go: How One Line of Code Saved Our Database](https://medium.com/@0x48core/singleflight-in-go-how-one-line-of-code-saved-our-database-from-a-cache-stampede-c969d20f8133) — Medium, March 2026
- [resenje.org/singleflight](https://pkg.go.dev/resenje.org/singleflight@v0.4.3) — pkg.go.dev
- [Go Singleflight Melts in Your Code, Not in Your DB](https://victoriametrics.com/blog/go-singleflight/) — VictoriaMetrics Blog
