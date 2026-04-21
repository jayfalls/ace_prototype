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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	"ace/internal/api/middleware"
	"ace/internal/api/model"
	"ace/internal/api/service"
)

func newTestTokenService(t *testing.T) *service.TokenService {
	t.Helper()
	svc, err := service.NewTokenService(&service.TokenConfig{
		Issuer:         "test",
		Audience:       "test",
		AccessTokenTTL: 15 * time.Minute,
	})
	require.NoError(t, err)
	return svc
}

func mintToken(t *testing.T, svc *service.TokenService, userID uuid.UUID, role model.UserRole) string {
	t.Helper()
	token, err := svc.GenerateAccessToken(&model.TokenClaims{
		Sub:  userID,
		Role: string(role),
		Iss:  "test",
		Aud:  "test",
	})
	require.NoError(t, err)
	return token
}

func newHandlerHub(t *testing.T) *Hub {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	meter := noop.NewMeterProvider().Meter("test")
	return NewHub(testNATSConn, logger, meter)
}

// newTestWSHandlerDeps creates WSHandlerDeps for testing.
func newTestWSHandlerDeps(metricMeter metric.Meter) WSHandlerDeps {
	messagesReceived, _ := metricMeter.Int64Counter(MetricWSMessagesReceived)
	messagesSent, _ := metricMeter.Int64Counter(MetricWSMessagesSent)
	wsErrors, _ := metricMeter.Int64Counter(MetricWSErrors)
	return WSHandlerDeps{
		Tracer:           otel.GetTracerProvider().Tracer("test"),
		MessagesReceived: messagesReceived,
		MessagesSent:     messagesSent,
		WsErrors:         wsErrors,
	}
}

// serveWS starts an httptest server running HandleWebSocket and returns its URL.
func serveWS(t *testing.T, hub *Hub, tokenSvc *service.TokenService) string {
	t.Helper()
	meter := noop.NewMeterProvider().Meter("test")
	deps := newTestWSHandlerDeps(meter)
	srv := httptest.NewServer(HandleWebSocket(hub, tokenSvc, deps))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// TestHandleWebSocket_ValidAuth tests that a valid JWT auth message results in auth_ok.
func TestHandleWebSocket_ValidAuth(t *testing.T) {
	hub := newHandlerHub(t)
	tokenSvc := newTestTokenService(t)
	userID := uuid.New()
	token := mintToken(t, tokenSvc, userID, model.RoleUser)

	conn, _, err := websocket.Dial(context.Background(), serveWS(t, hub, tokenSvc), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:  ClientMessageAuth,
		Token: token,
	}))

	var msg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &msg))
	assert.Equal(t, ServerMessageAuthOk, msg.Type)
	assert.NotEmpty(t, msg.ConnectionID)

	// Verify client is registered under the correct userID.
	hub.mu.RLock()
	_, registered := hub.clients[userID.String()]
	hub.mu.RUnlock()
	assert.True(t, registered)
}

// TestHandleWebSocket_InvalidToken tests that an invalid token closes the connection.
func TestHandleWebSocket_InvalidToken(t *testing.T) {
	hub := newHandlerHub(t)
	tokenSvc := newTestTokenService(t)

	conn, _, err := websocket.Dial(context.Background(), serveWS(t, hub, tokenSvc), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:  ClientMessageAuth,
		Token: "not-a-valid-jwt",
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var msg ServerMessage
	err = wsjson.Read(ctx, conn, &msg)
	assert.Error(t, err, "connection should be closed after invalid token")
}

// TestHandleWebSocket_WrongFirstMessageType tests that a non-auth first message closes the connection.
func TestHandleWebSocket_WrongFirstMessageType(t *testing.T) {
	hub := newHandlerHub(t)
	tokenSvc := newTestTokenService(t)

	conn, _, err := websocket.Dial(context.Background(), serveWS(t, hub, tokenSvc), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type: ClientMessagePing,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var msg ServerMessage
	err = wsjson.Read(ctx, conn, &msg)
	assert.Error(t, err, "connection should be closed when first message is not auth")
}

// TestHandleWebSocket_EmptyToken tests that an auth message with empty token is rejected.
func TestHandleWebSocket_EmptyToken(t *testing.T) {
	hub := newHandlerHub(t)
	tokenSvc := newTestTokenService(t)

	conn, _, err := websocket.Dial(context.Background(), serveWS(t, hub, tokenSvc), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:  ClientMessageAuth,
		Token: "",
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var msg ServerMessage
	err = wsjson.Read(ctx, conn, &msg)
	assert.Error(t, err, "connection should be closed on empty token")
}

// TestHandleWebSocket_SubscribeAfterAuth tests the full auth → subscribe flow.
func TestHandleWebSocket_SubscribeAfterAuth(t *testing.T) {
	hub := newHandlerHub(t)
	tokenSvc := newTestTokenService(t)
	userID := uuid.New()
	token := mintToken(t, tokenSvc, userID, model.RoleUser)

	conn, _, err := websocket.Dial(context.Background(), serveWS(t, hub, tokenSvc), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	// Auth
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:  ClientMessageAuth,
		Token: token,
	}))
	var authOk ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &authOk))
	assert.Equal(t, ServerMessageAuthOk, authOk.Type)

	// Subscribe to own usage topic
	topic := "usage:" + userID.String()
	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{
		Type:   ClientMessageSubscribe,
		Topics: []string{topic},
	}))
	var subMsg ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &subMsg))
	assert.Equal(t, ServerMessageSubscribed, subMsg.Type)
	assert.Equal(t, []string{topic}, subMsg.Topics)
}

// TestHandleWebSocket_PingPong tests ping → pong after auth.
func TestHandleWebSocket_PingPong(t *testing.T) {
	hub := newHandlerHub(t)
	tokenSvc := newTestTokenService(t)
	userID := uuid.New()
	token := mintToken(t, tokenSvc, userID, model.RoleUser)

	conn, _, err := websocket.Dial(context.Background(), serveWS(t, hub, tokenSvc), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{Type: ClientMessageAuth, Token: token}))
	var authOk ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &authOk))

	require.NoError(t, wsjson.Write(context.Background(), conn, ClientMessage{Type: ClientMessagePing}))
	var pong ServerMessage
	require.NoError(t, wsjson.Read(context.Background(), conn, &pong))
	assert.Equal(t, ServerMessagePong, pong.Type)
}

// servePolling starts an httptest server running HandlePolling with userID/role injected into context.
func servePolling(t *testing.T, hub *Hub, userID string, role model.UserRole) *httptest.Server {
	t.Helper()
	meter := noop.NewMeterProvider().Meter("test")
	pollRequests, _ := meter.Int64Counter(MetricPollRequests)
	pollEventsDelivered, _ := meter.Int64Counter(MetricPollEventsDelivered)
	bufferResyncRequired, _ := meter.Int64Counter(MetricBufferResyncRequired)
	tracer := otel.GetTracerProvider().Tracer("test")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), middleware.UserIDKey, uuid.MustParse(userID))
		ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
		HandlePolling(hub, pollRequests, pollEventsDelivered, bufferResyncRequired, tracer)(w, r.WithContext(ctx))
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// TestHandlePolling_EmptyTopics tests that an empty topics param returns an empty event list.
func TestHandlePolling_EmptyTopics(t *testing.T) {
	hub := newHandlerHub(t)
	userID := uuid.New().String()
	srv := servePolling(t, hub, userID, model.RoleUser)

	resp, err := http.Get(srv.URL + "/api/realtime/updates")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body pollingResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Empty(t, body.Events)
	assert.Empty(t, body.ResyncRequired)
}

// TestHandlePolling_ReturnsBufferedEvents tests events are returned for subscribed topics.
func TestHandlePolling_ReturnsBufferedEvents(t *testing.T) {
	hub := newHandlerHub(t)
	userID := uuid.New()
	topic := "usage:" + userID.String()

	hub.buffer.Append(topic, 1, []byte(`{"amount":10}`))
	hub.buffer.Append(topic, 2, []byte(`{"amount":20}`))

	srv := servePolling(t, hub, userID.String(), model.RoleUser)

	resp, err := http.Get(srv.URL + "/api/realtime/updates?topics=" + topic)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body pollingResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Len(t, body.Events, 2)
	assert.Equal(t, uint64(2), body.CurrentSeq)
	assert.Empty(t, body.ResyncRequired)
}

// TestHandlePolling_SinceSeq tests that since_seq filters older events.
func TestHandlePolling_SinceSeq(t *testing.T) {
	hub := newHandlerHub(t)
	userID := uuid.New()
	topic := "usage:" + userID.String()

	hub.buffer.Append(topic, 1, []byte(`{"a":1}`))
	hub.buffer.Append(topic, 2, []byte(`{"a":2}`))
	hub.buffer.Append(topic, 3, []byte(`{"a":3}`))

	srv := servePolling(t, hub, userID.String(), model.RoleUser)

	resp, err := http.Get(srv.URL + "/api/realtime/updates?topics=" + topic + "&since_seq=1")
	require.NoError(t, err)
	defer resp.Body.Close()

	var body pollingResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Len(t, body.Events, 2) // seq 2 and 3
	assert.Equal(t, uint64(3), body.CurrentSeq)
}

// TestHandlePolling_UnauthorizedTopicFiltered tests that events for another user's topic are not returned.
func TestHandlePolling_UnauthorizedTopicFiltered(t *testing.T) {
	hub := newHandlerHub(t)
	owner := uuid.New()
	requester := uuid.New()
	topic := "usage:" + owner.String()

	hub.buffer.Append(topic, 1, []byte(`{"amount":10}`))

	srv := servePolling(t, hub, requester.String(), model.RoleUser)

	resp, err := http.Get(srv.URL + "/api/realtime/updates?topics=" + topic)
	require.NoError(t, err)
	defer resp.Body.Close()

	var body pollingResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Empty(t, body.Events)
}

// TestHandlePolling_AdminSeesAll tests that an admin can poll any topic.
func TestHandlePolling_AdminSeesAll(t *testing.T) {
	hub := newHandlerHub(t)
	owner := uuid.New()
	admin := uuid.New()
	topic := "usage:" + owner.String()

	hub.buffer.Append(topic, 1, []byte(`{"amount":10}`))

	srv := servePolling(t, hub, admin.String(), model.RoleAdmin)

	resp, err := http.Get(srv.URL + "/api/realtime/updates?topics=" + topic)
	require.NoError(t, err)
	defer resp.Body.Close()

	var body pollingResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Len(t, body.Events, 1)
}
