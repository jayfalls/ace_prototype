package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"go.uber.org/zap"

	mw "ace/internal/api/middleware"
	"ace/internal/api/model"
	"ace/internal/api/service"
)

const wsAuthTimeout = 5 * time.Second

// HandleWebSocket upgrades the connection, performs auth handshake, then drives the client pumps.
// No HTTP auth middleware — auth is via the first WebSocket message.
func HandleWebSocket(hub *Hub, tokenService *service.TokenService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			CompressionMode:    websocket.CompressionDisabled,
			InsecureSkipVerify: true, // auth is JWT-in-first-message; no CSRF risk
		})
		if err != nil {
			hub.logger.Warn("websocket accept failed", zap.Error(err))
			return
		}

		authCtx, cancel := context.WithTimeout(r.Context(), wsAuthTimeout)
		defer cancel()

		var authMsg ClientMessage
		if err := wsjson.Read(authCtx, conn, &authMsg); err != nil {
			conn.Close(websocket.StatusPolicyViolation, "auth timeout")
			hub.logger.Warn("ws auth read failed", zap.Error(err))
			return
		}

		if authMsg.Type != ClientMessageAuth || authMsg.Token == "" {
			conn.Close(websocket.StatusPolicyViolation, "first message must be auth")
			return
		}

		claims, err := tokenService.ValidateAccessToken(authMsg.Token)
		if err != nil {
			conn.Close(websocket.StatusPolicyViolation, "invalid token")
			hub.logger.Warn("ws auth token invalid", zap.Error(err))
			return
		}

		c := NewClient(conn, claims.Sub.String(), model.UserRole(claims.Role), hub)
		hub.Register(c)

		connCtx := r.Context()
		go c.writePump(connCtx)
		c.readPump(connCtx)
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

// HandlePolling serves buffered events for authenticated users via HTTP GET.
// Relies on the existing auth middleware setting userID and role in context.
func HandlePolling(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		result := hub.PollEvents(userID, role, topics, sinceSeq)

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
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
