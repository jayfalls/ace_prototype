package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
)

// MockJetStreamManager is a mock implementation of nats.JetStreamManager for testing.
type MockJetStreamManager struct {
	StreamConfigs  map[string]*nats.StreamConfig
	ConsumerConfigs map[string]map[string]*nats.ConsumerConfig
	AddStreamErr   error
	AddConsumerErr error
	StreamInfoErr error
	DeleteStreamErr error
}

func NewMockJetStreamManager() *MockJetStreamManager {
	return &MockJetStreamManager{
		StreamConfigs:  make(map[string]*nats.StreamConfig),
		ConsumerConfigs: make(map[string]map[string]*nats.ConsumerConfig),
	}
}

func (m *MockJetStreamManager) AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	if m.AddStreamErr != nil {
		return nil, m.AddStreamErr
	}
	m.StreamConfigs[cfg.Name] = cfg
	return &nats.StreamInfo{Config: *cfg}, nil
}

func (m *MockJetStreamManager) UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	m.StreamConfigs[cfg.Name] = cfg
	return &nats.StreamInfo{Config: *cfg}, nil
}

func (m *MockJetStreamManager) DeleteStream(name string, opts ...nats.JSOpt) error {
	if m.DeleteStreamErr != nil {
		return m.DeleteStreamErr
	}
	delete(m.StreamConfigs, name)
	return nil
}

func (m *MockJetStreamManager) StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	if m.StreamInfoErr != nil {
		return nil, m.StreamInfoErr
	}
	if cfg, ok := m.StreamConfigs[stream]; ok {
		return &nats.StreamInfo{Config: *cfg}, nil
	}
	return nil, nats.ErrStreamNotFound
}

func (m *MockJetStreamManager) PurgeStream(name string, opts ...nats.JSOpt) error {
	return nil
}

func (m *MockJetStreamManager) Streams(opts ...nats.JSOpt) <-chan *nats.StreamInfo {
	ch := make(chan *nats.StreamInfo, len(m.StreamConfigs))
	for _, cfg := range m.StreamConfigs {
		ch <- &nats.StreamInfo{Config: *cfg}
	}
	close(ch)
	return ch
}

func (m *MockJetStreamManager) StreamNames(opts ...nats.JSOpt) <-chan string {
	ch := make(chan string, len(m.StreamConfigs))
	for name := range m.StreamConfigs {
		ch <- name
	}
	close(ch)
	return ch
}

func (m *MockJetStreamManager) GetMsg(name string, seq uint64, opts ...nats.JSOpt) (*nats.RawStreamMsg, error) {
	return nil, nil
}

func (m *MockJetStreamManager) GetLastMsg(name, subject string, opts ...nats.JSOpt) (*nats.RawStreamMsg, error) {
	return nil, nil
}

func (m *MockJetStreamManager) DeleteMsg(name string, seq uint64, opts ...nats.JSOpt) error {
	return nil
}

func (m *MockJetStreamManager) SecureDeleteMsg(name string, seq uint64, opts ...nats.JSOpt) error {
	return nil
}

func (m *MockJetStreamManager) AddConsumer(stream string, cfg *nats.ConsumerConfig, opts ...nats.JSOpt) (*nats.ConsumerInfo, error) {
	if m.AddConsumerErr != nil {
		return nil, m.AddConsumerErr
	}
	if _, ok := m.ConsumerConfigs[stream]; !ok {
		m.ConsumerConfigs[stream] = make(map[string]*nats.ConsumerConfig)
	}
	m.ConsumerConfigs[stream][cfg.Durable] = cfg
	return &nats.ConsumerInfo{Config: *cfg}, nil
}

func (m *MockJetStreamManager) UpdateConsumer(stream string, cfg *nats.ConsumerConfig, opts ...nats.JSOpt) (*nats.ConsumerInfo, error) {
	m.ConsumerConfigs[stream][cfg.Durable] = cfg
	return &nats.ConsumerInfo{Config: *cfg}, nil
}

func (m *MockJetStreamManager) DeleteConsumer(stream, consumer string, opts ...nats.JSOpt) error {
	delete(m.ConsumerConfigs[stream], consumer)
	return nil
}

func (m *MockJetStreamManager) ConsumerInfo(stream, name string, opts ...nats.JSOpt) (*nats.ConsumerInfo, error) {
	if consumers, ok := m.ConsumerConfigs[stream]; ok {
		if cfg, ok := consumers[name]; ok {
			return &nats.ConsumerInfo{Config: *cfg}, nil
		}
	}
	return nil, nats.ErrConsumerNotFound
}

func (m *MockJetStreamManager) Consumers(stream string, opts ...nats.JSOpt) <-chan *nats.ConsumerInfo {
	ch := make(chan *nats.ConsumerInfo, len(m.ConsumerConfigs[stream]))
	for _, cfg := range m.ConsumerConfigs[stream] {
		ch <- &nats.ConsumerInfo{Config: *cfg}
	}
	close(ch)
	return ch
}

func (m *MockJetStreamManager) ConsumerNames(stream string, opts ...nats.JSOpt) <-chan string {
	ch := make(chan string, len(m.ConsumerConfigs[stream]))
	for name := range m.ConsumerConfigs[stream] {
		ch <- name
	}
	close(ch)
	return ch
}

func (m *MockJetStreamManager) ConsumersInfo(stream string, opts ...nats.JSOpt) <-chan *nats.ConsumerInfo {
	return m.Consumers(stream)
}

func (m *MockJetStreamManager) StreamsInfo(opts ...nats.JSOpt) <-chan *nats.StreamInfo {
	return m.Streams()
}

func (m *MockJetStreamManager) AccountInfo(opts ...nats.JSOpt) (*nats.AccountInfo, error) {
	return &nats.AccountInfo{}, nil
}

func (m *MockJetStreamManager) StreamNameBySubject(subject string, opts ...nats.JSOpt) (string, error) {
	return "", nil
}

func TestStreamConfigs_Defined(t *testing.T) {
	// Verify all stream configs are defined
	assert.Len(t, StreamConfigs, 3)

	// COGNITIVE stream
	cognitive := StreamConfigs[0]
	assert.Equal(t, "COGNITIVE", cognitive.Name)
	assert.Contains(t, cognitive.Subjects, "ace.engine.>")
	assert.Contains(t, cognitive.Subjects, "ace.memory.>")
	assert.Contains(t, cognitive.Subjects, "ace.tools.>")
	assert.Contains(t, cognitive.Subjects, "ace.senses.>")
	assert.Contains(t, cognitive.Subjects, "ace.llm.request")
	assert.Contains(t, cognitive.Subjects, "ace.llm.response")
	assert.Equal(t, nats.LimitsPolicy, cognitive.Retention)
	assert.Equal(t, int64(1*1024*1024*1024), cognitive.MaxBytes) // 1GB
	assert.Equal(t, 24*time.Hour, cognitive.MaxAge)
	assert.Equal(t, nats.FileStorage, cognitive.Storage)

	// USAGE stream
	usage := StreamConfigs[1]
	assert.Equal(t, "USAGE", usage.Name)
	assert.Contains(t, usage.Subjects, "ace.usage.>")
	assert.Equal(t, nats.LimitsPolicy, usage.Retention)
	assert.Equal(t, int64(100*1024*1024), usage.MaxBytes) // 100MB
	assert.Equal(t, 30*24*time.Hour, usage.MaxAge)

	// SYSTEM stream
	system := StreamConfigs[2]
	assert.Equal(t, "SYSTEM", system.Name)
	assert.Contains(t, system.Subjects, "ace.system.>")
	assert.Equal(t, nats.WorkQueuePolicy, system.Retention)
	assert.Equal(t, int64(10*1024*1024), system.MaxBytes) // 10MB
	assert.Equal(t, nats.MemoryStorage, system.Storage)
}

func TestEnsureStreams(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	err := EnsureStreams(ctx, mock)
	assert.NoError(t, err)

	// Verify all streams were created
	assert.Len(t, mock.StreamConfigs, 3)
	assert.Contains(t, mock.StreamConfigs, "COGNITIVE")
	assert.Contains(t, mock.StreamConfigs, "USAGE")
	assert.Contains(t, mock.StreamConfigs, "SYSTEM")
}

func TestEnsureStreams_Error(t *testing.T) {
	mock := NewMockJetStreamManager()
	mock.AddStreamErr = nats.ErrStreamNameAlreadyInUse
	ctx := context.Background()

	err := EnsureStreams(ctx, mock)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "STREAM_CREATE_FAILED")
}

func TestEnsureDLQStream(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	err := EnsureDLQStream(ctx, mock)
	assert.NoError(t, err)

	// Verify DLQ stream was created
	assert.Contains(t, mock.StreamConfigs, "DLQ")
	assert.Equal(t, []string{"dlq.>"}, mock.StreamConfigs["DLQ"].Subjects)
}

func TestCreateConsumer(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	// First create the stream
	_, err := mock.AddStream(&nats.StreamConfig{
		Name:     "COGNITIVE",
		Subjects: []string{"ace.engine.>"},
	})
	assert.NoError(t, err)

	cfg := DefaultConsumerConfig("COGNITIVE", "test-consumer", "ace.engine.>")
	err = CreateConsumer(ctx, mock, cfg)
	assert.NoError(t, err)

	// Verify consumer was created
	assert.Contains(t, mock.ConsumerConfigs, "COGNITIVE")
	assert.Contains(t, mock.ConsumerConfigs["COGNITIVE"], "test-consumer")
}

func TestCreateConsumerWithQueueGroup(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	// First create the stream
	_, err := mock.AddStream(&nats.StreamConfig{
		Name:     "COGNITIVE",
		Subjects: []string{"ace.engine.>"},
	})
	assert.NoError(t, err)

	cfg := DefaultConsumerConfig("COGNITIVE", "test-consumer", "ace.engine.>")
	err = CreateConsumerWithQueueGroup(ctx, mock, cfg, "my-queue-group")
	assert.NoError(t, err)

	// Verify consumer was created with queue group
	assert.Contains(t, mock.ConsumerConfigs["COGNITIVE"], "test-consumer")
	assert.Equal(t, "my-queue-group", mock.ConsumerConfigs["COGNITIVE"]["test-consumer"].DeliverGroup)
}

func TestCreateConsumerWithDLQ(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	// First create the stream
	_, err := mock.AddStream(&nats.StreamConfig{
		Name:     "COGNITIVE",
		Subjects: []string{"ace.engine.>"},
	})
	assert.NoError(t, err)

	dlqCfg := DLQConfig{
		StreamName:    "COGNITIVE",
		ConsumerName:  "test-consumer",
		FilterSubject: "ace.engine.>",
		MaxDeliver:    3,
		AckWait:      30 * time.Second,
	}

	err = CreateConsumerWithDLQ(ctx, mock, dlqCfg)
	assert.NoError(t, err)

	// Verify DLQ stream was created
	assert.Contains(t, mock.StreamConfigs, "DLQ")

	// Verify main consumer was created
	assert.Contains(t, mock.ConsumerConfigs["COGNITIVE"], "test-consumer")

	// Verify DLQ consumer was created
	assert.Contains(t, mock.ConsumerConfigs["DLQ"], "dlq-test-consumer")
}

func TestGetStreamInfo(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	// Create a stream first
	_, err := mock.AddStream(&nats.StreamConfig{
		Name:        "COGNITIVE",
		Description: "Test stream",
		Subjects:    []string{"ace.engine.>"},
		MaxBytes:    1024,
	})
	assert.NoError(t, err)

	// Get stream info
	info, err := GetStreamInfo(ctx, mock, "COGNITIVE")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "COGNITIVE", info.Config.Name)
}

func TestGetStreamInfo_NotFound(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	_, err := GetStreamInfo(ctx, mock, "NONEXISTENT")
	assert.Error(t, err)
}

func TestDeleteStream(t *testing.T) {
	mock := NewMockJetStreamManager()
	ctx := context.Background()

	// Create a stream first
	_, err := mock.AddStream(&nats.StreamConfig{
		Name:     "COGNITIVE",
		Subjects: []string{"ace.engine.>"},
	})
	assert.NoError(t, err)

	// Delete the stream
	err = DeleteStream(ctx, mock, "COGNITIVE")
	assert.NoError(t, err)

	// Verify stream was deleted
	_, err = mock.StreamInfo("COGNITIVE")
	assert.Error(t, err)
}

func TestDefaultConsumerConfig(t *testing.T) {
	cfg := DefaultConsumerConfig("STREAM", "CONSUMER", "subject")

	assert.Equal(t, "STREAM", cfg.Stream)
	assert.Equal(t, "CONSUMER", cfg.Consumer)
	assert.Equal(t, "CONSUMER", cfg.Durable)
	assert.Equal(t, "CONSUMER", cfg.DeliverSubject)
	assert.Equal(t, "subject", cfg.FilterSubject)
	assert.Equal(t, nats.DeliverNewPolicy, cfg.DeliverPolicy)
	assert.Equal(t, nats.AckExplicitPolicy, cfg.AckPolicy)
	assert.Equal(t, 30*time.Second, cfg.AckWait)
	assert.Equal(t, 3, cfg.MaxDeliver)
}
