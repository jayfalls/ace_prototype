package caching

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock CacheBackend
// =============================================================================

// MockCacheBackend implements CacheBackend for testing.
type MockCacheBackend struct {
	mu       sync.Mutex
	store    map[string][]byte
	expireAt map[string]time.Time
}

func NewMockCacheBackend() *MockCacheBackend {
	return &MockCacheBackend{
		store:    make(map[string][]byte),
		expireAt: make(map[string]time.Time),
	}
}

func (m *MockCacheBackend) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check expiration
	if exp, ok := m.expireAt[key]; ok && time.Now().After(exp) {
		delete(m.store, key)
		delete(m.expireAt, key)
		return nil, nil
	}

	if v, ok := m.store[key]; ok {
		return v, nil
	}
	return nil, nil
}

func (m *MockCacheBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store[key] = value
	if ttl > 0 {
		m.expireAt[key] = time.Now().Add(ttl)
	}
	return nil
}

func (m *MockCacheBackend) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.store, key)
	delete(m.expireAt, key)
	return nil
}

func (m *MockCacheBackend) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string][]byte)
	for _, key := range keys {
		if v, ok := m.store[key]; ok {
			result[key] = v
		}
	}
	return result, nil
}

func (m *MockCacheBackend) SetMany(ctx context.Context, entries map[string][]byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, value := range entries {
		m.store[key] = value
		if ttl > 0 {
			m.expireAt[key] = time.Now().Add(ttl)
		}
	}
	return nil
}

func (m *MockCacheBackend) DeleteMany(ctx context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.store, key)
		delete(m.expireAt, key)
	}
	return nil
}

func (m *MockCacheBackend) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}

func (m *MockCacheBackend) DeleteByTag(ctx context.Context, tag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find all tag set keys matching the tag (format: _tags:{...}:{tag})
	tagSuffix := ":" + tag
	keysToDelete := make([]string, 0)
	for key := range m.store {
		if strings.HasSuffix(key, tagSuffix) && strings.HasPrefix(key, "_tags:") {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Collect all members to delete their _keytags entries
	allMembers := make(map[string]bool)
	for _, tagKey := range keysToDelete {
		if v, ok := m.store[tagKey]; ok {
			for _, member := range bytesToStrings(v) {
				allMembers[member] = true
			}
		}
		delete(m.store, tagKey)
		delete(m.expireAt, tagKey)
	}

	// Clean up _keytags for each member
	for member := range allMembers {
		keyTagsKey := "_keytags:" + member
		delete(m.store, keyTagsKey)
		delete(m.expireAt, keyTagsKey)
	}

	return nil
}

func (m *MockCacheBackend) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := m.store[key]; ok && v != nil {
		return true, nil
	}
	return false, nil
}

func (m *MockCacheBackend) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if exp, ok := m.expireAt[key]; ok {
		remaining := time.Until(exp)
		if remaining > 0 {
			return remaining, nil
		}
	}
	return 0, nil
}

func (m *MockCacheBackend) Close() error {
	return nil
}

func (m *MockCacheBackend) SAdd(ctx context.Context, key string, members []string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Retrieve existing set or create new one
	var existing []string
	if v, ok := m.store[key]; ok {
		existing = bytesToStrings(v)
	}
	seen := make(map[string]bool, len(existing))
	for _, e := range existing {
		seen[e] = true
	}
	for _, member := range members {
		if !seen[member] {
			existing = append(existing, member)
			seen[member] = true
		}
	}
	m.store[key] = stringsToBytes(existing)
	if ttl > 0 {
		m.expireAt[key] = time.Now().Add(ttl)
	}
	return nil
}

func (m *MockCacheBackend) SMembers(ctx context.Context, key string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := m.store[key]; ok {
		return bytesToStrings(v), nil
	}
	return []string{}, nil
}

func (m *MockCacheBackend) SRem(ctx context.Context, key string, members []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := m.store[key]; ok {
		existing := bytesToStrings(v)
		toRemove := make(map[string]bool, len(members))
		for _, member := range members {
			toRemove[member] = true
		}
		remaining := make([]string, 0, len(existing))
		for _, e := range existing {
			if !toRemove[e] {
				remaining = append(remaining, e)
			}
		}
		if len(remaining) == 0 {
			delete(m.store, key)
		} else {
			m.store[key] = stringsToBytes(remaining)
		}
	}
	return nil
}

// helpers to serialize []string ↔ []byte for mock store
func stringsToBytes(s []string) []byte {
	if len(s) == 0 {
		return []byte{}
	}
	out := make([]byte, 0, len(s)*32)
	for i, v := range s {
		if i > 0 {
			out = append(out, '\x00')
		}
		out = append(out, []byte(v)...)
	}
	return out
}

func bytesToStrings(b []byte) []string {
	if len(b) == 0 {
		return []string{}
	}
	parts := strings.Split(string(b), "\x00")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func bytesToSlice(b []byte) []string {
	if len(b) == 0 {
		return []string{}
	}
	return strings.Split(string(b), "\x00")
}

// =============================================================================
// Test Cases
// =============================================================================

// TestCache_Get_Miss tests that Get returns (nil, nil) on cache miss.
func TestCache_Get_Miss(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Act
	value, err := cache.Get(context.Background(), "some-key")

	// Assert
	require.NoError(t, err)
	assert.Nil(t, value)
}

// TestCache_Get_Hit tests that Get returns value after Set.
func TestCache_Get_Hit(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set a value first
	err := cache.Set(context.Background(), "user:123", []byte(`{"name":"test"}`))
	require.NoError(t, err)

	// Act
	value, err := cache.Get(context.Background(), "user:123")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"name":"test"}`), value)
}

// TestCache_Delete_Exists then TestCache_Get_Miss tests that Delete removes the value.
func TestCache_Delete_Exists(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set a value first
	err := cache.Set(context.Background(), "user:123", []byte(`{"name":"test"}`))
	require.NoError(t, err)

	// Verify the value exists
	value, err := cache.Get(context.Background(), "user:123")
	require.NoError(t, err)
	require.NotNil(t, value)

	// Act - Delete the value
	err = cache.Delete(context.Background(), "user:123")
	require.NoError(t, err)

	// Assert - Get should return nil (cache miss)
	value, err = cache.Get(context.Background(), "user:123")
	require.NoError(t, err)
	assert.Nil(t, value)
}

// TestGetOrFetch_Miss_CallsFetch tests that GetOrFetch calls fetchFn on cache miss.
func TestGetOrFetch_Miss_CallsFetch(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	fetchCalled := atomic.Bool{}
	fetchFn := func(ctx context.Context) ([]byte, error) {
		fetchCalled.Store(true)
		return []byte(`{"name":"fetched"}`), nil
	}

	// Act
	value, err := cache.GetOrFetch(context.Background(), "user:123", fetchFn)

	// Assert
	require.NoError(t, err)
	assert.True(t, fetchCalled.Load(), "fetchFn should have been called")
	assert.Equal(t, []byte(`{"name":"fetched"}`), value)

	// Verify value was stored in cache
	value, err = cache.Get(context.Background(), "user:123")
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"name":"fetched"}`), value)
}

// TestGetOrFetch_Hit_SkipsFetch tests that GetOrFetch returns cached value without calling fetchFn.
func TestGetOrFetch_Hit_SkipsFetch(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Pre-populate cache
	err := cache.Set(context.Background(), "user:123", []byte(`{"name":"cached"}`))
	require.NoError(t, err)

	fetchCalled := atomic.Bool{}
	fetchFn := func(ctx context.Context) ([]byte, error) {
		fetchCalled.Store(true)
		return []byte(`{"name":"fetched"}`), nil
	}

	// Act
	value, err := cache.GetOrFetch(context.Background(), "user:123", fetchFn)

	// Assert
	require.NoError(t, err)
	assert.False(t, fetchCalled.Load(), "fetchFn should not have been called")
	assert.Equal(t, []byte(`{"name":"cached"}`), value)
}

// TestGetOrFetch_StampedeProtection_Coalesced tests that stampede protection coalesces concurrent calls.
func TestGetOrFetch_StampedeProtection_Coalesced(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithStampedeProtection(true))

	fetchCount := atomic.Int64{}
	fetchFn := func(ctx context.Context) ([]byte, error) {
		// Simulate some work
		time.Sleep(50 * time.Millisecond)
		fetchCount.Add(1)
		return []byte("fetched-value"), nil
	}

	// Act - Make 100 concurrent calls
	const numGoroutines = 100
	var wg sync.WaitGroup
	var results [][]byte
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := cache.GetOrFetch(context.Background(), "user:123", fetchFn)
			mu.Lock()
			results = append(results, value)
			mu.Unlock()
			if err != nil {
				t.Logf("GetOrFetch error: %v", err)
			}
		}()
	}
	wg.Wait()

	// Assert
	assert.Equal(t, int64(1), fetchCount.Load(), "fetchFn should have been called exactly once")

	// All results should have the same value
	for _, v := range results {
		assert.Equal(t, []byte("fetched-value"), v)
	}
}

// TestWithNamespace_KeysIsolated tests that keys are isolated by namespace.
func TestWithNamespace_KeysIsolated(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set a value in namespace A
	cacheA := cache.WithNamespace("namespace-a")
	err := cacheA.Set(context.Background(), "user:123", []byte("value-a"))
	require.NoError(t, err)

	// Set the same key in namespace B
	cacheB := cache.WithNamespace("namespace-b")
	err = cacheB.Set(context.Background(), "user:123", []byte("value-b"))
	require.NoError(t, err)

	// Act - Get from each namespace
	valueA, err := cacheA.Get(context.Background(), "user:123")
	require.NoError(t, err)

	valueB, err := cacheB.Get(context.Background(), "user:123")
	require.NoError(t, err)

	// Assert
	assert.Equal(t, []byte("value-a"), valueA)
	assert.Equal(t, []byte("value-b"), valueB)
}

// TestWithAgentID_KeysIsolated tests that keys are isolated by agentID.
func TestWithAgentID_KeysIsolated(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set a value for agent-1
	err := cache.Set(context.Background(), "user:123", []byte("value-agent-1"))
	require.NoError(t, err)

	// Act - Create a new cache with different agentID and set the same key
	cache2 := cache.WithAgentID("agent-2")
	err = cache2.Set(context.Background(), "user:123", []byte("value-agent-2"))
	require.NoError(t, err)

	// Get from original cache
	value1, err := cache.Get(context.Background(), "user:123")
	require.NoError(t, err)

	// Get from new cache
	value2, err := cache2.Get(context.Background(), "user:123")
	require.NoError(t, err)

	// Assert - values should be different
	assert.Equal(t, []byte("value-agent-1"), value1)
	assert.Equal(t, []byte("value-agent-2"), value2)
}

// TestCache_Get_NoAgentID tests that Get returns ErrAgentIDMissing when agentID is empty.
func TestCache_Get_NoAgentID(t *testing.T) {
	// Arrange - create cache without agentID
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns")) // No agentID set

	// Act
	_, err := cache.Get(context.Background(), "some-key")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAgentIDMissing)
}

// TestCache_Set_ExceedsMaxSize tests that Set returns ErrMaxSizeExceeded when value is too large.
func TestCache_Set_ExceedsMaxSize(t *testing.T) {
	// Arrange - create cache with small max size
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithMaxSize(10))

	// Act
	err := cache.Set(context.Background(), "key", []byte("this value is too long"))

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxSizeExceeded)
}

// TestCache_Set_NilValue tests that Set returns error when value is nil.
func TestCache_Set_NilValue(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Act
	err := cache.Set(context.Background(), "key", nil)

	// Assert
	require.Error(t, err)
}

// =============================================================================
// Additional Test Cases
// =============================================================================

// TestCache_Set_WithTTL tests that Set respects TTL option.
func TestCache_Set_WithTTL(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set with custom TTL
	err := cache.Set(context.Background(), "user:123", []byte("value"), WithTTL(30*time.Second))
	require.NoError(t, err)

	// Verify TTL is set
	ttl, err := backend.TTL(context.Background(), "test-ns:agent-1:user:123:")
	require.NoError(t, err)
	assert.InDelta(t, 30*time.Second, ttl, float64(time.Second), "TTL should be approximately 30s")
}

// TestCache_Set_WithTags tests that Set respects tags option.
func TestCache_Set_WithTags(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set with tags
	err := cache.Set(context.Background(), "user:123", []byte("value"), WithTags("tag1", "tag2"))
	require.NoError(t, err)
	// Note: we're not checking the tag index here as it's implementation detail
	// The key should still be stored
	value, err := cache.Get(context.Background(), "user:123")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), value)
}

// TestCache_WithDefaultTTL tests that WithDefaultTTL creates new cache with default TTL.
func TestCache_WithDefaultTTL(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Create new cache with custom default TTL
	customCache := cache.WithDefaultTTL(2 * time.Hour)

	// Set using custom cache - should use new default TTL
	err := customCache.Set(context.Background(), "user:123", []byte("value"))
	require.NoError(t, err)

	// Verify TTL is 2 hours
	ttl, err := backend.TTL(context.Background(), "test-ns:agent-1:user:123:")
	require.NoError(t, err)
	assert.InDelta(t, 2*time.Hour, ttl, float64(time.Second), "TTL should be approximately 2h")
}

// TestCache_WithDefaultTags tests that WithDefaultTags creates new cache with default tags.
func TestCache_WithDefaultTags(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Create new cache with default tags
	customCache := cache.WithDefaultTags("default-tag")

	// Set using custom cache - should use default tags
	err := customCache.Set(context.Background(), "user:123", []byte("value"))
	require.NoError(t, err)
	// Just verify no error - tag index storage is implementation detail
	require.NoError(t, err)
}

// TestCache_GetOrFetch_FetchError tests that GetOrFetch returns error when fetchFn fails.
func TestCache_GetOrFetch_FetchError(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	fetchFn := func(ctx context.Context) ([]byte, error) {
		return nil, errors.New("fetch failed")
	}

	// Act
	_, err := cache.GetOrFetch(context.Background(), "user:123", fetchFn)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFetchFailed)
	assert.Contains(t, err.Error(), "fetch function failed")
}

// TestGetOrFetch_NoStampedeProtection tests that GetOrFetch works without stampede protection.
func TestGetOrFetch_NoStampedeProtection(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithStampedeProtection(false))

	// Make concurrent calls without singleflight
	var wg sync.WaitGroup
	fetchCount := atomic.Int64{}
	mu := sync.Mutex{}
	results := make([][]byte, 0, 10)

	fetchFn := func(ctx context.Context) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		fetchCount.Add(1)
		return []byte("fetched"), nil
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := cache.GetOrFetch(context.Background(), "key", fetchFn)
			mu.Lock()
			results = append(results, value)
			mu.Unlock()
			if err != nil {
				t.Logf("error: %v", err)
			}
		}()
	}
	wg.Wait()

	// Without stampede protection, fetchFn should be called multiple times
	// This is the expected behavior
	assert.Greater(t, fetchCount.Load(), int64(1))
}

// TestCache_Delete_WithFullKey tests that Delete works with fully qualified keys.
func TestCache_Delete_WithFullKey(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// First set a key
	err := cache.Set(context.Background(), "user:123", []byte("value"))
	require.NoError(t, err)

	// Delete with fully qualified key
	err = cache.Delete(context.Background(), "test-ns:agent-1:user:123::")
	require.NoError(t, err)

	// Verify it's gone
	value, err := cache.Get(context.Background(), "test-ns:agent-1:user:123::")
	require.NoError(t, err)
	assert.Nil(t, value)
}

// TestCache_Set_DefaultTTL tests that Set uses namespace default TTL.
func TestCache_Set_DefaultTTL(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithDefaultTTL(45*time.Minute))

	// Set without specifying TTL
	err := cache.Set(context.Background(), "user:123", []byte("value"))
	require.NoError(t, err)

	// Verify TTL is 45 minutes (the default)
	ttl, err := backend.TTL(context.Background(), "test-ns:agent-1:user:123:")
	require.NoError(t, err)
	assert.InDelta(t, 45*time.Minute, ttl, float64(time.Second), "TTL should be approximately 45m")
}

// =============================================================================
// Bulk Operations Tests (Phase 6)
// =============================================================================

// TestGetMany_PartialHits tests that GetMany returns a map with only the hits.
func TestGetMany_PartialHits(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set 2 of 3 keys
	err := cache.Set(context.Background(), "user:1", []byte("value1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:2", []byte("value2"))
	require.NoError(t, err)
	// Don't set user:3

	// Act
	keys := []string{"user:1", "user:2", "user:3"}
	result, err := cache.GetMany(context.Background(), keys)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2, "should have 2 hits")
	assert.Contains(t, result, "test-ns:agent-1:user:1:")
	assert.Contains(t, result, "test-ns:agent-1:user:2:")
	assert.NotContains(t, result, "test-ns:agent-1:user:3:")
}

// TestSetMany_Success tests that SetMany stores all entries.
func TestSetMany_Success(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	entries := map[string][]byte{
		"user:1": []byte("value1"),
		"user:2": []byte("value2"),
		"user:3": []byte("value3"),
	}

	// Act
	err := cache.SetMany(context.Background(), entries)

	// Assert
	require.NoError(t, err)

	// Verify all entries are stored
	value, err := cache.Get(context.Background(), "user:1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)

	value, err = cache.Get(context.Background(), "user:2")
	require.NoError(t, err)
	assert.Equal(t, []byte("value2"), value)

	value, err = cache.Get(context.Background(), "user:3")
	require.NoError(t, err)
	assert.Equal(t, []byte("value3"), value)
}

// TestDeleteMany_Success tests that DeleteMany removes all specified keys.
func TestDeleteMany_Success(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set some values
	err := cache.Set(context.Background(), "user:1", []byte("value1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:2", []byte("value2"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:3", []byte("value3"))
	require.NoError(t, err)

	// Act - Delete user:1 and user:2
	err = cache.DeleteMany(context.Background(), []string{"user:1", "user:2"})

	// Assert
	require.NoError(t, err)

	// Verify user:1 and user:2 are gone
	value, err := cache.Get(context.Background(), "user:1")
	require.NoError(t, err)
	assert.Nil(t, value)

	value, err = cache.Get(context.Background(), "user:2")
	require.NoError(t, err)
	assert.Nil(t, value)

	// Verify user:3 still exists
	value, err = cache.Get(context.Background(), "user:3")
	require.NoError(t, err)
	assert.Equal(t, []byte("value3"), value)
}

// TestDeletePattern_BareStarRejected tests that DeletePattern rejects bare wildcard.
func TestDeletePattern_BareStarRejected(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Act
	err := cache.DeletePattern(context.Background(), "*")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPatternInvalid)
}

// TestDeletePattern_ValidPattern tests that DeletePattern works with valid pattern.
func TestDeletePattern_ValidPattern(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set some values
	err := cache.Set(context.Background(), "user:1", []byte("value1"))
	require.NoError(t, err)

	// Act - Delete pattern that includes agentID prefix
	err = cache.DeletePattern(context.Background(), "test-ns:agent-1:*")

	// Assert
	require.NoError(t, err)
}

// TestDeleteByTag_TagExists tests that DeleteByTag removes entries with that tag.
func TestDeleteByTag_TagExists(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set values with tags
	err := cache.Set(context.Background(), "user:1", []byte("value1"), WithTags("tag1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:2", []byte("value2"), WithTags("tag1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:3", []byte("value3"), WithTags("tag2"))
	require.NoError(t, err)

	// Act - Delete by tag
	err = cache.DeleteByTag(context.Background(), "tag1")

	// Assert
	require.NoError(t, err)
}

// =============================================================================
// Mock CacheObserver
// =============================================================================

// recordingObserver records all observer calls for test assertions.
type recordingObserver struct {
	mu          sync.Mutex
	getCalls    []observerGetCall
	setCalls    []observerSetCall
	deleteCalls []observerDeleteCall
}

type observerGetCall struct {
	Namespace string
	Key       string
	Hit       bool
	LatencyMs float64
}

type observerSetCall struct {
	Namespace string
	Key       string
	SizeBytes int64
	LatencyMs float64
}

type observerDeleteCall struct {
	Namespace string
	Key       string
	Reason    string
}

func (r *recordingObserver) ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getCalls = append(r.getCalls, observerGetCall{Namespace: namespace, Key: key, Hit: hit, LatencyMs: latencyMs})
}

func (r *recordingObserver) ObserveSet(ctx context.Context, namespace, key string, sizeBytes int64, latencyMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setCalls = append(r.setCalls, observerSetCall{Namespace: namespace, Key: key, SizeBytes: sizeBytes, LatencyMs: latencyMs})
}

func (r *recordingObserver) ObserveDelete(ctx context.Context, namespace, key, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleteCalls = append(r.deleteCalls, observerDeleteCall{Namespace: namespace, Key: key, Reason: reason})
}

func (r *recordingObserver) ObserveEviction(ctx context.Context, namespace, key, reason string) {}

func (r *recordingObserver) ObserveWarming(ctx context.Context, namespace string, progress WarmingProgress) {
}

func (r *recordingObserver) getCallsCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.getCalls)
}

func (r *recordingObserver) setCallsCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.setCalls)
}

func (r *recordingObserver) deleteCallsCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.deleteCalls)
}

func (r *recordingObserver) getGetCalls() []observerGetCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]observerGetCall, len(r.getCalls))
	copy(out, r.getCalls)
	return out
}

func (r *recordingObserver) getSetCalls() []observerSetCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]observerSetCall, len(r.setCalls))
	copy(out, r.setCalls)
	return out
}

func (r *recordingObserver) getDeleteCalls() []observerDeleteCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]observerDeleteCall, len(r.deleteCalls))
	copy(out, r.deleteCalls)
	return out
}

// =============================================================================
// Additional Bulk Operation Tests
// =============================================================================

// TestGetMany_AllHits tests that GetMany returns a map with all entries when all keys are present.
func TestGetMany_AllHits(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	err := cache.Set(context.Background(), "user:1", []byte("value1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:2", []byte("value2"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:3", []byte("value3"))
	require.NoError(t, err)

	// Act
	keys := []string{"user:1", "user:2", "user:3"}
	result, err := cache.GetMany(context.Background(), keys)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 3, "should have 3 hits")
	assert.Contains(t, result, "test-ns:agent-1:user:1:")
	assert.Contains(t, result, "test-ns:agent-1:user:2:")
	assert.Contains(t, result, "test-ns:agent-1:user:3:")
	assert.Equal(t, []byte("value1"), result["test-ns:agent-1:user:1:"])
	assert.Equal(t, []byte("value2"), result["test-ns:agent-1:user:2:"])
	assert.Equal(t, []byte("value3"), result["test-ns:agent-1:user:3:"])
}

// TestGetMany_EmptyKeys tests that GetMany returns an empty map when given an empty keys slice.
func TestGetMany_EmptyKeys(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Act
	result, err := cache.GetMany(context.Background(), []string{})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0, "should return empty map for empty keys")
}

// TestSetMany_NilValue tests that SetMany returns ErrSerializationFailed when one value is nil.
func TestSetMany_NilValue(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	entries := map[string][]byte{
		"user:1": []byte("value1"),
		"user:2": nil,
	}

	// Act
	err := cache.SetMany(context.Background(), entries)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSerializationFailed)
}

// TestSetMany_ExceedsMaxSize tests that SetMany returns ErrMaxSizeExceeded when a value exceeds maxSize.
func TestSetMany_ExceedsMaxSize(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithMaxSize(10))

	entries := map[string][]byte{
		"user:1": []byte("ok"),
		"user:2": []byte("this value is way too long"),
	}

	// Act
	err := cache.SetMany(context.Background(), entries)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxSizeExceeded)
}

// TestSetMany_WithTags tests that SetMany stores entries when WithTags option is used.
func TestSetMany_WithTags(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	entries := map[string][]byte{
		"user:1": []byte("value1"),
		"user:2": []byte("value2"),
	}

	// Act
	err := cache.SetMany(context.Background(), entries, WithTags("tag-a", "tag-b"))
	require.NoError(t, err)

	// Assert - entries should still be stored
	value, err := cache.Get(context.Background(), "user:1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)

	value, err = cache.Get(context.Background(), "user:2")
	require.NoError(t, err)
	assert.Equal(t, []byte("value2"), value)
}

// TestDeleteByTag_EmptyTag tests that DeleteByTag returns ErrTagNotFound for empty tag.
func TestDeleteByTag_EmptyTag(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Act
	err := cache.DeleteByTag(context.Background(), "")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTagNotFound)
}

// TestGetMany_ObserverCalledForEachHit tests that the observer is called for each key in GetMany.
func TestGetMany_ObserverCalledForEachHit(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	err := cache.Set(context.Background(), "user:1", []byte("value1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:2", []byte("value2"))
	require.NoError(t, err)
	// user:3 is not set (miss)

	// Act
	_, err = cache.GetMany(context.Background(), []string{"user:1", "user:2", "user:3"})
	require.NoError(t, err)

	// Assert - observer should be called 3 times (once per key)
	getCalls := obs.getGetCalls()
	assert.Len(t, getCalls, 3, "observer should be called once per key")

	// Check that hits and misses are correctly reported
	hitCount := 0
	missCount := 0
	for _, call := range getCalls {
		if call.Hit {
			hitCount++
		} else {
			missCount++
		}
	}
	assert.Equal(t, 2, hitCount, "should have 2 hits")
	assert.Equal(t, 1, missCount, "should have 1 miss")
}

// TestSetMany_ObserverCalledForEach tests that the observer is called for each entry in SetMany.
func TestSetMany_ObserverCalledForEach(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	entries := map[string][]byte{
		"user:1": []byte("value1"),
		"user:2": []byte("value2"),
		"user:3": []byte("value3"),
	}

	// Act
	err := cache.SetMany(context.Background(), entries)
	require.NoError(t, err)

	// Assert - observer should be called 3 times (once per entry)
	setCalls := obs.getSetCalls()
	assert.Len(t, setCalls, 3, "observer should be called once per entry")

	// All calls should be for namespace "test-ns"
	for _, call := range setCalls {
		assert.Equal(t, "test-ns", call.Namespace)
		assert.True(t, call.SizeBytes > 0, "size should be positive")
	}
}

// TestDeleteMany_ObserverCalledForEach tests that the observer is called for each key in DeleteMany.
func TestDeleteMany_ObserverCalledForEach(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	// Set values first
	err := cache.Set(context.Background(), "user:1", []byte("value1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:2", []byte("value2"))
	require.NoError(t, err)

	// Clear set calls from the setup
	obs.setCalls = nil

	// Act
	err = cache.DeleteMany(context.Background(), []string{"user:1", "user:2"})
	require.NoError(t, err)

	// Assert - observer should be called 2 times (once per deleted key)
	deleteCalls := obs.getDeleteCalls()
	assert.Len(t, deleteCalls, 2, "observer should be called once per deleted key")

	// All calls should have reason "manual"
	for _, call := range deleteCalls {
		assert.Equal(t, "test-ns", call.Namespace)
		assert.Equal(t, "manual", call.Reason)
	}
}

// TestDeletePattern_ObserverCalled tests that the observer is called when DeletePattern is invoked.
func TestDeletePattern_ObserverCalled(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	// Set a value first
	err := cache.Set(context.Background(), "user:1", []byte("value1"))
	require.NoError(t, err)

	// Clear set calls from the setup
	obs.setCalls = nil

	// Act
	err = cache.DeletePattern(context.Background(), "test-ns:agent-1:*")
	require.NoError(t, err)

	// Assert - observer should be called once with reason "pattern"
	deleteCalls := obs.getDeleteCalls()
	require.Len(t, deleteCalls, 1, "observer should be called once for DeletePattern")
	assert.Equal(t, "test-ns", deleteCalls[0].Namespace)
	assert.Equal(t, "test-ns:agent-1:*", deleteCalls[0].Key)
	assert.Equal(t, "pattern", deleteCalls[0].Reason)
}

// TestDeleteByTag_ObserverCalled tests that the observer is called when DeleteByTag is invoked.
func TestDeleteByTag_ObserverCalled(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	// Set a value with a tag first
	err := cache.Set(context.Background(), "user:1", []byte("value1"), WithTags("my-tag"))
	require.NoError(t, err)

	// Clear set calls from the setup
	obs.setCalls = nil

	// Act
	err = cache.DeleteByTag(context.Background(), "my-tag")
	require.NoError(t, err)

	// Assert - observer should be called once with reason "tag"
	deleteCalls := obs.getDeleteCalls()
	require.Len(t, deleteCalls, 1, "observer should be called once for DeleteByTag")
	assert.Equal(t, "test-ns", deleteCalls[0].Namespace)
	assert.Equal(t, "_tags:my-tag", deleteCalls[0].Key)
	assert.Equal(t, "tag", deleteCalls[0].Reason)
}

// =============================================================================
// Phase 7: Invalidation Strategies Tests
// =============================================================================

// TestSet_WithTags_PopulatesTagIndex verifies that Set with tags uses SADD to populate
// tag sets (_tags:{namespace}:{agentId}:{tag}) and reverse lookup (_keytags:{resolvedKey}).
func TestSet_WithTags_PopulatesTagIndex(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Act
	err := cache.Set(context.Background(), "user:123", []byte("value"), WithTags("tag1", "tag2"))
	require.NoError(t, err)

	// Assert — tag sets should contain the resolved key
	resolvedKey := "test-ns:agent-1:user:123:"

	// Check tag1 set
	members, err := backend.SMembers(context.Background(), "_tags:test-ns:agent-1:tag1")
	require.NoError(t, err)
	assert.Contains(t, members, resolvedKey, "tag1 set should contain the resolved key")

	// Check tag2 set
	members, err = backend.SMembers(context.Background(), "_tags:test-ns:agent-1:tag2")
	require.NoError(t, err)
	assert.Contains(t, members, resolvedKey, "tag2 set should contain the resolved key")

	// Check reverse lookup
	tags, err := backend.SMembers(context.Background(), "_keytags:"+resolvedKey)
	require.NoError(t, err)
	assert.Contains(t, tags, "tag1", "reverse lookup should contain tag1")
	assert.Contains(t, tags, "tag2", "reverse lookup should contain tag2")
}

// TestDelete_CleansUpTagIndex verifies that Delete removes the key from all tag sets
// and deletes the reverse lookup set.
func TestDelete_CleansUpTagIndex(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set with tags
	err := cache.Set(context.Background(), "user:123", []byte("value"), WithTags("tag1", "tag2"))
	require.NoError(t, err)

	resolvedKey := "test-ns:agent-1:user:123:"

	// Act - Delete the key
	err = cache.Delete(context.Background(), "user:123")
	require.NoError(t, err)

	// Assert — tag sets should no longer contain the resolved key
	members, err := backend.SMembers(context.Background(), "_tags:test-ns:agent-1:tag1")
	require.NoError(t, err)
	assert.NotContains(t, members, resolvedKey, "tag1 set should no longer contain the resolved key")

	members, err = backend.SMembers(context.Background(), "_tags:test-ns:agent-1:tag2")
	require.NoError(t, err)
	assert.NotContains(t, members, resolvedKey, "tag2 set should no longer contain the resolved key")

	// Reverse lookup set should be deleted
	tags, err := backend.SMembers(context.Background(), "_keytags:"+resolvedKey)
	require.NoError(t, err)
	assert.Empty(t, tags, "reverse lookup should be empty after delete")
}

// TestInvalidateByVersion_Match verifies that InvalidateByVersion deletes the key
// when the stored version matches the expected version.
func TestInvalidateByVersion_Match(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	// Set a value with a version
	err := cache.Set(context.Background(), "user:123", []byte("value"))
	require.NoError(t, err)

	resolvedKey := "test-ns:agent-1:user:123:"

	// Manually set the version key (since Set doesn't store versions yet)
	err = backend.Set(context.Background(), "_version:"+resolvedKey, []byte("v1"), 0)
	require.NoError(t, err)

	// Act
	err = cache.InvalidateByVersion(context.Background(), "user:123", "v1")

	// Assert
	require.NoError(t, err)

	// Main key should be deleted
	value, err := cache.Get(context.Background(), "user:123")
	require.NoError(t, err)
	assert.Nil(t, value, "main key should be deleted after version match")

	// Version key should be deleted
	versionVal, err := backend.Get(context.Background(), "_version:"+resolvedKey)
	require.NoError(t, err)
	assert.Nil(t, versionVal, "version key should be deleted after version match")

	// Observer should have been called with reason "version"
	deleteCalls := obs.getDeleteCalls()
	require.Len(t, deleteCalls, 1, "observer should be called once for version invalidation")
	assert.Equal(t, "version", deleteCalls[0].Reason)
}

// TestInvalidateByVersion_Mismatch verifies that InvalidateByVersion is a no-op
// when the stored version doesn't match the expected version.
func TestInvalidateByVersion_Mismatch(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	// Set a value
	err := cache.Set(context.Background(), "user:123", []byte("value"))
	require.NoError(t, err)

	resolvedKey := "test-ns:agent-1:user:123:"

	// Manually set the version key to v1
	err = backend.Set(context.Background(), "_version:"+resolvedKey, []byte("v1"), 0)
	require.NoError(t, err)

	// Act - try to invalidate with v2 (mismatch)
	err = cache.InvalidateByVersion(context.Background(), "user:123", "v2")

	// Assert
	require.NoError(t, err)

	// Main key should still exist
	value, err := cache.Get(context.Background(), "user:123")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), value, "main key should still exist after version mismatch")

	// Version key should still exist
	versionVal, err := backend.Get(context.Background(), "_version:"+resolvedKey)
	require.NoError(t, err)
	assert.Equal(t, []byte("v1"), versionVal, "version key should still exist after version mismatch")

	// No delete observer calls should have been made
	deleteCalls := obs.getDeleteCalls()
	assert.Len(t, deleteCalls, 0, "observer should not be called on version mismatch")
}

// TestInvalidateByVersion_KeyMissing verifies that InvalidateByVersion is a no-op
// when the key (and version) doesn't exist.
func TestInvalidateByVersion_KeyMissing(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	// Act - try to invalidate a non-existent key
	err := cache.InvalidateByVersion(context.Background(), "user:999", "v1")

	// Assert
	require.NoError(t, err, "InvalidateByVersion on missing key should return nil")

	// No delete observer calls should have been made
	deleteCalls := obs.getDeleteCalls()
	assert.Len(t, deleteCalls, 0, "observer should not be called on missing key")
}

// TestInvalidateByVersion_Match_WithTags verifies that InvalidateByVersion with matching version
// also cleans up the tag index (tag sets and _keytags reverse lookup).
func TestInvalidateByVersion_Match_WithTags(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	obs := &recordingObserver{}
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"), WithObserver(obs))

	// Set a value with tags (populates tag index)
	err := cache.Set(context.Background(), "user:123", []byte("value"), WithTags("tag1", "tag2"))
	require.NoError(t, err)

	resolvedKey := "test-ns:agent-1:user:123:"

	// Manually set the version key
	err = backend.Set(context.Background(), "_version:"+resolvedKey, []byte("v1"), 0)
	require.NoError(t, err)

	// Act
	err = cache.InvalidateByVersion(context.Background(), "user:123", "v1")

	// Assert
	require.NoError(t, err)

	// Main key should be deleted
	value, err := cache.Get(context.Background(), "user:123")
	require.NoError(t, err)
	assert.Nil(t, value, "main key should be deleted after version match")

	// Version key should be deleted
	versionVal, err := backend.Get(context.Background(), "_version:"+resolvedKey)
	require.NoError(t, err)
	assert.Nil(t, versionVal, "version key should be deleted after version match")

	// Tag sets should no longer contain the resolved key
	members, err := backend.SMembers(context.Background(), "_tags:test-ns:agent-1:tag1")
	require.NoError(t, err)
	assert.NotContains(t, members, resolvedKey, "tag1 set should no longer contain the resolved key")

	members, err = backend.SMembers(context.Background(), "_tags:test-ns:agent-1:tag2")
	require.NoError(t, err)
	assert.NotContains(t, members, resolvedKey, "tag2 set should no longer contain the resolved key")

	// Reverse lookup set should be deleted
	tags, err := backend.SMembers(context.Background(), "_keytags:"+resolvedKey)
	require.NoError(t, err)
	assert.Empty(t, tags, "reverse lookup should be empty after version invalidation")

	// Observer should have been called with reason "version"
	deleteCalls := obs.getDeleteCalls()
	require.Len(t, deleteCalls, 1, "observer should be called once for version invalidation")
	assert.Equal(t, "version", deleteCalls[0].Reason)
}

// TestDeleteByTag_CleansUpKeyTags verifies that DeleteByTag cleans up the _keytags reverse lookup sets.
func TestDeleteByTag_CleansUpKeyTags(t *testing.T) {
	// Arrange
	backend := NewMockCacheBackend()
	cache := NewCache(backend, WithNamespace("test-ns"), WithAgentID("agent-1"))

	// Set values with tags
	err := cache.Set(context.Background(), "user:1", []byte("value1"), WithTags("tag1"))
	require.NoError(t, err)
	err = cache.Set(context.Background(), "user:2", []byte("value2"), WithTags("tag1"))
	require.NoError(t, err)

	resolvedKey1 := "test-ns:agent-1:user:1:"
	resolvedKey2 := "test-ns:agent-1:user:2:"

	// Verify _keytags exist before DeleteByTag
	tags1, err := backend.SMembers(context.Background(), "_keytags:"+resolvedKey1)
	require.NoError(t, err)
	assert.Contains(t, tags1, "tag1", "_keytags for user:1 should contain tag1 before DeleteByTag")

	tags2, err := backend.SMembers(context.Background(), "_keytags:"+resolvedKey2)
	require.NoError(t, err)
	assert.Contains(t, tags2, "tag1", "_keytags for user:2 should contain tag1 before DeleteByTag")

	// Act
	err = cache.DeleteByTag(context.Background(), "tag1")
	require.NoError(t, err)

	// Assert — _keytags should be cleaned up
	tags1, err = backend.SMembers(context.Background(), "_keytags:"+resolvedKey1)
	require.NoError(t, err)
	assert.Empty(t, tags1, "_keytags for user:1 should be cleaned up after DeleteByTag")

	tags2, err = backend.SMembers(context.Background(), "_keytags:"+resolvedKey2)
	require.NoError(t, err)
	assert.Empty(t, tags2, "_keytags for user:2 should be cleaned up after DeleteByTag")
}
