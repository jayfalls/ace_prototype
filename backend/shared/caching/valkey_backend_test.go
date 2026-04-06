package caching

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

// newTestBackend creates a valkeyBackend with a mock client for testing.
func newTestBackend(t *testing.T) (*valkeyBackend, *mock.Client) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	client := mock.NewClient(ctrl)
	return &valkeyBackend{client: client}, client
}

// TestValkeyBackend_Get_Miss verifies Get returns ErrCacheMiss for a missing key.
func TestValkeyBackend_Get_Miss(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.Match("GET", "missing")).Return(mock.Result(mock.ValkeyNil()))

	_, err := b.Get(ctx, "missing")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got: %v", err)
	}
}

// TestValkeyBackend_Get_Hit verifies Get returns value after Set.
func TestValkeyBackend_Get_Hit(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.Match("GET", "mykey")).Return(mock.Result(mock.ValkeyBlobString("myvalue")))

	val, err := b.Get(ctx, "mykey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "myvalue" {
		t.Fatalf("expected %q, got %q", "myvalue", string(val))
	}
}

// TestValkeyBackend_Set_WithTTL verifies TTL returns correct remaining duration.
func TestValkeyBackend_Set_WithTTL(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// SET with EX (seconds) — 10s TTL
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return len(cmd) >= 2 && cmd[0] == "SET" && cmd[1] == "ttlk"
	})).Return(mock.Result(mock.ValkeyString("OK")))

	err := b.Set(ctx, "ttlk", []byte("val"), 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// TTL returns 10 seconds
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return len(cmd) >= 2 && cmd[0] == "TTL" && cmd[1] == "ttlk"
	})).Return(mock.Result(mock.ValkeyInt64(10)))

	ttl, err := b.TTL(ctx, "ttlk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl < 9*time.Second || ttl > 11*time.Second {
		t.Fatalf("expected ~10s TTL, got %v", ttl)
	}
}

// TestValkeyBackend_Delete_Exists verifies subsequent Get returns ErrCacheMiss after delete.
func TestValkeyBackend_Delete_Exists(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// Delete the key
	client.EXPECT().Do(ctx, mock.Match("DEL", "delkey")).Return(mock.Result(mock.ValkeyInt64(1)))

	err := b.Delete(ctx, "delkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get returns cache miss
	client.EXPECT().Do(ctx, mock.Match("GET", "delkey")).Return(mock.Result(mock.ValkeyNil()))

	_, err = b.Get(ctx, "delkey")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got: %v", err)
	}
}

// TestValkeyBackend_GetMany_PartialHits verifies GetMany returns only present keys.
func TestValkeyBackend_GetMany_PartialHits(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// MGET returns: ["v1", nil, "v3"] for keys ["k1", "k2", "k3"]
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return cmd[0] == "MGET" && len(cmd) == 4 // MGET k1 k2 k3
	})).Return(mock.Result(mock.ValkeyArray(
		mock.ValkeyBlobString("v1"),
		mock.ValkeyNil(),
		mock.ValkeyBlobString("v3"),
	)))

	result, err := b.GetMany(ctx, []string{"k1", "k2", "k3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if string(result["k1"]) != "v1" {
		t.Fatalf("expected k1=v1, got %q", string(result["k1"]))
	}
	if string(result["k3"]) != "v3" {
		t.Fatalf("expected k3=v3, got %q", string(result["k3"]))
	}
	if _, ok := result["k2"]; ok {
		t.Fatal("k2 should not be in result (cache miss)")
	}
}

// TestValkeyBackend_DeletePattern_BareStar verifies DeletePattern returns ErrPatternInvalid for bare "*".
func TestValkeyBackend_DeletePattern_BareStar(t *testing.T) {
	b, _ := newTestBackend(t)
	ctx := context.Background()

	err := b.DeletePattern(ctx, "*")
	if !errors.Is(err, ErrPatternInvalid) {
		t.Fatalf("expected ErrPatternInvalid, got: %v", err)
	}
}

// TestValkeyBackend_DeletePattern_ValidPattern verifies DeletePattern succeeds for a valid pattern.
func TestValkeyBackend_DeletePattern_ValidPattern(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// SCAN with pattern returns 2 keys then cursor 0 (done)
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return len(cmd) >= 2 && cmd[0] == "SCAN"
	})).Return(mock.Result(mock.ValkeyArray(
		mock.ValkeyInt64(0), // cursor 0 = done
		mock.ValkeyArray(
			mock.ValkeyBlobString("prefix:a"),
			mock.ValkeyBlobString("prefix:b"),
		),
	)))

	// DEL the matching keys
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return cmd[0] == "DEL"
	})).Return(mock.Result(mock.ValkeyInt64(2)))

	err := b.DeletePattern(ctx, "prefix:*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_Close verifies the backend shuts down cleanly.
func TestValkeyBackend_Close(t *testing.T) {
	b, client := newTestBackend(t)

	client.EXPECT().Close()

	err := b.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_Get_BackendUnavailable verifies Get returns ErrBackendUnavailable on connection error.
func TestValkeyBackend_Get_BackendUnavailable(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.Match("GET", "key")).Return(mock.ErrorResult(errors.New("connection refused")))

	_, err := b.Get(ctx, "key")
	var ce *CacheError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *CacheError, got: %v", err)
	}
	if ce.Code != ErrCodeBackendUnavailable {
		t.Fatalf("expected code %q, got %q", ErrCodeBackendUnavailable, ce.Code)
	}
}

// TestValkeyBackend_Exists_True verifies Exists returns true when key exists.
func TestValkeyBackend_Exists_True(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.Match("EXISTS", "key")).Return(mock.Result(mock.ValkeyInt64(1)))

	exists, err := b.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
}

// TestValkeyBackend_Exists_False verifies Exists returns false when key does not exist.
func TestValkeyBackend_Exists_False(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.Match("EXISTS", "key")).Return(mock.Result(mock.ValkeyInt64(0)))

	exists, err := b.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false")
	}
}

// TestValkeyBackend_TTL_NoExpiry verifies TTL returns -1 for keys with no expiration.
func TestValkeyBackend_TTL_NoExpiry(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.Match("TTL", "key")).Return(mock.Result(mock.ValkeyInt64(-1)))

	ttl, err := b.TTL(ctx, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl != -1*time.Second {
		t.Fatalf("expected -1s, got %v", ttl)
	}
}

// TestValkeyBackend_TTL_KeyNotFound verifies TTL returns -2 for non-existent keys.
func TestValkeyBackend_TTL_KeyNotFound(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.Match("TTL", "missing")).Return(mock.Result(mock.ValkeyInt64(-2)))

	ttl, err := b.TTL(ctx, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl != -2*time.Second {
		t.Fatalf("expected -2s, got %v", ttl)
	}
}

// TestValkeyBackend_DeleteMany verifies DeleteMany sends DEL with multiple keys.
func TestValkeyBackend_DeleteMany(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		if cmd[0] != "DEL" {
			return false
		}
		// Check all expected keys are present
		keys := cmd[1:]
		hasK1, hasK2, hasK3 := false, false, false
		for _, k := range keys {
			switch k {
			case "k1":
				hasK1 = true
			case "k2":
				hasK2 = true
			case "k3":
				hasK3 = true
			}
		}
		return hasK1 && hasK2 && hasK3 && len(keys) == 3
	})).Return(mock.Result(mock.ValkeyInt64(3)))

	err := b.DeleteMany(ctx, []string{"k1", "k2", "k3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_SetMany verifies SetMany sends multiple SET commands via DoMulti.
func TestValkeyBackend_SetMany(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().DoMulti(ctx, gomock.Any(), gomock.Any()).Return([]valkey.ValkeyResult{
		mock.Result(mock.ValkeyString("OK")),
		mock.Result(mock.ValkeyString("OK")),
	})

	entries := map[string][]byte{
		"k1": []byte("v1"),
		"k2": []byte("v2"),
	}
	err := b.SetMany(ctx, entries, 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_DeleteByTag_EmptyTag verifies DeleteByTag handles missing tags gracefully.
func TestValkeyBackend_DeleteByTag_EmptyTag(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// SMEMBERS returns nil (tag doesn't exist)
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return len(cmd) >= 2 && cmd[0] == "SMEMBERS" && strings.HasPrefix(cmd[1], "_tags:")
	})).Return(mock.Result(mock.ValkeyNil()))

	err := b.DeleteByTag(ctx, "_tags:ns:agent:user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_Set_Success verifies Set without TTL sends SET with no expiration.
func TestValkeyBackend_Set_Success(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return len(cmd) >= 2 && cmd[0] == "SET" && cmd[1] == "noexpire"
	})).Return(mock.Result(mock.ValkeyString("OK")))

	err := b.Set(ctx, "noexpire", []byte("value"), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_Delete_NotExists verifies Delete is idempotent for a missing key.
func TestValkeyBackend_Delete_NotExists(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// DEL returns 0 (key didn't exist) — still no error
	client.EXPECT().Do(ctx, mock.Match("DEL", "ghost")).Return(mock.Result(mock.ValkeyInt64(0)))

	err := b.Delete(ctx, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_GetMany_AllHits verifies GetMany returns all keys when every key is present.
func TestValkeyBackend_GetMany_AllHits(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return cmd[0] == "MGET" && len(cmd) == 3
	})).Return(mock.Result(mock.ValkeyArray(
		mock.ValkeyBlobString("a"),
		mock.ValkeyBlobString("b"),
	)))

	result, err := b.GetMany(ctx, []string{"k1", "k2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if string(result["k1"]) != "a" {
		t.Fatalf("expected k1=a, got %q", string(result["k1"]))
	}
	if string(result["k2"]) != "b" {
		t.Fatalf("expected k2=b, got %q", string(result["k2"]))
	}
}

// TestValkeyBackend_GetMany_Empty verifies GetMany returns empty map for empty keys slice.
func TestValkeyBackend_GetMany_Empty(t *testing.T) {
	b, _ := newTestBackend(t)
	ctx := context.Background()

	result, err := b.GetMany(ctx, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(result))
	}
}

// TestValkeyBackend_SetMany_BatchesOver100 verifies SetMany batches commands in groups of 100.
func TestValkeyBackend_SetMany_BatchesOver100(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// 150 entries → 2 DoMulti calls: 100 + 50
	okResults100 := make([]valkey.ValkeyResult, 100)
	for i := range okResults100 {
		okResults100[i] = mock.Result(mock.ValkeyString("OK"))
	}
	okResults50 := make([]valkey.ValkeyResult, 50)
	for i := range okResults50 {
		okResults50[i] = mock.Result(mock.ValkeyString("OK"))
	}
	client.EXPECT().DoMulti(ctx, gomock.Len(100)).Return(okResults100)
	client.EXPECT().DoMulti(ctx, gomock.Len(50)).Return(okResults50)

	entries := make(map[string][]byte, 150)
	for i := 0; i < 150; i++ {
		entries[fmt.Sprintf("key%d", i)] = []byte("val")
	}
	err := b.SetMany(ctx, entries, 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_DeletePattern_EmptyPattern verifies DeletePattern returns ErrPatternInvalid for empty string.
func TestValkeyBackend_DeletePattern_EmptyPattern(t *testing.T) {
	b, _ := newTestBackend(t)
	ctx := context.Background()

	err := b.DeletePattern(ctx, "")
	if !errors.Is(err, ErrPatternInvalid) {
		t.Fatalf("expected ErrPatternInvalid, got: %v", err)
	}
}

// TestValkeyBackend_DeleteByTag_TagNotExists verifies DeleteByTag returns nil when tag doesn't exist.
func TestValkeyBackend_DeleteByTag_TagNotExists(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	// SMEMBERS returns nil for non-existent tag
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return len(cmd) >= 2 && cmd[0] == "SMEMBERS" && strings.HasPrefix(cmd[1], "_tags:")
	})).Return(mock.Result(mock.ValkeyNil()))

	err := b.DeleteByTag(ctx, "_tags:ns:agent:nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValkeyBackend_DeleteByTag_WithKeys verifies DeleteByTag deletes tag members and tag set.
func TestValkeyBackend_DeleteByTag_WithKeys(t *testing.T) {
	b, client := newTestBackend(t)
	ctx := context.Background()

	tagKey := "_tags:ns:agent:user"

	// SMEMBERS returns keys associated with the tag
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return len(cmd) >= 2 && cmd[0] == "SMEMBERS" && cmd[1] == tagKey
	})).Return(mock.Result(mock.ValkeyArray(
		mock.ValkeyBlobString("cache:key1"),
		mock.ValkeyBlobString("cache:key2"),
	)))

	// DEL the tag members
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return cmd[0] == "DEL"
	})).Return(mock.Result(mock.ValkeyInt64(2)))

	// DEL the _keytags: reverse index for each key
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return cmd[0] == "DEL"
	})).Return(mock.Result(mock.ValkeyInt64(1)))

	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return cmd[0] == "DEL"
	})).Return(mock.Result(mock.ValkeyInt64(1)))

	// DEL the tag set itself
	client.EXPECT().Do(ctx, mock.MatchFn(func(cmd []string) bool {
		return cmd[0] == "DEL"
	})).Return(mock.Result(mock.ValkeyInt64(1)))

	err := b.DeleteByTag(ctx, tagKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
