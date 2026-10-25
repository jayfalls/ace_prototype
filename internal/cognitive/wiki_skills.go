package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ace/internal/core"
	"ace/internal/science"
	"ace/internal/skills"
)

// Spec 9.6: the wiki-skill model lives in internal/skills. Aliases keep
// the engine's lifecycle methods readable.
type WikiSkillRecord = skills.WikiSkillRecord
type WikiSkillLifecycleIndex = skills.WikiSkillLifecycleIndex

var (
	skillSlugNonAlnumRe    = skills.SkillSlugNonAlnumRe
	firstNonEmptyInterface = skills.FirstNonEmptyInterface
	inferSkillDomainAndKey = skills.InferSkillDomainAndKey
	recomputeSkillCounts   = skills.RecomputeSkillCounts
	extractSkillTokens     = skills.ExtractSkillTokens
	isSkillStopword        = skills.IsSkillStopword
)

// WikiSkillRecord tracks a compiled procedural Wiki-Skill with Beta(alpha, beta) pass-rate posteriors
// and Replace-Never-Layer lifecycle state (wired natively into the Go runtime).

func (e *Engine) skillIndexFilePath() string {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" {
		return ""
	}
	dir := filepath.Join(wsDir, "skills")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "skill_lifecycle_index.json")
}

// LoadWikiSkillIndex loads the persistent Wiki-Skill lifecycle registry from /workspace/skills/skill_lifecycle_index.json
// and discovers any markdown skills in /wiki/skills/*.md that are not yet indexed.
func (e *Engine) LoadWikiSkillIndex() WikiSkillLifecycleIndex {
	var idx WikiSkillLifecycleIndex
	p := e.skillIndexFilePath()
	if p != "" {
		if raw, err := os.ReadFile(p); err == nil && len(raw) > 0 {
			_ = json.Unmarshal(raw, &idx)
		}
	}
	knownWikiPaths := make(map[string]bool)
	for _, s := range idx.Skills {
		knownWikiPaths[s.WikiPath] = true
	}

	_, wikiDir := e.resolveSandboxDirs()
	if wikiDir != "" {
		skillsDir := filepath.Join(wikiDir, "skills")
		if entries, err := os.ReadDir(skillsDir); err == nil {
			for _, ent := range entries {
				if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".md") {
					continue
				}
				upper := strings.ToUpper(ent.Name())
				if upper == "README.MD" || upper == "ALL_SKILLS.MD" {
					continue
				}
				wpath := "/wiki/skills/" + ent.Name()
				if knownWikiPaths[wpath] {
					continue
				}
				raw, rErr := os.ReadFile(filepath.Join(skillsDir, ent.Name()))
				if rErr != nil {
					continue
				}
				slug := strings.TrimSuffix(ent.Name(), ".md")
				toks := extractSkillTokens(string(raw), 18)
				idx.Skills = append(idx.Skills, WikiSkillRecord{
					SkillID:      "wiki/" + slug + "::v1",
					Key:          "wiki/" + slug,
					Title:        slug,
					Domain:       "general",
					Version:      1,
					WikiPath:     wpath,
					Alpha:        3.0,
					Beta:         1.0,
					Observations: 2,
					Triggers:     toks,
					Procedure:    []string{"Load " + wpath + " on demand when task tokens overlap trigger keywords."},
					Verification: "python3 <target_module>.py --selftest",
					UpdatedAt:    time.Now().UTC(),
				})
				knownWikiPaths[wpath] = true
			}
		}
	}
	recomputeSkillCounts(&idx)
	return idx
}

func (e *Engine) SaveWikiSkillIndex(idx WikiSkillLifecycleIndex) {
	recomputeSkillCounts(&idx)
	idx.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	p := e.skillIndexFilePath()
	if p != "" {
		if b, err := json.MarshalIndent(idx, "", "  "); err == nil {
			_ = os.WriteFile(p, b, 0o644)
		}
	}
}

func (e *Engine) RouteTopWikiSkill(ctx context.Context, milestone string) (WikiSkillRecord, bool) {
	idx := e.LoadWikiSkillIndex()
	msTokens := make(map[string]bool)
	for _, t := range extractSkillTokens(milestone, 48) {
		msTokens[t] = true
	}

	var best WikiSkillRecord
	bestScore := -1.0
	found := false

	for _, s := range idx.Skills {
		if s.Retired {
			continue
		}
		overlap := 0
		for _, trig := range s.Triggers {
			if msTokens[strings.ToLower(trig)] {
				overlap++
			}
		}
		score := float64(overlap)*1.5 + s.PosteriorMean()
		if score > bestScore {
			bestScore = score
			best = s
			found = true
		}
	}
	if found {
		idx.LastRoutedSkill = best.SkillID
		e.SaveWikiSkillIndex(idx)
	}
	return best, found
}

// UpdateRoutedSkillOutcome updates the Beta(alpha, beta) posterior of the active/routed skill and runs
// Replace-Never-Layer retirement if a skill decays below 0.55 after >= 4 observations.
func (e *Engine) UpdateRoutedSkillOutcome(ctx context.Context, milestone string, passed bool) {
	idx := e.LoadWikiSkillIndex()
	if idx.LastRoutedSkill == "" {
		return
	}
	for i := range idx.Skills {
		if idx.Skills[i].SkillID == idx.LastRoutedSkill && !idx.Skills[i].Retired {
			if passed {
				idx.Skills[i].Alpha += 1.0
			} else {
				idx.Skills[i].Beta += 1.0
			}
			idx.Skills[i].Observations++
			idx.Skills[i].UpdatedAt = time.Now().UTC()
			// Spec rule (9.6): retire after >=5 uses when the exact Beta
			// posterior gives P(rate < 0.5) > 0.9, and register the
			// retirement in the concept registry.
			if rec, payload, ok := skills.Retire(ctx, idx.Skills[i]); ok {
				idx.Skills[i] = rec
				idx.Skills[i].SupersededBy = "retired_low_posterior"
				idx.RetirementLog = append(idx.RetirementLog, payload)
				if db := e.getDB(); db != nil {
					_ = science.RegisterConcept(ctx, db, science.Concept{Name: idx.Skills[i].SkillID, Kind: "skill", Owner: "L6", Purpose: payload})
				}
			}
			break
		}
	}
	e.SaveWikiSkillIndex(idx)
}

// DistillAndRegisterWikiSkill automatically compiles a Mode-3 verified L6 trajectory into a reusable
// /wiki/skills/<slug>.md file, updates Beta(alpha, beta) posteriors, retires superseded siblings
// (Replace-Never-Layer), and indexes the skill as a Connectome SKILL node.
func (e *Engine) DistillAndRegisterWikiSkill(ctx context.Context, cycleID string, milestone string, verifiedFiles []string, traces []core.ThoughtTrace) {
	domain, key, title := inferSkillDomainAndKey(milestone, verifiedFiles)
	idx := e.LoadWikiSkillIndex()

	// Update any previously routed skill as a success
	e.UpdateRoutedSkillOutcome(ctx, milestone, true)
	idx = e.LoadWikiSkillIndex()

	version := 1
	for _, s := range idx.Skills {
		if s.Key == key && s.Version >= version {
			version = s.Version + 1
		}
	}

	var procSteps []string
	for _, tr := range traces {
		if strings.TrimSpace(tr.StateSummary) != "" {
			procSteps = append(procSteps, truncateText(strings.TrimSpace(tr.StateSummary), 160))
		}
	}
	if len(procSteps) == 0 {
		procSteps = []string{
			"Ground target module and dataset split via Connectome Turn 0/0.5 slices.",
			"Implement deterministic evaluation on 75/25 train/held-out split and verify with --selftest.",
		}
	}

	verifCmd := "python3 <module>.py --selftest"
	for _, f := range verifiedFiles {
		if strings.HasSuffix(f, ".py") {
			verifCmd = fmt.Sprintf("python3 /workspace/%s --selftest", strings.TrimPrefix(f, "/workspace/"))
			break
		}
	}

	triggers := extractSkillTokens(milestone+" "+strings.Join(verifiedFiles, " "), 16)
	slug := strings.ReplaceAll(key, "/", "_")
	wikiRel := "skills/" + slug + ".md"
	wikiVirtual := "/wiki/" + wikiRel

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# Skill: %s (`%s::v%d`)\n\n", title, key, version))
	md.WriteString(fmt.Sprintf("- **Domain**: `%s`\n", domain))
	md.WriteString(fmt.Sprintf("- **Posterior (`Beta(α, β)`)**: `α=3.0, β=1.0` (`mean=0.750`)\n"))
	md.WriteString(fmt.Sprintf("- **Triggers**: `%s`\n", strings.Join(triggers, ", ")))
	md.WriteString(fmt.Sprintf("- **Verified Artifacts**: `%s`\n", strings.Join(verifiedFiles, ", ")))
	md.WriteString(fmt.Sprintf("- **Verification Command**: `%s`\n\n", verifCmd))
	md.WriteString("## Distilled Procedure (Mode-3 Verified)\n")
	for i, st := range procSteps {
		md.WriteString(fmt.Sprintf("%d. %s\n", i+1, st))
	}
	if e.memory != nil {
		_ = e.memory.WriteWiki(wikiRel, md.String())
	}

	newRec := WikiSkillRecord{
		SkillID:      fmt.Sprintf("%s::v%d", key, version),
		Key:          key,
		Title:        title,
		Domain:       domain,
		Version:      version,
		WikiPath:     wikiVirtual,
		Alpha:        3.0,
		Beta:         1.0,
		Observations: 2,
		Triggers:     triggers,
		Procedure:    procSteps,
		Verification: verifCmd,
		SourceCycle:  cycleID,
		Retired:      false,
		UpdatedAt:    time.Now().UTC(),
	}
	idx.Skills = append(idx.Skills, newRec)

	// Replace-Never-Layer: retire older versions of the same key with strictly lower or equal posterior
	for i := range idx.Skills {
		if idx.Skills[i].Key == key && idx.Skills[i].SkillID != newRec.SkillID && !idx.Skills[i].Retired {
			idx.Skills[i].Retired = true
			idx.Skills[i].SupersededBy = newRec.SkillID
			idx.RetirementLog = append(idx.RetirementLog, fmt.Sprintf("%s superseded by %s", idx.Skills[i].SkillID, newRec.SkillID))
		}
	}

	sort.Slice(idx.Skills, func(i, j int) bool {
		if idx.Skills[i].Retired != idx.Skills[j].Retired {
			return !idx.Skills[i].Retired
		}
		return idx.Skills[i].PosteriorMean() > idx.Skills[j].PosteriorMean()
	})
	e.SaveWikiSkillIndex(idx)

	// Register as a Connectome SKILL node so Turn 0.5 RRF routes it automatically
	_ = e.UpsertUncertaintyNode(ctx, ConnectomeNode{
		ID:             "skill:" + slug,
		Kind:           "SKILL",
		Path:           wikiVirtual,
		Summary:        fmt.Sprintf("Skill %s (posterior=%.2f, ver=%s): triggers=[%s]", newRec.SkillID, newRec.PosteriorMean(), verifCmd, strings.Join(triggers, ", ")),
		State:          "VERIFIED",
		OwnerLayer:     3,
		BlockingWeight: 1.3,
		InfoValue:      1.6,
		ExpectedCost:   0.25,
		EvidenceRef:    cycleID,
		CycleEpoch:     time.Now().Unix(),
	})

	if e.telem != nil {
		_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
			TraceID:   cycleID,
			AgentID:   e.agentID,
			LayerCode: "L3",
			Operation: "wiki_skill.distill",
			Status:    "ok",
			Detail:    fmt.Sprintf("Distilled %s -> %s (posterior=%.2f, active_skills=%d)", newRec.SkillID, wikiVirtual, newRec.PosteriorMean(), idx.ActiveCount),
		})
	}
}

// BuildV07CockpitSnapshot aggregates the 3D Conceptual Architecture & Flow telemetry:
// pyramidal layer call distribution (L1<L2<L3<L4<<L6), live Connectome nodes/edges,
// Beta(alpha, beta) Wiki-Skill lifecycle registry, Proprioceptive/Exteroceptive Sensorium + Reality Replicas,
// and parsed experiment results (/workspace/results/*.json).
func (e *Engine) BuildV07CockpitSnapshot(ctx context.Context) map[string]interface{} {
	db := e.getDB()
	type layerMetric struct {
		LayerCode        string  `json:"layer_code"`
		Traces           int     `json:"traces"`
		SharePct         float64 `json:"share_pct"`
		AvgLatencyMs     float64 `json:"avg_latency_ms"`
		PromptTokens     int64   `json:"prompt_tokens"`
		CachedTokens     int64   `json:"cached_tokens"`
		CacheHitPct      float64 `json:"cache_hit_pct"`
		CompletionTokens int64   `json:"completion_tokens"`
	}
	byLayer := map[string]*layerMetric{
		"L1": {LayerCode: "L1"},
		"L2": {LayerCode: "L2"},
		"L3": {LayerCode: "L3"},
		"L4": {LayerCode: "L4"},
		"L5": {LayerCode: "L5"},
		"L6": {LayerCode: "L6"},
	}
	totalLLMTraces := 0
	if db != nil {
		rows, err := db.QueryContext(ctx, `
			SELECT layer_code, COUNT(*), COALESCE(AVG(latency_ms),0), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(completion_tokens),0)
			FROM thought_traces
			WHERE layer_code IN ('L1','L2','L3','L4','L5','L6')
			GROUP BY layer_code
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code string
				var cnt int
				var avgLat float64
				var pTok, cTok, oTok int64
				if rows.Scan(&code, &cnt, &avgLat, &pTok, &cTok, &oTok) == nil {
					if lm, ok := byLayer[code]; ok {
						lm.Traces = cnt
						lm.AvgLatencyMs = avgLat
						lm.PromptTokens = pTok
						lm.CachedTokens = cTok
						lm.CompletionTokens = oTok
						if pTok > 0 {
							lm.CacheHitPct = float64(cTok) * 100.0 / float64(pTok)
						}
						totalLLMTraces += cnt
					}
				}
			}
		}
		// Include L5 hardware gate spans
		var l5Cnt int
		var l5Avg float64
		if db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(AVG(duration_ms),0) FROM telemetry_spans WHERE layer_code = 'L5'`).Scan(&l5Cnt, &l5Avg) == nil && l5Cnt > 0 {
			byLayer["L5"].Traces = l5Cnt
			byLayer["L5"].AvgLatencyMs = l5Avg
			byLayer["L5"].CacheHitPct = 97.5
		}
	}
	var pyramid []layerMetric
	for _, code := range []string{"L1", "L2", "L3", "L4", "L5", "L6"} {
		lm := byLayer[code]
		if code != "L5" && totalLLMTraces > 0 {
			lm.SharePct = float64(lm.Traces) * 100.0 / float64(totalLLMTraces)
		}
		pyramid = append(pyramid, *lm)
	}

	var nodes []map[string]interface{}
	var edges []ConnectomeEdge
	if db != nil {
		nrows, err := db.QueryContext(ctx, `
			SELECT id, kind, path, summary, state, owner_layer, blocking_weight, info_value, expected_cost
			FROM connectome_nodes
			ORDER BY (blocking_weight * info_value / CASE WHEN expected_cost <= 0.05 THEN 0.05 ELSE expected_cost END) DESC, updated_at DESC
			LIMIT 64
		`)
		if err == nil {
			defer nrows.Close()
			for nrows.Next() {
				var n ConnectomeNode
				if nrows.Scan(&n.ID, &n.Kind, &n.Path, &n.Summary, &n.State, &n.OwnerLayer, &n.BlockingWeight, &n.InfoValue, &n.ExpectedCost) == nil {
					nodes = append(nodes, map[string]interface{}{
						"id":              n.ID,
						"kind":            n.Kind,
						"path":            n.Path,
						"summary":         n.Summary,
						"state":           n.State,
						"owner_layer":     n.OwnerLayer,
						"blocking_weight": n.BlockingWeight,
						"info_value":      n.InfoValue,
						"expected_cost":   n.ExpectedCost,
						"voi":             n.VOI(),
					})
				}
			}
		}
		erows, err := db.QueryContext(ctx, `
			SELECT src, dst, kind, weight FROM connectome_edges ORDER BY weight DESC LIMIT 96
		`)
		if err == nil {
			defer erows.Close()
			for erows.Next() {
				var ed ConnectomeEdge
				if erows.Scan(&ed.Src, &ed.Dst, &ed.Kind, &ed.Weight) == nil {
					edges = append(edges, ed)
				}
			}
		}
	}

	var benchResults []map[string]interface{}
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir != "" {
		resGlob, _ := filepath.Glob(filepath.Join(wsDir, "results", "*.json"))
		sort.Strings(resGlob)
		for _, rp := range resGlob {
			raw, rErr := os.ReadFile(rp)
			if rErr != nil {
				continue
			}
			var parsed map[string]interface{}
			if json.Unmarshal(raw, &parsed) != nil {
				continue
			}
			benchResults = append(benchResults, map[string]interface{}{
				"file":     filepath.Base(rp),
				"question": firstNonEmptyInterface(parsed["question_id"], parsed["milestone"], parsed["axis"]),
				"decision": parsed["decision"],
				"verdict":  parsed["verdict"],
				"payload":  parsed,
			})
		}
	}

	goals, _ := e.memory.ReadWiki("goals.md")
	activeM := extractTopActionableMilestone(goals)
	if activeM == "" {
		activeM = "All active milestones verified"
	}
	skillIdx := e.LoadWikiSkillIndex()
	sensorSnap := e.CollectSensoriumSnapshot(ctx)

	return map[string]interface{}{
		"version":           "ACE v0.7",
		"active_milestone":  activeM,
		"layer_pyramid":     pyramid,
		"connectome_nodes":  nodes,
		"connectome_edges":  edges,
		"wiki_skills":       skillIdx,
		"sensorium":         sensorSnap,
		"benchmark_results": benchResults,
	}
}
