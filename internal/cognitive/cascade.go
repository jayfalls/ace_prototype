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
