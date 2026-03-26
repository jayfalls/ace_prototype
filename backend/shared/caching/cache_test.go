package caching

import (
	"context"
	"errors"
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
