package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
