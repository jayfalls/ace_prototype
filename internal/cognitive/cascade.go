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

	"ace/internal/core"
	"ace/internal/llm"
)

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

				// 1. If NO actionable "- [ ]" milestone remains in goals.md (Horizon Expansion):
				//    - On initial mission bootstrap (autoPulseCount <= 1), run L1 -> L2 -> L3 -> L4 (4 hops).
				//    - On subsequent Horizon Expansions, L1 has already locked the mission constitution; dispatch L2 -> L3 -> L4 (3 hops) so L1 stays the rarest supervisory crown layer!
				if topMilestone == "" {
					if autoPulseCount <= 1 {
						e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: Initial Mission Bootstrap (L1 -> L2||L3 -> L4)", autoPulseCount))
						e.EnqueueCycle(
							"Initial Mission Bootstrap & Horizon Expansion (L1 -> L2||L3 -> L4): L1 sets epistemic/mission priorities, L2 researches the domain and uses action.type=\"wiki_write\" (target=\"goals.md\") to formulate 3 concrete, falsifiable '- [ ]' milestones (naming target dataset paths in `/workspace/benchmarks/datasets/<name>.json` and modular code paths in `/workspace/<package>/<module>.py`), and L3 specifies capability & dataset schemas.",
							core.LayerAspirational,
							4,
						)
					} else {
						e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: Strategic Horizon Expansion (L2||L3 -> L4)", autoPulseCount))
						e.EnqueueCycle(
							"Strategic Horizon Expansion (L2||L3 -> L4): All prior '- [ ]' milestones are complete. L2 researches the next scientific/engineering axis and uses action.type=\"wiki_write\" (target=\"goals.md\") to append 3 concrete, falsifiable '- [ ]' milestones, while L3 audits self-model & dataset schemas.",
							core.LayerGlobalStrategy,
							3,
						)
					}
					continue
				}

				// 2. Strict Top-Down Pyramidal 6-Layer Hierarchy (1 <= L1 < L2 < L3 < L4 << L6):
				// NO layer L_k may EVER execute before L1..L_{k-1} have established their top-down Southbound directives!
				// - Genesis / Uninitialized Upper Layers (L1 or L2 has 0 traces, or soul.md changed):
				//   Full Top-Down Cascade: L1 (Aspirational) -> L2 (Global Strategy) -> L3 (Agent Model) -> L4 (Executive) -> L5 (Cognitive Control) -> L6 (Task Prosecution) (6 hops).
				// - Strategic Re-Grounding (every 2nd completed milestone or CycleCount == 6):
				//   L2 -> L3 -> L4 -> L5 -> L6 (5 hops).
				// - Milestone Grounding (Cycle #1 of a new milestone, handoff.CycleCount == 0):
				//   L3 -> L4 -> L5 -> L6 (4 hops, under active L1 & L2 directives).
				// - Tactical Spec Revision (handoff.CycleCount > 0 && handoff.CycleCount % 3 == 0):
				//   L4 -> L5 -> L6 (3 hops).
				// - Workhorse Reflex Prosecution (handoff.CycleCount > 0 && handoff.CycleCount % 3 != 0):
				//   L5 -> L6 (2 hops, under active L1->L2->L3->L4 directives).
				handoff := LoadHandoffPacket(ctx, e.memory, topMilestone)
				startL, hops, reason := e.determineTopDownStartLayer(ctx, topMilestone, handoff, autoPulseCount)
				e.recordGlobalLoop("learning_loop", "active", fmt.Sprintf("Pulse #%d: %s", autoPulseCount, reason))
				e.EnqueueCycle(
					fmt.Sprintf("%s: %s", reason, topMilestone),
					startL,
					hops,
				)
			}
		}
	}
}

// determineTopDownStartLayer enforces Dave Shapiro's strict top-down causality law:
// No layer L_k (k > 1) can EVER run if any higher layer L_1..L_{k-1} has not yet executed and emitted its Southbound directive,
// while naturally producing the pyramidal call distribution (1 <= L1 < L2 < L3 < L4 << L6) via temporal persistence.
func (e *Engine) determineTopDownStartLayer(ctx context.Context, topMilestone string, handoff CrossCycleHandoffPacket, pulse int) (core.LayerID, int, string) {
	soul, _ := e.memory.ReadWiki("soul.md")
	soulHash := sha256Str(strings.TrimSpace(soul))[:16]
	lastSoulHash, _ := e.memory.LoadEngineMeta(ctx, "l1_grounded_soul_hash")

	l1Count, l2Count, l3Count, l4Count := e.layerTraceCounts(ctx)
	checkedCount := e.countCheckedMilestones()

	// 1. Hard Top-Down Genesis Law: L1 MUST run first on boot, whenever soul.md changes, or every 4 completed milestones
	if l1Count == 0 || lastSoulHash != soulHash || (checkedCount > 0 && checkedCount%4 == 0 && handoff.CycleCount == 0 && l1Count*4 <= l2Count) {
		return core.LayerAspirational, 6, fmt.Sprintf("Full Top-Down Genesis Cascade #%d (L1 -> L2 -> L3 -> L4 -> L5 -> L6)", pulse)
	}
	// 2. L2 MUST run after L1 if L2 has never run, or on every 2nd new milestone, or on major stuck pivot (CycleCount == 6)
	if l2Count == 0 || handoff.CycleCount == 6 || (handoff.CycleCount == 0 && l2Count*2 <= l3Count+1) {
		return core.LayerGlobalStrategy, 5, fmt.Sprintf("Top-Down Strategic Cascade #%d (L2 -> L3 -> L4 -> L5 -> L6)", pulse)
	}
	// 3. L3 MUST run on every new milestone (CycleCount == 0) or if L3 has never run
	if l3Count == 0 || handoff.CycleCount == 0 {
		return core.LayerAgentModel, 4, fmt.Sprintf("Top-Down Milestone Grounding Cascade #%d (L3 -> L4 -> L5 -> L6)", pulse)
	}
	// 4. L4 MUST run if L4 has never run or on every 3rd failed attempt on the active milestone
	if l4Count == 0 || handoff.CycleCount%3 == 0 {
		return core.LayerExecutive, 3, fmt.Sprintf("Top-Down Tactical Re-Plan Cascade #%d (L4 -> L5 -> L6, attempt #%d)", pulse, handoff.CycleCount+1)
	}
	// 5. Otherwise L1..L4 are all actively locked above; run L5 -> L6 workhorse prosecution
	return core.LayerCognitiveCtrl, 2, fmt.Sprintf("Top-Down Workhorse Prosecution #%d (L5 -> L6, attempt #%d)", pulse, handoff.CycleCount+1)
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

func (e *Engine) countCheckedMilestones() int {
	goals, _ := e.memory.ReadWiki("goals.md")
	cnt := 0
	for _, line := range strings.Split(collapseMultilineMilestoneBullets(goals), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- [x]") {
			cnt++
		}
	}
	return cnt
}

type l5GateEvalResult struct {
	PYes      float64
	Model     string
	LatencyMs int64
}

func (e *Engine) evaluateL5Gate(ctx context.Context, sharedState, gateQ string) (l5GateEvalResult, error) {
	gpuRes, err := llm.EvaluateLocalGPUGateWithState(ctx, sharedState, gateQ, 0.45)
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

	soul, _ := e.memory.ReadWiki("soul.md")
	soulHash := sha256Str(strings.TrimSpace(soul))[:16]
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
