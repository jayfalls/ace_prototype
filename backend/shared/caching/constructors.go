package caching

// =============================================================================
// Constructor Functions
// =============================================================================

// NewCache creates a new Cache with the given backend and options.
func NewCache(backend CacheBackend, opts ...CacheOption) Cache {
	cfg := applyCacheOptions(opts...)
	cfg.backend = backend
	return newCache(cfg)
}

// NewValkeyBackend creates a new Valkey cache backend.
func NewValkeyBackend(cfg ValkeyConfig) (CacheBackend, error) {
	return newValkeyBackend(cfg)
}

// NewKeyBuilder creates a new KeyBuilder with the given namespace and agentID.
func NewKeyBuilder(namespace, agentID string) *KeyBuilder {
	return &KeyBuilder{
		namespace: namespace,
		agentID:   agentID,
	}
}

// =============================================================================
// Internal Factory Functions (to be implemented by cache.go)
// =============================================================================

// newCache creates a new Cache implementation.
// This function is implemented in cache.go.
func newCache(cfg *cacheConfig) Cache {
	c := &cacheImpl{
		namespace:          cfg.namespace,
		agentID:            cfg.agentID,
		defaultTTL:         cfg.defaultTTL,
		defaultTags:        cfg.defaultTags,
		invalidation:       cfg.invalidation,
		stampedeProtection: cfg.stampedeProtection,
		maxSize:            cfg.maxSize,
		warming:            cfg.warming,
		backend:            cfg.backend,
		observer:           cfg.observer,
		singleFlight:       cfg.singleFlight,
	}

	// Create default SingleFlight if stampede protection is enabled and none provided
	if c.stampedeProtection && c.singleFlight == nil {
		c.singleFlight = NewSingleFlight()
	}

	// Create no-op observer if none provided
	if c.observer == nil {
		c.observer = NewNoopObserver()
	}

	return c
}
