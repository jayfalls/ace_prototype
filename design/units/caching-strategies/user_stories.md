# User Stories

<!--
Intent: Define user-facing behavior in executable format that can drive tests.
Scope: All user interactions and flows expressed in Gherkin syntax.
Used by: AI agents to generate acceptance tests and ensure feature meets user expectations.
-->

## Feature: Core Library Usage

### Background
```gherkin
Background: Backend service initialization
  Given the shared/caching package is imported by the service
  And a cache backend is configured via environment or config
  And the cache namespace is set for the service
```

### Scenario: Basic cache get when value exists
```gherkin
Scenario: Backend service retrieves a cached value
  Given the cache contains a value "tool-definition-123" in namespace "tool-executor"
  When the tool executor service calls Get with key "tool-definition-123"
  Then the cached value is returned
  And no backend fetch is performed
```

### Scenario: Basic cache set and retrieve
```gherkin
Scenario: Backend service stores and retrieves a value
  Given the cache is empty for namespace "memory-manager"
  When the memory manager service calls Set with key "l4-tree-root" and value "<tree-data>"
  And the service calls Get with key "l4-tree-root"
  Then the stored value "<tree-data>" is returned
```

### Scenario: Cache-aside pattern with GetOrFetch
```gherkin
Scenario: Backend service uses cache-aside to populate on miss
  Given the cache does not contain a value for key "skill-config-review"
  When the cognitive engine calls GetOrFetch with key "skill-config-review" and a fetch function
  Then the fetch function is executed to retrieve the data
  And the result is stored in the cache with the configured TTL
  And the fetched value is returned to the caller
```

### Scenario: Cache-aside pattern on subsequent hit
```gherkin
Scenario: GetOrFetch returns cached data without re-fetching
  Given the cache contains a value for key "skill-config-review" that was previously set by GetOrFetch
  When the cognitive engine calls GetOrFetch with key "skill-config-review" and a fetch function
  Then the cached value is returned directly
  And the fetch function is not executed
```

### Scenario: Delete a cached value
```gherkin
Scenario: Backend service invalidates a specific cache entry
  Given the cache contains a value for key "decision-tree-456"
  When the service calls Delete with key "decision-tree-456"
  Then the value is removed from the cache
  And a subsequent Get for key "decision-tree-456" returns a miss
```

### Scenario: Bulk get multiple keys
```gherkin
Scenario: Backend service retrieves multiple cached values at once
  Given the cache contains values for keys "ctx-1", "ctx-2", and "ctx-3" in namespace "cognitive-engine"
  When the service calls GetMany with keys ["ctx-1", "ctx-2", "ctx-3"]
  Then all three values are returned in a single operation
  And the operation completes faster than three individual Get calls
```

### Scenario: Bulk set multiple keys
```gherkin
Scenario: Backend service stores multiple values in one operation
  Given the cache is empty for namespace "memory-manager"
  When the service calls SetMany with a map of keys to values
  Then all provided key-value pairs are stored atomically
  And each pair is retrievable via individual Get calls
```

### Scenario: Namespace isolation between services
```gherkin
Scenario: Cache entries are isolated by namespace
  Given the cognitive engine stores a value with key "ctx-1" in namespace "cognitive-engine"
  And the memory manager stores a different value with key "ctx-1" in namespace "memory-manager"
  When each service retrieves key "ctx-1" from its own namespace
  Then each service receives its own distinct value
```

### Scenario: Pluggable backend — in-memory for development
```gherkin
Scenario: Service uses in-memory cache backend in development
  Given the service is configured with cache backend type "in-memory"
  When the service performs cache operations (Get, Set, Delete)
  Then all operations execute against the in-memory store
  And no external cache infrastructure is required
```

### Scenario: Pluggable backend — Redis for production
```gherkin
Scenario: Service uses Redis cache backend in production
  Given the service is configured with cache backend type "redis" with connection settings
  When the service performs cache operations (Get, Set, Delete)
  Then all operations execute against the Redis cluster
  And the service code is identical to the in-memory backend configuration
```

### Scenario: Backend switch without code changes
```gherkin
Scenario: Switching cache backend requires only configuration change
  Given a service is running with in-memory cache backend
  When the cache backend configuration is changed to "redis"
  And the service is restarted
  Then the service continues to operate correctly against Redis
  And no service code changes were required
```

---

## Feature: Frontend Caching

### Background
```gherkin
Background: SvelteKit frontend application
  Given the browser-side caching module is initialized
  And the user is authenticated with agentId
```

### Scenario: Cache API responses in the browser
```gherkin
Scenario: Frontend caches API responses to reduce network calls
  Given the frontend has fetched agent status from the API
  And the response was stored in the browser cache
  When the frontend requests agent status again within the TTL window
  Then the cached response is returned without a network request
  And the UI renders immediately
```

### Scenario: Frontend cache invalidation on user action
```gherkin
Scenario: Frontend cache clears when user triggers a state change
  Given the frontend has cached agent conversation history
  When the user deletes a conversation
  Then the conversation cache entry is invalidated
  And a subsequent fetch retrieves fresh data from the API
```

### Scenario: Frontend cache respects TTL
```gherkin
Scenario: Browser cache entries expire after configured TTL
  Given the frontend has cached a tool list response with a 5-minute TTL
  When 5 minutes have elapsed since the cache entry was set
  Then the cached entry is considered stale
  And the next request triggers a fresh API call
```

### Scenario: Offline cache fallback
```gherkin
Scenario: Frontend serves cached data when network is unavailable
  Given the frontend has cached skill configurations
  And the user loses network connectivity
  When the frontend requests skill configurations
  Then the cached data is served from the browser cache
  And the UI remains functional with stale data
```

---

## Feature: Backend Tool Selection

### Background
```gherkin
Background: Evaluating cache backend options
  Given the caching strategies unit is in the research phase
  And candidate backends include Redis, Memcached, and PostgreSQL-backed cache
```

### Scenario: Evaluate Redis as distributed cache backend
```gherkin
Scenario: Research team evaluates Redis for production use
  Given Redis is a candidate distributed cache backend
  When the research team benchmarks Redis against production workloads
  Then the evaluation captures latency, throughput, memory overhead, and operational complexity
  And a recommendation with trade-offs is documented
```

### Scenario: Evaluate in-memory cache libraries
```gherkin
Scenario: Research team evaluates Go in-memory cache libraries
  Given candidate libraries include ristretto, bigcache, and groupcache
  When the research team benchmarks each library
  Then the evaluation captures eviction policies, concurrency safety, memory overhead, and API ergonomics
  And a recommendation for the default in-memory backend is documented
```

### Scenario: Backend selection criteria matrix
```gherkin
Scenario: Backend options are scored against selection criteria
  Given backend candidates are evaluated against criteria: operational complexity, performance, consistency guarantees, and ecosystem maturity
  When the scoring matrix is completed
  Then each backend has a documented score per criterion
  And a primary recommendation and fallback option are identified
```

---

## Feature: Invalidation Strategies

### Background
```gherkin
Background: Cache invalidation setup
  Given the shared/caching package provides invalidation primitives
  And services are configured with namespace-specific invalidation strategies
```

### Scenario: TTL-based expiration
```gherkin
Scenario: Cache entry expires after configured TTL
  Given a cache entry is set with a TTL of 60 seconds
  When 60 seconds have elapsed
  Then a Get for that key returns a miss
  And the entry is either removed or marked stale
```

### Scenario: Sliding TTL extension on access
```gherkin
Scenario: Frequently accessed cache entries have their TTL extended
  Given a cache entry is set with a 60-second sliding TTL
  When the entry is accessed at the 50-second mark
  Then the TTL is reset to 60 seconds from the access time
  And the entry remains in the cache
```

### Scenario: Stale-while-revalidate serves stale data during refresh
```gherkin
Scenario: Stale data is served while background refresh is in progress
  Given a cache entry with stale-while-revalidate enabled has expired
  When a Get request arrives for the expired key
  Then the stale value is returned immediately
  And a background refresh is triggered to update the cache
```

### Scenario: Event-driven invalidation via NATS
```gherkin
Scenario: Cache entries are invalidated when upstream data changes
  Given service A has updated a tool definition
  And service A publishes an invalidation event to NATS
  When services B and C receive the invalidation event
  Then services B and C delete the corresponding cached tool definition
  And the next request for that tool definition fetches fresh data
```

### Scenario: Event-driven invalidation across namespaces
```gherkin
Scenario: A single invalidation event clears related entries across namespaces
  Given a memory update event is published on NATS
  When the cognitive engine receives the event
  Then cached decision trees that depend on that memory entry are invalidated
  And cached alignment evaluations referencing that memory are invalidated
```

### Scenario: Versioned invalidation detects staleness
```gherkin
Scenario: Version stamps detect stale cache entries without explicit invalidation
  Given a cache entry was stored with version stamp "v3"
  And the source data has been updated to version "v4"
  When a service retrieves the cached entry
  Then the version mismatch is detected
  And the stale entry is treated as a miss
  And fresh data is fetched and cached with version "v4"
```

### Scenario: Hybrid invalidation — TTL as safety net with event-driven primary
```gherkin
Scenario: Event-driven invalidation is the primary mechanism with TTL as fallback
  Given a cache entry is configured with event-driven invalidation and a 5-minute TTL
  And the entry has not received an invalidation event for 4 minutes
  When an invalidation event arrives for the entry
  Then the entry is invalidated immediately via the event
  And the TTL safety net is reset for future protection
```

### Scenario: Pattern-based invalidation
```gherkin
Scenario: Multiple cache keys are invalidated by pattern match
  Given the cache contains keys "tool:def:review", "tool:def:edit", and "tool:def:lint"
  When the service calls DeletePattern with pattern "tool:def:*"
  Then all three matching entries are removed from the cache
  And non-matching keys are unaffected
```

### Scenario: Tag-based invalidation
```gherkin
Scenario: Cache entries with a shared tag are invalidated together
  Given cache entries are tagged with "memory-tier-l4"
  When the service calls DeleteByTag with tag "memory-tier-l4"
  Then all entries with that tag are invalidated
  And entries without the tag are unaffected
```

---

## Feature: Observability Integration

### Background
```gherkin
Background: Observability pipeline is active
  Given the shared/telemetry package is initialized
  And the UsageEvent pipeline is accepting events
  And agentId is available in the current context
```

### Scenario: Cache hit emits UsageEvent
```gherkin
Scenario: Every cache hit is tracked as a UsageEvent
  Given the cache contains a value for key "ctx-1" in namespace "cognitive-engine"
  When the service calls Get and receives a hit
  Then a UsageEvent is emitted with OperationType "cache-hit"
  And the event includes agentId, namespace, key pattern, and latency
```

### Scenario: Cache miss emits UsageEvent
```gherkin
Scenario: Every cache miss is tracked as a UsageEvent
  Given the cache does not contain a value for key "ctx-999"
  When the service calls Get and receives a miss
  Then a UsageEvent is emitted with OperationType "cache-miss"
  And the event includes agentId, namespace, key pattern, and latency
```

### Scenario: Cache write emits UsageEvent
```gherkin
Scenario: Every cache write is tracked as a UsageEvent
  When the service calls Set with key "ctx-1" and value "<data>"
  Then a UsageEvent is emitted with OperationType "cache-write"
  And the event includes agentId, namespace, key, TTL, and value size
```

### Scenario: Cache invalidation emits UsageEvent
```gherkin
Scenario: Every cache invalidation is tracked as a UsageEvent
  When the service calls Delete or receives an invalidation event for key "ctx-1"
  Then a UsageEvent is emitted with OperationType "cache-invalidate"
  And the event includes agentId, namespace, key, and invalidation source (manual, event, TTL)
```

### Scenario: Cache eviction emits UsageEvent
```gherkin
Scenario: Every cache eviction is tracked as a UsageEvent
  Given the cache has reached its maximum size
  When a new entry causes an existing entry to be evicted
  Then a UsageEvent is emitted with OperationType "cache-evict"
  And the event includes agentId, namespace, evicted key, and eviction reason (capacity, TTL)
```

### Scenario: Hit rate metrics are aggregated per namespace
```gherkin
Scenario: Cache hit rates are available per namespace for operators
  Given cache operations have been occurring across namespaces "cognitive-engine", "memory-manager", and "tool-executor"
  When an operator views the cache efficiency dashboard
  Then hit rates, miss rates, and operation counts are displayed per namespace
  And the data is derived from aggregated UsageEvents
```

### Scenario: LLM cost savings are tracked per agent
```gherkin
Scenario: Cache hits quantify avoided LLM costs per agent
  Given cache hits on LLM prompt completions carry estimated token costs in UsageEvents
  When an operator views the cost savings report
  Then total estimated LLM cost avoided is displayed per agent per day
  And the data is derived from UsageEvents with OperationType "cache-hit" on LLM-related namespaces
```

### Scenario: Invalidation chain tracing
```gherkin
Scenario: Invalidation events trace which keys were cleared across services
  Given service A publishes an invalidation event for a tool definition
  When services B and C receive and process the invalidation
  Then the UsageEvents from all three services form a linked invalidation chain
  And an operator can trace which keys were invalidated across which services
```

---

## Feature: Stampede Protection

### Background
```gherkin
Background: Cache with stampede protection enabled
  Given the shared/caching package has single-flight coalescing enabled by default
  And a cache key "popular-query" is about to expire
```

### Scenario: Single-flight coalescing prevents thundering herd
```gherkin
Scenario: Only one fetch executes when multiple goroutines request an expired key
  Given the cache entry for key "popular-query" has just expired
  When 100 concurrent goroutines call GetOrFetch for key "popular-query"
  Then exactly 1 fetch function execution occurs
  And all 100 goroutines receive the same fetched result
  And the result is stored in the cache for future requests
```

### Scenario: Stampede protection during fetch failure
```gherkin
Scenario: Fetch failure propagates to all waiting goroutines
  Given the cache entry for key "failing-query" has just expired
  And 50 concurrent goroutines call GetOrFetch for key "failing-query"
  And the fetch function returns an error
  Then exactly 1 fetch function execution occurs
  And all 50 goroutines receive the same error
  And no stale data is served
```

### Scenario: Stampede protection does not block non-concurrent requests
```gherkin
Scenario: Sequential requests after a coalesced fetch use the new cached value
  Given a stampede coalesced fetch has completed for key "popular-query"
  When a new goroutine calls GetOrFetch for key "popular-query"
  Then the newly cached value is returned directly
  And the fetch function is not re-executed
```

### Scenario: Stampede protection is configurable per namespace
```gherkin
Scenario: Stampede protection can be disabled for low-contention namespaces
  Given namespace "low-traffic" is configured with stampede protection disabled
  When multiple goroutines request an expired key in namespace "low-traffic"
  Then each goroutine independently executes the fetch function
  And no coalescing occurs
```

---

## Feature: Cache Warming

### Background
```gherkin
Background: Service with cache warming configured
  Given the service has cache warming enabled for critical namespaces
  And warming schedules and data sources are defined per namespace
```

### Scenario: Cache warming on service startup
```gherkin
Scenario: Critical namespaces are pre-populated when the service starts
  Given the service is starting up
  And namespace "cognitive-engine-context" is configured for startup warming
  When the service initializes the cache
  Then the warming function is executed to pre-populate critical entries
  And warming progress is tracked and emitted as UsageEvents
  And the service is marked ready only after warming completes within the target time
```

### Scenario: Cache warming completes within target time
```gherkin
Scenario: Warming finishes within the configured deadline
  Given namespace "memory-manager-l4" has a warming deadline of 5 seconds
  When the warming function is executed on startup
  Then warming completes within 5 seconds
  And the service startup is not blocked beyond the deadline
```

### Scenario: Cache warming timeout triggers degraded mode
```gherkin
Scenario: Service starts with partial cache if warming exceeds deadline
  Given namespace "tool-executor-definitions" has a warming deadline of 3 seconds
  And the warming function takes 10 seconds to complete
  When the warming deadline is reached
  Then the service starts with partially warmed cache
  And a warning UsageEvent is emitted indicating incomplete warming
  And background warming continues to populate remaining entries
```

### Scenario: NATS-triggered cache warming
```gherkin
Scenario: Warming is triggered by an external event via NATS
  Given namespace "skill-configurations" is configured for NATS-triggered warming
  When a NATS message arrives on the warming trigger subject
  Then the warming function is executed to refresh the namespace
  And warming progress is tracked and emitted as UsageEvents
```

### Scenario: Warming progress is observable
```gherkin
Scenario: Operators can monitor cache warming progress
  Given a warming operation is in progress for namespace "cognitive-engine-context"
  When an operator views the cache warming dashboard
  Then warming progress (entries populated, entries remaining, elapsed time, success/failure count) is displayed
  And the data is derived from UsageEvents emitted during warming
```

---

## Feature: Multi-Agent Isolation

### Background
```gherkin
Background: Multi-agent system with cache isolation
  Given multiple agents are operating concurrently in the system
  And each agent has a unique agentId
  And agentId is threaded through the request context
```

### Scenario: Agent cache entries are isolated by agentId
```gherkin
Scenario: Two agents cannot see each other's cached data
  Given agent "agent-alpha" stores a value with key "decision-tree-1" in namespace "cognitive-engine"
  And agent "agent-beta" stores a different value with key "decision-tree-1" in namespace "cognitive-engine"
  When each agent retrieves key "decision-tree-1"
  Then agent "agent-alpha" receives its own value
  And agent "agent-beta" receives its own distinct value
```

### Scenario: Cache keys include agentId automatically
```gherkin
Scenario: The cache library automatically prepends agentId to cache keys
  Given the current context has agentId "agent-alpha"
  When the service calls Set with logical key "decision-tree-1"
  Then the actual cache key stored includes agentId as a prefix
  And the key format follows "{namespace}:{agentId}:{entityType}:{entityId}:{version}"
```

### Scenario: UsageEvents carry agentId for attribution
```gherkin
Scenario: Cache operations are attributed to the correct agent
  Given agent "agent-alpha" performs a cache hit
  When the UsageEvent is emitted
  Then the event includes agentId "agent-alpha"
  And the event can be correlated with other agent-specific telemetry
```

### Scenario: Namespace-level operations respect agentId scope
```gherkin
Scenario: Bulk operations only affect the current agent's entries
  Given agent "agent-alpha" has cached entries "ctx-1", "ctx-2", "ctx-3"
  And agent "agent-beta" has cached entries "ctx-1", "ctx-4"
  When agent "agent-alpha" calls GetMany for keys ["ctx-1", "ctx-2", "ctx-3"]
  Then only agent "agent-alpha"'s entries are returned
  And agent "agent-beta"'s entry "ctx-1" is not included
```

### Scenario: Event-driven invalidation respects agentId scope
```gherkin
Scenario: Invalidation events only clear entries for the targeted agent
  Given an invalidation event is published for agent "agent-alpha" and key "decision-tree-1"
  When the cognitive engine processes the event
  Then only agent "agent-alpha"'s entry for "decision-tree-1" is invalidated
  And agent "agent-beta"'s entry for "decision-tree-1" is unaffected
```

---

## Feature: Testing Strategy

### Background
```gherkin
Background: Test infrastructure for shared/caching
  Given the testing framework is configured with Go testing and testify
  And mock backends are available for unit tests
  And a NATS test server is available for integration tests
```

### Scenario: Unit tests cover each cache backend implementation
```gherkin
Scenario: In-memory backend passes all unit tests
  Given the in-memory cache backend is implemented
  When the unit test suite is executed against the in-memory backend
  Then all cache operations (Get, Set, Delete, GetOrFetch, bulk operations) pass
  And TTL behavior is verified
  And namespace isolation is verified
```

### Scenario: Redis backend passes all unit tests
```gherkin
Scenario: Redis backend passes all unit tests with a mock or test Redis instance
  Given the Redis cache backend is implemented
  When the unit test suite is executed against the Redis backend
  Then all cache operations pass
  And connection failure handling is verified
  And serialization correctness is verified
```

### Scenario: Integration test for cross-service invalidation
```gherkin
Scenario: Cache invalidation propagates across services via NATS
  Given service A and service B are running with shared/caching
  And both services subscribe to cache invalidation events on NATS
  When service A updates data and publishes an invalidation event
  Then service B receives the event and invalidates its cached copy
  And the end-to-end flow completes within the defined consistency window
```

### Scenario: Load test for stampede scenarios
```gherkin
Scenario: System handles thundering herd without duplicate fetches
  Given 1000 concurrent requests are generated for the same expired cache key
  When the load test is executed
  Then exactly 1 fetch occurs
  And all 1000 requests receive the result within acceptable latency
  And no cache stampede or excessive backend load is observed
```

### Scenario: Consistency test for distributed invalidation
```gherkin
Scenario: Distributed cache invalidation converges within the consistency window
  Given multiple service instances share a Redis cache backend
  When an invalidation event is published for a shared key
  Then all instances invalidate their local caches within the defined consistency window
  And no stale data is served after the window expires
```

### Scenario: Test patterns are documented for future features
```gherkin
Scenario: Caching test guidelines are available for services consuming shared/caching
  Given the testing strategy is finalized
  When a new service unit is created that uses shared/caching
  Then documented test patterns guide the service-specific cache testing
  And patterns cover unit, integration, and load test templates
```

---

## Acceptance Criteria Mapping

| Scenario | Acceptance Criteria | Test Priority |
|----------|---------------------|---------------|
| Backend service retrieves a cached value | Library adoption — 100% of services use shared/caching | Must |
| Backend service stores and retrieves a value | Library adoption — 100% of services use shared/caching | Must |
| Cache-aside pattern with GetOrFetch | Library adoption — core operations supported | Must |
| GetOrFetch returns cached data without re-fetching | Library adoption — cache-aside pattern verified | Must |
| Backend service invalidates a specific cache entry | Library adoption — Delete operation supported | Must |
| Bulk get multiple keys | Library adoption — bulk operations supported | Should |
| Bulk set multiple keys | Library adoption — bulk operations supported | Should |
| Cache entries are isolated by namespace | Library adoption — namespace isolation supported | Must |
| Service uses in-memory cache backend in development | Backend pluggability — zero code changes to switch | Must |
| Service uses Redis cache backend in production | Backend pluggability — zero code changes to switch | Must |
| Switching cache backend requires only configuration change | Backend pluggability — zero code changes to switch | Must |
| Frontend caches API responses to reduce network calls | Library adoption — frontend caching module available | Should |
| Frontend cache clears when user triggers a state change | Invalidation consistency — frontend invalidation supported | Should |
| Browser cache entries expire after configured TTL | Invalidation consistency — TTL expiration works | Should |
| Frontend serves cached data when network is unavailable | Library adoption — offline fallback supported | Could |
| Research team evaluates Redis for production use | Backend pluggability — informed selection | Must |
| Research team evaluates Go in-memory cache libraries | Backend pluggability — informed selection | Must |
| Backend options are scored against selection criteria | Backend pluggability — documented recommendation | Must |
| Cache entry expires after configured TTL | Invalidation consistency — TTL-based invalidation | Must |
| Frequently accessed cache entries have their TTL extended | Invalidation consistency — sliding TTL | Should |
| Stale data is served while background refresh is in progress | Invalidation consistency — stale-while-revalidate | Should |
| Cache entries are invalidated when upstream data changes | Invalidation consistency — event-driven via NATS | Must |
| A single invalidation event clears related entries across namespaces | Invalidation consistency — cross-namespace invalidation | Should |
| Version stamps detect stale cache entries without explicit invalidation | Invalidation consistency — versioned invalidation | Should |
| Event-driven invalidation is the primary mechanism with TTL as fallback | Invalidation consistency — hybrid invalidation | Should |
| Multiple cache keys are invalidated by pattern match | Invalidation consistency — pattern-based invalidation | Should |
| Cache entries with a shared tag are invalidated together | Invalidation consistency — tag-based invalidation | Could |
| Every cache hit is tracked as a UsageEvent | Observability coverage — 100% of operations emit events | Must |
| Every cache miss is tracked as a UsageEvent | Observability coverage — 100% of operations emit events | Must |
| Every cache write is tracked as a UsageEvent | Observability coverage — 100% of operations emit events | Must |
| Every cache invalidation is tracked as a UsageEvent | Observability coverage — 100% of operations emit events | Must |
| Every cache eviction is tracked as a UsageEvent | Observability coverage — 100% of operations emit events | Must |
| Cache hit rates are available per namespace for operators | Observability coverage — product feature data | Should |
| Cache hits quantify avoided LLM costs per agent | Cost savings — measurable and attributed via UsageEvents | Should |
| Invalidation events trace which keys were cleared across services | Observability coverage — invalidation chain tracing | Should |
| Only one fetch executes when multiple goroutines request an expired key | Stampede protection — maximum 1 concurrent duplicate fetch | Must |
| Fetch failure propagates to all waiting goroutines | Stampede protection — error handling under coalescing | Must |
| Sequential requests after a coalesced fetch use the new cached value | Stampede protection — post-coalescence correctness | Must |
| Stampede protection can be disabled for low-contention namespaces | Stampede protection — configurable per namespace | Could |
| Critical namespaces are pre-populated when the service starts | Cache warming — populated within target time | Must |
| Warming finishes within the configured deadline | Cache warming — configurable per namespace, default < 5s | Must |
| Service starts with partial cache if warming exceeds deadline | Cache warming — graceful degradation | Should |
| Warming is triggered by an external event via NATS | Cache warming — optional NATS-triggered events | Could |
| Operators can monitor cache warming progress | Observability coverage — warming progress tracked | Should |
| Two agents cannot see each other's cached data | Library adoption — multi-agent isolation | Must |
| The cache library automatically prepends agentId to cache keys | Library adoption — standardized key conventions | Must |
| Cache operations are attributed to the correct agent | Observability coverage — agentId in UsageEvents | Must |
| Bulk operations only affect the current agent's entries | Library adoption — agent-scoped operations | Must |
| Invalidation events only clear entries for the targeted agent | Invalidation consistency — agent-scoped invalidation | Must |
| In-memory backend passes all unit tests | Testing strategy — unit tests per backend | Must |
| Redis backend passes all unit tests with a mock or test Redis instance | Testing strategy — unit tests per backend | Must |
| Cache invalidation propagates across services via NATS | Testing strategy — integration tests for cross-service invalidation | Must |
| System handles thundering herd without duplicate fetches | Testing strategy — load tests for stampede scenarios | Must |
| Distributed cache invalidation converges within the consistency window | Testing strategy — consistency tests for distributed invalidation | Must |
| Caching test guidelines are available for services consuming shared/caching | Testing strategy — documented patterns for future features | Should |
