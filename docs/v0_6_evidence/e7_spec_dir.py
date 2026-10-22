"""E7: (a) bytes spec section 3 needs (paths + signatures of files the task used), (b) directive churn between consecutive L6 cycles."""
import re, sqlite3, os, difflib, statistics, json
from collections import defaultdict
ROOT = "/home/jay/programming/ace_prototype/archive/v0_3_decommissioned_20261022/ace-sandbox-v0_3-full"
from cgraph import nodes
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
def sig_bytes(n):
    p = files[n]
    if n.endswith(".py"): return len(n) + sum(len(l.strip()) + 1 for l in open(p, errors="replace") if re.match(r"\s*(def|class) ", l))
    if n.endswith(".json"):
        try:
            o = json.load(open(p)); k = list(o[0].keys()) if isinstance(o, list) and o and isinstance(o[0], dict) else (list(o.keys()) if isinstance(o, dict) else [])
            return len(n) + len(",".join(k[:40])) + 10
        except Exception: return len(n) + 10
    return len(n) + 10
c = sqlite3.connect(f"file:{ROOT}/ace.db?mode=ro", uri=True); c.text_factory = lambda b: b.decode("utf-8", "replace")
cyc = defaultdict(list); order = []
for cid, up_, at, tgt in c.execute("select cycle_id, user_prompt, action_type, action_target from thought_traces where layer_id=6 order by created_at"):
    if cid not in cyc: order.append(cid)
    cyc[cid].append((up_, at, tgt))
s3, dirs, nfiles = [], [], []
for cid in order:
    steps = cyc[cid]
    m = re.search(r"Southbound Directive[^\n]*", steps[0][0]); dirs.append(m.group(0) if m else "")
    used = set()
    for _, at, tgt in steps: used |= paths(tgt or "")
    if used:
        s3.append(sum(sig_bytes(n) for n in used)); nfiles.append(len(used))
q = lambda v, p: sorted(v)[int(p * (len(v) - 1))]
print(f"files used per task: p50={q(nfiles,.5)} p90={q(nfiles,.9)}; section-3 bytes (paths+signatures/keys): p50={q(s3,.5)} p75={q(s3,.75)} p90={q(s3,.9)}; share > 1400B (half the spec)={sum(x>1400 for x in s3)/len(s3):.0%}; > 2800B={sum(x>2800 for x in s3)/len(s3):.0%}")
print(f"directive length bytes: p50={q([len(d) for d in dirs],.5)} p90={q([len(d) for d in dirs],.9)}")
sims = [difflib.SequenceMatcher(None, a, b).ratio() for a, b in zip(dirs, dirs[1:]) if a and b]
print(f"consecutive directive similarity: p50={q(sims,.5):.2f}; share >=0.8 (near-repeat)={sum(s>=.8 for s in sims)/len(sims):.0%}; identical={sum(s==1 for s in sims)/len(sims):.0%}")
