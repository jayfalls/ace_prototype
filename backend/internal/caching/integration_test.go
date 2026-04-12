//go:build integration

package caching

import (
	"context"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// requireValkey checks if Valkey is reachable and fails the test if not.
// Valkey is a hard requirement for the caching-strategies unit.
func requireValkey(t *testing.T) {
	t.Helper()
	addr := os.Getenv("VALKEY_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Valkey is a hard requirement but is not available at %s: %v", addr, err)
	}
	conn.Close()
}

// setupIntegrationBackend creates a Valkey backend for integration tests.
func setupIntegrationBackend(t *testing.T) CacheBackend {
	t.Helper()
	requireValkey(t)

	addr := os.Getenv("VALKEY_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	cfg := ValkeyConfig{
		Addr: addr,
	}
	backend, err := NewValkeyBackend(cfg)
	if err != nil {
		t.Fatalf("failed to create Valkey backend: %v", err)
	}
	t.Cleanup(func() {
		backend.Close()
	})
	return backend
}

// setupIntegrationCache creates a cache instance for integration tests.
func setupIntegrationCache(t *testing.T, backend CacheBackend, opts ...CacheOption) Cache {
	t.Helper()
	cache := NewCache(backend, opts...)
	return cache
}

// TestIntegration_FullLifecycle tests the full lifecycle: Set → Get → Delete → Get returns nil.
func TestIntegration_FullLifecycle(t *testing.T) {
	backend := setupIntegrationBackend(t)
	ctx := context.Background()

	cache := setupIntegrationCache(t, backend,
		WithNamespace("integration_test"),
		WithAgentID("agent_full_lifecycle"),
		WithDefaultTTL(5*time.Minute),
		WithStampedeProtection(false),
	)

	// Set a value with tags
	err := cache.Set(ctx, "user:1", []byte(`{"name":"Alice"}`),
		WithTags("user", "active"),
	)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get should return the value
	value, err := cache.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value == nil {
		t.Fatal("expected value, got nil")
	}
	if string(value) != `{"name":"Alice"}` {
		t.Fatalf("expected {\"name\":\"Alice\"}, got %s", string(value))
	}

	// Delete should remove the value
	err = cache.Delete(ctx, "user:1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get after delete should return nil
	value, err = cache.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if value != nil {
		t.Fatalf("expected nil after delete, got %s", string(value))
	}
}

// TestIntegration_GetOrFetch_Coalescing tests that 50 concurrent calls to GetOrFetch
// with the same key result in fetchFn being called exactly once.
func TestIntegration_GetOrFetch_Coalescing(t *testing.T) {
	backend := setupIntegrationBackend(t)
	ctx := context.Background()

	cache := setupIntegrationCache(t, backend,
		WithNamespace("integration_test"),
		WithAgentID("agent_coalescing"),
		WithDefaultTTL(5*time.Minute),
		WithStampedeProtection(true),
	)

	var fetchCount atomic.Int64
	var wg sync.WaitGroup
	results := make([][]byte, 50)
	errs := make([]error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, err := cache.GetOrFetch(ctx, "coalesce_key", func(ctx context.Context) ([]byte, error) {
				fetchCount.Add(1)
				// Simulate slow fetch
				time.Sleep(50 * time.Millisecond)
				return []byte("fetched_once"), nil
			})
			results[idx] = val
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	if fetchCount.Load() != 1 {
		t.Fatalf("expected fetchFn to be called exactly once, was called %d times", fetchCount.Load())
	}

	for i := 0; i < 50; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Errorf("goroutine %d: expected value, got nil", i)
			continue
		}
		if string(results[i]) != "fetched_once" {
			t.Errorf("goroutine %d: expected 'fetched_once', got %s", i, string(results[i]))
		}
	}
}

// TestIntegration_TagIndexLifecycle tests Set with tags → DeleteByTag → all tagged keys deleted.
func TestIntegration_TagIndexLifecycle(t *testing.T) {
	backend := setupIntegrationBackend(t)
	ctx := context.Background()

	cache := setupIntegrationCache(t, backend,
		WithNamespace("integration_test"),
		WithAgentID("agent_tag_lifecycle"),
		WithDefaultTTL(5*time.Minute),
		WithStampedeProtection(false),
	)

	// Set multiple values with the same tag
	err := cache.Set(ctx, "item:1", []byte("value1"), WithTags("category_a"))
	if err != nil {
		t.Fatalf("Set item:1 failed: %v", err)
	}
	err = cache.Set(ctx, "item:2", []byte("value2"), WithTags("category_a"))
	if err != nil {
		t.Fatalf("Set item:2 failed: %v", err)
	}
	err = cache.Set(ctx, "item:3", []byte("value3"), WithTags("category_b"))
	if err != nil {
		t.Fatalf("Set item:3 failed: %v", err)
	}

	// Verify all values are retrievable
	for _, key := range []string{"item:1", "item:2", "item:3"} {
		val, err := cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get %s failed: %v", key, err)
		}
		if val == nil {
			t.Fatalf("expected value for %s, got nil", key)
		}
	}

	// Delete by tag "category_a" should remove item:1 and item:2 but not item:3
	err = cache.DeleteByTag(ctx, "category_a")
	if err != nil {
		t.Fatalf("DeleteByTag failed: %v", err)
	}

	// item:1 and item:2 should be gone
	for _, key := range []string{"item:1", "item:2"} {
		val, err := cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get %s after DeleteByTag failed: %v", key, err)
		}
		if val != nil {
			t.Errorf("expected nil for %s after DeleteByTag, got %s", key, string(val))
		}
	}

	// item:3 should still exist
	val, err := cache.Get(ctx, "item:3")
	if err != nil {
		t.Fatalf("Get item:3 after DeleteByTag failed: %v", err)
	}
	if val == nil {
		t.Error("expected item:3 to still exist after DeleteByTag for category_a")
	}
}

// TestIntegration_BulkOperations tests SetMany → GetMany → DeleteMany → correct hit/miss.
func TestIntegration_BulkOperations(t *testing.T) {
	backend := setupIntegrationBackend(t)
	ctx := context.Background()

	cache := setupIntegrationCache(t, backend,
		WithNamespace("integration_test"),
		WithAgentID("agent_bulk"),
		WithDefaultTTL(5*time.Minute),
		WithStampedeProtection(false),
	)

	// SetMany
	entries := map[string][]byte{
		"bulk:1": []byte("val1"),
		"bulk:2": []byte("val2"),
		"bulk:3": []byte("val3"),
	}
	err := cache.SetMany(ctx, entries)
	if err != nil {
		t.Fatalf("SetMany failed: %v", err)
	}

	// GetMany for all keys + one missing key
	keys := []string{"bulk:1", "bulk:2", "bulk:3", "bulk:missing"}
	result, err := cache.GetMany(ctx, keys)
	if err != nil {
		t.Fatalf("GetMany failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 hits, got %d", len(result))
	}
	for _, key := range []string{"bulk:1", "bulk:2", "bulk:3"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected %s in result", key)
		}
	}
	if _, ok := result["bulk:missing"]; ok {
		t.Error("did not expect bulk:missing in result")
	}

	// DeleteMany
	err = cache.DeleteMany(ctx, []string{"bulk:1", "bulk:2"})
	if err != nil {
		t.Fatalf("DeleteMany failed: %v", err)
	}

	// Verify deletion
	result, err = cache.GetMany(ctx, []string{"bulk:1", "bulk:2", "bulk:3"})
	if err != nil {
		t.Fatalf("GetMany after DeleteMany failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 hit after DeleteMany, got %d", len(result))
	}
	if _, ok := result["bulk:3"]; !ok {
		t.Error("expected bulk:3 to still exist")
	}
}

// TestIntegration_NamespaceIsolation tests that setting in namespace "ns1" is not visible from "ns2".
func TestIntegration_NamespaceIsolation(t *testing.T) {
	backend := setupIntegrationBackend(t)
	ctx := context.Background()

	cache := setupIntegrationCache(t, backend,
		WithNamespace("ns1"),
		WithAgentID("agent_ns"),
		WithDefaultTTL(5*time.Minute),
		WithStampedeProtection(false),
	)
	cacheNS2 := cache.WithNamespace("ns2")

	// Set in ns1
	err := cache.Set(ctx, "isolated_key", []byte("ns1_value"))
	if err != nil {
		t.Fatalf("Set in ns1 failed: %v", err)
	}

	// Get from ns2 should return nil
	val, err := cacheNS2.Get(ctx, "isolated_key")
	if err != nil {
		t.Fatalf("Get from ns2 failed: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil from ns2, got %s", string(val))
	}

	// Get from ns1 should return the value
	val, err = cache.Get(ctx, "isolated_key")
	if err != nil {
		t.Fatalf("Get from ns1 failed: %v", err)
	}
	if val == nil {
		t.Error("expected value from ns1, got nil")
	}
	if string(val) != "ns1_value" {
		t.Errorf("expected 'ns1_value', got %s", string(val))
	}
}

// TestIntegration_AgentIsolation tests that setting with agentID "agent1" is not visible from "agent2".
func TestIntegration_AgentIsolation(t *testing.T) {
	backend := setupIntegrationBackend(t)
	ctx := context.Background()

	cache := setupIntegrationCache(t, backend,
		WithNamespace("agent_iso_test"),
		WithAgentID("agent1"),
		WithDefaultTTL(5*time.Minute),
		WithStampedeProtection(false),
	)
	cacheAgent2 := cache.WithAgentID("agent2")

	// Set with agent1
	err := cache.Set(ctx, "shared_key", []byte("agent1_value"))
	if err != nil {
		t.Fatalf("Set with agent1 failed: %v", err)
	}

	// Get with agent2 should return nil
	val, err := cacheAgent2.Get(ctx, "shared_key")
	if err != nil {
		t.Fatalf("Get with agent2 failed: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil from agent2, got %s", string(val))
	}

	// Get with agent1 should return the value
	val, err = cache.Get(ctx, "shared_key")
	if err != nil {
		t.Fatalf("Get with agent1 failed: %v", err)
	}
	if val == nil {
		t.Error("expected value from agent1, got nil")
	}
	if string(val) != "agent1_value" {
		t.Errorf("expected 'agent1_value', got %s", string(val))
	}
}
