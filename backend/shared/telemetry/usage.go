package telemetry

import "time"

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
	OperationTypeLLMCall        = "llm_call"
	OperationTypeMemoryRead     = "memory_read"
	OperationTypeMemoryWrite    = "memory_write"
	OperationTypeToolExecute    = "tool_execute"
	OperationTypeDBQuery        = "db_query"
	OperationTypeNATSPublish    = "nats_publish"
	OperationTypeNATSSubscribe  = "nats_subscribe"
)

// Resource type constants
const (
	ResourceTypeAPI        = "api"
	ResourceTypeMemory    = "memory"
	ResourceTypeTool      = "tool"
	ResourceTypeDatabase  = "database"
	ResourceTypeMessaging = "messaging"
)
