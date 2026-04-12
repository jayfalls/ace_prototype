package caching

import (
	"time"
)

// =============================================================================
// Cache Configuration Options
// =============================================================================

// cacheConfig holds the internal configuration for a Cache.
type cacheConfig struct {
	namespace          string
	agentID            string
	defaultTTL         time.Duration
	defaultTags        []string
	invalidation       InvalidationStrategy
	stampedeProtection bool
	maxSize            int64
	warming            *WarmingConfig
	// Internal fields (not settable via options)
	singleFlight SingleFlight
	observer     CacheObserver
	backend      CacheBackend
}

// CacheOption is a functional option for configuring a Cache.
type CacheOption func(*cacheConfig)

// WithNamespace sets the namespace for the cache.
func WithNamespace(namespace string) CacheOption {
	return func(c *cacheConfig) {
		c.namespace = namespace
	}
}

// WithAgentID sets the default agent ID for the cache.
func WithAgentID(agentID string) CacheOption {
	return func(c *cacheConfig) {
		c.agentID = agentID
	}
}

// WithDefaultTTL sets the default TTL for cache entries.
func WithDefaultTTL(ttl time.Duration) CacheOption {
	return func(c *cacheConfig) {
		c.defaultTTL = ttl
	}
}

// WithDefaultTags sets the default tags for cache entries.
func WithDefaultTags(tags ...string) CacheOption {
	return func(c *cacheConfig) {
		c.defaultTags = tags
	}
}

// WithInvalidation sets the invalidation strategy.
func WithInvalidation(strategy InvalidationStrategy) CacheOption {
	return func(c *cacheConfig) {
		c.invalidation = strategy
	}
}

// WithStampedeProtection enables or disables stampede protection.
func WithStampedeProtection(enabled bool) CacheOption {
	return func(c *cacheConfig) {
		c.stampedeProtection = enabled
	}
}

// WithObserver sets the cache observer.
func WithObserver(observer CacheObserver) CacheOption {
	return func(c *cacheConfig) {
		c.observer = observer
	}
}

// WithSingleFlight sets the single-flight instance.
func WithSingleFlight(sf SingleFlight) CacheOption {
	return func(c *cacheConfig) {
		c.singleFlight = sf
	}
}

// WithWarming sets the warming configuration.
func WithWarming(config *WarmingConfig) CacheOption {
	return func(c *cacheConfig) {
		c.warming = config
	}
}

// WithMaxSize sets the maximum size for cache entries.
func WithMaxSize(maxSize int64) CacheOption {
	return func(c *cacheConfig) {
		c.maxSize = maxSize
	}
}

// =============================================================================
// Set Options
// =============================================================================

// WithTTL sets the TTL for a cache entry.
func WithTTL(ttl time.Duration) SetOption {
	return func(o *SetOptions) {
		o.TTL = ttl
	}
}

// WithTags sets the tags for a cache entry.
func WithTags(tags ...string) SetOption {
	return func(o *SetOptions) {
		o.Tags = tags
	}
}

// WithVersion sets the version for a cache entry.
func WithVersion(version string) SetOption {
	return func(o *SetOptions) {
		o.Version = version
	}
}

// =============================================================================
// Default Configuration
// =============================================================================

// defaultCacheConfig returns the default configuration.
func defaultCacheConfig() *cacheConfig {
	return &cacheConfig{
		defaultTTL:         DefaultTTL,
		stampedeProtection: true,
		maxSize:            DefaultMaxSize,
	}
}

// =============================================================================
// Option Helpers
// =============================================================================

// applyCacheOptions applies the given options to the configuration.
func applyCacheOptions(opts ...CacheOption) *cacheConfig {
	cfg := defaultCacheConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// applySetOptions applies the given options to the SetOptions.
func applySetOptions(opts ...SetOption) *SetOptions {
	o := &SetOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
