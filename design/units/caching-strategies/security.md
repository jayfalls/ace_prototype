# Security Considerations — shared/caching

## Security Overview

`shared/caching` is a **transport-agnostic Go library** — it does not handle authentication, authorization, or HTTP-level security. Those concerns are the responsibility of the calling service. The library's security focus is narrower but critical:

1. **Agent isolation** — one agent must never read, write, or invalidate another agent's cached data
2. **Data protection** — cached values must not persist beyond their intended TTL; sensitive data requires explicit handling by the caller
3. **Input validation** — keys, values, patterns, and tags must be validated before reaching Valkey
4. **Resource safety** — cache operations must not exhaust memory or block the Valkey server
5. **Connection security** — Valkey connections must use TLS in production

The library enforces these through mandatory `agentId` in every key, strict key format validation, per-namespace size limits, and `CacheObserver` emission on every operation for audit trail.

---

## Data Protection

### Sensitive Data Classification

The cache stores arbitrary `[]byte` values. The library has **no knowledge of data sensitivity** — callers are responsible for classifying and handling sensitive data appropriately.

| Data Type | Cached? | Risk if Leaked | Caller Responsibility |
|-----------|---------|----------------|-----------------------|
| Decision trees | Yes | Low — internal agent reasoning | Encrypt if containing PII |
| LLM completions | Yes | High — may contain user input | Encrypt at application layer before caching |
| Tool definitions | Yes | Low — public API schemas | None |
| Skill configurations | Yes | Medium — agent capabilities | Encrypt if containing secrets |
| User session data | Yes | High — authentication tokens | **Never cache** — use session store |
| Embeddings | Yes | Low — numerical vectors | None |
| Memory nodes | Yes | Medium — may contain user context | Encrypt if containing PII |

**Principle:** `shared/caching` is a key-value store, not a security boundary. If data requires encryption at rest, encrypt it **before** calling `Set()` and decrypt **after** `Get()`. The library does not perform encryption.

### TTL Enforcement

TTL is the primary mechanism for limiting data exposure in cache.

| Concern | Control |
|---------|---------|
| Data persists after TTL | Valkey handles native TTL eviction — expired keys are removed on access and lazily by Valkey's background expiry |
| Caller sets unreasonably long TTL | `NamespaceConfig.DefaultTTL` provides upper-bound guidance; no hard enforcement (callers can override with `WithTTL`) |
| Data persists after service shutdown | Valkey persists to disk via RDB/AOF; disable persistence for sensitive namespaces via Valkey config if needed |
| TTL bypassed via manual Set | Not possible — all `Set` operations pass through the library which always applies a TTL (namespace default or explicit) |

### Data in Transit

Values travel between the service process and Valkey over the network. See [Connection Security](#connection-security) for TLS requirements.

### Data at Rest

Valkey can persist data to disk (RDB snapshots, AOF log). For environments handling sensitive data:

| Setting | Recommendation | Rationale |
|---------|----------------|-----------|
| `save ""` | Disable RDB snapshots | Prevents unencrypted cache data from reaching disk |
| `appendonly no` | Disable AOF | Same rationale |
| Docker volume | Use `tmpfs` for Valkey data | Data lost on container restart — acceptable for cache |

For local development and staging, persistence is acceptable. For production with sensitive data, disable persistence.

---

## Input Validation

### Key Validation

All keys are validated before being sent to Valkey. Invalid keys return `ErrInvalidKey`.

| Validation Rule | Enforced By | Error |
|-----------------|-------------|-------|
| Key must match `{namespace}:{agentId}:{entityType}:{entityId}:{version}` | `KeyBuilder.Build()` | `ErrInvalidKey` |
| `agentId` must not be empty | `KeyBuilder.Build()` | `ErrAgentIDMissing` |
| `namespace` must not be empty | `KeyBuilder.Build()` | `ErrInvalidKey` |
| Key components must not contain `:` (colon separator collision) | `KeyBuilder.Build()` | `ErrInvalidKey` |
| Key length must not exceed Valkey's maximum (default 512MB, practically limited to ~1KB for performance) | `CacheBackend.Set()` | `ErrInvalidKey` |
| Key must be valid UTF-8 | `KeyBuilder.Build()` | `ErrInvalidKey` |

**Key injection prevention:** The `KeyBuilder` constructs keys from structured components, not raw strings. Direct key strings are not accepted by the `Cache` interface — callers must use `KeyBuilder` or the `WithNamespace`/`WithAgentID` scoping methods. This prevents injecting colons to forge keys that cross namespace or agent boundaries.

### Value Validation

| Validation Rule | Enforced By | Error |
|-----------------|-------------|-------|
| Value must not exceed `NamespaceConfig.MaxSize` | `Cache.Set()` | `ErrMaxSizeExceeded` |
| Value must be serializable `[]byte` | Caller (before `Set`) | N/A — library accepts `[]byte` directly |
| Value must not be `nil` | `Cache.Set()` | Returns error (nil values represent cache miss, not valid entries) |

### Pattern Validation

| Validation Rule | Enforced By | Error |
|-----------------|-------------|-------|
| Glob pattern must be valid (no unclosed brackets, valid wildcards) | `Cache.DeletePattern()` | `ErrPatternInvalid` |
| Pattern must not match all keys (`*` alone is rejected for `DeletePattern`) | `Cache.DeletePattern()` | `ErrPatternInvalid` |
| Pattern must include the caller's `agentId` prefix | `Cache.DeletePattern()` | `ErrPatternInvalid` (pattern string missing prefix); `ErrAgentIDMissing` (agentId absent from context) |

### Tag Validation

| Validation Rule | Enforced By | Error |
|-----------------|-------------|-------|
| Tag must not be empty string | `SetOption WithTags()` | Returns error |
| Tag must be valid UTF-8 | `SetOption WithTags()` | Returns error |
| Tag-based delete is scoped to the caller's agentId | `Cache.DeleteByTag()` | Agent-scoped by key prefix |

---

## Threat Modeling (STRIDE)

### Spoofing

| Threat | Description | Mitigation |
|--------|-------------|------------|
| Agent identity spoofing | Caller provides a different agentId to access another agent's cache | `agentId` is extracted from `context.Context` by the library, not provided by the caller directly. The calling service is responsible for setting `agentId` in context correctly. |
| Namespace spoofing | Caller uses `WithNamespace` to access data in a namespace they shouldn't | Namespace access control is the calling service's responsibility. The library enforces namespace isolation in keys but does not enforce authorization. |

### Tampering

| Threat | Description | Mitigation |
|--------|-------------|------------|
| Cache poisoning | Malicious value stored in cache that is later consumed by the service | Values are opaque `[]byte` — the calling service must validate deserialized data. The library does not inspect or sanitize value contents. |
| Key manipulation | Crafting keys to read/write across agent or namespace boundaries | `KeyBuilder` enforces structured key construction. `Cache` interface methods automatically prepend namespace and agentId from scoping. Direct key manipulation is not possible through the `Cache` interface. |
| Invalidation event tampering | Forged `InvalidationEvent` to delete another agent's cache | `InvalidationEvent.AgentID` must match the context `agentId` on receipt. Service-internal adapters enforce this validation. |

### Repudiation

| Threat | Description | Mitigation |
|--------|-------------|------------|
| Unattributed cache operations | Cache operations without agentId — cannot trace who caused them | `ErrAgentIDMissing` is terminal — no operation proceeds without `agentId`. All operations emit `UsageEvent` via `CacheObserver` with `agentId` included. |
| Unlogged cache modifications | Cache writes or deletes without observability trace | `CacheObserver` methods are called on every operation. If observer is `nil`, a no-op observer is used but a warning is logged at initialization. |

### Information Disclosure

| Threat | Description | Mitigation |
|--------|-------------|------------|
| Cross-agent data leakage | Agent A reads data cached by Agent B | All keys include `agentId` as a structural component. `Get`, `GetMany`, and `DeleteByTag` are inherently scoped. `DeletePattern` validates that the pattern includes the caller's `agentId`. |
| Sensitive data in Valkey persistence | Cache data written to disk via RDB/AOF | Disable Valkey persistence for production (see [Data at Rest](#data-at-rest)). Valkey is an in-memory store — persistence is optional. |
| Error messages leaking key contents | `ErrInvalidKey` includes the key in the error message | Key format is `{namespace}:{agentId}:{entityType}:{entityId}:{version}` — no sensitive data in key components. Error messages include key for debugging but never include values. |
| Observability events exposing values | `UsageEvent` metadata includes key contents | `UsageEvent` metadata includes namespace, key, hit/miss, latency, and size — **never** the cached value. |

### Denial of Service

| Threat | Description | Mitigation |
|--------|-------------|------------|
| Memory exhaustion via unbounded writes | Caller writes entries without size limits | `NamespaceConfig.MaxSize` enforces per-namespace limits. `ErrMaxSizeExceeded` rejects writes that exceed the limit. |
| Pattern invalidation blocking Valkey | `DeletePattern("*")` scans entire keyspace | Pattern must include agentId prefix, limiting scan scope. Batch size is configurable (default 100 keys per batch). |
| Single-flight lock exhaustion | Malicious or buggy caller floods single-flight with unique keys | Single-flight map entries are removed immediately on completion (success or failure). No memory leak — each entry is bounded in lifetime. Namespace-level disable is available. |
| Valkey connection exhaustion | Too many concurrent connections | `valkey-go` manages a connection pool (`PoolSize` default: 100). Connection pool exhaustion returns `ErrBackendUnavailable`. |
| Warming flooding | Repeated NATS warming triggers consuming resources | `WarmingConfig.Deadline` limits warming duration. Warming is non-blocking — service starts in degraded mode if deadline exceeded. |

### Elevation of Privilege

| Threat | Description | Mitigation |
|--------|-------------|------------|
| Bypassing agentId isolation | Calling `CacheBackend` directly with forged keys | `CacheBackend` is exposed for advanced use but keys must be fully qualified. The calling service's code review process is the primary control. |
| Cross-namespace access | Using `WithNamespace` to access restricted namespaces | Namespace access control is the calling service's responsibility. The library provides isolation, not authorization. |
| Injecting arbitrary Valkey commands | Exploiting the `CacheBackend` interface to execute arbitrary Valkey commands | `CacheBackend` interface exposes only cache-specific operations (`Get`, `Set`, `Delete`, etc.). Raw command execution is not available. |

---

## Key Isolation Architecture

### agentId Enforcement

`agentId` is **mandatory** for all cache operations. It is the foundation of multi-agent isolation.

```
cognitive-engine:agent-alpha:decision-tree:tree-456:v3
                 ^^^^^^^^^^^
                 This component prevents cross-agent access
```

**Enforcement points:**

| Layer | Check | Error |
|-------|-------|-------|
| `KeyBuilder.Build()` | agentID must not be empty | `ErrAgentIDMissing` |
| `Cache.Get()` | agentId extracted from context or scoping | `ErrAgentIDMissing` |
| `Cache.Set()` | agentId extracted from context or scoping | `ErrAgentIDMissing` |
| `Cache.Delete()` | agentId extracted from context or scoping | `ErrAgentIDMissing` |
| `Cache.DeletePattern()` | Pattern must include caller's agentId | `ErrPatternInvalid` |
| `Cache.DeleteByTag()` | Delete scoped to keys with caller's agentId prefix | Inherently scoped |
| `Cache.GetMany()` | Results filtered to caller's agentId | Inherently scoped |
| Invalidation events | `InvalidationEvent.AgentID` must match context | Validated by service adapter |

### Namespace Isolation

Namespaces provide a second isolation boundary. Keys always include the namespace prefix, so entries in different namespaces cannot collide even if entity types overlap.

```
cognitive-engine:agent-alpha:config:default:v1   ← namespace A
memory-manager:agent-alpha:config:default:v1     ← namespace B — different key, no collision
```

The `WithNamespace` method returns a new `Cache` instance scoped to the specified namespace. Namespace switching is explicit and intentional.

---

## Connection Security

### TLS for Valkey Connections

Production deployments **must** use TLS for Valkey connections. The `redis://` scheme in `VALKEY_URL` must be replaced with `rediss://` in production.

| Environment | Scheme | TLS Required |
|-------------|--------|-------------|
| Local development | `redis://` | No — Valkey runs on localhost in Docker |
| Staging | `rediss://` | Yes — treat as production-like |
| Production | `rediss://` | **Mandatory** |

**Valkey TLS configuration:**

```bash
# Production environment variable
VALKEY_URL=rediss://valkey:6380
```

**Valkey server TLS setup:**

```
# valkey.conf
tls-port 6380
port 0                    # Disable non-TLS port
tls-cert-file /tls/valkey.crt
tls-key-file /tls/valkey.key
tls-ca-cert-file /tls/ca.crt
tls-auth-clients optional # Set to 'yes' to require client certs
tls-protocols "TLSv1.3"
```

**valkey-go TLS client configuration:**

```go
client, err := valkey.NewClient(valkey.ClientOption{
    InitAddress: []string{"valkey:6380"},
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS13,
        ServerName: "valkey",
    },
})
```

### Connection Authentication

Valkey supports `requirepass` for connection authentication. If enabled, the password must be provided via `valkey-go`'s `ClientOption.Password` field, sourced from a secrets manager or environment variable — never hardcoded.

| Setting | Environment Variable | Purpose |
|---------|---------------------|---------|
| Password | `VALKEY_PASSWORD` | Connection authentication |
| TLS cert | `VALKEY_TLS_CERT` | Client certificate (optional mutual TLS) |
| TLS key | `VALKEY_TLS_KEY` | Client private key |

**Secrets management:**
- Local development: `.env` file (gitignored)
- Production: Kubernetes secrets or Vault — injected as environment variables
- Never in code, config files committed to VCS, or Docker images

### Network Isolation

Valkey should not be exposed to the public internet. In Docker Compose, bind to the internal network only. In Kubernetes, use a ClusterIP service.

```yaml
# docker-compose.yml — local development
services:
  valkey:
    image: valkey/valkey:8
    ports:
      - "127.0.0.1:6379:6379"  # Bind to localhost only, not 0.0.0.0
```

---

## Security Controls Summary

| Control | Type | Implementation |
|---------|------|----------------|
| agentId mandatory | Preventative | `ErrAgentIDMissing` — no operation proceeds without agentId |
| Key format validation | Preventative | `KeyBuilder.Build()` validates structure; `ErrInvalidKey` on failure |
| Namespace isolation | Preventative | Namespace is a structural component of every key |
| Per-namespace size limits | Preventative | `NamespaceConfig.MaxSize` — `ErrMaxSizeExceeded` on violation |
| TTL enforcement | Preventative | Valkey-native TTL — expired keys removed on access and lazily |
| Pattern scope restriction | Preventative | `DeletePattern` requires agentId prefix in pattern |
| Tag-based delete scoping | Preventative | `DeleteByTag` scoped to agentId prefix |
| Operation logging | Detective | `CacheObserver` emits `UsageEvent` on every operation |
| Backend unavailable handling | Preventative | `ErrBackendUnavailable` — service degrades gracefully |
| Input sanitization | Preventative | Key components cannot contain separator characters |
| Connection pool limits | Preventative | `ValkeyConfig.PoolSize` caps concurrent connections |
| Single-flight lock cleanup | Preventative | Locks released on completion (success or failure) |

---

## Valkey-Specific Security

### Valkey ACL (Access Control Lists)

Valkey 8+ supports ACLs for fine-grained command restriction. For production, create a dedicated cache user with minimal permissions:

```
# valkey.conf — ACL setup
user cache-service on >${VALKEY_PASSWORD} ~* +@read +@write +@connection -@dangerous -@admin -CONFIG -DEBUG -EVAL -SCRIPT
user default off
```

**Allowed commands:** `GET`, `SET`, `DEL`, `EXISTS`, `TTL`, `EXPIRE`, `SCAN`, `MGET`, `MSET`, `SADD`, `SMEMBERS`, `SREM`, `PING`

**Blocked commands:** `CONFIG`, `DEBUG`, `EVAL`, `SCRIPT`, `FLUSHALL`, `FLUSHDB`, `SHUTDOWN`, `SAVE`, `BGSAVE`, `CLUSTER`, `REPLICAOF`

### Valkey Memory Policy

Configure Valkey's `maxmemory-policy` to prevent uncontrolled memory growth:

```
# valkey.conf
maxmemory 2gb
maxmemory-policy allkeys-lru  # Evict least-recently-used keys when memory is full
```

The library's per-namespace `MaxSize` provides additional enforcement at the application level.

### Dangerous Commands

The following Valkey commands should be disabled or ACL-restricted in production:

| Command | Risk | Mitigation |
|---------|------|------------|
| `FLUSHALL` | Deletes all keys across all databases | ACL deny |
| `FLUSHDB` | Deletes all keys in current database | ACL deny |
| `CONFIG SET` | Modifies server configuration at runtime | ACL deny |
| `DEBUG SLEEP` | Blocks server for specified duration | ACL deny |
| `EVAL` / `EVALSHA` | Arbitrary Lua script execution | ACL deny |
| `SHUTDOWN` | Stops the Valkey server | ACL deny |

---

## Security Testing

- [ ] Unit tests for `KeyBuilder` rejecting keys with empty agentId (`ErrAgentIDMissing`)
- [ ] Unit tests for `KeyBuilder` rejecting keys with empty namespace (`ErrInvalidKey`)
- [ ] Unit tests for `KeyBuilder` rejecting keys with separator characters in components (`ErrInvalidKey`)
- [ ] Unit tests for `DeletePattern` rejecting patterns without agentId prefix
- [ ] Unit tests for `DeletePattern` rejecting bare `*` pattern
- [ ] Unit tests for `Set` rejecting values exceeding `NamespaceConfig.MaxSize` (`ErrMaxSizeExceeded`)
- [ ] Integration tests verifying agent isolation — Agent A cannot read Agent B's cached data
- [ ] Integration tests verifying namespace isolation — same logical key in different namespaces resolves to different Valkey keys
- [ ] Integration tests for Valkey TLS connections (test and production configs)
- [ ] Load tests for memory exhaustion scenarios — writes rejected when namespace limit reached
- [ ] Load tests for single-flight lock cleanup under failure conditions
- [ ] Dependency vulnerability scanning via `go vulncheck` (pre-commit hook)

---

## Incident Response

### Cache Data Breach

If cached data is suspected to be compromised:

1. **Immediate:** Flush the affected namespace in Valkey — `redis-cli -n <db> --scan --pattern "{namespace}:*" | xargs redis-cli DEL`
2. **Short-term:** Rotate Valkey password if connection credentials may be compromised
3. **Investigation:** Review `CacheObserver` `UsageEvent` logs for anomalous access patterns (unexpected agentId, unusual key patterns)
4. **Remediation:** If sensitive data was cached without encryption, assess data classification and notify per data handling policy

### agentId Isolation Breach

If one agent is found to have accessed another agent's cached data:

1. **Immediate:** Flush all affected namespaces
2. **Root cause:** Determine whether the breach was in the library (key validation failure) or the calling service (incorrect agentId in context)
3. **Fix:** If library-level, patch key validation. If service-level, fix context propagation.
4. **Verification:** Add integration test covering the specific cross-agent access pattern

### Valkey Compromise

If the Valkey server itself is compromised:

1. **Immediate:** Shut down Valkey, rotate all credentials
2. **Assumption:** All cached data is compromised — treat as data breach
3. **Recovery:** Redeploy Valkey with fresh TLS certificates and ACL configuration
4. **Prevention:** Review network isolation, ACL configuration, and TLS settings

---
