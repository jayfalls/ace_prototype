package cognitive

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ace/internal/core"
	"ace/internal/plugin"
	"ace/internal/sandbox"
	"ace/internal/tasks"
	"ace/internal/tools"

	"github.com/google/uuid"
)

// actionCtx unwraps the per-call metadata the tool dispatch injects.
func actionCtx(ctx context.Context) (m tools.Meta, layer core.LayerID) {
	m = tools.MetaFrom(ctx)
	return m, core.LayerID(m.Layer)
}

// registerBuiltinActions declares the engine-internal actions as tools on
// the "ace-core" builtin plugin (spec 9.1: the prosecutor switch becomes
// registry dispatch). Function bodies are the moved case bodies, verbatim.
func (e *Engine) registerBuiltinActions(reg *plugin.Registry, pluginName string) {
	add := func(name string, ro bool, layers []int, fn func(context.Context, tools.Args) string) {
		_ = reg.RegisterTool(pluginName, &tools.FnTool{NameV: name, ReadOnlyV: ro, LayersV: layers, Fn: func(ctx context.Context, a tools.Args) (string, error) {
			return fn(ctx, a), nil
		}})
	}
	add("gate_dispute", false, nil, e.actionGateDispute)
	add("uncertainty_open", false, nil, e.actionUncertaintyOpen)
	add("uncertainty_close", false, nil, e.actionUncertaintyClose)
	add("memory_remember", false, []int{1, 2, 3, 4, 6}, e.actionMemoryRemember)
	add("memory_search", true, []int{1, 2, 3, 4, 5, 6}, e.actionMemorySearch)
	add("workspace_read", true, []int{2, 3, 4, 6}, e.actionWorkspaceRead)
	add("workspace_edit", false, []int{6}, e.actionWorkspaceEdit)
	add("ticket", true, []int{4, 6}, e.actionTicket)
	add("kb", true, []int{3, 6}, e.actionKB)
	add("parallel_research", true, []int{6}, e.actionParallelResearch)
	add("wiki_read", true, []int{1, 2, 3, 4, 6}, e.actionWikiRead)
	add("wiki_write", false, []int{2, 3, 4, 6}, e.actionWikiWrite)
	add("workspace_write", false, []int{6}, e.actionWorkspaceWrite)
	add("sandbox_sh", false, []int{6}, e.actionSandboxSh)
	add("chat_proactive", false, []int{6}, e.actionChatProactive)
}

func (e *Engine) actionGateDispute(ctx context.Context, a tools.Args) string {
	m, _ := actionCtx(ctx)
	rawGoals, _ := e.memory.ReadWiki("goals.md")
	topMilestone := extractTopActionableMilestone(rawGoals)
	vhash, vkind, tpath := ComputeVetoReasonHash(topMilestone, a.Target+" "+a.Content)
	_ = e.EscalateGateDisputeOrStuck(ctx, m.CycleID, topMilestone, vhash, vkind, tpath, a.Target, a.Content)
	return fmt.Sprintf("Filed gate_dispute (%s on %s): opened CONTRADICTION node and escalated to L1.5/Telegram.", vhash, tpath)
}

func (e *Engine) actionUncertaintyOpen(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	nodeID := "uncertainty:" + sanitizeTaskID(a.Target)
	_ = e.UpsertUncertaintyNode(ctx, ConnectomeNode{
		ID:             nodeID,
		Kind:           "UNCERTAINTY",
		Path:           a.Target,
		Summary:        a.Content,
		State:          "OPEN",
		OwnerLayer:     int(layer),
		BlockingWeight: 1.5,
		InfoValue:      1.5,
		ExpectedCost:   1.0,
		EvidenceRef:    "cyc_" + m.CycleID,
		CycleEpoch:     time.Now().Unix(),
	})
	return "Opened Connectome UNCERTAINTY node: " + nodeID
}

func (e *Engine) actionUncertaintyClose(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	nodeID := strings.TrimSpace(a.Target)
	if !strings.HasPrefix(nodeID, "uncertainty:") && !strings.HasPrefix(nodeID, "contradiction:") {
		nodeID = "uncertainty:" + sanitizeTaskID(nodeID)
	}
	err := e.UpsertUncertaintyNode(ctx, ConnectomeNode{
		ID:          nodeID,
		Kind:        "UNCERTAINTY",
		Summary:     a.Content,
		State:       "CLOSED",
		OwnerLayer:  int(layer),
		EvidenceRef: "cyc_" + m.CycleID,
		CycleEpoch:  time.Now().Unix(),
	})
	if err != nil {
		return "uncertainty_close error: " + err.Error()
	}
	return "Closed Connectome UNCERTAINTY node: " + nodeID + " (evidence_ref=cyc_" + m.CycleID + ")"
}

func (e *Engine) actionMemoryRemember(ctx context.Context, a tools.Args) string {
	_, layer := actionCtx(ctx)
	title := strings.TrimSpace(a.Target)
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
		Content:   a.Content,
		Tags:      strings.ToLower(layer.Code()) + ",cognitive",
	})
	if err != nil {
		return "memory_remember error: " + err.Error()
	}
	return "Saved FTS5 memory: " + entry.ID + " (" + entry.Title + ")"
}

func (e *Engine) actionMemorySearch(ctx context.Context, a tools.Args) string {
	_, layer := actionCtx(ctx)
	query := strings.TrimSpace(a.Target)
	if query == "" {
		query = strings.TrimSpace(a.Content)
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
}

func (e *Engine) actionWorkspaceRead(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	cleanTgt := strings.TrimSpace(a.Target)
	lowerTgt := strings.ToLower(cleanTgt)
	if strings.HasSuffix(lowerTgt, ".db") || strings.HasSuffix(lowerTgt, ".sqlite") || strings.HasSuffix(lowerTgt, ".db-shm") || strings.HasSuffix(lowerTgt, ".db-wal") {
		return "[Binary SQLite database — do NOT call workspace_read on .db files! Exact table & column schemas are pre-injected in your prompt; use sqlite3 in L6 Python scripts to query rows.]"
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
	readTarget := strings.TrimSpace(a.Target)
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
		if altData, altErr := e.sandbox.ReadFile("modules/" + readTarget); altErr == nil {
			readTarget = "modules/" + readTarget
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
	e.RecordConnectomeInspection(ctx, m.CycleID, cleanRel, "INSPECTED", "cyc_"+m.CycleID, preloaded)
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
}

func (e *Engine) actionWorkspaceEdit(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	if layer != core.LayerTaskProsecution {
		return "workspace_edit restricted to L6"
	}
	editTgt := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(a.Target), "/workspace/"), "workspace/")
	if !strings.Contains(editTgt, "/") && strings.HasSuffix(editTgt, ".py") && editTgt != "workspace_audit.py" && editTgt != "script.py" {
		editTgt = "modules/" + editTgt
	}
	if err := e.sandbox.EditFile(editTgt, a.OldText, a.Content); err != nil {
		return "workspace_edit error: " + err.Error()
	}
	_, _ = e.AcquireFileLease(ctx, m.CycleID, editTgt, 5*time.Minute)
	e.RecordConnectomeInspection(ctx, m.CycleID, editTgt, "IMPLEMENTED", "cyc_"+m.CycleID, nil)
	diag := e.sandbox.DiagnoseFile(ctx, editTgt)
	e.telem.EmitLive("sandbox.updated", map[string]string{"path": editTgt})
	e.emitProactiveUpdate(ctx, m.CycleID, m.Model, fmt.Sprintf("✂️ Edited `/workspace/%s` (replaced %dB -> %dB) %s", editTgt, len(a.OldText), len(a.Content), diag))
	return strings.TrimSpace("Edited /workspace/" + editTgt + " " + diag)
}

func (e *Engine) actionTicket(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	if layer != core.LayerExecutive && layer != core.LayerTaskProsecution {
		return "ticket restricted to L4 (Executive) and L6 (Task Prosecution)"
	}
	args := strings.TrimSpace(a.Target)
	if args == "" {
		args = strings.TrimSpace(a.Content)
	}
	lowerArgs := strings.ToLower(args)
	if args == "" || lowerArgs == "log" || lowerArgs == "status" || lowerArgs == "check" {
		args = "list"
	}
	res, _ := e.sandbox.Exec(ctx, "ticket "+args)
	out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	e.emitProactiveUpdate(ctx, m.CycleID, m.Model, fmt.Sprintf("🎫 Pi Ticket (`ticket %s`):\n```json\n%s\n```", truncateText(args, 70), truncateText(out, 500)))
	return out
}

func (e *Engine) actionKB(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	if layer != core.LayerAgentModel && layer != core.LayerTaskProsecution {
		return "kb restricted to L3 (Agent Model) and L6 (Task Prosecution)"
	}
	args := strings.TrimSpace(a.Target)
	if args == "" {
		args = strings.TrimSpace(a.Content)
	}
	if args == "" {
		args = "search \"\""
	}
	res, _ := e.sandbox.Exec(ctx, "kb "+args)
	out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	e.emitProactiveUpdate(ctx, m.CycleID, m.Model, fmt.Sprintf("🧠 Pi KB (`kb %s`):\n```json\n%s\n```", truncateText(args, 70), truncateText(out, 500)))
	return out
}

func (e *Engine) actionParallelResearch(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	if layer != core.LayerGlobalStrategy && layer != core.LayerTaskProsecution {
		return "parallel_research restricted to L2 (Global Strategy) and L6 (Task Prosecution)"
	}
	rawQueries := strings.Split(a.Target, ";")
	if len(rawQueries) == 1 && strings.TrimSpace(a.Content) != "" {
		rawQueries = strings.Split(a.Content, ";")
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
			webRes, err := tools.SafeWebSearch(ctx, query)
			if err != nil {
				webRes = "web error: " + err.Error()
			}
			memHits, _ := e.memory.SearchForLayer(ctx, layer.Code(), query, 3)
			var memStr strings.Builder
			for _, mm := range memHits {
				memStr.WriteString(fmt.Sprintf("  * [Memory %s] %s: %s\n", mm.LayerCode, mm.Title, truncateText(mm.Content, 140)))
			}
			results[idx] = fmt.Sprintf("### Swarm Worker #%d (`%s`)\n%s\n%s", idx+1, query, truncateText(webRes, 900), memStr.String())
		}(i, q)
	}
	wg.Wait()
	combined := strings.Join(results, "\n\n")
	e.recordGlobalLoop("swarm_coordinator", "healthy", fmt.Sprintf("Completed %d parallel workers (%dB synthesized)", len(queries), len(combined)))
	e.emitProactiveUpdate(ctx, m.CycleID, m.Model, fmt.Sprintf("🐝 Swarm Coordinator (`%d parallel workers`):\n```\n%s\n```", len(queries), truncateText(combined, 600)))
	return combined
}

func (e *Engine) actionWikiRead(ctx context.Context, a tools.Args) string {
	target := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(a.Target), "/wiki/"), "wiki/"))
	if target == "notes/goals.md" || strings.HasSuffix(target, "/goals.md") {
		target = "goals.md"
	}
	wData, err := e.memory.ReadWiki(target)
	if err != nil {
		return "wiki_read error: " + err.Error()
	}
	return fmt.Sprintf("=== /wiki/%s (%d bytes) ===\n%s", target, len(wData), truncateText(wData, 16000))
}

func (e *Engine) actionWikiWrite(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	target := strings.TrimSpace(a.Target)
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
		target = "notes/insight_" + m.CycleID + ".md"
	}
	// Hard L1 Constitutional & Mission Immutability Guard (Dave Shapiro ACE Spec):
	// Neither L1 nor any downstream layer (L2..L6) may EVER mutate soul.md or mission.md.
	// The L1 Constitution (Heuristic Imperatives, UDHR) and # CURRENT MISSION are unshifting once set by the human.
	if target == "soul.md" || target == "mission.md" || strings.HasSuffix(target, "/soul.md") || strings.HasSuffix(target, "/mission.md") {
		return "CONSTITUTIONAL VETO: /wiki/soul.md (L1 Heuristic Imperatives, UDHR & # CURRENT MISSION) is unshifting and read-only to all agent layers. Only the human operator may steer the L1 Mission."
	}
	// Never overwrite an existing Wiki page with 0 bytes! If content is empty, treat as a read.
	if strings.TrimSpace(a.Content) == "" {
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
		newGoals := collapseMultilineMilestoneBullets(strings.TrimSpace(a.Content))
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
						if !alreadyPresent {
							updatedExisting = strings.TrimSpace(updatedExisting) + "\n" + tnl + "\n"
							appliedInPlace = true
						}
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
		a.Content = newGoals
	}
	if err := e.memory.WriteWiki(target, a.Content); err != nil {
		return "wiki_write error: " + err.Error()
	}
	existingLog, _ := e.memory.ReadWiki("log.md")
	logEntry := fmt.Sprintf("\n## [%s] %s Updated `%s`\n- Cycle: `%s`\n", time.Now().UTC().Format(time.RFC3339), layer.Code(), target, m.CycleID)
	_ = e.memory.WriteWiki("log.md", existingLog+logEntry)
	if layer == core.LayerTaskProsecution {
		e.emitProactiveUpdate(ctx, m.CycleID, m.Model, fmt.Sprintf("📚 Updated Wiki page `%s` (%d bytes).", target, len(a.Content)))
	}
	return "Updated Wiki page: " + target
}

func (e *Engine) actionWorkspaceWrite(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	if layer != core.LayerTaskProsecution {
		return "workspace_write restricted to L6"
	}
	if strings.TrimSpace(a.Content) == "" {
		return "workspace_write error: action.content cannot be empty"
	}
	writeTgt := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(a.Target), "/workspace/"), "workspace/")
	if !strings.Contains(writeTgt, "/") && strings.HasSuffix(writeTgt, ".py") && writeTgt != "workspace_audit.py" && writeTgt != "script.py" {
		writeTgt = "modules/" + writeTgt
	}
	if err := e.sandbox.WriteFile(writeTgt, a.Content); err != nil {
		return "workspace_write error: " + err.Error()
	}
	_, _ = e.AcquireFileLease(ctx, m.CycleID, writeTgt, 5*time.Minute)
	_ = e.StageWorkpadFile(m.CycleID, writeTgt, []byte(a.Content))
	e.RecordConnectomeInspection(ctx, m.CycleID, writeTgt, "IMPLEMENTED", "cyc_"+m.CycleID, nil)
	diag := e.sandbox.DiagnoseFile(ctx, writeTgt)
	e.telem.EmitLive("sandbox.updated", map[string]string{"path": writeTgt})
	e.emitProactiveUpdate(ctx, m.CycleID, m.Model, fmt.Sprintf("🛠️ Wrote `/workspace/%s` (%d bytes) %s", writeTgt, len(a.Content), diag))
	return strings.TrimSpace("Wrote /workspace/" + writeTgt + " " + diag)
}

func (e *Engine) actionSandboxSh(ctx context.Context, a tools.Args) string {
	_, layer := actionCtx(ctx)
	if layer != core.LayerTaskProsecution {
		return "sandbox_sh restricted to L6"
	}
	cmdStr := strings.TrimSpace(a.Target)
	bodyStr := strings.TrimSpace(a.Content)
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
	// Spec 9.4: past tasks.tool_timeout (120s) the call detaches to a
	// background handle the layer can poll — it is not an error.
	var res core.ExecResult
	var err error
	if e.taskMgr != nil {
		execFn := func(c context.Context, cmd string) (string, error) {
			var r core.ExecResult
			var e2 error
			if j, ok := e.sandbox.(interface {
				ExecWithTimeout(context.Context, string, time.Duration) (core.ExecResult, error)
			}); ok {
				r, e2 = j.ExecWithTimeout(c, cmd, 30*time.Minute)
			} else {
				r, e2 = e.sandbox.Exec(c, cmd)
			}
			res = r
			return r.Stdout + "\n" + r.Stderr, e2
		}
		var h *tasks.Handle
		_, h, err = e.taskMgr.ExecOrHandle(ctx, execFn, cmdStr, tasks.ToolTimeout)
		if h != nil {
			e.telem.EmitLive("sandbox.detached", map[string]string{"task": h.TaskID})
			return fmt.Sprintf("still running past %s — detached to background handle task %s (kind=exec_handle); poll job_status or read its checkpoint for output.", tasks.ToolTimeout, h.TaskID)
		}
	} else {
		res, err = e.sandbox.Exec(ctx, cmdStr)
	}
	e.telem.EmitLive("sandbox.exec", res)
	e.telem.EmitLive("sandbox.updated", map[string]string{"path": "/workspace"})
	out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	spilled := e.sandbox.SpilloverTruncate("exec", out, 4500)
	summary := fmt.Sprintf("exit=0 (%dms): %s", res.DurationMs, spilled)
	if err != nil {
		summary = fmt.Sprintf("exit=%d (%dms): %s", res.ExitCode, res.DurationMs, spilled)
	}
	return summary
}

func (e *Engine) actionChatProactive(ctx context.Context, a tools.Args) string {
	m, layer := actionCtx(ctx)
	if strings.TrimSpace(a.Content) == "" {
		return ""
	}
	msg := core.ChatMessage{
		ID:        "cht_" + uuid.NewString()[:12],
		AgentID:   e.agentID,
		Role:      "proactive",
		Source:    layer.Code(),
		Content:   a.Content,
		Model:     m.Model,
		CycleID:   m.CycleID,
		CreatedAt: time.Now().UTC(),
	}
	_ = e.memory.SaveChatMessage(ctx, msg)
	return "Sent proactive message to user chat"
}
