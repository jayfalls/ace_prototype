// Package cache provides cache backend initialization for the ACE application.
package cache

import (
	"fmt"

	"ace/internal/caching"
)

// Config holds the configuration for the cache backend.
type Config struct {
	// Mode is the cache mode: "embedded" or "external".
	Mode string
	// URL is the cache server URL (for external mode).
	URL string
	// MaxCost is the maximum cost for the in-process cache (bytes).
	MaxCost int64
	// BufferItems is the number of items to buffer per write.
	BufferItems int64
}

// Init initializes the cache backend based on configuration.
// It returns the cache backend and any error encountered.
func Init(cfg *Config) (caching.CacheBackend, error) {
	switch cfg.Mode {
	case "embedded":
		return InitInProcess(cfg)
	case "external":
		return InitExternal(cfg)
	default:
		return nil, fmt.Errorf("cache: invalid mode: %q (must be \"embedded\" or \"external\")", cfg.Mode)
	}
}

// InitExternal is a stub for non-external build.
// The actual implementation is in valkey.go with //go:build external tag.
func InitExternal(cfg *Config) (caching.CacheBackend, error) {
	return nil, fmt.Errorf("cache: external mode not available (build with -tags external)")
}
