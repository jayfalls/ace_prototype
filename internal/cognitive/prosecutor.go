package cognitive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ace/internal/core"
	"ace/internal/llm"
	"ace/internal/sandbox"
	"github.com/google/uuid"
)

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
		alreadyVerifiedInBatch := hasMutated && strings.Contains(trace.ActionType, "sandbox_sh") &&
			(strings.Contains(trace.ActionTarget, "--selftest") || strings.Contains(trace.ActionTarget, "py_compile"))
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
	if strings.TrimSpace(activeMilestone) == "" || len(traces) == 0 || ctx.Err() != nil {
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
		handoff.CycleCount++
		RecordHandoffAttempt(handoff, cycleID, core.LayerAction{
			Type:   "mode3_gate_check",
			Target: activeMilestone,
		}, 1, "[MODE-3 GATE VETO]: "+gateRes.Reason, false, nil, handoff.FileDigests)
		handoff.ConsecutiveNoOp = prevNoOp
		SaveHandoffPacket(ctx, e.memory, *handoff)
		e.UpdateRoutedSkillOutcome(ctx, activeMilestone, false)

		var lastWorkerThought string
		if len(traces) > 0 {
			lastWorkerThought = traces[len(traces)-1].Thought
		}
		if isStuck, vCount := e.RecordGateVetoAndCheckStuck(ctx, cycleID, activeMilestone, gateRes.Reason, lastWorkerThought); isStuck {
			// Only park [!] after >=5 identical non-code-debugging vetoes confirmed by L1.5 (§6.1)
			if rawGoals, gErr := e.memory.ReadWiki("goals.md"); gErr == nil && rawGoals != "" {
				axisID := extractMilestoneIdentifier(activeMilestone)
				var outLines []string
				parked := false
				for _, line := range strings.Split(rawGoals, "\n") {
					tline := strings.TrimSpace(line)
					if strings.HasPrefix(tline, "- [ ]") && (tline == activeMilestone || (axisID != "" && extractMilestoneIdentifier(tline) == axisID)) {
						outLines = append(outLines, strings.Replace(line, "- [ ]", "- [!]", 1)+fmt.Sprintf(" *(PARKED_HUMAN after %d identical vetoes — escalated via L1.5/Telegram)*", vCount))
						parked = true
					} else {
						outLines = append(outLines, line)
					}
				}
				if parked {
					_ = e.memory.WriteWiki("goals.md", strings.Join(outLines, "\n"))
				}
			}
		}
		return
	}

	if len(handoff.VerifiedFiles) >= 2 {
		e.RecordCoChangedEdges(ctx, handoff.VerifiedFiles)
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
	handoff.CycleCount = 0
	SaveHandoffPacket(ctx, e.memory, *handoff)

	// Automatically distill verified trajectory into /wiki/skills/*.md, update Beta(alpha,beta) posteriors, and index Connectome SKILL node!
	e.DistillAndRegisterWikiSkill(ctx, cycleID, activeMilestone, wsFiles, traces)
	e.MarkDAGTaskVerified(ctx, activeMilestone)
	if sqlDB := e.getDB(); sqlDB != nil {
		_, _ = sqlDB.ExecContext(ctx, `UPDATE connectome_nodes SET state = 'RESOLVED', blocking_weight = 0.0 WHERE kind = 'CONTRADICTION' AND state != 'RESOLVED'`)
	}
	_ = e.memory.SaveEngineMeta(ctx, "overnight_contradiction_digest", "")

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
			// L1..L5 COMPASS & TACTICAL CACHE ORDER (100% Static Trunk First for >75% Per-Layer KV Cache Hits!):
			// 1. Static Environmental & Literature Grounding Digest (100% invariant across milestones!)
			// 2. Static Workspace Data Sources & Benchmark Registry (100% invariant across milestones!)
			// 3. Static LayerRoleSuffix(layer) (100% invariant per dedicated layer session ID!)
			// 4. Dynamic Active Goals (goals.md) + File Manifest placed AFTER the static trunk so DAG updates never bust the trunk cache!
			if litDigest := e.buildEnvironmentalGroundingDigest(""); litDigest != "" {
				userPromptBuilder.WriteString(litDigest)
				userPromptBuilder.WriteString("\n\n")
			}
			if invDigest := e.buildStaticWorkspaceInventoryDigest(); invDigest != "" {
				userPromptBuilder.WriteString(invDigest)
				userPromptBuilder.WriteString("\n\n")
			}
			userPromptBuilder.WriteString(LayerRoleSuffix(layer))
			userPromptBuilder.WriteString("\n\n")
			if strings.TrimSpace(goals) != "" {
				userPromptBuilder.WriteString(fmt.Sprintf("=== ACTIVE GOALS & ORACLE CONFORMANCE GATE (goals.md) ===\n%s\n\n", truncateText(goals, SLABucketGoalsMaxBytes)))
			}
			userPromptBuilder.WriteString(manifestBlock)
			userPromptBuilder.WriteString("\n\n")
		}

		// DYNAMIC SUFFIX SEGMENT: Active Milestone Living Spec (<=2,800B) + Deterministic Connectome Slice (RRF PPR+BM25, <=8KB) + Wiki Notes
		_, livingSpecText := e.EnsureLivingSpecForMilestone(ctx, topMilestone)
		if livingSpecText != "" {
			userPromptBuilder.WriteString("\n=== ACTIVE MILESTONE LIVING SPEC (<=2,800B ENFORCED) ===\n")
			userPromptBuilder.WriteString(livingSpecText)
			userPromptBuilder.WriteString("\n")
		}
		connectomeSlice, preloadedPaths := e.BuildConnectomeSlice(ctx, cycleID, topMilestone, stimulus, goals, livingSpecText)
		if layer == core.LayerTaskProsecution {
			e.mu.Lock()
			e.lastPreloadedConnectome = preloadedPaths
			e.mu.Unlock()
		}
		if connectomeSlice != "" {
			userPromptBuilder.WriteString("\n")
			userPromptBuilder.WriteString(connectomeSlice)
			userPromptBuilder.WriteString("\n")
		}
		if activeNotes := e.buildActiveMilestoneWikiNotesDigest(topMilestone); activeNotes != "" {
			userPromptBuilder.WriteString(activeNotes)
			userPromptBuilder.WriteString("\n")
		}

		// === DYNAMIC CYCLE TAIL: BIDIRECTIONAL BUSES, LIVE TELEMETRY, PLUGGABLE SENSORIUM, LAYER STATES, EPISODIC MEMORIES & HANDOFF ===
		var zoneBuilder strings.Builder
		zoneBuilder.WriteString("=== 6-LAYER COGNITIVE STATE & BIDIRECTIONAL BUSES ===\n")
		zoneBuilder.WriteString(e.buildLiveTelemetrySummaryLine())
		zoneBuilder.WriteString("- " + e.FormatSensoriumPromptLine(ctx) + "\n")
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
	case "gate_dispute":
		rawGoals, _ := e.memory.ReadWiki("goals.md")
		topMilestone := extractTopActionableMilestone(rawGoals)
		vhash, vkind, tpath := ComputeVetoReasonHash(topMilestone, act.Target+" "+act.Content)
		_ = e.EscalateGateDisputeOrStuck(ctx, cycleID, topMilestone, vhash, vkind, tpath, act.Target, act.Content)
		return fmt.Sprintf("Filed gate_dispute (%s on %s): opened CONTRADICTION node and escalated to L1.5/Telegram.", vhash, tpath)

	case "uncertainty_open":
		nodeID := "uncertainty:" + sanitizeTaskID(act.Target)
		_ = e.UpsertUncertaintyNode(ctx, ConnectomeNode{
			ID:             nodeID,
			Kind:           "UNCERTAINTY",
			Path:           act.Target,
			Summary:        act.Content,
			State:          "OPEN",
			OwnerLayer:     int(layer),
			BlockingWeight: 1.5,
			InfoValue:      1.5,
			ExpectedCost:   1.0,
			EvidenceRef:    "cyc_" + cycleID,
			CycleEpoch:     time.Now().Unix(),
		})
		return "Opened Connectome UNCERTAINTY node: " + nodeID

	case "uncertainty_close":
		nodeID := strings.TrimSpace(act.Target)
		if !strings.HasPrefix(nodeID, "uncertainty:") && !strings.HasPrefix(nodeID, "contradiction:") {
			nodeID = "uncertainty:" + sanitizeTaskID(nodeID)
		}
		err := e.UpsertUncertaintyNode(ctx, ConnectomeNode{
			ID:          nodeID,
			Kind:        "UNCERTAINTY",
			Summary:     act.Content,
			State:       "CLOSED",
			OwnerLayer:  int(layer),
			EvidenceRef: "cyc_" + cycleID,
			CycleEpoch:  time.Now().Unix(),
		})
		if err != nil {
			return "uncertainty_close error: " + err.Error()
		}
		return "Closed Connectome UNCERTAINTY node: " + nodeID + " (evidence_ref=cyc_" + cycleID + ")"

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
		// Support optional #grep=<pattern> or line-range syntax on workspace_read (e.g. "pkg/module.py:300-440", "path.py#1-180", or "file.md#grep=^#{1,3}")
		readTarget := strings.TrimSpace(act.Target)
		var grepPattern string
		if grepIdx := strings.Index(readTarget, "#grep="); grepIdx > 0 {
			grepPattern = strings.TrimSpace(readTarget[grepIdx+len("#grep="):])
			readTarget = strings.TrimSpace(readTarget[:grepIdx])
		}
		startLine, endLine := 0, 0
		delimIdx := strings.IndexAny(readTarget, ":#")
		if delimIdx > 0 {
			filePart := strings.TrimSpace(readTarget[:delimIdx])
			rangePart := strings.TrimSpace(readTarget[delimIdx+1:])
			sep := "-"
			if strings.Contains(rangePart, ":") {
				sep = ":"
			}
			parts := strings.SplitN(rangePart, sep, 2)
			if s, sErr := strconv.Atoi(strings.TrimSpace(parts[0])); sErr == nil {
				readTarget = filePart
				if s > 0 {
					startLine = s
					if len(parts) == 2 {
						if eLine, eErr := strconv.Atoi(strings.TrimSpace(parts[1])); eErr == nil && eLine >= s {
							endLine = eLine
						}
					}
				}
			} else {
				// Strip unknown #fragment so openat never fails on file.md#section
				readTarget = filePart
			}
		}
		data, err := e.sandbox.ReadFile(readTarget)
		if err != nil && !strings.Contains(readTarget, "/") && strings.HasSuffix(readTarget, ".py") {
			if altData, altErr := e.sandbox.ReadFile("ace_v07/" + readTarget); altErr == nil {
				readTarget = "ace_v07/" + readTarget
				data = altData
				err = nil
			}
		}
		if err != nil && strings.Contains(readTarget, "SPEC_ACE_V0_8_SELF_BUILD.md") {
			if altData, altErr := e.sandbox.ReadFile("SPEC_ACE_V0_8_SELF_BUILDING.md"); altErr == nil {
				readTarget = "SPEC_ACE_V0_8_SELF_BUILDING.md"
				data = altData
				err = nil
			}
		}
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
		var preloaded map[string]bool
		if layer == core.LayerTaskProsecution {
			e.mu.RLock()
			preloaded = e.lastPreloadedConnectome
			e.mu.RUnlock()
		}
		e.RecordConnectomeInspection(ctx, cycleID, cleanRel, "INSPECTED", "cyc_"+cycleID, preloaded)
		if grepPattern != "" {
			re, rErr := regexp.Compile(grepPattern)
			lines := strings.Split(data, "\n")
			var matched []string
			for idx, ln := range lines {
				if (rErr == nil && re.MatchString(ln)) || (rErr != nil && strings.Contains(ln, grepPattern)) {
					matched = append(matched, fmt.Sprintf("L%d: %s", idx+1, ln))
				}
			}
			if len(matched) > 0 {
				return fmt.Sprintf("=== /workspace/%s (#grep=%s, %d matching lines) ===\n%s\n\n=== File Head Preview ===\n%s",
					cleanRel, grepPattern, len(matched), truncateText(strings.Join(matched, "\n"), 16000), truncateText(data, 16000))
			}
		}
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
		editTgt := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(act.Target), "/workspace/"), "workspace/")
		if !strings.Contains(editTgt, "/") && strings.HasSuffix(editTgt, ".py") && editTgt != "ace_rlm.py" && editTgt != "script.py" {
			editTgt = "ace_v07/" + editTgt
		}
		act.Target = editTgt
		if err := e.sandbox.EditFile(act.Target, act.OldText, act.Content); err != nil {
			return "workspace_edit error: " + err.Error()
		}
		_, _ = e.AcquireFileLease(ctx, cycleID, act.Target, 5*time.Minute)
		e.RecordConnectomeInspection(ctx, cycleID, act.Target, "IMPLEMENTED", "cyc_"+cycleID, nil)
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
		if layer == core.LayerAspirational || layer == core.LayerCognitiveCtrl {
			return "web_search restricted to L2, L3, L4, and L6"
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
		if layer == core.LayerAspirational || layer == core.LayerCognitiveCtrl {
			return "web_fetch restricted to L2, L3, L4, and L6"
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
		// Hard L1 Constitutional & Mission Immutability Guard (Dave Shapiro ACE Spec):
		// Neither L1 nor any downstream layer (L2..L6) may EVER mutate soul.md or mission.md.
		// The L1 Constitution (Heuristic Imperatives, UDHR) and # CURRENT MISSION are unshifting once set by the human.
		if target == "soul.md" || target == "mission.md" || strings.HasSuffix(target, "/soul.md") || strings.HasSuffix(target, "/mission.md") {
			return "CONSTITUTIONAL VETO: /wiki/soul.md (L1 Heuristic Imperatives, UDHR & # CURRENT MISSION) is unshifting and read-only to all agent layers. Only the human operator may steer the L1 Mission."
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
			newGoals := collapseMultilineMilestoneBullets(strings.TrimSpace(act.Content))
			if len(existingGoals) > 200 {
				existingGoals = collapseMultilineMilestoneBullets(existingGoals)
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
		writeTgt := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(act.Target), "/workspace/"), "workspace/")
		if !strings.Contains(writeTgt, "/") && strings.HasSuffix(writeTgt, ".py") && writeTgt != "ace_rlm.py" && writeTgt != "script.py" {
			writeTgt = "ace_v07/" + writeTgt
		}
		act.Target = writeTgt
		if err := e.sandbox.WriteFile(act.Target, act.Content); err != nil {
			return "workspace_write error: " + err.Error()
		}
		_, _ = e.AcquireFileLease(ctx, cycleID, act.Target, 5*time.Minute)
		_ = e.StageWorkpadFile(cycleID, act.Target, []byte(act.Content))
		e.RecordConnectomeInspection(ctx, cycleID, act.Target, "IMPLEMENTED", "cyc_"+cycleID, nil)
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
		if strings.Contains(cmdStr, "python") && strings.Contains(cmdStr, "-c") && strings.Contains(cmdStr, `\n`) {
			cmdStr = strings.ReplaceAll(cmdStr, `;\n`, "; ")
			cmdStr = strings.ReplaceAll(cmdStr, `\n`, "\n")
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
