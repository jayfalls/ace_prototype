"""Shared: build file graphs (AST or language-agnostic mention edges) and walk them."""
import ast, os, re
from collections import defaultdict
SKIP_DIRS = {"archive", "__pycache__", "research_corpus", "node_modules", ".git", ".venv", "venv", "dist", "build", ".angular", "vendor"}
TEXT_EXT = {".py", ".go", ".ts", ".js", ".md", ".json", ".yaml", ".yml", ".toml", ".txt", ".sql", ".html", ".scss", ".css", ".sh", ".cfg", ".ini", ".csv", ".jsonl"}

def nodes(roots):
    files = {}
    for base, pref in roots:
        for d, dirs, fs in os.walk(base):
            dirs[:] = [x for x in dirs if x not in SKIP_DIRS and not x.startswith(".")]
            for f in fs:
                p = os.path.join(d, f)
                files[pref + os.path.relpath(p, base)] = p
    return files

def mention_graph(files, max_read=400_000):
    """Edge A->B when A's text contains B's basename, or B's stem when the stem is long and unique."""
    by_tok = defaultdict(set); stems = defaultdict(set)
    for n in files:
        b = os.path.basename(n); by_tok[b].add(n); stems[os.path.splitext(b)[0]].add(n)
    for s, ns in stems.items():
        if len(s) >= 6 and len(ns) == 1 and s not in ("__init__", "index", "README", "config", "models", "utils", "common", "constants", "main", "types"):
            by_tok[s] |= ns
    down = defaultdict(set)
    for n, p in files.items():
        if os.path.splitext(p)[1] not in TEXT_EXT: continue
        try: txt = open(p, errors="replace").read(max_read)
        except OSError: continue
        for tok in set(re.findall(r"[\w.-]+", txt)):
            tok = tok.strip(".")
            for q in by_tok.get(tok, ()) or by_tok.get(tok.split("/")[-1], ()):
                if q != n: down[n].add(q)
            # dotted python module path a.b.c -> c
            if "." in tok and not os.path.splitext(tok)[1] in TEXT_EXT:
                for q in by_tok.get(tok.split(".")[-1], ()):
                    if q != n and len(tok.split(".")[-1]) >= 6: down[n].add(q)
    return down

def py_ast_graph(files, ws_prefix="/workspace/"):
    modname = {}
    for n in files:
        if n.endswith(".py"):
            rel = n[len(ws_prefix):-3] if n.startswith(ws_prefix) else n[:-3]
            modname[rel.replace("/", ".").removesuffix(".__init__")] = n
    by_base = defaultdict(set)
    for n in files: by_base[os.path.basename(n)].add(n)
    down = defaultdict(set)
    for n, p in files.items():
        if not n.endswith(".py"): continue
        try: tree = ast.parse(open(p, errors="replace").read())
        except (SyntaxError, ValueError): continue
        rel = n[len(ws_prefix):] if n.startswith(ws_prefix) else n
        pkg = os.path.dirname(rel).replace("/", ".")
        for x in ast.walk(tree):
            if isinstance(x, ast.Import):
                for a in x.names:
                    if a.name in modname: down[n].add(modname[a.name])
            elif isinstance(x, ast.ImportFrom):
                m = x.module or ""
                full = (pkg + "." + m).strip(".") if x.level else m
                for cand in (full, *(f"{full}.{a.name}" for a in x.names)):
                    if cand in modname: down[n].add(modname[cand])
            elif isinstance(x, ast.Constant) and isinstance(x.value, str) and len(x.value) < 300:
                for tok in re.findall(r"[\w./*-]+\.(?:json|jsonl|db|csv|md|txt|parquet)", x.value):
                    for q in by_base.get(os.path.basename(tok), ()): down[n].add(q)
    return down

def reverse(down):
    up = defaultdict(set)
    for a, bs in down.items():
        for b in bs: up[b].add(a)
    return up

def walk(seeds, hops, graphs, skip_expand=frozenset()):
    seen = set(seeds); frontier = set(seeds)
    for _ in range(hops):
        nxt = set()
        for n in frontier:
            if n in skip_expand: continue
            for g in graphs: nxt |= g.get(n, set())
        frontier = nxt - seen; seen |= nxt
    return seen
