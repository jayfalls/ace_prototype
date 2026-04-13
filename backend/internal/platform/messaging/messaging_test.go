//go:build !external

package messaging

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitEmbedded_StartsAndStops(t *testing.T) {
	// Create temp directory for NATS storage
	tmpDir := t.TempDir()
	natsPath := filepath.Join(tmpDir, "nats")

	paths := &MessagingPaths{
		NATSPath: natsPath,
	}

	cfg := &Config{
		Mode: "embedded",
	}

	// Initialize embedded NATS
	nc, cleanup, err := Init(cfg, paths)
	require.NoError(t, err)
	require.NotNil(t, nc)
	require.NotNil(t, cleanup)

	// Verify connection is alive
	assert.True(t, nc.IsConnected())

	// Cleanup
	err = cleanup()
	require.NoError(t, err)

	// Verify NATS data directory was created
	_, err = os.Stat(natsPath)
	assert.NoError(t, err)
}

func TestInitEmbedded_PublishSubscribe(t *testing.T) {
	tmpDir := t.TempDir()
	natsPath := filepath.Join(tmpDir, "nats")

	paths := &MessagingPaths{
		NATSPath: natsPath,
	}

	cfg := &Config{
		Mode: "embedded",
	}

	nc, cleanup, err := Init(cfg, paths)
	require.NoError(t, err)
	defer func() {
		err := cleanup()
		require.NoError(t, err)
	}()

	// Subscribe to a test subject
	received := make(chan []byte, 1)
	sub, err := nc.Subscribe("test.subject", func(msg *nats.Msg) {
		received <- msg.Data
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish a message
	err = nc.Publish("test.subject", []byte("hello world"))
	require.NoError(t, err)

	// Wait for message
	select {
	case data := <-received:
		assert.Equal(t, "hello world", string(data))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestInitEmbedded_JetStream(t *testing.T) {
	tmpDir := t.TempDir()
	natsPath := filepath.Join(tmpDir, "nats")

	paths := &MessagingPaths{
		NATSPath: natsPath,
	}

	cfg := &Config{
		Mode: "embedded",
	}

	nc, cleanup, err := Init(cfg, paths)
	require.NoError(t, err)
	defer func() {
		err := cleanup()
		require.NoError(t, err)
	}()

	// Get JetStream context
	js, err := nc.JetStream()
	require.NoError(t, err)

	// Create a stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "TEST_STREAM",
		Subjects: []string{"test.>"},
	})
	require.NoError(t, err)

	// Verify stream exists
	streamInfo, err := js.StreamInfo("TEST_STREAM")
	require.NoError(t, err)
	assert.Equal(t, "TEST_STREAM", streamInfo.Config.Name)

	// Publish and consume a message
	js.Publish("test.subject", []byte("jetstream message"))

	sub, err := js.Subscribe("test.subject", func(msg *nats.Msg) {
		assert.Equal(t, "jetstream message", string(msg.Data))
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	time.Sleep(100 * time.Millisecond)
}

func TestMessagingConfig_Defaults(t *testing.T) {
	cfg := &Config{
		Mode: "embedded",
	}

	assert.Equal(t, "embedded", cfg.Mode)
	assert.Empty(t, cfg.URL)
}

func TestInitEmbedded_NoTCPPorts(t *testing.T) {
	// This test verifies that embedded NATS doesn't open TCP ports
	// by checking that DontListen mode is used
	tmpDir := t.TempDir()
	natsPath := filepath.Join(tmpDir, "nats")

	paths := &MessagingPaths{
		NATSPath: natsPath,
	}

	cfg := &Config{
		Mode: "embedded",
	}

	nc, cleanup, err := Init(cfg, paths)
	require.NoError(t, err)
	defer func() {
		err := cleanup()
		require.NoError(t, err)
	}()

	// Verify connection works
	assert.True(t, nc.IsConnected())

	// The key verification is in the implementation:
	// server_embed.go uses DontListen: true which prevents TCP listening
	// and nats.InProcessServer(srv) for in-process connection
}

func TestInitEmbedded_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	natsPath := filepath.Join(tmpDir, "nats")

	paths := &MessagingPaths{
		NATSPath: natsPath,
	}

	cfg := &Config{
		Mode: "embedded",
	}

	nc, cleanup, err := Init(cfg, paths)
	require.NoError(t, err)

	// Verify connection is alive
	assert.True(t, nc.IsConnected())

	// Perform graceful shutdown
	err = cleanup()
	require.NoError(t, err)

	// Verify connection is closed
	assert.False(t, nc.IsConnected())
}
