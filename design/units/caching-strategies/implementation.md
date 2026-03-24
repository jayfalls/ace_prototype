# Implementation Plan — shared/caching

## Overview

This document defines the step-by-step execution plan for building the `shared/caching` package — a transport-agnostic Go library providing Valkey-backed cache operations, invalidation strategies, stampede protection, and observability integration.

**Package path:** `ace/shared/caching`

**Implementation principle:** Build bottom-up. Types and interfaces first, then the backend layer, then the high-level `Cache` interface, then advanced features. Each phase produces independently testable code.

---

## Phase 1: Package Scaffolding & Types

Create the package directory, Go module entry, and all public types/interfaces. No logic — just the type surface that other phases depend on.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 1.1 | Create `shared/caching/` directory | None |
| 1.2 | Create `shared/caching/types.go` — all public interfaces (`Cache`, `CacheBackend`, `CacheObserver`, `SingleFlight`, `WarmingManager`), core types (`FetchFunc`, `SetOption`, `SetOptions`, `CacheEntry`, `CacheStats`, `WarmingProgress`, `SingleFlightResult`, `VersionStamp`, `InvalidationEvent`, `KeyBuilder` struct definition, `ValkeyConfig`, `Config`, `NamespaceConfig`, `WarmingConfig`, `WarmFunc`), constants (`InvalidationStrategy` iota values, operation types, resource type) | 1.1 |
| 1.3 | Create `shared/caching/errors.go` — all sentinel error variables (`ErrCacheMiss`, `ErrBackendUnavailable`, `ErrAgentIDMissing`, `ErrInvalidKey`, `ErrTTLExpired`, `ErrVersionMismatch`, `ErrStampedeLock`, `ErrFetchFailed`, `ErrWarmingTimeout`, `ErrMaxSizeExceeded`, `ErrSerializationFailed`, `ErrNATSDisconnected`, `ErrPatternInvalid`, `ErrTagNotFound`) | 1.1 |
| 1.4 | Create `shared/caching/options.go` — `CacheOption` functional options (`WithNamespace`, `WithAgentID`, `WithDefaultTTL`, `WithDefaultTags`, `WithInvalidation`, `WithStampedeProtection`, `WithObserver`, `WithSingleFlight`, `WithWarming`, `WithMaxSize`), `cacheConfig` internal struct, `SetOption` functional options (`WithTTL`, `WithTags`, `WithVersion`) | 1.2 |
| 1.5 | Verify all types compile: `go build ./shared/caching/...` | 1.2, 1.3, 1.4 |

### Deliverables
- `shared/caching/types.go` — all interfaces and types
- `shared/caching/errors.go` — all sentinel errors
- `shared/caching/options.go` — all functional option constructors and internal config struct

### Verification
```bash
go build ./shared/caching/...
go vet ./shared/caching/...
```
All types compile. No logic errors possible — this is pure type definitions.

---

## Phase 2: KeyBuilder

Implement the key construction and validation logic. KeyBuilder is used by every other component.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 2.1 | Create `shared/caching/key_builder.go` — `NewKeyBuilder(namespace, agentID string) *KeyBuilder` constructor. Return error-free (store values, validate on `Build()`). | 1.2 |
| 2.2 | Implement `EntityType(t string) *KeyBuilder` — set entityType, return self for chaining | 2.1 |
| 2.3 | Implement `EntityID(id string) *KeyBuilder` — set entityID, return self for chaining | 2.1 |
| 2.4 | Implement `Version(v string) *KeyBuilder` — set version, return self for chaining | 2.1 |
| 2.5 | Implement `Build() (string, error)` — validate: namespace not empty, agentID not empty, no colon in any component, key length < 1024 bytes. Return `ErrInvalidKey` if namespace empty, components contain colons, or key too long. Return `ErrAgentIDMissing` if agentID empty. Format: `{namespace}:{agentId}:{entityType}:{entityId}:{version}` | 2.1–2.4 |
| 2.6 | Implement `Pattern() (string, error)` — same validation as `Build()`. Unset components (entityType, entityID, version) become `*`. Return `ErrInvalidKey` if namespace empty or components contain colons. Return `ErrAgentIDMissing` if agentID empty. | 2.1–2.4 |
| 2.7 | Create `shared/caching/key_builder_test.go` — test all validation rules: empty agentID → `ErrAgentIDMissing`, empty namespace → `ErrInvalidKey`, colon in component → `ErrInvalidKey`, key too long → `ErrInvalidKey`, normal build succeeds, pattern generation with partial components, pattern generation with all components set | 2.5, 2.6 |

### Deliverables
- `shared/caching/key_builder.go`
- `shared/caching/key_builder_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestKeyBuilder -v
```
- `NewKeyBuilder("", "agent")` → Build returns `ErrInvalidKey`
- `NewKeyBuilder("ns", "")` → Build returns `ErrAgentIDMissing`
- `NewKeyBuilder("ns", "agent:bad")` → Build returns `ErrInvalidKey`
- `NewKeyBuilder("ns", "agent").EntityType("et").EntityID("id").Version("v1").Build()` → returns `"ns:agent:et:id:v1", nil`
- `NewKeyBuilder("ns", "agent").Pattern()` → returns `"ns:agent:*", nil`
- `NewKeyBuilder("ns", "agent").EntityType("et").Pattern()` → returns `"ns:agent:et:*", nil`

---

## Phase 3: ValkeyBackend

Implement the `CacheBackend` interface backed by `valkey-go`. This is the sole cache backend.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 3.1 | Create `shared/caching/valkey_backend.go` — `valkeyBackend` struct with `client valkey.Client` field | 1.2 |
| 3.2 | Implement `NewValkeyBackend(cfg ValkeyConfig) (CacheBackend, error)` — create `valkey.Client` with config defaults (MaxRetries=3, DialTimeout=5s, ReadTimeout=3s, WriteTimeout=3s, PoolSize=100). Return error if connection fails. | 3.1 |
| 3.3 | Implement `Get(ctx, key) ([]byte, error)` — call `client.Do(ctx, client.B().Get().Key(key).Build())`. Return `ErrCacheMiss` on valkey nil response, `ErrBackendUnavailable` on connection error. | 3.1 |
| 3.4 | Implement `Set(ctx, key, value, ttl) error` — call `client.Do(ctx, client.B().Set().Key(key).Value(string(value)).Ex(ttl).Build())`. Return `ErrBackendUnavailable` on error. | 3.1 |
| 3.5 | Implement `Delete(ctx, key) error` — call `client.Do(ctx, client.B().Del().Key(key).Build())`. Idempotent — no error on missing key. | 3.1 |
| 3.6 | Implement `GetMany(ctx, keys) (map[string][]byte, error)` — call `client.Do(ctx, client.B().Mget().Key(keys...).Build())`. Parse array response. Omit nil (miss) entries from map. | 3.1 |
| 3.7 | Implement `SetMany(ctx, entries, ttl) error` — use `client.DoMulti()` with individual SET+EX commands (Valkey MSET doesn't support per-key TTL). Batch in groups of 100 to avoid oversized commands. | 3.1 |
| 3.8 | Implement `DeleteMany(ctx, keys) error` — call `client.Do(ctx, client.B().Del().Key(keys...).Build())`. | 3.1 |
| 3.9 | Implement `DeletePattern(ctx, pattern) error` — use SCAN loop (cursor-based, batch size 100) to find matching keys, then DEL in batches. Reject bare `*` pattern with `ErrPatternInvalid`. | 3.1 |
| 3.10 | Implement `DeleteByTag(ctx, tag) error` — read Valkey set at `_tags:{tag}`, then delete all members and the set itself. | 3.1 |
| 3.11 | Implement `Exists(ctx, key) (bool, error)` — call `client.Do(ctx, client.B().Exists().Key(key).Build())`. Return true if result > 0. | 3.1 |
| 3.12 | Implement `TTL(ctx, key) (time.Duration, error)` — call `client.Do(ctx, client.B().Ttl().Key(key).Build())`. Parse integer response as seconds. Return -1 if no TTL, -2 if key doesn't exist. | 3.1 |
| 3.13 | Implement `Close() error` — call `client.Close()`. | 3.1 |
| 3.14 | Create `shared/caching/valkey_backend_test.go` — use `valkey.NewMockClient()` for unit tests. Test all operations: Get hit/miss, Set with TTL, Delete, GetMany partial hits, SetMany, DeleteMany, DeletePattern, Exists, TTL, Close. | 3.2–3.13 |

### Deliverables
- `shared/caching/valkey_backend.go`
- `shared/caching/valkey_backend_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestValkeyBackend -v
```
- Get returns `ErrCacheMiss` for missing key
- Set stores value, Get retrieves it
- Set with TTL, TTL returns correct remaining duration
- Delete removes key, subsequent Get returns `ErrCacheMiss`
- GetMany returns only hits
- DeletePattern with valid pattern succeeds
- DeletePattern with bare `*` returns `ErrPatternInvalid`
- Close shuts down cleanly

---

## Phase 4: SingleFlight

Implement stampede protection wrapping `golang.org/x/sync/singleflight`.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 4.1 | Create `shared/caching/singleflight.go` — `singleFlightImpl` struct wrapping `singleflight.Group` | 1.2 |
| 4.2 | Implement `Do(key string, fn func() (interface{}, error)) (interface{}, error, bool)` — delegate to inner `group.Do`. The `shared` bool indicates whether the result was from another caller. | 4.1 |
| 4.3 | Implement `DoChan(key string, fn func() (interface{}, error)) <-chan SingleFlightResult` — delegate to inner `group.DoChan`, convert `singleflight.Result` to `SingleFlightResult`. | 4.1 |
| 4.4 | Implement `NewSingleFlight() SingleFlight` constructor | 4.1 |
| 4.5 | Create `shared/caching/singleflight_test.go` — test concurrent calls with same key (only 1 fetch executes), test `shared` flag correctness, test `DoChan` returns result on channel, test error propagation to all waiters | 4.2, 4.3 |

### Deliverables
- `shared/caching/singleflight.go`
- `shared/caching/singleflight_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestSingleFlight -v
```
- 100 concurrent `Do` calls with same key → fetch function called exactly once
- `shared=true` for all but the first caller
- Error from fetch propagates to all waiters
- `DoChan` receives result on returned channel

---

## Phase 5: Cache Core (Get, Set, Delete, GetOrFetch)

Implement the `Cache` interface core operations. This is the central component.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 5.1 | Create `shared/caching/cache.go` — `cacheImpl` struct with fields: `backend CacheBackend`, `sf SingleFlight`, `observer CacheObserver`, and all `cacheConfig` fields (namespace, agentID, defaultTTL, defaultTags, invalidation strategy, stampedeProtection, maxSize, warming) | 1.2, 1.4, 3.1, 4.1 |
| 5.2 | Implement `NewCache(backend CacheBackend, opts ...CacheOption) Cache` — apply options to `cacheConfig`, create default SingleFlight if stampede protection enabled, create no-op observer if none provided. Return `*cacheImpl`. | 5.1 |
| 5.3 | Implement `resolveKey(logicalKey string) (string, error)` — internal method. Use `KeyBuilder` with cache's namespace and agentID. If logicalKey already contains colons, validate it as a full key. Return fully qualified key. | 5.1, 2.5 |
| 5.4 | Implement `Get(ctx, key) ([]byte, error)` — resolve key, call `backend.Get`, call `observer.ObserveGet` with hit/miss and latency. Return `(nil, nil)` on miss (not `ErrCacheMiss`). Return `ErrAgentIDMissing` if agentID is empty. | 5.3 |
| 5.5 | Implement `Set(ctx, key, value, opts...) error` — resolve key, validate value not nil, check `maxSize` against `len(value)`, apply `SetOptions` (TTL defaults to namespace default, tags, version), call `backend.Set`, if tags provided call tag index update (store key in `_tags:{tag}` sets), call `observer.ObserveSet` with size and latency. Return `ErrMaxSizeExceeded` if value too large. | 5.3 |
| 5.6 | Implement `Delete(ctx, key) error` — resolve key, call `backend.Delete`, call `observer.ObserveDelete` with reason "manual". | 5.3 |
| 5.7 | Implement `GetOrFetch(ctx, key, fetchFn, opts...) ([]byte, error)` — resolve key. If stampede protection enabled: wrap in `sf.Do`. Inside the wrapped function: call `backend.Get`, if hit return value, if miss call `fetchFn`, if success call `backend.Set` with options, return value. If stampede protection disabled: same logic without single-flight. Call observer for hit/miss/write. On `fetchFn` error: return error directly (do not cache). | 5.3, 5.4, 5.5, 4.2 |
| 5.8 | Implement `WithNamespace(namespace string) Cache` — return new `cacheImpl` copy with namespace replaced, sharing same backend and observer | 5.1 |
| 5.9 | Implement `WithAgentID(agentID string) Cache` — return new `cacheImpl` copy with agentID replaced | 5.1 |
| 5.10 | Implement `WithDefaultTTL(ttl time.Duration) Cache` — return new `cacheImpl` copy with defaultTTL replaced | 5.1 |
| 5.11 | Implement `WithDefaultTags(tags ...string) Cache` — return new `cacheImpl` copy with defaultTags replaced | 5.1 |
| 5.12 | Create `shared/caching/cache_test.go` — use mock backend. Test: Get miss returns (nil, nil), Get hit returns value, Set stores value, Delete removes value, GetOrFetch calls fetchFn on miss, GetOrFetch returns cached value on hit, GetOrFetch with stampede protection coalesces concurrent calls, WithNamespace returns new instance, WithAgentID returns new instance, agentID missing returns `ErrAgentIDMissing`, value exceeding maxSize returns `ErrMaxSizeExceeded`, nil value returns error | 5.4–5.11 |

### Deliverables
- `shared/caching/cache.go`
- `shared/caching/cache_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestCache -v
```
- `Get` on empty cache → `(nil, nil)`
- `Set` then `Get` → returns stored value
- `Delete` then `Get` → `(nil, nil)`
- `GetOrFetch` miss → calls fetchFn, returns value, stores in cache
- `GetOrFetch` hit → does not call fetchFn, returns cached value
- 100 concurrent `GetOrFetch` with same key → fetchFn called exactly once
- `WithNamespace("other")` → keys resolved under different namespace
- `WithAgentID("other")` → keys resolved under different agentID
- No agentID → `ErrAgentIDMissing`
- Value > MaxSize → `ErrMaxSizeExceeded`
- Nil value → error

---

## Phase 6: Bulk Operations

Extend the cache with batch operations.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 6.1 | Implement `GetMany(ctx, keys) (map[string][]byte, error)` — resolve all keys, call `backend.GetMany`, call observer for each key (hit/miss). Return map of hits only. | 5.3 |
| 6.2 | Implement `SetMany(ctx, entries, opts...) error` — resolve all keys, validate all values not nil, check maxSize per value, call `backend.SetMany` with TTL, update tag index for all entries if tags provided, call observer for each entry. | 5.3 |
| 6.3 | Implement `DeleteMany(ctx, keys) error` — resolve all keys, call `backend.DeleteMany`, call observer for each key with reason "manual". | 5.3 |
| 6.4 | Implement `DeletePattern(ctx, pattern) error` — validate pattern is not bare `*`, validate pattern includes agentID prefix, call `backend.DeletePattern`, call observer with reason "pattern". Return `ErrPatternInvalid` on validation failure. | 5.3 |
| 6.5 | Implement `DeleteByTag(ctx, tag) error` — validate tag not empty, call `backend.DeleteByTag`, call observer with reason "tag". | 5.3 |
| 6.6 | Extend `cache_test.go` — test: GetMany partial hits, SetMany with tags, DeleteMany, DeletePattern rejects bare `*`, DeletePattern with valid pattern, DeleteByTag | 6.1–6.5 |

### Deliverables
- Bulk operation methods added to `shared/caching/cache.go`
- Extended `shared/caching/cache_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestCacheBulk -v
```
- `GetMany` with 2 of 3 keys present → map has 2 entries
- `SetMany` stores all entries
- `DeleteMany` removes all specified keys
- `DeletePattern("*")` → `ErrPatternInvalid`
- `DeletePattern("ns:agent:entity:*")` → succeeds
- `DeleteByTag("tag")` → removes all entries with that tag

---

## Phase 7: Invalidation Strategies

Implement versioned invalidation and tag-based index management.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 7.1 | Implement tag index update in `Set` — when `SetOptions.Tags` is non-empty, for each tag call `backend` to SADD the key to `_tags:{namespace}:{agentId}:{tag}` set | 5.5 |
| 7.2 | Implement tag index cleanup in `Delete` — when deleting a key, read its tags from a `_keytags:{key}` set and SREM the key from each `_tags:...` set, then delete `_keytags:{key}` | 5.6 |
| 7.3 | Implement `InvalidateByVersion(ctx, key, expectedVersion) error` — resolve key, call `backend.Get` to read current entry. If nil (miss): return nil (already invalidated). Read version from a separate `_version:{key}` Valkey key (consistent with tag index pattern using separate `_tags:` keys). If stored version != expectedVersion: return nil (no-op, already stale). If match: call `backend.Delete` and delete `_version:{key}`, call observer with reason "version". | 5.3, 5.6 |
| 7.4 | Extend `cache_test.go` — test: Set with tags populates tag index, DeleteByTag removes tagged entries, InvalidateByVersion deletes when version matches, InvalidateByVersion no-op when version mismatches, InvalidateByVersion no-op when key is already missing | 7.1–7.3 |

### Deliverables
- Tag index management in `shared/caching/cache.go`
- `InvalidateByVersion` in `shared/caching/cache.go`
- Extended `shared/caching/cache_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestInvalidation -v
```
- Set with tags → tag index has entries
- DeleteByTag → all entries with tag removed
- InvalidateByVersion with matching version → key deleted
- InvalidateByVersion with mismatched version → no-op
- InvalidateByVersion on missing key → no-op

---

## Phase 8: CacheObserver Integration

Ensure all operations emit telemetry. The observer is already wired in Phases 5–7, but this phase adds the no-op observer and validates full coverage.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 8.1 | Create `shared/caching/noop_observer.go` — `noopObserver` struct implementing `CacheObserver` with all methods as no-ops. Export `NewNoopObserver() CacheObserver`. | 1.2 |
| 8.2 | Audit all cache operations — verify every `Get`, `Set`, `Delete`, `GetOrFetch`, `GetMany`, `SetMany`, `DeleteMany`, `DeletePattern`, `DeleteByTag`, `InvalidateByVersion` calls the appropriate observer method | 5.4–7.3 |
| 8.3 | Implement `Stats(ctx) (*CacheStats, error)` — query backend for approximate stats. `cacheImpl` holds a reference to the raw `valkey.Client` (not just `CacheBackend`) for Valkey-specific commands: `DBSIZE` for entry count, `INFO memory` for total size. Hit/miss counts tracked in-memory via atomic counters on `cacheImpl`. | 5.1 |
| 8.4 | Extend `cache_test.go` — test: no-op observer does not panic, custom observer receives calls for each operation type, Stats returns correct hit/miss counts | 8.1–8.3 |

### Deliverables
- `shared/caching/noop_observer.go`
- Observer calls verified in `shared/caching/cache.go`
- `Stats` implementation in `shared/caching/cache.go`
- Extended `shared/caching/cache_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestObserver -v
```
- No-op observer: all operations complete without panic
- Custom observer: `ObserveGet` called on every `Get`, `ObserveSet` on every `Set`, etc.
- `Stats` after 10 hits and 5 misses → `HitCount=10`, `MissCount=5`, `HitRate=0.667`

---

## Phase 9: WarmingManager

Implement cache warming orchestration.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 9.1 | Create `shared/caching/warming.go` — `warmingManagerImpl` struct with fields: `configs map[string]WarmingConfig`, `cache Cache`, `observer CacheObserver`, `progress map[string]*WarmingProgress` (sync.Map or mutex-protected) | 1.2, 5.1 |
| 9.2 | Implement `NewWarmingManager(configs []WarmingConfig, cache Cache, observer CacheObserver) WarmingManager` constructor | 9.1 |
| 9.3 | Implement `Warm(ctx, namespace) error` — lookup WarmingConfig for namespace, create context with deadline, call `WarmFunc` with a cache scoped to the namespace, track progress via `WarmingProgress`, call `observer.ObserveWarming` periodically. If deadline exceeded: return `ErrWarmingTimeout`. | 9.1 |
| 9.4 | Implement `WarmOnStartup(ctx) error` — iterate all configs where `OnStartup=true`, call `Warm` for each, collect errors. Return first error (or multi-error) if any namespace fails. Non-blocking: log failures but continue. | 9.3 |
| 9.5 | Implement `TrackProgress(namespace) WarmingProgress` — return current progress snapshot for the namespace | 9.1 |
| 9.6 | Create `shared/caching/warming_test.go` — test: WarmOnStartup calls WarmFunc for enabled namespaces, Warm respects deadline (context cancellation), TrackProgress returns accurate counts, Warm with failing WarmFunc returns error, Warm with slow WarmFunc returns `ErrWarmingTimeout` | 9.3–9.5 |

### Deliverables
- `shared/caching/warming.go`
- `shared/caching/warming_test.go`

### Verification
```bash
go test ./shared/caching/ -run TestWarming -v
```
- `WarmOnStartup` calls `WarmFunc` for each namespace with `OnStartup=true`
- `Warm` with deadline exceeded → `ErrWarmingTimeout`
- `TrackProgress` reflects entries populated during warming
- `WarmFunc` error → propagated to caller

---

## Phase 10: Database Migration (version_stamps table)

Create the PostgreSQL migration for versioned invalidation support.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 10.1 | Create migration file `backend/services/api/migrations/YYYYMMDDHHMMSS_create_version_stamps.go` — Goose Go function migration. Table schema: `key VARCHAR(512) PRIMARY KEY`, `version VARCHAR(255) NOT NULL`, `source_hash VARCHAR(64)`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `updated_by VARCHAR(255)`. Index on `key`. | None |
| 10.2 | Create SQLC query file `backend/services/api/internal/repository/queries/version_stamps.sql` — queries: `GetVersionStamp` (:one, by key), `UpsertVersionStamp` (:exec, INSERT ON CONFLICT UPDATE), `DeleteVersionStamp` (:exec, by key) | 10.1 |
| 10.3 | Run `sqlc generate` to generate typed Go code | 10.2 |
| 10.4 | Create `shared/caching/version_store.go` — **unexported** `versionStore` interface with `GetVersion(ctx, key) (VersionStamp, error)` and `SetVersion(ctx, stamp VersionStamp) error`. These are thin wrappers calling the SQLC-generated repository. The unexported interface allows `cacheImpl` to query versions without importing `shared/database` directly. Services provide the implementation via `WithVersionStore()` option. | 10.3 |
| 10.5 | Verify migration: run `make up`, connect to PostgreSQL, verify `version_stamps` table exists with correct schema | 10.1 |

### Deliverables
- Goose migration file for `version_stamps` table
- SQLC query file for version stamp CRUD
- `shared/caching/version_store.go` interface

### Verification
```bash
make up
docker exec -it ace_db psql -U ace -d ace -c "\d version_stamps"
```
- Table `version_stamps` exists with correct columns
- Primary key on `key`
- SQLC generated files compile

---

## Phase 11: Integration & Final Verification

End-to-end verification that all components work together.

### Tasks

| Task | Description | Dependencies |
|------|-------------|--------------|
| 11.1 | Run full test suite: `go test ./shared/caching/... -v -race -count=1` | All phases |
| 11.2 | Verify `go vet` and `staticcheck` pass with no warnings | All phases |
| 11.3 | Verify no forbidden imports: `go list -f '{{.Imports}}' ./shared/caching/` — must NOT contain `net/http`, `github.com/nats-io/nats.go`, or any transport package | All phases |
| 11.4 | Write integration test `shared/caching/integration_test.go` — requires running Valkey. Test full lifecycle: create backend, create cache, Set with tags and version, Get returns value, GetOrFetch coalesces concurrent requests, DeleteByTag removes entries, InvalidateByVersion works, Stats reflects operations. | All phases |
| 11.5 | Verify `Cache` interface is satisfied: `var _ Cache = (*cacheImpl)(nil)` in `cache.go` | 5.1 |
| 11.6 | Verify `CacheBackend` interface is satisfied: `var _ CacheBackend = (*valkeyBackend)(nil)` in `valkey_backend.go` | 3.1 |
| 11.7 | Verify `SingleFlight` interface is satisfied: `var _ SingleFlight = (*singleFlightImpl)(nil)` in `singleflight.go` | 4.1 |
| 11.8 | Verify `WarmingManager` interface is satisfied: `var _ WarmingManager = (*warmingManagerImpl)(nil)` in `warming.go` | 9.1 |

### Deliverables
- All tests pass
- No forbidden imports
- Interface satisfaction checks pass
- Integration test passes with live Valkey

### Verification
```bash
go test ./shared/caching/... -v -race -count=1
go vet ./shared/caching/...
# Verify no forbidden imports
go list -f '{{join .Imports "\n"}}' ./shared/caching/ | grep -E "net/http|nats.io" && echo "FAIL: forbidden import" || echo "PASS"
```

---

## Implementation Checklist

- [ ] **Phase 1: Types & Constants**
  - [ ] `shared/caching/types.go` — all interfaces and types
  - [ ] `shared/caching/errors.go` — all sentinel errors
  - [ ] `shared/caching/options.go` — functional options and internal config
  - [ ] Compilation verified

- [ ] **Phase 2: KeyBuilder**
  - [ ] `shared/caching/key_builder.go` — key construction and validation
  - [ ] `shared/caching/key_builder_test.go` — all validation rules tested
  - [ ] Tests pass

- [ ] **Phase 3: ValkeyBackend**
  - [ ] `shared/caching/valkey_backend.go` — valkey-go wrapper
  - [ ] All 11 `CacheBackend` methods implemented
  - [ ] `shared/caching/valkey_backend_test.go` — mock-based tests
  - [ ] Tests pass

- [ ] **Phase 4: SingleFlight**
  - [ ] `shared/caching/singleflight.go` — stampede protection wrapper
  - [ ] `shared/caching/singleflight_test.go` — concurrency tests
  - [ ] Tests pass

- [ ] **Phase 5: Cache Core**
  - [ ] `shared/caching/cache.go` — `cacheImpl` struct, `NewCache`, `Get`, `Set`, `Delete`, `GetOrFetch`
  - [ ] Scoping methods: `WithNamespace`, `WithAgentID`, `WithDefaultTTL`, `WithDefaultTags`
  - [ ] `shared/caching/cache_test.go` — core operations tested
  - [ ] Tests pass

- [ ] **Phase 6: Bulk Operations**
  - [ ] `GetMany`, `SetMany`, `DeleteMany` implemented in `cache.go`
  - [ ] `DeletePattern`, `DeleteByTag` implemented in `cache.go`
  - [ ] Bulk operation tests pass

- [ ] **Phase 7: Invalidation Strategies**
  - [ ] Tag index management (Set populates, Delete cleans up)
  - [ ] `InvalidateByVersion` implemented
  - [ ] Invalidation tests pass

- [ ] **Phase 8: CacheObserver**
  - [ ] `shared/caching/noop_observer.go` — no-op observer
  - [ ] All operations emit observer calls (audited)
  - [ ] `Stats` implemented
  - [ ] Observer tests pass

- [ ] **Phase 9: WarmingManager**
  - [ ] `shared/caching/warming.go` — warming orchestration
  - [ ] `shared/caching/warming_test.go` — deadline, progress, error tests
  - [ ] Tests pass

- [ ] **Phase 10: Database Migration**
  - [ ] Goose migration for `version_stamps` table
  - [ ] SQLC queries for version stamp CRUD
  - [ ] `shared/caching/version_store.go` interface
  - [ ] `sqlc generate` succeeds

- [ ] **Phase 11: Integration**
  - [ ] Full test suite passes with `-race`
  - [ ] No forbidden imports verified
  - [ ] Interface satisfaction checks pass
  - [ ] Integration test passes with live Valkey

---

## Rollback Plan

If implementation fails at any phase:

| Phase Failing | Rollback Action |
|---------------|-----------------|
| Phase 1–2 | Delete `shared/caching/` directory. No other code depends on it yet. |
| Phase 3–4 | Delete `shared/caching/`. ValkeyBackend and SingleFlight are internal — no downstream consumers. |
| Phase 5–6 | Delete `shared/caching/`. Cache core has no consumers yet (no service imports it). |
| Phase 7–9 | Revert to Phase 5/6 checkpoint. Advanced features are additive — removing them doesn't break core. |
| Phase 10 | Run Goose down migration: `goose -dir migrations down`. Drop `version_stamps` table. Delete SQLC query file and regenerate. |
| Phase 11 | Review test failures. Fix individual phase — no full rollback needed. |

**General rollback:** `shared/caching` is a new package with zero consumers during implementation. Deletion is always safe until a service begins importing it.

---

## Implementation Notes

- **valkey-go mock:** Use `valkey.NewMockClient()` for all unit tests. The mock supports `Do`, `DoMulti`, and `Close`. Register expected commands via `mock.Register()`.
- **No `interface{}` or `any`:** All types must be explicit per ACE constraints. `SingleFlight.Do` returns `interface{}` because it wraps `x/sync/singleflight` — this is the one exception, and callers must type-assert.
- **Tag index keys use `_tags:` prefix and `_keytags:` prefix** to avoid collision with user cache keys. These are internal implementation details, not exposed in the public API.
- **`cacheImpl` is immutable via scoping methods:** `WithNamespace`, `WithAgentID`, etc. return a new `*cacheImpl` with the field changed. The original is never mutated. This is safe for concurrent use.
- **SCAN batch size:** Default 100 keys per SCAN iteration for `DeletePattern`. Configurable if needed.
- **Integration tests require Valkey:** Mark with `//go:build integration` build tag. Run via `go test -tags=integration ./shared/caching/...`.

---

## Files Created

- `design/units/caching-strategies/implementation.md`
