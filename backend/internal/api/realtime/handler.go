package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	mw "ace/internal/api/middleware"
	"ace/internal/api/model"
	"ace/internal/api/service"
)

// wsHandlerDeps holds OTel dependencies for WebSocket handler.
type WSHandlerDeps struct {
	Tracer           trace.Tracer
	Meter            metric.Meter
	MessagesReceived metric.Int64Counter
	MessagesSent     metric.Int64Counter
	WsErrors         metric.Int64Counter
}

// HandleWebSocket upgrades the connection, performs auth handshake, then drives the client pumps.
// No HTTP auth middleware — auth is via the first WebSocket message.
func HandleWebSocket(hub *Hub, tokenService *service.TokenService, deps WSHandlerDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		connectionID := uuid.New().String()

		// Start span for WebSocket upgrade
		ctx, span := deps.Tracer.Start(ctx, SpanWSUpgrade)
		defer span.End()
		span.SetAttributes(
			attribute.String("connection_id", connectionID),
		)

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			CompressionMode:    websocket.CompressionDisabled,
			InsecureSkipVerify: true, // auth is JWT-in-first-message; no CSRF risk
		})
		if err != nil {
			hub.logger.Warn("websocket accept failed",
				zap.String("connection_id", connectionID),
				zap.Error(err),
			)
			span.SetAttributes(attribute.Bool("accept_success", false))
			return
		}
		span.SetAttributes(attribute.Bool("accept_success", true))

		authCtx, cancel := context.WithTimeout(ctx, WS_AUTH_TIMEOUT)

		var authMsg ClientMessage
		if err := wsjson.Read(authCtx, conn, &authMsg); err != nil {
			conn.Close(websocket.StatusPolicyViolation, "auth timeout")
			hub.logger.Warn("ws auth read failed",
				zap.String("connection_id", connectionID),
				zap.Error(err),
			)
			cancel()
			deps.WsErrors.Add(ctx, 1)
			return
		}
		cancel()

		// Start span for auth handshake
		_, authSpan := deps.Tracer.Start(ctx, SpanWSAuth)
		authSpan.SetAttributes(attribute.String("connection_id", connectionID))

		if authMsg.Type != ClientMessageAuth || authMsg.Token == "" {
			conn.Close(websocket.StatusPolicyViolation, "first message must be auth")
			hub.logger.Warn("ws auth failed - not auth message",
				zap.String("connection_id", connectionID),
				zap.String("msg_type", string(authMsg.Type)),
			)
			authSpan.SetAttributes(attribute.Bool("success", false))
			authSpan.End()
			deps.WsErrors.Add(ctx, 1)
			return
		}

		claims, err := tokenService.ValidateAccessToken(authMsg.Token)
		if err != nil {
			conn.Close(websocket.StatusPolicyViolation, "invalid token")
			hub.logger.Warn("ws auth token invalid",
				zap.String("connection_id", connectionID),
				zap.Error(err),
			)
			authSpan.SetAttributes(attribute.Bool("success", false))
			authSpan.End()
			deps.WsErrors.Add(ctx, 1)
			return
		}
		authSpan.SetAttributes(
			attribute.Bool("success", true),
			attribute.String("user_id", claims.Sub.String()),
		)
		authSpan.End()

		userID := claims.Sub.String()

		// Record message received (the auth message)
		deps.MessagesReceived.Add(ctx, 1, metric.WithAttributes(
			attribute.String("user_id", userID),
			attribute.String("connection_id", connectionID),
		))

		c := NewClient(conn, userID, model.UserRole(claims.Role), hub)
		c.id = connectionID // Use the same connection ID for consistency
		hub.Register(c)

		// Record message sent (auth_ok)
		deps.MessagesSent.Add(ctx, 1, metric.WithAttributes(
			attribute.String("user_id", userID),
			attribute.String("connection_id", connectionID),
		))

		connCtx := r.Context()
		go c.writePump(connCtx)
		c.readPump(connCtx)
	}
}

// HandlePolling serves buffered events for authenticated users via HTTP GET.
// Relies on the existing auth middleware setting userID and role in context.
func HandlePolling(hub *Hub, pollRequests metric.Int64Counter, pollEventsDelivered metric.Int64Counter, bufferResyncRequired metric.Int64Counter, tracer trace.Tracer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		rawID := mw.GetUserIDFromContext(r.Context())
		var userID string
		switch v := rawID.(type) {
		case uuid.UUID:
			userID = v.String()
		case string:
			userID = v
		}
		role := mw.GetUserRoleFromContext(r.Context())

		var topics []string
		if raw := r.URL.Query().Get("topics"); raw != "" {
			topics = strings.Split(raw, ",")
		}

		var sinceSeq uint64
		if raw := r.URL.Query().Get("since_seq"); raw != "" {
			if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
				sinceSeq = v
			}
		}

		// Start span for polling request
		ctx, span := tracer.Start(ctx, SpanPoll)
		span.SetAttributes(
			attribute.String("user_id", userID),
			attribute.StringSlice("topics", topics),
			attribute.Int64("since_seq", int64(sinceSeq)),
		)
		defer span.End()

		result := hub.PollEvents(userID, role, topics, sinceSeq)

		// Increment poll requests metric
		pollRequests.Add(ctx, 1)

		resp := pollingResponse{
			Events:         make([]pollingEvent, 0, len(result.Events)),
			ResyncRequired: result.ResyncRequired,
		}
		for _, e := range result.Events {
			resp.Events = append(resp.Events, pollingEvent{
				Topic: e.Topic,
				Seq:   e.Seq,
				Data:  e.Data,
			})
			if e.Seq > resp.CurrentSeq {
				resp.CurrentSeq = e.Seq
			}
			// Increment events delivered metric
			pollEventsDelivered.Add(ctx, 1)
		}

		// Increment resync required metric
		if len(result.ResyncRequired) > 0 {
			bufferResyncRequired.Add(ctx, int64(len(result.ResyncRequired)))
			span.SetAttributes(attribute.Bool("resync_required", true))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// pollingResponse is the JSON payload for GET /api/realtime/updates.
type pollingResponse struct {
	Events         []pollingEvent `json:"events"`
	CurrentSeq     uint64         `json:"current_seq"`
	HasMore        bool           `json:"has_more"`
	ResyncRequired []string       `json:"resync_required,omitempty"`
}

type pollingEvent struct {
	Topic string          `json:"topic"`
	Seq   uint64          `json:"seq"`
	Data  json.RawMessage `json:"data"`
}
