"""E3: language-agnostic mention graph vs Python AST graph, replayed on v0.3 L6 cycles."""
import re, sqlite3, os, sys
from collections import defaultdict
from cgraph import nodes, mention_graph, py_ast_graph, reverse, walk
ROOT = "/home/jay/programming/ace_prototype/archive/v0_3_decommissioned_20261022/ace-sandbox-v0_3-full"
files = nodes([(f"{ROOT}/workspace", "/workspace/"), (f"{ROOT}/wiki", "/wiki/")])
by_base = defaultdict(set)
for p in files: by_base[os.path.basename(p)].add(p)
PATH = re.compile(r"(?:/workspace/|/wiki/)?[\w./-]*[\w-]+\.(?:py|json|jsonl|db|csv|md|txt|parquet)")
def resolve(tok):
    if tok in files: return {tok}
    for pref in ("/workspace/", "/wiki/"):
        if pref + tok.lstrip("/") in files: return {pref + tok.lstrip("/")}
    return set(by_base.get(os.path.basename(tok), ()))
def paths(s): return set().union(*[resolve(t) for t in PATH.findall(s)]) if PATH.findall(s) else set()
c = sqlite3.connect(f"file:{ROOT}/ace.db?mode=ro", uri=True); c.text_factory = lambda b: b.decode("utf-8", "replace")
cyc = defaultdict(list)
for cid, up_, at, tgt in c.execute("select cycle_id, user_prompt, action_type, action_target from thought_traces where layer_id=6 order by created_at"):
    cyc[cid].append((up_, at, tgt))
cases = []
for cid, steps in cyc.items():
    m = re.search(r"Southbound Directive[^\n]*", steps[0][0])
    if not m: continue
    g = re.search(r"## Active Directive.*?(?=Southbound Directive)", steps[0][0], re.S)
    txt = m.group(0) + (g.group(0) if g else "")
    words = set(re.findall(r"[A-Za-z_]\w{4,}", txt.lower()))
    anchors = paths(txt) | {p for p in files if os.path.splitext(os.path.basename(p))[0].lower() in words}
    danchors = paths(m.group(0))
    written, touched = set(), set()
    for _, at, tgt in steps:
        for k, part in zip(at.split("+"), (tgt or "").split(" ; ") + [""] * 9):
            ps = paths(part)
            if k == "workspace_write": written |= ps - touched
            touched |= ps
    need = touched - written
    if need: cases.append((anchors, need, danchors))
G = {"ast": py_ast_graph(files), "mention": mention_graph(files)}
G["union"] = {k: G["ast"].get(k, set()) | G["mention"].get(k, set()) for k in set(G["ast"]) | set(G["mention"])}
print(f"cases={len(cases)} nodes={len(files)}")
for name, d in G.items():
    u = reverse(d); E = sum(len(v) for v in d.values())
    for ai, amode in ((2, "directive paths"), (0, "directive+goal")):
        for hops in (1, 2):
            h = t = sz = 0
            for cs in cases:
                anchors, need = cs[ai], cs[1]
                sl = walk(anchors, hops, [d, u]); h += len(need & sl); t += len(need); sz += len(sl)
            print(f"{name:8s} edges={E:4d} anchors={amode:16s} {hops}-hop both: recall={100*h/t:5.1f}%  mean nodes={sz/len(cases):5.1f}")
