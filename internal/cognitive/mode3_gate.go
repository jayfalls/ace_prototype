package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ace/internal/core"
	"ace/internal/specs"
)

// Mode3GateResult captures the deterministic falsification verdict for a milestone checkoff attempt.
type Mode3GateResult struct {
	Passed            bool     `json:"passed"`
	Milestone         string   `json:"milestone"`
	RequiredArtifacts []string `json:"required_artifacts"`
	MissingArtifacts  []string `json:"missing_artifacts"`
	VacuousScripts    []string `json:"vacuous_scripts"`
	FailingScripts    []string `json:"failing_scripts"`
	RegressedScripts  []string `json:"regressed_scripts"`
	HygieneViolations []string `json:"hygiene_violations"`
	Reason            string   `json:"reason"`
	// MutationInconclusive is true when the mutation sample fell below
	// gate.min_mutants — the task completes UNVERIFIED, never COMPLETED.
	MutationInconclusive bool `json:"mutation_inconclusive"`
}

var artifactPathRe = regexp.MustCompile("(?:`/workspace/([^`\\s]+)`|`/wiki/([^`\\s]+)`|/workspace/([a-zA-Z0-9_./-]+\\.[a-zA-Z0-9]+)|\\b(notes/[a-zA-Z0-9_./-]+\\.md)\\b|\\b([a-zA-Z0-9_-]+\\.(?:py|json|md|tsv))\\b)")

func isIgnoredOrWildcardArtifact(p string) bool {
	if p == "" || strings.ContainsAny(p, "*?[]<>{}") || strings.Contains(p, "...") {
		return true
	}
	base := filepath.Base(p)
	// Ignore pre-existing reference corpus papers, domain syntheses, and competitor studies mentioned in "Grounded in ..." clauses
	if strings.HasPrefix(p, "corpus/") ||
		strings.HasPrefix(base, "domain-synthesis-") ||
		strings.HasPrefix(base, "agents-study-") ||
		strings.HasPrefix(base, "extract-") ||
		strings.HasPrefix(base, "paper-") ||
		strings.HasPrefix(base, "meta-harness-") ||
		strings.HasPrefix(base, "public_") {
		return true
	}
	switch p {
	case "workspace_audit.py", "goals.md", "soul.md", "mission.md", "index.md", "log.md", "README.md",
		"live_traces.db", "registry.json":
		return true
	}
	return false
}

// ExtractMilestoneArtifacts parses all required `/workspace/...` and `/wiki/...` artifacts from a milestone line.
func ExtractMilestoneArtifacts(milestoneLine string) (wsFiles []string, wikiFiles []string) {
	seenWS := make(map[string]bool)
	seenWiki := make(map[string]bool)
	scanLine := milestoneLine
	for _, cutMarker := range []string{"L4 Executive Plan:", "| [L4 Revised Plan]:"} {
		if idx := strings.Index(scanLine, cutMarker); idx > 0 {
			scanLine = scanLine[:idx]
		}
	}
	matches := artifactPathRe.FindAllStringSubmatch(scanLine, -1)
	for _, m := range matches {
		wsCand := firstNonEmpty(m[1], m[3], m[5])
		wikiCand := firstNonEmpty(m[2], m[4])
		if strings.HasPrefix(wsCand, "notes/") {
			wikiCand = wsCand
			wsCand = ""
		}
		if wsCand != "" {
			clean := strings.TrimSuffix(normalizeRelPath(wsCand), ".")
			if !isIgnoredOrWildcardArtifact(clean) && !seenWS[clean] {
				seenWS[clean] = true
				wsFiles = append(wsFiles, clean)
			}
		}
		if wikiCand != "" {
			clean := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(wikiCand), "/wiki/"), "wiki/"), ".")
			if !isIgnoredOrWildcardArtifact(clean) && !seenWiki[clean] {
				seenWiki[clean] = true
				wikiFiles = append(wikiFiles, clean)
			}
		}
	}
	return wsFiles, wikiFiles
}


// collapseMultilineMilestoneBullets flattens indented continuation lines beneath `- [ ]` or `- [x]` bullets
// into a single canonical line so multi-line L2 roadmap entries never lose their inline artifact paths or H0/H1 clauses.
func collapseMultilineMilestoneBullets(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	inMilestone := false
	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
			out = append(out, trimmed)
			inMilestone = true
			continue
		}
		if inMilestone {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				inMilestone = false
				out = append(out, rawLine)
				continue
			}
			isIndented := len(rawLine) > 0 && (rawLine[0] == ' ' || rawLine[0] == '\t')
			if isIndented && len(out) > 0 {
				cont := strings.TrimPrefix(trimmed, "- ")
				sep := " "
				if strings.HasSuffix(out[len(out)-1], ":") {
					sep = " "
				} else if !strings.HasSuffix(out[len(out)-1], ".") && !strings.HasSuffix(out[len(out)-1], ";") {
					sep = " — "
				}
				out[len(out)-1] = out[len(out)-1] + sep + cont
				continue
			}
			inMilestone = false
		}
		out = append(out, rawLine)
	}
	return strings.Join(out, "\n")
}

// ExtractPrimaryMilestoneScript returns the first non-__init__ `.py` script referenced in the milestone line (Fix S11).
func ExtractPrimaryMilestoneScript(milestoneLine string) string {
	wsFiles, _ := ExtractMilestoneArtifacts(milestoneLine)
	for _, f := range wsFiles {
		if strings.HasSuffix(f, ".py") && !strings.HasSuffix(f, "__init__.py") {
			return f
		}
	}
	for _, f := range wsFiles {
		if strings.HasSuffix(f, ".py") {
			return f
		}
	}
	return ""
}

// discoverWorkspacePackageDirs returns all subdirectories in workspaceDir that contain Python modules or __init__.py.
func discoverWorkspacePackageDirs(workspaceDir string) []string {
	if workspaceDir == "" {
		return nil
	}
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return nil
	}
	var pkgs []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		if name == "archive" || name == "bin" || name == "datasets" || name == "results" {
			continue
		}
		if _, err := os.Stat(filepath.Join(workspaceDir, name, "__init__.py")); err == nil {
			pkgs = append(pkgs, name)
			continue
		}
		if strings.HasPrefix(name, "ace_") {
			pkgs = append(pkgs, name)
		}
	}
	return pkgs
}

// ResolveWorkspaceArtifactPath resolves bare filenames (e.g. "harness2_improver.py") to "<pkg>/harness2_improver.py"
// (or vice versa) across any discovered workspace package directory, preventing split-brain path mismatches.
func ResolveWorkspaceArtifactPath(workspaceDir string, wf string) string {
	wf = strings.TrimSuffix(strings.TrimSuffix(normalizeRelPath(wf), "."), "/")
	if workspaceDir == "" || wf == "" {
		return wf
	}
	if info, err := os.Stat(filepath.Join(workspaceDir, wf)); err == nil && (info.IsDir() || info.Size() > 0) {
		return wf
	}
	if !strings.Contains(wf, "/") {
		for _, sub := range []string{"datasets", "results", "modules"} {
			cand := filepath.Join(sub, wf)
			if info, err := os.Stat(filepath.Join(workspaceDir, cand)); err == nil && info.Size() > 0 {
				return filepath.ToSlash(cand)
			}
		}
		for _, pkg := range discoverWorkspacePackageDirs(workspaceDir) {
			cand := filepath.Join(pkg, wf)
			if info, err := os.Stat(filepath.Join(workspaceDir, cand)); err == nil && info.Size() > 0 {
				return filepath.ToSlash(cand)
			}
		}
	} else {
		base := filepath.Base(wf)
		if info, err := os.Stat(filepath.Join(workspaceDir, base)); err == nil && info.Size() > 0 {
			return base
		}
		for _, sub := range []string{"datasets", "results", "modules"} {
			cand := filepath.Join(sub, base)
			if info, err := os.Stat(filepath.Join(workspaceDir, cand)); err == nil && info.Size() > 0 {
				return filepath.ToSlash(cand)
			}
		}
		for _, pkg := range discoverWorkspacePackageDirs(workspaceDir) {
			cand := filepath.Join(pkg, base)
			if info, err := os.Stat(filepath.Join(workspaceDir, cand)); err == nil && info.Size() > 0 {
				return filepath.ToSlash(cand)
			}
		}
	}
	return wf
}

// BuildMilestoneOraclePacket generates the deterministic conformance checklist for the dynamic suffix
// by dynamically discovering required milestone artifacts and any SQLite/CSV/dataset files in /workspace/.
// Byte counts are intentionally omitted so minor file edits do not churn the OracleGate string.
func BuildMilestoneOraclePacket(milestoneLine string, workspaceDir string, wikiDir string) string {
	if strings.TrimSpace(milestoneLine) == "" {
		return ""
	}
	wsFiles, wikiFiles := ExtractMilestoneArtifacts(milestoneLine)
	var checks []string
	for _, rawWf := range wsFiles {
		wf := ResolveWorkspaceArtifactPath(workspaceDir, rawWf)
		abs := filepath.Join(workspaceDir, wf)
		info, err := os.Stat(abs)
		if err != nil || (!info.IsDir() && info.Size() == 0) {
			checks = append(checks, fmt.Sprintf("[MISSING:/workspace/%s]", wf))
		} else {
			checks = append(checks, fmt.Sprintf("[OK:/workspace/%s]", wf))
		}
	}
	for _, kf := range wikiFiles {
		abs := filepath.Join(wikiDir, kf)
		info, err := os.Stat(abs)
		if err != nil || info.Size() < 120 {
			checks = append(checks, fmt.Sprintf("[MISSING:/wiki/%s]", kf))
		} else {
			checks = append(checks, fmt.Sprintf("[OK:/wiki/%s]", kf))
		}
	}
	var dbList []string
	for _, sub := range []string{"", "datasets", "results"} {
		dir := filepath.Join(workspaceDir, sub)
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				ln := strings.ToLower(name)
				if strings.HasSuffix(ln, ".db") || strings.HasSuffix(ln, ".sqlite") || strings.HasSuffix(ln, ".csv") {
					rel := name
					if sub != "" {
						rel = sub + "/" + name
					}
					dbList = append(dbList, "/workspace/"+rel)
				}
			}
		}
	}
	dbSummary := "none yet (use web_search/web_fetch or build a dataset under /workspace/datasets/)"
	if len(dbList) > 0 {
		dbSummary = strings.Join(dbList, ", ")
	}
	out := "OracleGate{" + strings.Join(checks, ", ") + " | AuditSDK:/workspace/workspace_audit.py[audit_script_mutants(path)->dict, audit_conceptual_flows(path)->dict] | DataSources:[" + dbSummary + "] | DatasetStandard:/workspace/datasets/<name>.json{dataset_id,version,provenance,train_split:[>=5],held_out_split:[>=5]} + /workspace/registry.json}"
	if len(out) > 1300 {
		out = out[:1300]
	}
	return out
}

// EvaluateMode3MilestoneGate runs the 5-stage deterministic falsification gate before allowing a milestone `[x]` checkoff:
// 1. Artifact Existence & Schema Non-Empty Check (/workspace/... and /wiki/...)
// 2. AST Non-Vacuity & Zero-Mocking Audit on target `.py` scripts
// 3. Independent `--selftest` Execution (`f2p` verification) inside `bwrap` (`exit == 0`)
// 4. `p2p` Regression Suite across previously verified active scripts
// 5. 4-Pass `AST-PDG` Conceptual Flow & Unused-Import Hygiene Audit (`workspace_audit.audit_conceptual_flows`)
func EvaluateMode3MilestoneGate(ctx context.Context, eng *Engine, milestoneLine string, sandbox core.Sandbox, workspaceDir string, wikiDir string, verifiedFiles []string, minMutants int) Mode3GateResult {
	rawWsFiles, wikiFiles := ExtractMilestoneArtifacts(milestoneLine)
	var wsFiles []string
	seenResolved := make(map[string]bool)
	for _, rwf := range rawWsFiles {
		resolved := ResolveWorkspaceArtifactPath(workspaceDir, rwf)
		if !seenResolved[resolved] {
			seenResolved[resolved] = true
			wsFiles = append(wsFiles, resolved)
		}
	}
	// If the milestone identifies an Axis/Milestone (e.g., "axis 8.1"), also attach any existing matching JSON result file
	// under results/ so truncated milestone lines can never bypass ValidateJSONExperimentSubstance.
	if axisID := extractMilestoneIdentifier(milestoneLine); axisID != "" && workspaceDir != "" {
		parts := strings.Fields(axisID)
		if len(parts) == 2 {
			slug := parts[0] + strings.ReplaceAll(parts[1], ".", "_")
			resDir := filepath.Join(workspaceDir, "results")
			if entries, err := os.ReadDir(resDir); err == nil {
				for _, e := range entries {
					if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
						continue
					}
					if strings.Contains(strings.ToLower(e.Name()), slug) {
						rel := filepath.ToSlash(filepath.Join("results", e.Name()))
						if !seenResolved[rel] {
							seenResolved[rel] = true
							wsFiles = append(wsFiles, rel)
						}
					}
				}
			}
		}
	}
	// Support plain-English user milestones that do not explicitly embed `/workspace/...` file paths
	// by validating the files actually created/verified in /workspace/ during the milestone!
	if len(wsFiles) == 0 && len(wikiFiles) == 0 {
		seen := make(map[string]bool)
		for _, vf := range verifiedFiles {
			clean := normalizeRelPath(vf)
			if clean != "" && !isIgnoredOrWildcardArtifact(clean) && !seen[clean] {
				seen[clean] = true
				wsFiles = append(wsFiles, clean)
			}
		}
		if len(wsFiles) == 0 {
			return Mode3GateResult{
				Passed:    false,
				Milestone: milestoneLine,
				Reason:    "No /workspace/ or /wiki/ artifacts created or verified yet for this milestone",
			}
		}
	}
	res := Mode3GateResult{
		Passed:            true,
		Milestone:         milestoneLine,
		RequiredArtifacts: append(append([]string{}, wsFiles...), wikiFiles...),
	}

	// Spec 9.6 output-vs-spec check: touched files declared out of scope by
	// the milestone's living spec are divergence — reject before Stage 1.
	if wikiDir != "" && len(wsFiles) > 0 {
		specID := sanitizeTaskID(extractMilestoneShortID(milestoneLine))
		if raw, err := os.ReadFile(filepath.Join(wikiDir, "specs", specID+".md")); err == nil && len(raw) > 0 {
			if err := specs.CheckOutOfScope(string(raw), wsFiles); err != nil {
				res.Passed = false
				res.Reason = "living spec divergence: " + err.Error()
				return res
			}
		}
	}

	// Stage 1: Artifact existence & non-empty verification (JSON substance verified after Stage 2 --selftest runs)
	for _, wf := range wsFiles {
		abs := filepath.Join(workspaceDir, wf)
		if info, sErr := os.Stat(abs); sErr == nil && info.IsDir() {
			entries, dErr := os.ReadDir(abs)
			if dErr != nil || len(entries) == 0 {
				res.Passed = false
				res.MissingArtifacts = append(res.MissingArtifacts, "/workspace/"+wf)
			}
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil || len(strings.TrimSpace(string(data))) == 0 {
			res.Passed = false
			res.MissingArtifacts = append(res.MissingArtifacts, "/workspace/"+wf)
			continue
		}
	}
	for _, kf := range wikiFiles {
		abs := filepath.Join(wikiDir, kf)
		data, err := os.ReadFile(abs)
		if err != nil || len(strings.TrimSpace(string(data))) < 120 {
			res.Passed = false
			res.MissingArtifacts = append(res.MissingArtifacts, "/wiki/"+kf)
		}
	}
	if len(res.MissingArtifacts) > 0 {
		res.Reason = "Missing or empty required milestone artifacts: " + strings.Join(res.MissingArtifacts, ", ")
		return res
	}

	// Stage 2a: Fast in-memory AST Non-Vacuity check across ALL `.py` artifacts FIRST (<1ms) before spawning any subprocesses!
	var pyFiles []string
	for _, wf := range wsFiles {
		if !strings.HasSuffix(wf, ".py") {
			continue
		}
		abs := filepath.Join(workspaceDir, wf)
		srcBytes, err := os.ReadFile(abs)
		if err != nil {
			res.Passed = false
			res.MissingArtifacts = append(res.MissingArtifacts, "/workspace/"+wf)
			continue
		}
		if strings.HasSuffix(wf, "__init__.py") {
			pyFiles = append(pyFiles, wf)
			continue
		}
		if vacReason := checkPythonASTNonVacuity(string(srcBytes)); vacReason != "" {
			res.Passed = false
			res.VacuousScripts = append(res.VacuousScripts, fmt.Sprintf("%s(%s)", wf, vacReason))
			continue
		}
		pyFiles = append(pyFiles, wf)
	}
	if !res.Passed {
		return finalizeMode3GateReason(res)
	}

	primaryPy := ResolveWorkspaceArtifactPath(workspaceDir, ExtractPrimaryMilestoneScript(milestoneLine))

	// Stage 2b & 3: Independent `--selftest` (`f2p`) + AST-PDG flow check (+ SWE-Mutation on primary target script)
	for idx, wf := range pyFiles {
		// Package __init__.py files re-export package symbols and do not run standalone --selftest
		if strings.HasSuffix(wf, "__init__.py") {
			if sandbox != nil {
				execRes, err := sandbox.Exec(ctx, fmt.Sprintf("python3 -m py_compile /workspace/%s", wf))
				if err != nil || execRes.ExitCode != 0 {
					res.Passed = false
					res.FailingScripts = append(res.FailingScripts, fmt.Sprintf("%s(syntax error)", wf))
				}
			}
			continue
		}
		if sandbox != nil {
			execRes, err := sandbox.Exec(ctx, fmt.Sprintf("python3 /workspace/%s --selftest", wf))
			if err != nil || execRes.ExitCode != 0 {
				if eng != nil {
					eng.recordModuleSelftest(ctx, wf, "fail")
				}
				res.Passed = false
				errTail := strings.TrimSpace(execRes.Stderr + " " + execRes.Stdout)
				if len(errTail) > 160 {
					errTail = errTail[len(errTail)-160:]
				}
				res.FailingScripts = append(res.FailingScripts, fmt.Sprintf("%s(exit=%d: %s)", wf, execRes.ExitCode, errTail))
				continue
			}
			if eng != nil {
				eng.recordModuleSelftest(ctx, wf, "pass")
			}
			// Run expensive multi-subprocess mutation testing only on the primary target script (or when <= 2 scripts in milestone)
			runMutants := "False"
			if len(pyFiles) <= 2 || wf == primaryPy || (primaryPy == "" && idx == len(pyFiles)-1) {
				runMutants = "True"
			}
			// Stage 5: AST-PDG Conceptual Flow, Anti-Cheating Tautology & Mutation Falsification check on target script
			hygCmd := fmt.Sprintf(`python3 -c '
import ast, json, sys
sys.path.insert(0, "/workspace")
import workspace_audit

path = "/workspace/%s"
run_mutants = %s
src = open(path, "r", encoding="utf-8").read()
tree = ast.parse(src, filename=path)

# 1. Reject tautological assertions (assert True, assert 1 == 1, assert x == x)
tautologies = []
assert_count = 0
for node in ast.walk(tree):
    if isinstance(node, ast.Raise):
        assert_count += 1
    elif isinstance(node, ast.Assert):
        assert_count += 1
        t = node.test
        if isinstance(t, ast.Constant) and bool(t.value) is True:
            tautologies.append(f"line {node.lineno}: assert True")
        elif isinstance(t, ast.Compare) and len(t.comparators) == 1:
            left_d = ast.dump(t.left)
            right_d = ast.dump(t.comparators[0])
            if left_d == right_d or (isinstance(t.left, ast.Constant) and isinstance(t.comparators[0], ast.Constant)):
                tautologies.append(f"line {node.lineno}: constant/self comparison")

# 2. Check AST-PDG conceptual cleanliness
flow_rep = workspace_audit.audit_conceptual_flows(path)

# 3. Run SWE-Mutation falsification check. Below gate.min_mutants the
# sample is too thin to prove anything: inconclusive, not a veto.
MIN_MUT = %d
if run_mutants:
    mut_rep = workspace_audit.audit_script_mutants(path, max_mutants=max(6, MIN_MUT))
    mut_total = mut_rep["total_mutants"]
    mut_ok = (mut_total < MIN_MUT) or (mut_rep["killed"] > 0)
else:
    mut_rep = {"total_mutants": 0, "killed": 0, "skipped_secondary": True}
    mut_total = 0
    mut_ok = True

ok = (assert_count > 0) and (len(tautologies) == 0) and bool(flow_rep.get("clean")) and mut_ok
flow_issues = {k: v for k, v in flow_rep.items() if k != "clean" and v}
# Surface surviving mutants so the worker can target the exact line -
# "killed: 0" alone is a blind veto that produces identical rewrites.
survivors = ["line {}: {}".format(d.get("line"), d.get("mutation")) for d in mut_rep.get("details", []) if not d.get("killed")]
print(json.dumps({"ok": ok, "assert_count": assert_count, "tautologies": tautologies, "flow_clean": bool(flow_rep.get("clean")), "flow_issues": flow_issues, "mutants": {"total": mut_rep["total_mutants"], "killed": mut_rep["killed"], "inconclusive": mut_total < MIN_MUT, "survivors": survivors[:4]}}))
sys.exit(0 if ok else 2)
'`, wf, runMutants, minMutants)
			hygRes, hygErr := sandbox.Exec(ctx, hygCmd)
			if hygErr == nil && hygRes.ExitCode == 0 {
				// Parse the audit verdict even on pass: an inconclusive
				// mutation sample completes the task UNVERIFIED.
				var audit struct {
					Mutants struct {
						Inconclusive bool `json:"inconclusive"`
					} `json:"mutants"`
				}
				if json.Unmarshal([]byte(strings.TrimSpace(hygRes.Stdout)), &audit) == nil && audit.Mutants.Inconclusive {
					res.MutationInconclusive = true
				}
			}
			if hygErr != nil || hygRes.ExitCode != 0 {
				res.Passed = false
				hygMsg := strings.TrimSpace(hygRes.Stdout + " " + hygRes.Stderr)
				if hygErr != nil {
					hygMsg = strings.TrimSpace(hygMsg + " " + hygErr.Error())
				}
				res.HygieneViolations = append(res.HygieneViolations, fmt.Sprintf("%s: %s", wf, truncateForVeto(hygMsg, 320)))
			}
		}
	}

	// Stage 3.5: Dataset Split, Provenance & Hypothesis Falsification Gate on every `.json` artifact
	for _, wf := range wsFiles {
		if !strings.HasSuffix(wf, ".json") {
			continue
		}
		abs := filepath.Join(workspaceDir, wf)
		data, err := os.ReadFile(abs)
		if err != nil || len(strings.TrimSpace(string(data))) == 0 {
			res.Passed = false
			res.MissingArtifacts = append(res.MissingArtifacts, "/workspace/"+wf)
			continue
		}
		var parsed interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			res.Passed = false
			res.MissingArtifacts = append(res.MissingArtifacts, fmt.Sprintf("/workspace/%s(invalid JSON)", wf))
		} else if m, ok := parsed.(map[string]interface{}); ok && len(m) == 0 {
			res.Passed = false
			res.MissingArtifacts = append(res.MissingArtifacts, fmt.Sprintf("/workspace/%s(empty JSON object)", wf))
		} else if dsErr := verifyDatasetOrExperimentJSON(wf, data); dsErr != "" {
			res.Passed = false
			res.VacuousScripts = append(res.VacuousScripts, fmt.Sprintf("%s(%s)", wf, dsErr))
		}
	}

	// Stage 4: Bounded O(1) `p2p` Regression check on any discovered `benchmark_runner.py` + up to 4 most recent verified `.py` scripts.
	// A module only counts as a regression if it passed before this task and
	// fails after — module_selftest supplies the per-module baseline. With no
	// recorded pass, a failing module is already-failing: it is retired from
	// the regression set (its repair is its own task) and never blocks this one.
	if res.Passed && sandbox != nil {
		var regTargets []string
		runnerRel := ""
		for _, pkg := range discoverWorkspacePackageDirs(workspaceDir) {
			cand := filepath.ToSlash(filepath.Join(pkg, "benchmark_runner.py"))
			if _, err := os.Stat(filepath.Join(workspaceDir, cand)); err == nil {
				runnerRel = cand
				regTargets = append(regTargets, runnerRel)
				break
			}
		}
		for i := len(verifiedFiles) - 1; i >= 0 && len(regTargets) < 5; i-- {
			clean := normalizeRelPath(verifiedFiles[i])
			if !strings.HasSuffix(clean, ".py") || strings.HasSuffix(clean, "__init__.py") || clean == "workspace_audit.py" || clean == runnerRel {
				continue
			}
			// c_s22_harness's self-test asserts a shipped snapshot against the
			// live tree — it fails regardless of what any task changes, so it
			// is not regression evidence. Out of the set until that is fixed.
			if clean == "modules/c_s22_harness.py" {
				continue
			}
			if eng != nil && eng.moduleSelftestPrior(ctx, clean) == "retired" {
				continue
			}
			alreadyInWs := false
			for _, wf := range wsFiles {
				if wf == clean {
					alreadyInWs = true
					break
				}
			}
			if alreadyInWs {
				continue
			}
			if _, err := os.Stat(filepath.Join(workspaceDir, clean)); err == nil {
				regTargets = append(regTargets, clean)
			}
		}
		for _, clean := range regTargets {
			prior := ""
			if eng != nil {
				prior = eng.moduleSelftestPrior(ctx, clean)
			}
			execRes, err := sandbox.Exec(ctx, fmt.Sprintf("python3 /workspace/%s --selftest", clean))
			failed := err != nil || execRes.ExitCode != 0
			if !failed {
				if eng != nil {
					eng.recordModuleSelftest(ctx, clean, "pass")
				}
				continue
			}
			if prior == "pass" {
				eng.recordModuleSelftest(ctx, clean, "fail")
				res.Passed = false
				res.RegressedScripts = append(res.RegressedScripts, clean)
				continue
			}
			// No recorded pass before this task — it was already failing.
			// Retire it from the regression set rather than blocking the task.
			if eng != nil {
				eng.recordModuleSelftest(ctx, clean, "retired")
				eng.NotifyOwner(ctx, "digest", "module "+clean+" fails its self-test independently of any task — retired from the regression set; it needs its own repair")
			}
		}
	}

	return finalizeMode3GateReason(res)
}

// moduleSelftestPrior returns the last recorded selftest status for a
// module ("" = never observed, "pass", "fail", "retired").
func (e *Engine) moduleSelftestPrior(ctx context.Context, module string) string {
	db := e.getDB()
	if db == nil {
		return ""
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM module_selftest WHERE module = ?`, module).Scan(&status); err != nil {
		return ""
	}
	return status
}

// recordModuleSelftest stores the latest selftest outcome per module — the
// baseline the p2p regression check compares against.
func (e *Engine) recordModuleSelftest(ctx context.Context, module, status string) {
	db := e.getDB()
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `INSERT INTO module_selftest (module, status, checked_at) VALUES (?,?,?)
		ON CONFLICT(module) DO UPDATE SET status=excluded.status, checked_at=excluded.checked_at`,
		module, status, time.Now().UTC().Format(time.RFC3339))
}

func finalizeMode3GateReason(res Mode3GateResult) Mode3GateResult {
	if !res.Passed {
		var reasons []string
		if len(res.VacuousScripts) > 0 {
			reasons = append(reasons, "Substance/Dataset/Hypothesis Gate failed: "+strings.Join(res.VacuousScripts, "; "))
		}
		if len(res.FailingScripts) > 0 {
			reasons = append(reasons, "Failing --selftest: "+strings.Join(res.FailingScripts, "; "))
		}
		if len(res.HygieneViolations) > 0 {
			reasons = append(reasons, "AST-PDG Hygiene violation: "+strings.Join(res.HygieneViolations, "; "))
		}
		if len(res.RegressedScripts) > 0 {
			reasons = append(reasons, "p2p Regression in: "+strings.Join(res.RegressedScripts, ", "))
		}
		res.Reason = strings.Join(reasons, " | ")
	} else {
		res.Reason = "All Mode-3 dataset provenance, hypothesis falsification, --selftest, AST-PDG hygiene, and p2p regression checks passed."
	}
	return res
}

func verifyDatasetOrExperimentJSON(relPath string, data []byte) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		var arr []interface{}
		if errArr := json.Unmarshal(data, &arr); errArr != nil {
			return "invalid JSON syntax: " + err.Error()
		}
		if len(arr) < 5 {
			return fmt.Sprintf("JSON array has only %d items (minimum 5 real records required)", len(arr))
		}
		return ""
	}

	norm := filepath.ToSlash(relPath)
	if strings.HasPrefix(norm, "datasets/") {
		if _, ok := parsed["dataset_id"]; !ok {
			return "dataset JSON missing required 'dataset_id' field"
		}
		if _, ok := parsed["provenance"]; !ok {
			if _, ok2 := parsed["source_db"]; !ok2 {
				return "dataset JSON missing required 'provenance' or 'source_db' field"
			}
		}
		trainSplit, hasTrain := parsed["train_split"].([]interface{})
		heldOutSplit, hasHeldOut := parsed["held_out_split"].([]interface{})
		if hasTrain && hasHeldOut {
			if len(trainSplit) < 5 || len(heldOutSplit) < 5 {
				return fmt.Sprintf("dataset splits too small: train_split=%d, held_out_split=%d (each must have >= 5 real records)", len(trainSplit), len(heldOutSplit))
			}
		} else if records, hasRecs := parsed["records"].([]interface{}); hasRecs {
			if len(records) < 10 {
				return fmt.Sprintf("dataset 'records' has only %d items (minimum 10 real records required)", len(records))
			}
		} else {
			return "dataset JSON must contain either ('train_split' and 'held_out_split' arrays) or a 'records' array (>=10 real items)"
		}
	}

	if norm == "registry.json" {
		if ds, ok := parsed["datasets"].([]interface{}); ok {
			if len(ds) == 0 {
				return "registry.json has empty 'datasets' list"
			}
		} else if dsMap, ok := parsed["datasets"].(map[string]interface{}); ok {
			if len(dsMap) == 0 {
				return "registry.json has empty 'datasets' map"
			}
		} else {
			return "registry.json must contain a non-empty 'datasets' list or map"
		}
	}

	falseKeys := []string{"h1_accepted", "hypothesis_accepted", "gate_passed", "overall_pass", "target_met"}
	for _, k := range falseKeys {
		if v, exists := parsed[k]; exists {
			if b, isBool := v.(bool); isBool && !b {
				return fmt.Sprintf("SCIENTIFIC FALSIFICATION: %q is false in %s (H0 was NOT rejected; you must iterate/improve the architecture or revert the failed mechanism rather than checking off a failed hypothesis!)", k, relPath)
			}
		}
	}
	for _, k := range []string{"decision", "verdict", "hypothesis_verdict"} {
		if v, exists := parsed[k]; exists {
			if s, isStr := v.(string); isStr {
				sl := strings.ToLower(strings.TrimSpace(s))
				if sl == "h0_not_rejected" || sl == "reject_h1" || sl == "fail" || sl == "failed" || sl == "revert" {
					return fmt.Sprintf("SCIENTIFIC FALSIFICATION: %s=%q in %s (do not check off milestone when hypothesis is falsified; refine the mechanism until empirical held-out evaluation succeeds!)", k, s, relPath)
				}
			}
		}
	}

	return ""
}

func checkPythonASTNonVacuity(src string) string {
	if !strings.Contains(src, "--selftest") && !strings.Contains(src, "selftest") {
		return "missing --selftest support"
	}
	if strings.Contains(src, "assert True") {
		return "contains vacuous `assert True`"
	}
	for _, bannedRng := range []string{"random.uniform(", "random.gauss(", "random.normalvariate(", "np.random."} {
		if strings.Contains(src, bannedRng) {
			return fmt.Sprintf("contains synthetic random number generator %q (all datasets and benchmarks must use real SQLite traces or corpus data, never synthetic random floats)", bannedRng)
		}
	}
	if !strings.Contains(src, "assert ") && !strings.Contains(src, "assert(") && !strings.Contains(src, "assertEqual") && !strings.Contains(src, "raise ") && !strings.Contains(src, "sys.exit(1)") && !strings.Contains(src, "sys.exit(2)") {
		return "selftest contains no real assertions or non-zero failure exits"
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
