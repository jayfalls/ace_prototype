# Test Plan — shared/caching

## Overview

`shared/caching` is a transport-agnostic Go library — a shared package, not a service or UI. Testing strategy reflects this: unit tests for every component, integration tests with a live Valkey instance, and no E2E/UI tests. The testing pyramid is inverted at the base — heavy unit test coverage with a focused set of integration tests that validate the full Valkey lifecycle.

All tests use Go's standard `testing` package. Mock dependencies use `valkey.NewMockClient()` from valkey-go and a mock `CacheObserver` interface. PostgreSQL dependency (version stamps) uses a mock `versionStore` interface or testcontainers for integration tests.

---

## Test Strategy

### Testing Pyramid

```
         /\
        /  \      E2E Tests: NONE (library, no UI)
       /----\     
      /      \
     /--------\  Integration Tests (Focused)
    /          \ - Live Valkey lifecycle, full cache flow
   /------------\ Unit Tests (Heavy)
  /              \ - KeyBuilder, ValkeyBackend, Cache, SingleFlight, WarmingManager, Observer
```

### Test Split

| Type | Count (Est.) | Coverage Target | Description |
|------|-------------|-----------------|-------------|
| Unit | ~80% of tests | 90% line coverage | Per-component tests with mocks |
| Integration | ~20% of tests | Full lifecycle coverage | Live Valkey, full cache flow |
| E2E | 0 | N/A | Library — no UI or service layer |

### Test Priorities

| Priority | Coverage Target | Description |
|----------|-----------------|-------------|
| Must | 90% line, 80% branch | All `Cache` interface methods, `KeyBuilder` validation, `SingleFlight` concurrency |
| Must | 100% function | Every exported function/method has at least one test |
| Must | 100% sentinel error | Every sentinel error is triggered by at least one test |
| Should | 80% line | `WarmingManager`, `VersionStamp` store integration |
| Should | 100% observer | Every `CacheObserver` method called for every cache operation |

### Test Tooling

| Tool | Purpose |
|------|---------|
| `go test` | Test runner (standard) |
| `go test -race` | Race condition detection (mandatory for concurrency tests) |
| `go test -cover` | Coverage measurement |
| `valkey.NewMockClient()` | Mock Valkey client for unit tests |
| `go test -tags=integration` | Integration tests requiring live Valkey |
| `testcontainers-go` | Optional: spin up Valkey/PostgreSQL for integration tests |

---

## Unit Tests

### Test File Structure

```
shared/caching/
├── types.go
├── errors.go
├── options.go
├── key_builder.go
├── key_builder_test.go
├── valkey_backend.go
├── valkey_backend_test.go
├── singleflight.go
├── singleflight_test.go
├── cache.go
├── cache_test.go
├── warming.go
├── warming_test.go
├── noop_observer.go
├── version_store.go
└── integration_test.go   // +build integration
```

---

### KeyBuilder Tests (`key_builder_test.go`)

| Test Function | Input | Expected Result |
|--------------|-------|-----------------|
| `TestNewKeyBuilder_ValidInputs` | `NewKeyBuilder("ns", "agent")` | Returns non-nil builder, no error |
| `TestNewKeyBuilder_EmptyAgentID` | `NewKeyBuilder("ns", "")` | Returns builder; `Build()` returns `ErrAgentIDMissing` |
| `TestNewKeyBuilder_EmptyNamespace` | `NewKeyBuilder("", "agent")` | Returns builder; `Build()` returns `ErrInvalidKey` |
| `TestBuild_FullKey` | `.EntityType("et").EntityID("id").Version("v1").Build()` | `"ns:agent:et:id:v1"` |
| `TestBuild_PartialKey_NoEntityType` | `.EntityID("id").Build()` | `"ns:agent::id:"` or error |
| `TestBuild_ColonInComponent` | `NewKeyBuilder("ns", "agent:bad")` | `Build()` returns `ErrInvalidKey` |
| `TestBuild_ColonInEntityType` | `.EntityType("et:bad")` | `Build()` returns `ErrInvalidKey` |
| `TestBuild_ColonInEntityID` | `.EntityID("id:bad")` | `Build()` returns `ErrInvalidKey` |
| `TestBuild_KeyTooLong` | Components exceeding 1024 bytes | `Build()` returns `ErrInvalidKey` |
| `TestPattern_AllComponentsSet` | `.EntityType("et").EntityID("id").Version("v1").Pattern()` | `"ns:agent:et:id:v1"` |
| `TestPattern_EntityTypeOnly` | `.EntityType("et").Pattern()` | `"ns:agent:et:*"` |
| `TestPattern_NoComponents` | `.Pattern()` | `"ns:agent:*"` |
| `TestPattern_EmptyAgentID` | `NewKeyBuilder("ns", "").Pattern()` | Returns `ErrAgentIDMissing` |
| `TestPattern_EmptyNamespace` | `NewKeyBuilder("", "agent").Pattern()` | Returns `ErrInvalidKey` |
| `TestBuild_ChainingOrder` | Different method call orders | Same result regardless of order |
| `TestBuild_UTF8Validation` | Non-UTF8 bytes in components | Returns `ErrInvalidKey` |

---

### ValkeyBackend Tests (`valkey_backend_test.go`)

Uses `valkey.NewMockClient()` — register expected commands via `mock.Register()`.

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestValkeyBackend_Get_Hit` | Key exists | Returns value, nil error |
| `TestValkeyBackend_Get_Miss` | Key does not exist | Returns nil, `ErrCacheMiss` |
| `TestValkeyBackend_Set_Success` | Valid key + value + TTL | No error |
| `TestValkeyBackend_Set_WithTTL` | Set with 5s TTL | No error, `TTL()` returns ~5s |
| `TestValkeyBackend_Delete_Exists` | Delete existing key | No error |
| `TestValkeyBackend_Delete_NotExists` | Delete non-existent key | No error (idempotent) |
| `TestValkeyBackend_GetMany_AllHits` | All keys exist | Returns map with all entries |
| `TestValkeyBackend_GetMany_PartialHits` | 2 of 3 keys exist | Returns map with 2 entries |
| `TestValkeyBackend_GetMany_Empty` | Empty key list | Returns empty map, no error |
| `TestValkeyBackend_SetMany_Success` | Multiple entries | No error, all retrievable |
| `TestValkeyBackend_SetMany_BatchesOver100` | 250 entries | No error (tests batching logic) |
| `TestValkeyBackend_DeleteMany_Success` | Multiple keys | No error |
| `TestValkeyBackend_DeletePattern_ValidPattern` | `"ns:agent:et:*"` | No error |
| `TestValkeyBackend_DeletePattern_BareStar` | `"*"` | Returns `ErrPatternInvalid` |
| `TestValkeyBackend_DeletePattern_EmptyPattern` | `""` | Returns `ErrPatternInvalid` |
| `TestValkeyBackend_DeleteByTag_TagExists` | Tag set has members | All tagged keys deleted |
| `TestValkeyBackend_DeleteByTag_TagNotExists` | Tag set empty | No error (idempotent) |
| `TestValkeyBackend_Exists_True` | Key exists | Returns true, nil |
| `TestValkeyBackend_Exists_False` | Key missing | Returns false, nil |
| `TestValkeyBackend_TTL_WithExpiry` | Key with 30s TTL | Returns ~30s |
| `TestValkeyBackend_TTL_NoExpiry` | Key with no TTL | Returns -1 |
| `TestValkeyBackend_TTL_KeyMissing` | Key does not exist | Returns -2 |
| `TestValkeyBackend_Close` | Close backend | No error, underlying client closed |
| `TestValkeyBackend_ConnectionError` | Mock client returns connection error | Returns `ErrBackendUnavailable` |

---

### SingleFlight Tests (`singleflight_test.go`)

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestSingleFlight_Do_FirstCaller` | First call for a key | Executes fn, returns `(val, nil, false)` |
| `TestSingleFlight_Do_SecondCaller_Waits` | Concurrent call for same key | Returns `(val, nil, true)` — shared=true |
| `TestSingleFlight_Do_100ConcurrentSameKey` | 100 goroutines, same key | fn called exactly once |
| `TestSingleFlight_Do_ErrorPropagation` | fn returns error | Error propagated to all waiters |
| `TestSingleFlight_Do_DifferentKeys` | Concurrent calls, different keys | Each fn called independently |
| `TestSingleFlight_DoChan_ResultOnChannel` | Async call | Channel receives `SingleFlightResult` |
| `TestSingleFlight_DoChan_ErrorOnChannel` | Async call, fn errors | Channel receives error |
| `TestSingleFlight_Do_NilResult` | fn returns nil | All callers receive nil (valid cache miss scenario) |
| `TestSingleFlight_RaceCondition` | `go test -race` with concurrent Do/DoChan | No data races |

---

### Cache Core Tests (`cache_test.go`)

Uses a mock `CacheBackend` (satisfies `CacheBackend` interface, not Valkey-specific) and mock `CacheObserver`.

#### Core Operations

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestCache_Get_Miss` | Key not in cache | Returns `(nil, nil)` |
| `TestCache_Get_Hit` | Key set then got | Returns value, nil |
| `TestCache_Get_NoAgentID` | Cache created without agentID | Returns `ErrAgentIDMissing` |
| `TestCache_Get_ObserverCalled` | Get with observer | `ObserveGet` called with hit=false/true |
| `TestCache_Get_LatencyRecorded` | Get with observer | `latencyMs > 0` |
| `TestCache_Set_Success` | Valid key + value | No error |
| `TestCache_Set_NilValue` | Set with nil value | Returns error |
| `TestCache_Set_ExceedsMaxSize` | Value > NamespaceConfig.MaxSize | Returns `ErrMaxSizeExceeded` |
| `TestCache_Set_WithTTL` | `Set(ctx, key, val, WithTTL(5m))` | Stored with custom TTL |
| `TestCache_Set_WithTags` | `Set(ctx, key, val, WithTags("t1"))` | Tag index updated |
| `TestCache_Set_WithVersion` | `Set(ctx, key, val, WithVersion("v3"))` | Version stored |
| `TestCache_Set_NoAgentID` | Cache without agentID | Returns `ErrAgentIDMissing` |
| `TestCache_Set_ObserverCalled` | Set with observer | `ObserveSet` called with size and latency |
| `TestCache_Delete_Exists` | Delete existing key | No error |
| `TestCache_Delete_NotExists` | Delete non-existent key | No error (idempotent) |
| `TestCache_Delete_ObserverCalled` | Delete with observer | `ObserveDelete` called with reason="manual" |

#### Cache-Aside Pattern (GetOrFetch)

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestGetOrFetch_Miss_CallsFetch` | Cache miss | fetchFn called, value stored and returned |
| `TestGetOrFetch_Hit_SkipsFetch` | Cache hit | fetchFn NOT called, cached value returned |
| `TestGetOrFetch_FetchError_Propagated` | fetchFn returns error | Error returned, value NOT cached |
| `TestGetOrFetch_StampedeProtection_Coalesced` | 100 concurrent same-key calls | fetchFn called exactly once |
| `TestGetOrFetch_StampedeProtection_ErrorBroadcast` | 100 concurrent, fetchFn errors | Error propagated to all 100 callers |
| `TestGetOrFetch_NoStampedeProtection` | Disabled per namespace | Each miss calls fetchFn independently |
| `TestGetOrFetch_WithOptions` | WithTTL, WithTags options | Stored with options applied |
| `TestGetOrFetch_NilResult` | fetchFn returns nil | Returns nil (valid miss pattern) |

#### Configuration Scoping

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestWithNamespace_ReturnsNewInstance` | `cache.WithNamespace("other")` | New instance, original unchanged |
| `TestWithNamespace_KeysIsolated` | Same logical key, different namespace | Different fully qualified keys |
| `TestWithAgentID_ReturnsNewInstance` | `cache.WithAgentID("other")` | New instance, original unchanged |
| `TestWithAgentID_KeysIsolated` | Same logical key, different agent | Different fully qualified keys |
| `TestWithDefaultTTL_ReturnsNewInstance` | `cache.WithDefaultTTL(30m)` | New instance with new default TTL |
| `TestWithDefaultTags_ReturnsNewInstance` | `cache.WithDefaultTags("t1")` | New instance with default tags |
| `TestScoping_Immutable` | Call With* methods | Original cache instance unchanged |

#### Bulk Operations

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestGetMany_AllHits` | All keys present | Returns map with all entries |
| `TestGetMany_PartialHits` | 2 of 3 keys present | Returns map with 2 entries |
| `TestGetMany_EmptyKeys` | Empty slice | Returns empty map |
| `TestGetMany_ObserverCalledForEachHit` | 3 keys, 2 hits | `ObserveGet` called 3 times |
| `TestSetMany_Success` | Multiple entries | No error |
| `TestSetMany_NilValue` | One value is nil | Returns error |
| `TestSetMany_ExceedsMaxSize` | One value too large | Returns `ErrMaxSizeExceeded` |
| `TestSetMany_WithTags` | Bulk set with tags | Tag index updated for all |
| `TestSetMany_ObserverCalledForEach` | 3 entries | `ObserveSet` called 3 times |
| `TestDeleteMany_Success` | Multiple keys | No error |
| `TestDeleteMany_ObserverCalledForEach` | 3 keys | `ObserveDelete` called 3 times |

#### Pattern & Tag Invalidation

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestDeletePattern_BareStarRejected` | `"*"` | Returns `ErrPatternInvalid` |
| `TestDeletePattern_ValidPattern` | `"ns:agent:et:*"` | Delegates to backend |
| `TestDeletePattern_ObserverCalled` | Valid pattern | `ObserveDelete` with reason="pattern" |
| `TestDeleteByTag_TagExists` | Tag in index | Backend's DeleteByTag called |
| `TestDeleteByTag_EmptyTag` | `""` tag | Returns error |
| `TestDeleteByTag_ObserverCalled` | Tag delete | `ObserveDelete` with reason="tag" |

#### Versioned Invalidation

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestInvalidateByVersion_Match` | Stored version matches expected | Key deleted, `_version:{key}` deleted |
| `TestInvalidateByVersion_Mismatch` | Stored version differs from expected | No-op (already stale) |
| `TestInvalidateByVersion_KeyMissing` | Key not in cache | No-op (already invalidated) |
| `TestInvalidateByVersion_ObserverCalled` | Version match | `ObserveDelete` with reason="version" |

#### Observability

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestNoopObserver_NoPanic` | All operations with noop observer | No panics |
| `TestStats_AfterOperations` | 10 hits, 5 misses | `HitCount=10`, `MissCount=5`, `HitRate≈0.667` |
| `TestStats_EmptyCache` | No operations | `HitCount=0`, `MissCount=0` |
| `TestObserver_AllOperationTypesCovered` | Run Get, Set, Delete, GetOrFetch, Bulk | Every observer method called |

---

### WarmingManager Tests (`warming_test.go`)

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestWarmOnStartup_CallsWarmFuncForEnabled` | 2 namespaces, 1 with OnStartup=true | WarmFunc called once |
| `TestWarmOnStartup_NoneEnabled` | No namespaces have OnStartup=true | WarmFunc never called |
| `TestWarm_Success` | WarmFunc completes within deadline | No error, progress tracked |
| `TestWarm_DeadlineExceeded` | WarmFunc exceeds WarmingConfig.Deadline | Returns `ErrWarmingTimeout` |
| `TestWarm_ContextCancelled` | Context cancelled during warm | Returns context error |
| `TestWarm_FuncError` | WarmFunc returns error | Error propagated |
| `TestWarm_ObserverCalled` | Warming with observer | `ObserveWarming` called with progress |
| `TestTrackProgress_DuringWarm` | Call TrackProgress mid-warming | Progress reflects entries populated |
| `TestTrackProgress_AfterWarm` | Call TrackProgress after completion | `EntriesRemaining=0`, `SuccessCount` populated |
| `TestTrackProgress_UnknownNamespace` | Namespace not configured | Returns zero-value `WarmingProgress` |
| `TestWarm_ConcurrentNamespaces` | 3 namespaces warmed in parallel | All complete, no races |

---

## Integration Tests (`integration_test.go`)

Marked with `//go:build integration` build tag. Requires a running Valkey instance.

```bash
go test -tags=integration ./shared/caching/... -v -race -count=1
```

### Test Cases

| Test Function | Scenario | Expected Result |
|--------------|----------|-----------------|
| `TestIntegration_FullLifecycle` | Create backend → Cache → Set → Get → Delete → Get | Full round-trip, no errors |
| `TestIntegration_GetOrFetch_Coalescing` | 50 concurrent GetOrFetch calls | fetchFn called once, all get value |
| `TestIntegration_TagIndexLifecycle` | Set with tags → DeleteByTag → Get all tagged | All tagged keys deleted |
| `TestIntegration_PatternInvalidation` | Set 10 keys → DeletePattern → Verify all gone | Pattern-matching keys removed |
| `TestIntegration_BulkOperations` | SetMany → GetMany → DeleteMany → GetMany | Correct hit/miss behavior |
| `TestIntegration_StatsAccuracy` | 20 hits, 10 misses | Stats match expected counts |
| `TestIntegration_NamespaceIsolation` | Same logical key, 2 namespaces | Different Valkey keys, no cross-namespace access |
| `TestIntegration_AgentIsolation` | Same logical key, 2 agents | Different Valkey keys, no cross-agent access |
| `TestIntegration_TTLExpiration` | Set with 1s TTL → wait 2s → Get | Returns `(nil, nil)` (expired) |
| `TestIntegration_VersionedInvalidation` | Set with version → InvalidateByVersion match → Get | Key deleted |
| `TestIntegration_VersionedInvalidation_NoOp` | Set with version → InvalidateByVersion mismatch | Key remains |
| `TestIntegration_BackendClose` | Full lifecycle → Close → Try Get | Backend operations fail after close |
| `TestIntegration_LargeValues` | Set with 500KB value | Stored and retrieved correctly |
| `TestIntegration_ManyKeys` | Set 1000 keys → GetMany all | All retrieved |

### Test Fixtures

| Fixture | Purpose | Setup |
|---------|---------|-------|
| `TestValkeyBackend` | Real Valkey connection | `NewValkeyBackend(ValkeyConfig{URL: "redis://localhost:6379"})` |
| `TestCache` | Cache with observer | `NewCache(backend, WithNamespace(...), WithObserver(observer))` |
| `MockObserver` | Captures observer calls | Struct recording all method invocations |

---

## Mock Dependencies

| Dependency | Mock Approach | Rationale |
|------------|---------------|-----------|
| Valkey client | `valkey.NewMockClient()` | No external Valkey needed for unit tests |
| `CacheBackend` | Custom mock struct implementing `CacheBackend` | Test `Cache` in isolation from Valkey |
| `CacheObserver` | Custom mock struct implementing `CacheObserver` | Verify observer calls and arguments |
| `SingleFlight` | Custom mock struct implementing `SingleFlight` | Test `GetOrFetch` without actual coalescing |
| PostgreSQL (version stamps) | Mock `versionStore` interface | Unit tests don't need real DB |
| PostgreSQL (integration) | `testcontainers-go` | Spins up PostgreSQL container for integration tests |
| Context with agentID | `context.WithValue(ctx, agentIDKey, "test-agent")` | Standard context threading |

### Mock Implementation Examples

**Mock CacheBackend:**
```go
type mockBackend struct {
    data    map[string][]byte
    deleted []string
    getFn   func(ctx context.Context, key string) ([]byte, error)
}

func (m *mockBackend) Get(ctx context.Context, key string) ([]byte, error) {
    if m.getFn != nil {
        return m.getFn(ctx, key)
    }
    val, ok := m.data[key]
    if !ok {
        return nil, ErrCacheMiss
    }
    return val, nil
}
// ... implement all CacheBackend methods
```

**Mock CacheObserver:**
```go
type mockObserver struct {
    getCalls    []observeGetCall
    setCalls    []observeSetCall
    deleteCalls []observeDeleteCall
}

func (m *mockObserver) ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64) {
    m.getCalls = append(m.getCalls, observeGetCall{namespace, key, hit, latencyMs})
}
// ... implement all CacheObserver methods
```

**Mock SingleFlight:**
```go
type mockSingleFlight struct {
    callCount int
    result    interface{}
    err       error
    shared    bool
}

func (m *mockSingleFlight) Do(key string, fn func() (interface{}, error)) (interface{}, error, bool) {
    m.callCount++
    return m.result, m.err, m.shared
}
```

---

## Coverage Targets

### Per-Component Coverage

| Component | Line | Branch | Function |
|-----------|------|--------|----------|
| KeyBuilder | 95% | 90% | 100% |
| ValkeyBackend | 90% | 85% | 100% |
| Cache (core) | 95% | 90% | 100% |
| Cache (bulk) | 90% | 85% | 100% |
| Cache (invalidation) | 90% | 85% | 100% |
| SingleFlight | 95% | 90% | 100% |
| WarmingManager | 85% | 80% | 100% |
| CacheObserver | 100% | N/A | 100% |
| VersionStore | 80% | 75% | 100% |

### Package-Level Targets

| Metric | Target | Minimum |
|--------|--------|---------|
| Line Coverage | 90% | 80% |
| Branch Coverage | 85% | 70% |
| Function Coverage | 100% | 95% |

### Coverage Exclusions

| File | Reason |
|------|--------|
| `integration_test.go` | Integration tests require live Valkey — excluded from unit coverage |

---

## Test Execution

### Commands

```bash
# All unit tests
go test ./shared/caching/... -v

# With race detection (mandatory for concurrency-sensitive code)
go test ./shared/caching/... -v -race

# With coverage report
go test ./shared/caching/... -v -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Specific test group
go test ./shared/caching/ -run TestKeyBuilder -v
go test ./shared/caching/ -run TestValkeyBackend -v
go test ./shared/caching/ -run TestSingleFlight -v -race
go test ./shared/caching/ -run TestCache -v
go test ./shared/caching/ -run TestCacheBulk -v
go test ./shared/caching/ -run TestInvalidation -v
go test ./shared/caching/ -run TestWarming -v
go test ./shared/caching/ -run TestObserver -v

# Integration tests (requires running Valkey)
go test -tags=integration ./shared/caching/... -v -race -count=1

# Full verification suite (as in implementation.md Phase 11)
go test ./shared/caching/... -v -race -count=1
go vet ./shared/caching/...

# Verify no forbidden imports
go list -f '{{join .Imports "\n"}}' ./shared/caching/ | grep -E "net/http|nats.io" && echo "FAIL: forbidden import" || echo "PASS"
```

### CI/CD Integration

| Check | Trigger | Required |
|-------|---------|----------|
| Unit tests | Every PR and push to main | Yes — must pass |
| Race detection | Every PR | Yes — must pass |
| Coverage gate | Every PR | Yes — 80% minimum |
| `go vet` | Every PR | Yes — must pass |
| Integration tests | On merge to main | Yes — must pass |
| Forbidden import check | Every PR | Yes — must pass |

### Pre-commit Hook

The existing pre-commit hook runs:
- `go build ./shared/caching/...` (compilation check)
- `go vet ./shared/caching/...` (static analysis)
- `go test ./shared/caching/... -race` (unit tests with race detection)

---

## Security Tests

Derived from `security.md`. Each security control has a corresponding test.

### Agent Isolation

| Test | Description | Expected |
|------|-------------|----------|
| `TestSecurity_AgentIDMissing_KeyBuilder` | `NewKeyBuilder("ns", "").Build()` | `ErrAgentIDMissing` |
| `TestSecurity_AgentIDMissing_CacheGet` | Cache without agentID, call Get | `ErrAgentIDMissing` |
| `TestSecurity_AgentIDMissing_CacheSet` | Cache without agentID, call Set | `ErrAgentIDMissing` |
| `TestSecurity_AgentIDMissing_CacheDelete` | Cache without agentID, call Delete | `ErrAgentIDMissing` |
| `TestSecurity_AgentIsolation_Integration` | Agent A sets key, Agent B tries to get | Agent B gets `(nil, nil)` — different fully qualified keys |
| `TestSecurity_NamespaceIsolation_Integration` | Same logical key, 2 namespaces | Different Valkey keys, no collision |

### Input Validation

| Test | Description | Expected |
|------|-------------|----------|
| `TestSecurity_ColonInComponent` | Key component contains `:` | `ErrInvalidKey` |
| `TestSecurity_EmptyNamespace` | `NewKeyBuilder("", "agent")` | `ErrInvalidKey` |
| `TestSecurity_KeyTooLong` | Key exceeds 1024 bytes | `ErrInvalidKey` |
| `TestSecurity_NilValue` | `Set(ctx, key, nil)` | Returns error |
| `TestSecurity_EmptyTag` | `WithTags("")` | Returns error |
| `TestSecurity_BareStarPattern` | `DeletePattern("*")` | `ErrPatternInvalid` |
| `TestSecurity_PatternMissingAgentPrefix` | Pattern without agentID | `ErrPatternInvalid` |

### Resource Safety

| Test | Description | Expected |
|------|-------------|----------|
| `TestSecurity_MaxSizeExceeded` | Value > `NamespaceConfig.MaxSize` | `ErrMaxSizeExceeded` |
| `TestSecurity_SingleFlight_LockCleanup` | SingleFlight on error path | Lock released, no memory leak |
| `TestSecurity_ConnectionPoolLimit` | Backend pool exhaustion | `ErrBackendUnavailable` |

### Observability Audit Trail

| Test | Description | Expected |
|------|-------------|----------|
| `TestSecurity_Get_ObserverCalled` | Every Get emits `ObserveGet` | Observer method invoked |
| `TestSecurity_Set_ObserverCalled` | Every Set emits `ObserveSet` | Observer method invoked |
| `TestSecurity_Delete_ObserverCalled` | Every Delete emits `ObserveDelete` | Observer method invoked |
| `TestSecurity_UsageEvent_NeverIncludesValue` | Observer metadata check | `sizeBytes` present, value absent |

### Transport Agnosticism

| Test | Description | Expected |
|------|-------------|----------|
| `TestSecurity_NoNetHTTPImport` | `go list` check | `net/http` not in imports |
| `TestSecurity_NoNATSImport` | `go list` check | `nats.io` not in imports |
| `TestSecurity_NoTransportImport` | `go list` check | No transport packages imported |

---

## Test Maintenance

- Review test coverage quarterly — ensure new code has corresponding tests
- Remove tests for deleted functionality
- Update mock behavior when `CacheBackend` or `CacheObserver` interfaces change
- Integration tests require Valkey — ensure CI environment provides it
- Race detection is non-negotiable — all concurrency tests must pass `-race`

---

## Files Created

- `design/units/caching-strategies/testing.md`
