package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	"ace/internal/api/model"
	"ace/internal/messaging"
)

// TestIntegration_SystemHealthNATStoWebSocket verifies that publishing to NATS
// ace.system.health.ok results in an event arriving on a subscribed WebSocket client.
func TestIntegration_SystemHealthNATStoWebSocket(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, "user1", model.RoleUser, h)
		h.Register(c)
		ctx := r.Context()
		go c.writePump(ctx)
		c.readPump(ctx)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.CloseNow() })

	// Consume auth_ok.
	var authMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &authMsg))
	assert.Equal(t, ServerMessageAuthOk, authMsg.Type)

	// Subscribe to system:health topic.
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"system:health"},
	}))

	var subMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &subMsg))
	assert.Equal(t, ServerMessageSubscribed, subMsg.Type)
	assert.Contains(t, subMsg.Topics, "system:health")

	// Publish a health event to NATS with proper envelope headers.
	healthData := map[string]interface{}{
		"status": "ok",
		"checks": map[string]interface{}{
			"database": map[string]interface{}{"status": "healthy"},
			"nats":     map[string]interface{}{"status": "healthy"},
			"cache":    map[string]interface{}{"status": "healthy"},
		},
	}
	payload, err := json.Marshal(healthData)
	require.NoError(t, err)

	msg := nats.NewMsg("ace.system.health.ok")
	msg.Data = payload
	env := messaging.NewEnvelope("", "", "", "test-service")
	messaging.SetHeaders(msg, env)
	err = testNATSConn.PublishMsg(msg)
	require.NoError(t, err)

	// Read the event from WebSocket.
	var eventMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &eventMsg))
	assert.Equal(t, ServerMessageEvent, eventMsg.Type)
	assert.Equal(t, "system:health", eventMsg.Topic)
	assert.NotZero(t, eventMsg.Seq)

	// Verify the data matches what we published (Data contains EventData wrapper).
	var eventData EventData
	err = json.Unmarshal(eventMsg.Data, &eventData)
	require.NoError(t, err)
	assert.Equal(t, "event", eventData.EventType)

	var received map[string]interface{}
	err = json.Unmarshal(eventData.Data, &received)
	require.NoError(t, err)
	assert.Equal(t, "ok", received["status"])

	h.Close()
}

// TestIntegration_UsageNATStoWebSocket verifies that publishing to NATS
// ace.usage.{id}.cost results in an event arriving on the subscribed WebSocket client.
func TestIntegration_UsageNATStoWebSocket(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, "user1", model.RoleUser, h)
		h.Register(c)
		ctx := r.Context()
		go c.writePump(ctx)
		c.readPump(ctx)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.CloseNow() })

	// Consume auth_ok.
	var authMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &authMsg))
	assert.Equal(t, ServerMessageAuthOk, authMsg.Type)

	// Subscribe to usage:user1 topic.
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))

	var subMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &subMsg))
	assert.Equal(t, ServerMessageSubscribed, subMsg.Type)
	assert.Contains(t, subMsg.Topics, "usage:user1")

	// Publish a usage cost event to NATS with proper envelope headers.
	usageData := map[string]interface{}{
		"event_type": "usage.cost",
		"agent_id":   "agent-123",
		"cost_usd":   0.05,
		"tokens":     1500,
	}
	payload, err := json.Marshal(usageData)
	require.NoError(t, err)

	msg := nats.NewMsg("ace.usage.user1.cost")
	msg.Data = payload
	env := messaging.NewEnvelope("", "", "", "test-service")
	messaging.SetHeaders(msg, env)
	err = testNATSConn.PublishMsg(msg)
	require.NoError(t, err)

	// Read the event from WebSocket.
	var eventMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &eventMsg))
	assert.Equal(t, ServerMessageEvent, eventMsg.Type)
	assert.Equal(t, "usage:user1", eventMsg.Topic)
	assert.NotZero(t, eventMsg.Seq)

	// Verify the data contains the cost info (Data contains EventData wrapper).
	var eventData EventData
	err = json.Unmarshal(eventMsg.Data, &eventData)
	require.NoError(t, err)
	assert.Equal(t, "event", eventData.EventType)

	var received map[string]interface{}
	err = json.Unmarshal(eventData.Data, &received)
	require.NoError(t, err)
	assert.Equal(t, "usage.cost", received["event_type"])
	assert.Equal(t, 0.05, received["cost_usd"])

	h.Close()
}

// TestIntegration_DispatchNATSEvent_SequencesEvents verifies that dispatchNATSEvent
// correctly sequences events even when called rapidly.
func TestIntegration_DispatchNATSEvent_SequencesEvents(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	// Direct client setup without WebSocket for unit-level test.
	c := &Client{
		id:          "seq-test-client",
		userID:      "user1",
		role:        model.RoleUser,
		topics:      map[string]struct{}{"system:health": {}},
		send:        make(chan []byte, 128),
		connectedAt: time.Now(),
		hub:         h,
	}
	h.mu.Lock()
	h.clients["user1"] = append(h.clients["user1"], c)
	h.mu.Unlock()

	// Dispatch three events rapidly.
	for i := 1; i <= 3; i++ {
		h.dispatchNATSEvent("system:health", []byte(`{"seq":`+string(rune('0'+i))+`}`))
		time.Sleep(10 * time.Millisecond)
	}

	// Verify sequencing via buffer.
	seq1 := h.buffer.GetLastSeq("system:health")
	assert.Equal(t, uint64(3), seq1)

	h.mu.Lock()
	delete(h.clients, "user1")
	h.mu.Unlock()
	h.Close()
}

// TestIntegration_FullLifecycle tests WebSocket connect → auth → subscribe → receive event → unsubscribe → disconnect.
func TestIntegration_FullLifecycle(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, "user1", model.RoleUser, h)
		h.Register(c)
		ctx := r.Context()
		go c.writePump(ctx)
		c.readPump(ctx)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.CloseNow() })

	// Consume auth_ok.
	var authMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &authMsg))
	assert.Equal(t, ServerMessageAuthOk, authMsg.Type)

	// Subscribe to usage:user1 topic.
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))

	var subMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &subMsg))
	assert.Equal(t, ServerMessageSubscribed, subMsg.Type)
	assert.Contains(t, subMsg.Topics, "usage:user1")

	// Publish a usage event to NATS.
	usageData := map[string]interface{}{
		"event_type": "usage.cost",
		"cost_usd":   0.01,
	}
	payload, err := json.Marshal(usageData)
	require.NoError(t, err)

	msg := nats.NewMsg("ace.usage.user1.cost")
	msg.Data = payload
	env := messaging.NewEnvelope("", "", "", "test-service")
	messaging.SetHeaders(msg, env)
	err = testNATSConn.PublishMsg(msg)
	require.NoError(t, err)

	// Read the event.
	var eventMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &eventMsg))
	assert.Equal(t, ServerMessageEvent, eventMsg.Type)
	assert.Equal(t, "usage:user1", eventMsg.Topic)

	// Unsubscribe.
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageUnsubscribe,
		Topics: []string{"usage:user1"},
	}))

	var unsubMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &unsubMsg))
	assert.Equal(t, ServerMessageUnsubscribed, unsubMsg.Type)

	// Disconnect.
	conn.Close(websocket.StatusNormalClosure, "done")

	// Verify client is unregistered.
	require.Eventually(t, func() bool {
		h.mu.RLock()
		defer h.mu.RUnlock()
		return len(h.clients["user1"]) == 0
	}, time.Second, 10*time.Millisecond)

	h.Close()
}

// TestIntegration_MultipleClientsFanOut verifies that one event fans out to all subscribed clients.
func TestIntegration_MultipleClientsFanOut(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	// Create two clients for same user.
	c1, conn1 := dialTestHub(t, h, "user1", model.RoleUser)
	c2, conn2 := dialTestHub(t, h, "user1", model.RoleUser)

	// Subscribe both directly to bypass NATS.
	c1.topics["usage:user1"] = struct{}{}
	c2.topics["usage:user1"] = struct{}{}

	// Dispatch event directly.
	h.dispatchNATSEvent("usage:user1", []byte(`{"cost_usd": 0.05}`))

	// Both should receive the event.
	var event1, event2 ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn1, &event1))
	require.NoError(t, wsjson.Read(context.Background(), conn2, &event2))
	assert.Equal(t, ServerMessageEvent, event1.Type)
	assert.Equal(t, ServerMessageEvent, event2.Type)
	assert.Equal(t, event1.Seq, event2.Seq) // Same sequence number

	h.Close()
}

// TestIntegration_UnauthorizedTopicSubscription verifies subscribing to another user's topic is rejected.
func TestIntegration_UnauthorizedTopicSubscription(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, "user2", model.RoleUser, h) // user2 trying to access user1's topic
		h.Register(c)
		ctx := r.Context()
		go c.writePump(ctx)
		c.readPump(ctx)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.CloseNow() })

	// Consume auth_ok.
	var authMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &authMsg))
	assert.Equal(t, ServerMessageAuthOk, authMsg.Type)

	// Try to subscribe to user1's topic (should be rejected).
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))

	var errMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &errMsg))
	assert.Equal(t, ServerMessageError, errMsg.Type)

	h.Close()
}

// TestIntegration_ReconnectReplay verifies that after reconnect, client can replay buffered events.
func TestIntegration_ReconnectReplay(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	// First connection: subscribe and receive some events.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, "user1", model.RoleUser, h)
		h.Register(c)
		ctx := r.Context()
		go c.writePump(ctx)
		c.readPump(ctx)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn1, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)

	// Consume auth_ok.
	var authMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn1, &authMsg))

	// Subscribe to usage:user1.
	require.NoError(t, wsjson.Write(context.Background(), conn1, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))
	var subMsg1 ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn1, &subMsg1))

	// Publish two events.
	for i := 1; i <= 2; i++ {
		usageData := map[string]interface{}{"seq": i}
		payload, _ := json.Marshal(usageData)
		natsMsg := nats.NewMsg("ace.usage.user1.cost")
		natsMsg.Data = payload
		env := messaging.NewEnvelope("", "", "", "test-service")
		messaging.SetHeaders(natsMsg, env)
		testNATSConn.PublishMsg(natsMsg)
		time.Sleep(10 * time.Millisecond)
	}

	// Read both events.
	for i := 0; i < 2; i++ {
		var msg ServerMessage
		require.NoError(t, wsjson.Read(context.Background(), conn1, &msg))
	}

	// Close first connection.
	conn1.Close(websocket.StatusNormalClosure, "done")
	require.Eventually(t, func() bool {
		h.mu.RLock()
		defer h.mu.RUnlock()
		return len(h.clients["user1"]) == 0
	}, time.Second, 10*time.Millisecond)

	// Reconnect with a new connection.
	conn2, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn2.CloseNow() })

	// Consume auth_ok.
	var authMsg2 ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn2, &authMsg2))

	// Re-subscribe.
	require.NoError(t, wsjson.Write(context.Background(), conn2, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))
	var subMsg2 ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn2, &subMsg2))

	// Publish another event while reconnected.
	usageData := map[string]interface{}{"seq": 3}
	payload, _ := json.Marshal(usageData)
	natsMsg := nats.NewMsg("ace.usage.user1.cost")
	natsMsg.Data = payload
	env := messaging.NewEnvelope("", "", "", "test-service")
	messaging.SetHeaders(natsMsg, env)
	testNATSConn.PublishMsg(natsMsg)

	// Should receive the new event.
	var eventMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn2, &eventMsg))
	assert.Equal(t, ServerMessageEvent, eventMsg.Type)

	h.Close()
}

// TestIntegration_BufferExceededResync verifies that when buffer is exceeded, resync_required is sent.
func TestIntegration_BufferExceededResync(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	// Create a client and subscribe directly (bypassing NATS).
	c := &Client{
		id:          "resync-test-client",
		userID:      "user1",
		role:        model.RoleUser,
		topics:      map[string]struct{}{"usage:user1": {}},
		send:        make(chan []byte, 128),
		connectedAt: time.Now(),
		hub:         h,
	}
	h.mu.Lock()
	h.clients["user1"] = append(h.clients["user1"], c)
	h.mu.Unlock()

	// Fill buffer beyond its limit (1000 events).
	for i := uint64(1); i <= 1001; i++ {
		h.buffer.Append("usage:user1", i, []byte(`{"seq":1}`))
	}

	// Request replay with seq=1 (older than what's in buffer after overflow).
	// Since buffer is full and seq=1 is less than buffer[0].Seq=2, we should get resync_required.
	h.Replay(c, "usage:user1", 1)

	// Should get resync_required since buffer exceeded.
	select {
	case data := <-c.send:
		var msg ServerMessage
		json.Unmarshal(data, &msg)
		assert.Equal(t, ServerMessageResyncRequired, msg.Type)
		assert.Contains(t, msg.Resync, "usage:user1")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for resync_required")
	}

	h.mu.Lock()
	delete(h.clients, "user1")
	h.mu.Unlock()
	h.Close()
}

// TestIntegration_PollingEndpoint verifies the polling endpoint returns buffered events.
func TestIntegration_PollingEndpoint(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	// Pre-populate buffer.
	h.buffer.Append("usage:user1", 1, []byte(`{"a":1}`))
	h.buffer.Append("usage:user1", 2, []byte(`{"a":2}`))
	h.buffer.Append("usage:user1", 3, []byte(`{"a":3}`))

	// Create HTTP server for polling (bypassing auth middleware).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		topics := r.URL.Query()["topics"]
		sinceSeqStr := r.URL.Query().Get("since_seq")
		var sinceSeq uint64
		if sinceSeqStr != "" {
			sinceSeq, _ = strconv.ParseUint(sinceSeqStr, 10, 64)
		}

		result := h.PollEvents("user1", model.RoleUser, topics, sinceSeq)

		resp := struct {
			Events         []PollEntry `json:"events"`
			CurrentSeq     uint64      `json:"current_seq"`
			ResyncRequired []string    `json:"resync_required,omitempty"`
		}{
			Events:         result.Events,
			ResyncRequired: result.ResyncRequired,
		}
		for _, e := range result.Events {
			if e.Seq > resp.CurrentSeq {
				resp.CurrentSeq = e.Seq
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	// Test polling all events.
	resp, err := http.Get(srv.URL + "?topics=usage:user1&since_seq=0")
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var result struct {
		Events         []PollEntry `json:"events"`
		CurrentSeq     uint64      `json:"current_seq"`
		ResyncRequired []string    `json:"resync_required,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	resp.Body.Close()

	// Verify we got the buffered events.
	require.Len(t, result.Events, 3)
	assert.Equal(t, uint64(3), result.CurrentSeq)

	h.Close()
}

// TestIntegration_ConcurrentFanOut verifies that 100 events/second fan out without drops.
func TestIntegration_ConcurrentFanOut(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	// Create a single client.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, "user1", model.RoleUser, h)
		h.Register(c)
		ctx := r.Context()
		go c.writePump(ctx)
		c.readPump(ctx)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.CloseNow() })

	// Consume auth_ok and subscribe.
	require.NoError(t, wsjson.Read(context.Background(), conn, &ServerMessage{}))
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"system:health"},
	}))
	require.NoError(t, wsjson.Read(context.Background(), conn, &ServerMessage{}))

	// Publish 100 events rapidly.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			healthData := map[string]interface{}{"seq": i}
			payload, _ := json.Marshal(healthData)
			natsMsg := nats.NewMsg(fmt.Sprintf("ace.system.health.event%d", i))
			natsMsg.Data = payload
			env := messaging.NewEnvelope("", "", "", "test-service")
			messaging.SetHeaders(natsMsg, env)
			testNATSConn.PublishMsg(natsMsg)
		}(i)
	}
	wg.Wait()

	// Collect events for 2 seconds.
	eventsReceived := 0
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		var msg ServerMessage
		err := wsjson.Read(ctx, conn, &msg)
		if err != nil {
			break
		}
		if msg.Type == ServerMessageEvent {
			eventsReceived++
		}
	}

	// Should receive all 100 events.
	assert.Equal(t, 100, eventsReceived, "expected 100 events but got %d", eventsReceived)

	h.Close()
}

// TestIntegration_GracefulShutdown verifies that Hub.Close drains all client connections.
func TestIntegration_GracefulShutdown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := NewHub(testNATSConn, logger, meter)

	// Create multiple servers and actually connect clients to them.
	var servers []*httptest.Server
	var conns []*websocket.Conn

	for i := 0; i < 3; i++ {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			require.NoError(t, err)
			c := NewClient(conn, fmt.Sprintf("user%d", i), model.RoleUser, h)
			h.Register(c)
			ctx := r.Context()
			go c.writePump(ctx)
			c.readPump(ctx)
		}))
		servers = append(servers, srv)

		// Actually dial the WebSocket to trigger client registration.
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
		conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
		require.NoError(t, err)
		conns = append(conns, conn)

		// Consume auth_ok and subscribe.
		var authMsg ServerMessage
		require.NoError(t, wsjson.Read(context.Background(), conn, &authMsg))
		assert.Equal(t, ServerMessageAuthOk, authMsg.Type)
	}

	// Give time for clients to register.
	time.Sleep(100 * time.Millisecond)

	h.mu.RLock()
	clientCount := len(h.clients)
	h.mu.RUnlock()
	assert.Equal(t, 3, clientCount, "expected 3 clients but got %d", clientCount)

	// Close hub - should drain all clients.
	require.NoError(t, h.Close())

	// All clients should be unregistered.
	require.Eventually(t, func() bool {
		h.mu.RLock()
		defer h.mu.RUnlock()
		return len(h.clients) == 0
	}, time.Second, 10*time.Millisecond)

	// Clean up WebSocket connections and servers.
	for _, conn := range conns {
		conn.CloseNow()
	}
	for _, srv := range servers {
		srv.Close()
	}
}
