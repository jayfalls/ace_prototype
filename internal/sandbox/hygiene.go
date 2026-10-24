package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ChangeControlLedger tracks active vs retired conceptual flows and files across the workspace.
type ChangeControlLedger struct {
	Version                string   `json:"version"`
	UpdatedAt              string   `json:"updated_at"`
	ActiveFlows            []string `json:"active_flows"`
	RetiredFlows           []string `json:"retired_flows"`
	RetiredConceptKeywords []string `json:"retired_concept_keywords"`
}

func (j *Jail) initWorkspaceHygiene() {
	j.QuarantineWorkspaceClutter()
	j.seedHarnessD()
	j.seedWorkspaceAuditModule()
	j.InitWorkspaceGit()
}

// QuarantineWorkspaceClutter moves retired v0.1 B2B SaaS and scratch files into /workspace/archive/v0_1_legacy/
// and removes shadow Wiki files (notes/goals.md, read.md) so L6 never mistakes dead code/docs for live ones.
func (j *Jail) QuarantineWorkspaceClutter() int {
	archiveDir := filepath.Join(j.workspaceDir, "archive", "v0_1_legacy")
	_ = os.MkdirAll(archiveDir, 0o755)

	keepRoot := map[string]bool{
		"README.md":                 true,
		"workspace_audit.py":        true,
		"ace_v1_traces.db":          true,
		"corpus_synthesizer.py":     true,
		"literature_grounding.json": true,
		"v1_forensics.py":           true,
		"v1_forensics_report.json":  true,
		"drift_detector.py":         true,
		"accept_phase15.py":         true,
		"model-viz.html":            true,
	}
	keepDirs := map[string]bool{
		"bin":        true,
		"datasets":   true,
		"results":    true,
		"modules":    true,
		"corpus":     true,
		"skills":     true,
		"tools":      true,
		"archive":    true,
		"showcase":   true,
		".harness_d": true,
		".tickets":   true,
		".kb":        true,
		".spillover": true,
		".git":       true,
	}

	moved := 0
	entries, err := os.ReadDir(j.workspaceDir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				if keepDirs[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "ace_") {
					continue
				}
				// Keep any Python package or module directory dynamically (domain & version agnostic)
				if _, errInit := os.Stat(filepath.Join(j.workspaceDir, name, "__init__.py")); errInit == nil {
					continue
				}
				src := filepath.Join(j.workspaceDir, name)
				dst := filepath.Join(archiveDir, name)
				if err := os.Rename(src, dst); err == nil {
					moved++
				}
				continue
			}
			if keepRoot[name] || name == ".gitignore" || strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".md") {
				continue
			}
			src := filepath.Join(j.workspaceDir, name)
			dst := filepath.Join(archiveDir, name)
			if err := os.Rename(src, dst); err == nil {
				moved++
			}
		}
	}

	// Clean shadow Wiki files (wiki/notes/goals.md, wiki/read.md)
	for _, shadow := range []string{
		filepath.Join(j.wikiDir, "notes", "goals.md"),
		filepath.Join(j.wikiDir, "read.md"),
	} {
		if info, err := os.Stat(shadow); err == nil && !info.IsDir() {
			dst := filepath.Join(archiveDir, "wiki_"+filepath.Base(shadow))
			_ = os.Rename(shadow, dst)
		}
	}
	return moved
}

// ScrubEphemeralScrap removes untracked scratch files (.tmp*, *.bak*, *.log, *.out, *.err) after each cycle.
func (j *Jail) ScrubEphemeralScrap() int {
	removed := 0
	entries, err := os.ReadDir(j.workspaceDir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".tmp") ||
			strings.Contains(name, ".bak") ||
			strings.HasSuffix(name, ".log") ||
			strings.HasSuffix(name, ".out") ||
			strings.HasSuffix(name, ".err") ||
			strings.HasSuffix(name, ".pyc") {
			_ = os.Remove(filepath.Join(j.workspaceDir, name))
			removed++
		}
	}
	return removed
}

func (j *Jail) seedHarnessD() {
	hd := filepath.Join(j.workspaceDir, ".harness_d")
	_ = os.MkdirAll(filepath.Join(hd, "traces"), 0o755)

	sbPath := filepath.Join(hd, "scoreboard.tsv")
	if _, err := os.Stat(sbPath); err != nil {
		header := "cycle_id\ttimestamp\tmilestone\tmodel\tprompt_tokens\tcached_tokens\tcache_hit_pct\tlatency_ms\tpassed\n"
		_ = os.WriteFile(sbPath, []byte(header), 0o644)
	}

	paretoPath := filepath.Join(hd, "pareto.json")
	if _, err := os.Stat(paretoPath); err != nil {
		seedPareto := map[string]interface{}{
			"version":    "ACE 6-Layer Runtime",
			"updated_at": time.Now().UTC().Format(time.RFC3339),
			"frontier":   []interface{}{},
		}
		b, _ := json.MarshalIndent(seedPareto, "", "  ")
		_ = os.WriteFile(paretoPath, b, 0o644)
	}

	ccPath := filepath.Join(hd, "change_control.json")
	if _, err := os.Stat(ccPath); err != nil {
		ledger := ChangeControlLedger{
			Version:                "ACE 6-Layer Runtime",
			UpdatedAt:              time.Now().UTC().Format(time.RFC3339),
			ActiveFlows:            []string{"workspace_audit.py"},
			RetiredFlows:           []string{},
			RetiredConceptKeywords: []string{},
		}
		b, _ := json.MarshalIndent(ledger, "", "  ")
		_ = os.WriteFile(ccPath, b, 0o644)
	}
}

func (j *Jail) InitWorkspaceGit() {
	gitignorePath := filepath.Join(j.workspaceDir, ".gitignore")
	gitignore := strings.Join([]string{
		"__pycache__/",
		".spillover/",
		".tickets/",
		".kb/",
		".harness_d/",
		"archive/",
		"corpus/",
		"*.db",
		"*.pyc",
		"*.log",
		"*.out",
		"*.err",
		"*.bak*",
		"*.tmp*",
		".tmp*",
		"",
	}, "\n")
	_ = os.WriteFile(gitignorePath, []byte(gitignore), 0o644)

	gitDir := filepath.Join(j.workspaceDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		_ = j.runHostGit("init", "-q")
		_ = j.runHostGit("config", "user.email", "ace@local.sandbox")
		_ = j.runHostGit("config", "user.name", "ACE v0.2 Runtime")
		_ = j.runHostGit("add", ".")
		_ = j.runHostGit("commit", "-q", "-m", "[unit: baseline] initialize clean ACE v0.2 workspace", "--allow-empty")
	}
}

func (j *Jail) CommitMilestoneArtifacts(milestone string, files []string) {
	j.ScrubEphemeralScrap()
	if len(files) > 0 {
		args := append([]string{"add", "--"}, files...)
		_ = j.runHostGit(args...)
	} else {
		_ = j.runHostGit("add", ".")
	}
	msg := fmt.Sprintf("[unit: milestone] %s", strings.TrimSpace(milestone))
	if len(msg) > 120 {
		msg = msg[:120]
	}
	_ = j.runHostGit("commit", "-q", "-m", msg)
}

func (j *Jail) runHostGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = j.workspaceDir
	return cmd.Run()
}

// seedWorkspaceAuditModule writes the minimal AST-PDG conceptual-flow and
// SWE-mutation audit module used by the Mode-3 gate. Statistical functions
// (beta CI, McNemar, Wilcoxon) are intentionally absent: they are being
// re-implemented and validated in Go under internal/science (v0.8 slice 5).
func (j *Jail) seedWorkspaceAuditModule() {
	_ = os.MkdirAll(filepath.Join(j.workspaceDir, "datasets"), 0o755)
	_ = os.MkdirAll(filepath.Join(j.workspaceDir, "results"), 0o755)
	_ = os.MkdirAll(filepath.Join(j.workspaceDir, "modules"), 0o755)
	auditPath := filepath.Join(j.workspaceDir, "workspace_audit.py")
	auditCode := `#!/usr/bin/env python3
"""Minimal workspace audit module: 4-pass AST-PDG conceptual flow hygiene + SWE-mutation falsification."""
import ast
import json
import os
import re
import subprocess
import sys
import tempfile
from typing import Any


def audit_script_mutants(script_path: str, max_mutants: int = 6) -> dict[str, Any]:
    """SWE-Mutation style AST single-site mutation tester running '--selftest' against generated mutants."""
    with open(script_path, "r", encoding="utf-8") as f:
        src = f.read()
    lines = src.splitlines()
    swap_rules = [
        (" >= ", " < "),
        (" <= ", " > "),
        (" == ", " != "),
        (" > ", " <= "),
        (" < ", " >= "),
        (" + ", " - "),
        ("True", "False"),
    ]
    all_candidates: list[tuple[int, str, str]] = []
    in_selftest = False
    for idx, line in enumerate(lines):
        stripped = line.strip()
        if stripped.startswith("#") or stripped.startswith("import ") or stripped.startswith("from "):
            continue
        if "def selftest" in stripped or "def _selftest" in stripped or "--selftest" in stripped:
            in_selftest = True
        if in_selftest and stripped.startswith("def ") and "selftest" not in stripped:
            in_selftest = False
        if in_selftest:
            continue
        for old_op, new_op in swap_rules:
            if old_op in line and "__name__" not in line and "argparse" not in line:
                mut_line = line.replace(old_op, new_op, 1)
                all_candidates.append((idx + 1, f"{old_op.strip()}->{new_op.strip()}", mut_line))
                break
    candidates = list(reversed(all_candidates))[:max_mutants]

    json_backups: dict[str, bytes] = {}
    if candidates:
        for bdir in ("/workspace/datasets", "/workspace/results", "/workspace/skills"):
            if os.path.isdir(bdir):
                for fn in os.listdir(bdir):
                    if fn.endswith(".json"):
                        jp = os.path.join(bdir, fn)
                        try:
                            with open(jp, "rb") as jf:
                                json_backups[jp] = jf.read()
                        except Exception:
                            pass

    killed = 0
    survived = 0
    details = []
    try:
        for line_no, desc, mut_line in candidates:
            mut_lines = list(lines)
            mut_lines[line_no - 1] = mut_line
            mut_src = "\n".join(mut_lines) + "\n"
            with tempfile.NamedTemporaryFile("w", suffix=".py", delete=False, encoding="utf-8") as tf:
                tf.write(mut_src)
                tmp_path = tf.name
            try:
                proc = subprocess.run(
                    [sys.executable, tmp_path, "--selftest"],
                    capture_output=True,
                    text=True,
                    timeout=8,
                )
                is_killed = proc.returncode != 0
            except subprocess.TimeoutExpired:
                is_killed = True
            finally:
                try:
                    os.remove(tmp_path)
                except OSError:
                    pass
            if is_killed:
                killed += 1
            else:
                survived += 1
            details.append({"line": line_no, "mutation": desc, "killed": is_killed})
            if max_mutants <= 2 and killed >= 1:
                break
            if killed >= 2:
                break
    finally:
        for jp, raw_b in json_backups.items():
            try:
                cur_b = open(jp, "rb").read() if os.path.exists(jp) else b""
                if cur_b != raw_b:
                    tmp_jp = f"{jp}.restore.{os.getpid()}"
                    with open(tmp_jp, "wb") as wf:
                        wf.write(raw_b)
                    os.replace(tmp_jp, jp)
            except Exception:
                pass

    total = len(details) if details else len(candidates)
    rate = (killed / total) if total > 0 else 1.0
    return {
        "script": os.path.basename(script_path),
        "total_mutants": total,
        "killed": killed,
        "survived": survived,
        "mutant_kill_rate": round(rate, 4),
        "details": details,
    }


def audit_conceptual_flows(target_path: str = "/workspace") -> dict[str, Any]:
    """
    4-Pass Cross-File AST-PDG Conceptual Flow & Zombie Subgraph Analyzer:
    - Pass A (test_only_zombies): functions reachable only from test entrypoints.
    - Pass B (unused_imports & shadowed dict writes).
    - Pass C (competing_twin_flows): overlapping SQL tables + output schemas (Jaccard >= 0.65).
    - Pass D (except_fallback_shims): functions invoked only inside broad except blocks.
    """
    py_files: list[str] = []
    if os.path.isfile(target_path) and target_path.endswith(".py"):
        py_files = [target_path]
    elif os.path.isdir(target_path):
        for root, dirs, files in os.walk(target_path):
            dirs[:] = [d for d in dirs if not d.startswith(".") and d not in ("archive", "__pycache__", "bin")]
            for fn in sorted(files):
                if fn.endswith(".py"):
                    py_files.append(os.path.join(root, fn))

    unused_imports: list[dict[str, Any]] = []
    test_only_zombies: list[dict[str, Any]] = []
    shadowed_writes: list[dict[str, Any]] = []
    competing_twins: list[dict[str, Any]] = []
    except_shims: list[dict[str, Any]] = []

    cross_file_imported_modules: set[str] = set()
    parsed_trees: dict[str, ast.AST] = {}
    for fp in py_files:
        try:
            with open(fp, "r", encoding="utf-8") as f:
                tree = ast.parse(f.read(), filename=fp)
            parsed_trees[fp] = tree
            for node in ast.walk(tree):
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        cross_file_imported_modules.add(alias.name.split(".")[0])
                elif isinstance(node, ast.ImportFrom) and node.module:
                    cross_file_imported_modules.add(node.module.split(".")[0])
        except Exception:
            continue

    for fp, tree in parsed_trees.items():
        mod_name = os.path.splitext(os.path.basename(fp))[0]
        parent_dir = os.path.dirname(fp)
        is_pkg_dir = os.path.exists(os.path.join(parent_dir, "__init__.py"))
        is_shared_lib = mod_name in cross_file_imported_modules or is_pkg_dir

        imported_names: dict[str, int] = {}
        used_names: set[str] = set()
        if os.path.basename(fp) != "__init__.py":
            for node in ast.walk(tree):
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        bound = alias.asname or alias.name.split(".")[0]
                        imported_names[bound] = getattr(node, "lineno", 1)
                elif isinstance(node, ast.ImportFrom):
                    for alias in node.names:
                        if alias.name != "*":
                            bound = alias.asname or alias.name
                            imported_names[bound] = getattr(node, "lineno", 1)
                elif isinstance(node, ast.Name) and isinstance(node.ctx, ast.Load):
                    used_names.add(node.id)
            for name, lineno in imported_names.items():
                if name not in used_names:
                    unused_imports.append({"file": os.path.basename(fp), "symbol": name, "line": lineno})

        funcs: dict[str, ast.FunctionDef] = {}
        for node in getattr(tree, "body", []):
            if isinstance(node, ast.FunctionDef):
                funcs[node.name] = node

        callees_by_fn: dict[str, set[str]] = {}
        except_only_callees: set[str] = set()
        outside_except_callees: set[str] = set()
        io_fingerprints: dict[str, tuple[set[str], set[str]]] = {}

        for fn_name, fn_node in funcs.items():
            callees: set[str] = set()
            sql_tables: set[str] = set()
            dict_keys: set[str] = set()

            seen_assign_keys: dict[tuple[str, str], int] = {}
            for stmt in fn_node.body:
                if isinstance(stmt, ast.Assign) and len(stmt.targets) == 1:
                    t = stmt.targets[0]
                    if isinstance(t, ast.Subscript) and isinstance(t.value, ast.Name) and isinstance(t.slice, ast.Constant) and isinstance(t.slice.value, str):
                        pair = (t.value.id, t.slice.value)
                        rhs_names = {n.id for n in ast.walk(stmt.value) if isinstance(n, ast.Name)}
                        if pair in seen_assign_keys and t.value.id not in rhs_names:
                            shadowed_writes.append({
                                "file": os.path.basename(fp),
                                "function": fn_name,
                                "dict_key": f"{pair[0]}['{pair[1]}']",
                                "first_line": seen_assign_keys[pair],
                                "overwritten_line": stmt.lineno,
                            })
                        seen_assign_keys[pair] = stmt.lineno

            for sub in ast.walk(fn_node):
                if isinstance(sub, ast.Call):
                    if isinstance(sub.func, ast.Name):
                        callees.add(sub.func.id)
                elif isinstance(sub, ast.Constant) and isinstance(sub.value, str):
                    for m in re.findall(r"\bFROM\s+([a-zA-Z_][a-zA-Z0-9_]*)", sub.value, flags=re.IGNORECASE):
                        sql_tables.add(m.lower())
                elif isinstance(sub, ast.Dict):
                    for k in sub.keys:
                        if isinstance(k, ast.Constant) and isinstance(k.value, str):
                            dict_keys.add(k.value)
            callees_by_fn[fn_name] = callees
            if sql_tables and len(dict_keys) >= 2:
                io_fingerprints[fn_name] = (sql_tables, dict_keys)

            in_except_nodes: set[int] = set()
            for sub in ast.walk(fn_node):
                if isinstance(sub, ast.ExceptHandler):
                    for child in ast.walk(sub):
                        in_except_nodes.add(id(child))
            for sub in ast.walk(fn_node):
                if isinstance(sub, ast.Call) and isinstance(sub.func, ast.Name) and sub.func.id in funcs:
                    if id(sub) in in_except_nodes:
                        except_only_callees.add(sub.func.id)
                    else:
                        outside_except_callees.add(sub.func.id)

        for cand in except_only_callees - outside_except_callees:
            except_shims.append({"file": os.path.basename(fp), "function": cand})

        if not is_shared_lib and funcs:
            test_roots = {f for f in funcs if "test" in f.lower()}
            prod_roots = {f for f in funcs if f in ("main", "run", "cli") or (f not in test_roots and not f.startswith("_"))}
            for stmt in getattr(tree, "body", []):
                if isinstance(stmt, ast.FunctionDef):
                    continue
                for sub in ast.walk(stmt):
                    if isinstance(sub, ast.Call) and isinstance(sub.func, ast.Name) and sub.func.id in funcs:
                        if "test" not in sub.func.id.lower():
                            prod_roots.add(sub.func.id)

            r_prod: set[str] = set()
            stack = list(prod_roots - test_roots)
            while stack:
                cur = stack.pop()
                if cur in r_prod:
                    continue
                r_prod.add(cur)
                for nxt in callees_by_fn.get(cur, set()):
                    if nxt in funcs and nxt not in r_prod and nxt not in test_roots:
                        stack.append(nxt)

            r_test: set[str] = set()
            tstack = list(test_roots)
            while tstack:
                cur = tstack.pop()
                if cur in r_test:
                    continue
                r_test.add(cur)
                for nxt in callees_by_fn.get(cur, set()):
                    if nxt in funcs and nxt not in r_test:
                        tstack.append(nxt)

            for z in sorted((r_test - r_prod) - test_roots):
                test_only_zombies.append({"file": os.path.basename(fp), "function": z})

        fn_list = sorted(io_fingerprints.keys())
        for i in range(len(fn_list)):
            for j_idx in range(i + 1, len(fn_list)):
                f1, f2 = fn_list[i], fn_list[j_idx]
                t1, k1 = io_fingerprints[f1]
                t2, k2 = io_fingerprints[f2]
                if t1 & t2:
                    inter = len(k1 & k2)
                    union = len(k1 | k2)
                    jacc = (inter / union) if union > 0 else 0.0
                    if jacc >= 0.65 and inter >= 3:
                        competing_twins.append({
                            "file": os.path.basename(fp),
                            "functions": [f1, f2],
                            "shared_tables": sorted(t1 & t2),
                            "schema_jaccard": round(jacc, 4),
                        })

    clean = (
        len(unused_imports) == 0
        and len(test_only_zombies) == 0
        and len(shadowed_writes) == 0
        and len(competing_twins) == 0
        and len(except_shims) == 0
    )
    return {
        "clean": clean,
        "unused_imports": unused_imports,
        "test_only_zombies": test_only_zombies,
        "shadowed_writes": shadowed_writes,
        "competing_twin_flows": competing_twins,
        "except_fallback_shims": except_shims,
    }


if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else "/workspace"
    print(json.dumps(audit_conceptual_flows(target), indent=2))
`
	_ = os.WriteFile(auditPath, []byte(auditCode), 0o755)
}
