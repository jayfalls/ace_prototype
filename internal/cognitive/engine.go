package cognitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ace/internal/core"
	"ace/internal/llm"
	"ace/internal/sandbox"
	"github.com/google/uuid"
)

const (
	defaultAgentID = "ace-primary"
	maxPiLoopSteps = 3
)

type cycleJob struct {
	CycleID    string
	Trigger    string
	StartLayer core.LayerID
	MaxHops    int
}

// Engine orchestrates the 6-layer ACE cognitive hierarchy, Northbound/Southbound bus propagation,
// Pi-style L6 Inner Task Prosecutor Loop, 5 HRM Global Loops, and Reactive / Manual Step / Autonomous execution modes.
type Engine struct {
	mu                  sync.RWMutex
	agentID             string
	llm                 core.LLMProvider
	bus                 core.Bus
	memory              core.MemoryStore
	sandbox             core.Sandbox
	telem               core.TelemetrySink
	mode                core.EngineMode
	autoIntervalSeconds int
	layers              map[core.LayerID]*core.LayerStatus
	globalLoops         map[string]*core.GlobalLoopStatus
	busy                bool
	currentCycleID      string
	currentLayer        int
	totalCycles          int
	totalThoughts        int
	jobQueue             chan cycleJob
	cancelCurrent        context.CancelFunc
	lastL6ReconMilestone string
	lastL6ReconTurns     []core.ChatTurn
	l6MutatedInCycle     bool
}

func NewEngine(
	llm core.LLMProvider,
	bus core.Bus,
	mem core.MemoryStore,
	sbx core.Sandbox,
	telem core.TelemetrySink,
) *Engine {
	now := time.Now().UTC()
	layers := make(map[core.LayerID]*core.LayerStatus, 6)
	for _, id := range core.AllLayers() {
		maxInner := 1
		if id == core.LayerExecutive {
			maxInner = 2
		} else if id == core.LayerTaskProsecution {
			maxInner = maxPiLoopSteps
		}
		layers[id] = &core.LayerStatus{
			LayerID:      id,
			Code:         id.Code(),
			Name:         id.Name(),
			Role:         id.RoleSummary(),
			State:        "idle",
			Summary:      "Ready — awaiting stimulus on Northbound/Southbound bus.",
			Confidence:   1.0,
			Model:        llm.ActiveModel(),
			InnerLoopMax: maxInner,
			UpdatedAt:    now,
		}
	}

	globalLoops := map[string]*core.GlobalLoopStatus{
		"fast_chat": {
			Name:        "Fast Chat",
			Frequency:   "fast",
			State:       "healthy",
			LastSummary: "Bidirectional user interface grounded in 6-layer state & FTS5 memory",
			UpdatedAt:   now,
		},
		"safety_monitor": {
			Name:        "Safety Monitor",
			Frequency:   "constant",
			State:       "healthy",
			LastSummary: "Enforcing L1 constitution, SSRF block, and bwrap/os.Root confinement",
			UpdatedAt:   now,
		},
		"swarm_coordinator": {
			Name:        "Swarm Coordinator",
			Frequency:   "parallel-fanout",
			State:       "healthy",
			LastSummary: "Concurrent goroutine worker fan-out for parallel_research & L4/L6 sub-tasks",
			UpdatedAt:   now,
		},
		"pi_prosecutor": {
			Name:        "Pi Task Prosecutor",
			Frequency:   "inner-loop (max 4)",
			State:       "healthy",
			LastSummary: "Multi-step Read/Edit/Write/Exec/Ticket/KB + Verification loop in L6",
			UpdatedAt:   now,
		},
		"learning_loop": {
			Name:        "Learning Loop",
			Frequency:   "post-cycle",
			State:       "healthy",
			LastSummary: "Bidirectional Northbound (L6->L1) outcome feedback & adaptation",
			UpdatedAt:   now,
		},
		"memory_manager": {
			Name:        "Memory Manager",
			Frequency:   "periodic",
			State:       "healthy",
			LastSummary: "Layer-isolated (short/medium/long) + GLOBAL FTS5 & Wiki consolidation",
			UpdatedAt:   now,
		},
	}

	return &Engine{
		agentID:             defaultAgentID,
		llm:                 llm,
		bus:                 bus,
		memory:              mem,
		sandbox:             sbx,
		telem:               telem,
		mode:                core.ModeAutonomous,
		autoIntervalSeconds: 20,
		layers:              layers,
		globalLoops:         globalLoops,
		jobQueue:            make(chan cycleJob, 32),
	}
}

func (e *Engine) Start(ctx context.Context) {
	e.HydrateFromHistory(ctx)
	go e.workerLoop(ctx)
	go e.autonomousTickerLoop(ctx)
}

type sessionPersistable interface {
	SessionID() string
	SetSessionID(id string)
}

func (e *Engine) persistAllLayersAndMeta(ctx context.Context) {
	e.mu.RLock()
	snaps := make([]core.LayerStatus, 0, len(e.layers))
	for _, id := range core.AllLayers() {
		if st, ok := e.layers[id]; ok {
			snaps = append(snaps, *st)
		}
	}
	mode := string(e.mode)
	interval := fmt.Sprintf("%d", e.autoIntervalSeconds)
	cycles := fmt.Sprintf("%d", e.totalCycles)
	loopsJSON, _ := json.Marshal(e.globalLoops)
	e.mu.RUnlock()

	for _, s := range snaps {
		_ = e.memory.SaveLayerStatus(ctx, s)
	}
	_ = e.memory.SaveEngineMeta(ctx, "mode", mode)
	_ = e.memory.SaveEngineMeta(ctx, "auto_interval_seconds", interval)
	_ = e.memory.SaveEngineMeta(ctx, "total_cycles", cycles)
	if len(loopsJSON) > 0 {
		_ = e.memory.SaveEngineMeta(ctx, "global_loops", string(loopsJSON))
	}
	if sp, ok := e.llm.(sessionPersistable); ok {
		_ = e.memory.SaveEngineMeta(ctx, "opencode_session_id", sp.SessionID())
	}
}

// HydrateFromHistory restores all 6 layer states, engine mode, global loops, and OpenCode session ID
// from SQLite (`layer_states` and `engine_meta`), falling back to `thought_traces` on first migration.
func (e *Engine) HydrateFromHistory(ctx context.Context) {
	// 1. Restore persisted OpenCode session ID so upstream KV prefix cache affinity survives restarts
	if sp, ok := e.llm.(sessionPersistable); ok {
		if savedSess, err := e.memory.LoadEngineMeta(ctx, "opencode_session_id"); err == nil && strings.TrimSpace(savedSess) != "" {
			sp.SetSessionID(savedSess)
		} else {
			_ = e.memory.SaveEngineMeta(ctx, "opencode_session_id", sp.SessionID())
		}
	}

	// 2. Restore persisted mode, interval, total_cycles, and global_loops
	e.mu.Lock()
	if savedMode, err := e.memory.LoadEngineMeta(ctx, "mode"); err == nil && savedMode != "" {
		e.mode = core.EngineMode(savedMode)
	}
	if savedInt, err := e.memory.LoadEngineMeta(ctx, "auto_interval_seconds"); err == nil {
		var iv int
		if _, sErr := fmt.Sscanf(savedInt, "%d", &iv); sErr == nil && iv >= 5 && iv <= 600 {
			e.autoIntervalSeconds = iv
		}
	}
	if savedCyc, err := e.memory.LoadEngineMeta(ctx, "total_cycles"); err == nil {
		var cyc int
		if _, sErr := fmt.Sscanf(savedCyc, "%d", &cyc); sErr == nil && cyc > 0 {
			e.totalCycles = cyc
		}
	}
	if savedLoops, err := e.memory.LoadEngineMeta(ctx, "global_loops"); err == nil && savedLoops != "" {
		var restored map[string]*core.GlobalLoopStatus
		if json.Unmarshal([]byte(savedLoops), &restored) == nil {
			for k, v := range restored {
				if v != nil {
					e.globalLoops[k] = v
				}
			}
		}
	}
	e.mu.Unlock()

	// 3. Restore persisted layer_states table (includes L5 4-Bit GPU Gate state!)
	savedLayers, lErr := e.memory.LoadLayerStatuses(ctx)
	traces, _ := e.telem.RecentThoughts(ctx, 100, 0)

	e.mu.Lock()
	if len(traces) > 0 {
		e.totalThoughts = len(traces)
	}
	if lErr == nil && len(savedLayers) > 0 {
		for _, s := range savedLayers {
			if target, ok := e.layers[s.LayerID]; ok {
				restored := s
				if restored.State == "thinking" {
					restored.State = "idle"
				}
				// Keep latest thought from thought_traces if available
				for _, t := range traces {
					if t.LayerID == int(s.LayerID) {
						restored.LastThought = t.Thought
						break
					}
				}
				*target = restored
			}
		}
		e.mu.Unlock()
		return
	}

	// First-time fallback migration from thought_traces if layer_states table was empty
	seenCycles := make(map[string]bool)
	seenLayer := make(map[int]bool)
	for _, t := range traces {
		if t.CycleID != "" {
			seenCycles[t.CycleID] = true
		}
		lid := core.LayerID(t.LayerID)
		st, ok := e.layers[lid]
		if !ok {
			continue
		}
		st.TotalInvocations++
		if !seenLayer[t.LayerID] {
			seenLayer[t.LayerID] = true
			st.Summary = t.StateSummary
			st.LastThought = t.Thought
			st.ActiveDirective = t.Southbound
			st.NorthboundSignal = t.Northbound
			if t.ActionType != "" && t.ActionType != "none" && !strings.Contains(t.ActionResult, "restricted to L6") {
				st.LastAction = fmt.Sprintf("%s(%s): %s", t.ActionType, truncateText(t.ActionTarget, 50), truncateText(t.ActionResult, 180))
			}
			st.Confidence = t.Confidence
			st.Model = t.Model
			st.LastLatencyMs = t.LatencyMs
			st.UpdatedAt = t.CreatedAt
		}
	}
	if e.totalCycles == 0 {
		e.totalCycles = len(seenCycles)
	}
	e.mu.Unlock()
	e.persistAllLayersAndMeta(ctx)
}

func (e *Engine) Status() core.EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	layerSlice := make([]core.LayerStatus, 0, 6)
	for _, id := range core.AllLayers() {
		st := *e.layers[id]
		layerSlice = append(layerSlice, st)
	}

	activeModel := e.llm.ActiveModel()
	var fallbacks []string
	for _, m := range e.llm.Models() {
		if m.ID != activeModel {
			fallbacks = append(fallbacks, m.ID)
		}
	}

	loopOrder := []string{"fast_chat", "safety_monitor", "swarm_coordinator", "pi_prosecutor", "learning_loop", "memory_manager"}
	loops := make([]core.GlobalLoopStatus, 0, len(loopOrder))
	for _, k := range loopOrder {
		if gl, ok := e.globalLoops[k]; ok {
			loops = append(loops, *gl)
		}
	}

	return core.EngineStatus{
		AgentID:             e.agentID,
		Mode:                e.mode,
		AutoIntervalSeconds: e.autoIntervalSeconds,
		ActiveModel:         activeModel,
		FallbackModels:      fallbacks,
		Busy:                e.busy,
		CurrentCycleID:      e.currentCycleID,
		CurrentLayer:        e.currentLayer,
		PendingQueueDepth:   len(e.jobQueue),
		TotalCycles:         e.totalCycles,
		TotalThoughts:       e.totalThoughts,
		Layers:              layerSlice,
		GlobalLoops:         loops,
		Metrics:             e.telem.Metrics(context.Background()),
		Sandbox:             e.sandbox.Status(),
	}
}

func (e *Engine) recordGlobalLoop(key string, state string, summary string) {
	e.mu.Lock()
	if gl, ok := e.globalLoops[key]; ok {
		gl.State = state
		gl.LastSummary = truncateText(summary, 140)
		gl.RunCount++
		gl.UpdatedAt = time.Now().UTC()
	}
	loopsJSON, _ := json.Marshal(e.globalLoops)
	e.mu.Unlock()
	if len(loopsJSON) > 0 {
		_ = e.memory.SaveEngineMeta(context.Background(), "global_loops", string(loopsJSON))
	}
}

func (e *Engine) SetMode(mode core.EngineMode, intervalSeconds int) core.EngineStatus {
	e.mu.Lock()
	switch mode {
	case core.ModeReactive, core.ModeManualStep, core.ModeAutonomous, core.ModePaused:
		e.mode = mode
	}
	if intervalSeconds >= 5 && intervalSeconds <= 600 {
		e.autoIntervalSeconds = intervalSeconds
	}
	if mode == core.ModePaused && e.cancelCurrent != nil {
		e.cancelCurrent()
	}
	curMode := string(e.mode)
	curInt := fmt.Sprintf("%d", e.autoIntervalSeconds)
	e.mu.Unlock()

	_ = e.memory.SaveEngineMeta(context.Background(), "mode", curMode)
	_ = e.memory.SaveEngineMeta(context.Background(), "auto_interval_seconds", curInt)

	st := e.Status()
	e.telem.EmitLive("engine.status", st)
	return st
}

func (e *Engine) StopCurrentCycle() core.EngineStatus {
	e.mu.Lock()
	if e.cancelCurrent != nil {
		e.cancelCurrent()
	}
	for len(e.jobQueue) > 0 {
		<-e.jobQueue
	}
	e.mu.Unlock()
	st := e.Status()
	e.telem.EmitLive("engine.status", st)
	return st
}

// Chat executes the Fast Chat Loop immediately (streaming tokens to the UI) and
// simultaneously emits a Northbound sense event to trigger cognitive reflection if in Reactive/Autonomous mode.
func (e *Engine) Chat(ctx context.Context, userContent string) (core.ChatMessage, error) {
	userContent = strings.TrimSpace(userContent)
	if userContent == "" {
		return core.ChatMessage{}, errors.New("message content cannot be empty")
	}

	cycleID := "cyc_" + uuid.NewString()[:8]
	userMsg := core.ChatMessage{
		ID:        "cht_" + uuid.NewString()[:12],
		AgentID:   e.agentID,
		Role:      "user",
		Source:    "user",
		Content:   userContent,
		CycleID:   cycleID,
		CreatedAt: time.Now().UTC(),
	}
	if err := e.memory.SaveChatMessage(ctx, userMsg); err != nil {
		return core.ChatMessage{}, err
	}

	// Publish Northbound sense event from user chat
	_ = e.bus.Publish(ctx, core.BusEnvelope{
		CycleID:   cycleID,
		AgentID:   e.agentID,
		Topic:     "ace.bus.north.sense.user_chat",
		Direction: core.DirectionNorthbound,
		FromLayer: 6,
		ToLayer:   1,
		Kind:      "sense",
		Summary:   "User chat: " + truncateText(userContent, 90),
		Payload:   userContent,
	})

	// Build Fast Chat prompt with a STATIC SystemPrompt (for >85% KV prefix cache hit across turns & tool steps)
	// and attach dynamic turn state (layers, memories, recent execs) to the final user turn!
	soul, _ := e.memory.ReadWiki("soul.md")
	index := e.buildCanonicalIndexWithSkills()
	goals, _ := e.memory.ReadWiki("goals.md")
	st := e.Status()
	memories, _ := e.memory.Search(ctx, userContent, 5)
	wikiPages, _ := e.memory.ListWiki()
	wsFiles, _ := e.sandbox.ListFiles()
	history, _ := e.memory.RecentChatMessages(ctx, 10)

	sysPrompt := FastChatSystemPrompt(soul, index, extractActiveGoals(goals), wikiPages, wsFiles)
	turnCtx := FastChatTurnContext(st.Layers, memories, e.sandbox.RecentExecs(4))

	turns := make([]core.ChatTurn, 0, len(history))
	for _, h := range history {
		if h.Role != "user" && h.Role != "assistant" {
			continue // Never feed proactive tool logs into conversational chat history!
		}
		if strings.Contains(h.Content, "⚡ Executed in `bwrap` sandbox") || strings.HasPrefix(h.Content, "Ran `workspace_read") {
			continue
		}
		turns = append(turns, core.ChatTurn{
			Role:    h.Role,
			Content: h.Content,
		})
	}
	// Attach live cognitive context to the final user message so SystemPrompt + all prior turns stay 100% KV-cached!
	if len(turns) > 0 && turns[len(turns)-1].Role == "user" {
		turns[len(turns)-1].Content = turnCtx + "\nUser Message from Jay:\n" + turns[len(turns)-1].Content
	} else {
		turns = append(turns, core.ChatTurn{
			Role:    "user",
			Content: turnCtx + "\nUser Message from Jay:\n" + userContent,
		})
	}

	msgID := "cht_" + uuid.NewString()[:12]
	var resp core.CompletionResponse
	var err error
	var executedActions []string
	var lastAct core.LayerAction
	var lastActResult string

	// Agentic Chat Loop (up to 2 batched tool turns + 1 guaranteed natural synthesis pass!)
	for chatStep := 1; chatStep <= 3; chatStep++ {
		resp, err = e.llm.StreamComplete(ctx, core.CompletionRequest{
			SystemPrompt: sysPrompt,
			Messages:     turns,
			MaxTokens:    8192,
			Temperature:  0.4,
		}, func(d core.StreamDelta) {
			e.telem.EmitLive("chat.delta", map[string]string{
				"id":              msgID,
				"cycle_id":        cycleID,
				"content_delta":   d.ContentDelta,
				"reasoning_delta": d.ReasoningDelta,
			})
		})
		if err != nil {
			return core.ChatMessage{}, err
		}

		rawOut := resp.Content
		if strings.TrimSpace(rawOut) == "" && strings.TrimSpace(resp.Reasoning) != "" {
			rawOut = resp.Reasoning
		}

		acts := ParseChatActions(rawOut)
		if len(acts) == 0 {
			break
		}

		// Execute ALL <action> blocks emitted in this turn in one batch (Hermes batched pattern)
		var batchOutputs []string
		for _, act := range acts {
			lastAct = act
			resOut := e.executeLayerAction(ctx, cycleID, core.LayerTaskProsecution, act, resp.Model)
			lastActResult = resOut
			executedActions = append(executedActions, fmt.Sprintf("%s(%s)", act.Type, truncateText(act.Target, 40)))
			batchOutputs = append(batchOutputs, fmt.Sprintf("=== Tool %s(%s) ===\n%s", act.Type, act.Target, truncateText(resOut, 5000)))
		}

		synthDirective := "Now respond directly to Jay in your sharp co-founder voice with the concrete answer. If you already have what you need, do NOT emit any more <action> blocks."
		if chatStep >= 2 {
			synthDirective = "FINAL SYNTHESIS PASS: You have already executed the sandbox tools above. Now write your direct, natural conversational response to Jay. Do NOT emit any <action> blocks."
		}

		turns = append(turns,
			core.ChatTurn{Role: "assistant", Content: rawOut},
			core.ChatTurn{
				Role:    "user",
				Content: strings.Join(batchOutputs, "\n\n") + "\n\n" + synthDirective,
			},
		)
	}

	cleanChatContent := StripChatAction(StripDSML(resp.Content))
	if idx := strings.Index(cleanChatContent, "⚡ Executed in `bwrap` sandbox"); idx >= 0 {
		cleanChatContent = strings.TrimSpace(cleanChatContent[:idx])
	}
	if cleanChatContent == "" && strings.TrimSpace(resp.Reasoning) != "" {
		cleanChatContent = StripChatAction(StripDSML(resp.Reasoning))
	}
	// If the model STILL only emitted an <action> tag with no prose, force 1 short synthesis completion without tools
	if cleanChatContent == "" && len(executedActions) > 0 {
		turns = append(turns, core.ChatTurn{
			Role:    "user",
			Content: "Write your direct, plain-English reply to Jay summarizing what you found/did above. Do NOT emit <action> tags or raw file dumps.",
		})
		if synthResp, sErr := e.llm.StreamComplete(ctx, core.CompletionRequest{
			SystemPrompt: sysPrompt,
			Messages:     turns,
			MaxTokens:    8192,
			Temperature:  0.3,
		}, nil); sErr == nil {
			cleanChatContent = StripChatAction(StripDSML(synthResp.Content))
		}
	}
	if cleanChatContent == "" {
		cleanChatContent = "I inspected the workspace files — let me know which specific part you want me to change or run next."
	}

	replyMsg := core.ChatMessage{
		ID:        msgID,
		AgentID:   e.agentID,
		Role:      "assistant",
		Source:    "fast_chat",
		Content:   cleanChatContent,
		Reasoning: resp.Reasoning,
		Model:     resp.Model,
		LatencyMs: resp.LatencyMs,
		CycleID:   cycleID,
		CreatedAt: time.Now().UTC(),
	}
	_ = e.memory.SaveChatMessage(ctx, replyMsg)

	// Record Fast Chat trace in TelemetrySink so it's also inspectable
	chatActType := "none"
	if lastAct.Type != "" {
		chatActType = lastAct.Type
	}
	_ = e.telem.RecordThought(ctx, core.ThoughtTrace{
		CycleID:          cycleID,
		AgentID:          e.agentID,
		LayerID:          0,
		LayerCode:        "CHAT",
		LoopKind:         "fast_chat",
		Model:            resp.Model,
		TriggerSummary:   "User message: " + truncateText(userContent, 80),
		SystemPrompt:     sysPrompt,
		UserPrompt:       userContent,
		RawReasoning:     resp.Reasoning,
		RawResponse:      resp.Content,
		Thought:          resp.Reasoning,
		StateSummary:     truncateText(cleanChatContent, 120),
		Confidence:       0.95,
		ActionType:       chatActType,
		ActionTarget:     lastAct.Target,
		ActionResult:     truncateText(lastActResult, 4000),
		LatencyMs:        resp.LatencyMs,
		PromptTokens:     resp.PromptTokens,
		CachedTokens:     resp.CachedTokens,
		CompletionTokens: resp.CompletionTokens,
	})

	e.recordGlobalLoop("fast_chat", "healthy", fmt.Sprintf("Answered user chat (%dms): %s", resp.LatencyMs, truncateText(userContent, 70)))

	// Smart Fast-Chat Intent Classifier:
	// - If Agentic Chat already executed synchronous tools in this turn (len(executedActions) > 0), skip redundant background cascade!
	// - Conversational greetings / status probes -> answered 100% by Agentic Chat (0 background cascades!)
	// - Tactical execution tasks -> Fast-Path L4->L6 (L5 bypassed = 2 hops)
	// - Strategy / roadmap changes -> Parallel(L2||L3)->L4->L6 (never L1 unless soul/constitution changes)
	e.mu.RLock()
	mode := e.mode
	e.mu.RUnlock()
	if len(executedActions) == 0 && (mode == core.ModeReactive || mode == core.ModeAutonomous) {
		if shouldRun, startLayer, maxHops := classifyChatIntent(userContent); shouldRun {
			e.EnqueueCycle(fmt.Sprintf("User request from Jay: %s", userContent), startLayer, maxHops)
		}
	}

	return replyMsg, nil
}

// classifyChatIntent prevents greetings/probes from triggering noisy 6-layer cascades.
// Crucially, routine probes NEVER start at L1 (Aspirational):
// - L1 is reserved strictly for constitutional/soul.md/ethical boundary revisions.
// - Strategy/roadmap shifts start at L2 (Parallel L2||L3 -> L4 -> L6).
// - Tactical execution starts at L4 (L4 -> L6 fast-path with L5 bypassed).
func classifyChatIntent(text string) (shouldCascade bool, startLayer core.LayerID, maxHops int) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false, core.LayerExecutive, 0
	}

	// Pure greetings, acknowledgements, or status probes are already answered by Agentic Fast Chat
	conversationalExact := map[string]bool{
		"hi": true, "hello": true, "hey": true, "yo": true, "sup": true,
		"thanks": true, "thank you": true, "ok": true, "okay": true, "cool": true,
		"status": true, "status?": true, "how are you": true, "how is it going": true,
		"what's up": true, "whats up": true, "ping": true, "test": true, "so?": true,
	}
	if conversationalExact[lower] {
		return false, core.LayerExecutive, 0
	}

	// Only explicit constitutional / ethical changes ever start at L1 (Aspirational)
	constitutionalKeywords := []string{"soul.md", "constitution", "imperative", "ethics", "moral"}
	for _, kw := range constitutionalKeywords {
		if strings.Contains(lower, kw) {
			return true, core.LayerAspirational, 6
		}
	}

	// Strategic / roadmap changes start at L2 (Parallel L2||L3 -> L4 -> L6)
	strategicKeywords := []string{"goal", "mission", "strategy", "phase", "roadmap", "pivot", "venture", "business"}
	for _, kw := range strategicKeywords {
		if strings.Contains(lower, kw) {
			return true, core.LayerGlobalStrategy, 5
		}
	}

	tacticalKeywords := []string{
		"build", "create", "write", "edit", "fix", "run", "execute", "deploy",
		"search", "fetch", "research", "analyze", "model", "simulate", "verify",
		"script", "file", "wiki", "notes/", "/workspace", "ticket", "kb", "update", "implement",
	}
	for _, kw := range tacticalKeywords {
		if strings.Contains(lower, kw) {
			return true, core.LayerExecutive, 3 // L4 -> (L5 bypassed) -> L6 Fast-Path!
		}
	}

	// Short conversational messages or questions without action verbs stay in Fast Chat
	if len(lower) < 40 || strings.HasSuffix(lower, "?") {
		return false, core.LayerExecutive, 0
	}

	// For non-trivial ambiguous messages, consult the local 4-bit RTX 3050 system_one Choice Router (0 cloud tokens)
	if choiceRes, err := llm.EvaluateLocalGPUChoice(
		context.Background(),
		"ACE 3-Zone Cognitive Router",
		fmt.Sprintf("Classify this user message into the appropriate cognitive zone: %q", truncateText(text, 200)),
		map[string]string{
			"chat_only":     "Conversational discussion, opinion, or question already answered in chat",
			"compass_l2":    "High-level product strategy, roadmap, or market pivot",
			"tactical_l4":   "Multi-step feature planning and implementation",
			"reflex_direct": "Direct single-step code edit, shell command, or test execution",
		},
	); err == nil {
		switch choiceRes.Choice {
		case "chat_only":
			return false, core.LayerExecutive, 0
		case "compass_l2":
			return true, core.LayerGlobalStrategy, 5
		case "reflex_direct":
			return true, core.LayerTaskProsecution, 1
		}
	}
	return true, core.LayerExecutive, 3
}

// EnqueueCycle queues a cognitive cascade starting at startLayer for up to maxHops layers.
// Defaults to LayerExecutive (L4) rather than L1 (Aspirational) if startLayer is out of range.
func (e *Engine) EnqueueCycle(trigger string, startLayer core.LayerID, maxHops int) string {
	if startLayer < core.LayerAspirational || startLayer > core.LayerTaskProsecution {
		startLayer = core.LayerExecutive
	}
	if maxHops <= 0 || maxHops > 6 {
		maxHops = 3
	}
	cycleID := "cyc_" + uuid.NewString()[:8]
	select {
	case e.jobQueue <- cycleJob{
		CycleID:    cycleID,
		Trigger:    trigger,
		StartLayer: startLayer,
		MaxHops:    maxHops,
	}:
	default:
	}
	e.telem.EmitLive("engine.status", e.Status())
	return cycleID
}

// StepSingleLayer synchronously executes one cognitive layer (used by "Step 1 Layer" or manual layer injection).
// Defaults to LayerExecutive (L4) instead of LayerAspirational (L1) when unspecified.
func (e *Engine) StepSingleLayer(ctx context.Context, layer core.LayerID, stimulus string) (core.ThoughtTrace, error) {
	if layer < core.LayerAspirational || layer > core.LayerTaskProsecution {
		layer = core.LayerExecutive
	}
	if strings.TrimSpace(stimulus) == "" {
		e.mu.RLock()
		prevDir := e.layers[layer].ActiveDirective
		e.mu.RUnlock()
		stimulus = prevDir
		if stimulus == "" {
			stimulus = "Perform a cognitive reflection pass on current goals, wiki state, and user context."
		}
	}
	cycleID := "cyc_" + uuid.NewString()[:8]
	return e.executeLayerStep(ctx, cycleID, layer, stimulus)
}

func (e *Engine) workerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-e.jobQueue:
			runCtx, cancel := context.WithCancel(ctx)
			e.mu.Lock()
			e.busy = true
			e.currentCycleID = job.CycleID
			e.totalCycles++
			e.cancelCurrent = cancel
			e.mu.Unlock()
			e.telem.EmitLive("engine.status", e.Status())

			e.runCascade(runCtx, job)

			cancel()
			e.mu.Lock()
			e.busy = false
			e.currentCycleID = ""
			e.currentLayer = 0
			e.cancelCurrent = nil
			e.mu.Unlock()
			e.telem.EmitLive("engine.status", e.Status())
		}
	}
}

func (e *Engine) autonomousTickerLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastFinished := time.Now()
	var autoPulseCount int

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.mu.RLock()
			mode := e.mode
			interval := time.Duration(e.autoIntervalSeconds) * time.Second
			busy := e.busy
			qLen := len(e.jobQueue)
			e.mu.RUnlock()

			if busy || qLen > 0 {
				lastFinished = time.Now()
				continue
			}

			if mode == core.ModeAutonomous && time.Since(lastFinished) >= interval {
				lastFinished = time.Now()
				autoPulseCount++
				goals, _ := e.memory.ReadWiki("goals.md")
				topMilestone := extractTopActionableMilestone(goals)

				// 1. If NO actionable "- [ ]" milestone remains in goals.md (either because the user wrote a high-level goal with no checkboxes yet,
				//    or because all prior "- [ ]" milestones are complete), run a 4-hop Compass-to-Tactical Goal Decomposition & Horizon Expansion (Parallel L1||L2||L3 -> L4)!
				if topMilestone == "" {
					e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: Full Compass Horizon Expansion (L1||L2||L3 -> L4)", autoPulseCount))
					e.EnqueueCycle(
						"Zone 1 Strategic Goal Decomposition & Horizon Expansion (L1||L2||L3 -> L4): Either goals.md contains a high-level user goal that needs initial decomposition into concrete '- [ ]' milestones, or all prior '- [ ]' milestones are complete. L1 sets mission/epistemic priorities, L2 researches the domain (via local notes or web_search/web_fetch) and uses action.type=\"wiki_write\" (target=\"goals.md\") to formulate 2-3 concrete, falsifiable '- [ ]' milestones (explicitly naming target dataset paths in `/workspace/benchmarks/datasets/<name>.json` and modular code paths in `/workspace/<package>/<module>.py` — NEVER wildcard globs), and L3 specifies capability & dataset schemas.",
						core.LayerAspirational,
						4,
					)
					continue
				}

				// 2. True 6-Layer ACE Scientific State Machine (Quota & Cache Optimized):
				// - Attempt #1 of any milestone (handoff.AttemptCount == 0), or every 3rd retry (AttemptCount % 3 == 0):
				//   Run the FULL 6-Layer Scientific Cascade: Parallel(L1 Aspirational || L2 Global Strategy || L3 Agent Model) -> L4 Executive Plan -> L5 GPU Gate -> L6 Prosecutor!
				// - Attempt #2 (handoff.AttemptCount % 3 == 1):
				//   L1..L4 have already grounded the milestone and written the experiment contract; run Zone 3 Hardware-Gated Reflex (L5 GPU Gate -> L6 Prosecutor, 0 upper-layer cloud calls) so L6 keeps its warm KV prefix cache!
				// - Attempt #3 (handoff.AttemptCount % 3 == 2):
				//   Run Tactical-to-Reflex Retry Cascade (L4 Contingency Plan -> L5 GPU Gate -> L6 Prosecutor)!
				handoff := LoadHandoffPacket(ctx, e.memory, topMilestone)
				switch {
				case handoff.AttemptCount == 0 || handoff.AttemptCount%3 == 0:
					e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: Full 6-Layer Scientific Cascade (L1||L2||L3 -> L4 -> L5 -> L6)", autoPulseCount))
					e.EnqueueCycle(
						fmt.Sprintf("Full 6-Layer ACE Scientific Cascade #%d (L1 Aspirational Audit || L2 Strategy & Literature Grounding || L3 Self-Model & Dataset Spec -> L4 Project & Hypothesis Plan -> L5 Cognitive Control -> L6 Execution): %s", autoPulseCount, topMilestone),
						core.LayerAspirational,
						6,
					)
				case handoff.AttemptCount%3 == 1:
					e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: Zone 3 Hardware-Gated Reflex Retry (L5 GPU -> L6, attempt #%d)", autoPulseCount, handoff.AttemptCount+1))
					e.EnqueueCycle(
						fmt.Sprintf("Zone 3 Hardware-Gated Reflex Continuation #%d (L5 Cognitive Control GPU Gate -> L6 Execution on attempt #%d): %s", autoPulseCount, handoff.AttemptCount+1, topMilestone),
						core.LayerCognitiveCtrl,
						2,
					)
				default:
					e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: Tactical-to-Reflex Retry Cascade (L4 -> L5 -> L6, attempt #%d)", autoPulseCount, handoff.AttemptCount+1))
					e.EnqueueCycle(
						fmt.Sprintf("Zone 2->3 Scientific Retry Cascade #%d (L4 Plan & Contingency Refinement -> L5 Cognitive Control -> L6 Execution on attempt #%d): %s", autoPulseCount, handoff.AttemptCount+1, topMilestone),
						core.LayerExecutive,
						3,
					)
				}
			}
		}
	}
}

func (e *Engine) runCascade(ctx context.Context, job cycleJob) {
	cycleStart := time.Now()
	cycleSpanID := "spn_cyc_" + job.CycleID
	stimulus := job.Trigger
	currLayer := job.StartLayer
	var l6Traces []core.ThoughtTrace

	for hop := 0; hop < job.MaxHops; hop++ {
		if ctx.Err() != nil {
			return
		}
		if currLayer == core.LayerTaskProsecution {
			l6Traces = e.runPiProsecutorLoop(ctx, job.CycleID, cycleSpanID, stimulus)
			break
		}

		// Concurrency Optimization: When starting at L1 (Full 6-Layer Cascade), execute L1 (Aspirational), L2 (Global Strategy / Literature), and L3 (Agent Model / Dataset Spec) IN PARALLEL!
		if currLayer == core.LayerAspirational && hop+2 < job.MaxHops {
			mergedDir, err := e.executeParallelL1L2L3(ctx, job.CycleID, cycleSpanID, stimulus)
			if err != nil {
				return
			}
			stimulus = mergedDir
			currLayer = core.LayerExecutive // Advance directly to L4 (Hypothesis & Experiment Designer)!
			hop += 2                        // Consumed L1, L2, and L3 hops concurrently in 1 wall-clock step
			continue
		}

		// Concurrency Optimization: Execute L2 (Global Strategy) and L3 (Agent Model) IN PARALLEL!
		if currLayer == core.LayerGlobalStrategy && hop+1 < job.MaxHops {
			mergedDir, err := e.executeParallelL2L3(ctx, job.CycleID, cycleSpanID, stimulus)
			if err != nil {
				return
			}
			stimulus = mergedDir
			currLayer = core.LayerExecutive // Advance directly to L4!
			hop++                            // Consumed both L2 and L3 hops concurrently
			continue
		}

		var trace core.ThoughtTrace
		var err error
		nextDirective := stimulus
		if currLayer != core.LayerCognitiveCtrl {
			if currLayer == core.LayerExecutive {
				trace, err = e.runExecutivePlanningLoop(ctx, job.CycleID, stimulus)
			} else {
				trace, err = e.executeLayerStep(ctx, job.CycleID, currLayer, stimulus)
			}
			if err != nil {
				return
			}
			nextDirective = strings.TrimSpace(trace.Southbound)
			if nextDirective == "" {
				nextDirective = trace.StateSummary
			}
			if ar := strings.TrimSpace(trace.ActionResult); ar != "" && !strings.HasPrefix(ar, "Error:") {
				nextDirective = fmt.Sprintf("%s\n[%s Tool Finding (%s %s)]: %s", nextDirective, currLayer.Code(), trace.ActionType, truncateText(trace.ActionTarget, 60), truncateText(ar, 900))
			}
		} else {
			e.mu.RLock()
			if l4 := e.layers[core.LayerExecutive]; l4 != nil {
				if strings.TrimSpace(l4.ActiveDirective) != "" {
					nextDirective = l4.ActiveDirective
				} else if strings.TrimSpace(l4.Summary) != "" && !strings.HasPrefix(l4.Summary, "Error:") {
					nextDirective = l4.Summary + " | " + stimulus
				}
			}
			e.mu.RUnlock()
			if wsDir, _ := e.resolveSandboxDirs(); wsDir != "" {
				if rawExp, rErr := os.ReadFile(filepath.Join(wsDir, ".harness_d", "active_experiment.json")); rErr == nil {
					var expMap map[string]interface{}
					if json.Unmarshal(rawExp, &expMap) == nil {
						if planStr, ok := expMap["l4_executive_plan"].(string); ok && strings.TrimSpace(planStr) != "" {
							nextDirective = strings.TrimSpace(planStr)
						}
					}
				}
			}
		}

		// Zone 2 -> Zone 3 Hardware Gate (L5 Cognitive Control: Task Selection, Dynamic Task Switching & Frustration Damping):
		// L5 evaluates L4's project plan via the local 4-bit System-1 GPU gate (~1-50ms, 0 cloud tokens, KV trunk cached),
		// monitors frustration/failure counters (ConsecutiveNoOp, FailedAttempts), dynamically switches L6 to Diagnostic Root-Cause Mode when blocked,
		// and publishes both Southbound (to L6) and Northbound (to L4) bus signals!
		if (currLayer == core.LayerExecutive || currLayer == core.LayerCognitiveCtrl) && nextDirective != "" && hop+1 < job.MaxHops {
			pYes := 0.96
			gateModel := "d1-4bit-gpu-gate"
			var gateMs int64
			rawGoals, _ := e.memory.ReadWiki("goals.md")
			topM := truncateText(extractTopActionableMilestone(rawGoals), 160)
			handoff := LoadHandoffPacket(ctx, e.memory, topM)
			// Persist L4's active project/hypothesis plan to /workspace/.harness_d/active_experiment.json for L5/L6 grounding
			if currLayer == core.LayerExecutive {
				if wsDir, _ := e.resolveSandboxDirs(); wsDir != "" {
					harnessDir := filepath.Join(wsDir, ".harness_d")
					_ = os.MkdirAll(harnessDir, 0755)
					expPlan := map[string]interface{}{
						"cycle_id":             job.CycleID,
						"milestone":            topM,
						"l4_executive_plan":    nextDirective,
						"l4_state_summary":     trace.StateSummary,
						"consecutive_no_op":    handoff.ConsecutiveNoOp,
						"failed_attempt_count": len(handoff.FailedAttempts),
						"updated_at":           time.Now().UTC().Format(time.RFC3339),
					}
					if b, mErr := json.MarshalIndent(expPlan, "", "  "); mErr == nil {
						_ = os.WriteFile(filepath.Join(harnessDir, "active_experiment.json"), b, 0644)
					}
				}
			}
			// Keep sharedState 100% static across cycles so the GPU KV Trunk Cache hits in <2ms on every L5 gate call!
			sharedState := "ACE L5 Cognitive Control & Somatic Valence Gate | Constitutional 6-Layer Scientist Core | Verify concrete dataset + modular execution directive"
			gateQ := fmt.Sprintf("Milestone: %s\nStatement: L4 Executive Layer produced execution plan %q.\nQuestion: Is this a concrete, safe scientific execution plan ready for Layer 6?", topM, truncateText(nextDirective, 220))
			if gpuRes, gErr := llm.EvaluateLocalGPUGateWithState(ctx, sharedState, gateQ, 0.45); gErr == nil {
				pYes = gpuRes.PYes
				gateModel = gpuRes.Model
				gateMs = int64(gpuRes.LatencyMs)
			}
			lowerDir := strings.ToLower(nextDirective)
			isExplicitHalt := (trace.ActionType == "" || trace.ActionType == "none") &&
				(strings.Contains(lowerDir, "halt loop") || strings.Contains(lowerDir, "halting honestly") || strings.HasPrefix(lowerDir, "do not "))
			gateApproved := pYes >= 0.38 && !isExplicitHalt

			// L5 Dynamic Task Switching & Frustration Tolerance (Shapiro et al. Section 3.6):
			taskMode := "NORMAL_PROSECUTION"
			l5Directive := nextDirective
			if len(handoff.FailedAttempts) > 0 || handoff.ConsecutiveNoOp > 0 {
				taskMode = fmt.Sprintf("DIAGNOSTIC_RECOVERY (FailedAttempts=%d, ConsecutiveNoOp=%d)", len(handoff.FailedAttempts), handoff.ConsecutiveNoOp)
				l5Directive = fmt.Sprintf("[L5 TASK SWITCH -> %s]: Do NOT repeat previous failed commands; isolate the root cause, surgically edit the target module or dataset builder, and verify with --selftest. Plan: %s", taskMode, nextDirective)
			}

			e.mu.Lock()
			l5 := e.layers[core.LayerCognitiveCtrl]
			l5.State = "idle"
			if gateApproved {
				l5.Summary = fmt.Sprintf("L5 [%s] PASS (P(yes)=%.2f, %dms): %s", taskMode, pYes, gateMs, truncateText(nextDirective, 70))
				l5.LastAction = fmt.Sprintf("d1_4bit_gate(P=%.2f, %dms, mode=%s) -> L6 dispatch", pYes, gateMs, taskMode)
			} else {
				l5.Summary = fmt.Sprintf("L5 VETO (P(yes)=%.2f, %dms): %s", pYes, gateMs, truncateText(nextDirective, 70))
				l5.LastAction = fmt.Sprintf("d1_4bit_gate(P=%.2f, %dms) -> VETO L6 (no-op/halt directive)", pYes, gateMs)
			}
			l5.ActiveDirective = l5Directive
			l5.Confidence = pYes
			l5.Model = gateModel
			l5.LastLatencyMs = gateMs
			l5.TotalInvocations++
			l5.UpdatedAt = time.Now().UTC()
			l5Snap := *l5
			e.mu.Unlock()
			_ = e.memory.SaveLayerStatus(ctx, l5Snap)
			e.telem.EmitLive("layer.status", l5Snap)
			// Publish L5 Southbound (to L6) and Northbound (to L4) bus messages so L5 is fully wired on both buses
			_ = e.bus.Publish(ctx, core.BusEnvelope{
				CycleID:   job.CycleID,
				AgentID:   e.agentID,
				Topic:     "ace.channel.tactical.L5->L6",
				Direction: core.DirectionSouthbound,
				FromLayer: int(core.LayerCognitiveCtrl),
				ToLayer:   int(core.LayerTaskProsecution),
				Kind:      "directive",
				Summary:   truncateText(l5Directive, 120),
				Payload:   l5Directive,
			})
			_ = e.bus.Publish(ctx, core.BusEnvelope{
				CycleID:   job.CycleID,
				AgentID:   e.agentID,
				Topic:     "ace.channel.telemetry.L5->L4",
				Direction: core.DirectionNorthbound,
				FromLayer: int(core.LayerCognitiveCtrl),
				ToLayer:   int(core.LayerExecutive),
				Kind:      "telemetry",
				Summary:   fmt.Sprintf("L5 mode=%s approved=%v P(yes)=%.2f", taskMode, gateApproved, pYes),
				Payload:   fmt.Sprintf("L5 Cognitive Control status: mode=%s, approved=%v, P(yes)=%.2f, ConsecutiveNoOp=%d, FailedAttempts=%d", taskMode, gateApproved, pYes, handoff.ConsecutiveNoOp, len(handoff.FailedAttempts)),
			})
			_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
				TraceID:      job.CycleID,
				ParentSpanID: cycleSpanID,
				AgentID:      e.agentID,
				LayerCode:    "L5",
				Operation:    "gate.l5_bypass",
				Model:        gateModel,
				Status:       "ok",
				DurationMs:   gateMs,
				Detail:       fmt.Sprintf("L5 [%s] P(yes)=%.3f approved=%v (%dms, 0 cloud tokens): %s", taskMode, pYes, gateApproved, gateMs, truncateText(nextDirective, 100)),
			})
			if !gateApproved {
				// L5 System-1 Gate vetoed L6 execution because L4 emitted a halt/no-op directive!
				break
			}
			stimulus = fmt.Sprintf("Southbound Directive from L5 Cognitive Control [%s, P(yes)=%.2f]: %s", taskMode, pYes, l5Directive)
			if currLayer == core.LayerExecutive {
				hop++ // Consumed both L4 and L5 in this loop iteration
			}
			currLayer = core.LayerTaskProsecution
			continue
		}

		stimulus = fmt.Sprintf("Southbound Directive from %s: %s", currLayer.Code(), nextDirective)
		currLayer++
	}

	// Post-Cascade Global Loops: Learning Loop (Northbound L6->L1 reflection) + Memory Manager consolidation + Live v0.3 Trace DB Mirror
	e.consolidateCycleNorthbound(ctx, job.CycleID, cycleSpanID, l6Traces)
	e.syncLiveTracesToWorkspaceBenchmarks()

	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		SpanID:     cycleSpanID,
		TraceID:    job.CycleID,
		AgentID:    e.agentID,
		LayerCode:  "CYCLE",
		Operation:  "cycle.cascade",
		Model:      e.llm.ActiveModel(),
		Status:     "ok",
		DurationMs: time.Since(cycleStart).Milliseconds(),
		Detail:     truncateText(job.Trigger, 180),
	})
}

// executeParallelL1L2L3 runs Layer 1 (Epistemic & Pareto Audit), Layer 2 (Literature & Corpus Scientist),
// and Layer 3 (Self-Telemetry & Dataset Architect) concurrently in 3 goroutines (1 wall-clock hop!)
// and merges their Southbound scientific outputs for Layer 4 (Hypothesis & Experiment Designer).
func (e *Engine) executeParallelL1L2L3(
	ctx context.Context,
	cycleID string,
	parentSpanID string,
	stimulus string,
) (string, error) {
	parStart := time.Now()
	e.recordGlobalLoop("swarm_coordinator", "active", "Running L1 (Epistemic) || L2 (Literature) || L3 (Dataset/Telemetry) concurrently")

	var (
		t1, t2, t3         core.ThoughtTrace
		err1, err2, err3   error
		wg                 sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		t1, err1 = e.executeLayerStep(ctx, cycleID, core.LayerAspirational, stimulus)
	}()
	go func() {
		defer wg.Done()
		time.Sleep(350 * time.Millisecond)
		t2, err2 = e.executeLayerStep(ctx, cycleID, core.LayerGlobalStrategy, stimulus)
	}()
	go func() {
		defer wg.Done()
		time.Sleep(700 * time.Millisecond)
		t3, err3 = e.executeLayerStep(ctx, cycleID, core.LayerAgentModel, stimulus)
	}()
	wg.Wait()

	if err1 != nil && err2 != nil && err3 != nil {
		return "", err1
	}

	dir1 := strings.TrimSpace(t1.Southbound)
	if dir1 == "" {
		dir1 = t1.StateSummary
	}
	if ar1 := strings.TrimSpace(t1.ActionResult); ar1 != "" && !strings.HasPrefix(ar1, "Error:") {
		dir1 = fmt.Sprintf("%s | [L1 Tool (%s)]: %s", dir1, t1.ActionType, truncateText(ar1, 1400))
	}
	dir2 := strings.TrimSpace(t2.Southbound)
	if dir2 == "" {
		dir2 = t2.StateSummary
	}
	if ar2 := strings.TrimSpace(t2.ActionResult); ar2 != "" && !strings.HasPrefix(ar2, "Error:") {
		dir2 = fmt.Sprintf("%s | [L2 Tool (%s %s)]: %s", dir2, t2.ActionType, truncateText(t2.ActionTarget, 60), truncateText(ar2, 2200))
	}
	dir3 := strings.TrimSpace(t3.Southbound)
	if dir3 == "" {
		dir3 = t3.StateSummary
	}
	if ar3 := strings.TrimSpace(t3.ActionResult); ar3 != "" && !strings.HasPrefix(ar3, "Error:") {
		dir3 = fmt.Sprintf("%s | [L3 Tool (%s %s)]: %s", dir3, t3.ActionType, truncateText(t3.ActionTarget, 60), truncateText(ar3, 2200))
	}
	if wsDir, _ := e.resolveSandboxDirs(); wsDir != "" && (strings.TrimSpace(t2.ActionResult) != "" || strings.TrimSpace(t3.ActionResult) != "") {
		harnessDir := filepath.Join(wsDir, ".harness_d")
		_ = os.MkdirAll(harnessDir, 0o755)
		groundingDoc := fmt.Sprintf("# Latest L2/L3 Research & Self-Model Grounding (%s)\n\n## L2 (%s: %s)\n%s\n\n## L3 (%s: %s)\n%s\n",
			cycleID, t2.ActionType, t2.ActionTarget, truncateText(t2.ActionResult, 8000),
			t3.ActionType, t3.ActionTarget, truncateText(t3.ActionResult, 8000))
		_ = os.WriteFile(filepath.Join(harnessDir, "latest_research_grounding.md"), []byte(groundingDoc), 0o644)
	}

	dur := time.Since(parStart).Milliseconds()
	seqSum := t1.LatencyMs + t2.LatencyMs + t3.LatencyMs
	e.recordGlobalLoop("swarm_coordinator", "healthy", fmt.Sprintf("Parallel L1||L2||L3 completed in %dms (saved ~%dms)", dur, maxInt64(0, seqSum-dur)))

	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:          cycleID,
		ParentSpanID:     parentSpanID,
		AgentID:          e.agentID,
		LayerCode:        "L1||L2||L3",
		Operation:        "parallel.l1_l2_l3",
		Model:            firstNonEmpty(t2.Model, t1.Model, t3.Model),
		Status:           "ok",
		DurationMs:       dur,
		PromptTokens:     t1.PromptTokens + t2.PromptTokens + t3.PromptTokens,
		CompletionTokens: t1.CompletionTokens + t2.CompletionTokens + t3.CompletionTokens,
		Detail:           fmt.Sprintf("Concurrent L1 (%dms) + L2 (%dms) + L3 (%dms) -> %dms wall time", t1.LatencyMs, t2.LatencyMs, t3.LatencyMs, dur),
	})

	return fmt.Sprintf("Concurrent 3-Layer Scientific Synthesis for L4 (Hypothesis & Experiment Designer):\n- From L1 (Epistemic & Pareto Audit): %s\n- From L2 (Literature & Paper Mechanism): %s\n- From L3 (Self-Telemetry & Dataset Spec): %s", dir1, dir2, dir3), nil
}

// executeParallelL2L3 runs Layer 2 (Global Strategy) and Layer 3 (Agent Model / Sandbox Grounding)
// concurrently in separate goroutines and merges their Southbound outputs for Layer 4.
func (e *Engine) executeParallelL2L3(
	ctx context.Context,
	cycleID string,
	parentSpanID string,
	stimulus string,
) (string, error) {
	parStart := time.Now()
	e.recordGlobalLoop("swarm_coordinator", "active", "Running L2 (Strategy) || L3 (Agent Model) concurrently")

	var (
		t2, t3     core.ThoughtTrace
		err2, err3 error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		t2, err2 = e.executeLayerStep(ctx, cycleID, core.LayerGlobalStrategy, stimulus)
	}()
	go func() {
		defer wg.Done()
		t3, err3 = e.executeLayerStep(ctx, cycleID, core.LayerAgentModel, stimulus)
	}()
	wg.Wait()

	if err2 != nil && err3 != nil {
		return "", err2
	}

	dir2 := strings.TrimSpace(t2.Southbound)
	if dir2 == "" {
		dir2 = t2.StateSummary
	}
	dir3 := strings.TrimSpace(t3.Southbound)
	if dir3 == "" {
		dir3 = t3.StateSummary
	}

	dur := time.Since(parStart).Milliseconds()
	e.recordGlobalLoop("swarm_coordinator", "healthy", fmt.Sprintf("Parallel L2||L3 completed in %dms (saved ~%dms)", dur, maxInt64(0, (t2.LatencyMs+t3.LatencyMs)-dur)))

	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:          cycleID,
		ParentSpanID:     parentSpanID,
		AgentID:          e.agentID,
		LayerCode:        "L2||L3",
		Operation:        "parallel.l2_l3",
		Model:            t2.Model,
		Status:           "ok",
		DurationMs:       dur,
		PromptTokens:     t2.PromptTokens + t3.PromptTokens,
		CompletionTokens: t2.CompletionTokens + t3.CompletionTokens,
		Detail:           fmt.Sprintf("Concurrent L2 (%dms) + L3 (%dms) -> %dms wall time", t2.LatencyMs, t3.LatencyMs, dur),
	})

	return fmt.Sprintf("Concurrent Southbound Directives:\n- From L2 (Strategy): %s\n- From L3 (Sandbox/Capabilities): %s", dir2, dir3), nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// runExecutivePlanningLoop executes Zone 2 Tactical Planning in a single crisp pass (eliminating the 2-pass token burn).
func (e *Engine) runExecutivePlanningLoop(ctx context.Context, cycleID string, stimulus string) (core.ThoughtTrace, error) {
	e.mu.Lock()
	e.layers[core.LayerExecutive].InnerLoopStep = 1
	e.layers[core.LayerExecutive].InnerLoopMax = 1
	e.mu.Unlock()

	trace, err := e.executeLayerStep(ctx, cycleID, core.LayerExecutive, stimulus)
	e.mu.Lock()
	e.layers[core.LayerExecutive].InnerLoopStep = 0
	e.mu.Unlock()
	return trace, err
}

// extractTopActionableMilestone returns the first unchecked "- [ ]" line in goals.md that is not human/credential-gated.
func extractTopActionableMilestone(goals string) string {
	for _, line := range strings.Split(goals, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			ll := strings.ToLower(trimmed)
			isGated := strings.Contains(ll, "stripe_secret_key") ||
				strings.Contains(ll, "credential-gated") ||
				strings.Contains(ll, "operator-gated") ||
				strings.Contains(ll, "human-gated") ||
				strings.Contains(ll, "[blocked") ||
				strings.Contains(ll, "paying customer") ||
				strings.Contains(ll, "real external payment")
			if !isGated {
				return truncateText(trimmed, 1200)
			}
		}
	}
	return ""
}

// extractActiveGoals implements SLA-bounded compaction for goals.md (<= 1,800B) that NEVER truncates
// the active "- [ ]" milestone bullet mid-sentence (Fix S8).
func extractActiveGoals(goals string) string {
	if len(goals) <= SLABucketGoalsMaxBytes {
		return goals
	}
	topMilestone := extractTopActionableMilestone(goals)
	lines := strings.Split(goals, "\n")
	var headerLine string
	var sectionHeader string
	var nextMilestones []string
	var completedRecent []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if headerLine == "" && strings.HasPrefix(trimmed, "# ") {
			headerLine = truncateText(trimmed, 160)
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			sectionHeader = truncateText(trimmed, 140)
			continue
		}
		if strings.HasPrefix(trimmed, "- [x]") {
			completedRecent = append(completedRecent, truncateText(trimmed, 140))
		} else if strings.HasPrefix(trimmed, "- [ ]") && trimmed != topMilestone && len(nextMilestones) < 2 {
			nextMilestones = append(nextMilestones, truncateText(trimmed, 180))
		}
	}

	var out strings.Builder
	if headerLine != "" {
		out.WriteString(headerLine + "\n")
	}
	if len(completedRecent) > 0 {
		lastDone := completedRecent[len(completedRecent)-1]
		out.WriteString("Last Verified: " + lastDone + "\n")
	}
	if sectionHeader != "" {
		out.WriteString(sectionHeader + "\n")
	}
	if topMilestone != "" {
		// Reserve full untruncated space for the active milestone bullet so L6 sees 100% of requirements!
		out.WriteString("ACTIVE MILESTONE (COMPLETE THIS NOW):\n" + topMilestone + "\n")
		for _, nm := range nextMilestones {
			if out.Len()+len(nm)+20 <= SLABucketGoalsMaxBytes {
				out.WriteString(nm + "\n")
			}
		}
	} else {
		// Horizon Expansion Mode: All prior milestones are [x]! Show all completed axis titles so L2 never re-emits a completed axis.
		out.WriteString("[ALL PRIOR MILESTONES COMPLETED & MODE-3 VERIFIED — LAYER 2 MUST NOW EMIT 3 NEW FALSIFIABLE '- [ ]' MILESTONES ADVANCING TO THE NEXT SCIENTIFIC AXIS]\n")
		for _, done := range completedRecent {
			if out.Len()+len(done)+10 <= SLABucketGoalsMaxBytes {
				out.WriteString(done + "\n")
			}
		}
	}
	return truncateText(strings.TrimSpace(out.String()), SLABucketGoalsMaxBytes)
}

// smartFormatHotFile formats a workspace file with Pi Hashlines bounded by maxBytes.
// When a file exceeds maxBytes (e.g., app_server.py), instead of blindly truncating the bottom of the file
// (where HTTP routes, do_GET/do_POST, and main() live!), it builds:
//  1. A complete Symbol & Route Index (all def/class/route lines with exact LINE#HASH)
//  2. The Head (imports, constants, schema)
//  3. The Tail (HTTP route handlers & entrypoint at the bottom of the file)
func smartFormatHotFile(rawData string, maxBytes int) string {
	trimmedRaw := strings.TrimSpace(rawData)
	// For JSON benchmark/dataset files, generate a deterministic JSON Schema & 1-Row Sample Summary
	// so L6 sees every top-level key, nested dict key, and array sample element without ever running inspection scripts!
	if strings.HasPrefix(trimmedRaw, "{") {
		var topMap map[string]any
		if err := json.Unmarshal([]byte(trimmedRaw), &topMap); err == nil && len(topMap) > 0 {
			keys := make([]string, 0, len(topMap))
			for k := range topMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var jb strings.Builder
			jb.WriteString("[JSON SCHEMA & SAMPLE RECORD INDEX — DO NOT RUN sandbox_sh TO INSPECT KEYS]\n")
			jb.WriteString("Top-level keys: [" + strings.Join(keys, ", ") + "]\n")
			for _, k := range keys {
				val := topMap[k]
				switch tv := val.(type) {
				case []any:
					jb.WriteString(fmt.Sprintf("- %s: list (len=%d)\n", k, len(tv)))
					if len(tv) > 0 {
						if firstMap, ok := tv[0].(map[string]any); ok {
							elemKeys := make([]string, 0, len(firstMap))
							for ek := range firstMap {
								elemKeys = append(elemKeys, ek)
							}
							sort.Strings(elemKeys)
							jb.WriteString(fmt.Sprintf("  %s[0] keys: %v\n", k, elemKeys))
						}
						if sampleBytes, mErr := json.Marshal(tv[0]); mErr == nil {
							jb.WriteString(fmt.Sprintf("  sample %s[0]: %s\n", k, truncateText(string(sampleBytes), 2400)))
						}
					}
				case map[string]any:
					subKeys := make([]string, 0, len(tv))
					for sk := range tv {
						subKeys = append(subKeys, sk)
					}
					sort.Strings(subKeys)
					jb.WriteString(fmt.Sprintf("- %s: dict (keys=%v)\n", k, subKeys))
					if sampleBytes, mErr := json.Marshal(tv); mErr == nil {
						jb.WriteString(fmt.Sprintf("  value: %s\n", truncateText(string(sampleBytes), 2800)))
					}
				default:
					jb.WriteString(fmt.Sprintf("- %s: %v\n", k, tv))
				}
			}
			if jb.Len() <= maxBytes {
				return jb.String()
			}
			return truncateText(jb.String(), maxBytes)
		}
	}

	numbered := sandbox.FormatWithLineHashes(rawData)
	if len(numbered) <= maxBytes {
		return numbered
	}
	lines := strings.Split(numbered, "\n")
	var symIndex strings.Builder
	symIndex.WriteString("[SYMBOL & FUNCTION INDEX — FULL FILE MAP]\n")
	for _, ln := range lines {
		ll := strings.ToLower(ln)
		if strings.Contains(ll, "def ") || strings.Contains(ll, "class ") || strings.Contains(ll, "__main__") {
			symIndex.WriteString(truncateText(ln, 110) + "\n")
			if symIndex.Len() >= 1600 {
				break
			}
		}
	}
	headBudget := (maxBytes - symIndex.Len()) / 3
	if headBudget < 2400 {
		headBudget = 2400
	}
	tailBudget := maxBytes - symIndex.Len() - headBudget - 120
	if tailBudget < 6000 {
		tailBudget = 6000
	}
	headPart := truncateText(numbered, headBudget)
	tailStart := len(numbered) - tailBudget
	if tailStart < 0 {
		tailStart = 0
	}
	if nlIdx := strings.Index(numbered[tailStart:], "\n"); nlIdx >= 0 && tailStart+nlIdx+1 < len(numbered) {
		tailStart = tailStart + nlIdx + 1
	}
	tailPart := truncateText(numbered[tailStart:], tailBudget)
	return symIndex.String() + "\n[HEAD]\n" + headPart + "\n... [middle lines omitted; see SYMBOL & FUNCTION INDEX above or use workspace_read(\"path:start-end\")] ...\n[TAIL & SELFTEST ENTRYPOINT]\n" + tailPart
}

// buildImmutableReferencePreInjection builds the 100% static reference block (/workspace/ace_rlm.py)
// placed at the VERY TOP of L6 UserPrompt ABOVE any mutable file manifests or target scripts,
// guaranteeing >90% KV prefix cache hits even when files are created or edited!
func (e *Engine) buildImmutableReferencePreInjection() string {
	var b strings.Builder
	if rlmData, err := e.sandbox.ReadFile("ace_rlm.py"); err == nil && rlmData != "" {
		b.WriteString("=== IMMUTABLE REFERENCE SDK /workspace/ace_rlm.py (ALREADY IN CONTEXT — DO NOT CALL workspace_read ON THIS FILE) ===\n")
		b.WriteString(smartFormatHotFile(rlmData, 36000))
		b.WriteString("\n\n")
	}
	return b.String()
}

// buildStaticWorkspaceInventoryDigest constructs the cycle-stable workspace data-source, module, and dataset registry inventory
// shared across L1..L4 in the cached prompt prefix (without volatile per-step trace counters that would bust the KV prefix cache).
func (e *Engine) buildStaticWorkspaceInventoryDigest() string {
	var b strings.Builder
	b.WriteString("=== WORKSPACE DATA SOURCES, MODULES & BENCHMARK REGISTRY ===\n")
	if allFiles, err := e.sandbox.ListFiles(); err == nil && len(allFiles) > 0 {
		var dataSources []string
		var codeModules []string
		for _, f := range allFiles {
			lp := strings.ToLower(f.Path)
			if strings.HasSuffix(lp, ".db") || strings.HasSuffix(lp, ".sqlite") || strings.HasSuffix(lp, ".csv") || strings.HasSuffix(lp, ".parquet") || strings.HasPrefix(lp, "benchmarks/datasets/") {
				dataSources = append(dataSources, fmt.Sprintf("/workspace/%s", f.Path))
			} else if strings.HasSuffix(lp, ".py") && f.Path != "ace_rlm.py" {
				codeModules = append(codeModules, fmt.Sprintf("/workspace/%s", f.Path))
			}
		}
		if len(dataSources) > 0 {
			b.WriteString(fmt.Sprintf("- Discovered Data Sources & Datasets (%d): %s\n", len(dataSources), truncateText(strings.Join(dataSources, ", "), 900)))
		} else {
			b.WriteString("- Discovered Data Sources & Datasets: none yet (use web_search/web_fetch, ace_rlm.query_sqlite, or ace_rlm.build_and_register_dataset to acquire and register datasets).\n")
		}
		if len(codeModules) > 0 {
			b.WriteString(fmt.Sprintf("- Discovered Workspace Python Modules (%d): %s\n", len(codeModules), truncateText(strings.Join(codeModules, ", "), 700)))
		}
	}
	if wikiFiles, err := e.memory.ListWiki(); err == nil && len(wikiFiles) > 0 {
		var skillFiles []string
		for _, wf := range wikiFiles {
			if strings.HasPrefix(wf.Path, "skills/") && strings.HasSuffix(wf.Path, ".md") {
				skillFiles = append(skillFiles, fmt.Sprintf("/wiki/%s (%dB)", wf.Path, wf.SizeBytes))
			}
		}
		if len(skillFiles) > 0 {
			b.WriteString(fmt.Sprintf("- On-Demand C6 Skills Library (/wiki/skills/): %s\n", strings.Join(skillFiles, ", ")))
		}
	}
	if regData, err := e.sandbox.ReadFile("benchmarks/registry.json"); err == nil && strings.TrimSpace(regData) != "" {
		b.WriteString("- Current Dataset Registry (/workspace/benchmarks/registry.json):\n")
		b.WriteString(truncateText(regData, 1800) + "\n")
	} else {
		b.WriteString("- Current Dataset Registry (/workspace/benchmarks/registry.json): [EMPTY — L3/L6 must build and register versioned datasets in /workspace/benchmarks/datasets/*.json using ace_rlm.build_and_register_dataset!]\n")
	}
	return b.String()
}

// buildLiveTelemetrySummaryLine returns the volatile per-step runtime telemetry summary placed in the dynamic cycle tail
// at the bottom of UserPrompt so incrementing trace/token counters never invalidate the shared KV prefix cache!
func (e *Engine) buildLiveTelemetrySummaryLine() string {
	m := e.telem.Metrics(context.Background())
	cachePct := 0.0
	if m.TotalPromptTokens > 0 {
		cachePct = float64(m.TotalCachedTokens) * 100.0 / float64(m.TotalPromptTokens)
	}
	e.mu.RLock()
	cycles := e.totalCycles
	thoughts := e.totalThoughts
	var layerStats []string
	for _, id := range core.AllLayers() {
		if st := e.layers[id]; st != nil && st.TotalInvocations > 0 {
			layerStats = append(layerStats, fmt.Sprintf("%s(n=%d, last=%dms, model=%s)", st.Code, st.TotalInvocations, st.LastLatencyMs, st.Model))
		}
	}
	e.mu.RUnlock()
	line := fmt.Sprintf("- Live Runtime Telemetry: %d cycles, %d thought traces | Live KV Prefix Cache Hit Rate: %.2f%% | Prompt Tokens: %d (Cached: %d) | Completion Tokens: %d\n",
		cycles, thoughts, cachePct, m.TotalPromptTokens, m.TotalCachedTokens, m.TotalCompletionToks)
	if len(layerStats) > 0 {
		line += "- Layer Execution Telemetry: " + strings.Join(layerStats, ", ") + "\n"
	}
	return line
}

// buildProprioceptiveSelfModelDigest combines static workspace inventory and live runtime telemetry.
func (e *Engine) buildProprioceptiveSelfModelDigest() string {
	return e.buildStaticWorkspaceInventoryDigest() + e.buildLiveTelemetrySummaryLine()
}

// buildActiveMilestoneWikiNotesDigest extracts up to 2 /wiki/notes/* files matching the active milestone (by normalized Axis ID
// or filename keywords) and places them in the dynamic suffix so L6 sees L2/L4 contracts automatically without busting the KV prefix cache!
func (e *Engine) buildActiveMilestoneWikiNotesDigest(milestone string) string {
	wikiFiles, _ := e.memory.ListWiki()
	if len(wikiFiles) == 0 || strings.TrimSpace(milestone) == "" {
		return ""
	}
	lowerDir := strings.ToLower(milestone)
	axisID := strings.ToLower(strings.TrimSpace(extractMilestoneIdentifier(milestone)))
	axisCompact := strings.ReplaceAll(strings.ReplaceAll(axisID, " ", ""), "_", "-")
	axisNoSep := strings.ReplaceAll(strings.ReplaceAll(axisCompact, "-", ""), " ", "")

	var b strings.Builder
	matched := 0
	for i := len(wikiFiles) - 1; i >= 0 && matched < 2; i-- {
		wf := wikiFiles[i]
		if wf.Path == "soul.md" || wf.Path == "index.md" || wf.Path == "goals.md" || wf.Path == "log.md" || wf.SizeBytes <= 0 || wf.SizeBytes > 16000 {
			continue
		}
		lp := strings.ToLower(wf.Path)
		lpCompact := strings.ReplaceAll(lp, "_", "-")
		lpNoSep := strings.ReplaceAll(lpCompact, "-", "")
		baseName := lp
		if idx := strings.LastIndex(lp, "/"); idx >= 0 {
			baseName = lp[idx+1:]
		}
		isMatch := strings.Contains(lowerDir, lp) || (len(baseName) > 4 && strings.Contains(lowerDir, baseName))
		if !isMatch && axisNoSep != "" && len(axisNoSep) >= 5 {
			if strings.Contains(lpNoSep, axisNoSep) || (axisCompact != "" && strings.Contains(lpCompact, axisCompact)) {
				isMatch = true
			}
		}
		if isMatch {
			if wData, err := e.memory.ReadWiki(wf.Path); err == nil && wData != "" {
				b.WriteString(fmt.Sprintf("\n=== ACTIVE MILESTONE WIKI NOTE /wiki/%s (ALREADY IN CONTEXT — DO NOT CALL workspace_read ON THIS FILE) ===\n%s\n", wf.Path, truncateText(wData, 3200)))
				matched++
			}
		}
	}
	return b.String()
}

// buildEnvironmentalGroundingDigest extracts the static research corpus index and reminds L2 of its live external research capabilities
// without walking mutable /wiki/notes/* files in the static prefix (active milestone wiki notes are injected in the dynamic suffix via buildActiveMilestoneWikiNotesDigest).
func (e *Engine) buildEnvironmentalGroundingDigest(milestone string) string {
	var b strings.Builder
	b.WriteString("=== LAYER 2 ENVIRONMENTAL & LITERATURE GROUNDING CONTEXT ===\n")
	b.WriteString("- External Discovery Actuators: You can call action.type=\"web_search\" or \"web_fetch\" directly in L2 (or instruct L6 to run ace_rlm.search_arxiv(query) / web_search) whenever researching a new domain or external environment.\n")
	if idxData, err := e.sandbox.ReadFile("research_corpus/CORPUS_INDEX.md"); err == nil && idxData != "" {
		b.WriteString("[/workspace/research_corpus/CORPUS_INDEX.md Summary]:\n")
		b.WriteString(truncateText(idxData, 2200) + "\n\n")
	}
	return b.String()
}

// buildBidirectionalBusContext formats recent Southbound (Top-Down Control) and Northbound (Bottom-Up Telemetry)
// bus messages relevant to the current layer, plus the active L4 Experiment Contract if present.
func (e *Engine) buildBidirectionalBusContext(ctx context.Context, layer core.LayerID) string {
	var sb strings.Builder
	envelopes, _ := e.bus.Recent(18)
	var southboundLines []string
	var northboundLines []string
	for _, env := range envelopes {
		if env.Direction == core.DirectionSouthbound && env.ToLayer >= int(layer)-1 && len(southboundLines) < 3 {
			southboundLines = append(southboundLines, fmt.Sprintf("  * [L%d->L%d] %s", env.FromLayer, env.ToLayer, truncateText(env.Payload, 180)))
		} else if env.Direction == core.DirectionNorthbound && env.ToLayer <= int(layer)+1 && len(northboundLines) < 3 {
			northboundLines = append(northboundLines, fmt.Sprintf("  * [L%d->L%d] %s", env.FromLayer, env.ToLayer, truncateText(env.Payload, 180)))
		}
	}
	if len(southboundLines) > 0 {
		sb.WriteString("- Southbound Control Bus (Top-Down Directives):\n" + strings.Join(southboundLines, "\n") + "\n")
	}
	if len(northboundLines) > 0 {
		sb.WriteString("- Northbound Telemetry Bus (Bottom-Up Status & Signals):\n" + strings.Join(northboundLines, "\n") + "\n")
	}
	if layer >= core.LayerExecutive {
		if expData, err := e.sandbox.ReadFile(".harness_d/active_experiment.json"); err == nil && len(expData) > 0 {
			sb.WriteString("- Active L4 Project/Hypothesis Contract (/workspace/.harness_d/active_experiment.json):\n" + truncateText(strings.TrimSpace(expData), 450) + "\n")
		}
	}
	return sb.String()
}

// syncLiveTracesToWorkspaceBenchmarks mirrors the live .ace-sandbox/ace.db into /workspace/benchmarks/live_traces.db
// (and /workspace/benchmarks/ace_v0_3_live_traces.db for backward compatibility) so L3 and L6 scripts can query live self-telemetry.
func (e *Engine) syncLiveTracesToWorkspaceBenchmarks() {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" {
		return
	}
	sandboxRoot := filepath.Dir(wsDir)
	srcDB := filepath.Join(sandboxRoot, "ace.db")
	dstDir := filepath.Join(wsDir, "benchmarks")
	_ = os.MkdirAll(dstDir, 0o755)
	if data, err := os.ReadFile(srcDB); err == nil && len(data) > 0 {
		for _, name := range []string{"live_traces.db", "ace_v0_3_live_traces.db"} {
			dstDB := filepath.Join(dstDir, name)
			tmpPath := dstDB + ".tmp"
			if wErr := os.WriteFile(tmpPath, data, 0o644); wErr == nil {
				_ = os.Rename(tmpPath, dstDB)
			}
		}
	}
}

// buildHotFilePreInjection implements Gate 2 (L6 Workspace Hot-File Pre-Injection Gate).
// Read-only reference/dataset/imported files are placed FIRST, and the mutable primary `.py` target script is placed LAST
// so that editing the bottom of primaryPy keeps 100% of SystemPrompt + ace_rlm.py + reference files + the unchanged top lines of primaryPy in the KV prefix cache!
func (e *Engine) buildHotFilePreInjection(directive string) string {
	wsFiles, _ := e.sandbox.ListFiles()
	var injected strings.Builder
	count := 0
	lowerDir := strings.ToLower(directive)

	preInjectedPaths := map[string]bool{
		"ace_rlm.py":                            true,
		"benchmarks/v0_1_benchmark_suite.json": true,
	}
	var preInjectedRaw strings.Builder

	wsDir, _ := e.resolveSandboxDirs()
	primaryPy := ResolveWorkspaceArtifactPath(wsDir, ExtractPrimaryMilestoneScript(directive))
	var primaryRawData string
	if primaryPy != "" && !preInjectedPaths[primaryPy] {
		if rawData, err := e.sandbox.ReadFile(primaryPy); err == nil && rawData != "" {
			primaryRawData = rawData
			preInjectedPaths[primaryPy] = true
			preInjectedRaw.WriteString("\n" + strings.ToLower(rawData))
			count++
		}
	}

	// 1. Always pre-inject /workspace/benchmarks/registry.json first so L6 sees all built datasets!
	if !preInjectedPaths["benchmarks/registry.json"] {
		if regData, err := e.sandbox.ReadFile("benchmarks/registry.json"); err == nil && strings.TrimSpace(regData) != "" {
			injected.WriteString(fmt.Sprintf("\n=== PRE-INJECTED DATASET REGISTRY /workspace/benchmarks/registry.json ===\n%s\n", truncateText(regData, 3200)))
			preInjectedPaths["benchmarks/registry.json"] = true
		}
	}

	// 2. Pre-inject up to 2 secondary /workspace reference/dataset files referenced in the directive BEFORE primaryPy
	for _, f := range wsFiles {
		if count >= 3 || injected.Len() >= SLABucketHotFilesTotalMaxBytes/2 {
			break
		}
		if preInjectedPaths[f.Path] || f.SizeBytes <= 0 || f.SizeBytes > 250000 {
			continue
		}
		lp := strings.ToLower(f.Path)
		base := lp
		if idx := strings.LastIndex(lp, "/"); idx >= 0 {
			base = lp[idx+1:]
		}
		stem := strings.TrimSuffix(base, ".py")
		if strings.Contains(lowerDir, lp) || (len(base) >= 4 && strings.Contains(lowerDir, base)) ||
			(len(stem) >= 5 && strings.HasSuffix(base, ".py") && strings.Contains(lowerDir, stem)) {
			if rawData, err := e.sandbox.ReadFile(f.Path); err == nil && rawData != "" {
				formatted := smartFormatHotFile(rawData, 5000)
				injected.WriteString(fmt.Sprintf("\n=== PRE-INJECTED /workspace/%s (PI HASHLINE CACHE — ALREADY IN CONTEXT, DO NOT CALL workspace_read ON THIS FILE) ===\n%s\n", f.Path, formatted))
				preInjectedPaths[f.Path] = true
				preInjectedRaw.WriteString("\n" + strings.ToLower(rawData))
				count++
			}
		}
	}
	// 2b. Transitive Local Import Pre-Injection
	if count > 0 && count < 3 && injected.Len() < SLABucketHotFilesTotalMaxBytes/2 {
		combinedRaw := preInjectedRaw.String()
		for _, f := range wsFiles {
			if count >= 3 || injected.Len() >= SLABucketHotFilesTotalMaxBytes/2 || preInjectedPaths[f.Path] || f.SizeBytes <= 0 || f.SizeBytes > 28000 || !strings.HasSuffix(f.Path, ".py") {
				continue
			}
			stem := strings.TrimSuffix(strings.ToLower(f.Path), ".py")
			if idx := strings.LastIndex(stem, "/"); idx >= 0 {
				stem = stem[idx+1:]
			}
			if len(stem) >= 4 && (strings.Contains(combinedRaw, "import "+stem) || strings.Contains(combinedRaw, "from "+stem+" ")) {
				if rawData, err := e.sandbox.ReadFile(f.Path); err == nil && rawData != "" {
					formatted := smartFormatHotFile(rawData, 4500)
					injected.WriteString(fmt.Sprintf("\n=== PRE-INJECTED /workspace/%s (IMPORTED BY TARGET SCRIPT — ALREADY IN CONTEXT, DO NOT CALL workspace_read ON THIS FILE) ===\n%s\n", f.Path, formatted))
					preInjectedPaths[f.Path] = true
					count++
				}
			}
		}
	}

	// 3. Emit the mutable primary `.py` target script LAST among the pre-injected files so editing it preserves all prefixes above it!
	if primaryPy != "" && primaryRawData != "" {
		formatted := smartFormatHotFile(primaryRawData, SLABucketHotFileSingleMaxBytes)
		injected.WriteString(fmt.Sprintf("\n=== PRE-INJECTED PRIMARY TARGET /workspace/%s (PI HASHLINE CACHE — ALREADY IN CONTEXT, DO NOT CALL workspace_read ON THIS FILE) ===\n%s\n", primaryPy, formatted))
	}

	return injected.String()
}

// tryDirectWorldReflex uses the local 4-Bit d1 GPU Gate to execute pure outside-world verification/sensor commands
// in L6 with 0 Cloud LLM tokens when no file creation/editing is requested.
func (e *Engine) tryDirectWorldReflex(
	ctx context.Context,
	cycleID string,
	parentSpanID string,
	directive string,
) (core.ThoughtTrace, bool) {
	lower := strings.ToLower(directive)
	for _, writeWord := range []string{"edit ", "write ", "implement ", "replace ", "create ", "build ", "refactor", "milestone"} {
		if strings.Contains(lower, writeWord) {
			return core.ThoughtTrace{}, false
		}
	}

	var cmdToRun string
	if primaryPy := ExtractPrimaryMilestoneScript(directive); primaryPy != "" && strings.Contains(lower, "--selftest") {
		cmdToRun = fmt.Sprintf("python3 /workspace/%s --selftest", primaryPy)
	}
	if cmdToRun == "" {
		return core.ThoughtTrace{}, false
	}

	gateQ := fmt.Sprintf("Statement: L6 received directive %q.\nQuestion: Is running %q directly in the sandbox sufficient to execute this check?",
		truncateText(directive, 180), cmdToRun)
	gpuRes, gErr := llm.EvaluateLocalGPUGateWithState(ctx, truncateText(directive, 200), gateQ, 0.45)
	if gErr != nil || !gpuRes.Decision {
		return core.ThoughtTrace{}, false
	}

	start := time.Now()
	actRes := e.executeLayerAction(ctx, cycleID, core.LayerTaskProsecution, core.LayerAction{
		Type:   "sandbox_sh",
		Target: cmdToRun,
		Done:   true,
	}, gpuRes.Model)

	// If the outside-world command failed (exit != 0), fall through to the System-2 Cloud LLM so it can diagnose and fix the code!
	if !strings.HasPrefix(actRes, "exit=0") {
		return core.ThoughtTrace{}, false
	}

	latMs := time.Since(start).Milliseconds()
	trace := core.ThoughtTrace{
		ID:               "tht_" + uuid.NewString()[:12],
		CycleID:          cycleID,
		AgentID:          e.agentID,
		LayerID:          int(core.LayerTaskProsecution),
		LayerCode:        "L6",
		LoopKind:         "d1_reflex",
		Model:            gpuRes.Model,
		TriggerSummary:   truncateText(directive, 120),
		Thought:          fmt.Sprintf("d1 System-1 Outside-World Reflex (P(yes)=%.4f, %.1fms, 0 cloud tokens): executed `%s` directly in bwrap sandbox.", gpuRes.PYes, gpuRes.LatencyMs, cmdToRun),
		StateSummary:     fmt.Sprintf("d1 Reflex verified `%s` (exit=0, %dms, 0 cloud tok)", cmdToRun, latMs),
		Northbound:       truncateText(actRes, 220),
		ActionType:       "sandbox_sh",
		ActionTarget:     cmdToRun,
		ActionResult:     actRes,
		Confidence:       gpuRes.PYes,
		LatencyMs:        latMs,
		PromptTokens:     0,
		CachedTokens:     0,
		CompletionTokens: 0,
		CreatedAt:        time.Now().UTC(),
	}
	_ = e.telem.RecordThought(ctx, trace)
	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:      cycleID,
		ParentSpanID: parentSpanID,
		AgentID:      e.agentID,
		LayerCode:    "L6",
		Operation:    "gate.l6_d1_reflex",
		Model:        gpuRes.Model,
		Status:       "ok",
		DurationMs:   latMs,
		Detail:       fmt.Sprintf("L6 Zero-Token d1 Reflex (%s, P=%.2f): %s -> exit=0", gpuRes.Model, gpuRes.PYes, cmdToRun),
	})

	e.mu.Lock()
	st := e.layers[core.LayerTaskProsecution]
	st.State = "idle"
	st.Summary = trace.StateSummary
	st.LastThought = trace.Thought
	st.LastAction = fmt.Sprintf("sandbox_sh(%s): %s", cmdToRun, truncateText(actRes, 140))
	st.Confidence = gpuRes.PYes
	st.Model = gpuRes.Model
	st.LastLatencyMs = latMs
	st.TotalInvocations++
	st.UpdatedAt = time.Now().UTC()
	snap := *st
	e.mu.Unlock()
	_ = e.memory.SaveLayerStatus(ctx, snap)
	e.telem.EmitLive("layer.status", snap)

	return trace, true
}

// runPiProsecutorLoop implements Zone 3 (Reflex / Outside-World Interface) with:
// 1. Local 4-Bit d1 System-1 Outside-World Reflexes (auto-verification of code writes, post-action world observation gating, and direct sensor execution)
// 2. True Append-Only Multi-Turn Messages[] (keeping SystemPrompt + Turn 1 UserPrompt 100% static for >82% KV Prefix Cache Hit!)
// 3. Multi-Action Batching per turn (up to 3 actions executed in 1 turn)
// 4. Pi Blackhole Spillover truncation on tool outputs
func (e *Engine) runPiProsecutorLoop(
	ctx context.Context,
	cycleID string,
	parentSpanID string,
	initialDirective string,
) []core.ThoughtTrace {
	var traces []core.ThoughtTrace
	rawDirective := initialDirective
	if reflexTrace, ok := e.tryDirectWorldReflex(ctx, cycleID, parentSpanID, rawDirective); ok {
		return []core.ThoughtTrace{reflexTrace}
	}

	var priorTurns []core.ChatTurn
	var frozenFirstUserPrompt string
	rawGoals, _ := e.memory.ReadWiki("goals.md")
	activeMilestone := extractTopActionableMilestone(rawGoals)

	wsDir, _ := e.resolveSandboxDirs()
	var fiber *CordisFiber
	if wsDir != "" {
		fiber = NewCordisFiber(wsDir)
	}
	handoff := LoadHandoffPacket(ctx, e.memory, activeMilestone)

	// Cross-Cycle Recon Carryover (Fix S13): Only carry over non-failing inspection turns on the exact same milestone
	e.mu.RLock()
	if activeMilestone != "" && e.lastL6ReconMilestone == activeMilestone && len(e.lastL6ReconTurns) > 0 && len(handoff.FailedAttempts) == 0 {
		priorTurns = append(priorTurns, e.lastL6ReconTurns...)
	}
	e.mu.RUnlock()

	anyMutatedInCycle := false
	e.mu.Lock()
	e.l6MutatedInCycle = false
	e.mu.Unlock()
	for step := 1; step <= maxPiLoopSteps; step++ {
		if ctx.Err() != nil {
			break
		}
		e.mu.Lock()
		e.layers[core.LayerTaskProsecution].InnerLoopStep = step
		e.layers[core.LayerTaskProsecution].InnerLoopMax = maxPiLoopSteps
		e.mu.Unlock()

		e.recordGlobalLoop("pi_prosecutor", "active", fmt.Sprintf("Cycle %s: Zone 3 Reflex step %d/%d", cycleID, step, maxPiLoopSteps))

		stepStart := time.Now()
		trace, err := e.executeLayerStepWithFrozenPrompt(ctx, cycleID, core.LayerTaskProsecution, initialDirective, &frozenFirstUserPrompt, priorTurns)
		if err != nil {
			break
		}

		dec := ParseLayerDecision(trace.RawResponse)
		actType := strings.ToLower(strings.TrimSpace(trace.ActionType))

		// L6 System-1 Reflex 1: Zero-Cloud-Token Auto-Verification in the Outside World!
		hasMutated := strings.Contains(actType, "workspace_write") || strings.Contains(actType, "workspace_edit") || strings.Contains(actType, "wiki_write")
		if hasMutated {
			anyMutatedInCycle = true
			e.mu.Lock()
			e.l6MutatedInCycle = true
			e.lastL6ReconTurns = nil
			e.lastL6ReconMilestone = ""
			e.mu.Unlock()
		}
		alreadyVerifiedInBatch := hasMutated && strings.Contains(trace.ActionType, "sandbox_sh")
		needsVerification := !alreadyVerifiedInBatch &&
			(strings.Contains(actType, "workspace_write") || strings.Contains(actType, "workspace_edit")) &&
			(strings.Contains(trace.ActionTarget, ".py") || strings.Contains(trace.ActionTarget, ".sh"))

		if needsVerification {
			var scriptTargets []string
			seenTargets := map[string]bool{}
			for _, part := range strings.Split(trace.ActionTarget, ";") {
				cleanPart := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(part), "/workspace/"), "workspace/")
				if (strings.HasSuffix(cleanPart, ".py") || strings.HasSuffix(cleanPart, ".sh")) && !seenTargets[cleanPart] {
					seenTargets[cleanPart] = true
					scriptTargets = append(scriptTargets, cleanPart)
				}
			}
			if len(scriptTargets) > 0 {
				var verifyCmds []string
				for _, cleanFile := range scriptTargets {
					qPath := "/workspace/" + cleanFile
					if strings.HasSuffix(cleanFile, ".sh") {
						verifyCmds = append(verifyCmds, fmt.Sprintf("sh -n %q", qPath))
					} else {
						verifyCmds = append(verifyCmds, fmt.Sprintf("if grep -q -- '--selftest' %q 2>/dev/null; then python3 %q --selftest; else python3 -m py_compile %q; fi", qPath, qPath, qPath))
					}
				}
				verifyCmd := strings.Join(verifyCmds, " && ")
				vRes := e.executeLayerAction(ctx, cycleID, core.LayerTaskProsecution, core.LayerAction{
					Type:   "sandbox_sh",
					Target: verifyCmd,
				}, "d1-4bit-l6-reflex")
				trace.ActionType = trace.ActionType + "+sandbox_sh(d1-reflex)"
				trace.ActionResult = trace.ActionResult + "\n---\n[L6 d1 Outside-World Auto-Verify: `" + verifyCmd + "`]\n" + vRes
				if strings.HasPrefix(strings.TrimSpace(vRes), "exit=0") {
					alreadyVerifiedInBatch = true
				}
			}
			needsVerification = false
		}

		traces = append(traces, trace)

		_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
			TraceID:          cycleID,
			ParentSpanID:     parentSpanID,
			AgentID:          e.agentID,
			LayerCode:        "L6",
			Operation:        "l6.pi_loop",
			Model:            trace.Model,
			Status:           "ok",
			DurationMs:       time.Since(stepStart).Milliseconds(),
			PromptTokens:     trace.PromptTokens,
			CompletionTokens: trace.CompletionTokens,
			Detail:           fmt.Sprintf("step %d/%d (cached=%d): %s -> %s", step, maxPiLoopSteps, trace.CachedTokens, trace.ActionType, truncateText(trace.ActionTarget, 80)),
		})

		needsSynthesis := !anyMutatedInCycle
		hadError := strings.Contains(trace.ActionResult, "error:") ||
			strings.Contains(trace.ActionResult, "[LTL VETO") ||
			strings.Contains(trace.ActionResult, "SYNTAX ERROR") ||
			strings.Contains(trace.ActionResult, "SELFTEST_FAILED") ||
			strings.HasPrefix(trace.ActionResult, "exit=1") ||
			strings.HasPrefix(trace.ActionResult, "exit=2") ||
			strings.Contains(trace.ActionResult, "\nexit=1") ||
			strings.Contains(trace.ActionResult, "\nexit=2") ||
			strings.Contains(trace.ActionResult, "Traceback (most recent call last)")

		if hadError {
			alreadyVerifiedInBatch = false
		}

		// Record LLM output-contract health in Bayesian Cost-of-Pass router: only penalize the model when it fails to emit a valid action
		// (never switch away from a warm-cache model just because the Python script it is debugging exited 1!)
		if rec, ok := e.llm.(interface{ RecordTaskOutcome(string, bool) }); ok && trace.Model != "" {
			rec.RecordTaskOutcome(trace.Model, actType != "" && actType != "none")
		}
		exitCode := 0
		if hadError {
			exitCode = 1
		}
		var digests map[string]string
		hasDiff := hasMutated
		if fiber != nil {
			digests = fiber.RefreshDigests()
		}
		recordedAct := core.LayerAction{
			Type:   trace.ActionType,
			Target: trace.ActionTarget,
		}
		RecordHandoffAttempt(&handoff, cycleID, recordedAct, exitCode, trace.ActionResult, hasDiff, nil, digests)
		SaveHandoffPacket(ctx, e.memory, handoff)

		if actType == "" || actType == "none" {
			break
		}

		// L6 System-1 Reflex 2: Local 4-Bit d1 Outside-World Observation Gate (<65ms trunk-cached, 0 cloud tokens, calibrated tau=0.62)!
		d1WorldSatisfied := false
		if hasMutated && !needsSynthesis && !hadError {
			sharedL6Trunk := "ACE L6 Outside-World Observation & Verification Gate | Verify empirical exit=0 execution and dataset/hypothesis output"
			worldQ := fmt.Sprintf("Directive: %q\nL6 executed %s(%s) and observed outside-world output: %q.\nQuestion: Did this outside-world action succeed without errors and satisfy the step?",
				truncateText(rawDirective, 160), trace.ActionType, truncateText(trace.ActionTarget, 60), truncateText(trace.ActionResult, 220))
			if gpuRes, gErr := llm.EvaluateLocalGPUGateWithState(ctx, sharedL6Trunk, worldQ, 0.62); gErr == nil {
				d1WorldSatisfied = gpuRes.Decision
				_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
					TraceID:      cycleID,
					ParentSpanID: parentSpanID,
					AgentID:      e.agentID,
					LayerCode:    "L6",
					Operation:    "gate.l6_d1_world",
					Model:        gpuRes.Model,
					Status:       "ok",
					DurationMs:   int64(gpuRes.LatencyMs),
					Detail:       fmt.Sprintf("L6 d1 World Gate P(yes)=%.4f (%s, %.1fms) on %s -> done=%v", gpuRes.PYes, gpuRes.Model, gpuRes.LatencyMs, trace.ActionType, d1WorldSatisfied),
				})
			}
		}

		if (dec.Action.Done || alreadyVerifiedInBatch || (d1WorldSatisfied && hasMutated)) && !needsVerification && !needsSynthesis && !hadError {
			break
		}

		isWebResearch := strings.Contains(actType, "web_search") || strings.Contains(actType, "web_fetch")
		nextHint := "Complete the remaining step (or batch edit+test via actions:[...]) and set action.done=true."
		if needsVerification {
			nextHint = fmt.Sprintf("You wrote/edited /workspace/%s. Now run and verify it using action.type=\"sandbox_sh\" and set action.done=true.", trace.ActionTarget)
		} else if isWebResearch && !hasMutated && !hadError {
			nextHint = fmt.Sprintf("You gathered external web/research data via %s(%s). Now synthesize those findings into your implementation using action.type=\"workspace_write\" or \"workspace_edit\" (batched with sandbox_sh --selftest in actions:[...])!", trace.ActionType, truncateText(trace.ActionTarget, 50))
		} else if needsSynthesis {
			nextHint = "⚠️ LTL G4 LOCK ACTIVE (noop=1): You have used your 1 allowed read-only inspection turn! Any further sandbox_sh or workspace_read turn WILL BE VETOED BY G4. Emit action.type=\"workspace_write\" or \"workspace_edit\" (batched with sandbox_sh --selftest in actions:[...]) NOW!"
		} else if hadError {
			nextHint = "Your previous action returned an error/traceback or LTL veto. If you need external domain/algorithm/API answers to solve this problem, you may include web_search/web_fetch alongside workspace_edit or workspace_write (do NOT repeat the failed command without editing code) and verify in a single batched turn."
		}

		obsContent := fmt.Sprintf("[Step %d Actuator Output: %s(%s)]\n%s\n\nNEXT INSTRUCTION (Step %d/%d): %s",
			step, trace.ActionType, trace.ActionTarget, truncateText(trace.ActionResult, SLABucketStepObsMaxBytes), step+1, maxPiLoopSteps, nextHint)
		priorTurns = append(priorTurns,
			core.ChatTurn{Role: "assistant", Content: trace.RawResponse},
			core.ChatTurn{Role: "user", Content: obsContent},
		)
	}

	// Post-Cycle Mode-3 Deterministic Milestone Verification & Runner Auto-Checkoff (Fix S5 & S16)
	e.verifyAndAutoCheckoffMilestone(ctx, cycleID, parentSpanID, activeMilestone, &handoff, traces)

	if jl, ok := e.sandbox.(*sandbox.Jail); ok {
		jl.ScrubEphemeralScrap()
	}

	if !anyMutatedInCycle && activeMilestone != "" && len(priorTurns) >= 2 {
		lastUser := priorTurns[len(priorTurns)-1].Content
		if !strings.Contains(lastUser, "[LTL VETO") {
			e.mu.Lock()
			e.lastL6ReconMilestone = activeMilestone
			e.lastL6ReconTurns = append([]core.ChatTurn(nil), priorTurns[len(priorTurns)-2:]...)
			e.mu.Unlock()
		}
	}

	e.mu.Lock()
	e.layers[core.LayerTaskProsecution].InnerLoopStep = 0
	e.mu.Unlock()
	e.recordGlobalLoop("pi_prosecutor", "healthy", fmt.Sprintf("Completed %d inner step(s) in %s", len(traces), cycleID))
	return traces
}

func (e *Engine) resolveSandboxDirs() (string, string) {
	if jl, ok := e.sandbox.(*sandbox.Jail); ok {
		return jl.WorkspaceDir(), jl.WikiDir()
	}
	return "", ""
}

func (e *Engine) verifyAndAutoCheckoffMilestone(
	ctx context.Context,
	cycleID string,
	parentSpanID string,
	activeMilestone string,
	handoff *CrossCycleHandoffPacket,
	traces []core.ThoughtTrace,
) {
	if strings.TrimSpace(activeMilestone) == "" {
		return
	}
	wsDir, wikiDir := e.resolveSandboxDirs()
	if wsDir == "" || wikiDir == "" {
		return
	}
	gateStart := time.Now()
	gateRes := EvaluateMode3MilestoneGate(ctx, activeMilestone, e.sandbox, wsDir, wikiDir, handoff.VerifiedFiles)
	status := "veto"
	if gateRes.Passed {
		status = "ok"
	}
	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:      cycleID,
		ParentSpanID: parentSpanID,
		AgentID:      e.agentID,
		LayerCode:    "L6",
		Operation:    "gate.mode3",
		Status:       status,
		DurationMs:   time.Since(gateStart).Milliseconds(),
		Detail:       fmt.Sprintf("Mode-3 Gate on %s -> passed=%v (%s)", truncateText(activeMilestone, 70), gateRes.Passed, truncateText(gateRes.Reason, 120)),
	})

	if !gateRes.Passed {
		prevNoOp := handoff.ConsecutiveNoOp
		RecordHandoffAttempt(handoff, cycleID, core.LayerAction{
			Type:   "mode3_gate_check",
			Target: activeMilestone,
		}, 1, "[MODE-3 GATE VETO]: "+gateRes.Reason, false, nil, handoff.FileDigests)
		handoff.ConsecutiveNoOp = prevNoOp
		SaveHandoffPacket(ctx, e.memory, *handoff)
		return
	}

	// Auto-checkoff [x] in /wiki/goals.md (matching both exact line and Axis/Milestone ID across all lines)
	rawGoals, err := e.memory.ReadWiki("goals.md")
	if err == nil && rawGoals != "" {
		axisID := extractMilestoneIdentifier(activeMilestone)
		var outLines []string
		changed := false
		for _, line := range strings.Split(rawGoals, "\n") {
			tline := strings.TrimSpace(line)
			if strings.HasPrefix(tline, "- [ ]") && (tline == activeMilestone || (axisID != "" && extractMilestoneIdentifier(tline) == axisID)) {
				outLines = append(outLines, strings.Replace(line, "- [ ]", "- [x]", 1))
				changed = true
			} else {
				outLines = append(outLines, line)
			}
		}
		if changed {
			_ = e.memory.WriteWiki("goals.md", strings.Join(outLines, "\n"))
		}
	}

	wsFiles, _ := ExtractMilestoneArtifacts(activeMilestone)
	RecordHandoffAttempt(handoff, cycleID, core.LayerAction{Type: "mode3_checkoff", Target: activeMilestone}, 0, "", true, wsFiles, handoff.FileDigests)
	handoff.FailedAttempts = nil
	SaveHandoffPacket(ctx, e.memory, *handoff)

	if jl, ok := e.sandbox.(*sandbox.Jail); ok {
		jl.CommitMilestoneArtifacts(activeMilestone, wsFiles)
	}

	verifiedMsg := "[MODE-3 GATE VERIFIED & CHECKED OFF]: " + truncateText(activeMilestone, 180)
	e.mu.Lock()
	if st := e.layers[core.LayerTaskProsecution]; st != nil {
		st.Summary = verifiedMsg
	}
	e.layers[core.LayerAspirational].NorthboundSignal = verifiedMsg
	e.layers[core.LayerGlobalStrategy].NorthboundSignal = verifiedMsg
	e.layers[core.LayerAgentModel].NorthboundSignal = verifiedMsg
	e.layers[core.LayerExecutive].NorthboundSignal = verifiedMsg
	e.mu.Unlock()
	_ = e.bus.Publish(ctx, core.BusEnvelope{
		CycleID:   cycleID,
		AgentID:   e.agentID,
		Topic:     "ace.bus.north.L6->L2.milestone_verified",
		Direction: core.DirectionNorthbound,
		FromLayer: 6,
		ToLayer:   2,
		Kind:      "milestone_verified",
		Summary:   verifiedMsg,
		Payload:   verifiedMsg,
	})

	// Append verified milestone score row to /workspace/.harness_d/scoreboard.tsv
	sbPath := filepath.Join(wsDir, ".harness_d", "scoreboard.tsv")
	var totPrompt, totCached int
	var totLat int64
	var modelUsed string
	for _, t := range traces {
		totPrompt += t.PromptTokens
		totCached += t.CachedTokens
		totLat += t.LatencyMs
		if t.Model != "" {
			modelUsed = t.Model
		}
	}
	hitPct := 0.0
	if totPrompt > 0 {
		hitPct = (float64(totCached) * 100.0) / float64(totPrompt)
	}
	row := fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%d\t%.1f\t%d\ttrue\n",
		cycleID, time.Now().UTC().Format(time.RFC3339), truncateText(activeMilestone, 80), modelUsed, totPrompt, totCached, hitPct, totLat)
	if f, err := os.OpenFile(sbPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString(row)
		_ = f.Close()
	}
}

// consolidateCycleNorthbound runs the post-cycle Learning Loop (L6 -> L1 feedback) and Memory Manager.
func (e *Engine) consolidateCycleNorthbound(
	ctx context.Context,
	cycleID string,
	parentSpanID string,
	l6Traces []core.ThoughtTrace,
) {
	if len(l6Traces) == 0 {
		return
	}
	start := time.Now()
	last := l6Traces[len(l6Traces)-1]
	var summaryParts []string
	for i, t := range l6Traces {
		if t.ActionType != "" && t.ActionType != "none" {
			summaryParts = append(summaryParts, fmt.Sprintf("Step %d: %s(%s) -> %s",
				i+1, t.ActionType, truncateText(t.ActionTarget, 40), truncateText(t.ActionResult, 140)))
		}
	}
	if len(summaryParts) == 0 {
		return
	}
	consolidated := strings.Join(summaryParts, " | ")

	// 1. Learning Loop: propagate Northbound completion signal from L6 up to L1 & L2
	_ = e.bus.Publish(ctx, core.BusEnvelope{
		CycleID:   cycleID,
		AgentID:   e.agentID,
		Topic:     "ace.bus.north.L6->L1.learning_loop",
		Direction: core.DirectionNorthbound,
		FromLayer: 6,
		ToLayer:   1,
		Kind:      "action_result",
		Summary:   "L6 Verified Outcome: " + truncateText(consolidated, 140),
		Payload:   consolidated,
	})

	e.mu.Lock()
	e.layers[core.LayerAspirational].NorthboundSignal = "L6 Outcome: " + truncateText(consolidated, 140)
	e.layers[core.LayerGlobalStrategy].NorthboundSignal = "L6 Outcome: " + truncateText(consolidated, 140)
	l1Snap := *e.layers[core.LayerAspirational]
	l2Snap := *e.layers[core.LayerGlobalStrategy]
	e.mu.Unlock()
	_ = e.memory.SaveLayerStatus(ctx, l1Snap)
	_ = e.memory.SaveLayerStatus(ctx, l2Snap)
	e.recordGlobalLoop("learning_loop", "healthy", "Propagated L6 outcome up to L1/L2: "+truncateText(last.StateSummary, 90))

	// 2. Memory Manager: only store verified, non-failing cycle outcomes in GLOBAL FTS5 memory (Fix S9)
	isCleanSuccess := !strings.Contains(consolidated, "exit=1") &&
		!strings.Contains(consolidated, "exit=2") &&
		!strings.Contains(consolidated, "[LTL VETO") &&
		!strings.Contains(consolidated, "Traceback") &&
		!strings.Contains(consolidated, "error:")
	if isCleanSuccess {
		_, _ = e.memory.Remember(ctx, core.MemoryEntry{
			AgentID:   e.agentID,
			LayerCode: "GLOBAL",
			Tier:      "medium",
			Category:  "episodic",
			Title:     fmt.Sprintf("Cycle %s Outcome: %s", cycleID, truncateText(last.StateSummary, 60)),
			Content:   consolidated,
			Tags:      "global,cycle_outcome,l6_verified",
		})
	}
	e.recordGlobalLoop("memory_manager", "healthy", fmt.Sprintf("Consolidated cycle %s (%d L6 steps) into GLOBAL medium-term memory", cycleID, len(l6Traces)))

	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:      cycleID,
		ParentSpanID: parentSpanID,
		AgentID:      e.agentID,
		LayerCode:    "GLOBAL",
		Operation:    "northbound.reflect",
		Status:       "ok",
		DurationMs:   time.Since(start).Milliseconds(),
		Detail:       truncateText(consolidated, 200),
	})
}

// buildCanonicalIndexWithSkills returns index.md + the compact Pi Skills block identically for ALL layers and Chat,
// guaranteeing that the first ~8,500 bytes of SystemPrompt are 100% byte-for-byte identical across L1..L6 and FastChat!
func (e *Engine) buildCanonicalIndexWithSkills() string {
	index, _ := e.memory.ReadWiki("index.md")
	index = truncateText(index, 2000)
	var skillsBuf strings.Builder
	for _, sp := range []string{"skills/ponytail.md", "skills/tickets.md", "skills/lyteknowledge.md"} {
		if sc, err := e.memory.ReadWiki(sp); err == nil {
			skillsBuf.WriteString("\n" + strings.TrimSpace(sc) + "\n")
		}
	}
	if skillsBuf.Len() > 0 {
		index = index + "\n\n=== ACTIVE PI SKILLS ===\n" + skillsBuf.String()
	}
	return index
}

func (e *Engine) executeLayerStep(
	ctx context.Context,
	cycleID string,
	layer core.LayerID,
	stimulus string,
) (core.ThoughtTrace, error) {
	return e.executeLayerStepWithTurns(ctx, cycleID, layer, stimulus, nil)
}

func (e *Engine) executeLayerStepWithTurns(
	ctx context.Context,
	cycleID string,
	layer core.LayerID,
	stimulus string,
	priorTurns []core.ChatTurn,
) (core.ThoughtTrace, error) {
	var frozen string
	return e.executeLayerStepWithFrozenPrompt(ctx, cycleID, layer, stimulus, &frozen, priorTurns)
}

func (e *Engine) executeLayerStepWithFrozenPrompt(
	ctx context.Context,
	cycleID string,
	layer core.LayerID,
	stimulus string,
	frozenFirstUserPrompt *string,
	priorTurns []core.ChatTurn,
) (core.ThoughtTrace, error) {
	e.mu.Lock()
	e.currentLayer = int(layer)
	st := e.layers[layer]
	st.State = "thinking"
	st.LiveStream = ""
	st.ActiveDirective = stimulus
	st.Model = e.llm.ActiveModel()
	st.UpdatedAt = time.Now().UTC()
	snapshot := *st
	e.mu.Unlock()

	e.telem.EmitLive("layer.status", snapshot)
	e.telem.EmitLive("engine.status", e.Status())

	soul, _ := e.memory.ReadWiki("soul.md")
	index := e.buildCanonicalIndexWithSkills()
	sysPrompt := LayerSystemPrompt(layer, soul, index)

	var userPrompt string
	if frozenFirstUserPrompt != nil && *frozenFirstUserPrompt != "" {
		// Reuse the exact byte-for-byte first user turn across Steps 2..N of the L6 inner loop
		// so that 100% of SystemPrompt + Turn 1 UserPrompt + prior assistant/user turns hit the KV prefix cache!
		userPrompt = *frozenFirstUserPrompt
	} else {
		rawGoals, _ := e.memory.ReadWiki("goals.md")
		topMilestone := extractTopActionableMilestone(rawGoals)
		goals := extractActiveGoals(rawGoals)
		wsDir, wikiDir := e.resolveSandboxDirs()
		var oraclePacket string
		if topMilestone != "" && wsDir != "" {
			oraclePacket = BuildMilestoneOraclePacket(topMilestone, wsDir, wikiDir)
		}

		wsFiles, _ := e.sandbox.ListFiles()
		wikiList, _ := e.memory.ListWiki()

		var manifestBuilder strings.Builder
		if len(wsFiles) > 0 {
			manifestBuilder.WriteString("=== SANDBOX /workspace FILES ===\n")
			for _, f := range wsFiles {
				lp := strings.ToLower(f.Path)
				if strings.HasSuffix(lp, ".log") || strings.HasSuffix(lp, ".out") ||
					strings.HasSuffix(lp, ".err") || strings.Contains(lp, ".bak") ||
					strings.HasSuffix(lp, ".pid") || strings.HasPrefix(lp, ".tmp") ||
					strings.HasSuffix(lp, ".txt") {
					continue
				}
				manifestBuilder.WriteString(fmt.Sprintf("- %s\n", f.Path))
			}
			manifestBuilder.WriteString("\n")
		}
		if len(wikiList) > 0 {
			manifestBuilder.WriteString("=== CANONICAL WIKI FILES (/wiki) ===\n")
			for _, w := range wikiList {
				if strings.HasPrefix(w.Path, "notes/") {
					continue
				}
				manifestBuilder.WriteString(fmt.Sprintf("- %s\n", w.Path))
			}
		}
		manifestBlock := truncateText(manifestBuilder.String(), SLABucketManifestMaxBytes)

		var userPromptBuilder strings.Builder
		if layer == core.LayerTaskProsecution {
			// L6 REFLEX CACHE ORDER:
			// 1. LayerRoleSuffix(L6) (100% static across all L6 calls)
			// 2. Immutable Reference SDK (/workspace/ace_rlm.py, 100% static across all L6 calls)
			// 3. Milestone-Pinned Active Goals + Oracle Gate (100% static for the milestone)
			// 4. Hot Files (read-only reference files FIRST, mutable primary target script LAST)
			// 5. File Manifest (placed AFTER hot files so L2/L4 wiki notes never bust the 64KB hot-file prefix!)
			userPromptBuilder.WriteString(LayerRoleSuffix(layer))
			userPromptBuilder.WriteString("\n\n")
			if immRef := e.buildImmutableReferencePreInjection(); immRef != "" {
				userPromptBuilder.WriteString(immRef)
				userPromptBuilder.WriteString("\n")
			}
			if strings.TrimSpace(goals) != "" {
				userPromptBuilder.WriteString(fmt.Sprintf("=== ACTIVE GOALS & ORACLE CONFORMANCE GATE (goals.md) ===\n%s\n\n", truncateText(goals, SLABucketGoalsMaxBytes)))
			}
			hotKey := topMilestone
			if hotKey == "" {
				hotKey = goals
			}
			if hotFiles := e.buildHotFilePreInjection(hotKey); hotFiles != "" {
				userPromptBuilder.WriteString(truncateText(strings.TrimSpace(hotFiles), SLABucketHotFilesTotalMaxBytes))
				userPromptBuilder.WriteString("\n\n")
			}
			userPromptBuilder.WriteString(manifestBlock)
			userPromptBuilder.WriteString("\n\n")
		} else {
			// L1..L5 COMPASS & TACTICAL CACHE ORDER:
			// 1. Milestone-Pinned Active Goals + Oracle Gate (100% identical across L1..L4)
			// 2. Environmental & Literature Grounding Digest (100% identical across L1..L4)
			// 3. Static Workspace Data Sources & Benchmark Registry (100% identical across L1..L4, no volatile trace counters!)
			// 4. File Manifest (100% identical across L1..L3 and L4)
			// 5. LayerRoleSuffix(layer) placed at the suffix right before the dynamic bus tail!
			if strings.TrimSpace(goals) != "" {
				userPromptBuilder.WriteString(fmt.Sprintf("=== ACTIVE GOALS & ORACLE CONFORMANCE GATE (goals.md) ===\n%s\n\n", truncateText(goals, SLABucketGoalsMaxBytes)))
			}
			if litDigest := e.buildEnvironmentalGroundingDigest(topMilestone); litDigest != "" {
				userPromptBuilder.WriteString(litDigest)
				userPromptBuilder.WriteString("\n\n")
			}
			if invDigest := e.buildStaticWorkspaceInventoryDigest(); invDigest != "" {
				userPromptBuilder.WriteString(invDigest)
				userPromptBuilder.WriteString("\n\n")
			}
			userPromptBuilder.WriteString(manifestBlock)
			userPromptBuilder.WriteString("\n\n")
			userPromptBuilder.WriteString(LayerRoleSuffix(layer))
			userPromptBuilder.WriteString("\n\n")
		}

		// DYNAMIC SUFFIX SEGMENT: Active Milestone Wiki Notes (L2 grounding / L4 contract notes placed in suffix so writing them never busts the KV prefix!)
		if activeNotes := e.buildActiveMilestoneWikiNotesDigest(topMilestone); activeNotes != "" {
			userPromptBuilder.WriteString(activeNotes)
			userPromptBuilder.WriteString("\n")
		}

		// === DYNAMIC CYCLE TAIL: BIDIRECTIONAL BUSES, LIVE TELEMETRY, LAYER STATES, EPISODIC MEMORIES & HANDOFF ===
		var zoneBuilder strings.Builder
		zoneBuilder.WriteString("=== 6-LAYER COGNITIVE STATE & BIDIRECTIONAL BUSES ===\n")
		zoneBuilder.WriteString(e.buildLiveTelemetrySummaryLine())
		if oraclePacket != "" {
			zoneBuilder.WriteString("- " + oraclePacket + "\n")
		}
		e.mu.RLock()
		for _, id := range core.AllLayers() {
			l := e.layers[id]
			zoneBuilder.WriteString(fmt.Sprintf("- %s [%s]: %s\n", l.Code, id.Zone(), truncateText(l.Summary, 70)))
		}
		e.mu.RUnlock()
		if busCtx := e.buildBidirectionalBusContext(ctx, layer); busCtx != "" {
			zoneBuilder.WriteString(busCtx)
		}
		if memHits, mErr := e.memory.SearchForLayer(ctx, layer.Code(), topMilestone, 3); mErr == nil && len(memHits) > 0 {
			var memLines []string
			for _, m := range memHits {
				memLines = append(memLines, fmt.Sprintf("  * [%s/%s] %s: %s", m.LayerCode, m.Tier, m.Title, truncateText(m.Content, 140)))
			}
			zoneBuilder.WriteString("- Relevant Episodic/Semantic Memories:\n" + truncateText(strings.Join(memLines, "\n"), SLABucketMemoriesMaxBytes) + "\n")
		}
		handoff := LoadHandoffPacket(ctx, e.memory, topMilestone)
		userPromptBuilder.WriteString(FormatHandoffBucket(handoff, zoneBuilder.String()))

		// SLA BUCKET 8 (STIMULUS <= 1,600B):
		userPromptBuilder.WriteString(fmt.Sprintf("\n\n=== INCOMING STIMULUS / DIRECTIVE (%s) ===\n%s",
			layer.Channel(), truncateText(stimulus, SLABucketStimulusMaxBytes)))
		userPrompt = userPromptBuilder.String()
		if frozenFirstUserPrompt != nil {
			*frozenFirstUserPrompt = userPrompt
		}
	}

	messages := make([]core.ChatTurn, 0, 1+len(priorTurns))
	messages = append(messages, core.ChatTurn{Role: "user", Content: userPrompt})
	messages = append(messages, priorTurns...)

	maxTok := 8192
	if layer == core.LayerTaskProsecution {
		maxTok = 32768
	}

	var streamAcc strings.Builder
	resp, err := e.llm.StreamComplete(ctx, core.CompletionRequest{
		SystemPrompt: sysPrompt,
		Messages:     messages,
		MaxTokens:    maxTok,
		Temperature:  0.3,
	}, func(d core.StreamDelta) {
		piece := d.ReasoningDelta + d.ContentDelta
		if piece != "" {
			streamAcc.WriteString(piece)
			e.mu.Lock()
			if curSt := e.layers[layer]; curSt != nil {
				curSt.LiveStream += piece
				if len(curSt.LiveStream) > 4000 {
					curSt.LiveStream = curSt.LiveStream[len(curSt.LiveStream)-4000:]
				}
			}
			e.mu.Unlock()
			e.telem.EmitLive("layer.stream", map[string]interface{}{
				"layer_id":   int(layer),
				"layer_code": layer.Code(),
				"cycle_id":   cycleID,
				"delta":      piece,
			})
		}
	})

	if err != nil {
		e.mu.Lock()
		st := e.layers[layer]
		isCooldown := strings.Contains(err.Error(), "cooling down") || strings.Contains(err.Error(), "FreeUsageLimitError") || strings.Contains(err.Error(), "HTTP 429")
		if isCooldown {
			st.State = "idle"
			if strings.HasPrefix(st.Summary, "Error:") || strings.HasPrefix(st.Summary, "Ready") || st.Summary == "" {
				st.Summary = "Waiting for provider rate-limit window to clear (auto-resumes on Retry-After)"
			}
		} else {
			st.State = "error"
			st.Summary = "Error: " + truncateText(err.Error(), 100)
		}
		st.UpdatedAt = time.Now().UTC()
		snap := *st
		e.mu.Unlock()
		e.telem.EmitLive("layer.status", snap)
		return core.ThoughtTrace{}, err
	}

	decision := ParseLayerDecision(resp.Content)
	if decision.Thought == "" && resp.Reasoning != "" {
		decision.Thought = resp.Reasoning
	}

	// Execute single action OR Hermes-style batched actions[] in a single turn!
	var rawCandidates []core.LayerAction
	if decision.Action.Type != "" && decision.Action.Type != "none" {
		rawCandidates = append(rawCandidates, decision.Action)
	}
	for _, a := range decision.Actions {
		if a.Type != "" && a.Type != "none" {
			rawCandidates = append(rawCandidates, a)
		}
	}

	hasWorkspaceMutationInBatch := false
	for _, c := range rawCandidates {
		ct := strings.ToLower(strings.TrimSpace(c.Type))
		if ct == "workspace_write" || ct == "workspace_edit" {
			hasWorkspaceMutationInBatch = true
			break
		}
	}

	var mutationActions []core.LayerAction
	var shellOrOtherActions []core.LayerAction
	var deferredGoalsWrites []core.LayerAction
	for _, c := range rawCandidates {
		ct := strings.ToLower(strings.TrimSpace(c.Type))
		// If the model emitted simulated workspace_read turns followed by workspace_write/workspace_edit in the same response,
		// drop the redundant workspace_read actions so they don't consume batch slots or trigger LTL G4 vetoes!
		if hasWorkspaceMutationInBatch && ct == "workspace_read" {
			continue
		}
		isDup := false
		for i, ex := range mutationActions {
			if strings.EqualFold(ex.Type, c.Type) && strings.EqualFold(strings.TrimSpace(ex.Target), strings.TrimSpace(c.Target)) {
				if (ct == "wiki_write" || ct == "workspace_write") && len(c.Content) > len(ex.Content) {
					mutationActions[i] = c
				}
				if ex.Content == c.Content || ct == "wiki_write" || ct == "workspace_write" {
					isDup = true
					break
				}
			}
		}
		if !isDup {
			for _, ex := range shellOrOtherActions {
				if ex.Type == c.Type && ex.Target == c.Target && ex.Content == c.Content {
					isDup = true
					break
				}
			}
		}
		if isDup {
			continue
		}
		if ct == "wiki_write" && strings.Contains(strings.ToLower(c.Target), "goals.md") {
			deferredGoalsWrites = append(deferredGoalsWrites, c)
			continue
		}
		if ct == "workspace_write" || ct == "workspace_edit" || ct == "wiki_write" {
			mutationActions = append(mutationActions, c)
		} else {
			shellOrOtherActions = append(shellOrOtherActions, c)
		}
	}
	// Always execute workspace_write/workspace_edit/wiki_write BEFORE sandbox_sh in a batched turn,
	// even if the model placed sandbox_sh in top-level `action` and workspace_edit in `actions[]`!
	primaryActions := append(mutationActions, shellOrOtherActions...)
	if len(primaryActions) > 6 {
		primaryActions = primaryActions[:6]
	}
	actionsToRun := append(primaryActions, deferredGoalsWrites...)
	if len(actionsToRun) > 7 {
		actionsToRun = actionsToRun[:7]
	}

	// Enforce ACE Layer Boundaries (Shapiro et al. Section 3):
	// - L1 (Aspirational) is strictly introspective & read-only on wiki/memory (no external or workspace actuators).
	// - L2 (Global Strategy) may use web_search, web_fetch, workspace_read, wiki_read, wiki_write, memory_search, memory_remember.
	// - L3 (Agent Model) may use workspace_read, wiki_read, wiki_write, memory_search, memory_remember, kb.
	// - L4 (Executive) may use workspace_read, wiki_read, wiki_write, memory_search, memory_remember, ticket.
	// - L5 (Cognitive Control) may use memory_search.
	// - L6 (Task Prosecution) has full environmental & workspace actuator access.
	if layer < core.LayerTaskProsecution && len(actionsToRun) > 0 {
		var filtered []core.LayerAction
		wikiWriteCount := 0
		for _, act := range actionsToRun {
			at := strings.ToLower(strings.TrimSpace(act.Type))
			tgt := strings.TrimSpace(act.Target)
			// L1 is strictly introspective: block workspace_read, wiki_write, and external actuators in L1
			if layer == core.LayerAspirational && at != "wiki_read" && at != "memory_search" && at != "memory_remember" {
				if decision.SouthboundDirective == "" && act.Content != "" {
					decision.SouthboundDirective = truncateText(act.Content, 240)
				}
				continue
			}
			// Block ephemeral pulse notes in L1..L4, restrict goals.md writes strictly to L2 (Global Strategy), and cap wiki_write to 1 per upper-layer turn
			if at == "wiki_write" {
				isGoals := strings.Contains(strings.ToLower(tgt), "goals.md")
				if strings.Contains(tgt, "pulse") || wikiWriteCount >= 1 || (isGoals && layer != core.LayerGlobalStrategy) {
					if decision.SouthboundDirective == "" && act.Content != "" {
						decision.SouthboundDirective = truncateText(act.Content, 240)
					}
					continue
				}
				wikiWriteCount++
			}
			// Only L6 may execute sandbox_sh, workspace_edit, workspace_write, or http_request
			if at == "sandbox_sh" || at == "workspace_edit" || at == "workspace_write" || at == "http_request" {
				if decision.SouthboundDirective == "" && tgt != "" {
					decision.SouthboundDirective = fmt.Sprintf("Execute %s on %s", at, truncateText(tgt, 180))
				}
				continue
			}
			filtered = append(filtered, act)
		}
		actionsToRun = filtered
		if len(actionsToRun) == 0 {
			decision.Action = core.LayerAction{Type: "none"}
		}
	}

	var actionResults []string
	var actionTypes []string
	var actionTargets []string

	// Enforce Batch-Level LTL Invariant G4 (Anti-Analysis-Paralysis Guard) on L6:
	// Once L6 has already used 1 read-only inspection turn (handoff.ConsecutiveNoOp >= 1), any subsequent turn that does NOT
	// include at least one mutating action (workspace_write, workspace_edit, wiki_write, or sandbox_sh writing a .py/.sh file) is VETOED!
	if layer == core.LayerTaskProsecution && len(actionsToRun) > 0 {
		rawGoals, _ := e.memory.ReadWiki("goals.md")
		topMilestone := extractTopActionableMilestone(rawGoals)
		handoff := LoadHandoffPacket(ctx, e.memory, topMilestone)
		maxAllowedNoOp := 1
		e.mu.RLock()
		if e.l6MutatedInCycle {
			maxAllowedNoOp = 2
		}
		e.mu.RUnlock()
		if handoff.ConsecutiveNoOp >= maxAllowedNoOp {
			batchHasMutation := false
			for _, act := range actionsToRun {
				at := strings.ToLower(strings.TrimSpace(act.Type))
				tgt := strings.TrimSpace(act.Target)
				if at == "workspace_write" || at == "workspace_edit" || at == "wiki_write" {
					batchHasMutation = true
					break
				}
				if at == "sandbox_sh" && strings.TrimSpace(act.Content) != "" && (strings.HasSuffix(tgt, ".py") || strings.HasSuffix(tgt, ".sh")) {
					batchHasMutation = true
					break
				}
			}
			if !batchHasMutation {
				vetoMsg := fmt.Sprintf("[LTL VETO G4 — Analysis Paralysis Guard] You have already performed %d consecutive read-only inspection turn(s) without creating any target artifact. Further read-only sandbox_sh/workspace_read turns are BLOCKED. Emit `workspace_write` and/or `wiki_write` NOW to write the required milestone artifact!", handoff.ConsecutiveNoOp)
				_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
					TraceID:   cycleID,
					AgentID:   e.agentID,
					LayerCode: layer.Code(),
					Operation: "gate.ltl",
					Model:     resp.Model,
					Status:    "veto",
					Detail:    truncateText(vetoMsg, 160),
				})
				actionResults = append(actionResults, vetoMsg)
				actionTypes = append(actionTypes, actionsToRun[0].Type)
				actionTargets = append(actionTargets, actionsToRun[0].Target)
				actionsToRun = nil
			}
		}
	}

	for _, act := range actionsToRun {
		res := e.executeLayerAction(ctx, cycleID, layer, act, resp.Model)
		actionResults = append(actionResults, res)
		actionTypes = append(actionTypes, act.Type)
		actionTargets = append(actionTargets, act.Target)
		// If an in-batch file mutation succeeded, immediately clear stale LTL G2 failed attempts before the next action (e.g. sandbox_sh --selftest) in the same batch runs!
		if layer == core.LayerTaskProsecution {
			at := strings.ToLower(strings.TrimSpace(act.Type))
			if (at == "workspace_write" || at == "workspace_edit" || at == "wiki_write") &&
				!strings.HasPrefix(res, "error:") && !strings.Contains(res, "[LTL VETO") && !strings.Contains(res, "SYNTAX ERROR") {
				e.mu.Lock()
				e.l6MutatedInCycle = true
				e.mu.Unlock()
				rawGoals, _ := e.memory.ReadWiki("goals.md")
				topMilestone := extractTopActionableMilestone(rawGoals)
				if topMilestone != "" {
					ho := LoadHandoffPacket(ctx, e.memory, topMilestone)
					ho.FailedAttempts = nil
					ho.ConsecutiveNoOp = 0
					SaveHandoffPacket(ctx, e.memory, ho)
				}
			}
		}
	}
	actionResult := strings.Join(actionResults, "\n---\n")
	combinedActType := decision.Action.Type
	combinedActTarget := decision.Action.Target
	if len(actionTypes) > 1 {
		combinedActType = strings.Join(actionTypes, "+")
		combinedActTarget = strings.Join(actionTargets, " ; ")
	}

	// Publish Southbound directive on Zone Channel if present and not at L6
	if strings.TrimSpace(decision.SouthboundDirective) != "" && layer < core.LayerTaskProsecution {
		toLayer := int(layer) + 1
		_ = e.bus.Publish(ctx, core.BusEnvelope{
			CycleID:   cycleID,
			AgentID:   e.agentID,
			Topic:     fmt.Sprintf("%s.%s->L%d", layer.Channel(), layer.Code(), toLayer),
			Direction: core.DirectionSouthbound,
			FromLayer: int(layer),
			ToLayer:   toLayer,
			Kind:      "directive",
			Summary:   truncateText(decision.SouthboundDirective, 120),
			Payload:   decision.SouthboundDirective,
		})
	}

	// Publish Northbound telemetry/escalation if present and above L1
	northSignal := strings.TrimSpace(decision.NorthboundSignal)
	if actionResult != "" {
		if northSignal != "" {
			northSignal = northSignal + " | Action Result: " + truncateText(actionResult, 220)
		} else {
			northSignal = "Action Result: " + truncateText(actionResult, 220)
		}
	}
	if northSignal != "" && layer > core.LayerAspirational {
		toLayer := int(layer) - 1
		_ = e.bus.Publish(ctx, core.BusEnvelope{
			CycleID:   cycleID,
			AgentID:   e.agentID,
			Topic:     fmt.Sprintf("ace.channel.telemetry.%s->L%d", layer.Code(), toLayer),
			Direction: core.DirectionNorthbound,
			FromLayer: int(layer),
			ToLayer:   toLayer,
			Kind:      "telemetry",
			Summary:   truncateText(northSignal, 160),
			Payload:   northSignal,
		})
	}

	trace := core.ThoughtTrace{
		ID:               "tht_" + uuid.NewString()[:12],
		CycleID:          cycleID,
		AgentID:          e.agentID,
		LayerID:          int(layer),
		LayerCode:        layer.Code(),
		LoopKind:         "cognitive",
		Model:            resp.Model,
		TriggerSummary:   truncateText(stimulus, 120),
		SystemPrompt:     sysPrompt,
		UserPrompt:       userPrompt,
		RawReasoning:     resp.Reasoning,
		RawResponse:      resp.Content,
		Thought:          decision.Thought,
		StateSummary:     decision.StateSummary,
		Southbound:       decision.SouthboundDirective,
		Northbound:       decision.NorthboundSignal,
		ActionType:       combinedActType,
		ActionTarget:     combinedActTarget,
		ActionResult:     actionResult,
		Confidence:       decision.Confidence,
		LatencyMs:        resp.LatencyMs,
		PromptTokens:     resp.PromptTokens,
		CachedTokens:     resp.CachedTokens,
		CompletionTokens: resp.CompletionTokens,
		CreatedAt:        time.Now().UTC(),
	}
	_ = e.telem.RecordThought(ctx, trace)
	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:          cycleID,
		AgentID:          e.agentID,
		LayerCode:        layer.Code(),
		Operation:        "layer.step",
		Model:            resp.Model,
		Status:           "ok",
		DurationMs:       resp.LatencyMs,
		PromptTokens:     trace.PromptTokens,
		CompletionTokens: trace.CompletionTokens,
		Detail:           fmt.Sprintf("%s: %s", layer.Code(), truncateText(decision.StateSummary, 100)),
	})

	e.mu.Lock()
	e.totalThoughts++
	st = e.layers[layer]
	st.State = "idle"
	st.Summary = decision.StateSummary
	st.LastThought = decision.Thought
	st.LiveStream = ""
	if decision.SouthboundDirective != "" {
		st.ActiveDirective = decision.SouthboundDirective
	}
	if northSignal != "" {
		st.NorthboundSignal = northSignal
	}
	if actionResult != "" && !strings.Contains(actionResult, "restricted to L6") {
		st.LastAction = fmt.Sprintf("%s(%s): %s", decision.Action.Type, truncateText(decision.Action.Target, 50), truncateText(actionResult, 160))
	}
	st.Confidence = decision.Confidence
	st.Model = resp.Model
	st.LastLatencyMs = resp.LatencyMs
	st.TotalInvocations++
	st.UpdatedAt = time.Now().UTC()
	finalSnap := *st
	totalCyc := fmt.Sprintf("%d", e.totalCycles)
	e.mu.Unlock()

	_ = e.memory.SaveLayerStatus(ctx, finalSnap)
	_ = e.memory.SaveEngineMeta(ctx, "total_cycles", totalCyc)
	e.telem.EmitLive("layer.status", finalSnap)
	return trace, nil
}

func (e *Engine) checkSafety(ctx context.Context, cycleID string, layer core.LayerID, act core.LayerAction) error {
	actType := strings.ToLower(strings.TrimSpace(act.Type))
	if actType == "" || actType == "none" {
		return nil
	}
	// Safety Monitor Global Loop checks L1 boundaries + sandbox invariants
	combined := strings.ToLower(act.Target + " " + act.Content)
	forbidden := []string{"/etc/shadow", "rm -rf /", "mkfs", ":(){:|:&};:"}
	if homeDir, hErr := os.UserHomeDir(); hErr == nil && homeDir != "" {
		forbidden = append(forbidden, strings.ToLower(homeDir))
	}
	for _, pat := range forbidden {
		if strings.Contains(combined, pat) {
			msg := fmt.Sprintf("Blocked forbidden pattern %q in %s(%s)", pat, layer.Code(), actType)
			e.recordGlobalLoop("safety_monitor", "blocked", msg)
			_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
				TraceID:   cycleID,
				AgentID:   e.agentID,
				LayerCode: layer.Code(),
				Operation: "safety.check",
				Status:    "blocked",
				Detail:    msg,
			})
			return errors.New(msg)
		}
	}
	e.recordGlobalLoop("safety_monitor", "healthy", fmt.Sprintf("Verified %s %s(%s)", layer.Code(), actType, truncateText(act.Target, 40)))
	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:   cycleID,
		AgentID:   e.agentID,
		LayerCode: layer.Code(),
		Operation: "safety.check",
		Status:    "ok",
		Detail:    fmt.Sprintf("%s %s(%s)", layer.Code(), actType, truncateText(act.Target, 60)),
	})
	return nil
}

func (e *Engine) executeLayerAction(
	ctx context.Context,
	cycleID string,
	layer core.LayerID,
	act core.LayerAction,
	model string,
) string {
	actType := strings.ToLower(strings.TrimSpace(act.Type))
	if actType == "" || actType == "none" {
		return ""
	}

	if err := e.checkSafety(ctx, cycleID, layer, act); err != nil {
		return "safety_monitor blocked action: " + err.Error()
	}

	// Enforce AgentLTL Temporal Safety & Anti-Amnesia Monitor (G1, G2, G3) on L6 actions
	if layer == core.LayerTaskProsecution {
		rawGoals, _ := e.memory.ReadWiki("goals.md")
		topMilestone := extractTopActionableMilestone(rawGoals)
		handoff := LoadHandoffPacket(ctx, e.memory, topMilestone)
		if ok, vetoReason := EvaluateLTLAction(act, handoff, nil); !ok {
			_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
				TraceID:   cycleID,
				AgentID:   e.agentID,
				LayerCode: layer.Code(),
				Operation: "gate.ltl",
				Model:     model,
				Status:    "veto",
				Detail:    truncateText(vetoReason, 160),
			})
			return vetoReason
		}
	}

	// Cordis Spatiotemporal LIFO Snapshot before workspace file mutations; auto-revert on broken Python AST syntax!
	wsDir, _ := e.resolveSandboxDirs()
	var fiber *CordisFiber
	if layer == core.LayerTaskProsecution && wsDir != "" && (actType == "workspace_write" || actType == "workspace_edit") {
		fiber = NewCordisFiber(wsDir)
		fiber.SnapshotFile(act.Target)
	}

	toolStart := time.Now()
	resStr := e.dispatchLayerAction(ctx, cycleID, layer, actType, act, model)
	if fiber != nil && strings.Contains(resStr, "[Pi-Lens Python AST SYNTAX ERROR]") {
		reverted := fiber.RevertAll()
		resStr = fmt.Sprintf("%s\n[CORDIS LIFO ROLLBACK]: Automatically reverted %v to pre-mutation state due to Python AST syntax error.", resStr, reverted)
		_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
			TraceID:   cycleID,
			AgentID:   e.agentID,
			LayerCode: layer.Code(),
			Operation: "gate.cordis",
			Model:     model,
			Status:    "veto",
			Detail:    "Reverted broken AST mutation on " + act.Target,
		})
	}
	_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
		TraceID:    cycleID,
		AgentID:    e.agentID,
		LayerCode:  layer.Code(),
		Operation:  "tool.exec",
		Model:      model,
		Status:     "ok",
		DurationMs: time.Since(toolStart).Milliseconds(),
		Detail:     fmt.Sprintf("%s(%s) -> %s", actType, truncateText(act.Target, 50), truncateText(resStr, 90)),
	})
	return resStr
}

func (e *Engine) dispatchLayerAction(
	ctx context.Context,
	cycleID string,
	layer core.LayerID,
	actType string,
	act core.LayerAction,
	model string,
) string {
	switch actType {
	case "memory_remember":
		title := strings.TrimSpace(act.Target)
		if title == "" {
			title = fmt.Sprintf("%s Cognitive Insight", layer.Code())
		}
		tier := "medium"
		if layer <= core.LayerGlobalStrategy {
			tier = "long"
		}
		entry, err := e.memory.Remember(ctx, core.MemoryEntry{
			AgentID:   e.agentID,
			LayerCode: layer.Code(),
			Tier:      tier,
			Category:  "insight",
			Title:     title,
			Content:   act.Content,
			Tags:      strings.ToLower(layer.Code()) + ",cognitive",
		})
		if err != nil {
			return "memory_remember error: " + err.Error()
		}
		return "Saved FTS5 memory: " + entry.ID + " (" + entry.Title + ")"

	case "memory_search":
		query := strings.TrimSpace(act.Target)
		if query == "" {
			query = strings.TrimSpace(act.Content)
		}
		hits, err := e.memory.SearchForLayer(ctx, layer.Code(), query, 5)
		if err != nil {
			return "memory_search error: " + err.Error()
		}
		if len(hits) == 0 {
			return fmt.Sprintf("memory_search(%q): 0 matching memories found", query)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("=== Episodic/Semantic Memory Search (%q, %d hits) ===\n", query, len(hits)))
		for _, h := range hits {
			sb.WriteString(fmt.Sprintf("- [%s/%s] %s: %s\n", h.LayerCode, h.Tier, h.Title, truncateText(h.Content, 220)))
		}
		return sb.String()

	case "workspace_read":
		cleanTgt := strings.TrimSpace(act.Target)
		lowerTgt := strings.ToLower(cleanTgt)
		if strings.HasSuffix(lowerTgt, ".db") || strings.HasSuffix(lowerTgt, ".sqlite") || strings.HasSuffix(lowerTgt, ".db-shm") || strings.HasSuffix(lowerTgt, ".db-wal") {
			return "[Binary SQLite database — do NOT call workspace_read on .db files! Exact table & column schemas are pre-injected in your prompt; use ace_rlm.inspect_db_schema(db_path) or ace_rlm.query_sqlite(sql, params, db_path) in L6 Python scripts to query rows.]"
		}
		if cleanTgt == "goals.md" || strings.HasSuffix(cleanTgt, "/goals.md") || strings.HasPrefix(cleanTgt, "../wiki/") || strings.HasPrefix(cleanTgt, "/wiki/") || strings.HasPrefix(cleanTgt, "wiki/") || strings.HasPrefix(cleanTgt, "notes/") {
			wikiRel := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(cleanTgt, "../wiki/"), "/wiki/"), "wiki/")
			if wikiRel == "notes/goals.md" || strings.HasSuffix(wikiRel, "/goals.md") {
				wikiRel = "goals.md"
			}
			wData, wErr := e.memory.ReadWiki(wikiRel)
			if wErr != nil {
				return "wiki read error: " + wErr.Error()
			}
			return fmt.Sprintf("=== /wiki/%s (%d bytes) ===\n%s", wikiRel, len(wData), truncateText(wData, 64000))
		}
		// Support optional line-range syntax on workspace_read (e.g. "pkg/module.py:300-440" or "path.py:300:440")
		readTarget := strings.TrimSpace(act.Target)
		startLine, endLine := 0, 0
		if colonIdx := strings.Index(readTarget, ":"); colonIdx > 0 {
			filePart := strings.TrimSpace(readTarget[:colonIdx])
			rangePart := strings.TrimSpace(readTarget[colonIdx+1:])
			sep := "-"
			if strings.Contains(rangePart, ":") {
				sep = ":"
			}
			parts := strings.SplitN(rangePart, sep, 2)
			if s, sErr := strconv.Atoi(strings.TrimSpace(parts[0])); sErr == nil && s > 0 {
				startLine = s
				readTarget = filePart
				if len(parts) == 2 {
					if eLine, eErr := strconv.Atoi(strings.TrimSpace(parts[1])); eErr == nil && eLine >= s {
						endLine = eLine
					}
				}
			}
		}
		data, err := e.sandbox.ReadFile(readTarget)
		if err != nil {
			if strings.Contains(err.Error(), "is a directory") {
				dirPrefix := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(readTarget, "/workspace/"), "workspace/"), "/")
				allFiles, _ := e.sandbox.ListFiles()
				var listing strings.Builder
				listing.WriteString(fmt.Sprintf("=== Directory /workspace/%s listing ===\n", dirPrefix))
				for _, f := range allFiles {
					if dirPrefix == "" || strings.HasPrefix(f.Path, dirPrefix+"/") {
						listing.WriteString(fmt.Sprintf("- /workspace/%s (%d bytes)\n", f.Path, f.SizeBytes))
					}
				}
				return listing.String()
			}
			return "workspace_read error: " + err.Error()
		}
		cleanRel := strings.TrimPrefix(strings.TrimPrefix(readTarget, "/workspace/"), "workspace/")
		if startLine > 0 {
			hashedLines := strings.Split(sandbox.FormatWithLineHashes(data), "\n")
			if startLine > len(hashedLines) {
				startLine = len(hashedLines)
			}
			if endLine <= 0 || endLine > len(hashedLines) {
				endLine = minInt(len(hashedLines), startLine+200)
			}
			sliced := strings.Join(hashedLines[startLine-1:endLine], "\n")
			return fmt.Sprintf("=== /workspace/%s (lines %d-%d of %d, Pi Hashline Format LINE#HASH | text) ===\n%s",
				cleanRel, startLine, endLine, len(hashedLines), truncateText(sliced, 48000))
		}
		return fmt.Sprintf("=== /workspace/%s (%d bytes, Pi Hashline Format LINE#HASH | text) ===\n%s",
			cleanRel, len(data), smartFormatHotFile(data, 48000))

	case "workspace_edit":
		if layer != core.LayerTaskProsecution {
			return "workspace_edit restricted to L6"
		}
		if err := e.sandbox.EditFile(act.Target, act.OldText, act.Content); err != nil {
			return "workspace_edit error: " + err.Error()
		}
		diag := e.sandbox.DiagnoseFile(ctx, act.Target)
		e.telem.EmitLive("sandbox.updated", map[string]string{"path": act.Target})
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("✂️ Edited `/workspace/%s` (replaced %dB -> %dB) %s", act.Target, len(act.OldText), len(act.Content), diag))
		return strings.TrimSpace("Edited /workspace/" + act.Target + " " + diag)

	case "ticket":
		if layer != core.LayerExecutive && layer != core.LayerTaskProsecution {
			return "ticket restricted to L4 (Executive) and L6 (Task Prosecution)"
		}
		args := strings.TrimSpace(act.Target)
		if args == "" {
			args = strings.TrimSpace(act.Content)
		}
		lowerArgs := strings.ToLower(args)
		if args == "" || lowerArgs == "log" || lowerArgs == "status" || lowerArgs == "check" {
			args = "list"
		}
		res, _ := e.sandbox.Exec(ctx, "ticket "+args)
		out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("🎫 Pi Ticket (`ticket %s`):\n```json\n%s\n```", truncateText(args, 70), truncateText(out, 500)))
		return out

	case "kb":
		if layer != core.LayerAgentModel && layer != core.LayerTaskProsecution {
			return "kb restricted to L3 (Agent Model) and L6 (Task Prosecution)"
		}
		args := strings.TrimSpace(act.Target)
		if args == "" {
			args = strings.TrimSpace(act.Content)
		}
		if args == "" {
			args = "search \"\""
		}
		res, _ := e.sandbox.Exec(ctx, "kb "+args)
		out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("🧠 Pi KB (`kb %s`):\n```json\n%s\n```", truncateText(args, 70), truncateText(out, 500)))
		return out

	case "parallel_research":
		if layer != core.LayerGlobalStrategy && layer != core.LayerTaskProsecution {
			return "parallel_research restricted to L2 (Global Strategy) and L6 (Task Prosecution)"
		}
		rawQueries := strings.Split(act.Target, ";")
		if len(rawQueries) == 1 && strings.TrimSpace(act.Content) != "" {
			rawQueries = strings.Split(act.Content, ";")
		}
		var queries []string
		for _, q := range rawQueries {
			if sq := strings.TrimSpace(q); sq != "" {
				queries = append(queries, sq)
			}
		}
		if len(queries) == 0 {
			return "parallel_research error: provide 1-4 semicolon-separated queries in action.target"
		}
		if len(queries) > 4 {
			queries = queries[:4]
		}
		e.recordGlobalLoop("swarm_coordinator", "active", fmt.Sprintf("Fanning out %d parallel research workers", len(queries)))
		results := make([]string, len(queries))
		var wg sync.WaitGroup
		for i, q := range queries {
			wg.Add(1)
			go func(idx int, query string) {
				defer wg.Done()
				webRes, err := SafeWebSearch(ctx, query)
				if err != nil {
					webRes = "web error: " + err.Error()
				}
				memHits, _ := e.memory.SearchForLayer(ctx, layer.Code(), query, 3)
				var memStr strings.Builder
				for _, m := range memHits {
					memStr.WriteString(fmt.Sprintf("  * [Memory %s] %s: %s\n", m.LayerCode, m.Title, truncateText(m.Content, 140)))
				}
				results[idx] = fmt.Sprintf("### Swarm Worker #%d (`%s`)\n%s\n%s", idx+1, query, truncateText(webRes, 900), memStr.String())
			}(i, q)
		}
		wg.Wait()
		combined := strings.Join(results, "\n\n")
		e.recordGlobalLoop("swarm_coordinator", "healthy", fmt.Sprintf("Completed %d parallel workers (%dB synthesized)", len(queries), len(combined)))
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("🐝 Swarm Coordinator (`%d parallel workers`):\n```\n%s\n```", len(queries), truncateText(combined, 600)))
		return combined

	case "web_search":
		if layer != core.LayerGlobalStrategy && layer != core.LayerTaskProsecution {
			return "web_search restricted to L2 (Global Strategy) and L6 (Task Prosecution)"
		}
		query := strings.TrimSpace(act.Target)
		if query == "" {
			query = strings.TrimSpace(act.Content)
		}
		resText, err := SafeWebSearch(ctx, query)
		if err != nil {
			return "web_search error: " + err.Error()
		}
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("🌐 Live Web Search (`%s`):\n```\n%s\n```", query, truncateText(resText, 600)))
		return resText

	case "web_fetch":
		if layer != core.LayerGlobalStrategy && layer != core.LayerTaskProsecution {
			return "web_fetch restricted to L2 (Global Strategy) and L6 (Task Prosecution)"
		}
		targetURL := strings.TrimSpace(act.Target)
		if targetURL == "" {
			targetURL = strings.TrimSpace(act.Content)
		}
		resText, err := SafeWebFetch(ctx, targetURL)
		if err != nil {
			return "web_fetch error: " + err.Error()
		}
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("🌐 Fetched URL (`%s`):\n```\n%s\n```", targetURL, truncateText(resText, 500)))
		return resText

	case "telegraph_publish":
		if layer != core.LayerTaskProsecution {
			return "telegraph_publish restricted to L6"
		}
		resText, err := PublishToTelegraph(ctx, e.sandbox, act.Target, act.Content)
		if err != nil {
			return "telegraph_publish error: " + err.Error()
		}
		e.emitProactiveUpdate(ctx, cycleID, model, "🚀 "+resText)
		return resText

	case "mail_tm":
		if layer != core.LayerTaskProsecution {
			return "mail_tm restricted to L6"
		}
		resText, err := ExecuteMailTm(ctx, e.sandbox, act.Target)
		if err != nil {
			return "mail_tm error: " + err.Error()
		}
		e.emitProactiveUpdate(ctx, cycleID, model, "📬 "+resText)
		return resText

	case "http_request":
		if layer != core.LayerTaskProsecution {
			return "http_request restricted to L6"
		}
		resText, err := SafeHTTPRequest(ctx, act.Target, act.Content)
		if err != nil {
			return "http_request error: " + err.Error()
		}
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("🌍 Outbound API Request (`%s`):\n```\n%s\n```", act.Target, truncateText(resText, 400)))
		return resText

	case "wiki_read":
		target := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(act.Target), "/wiki/"), "wiki/"))
		if target == "notes/goals.md" || strings.HasSuffix(target, "/goals.md") {
			target = "goals.md"
		}
		wData, err := e.memory.ReadWiki(target)
		if err != nil {
			return "wiki_read error: " + err.Error()
		}
		return fmt.Sprintf("=== /wiki/%s (%d bytes) ===\n%s", target, len(wData), truncateText(wData, 16000))

	case "wiki_write":
		target := strings.TrimSpace(act.Target)
		if strings.HasPrefix(strings.ToLower(target), "read ") {
			target = strings.TrimSpace(target[5:])
			if target == "notes/goals.md" || strings.HasSuffix(target, "/goals.md") {
				target = "goals.md"
			}
			wData, err := e.memory.ReadWiki(target)
			if err != nil {
				return "wiki_read error: " + err.Error()
			}
			return fmt.Sprintf("=== /wiki/%s (%d bytes) ===\n%s", target, len(wData), truncateText(wData, 16000))
		}
		target = strings.TrimPrefix(strings.TrimPrefix(target, "/wiki/"), "wiki/")
		if target == "notes/goals.md" || strings.HasSuffix(target, "/goals.md") {
			target = "goals.md"
		}
		if target == "" {
			target = "notes/insight_" + cycleID + ".md"
		}
		// Never overwrite an existing Wiki page with 0 bytes! If content is empty, treat as a read.
		if strings.TrimSpace(act.Content) == "" {
			if wData, err := e.memory.ReadWiki(target); err == nil {
				return fmt.Sprintf("=== /wiki/%s (%d bytes, non-destructive read since content was empty) ===\n%s", target, len(wData), truncateText(wData, 16000))
			}
			return "wiki_write skipped: action.content cannot be empty when creating a wiki note"
		}
		// Guard against destructive overwrites of goals.md (e.g., dropping remaining - [ ] milestones when checking off [x]):
		if target == "goals.md" {
			existingGoals, _ := e.memory.ReadWiki("goals.md")
			if layer != core.LayerTaskProsecution && extractTopActionableMilestone(existingGoals) != "" {
				return "wiki_write(goals.md) skipped: goals.md already has active '- [ ]' milestones; only L6 may update goals.md when checking off '[x]' after test verification."
			}
			newGoals := strings.TrimSpace(act.Content)
			if len(existingGoals) > 200 {
				wsDir, wikiDir := e.resolveSandboxDirs()
				handoff := LoadHandoffPacket(ctx, e.memory, extractTopActionableMilestone(existingGoals))
				// Apply any newly checked-off "- [x]" milestones in-place onto existingGoals ONLY if Mode-3 Falsification Gate passes (Fix S16)!
				updatedExisting := existingGoals
				appliedInPlace := false
				for _, nl := range strings.Split(newGoals, "\n") {
					tnl := strings.TrimSpace(nl)
					if strings.HasPrefix(tnl, "- [x]") {
						body := strings.TrimSpace(strings.TrimPrefix(tnl, "- [x]"))
						prefix := body
						if len(prefix) > 32 {
							prefix = prefix[:32]
						}
						nlAxisID := extractMilestoneIdentifier(tnl)
						if len(prefix) >= 8 || nlAxisID != "" {
							for _, el := range strings.Split(updatedExisting, "\n") {
								tel := strings.TrimSpace(el)
								if strings.HasPrefix(tel, "- [ ]") && ((len(prefix) >= 8 && strings.Contains(tel, prefix)) || (nlAxisID != "" && extractMilestoneIdentifier(tel) == nlAxisID)) {
									if wsDir != "" && wikiDir != "" {
										gateRes := EvaluateMode3MilestoneGate(ctx, tel, e.sandbox, wsDir, wikiDir, handoff.VerifiedFiles)
										if !gateRes.Passed {
											return fmt.Sprintf("[MODE-3 GATE REJECTED CHECKOFF OF %s]: %s. Fix the missing/failing artifacts before checking off [x].", truncateText(tel, 70), gateRes.Reason)
										}
									}
									checkedLine := strings.Replace(el, "- [ ]", "- [x]", 1)
									updatedExisting = strings.Replace(updatedExisting, el, checkedLine, 1)
									appliedInPlace = true
								}
							}
						}
					}
				}
				// Also append any brand-new "- [ ]" milestones from newGoals that do not exist yet in updatedExisting
				for _, nl := range strings.Split(newGoals, "\n") {
					tnl := strings.TrimSpace(nl)
					if strings.HasPrefix(tnl, "- [ ]") {
						nlAxisID := extractMilestoneIdentifier(tnl)
						alreadyPresent := strings.Contains(updatedExisting, tnl)
						if !alreadyPresent && nlAxisID != "" {
							for _, el := range strings.Split(updatedExisting, "\n") {
								if extractMilestoneIdentifier(strings.TrimSpace(el)) == nlAxisID {
									alreadyPresent = true
									break
								}
							}
						}
						if !alreadyPresent {
							updatedExisting = strings.TrimSpace(updatedExisting) + "\n" + tnl + "\n"
							appliedInPlace = true
						}
					}
				}
				if appliedInPlace {
					newGoals = updatedExisting
				} else if !strings.HasPrefix(strings.TrimSpace(newGoals), "# ") && len(newGoals) < len(existingGoals)/2 {
					newGoals = strings.TrimSpace(existingGoals) + "\n\n" + newGoals
				} else {
					// Preserve existingGoals rather than duplicating headers if no new [x] or [ ] changes applied
					newGoals = updatedExisting
				}
			}
			act.Content = newGoals
		}
		if err := e.memory.WriteWiki(target, act.Content); err != nil {
			return "wiki_write error: " + err.Error()
		}
		existingLog, _ := e.memory.ReadWiki("log.md")
		logEntry := fmt.Sprintf("\n## [%s] %s Updated `%s`\n- Cycle: `%s`\n", time.Now().UTC().Format(time.RFC3339), layer.Code(), target, cycleID)
		_ = e.memory.WriteWiki("log.md", existingLog+logEntry)
		if layer == core.LayerTaskProsecution {
			e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("📚 Updated Wiki page `%s` (%d bytes).", target, len(act.Content)))
		}
		return "Updated Wiki page: " + target

	case "workspace_write":
		if layer != core.LayerTaskProsecution {
			return "workspace_write restricted to L6"
		}
		if strings.TrimSpace(act.Content) == "" {
			return "workspace_write error: action.content cannot be empty"
		}
		if err := e.sandbox.WriteFile(act.Target, act.Content); err != nil {
			return "workspace_write error: " + err.Error()
		}
		diag := e.sandbox.DiagnoseFile(ctx, act.Target)
		e.telem.EmitLive("sandbox.updated", map[string]string{"path": act.Target})
		e.emitProactiveUpdate(ctx, cycleID, model, fmt.Sprintf("🛠️ Wrote `/workspace/%s` (%d bytes) %s", act.Target, len(act.Content), diag))
		return strings.TrimSpace("Wrote /workspace/" + act.Target + " " + diag)

	case "sandbox_sh":
		if layer != core.LayerTaskProsecution {
			return "sandbox_sh restricted to L6"
		}
		cmdStr := strings.TrimSpace(act.Target)
		bodyStr := strings.TrimSpace(act.Content)
		if bodyStr != "" {
			if cmdStr == "python3" || cmdStr == "python" {
				_ = e.sandbox.WriteFile("script.py", bodyStr)
				cmdStr = "python3 /workspace/script.py"
			} else if strings.HasSuffix(cmdStr, ".py") {
				cleanTarget := strings.TrimPrefix(cmdStr, "/workspace/")
				_ = e.sandbox.WriteFile(cleanTarget, bodyStr)
				cmdStr = "python3 /workspace/" + cleanTarget
			} else if strings.HasSuffix(cmdStr, ".sh") {
				cleanTarget := strings.TrimPrefix(cmdStr, "/workspace/")
				_ = e.sandbox.WriteFile(cleanTarget, bodyStr)
				cmdStr = "sh /workspace/" + cleanTarget
			} else if cmdStr == "" || cmdStr == "sh" || cmdStr == "bash" || cmdStr == "/bin/sh" || strings.Contains(bodyStr, "\n") {
				cmdStr = bodyStr
			}
		}
		res, err := e.sandbox.Exec(ctx, cmdStr)
		e.telem.EmitLive("sandbox.exec", res)
		e.telem.EmitLive("sandbox.updated", map[string]string{"path": "/workspace"})
		out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
		spilled := e.sandbox.SpilloverTruncate("exec", out, 4500)
		summary := fmt.Sprintf("exit=0 (%dms): %s", res.DurationMs, spilled)
		if err != nil {
			summary = fmt.Sprintf("exit=%d (%dms): %s", res.ExitCode, res.DurationMs, spilled)
		}
		return summary

	case "chat_proactive":
		if strings.TrimSpace(act.Content) == "" {
			return ""
		}
		msg := core.ChatMessage{
			ID:        "cht_" + uuid.NewString()[:12],
			AgentID:   e.agentID,
			Role:      "proactive",
			Source:    layer.Code(),
			Content:   act.Content,
			Model:     model,
			CycleID:   cycleID,
			CreatedAt: time.Now().UTC(),
		}
		_ = e.memory.SaveChatMessage(ctx, msg)
		return "Sent proactive message to user chat"
	}

	return ""
}

func (e *Engine) emitProactiveUpdate(ctx context.Context, cycleID string, model string, content string) {
	// Raw L6 tool steps (sandbox_sh, workspace_write, workspace_edit, wiki_write) are already broadcast
	// via sandbox.exec, sandbox.updated, wiki.updated, and thought.recorded SSE streams.
	// Do NOT pollute chat_messages with raw tool step logs, as FastChat reads chat_messages as assistant history!
}
