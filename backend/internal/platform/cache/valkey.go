//go:build external

// Package cache provides external Valkey-based cache backend.
package cache

import (
	"fmt"

	"ace/internal/caching"
)

// InitExternal creates a new external Valkey cache backend.
func InitExternal(cfg *Config) (caching.CacheBackend, error) {
	// Valkey backend is imported and used directly from caching package
	return nil, fmt.Errorf("cache: external Valkey mode not yet implemented")
}
