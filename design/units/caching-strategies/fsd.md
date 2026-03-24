# Functional Specification Document

## Overview

The `shared/caching` package provides transport-agnostic caching primitives for the ACE Framework. It establishes a Valkey-backed cache layer, standardized cache key conventions, multiple invalidation strategies, stampede protection, and mandatory observability integration. The package follows the same pattern as `shared/messaging` and `shared/telemetry` — defining interfaces and patterns. Valkey is the sole cache backend and runs in all environments including local development via Docker Compose.

The caching system operates across three layers:
- **Core Primitives**: `Get`, `Set`, `Delete`, `GetOrFetch`, bulk operations
- **Invalidation Strategies**: TTL-based, event-driven, versioned, hybrid, pattern-based, tag-based
- **Cross-Cutting Concerns**: Stampede protection, cache warming, multi-agent isolation, observability

A separate frontend caching module (SvelteKit/browser-side) handles client-side caching, offline fallback, and API response caching — designed but implementation deferred to the frontend unit.

## Technical Requirements

| Requirement | Type | Priority | Notes |
|-------------|------|----------|-------|
| Valkey-only cache backend | Functional | Must | Single backend for all environments via Docker Compose |
| Core CRUD operations (Get, Set, Delete) | Functional | Must | Basic cache operations with namespace isolation |
| Cache-aside pattern (GetOrFetch) | Functional | Must | Retrieve from cache or execute fetch, populate, return |
| Bulk operations (GetMany, SetMany, DeleteMany) | Functional | Should | Batch processing for performance |
| TTL management (standard, sliding, stale-while-revalidate) | Functional | Must | Configurable per-key and per-namespace |
| Namespace isolation | Functional | Must | Logical isolation by service, agent, or feature |
| Multi-agent cache isolation | Functional | Must | agentId threaded through all keys |
| Standardized cache key format | Functional | Must | `{namespace}:{agentId}:{entityType}:{entityId}:{version}` |
| TTL-based invalidation | Functional | Must | Standard expiration with configurable TTL |
| Sliding TTL extension | Functional | Should | Reset TTL on access for frequently used entries |
| Stale-while-revalidate | Functional | Should | Serve stale data, refresh in background |
| Event-driven invalidation via NATS | Functional | Must | Invalidation events flow through NATS subjects |
| Cross-namespace invalidation | Functional | Should | Single event clears related entries across namespaces |
| Versioned invalidation | Functional | Should | Content-hash or counter-based staleness detection |
| Hybrid invalidation (TTL + event) | Functional | Should | Event-driven primary with TTL safety net |
| Pattern-based invalidation | Functional | Should | DeleteByPattern with glob/regex matching |
| Tag-based invalidation | Functional | Could | Entries with shared tags invalidated together |
| Single-flight coalescing | Functional | Must | Prevent thundering herd on expired keys |
| Configurable stampede protection | Functional | Could | Enable/disable per namespace |
| Cache warming on startup | Functional | Must | Pre-populate critical namespaces within deadline |
| NATS-triggered warming | Functional | Could | Warming triggered by external events |
| UsageEvent emission on all operations | Non-functional | Must | 100% observability coverage |
| agentId in all UsageEvents | Non-functional | Must | Attribution for cost and debugging |
| Namespace-scoped operations | Functional | Must | Bulk operations respect agentId scope |
| Transport-agnostic design | Non-functional | Must | No imports of `net/http`, NATS client, etc. |
| Frontend caching module | Functional | Should | Browser-side caching for UI data |
| Offline cache fallback | Functional | Could | Serve cached data when network unavailable |
| Backend tool evaluation | Functional | Must | Documented recommendation with trade-offs |

## Data Model

### Cache Key Structure

Cache keys follow a standardized format that encodes namespace, agent identity, entity type, entity identifier, and version:

```
{namespace}:{agentId}:{entityType}:{entityId}:{version}
```

**Components:**

| Component | Description | Example |
|-----------|-------------|---------|
| `namespace` | Logical grouping by service or feature | `cognitive-engine`, `memory-manager`, `tool-executor` |
| `agentId` | Unique agent identifier for multi-agent isolation | `agent-alpha`, `agent-beta` |
| `entityType` | Category of cached data | `decision-tree`, `tool-def`, `skill-config` |
| `entityId` | Unique identifier within entity type | `review`, `l4-tree-root`, `ctx-1` |
| `version` | Version stamp for invalidation detection | `v3`, `hash-a1b2c3` |

**Example Keys:**
```
cognitive-engine:agent-alpha:decision-tree:tree-456:v3
memory-manager:agent-beta:l4-tree:root:v1
tool-executor:agent-alpha:tool-def:review:hash-a1b2c3
```

**Key Resolution:**
- Services provide logical keys (e.g., `"decision-tree-456"`)
- The library automatically prepends `namespace` and `agentId` from context
- Version is appended based on invalidation strategy (TTL-based uses timestamp, versioned uses content hash)

### Namespace Design

Namespaces provide logical isolation and configuration boundaries:

```go
type NamespaceConfig struct {
    Name               string
    DefaultTTL         time.Duration
    MaxSize            int64
    InvalidationStrategy InvalidationStrategy
    StampedeProtection bool
    WarmingEnabled     bool
    WarmingDeadline    time.Duration
}
```

**Default Namespaces:**

| Namespace | Default TTL | Invalidation | Warming |
|-----------|-------------|--------------|---------|
| `cognitive-engine` | 5 min | Hybrid (event + TTL) | Yes (startup) |
| `memory-manager` | 10 min | Versioned | Yes (startup) |
| `tool-executor` | 15 min | Event-driven | No |
| `skill-config` | 30 min | Event-driven | No |
| `llm-completions` | 1 hour | TTL-only | No |
| `embeddings` | 24 hours | Versioned | No |

### Cache Entry Metadata

Each cached entry carries metadata for observability and invalidation:

```go
type CacheEntry struct {
    Key           string
    Value         []byte
    Namespace     string
    AgentID       string
    EntityType    string
    EntityID      string
    Version       string
    Tags          []string
    CreatedAt     time.Time
    AccessedAt    time.Time
    ExpiresAt     time.Time
    TTL           time.Duration
    SizeBytes     int64
    HitCount      int64
    InvalidatedBy string    // "ttl", "event", "manual", "version"
}
```

### Version Stamp Schema

Version stamps enable staleness detection without explicit invalidation:

```go
type VersionStamp struct {
    Key         string
    Version     string    // Semantic version, content hash, or monotonic counter
    SourceHash  string    // Hash of source data for content-based versioning
    UpdatedAt   time.Time
    UpdatedBy   string    // Service that last updated the version
}
```

Version stamps are stored in PostgreSQL for distributed consistency and queried via SQLC.

### Invalidation Event Schema

Invalidation events flow through NATS for cross-service coordination:

```go
type InvalidationEvent struct {
    EventID       string
    SourceService string
    AgentID       string
    Namespace     string
    KeyPattern     string    // Exact key or glob pattern
    Tags          []string  // Optional: invalidate by tags
    Version       string    // Optional: version-based invalidation
    Reason        string    // "data-change", "ttl-expired", "manual"
    Timestamp     time.Time
    CorrelationID string
}
```

## API Interface

### Core Types

```go
// CacheBackend wraps the Valkey client
type CacheBackend interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    GetMany(ctx context.Context, keys []string) (map[string][]byte, error)
    SetMany(ctx context.Context, entries map[string][]byte, ttl time.Duration) error
    DeleteMany(ctx context.Context, keys []string) error
    DeletePattern(ctx context.Context, pattern string) error
    DeleteByTag(ctx context.Context, tag string) error
    Exists(ctx context.Context, key string) (bool, error)
    TTL(ctx context.Context, key string) (time.Duration, error)
    Close() error
}

// Cache is the high-level interface services use
type Cache interface {
    // Core operations
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, opts ...SetOption) error
    Delete(ctx context.Context, key string) error

    // Cache-aside pattern
    GetOrFetch(ctx context.Context, key string, fetchFn FetchFunc, opts ...SetOption) ([]byte, error)

    // Bulk operations
    GetMany(ctx context.Context, keys []string) (map[string][]byte, error)
    SetMany(ctx context.Context, entries map[string][]byte, opts ...SetOption) error
    DeleteMany(ctx context.Context, keys []string) error

    // Invalidation
    DeletePattern(ctx context.Context, pattern string) error
    DeleteByTag(ctx context.Context, tag string) error
    InvalidateByVersion(ctx context.Context, key string, expectedVersion string) error

    // Configuration
    WithNamespace(namespace string) Cache
    WithAgentID(agentID string) Cache
    WithTTL(ttl time.Duration) Cache
    WithTags(tags ...string) Cache

    // Observability
    Stats(ctx context.Context) (*CacheStats, error)
}

// FetchFunc is the function called on cache miss
type FetchFunc func(ctx context.Context) ([]byte, error)

// SetOption configures Set operations
type SetOption func(*SetOptions)

type SetOptions struct {
    TTL      time.Duration
    Tags     []string
    Version  string
}

// CacheStats provides observability data
type CacheStats struct {
    Namespace     string
    HitCount      int64
    MissCount     int64
    HitRate       float64
    EntryCount    int64
    TotalSize     int64
    EvictionCount int64
    AvgLatencyMs  float64
}

// InvalidationStrategy defines how entries become stale
type InvalidationStrategy int

const (
    InvalidationTTL InvalidationStrategy = iota
    InvalidationEventDriven
    InvalidationVersioned
    InvalidationHybrid
)

// Config holds cache configuration
type Config struct {
    Namespace        string
    DefaultTTL       time.Duration
    MaxSize          int64
    Invalidation     InvalidationStrategy
    StampedeProtection bool
    Warming          *WarmingConfig
    ValkeyURL        string
}

type WarmingConfig struct {
    Enabled    bool
    Deadline   time.Duration
    WarmFunc   WarmFunc
    OnStartup  bool
    NATSTrigger bool
}

type WarmFunc func(ctx context.Context, cache Cache) error
```

### Cache Key Builder

```go
// KeyBuilder constructs standardized cache keys
type KeyBuilder struct {
    namespace  string
    agentID    string
    entityType string
    entityID   string
    version    string
}

func NewKeyBuilder(namespace, agentID string) *KeyBuilder

func (kb *KeyBuilder) EntityType(t string) *KeyBuilder
func (kb *KeyBuilder) EntityID(id string) *KeyBuilder
func (kb *KeyBuilder) Version(v string) *KeyBuilder
func (kb *KeyBuilder) Build() string
func (kb *KeyBuilder) Pattern() string  // Returns glob pattern for invalidation
```

### Stampede Protection Interface

```go
// SingleFlight provides request coalescing for expired keys
type SingleFlight interface {
    Do(key string, fn func() (interface{}, error)) (interface{}, error, bool)
    DoChan(key string, fn func() (interface{}, error)) <-chan SingleFlightResult
}

type SingleFlightResult struct {
    Val interface{}
    Err error
}
```

### Observability Integration

```go
// Operation types for UsageEvent emission
const (
    OperationTypeCacheHit        = "cache-hit"
    OperationTypeCacheMiss       = "cache-miss"
    OperationTypeCacheWrite      = "cache-write"
    OperationTypeCacheInvalidate = "cache-invalidate"
    OperationTypeCacheEvict      = "cache-evict"
    OperationTypeCacheWarming    = "cache-warming"
)

// Resource type for cache operations
const (
    ResourceTypeCache = "cache"
)

// CacheObserver wraps cache operations with telemetry emission
type CacheObserver interface {
    ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64)
    ObserveSet(ctx context.Context, namespace, key string, sizeBytes int64, latencyMs float64)
    ObserveDelete(ctx context.Context, namespace, key string, reason string)
    ObserveEviction(ctx context.Context, namespace, key string, reason string)
    ObserveWarming(ctx context.Context, namespace string, progress WarmingProgress)
}

type WarmingProgress struct {
    EntriesPopulated int
    EntriesRemaining int
    ElapsedMs        float64
    SuccessCount     int
    FailureCount     int
}
```

## Business Logic

### Core Algorithms

#### Cache-Aside Pattern (GetOrFetch)

The `GetOrFetch` method implements the cache-aside pattern with stampede protection:

```
1. Construct full cache key from namespace + agentId + logical key
2. Check if stampede protection is enabled for namespace
3. If enabled:
   a. Acquire single-flight lock for key
   b. If another goroutine is fetching, wait for result
   c. If no goroutine is fetching, proceed to step 4
4. Attempt Get from backend
5. If hit:
   a. Check if entry is stale (version mismatch or expired)
   b. If not stale: return cached value, emit cache-hit UsageEvent
   c. If stale and stale-while-revalidate enabled:
      - Return stale value immediately
      - Trigger background refresh via goroutine
      - Emit cache-hit with stale flag
6. If miss:
   a. Execute fetchFn
   b. If fetchFn succeeds:
      - Store result with configured TTL and tags
      - Emit cache-write UsageEvent
      - Return fetched value
   c. If fetchFn fails:
      - Release single-flight lock
      - Return error
7. If single-flight was used, broadcast result to all waiters
```

#### Stampede Protection (Single-Flight Coalescing)

The single-flight pattern prevents thundering herd when popular cache keys expire:

```
1. On GetOrFetch for key X:
   a. Check single-flight map for active request on X
   b. If no active request:
      - Create channel for result broadcast
      - Mark request as active
      - Execute fetch
      - Broadcast result to all waiters
      - Remove from active map
   c. If active request exists:
      - Subscribe to broadcast channel
      - Wait for result
      - Return received result (no duplicate fetch)
2. On fetch failure:
   a. Broadcast error to all waiters
   b. Remove from active map
   c. Do not cache error
3. Configurable per namespace (can be disabled for low-contention namespaces)
```

#### Invalidation Strategies

**TTL-Based Invalidation:**
```
1. Entry stored with ExpiresAt = Now() + TTL
2. On Get:
   a. If Now() > ExpiresAt: return miss
   b. If sliding TTL: reset ExpiresAt = Now() + TTL
3. Backend handles eviction (Valkey: native TTL)
```

**Event-Driven Invalidation:**
```
1. Service publishes InvalidationEvent to NATS subject
2. All subscribing services receive event
3. On receive:
   a. Parse event for namespace, key pattern, tags
   b. If exact key: Delete(key)
   c. If pattern: DeletePattern(pattern)
   d. If tags: DeleteByTag(tags)
   e. Emit cache-invalidate UsageEvent with source "event"
4. Event correlation via CorrelationID for chain tracing
```

**Versioned Invalidation:**
```
1. Entry stored with VersionStamp (content hash or counter)
2. On Get:
   a. Retrieve cached entry
   b. Query current VersionStamp from PostgreSQL
   c. If cached version != current version:
      - Treat as miss
      - Trigger fetch and re-cache with new version
   d. If versions match: return cached value
3. On source data change:
   a. Update VersionStamp in PostgreSQL
   b. Next Get detects mismatch and refreshes
```

**Hybrid Invalidation (Event + TTL):**
```
1. Entry stored with TTL and event subscription
2. On data change: event invalidates immediately (primary mechanism)
3. On TTL expiry without event: TTL acts as safety net
4. Prevents stale data if event delivery fails
5. TTL reset on successful event-driven invalidation
```

#### Cache Warming

Warming pre-populates critical namespaces on service startup:

```
1. Service startup triggers warming for configured namespaces
2. For each warming-enabled namespace:
   a. Execute WarmFunc with deadline context
   b. WarmFunc populates cache entries via Set operations
   c. Progress tracked via WarmingProgress struct
   d. UsageEvents emitted per batch of entries
3. If warming completes within deadline:
   a. Service marked ready
   b. Final UsageEvent emitted with success metrics
4. If warming exceeds deadline:
   a. Service starts with partial cache (degraded mode)
   b. Warning UsageEvent emitted
   c. Background warming continues
5. NATS-triggered warming:
   a. Subscribe to warming trigger subject
   b. On message: re-execute WarmFunc
   c. Useful for refreshing after bulk data changes
```

### State Machines

#### Cache Entry Lifecycle

```
        ┌─────────┐
        │ CREATED │
        └────┬────┘
             │
        ┌────▼────┐
        │  ACTIVE │◄─────────────────────────┐
        └────┬────┘                          │
             │                               │
    ┌────────┼────────┐                      │
    │        │        │                      │
    ▼        ▼        ▼                      │
┌──────┐ ┌──────┐ ┌──────┐                  │
│ HIT  │ │ MISS │ │STALE │                  │
└──┬───┘ └──┬───┘ └──┬───┘                  │
   │        │        │                      │
   │        │        ▼                      │
   │        │   ┌─────────┐                 │
   │        │   │REFRESH  │─────────────────┘
   │        │   └─────────┘
   │        │
   ▼        ▼
┌──────────────┐
│  INVALIDATED │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   EVICTED    │
└──────────────┘
```

#### Invalidation Event Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────────┐
│ DATA CHANGE │────►│ PUBLISH     │────►│ NATS SUBJECT    │
│ (service A) │     │ INVALIDATE  │     │ ace.cache.%.inv │
└─────────────┘     └─────────────┘     └────────┬────────┘
                                                 │
                              ┌───────────────────┼───────────────────┐
                              │                   │                   │
                              ▼                   ▼                   ▼
                    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
                    │ SERVICE B       │ │ SERVICE C       │ │ SERVICE D       │
                    │ RECEIVE EVENT   │ │ RECEIVE EVENT   │ │ RECEIVE EVENT   │
                    └────────┬────────┘ └────────┬────────┘ └────────┬────────┘
                             │                   │                   │
                             ▼                   ▼                   ▼
                    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
                    │ DELETE CACHE    │ │ DELETE CACHE    │ │ DELETE CACHE    │
                    │ EMIT EVENT      │ │ EMIT EVENT      │ │ EMIT EVENT      │
                    └─────────────────┘ └─────────────────┘ └─────────────────┘
```

### Multi-Agent Isolation Algorithm

```
1. On any cache operation:
   a. Extract agentID from context.Context
   b. If agentID missing: return error (agentId is mandatory)
2. Construct key: namespace:agentID:entityType:entityID:version
3. On Get/Set/Delete: agentID is part of key, ensuring isolation
4. On GetMany: filter results to only current agent's keys
5. On invalidation event: verify event.AgentID matches context agentID
6. On UsageEvent emission: include agentID for attribution
```

## Edge Cases

| Scenario | Expected Behavior | Handling |
|----------|-------------------|----------|
| Cache backend unavailable | Return error, log failure, emit error UsageEvent | Service continues without cache (graceful degradation) |
| Fetch function returns error | Propagate error to all waiting goroutines, do not cache error | Single-flight releases lock, no stale data served |
| Concurrent requests for same expired key | Exactly 1 fetch executes, all waiters receive result | Single-flight coalescing |
| TTL expired but data still valid | Depends on strategy: TTL returns miss, stale-while-revalidate serves stale | Strategy determines behavior |
| Key collision across namespaces | Entries isolated by namespace prefix in key | Namespace is part of key, no collision possible |
| agentId missing from context | Return error, do not cache | agentId is mandatory for all operations |
| Warming exceeds deadline | Service starts with partial cache, background warming continues | Degraded mode with warning event |
| Invalidating key that doesn't exist | No-op, no error | Delete is idempotent |
| Bulk operation with mixed hit/miss | Return map of hits only, misses omitted | Caller checks map length vs request length |
| Version stamp not found in DB | Treat as version mismatch, trigger refresh | New entry cached with current version |
| NATS connection lost during invalidation | TTL safety net catches stale entries | Hybrid strategy with TTL fallback |
| Cache entry larger than max size | Reject write, emit eviction event | Max size enforced per namespace |
| Valkey cluster partition | Return error, emit error event | Service continues without cache |
| Sliding TTL on every access | TTL extended on access | Configurable, can be disabled |
| Pattern invalidation matches too many keys | Bulk delete with rate limiting | Configurable batch size for pattern deletes |
| Tag assigned to non-existent entry | No-op on tag index | Tag index is eventually consistent |
| Frontend cache corrupted | Clear cache, fetch fresh data from API | Browser cache has corruption detection |
| Offline mode with stale cache | Serve stale data, mark as offline | Frontend handles offline state |

## Error Handling

| Error Code | Condition | Response |
|------------|-----------|----------|
| `ErrCacheMiss` | Key not found in cache | Return nil value, no error (caller checks for miss) |
| `ErrBackendUnavailable` | Cache backend connection failed | Log error, emit UsageEvent, return error to caller |
| `ErrAgentIDMissing` | agentId not in context.Context | Return error, do not execute operation |
| `ErrInvalidKey` | Key format does not match convention | Return error with key validation message |
| `ErrTTLExpired` | Entry TTL has expired | Treat as cache miss, trigger refresh if applicable |
| `ErrVersionMismatch` | Cached version != current version | Treat as miss, fetch fresh data |
| `ErrStampedeLock` | Single-flight lock acquisition timeout | Return error, caller may retry |
| `ErrFetchFailed` | Fetch function returned error | Propagate error to all waiters |
| `ErrWarmingTimeout` | Cache warming exceeded deadline | Emit warning event, start service in degraded mode |
| `ErrMaxSizeExceeded` | Write would exceed namespace size limit | Reject write, emit eviction event |
| `ErrSerializationFailed` | Value cannot be serialized/deserialized | Return error, log failure |
| `ErrNATSDisconnected` | NATS connection lost for invalidation events | Log error, rely on TTL safety net |
| `ErrPatternInvalid` | Invalid glob/regex pattern for invalidation | Return error with pattern validation |
| `ErrTagNotFound` | Tag does not exist in index | No-op (idempotent delete) |
| `ErrFrontendCacheCorrupt` | Browser cache corruption detected | Clear cache, fetch fresh data |

**Error Propagation Rules:**
- Cache miss is not an error — callers check for nil value
- Backend errors are logged and emitted as UsageEvents but may be degraded (return stale data)
- Fetch errors propagate to all single-flight waiters
- agentId errors are always terminal (no operation proceeds without attribution)
- Warming errors are non-blocking (service starts in degraded mode)

## Performance Requirements

### Latency Targets

| Operation | Target Latency | Notes |
|-----------|----------------|-------|
| Valkey Get | < 5ms (p99) | Network round-trip + Valkey lookup |
| Valkey Set | < 5ms (p99) | Network round-trip + Valkey write |
| GetOrFetch (cache hit) | < 5ms (p99) | Valkey hit path |
| GetOrFetch (cache miss) | Depends on fetchFn | Fetch latency dominates |
| Bulk Get (100 keys) | < 10ms (p99) | Batch optimization |
| Pattern invalidation | < 50ms (p99) | Depends on pattern match count |
| Stampede protection overhead | < 0.5ms | Single-flight lock acquisition |

### Throughput Targets

| Metric | Target | Notes |
|--------|--------|-------|
| Valkey ops/sec | > 10,000 per connection | Depends on Valkey cluster size |
| Concurrent single-flight waiters | > 1,000 per key | Goroutine efficiency |
| Bulk operation batch size | 100-1000 keys | Configurable per operation |
| Pattern invalidation batch size | 100 keys per batch | Rate limiting to prevent blocking |

### Resource Constraints

| Resource | Limit | Notes |
|----------|-------|-------|
| Valkey memory per namespace | Configurable (default 1GB) | Valkey-native eviction |
| Single-flight map size | Unbounded (cleaned on completion) | Entries removed after broadcast |
| Version stamp query frequency | Per Get with versioned strategy | Cached in PostgreSQL, indexed |
| UsageEvent emission rate | No hard limit | Async emission, batched if needed |
| Warming concurrency | Configurable (default 10 goroutines) | Parallel namespace warming |

### Observability Performance

| Metric | Target | Notes |
|--------|--------|-------|
| UsageEvent emission latency | < 1ms (non-blocking) | Async publish |
| Hit rate calculation | Real-time aggregation | From UsageEvent stream |
| Invalidation chain trace | < 100ms to correlate | Via CorrelationID in events |
| Cost savings calculation | Batched hourly | Aggregated from LLM-related cache hits |

## Frontend Caching Module

The frontend caching module (SvelteKit/browser-side) is designed but implementation is deferred to the frontend unit. Design decisions:

### Browser Storage Strategy

| Store | Use Case | TTL | Offline |
|-------|----------|-----|---------|
| Memory (JS Map) | Session state, UI state | Session | No |
| localStorage | User preferences, agent config | Persistent | Yes |
| sessionStorage | API responses, conversation history | Session | No |
| IndexedDB | Large datasets, skill configurations | Configurable | Yes |
| Service Worker Cache | Static assets, API responses | Configurable | Yes |

### Frontend Cache API

```typescript
interface FrontendCache {
  get<T>(key: string): T | null;
  set<T>(key: string, value: T, ttl?: number): void;
  delete(key: string): void;
  clear(): void;
  getOrFetch<T>(key: string, fetchFn: () => Promise<T>, ttl?: number): Promise<T>;
  invalidate(pattern: string): void;
  isOffline(): boolean;
}
```

### Offline Fallback Behavior

1. On network loss: serve cached data with "offline" indicator
2. On reconnect: queue invalidations, refresh stale entries
3. Conflict resolution: server data wins on reconnect

## Backend Tool Selection

### Evaluation Criteria

| Criterion | Weight | Description |
|-----------|--------|-------------|
| Operational complexity | 25% | Setup, maintenance, monitoring overhead |
| Performance | 25% | Latency, throughput, memory efficiency |
| Consistency guarantees | 20% | Data consistency, replication, partition tolerance |
| Ecosystem maturity | 15% | Community, documentation, Go client quality |
| Cost | 15% | Infrastructure cost, licensing |

### Candidate Evaluation Matrix

| Backend | Operational | Performance | Consistency | Ecosystem | Cost | Recommendation |
|---------|-------------|-------------|-------------|-----------|------|----------------|
| Valkey | Medium | High | High (with config) | Excellent | Medium | **Sole backend — all environments** |
| PostgreSQL | Low | Medium | Very High | Excellent | Low | Metadata store (version stamps) |

### Recommended Architecture

- **All environments**: Valkey runs in every environment (local dev via Docker Compose, staging, production)
- **Metadata**: PostgreSQL for version stamps, warming schedules, cache statistics
- **No pluggable abstraction**: One client (valkey-go), one backend (Valkey), all environments

## NATS Integration Points

While `shared/caching` is transport-agnostic, service-internal adapters wire NATS integration:

### Invalidation Event Subjects

```
ace.cache.{namespace}.invalidate    # Namespace-scoped invalidation
ace.cache.{agentId}.invalidate      # Agent-scoped invalidation
ace.cache.global.invalidate         # Global invalidation (rare)
```

### Warming Trigger Subjects

```
ace.cache.{namespace}.warm          # Trigger warming for namespace
```

### Event Flow

1. Service publishes `InvalidationEvent` to NATS
2. Subscribing services receive and process invalidation
3. Each service emits UsageEvents for observability correlation
4. CorrelationID links all invalidation events in a chain

## Testing Requirements

| Test Type | Scope | Priority |
|-----------|-------|----------|
| Unit tests | Cache operations (Valkey mock and test instance) | Must |
| Unit tests | Key builder, namespace isolation, stampede protection | Must |
| Integration tests | Cross-service invalidation via NATS | Must |
| Integration tests | Versioned invalidation with PostgreSQL | Must |
| Load tests | Stampede scenarios (1000+ concurrent requests) | Must |
| Load tests | Bulk operations under load | Should |
| Consistency tests | Distributed invalidation convergence | Must |
| Consistency tests | Eventual consistency windows | Should |
| Edge case tests | All edge cases from table above | Must |
| Performance tests | Latency and throughput benchmarks | Should |
| Observability tests | UsageEvent emission correctness | Must |
