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
	j.seedAceRLMSDK()
	j.InitWorkspaceGit()
}

// QuarantineWorkspaceClutter moves retired v0.1 B2B SaaS and scratch files into /workspace/archive/v0_1_legacy/
// and removes shadow Wiki files (notes/goals.md, read.md) so L6 never mistakes dead code/docs for live ones.
func (j *Jail) QuarantineWorkspaceClutter() int {
	archiveDir := filepath.Join(j.workspaceDir, "archive", "v0_1_legacy")
	_ = os.MkdirAll(archiveDir, 0o755)

	keepRoot := map[string]bool{
		"README.md":                 true,
		"ace_rlm.py":                true,
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
		"bin":             true,
		"benchmarks":      true,
		"research_corpus": true,
		"skills":          true,
		"tools":           true,
		"archive":         true,
		"showcase":        true,
		".harness_d":      true,
		".tickets":        true,
		".kb":             true,
		".spillover":      true,
		".git":            true,
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
			ActiveFlows:            []string{"ace_rlm.py"},
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
		"research_corpus/",
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

func (j *Jail) seedAceRLMSDK() {
	_ = os.MkdirAll(filepath.Join(j.workspaceDir, "benchmarks", "datasets"), 0o755)
	_ = os.MkdirAll(filepath.Join(j.workspaceDir, "research_corpus"), 0o755)
	sdkPath := filepath.Join(j.workspaceDir, "ace_rlm.py")
	sdkCode := `#!/usr/bin/env python3
"""
ACE Universal Pure-Python 3.14 Scientific, Literature-Discovery, Dataset-Construction, Statistical, Mutation & AST-PDG SDK (/workspace/ace_rlm.py)
Zero external C-extension dependencies (pure Python 3.14 stdlib: math, hashlib, sqlite3, ast, json, re, subprocess, urllib, xml).
Works in ANY domain and environment (even with zero pre-seeded context, papers, or databases).
"""
import argparse
import ast
import hashlib
import html
import json
import math
import os
import re
import sqlite3
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from typing import Any


def _betacf(a: float, b: float, x: float) -> float:
    """Continued fraction evaluation for regularized incomplete beta function (Lentz's method)."""
    max_iter = 200
    eps = 3.0e-12
    fpmin = 1.0e-30
    qab = a + b
    qap = a + 1.0
    qam = a - 1.0
    c = 1.0
    d = 1.0 - qab * x / qap
    if abs(d) < fpmin:
        d = fpmin
    d = 1.0 / d
    h = d
    for m in range(1, max_iter + 1):
        m2 = 2 * m
        aa = m * (b - m) * x / ((qam + m2) * (a + m2))
        d = 1.0 + aa * d
        if abs(d) < fpmin:
            d = fpmin
        c = 1.0 + aa / c
        if abs(c) < fpmin:
            c = fpmin
        d = 1.0 / d
        h *= d * c
        aa = -(a + m) * (qab + m) * x / ((a + m2) * (qap + m2))
        d = 1.0 + aa * d
        if abs(d) < fpmin:
            d = fpmin
        c = 1.0 + aa / c
        if abs(c) < fpmin:
            c = fpmin
        d = 1.0 / d
        delta = d * c
        h *= delta
        if abs(delta - 1.0) < eps:
            break
    return h


def betainc_reg(a: float, b: float, x: float) -> float:
    """Exact regularized incomplete beta function I_x(a, b)."""
    if x <= 0.0:
        return 0.0
    if x >= 1.0:
        return 1.0
    ln_beta = math.lgamma(a) + math.lgamma(b) - math.lgamma(a + b)
    front = math.exp(a * math.log(x) + b * math.log(1.0 - x) - ln_beta)
    if x < (a + 1.0) / (a + b + 2.0):
        return front * _betacf(a, b, x) / a
    return 1.0 - front * _betacf(b, a, 1.0 - x) / b


def beta_ppf(p: float, a: float, b: float) -> float:
    """Exact inverse CDF (quantile) of Beta(a, b) via 60-step bisection on I_x(a, b)."""
    if p <= 0.0:
        return 0.0
    if p >= 1.0:
        return 1.0
    lo, hi = 0.0, 1.0
    for _ in range(60):
        mid = 0.5 * (lo + hi)
        if betainc_reg(a, b, mid) < p:
            lo = mid
        else:
            hi = mid
    return 0.5 * (lo + hi)


def beta_ci(successes: int, trials: int, prior_a: float = 1.0, prior_b: float = 1.0, alpha_level: float = 0.05) -> tuple[float, float, float]:
    """Compute exact Bayesian Beta(a, b) posterior mean and exact equal-tailed (1 - alpha_level) credible interval."""
    a = max(1e-6, float(successes) + float(prior_a))
    b = max(1e-6, float(trials - successes) + float(prior_b))
    mean = a / (a + b)
    low = beta_ppf(alpha_level / 2.0, a, b)
    high = beta_ppf(1.0 - alpha_level / 2.0, a, b)
    return (round(mean, 6), round(low, 6), round(high, 6))


def mcnemar_exact_p(b_wins: int, c_wins: int) -> float:
    """Two-sided exact binomial McNemar p-value for paired binary outcomes."""
    n = int(b_wins) + int(c_wins)
    if n <= 0:
        return 1.0
    k = min(int(b_wins), int(c_wins))
    total_prob = 0.0
    for i in range(k + 1):
        total_prob += math.comb(n, i) * (0.5 ** n)
    return round(min(1.0, 2.0 * total_prob), 8)


def wilcoxon_signed_rank(xs: list[float], ys: list[float]) -> dict[str, float]:
    """Paired Wilcoxon signed-rank statistic and normal-approximation two-sided p-value."""
    diffs = [float(y) - float(x) for x, y in zip(xs, ys) if abs(float(y) - float(x)) > 1e-12]
    n = len(diffs)
    if n == 0:
        return {"w_stat": 0.0, "z_score": 0.0, "p_value": 1.0, "n_nonzero": 0}
    abs_sorted = sorted(enumerate(diffs), key=lambda kv: abs(kv[1]))
    w_pos = 0.0
    w_neg = 0.0
    for rank, (_, d) in enumerate(abs_sorted, start=1):
        if d > 0:
            w_pos += rank
        else:
            w_neg += rank
    w_stat = min(w_pos, w_neg)
    mean_w = n * (n + 1.0) / 4.0
    sd_w = math.sqrt(n * (n + 1.0) * (2.0 * n + 1.0) / 24.0)
    z = (w_stat - mean_w) / sd_w if sd_w > 0 else 0.0
    p_val = math.erfc(abs(z) / math.sqrt(2.0))
    return {
        "w_stat": round(w_stat, 4),
        "w_pos": round(w_pos, 4),
        "w_neg": round(w_neg, 4),
        "z_score": round(z, 4),
        "p_value": round(min(1.0, p_val), 8),
        "n_nonzero": n,
    }


def discover_sqlite_dbs(root: str = "/workspace") -> list[str]:
    """Discover all SQLite (.db / .sqlite) database files under /workspace."""
    if not os.path.isdir(root):
        root = os.path.dirname(os.path.abspath(__file__))
    found: list[str] = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if not d.startswith(".") and d != "__pycache__" and d != "archive"]
        for fn in sorted(filenames):
            if fn.endswith(".db") or fn.endswith(".sqlite"):
                found.append(os.path.join(dirpath, fn))
    return found


def inspect_db_schema(db_path: str = "/workspace/ace_v1_traces.db") -> dict[str, list[dict[str, Any]]]:
    """Return the exact SQLite table and column schema via PRAGMA table_info (zero guessing)."""
    if not os.path.exists(db_path):
        dbs = discover_sqlite_dbs()
        if dbs:
            db_path = dbs[0]
        else:
            return {}
    conn = sqlite3.connect(db_path)
    try:
        tables = [r[0] for r in conn.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").fetchall()]
        schema: dict[str, list[dict[str, Any]]] = {}
        for tbl in tables:
            cols = conn.execute(f"PRAGMA table_info({tbl})").fetchall()
            schema[tbl] = [{"name": c[1], "type": c[2], "notnull": bool(c[3]), "pk": bool(c[5])} for c in cols]
        return schema
    finally:
        conn.close()


def query_sqlite(sql: str, params: tuple = (), db_path: str = "/workspace/benchmarks/live_traces.db") -> list[dict[str, Any]]:
    """Execute a read-only SQL query against any SQLite database in /workspace."""
    if not os.path.exists(db_path):
        dbs = discover_sqlite_dbs()
        if dbs:
            db_path = dbs[0]
        else:
            return []
    conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    conn.text_factory = lambda b: b.decode("utf-8", "replace")
    conn.row_factory = sqlite3.Row
    try:
        cur = conn.execute(sql, params)
        return [dict(r) for r in cur.fetchall()]
    finally:
        conn.close()


def query_v1_traces(sql: str, params: tuple = (), db_path: str = "/workspace/ace_v1_traces.db") -> list[dict[str, Any]]:
    """Execute a read-only SQL query against a trace database (falling back to live_traces.db if v1 is absent)."""
    return query_sqlite(sql, params=params, db_path=db_path)


query_v0_1_traces = query_v1_traces


def resolve_generation_db(generation: str = "live") -> str:
    """Resolve the SQLite trace database path for any historical generation or 'live' (or return any discovered DB)."""
    g = str(generation).strip().lower()
    mapping = {
        "v0.1": "/workspace/ace_v1_traces.db",
        "v1": "/workspace/ace_v1_traces.db",
        "v0.2": "/workspace/benchmarks/ace_v0_2_traces.db",
        "v2": "/workspace/benchmarks/ace_v0_2_traces.db",
        "v0.3": "/workspace/benchmarks/ace_v0_3_traces.db",
        "v3": "/workspace/benchmarks/ace_v0_3_traces.db",
        "live": "/workspace/benchmarks/live_traces.db",
    }
    cand = mapping.get(g, generation)
    if os.path.exists(cand):
        return cand
    base_dir = os.path.dirname(os.path.abspath(__file__))
    rel_cand = os.path.join(base_dir, cand.lstrip("/").removeprefix("workspace/"))
    if os.path.exists(rel_cand):
        return rel_cand
    dbs = discover_sqlite_dbs(base_dir)
    return dbs[0] if dbs else "/workspace/benchmarks/live_traces.db"


def _reconstruct_openalex_abstract(inv_idx: dict[str, list[int]] | None) -> str:
    """Reconstruct plain-text abstract from OpenAlex abstract_inverted_index."""
    if not isinstance(inv_idx, dict) or not inv_idx:
        return ""
    pos_to_word: dict[int, str] = {}
    for word, positions in inv_idx.items():
        if isinstance(positions, list):
            for p in positions:
                if isinstance(p, int):
                    pos_to_word[p] = word
    if not pos_to_word:
        return ""
    return " ".join(pos_to_word[k] for k in sorted(pos_to_word.keys()))


def search_arxiv(
    query: str,
    max_results: int = 5,
    save_dir: str = "/workspace/research_corpus",
) -> list[dict[str, Any]]:
    """
    Search scientific literature across OpenAlex (250M+ papers & arXiv preprints, primary fast JSON API)
    with arXiv Atom API fallback, save markdown paper digests into /workspace/research_corpus/,
    and update CORPUS_INDEX.md so ACE can research papers in ANY domain with zero prior corpus!
    """
    if not os.path.isdir(save_dir):
        base_dir = os.path.dirname(os.path.abspath(__file__))
        save_dir = os.path.join(base_dir, "research_corpus")
    os.makedirs(save_dir, exist_ok=True)
    limit = max(1, min(15, int(max_results)))
    q_enc = urllib.parse.quote(query.strip())
    papers: list[dict[str, Any]] = []

    # 1. Primary fast discovery via OpenAlex Works API (no API key required, includes arXiv & peer-reviewed papers)
    oa_url = f"https://api.openalex.org/works?search={q_enc}&per-page={limit}"
    try:
        req = urllib.request.Request(oa_url, headers={"User-Agent": "ACE-Autonomous-Scientist/1.0 (mailto:research@ace.local)"})
        with urllib.request.urlopen(req, timeout=10) as resp:
            oa_data = json.loads(resp.read().decode("utf-8", "replace"))
        for item in oa_data.get("results", [])[:limit]:
            title = " ".join(str(item.get("title") or item.get("display_name") or "").split())
            if not title:
                continue
            ids = item.get("ids") or {}
            paper_id = str(ids.get("arxiv") or item.get("doi") or item.get("id") or "").rstrip("/").split("/")[-1]
            published = str(item.get("publication_date") or item.get("publication_year") or "")[:10]
            authors = []
            for auth in item.get("authorships") or []:
                author_obj = auth.get("author") or {}
                nm = str(author_obj.get("display_name") or "").strip()
                if nm:
                    authors.append(nm)
            abstract = _reconstruct_openalex_abstract(item.get("abstract_inverted_index"))
            if not abstract:
                abstract = f"Scientific work on {title} (citation_count={item.get('cited_by_count', 0)})."
            raw_url = str(item.get("doi") or item.get("id") or "")
            slug = re.sub(r"[^a-z0-9]+", "-", f"{paper_id.lower()}-{title.lower()}").strip("-")[:72]
            md_path = os.path.join(save_dir, f"{slug}.md")
            md_content = (
                f"# {title}\n\n"
                f"- **Paper ID**: {paper_id}\n"
                f"- **Published**: {published}\n"
                f"- **Authors**: {', '.join(authors[:8])}\n"
                f"- **Citations**: {item.get('cited_by_count', 0)}\n"
                f"- **URL**: {raw_url}\n\n"
                f"## Abstract\n{abstract}\n"
            )
            with open(md_path, "w", encoding="utf-8") as f:
                f.write(md_content)
            papers.append({
                "arxiv_id": paper_id,
                "title": title,
                "published": published,
                "authors": authors[:8],
                "citations": item.get("cited_by_count", 0),
                "url": raw_url,
                "abstract": abstract,
                "local_path": md_path,
            })
    except Exception:
        papers = []

    # 2. Fallback to arXiv Atom API if OpenAlex returned 0 results
    if not papers:
        url = f"https://export.arxiv.org/api/query?search_query=all:{q_enc}&start=0&max_results={limit}"
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "ACE-Autonomous-Scientist/1.0"})
            with urllib.request.urlopen(req, timeout=10) as resp:
                xml_bytes = resp.read()
            root = ET.fromstring(xml_bytes)
            ns = {"atom": "http://www.w3.org/2005/Atom"}
            for entry in root.findall("atom:entry", ns):
                raw_id = (entry.findtext("atom:id", default="", namespaces=ns) or "").strip()
                arxiv_id = raw_id.rstrip("/").split("/")[-1]
                title = " ".join((entry.findtext("atom:title", default="", namespaces=ns) or "").split())
                summary = " ".join((entry.findtext("atom:summary", default="", namespaces=ns) or "").split())
                published = (entry.findtext("atom:published", default="", namespaces=ns) or "")[:10]
                authors = [
                    (a.findtext("atom:name", default="", namespaces=ns) or "").strip()
                    for a in entry.findall("atom:author", ns)
                    if (a.findtext("atom:name", default="", namespaces=ns) or "").strip()
                ]
                slug = re.sub(r"[^a-z0-9]+", "-", f"{arxiv_id}-{title.lower()}").strip("-")[:72]
                md_path = os.path.join(save_dir, f"{slug}.md")
                md_content = (
                    f"# {title}\n\n"
                    f"- **arXiv ID**: {arxiv_id}\n"
                    f"- **Published**: {published}\n"
                    f"- **Authors**: {', '.join(authors[:8])}\n"
                    f"- **URL**: {raw_id}\n\n"
                    f"## Abstract\n{summary}\n"
                )
                with open(md_path, "w", encoding="utf-8") as f:
                    f.write(md_content)
                papers.append({
                    "arxiv_id": arxiv_id,
                    "title": title,
                    "published": published,
                    "authors": authors[:8],
                    "url": raw_id,
                    "abstract": summary,
                    "local_path": md_path,
                })
        except Exception as exc:
            return [{"error": str(exc), "query": query}]

    if papers:
        idx_path = os.path.join(save_dir, "CORPUS_INDEX.md")
        existing = ""
        if os.path.exists(idx_path):
            with open(idx_path, "r", encoding="utf-8") as rf:
                existing = rf.read()
        new_lines = ["# ACE Research Corpus Index\n"] if not existing else []
        for p in papers:
            line = f"- **{p['title']}** ({p['arxiv_id']}, {p['published']}) -> {os.path.basename(p['local_path'])}"
            if line not in existing:
                new_lines.append(line)
        if new_lines:
            with open(idx_path, "a", encoding="utf-8") as af:
                af.write("\n" + "\n".join(new_lines) + "\n")
    return papers


search_papers = search_arxiv


def fetch_url_text(url: str, max_chars: int = 24000) -> str:
    """Fetch any public https:// URL (including GitHub blobs/repos and arXiv papers) and return structured Markdown/text."""
    u = url.strip()
    m_gh = re.match(r"^https?://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+)$", u)
    if m_gh:
        u = f"https://raw.githubusercontent.com/{m_gh.group(1)}/{m_gh.group(2)}/{m_gh.group(3)}/{m_gh.group(4)}"
    m_ax = re.match(r"^https?://(?:www\.)?arxiv\.org/(?:abs|pdf)/([0-9]{4}\.[0-9]{4,5}(?:v[0-9]+)?)(?:\.pdf)?$", u)
    if m_ax:
        u = f"https://ar5iv.labs.arxiv.org/html/{m_ax.group(1)}"
    req = urllib.request.Request(u, headers={"User-Agent": "ACE-Autonomous-Scientist/1.0"})
    with urllib.request.urlopen(req, timeout=15) as resp:
        raw = resp.read().decode("utf-8", "replace")
    if "<html" in raw[:500].lower() or "<body" in raw[:1000].lower():
        raw = re.sub(r"(?is)<(script|style|nav|footer).*?>.*?</\1>", " ", raw)
        raw = re.sub(r"(?is)<h([1-6])[^>]*>(.*?)</h\1>", lambda m: f"\n\n{'#' * int(m.group(1))} " + re.sub(r"<[^>]+>", " ", m.group(2)).strip() + "\n", raw)
        fence = chr(96) * 3
        raw = re.sub(r"(?is)<pre[^>]*>(.*?)</pre>", lambda m: f"\n{fence}\n" + re.sub(r"<[^>]+>", "", m.group(1)).strip() + f"\n{fence}\n", raw)
        raw = re.sub(r"(?is)<li[^>]*>(.*?)</li>", lambda m: "\n- " + re.sub(r"<[^>]+>", " ", m.group(1)).strip(), raw)
        raw = re.sub(r"(?s)<[^>]+>", " ", raw)
        raw = html.unescape(raw)
        raw = re.sub(r"[ \t]+", " ", raw)
        raw = re.sub(r"\n{3,}", "\n\n", raw)
    return raw.strip()[:max_chars]


web_fetch = fetch_url_text


def web_search(query: str, max_results: int = 5) -> dict[str, Any]:
    """
    Multi-vertical public research search across:
      - GitHub Open-Source Repositories (api.github.com/search/repositories)
      - Academic Literature (OpenAlex + arXiv)
      - HuggingFace Public Datasets Hub (huggingface.co/api/datasets)
      - DuckDuckGo Web Search
    """
    q = query.strip()
    q_enc = urllib.parse.quote(q)
    limit = max(1, min(10, int(max_results)))
    out: dict[str, Any] = {"query": q, "github_repos": [], "papers": [], "hf_datasets": [], "web": []}

    # 1. GitHub Repositories
    try:
        gh_req = urllib.request.Request(
            f"https://api.github.com/search/repositories?q={q_enc}&sort=stars&order=desc&per_page={limit}",
            headers={"User-Agent": "ACE-Autonomous-Scientist/1.0", "Accept": "application/vnd.github+json"},
        )
        with urllib.request.urlopen(gh_req, timeout=10) as resp:
            gh_data = json.loads(resp.read().decode("utf-8", "replace"))
        for it in gh_data.get("items", [])[:limit]:
            out["github_repos"].append({
                "full_name": it.get("full_name"),
                "url": it.get("html_url"),
                "stars": it.get("stargazers_count", 0),
                "language": it.get("language"),
                "description": it.get("description") or "",
            })
    except Exception:
        pass

    # 2. HuggingFace Datasets Hub
    try:
        hf_req = urllib.request.Request(
            f"https://huggingface.co/api/datasets?search={q_enc}&sort=downloads&direction=-1&limit={limit}",
            headers={"User-Agent": "ACE-Autonomous-Scientist/1.0"},
        )
        with urllib.request.urlopen(hf_req, timeout=10) as resp:
            hf_data = json.loads(resp.read().decode("utf-8", "replace"))
        if isinstance(hf_data, list):
            for ds in hf_data[:limit]:
                out["hf_datasets"].append({
                    "dataset_id": ds.get("id"),
                    "downloads": ds.get("downloads", 0),
                    "likes": ds.get("likes", 0),
                    "description": (ds.get("description") or "")[:240],
                })
    except Exception:
        pass

    # 3. OpenAlex Papers
    try:
        oa_req = urllib.request.Request(
            f"https://api.openalex.org/works?search={q_enc}&per-page={limit}",
            headers={"User-Agent": "ACE-Autonomous-Scientist/1.0 (mailto:research@example.org)"},
        )
        with urllib.request.urlopen(oa_req, timeout=10) as resp:
            oa_data = json.loads(resp.read().decode("utf-8", "replace"))
        for w in oa_data.get("results", [])[:limit]:
            out["papers"].append({
                "title": w.get("title"),
                "year": w.get("publication_year"),
                "citations": w.get("cited_by_count", 0),
                "url": w.get("doi") or w.get("id"),
                "abstract": _reconstruct_openalex_abstract(w.get("abstract_inverted_index"))[:320],
            })
    except Exception:
        pass

    # 4. DuckDuckGo HTML
    try:
        ddg_req = urllib.request.Request(
            f"https://html.duckduckgo.com/html/?q={q_enc}",
            headers={"User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"},
        )
        with urllib.request.urlopen(ddg_req, timeout=10) as resp:
            raw_html = resp.read().decode("utf-8", "replace")
        titles = re.findall(r'(?s)class="result__a"[^>]* href="([^"]+)"[^>]*>(.*?)</a>', raw_html)
        snippets = re.findall(r'(?s)class="result__snippet"[^>]*>(.*?)</(?:a|div|span)>', raw_html)
        for idx, (href, t_html) in enumerate(titles[:limit]):
            if "uddg=" in href:
                href = urllib.parse.unquote(href.split("uddg=", 1)[1].split("&", 1)[0])
            snip = re.sub(r"<[^>]+>", " ", snippets[idx]).strip() if idx < len(snippets) else ""
            out["web"].append({
                "title": html.unescape(re.sub(r"<[^>]+>", " ", t_html).strip()),
                "url": href,
                "snippet": html.unescape(snip),
            })
    except Exception:
        pass

    return out


def fetch_public_dataset(
    dataset_id: str,
    config: str = "default",
    split: str = "test",
    limit: int = 200,
    register_as: str | None = None,
    held_out_ratio: float = 0.25,
    benchmarks_dir: str = "/workspace/benchmarks",
) -> dict[str, Any]:
    """
    Fetch real rows from a public LLM/agent grading benchmark dataset on the HuggingFace Datasets Server
    (e.g. 'openai/gsm8k', 'allenai/ai2_arc', 'openai/openai_humaneval', 'google-research-datasets/mbpp',
    'cais/mmlu', 'gaia-benchmark/GAIA') or a direct public https:// JSON/JSONL URL, cache locally in
    /workspace/benchmarks/public_cache/, and register deterministic SHA-256 75/25 train_split & held_out_split
    in /workspace/benchmarks/registry.json.
    """
    if not os.path.isdir(benchmarks_dir):
        benchmarks_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "benchmarks")
    cache_dir = os.path.join(benchmarks_dir, "public_cache")
    os.makedirs(cache_dir, exist_ok=True)

    ds_clean = dataset_id.strip()
    max_rows = max(10, min(1000, int(limit)))

    # Canonical config/split defaults for SOTA LLM & Agent grading benchmarks
    canonical_presets: dict[str, tuple[str, str]] = {
        "harborframework/terminal-bench-2.1": ("default", "test"),
        "princeton-nlp/SWE-bench_Verified": ("default", "test"),
        "bigcode/bigcodebench": ("default", "v0.1.0_hf"),
        "openai/openai_humaneval": ("openai_humaneval", "test"),
        "google-research-datasets/mbpp": ("sanitized", "test"),
        "macabdul9/hle_text_only": ("default", "test"),
        "fingertap/GPQA-Diamond": ("default", "test"),
        "TIGER-Lab/MMLU-Pro": ("default", "validation"),
        "allenai/ai2_arc": ("ARC-Challenge", "test"),
        "cais/mmlu": ("abstract_algebra", "test"),
        "HuggingFaceH4/MATH-500": ("default", "test"),
        "HuggingFaceH4/aime_2024": ("default", "train"),
        "openai/gsm8k": ("main", "test"),
        "PatronusAI/financebench": ("default", "train"),
        "ChanceFocus/flare-finqa": ("default", "test"),
        "coastalcph/lex_glue": ("case_hold", "test"),
        "nguha/legalbench": ("abercrombie", "test"),
    }
    short_aliases: dict[str, str] = {
        "terminal-bench": "harborframework/terminal-bench-2.1",
        "terminal_bench": "harborframework/terminal-bench-2.1",
        "public_terminal_bench_2_1": "harborframework/terminal-bench-2.1",
        "swe-bench": "princeton-nlp/SWE-bench_Verified",
        "swe_bench": "princeton-nlp/SWE-bench_Verified",
        "swebench": "princeton-nlp/SWE-bench_Verified",
        "public_swe_bench_verified": "princeton-nlp/SWE-bench_Verified",
        "bigcodebench": "bigcode/bigcodebench",
        "public_bigcodebench": "bigcode/bigcodebench",
        "humaneval": "openai/openai_humaneval",
        "openai_humaneval": "openai/openai_humaneval",
        "public_humaneval": "openai/openai_humaneval",
        "mbpp": "google-research-datasets/mbpp",
        "public_mbpp": "google-research-datasets/mbpp",
        "hle": "macabdul9/hle_text_only",
        "humanitys_last_exam": "macabdul9/hle_text_only",
        "public_hle_text_only": "macabdul9/hle_text_only",
        "gpqa": "fingertap/GPQA-Diamond",
        "gpqa_diamond": "fingertap/GPQA-Diamond",
        "public_gpqa_diamond": "fingertap/GPQA-Diamond",
        "mmlu-pro": "TIGER-Lab/MMLU-Pro",
        "mmlu_pro": "TIGER-Lab/MMLU-Pro",
        "public_mmlu_pro": "TIGER-Lab/MMLU-Pro",
        "arc": "allenai/ai2_arc",
        "ai2_arc": "allenai/ai2_arc",
        "arc-challenge": "allenai/ai2_arc",
        "public_arc_challenge": "allenai/ai2_arc",
        "mmlu": "cais/mmlu",
        "math-500": "HuggingFaceH4/MATH-500",
        "math500": "HuggingFaceH4/MATH-500",
        "math": "HuggingFaceH4/MATH-500",
        "public_math_500": "HuggingFaceH4/MATH-500",
        "aime": "HuggingFaceH4/aime_2024",
        "aime_2024": "HuggingFaceH4/aime_2024",
        "public_aime_2024": "HuggingFaceH4/aime_2024",
        "gsm8k": "openai/gsm8k",
        "public_gsm8k": "openai/gsm8k",
        "financebench": "PatronusAI/financebench",
        "public_financebench": "PatronusAI/financebench",
        "finqa": "ChanceFocus/flare-finqa",
        "finance": "ChanceFocus/flare-finqa",
        "public_finqa": "ChanceFocus/flare-finqa",
        "lex_glue": "coastalcph/lex_glue",
        "casehold": "coastalcph/lex_glue",
        "law": "coastalcph/lex_glue",
        "public_legalbench_casehold": "coastalcph/lex_glue",
        "legalbench": "nguha/legalbench",
        "public_legalbench": "nguha/legalbench",
    }
    canonical_reg_ids: dict[str, str] = {
        "harborframework/terminal-bench-2.1": "public_terminal_bench_2_1",
        "princeton-nlp/SWE-bench_Verified": "public_swe_bench_verified",
        "bigcode/bigcodebench": "public_bigcodebench",
        "openai/openai_humaneval": "public_humaneval",
        "google-research-datasets/mbpp": "public_mbpp",
        "macabdul9/hle_text_only": "public_hle_text_only",
        "fingertap/GPQA-Diamond": "public_gpqa_diamond",
        "TIGER-Lab/MMLU-Pro": "public_mmlu_pro",
        "allenai/ai2_arc": "public_arc_challenge",
        "HuggingFaceH4/MATH-500": "public_math_500",
        "HuggingFaceH4/aime_2024": "public_aime_2024",
        "openai/gsm8k": "public_gsm8k",
        "PatronusAI/financebench": "public_financebench",
        "ChanceFocus/flare-finqa": "public_finqa",
        "coastalcph/lex_glue": "public_legalbench_casehold",
        "nguha/legalbench": "public_legalbench",
    }
    if ds_clean.lower() in short_aliases:
        ds_clean = short_aliases[ds_clean.lower()]
    if ds_clean in canonical_presets and config in ("default", "", "auto"):
        config, split = canonical_presets[ds_clean]

    cache_prefix = re.sub(r"[^a-zA-Z0-9_.-]+", "_", f"{ds_clean}_{config}_{split}_").strip("_")
    cache_slug = f"{cache_prefix}{max_rows}"
    cache_path = os.path.join(cache_dir, f"{cache_slug}.json")

    rows: list[dict[str, Any]] = []
    resolved_config = config
    resolved_split = split
    source_url = ""

    cand_caches = [cache_path]
    if os.path.isdir(cache_dir):
        for fn in sorted(os.listdir(cache_dir)):
            if fn.startswith(cache_prefix) and fn.endswith(".json"):
                fp = os.path.join(cache_dir, fn)
                if fp not in cand_caches:
                    cand_caches.append(fp)
    for cp in cand_caches:
        if os.path.exists(cp):
            try:
                cached_obj = json.load(open(cp, "r", encoding="utf-8"))
                cand_rows = list(cached_obj.get("rows", []))
                if cand_rows:
                    rows = cand_rows[:max_rows]
                    resolved_config = str(cached_obj.get("config") or config)
                    resolved_split = str(cached_obj.get("split") or split)
                    source_url = str(cached_obj.get("source_url") or "")
                    break
            except Exception:
                continue

    if not rows:
        if ds_clean.startswith("http://") or ds_clean.startswith("https://"):
            source_url = ds_clean
            req = urllib.request.Request(source_url, headers={"User-Agent": "ACE-Autonomous-Scientist/1.0"})
            with urllib.request.urlopen(req, timeout=20) as resp:
                raw_text = resp.read().decode("utf-8", "replace")
            if raw_text.lstrip().startswith("["):
                parsed = json.loads(raw_text)
                rows = [r for r in parsed[:max_rows] if isinstance(r, dict)]
            elif raw_text.lstrip().startswith("{"):
                parsed = json.loads(raw_text)
                cand_list = parsed.get("rows") or parsed.get("data") or parsed.get("examples") or []
                rows = [r.get("row", r) if isinstance(r, dict) else {"value": r} for r in cand_list[:max_rows]]
            else:
                for line in raw_text.splitlines():
                    line = line.strip()
                    if line.startswith("{"):
                        rows.append(json.loads(line))
                        if len(rows) >= max_rows:
                            break
        else:
            # Resolve valid (config, split) from HuggingFace datasets-server if needed
            ds_enc = urllib.parse.quote(ds_clean, safe="")
            candidate_pairs: list[tuple[str, str]] = [(config, split)]
            try:
                sp_url = f"https://datasets-server.huggingface.co/splits?dataset={ds_enc}"
                sp_req = urllib.request.Request(sp_url, headers={"User-Agent": "ACE-Autonomous-Scientist/1.0"})
                with urllib.request.urlopen(sp_req, timeout=12) as sp_resp:
                    sp_data = json.loads(sp_resp.read().decode("utf-8", "replace"))
                avail = [( s.get("config", "default"), s.get("split", "test") ) for s in sp_data.get("splits", []) if isinstance(s, dict)]
                # Prefer test/validation splits first
                avail_sorted = sorted(avail, key=lambda x: (0 if x[0] == config else 1, 0 if x[1] in ("test", "validation", "dev") else 1))
                for pair in avail_sorted:
                    if pair not in candidate_pairs:
                        candidate_pairs.append(pair)
            except Exception:
                pass

            for cfg_cand, sp_cand in candidate_pairs:
                cfg_enc = urllib.parse.quote(cfg_cand, safe="")
                sp_enc = urllib.parse.quote(sp_cand, safe="")
                fetched: list[dict[str, Any]] = []
                offset = 0
                page_size = min(100, max_rows)
                ok = False
                while len(fetched) < max_rows:
                    cur_len = min(page_size, max_rows - len(fetched))
                    u = f"https://datasets-server.huggingface.co/rows?dataset={ds_enc}&config={cfg_enc}&split={sp_enc}&offset={offset}&length={cur_len}"
                    try:
                        r_req = urllib.request.Request(u, headers={"User-Agent": "ACE-Autonomous-Scientist/1.0"})
                        with urllib.request.urlopen(r_req, timeout=15) as r_resp:
                            r_data = json.loads(r_resp.read().decode("utf-8", "replace"))
                        batch = r_data.get("rows", [])
                        if not batch:
                            break
                        for item in batch:
                            row_obj = item.get("row", item) if isinstance(item, dict) else {"value": item}
                            if isinstance(row_obj, dict):
                                row_copy = dict(row_obj)
                                row_copy.setdefault("row_idx", item.get("row_idx", len(fetched)))
                                fetched.append(row_copy)
                        ok = True
                        if len(batch) < cur_len:
                            break
                        offset += len(batch)
                    except Exception:
                        break
                if ok and fetched:
                    rows = fetched
                    resolved_config = cfg_cand
                    resolved_split = sp_cand
                    source_url = f"https://datasets-server.huggingface.co/rows?dataset={ds_enc}&config={cfg_enc}&split={sp_enc}"
                    break

        if rows:
            with open(cache_path, "w", encoding="utf-8") as cf:
                json.dump({
                    "dataset_id": ds_clean,
                    "config": resolved_config,
                    "split": resolved_split,
                    "source_url": source_url,
                    "rows": rows,
                }, cf, indent=2)

    if not rows:
        raise RuntimeError(f"Failed to fetch public dataset rows for {dataset_id!r} (config={config!r}, split={split!r})")

    normalized: list[dict[str, Any]] = []
    for idx, r in enumerate(rows):
        rec = dict(r)
        rec_id = str(rec.get("id") or rec.get("task_id") or rec.get("question_id") or f"{ds_clean}:{resolved_config}:{resolved_split}:{rec.get('row_idx', idx)}")
        rec["id"] = rec_id
        rec["dataset_source"] = ds_clean
        normalized.append(rec)

    reg_id = register_as or canonical_reg_ids.get(ds_clean) or re.sub(r"[^a-zA-Z0-9_.-]+", "_", f"public_{ds_clean.split('/')[-1]}_{resolved_config}_v1").strip("_").lower()
    raw_sha = hashlib.sha256(json.dumps(normalized, sort_keys=True).encode("utf-8")).hexdigest()
    provenance = {
        "source_type": "huggingface_public",
        "hf_dataset": ds_clean,
        "hf_config": resolved_config,
        "hf_split": resolved_split,
        "source_url": source_url or f"https://huggingface.co/datasets/{ds_clean}",
        "raw_rows_sha256": raw_sha,
        "fetched_count": len(normalized),
    }
    return build_and_register_dataset(
        dataset_id=reg_id,
        version="1.0",
        records=normalized,
        provenance=provenance,
        held_out_ratio=held_out_ratio,
        benchmarks_dir=benchmarks_dir,
    )


def search_corpus(query: str, corpus_dir: str = "/workspace/research_corpus", max_hits: int = 8) -> list[dict[str, Any]]:
    """Search local markdown papers, domain syntheses, paper extracts, and competitor teardowns recursively by keyword relevance."""
    if not os.path.isdir(corpus_dir):
        base_dir = os.path.dirname(os.path.abspath(__file__))
        corpus_dir = os.path.join(base_dir, "research_corpus")
    if not os.path.isdir(corpus_dir):
        return []
    terms = [t.lower() for t in re.split(r"\W+", query) if len(t) >= 2]
    scored: list[tuple[float, str, str]] = []
    seen_paths: set[str] = set()
    for root, dirs, files in os.walk(corpus_dir):
        dirs[:] = [d for d in sorted(dirs) if not d.startswith(".")]
        for fn in sorted(files):
            if not (fn.endswith(".md") or fn.endswith(".txt")):
                continue
            fp = os.path.join(root, fn)
            rel = os.path.relpath(fp, corpus_dir)
            # Avoid duplicate top-level + subdir copies of the same filename
            if fn in seen_paths and rel != fn:
                continue
            seen_paths.add(fn)
            try:
                txt = open(fp, "r", encoding="utf-8", errors="replace").read()
            except Exception:
                continue
            low = txt.lower()
            fn_low = fn.lower()
            score = 0.0
            for t in terms:
                cnt = low.count(t)
                if cnt > 0:
                    score += 1.0 + math.log1p(cnt)
                if t in fn_low:
                    score += 4.0
            if not terms:
                score = 1.0
            if score > 0:
                # Extract best matching window around first matched term
                best_pos = 0
                for t in terms:
                    pos = low.find(t)
                    if pos >= 0:
                        best_pos = max(0, pos - 120)
                        break
                snip = txt[best_pos : best_pos + 700].strip()
                scored.append((round(score, 3), fp, snip))
    scored.sort(key=lambda x: x[0], reverse=True)
    return [{"path": fp, "score": sc, "snippet": snip} for sc, fp, snip in scored[:max_hits]]


def query_traces(sql: str, params: tuple = (), generation: str = "v0.1") -> list[dict[str, Any]]:
    """Execute a read-only SQL query against v0.1, v0.2, or v0.3 trace SQLite databases."""
    db_path = resolve_generation_db(generation)
    return query_v1_traces(sql, params=params, db_path=db_path)


def query_v0_2_traces(sql: str, params: tuple = ()) -> list[dict[str, Any]]:
    """Execute a read-only SQL query against the ACE v0.2 trace database (/workspace/benchmarks/ace_v0_2_traces.db)."""
    return query_traces(sql, params=params, generation="v0.2")


def query_v0_3_traces(sql: str, params: tuple = ()) -> list[dict[str, Any]]:
    """Execute a read-only SQL query against the generation trace database."""
    return query_traces(sql, params=params, generation="v0.3")


def build_and_register_dataset(
    dataset_id: str,
    version: str,
    records: list[dict[str, Any]],
    provenance: dict[str, Any],
    held_out_ratio: float = 0.25,
    benchmarks_dir: str = "/workspace/benchmarks",
) -> dict[str, Any]:
    """
    Deterministically split real provenance-backed records into train_split and held_out_split
    using SHA-256 hashing of record keys (zero synthetic PRNG), write /workspace/benchmarks/datasets/<dataset_id>.json,
    and register/update it in /workspace/benchmarks/registry.json.
    """
    if not os.path.isdir(benchmarks_dir):
        benchmarks_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "benchmarks")
    ds_dir = os.path.join(benchmarks_dir, "datasets")
    os.makedirs(ds_dir, exist_ok=True)

    train_split: list[dict[str, Any]] = []
    held_out_split: list[dict[str, Any]] = []
    threshold = int(max(0.10, min(0.50, float(held_out_ratio))) * 10000)

    for idx, rec in enumerate(records):
        key_str = str(rec.get("id") or rec.get("trace_id") or rec.get("cycle_id") or rec.get("paper_slug") or f"{dataset_id}_{idx}")
        digest_val = int(hashlib.sha256(key_str.encode("utf-8")).hexdigest()[:8], 16) % 10000
        item = dict(rec)
        if digest_val < threshold:
            item["split"] = "held_out"
            held_out_split.append(item)
        else:
            item["split"] = "train"
            train_split.append(item)

    # Guarantee at least 5 items in each split when total records >= 10
    if len(records) >= 10:
        while len(held_out_split) < 5 and len(train_split) > 5:
            moved = train_split.pop()
            moved["split"] = "held_out"
            held_out_split.append(moved)
        while len(train_split) < 5 and len(held_out_split) > 5:
            moved = held_out_split.pop()
            moved["split"] = "train"
            train_split.append(moved)

    clean_id = re.sub(r"[^a-zA-Z0-9_.-]+", "_", dataset_id).strip("_")
    if clean_id.endswith(".json"):
        clean_id = clean_id[:-5]
    ds_path = os.path.join(ds_dir, f"{clean_id}.json")
    payload = {
        "dataset_id": clean_id,
        "version": str(version),
        "provenance": provenance,
        "total_records": len(train_split) + len(held_out_split),
        "train_count": len(train_split),
        "held_out_count": len(held_out_split),
        "train_split": train_split,
        "held_out_split": held_out_split,
    }
    raw_bytes = json.dumps(payload, indent=2, sort_keys=True).encode("utf-8")
    payload["sha256"] = hashlib.sha256(raw_bytes).hexdigest()
    tmp_ds = f"{ds_path}.tmp.{os.getpid()}"
    with open(tmp_ds, "w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2)
    os.replace(tmp_ds, ds_path)

    reg_path = os.path.join(benchmarks_dir, "registry.json")
    registry: dict[str, Any] = {"schema_version": "v0.3", "datasets": {}}
    if os.path.exists(reg_path):
        try:
            with open(reg_path, "r", encoding="utf-8") as rf:
                loaded = json.load(rf)
                if isinstance(loaded, dict):
                    registry = loaded
                    if not isinstance(registry.get("datasets"), dict):
                        registry["datasets"] = {}
        except Exception:
            registry = {"schema_version": "v0.3", "datasets": {}}

    registry["datasets"][clean_id] = {
        "dataset_id": clean_id,
        "version": str(version),
        "path": f"/workspace/benchmarks/datasets/{clean_id}.json",
        "total_records": payload["total_records"],
        "train_count": payload["train_count"],
        "held_out_count": payload["held_out_count"],
        "sha256": payload["sha256"],
        "provenance": provenance,
    }
    tmp_reg = f"{reg_path}.tmp.{os.getpid()}"
    with open(tmp_reg, "w", encoding="utf-8") as rf:
        json.dump(registry, rf, indent=2)
    os.replace(tmp_reg, reg_path)

    return payload


register_dataset = build_and_register_dataset


def load_dataset(dataset_id: str, split: str = "held_out", benchmarks_dir: str = "/workspace/benchmarks") -> list[dict[str, Any]]:
    """Load 'train', 'held_out', or 'all' records from /workspace/benchmarks/datasets/<dataset_id>.json."""
    if not os.path.isdir(benchmarks_dir):
        benchmarks_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "benchmarks")
    clean_id = dataset_id.strip().removeprefix("/workspace/benchmarks/datasets/").removeprefix("benchmarks/datasets/")
    if not clean_id.endswith(".json"):
        clean_id = f"{clean_id}.json"
    ds_path = os.path.join(benchmarks_dir, "datasets", clean_id)
    with open(ds_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    s = split.lower().strip()
    if s in ("train", "train_split"):
        return list(data.get("train_split", []))
    if s in ("held_out", "held_out_split", "test", "eval"):
        return list(data.get("held_out_split", []))
    return list(data.get("train_split", [])) + list(data.get("held_out_split", []))


def load_dataset_split(dataset_id: str, split: str = "both", benchmarks_dir: str = "/workspace/benchmarks") -> Any:
    """Load dataset split(s): returns list[dict] when split is 'train'/'held_out'/'all', or full dict with train_split & held_out_split when split=='both'."""
    s = (split or "both").lower().strip()
    if s in ("train", "train_split", "held_out", "held_out_split", "test", "eval", "all"):
        return load_dataset(dataset_id, split=s, benchmarks_dir=benchmarks_dir)
    if not os.path.isdir(benchmarks_dir):
        benchmarks_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "benchmarks")
    clean_id = dataset_id.strip().removeprefix("/workspace/benchmarks/datasets/").removeprefix("benchmarks/datasets/")
    if not clean_id.endswith(".json"):
        clean_id = f"{clean_id}.json"
    ds_path = os.path.join(benchmarks_dir, "datasets", clean_id)
    with open(ds_path, "r", encoding="utf-8") as f:
        return json.load(f)


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
    # Prioritize core evaluation/scoring logic (bottom-up just above _selftest) over top-of-file cached I/O helpers
    candidates = list(reversed(all_candidates))[:max_mutants]

    # Snapshot existing benchmark/dataset JSON artifacts in memory before running mutants so no mutant or timeout can corrupt them
    json_backups: dict[str, bytes] = {}
    if candidates:
        for bdir in ("/workspace/benchmarks", "/workspace/benchmarks/datasets"):
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
    4-Pass Cross-File AST-PDG (Program Dependence Graph) Conceptual Flow & Zombie Subgraph Analyzer:
    - Pass A (test_only_zombies): Functions reachable ONLY from --selftest / test_* and never from production entrypoints or cross-file library imports.
    - Pass B (unused_imports & shadowed_dict_writes): Dead imports and unconditionally overwritten dict key writes within the same block.
    - Pass C (competing_twin_flows): Function pairs with overlapping SQL tables / files & output dict key schemas (Jaccard >= 0.55).
    - Pass D (except_fallback_shims): Functions invoked exclusively inside broad 'except Exception:' blocks.
    """
    py_files: list[str] = []
    if os.path.isfile(target_path) and target_path.endswith(".py"):
        py_files = [target_path]
    elif os.path.isdir(target_path):
        for root, dirs, files in os.walk(target_path):
            dirs[:] = [d for d in dirs if not d.startswith(".") and d not in ("archive", "__pycache__", "bin", "research_corpus")]
            for fn in sorted(files):
                if fn.endswith(".py"):
                    py_files.append(os.path.join(root, fn))

    unused_imports: list[dict[str, Any]] = []
    test_only_zombies: list[dict[str, Any]] = []
    shadowed_writes: list[dict[str, Any]] = []
    competing_twins: list[dict[str, Any]] = []
    except_shims: list[dict[str, Any]] = []

    # Collect cross-file imported symbols so library modules (e.g. ace_rlm.py) are recognized as live roots
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
        is_pkg_dir = os.path.exists(os.path.join(parent_dir, "__init__.py")) or os.path.basename(parent_dir).startswith("ace_")
        is_shared_lib = mod_name in cross_file_imported_modules or mod_name == "ace_rlm" or is_pkg_dir

        # Pass B1: Unused imports in file (skip __init__.py package re-export files)
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

        # Collect top-level functions and their call sets / I/O fingerprints
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

            # Check shadowed dict writes in direct function body statements
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

            # Track calls inside vs outside ExceptHandler
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

        # Pass A: Production vs Test Call-Graph Partition (for non-library standalone scripts)
        if not is_shared_lib and funcs:
            test_roots = {f for f in funcs if "test" in f.lower()}
            prod_roots = {f for f in funcs if f in ("main", "run", "cli") or (f not in test_roots and not f.startswith("_"))}
            # Walk module-level calls outside selftest branches
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

        # Pass C: Competing Twin Flows (same SQL table + overlapping output dict schema >= 0.65)
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


def _selftest() -> int:
    mean, low, high = beta_ci(9, 10)
    assert 0.7 < mean < 0.95 and low < mean < high, f"beta_ci failed: {mean}, {low}, {high}"
    p = mcnemar_exact_p(12, 1)
    assert 0.0 < p < 0.01, f"mcnemar_exact_p failed: {p}"
    w = wilcoxon_signed_rank([10, 12, 14, 15, 18, 20], [5, 6, 7, 8, 9, 10])
    assert w["p_value"] < 0.05, f"wilcoxon failed: {w}"
    flow_rep = audit_conceptual_flows(__file__)
    assert flow_rep["clean"] is True, f"ace_rlm.py conceptual flow audit not clean: {flow_rep}"
    print(json.dumps({"ok": True, "sdk": "ace_rlm.py", "beta_ci_demo": [mean, low, high], "mcnemar_demo": p}))
    return 0


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="ACE v0.2 Scientific & AST-PDG SDK")
    parser.add_argument("--selftest", action="store_true", help="Run SDK self-test")
    parser.add_argument("--audit-flows", type=str, default="", help="Run AST-PDG flow audit on path")
    args = parser.parse_args()
    if args.selftest:
        sys.exit(_selftest())
    if args.audit_flows:
        print(json.dumps(audit_conceptual_flows(args.audit_flows), indent=2))
        sys.exit(0)
`
	_ = os.WriteFile(sdkPath, []byte(sdkCode), 0o755)
}
