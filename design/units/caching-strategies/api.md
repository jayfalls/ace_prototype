# API Specification — shared/caching

## Overview

`shared/caching` is a transport-agnostic Go library providing Valkey-backed cache operations, multiple invalidation strategies, stampede protection, and mandatory observability integration. It is imported by all ACE services as a foundational shared package, alongside `shared/messaging` and `shared/telemetry`.

**Import path:** `ace/shared/caching`

**Dependencies:**
- `github.com/valkey-io/valkey-go` — Valkey client (sole cache backend)
- `golang.org/x/sync/singleflight` — Stampede protection

**Design principles:**
- Transport-agnostic: no imports of `net/http`, NATS, or any transport layer
- Valkey-only: one backend, one client, no pluggable abstraction
- `agentId` mandatory on all operations for multi-agent isolation
- 100% observability coverage via `CacheObserver` interface

---

## Package Interface

### `Cache` — High-Level Cache Interface

The primary interface services interact with. All methods require `agentId` in context.

```go
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

    // Configuration — returns a new Cache scoped to the given parameters
    WithNamespace(namespace string) Cache
    WithAgentID(agentID string) Cache
    WithDefaultTTL(ttl time.Duration) Cache
    WithDefaultTags(tags ...string) Cache

    // Observability
    Stats(ctx context.Context) (*CacheStats, error)
}
```

**Behaviour notes:**
- `Get` returns `(nil, nil)` on cache miss — callers check for nil value, not an error.
- `WithNamespace`, `WithAgentID`, `WithDefaultTTL`, `WithDefaultTags` return a **new** `Cache` instance with the parameter applied; the original is unchanged.
- `GetOrFetch` uses stampede protection when the namespace has it enabled.
- `DeletePattern` accepts glob patterns (e.g., `cognitive-engine:agent-alpha:decision-tree:*`).
- `InvalidateByVersion` compares the cached version against `expectedVersion`; mismatch triggers a cache miss.

---

### `CacheBackend` — Low-Level Valkey Operations

Wraps direct Valkey commands. Used internally by `Cache` and exposed for advanced use cases.

```go
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
```

**Behaviour notes:**
- Keys at this layer are **fully qualified** (already include namespace, agentId, entityType, entityId, version).
- `DeletePattern` uses Valkey `SCAN` + `DEL` in batches to avoid blocking.
- `DeleteByTag` operates on a tag index maintained in Valkey sets.
- `Close` shuts down the underlying Valkey client connection pool.

---

### `CacheObserver` — Observability Interface

Emits telemetry for every cache operation. Services wire this to `shared/telemetry`.

```go
type CacheObserver interface {
    ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64)
    ObserveSet(ctx context.Context, namespace, key string, sizeBytes int64, latencyMs float64)
    ObserveDelete(ctx context.Context, namespace, key string, reason string)
    ObserveEviction(ctx context.Context, namespace, key string, reason string)
    ObserveWarming(ctx context.Context, namespace string, progress WarmingProgress)
}
```

**Wiring example (in service-internal code):**
```go
observer := &TelemetryCacheObserver{UsagePublisher: tel.Usage}
cache := caching.NewCache(backend, caching.WithObserver(observer))
```

---

### `SingleFlight` — Stampede Protection

Request coalescing for expired keys. Wraps `golang.org/x/sync/singleflight`.

```go
type SingleFlight interface {
    Do(key string, fn func() (interface{}, error)) (interface{}, error, bool)
    DoChan(key string, fn func() (interface{}, error)) <-chan SingleFlightResult
}
```

| Method | Returns | Description |
|--------|---------|-------------|
| `Do` | `(val, err, shared)` | Executes `fn` if no in-flight request for `key`; otherwise waits. `shared=true` means result came from another caller. |
| `DoChan` | channel | Async variant. Returns a channel that receives the result when ready. |

---

## Core Types

### `FetchFunc`

Function called on cache miss by `GetOrFetch`.

```go
type FetchFunc func(ctx context.Context) ([]byte, error)
```

---

### `SetOption` / `SetOptions`

Functional options for `Set` and `SetMany` operations.

```go
type SetOption func(*SetOptions)

type SetOptions struct {
    TTL     time.Duration
    Tags    []string
    Version string
}
```

**Built-in option functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `WithTTL` | `func(ttl time.Duration) SetOption` | Override the namespace default TTL for this operation |
| `WithTags` | `func(tags ...string) SetOption` | Associate tags for tag-based invalidation |
| `WithVersion` | `func(version string) SetOption` | Set version stamp for versioned invalidation |

---

### `KeyBuilder`

Constructs standardized cache keys in the format `{namespace}:{agentId}:{entityType}:{entityId}:{version}`.

```go
type KeyBuilder struct {
    // unexported fields
}

func NewKeyBuilder(namespace, agentID string) *KeyBuilder
func (kb *KeyBuilder) EntityType(t string) *KeyBuilder
func (kb *KeyBuilder) EntityID(id string) *KeyBuilder
func (kb *KeyBuilder) Version(v string) *KeyBuilder
func (kb *KeyBuilder) Build() (string, error)
func (kb *KeyBuilder) Pattern() (string, error)
```

| Method | Returns | Description |
|--------|---------|-------------|
| `NewKeyBuilder` | `*KeyBuilder` | Creates a builder with namespace and agentID. Returns error if agentID is empty. |
| `EntityType` | `*KeyBuilder` | Sets entity type. Chainable. |
| `EntityID` | `*KeyBuilder` | Sets entity ID. Chainable. |
| `Version` | `*KeyBuilder` | Sets version stamp. Chainable. |
| `Build` | `(string, error)` | Returns the full key string. Returns `ErrAgentIDMissing` if agentID is empty. |
| `Pattern` | `(string, error)` | Returns a glob pattern for invalidation. Unset components become `*`. |

**Pattern generation examples:**
```go
// Invalidate all decision trees for a specific agent
kb := NewKeyBuilder("cognitive-engine", "agent-alpha")
kb.EntityType("decision-tree")
pattern, _ := kb.Pattern()
// "cognitive-engine:agent-alpha:decision-tree:*"

// Invalidate everything for an agent
kb := NewKeyBuilder("cognitive-engine", "agent-alpha")
pattern, _ := kb.Pattern()
// "cognitive-engine:agent-alpha:*"
```

---

### `CacheEntry`

Metadata for a cached entry.

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
    InvalidatedBy string // "ttl", "event", "manual", "version"
}
```

---

### `CacheStats`

Aggregated statistics for a namespace.

```go
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
```

---

### `WarmingProgress`

Progress tracking for cache warming operations.

```go
type WarmingProgress struct {
    EntriesPopulated int
    EntriesRemaining int
    ElapsedMs        float64
    SuccessCount     int
    FailureCount     int
}
```

---

### `SingleFlightResult`

Result from an async single-flight operation.

```go
type SingleFlightResult struct {
    Val    interface{}
    Err    error
}
```

---

### `VersionStamp`

Version metadata for versioned invalidation. Stored in PostgreSQL.

```go
type VersionStamp struct {
    Key        string
    Version    string    // Semantic version, content hash, or monotonic counter
    SourceHash string    // Hash of source data for content-based versioning
    UpdatedAt  time.Time
    UpdatedBy  string    // Service that last updated the version
}
```

---

### `InvalidationEvent`

Event schema for cross-service invalidation. Published/subscribed via service-internal NATS adapters.

```go
type InvalidationEvent struct {
    EventID       string
    SourceService string
    AgentID       string
    Namespace     string
    KeyPattern    string    // Exact key or glob pattern
    Tags          []string  // Optional: invalidate by tags
    Version       string    // Optional: version-based invalidation
    Reason        string    // "data-change", "ttl-expired", "manual"
    Timestamp     time.Time
    CorrelationID string
}
```

---

## Constructor Functions

### `NewCache`

Creates a new `Cache` instance.

```go
func NewCache(backend CacheBackend, opts ...CacheOption) Cache
```

### `NewValkeyBackend`

Creates a `CacheBackend` backed by Valkey.

```go
func NewValkeyBackend(cfg ValkeyConfig) (CacheBackend, error)
```

**`ValkeyConfig`:**

```go
type ValkeyConfig struct {
    URL          string        // e.g., "redis://valkey:6379"
    MaxRetries   int           // Default: 3
    DialTimeout  time.Duration // Default: 5s
    ReadTimeout  time.Duration // Default: 3s
    WriteTimeout time.Duration // Default: 3s
    PoolSize     int           // Default: 100
}
```

### `NewKeyBuilder`

Creates a key builder. See [KeyBuilder](#keybuilder) section.

```go
func NewKeyBuilder(namespace, agentID string) *KeyBuilder
```

---

## Option Functions

### `CacheOption`

Functional options for `NewCache`.

```go
type CacheOption func(*cacheConfig)
```

| Function | Signature | Description |
|----------|-----------|-------------|
| `WithNamespace` | `func(namespace string) CacheOption` | Set default namespace for the cache instance |
| `WithAgentID` | `func(agentID string) CacheOption` | Set default agentID for the cache instance |
| `WithDefaultTTL` | `func(ttl time.Duration) CacheOption` | Set default TTL for the namespace |
| `WithDefaultTags` | `func(tags ...string) CacheOption` | Set default tags for all entries |
| `WithInvalidation` | `func(strategy InvalidationStrategy) CacheOption` | Set invalidation strategy |
| `WithStampedeProtection` | `func(enabled bool) CacheOption` | Enable/disable single-flight coalescing |
| `WithObserver` | `func(observer CacheObserver) CacheOption` | Wire observability (required for telemetry) |
| `WithSingleFlight` | `func(sf SingleFlight) CacheOption` | Provide custom SingleFlight implementation |
| `WithWarming` | `func(cfg WarmingConfig) CacheOption` | Configure cache warming |
| `WithMaxSize` | `func(bytes int64) CacheOption` | Set maximum cache size for the namespace |

### `SetOption`

Functional options for `Set`, `SetMany`, and `GetOrFetch`.

| Function | Signature | Description |
|----------|-----------|-------------|
| `WithTTL` | `func(ttl time.Duration) SetOption` | Override default TTL |
| `WithTags` | `func(tags ...string) SetOption` | Associate tags for tag-based invalidation |
| `WithVersion` | `func(version string) SetOption` | Set version for versioned invalidation |

---

## Constants

### `InvalidationStrategy`

Defines how entries become stale.

```go
type InvalidationStrategy int

const (
    InvalidationTTL          InvalidationStrategy = iota // Standard TTL expiration
    InvalidationEventDriven                              // NATS-driven invalidation
    InvalidationVersioned                                // PostgreSQL version stamp comparison
    InvalidationHybrid                                   // Event-driven primary + TTL safety net
)
```

| Constant | Value | Description |
|----------|-------|-------------|
| `InvalidationTTL` | `0` | Entries expire after TTL. No external coordination. |
| `InvalidationEventDriven` | `1` | Service adapters subscribe to NATS; events trigger deletion. |
| `InvalidationVersioned` | `2` | On read, compare cached version against PostgreSQL version stamp. |
| `InvalidationHybrid` | `3` | Event-driven primary with TTL as fallback for missed events. |

### Operation Types

Constants for `UsageEvent` emission via `CacheObserver`.

```go
const (
    OperationTypeCacheHit        = "cache-hit"
    OperationTypeCacheMiss       = "cache-miss"
    OperationTypeCacheWrite      = "cache-write"
    OperationTypeCacheInvalidate = "cache-invalidate"
    OperationTypeCacheEvict      = "cache-evict"
    OperationTypeCacheWarming    = "cache-warming"
)
```

### Resource Type

```go
const (
    ResourceTypeCache = "cache"
)
```

---

## Configuration

### `Config`

Top-level cache configuration.

```go
type Config struct {
    Namespace          string
    DefaultTTL         time.Duration
    MaxSize            int64
    Invalidation       InvalidationStrategy
    StampedeProtection bool
    Warming            *WarmingConfig
    ValkeyURL          string
}
```

### `NamespaceConfig`

Per-namespace configuration boundaries.

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

**Default namespaces:**

| Namespace | Default TTL | Invalidation | Warming | Stampede Protection |
|-----------|-------------|--------------|---------|---------------------|
| `cognitive-engine` | 5 min | Hybrid | Yes (startup) | Yes |
| `memory-manager` | 10 min | Versioned | Yes (startup) | Yes |
| `tool-executor` | 15 min | Event-driven | No | Yes |
| `skill-config` | 30 min | Event-driven | No | No |
| `llm-completions` | 1 hour | TTL-only | No | Yes |
| `embeddings` | 24 hours | Versioned | No | No |

### `WarmingConfig`

Cache warming configuration.

```go
type WarmingConfig struct {
    Enabled     bool
    Deadline    time.Duration
    WarmFunc    WarmFunc
    OnStartup   bool
    NATSTrigger bool
}

type WarmFunc func(ctx context.Context, cache Cache) error
```

| Field | Description |
|-------|-------------|
| `Enabled` | Enable warming for this namespace |
| `Deadline` | Maximum time to wait before starting service in degraded mode |
| `WarmFunc` | Function that populates the cache — called with a `Cache` instance scoped to the namespace |
| `OnStartup` | Execute warming when the service starts |
| `NATSTrigger` | Allow re-warming triggered by NATS events (service adapter wires this) |

### `WarmingManager`

Orchestrates cache warming operations across namespaces.

```go
type WarmingManager interface {
    Warm(ctx context.Context, namespace string) error
    WarmOnStartup(ctx context.Context) error
    TrackProgress(namespace string) WarmingProgress
}
```

| Method | Returns | Description |
|--------|---------|-------------|
| `Warm` | `error` | Execute `WarmingConfig.WarmFunc` for the given namespace with the configured deadline context |
| `WarmOnStartup` | `error` | Warm all namespaces that have `WarmingConfig.OnStartup = true`. Exceeding a namespace's deadline starts the service in degraded mode for that namespace. |
| `TrackProgress` | `WarmingProgress` | Returns current warming progress for a namespace |

---

## Error Types

All errors are sentinel values for reliable `errors.Is()` matching.

```go
var (
    ErrCacheMiss           = errors.New("cache: key not found")
    ErrBackendUnavailable  = errors.New("cache: backend connection failed")
    ErrAgentIDMissing      = errors.New("cache: agentId missing from context")
    ErrInvalidKey          = errors.New("cache: key format does not match convention")
    ErrTTLExpired          = errors.New("cache: entry TTL has expired")
    ErrVersionMismatch     = errors.New("cache: cached version does not match current")
    ErrStampedeLock        = errors.New("cache: single-flight lock acquisition timeout")
    ErrFetchFailed         = errors.New("cache: fetch function returned error")
    ErrWarmingTimeout      = errors.New("cache: warming exceeded deadline")
    ErrMaxSizeExceeded     = errors.New("cache: write would exceed namespace size limit")
    ErrSerializationFailed = errors.New("cache: value serialization/deserialization failed")
    ErrNATSDisconnected    = errors.New("cache: NATS connection lost for invalidation")
    ErrPatternInvalid      = errors.New("cache: invalid glob/regex pattern")
    ErrTagNotFound         = errors.New("cache: tag does not exist in index")
)
```

| Error | Condition | Handling |
|-------|-----------|----------|
| `ErrCacheMiss` | Key not found | Not an error in the traditional sense — `Get` returns `(nil, nil)`, caller checks for nil value |
| `ErrBackendUnavailable` | Valkey connection failed | Log error, emit UsageEvent, service continues without cache |
| `ErrAgentIDMissing` | `agentId` not in `context.Context` | Terminal — no operation proceeds without attribution |
| `ErrInvalidKey` | Key doesn't match `{namespace}:{agentId}:{entityType}:{entityId}:{version}` | Return error with validation details |
| `ErrVersionMismatch` | Cached version ≠ current version (from PostgreSQL) | Treat as miss, trigger re-fetch |
| `ErrStampedeLock` | Single-flight lock timeout | Caller may retry |
| `ErrFetchFailed` | `FetchFunc` returned error | Propagated to all single-flight waiters; not cached |
| `ErrWarmingTimeout` | Warming exceeded `WarmingConfig.Deadline` | Service starts in degraded mode; background warming continues |
| `ErrMaxSizeExceeded` | Write exceeds `NamespaceConfig.MaxSize` | Reject write, emit eviction event |
| `ErrPatternInvalid` | Invalid pattern in `DeletePattern` | Return error with pattern validation message |
| `ErrTagNotFound` | Tag not in index | No-op (idempotent delete) |

**Error propagation rules:**
- Cache miss is not an error — callers check for nil value.
- Backend errors are logged and emitted as UsageEvents but may be degraded (return stale data).
- Fetch errors propagate to all single-flight waiters.
- `agentId` errors are always terminal.
- Warming errors are non-blocking (service starts in degraded mode).

---

## Usage Examples

### Basic Operations

```go
// Create backend and cache
backend, err := caching.NewValkeyBackend(caching.ValkeyConfig{
    URL: "redis://valkey:6379",
})
if err != nil {
    log.Fatal(err)
}
defer backend.Close()

cache := caching.NewCache(backend,
    caching.WithNamespace("cognitive-engine"),
    caching.WithDefaultTTL(5*time.Minute),
    caching.WithObserver(myObserver),
)

// Scoped to a specific agent
agentCache := cache.WithAgentID("agent-alpha")

// Set a value
err = agentCache.Set(ctx, "decision-tree-456", data,
    caching.WithTTL(10*time.Minute),
    caching.WithTags("decision-tree", "tree-456"),
)

// Get a value
val, err := agentCache.Get(ctx, "decision-tree-456")
if val == nil {
    // Cache miss
}

// Delete
err = agentCache.Delete(ctx, "decision-tree-456")
```

### Cache-Aside Pattern (GetOrFetch)

```go
val, err := agentCache.GetOrFetch(ctx, "decision-tree-456",
    func(ctx context.Context) ([]byte, error) {
        // Called only on cache miss
        tree, err := repo.GetDecisionTree(ctx, "456")
        if err != nil {
            return nil, err
        }
        return json.Marshal(tree)
    },
    caching.WithTTL(10*time.Minute),
    caching.WithTags("decision-tree"),
)
if err != nil {
    return err
}
// val is guaranteed non-nil on success
```

Stampede protection is automatic when the namespace has it enabled. Concurrent requests for the same expired key result in exactly one fetch.

### Bulk Operations

```go
// Get multiple keys
results, err := agentCache.GetMany(ctx, []string{
    "decision-tree-456",
    "decision-tree-789",
    "tool-def-review",
})
// results is a map of hits only — misses are omitted
for key, val := range results {
    // process each hit
}

// Set multiple entries
entries := map[string][]byte{
    "decision-tree-456": treeData1,
    "decision-tree-789": treeData2,
}
err = agentCache.SetMany(ctx, entries,
    caching.WithTTL(10*time.Minute),
    caching.WithTags("decision-tree"),
)

// Delete multiple keys
err = agentCache.DeleteMany(ctx, []string{
    "decision-tree-456",
    "decision-tree-789",
})
```

### Pattern-Based Invalidation

```go
// Using KeyBuilder to generate patterns
kb := caching.NewKeyBuilder("cognitive-engine", "agent-alpha")
kb.EntityType("decision-tree")
pattern, _ := kb.Pattern()
// pattern: "cognitive-engine:agent-alpha:decision-tree:*"

err = agentCache.DeletePattern(ctx, pattern)
```

### Tag-Based Invalidation

```go
// Set entries with tags
err = agentCache.Set(ctx, "tree-456", data,
    caching.WithTags("decision-tree", "review-flow"),
)
err = agentCache.Set(ctx, "tree-789", data,
    caching.WithTags("decision-tree", "onboarding-flow"),
)

// Invalidate all entries with a specific tag
err = agentCache.DeleteByTag(ctx, "decision-tree")
// Both tree-456 and tree-789 are deleted
```

### Versioned Invalidation

```go
// Set with a version
err = agentCache.Set(ctx, "tree-456", data,
    caching.WithVersion("v3"),
)

// Later, invalidate if version matches
err = agentCache.InvalidateByVersion(ctx, "tree-456", "v3")
// Entry is deleted

// If version doesn't match, it's a no-op (already stale)
err = agentCache.InvalidateByVersion(ctx, "tree-456", "v2")
// No-op: cached version is v3, expected v2
```

### Scoped Configuration

```go
// Base cache for cognitive-engine namespace
base := caching.NewCache(backend,
    caching.WithNamespace("cognitive-engine"),
    caching.WithDefaultTTL(5*time.Minute),
)

// Per-agent scope — returns new instance
agentAlpha := base.WithAgentID("agent-alpha")
agentBeta := base.WithAgentID("agent-beta")

// These operate on different keys despite the same logical key
agentAlpha.Set(ctx, "config", alphaData)  // key: cognitive-engine:agent-alpha:...
agentBeta.Set(ctx, "config", betaData)    // key: cognitive-engine:agent-beta:...

// Per-operation TTL override
agentAlpha.Set(ctx, "config", data, caching.WithTTL(30*time.Minute))
```

### Custom Observer Wiring

```go
// In service-internal code — wire CacheObserver to shared/telemetry
type TelemetryCacheObserver struct {
    UsagePublisher *telemetry.UsagePublisher
}

func (o *TelemetryCacheObserver) ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64) {
    opType := caching.OperationTypeCacheHit
    if !hit {
        opType = caching.OperationTypeCacheMiss
    }
    o.UsagePublisher.Publish(ctx, telemetry.UsageEvent{
        AgentID:       extractAgentID(ctx),
        OperationType: opType,
        ResourceType:  caching.ResourceTypeCache,
        DurationMs:    latencyMs,
        Metadata: map[string]string{
            "namespace": namespace,
            "key":       key,
        },
    })
}

// ... implement ObserveSet, ObserveDelete, ObserveEviction, ObserveWarming

// Wire into cache
cache := caching.NewCache(backend,
    caching.WithNamespace("cognitive-engine"),
    caching.WithObserver(&TelemetryCacheObserver{UsagePublisher: tel.Usage}),
)
```

---

## Key Format Reference

All keys follow the standardized format:

```
{namespace}:{agentId}:{entityType}:{entityId}:{version}
```

| Component | Source | Example |
|-----------|--------|---------|
| `namespace` | `WithNamespace()` or `Config.Namespace` | `cognitive-engine` |
| `agentId` | `WithAgentID()` — mandatory | `agent-alpha` |
| `entityType` | `KeyBuilder.EntityType()` or logical key prefix | `decision-tree` |
| `entityId` | `KeyBuilder.EntityID()` or logical key suffix | `tree-456` |
| `version` | `KeyBuilder.Version()` or auto-generated | `v3` |

**Example keys:**
```
cognitive-engine:agent-alpha:decision-tree:tree-456:v3
memory-manager:agent-beta:l4-tree:root:v1
tool-executor:agent-alpha:tool-def:review:hash-a1b2c3
```

---

## Files Created

- `design/units/caching-strategies/api.md`
