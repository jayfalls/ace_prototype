package cognitive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ace/internal/bus"
	"ace/internal/config"
	"ace/internal/constitution"
	"ace/internal/contextsys"
	"ace/internal/core"
	"ace/internal/gate"
	"ace/internal/layers"
	"ace/internal/plugin"
	"ace/internal/provider"
	"ace/internal/science"
	"ace/internal/tasks"
	"ace/internal/tools"
	"ace/internal/watchdog"
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
	mu                      sync.RWMutex
	agentID                 string
	llm                     core.LLMProvider
	bus                     core.Bus
	memory                  core.MemoryStore
	sandbox                 core.Sandbox
	telem                   core.TelemetrySink
	mode                    core.EngineMode
	autoIntervalSeconds     int
	layers                  map[core.LayerID]*core.LayerStatus
	globalLoops             map[string]*core.GlobalLoopStatus
	busy                    bool
	currentCycleID          string
	currentLayer            int
	gate                    *gate.Gate
	activeDAGTaskID         string
	taskMgr                 *tasks.Manager
	injLog                  *contextsys.InjectionLog
	ctxMap                  *contextsys.Map
	overflow                *contextsys.Overflow
	compactors              map[int]*contextsys.Compactor
	lastPromptToks          map[int]int
	quotaUnits              float64 // spec-9.4 budget governor window limit (tokens); 0 = unbounded
	quotaWindowSecs         int
	lastToolCallAt          string
	lastActionAt            string
	totalCycles             int
	totalThoughts           int
	jobQueue                chan cycleJob
	cancelCurrent           context.CancelFunc
	lastL6ReconMilestone    string
	lastL6ReconTurns        []core.ChatTurn
	l6MutatedInCycle        bool
	lastPreloadedConnectome map[string]bool
	lastTurn05CycleID       string
	lastTurn05Picked        []ConnectomeNode
	plugins                 *plugin.Registry
	mcpCleanup              []func()
	toolsExec               atomic.Pointer[layers.Executor]
	layerKnobs              atomic.Value // config.LayerKnobs
}

func NewEngine(
	llm core.LLMProvider,
	bus core.Bus,
	mem core.MemoryStore,
	sbx core.Sandbox,
	telem core.TelemetrySink,
) *Engine {
	now := time.Now().UTC()
	layerStatus := make(map[core.LayerID]*core.LayerStatus, 6)
	for _, id := range core.AllLayers() {
		maxInner := 1
		if id == core.LayerExecutive {
			maxInner = 2
		} else if id == core.LayerTaskProsecution {
			maxInner = maxPiLoopSteps
		}
		layerStatus[id] = &core.LayerStatus{
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

	var gdb *sql.DB
	if s, ok := mem.(interface{ DB() *sql.DB }); ok {
		gdb = s.DB()
	}
	// pre_model invariant (spec 9.1): every request logged before dispatch.
	llm = plugin.NewRequestLogger(llm, gdb)
	eng := &Engine{
		agentID:             defaultAgentID,
		llm:                 llm,
		bus:                 bus,
		memory:              mem,
		sandbox:             sbx,
		telem:               telem,
		gate:                gate.New(gdb, llm),
		taskMgr:             tasks.New(gdb),
		injLog:              contextsys.NewInjectionLog(gdb),
		ctxMap:              contextsys.NewMap(gdb),
		compactors:          map[int]*contextsys.Compactor{},
		lastPromptToks:      map[int]int{},
		mode:                core.ModeAutonomous,
		autoIntervalSeconds: 20,
		layers:              layerStatus,
		globalLoops:         globalLoops,
		jobQueue:            make(chan cycleJob, 32),
	}
	// Owner-facing gate messages (access requests, spec 8.4) reach the owner
	// through the single owner-voice path — gate stays channels-free.
	eng.gate.Notify = func(msg string) {
		eng.NotifyOwner(context.Background(), "access", msg)
	}
	// Plugin registry (spec 9.1): the "ace-core" builtin plugin hosts the
	// layer-action tools the prosecutor dispatch resolves by name.
	knobs := config.LoadLayerKnobs()
	eng.layerKnobs.Store(knobs)
	eng.toolsExec.Store(layers.NewExecutor(knobs.ParallelReads))
	eng.plugins = plugin.New(func(kind, summary string) {
		telem.EmitLive(kind, map[string]string{"summary": summary})
	})
	_, _ = eng.plugins.Builtin(plugin.Manifest{Name: "ace-core", Kind: "tool", Provides: []string{"builtin-actions"}})
	eng.registerBuiltinActions(eng.plugins, "ace-core")
	tools.RegisterWebTools(eng.plugins, "ace-core", tools.WebDeps{Sandbox: sbx})
	eng.plugins.Isolate("ace-core")
	// Manifest-declared plugins (plugins.dirs: config dir + repo ./plugins).
	// MCP entries spawn their server now so remote tools dispatch like
	// builtins — a manifest alone adds a tool, no Go change (spec 9.1).
	for _, dir := range []string{filepath.Join(config.Dir(), "plugins"), "plugins"} {
		if err := eng.plugins.Load(dir); err != nil {
			telem.EmitLive("plugin_disabled", map[string]string{"summary": fmt.Sprintf("load %s: %v", dir, err)})
			continue
		}
		for _, p := range eng.plugins.List() {
			if p.State != "loaded" || !strings.HasPrefix(p.Manifest.Entry, "mcp:") || len(p.Tools()) > 0 {
				continue
			}
			remote, cleanup, err := plugin.LoadMCP(context.Background(), p.Manifest.Entry)
			if err != nil {
				eng.plugins.Disable(p.Manifest.Name, fmt.Sprintf("mcp start: %v", err))
				continue
			}
			eng.mcpCleanup = append(eng.mcpCleanup, cleanup)
			for _, t := range remote {
				_ = eng.plugins.RegisterTool(p.Manifest.Name, t)
			}
			eng.plugins.Isolate(p.Manifest.Name)
		}
	}
	// Spec 9.5 overflow: over-cap tool output spills to the workspace, the
	// model gets head + size + path + read-by-range instead of truncation.
	if ws, ok := sbx.(interface{ WorkspaceDir() string }); ok {
		eng.overflow = contextsys.NewOverflow(ws.WorkspaceDir())
	}
	return eng
}

// Plugins exposes the registry for the /api/plugins listing and for plugin
// loading at startup (spec 9.1).
func (e *Engine) Plugins() *plugin.Registry { return e.plugins }

func (e *Engine) Start(ctx context.Context) {
	e.HydrateFromHistory(ctx)
	go e.workerLoop(ctx)
	go e.schedulerLoop(ctx)
	go e.watchdogHeartbeatLoop(ctx)
	go config.WatchLayerKnobs(ctx, e.applyLayerKnobs)
}

// applyLayerKnobs hot-swaps the config.d knobs: executor cap, scheduler
// cadence, senses allowlists — and publishes the in-band reload notice the
// spec requires (adjacent L6->L5 hop, kind=config_reload).
func (e *Engine) applyLayerKnobs(knobs config.LayerKnobs) {
	prev := e.layerKnobs.Load().(config.LayerKnobs)
	e.layerKnobs.Store(knobs)
	if knobs.ParallelReads != prev.ParallelReads {
		e.toolsExec.Store(layers.NewExecutor(knobs.ParallelReads))
	}
	_ = e.bus.Publish(context.Background(), core.BusEnvelope{
		CycleID:   "cyc_config",
		AgentID:   e.agentID,
		Topic:     bus.InputTopic(e.agentID, int(core.LayerCognitiveCtrl)),
		Direction: core.DirectionNorthbound,
		FromLayer: int(core.LayerTaskProsecution),
		ToLayer:   int(core.LayerCognitiveCtrl),
		Kind:      "config_reload",
		Summary:   fmt.Sprintf("config.d hot reload: l1_review=%s parallel_reads=%d l5_dwell=%s", knobs.L1ReviewInterval, knobs.ParallelReads, knobs.L5MinDwell),
		Payload:   "layer knobs reloaded from config.d/*.toml",
	})
	e.telem.EmitLive("config.reload", knobs)
	// Spec 9.5 injection log: control-plane changes reach the model as an
	// in-band notice on the next prompt.
	if e.injLog != nil {
		e.injLog.Record(context.Background(), contextsys.InjectionEvent{
			Source: "config.reload", Kind: "control",
			Notice: fmt.Sprintf("[context: config.d reloaded — l1_review=%s l5=%s/%s parallel_reads=%d]",
				knobs.L1ReviewInterval, knobs.L5Period, knobs.L5MinDwell, knobs.ParallelReads),
		})
	}
}

// watchdogHeartbeatLoop publishes the atomic status snapshot every 15s
// regardless of cycle state — a heartbeat must beat while a long L6
// prosecution is mid-flight, or the watchdog can't tell "busy" from "dead"
// (spec 8.5).
func (e *Engine) watchdogHeartbeatLoop(ctx context.Context) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.writeWatchdogSnapshot()
		}
	}
}

// currentTaskID returns the L4 task the running L6 prosecution serves (for
// L1 Tier-2 redirect routing).
func (e *Engine) currentTaskID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.activeDAGTaskID
}

func (e *Engine) setActiveDAGTask(id string) {
	e.mu.Lock()
	e.activeDAGTaskID = id
	e.mu.Unlock()
}

// SetBudgetQuota configures the 9.4 budget governor: quotaUnits tokens per
// quotaWindowSecs rolling window. Zero disables the slowdown (unbounded).
func (e *Engine) SetBudgetQuota(quotaUnits float64, quotaWindowSecs int) {
	e.mu.Lock()
	e.quotaUnits = quotaUnits
	e.quotaWindowSecs = quotaWindowSecs
	e.mu.Unlock()
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
}

// HydrateFromHistory restores all 6 layer states, engine mode, and global loops
// from SQLite (`layer_states` and `engine_meta`), falling back to `thought_traces` on first migration.
func (e *Engine) HydrateFromHistory(ctx context.Context) {

	// 1. Restore persisted mode, interval, total_cycles, and global_loops
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

	// 2. Restore persisted layer_states table (includes L5 4-Bit GPU Gate state!)
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
		ConstitutionVersion: constitution.Version(),
		MissionVersion:      config.MissionVersion(),
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

// mutationMinMutants reads the hot-reloadable gate.min_mutants knob
// (config.d). Below this mutant count the mutation sample is
// inconclusive and cannot veto a milestone.
func (e *Engine) mutationMinMutants() int {
	if k, ok := e.layerKnobs.Load().(config.LayerKnobs); ok && k.MutationMinMutants > 0 {
		return k.MutationMinMutants
	}
	return 3
}

// NotifyRunawayPause sends the single Telegram report a watchdog-pause
// episode is allowed — the watchdog acts (pause), this reports once.
// Re-arms when the engine leaves ModePaused.
func (e *Engine) NotifyRunawayPause(ctx context.Context) {
	e.mu.RLock()
	paused := e.mode == core.ModePaused
	e.mu.RUnlock()
	if !paused {
		return
	}
	if prev, _ := e.memory.LoadEngineMeta(ctx, "runaway_pause_reported"); prev == "1" {
		return
	}
	_ = e.memory.SaveEngineMeta(ctx, "runaway_pause_reported", "1")
	// The runaway pause is digest news (Human voice): the owner learns
	// about it in the digest or by asking, never as a log dump.
	e.NotifyOwner(ctx, "digest", "worker paused — model calls kept flowing but no mission progress for the configured window; owner can resume when ready")
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
	if mode != core.ModePaused {
		_ = e.memory.SaveEngineMeta(context.Background(), "runaway_pause_reported", "")
	}

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
	ctx = provider.WithLayer(ctx, "chat")
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
		Topic:     bus.InputTopic(e.agentID, 5),
		Direction: core.DirectionNorthbound,
		FromLayer: 6,
		ToLayer:   5,
		Kind:      "sense",
		Summary:   "User chat: " + truncateText(userContent, 90),
		Payload:   userContent,
	})

	// Build Fast Chat prompt with a STATIC SystemPrompt (for >85% KV prefix cache hit across turns & tool steps)
	// and attach dynamic turn state (layers, memories, recent execs) to the final user turn!
	soul := constitution.Text()
	index := e.buildCanonicalIndexWithSkills()
	goals, _ := e.memory.ReadWiki("goals.md")
	st := e.Status()
	wikiPages, _ := e.memory.ListWiki()
	wsFiles, _ := e.sandbox.ListFiles()
	history, _ := e.memory.RecentChatMessages(ctx, 10)

	sysPrompt := FastChatSystemPrompt(soul, index, extractActiveGoals(goals), wikiPages, wsFiles)
	// The owner is a friend, not an operator: the reply writer sees a short
	// plain-language status line instead of the full ops dump, so replies
	// answer what was actually said. Tools still supply details on demand.
	turnCtx := e.plainOwnerStatus(st)

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

		// Execute ALL <action> blocks emitted in this turn in one batch
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
	// One writer, one check: the text the owner sees is the text we store.
	checked, ok := e.CheckOwnerReply(ctx, userContent, cleanChatContent)
	if !ok {
		return core.ChatMessage{ID: msgID, Role: "assistant", Content: ""}, nil
	}
	cleanChatContent = checked

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
	if choiceRes, err := gate.EvaluateLocalGPUChoice(
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
			e.recordCycleReport(runCtx, job.CycleID)

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

// recordCycleReport appends the per-cycle digest row (spec 8.4): what
// completed, what was measured, open owner directives, open uncertainties.
func (e *Engine) recordCycleReport(ctx context.Context, cycleID string) {
	db := e.getDB()
	if db == nil {
		return
	}
	var done, uncertainties int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE state='COMPLETED'`).Scan(&done)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM connectome_nodes WHERE state IN ('OPEN','QUARANTINED') AND kind='CONTRADICTION'`).Scan(&uncertainties)
	var allow, deny int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM verdict_rows WHERE verdict='allow'`).Scan(&allow)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM verdict_rows WHERE verdict IN ('deny','reject')`).Scan(&deny)
	var openDirs int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM directives WHERE status='OPEN'`).Scan(&openDirs)
	// 8.7: standing metrics every cycle.
	science.RecordStandingMetrics(ctx, db, 600)
	_, _ = db.ExecContext(ctx, `INSERT INTO cycle_reports (cycle_id, done, measured, goals, uncertainties, created_at) VALUES (?,?,?,?,?,?)`,
		cycleID,
		fmt.Sprintf("%d dag nodes completed", done),
		fmt.Sprintf("gate verdicts: %d allow / %d deny", allow, deny),
		fmt.Sprintf("%d open owner directives", openDirs),
		fmt.Sprintf("%d open contradictions", uncertainties),
		time.Now().UTC().Format(time.RFC3339))
	e.writeWatchdogSnapshot()
}

// writeWatchdogSnapshot publishes the atomic per-cycle status file the
// watchdog process reads (spec 8.5) — never via the bus.
func (e *Engine) writeWatchdogSnapshot() {
	e.mu.RLock()
	snap := watchdog.Snapshot{
		Heartbeat:       time.Now().UTC().Format(time.RFC3339),
		TotalCycles:     e.totalCycles,
		Mode:            string(e.mode),
		Busy:            e.busy,
		CurrentCycleID:  e.currentCycleID,
		ActiveTaskID:    e.activeDAGTaskID,
		LastToolCallAt:  e.lastToolCallAt,
		LastActionAt:    e.lastActionAt,
		ConstitutionSHA: constitution.Version(),
		MissionVersion:  config.MissionVersion(),
		ConfigChecksums: watchdog.ConfigChecksums(config.Dir()),
	}
	e.mu.RUnlock()
	if db := e.getDB(); db != nil {
		_ = db.QueryRow(`SELECT COUNT(*) FROM verdict_rows WHERE verdict='allow'`).Scan(&snap.VerdictAllow)
		_ = db.QueryRow(`SELECT COUNT(*) FROM verdict_rows WHERE verdict IN ('deny','reject')`).Scan(&snap.VerdictDeny)
	}
	if jl, ok := e.sandbox.(interface{ Status() core.SandboxStatus }); ok {
		_ = watchdog.WriteSnapshot(jl.Status().SandboxDir, snap)
	}
}
