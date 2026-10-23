package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ace/internal/core"
	"ace/internal/sandbox"
)

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

