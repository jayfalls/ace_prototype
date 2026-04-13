//go:build !external

// Package messaging provides NATS messaging infrastructure initialization.
package messaging

import (
	"fmt"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// initEmbedded starts an embedded NATS server with JetStream enabled.
func initEmbedded(paths *MessagingPaths) (*nats.Conn, func() error, error) {
	// Create the embedded NATS server
	srv, err := startEmbeddedNATS(paths)
	if err != nil {
		return nil, nil, fmt.Errorf("messaging: start embedded NATS: %w", err)
	}

	// Wait for server to be ready
	if !srv.ReadyForConnections(5 * time.Second) {
		srv.Shutdown()
		return nil, nil, fmt.Errorf("messaging: NATS server not ready")
	}

	// Connect to the embedded server using in-process connection
	// This uses srv.InProcessConn() which creates a loopback connection
	// without opening any TCP ports (DontListen: true)
	nc, err := nats.Connect(srv.ClientURL(), nats.InProcessServer(srv))
	if err != nil {
		srv.Shutdown()
		return nil, nil, fmt.Errorf("messaging: connect to embedded NATS: %w", err)
	}

	// Create cleanup function
	cleanup := func() error {
		// Drain the client connection first
		if nc != nil {
			_ = nc.Drain()
			nc.Close()
		}
		// Shutdown the server
		srv.Shutdown()
		return nil
	}

	return nc, cleanup, nil
}

// startEmbeddedNATS creates and configures an embedded NATS server.
func startEmbeddedNATS(paths *MessagingPaths) (*server.Server, error) {
	opts := &server.Options{
		DontListen: true, // No TCP ports - in-process only
		JetStream:  true,
		StoreDir:   paths.NATSPath,
	}

	srv, err := server.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("create NATS server: %w", err)
	}

	// Start the server in a goroutine
	go srv.Start()

	return srv, nil
}
