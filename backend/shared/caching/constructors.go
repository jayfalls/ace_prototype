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
// This function should be implemented in cache.go.
func newCache(cfg *cacheConfig) Cache {
	// TODO: Implement cache.go with actual cache logic
	panic("not implemented: newCache")
}
