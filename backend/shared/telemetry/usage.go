package telemetry

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

// UsageEvent represents a tracked resource consumption event
type UsageEvent struct {
	Timestamp     time.Time         `json:"timestamp"`
	AgentID       string            `json:"agent_id"`
	CycleID       string            `json:"cycle_id"`
	SessionID     string            `json:"session_id"`
	ServiceName   string            `json:"service_name"`
	OperationType string            `json:"operation_type"`
	ResourceType  string            `json:"resource_type"`
	CostUSD       float64           `json:"cost_usd,omitempty"`
	DurationMs    int64             `json:"duration_ms,omitempty"`
	TokenCount    int64             `json:"token_count,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Operation type constants
const (
	OperationTypeLLMCall       = "llm_call"
	OperationTypeMemoryRead    = "memory_read"
	OperationTypeMemoryWrite   = "memory_write"
	OperationTypeToolExecute   = "tool_execute"
	OperationTypeDBQuery       = "db_query"
	OperationTypeNATSPublish   = "nats_publish"
	OperationTypeNATSSubscribe = "nats_subscribe"
)

// Resource type constants
const (
	ResourceTypeAPI       = "api"
	ResourceTypeMemory    = "memory"
	ResourceTypeTool      = "tool"
	ResourceTypeDatabase  = "database"
	ResourceTypeMessaging = "messaging"
)

// UsagePublisher publishes usage events to NATS
type UsagePublisher struct {
	nc *nats.Conn
}

// NewUsagePublisher creates a new usage event publisher
func NewUsagePublisher(nc *nats.Conn) *UsagePublisher {
	return &UsagePublisher{nc: nc}
}

// Publish emits a usage event to NATS
func (p *UsagePublisher) Publish(ctx context.Context, event UsageEvent) error {
	if p.nc == nil {
		return ErrNATSNotConnected
	}

	event.Timestamp = time.Now().UTC()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &nats.Msg{
		Subject: SubjectUsageEvent,
		Data:    data,
	}

	// Inject trace context if available
	InjectTraceContext(ctx, msg)

	return p.nc.PublishMsg(msg)
}

// LLMCall publishes an LLM call usage event
func (p *UsagePublisher) LLMCall(ctx context.Context, agentID, cycleID, sessionID, service string, tokens int64, costUSD float64, durationMs int64) error {
	return p.Publish(ctx, UsageEvent{
		AgentID:       agentID,
		CycleID:       cycleID,
		SessionID:     sessionID,
		ServiceName:   service,
		OperationType: OperationTypeLLMCall,
		ResourceType:  ResourceTypeAPI,
		TokenCount:    tokens,
		CostUSD:       costUSD,
		DurationMs:    durationMs,
	})
}

// MemoryRead publishes a memory read usage event
func (p *UsagePublisher) MemoryRead(ctx context.Context, agentID, cycleID, sessionID, service string, durationMs int64) error {
	return p.Publish(ctx, UsageEvent{
		AgentID:       agentID,
		CycleID:       cycleID,
		SessionID:     sessionID,
		ServiceName:   service,
		OperationType: OperationTypeMemoryRead,
		ResourceType:  ResourceTypeMemory,
		DurationMs:    durationMs,
	})
}

// ToolExecute publishes a tool execution usage event
func (p *UsagePublisher) ToolExecute(ctx context.Context, agentID, cycleID, sessionID, service, toolName string, durationMs int64) error {
	return p.Publish(ctx, UsageEvent{
		AgentID:       agentID,
		CycleID:       cycleID,
		SessionID:     sessionID,
		ServiceName:   service,
		OperationType: OperationTypeToolExecute,
		ResourceType:  ResourceTypeTool,
		DurationMs:    durationMs,
		Metadata:      map[string]string{"tool_name": toolName},
	})
}
