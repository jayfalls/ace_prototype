package messaging

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// SchemaVersion is the current message envelope schema version.
const SchemaVersion = "1.0"

// Header constants for NATS message headers.
const (
	HeaderMessageID     = "X-Message-ID"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderAgentID       = "X-Agent-ID"
	HeaderCycleID       = "X-Cycle-ID"
	HeaderSourceService = "X-Source-Service"
	HeaderTimestamp     = "X-Timestamp"
	HeaderSchemaVersion = "X-Schema-Version"
)

// Envelope represents a message envelope with metadata for routing and correlation.
type Envelope struct {
	MessageID     string          `json:"message_id"`
	CorrelationID string          `json:"correlation_id"`
	AgentID       string          `json:"agent_id,omitempty"`
	CycleID       string          `json:"cycle_id,omitempty"`
	SourceService string          `json:"source_service"`
	Timestamp     time.Time       `json:"timestamp"`
	SchemaVersion string          `json:"schema_version"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// NewEnvelope creates a new envelope with generated ID and timestamp.
func NewEnvelope(correlationID, agentID, cycleID, sourceService string) *Envelope {
	return &Envelope{
		MessageID:     GenerateMessageID(),
		CorrelationID: correlationID,
		AgentID:       agentID,
		CycleID:       cycleID,
		SourceService: sourceService,
		Timestamp:     time.Now().UTC(),
		SchemaVersion: SchemaVersion,
		Payload:       nil,
	}
}

// EnvelopeFromHeaders creates an envelope from NATS message headers.
func EnvelopeFromHeaders(msg *nats.Msg) (*Envelope, error) {
	if msg == nil {
		return nil, errors.New("nil message")
	}

	env := &Envelope{
		Timestamp:     time.Now().UTC(),
		SchemaVersion: SchemaVersion,
	}

	// Extract headers from the message
	headers := msg.Header

	// Required fields
	if v := headers.Get(HeaderMessageID); v != "" {
		env.MessageID = v
	} else {
		env.MessageID = GenerateMessageID()
	}

	if v := headers.Get(HeaderCorrelationID); v != "" {
		env.CorrelationID = v
	}

	if v := headers.Get(HeaderSourceService); v != "" {
		env.SourceService = v
	} else {
		return nil, &MessagingError{
			Code:    "INVALID_ENVELOPE",
			Message: "missing required header: X-Source-Service",
		}
	}

	// Optional fields
	env.AgentID = headers.Get(HeaderAgentID)
	env.CycleID = headers.Get(HeaderCycleID)

	// Timestamp
	if v := headers.Get(HeaderTimestamp); v != "" {
		ts, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return nil, &MessagingError{
				Code:    "INVALID_TIMESTAMP",
				Message: "invalid timestamp format in header",
				Err:     err,
			}
		}
		env.Timestamp = ts
	}

	// Schema version
	if v := headers.Get(HeaderSchemaVersion); v != "" {
		env.SchemaVersion = v
	}

	return env, nil
}

// SetHeaders sets envelope fields as NATS headers on a message.
func SetHeaders(msg *nats.Msg, env *Envelope) {
	if msg == nil || env == nil {
		return
	}

	if msg.Header == nil {
		msg.Header = make(nats.Header)
	}

	msg.Header.Set(HeaderMessageID, env.MessageID)

	if env.CorrelationID != "" {
		msg.Header.Set(HeaderCorrelationID, env.CorrelationID)
	}

	if env.AgentID != "" {
		msg.Header.Set(HeaderAgentID, env.AgentID)
	}

	if env.CycleID != "" {
		msg.Header.Set(HeaderCycleID, env.CycleID)
	}

	msg.Header.Set(HeaderSourceService, env.SourceService)
	msg.Header.Set(HeaderTimestamp, env.Timestamp.Format(time.RFC3339Nano))
	msg.Header.Set(HeaderSchemaVersion, env.SchemaVersion)
}

// GenerateMessageID returns a new UUID v4.
func GenerateMessageID() string {
	return uuid.NewString()
}

// SetPayload sets the JSON payload on the envelope.
func (e *Envelope) SetPayload(data interface{}) error {
	if data == nil {
		e.Payload = nil
		return nil
	}

	var err error
	e.Payload, err = json.Marshal(data)
	if err != nil {
		return &MessagingError{
			Code:    "MARSHAL_ERROR",
			Message: "failed to marshal payload",
			Err:     err,
		}
	}
	return nil
}

// GetPayload unmarshals the payload into the given type.
func (e *Envelope) GetPayload(v interface{}) error {
	if e.Payload == nil {
		return errors.New("no payload in envelope")
	}

	if err := json.Unmarshal(e.Payload, v); err != nil {
		return &MessagingError{
			Code:    "UNMARSHAL_ERROR",
			Message: "failed to unmarshal payload",
			Err:     err,
		}
	}
	return nil
}

// Validate checks if the envelope has all required fields.
func (e *Envelope) Validate() error {
	if e.MessageID == "" {
		return &MessagingError{
			Code:    "INVALID_ENVELOPE",
			Message: "message_id is required",
		}
	}

	if e.SourceService == "" {
		return &MessagingError{
			Code:    "INVALID_ENVELOPE",
			Message: "source_service is required",
		}
	}

	if e.SchemaVersion == "" {
		return &MessagingError{
			Code:    "INVALID_ENVELOPE",
			Message: "schema_version is required",
		}
	}

	return nil
}

// Clone creates a copy of the envelope with a new message ID.
func (e *Envelope) Clone() *Envelope {
	clone := *e
	clone.MessageID = GenerateMessageID()
	clone.Timestamp = time.Now().UTC()

	// Deep copy payload if present
	if e.Payload != nil {
		clone.Payload = make(json.RawMessage, len(e.Payload))
		copy(clone.Payload, e.Payload)
	}

	return &clone
}
