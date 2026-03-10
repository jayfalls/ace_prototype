package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Message represents a chat message
type Message struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	AgentID   uuid.UUID
	Role      MessageRole
	Content   string
	Timestamp time.Time
}

// MessageRole defines message sender
type MessageRole int

const (
	RoleUser MessageRole = iota
	RoleAgent
	RoleSystem
)

// Session represents a chat session
type Session struct {
	ID        uuid.UUID
	AgentID   uuid.UUID
	UserID    uuid.UUID
	Messages  []Message
	Status    SessionStatus
	CreatedAt time.Time
	EndedAt   *time.Time
}

// SessionStatus defines session state
type SessionStatus int

const (
	SessionActive SessionStatus = iota
	SessionEnded
)

// Handler handles chat WebSocket connections
type Handler struct {
	mu          sync.RWMutex
	sessions    map[uuid.UUID]*Session
	upgrader    websocket.Upgrader
	engine      *layers.Engine
}

// NewHandler creates a chat handler
func NewHandler(engine *layers.Engine) *Handler {
	return &Handler{
		sessions: make(map[uuid.UUID]*Session),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		engine: engine,
	}
}

// CreateSession starts a new chat session
func (h *Handler) CreateSession(ctx context.Context, agentID, userID uuid.UUID) (*Session, error) {
	session := &Session{
		ID:        uuid.New(),
		AgentID:   agentID,
		UserID:    userID,
		Messages:  []Message{},
		Status:    SessionActive,
		CreatedAt: time.Now(),
	}

	h.mu.Lock()
	h.sessions[session.ID] = session
	h.mu.Unlock()

	return session, nil
}

// GetSession returns a session
func (h *Handler) GetSession(sessionID uuid.UUID) (*Session, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	session, ok := h.sessions[sessionID]
	return session, ok
}

// AddMessage adds a message to session
func (h *Handler) AddMessage(sessionID uuid.UUID, msg Message) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, ok := h.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}

	msg.ID = uuid.New()
	msg.Timestamp = time.Now()
	session.Messages = append(session.Messages, msg)

	return nil
}

// ProcessInput sends input through layers and returns response
func (h *Handler) ProcessInput(ctx context.Context, sessionID uuid.UUID, content string) (string, error) {
	session, ok := h.GetSession(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found")
	}

	// Add user message
	userMsg := Message{
		SessionID: sessionID,
		AgentID:   session.AgentID,
		Role:      RoleUser,
		Content:   content,
	}
	_ = h.AddMessage(sessionID, userMsg)

	// Process through layers
	result, err := h.engine.ProcessCycle(ctx, content)
	if err != nil {
		return "", err
	}

	// Extract response from layer outputs
	response := "Processed: " + content
	for lt, output := range result.LayerOutputs {
		if data, ok := output.Data.(map[string]interface{}); ok {
			if str, ok := data["decision"].(string); ok {
				response = str
				break
			}
			if str, ok := data["executed"].(string); ok {
				response = str
				break
			}
		}
		_ = lt // suppress unused warning
	}

	// Add agent response
	agentMsg := Message{
		SessionID: sessionID,
		AgentID:   session.AgentID,
		Role:      RoleAgent,
		Content:   response,
	}
	_ = h.AddMessage(sessionID, agentMsg)

	return response, nil
}

// HandleWebSocket upgrades HTTP to WebSocket
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request, sessionID uuid.UUID) error {
	session, ok := h.GetSession(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Send existing messages
	for _, msg := range session.Messages {
		if err := conn.WriteJSON(msg); err != nil {
			return err
		}
	}

	// Handle incoming messages
	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg Message
		if err := json.Unmarshal(msgData, &msg); err != nil {
			continue
		}

		response, err := h.ProcessInput(r.Context(), sessionID, msg.Content)
		if err != nil {
			errorMsg := Message{
				Role:    RoleSystem,
				Content: "Error: " + err.Error(),
			}
			conn.WriteJSON(errorMsg)
			continue
		}

		// Send response
		responseMsg := Message{
			SessionID: sessionID,
			AgentID:   session.AgentID,
			Role:      RoleAgent,
			Content:   response,
		}
		if err := conn.WriteJSON(responseMsg); err != nil {
			break
		}
	}

	return nil
}

// EndSession ends a chat session
func (h *Handler) EndSession(sessionID uuid.UUID) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, ok := h.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}

	now := time.Now()
	session.Status = SessionEnded
	session.EndedAt = &now

	return nil
}
