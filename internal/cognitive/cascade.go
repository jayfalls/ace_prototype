package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"database/sql"

	"ace/internal/bus"
	"ace/internal/config"
	"ace/internal/core"
	"ace/internal/gate"
	"ace/internal/layers"
	"ace/internal/tasks"
)

// schedulerLoop is the spec-9.3 mailbox/cadence dispatcher. It replaces the
// old 2-second autonomousTickerLoop: layer mailboxes fill from bus traffic,
// L1 fires on events plus a 15-minute review, L5 runs on its own clock gated
// by min_dwell, and L6 prosecution runs continuously while work exists. The
// loop blocks on scheduler wake signals and timers — zero polling.
func (e *Engine) schedulerLoop(ctx context.Context) {
	knobs := e.layerKnobs.Load().(config.LayerKnobs)
	sched := layers.NewScheduler(layers.SchedulerConfig{
		L1ReviewInterval: knobs.L1ReviewInterval,
		L5Period:         knobs.L5Period,
		L5MinDwell:       knobs.L5MinDwell,
	})
	sched.DataBelow = e.schedDataBelow
	var l5Dispatches []time.Time // thrash-per-hour window
	lastScale := 1.0             // budget-governor cadence factor
	seededSched := false         // schedule commitments are upserted once
	unsubscribe := e.bus.Subscribe(func(env core.BusEnvelope) { sched.Deliver(env) })
	defer unsubscribe()

	// Crash recovery (spec 9.4): tasks left QUEUED/RUNNING/BLOCKED by a kill
	// are re-armed into L5's mailbox so handles/jobs get re-polled.
	if e.taskMgr != nil {
		if recovered, rerr := e.taskMgr.Resume(ctx); rerr == nil {
			for _, t := range recovered {
				sched.Deliver(core.BusEnvelope{
					CycleID: "system", AgentID: e.agentID,
					Topic:     bus.InputTopic(e.agentID, 5),
					Direction: core.DirectionNorthbound,
					FromLayer: 6, ToLayer: 5,
					Kind:    "task_recovery",
					Summary: fmt.Sprintf("recovered live task %s (%s, %s)", t.ID, t.Kind, t.State),
					Payload: t.ID,
				})
			}
		}
	}

	var autoPulseCount int
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(sched.NextWake(time.Now()))
		select {
		case <-ctx.Done():
			return
		case <-sched.Wake():
		case <-timer.C:
		}
		now := time.Now()

		e.mu.RLock()
		mode := e.mode
		busy := e.busy
		qLen := len(e.jobQueue)
		e.mu.RUnlock()
		if busy || qLen > 0 || mode != core.ModeAutonomous {
			continue
		}

		// Hot-reload: pick up config.d knob changes each pass.
		if k := e.layerKnobs.Load().(config.LayerKnobs); k.L1ReviewInterval != knobs.L1ReviewInterval ||
			k.L5Period != knobs.L5Period || k.L5MinDwell != knobs.L5MinDwell {
			knobs = k
			sched.SetConfig(layers.SchedulerConfig{
				L1ReviewInterval: k.L1ReviewInterval,
				L5Period:         k.L5Period,
				L5MinDwell:       k.L5MinDwell,
			})
		}

		// Budget governor (spec 9.4): when projected burn exceeds remaining
		// window quota, non-L6 cadences slow; L6 prosecution stays runnable.
		e.mu.RLock()
		qUnits, qSecs := e.quotaUnits, e.quotaWindowSecs
		e.mu.RUnlock()
		scale := 1.0
		if s, ok := e.memory.(interface{ DB() *sql.DB }); ok && s.DB() != nil {
			scale = tasks.CadenceScale(ctx, s.DB(), qSecs, qUnits)
		}
		if scale != lastScale {
			lastScale = scale
			if scale < 1.0 {
				sched.SetConfig(layers.SchedulerConfig{
					L1ReviewInterval: time.Duration(float64(knobs.L1ReviewInterval) / scale),
					L5Period:         time.Duration(float64(knobs.L5Period) / scale),
					L5MinDwell:       knobs.L5MinDwell,
				})
			} else {
				sched.SetConfig(layers.SchedulerConfig{
					L1ReviewInterval: knobs.L1ReviewInterval,
					L5Period:         knobs.L5Period,
					L5MinDwell:       knobs.L5MinDwell,
				})
			}
		}

		// L5 time loop (spec 9.4): tick monitors, fire due schedules. A due
		// schedule submits a deduped run row (idempotency name@due) and
		// nudges L5; next_due advances only after that run reaches DONE.
		if e.taskMgr != nil {
			if !seededSched {
				seededSched = true
				_, _ = e.taskMgr.UpsertSchedule(ctx, "self_metrics_rollup", "FREQ=HOURLY")
				// Weekly schedule (spec acceptance) — same durable fire path.
				_, _ = e.taskMgr.UpsertSchedule(ctx, "weekly_consolidation", "FREQ=WEEKLY")
				// A durable monitor (spec 9.4): fires when episodic memory
				// pressure exceeds 80% of cap; re-arms at boot via its
				// idempotency key so it survives restarts.
				_, _ = e.taskMgr.RegisterMonitor(ctx, "memory_episodic_pressure",
					func(ctx context.Context) (bool, string) {
						var n int
						if db := e.getDB(); db != nil {
							_ = db.QueryRowContext(ctx,
								`SELECT COUNT(*) FROM memories WHERE COALESCE(type,'episodic')='episodic'`).Scan(&n)
						}
						return n > 8000, fmt.Sprintf("episodic memory %d/10000", n)
					},
					func(detail string) {
						e.recordGlobalLoop("monitor_memory_episodic_pressure", "fired", detail)
					})
			}
			e.taskMgr.HeartbeatTick(ctx, func(s tasks.Schedule) {
				t, terr := e.taskMgr.Submit(ctx, tasks.Task{
					Kind: "schedule_run", Layer: 5, State: tasks.StateRunning,
					Payload:        s.Name,
					IdempotencyKey: fmt.Sprintf("%s@%d", s.Name, s.NextDue.Unix()),
				})
				if terr != nil {
					return // (name, due) already recorded — dedupe
				}
				e.taskMgr.RecordScheduleRun(ctx, s.ID, t.ID)
				sched.Deliver(core.BusEnvelope{
					CycleID: "system", AgentID: e.agentID,
					Topic:     bus.InputTopic(e.agentID, 5),
					Direction: core.DirectionNorthbound,
					FromLayer: 6, ToLayer: 5,
					Kind:    "schedule",
					Summary: "schedule " + s.Name + " due",
					Payload: s.Name,
				})
				_ = e.taskMgr.Complete(ctx, t.ID)
			}, nil)
			_ = e.taskMgr.ReconcileSchedules(ctx)
		}

		n := sched.ReadyNow(now)
		if n == 0 {
			continue
		}
		envs := sched.Dispatch(n, now)
		autoPulseCount++
		if n == 5 {
			l5Dispatches = append(l5Dispatches, now)
			cutoff := now.Add(-time.Hour)
			kept := l5Dispatches[:0]
			for _, t := range l5Dispatches {
				if t.After(cutoff) {
					kept = append(kept, t)
				}
			}
			l5Dispatches = kept
			_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
				TraceID:   "cyc_sched",
				AgentID:   e.agentID,
				LayerCode: "L5",
				Operation: "l5.thrash_per_hour",
				Status:    "ok",
				Detail:    fmt.Sprintf("l5 dispatches in trailing hour: %d", len(l5Dispatches)),
			})
		}
		e.enqueueScheduled(n, envs, autoPulseCount)
	}
}

// schedDataBelow is the non-bus "data from below" signal for L2–L4: a live
// (non-terminal) mission-DAG node whose work feeds the cluster, or prior
// traces proving the layer has produced output below before.
func (e *Engine) schedDataBelow(layer int) bool {
	db := e.getDB()
	if db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var n int
	if db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mission_dag_nodes WHERE state NOT IN ('COMPLETED','STALE','RETIRED','DUPLICATE_OF')`).Scan(&n) == nil && n > 0 {
		return true
	}
	code := fmt.Sprintf("L%d", layer)
	return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM thought_traces WHERE layer_code=?`, code).Scan(&n) == nil && n > 0
}

// enqueueScheduled translates a ready layer into a cascade job. The mailbox
// supplies the stimulus; absent mail falls back to the layer's standing
// trigger so cadence (milestone workhorse, horizon expansion) is preserved.
func (e *Engine) enqueueScheduled(layer int, envs []core.BusEnvelope, pulse int) {
	stimulus := ""
	for i := len(envs) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(envs[i].Summary); s != "" {
			stimulus = s
			break
		}
	}
	goals, _ := e.memory.ReadWiki("goals.md")
	topMilestone := extractTopActionableMilestone(goals)

	var trigger string
	var start core.LayerID
	var hops int
	switch {
	case layer == 1:
		if stimulus == "" {
			stimulus = "scheduled supervisory review"
		}
		trigger = fmt.Sprintf("L1 Aspirational review (event/interval): %s", stimulus)
		start, hops = core.LayerAspirational, 6
	case layer >= 2 && layer <= 4:
		if stimulus == "" {
			stimulus = "cluster data ready"
		}
		trigger = fmt.Sprintf("L%d cluster run (mailbox): %s", layer, stimulus)
		start, hops = core.LayerID(layer), 7-layer
	default:
		if topMilestone == "" {
			// Horizon expansion: bootstrap via L1 the first time, L2 after.
			if pulse <= 1 {
				trigger = "Initial Mission Bootstrap & Horizon Expansion (L1 -> L2||L3 -> L4): L1 sets epistemic/mission priorities, L2 researches the domain and uses action.type=\"wiki_write\" (target=\"goals.md\") to formulate 3 concrete, falsifiable '- [ ]' milestones (naming target dataset paths in `/workspace/datasets/<name>.json` and modular code paths in `/workspace/modules/<module>.py`), and L3 specifies capability & dataset schemas."
				start, hops = core.LayerAspirational, 4
			} else {
				trigger = "Strategic Horizon Expansion (L2||L3 -> L4): All prior '- [ ]' milestones are complete. L2 researches the next mission workstream and uses action.type=\"wiki_write\" (target=\"goals.md\") to append 3 concrete, falsifiable '- [ ]' milestones, while L3 audits self-model & dataset schemas."
				start, hops = core.LayerGlobalStrategy, 3
			}
		} else {
			trigger = fmt.Sprintf("Top-Down Workhorse Prosecution #%d (L5 -> L6): %s", pulse, topMilestone)
			start, hops = core.LayerCognitiveCtrl, 2
		}
	}
	e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: %s", pulse, trigger))
	e.EnqueueCycle(trigger, start, hops)
}

func (e *Engine) layerTraceCounts(ctx context.Context) (l1, l2, l3, l4 int) {
	db := e.getDB()
	if db == nil {
		return 0, 0, 0, 0
	}
	rows, err := db.QueryContext(ctx, `SELECT layer_code, COUNT(*) FROM thought_traces WHERE layer_code IN ('L1','L2','L3','L4') GROUP BY layer_code`)
	if err != nil {
		return 0, 0, 0, 0
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var cnt int
		if rows.Scan(&code, &cnt) == nil {
			switch code {
			case "L1":
				l1 = cnt
			case "L2":
				l2 = cnt
			case "L3":
				l3 = cnt
			case "L4":
				l4 = cnt
			}
		}
	}
	return l1, l2, l3, l4
}

type l5GateEvalResult struct {
	PYes      float64
	Model     string
	LatencyMs int64
}

func (e *Engine) evaluateL5Gate(ctx context.Context, sharedState, gateQ string) (l5GateEvalResult, error) {
	gpuRes, err := gate.EvaluateLocalGPUGateWithState(ctx, sharedState, gateQ, 0.45)
	if err != nil {
		return l5GateEvalResult{}, err
	}
	return l5GateEvalResult{
		PYes:      gpuRes.PYes,
		Model:     gpuRes.Model,
		LatencyMs: int64(gpuRes.LatencyMs),
	}, nil
}

func (e *Engine) runCascade(ctx context.Context, job cycleJob) {
	e.maybeRunRetentionSweep(ctx)
	cycleStart := time.Now()
	cycleSpanID := "spn_cyc_" + job.CycleID
	stimulus := job.Trigger

	// ACE v0.7.1 Pipelined Top-Down Mission Dependency Tree (`L1 -> [L2 || L3 || L4] -> L5 -> L6`):
	// 1. Enforces hard L1 Aspirational genesis before any L2..L6 node can exist.
	// 2. Runs ready nodes across {L2, L3, L4} in conditional parallel waves over the Dependency Tree.
	// 3. Verifies cryptographic L1->L2->L3->L4 provenance before L5 (GPU Gate) dispatches L6 (Prosecutor Loop).
	l1b, l2b, l3b, l4b := e.layerTraceCounts(ctx)
	l6Traces := e.StepMissionDAG(ctx, job.CycleID, cycleSpanID, stimulus)
	l1a, l2a, l3a, l4a := e.layerTraceCounts(ctx)
	upperRan := (l1a > l1b) || (l2a > l2b) || (l3a > l3b) || (l4a > l4b)

	// Guard against empty idle ticks: if neither upper-layer DAG waves nor L6 executed in this tick,
	// do NOT copy the entire SQLite DB (which caused 19s db_write_wait_ms lock contention) or record empty cycle.cascade spans!
	if len(l6Traces) == 0 && !upperRan {
		return
	}

	// Post-Cascade Global Loops: Learning Loop (Northbound L6->L1 reflection) + Memory Manager consolidation + Live Trace DB Mirror
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

func (e *Engine) runLegacySequentialCascade(ctx context.Context, job cycleJob) {
	cycleStart := time.Now()
	cycleSpanID := "spn_cyc_" + job.CycleID
	stimulus := job.Trigger
	currLayer := job.StartLayer
	maxHops := job.MaxHops

	soulHash := e.currentSoulHash()
	lastSoulHash, _ := e.memory.LoadEngineMeta(ctx, "l1_grounded_soul_hash")
	l1Count, l2Count, l3Count, l4Count := e.layerTraceCounts(ctx)

	if currLayer > core.LayerAspirational && (l1Count == 0 || lastSoulHash != soulHash) {
		currLayer = core.LayerAspirational
		maxHops = 6
	} else if currLayer > core.LayerGlobalStrategy && l2Count == 0 {
		currLayer = core.LayerGlobalStrategy
		if maxHops < 5 {
			maxHops = 5
		}
	} else if currLayer > core.LayerAgentModel && l3Count == 0 {
		currLayer = core.LayerAgentModel
		if maxHops < 4 {
			maxHops = 4
		}
	} else if currLayer > core.LayerExecutive && l4Count == 0 {
		currLayer = core.LayerExecutive
		if maxHops < 3 {
			maxHops = 3
		}
	}

	var l6Traces []core.ThoughtTrace

	for hop := 0; hop < maxHops; hop++ {
		if ctx.Err() != nil {
			return
		}
		if currLayer == core.LayerTaskProsecution {
			l6Traces = e.runPiProsecutorLoop(ctx, job.CycleID, cycleSpanID, stimulus)
			break
		}

		// STRICT SEQUENTIAL TOP-DOWN PROPAGATION (L1 -> L2 -> L3 -> L4 -> L5 -> L6):
		// Each layer receives the Southbound directive emitted by the layer immediately above it,
		// evaluates its constitutional/strategic/self-model/executive role, and emits its Southbound directive to the next layer down.
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
			if currLayer == core.LayerAspirational {
				_ = e.memory.SaveEngineMeta(ctx, "l1_grounded_soul_hash", soulHash)
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
		if (currLayer == core.LayerExecutive || currLayer == core.LayerCognitiveCtrl) && nextDirective != "" && hop+1 < maxHops {
			// fail closed: a gate error leaves pYes at 0 and vetoes the dispatch
			pYes := 0.0
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
			if gpuRes, gErr := gate.EvaluateLocalGPUGateWithState(ctx, sharedState, gateQ, 0.45); gErr == nil {
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
				Topic:     bus.InputTopic(e.agentID, int(core.LayerTaskProsecution)),
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
				Topic:     bus.InputTopic(e.agentID, int(core.LayerExecutive)),
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
		t1, t2, t3       core.ThoughtTrace
		err1, err2, err3 error
		wg               sync.WaitGroup
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

// extractTopActionableMilestone returns the first unchecked "- [ ]" line in goals.md.
// v0.8: there are no human-approval milestones — access needs route through the L1 ruling.
func extractTopActionableMilestone(goals string) string {
	for _, line := range strings.Split(goals, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			return truncateText(trimmed, 1200)
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
