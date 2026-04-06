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
// Mock Cache for Warming Tests
// =============================================================================

// mockCache implements Cache for testing warming.
type mockCache struct {
	mu       sync.Mutex
	store    map[string][]byte
	expireAt map[string]time.Time
	ns       string
}

func newMockCache() *mockCache {
	return &mockCache{
		store:    make(map[string][]byte),
		expireAt: make(map[string]time.Time),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.store[key]; ok {
		return v, nil
	}
	return nil, nil
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, opts ...SetOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	setOpts := applySetOptions(opts...)
	m.store[key] = value
	if setOpts.TTL > 0 {
		m.expireAt[key] = time.Now().Add(setOpts.TTL)
	}
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	delete(m.expireAt, key)
	return nil
}

func (m *mockCache) GetOrFetch(ctx context.Context, key string, fetchFn FetchFunc, opts ...SetOption) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.store[key]; ok {
		return v, nil
	}
	value, err := fetchFn(ctx)
	if err != nil {
		return nil, err
	}
	m.store[key] = value
	return value, nil
}

func (m *mockCache) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
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

func (m *mockCache) SetMany(ctx context.Context, entries map[string][]byte, opts ...SetOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range entries {
		m.store[key] = value
	}
	return nil
}

func (m *mockCache) DeleteMany(ctx context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.store, key)
		delete(m.expireAt, key)
	}
	return nil
}

func (m *mockCache) DeletePattern(ctx context.Context, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.store {
		if match, err := patternMatch(pattern, key); err == nil && match {
			delete(m.store, key)
			delete(m.expireAt, key)
		}
	}
	return nil
}

func (m *mockCache) DeleteByTag(ctx context.Context, tag string) error {
	return nil
}

func (m *mockCache) InvalidateByVersion(ctx context.Context, key string, expectedVersion string) error {
	return nil
}

func (m *mockCache) Stats(ctx context.Context) (*CacheStats, error) {
	return &CacheStats{}, nil
}

func (m *mockCache) WithNamespace(namespace string) Cache {
	// Return a new mockCache that shares the same underlying store and mutex.
	// We create a wrapper that delegates to the original.
	return &namespacedCache{inner: m, namespace: namespace}
}

// namespacedCache wraps a mockCache and applies a namespace prefix to keys.
type namespacedCache struct {
	inner     *mockCache
	namespace string
}

func (n *namespacedCache) Get(ctx context.Context, key string) ([]byte, error) {
	return n.inner.Get(ctx, n.namespace+":"+key)
}

func (n *namespacedCache) Set(ctx context.Context, key string, value []byte, opts ...SetOption) error {
	return n.inner.Set(ctx, n.namespace+":"+key, value, opts...)
}

func (n *namespacedCache) Delete(ctx context.Context, key string) error {
	return n.inner.Delete(ctx, n.namespace+":"+key)
}

func (n *namespacedCache) GetOrFetch(ctx context.Context, key string, fetchFn FetchFunc, opts ...SetOption) ([]byte, error) {
	return n.inner.GetOrFetch(ctx, n.namespace+":"+key, fetchFn, opts...)
}

func (n *namespacedCache) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	resolved := make([]string, len(keys))
	for i, k := range keys {
		resolved[i] = n.namespace + ":" + k
	}
	return n.inner.GetMany(ctx, resolved)
}

func (n *namespacedCache) SetMany(ctx context.Context, entries map[string][]byte, opts ...SetOption) error {
	resolved := make(map[string][]byte)
	for k, v := range entries {
		resolved[n.namespace+":"+k] = v
	}
	return n.inner.SetMany(ctx, resolved, opts...)
}

func (n *namespacedCache) DeleteMany(ctx context.Context, keys []string) error {
	resolved := make([]string, len(keys))
	for i, k := range keys {
		resolved[i] = n.namespace + ":" + k
	}
	return n.inner.DeleteMany(ctx, resolved)
}

func (n *namespacedCache) DeletePattern(ctx context.Context, pattern string) error {
	return n.inner.DeletePattern(ctx, n.namespace+":"+pattern)
}

func (n *namespacedCache) DeleteByTag(ctx context.Context, tag string) error {
	return n.inner.DeleteByTag(ctx, tag)
}

func (n *namespacedCache) InvalidateByVersion(ctx context.Context, key string, expectedVersion string) error {
	return n.inner.InvalidateByVersion(ctx, n.namespace+":"+key, expectedVersion)
}

func (n *namespacedCache) Stats(ctx context.Context) (*CacheStats, error) {
	return n.inner.Stats(ctx)
}

func (n *namespacedCache) WithNamespace(namespace string) Cache {
	return &namespacedCache{inner: n.inner, namespace: namespace}
}

func (n *namespacedCache) WithAgentID(agentID string) Cache {
	return n
}

func (n *namespacedCache) WithDefaultTTL(ttl time.Duration) Cache {
	return n
}

func (n *namespacedCache) WithDefaultTags(tags ...string) Cache {
	return n
}

func (m *mockCache) WithAgentID(agentID string) Cache {
	return m
}

func (m *mockCache) WithDefaultTTL(ttl time.Duration) Cache {
	return m
}

func (m *mockCache) WithDefaultTags(tags ...string) Cache {
	return m
}

// patternMatch checks if a key matches a simple glob pattern (supports * only).
func patternMatch(pattern, key string) (bool, error) {
	if pattern == "*" {
		return true, nil
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, prefix), nil
	}
	return key == pattern, nil
}

// =============================================================================
// WarmingManager Tests
// =============================================================================

// TestWarmingManager_NewWarmingManager tests that the constructor creates the correct config map.
func TestWarmingManager_NewWarmingManager(t *testing.T) {
	// Arrange
	cache := newMockCache()
	configs := []WarmingConfig{
		{Namespace: "ns1", Enabled: true, OnStartup: true, Parallel: true},
		{Namespace: "ns2", Enabled: true, OnStartup: false, Parallel: false},
		{Namespace: "ns3", Enabled: true, OnStartup: true, Parallel: false},
	}

	// Act
	mgr := NewWarmingManager(configs, cache, nil)

	// Assert
	require.NotNil(t, mgr)

	// Verify that progress can be tracked for each namespace
	for _, ns := range []string{"ns1", "ns2", "ns3"} {
		prog := mgr.TrackProgress(ns)
		assert.Equal(t, ns, prog.Namespace)
		assert.Equal(t, int64(0), prog.SuccessCount)
		assert.Equal(t, int64(0), prog.FailureCount)
	}
}

// TestWarmOnStartup_CallsWarmFuncForEnabled tests that WarmFunc is called for each
// namespace with OnStartup=true.
func TestWarmOnStartup_CallsWarmFuncForEnabled(t *testing.T) {
	// Arrange
	cache := newMockCache()
	var called sync.Map // map[string]bool

	configs := []WarmingConfig{
		{
			Namespace: "ns1",
			Enabled:   true,
			OnStartup: true,
			Parallel:  true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				called.Store("ns1", true)
				return nil
			},
		},
		{
			Namespace: "ns2",
			Enabled:   true,
			OnStartup: false, // Should NOT be called
			Parallel:  true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				called.Store("ns2", true)
				return nil
			},
		},
		{
			Namespace: "ns3",
			Enabled:   true,
			OnStartup: true,
			Parallel:  false,
			WarmFunc: func(ctx context.Context, c Cache) error {
				called.Store("ns3", true)
				return nil
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.WarmOnStartup(context.Background())

	// Assert
	require.NoError(t, err)

	_, ns1Called := called.Load("ns1")
	_, ns2Called := called.Load("ns2")
	_, ns3Called := called.Load("ns3")

	assert.True(t, ns1Called, "ns1 WarmFunc should have been called (OnStartup=true)")
	assert.False(t, ns2Called, "ns2 WarmFunc should NOT have been called (OnStartup=false)")
	assert.True(t, ns3Called, "ns3 WarmFunc should have been called (OnStartup=true)")
}

// TestWarm_DeadlineExceeded tests that a slow WarmFunc returns ErrWarmingTimeout.
func TestWarm_DeadlineExceeded(t *testing.T) {
	// Arrange
	cache := newMockCache()

	slowWarmFunc := func(ctx context.Context, c Cache) error {
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	configs := []WarmingConfig{
		{
			Namespace: "ns-slow",
			Enabled:   true,
			Deadline:  50 * time.Millisecond,
			WarmFunc:  slowWarmFunc,
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.Warm(context.Background(), "ns-slow")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWarmingTimeout)
}

// TestTrackProgress_AfterWarm tests that TrackProgress reflects entries populated during warming.
func TestTrackProgress_AfterWarm(t *testing.T) {
	// Arrange
	cache := newMockCache()

	configs := []WarmingConfig{
		{
			Namespace: "ns1",
			Enabled:   true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				// Simulate populating some entries
				err := c.Set(ctx, "key1", []byte("value1"))
				if err != nil {
					return err
				}
				err = c.Set(ctx, "key2", []byte("value2"))
				if err != nil {
					return err
				}
				return nil
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.Warm(context.Background(), "ns1")

	// Assert
	require.NoError(t, err)

	prog := mgr.TrackProgress("ns1")
	assert.Equal(t, "ns1", prog.Namespace)
	assert.Equal(t, int64(1), prog.SuccessCount)
	assert.Equal(t, int64(0), prog.FailureCount)
	assert.True(t, prog.ElapsedMs > 0, "elapsed time should be positive")

	// Verify entries were stored in cache
	value, err := cache.Get(context.Background(), "ns1:key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)

	value, err = cache.Get(context.Background(), "ns1:key2")
	require.NoError(t, err)
	assert.Equal(t, []byte("value2"), value)
}

// TestWarm_FuncError tests that errors from WarmFunc are propagated to the caller.
func TestWarm_FuncError(t *testing.T) {
	// Arrange
	cache := newMockCache()
	expectedErr := errors.New("warming function failed")

	configs := []WarmingConfig{
		{
			Namespace: "ns-fail",
			Enabled:   true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				return expectedErr
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.Warm(context.Background(), "ns-fail")

	// Assert
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)

	// Progress should reflect failure
	prog := mgr.TrackProgress("ns-fail")
	assert.Equal(t, int64(0), prog.SuccessCount)
	assert.Equal(t, int64(1), prog.FailureCount)
}

// TestWarm_ConcurrentNamespaces tests that 3 namespaces can be warmed in parallel with no races.
func TestWarm_ConcurrentNamespaces(t *testing.T) {
	// Arrange
	cache := newMockCache()

	configs := []WarmingConfig{
		{
			Namespace: "ns-a",
			Enabled:   true,
			Parallel:  true,
			OnStartup: true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				time.Sleep(10 * time.Millisecond)
				return c.Set(ctx, "key-a", []byte("value-a"))
			},
		},
		{
			Namespace: "ns-b",
			Enabled:   true,
			Parallel:  true,
			OnStartup: true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				time.Sleep(10 * time.Millisecond)
				return c.Set(ctx, "key-b", []byte("value-b"))
			},
		},
		{
			Namespace: "ns-c",
			Enabled:   true,
			Parallel:  true,
			OnStartup: true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				time.Sleep(10 * time.Millisecond)
				return c.Set(ctx, "key-c", []byte("value-c"))
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act — WarmOnStartup runs all 3 in parallel
	err := mgr.WarmOnStartup(context.Background())

	// Assert
	require.NoError(t, err)

	// All namespaces should have success progress
	for _, ns := range []string{"ns-a", "ns-b", "ns-c"} {
		prog := mgr.TrackProgress(ns)
		assert.Equal(t, int64(1), prog.SuccessCount, "namespace %s should have 1 success", ns)
		assert.Equal(t, int64(0), prog.FailureCount, "namespace %s should have 0 failures", ns)
	}
}

// TestWarm_UnknownNamespace tests that warming an unknown namespace returns ErrInvalidKey.
func TestWarm_UnknownNamespace(t *testing.T) {
	// Arrange
	cache := newMockCache()
	configs := []WarmingConfig{
		{Namespace: "known-ns", Enabled: true, WarmFunc: func(ctx context.Context, c Cache) error { return nil }},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.Warm(context.Background(), "unknown-ns")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidKey)
}

// TestWarm_DisabledNamespace_ReturnsNil tests that warming a disabled namespace
// returns nil without calling the WarmFunc.
func TestWarm_DisabledNamespace_ReturnsNil(t *testing.T) {
	// Arrange
	cache := newMockCache()
	warmFuncCalled := false

	configs := []WarmingConfig{
		{
			Namespace: "ns-disabled",
			Enabled:   false,
			WarmFunc: func(ctx context.Context, c Cache) error {
				warmFuncCalled = true
				return nil
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.Warm(context.Background(), "ns-disabled")

	// Assert
	require.NoError(t, err)
	assert.False(t, warmFuncCalled, "WarmFunc should not be called for disabled namespace")
}

// TestWarmOnStartup_SkipsDisabledConfigs tests that WarmOnStartup skips configs
// with Enabled=false even when OnStartup=true.
func TestWarmOnStartup_SkipsDisabledConfigs(t *testing.T) {
	// Arrange
	cache := newMockCache()
	var called sync.Map

	configs := []WarmingConfig{
		{
			Namespace: "ns-disabled",
			Enabled:   false,
			OnStartup: true,
			Parallel:  true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				called.Store("ns-disabled", true)
				return nil
			},
		},
		{
			Namespace: "ns-enabled",
			Enabled:   true,
			OnStartup: true,
			Parallel:  true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				called.Store("ns-enabled", true)
				return nil
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.WarmOnStartup(context.Background())

	// Assert
	require.NoError(t, err)

	_, disabledCalled := called.Load("ns-disabled")
	_, enabledCalled := called.Load("ns-enabled")

	assert.False(t, disabledCalled, "disabled namespace WarmFunc should NOT be called")
	assert.True(t, enabledCalled, "enabled namespace WarmFunc should be called")
}

// TestWarmOnStartup_PartialFailure tests that WarmOnStartup continues after a failure
// and returns an aggregated error.
func TestWarmOnStartup_PartialFailure(t *testing.T) {
	// Arrange
	cache := newMockCache()
	failedErr := errors.New("namespace failed")

	configs := []WarmingConfig{
		{
			Namespace: "ns-ok",
			Enabled:   true,
			OnStartup: true,
			Parallel:  false,
			WarmFunc: func(ctx context.Context, c Cache) error {
				return nil
			},
		},
		{
			Namespace: "ns-fail",
			Enabled:   true,
			OnStartup: true,
			Parallel:  false,
			WarmFunc: func(ctx context.Context, c Cache) error {
				return failedErr
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.WarmOnStartup(context.Background())

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "warming failed for 1 namespace(s)")
	assert.Contains(t, err.Error(), failedErr.Error())

	// The successful namespace should still have progress
	prog := mgr.TrackProgress("ns-ok")
	assert.Equal(t, int64(1), prog.SuccessCount)
}

// TestWarmOnStartup_ParallelRunsConcurrently tests that parallel configs actually run concurrently.
func TestWarmOnStartup_ParallelRunsConcurrently(t *testing.T) {
	// Arrange
	cache := newMockCache()
	var concurrent atomic.Int64
	var maxConcurrent atomic.Int64

	warmFunc := func(ctx context.Context, c Cache) error {
		cur := concurrent.Add(1)
		// Track peak concurrency
		for {
			old := maxConcurrent.Load()
			if cur <= old {
				break
			}
			if maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	}

	configs := []WarmingConfig{
		{Namespace: "ns-1", Enabled: true, OnStartup: true, Parallel: true, WarmFunc: warmFunc},
		{Namespace: "ns-2", Enabled: true, OnStartup: true, Parallel: true, WarmFunc: warmFunc},
		{Namespace: "ns-3", Enabled: true, OnStartup: true, Parallel: true, WarmFunc: warmFunc},
	}

	mgr := NewWarmingManager(configs, cache, nil)

	// Act
	err := mgr.WarmOnStartup(context.Background())

	// Assert
	require.NoError(t, err)
	// All 3 ran in parallel, so maxConcurrent should be 3
	assert.Equal(t, int64(3), maxConcurrent.Load(), "all 3 parallel configs should have run concurrently")
}

// TestTrackProgress_UnknownNamespace tests that TrackProgress returns a zero-value for unknown namespaces.
func TestTrackProgress_UnknownNamespace(t *testing.T) {
	// Arrange
	cache := newMockCache()
	mgr := NewWarmingManager(nil, cache, nil)

	// Act
	prog := mgr.TrackProgress("never-warmed")

	// Assert
	assert.Equal(t, "never-warmed", prog.Namespace)
	assert.Equal(t, int64(0), prog.SuccessCount)
	assert.Equal(t, int64(0), prog.FailureCount)
	assert.Equal(t, float64(0), prog.ElapsedMs)
}

// TestWarm_ObserverCalled tests that the observer is called after warming completes.
func TestWarm_ObserverCalled(t *testing.T) {
	// Arrange
	cache := newMockCache()
	obs := &recordingWarmingObserver{}

	configs := []WarmingConfig{
		{
			Namespace: "ns1",
			Enabled:   true,
			WarmFunc: func(ctx context.Context, c Cache) error {
				return nil
			},
		},
	}

	mgr := NewWarmingManager(configs, cache, obs)

	// Act
	err := mgr.Warm(context.Background(), "ns1")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, obs.callCount(), "observer should be called once")
	assert.Equal(t, "ns1", obs.lastNamespace())
}

// =============================================================================
// Recording Warming Observer
// =============================================================================

// recordingWarmingObserver records ObserveWarming calls for test assertions.
type recordingWarmingObserver struct {
	mu         sync.Mutex
	calls      []warmingObserverCall
	lastNs     string
	totalCalls int
}

type warmingObserverCall struct {
	Namespace string
	Progress  WarmingProgress
}

func (r *recordingWarmingObserver) ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64) {
}

func (r *recordingWarmingObserver) ObserveSet(ctx context.Context, namespace, key string, sizeBytes int64, latencyMs float64) {
}

func (r *recordingWarmingObserver) ObserveDelete(ctx context.Context, namespace, key, reason string) {
}

func (r *recordingWarmingObserver) ObserveEviction(ctx context.Context, namespace, key, reason string) {
}

func (r *recordingWarmingObserver) ObserveWarming(ctx context.Context, namespace string, progress WarmingProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, warmingObserverCall{Namespace: namespace, Progress: progress})
	r.lastNs = namespace
	r.totalCalls++
}

func (r *recordingWarmingObserver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalCalls
}

func (r *recordingWarmingObserver) lastNamespace() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastNs
}
