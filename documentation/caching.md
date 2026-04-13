# Caching

The ACE Framework uses an in-memory cache library at `backend/shared/caching/`. It provides tag-based invalidation, versioned invalidation, stampede protection, bulk operations, cache warming, and observability.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Service Layer                     │
│         (imports shared/caching, uses Cache)         │
├─────────────────────────────────────────────────────┤
│                   Cache Interface                    │
│  Get / Set / Delete / GetOrFetch / GetMany / ...     │
├──────────────┬──────────────────┬───────────────────┤
│ SingleFlight │  CacheObserver   │ WarmingManager    │
│ (stampede)   │  (observability) │ (cache warming)   │
├──────────────┴──────────────────┴───────────────────┤
│                  CacheBackend                        │
│              (memoryBackend)                        │
└─────────────────────────────────────────────────────┘
```

The `Cache` interface is the only thing services import. `CacheBackend` is an internal implementation detail — currently in-memory only.

## Quick Start

```go
import "ace/shared/caching"

// Create backend
backend := caching.NewMemoryBackend()

// Create cache
cache := caching.NewCache(backend,
    caching.WithNamespace("agents"),
    caching.WithDefaultTTL(5*time.Minute),
)

// Basic operations
cache.Set(ctx, "user:123", []byte(`{"name":"Alice"}`),
    caching.WithTags("user", "active"),
)

value, err := cache.Get(ctx, "user:123")
// value = []byte(`{"name":"Alice"}`), err = nil

cache.Delete(ctx, "user:123")
```

## Key Format

All cache keys follow the format: `{namespace}:{agentId}:{entityType}:{entityId}:{version}`

The `agentId` is mandatory — it can be provided via context or set on the cache via `WithAgentID()`.

```go
// Logical key → fully qualified key
cache.Set(ctx, "user:123", value)
// Resolves to: "agents:agent-uuid:user:123:"

// Or set agentID on the cache itself
cache = cache.WithAgentID("agent-uuid")
cache.Set(ctx, "user:123", value)
// Same result: "agents:agent-uuid:user:123:"
```

### KeyBuilder

For explicit key construction:

```go
key, err := caching.NewKeyBuilder("agents", "agent-uuid").
    EntityType("user").
    EntityID("123").
    Build()
// key = "agents:agent-uuid:user:123:"
```

### Patterns

For bulk deletion by pattern:

```go
pattern, err := caching.NewKeyBuilder("agents", "agent-uuid").
    EntityType("user").
    Pattern()
// pattern = "agents:agent-uuid:user:*:*"

cache.DeletePattern(ctx, pattern)
```

## Core Operations

### Get

Returns `(nil, nil)` on cache miss — NOT an error.

```go
value, err := cache.Get(ctx, "user:123")
if err != nil {
    // Backend error (connection failure, etc.)
}
if value == nil {
    // Cache miss — fetch from source
}
```

### Set

```go
err := cache.Set(ctx, "user:123", value,
    caching.WithTTL(10*time.Minute),
    caching.WithTags("user", "active"),
)
```

**Constraints:**
- Value must not be nil
- Value size must not exceed `MaxSize` (default 1MB)
- Tags populate the tag index for later bulk invalidation

### Delete

```go
err := cache.Delete(ctx, "user:123")
```

Automatically cleans up the tag index — the key is removed from all tag sets it belongs to.

### GetOrFetch (Cache-Aside Pattern)

```go
value, err := cache.GetOrFetch(ctx, "user:123", func(ctx context.Context) ([]byte, error) {
    // Expensive operation — database query, API call, etc.
    return db.GetUser(ctx, "123")
}, caching.WithTTL(5*time.Minute))
```

**Stampede protection:** By default enabled. If 50 concurrent goroutines call `GetOrFetch` for the same key simultaneously, the fetch function runs exactly once. The other 49 wait and receive the same result.

## Bulk Operations

### GetMany

```go
results, err := cache.GetMany(ctx, []string{"user:1", "user:2", "user:3"})
// results = map[string][]byte{"user:1": [...], "user:3": [...]}
// "user:2" omitted if it was a miss
```

### SetMany

```go
entries := map[string][]byte{
    "user:1": []byte(`{"name":"Alice"}`),
    "user:2": []byte(`{"name":"Bob"}`),
}
err := cache.SetMany(ctx, entries, caching.WithTags("user"))
```

Batched internally (100 keys per batch).

### DeleteMany

```go
err := cache.DeleteMany(ctx, []string{"user:1", "user:2"})
```

### DeletePattern

```go
// Delete all keys matching pattern
err := cache.DeletePattern(ctx, "agents:agent-uuid:user:*:*")
```

**Safety:** Bare `"*"` is rejected — patterns must include the `agentID` prefix.

### DeleteByTag

```go
// Delete ALL keys tagged with "user"
err := cache.DeleteByTag(ctx, "user")
```

This is the most powerful invalidation method — it deletes every key that was set with the given tag, regardless of namespace or key structure.

## Tag Index

When you `Set` a key with tags, a bidirectional index is maintained:

**Forward index** (tag → keys):
- Key: `_tags:{namespace}:{agentId}:{tag}`
- Type: in-memory set
- On `Set`: add resolvedKey to each tag's set
- On `DeleteByTag`: get all keys, delete each key, then delete the tag set

**Reverse index** (key → tags):
- Key: `_keytags:{resolvedKey}`
- Type: in-memory set
- On `Set`: add all tags to this set
- On `Delete`: get tags, remove key from each tag set, delete the reverse lookup

This ensures that when a key is deleted individually, it is removed from all tag sets it belongs to — preventing stale references.

## Versioned Invalidation

```go
// Set a value with a version
cache.Set(ctx, "config:v2", value, caching.WithVersion("2.0"))

// Later: invalidate only if the cached version is still "2.0"
err := cache.InvalidateByVersion(ctx, "config:v2", "2.0")
// If version matches → key deleted
// If version is different → no-op (already stale)
// If no version stored → no-op (already invalidated)
```

**How it works:**
1. When `Set` is called with `WithVersion()`, the version is stored in `_version:{resolvedKey}`
2. `InvalidateByVersion` reads the stored version and compares
3. If versions match, the key and all associated index entries are deleted

## Scoping

Cache instances are immutable. Scoping methods return new instances:

```go
// Base cache
cache := caching.NewCache(backend,
    caching.WithNamespace("agents"),
    caching.WithAgentID("agent-1"),
    caching.WithDefaultTTL(1*time.Hour),
    caching.WithDefaultTags("global"),
)

// Scoped copies — original is unchanged
agentsCache := cache.WithNamespace("agents")
usersCache := cache.WithNamespace("users")
shortTTLCache := cache.WithDefaultTTL(5*time.Minute)
taggedCache := cache.WithDefaultTags("user", "active")
```

## Statistics

```go
stats, err := cache.Stats(ctx)
// stats.HitCount, stats.MissCount, stats.HitRate
// stats.EntryCount and stats.TotalSize
// stats.EvictionCount and stats.AvgLatencyMs are also available
```

Hit/miss counters are atomic (`sync/atomic.Int64`) and thread-safe. `HitRate` is computed as `hits / (hits + misses)`.

## Observability

Every cache operation calls the `CacheObserver`:

```go
type CacheObserver interface {
    ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64)
    ObserveSet(ctx context.Context, namespace, key string, sizeBytes int64, latencyMs float64)
    ObserveDelete(ctx context.Context, namespace, key, reason string)
    ObserveEviction(ctx context.Context, namespace, key, reason string)
    ObserveWarming(ctx context.Context, namespace string, progress WarmingProgress)
}
```

Provide a custom observer via `caching.WithObserver(observer)`. If not provided, a no-op observer is used.

## Cache Warming

```go
warming := caching.NewWarmingManager([]caching.WarmingConfig{
    {
        Namespace: "agents",
        Enabled:   true,
        OnStartup: true,
        Parallel:  true,
        Deadline:  30 * time.Second,
        WarmFunc: func(ctx context.Context, cache caching.Cache) error {
            agents, err := db.GetAllAgents(ctx)
            if err != nil {
                return err
            }
            for _, a := range agents {
                data, _ := json.Marshal(a)
                if err := cache.Set(ctx, a.ID, data); err != nil {
                    return err
                }
            }
            return nil
        },
    },
}, cache, observer)

// Warm on startup
err := warming.WarmOnStartup(ctx)

// Track progress
progress := warming.TrackProgress("agents")
// progress.SuccessCount, progress.FailureCount, progress.ElapsedMs
```

**WarmOnStartup behavior:**
- Configs with `Parallel: true` run concurrently
- Configs with `Parallel: false` run sequentially
- Errors are aggregated — all namespaces are attempted even if some fail
- Each namespace has a deadline (default 30s) — returns `ErrWarmingTimeout` if exceeded

## Error Handling

All errors are sentinel errors or `CacheError` wrappers:

```go
if err != nil {
    if errors.Is(err, caching.ErrCacheMiss) {
        // Cache miss (note: Get returns nil, nil on miss — not ErrCacheMiss)
    }
    if errors.Is(err, caching.ErrAgentIDMissing) {
        // agentID not provided
    }
    if errors.Is(err, caching.ErrWarmingTimeout) {
        // Warming exceeded deadline
    }
}
```

**Available sentinel errors:**
- `ErrCacheMiss` — key not found
- `ErrBackendUnavailable` — memory backend error
- `ErrAgentIDMissing` — agentID required but not provided
- `ErrInvalidKey` — key validation failed
- `ErrTTLExpired` — TTL has expired
- `ErrVersionMismatch` — version doesn't match expected
- `ErrStampedeLock` — stampede protection failed
- `ErrFetchFailed` — fetch function returned error
- `ErrWarmingTimeout` — warming exceeded deadline
- `ErrMaxSizeExceeded` — value exceeds 1MB limit
- `ErrSerializationFailed` — value is nil
- `ErrPatternInvalid` — pattern is `"*"` or empty
- `ErrTagNotFound` — tag is empty

## Configuration Reference

### Cache Defaults

| Setting | Default |
|---------|---------|
| `DefaultTTL` | 1 hour |
| `MaxSize` | 1 MB |
| `StampedeProtection` | Enabled |
| `InvalidationStrategy` | TTL |

## Unit Tests

```bash
# Run unit tests only
go test ./backend/shared/caching/... -v
```

## Constraints

- **`shared/caching` is transport-agnostic** — never imports `net/http`, NATS, or any transport package
- **No `any` or `interface{}`** — except `SingleFlight.Do` which wraps `x/sync/singleflight`
- **Cache instances are immutable** — scoping methods return new instances, never mutate
- **Get returns `(nil, nil)` on miss** — not `ErrCacheMiss`
- **Bare `"*"` patterns are rejected** — must include `agentID` prefix
- **Tag index keys use `_tags:` and `_keytags:` prefixes** — internal, not exposed in public API
