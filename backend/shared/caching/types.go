package caching

import (
	"context"
	"time"
)

// =============================================================================
// Core Interfaces
// =============================================================================

// Cache is the high-level cache interface that services interact with.
// All methods require agentId in context or set via WithAgentID.
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
}

// CacheBackend is the low-level backend interface for cache operations.
// Implementations must be thread-safe.
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

// CacheObserver is the interface for observing cache operations.
// All cache operations MUST call the appropriate observer method for observability.
type CacheObserver interface {
	ObserveGet(ctx context.Context, key string, hit bool, latency time.Duration)
	ObserveSet(ctx context.Context, key string, size int, latency time.Duration)
	ObserveDelete(ctx context.Context, key string, reason string)
	ObserveGetMany(ctx context.Context, keys []string, hits int, latency time.Duration)
	ObserveSetMany(ctx context.Context, count int, latency time.Duration)
	ObserveDeleteMany(ctx context.Context, count int, latency time.Duration)
	ObserveDeletePattern(ctx context.Context, pattern string, count int, latency time.Duration)
	ObserveDeleteByTag(ctx context.Context, tag string, count int, latency time.Duration)
	ObserveInvalidateByVersion(ctx context.Context, key string, success bool, latency time.Duration)
	ObserveWarming(ctx context.Context, namespace string, progress *WarmingProgress)
}

// SingleFlight is the interface for stampede protection.
// Wraps golang.org/x/sync/singleflight.
type SingleFlight interface {
	Do(key string, fn func() (interface{}, error)) (interface{}, error, bool)
	DoChan(key string, fn func() (interface{}, error)) <-chan SingleFlightResult
}

// WarmingManager is the interface for cache warming orchestration.
type WarmingManager interface {
	Warm(ctx context.Context, namespace string) error
	WarmOnStartup(ctx context.Context) error
	TrackProgress(namespace string) *WarmingProgress
}

// =============================================================================
// Core Types
// =============================================================================

// FetchFunc is the function type for GetOrFetch operations.
// Returns the value to cache, or an error if the fetch fails.
type FetchFunc func(ctx context.Context) ([]byte, error)

// SetOption is a functional option for Set operations.
type SetOption func(*SetOptions)

// SetOptions holds the configuration for a single Set operation.
type SetOptions struct {
	TTL     time.Duration
	Tags    []string
	Version string
}

// CacheEntry represents a cached value with metadata.
type CacheEntry struct {
	Key        string
	Value      []byte
	TTL        time.Duration
	Tags       []string
	Version    string
	InsertedAt time.Time
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	HitRate    float64
	TotalSize  int64
	EntryCount int64
}

// WarmingProgress holds the progress of cache warming.
type WarmingProgress struct {
	Namespace    string
	EntriesDone  int64
	EntriesTotal int64
	StartedAt    time.Time
	CompletedAt  *time.Time
	Err          error
}

// SingleFlightResult is the result of a SingleFlight operation.
type SingleFlightResult struct {
	Value  interface{}
	Err    error
	Shared bool
}

// VersionStamp represents a version stamp for versioned invalidation.
type VersionStamp struct {
	Key        string
	Version    string
	SourceHash string
	UpdatedAt  time.Time
	UpdatedBy  string
}

// InvalidationEvent represents a cache invalidation event.
type InvalidationEvent struct {
	AgentID   string
	Namespace string
	Key       string
	Tag       string
	Pattern   string
	Version   string
	Reason    string
	Timestamp time.Time
}

// =============================================================================
// KeyBuilder
// =============================================================================

// KeyBuilder constructs and validates cache keys.
// Key format: {namespace}:{agentId}:{entityType}:{entityId}:{version}
type KeyBuilder struct {
	namespace  string
	agentID    string
	entityType string
	entityID   string
	version    string
}

// =============================================================================
// Configuration Types
// =============================================================================

// ValkeyConfig holds the configuration for the Valkey backend.
type ValkeyConfig struct {
	// Addr is the Valkey address (host:port). Default: "localhost:6379"
	Addr string

	// Password for authentication. Optional.
	Password string

	// DB is the database number. Default: 0
	DB int

	// MaxRetries is the maximum number of retries. Default: 3
	MaxRetries int

	// DialTimeout is the dial timeout. Default: 5s
	DialTimeout time.Duration

	// ReadTimeout is the read timeout. Default: 3s
	ReadTimeout time.Duration

	// WriteTimeout is the write timeout. Default: 3s
	WriteTimeout time.Duration

	// PoolSize is the connection pool size. Default: 100
	PoolSize int

	// MinIdleConns is the minimum number of idle connections. Default: 10
	MinIdleConns int

	// TLSConfig is optional TLS configuration.
	// TLSConfig *tls.Config
}

// Config holds the configuration for creating a Cache.
type Config struct {
	// Backend is the cache backend. Required.
	Backend CacheBackend

	// Namespace is the default namespace for cache keys. Optional.
	Namespace string

	// AgentID is the default agent ID for cache keys. Optional.
	AgentID string

	// DefaultTTL is the default TTL for cache entries. Default: 1h
	DefaultTTL time.Duration

	// DefaultTags is the default tags for cache entries. Optional.
	DefaultTags []string

	// Invalidation is the invalidation strategy. Default: InvalidationStrategyTTL
	Invalidation InvalidationStrategy

	// StampedeProtection enables single-flight for GetOrFetch. Default: true
	StampedeProtection bool

	// SingleFlight is the single-flight instance. Optional (created internally if nil)
	SingleFlight SingleFlight

	// Observer is the cache observer. Optional (created internally as no-op if nil)
	Observer CacheObserver

	// MaxSize is the maximum size in bytes for a single cache entry. Default: 1MB
	MaxSize int64

	// Warming is the warming configuration. Optional.
	Warming *WarmingConfig
}

// NamespaceConfig holds namespace-specific configuration.
type NamespaceConfig struct {
	// Namespace is the namespace name. Required.
	Namespace string

	// DefaultTTL is the default TTL for this namespace. Default: 1h
	DefaultTTL time.Duration

	// MaxSize is the maximum size in bytes for entries in this namespace. Default: 1MB
	MaxSize int64

	// EvictionPolicy is the eviction policy. Default: EvictionPolicyLRU
	EvictionPolicy EvictionPolicy
}

// WarmingConfig holds the configuration for cache warming.
type WarmingConfig struct {
	// Namespace is the namespace to warm. Required.
	Namespace string

	// WarmFunc is the function to populate the cache. Required.
	WarmFunc WarmFunc

	// OnStartup indicates if warming should occur on service startup. Default: false
	OnStartup bool

	// Deadline is the maximum time allowed for warming. Default: 30s
	Deadline time.Duration

	// Parallel indicates if warming should run in parallel with other namespaces. Default: true
	Parallel bool
}

// WarmFunc is the function type for cache warming.
type WarmFunc func(ctx context.Context, cache Cache) error

// =============================================================================
// Constants
// =============================================================================

// InvalidationStrategy defines the invalidation strategy.
type InvalidationStrategy int

const (
	// InvalidationStrategyTTL uses TTL-based expiration.
	InvalidationStrategyTTL InvalidationStrategy = iota

	// InvalidationStrategyEvent uses event-driven invalidation via NATS.
	InvalidationStrategyEvent

	// InvalidationStrategyVersion uses version-based invalidation.
	InvalidationStrategyVersion

	// InvalidationStrategyHybrid combines TTL as safety net with event-driven.
	InvalidationStrategyHybrid
)

// EvictionPolicy defines the eviction policy.
type EvictionPolicy int

const (
	// EvictionPolicyLRU evicts least recently used keys.
	EvictionPolicyLRU EvictionPolicy = iota

	// EvictionPolicyLFU evicts least frequently used keys.
	EvictionPolicyLFU

	// EvictionPolicyTTL evicts keys with earliest expiration.
	EvictionPolicyTTL

	// EvictionPolicyRandom evicts keys randomly.
	EvictionPolicyRandom
)

// Default values
const (
	DefaultTTL       = time.Hour
	DefaultMaxSize   = 1024 * 1024 // 1MB
	DefaultPoolSize  = 100
	DefaultBatchSize = 100
)
