package caching

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/valkey-io/valkey-go"
)

// valkeyBackend implements CacheBackend using a Valkey client.
type valkeyBackend struct {
	client valkey.Client
}

// Compile-time interface check.
var _ CacheBackend = (*valkeyBackend)(nil)

// newValkeyBackend creates a new Valkey cache backend with connection defaults applied.
// Defaults: MaxRetries=3, DialTimeout=5s, ReadTimeout=3s, WriteTimeout=3s, PoolSize=100.
// When cfg.URL is set, it is parsed via valkey.ParseURL and fields like Addr/Password/DB are overridden.
func newValkeyBackend(cfg ValkeyConfig) (CacheBackend, error) {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 3 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 3 * time.Second
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 100
	}

	var opt valkey.ClientOption

	if cfg.URL != "" {
		parsed, err := valkey.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid URL: %v", ErrBackendUnavailable, err)
		}
		opt = parsed
	} else {
		opt.InitAddress = []string{cfg.Addr}
		opt.Password = cfg.Password
		opt.SelectDB = cfg.DB
	}

	// Apply timeouts and pool size over parsed/manual options.
	opt.Dialer = net.Dialer{
		Timeout: cfg.DialTimeout,
	}
	opt.ConnWriteTimeout = cfg.WriteTimeout
	opt.BlockingPoolSize = cfg.PoolSize

	client, err := valkey.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackendUnavailable, err)
	}

	return &valkeyBackend{client: client}, nil
}

// Get retrieves a value by key. Returns ErrCacheMiss on nil, ErrBackendUnavailable on connection error.
func (b *valkeyBackend) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := b.client.Do(ctx, b.client.B().Get().Key(key).Build()).AsBytes()
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, BackendUnavailableError(err)
	}
	return resp, nil
}

// Set stores a value with a TTL. Returns ErrBackendUnavailable on error.
func (b *valkeyBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var cmd valkey.Completed
	if ttl > 0 {
		ms := ttl.Milliseconds()
		if ms%1000 == 0 {
			cmd = b.client.B().Set().Key(key).Value(valkey.BinaryString(value)).Ex(time.Duration(ms/1000) * time.Second).Build()
		} else {
			cmd = b.client.B().Set().Key(key).Value(valkey.BinaryString(value)).Px(ttl).Build()
		}
	} else {
		cmd = b.client.B().Set().Key(key).Value(valkey.BinaryString(value)).Build()
	}
	if err := b.client.Do(ctx, cmd).Error(); err != nil {
		return BackendUnavailableError(err)
	}
	return nil
}

// Delete removes a key. Idempotent — no error on missing key.
func (b *valkeyBackend) Delete(ctx context.Context, key string) error {
	if err := b.client.Do(ctx, b.client.B().Del().Key(key).Build()).Error(); err != nil {
		return BackendUnavailableError(err)
	}
	return nil
}

// GetMany retrieves multiple keys. Omit nil (miss) entries from the result map.
func (b *valkeyBackend) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return map[string][]byte{}, nil
	}

	result := b.client.Do(ctx, b.client.B().Mget().Key(keys...).Build())
	arr, err := result.ToArray()
	if err != nil {
		return nil, BackendUnavailableError(err)
	}

	ret := make(map[string][]byte, len(keys))
	for i, msg := range arr {
		if i >= len(keys) {
			break
		}
		if msg.IsNil() {
			continue
		}
		val, err := msg.AsBytes()
		if err != nil {
			continue
		}
		ret[keys[i]] = val
	}
	return ret, nil
}

// SetMany stores multiple entries. Uses DoMulti, batched in groups of 100.
func (b *valkeyBackend) SetMany(ctx context.Context, entries map[string][]byte, ttl time.Duration) error {
	if len(entries) == 0 {
		return nil
	}

	// Build all commands
	cmds := make([]valkey.Completed, 0, len(entries))
	for key, value := range entries {
		var cmd valkey.Completed
		if ttl > 0 {
			ms := ttl.Milliseconds()
			if ms%1000 == 0 {
				cmd = b.client.B().Set().Key(key).Value(valkey.BinaryString(value)).Ex(time.Duration(ms/1000) * time.Second).Build()
			} else {
				cmd = b.client.B().Set().Key(key).Value(valkey.BinaryString(value)).Px(ttl).Build()
			}
		} else {
			cmd = b.client.B().Set().Key(key).Value(valkey.BinaryString(value)).Build()
		}
		cmds = append(cmds, cmd)
	}

	// Batch in groups of DefaultBatchSize
	for i := 0; i < len(cmds); i += DefaultBatchSize {
		end := i + DefaultBatchSize
		if end > len(cmds) {
			end = len(cmds)
		}
		results := b.client.DoMulti(ctx, cmds[i:end]...)
		for _, r := range results {
			if err := r.Error(); err != nil {
				return BackendUnavailableError(err)
			}
		}
	}
	return nil
}

// DeleteMany removes multiple keys using DEL with multiple keys.
func (b *valkeyBackend) DeleteMany(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := b.client.Do(ctx, b.client.B().Del().Key(keys...).Build()).Error(); err != nil {
		return BackendUnavailableError(err)
	}
	return nil
}

// DeletePattern removes all keys matching a pattern using SCAN loop (batch size 100).
// Rejects bare "*" and empty pattern "" with ErrPatternInvalid.
func (b *valkeyBackend) DeletePattern(ctx context.Context, pattern string) error {
	if pattern == "" || pattern == "*" {
		return ErrPatternInvalid
	}

	var cursor uint64
	for {
		scanCmd := b.client.B().Scan().Cursor(cursor).Match(pattern).Count(100).Build()
		entry, err := b.client.Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return BackendUnavailableError(err)
		}

		if len(entry.Elements) > 0 {
			delCmd := b.client.B().Del().Key(entry.Elements...).Build()
			if err := b.client.Do(ctx, delCmd).Error(); err != nil {
				return BackendUnavailableError(err)
			}
		}

		cursor = entry.Cursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// DeleteByTag reads the Valkey set at _tags:{tag}, deletes all members and the set.
func (b *valkeyBackend) DeleteByTag(ctx context.Context, tag string) error {
	tagKey := "_tags:{" + tag + "}"

	// Get all keys associated with this tag
	keys, err := b.client.Do(ctx, b.client.B().Smembers().Key(tagKey).Build()).AsStrSlice()
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil // Tag doesn't exist, nothing to delete
		}
		return BackendUnavailableError(err)
	}

	// Delete all keys in batches of 100
	for i := 0; i < len(keys); i += DefaultBatchSize {
		end := i + DefaultBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]
		if len(batch) > 0 {
			if err := b.client.Do(ctx, b.client.B().Del().Key(batch...).Build()).Error(); err != nil {
				return BackendUnavailableError(err)
			}
		}
	}

	// Delete the tag set itself
	if err := b.client.Do(ctx, b.client.B().Del().Key(tagKey).Build()).Error(); err != nil {
		return BackendUnavailableError(err)
	}

	return nil
}

// SAdd adds members to a set with an optional TTL. Returns ErrBackendUnavailable on error.
func (b *valkeyBackend) SAdd(ctx context.Context, key string, members []string, ttl time.Duration) error {
	if len(members) == 0 {
		return nil
	}

	saddCmd := b.client.B().Sadd().Key(key).Member(members...).Build()

	if ttl > 0 {
		expireCmd := b.client.B().Expire().Key(key).Seconds(int64(ttl.Seconds())).Build()
		results := b.client.DoMulti(ctx, saddCmd, expireCmd)
		for _, r := range results {
			if err := r.Error(); err != nil {
				return BackendUnavailableError(err)
			}
		}
		return nil
	}

	if err := b.client.Do(ctx, saddCmd).Error(); err != nil {
		return BackendUnavailableError(err)
	}
	return nil
}

// SMembers returns all members of a set. Returns empty slice on missing key.
func (b *valkeyBackend) SMembers(ctx context.Context, key string) ([]string, error) {
	members, err := b.client.Do(ctx, b.client.B().Smembers().Key(key).Build()).AsStrSlice()
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return []string{}, nil
		}
		return nil, BackendUnavailableError(err)
	}
	return members, nil
}

// SRem removes members from a set. Idempotent — no error on missing members.
func (b *valkeyBackend) SRem(ctx context.Context, key string, members []string) error {
	if len(members) == 0 {
		return nil
	}

	if err := b.client.Do(ctx, b.client.B().Srem().Key(key).Member(members...).Build()).Error(); err != nil {
		return BackendUnavailableError(err)
	}
	return nil
}

// Exists returns true if the key exists (result > 0).
func (b *valkeyBackend) Exists(ctx context.Context, key string) (bool, error) {
	n, err := b.client.Do(ctx, b.client.B().Exists().Key(key).Build()).AsInt64()
	if err != nil {
		return false, BackendUnavailableError(err)
	}
	return n > 0, nil
}

// TTL returns the remaining TTL. Returns -1 if no TTL, -2 if key doesn't exist.
func (b *valkeyBackend) TTL(ctx context.Context, key string) (time.Duration, error) {
	secs, err := b.client.Do(ctx, b.client.B().Ttl().Key(key).Build()).AsInt64()
	if err != nil {
		return 0, BackendUnavailableError(err)
	}
	return time.Duration(secs) * time.Second, nil
}

// Close shuts down the client connection.
func (b *valkeyBackend) Close() error {
	b.client.Close()
	return nil
}
