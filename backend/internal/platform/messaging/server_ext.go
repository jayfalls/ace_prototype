//go:build external

// Package messaging provides NATS messaging infrastructure initialization.
package messaging

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// initExternal connects to an external NATS server.
func initExternal(url string) (*nats.Conn, func() error, error) {
	if url == "" {
		return nil, nil, fmt.Errorf("messaging: URL is required for external mode")
	}

	nc, err := nats.Connect(url,
		nats.Name("ace-app"),
		nats.Timeout(10*time.Second),
		nats.MaxReconnects(3),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("messaging: connect to NATS: %w", err)
	}

	cleanup := func() error {
		if nc != nil {
			_ = nc.Drain()
			nc.Close()
		}
		return nil
	}

	return nc, cleanup, nil
}
