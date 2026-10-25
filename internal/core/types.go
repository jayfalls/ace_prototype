package core

import (
	"encoding/json"
	"time"
)

// LayerID identifies one of Dave Shapiro's 6 ACE layers.
type LayerID int

const (
	LayerAspirational    LayerID = 1 // L1: Constitution, heuristic imperatives, mission
	LayerGlobalStrategy  LayerID = 2 // L2: World context, user intent, strategic goals
	LayerAgentModel      LayerID = 3 // L3: Self-capabilities, sandbox state, identity
	LayerExecutive       LayerID = 4 // L4: Multi-step plans, resource & risk bounds
	LayerCognitiveCtrl   LayerID = 5 // L5: Task selection, loop guard, step dispatch
	LayerTaskProsecution LayerID = 6 // L6: Sandboxed execution, wiki updates, proactive chat
)

func AllLayers() []LayerID {
	return []LayerID{
		LayerAspirational,
		LayerGlobalStrategy,
		LayerAgentModel,
		LayerExecutive,
		LayerCognitiveCtrl,
		LayerTaskProsecution,
	}
}

func (l LayerID) Code() string {
	switch l {
	case LayerAspirational:
		return "L1"
	case LayerGlobalStrategy:
		return "L2"
	case LayerAgentModel:
		return "L3"
	case LayerExecutive:
		return "L4"
	case LayerCognitiveCtrl:
		return "L5"
	case LayerTaskProsecution:
		return "L6"
	}
	return "L0"
}

func (l LayerID) Name() string {
	switch l {
	case LayerAspirational:
		return "Aspirational"
	case LayerGlobalStrategy:
		return "Global Strategy"
	case LayerAgentModel:
		return "Agent Model"
	case LayerExecutive:
		return "Executive Function"
	case LayerCognitiveCtrl:
		return "Cognitive Control"
	case LayerTaskProsecution:
		return "Task Prosecution"
	}
	return "System"
}

func (l LayerID) RoleSummary() string {
	switch l {
	case LayerAspirational:
		return "Zone 1 (Compass): Constitution, Heuristic Imperatives & Mission (soul.md)"
	case LayerGlobalStrategy:
		return "Zone 1 (Compass): World Context, Horizon Epochs & Strategic Framing"
	case LayerAgentModel:
		return "Zone 2 (Tactical): Self-Capabilities, Sandbox State & Hardware Model"
	case LayerExecutive:
		return "Zone 2 (Tactical): Single-Pass Execution Plan & Verification Spec"
	case LayerCognitiveCtrl:
		return "Zone 2 (Tactical): Local GPU System-1 Gate (P(yes)) & Loop Guard"
	case LayerTaskProsecution:
		return "Zone 3 (Reflex): Prefix-Cached Multi-Action Pi Prosecutor & Actuators"
	}
	return "System Bus"
}

func (l LayerID) Zone() string {
	switch l {
	case LayerAspirational, LayerGlobalStrategy:
		return "zone.compass"
	case LayerAgentModel, LayerExecutive, LayerCognitiveCtrl:
		return "zone.tactical"
	case LayerTaskProsecution:
		return "zone.reflex"
	}
	return "zone.system"
}

func (l LayerID) Channel() string {
	switch l {
	case LayerAspirational, LayerGlobalStrategy:
		return "ace.channel.compass"
	case LayerAgentModel, LayerExecutive, LayerCognitiveCtrl:
		return "ace.channel.tactical"
	case LayerTaskProsecution:
		return "ace.channel.reflex"
	}
	return "ace.channel.telemetry"
}

type BusDirection string

const (
	DirectionSouthbound BusDirection = "southbound" // Directives, plans, constraints (L1 -> L6)
	DirectionNorthbound BusDirection = "northbound" // Telemetry, senses, escalations (L6 -> L1)
)

// BusEnvelope is the canonical message structure travelling on the Northbound and Southbound buses.
type BusEnvelope struct {
	ID        string       `json:"id"`
	CycleID   string       `json:"cycle_id"`
	AgentID   string       `json:"agent_id"`
	TaskID    string       `json:"task_id,omitempty"`
	Topic     string       `json:"topic"`
	Direction BusDirection `json:"direction"`
	FromLayer int          `json:"from_layer"`
	ToLayer   int          `json:"to_layer"`
	Kind      string       `json:"kind"` // directive, telemetry, escalation, sense, action_result
	Summary   string       `json:"summary"`
	Payload   string       `json:"payload"`
	CreatedAt time.Time    `json:"created_at"`
}

// LayerStatus represents the real-time observable state of one ACE layer.
type LayerStatus struct {
	LayerID          LayerID   `json:"layer_id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Role             string    `json:"role"`
	State            string    `json:"state"` // idle, thinking, acting, error
	Summary          string    `json:"summary"`
	LastThought      string    `json:"last_thought"`
	LiveStream       string    `json:"live_stream"`
	ActiveDirective  string    `json:"active_directive"`
	NorthboundSignal string    `json:"northbound_signal"`
	LastAction       string    `json:"last_action"`
	Confidence       float64   `json:"confidence"`
	Model            string    `json:"model"`
	LastLatencyMs    int64     `json:"last_latency_ms"`
	TotalInvocations int       `json:"total_invocations"`
	InnerLoopStep    int       `json:"inner_loop_step"`
	InnerLoopMax     int       `json:"inner_loop_max"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// LayerAction is a structured action requested by a cognitive layer (Pi-style tool suite in L6).
type LayerAction struct {
	Type    string `json:"type"`               // none, sandbox_sh, workspace_write, workspace_read, workspace_edit, ticket, kb, web_search, web_fetch, wiki_write, memory_remember, chat_proactive
	Target  string `json:"target"`             // command, relative file path, search query, URL, ticket/kb args, or memory tag
	Content string `json:"content"`            // file content, replacement newText, memory body, or proactive message text
	OldText string `json:"old_text,omitempty"` // exact text to replace when type == "workspace_edit"
	Done    bool   `json:"done,omitempty"`     // true when L6 Pi inner loop has verified task completion
}

// LayerDecision is the canonical JSON schema returned by every layer LLM invocation.
type LayerDecision struct {
	Thought             string          `json:"thought"`
	StateSummary        string          `json:"state_summary"`
	Confidence          float64         `json:"confidence"`
	SouthboundDirective string          `json:"southbound_directive"`
	NorthboundSignal    string          `json:"northbound_signal"`
	Action              LayerAction     `json:"action"`
	Actions             []LayerAction   `json:"actions,omitempty"`
	Contract            json.RawMessage `json:"contract,omitempty"`
}

// ThoughtTrace stores the full raw prompt, reasoning stream, and parsed decision (Meta-Harness raw trace principle).
type ThoughtTrace struct {
	ID               string    `json:"id"`
	CycleID          string    `json:"cycle_id"`
	AgentID          string    `json:"agent_id"`
	LayerID          int       `json:"layer_id"`
	LayerCode        string    `json:"layer_code"`
	LoopKind         string    `json:"loop_kind"` // cognitive, fast_chat, memory_consolidation
	Model            string    `json:"model"`
	TriggerSummary   string    `json:"trigger_summary"`
	SystemPrompt     string    `json:"system_prompt"`
	UserPrompt       string    `json:"user_prompt"`
	RawReasoning     string    `json:"raw_reasoning"`
	RawResponse      string    `json:"raw_response"`
	Thought          string    `json:"thought"`
	StateSummary     string    `json:"state_summary"`
	Southbound       string    `json:"southbound"`
	Northbound       string    `json:"northbound"`
	ActionType       string    `json:"action_type"`
	ActionTarget     string    `json:"action_target"`
	ActionResult     string    `json:"action_result"`
	Confidence       float64   `json:"confidence"`
	LatencyMs        int64     `json:"latency_ms"`
	PromptTokens     int       `json:"prompt_tokens"`
	CachedTokens     int       `json:"cached_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CreatedAt        time.Time `json:"created_at"`
}

// ChatMessage represents a message in the user <-> ACE conversation.
type ChatMessage struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`   // user, assistant, proactive
	Source    string    `json:"source"` // fast_chat, L6, user
	Content   string    `json:"content"`
	Reasoning string    `json:"reasoning"`
	Model     string    `json:"model"`
	LatencyMs int64     `json:"latency_ms"`
	CycleID   string    `json:"cycle_id"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryEntry is an episodic/semantic memory item indexed in SQLite FTS5 with layer isolation and tiering.
type MemoryEntry struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	LayerCode string    `json:"layer_code"` // L1..L6 or GLOBAL
	Tier      string    `json:"tier"`       // short, medium, long
	Category  string    `json:"category"`   // episodic, goal, observation, insight, user_fact
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      string    `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// v0.9 memory metadata (spec 9.6).
	Type       string    `json:"type"`        // episodic, semantic, procedural
	Source     string    `json:"source"`      // provenance: cycle/tool/caller that produced the fact
	VerifiedAt time.Time `json:"verified_at"` // when a semantic fact was last verified; zero = unverified
	Importance float64   `json:"importance"`  // retention weight; 0 defaults to 1
}

// TelemetrySpan represents an OpenTelemetry-style execution span persisted in SQLite.
type TelemetrySpan struct {
	SpanID           string    `json:"span_id"`
	TraceID          string    `json:"trace_id"` // matches cycle_id
	ParentSpanID     string    `json:"parent_span_id,omitempty"`
	AgentID          string    `json:"agent_id"`
	LayerCode        string    `json:"layer_code"`
	Operation        string    `json:"operation"` // layer.step, l6.pi_loop, tool.exec, northbound.reflect, safety.check, memory.consolidate
	Model            string    `json:"model,omitempty"`
	Status           string    `json:"status"` // ok, error, blocked
	DurationMs       int64     `json:"duration_ms"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	Detail           string    `json:"detail"`
	CreatedAt        time.Time `json:"created_at"`
}

// TelemetryMetrics summarizes engine observability, token usage, and safety metrics.
type TelemetryMetrics struct {
	TotalSpans          int64            `json:"total_spans"`
	TotalPromptTokens   int64            `json:"total_prompt_tokens"`
	TotalCachedTokens   int64            `json:"total_cached_tokens"`
	TotalCompletionToks int64            `json:"total_completion_tokens"`
	TotalToolCalls      int64            `json:"total_tool_calls"`
	SafetyChecksPassed  int64            `json:"safety_checks_passed"`
	SafetyInterventions int64            `json:"safety_interventions"`
	AvgLayerLatencyMs   map[string]int64 `json:"avg_layer_latency_ms"`
	SpansByOperation    map[string]int64 `json:"spans_by_operation"`
}

// GlobalLoopStatus reports the state of ACE's 5 HRM Global Loops from the original architecture.
type GlobalLoopStatus struct {
	Name        string    `json:"name"`      // Fast Chat, Safety Monitor, Pi Task Prosecutor, Learning Loop, Memory Manager
	Frequency   string    `json:"frequency"` // fast, constant, inner-loop, post-cycle, periodic
	State       string    `json:"state"`     // active, healthy, idle, blocked
	LastSummary string    `json:"last_summary"`
	RunCount    int64     `json:"run_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WikiPageMeta describes a markdown file in ACE's Karpathy-style LLM Wiki (soul.md, index.md, log.md, notes/*.md).
type WikiPageMeta struct {
	Path      string    `json:"path"`
	Title     string    `json:"title"`
	SizeBytes int64     `json:"size_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkspaceFile describes a file inside the jailed /workspace directory.
type WorkspaceFile struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExecResult is the outcome of a sandboxed bwrap command execution.
type ExecResult struct {
	Command    string    `json:"command"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	ExitCode   int       `json:"exit_code"`
	DurationMs int64     `json:"duration_ms"`
	Isolated   bool      `json:"isolated"`
	ExecutedAt time.Time `json:"executed_at"`
}

// SandboxStatus reports the verified security state of the OS/Kernel sandbox.
type SandboxStatus struct {
	Healthy          bool      `json:"healthy"`
	BwrapAvailable   bool      `json:"bwrap_available"`
	BwrapVersion     string    `json:"bwrap_version"`
	OSRootActive     bool      `json:"os_root_active"`
	TraversalBlocked bool      `json:"traversal_blocked"`
	HomeHidden       bool      `json:"home_hidden"`
	NetworkUnshared  bool      `json:"network_unshared"`
	SandboxDir       string    `json:"sandbox_dir"`
	LastVerifiedAt   time.Time `json:"last_verified_at"`
	VerificationLog  string    `json:"verification_log"`
}

// EngineMode controls how the cognitive loops advance.
type EngineMode string

const (
	ModeReactive   EngineMode = "reactive"   // Runs cognitive cascade on stimulus (chat/nudge), then idles
	ModeManualStep EngineMode = "manual"     // Advances 1 layer or 1 cycle only when user clicks Step/Cycle
	ModeAutonomous EngineMode = "autonomous" // Runs periodic autonomous cognitive pulses + reactive
	ModePaused     EngineMode = "paused"     // Cognitive cascade paused; Fast Chat still available
)

// EngineStatus is the top-level runtime snapshot sent to the Web Cockpit.
type EngineStatus struct {
	AgentID             string             `json:"agent_id"`
	ConstitutionVersion string             `json:"constitution_version"` // pinned constitution hash prefix (plan 8.1)
	MissionVersion      string             `json:"mission_version"`      // content hash of owner mission.md
	Mode                EngineMode         `json:"mode"`
	AutoIntervalSeconds int                `json:"auto_interval_seconds"`
	ActiveModel         string             `json:"active_model"`
	FallbackModels      []string           `json:"fallback_models"`
	Busy                bool               `json:"busy"`
	CurrentCycleID      string             `json:"current_cycle_id"`
	CurrentLayer        int                `json:"current_layer"`
	PendingQueueDepth   int                `json:"pending_queue_depth"`
	TotalCycles         int                `json:"total_cycles"`
	TotalThoughts       int                `json:"total_thoughts"`
	Layers              []LayerStatus      `json:"layers"`
	GlobalLoops         []GlobalLoopStatus `json:"global_loops"`
	Metrics             TelemetryMetrics   `json:"metrics"`
	Sandbox             SandboxStatus      `json:"sandbox"`
}

// LiveEvent is broadcast over the WebSocket hub to all connected Web Cockpits.
type LiveEvent struct {
	Type      string      `json:"type"` // engine.status, layer.status, layer.stream, bus.envelope, thought.recorded, chat.delta, chat.message, wiki.updated, sandbox.exec
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}
