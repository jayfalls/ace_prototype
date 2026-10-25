package core

import (
	"context"
	"encoding/json"
)

// ModelInfo describes an LLM model available through a Provider, including circuit-breaker health.
type ModelInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Modalities   []string `json:"modalities"`
	Reasoning    bool     `json:"reasoning"`
	ContextSize  int      `json:"context_size"`
	OutputSize   int      `json:"output_size"`
	CircuitState string   `json:"circuit_state"` // closed (healthy), open (cooldown)
	SuccessCount int64    `json:"success_count"`
	FailureCount int64    `json:"failure_count"`
	AvgLatencyMs int64    `json:"avg_latency_ms"`
}

// ChatTurn is a single message in a completion request.
type ChatTurn struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// CompletionRequest is a provider-agnostic LLM request.
type CompletionRequest struct {
	Model        string     `json:"model"`
	SystemPrompt string     `json:"system_prompt"`
	Messages     []ChatTurn `json:"messages"`
	MaxTokens    int        `json:"max_tokens"`
	Temperature  float64    `json:"temperature"`
}

// StreamDelta is emitted live as tokens arrive from the LLM.
type StreamDelta struct {
	ContentDelta   string `json:"content_delta"`
	ReasoningDelta string `json:"reasoning_delta"`
}

// CompletionResponse is the final aggregated response from an LLM call.
type CompletionResponse struct {
	Model            string `json:"model"`
	Content          string `json:"content"`
	Reasoning        string `json:"reasoning"`
	PromptTokens     int    `json:"prompt_tokens"`
	CachedTokens     int    `json:"cached_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
}

// LLMProvider is the modular port for model inference (OpenCode Free by default).
type LLMProvider interface {
	Name() string
	Models() []ModelInfo
	ActiveModel() string
	SetActiveModel(modelID string) error
	StreamComplete(ctx context.Context, req CompletionRequest, onDelta func(StreamDelta)) (CompletionResponse, error)
}

// Bus is the modular port for Northbound and Southbound inter-layer messaging.
type Bus interface {
	Publish(ctx context.Context, env BusEnvelope) error
	Subscribe(handler func(BusEnvelope)) (unsubscribe func())
	Recent(limit int) ([]BusEnvelope, error)
}

// MemoryStore is the modular port for episodic FTS5 memory, chat history, and the LLM Wiki.
type MemoryStore interface {
	Remember(ctx context.Context, entry MemoryEntry) (MemoryEntry, error)
	Search(ctx context.Context, query string, limit int) ([]MemoryEntry, error)
	SearchForLayer(ctx context.Context, layerCode string, query string, limit int) ([]MemoryEntry, error)
	RecentMemories(ctx context.Context, limit int) ([]MemoryEntry, error)

	ReadWiki(relPath string) (string, error)
	WriteWiki(relPath string, content string) error
	ListWiki() ([]WikiPageMeta, error)

	SaveChatMessage(ctx context.Context, msg ChatMessage) error
	RecentChatMessages(ctx context.Context, limit int) ([]ChatMessage, error)

	SaveLayerStatus(ctx context.Context, st LayerStatus) error
	LoadLayerStatuses(ctx context.Context) ([]LayerStatus, error)
	SaveEngineMeta(ctx context.Context, key string, value string) error
	LoadEngineMeta(ctx context.Context, key string) (string, error)
}

// Sandbox is the modular port for kernel/namespace-isolated file and shell execution.
type Sandbox interface {
	Verify(ctx context.Context) SandboxStatus
	Status() SandboxStatus
	Exec(ctx context.Context, command string) (ExecResult, error)
	ReadFile(relPath string) (string, error)
	EditFile(relPath string, oldText string, newText string) error
	WriteFile(relPath string, content string) error
	DiagnoseFile(ctx context.Context, relPath string) string
	ListFiles() ([]WorkspaceFile, error)
	RecentExecs(limit int) []ExecResult
	SpilloverTruncate(kind string, raw string, maxChars int) string
}

// TelemetrySink is the modular port for storing raw thought traces, OTel spans, and broadcasting live events.
type TelemetrySink interface {
	RecordThought(ctx context.Context, trace ThoughtTrace) error
	RecentThoughts(ctx context.Context, limit int, layerFilter int) ([]ThoughtTrace, error)
	GetThought(ctx context.Context, id string) (*ThoughtTrace, error)
	RecordSpan(ctx context.Context, span TelemetrySpan) error
	RecentSpans(ctx context.Context, limit int, traceID string) ([]TelemetrySpan, error)
	Metrics(ctx context.Context) TelemetryMetrics
	EmitLive(eventType string, payload interface{})
	SubscribeLive(handler func(LiveEvent)) (unsubscribe func())
}

// Sense is a pluggable sensory input that feeds Northbound signals into the cognitive engine.
type Sense interface {
	Name() string
	Start(ctx context.Context, bus Bus) error
}

// ToolResult is one tool invocation's output.
type ToolResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// Tool is the modular port for an agent-invokable capability (spec 9.1).
// ReadOnly tools may run without a gate verdict; mutating tools go through L5.
type Tool interface {
	Name() string
	Schema() json.RawMessage
	ReadOnly() bool
	Call(ctx context.Context, args json.RawMessage) (ToolResult, error)
}

// ChannelMsg is one outbound message on a channel.
type ChannelMsg struct {
	Target  string `json:"target"`
	Content string `json:"content"`
}

// Channel is the modular port for an external message surface (Telegram, …).
type Channel interface {
	Name() string
	Start(ctx context.Context, bus Bus) error
	Send(ctx context.Context, msg ChannelMsg) error
}

// Gate is the modular port for the tiered L1 gate (wraps internal/gate).
type Gate interface {
	Decide(ctx context.Context, action string, layer int) (verdict string, reason string)
}

// ContextBlock is one injectable prompt block.
type ContextBlock struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// ContextProvider is the modular port for per-layer prompt injection.
type ContextProvider interface {
	Name() string
	Inputs(layer int) []ContextBlock
}
