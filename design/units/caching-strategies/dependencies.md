# Dependencies

## Overview

The `shared/caching` package depends on **Valkey** as the sole cache backend, plus internal ACE shared packages for messaging, telemetry, and database access. Valkey runs in ALL environments including local development via Docker Compose.

## Infrastructure Dependencies

### Required Infrastructure

| Resource | Type | Specification | Purpose |
|----------|------|---------------|---------|
| **Valkey** | Cache server | Valkey 8.1+ | Cache backend (HARD REQUIREMENT — all environments including local dev) |
| PostgreSQL 18 | Database | Existing `ace_db` | Version stamps, warming schedules, cache statistics |
| NATS 2.12+ | Message broker | Existing `ace_broker` | Cache invalidation events via `shared/messaging` |

#### Valkey — Hard Requirement

**Valkey is the only cache backend.** Redis, Memcached, Dragonfly, PostgreSQL-backed caches, and in-memory cache libraries are not supported. Every environment — including local development — includes a Valkey instance via Docker Compose.

| Property | Value |
|----------|-------|
| Type | In-memory data store |
| Provider | Valkey (Linux Foundation) |
| Version | 8.1+ (latest stable) |
| License | BSD 3-clause |
| Docker image | `valkey/valkey:8` |
| Default port | 6379 |
| Configuration | `valkey.conf` with `io-threads` for multi-threaded I/O |

#### Docker Compose Configuration

```yaml
services:
  valkey:
    image: valkey/valkey:8
    ports:
      - "6379:6379"
    volumes:
      - valkey-data:/data
    command: >
      valkey-server
      --save 60 1
      --loglevel notice
      --io-threads 2
      --io-threads-do-reads yes
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

volumes:
  valkey-data:
```

#### Environment Variables

```bash
# redis:// scheme retained for RESP protocol compatibility; Valkey maintains full wire-level compatibility with Redis
export VALKEY_URL="redis://valkey:6379"
export VALKEY_MAX_RETRIES=3
export VALKEY_DIAL_TIMEOUT="5s"
export VALKEY_READ_TIMEOUT="3s"
export VALKEY_WRITE_TIMEOUT="3s"
export VALKEY_POOL_SIZE=100
```

---

## Go Module Dependencies

### Direct Dependencies

| Module | Version | Purpose | License |
|--------|---------|---------|---------|
| `github.com/valkey-io/valkey-go` | `^1.0.73` | Valkey Go client (auto pipelining, client-side caching) | Apache 2.0 |
| `golang.org/x/sync` | `^0.15.0` | `singleflight` package for stampede protection | BSD 3-clause |

### Internal Dependencies

| Package | Purpose | Import Path |
|---------|---------|-------------|
| `shared/messaging` | NATS integration for invalidation events | `ace/shared/messaging` |
| `shared/telemetry` | UsageEvent emission for all cache operations | `ace/shared/telemetry` |

### Test Dependencies

| Module | Version | Purpose |
|--------|---------|---------|
| `github.com/stretchr/testify` | `^1.11.1` | Test assertions |
| `github.com/valkey-io/valkey-go/mock` | (bundled) | Valkey mock for unit tests |

---

## Dependency Details

### 1. Valkey Go Client (`github.com/valkey-io/valkey-go`)

The official Valkey Go client, previously known as rueidis. Selected over the GLIDE Go client (which uses CGO/Rust core) for pure Go compilation and superior performance.

| Property | Value |
|----------|-------|
| Module | `github.com/valkey-io/valkey-go` |
| Version | `^1.0.73` (latest as of March 2026) |
| License | Apache 2.0 |
| Maintainer | Valkey organization (Linux Foundation) |
| Created | April 2024 |
| Releases | 37 |
| Contributors | 130+ |

**Key features used by `shared/caching`**:
- Auto pipelining for non-blocking commands
- Server-assisted client-side caching
- Valkey Cluster support
- Pub/Sub for cross-service invalidation (if needed alongside NATS)
- Built-in test mock (`valkey.NewMockClient`)
- OpenTelemetry integration hooks

**Installation**:
```bash
go get github.com/valkey-io/valkey-go@v1.0.73
```

**Client setup**:
```go
import "github.com/valkey-io/valkey-go"

client, err := valkey.NewClient(valkey.ClientOption{
    InitAddress:    []string{"valkey:6379"},
    SelectDB:       0,
    DialTimeout:    5 * time.Second,
    ReadTimeout:    3 * time.Second,
    WriteTimeout:   3 * time.Second,
    PipelineBuffer: 0, // auto pipelining
})
```

**Why valkey-go over alternatives**:
- Pure Go (no CGO dependency unlike valkey-glide)
- ~14x throughput improvement over go-redis in benchmarks
- Native Valkey client with server-assisted client-side caching
- Most active Valkey Go client (37 releases, weekly updates)
- Compatible with Valkey 7.2, 8.0, 8.1, and 9.0

### 2. `golang.org/x/sync` (singleflight)

The standard Go extended library providing stampede protection via function call deduplication.

| Property | Value |
|----------|-------|
| Module | `golang.org/x/sync` |
| Version | `^0.15.0` |
| License | BSD 3-clause |
| Maintained by | Go team |

**Installation**:
```bash
go get golang.org/x/sync@v0.15.0
```

**Usage** (only `singleflight` subpackage needed):
```go
import "golang.org/x/sync/singleflight"

var group singleflight.Group

// Coalesces concurrent calls with the same key
val, err, shared := group.Do("cache-key", func() (interface{}, error) {
    return expensiveFetch()
})
```

**Why singleflight**:
- Official Go team package — zero external dependency beyond `x/sync`
- Battle-tested across the entire Go ecosystem
- The FSD already specifies the `SingleFlight` interface matching this API
- Simple integration with `GetOrFetch` cache-aside pattern

### 3. `shared/messaging` (Internal)

Existing ACE package providing NATS integration. Used by `shared/caching` for cross-service cache invalidation events.

| Property | Value |
|----------|-------|
| Import path | `ace/shared/messaging` |
| Status | Existing (completed unit) |
| Purpose | Publish and subscribe to cache invalidation events |

**Integration points**:
- `messaging.Publish()` — emit `InvalidationEvent` to NATS subjects
- `messaging.SubscribeWithEnvelope()` — receive invalidation events from other services
- Subject format: `ace.cache.{namespace}.invalidate`
- CorrelationID propagation for invalidation chain tracing

**Note**: `shared/caching` is transport-agnostic — it does not import `shared/messaging` directly. Service-internal adapters wire NATS integration by implementing the invalidation handler interface and calling `messaging.Publish`/`Subscribe`.

### 4. `shared/telemetry` (Internal)

Existing ACE package providing observability primitives. Used for `UsageEvent` emission on all cache operations.

| Property | Value |
|----------|-------|
| Import path | `ace/shared/telemetry` |
| Status | Existing (completed unit) |
| Purpose | Emit UsageEvents for cache hits, misses, writes, invalidations, evictions, warming |

**Integration points**:
- `tel.Usage.Publish()` — emit cache operation UsageEvents
- Operation types: `cache-hit`, `cache-miss`, `cache-write`, `cache-invalidate`, `cache-evict`, `cache-warming`
- agentId included in all events for attribution
- Latency and size metrics in event metadata

**Note**: Similar to messaging, `shared/caching` defines an observer interface. Service adapters wire `shared/telemetry` to emit UsageEvents.

### 5. PostgreSQL (Existing Infrastructure)

Version stamps for versioned invalidation strategy are stored in PostgreSQL. No new database dependencies.

| Property | Value |
|----------|-------|
| Instance | Existing `ace_db` (PostgreSQL 18) |
| Purpose | Store `VersionStamp` records |
| Query layer | SQLC (type-safe generated queries) |
| Migrations | Goose Go functions |

**Tables used**:
- `version_stamps` — key, version, source_hash, updated_at, updated_by
- Indexed on key for fast lookups during versioned invalidation checks

---

## Dependency Compatibility Matrix

| Dependency | Go Version | ACE Go Version | Compatible |
|------------|-----------|----------------|------------|
| `valkey-go ^1.0.73` | 1.21+ | 1.26 | Yes |
| `x/sync ^0.15.0` | 1.22+ | 1.26 | Yes |
| `shared/messaging` | 1.26 | 1.26 | Yes |
| `shared/telemetry` | 1.26 | 1.26 | Yes |
| Valkey server 8.1+ | N/A | N/A | Yes (all environments) |

---

## Version Pinning Strategy

| Dependency | Strategy | Rationale |
|------------|----------|-----------|
| `valkey-go` | Pin to `^1.0.x` | Active development (37 releases). Minor versions add features; patch versions fix bugs. |
| `x/sync` | Pin to `^0.15.0` | Go team maintained. Safe to track latest minor. |
| Valkey server | Pin to `8.x` | Major version for production stability. 8.1+ for I/O threading. |

---

## Dependency Security

| Dependency | Known CVEs (as of March 2026) | Notes |
|------------|-------------------------------|-------|
| `valkey-go` | None | Apache 2.0, Linux Foundation governed |
| `x/sync` | None | Go team maintained, part of official extended library |
| Valkey server | None | BSD 3-clause, Linux Foundation governed |

### Vulnerability Scanning
- [x] Scan dependencies on build (via `go vulncheck` in pre-commit)
- [x] Automated CVE alerts (via Dependabot/Renovate)
- [x] Update policy: Security patches within 48 hours, minor versions monthly

---

## Dependency Update Strategy
- **Security patches**: Within 48 hours of disclosure
- **Minor/patch versions**: Monthly review
- **Major versions**: Evaluate per case — test in staging before production
- **Valkey server**: Follow Valkey stable release cycle (~quarterly)
- **Go version**: Track latest two minor releases
