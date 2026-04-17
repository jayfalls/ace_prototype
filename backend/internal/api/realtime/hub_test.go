package realtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
)

// newTestHub creates a Hub without a real NATS connection for unit testing.
// topicReg dispatch is wired to a no-op NATS stub so Add/Remove work on ref counts only.
func newTestHub(t *testing.T) *Hub {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	h := &Hub{
		clients: make(map[string][]*Client),
		buffer:  NewSeqBuffer(DefaultSeqBufferConfig()),
		logger:  logger,
		meter:   meter,
	}
	// TopicReg with nil NATS — tests that call Add/Remove must use topics that
	// won't attempt a real NATS Dial. We use a custom dispatch-only reg.
	h.topics = &TopicReg{
		refs:     make(map[string]int),
		subs:     make(map[string]*nats.Subscription),
		logger:   logger,
		dispatch: h.dispatchNATSEvent,
	}
	return h
}

// dialTestHub starts an httptest server that upgrades to WebSocket and returns
// a connected *websocket.Conn pair (server-side client + raw conn for assertions).
func dialTestHub(t *testing.T, h *Hub, userID string, role model.UserRole) (*Client, *websocket.Conn) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, userID, role, h)
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

	// Consume the auth_ok sent by Register.
	var msg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &msg))
	require.Equal(t, ServerMessageAuthOk, msg.Type)

	// Retrieve the server-side client from the hub.
	var serverClient *Client
	require.Eventually(t, func() bool {
		h.mu.RLock()
		defer h.mu.RUnlock()
		conns := h.clients[userID]
		if len(conns) > 0 {
			serverClient = conns[len(conns)-1]
			return true
		}
		return false
	}, time.Second, 10*time.Millisecond)

	return serverClient, conn
}

func readMsg(t *testing.T, conn *websocket.Conn) ServerMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var msg ServerMessage
	require.NoError(t, wsjson.Read(ctx, conn, &msg))
	return msg
}

// TestHub_RegisterUnregister verifies the full register/unregister lifecycle.
func TestHub_RegisterUnregister(t *testing.T) {
	h := newTestHub(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		require.NoError(t, err)
		c := NewClient(conn, "user1", model.RoleUser, h)
		h.Register(c)
		ctx := r.Context()
		go c.writePump(ctx)
		c.readPump(ctx)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)

	msg := readMsg(t, conn)
	assert.Equal(t, ServerMessageAuthOk, msg.Type)
	assert.NotEmpty(t, msg.ConnectionID)

	// Verify registered.
	h.mu.RLock()
	assert.Len(t, h.clients["user1"], 1)
	h.mu.RUnlock()

	// Close connection → readPump exits → Unregister called.
	conn.Close(websocket.StatusNormalClosure, "done")
	require.Eventually(t, func() bool {
		h.mu.RLock()
		defer h.mu.RUnlock()
		return len(h.clients["user1"]) == 0
	}, time.Second, 10*time.Millisecond)
}

// TestHub_SubscribeUnsubscribe verifies topic subscribe/unsubscribe message flow.
func TestHub_SubscribeUnsubscribe(t *testing.T) {
	h := newTestHub(t)
	_, conn := dialTestHub(t, h, "user1", model.RoleUser)

	// Subscribe
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))
	msg := readMsg(t, conn)
	assert.Equal(t, ServerMessageSubscribed, msg.Type)
	assert.Equal(t, []string{"usage:user1"}, msg.Topics)

	// Unsubscribe
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageUnsubscribe,
		Topics: []string{"usage:user1"},
	}))
	msg = readMsg(t, conn)
	assert.Equal(t, ServerMessageUnsubscribed, msg.Type)
	assert.Equal(t, []string{"usage:user1"}, msg.Topics)
}

// TestHub_Ping verifies ping → pong.
func TestHub_Ping(t *testing.T) {
	h := newTestHub(t)
	_, conn := dialTestHub(t, h, "user1", model.RoleUser)

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{Type: ClientMessagePing}))
	msg := readMsg(t, conn)
	assert.Equal(t, ServerMessagePong, msg.Type)
}

// TestHub_Dispatch_FanOut verifies that dispatchNATSEvent delivers to all subscribed clients.
func TestHub_Dispatch_FanOut(t *testing.T) {
	h := newTestHub(t)
	c1, conn1 := dialTestHub(t, h, "user1", model.RoleUser)
	c2, conn2 := dialTestHub(t, h, "user1", model.RoleUser)

	// Subscribe both server-side clients directly (no NATS needed for this).
	c1.topics["usage:user1"] = struct{}{}
	c2.topics["usage:user1"] = struct{}{}

	h.dispatchNATSEvent("usage:user1", []byte(`{"amount":5}`))

	msg1 := readMsg(t, conn1)
	msg2 := readMsg(t, conn2)
	assert.Equal(t, ServerMessageEvent, msg1.Type)
	assert.Equal(t, ServerMessageEvent, msg2.Type)
	assert.Equal(t, "usage:user1", msg1.Topic)
	assert.Equal(t, "usage:user1", msg2.Topic)
}

// TestHub_Dispatch_AuthFilter verifies unauthorized clients don't receive events.
func TestHub_Dispatch_AuthFilter(t *testing.T) {
	h := newTestHub(t)
	// user2 subscribes to user1's topic (unauthorized).
	c2, conn2 := dialTestHub(t, h, "user2", model.RoleUser)
	c2.topics["usage:user1"] = struct{}{}

	h.dispatchNATSEvent("usage:user1", []byte(`{}`))

	// No event should arrive for user2.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var msg ServerMessage
	err := wsjson.Read(ctx, conn2, &msg)
	assert.Error(t, err, "user2 should not receive events for user1's topic")
}

// TestHub_Subscribe_UnauthorizedTopic verifies subscribe to another user's topic is rejected.
func TestHub_Subscribe_UnauthorizedTopic(t *testing.T) {
	h := newTestHub(t)
	_, conn := dialTestHub(t, h, "user2", model.RoleUser)

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))
	msg := readMsg(t, conn)
	assert.Equal(t, ServerMessageError, msg.Type)
}

// TestHub_Subscribe_MaxTopics verifies the 50-topic cap is enforced.
func TestHub_Subscribe_MaxTopics(t *testing.T) {
	h := newTestHub(t)
	c, conn := dialTestHub(t, h, "user1", model.RoleUser)

	// Fill up to the limit directly.
	for i := 0; i < maxTopicsPerClient; i++ {
		c.topics[fmt.Sprintf("usage:user1-%d", i)] = struct{}{}
	}

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{"usage:user1"},
	}))
	msg := readMsg(t, conn)
	assert.Equal(t, ServerMessageError, msg.Type)
}

// TestHub_MultipleClientsPerUser verifies separate connections for the same user coexist.
func TestHub_MultipleClientsPerUser(t *testing.T) {
	h := newTestHub(t)
	dialTestHub(t, h, "user1", model.RoleUser)
	dialTestHub(t, h, "user1", model.RoleUser)

	h.mu.RLock()
	assert.Len(t, h.clients["user1"], 2)
	h.mu.RUnlock()
}

// TestHub_Close_DrainsAllClients verifies Close shuts down all connections.
func TestHub_Close_DrainsAllClients(t *testing.T) {
	h := newTestHub(t)
	dialTestHub(t, h, "user1", model.RoleUser)
	dialTestHub(t, h, "user2", model.RoleUser)

	require.NoError(t, h.Close())

	require.Eventually(t, func() bool {
		h.mu.RLock()
		defer h.mu.RUnlock()
		return len(h.clients) == 0
	}, time.Second, 10*time.Millisecond)
}

// TestHub_Admin_SeesAllTopics verifies admin authorization bypasses ownership check.
func TestHub_Admin_SeesAllTopics(t *testing.T) {
	h := newTestHub(t)
	assert.True(t, h.isAuthorized("admin1", model.RoleAdmin, "usage:user1"))
	assert.True(t, h.isAuthorized("admin1", model.RoleAdmin, "agent:user2:status"))
}

// TestHub_SystemHealth_AllUsers verifies system:health is visible to all roles.
func TestHub_SystemHealth_AllUsers(t *testing.T) {
	h := newTestHub(t)
	assert.True(t, h.isAuthorized("user1", model.RoleUser, "system:health"))
	assert.True(t, h.isAuthorized("viewer1", model.RoleViewer, "system:health"))
}

// TestHub_PollEvents verifies PollEvents returns buffered events and resync list.
func TestHub_PollEvents(t *testing.T) {
	h := newTestHub(t)

	h.buffer.Append("usage:user1", 1, []byte(`{"a":1}`))
	h.buffer.Append("usage:user1", 2, []byte(`{"a":2}`))

	result := h.PollEvents("user1", model.RoleUser, []string{"usage:user1"}, 0)
	assert.Len(t, result.Events, 2)
	assert.Empty(t, result.ResyncRequired)

	// Unauthorized topic filtered out.
	result2 := h.PollEvents("user2", model.RoleUser, []string{"usage:user1"}, 0)
	assert.Empty(t, result2.Events)
}

// TestHub_Concurrent_RegisterUnregister stress-tests concurrent hub access.
func TestHub_Concurrent_RegisterUnregister(t *testing.T) {
	h := newTestHub(t)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := fmt.Sprintf("user%d", i)
			c := &Client{
				id:          fmt.Sprintf("c%d", i),
				userID:      userID,
				role:        model.RoleUser,
				topics:      make(map[string]struct{}),
				send:        make(chan []byte, sendChannelSize),
				connectedAt: time.Now(),
				hub:         h,
			}
			h.mu.Lock()
			h.clients[userID] = append(h.clients[userID], c)
			h.mu.Unlock()
			h.Unregister(c)
		}(i)
	}
	wg.Wait()

	h.mu.RLock()
	assert.Empty(t, h.clients)
	h.mu.RUnlock()
}
