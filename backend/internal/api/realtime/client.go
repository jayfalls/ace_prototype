package realtime

import (
	"context"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"ace/internal/api/model"
)

const sendChannelSize = 128

// Client represents a single WebSocket connection.
type Client struct {
	id        string
	userID    string
	role      model.UserRole
	conn      *websocket.Conn
	topics    map[string]struct{}
	send      chan []byte
	hub       *Hub
	connectedAt time.Time
	done      chan struct{}
}

// NewClient creates a new Client for the given WebSocket connection.
func NewClient(conn *websocket.Conn, userID string, role model.UserRole, hub *Hub) *Client {
	return &Client{
		id:          uuid.New().String(),
		userID:      userID,
		role:        role,
		conn:        conn,
		topics:      make(map[string]struct{}),
		send:        make(chan []byte, sendChannelSize),
		hub:         hub,
		connectedAt: time.Now(),
		done:        make(chan struct{}),
	}
}

// Send marshals msg to JSON and queues it for delivery. Non-blocking: drops if channel full.
func (c *Client) Send(msg ServerMessage) {
	data := msg.Marshal()
	select {
	case c.send <- data:
	default:
		c.hub.logger.Warn("send channel full, dropping message",
			zap.String("client_id", c.id),
			zap.String("user_id", c.userID),
			zap.String("msg_type", string(msg.Type)),
		)
	}
}

// writePump reads from the send channel and writes to the WebSocket until done.
func (c *Client) writePump(ctx context.Context) {
	defer close(c.done)
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
				c.hub.logger.Debug("write error",
					zap.String("client_id", c.id),
					zap.Error(err),
				)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// readPump reads messages from the WebSocket and dispatches them until the connection closes.
func (c *Client) readPump(ctx context.Context) {
	defer c.hub.Unregister(c)
	for {
		var msg ClientMessage
		if err := wsjson.Read(ctx, c.conn, &msg); err != nil {
			if ctx.Err() == nil {
				c.hub.logger.Debug("read error",
					zap.String("client_id", c.id),
					zap.Error(err),
				)
			}
			return
		}
		c.handleMessage(ctx, msg)
	}
}

// handleMessage routes an incoming client message to the appropriate Hub operation.
func (c *Client) handleMessage(ctx context.Context, msg ClientMessage) {
	switch msg.Type {
	case ClientMessageSubscribe:
		c.hub.Subscribe(c, msg.Topics)
	case ClientMessageUnsubscribe:
		c.hub.Unsubscribe(c, msg.Topics)
	case ClientMessageReplay:
		c.hub.Replay(c, msg.Topic, msg.SinceSeq)
	case ClientMessagePing:
		c.Send(NewPongMessage())
	case ClientMessageAuth:
		// Token refresh on existing connection — no reconnect needed.
		// Hub re-validates; on failure it closes the connection.
		c.hub.RefreshAuth(c, msg.Token)
	}
}
