"""E5: Turn 0.5 router pilot on v0.3 L6 cycles (user's own project data, same free endpoint ACE already uses).
Deterministic ranking = RRF(personalized PageRank over mention graph from directive paths, lexical directive+goal).
Router = beam-free picks <=10 files from the deterministic top-40. Compare recall@k."""
import re, sqlite3, os, json, math, sys, time, urllib.request
from collections import defaultdict, Counter
from cgraph import nodes, mention_graph
ROOT = "/home/jay/programming/ace_prototype/archive/v0_3_decommissioned_20261022/ace-sandbox-v0_3-full"
files = nodes([(f"{ROOT}/workspace", "/workspace/"), (f"{ROOT}/wiki", "/wiki/")])
down = mention_graph(files); und = defaultdict(set)
for a, bs in down.items():
    for b in bs: und[a].add(b); und[b].add(a)
by_base = defaultdict(set)
for p in files: by_base[os.path.basename(p)].add(p)
PATH = re.compile(r"(?:/workspace/|/wiki/)?[\w./-]*[\w-]+\.(?:py|json|jsonl|db|csv|md|txt|parquet)")
def resolve(tok):
    if tok in files: return {tok}
    for pref in ("/workspace/", "/wiki/"):
        if pref + tok.lstrip("/") in files: return {pref + tok.lstrip("/")}
    return set(by_base.get(os.path.basename(tok), ()))
def paths(s): return set().union(*[resolve(t) for t in PATH.findall(s)]) if PATH.findall(s) else set()
toks = lambda s: set(re.findall(r"[a-z][a-z0-9]{3,}", s.lower()))
ptoks = {n: toks(n.replace("/", " ").replace("_", " ").replace(".", " ")) for n in files}
df = Counter(t for s in ptoks.values() for t in s); NF = len(files)
def ppr(seeds, alpha=0.6, it=25):
    if not seeds: return {}
    p = {s: 1 / len(seeds) for s in seeds}; r = dict(p)
    for _ in range(it):
        nr = defaultdict(float)
        for n, v in r.items():
            nb = und.get(n)
            if not nb: nr[n] += alpha * v; continue
            for m in nb: nr[m] += alpha * v / len(nb)
        r = {n: v + (1 - alpha) * p.get(n, 0) for n, v in nr.items()}
    return r
def summary(n):
    p = files[n]; sz = os.path.getsize(p)
    if n.endswith(".py"):
        sig = [l.strip()[:70] for l in open(p, errors="replace") if re.match(r"(def|class) ", l)][:4]
        return f"{n} ({sz}B) defs: {'; '.join(sig)}"
    if n.endswith(".md"):
        first = next((l.strip() for l in open(p, errors="replace") if l.strip()), "")[:80]
        return f"{n} ({sz}B) {first}"
    return f"{n} ({sz}B)"
c = sqlite3.connect(f"file:{ROOT}/ace.db?mode=ro", uri=True); c.text_factory = lambda b: b.decode("utf-8", "replace")
cyc = defaultdict(list)
for cid, up_, at, tgt in c.execute("select cycle_id, user_prompt, action_type, action_target from thought_traces where layer_id=6 order by created_at"):
    cyc[cid].append((up_, at, tgt))
import subprocess
def call(prompt):
    t = time.time()
    r = subprocess.run([os.path.expanduser("~/.opencode/bin/opencode"), "run", "-m", "opencode-go/beam-free", prompt], capture_output=True, text=True, timeout=120, cwd=os.path.abspath("empty"))
    out = re.sub(r"\x1b\[[0-9;]*m", "", r.stdout)
    if "[" not in out: raise RuntimeError(out[-200:] + r.stderr[-200:])
    return out[out.rfind("["):], time.time() - t, {}
def _unused(prompt):
    body = json.dumps({"model": "beam-free", "messages": [{"role": "user", "content": prompt}], "max_tokens": 600, "temperature": 0}).encode()
    req = urllib.request.Request("https://opencode.ai/inference/openai/v1/chat/completions", data=body, headers={
        "Authorization": "Bearer public", "Content-Type": "application/json", "User-Agent": "opencode/1.18.21 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14",
        "x-opencode-client": "cli", "x-opencode-project": "global", "x-opencode-session": "ace-router-pilot"})
    t = time.time()
    with urllib.request.urlopen(req, timeout=90) as r: d = json.load(r)
    return d["choices"][0]["message"]["content"] or "", time.time() - t, d.get("usage", {})
res = defaultdict(list); lat = []; usage = Counter(); fails = 0
limit = int(sys.argv[1]) if len(sys.argv) > 1 else 81
done = 0
for cid, steps in cyc.items():
    m = re.search(r"Southbound Directive[^\n]*", steps[0][0])
    if not m: continue
    g = re.search(r"## Active Directive.*?(?=Southbound Directive)", steps[0][0], re.S)
    goal = (g.group(0) if g else "")[:3000]; directive = m.group(0)
    written, touched = set(), set()
    for _, at, tgt in steps:
        for k, part in zip(at.split("+"), (tgt or "").split(" ; ") + [""] * 9):
            ps = paths(part)
            if k == "workspace_write": written |= ps - touched
            touched |= ps
    need = touched - written
    if not need: continue
    anchors = paths(directive)
    pr = ppr(anchors); mt = toks(directive + " " + goal)
    lex = {n: sum(math.log(NF / df[t]) for t in mt & ptoks[n]) for n in files}
    rr = defaultdict(float)
    for sc in (pr, {n: v for n, v in lex.items() if v > 0}):
        for rank, n in enumerate(sorted(sc, key=lambda n: -sc[n])[:100]): rr[n] += 1 / (60 + rank)
    det = sorted(rr, key=lambda n: -rr[n])
    cand = det[:40]
    prompt = ("You choose which files a coding agent must have in context before it starts. Task directive:\n" + directive[:1500] +
              "\n\nProject goal excerpt:\n" + goal[:1500] + "\n\nCandidate files (path, size, summary):\n" +
              "\n".join(f"{i+1}. {summary(n)}" for i, n in enumerate(cand)) +
              "\n\nReturn ONLY a JSON array of up to 10 candidate numbers, most necessary first. Include files the agent will need to read, run, or modify.")
    try:
        out, dt, u = call(prompt); lat.append(dt); usage.update({k: v for k, v in u.items() if isinstance(v, int)})
        picks = [int(x) for x in re.findall(r"\d+", re.search(r"\[[^\]]*\]", out).group(0))][:10]
        routed = [cand[i - 1] for i in picks if 1 <= i <= len(cand)]
    except Exception as e:
        fails += 1; routed = []; print("fail", type(e).__name__, str(e)[:100], file=sys.stderr)
    rk = lambda lst, k: len(need & set(lst[:k])) / len(need)
    for k in (5, 10, 20, 40): res[f"det@{k}"].append(rk(det, k))
    res["router@5"].append(rk(routed, 5)); res["router@10"].append(rk(routed, 10))
    res["router@10 + det fill to 20"].append(rk(routed + [n for n in det if n not in routed], 20))
    res["ceiling: need within top-40"].append(rk(cand, 40))
    done += 1
    if done >= limit: break
print(f"cases={done} router fails={fails} latency p50={sorted(lat)[len(lat)//2]:.1f}s p90={sorted(lat)[int(.9*len(lat))]:.1f}s usage={dict(usage)}")
for k, v in res.items(): print(f"  {k:30s} recall={100*sum(v)/len(v):5.1f}%")
