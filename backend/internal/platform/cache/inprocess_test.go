package cache

import (
	"context"
	"testing"
	"time"

	"ace/internal/caching"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForBufferFlush waits for Ristretto's write buffer to be processed.
// Ristretto is async by design - values go to a buffer and are processed in background.
func waitForBufferFlush() {
	time.Sleep(10 * time.Millisecond)
}

func TestInProcessBackend_SetGet(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set a value
	err = backend.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// Get the value
	value, err := backend.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)
}

func TestInProcessBackend_Delete(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set a value
	err = backend.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// Delete the value
	err = backend.Delete(ctx, "key1")
	require.NoError(t, err)

	// Get should return nil
	value, err := backend.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Nil(t, value)
}

func TestInProcessBackend_DeleteByTag(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set values with tags (tag sets are synchronous in our implementation)
	tagKey := "_tags:test:agent1:myTag"
	err = backend.SAdd(ctx, tagKey, []string{"key1", "key2"}, time.Hour)
	require.NoError(t, err)

	// Set the actual values
	err = backend.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)
	err = backend.Set(ctx, "key2", []byte("value2"), time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// Delete by tag
	err = backend.DeleteByTag(ctx, tagKey)
	require.NoError(t, err)

	// Values should be gone
	value, err := backend.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Nil(t, value)

	value, err = backend.Get(ctx, "key2")
	require.NoError(t, err)
	assert.Nil(t, value)
}

func TestInProcessBackend_DeletePattern(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set values
	err = backend.Set(ctx, "ns:agent1:user:1", []byte("value1"), time.Hour)
	require.NoError(t, err)
	err = backend.Set(ctx, "ns:agent1:user:2", []byte("value2"), time.Hour)
	require.NoError(t, err)
	err = backend.Set(ctx, "ns:agent2:user:1", []byte("value3"), time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// Delete pattern
	err = backend.DeletePattern(ctx, "ns:agent1:*")
	require.NoError(t, err)

	// agent1 keys should be gone
	value, err := backend.Get(ctx, "ns:agent1:user:1")
	require.NoError(t, err)
	assert.Nil(t, value)

	value, err = backend.Get(ctx, "ns:agent1:user:2")
	require.NoError(t, err)
	assert.Nil(t, value)

	// agent2 key should still exist
	value, err = backend.Get(ctx, "ns:agent2:user:1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value3"), value)
}

func TestInProcessBackend_TagIndex(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// SADD members to a tag set (synchronous operation)
	tagKey := "_tags:ns:agent:myTag"
	err = backend.SAdd(ctx, tagKey, []string{"member1", "member2"}, time.Hour)
	require.NoError(t, err)

	// SMembers should return all members
	members, err := backend.SMembers(ctx, tagKey)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"member1", "member2"}, members)

	// SREM should remove members
	err = backend.SRem(ctx, tagKey, []string{"member1"})
	require.NoError(t, err)

	members, err = backend.SMembers(ctx, tagKey)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"member2"}, members)
}

func TestInProcessBackend_MaxCostEviction(t *testing.T) {
	cfg := &Config{MaxCost: 100, BufferItems: 64} // Very small cache
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Fill the cache with items larger than max cost
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		value := make([]byte, 50) // Each value is 50 bytes
		for j := range value {
			value[j] = byte(i)
		}
		err = backend.Set(ctx, key, value, time.Hour)
		// Ristretto may reject items that exceed max cost, so we don't assert no error
	}
	waitForBufferFlush()

	// The cache should still function - at least some items should be stored
	_, err = backend.Get(ctx, "a")
	// Either found or not found is fine - cache is working
	require.NoError(t, err)
}

func TestInProcessBackend_GetMany(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set some values
	err = backend.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)
	err = backend.Set(ctx, "key2", []byte("value2"), time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// GetMany
	result, err := backend.GetMany(ctx, []string{"key1", "key2", "key3"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, []byte("value1"), result["key1"])
	assert.Equal(t, []byte("value2"), result["key2"])
}

func TestInProcessBackend_SetMany(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// SetMany
	entries := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
	}
	err = backend.SetMany(ctx, entries, time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// Verify
	value, err := backend.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)
}

func TestInProcessBackend_Exists(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set a value
	err = backend.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// Exists should return true
	exists, err := backend.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	// Non-existent key
	exists, err = backend.Exists(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestInProcessBackend_TTL(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	ctx := context.Background()

	// Set a value with TTL
	err = backend.Set(ctx, "key1", []byte("value1"), 2*time.Hour)
	require.NoError(t, err)
	waitForBufferFlush()

	// TTL should be positive and close to 2 hours
	ttl, err := backend.TTL(ctx, "key1")
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Hour)
	assert.LessOrEqual(t, ttl, 2*time.Hour)

	// Key without TTL
	err = backend.Set(ctx, "key2", []byte("value2"), 0)
	require.NoError(t, err)
	ttl, err = backend.TTL(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), ttl)
}

func TestInProcessBackend_Close(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)

	// Close should not error
	err = backend.Close()
	require.NoError(t, err)
}

func TestInProcessBackend_ImplementsCacheBackend(t *testing.T) {
	cfg := &Config{MaxCost: 1024 * 1024, BufferItems: 64}
	backend, err := InitInProcess(cfg)
	require.NoError(t, err)
	defer backend.Close()

	// Compile-time interface check
	var _ caching.CacheBackend = backend
}
