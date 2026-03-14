package messaging

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnvelope(t *testing.T) {
	env := NewEnvelope("corr-123", "agent-1", "cycle-1", "test-service")

	assert.NotEmpty(t, env.MessageID, "MessageID should be generated")
	assert.Equal(t, "corr-123", env.CorrelationID)
	assert.Equal(t, "agent-1", env.AgentID)
	assert.Equal(t, "cycle-1", env.CycleID)
	assert.Equal(t, "test-service", env.SourceService)
	assert.NotZero(t, env.Timestamp, "Timestamp should be set")
	assert.Equal(t, SchemaVersion, env.SchemaVersion)
}

func TestNewEnvelope_EmptyOptionalFields(t *testing.T) {
	env := NewEnvelope("", "", "", "test-service")

	assert.NotEmpty(t, env.MessageID)
	assert.Empty(t, env.CorrelationID)
	assert.Empty(t, env.AgentID)
	assert.Empty(t, env.CycleID)
	assert.Equal(t, "test-service", env.SourceService)
}

func TestGenerateMessageID(t *testing.T) {
	id1 := GenerateMessageID()
	id2 := GenerateMessageID()

	assert.NotEmpty(t, id1)
	assert.NotEqual(t, id1, id2, "Message IDs should be unique")
}

func TestEnvelope_SetPayload(t *testing.T) {
	env := NewEnvelope("", "", "", "test-service")

	type TestPayload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	payload := TestPayload{Name: "John", Age: 30}

	err := env.SetPayload(payload)
	require.NoError(t, err)

	assert.NotEmpty(t, env.Payload)

	var retrieved TestPayload
	err = env.GetPayload(&retrieved)
	require.NoError(t, err)
	assert.Equal(t, payload, retrieved)
}

func TestEnvelope_SetPayload_Nil(t *testing.T) {
	env := NewEnvelope("", "", "", "test-service")

	err := env.SetPayload(nil)
	require.NoError(t, err)
	assert.Empty(t, env.Payload)
}

func TestEnvelope_GetPayload_Empty(t *testing.T) {
	env := NewEnvelope("", "", "", "test-service")

	var data map[string]interface{}
	err := env.GetPayload(&data)
	assert.Error(t, err)
}

func TestEnvelope_GetPayload_InvalidJSON(t *testing.T) {
	env := &Envelope{
		Payload: json.RawMessage(`{invalid`),
	}

	var data map[string]interface{}
	err := env.GetPayload(&data)
	assert.Error(t, err)
}

func TestEnvelope_Validate(t *testing.T) {
	tests := []struct {
		name    string
		env     *Envelope
		wantErr bool
	}{
		{
			name: "valid envelope",
			env: &Envelope{
				MessageID:     "msg-123",
				SourceService: "test-service",
				SchemaVersion: SchemaVersion,
			},
			wantErr: false,
		},
		{
			name: "missing message ID",
			env: &Envelope{
				MessageID:     "",
				SourceService: "test-service",
				SchemaVersion: SchemaVersion,
			},
			wantErr: true,
		},
		{
			name: "missing source service",
			env: &Envelope{
				MessageID:     "msg-123",
				SourceService: "",
				SchemaVersion: SchemaVersion,
			},
			wantErr: true,
		},
		{
			name: "missing schema version",
			env: &Envelope{
				MessageID:     "msg-123",
				SourceService: "test-service",
				SchemaVersion: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.env.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnvelopeFromHeaders(t *testing.T) {
	headers := nats.Header{
		HeaderMessageID:     []string{"msg-123"},
		HeaderCorrelationID: []string{"corr-456"},
		HeaderAgentID:       []string{"agent-1"},
		HeaderCycleID:       []string{"cycle-1"},
		HeaderSourceService: []string{"test-service"},
		HeaderSchemaVersion: []string{SchemaVersion},
	}

	msg := &nats.Msg{
		Header: headers,
	}

	env, err := EnvelopeFromHeaders(msg)
	require.NoError(t, err)

	assert.Equal(t, "msg-123", env.MessageID)
	assert.Equal(t, "corr-456", env.CorrelationID)
	assert.Equal(t, "agent-1", env.AgentID)
	assert.Equal(t, "cycle-1", env.CycleID)
	assert.Equal(t, "test-service", env.SourceService)
	assert.Equal(t, SchemaVersion, env.SchemaVersion)
}

func TestEnvelopeFromHeaders_MissingRequired(t *testing.T) {
	headers := nats.Header{
		HeaderMessageID: []string{"msg-123"},
		// Missing HeaderSourceService
	}

	msg := &nats.Msg{
		Header: headers,
	}

	env, err := EnvelopeFromHeaders(msg)
	assert.Error(t, err)
	assert.Nil(t, env)
	assert.Contains(t, err.Error(), "X-Source-Service")
}

func TestEnvelopeFromHeaders_NilMessage(t *testing.T) {
	env, err := EnvelopeFromHeaders(nil)
	assert.Error(t, err)
	assert.Nil(t, env)
}

func TestEnvelopeFromHeaders_InvalidTimestamp(t *testing.T) {
	headers := nats.Header{
		HeaderMessageID:     []string{"msg-123"},
		HeaderSourceService: []string{"test-service"},
		HeaderTimestamp:     []string{"invalid-timestamp"},
	}

	msg := &nats.Msg{
		Header: headers,
	}

	env, err := EnvelopeFromHeaders(msg)
	assert.Error(t, err)
	assert.Nil(t, env)
}

func TestEnvelopeFromHeaders_WithValidTimestamp(t *testing.T) {
	expectedTime := time.Now().UTC().Truncate(time.Nanosecond)
	headers := nats.Header{
		HeaderMessageID:     []string{"msg-123"},
		HeaderSourceService: []string{"test-service"},
		HeaderTimestamp:     []string{expectedTime.Format(time.RFC3339Nano)},
	}

	msg := &nats.Msg{
		Header: headers,
	}

	env, err := EnvelopeFromHeaders(msg)
	require.NoError(t, err)
	assert.True(t, env.Timestamp.Equal(expectedTime))
}

func TestEnvelopeFromHeaders_MissingMessageID_GeneratesNew(t *testing.T) {
	headers := nats.Header{
		HeaderSourceService: []string{"test-service"},
	}

	msg := &nats.Msg{
		Header: headers,
	}

	env, err := EnvelopeFromHeaders(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, env.MessageID)
}

func TestSetHeaders(t *testing.T) {
	env := &Envelope{
		MessageID:     "msg-123",
		CorrelationID: "corr-456",
		AgentID:       "agent-1",
		CycleID:       "cycle-1",
		SourceService: "test-service",
		Timestamp:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		SchemaVersion: SchemaVersion,
	}

	msg := &nats.Msg{}
	SetHeaders(msg, env)

	assert.Equal(t, "msg-123", msg.Header.Get(HeaderMessageID))
	assert.Equal(t, "corr-456", msg.Header.Get(HeaderCorrelationID))
	assert.Equal(t, "agent-1", msg.Header.Get(HeaderAgentID))
	assert.Equal(t, "cycle-1", msg.Header.Get(HeaderCycleID))
	assert.Equal(t, "test-service", msg.Header.Get(HeaderSourceService))
	assert.Equal(t, "2024-01-01T00:00:00Z", msg.Header.Get(HeaderTimestamp))
	assert.Equal(t, SchemaVersion, msg.Header.Get(HeaderSchemaVersion))
}

func TestSetHeaders_NilMessage(t *testing.T) {
	env := NewEnvelope("", "", "", "test-service")
	SetHeaders(nil, env)
	// Should not panic
}

func TestSetHeaders_NilEnvelope(t *testing.T) {
	msg := &nats.Msg{}
	SetHeaders(msg, nil)
	// Should not panic
}

func TestSetHeaders_EmptyOptionalFields(t *testing.T) {
	env := &Envelope{
		MessageID:     "msg-123",
		SourceService: "test-service",
		Timestamp:     time.Now().UTC(),
		SchemaVersion: SchemaVersion,
		// AgentID, CycleID, CorrelationID are empty
	}

	msg := &nats.Msg{}
	SetHeaders(msg, env)

	assert.Equal(t, "msg-123", msg.Header.Get(HeaderMessageID))
	assert.Equal(t, "", msg.Header.Get(HeaderCorrelationID))
	assert.Equal(t, "", msg.Header.Get(HeaderAgentID))
	assert.Equal(t, "", msg.Header.Get(HeaderCycleID))
	assert.Equal(t, "test-service", msg.Header.Get(HeaderSourceService))
}

func TestEnvelope_Clone(t *testing.T) {
	original := NewEnvelope("corr-123", "agent-1", "cycle-1", "test-service")
	original.SetPayload(map[string]string{"key": "value"})

	cloned := original.Clone()

	// Should have different MessageID
	assert.NotEqual(t, original.MessageID, cloned.MessageID)

	// Should have different Timestamp
	assert.NotEqual(t, original.Timestamp, cloned.Timestamp)

	// Should preserve other fields
	assert.Equal(t, original.CorrelationID, cloned.CorrelationID)
	assert.Equal(t, original.AgentID, cloned.AgentID)
	assert.Equal(t, original.CycleID, cloned.CycleID)
	assert.Equal(t, original.SourceService, cloned.SourceService)

	// Payload should be deep copied (compare contents and ensure different slices)
	assert.Equal(t, original.Payload, cloned.Payload)
	assert.True(t, &original.Payload != &cloned.Payload, "Payload slices should be different memory addresses")
}

func TestEnvelope_Clone_WithNilPayload(t *testing.T) {
	original := NewEnvelope("", "", "", "test-service")
	cloned := original.Clone()

	assert.Empty(t, cloned.Payload)
}

// Benchmark for performance testing
func BenchmarkGenerateMessageID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateMessageID()
	}
}

func BenchmarkNewEnvelope(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewEnvelope("corr-123", "agent-1", "cycle-1", "test-service")
	}
}
