package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	logger     zerolog.Logger
	engine     *layers.Engine
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	userID    string
	sessionID string
	agentID   string
}

type Message struct {
	Type     string          `json:"type"`
	Content  string          `json:"content"`
	Layer    string          `json:"layer,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

type ThoughtMessage struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
	AgentID string      `json:"agent_id,omitempty"`
	CycleID string      `json:"cycle_id,omitempty"`
}

func NewHub(logger zerolog.Logger, engine *layers.Engine) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		engine:     engine,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.logger.Info().Str("user_id", client.userID).Str("agent_id", client.agentID).Msg("Client connected")

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.logger.Info().Str("user_id", client.userID).Msg("Client disconnected")
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) RegisterClient(conn *websocket.Conn, userID, sessionID, agentID string) *Client {
	client := &Client{
		hub:       h,
		conn:      conn,
		send:      make(chan []byte, 256),
		userID:    userID,
		sessionID: sessionID,
		agentID:   agentID,
	}
	h.register <- client
	return client
}

// processThroughLayers sends input through actual ACE layers
func (h *Hub) processThroughLayers(agentID string, input interface{}) {
	if h.engine == nil {
		log.Printf("No engine available for agent %s", agentID)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Process through the engine
	result, err := h.engine.ProcessCycle(ctx, input)
	if err != nil {
		errMsg, _ := json.Marshal(ThoughtMessage{
			Type:    "error",
			Data:    err.Error(),
			AgentID: agentID,
		})
		h.broadcast <- errMsg
		return
	}

	// Broadcast thoughts from each layer
	for _, thought := range result.Thoughts {
		msg := ThoughtMessage{
			Type:    "thought",
			Data:    thought,
			AgentID: agentID,
			CycleID: result.CycleID.String(),
		}
		data, _ := json.Marshal(msg)
		h.broadcast <- data
	}

	// Broadcast layer outputs
	for layerType, output := range result.LayerOutputs {
		msg := ThoughtMessage{
			Type:    "layer_output",
			Data:    map[string]interface{}{"layer": layerType.String(), "data": output.Data},
			AgentID: agentID,
			CycleID: result.CycleID.String(),
		}
		data, _ := json.Marshal(msg)
		h.broadcast <- data
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	sessionID := r.URL.Query().Get("session_id")
	agentID := r.URL.Query().Get("agent_id")

	if token == "" || sessionID == "" {
		http.Error(w, "Missing token or session_id", http.StatusBadRequest)
		return
	}

	// In production, validate token and extract userID
	userID := "demo-user"

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	client := h.RegisterClient(conn, userID, sessionID, agentID)
	
	// Send welcome message
	welcomeMsg, _ := json.Marshal(ThoughtMessage{
		Type:    "connected",
		Data:    map[string]string{"agent_id": agentID, "message": "Connected to ACE engine"},
		AgentID: agentID,
	})
	conn.WriteMessage(websocket.TextMessage, welcomeMsg)

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error().Err(err).Msg("WebSocket error")
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			c.hub.logger.Error().Err(err).Msg("Failed to parse message")
			continue
		}

		msg.Timestamp = time.Now()
		c.hub.logger.Info().
			Str("user_id", c.userID).
			Str("type", msg.Type).
			Msg("Received message")

		// Process user input through ACE layers
		if msg.Type == "input" || msg.Type == "chat" {
			go c.hub.processThroughLayers(c.agentID, msg.Content)
		} else {
			// Broadcast to all clients (in production, filter by session)
			c.hub.broadcast <- message
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
