// Package messaging provides NATS messaging infrastructure initialization.
package messaging

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// MessagingPaths holds filesystem paths needed for messaging infrastructure.
type MessagingPaths struct {
	// NATSPath is the directory for NATS JetStream storage.
	NATSPath string
}

// Config holds messaging configuration.
type Config struct {
	// Mode is the messaging mode: "embedded" or "external".
	Mode string
	// URL is the NATS server URL (for external mode).
	URL string
}

// Init initializes the NATS connection based on configuration.
// It returns the connection, a cleanup function, and any error encountered.
func Init(cfg *Config, paths *MessagingPaths) (*nats.Conn, func() error, error) {
	switch cfg.Mode {
	case "embedded":
		return initEmbedded(paths)
	case "external":
		return initExternal(cfg.URL)
	default:
		return nil, nil, fmt.Errorf("messaging: invalid mode: %q (must be \"embedded\" or \"external\")", cfg.Mode)
	}
}

// initExternal is a stub for non-external build.
// The actual implementation is in server_ext.go with //go:build external tag.
func initExternal(url string) (*nats.Conn, func() error, error) {
	return nil, nil, fmt.Errorf("messaging: external mode not available (build with -tags external)")
}
