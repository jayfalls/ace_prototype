//go:build !external

// Package cache provides Ristretto-based in-process cache backend.
package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"ace/internal/caching"

	"github.com/dgraph-io/ristretto"
)

// InProcessBackend implements caching.CacheBackend using Ristretto.
type InProcessBackend struct {
	cache    *ristretto.Cache
	tagIndex *sync.Map // map[string]map[string]struct{} (tagKey -> set of keys)
	keyIndex *sync.Map // map[string]map[string]struct{} (key -> set of tags)
	ttlIndex *sync.Map // map[string]time.Time (key -> expiration time)
	allKeys  *sync.Map // map[string]struct{} (all stored keys for pattern matching)
	mu       sync.RWMutex
}

// Compile-time interface check.
var _ caching.CacheBackend = (*InProcessBackend)(nil)

// InitInProcess creates a new in-process cache backend using Ristretto.
func InitInProcess(cfg *Config) (*InProcessBackend, error) {
	maxCost := cfg.MaxCost
	if maxCost == 0 {
		maxCost = 52428800 // 50MB default
	}

	bufferItems := cfg.BufferItems
	if bufferItems == 0 {
		bufferItems = 64
	}

	cache, err := ristretto.NewCache(&ristretto.Config{
		MaxCost:     maxCost,
		BufferItems: int64(bufferItems),
		NumCounters: maxCost / 10,
	})
	if err != nil {
		return nil, fmt.Errorf("cache: create Ristretto cache: %w", err)
	}

	return &InProcessBackend{
		cache:    cache,
		tagIndex: &sync.Map{},
		keyIndex: &sync.Map{},
		ttlIndex: &sync.Map{},
		allKeys:  &sync.Map{},
	}, nil
}

// Get retrieves a value by key. Returns nil on cache miss.
func (b *InProcessBackend) Get(ctx context.Context, key string) ([]byte, error) {
	// Check TTL first
	if b.isExpired(key) {
		b.Delete(ctx, key)
		return nil, nil
	}

	value, ok := b.cache.Get(key)
	if !ok || value == nil {
		return nil, nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil, nil
	}
	return bytes, nil
}

// Set stores a value with a TTL.
func (b *InProcessBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl > 0 {
		expTime := time.Now().Add(ttl)
		b.ttlIndex.Store(key, expTime)
	} else {
		b.ttlIndex.Delete(key)
	}

	b.cache.Set(key, value, int64(len(value)))
	b.allKeys.Store(key, struct{}{})
	return nil
}

// Delete removes a key.
func (b *InProcessBackend) Delete(ctx context.Context, key string) error {
	b.cache.Del(key)
	b.ttlIndex.Delete(key)
	b.allKeys.Delete(key)

	// Clean up key -> tags mapping
	if tags, ok := b.keyIndex.Load(key); ok {
		tagSet := tags.(map[string]struct{})
		for tag := range tagSet {
			if tagMap, ok := b.tagIndex.Load(tag); ok {
				delete(tagMap.(map[string]struct{}), key)
			}
		}
		b.keyIndex.Delete(key)
	}

	return nil
}

// GetMany retrieves multiple keys. Returns only found (hit) entries.
func (b *InProcessBackend) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(keys))
	for _, key := range keys {
		if b.isExpired(key) {
			b.Delete(ctx, key)
			continue
		}

		value, ok := b.cache.Get(key)
		if !ok {
			continue
		}

		if bytes, ok := value.([]byte); ok {
			result[key] = bytes
		}
	}
	return result, nil
}

// SetMany stores multiple entries.
func (b *InProcessBackend) SetMany(ctx context.Context, entries map[string][]byte, ttl time.Duration) error {
	for key, value := range entries {
		if err := b.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMany removes multiple keys.
func (b *InProcessBackend) DeleteMany(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := b.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// DeletePattern removes all keys matching the pattern.
// Pattern supports * wildcard matching.
func (b *InProcessBackend) DeletePattern(ctx context.Context, pattern string) error {
	if pattern == "" || pattern == "*" {
		return caching.ErrPatternInvalid
	}

	// Collect matching keys from our key index
	var toDelete []string
	b.allKeys.Range(func(key, _ interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if matchPattern(k, pattern) {
			toDelete = append(toDelete, k)
		}
		return true
	})

	// Delete matching keys
	for _, key := range toDelete {
		b.Delete(ctx, key)
	}

	return nil
}

// DeleteByTag removes all entries with the given tag.
// The tagKey is the full tag key (e.g., _tags:{namespace}:{agentId}:{tag}).
func (b *InProcessBackend) DeleteByTag(ctx context.Context, tagKey string) error {
	// Get all keys associated with this tag
	tagMap, ok := b.tagIndex.Load(tagKey)
	if !ok {
		return nil // Tag doesn't exist
	}

	// Collect keys to delete
	keys := make([]string, 0, len(tagMap.(map[string]struct{})))
	for key := range tagMap.(map[string]struct{}) {
		keys = append(keys, key)
	}

	// Delete all keys
	for _, key := range keys {
		b.Delete(ctx, key)
	}

	// Delete the tag set itself
	b.tagIndex.Delete(tagKey)

	return nil
}

// SAdd adds members to a set with an optional TTL.
func (b *InProcessBackend) SAdd(ctx context.Context, key string, members []string, ttl time.Duration) error {
	if len(members) == 0 {
		return nil
	}

	// Get or create the tag set
	set, _ := b.tagIndex.LoadOrStore(key, make(map[string]struct{}))
	tagSet := set.(map[string]struct{})

	// Add members to the set
	for _, member := range members {
		// Track this tag in the key's tag set
		b.addKeyTag(member, key)

		// Add to tag set
		tagSet[member] = struct{}{}
	}

	// Update TTL on the tag set if needed
	if ttl > 0 {
		expTime := time.Now().Add(ttl)
		b.ttlIndex.Store(key, expTime)
	}

	return nil
}

// SMembers returns all members of a set.
func (b *InProcessBackend) SMembers(ctx context.Context, key string) ([]string, error) {
	// Check if expired
	if b.isExpired(key) {
		b.Delete(ctx, key)
		return []string{}, nil
	}

	set, ok := b.tagIndex.Load(key)
	if !ok {
		return []string{}, nil
	}

	members := make([]string, 0, len(set.(map[string]struct{})))
	for member := range set.(map[string]struct{}) {
		members = append(members, member)
	}

	return members, nil
}

// SRem removes members from a set.
func (b *InProcessBackend) SRem(ctx context.Context, key string, members []string) error {
	if len(members) == 0 {
		return nil
	}

	set, ok := b.tagIndex.Load(key)
	if !ok {
		return nil
	}

	tagSet := set.(map[string]struct{})

	for _, member := range members {
		delete(tagSet, member)
		// Remove tag tracking from key
		b.removeKeyTag(member, key)
	}

	return nil
}

// Exists returns true if the key exists.
func (b *InProcessBackend) Exists(ctx context.Context, key string) (bool, error) {
	if b.isExpired(key) {
		b.Delete(ctx, key)
		return false, nil
	}

	_, ok := b.cache.Get(key)
	return ok, nil
}

// TTL returns the remaining TTL for a key.
func (b *InProcessBackend) TTL(ctx context.Context, key string) (time.Duration, error) {
	expTime, ok := b.ttlIndex.Load(key)
	if !ok {
		return 0, nil
	}

	remaining := time.Until(expTime.(time.Time))
	if remaining <= 0 {
		b.Delete(ctx, key)
		return 0, nil
	}

	return remaining, nil
}

// Close shuts down the cache.
func (b *InProcessBackend) Close() error {
	b.cache.Close()
	return nil
}

// addKeyTag records that key has a given tag.
func (b *InProcessBackend) addKeyTag(key, tag string) {
	set, _ := b.keyIndex.LoadOrStore(key, make(map[string]struct{}))
	tagSet := set.(map[string]struct{})
	tagSet[tag] = struct{}{}
}

// removeKeyTag removes tag tracking from key.
func (b *InProcessBackend) removeKeyTag(key, tag string) {
	if tags, ok := b.keyIndex.Load(key); ok {
		delete(tags.(map[string]struct{}), tag)
	}
}

// isExpired checks if a key has expired.
func (b *InProcessBackend) isExpired(key string) bool {
	expTime, ok := b.ttlIndex.Load(key)
	if !ok {
		return false
	}
	return time.Now().After(expTime.(time.Time))
}

// matchPattern checks if key matches the given glob pattern.
// Supports * wildcard which matches any sequence of characters.
func matchPattern(key, pattern string) bool {
	if pattern == "" {
		return key == ""
	}

	// Convert glob pattern to regex-like matching
	// * matches any sequence of characters
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return key == pattern
	}

	// Simple prefix/suffix matching for single wildcard
	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return false
		}
		if suffix != "" && !strings.HasSuffix(key, suffix) {
			return false
		}
		return true
	}

	// Multiple wildcards - more complex matching
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			if !strings.HasPrefix(key, part) {
				return false
			}
			pos = len(part)
		} else {
			idx := strings.Index(key[pos:], part)
			if idx == -1 {
				return false
			}
			pos += idx + len(part)
		}
	}
	return true
}
