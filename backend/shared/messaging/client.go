package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Config holds the configuration for the NATS client.
type Config struct {
	// URLs is the NATS server URLs (comma-separated).
	URLs string
	// Name is the client name.
	Name string
	// Username is the optional credentials.
	Username string
	// Password is the optional credentials.
	Password string
	// Timeout is the connection timeout.
	Timeout time.Duration
	// MaxReconnect is the maximum reconnection attempts.
	MaxReconnect int
	// ReconnectWait is the wait time between reconnects.
	ReconnectWait time.Duration
}

// MsgHandler is a function that handles incoming NATS messages.
type MsgHandler func(msg *nats.Msg)

// Subscription represents a NATS subscription.
type Subscription interface {
	Unsubscribe() error
}

// natsSubscription wraps a NATS subscription.
type natsSubscription struct {
	sub *nats.Subscription
}

// Unsubscribe removes the subscription.
func (s *natsSubscription) Unsubscribe() error {
	if s.sub != nil {
		return s.sub.Unsubscribe()
	}
	return nil
}

// Client defines the interface for NATS messaging operations.
type Client interface {
	// Publish sends a message without waiting for response.
	Publish(subject string, data []byte, headers nats.Header) error

	// Request sends a message and waits for response.
	Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error)

	// Subscribe creates a subscription.
	Subscribe(subject string, handler MsgHandler) (Subscription, error)

	// SubscribeToStream creates a JetStream consumer.
	SubscribeToStream(ctx context.Context, stream, consumer, subject string, handler MsgHandler) error

	// HealthCheck verifies connection and JetStream.
	HealthCheck() error

	// Drain gracefully closes connection.
	Drain() error

	// Close closes connection.
	Close()
}

// natsClient implements the Client interface using NATS.
type natsClient struct {
	nc   *nats.Conn
	js   nats.JetStreamContext
	cfg  Config
	mu   sync.RWMutex
	done chan struct{}
}

// NewClient creates a new NATS client.
func NewClient(cfg Config) (Client, error) {
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.Timeout(cfg.Timeout),
		nats.MaxReconnects(cfg.MaxReconnect),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
			// Log disconnect but let reconnect handle it
			_ = err
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			// Log reconnect
			_ = conn
		}),
		nats.ClosedHandler(func(conn *nats.Conn) {
			// Log connection closed
			_ = conn
		}),
	}

	// Add credentials if provided
	if cfg.Username != "" && cfg.Password != "" {
		opts = append(opts, nats.UserInfo(cfg.Username, cfg.Password))
	}

	nc, err := nats.Connect(cfg.URLs, opts...)
	if err != nil {
		return nil, ConnectionError(err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, JetStreamError(err)
	}

	client := &natsClient{
		nc:   nc,
		js:   js,
		cfg:  cfg,
		done: make(chan struct{}),
	}

	return client, nil
}

// Publish sends a message without waiting for response.
func (c *natsClient) Publish(subject string, data []byte, headers nats.Header) error {
	if c.nc == nil {
		return ConnectionError(fmt.Errorf("not connected"))
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  headers,
	}

	return c.nc.PublishMsg(msg)
}

// Request sends a message and waits for response.
func (c *natsClient) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	if c.nc == nil {
		return nil, ConnectionError(fmt.Errorf("not connected"))
	}

	msg, err := c.nc.Request(subject, data, timeout)
	if err != nil {
		if err == nats.ErrTimeout {
			return nil, TimeoutError(err)
		}
		if err == nats.ErrNoResponders {
			return nil, NoRespondersError(err)
		}
		return nil, err
	}

	return msg, nil
}

// Subscribe creates a subscription.
func (c *natsClient) Subscribe(subject string, handler MsgHandler) (Subscription, error) {
	if c.nc == nil {
		return nil, ConnectionError(fmt.Errorf("not connected"))
	}

	sub, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg)
	})
	if err != nil {
		return nil, err
	}

	return &natsSubscription{sub: sub}, nil
}

// SubscribeToStream creates a JetStream consumer.
func (c *natsClient) SubscribeToStream(ctx context.Context, stream, consumer, subject string, handler MsgHandler) error {
	if c.js == nil {
		return JetStreamError(fmt.Errorf("jetstream not available"))
	}

	_, err := c.js.ConsumerInfo(stream, consumer)
	if err != nil {
		if err != nats.ErrConsumerNotFound {
			return JetStreamError(err)
		}

		// Create consumer if it doesn't exist
		_, err = c.js.AddConsumer(stream, &nats.ConsumerConfig{
			Durable:        consumer,
			DeliverSubject: consumer,
			DeliverPolicy:  nats.DeliverNewPolicy,
			AckPolicy:      nats.AckExplicitPolicy,
			AckWait:        30 * time.Second,
			MaxDeliver:     3,
			FilterSubject:   subject,
		})
		if err != nil {
			return JetStreamError(err)
		}
	}

	// Convert our MsgHandler to nats.MsgHandler
	wrappedHandler := func(msg *nats.Msg) {
		handler(msg)
	}

	_, err = c.js.Subscribe(subject, wrappedHandler,
		nats.Bind(stream, consumer),
		nats.AckExplicit(),
	)
	if err != nil {
		return JetStreamError(err)
	}

	return nil
}

// HealthCheck verifies connection and JetStream.
func (c *natsClient) HealthCheck() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.nc == nil {
		return ConnectionError(fmt.Errorf("not connected"))
	}

	// Check connection using IsConnected
	if !c.nc.IsConnected() {
		return ConnectionError(fmt.Errorf("not connected"))
	}

	// Check JetStream
	if c.js == nil {
		return JetStreamError(fmt.Errorf("jetstream not initialized"))
	}

	_, err := c.js.AccountInfo()
	if err != nil {
		return JetStreamError(err)
	}

	return nil
}

// Drain gracefully closes connection.
func (c *natsClient) Drain() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nc == nil {
		return nil
	}

	if err := c.nc.Drain(); err != nil {
		return err
	}

	// Wait for drain to complete
	for {
		if c.nc.IsDraining() && !c.nc.IsClosed() {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}

	return nil
}

// Close closes the connection.
func (c *natsClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nc != nil {
		c.nc.Close()
		c.nc = nil
	}
	c.js = nil

	close(c.done)
}

// MockClient is a mock implementation of Client for testing.
type MockClient struct {
	mu              sync.RWMutex
	PublishedMsgs   []*MockMsg
	Subscriptions  []*MockSubscription
	RequestResp    *nats.Msg
	RequestErr     error
	HealthCheckErr error
	DrainErr       error
	CloseCalled    bool
}

// MockMsg represents a mock message.
type MockMsg struct {
	Subject string
	Data    []byte
	Headers nats.Header
}

// MockSubscription represents a mock subscription.
type MockSubscription struct {
	Subject string
	Handler MsgHandler
}

// Publish records a published message.
func (m *MockClient) Publish(subject string, data []byte, headers nats.Header) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishedMsgs = append(m.PublishedMsgs, &MockMsg{
		Subject: subject,
		Data:    data,
		Headers: headers,
	})

	return nil
}

// Request returns a mock response.
func (m *MockClient) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.RequestErr != nil {
		return nil, m.RequestErr
	}

	if m.RequestResp != nil {
		return m.RequestResp, nil
	}

	// Return default response
	return &nats.Msg{
		Subject: subject,
		Data:    []byte("mock response"),
	}, nil
}

// Subscribe records a subscription.
func (m *MockClient) Subscribe(subject string, handler MsgHandler) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub := &MockSubscription{
		Subject: subject,
		Handler: handler,
	}
	m.Subscriptions = append(m.Subscriptions, sub)

	return sub, nil
}

// SubscribeToStream is a no-op for mock client.
func (m *MockClient) SubscribeToStream(ctx context.Context, stream, consumer, subject string, handler MsgHandler) error {
	_, err := m.Subscribe(subject, handler)
	return err
}

// HealthCheck returns the configured error.
func (m *MockClient) HealthCheck() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.HealthCheckErr
}

// Drain is a no-op for mock client.
func (m *MockClient) Drain() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.DrainErr
}

// Close marks the mock client as closed.
func (m *MockClient) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseCalled = true
}

// GetPublishedMessages returns the published messages.
func (m *MockClient) GetPublishedMessages() []*MockMsg {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MockMsg, len(m.PublishedMsgs))
	copy(result, m.PublishedMsgs)
	return result
}

// GetSubscriptions returns the subscriptions.
func (m *MockClient) GetSubscriptions() []*MockSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MockSubscription, len(m.Subscriptions))
	copy(result, m.Subscriptions)
	return result
}

// Unsubscribe is a no-op for mock subscription.
func (m *MockSubscription) Unsubscribe() error {
	return nil
}
