package telemetry

import (
	"encoding/json"
	"errors"
)

// ErrNATSNotConnected is returned when NATS is not connected
var ErrNATSNotConnected = errors.New("nats: connection not available")

// SpanAttributes defines the mandatory span attributes for agent work
type SpanAttributes struct {
	AgentID     string
	CycleID     string
	ServiceName string
}

// MarshalJSON implements custom JSON marshaling for SpanAttributes
func (s SpanAttributes) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"agent_id":     s.AgentID,
		"cycle_id":     s.CycleID,
		"service_name": s.ServiceName,
	})
}
