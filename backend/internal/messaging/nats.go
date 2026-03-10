package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Message represents a message in the system
type Message struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Subject   string          `json:"subject"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// Publisher interface for publishing messages
type Publisher interface {
	Publish(ctx context.Context, subject string, msg interface{}) error
	Close()
}

// Subscriber interface for subscribing to messages
type Subscriber interface {
	Subscribe(subject string, handler func(msg *Message)) error
	Close()
}

// NATSClient is a NATS messaging client
type NATSClient struct {
	conn *nats.Conn
}

// NewNATSClient creates a new NATS client
func NewNATSClient(url string) (*NATSClient, error) {
	conn, err := nats.Connect(url, nats.Name("ace-framework"))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	return &NATSClient{conn: conn}, nil
}

// NewNATSClientWithCreds creates a new NATS client with credentials
func NewNATSClientWithCreds(url, credsFile string) (*NATSClient, error) {
	conn, err := nats.Connect(url, nats.Name("ace-framework"), nats.UserCredentials(credsFile))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	return &NATSClient{conn: conn}, nil
}

// Publish publishes a message to a subject
func (c *NATSClient) Publish(ctx context.Context, subject string, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return c.conn.Publish(subject, data)
}

// PublishAsync publishes a message asynchronously
func (c *NATSClient) PublishAsync(subject string, msg interface{}) (any, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	return nil, c.conn.Publish(subject, data)
}

// Subscribe subscribes to a subject
func (c *NATSClient) Subscribe(subject string, handler func(msg *Message)) error {
	_, err := c.conn.Subscribe(subject, func(natsMsg *nats.Msg) {
		var msg Message
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			fmt.Printf("failed to unmarshal message: %v\n", err)
			return
		}
		handler(&msg)
	})
	return err
}

// QueueSubscribe subscribes to a subject with a queue
func (c *NATSClient) QueueSubscribe(subject, queue string, handler func(msg *Message)) error {
	_, err := c.conn.QueueSubscribe(subject, queue, func(natsMsg *nats.Msg) {
		var msg Message
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			fmt.Printf("failed to unmarshal message: %v\n", err)
			return
		}
		handler(&msg)
	})
	return err
}

// Close closes the connection
func (c *NATSClient) Close() {
	if c.conn != nil {
		c.conn.Drain()
	}
}

// ============ In-Memory Messaging (for development/MVP) ============

// InMemoryMessageBus is a simple in-memory message bus
type InMemoryMessageBus struct {
	subscribers map[string][]chan *Message
}

// NewInMemoryMessageBus creates a new in-memory message bus
func NewInMemoryMessageBus() *InMemoryMessageBus {
	return &InMemoryMessageBus{
		subscribers: make(map[string][]chan *Message),
	}
}

// Publish publishes a message to a subject
func (b *InMemoryMessageBus) Publish(ctx context.Context, subject string, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}
	
	// Get subscribers
	subscribers := b.subscribers[subject]
	
	// Publish to all subscribers
	for _, ch := range subscribers {
		select {
		case ch <- &message:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Non-blocking
		}
	}
	
	return nil
}

// Subscribe subscribes to a subject
func (b *InMemoryMessageBus) Subscribe(subject string, handler func(msg *Message)) error {
	ch := make(chan *Message, 100) // Buffer size
	b.subscribers[subject] = append(b.subscribers[subject], ch)
	
	// Start handler goroutine
	go func() {
		for msg := range ch {
			handler(msg)
		}
	}()
	
	return nil
}

// Close closes the message bus
func (b *InMemoryMessageBus) Close() {
	for _, subs := range b.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	b.subscribers = make(map[string][]chan *Message)
}

// Subject constants
const (
	SubjectAgentThought   = "agent.thought"
	SubjectAgentAction   = "agent.action"
	SubjectAgentResponse = "agent.response"
	SubjectAgentError    = "agent.error"
	SubjectMemoryStored = "memory.stored"
	SubjectMemorySearch = "memory.search"
)
