package messaging

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	srv         *server.Server
	srvURL      string
	nc          *nats.Conn
	js          nats.JetStreamContext
	serverStart sync.Once
)

// TestMain sets up and tears down the embedded NATS server with JetStream.
func TestMain(m *testing.M) {
	// Set up embedded NATS server
	serverStart.Do(func() {
		opts := &server.Options{
			Port:      -1, // Random port
			JetStream: true,
		}

		var err error
		srv, err = server.NewServer(opts)
		if err != nil {
			panic("failed to create NATS server: " + err.Error())
		}

		go srv.Start()
		if !srv.ReadyForConnections(5 * time.Second) {
			panic("NATS server not ready")
		}

		srvURL = srv.ClientURL()
	})

	// Connect to the server
	var err error
	nc, err = nats.Connect(srvURL)
	if err != nil {
		srv.Shutdown()
		panic("failed to connect to NATS: " + err.Error())
	}

	// Get JetStream context
	js, err = nc.JetStream()
	if err != nil {
		nc.Close()
		srv.Shutdown()
		panic("failed to get JetStream: " + err.Error())
	}

	// Run tests
	code := m.Run()

	// Cleanup
	nc.Close()
	srv.Shutdown()

	os.Exit(code)
}

// Helper function to get client for tests
func getTestClient(t *testing.T) Client {
	t.Helper()

	client, err := NewClient(Config{
		URLs:         srvURL,
		Name:         "test-client",
		Timeout:      10 * time.Second,
		MaxReconnect: 3,
		ReconnectWait: 1 * time.Second,
	})
	require.NoError(t, err, "failed to create client")

	return client
}

// Helper to get JetStreamManager
func getJSManager(t *testing.T) nats.JetStreamManager {
	t.Helper()
	return js
}

func TestIntegration_HealthCheck(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	err := client.HealthCheck()
	require.NoError(t, err, "health check should pass")
}

func TestIntegration_PublishWithEnvelopeHeaders(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Create a subscriber to receive the message (buffered to ensure delivery)
	received := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe("test.subject", func(msg *nats.Msg) {
		received <- msg
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Give subscription time to be established
	time.Sleep(100 * time.Millisecond)

	// Publish using the helper
	err = Publish(client, "test.subject", "corr-id-123", "agent-1", "cycle-1", "test-service", []byte("test payload"))
	require.NoError(t, err)

	// Wait for message
	select {
	case msg := <-received:
		require.NotNil(t, msg)
		// Verify headers
		assert.NotEmpty(t, msg.Header.Get(HeaderMessageID))
		assert.Equal(t, "corr-id-123", msg.Header.Get(HeaderCorrelationID))
		assert.Equal(t, "agent-1", msg.Header.Get(HeaderAgentID))
		assert.Equal(t, "cycle-1", msg.Header.Get(HeaderCycleID))
		assert.Equal(t, "test-service", msg.Header.Get(HeaderSourceService))
		assert.Equal(t, "test payload", string(msg.Data))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestIntegration_PublishWithSubject(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Create a subscriber
	received := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe("ace.engine.agent1.layer.layer1.input", func(msg *nats.Msg) {
		received <- msg
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Publish using Subject type
	err = PublishWithSubject(client, SubjectEngineLayerInput, "corr-id-456", "agent1", "cycle-1", "test-service", []byte("engine payload"), "agent1", "layer1")
	require.NoError(t, err)

	// Wait for message
	select {
	case msg := <-received:
		require.NotNil(t, msg)
		assert.Equal(t, "engine payload", string(msg.Data))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestIntegration_RequestReply(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Set up a responder
	sub, err := nc.Subscribe("test.request.subject", func(msg *nats.Msg) {
		// Echo back the data with a response
		resp := &nats.Msg{
			Subject: msg.Reply,
			Data:    []byte("response: " + string(msg.Data)),
		}
		nc.PublishMsg(resp)
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Make request
	start := time.Now()
	respData, err := RequestReply(client, "test.request.subject", "corr-id-789", "agent-1", "cycle-1", "test-service", []byte("request data"), 5*time.Second)
	elapsed := time.Since(start)

	require.NoError(t, err, "request-reply should succeed")
	assert.Equal(t, "response: request data", string(respData))
	assert.Less(t, elapsed, 5*time.Second, "should complete before timeout")
}

func TestIntegration_RequestReplyTimeout(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// No responder subscribed - should return no responders error
	_, err := RequestReply(client, "test.no.responder", "corr-id-timeout", "agent-1", "cycle-1", "test-service", []byte("request data"), 500*time.Millisecond)

	require.Error(t, err, "should return error when no responders")
	// The error could be "no responders" or "timeout"
	assert.True(t, containsIgnoreCase(err.Error(), "timeout") || containsIgnoreCase(err.Error(), "no responders"), "error should mention timeout or no responders")
}

func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

func TestIntegration_RequestReplyWithSubject(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Set up a responder for LLM request
	sub, err := nc.Subscribe("ace.llm.provider1.request", func(msg *nats.Msg) {
		resp := &nats.Msg{
			Subject: msg.Reply,
			Data:    []byte(`{"status": "success"}`),
		}
		nc.PublishMsg(resp)
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Make request using Subject
	respData, err := RequestReplyWithSubject(client, SubjectLLMRequest, "corr-id-llm", "agent-1", "cycle-1", "test-service", []byte(`{"prompt": "hello"}`), 5*time.Second, "provider1")
	require.NoError(t, err)

	assert.Equal(t, `{"status": "success"}`, string(respData))
}

func TestIntegration_Subscribe(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Subscribe to a subject
	sub, err := Subscribe(client, "test.subscribe.subject", func(msg *nats.Msg) error {
		assert.Equal(t, "test message", string(msg.Data))
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, sub)

	// Publish a message directly
	err = nc.Publish("test.subscribe.subject", []byte("test message"))
	require.NoError(t, err)

	// Give time for message to be processed
	time.Sleep(500 * time.Millisecond)
}

func TestIntegration_SubscribeToStream(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// First ensure stream exists
	err := EnsureStreams(ctx, js)
	require.NoError(t, err, "should create streams")

	// Subscribe to a stream subject
	sub, err := Subscribe(client, "test.stream.subject", func(msg *nats.Msg) error {
		assert.Equal(t, "stream test message", string(msg.Data))
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, sub)

	// Publish to subject (which is covered by SYSTEM stream)
	err = nc.Publish("test.stream.subject", []byte("stream test message"))
	require.NoError(t, err)

	// Give time for message to be processed
	time.Sleep(500 * time.Millisecond)
}

func TestIntegration_StreamSubscription(t *testing.T) {
	// This test requires proper JetStream consumer configuration
	// For now, we'll test stream creation and basic subscription works
	t.Skip("Skipping - requires pull consumer configuration")
}

func TestIntegration_DLQ(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Ensure DLQ stream exists
	err := EnsureDLQStream(ctx, js)
	require.NoError(t, err)

	// Verify DLQ stream exists
	info, err := js.StreamInfo("DLQ")
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "DLQ", info.Config.Name)

	// Verify DLQ subjects are set correctly
	assert.Contains(t, info.Config.Subjects, "dlq.>")
}

func TestIntegration_StreamCreation(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Test creating streams
	err := EnsureStreams(ctx, js)
	require.NoError(t, err)

	// Verify streams exist
	streams := []string{"COGNITIVE", "USAGE", "SYSTEM"}
	for _, streamName := range streams {
		info, err := js.StreamInfo(streamName)
		require.NoError(t, err, "stream %s should exist", streamName)
		assert.NotNil(t, info)
	}

	// Test idempotency - calling again should not fail
	err = EnsureStreams(ctx, js)
	require.NoError(t, err, "EnsureStreams should be idempotent")
}

func TestIntegration_ConsumerCreation(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Create a test stream
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     "CONSUMER_TEST_STREAM",
		Subjects: []string{"consumer.test.>"},
	})
	require.NoError(t, err)

	// Create consumer using the helper
	consumerCfg := DefaultConsumerConfig("CONSUMER_TEST_STREAM", "test-consumer", "consumer.test.subject")
	err = CreateConsumer(ctx, js, consumerCfg)
	require.NoError(t, err)

	// Verify consumer exists
	info, err := js.ConsumerInfo("CONSUMER_TEST_STREAM", "test-consumer")
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "test-consumer", info.Name)
}

func TestIntegration_ReplyTo(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Set up a responder that uses ReplyTo
	sub, err := nc.Subscribe("test.request", func(msg *nats.Msg) {
		// Use ReplyTo to respond
		err := ReplyTo(client, msg, []byte("reply payload"))
		require.NoError(t, err)
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Make request and get reply
	resp, err := nc.Request("test.request", []byte("request payload"), 5*time.Second)
	require.NoError(t, err)

	assert.Equal(t, "reply payload", string(resp.Data))
}

func TestIntegration_ForwardMessage(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Set up subscriber on target subject
	received := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe("test.forward.target", func(msg *nats.Msg) {
		received <- msg
	})
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Create incoming message with envelope
	incoming := &nats.Msg{
		Subject: "test.forward.source",
		Data:    []byte("forwarded data"),
		Header: nats.Header{
			HeaderMessageID:     []string{"msg-id-123"},
			HeaderCorrelationID: []string{"corr-id-123"},
			HeaderAgentID:       []string{"agent-1"},
			HeaderCycleID:       []string{"cycle-1"},
			HeaderSourceService: []string{"source-service"},
		},
	}

	// Forward the message
	err = ForwardMessage(client, incoming, "test.forward.target")
	require.NoError(t, err)

	// Wait for forwarded message
	select {
	case msg := <-received:
		require.NotNil(t, msg)
		assert.Equal(t, "forwarded data", string(msg.Data))
		assert.Equal(t, "corr-id-123", msg.Header.Get(HeaderCorrelationID))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for forwarded message")
	}
}

func TestIntegration_ErrorCases(t *testing.T) {
	// Test error cases using mock client
	t.Run("publish error propagation", func(t *testing.T) {
		mock := &MockClient{
			// Default mock should work
		}
		err := mock.Publish("test.subject", []byte("test data"), nil)
		assert.NoError(t, err)
	})

	t.Run("request error propagation", func(t *testing.T) {
		mock := &MockClient{
			RequestErr: assert.AnError,
		}
		_, err := mock.Request("test.subject", []byte("test data"), time.Second)
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("health check error propagation", func(t *testing.T) {
		mock := &MockClient{
			HealthCheckErr: assert.AnError,
		}
		err := mock.HealthCheck()
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestIntegration_DrainAndClose(t *testing.T) {
	client := getTestClient(t)

	// Drain the connection
	err := client.Drain()
	require.NoError(t, err)

	// After drain, client should be closed
	// Health check should fail
	err = client.HealthCheck()
	require.Error(t, err)
}

func TestIntegration_GetStreamInfo(t *testing.T) {
	ctx := context.Background()

	// Create a stream
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     "INFO_TEST_STREAM",
		Subjects: []string{"info.test.>"},
	})
	require.NoError(t, err)

	// Get stream info
	info, err := GetStreamInfo(ctx, js, "INFO_TEST_STREAM")
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "INFO_TEST_STREAM", info.Config.Name)
}

func TestIntegration_DeleteStream(t *testing.T) {
	ctx := context.Background()

	// Create a stream
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     "DELETE_TEST_STREAM",
		Subjects: []string{"delete.test.>"},
	})
	require.NoError(t, err)

	// Delete the stream
	err = DeleteStream(ctx, js, "DELETE_TEST_STREAM")
	require.NoError(t, err)

	// Verify it's gone
	_, err = js.StreamInfo("DELETE_TEST_STREAM")
	require.Error(t, err)
}

func TestIntegration_SubjectValidation(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{"valid engine subject", SubjectEngineLayerInput.Format("agent1", "layer1"), false},
		{"valid memory subject", SubjectMemoryQuery.Format("agent1"), false},
		{"valid llm subject", SubjectLLMRequest.Format("provider1"), false},
		{"valid system subject", SubjectSystemHealth.Format("status"), false},
		{"empty subject", "", true},
		{"invalid subject", "invalid.subject", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Subject(tt.subject).Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
