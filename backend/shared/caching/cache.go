package caching

import (
	"context"
	"strings"
	"time"
)

// =============================================================================
// Cache Implementation
// =============================================================================

// cacheImpl is the internal implementation of Cache.
type cacheImpl struct {
	// From cacheConfig
	namespace          string
	agentID            string
	defaultTTL         time.Duration
	defaultTags        []string
	invalidation       InvalidationStrategy
	stampedeProtection bool
	maxSize            int64
	warming            *WarmingConfig
	// Internal fields
	singleFlight SingleFlight
	observer     CacheObserver
	backend      CacheBackend
}

// noOpObserver implements CacheObserver with no-op methods.
type noOpObserver struct{}

func (n *noOpObserver) ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64) {
}
func (n *noOpObserver) ObserveSet(ctx context.Context, namespace, key string, sizeBytes int64, latencyMs float64) {
}
func (n *noOpObserver) ObserveDelete(ctx context.Context, namespace, key, reason string)   {}
func (n *noOpObserver) ObserveEviction(ctx context.Context, namespace, key, reason string) {}
func (n *noOpObserver) ObserveWarming(ctx context.Context, namespace string, progress WarmingProgress) {
}

// resolveKey resolves a logical key to a fully qualified key.
// If logicalKey already contains colons, parse as entityType:entityID and rebuild with namespace/agentID.
// Returns ErrAgentIDMissing if agentID is not set.
func (c *cacheImpl) resolveKey(logicalKey string) (string, error) {
	// If empty logical key, return error
	if logicalKey == "" {
		return "", ErrInvalidKey
	}

	// Validate that agentID is provided
	if c.agentID == "" && !strings.ContainsRune(logicalKey, ':') {
		return "", ErrAgentIDMissing
	}

	// If logicalKey contains colons, parse as entityType:entityID
	// and build full key with namespace and agentID
	if strings.ContainsRune(logicalKey, ':') {
		parts := strings.Split(logicalKey, ":")
		// Should have at least entityType:entityID
		if len(parts) < 2 {
			return "", ErrInvalidKey
		}

		// For full key validation, we need namespace and agentID
		// If the key already has namespace:agent prefix, validate
		if len(parts) >= 2 && c.agentID != "" && parts[1] == c.agentID {
			// Already has correct namespace and agentID format
			return logicalKey, nil
		}

		// Parse as entityType:entityID and rebuild with namespace:agentID
		entityType := parts[0]
		entityID := parts[1]
		kb := NewKeyBuilder(c.namespace, c.agentID).EntityType(entityType).EntityID(entityID)
		fullKey, err := kb.Build()
		if err != nil {
			return "", err
		}
		return fullKey, nil
	}

	// If logicalKey has no colons, it can be used as entityType
	// Build a full key using KeyBuilder: namespace:agentID:logicalKey::(empty entityID and version)
	kb := NewKeyBuilder(c.namespace, c.agentID).EntityType(logicalKey)
	fullKey, err := kb.Build()
	if err != nil {
		return "", err
	}
	return fullKey, nil
}

// Get retrieves a value from the cache.
// Returns (nil, nil) on cache miss.
// Returns ErrAgentIDMissing if agentID is not set.
func (c *cacheImpl) Get(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()

	// Resolve the key
	resolvedKey, err := c.resolveKey(key)
	if err != nil {
		return nil, err
	}

	// Call backend Get
	value, err := c.backend.Get(ctx, resolvedKey)
	if err != nil {
		return nil, err
	}

	// Calculate latency
	latencyMs := float64(time.Since(start).Nanoseconds()) / 1e6
	hit := value != nil

	// Call observer
	c.observer.ObserveGet(ctx, c.namespace, resolvedKey, hit, latencyMs)

	// Return (nil, nil) on miss - do NOT return ErrCacheMiss
	return value, nil
}

// Set stores a value in the cache.
// Returns ErrMaxSizeExceeded if value exceeds maxSize.
// Returns error if value is nil.
func (c *cacheImpl) Set(ctx context.Context, key string, value []byte, opts ...SetOption) error {
	// Validate value not nil
	if value == nil {
		return ErrSerializationFailed
	}

	// Check maxSize
	if int64(len(value)) > c.maxSize {
		return MaxSizeExceededError(int64(len(value)), c.maxSize)
	}

	start := time.Now()

	// Resolve the key
	resolvedKey, err := c.resolveKey(key)
	if err != nil {
		return err
	}

	// Apply SetOptions
	setOpts := applySetOptions(opts...)

	// Apply defaults
	ttl := setOpts.TTL
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	tags := setOpts.Tags
	if len(tags) == 0 {
		tags = c.defaultTags
	}

	// Call backend Set
	err = c.backend.Set(ctx, resolvedKey, value, ttl)
	if err != nil {
		return err
	}

	// If tags provided, update tag index
	if len(tags) > 0 {
		// Store key in tag sets (_tags:{tag})
		for _, tag := range tags {
			tagKey := "_tags:" + tag
			_ = c.backend.Set(ctx, tagKey+":"+resolvedKey, []byte("1"), ttl)
		}
	}

	// Calculate latency
	latencyMs := float64(time.Since(start).Nanoseconds()) / 1e6

	// Call observer
	c.observer.ObserveSet(ctx, c.namespace, resolvedKey, int64(len(value)), latencyMs)

	return nil
}

// Delete removes a value from the cache.
func (c *cacheImpl) Delete(ctx context.Context, key string) error {
	// Resolve the key
	resolvedKey, err := c.resolveKey(key)
	if err != nil {
		return err
	}

	// Call backend Delete
	err = c.backend.Delete(ctx, resolvedKey)
	if err != nil {
		return err
	}

	// Call observer with manual reason
	c.observer.ObserveDelete(ctx, c.namespace, resolvedKey, "manual")

	return nil
}

// GetOrFetch retrieves a value from the cache or fetches it using the provided function.
// If stampede protection is enabled, concurrent requests for the same key will be coalesced.
func (c *cacheImpl) GetOrFetch(ctx context.Context, key string, fetchFn FetchFunc, opts ...SetOption) ([]byte, error) {
	start := time.Now()

	// Resolve the key
	resolvedKey, err := c.resolveKey(key)
	if err != nil {
		return nil, err
	}

	// If stampede protection enabled, wrap in SingleFlight
	if c.stampedeProtection {
		return c.getOrFetchWithStampede(ctx, resolvedKey, fetchFn, opts, start)
	}

	return c.getOrFetchWithoutStampede(ctx, resolvedKey, fetchFn, opts, start)
}

// getOrFetchWithStampede handles GetOrFetch with stampede protection.
func (c *cacheImpl) getOrFetchWithStampede(ctx context.Context, resolvedKey string, fetchFn FetchFunc, opts []SetOption, start time.Time) ([]byte, error) {
	result, err, _ := c.singleFlight.Do(resolvedKey, func() (interface{}, error) {
		return c.doGetOrFetch(ctx, resolvedKey, fetchFn, opts, start)
	})
	if err != nil {
		return nil, err
	}
	return result.([]byte), nil
}

// getOrFetchWithoutStampede handles GetOrFetch without stampede protection.
func (c *cacheImpl) getOrFetchWithoutStampede(ctx context.Context, resolvedKey string, fetchFn FetchFunc, opts []SetOption, start time.Time) ([]byte, error) {
	return c.doGetOrFetch(ctx, resolvedKey, fetchFn, opts, start)
}

// doGetOrFetch performs the actual GetOrFetch logic (both with and without stampede protection).
func (c *cacheImpl) doGetOrFetch(ctx context.Context, resolvedKey string, fetchFn FetchFunc, opts []SetOption, start time.Time) ([]byte, error) {
	// Try to get from cache first
	value, err := c.backend.Get(ctx, resolvedKey)
	if err != nil {
		return nil, err
	}

	// If hit, observe and return
	if value != nil {
		latencyMs := float64(time.Since(start).Nanoseconds()) / 1e6
		c.observer.ObserveGet(ctx, c.namespace, resolvedKey, true, latencyMs)
		return value, nil
	}

	// Cache miss - call fetch function
	fetchValue, err := fetchFn(ctx)
	if err != nil {
		// On fetch error, wrap with FetchFailedError and return without caching
		return nil, FetchFailedError(err)
	}

	// Apply SetOptions
	setOpts := applySetOptions(opts...)
	ttl := setOpts.TTL
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	// Store fetched value in cache
	err = c.backend.Set(ctx, resolvedKey, fetchValue, ttl)
	if err != nil {
		return nil, err
	}

	// Observe the miss/write
	latencyMs := float64(time.Since(start).Nanoseconds()) / 1e6
	c.observer.ObserveGet(ctx, c.namespace, resolvedKey, false, latencyMs)

	return fetchValue, nil
}

// WithNamespace returns a new Cache with the given namespace.
func (c *cacheImpl) WithNamespace(namespace string) Cache {
	newCache := &cacheImpl{
		namespace:          namespace,
		agentID:            c.agentID,
		defaultTTL:         c.defaultTTL,
		defaultTags:        c.defaultTags,
		invalidation:       c.invalidation,
		stampedeProtection: c.stampedeProtection,
		maxSize:            c.maxSize,
		warming:            c.warming,
		singleFlight:       c.singleFlight,
		observer:           c.observer,
		backend:            c.backend,
	}
	return newCache
}

// WithAgentID returns a new Cache with the given agentID.
func (c *cacheImpl) WithAgentID(agentID string) Cache {
	newCache := &cacheImpl{
		namespace:          c.namespace,
		agentID:            agentID,
		defaultTTL:         c.defaultTTL,
		defaultTags:        c.defaultTags,
		invalidation:       c.invalidation,
		stampedeProtection: c.stampedeProtection,
		maxSize:            c.maxSize,
		warming:            c.warming,
		singleFlight:       c.singleFlight,
		observer:           c.observer,
		backend:            c.backend,
	}
	return newCache
}

// WithDefaultTTL returns a new Cache with the given defaultTTL.
func (c *cacheImpl) WithDefaultTTL(ttl time.Duration) Cache {
	newCache := &cacheImpl{
		namespace:          c.namespace,
		agentID:            c.agentID,
		defaultTTL:         ttl,
		defaultTags:        c.defaultTags,
		invalidation:       c.invalidation,
		stampedeProtection: c.stampedeProtection,
		maxSize:            c.maxSize,
		warming:            c.warming,
		singleFlight:       c.singleFlight,
		observer:           c.observer,
		backend:            c.backend,
	}
	return newCache
}

// WithDefaultTags returns a new Cache with the given defaultTags.
func (c *cacheImpl) WithDefaultTags(tags ...string) Cache {
	newCache := &cacheImpl{
		namespace:          c.namespace,
		agentID:            c.agentID,
		defaultTTL:         c.defaultTTL,
		defaultTags:        tags,
		invalidation:       c.invalidation,
		stampedeProtection: c.stampedeProtection,
		maxSize:            c.maxSize,
		warming:            c.warming,
		singleFlight:       c.singleFlight,
		observer:           c.observer,
		backend:            c.backend,
	}
	return newCache
}

// GetMany implements the GetMany method (not required for this phase).
func (c *cacheImpl) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	return nil, nil
}

// SetMany implements the SetMany method (not required for this phase).
func (c *cacheImpl) SetMany(ctx context.Context, entries map[string][]byte, opts ...SetOption) error {
	return nil
}

// DeleteMany implements the DeleteMany method (not required for this phase).
func (c *cacheImpl) DeleteMany(ctx context.Context, keys []string) error {
	return nil
}

// DeletePattern implements the DeletePattern method (not required for this phase).
func (c *cacheImpl) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}

// DeleteByTag implements the DeleteByTag method (not required for this phase).
func (c *cacheImpl) DeleteByTag(ctx context.Context, tag string) error {
	return nil
}

// InvalidateByVersion implements the InvalidateByVersion method (not required for this phase).
func (c *cacheImpl) InvalidateByVersion(ctx context.Context, key string, expectedVersion string) error {
	return nil
}

// Stats implements the Stats method (not required for this phase).
func (c *cacheImpl) Stats(ctx context.Context) (*CacheStats, error) {
	return nil, nil
}
