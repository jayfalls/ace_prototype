"""E4: replay recent commits. Directive = commit message, seed = one changed file; target = the other changed files.
Measures hub blow-up of BFS slices and recall@k for ranking strategies. Language-agnostic graph only."""
import subprocess, sys, os, re, math, random
from collections import defaultdict, Counter
from cgraph import nodes, mention_graph, walk, reverse
repo = sys.argv[1]; N = int(sys.argv[2]) if len(sys.argv) > 2 else 200
tracked = set(subprocess.run(["git", "-C", repo, "ls-files"], capture_output=True, text=True).stdout.split("\n")) - {""}
files = {n: p for n, p in nodes([(repo, "")]).items() if n in tracked}
down = mention_graph(files); up = reverse(down)
und = defaultdict(set)
for a, bs in down.items():
    for b in bs: und[a].add(b); und[b].add(a)
deg = sorted((len(und[n]) for n in files), reverse=True)
log = subprocess.run(["git", "-C", repo, "log", "--no-merges", "--format=@@%H%x09%s%x09%b", "--name-only", "-n", "3000"], capture_output=True, text=True).stdout
commits = []
for blk in log.split("@@")[1:]:
    head, *rest = blk.split("\n"); parts = head.split("\t")
    commits.append((parts[1] + " " + (parts[2] if len(parts) > 2 else ""), [f for f in rest if f.strip()]))
commits.reverse()  # oldest first
toks = lambda s: set(t for t in re.findall(r"[a-z][a-z0-9]{2,}", re.sub(r"([a-z])([A-Z])", r"\1 \2", s).lower()) if t not in {"the", "and", "for", "with", "fix", "add", "from", "into", "update", "feat", "chore", "refactor", "use", "when", "not", "this", "that"})
ptoks = {n: toks(n.replace("/", " ").replace("_", " ").replace("-", " ").replace(".", " ")) for n in files}
df = Counter(t for s in ptoks.values() for t in s); NF = len(files)
def ppr(seeds, alpha=0.6, it=25):
    p = {s: 1 / len(seeds) for s in seeds}; r = dict(p)
    for _ in range(it):
        nr = defaultdict(float)
        for n, v in r.items():
            nb = und.get(n)
            if not nb: nr[n] += alpha * v; continue
            for m in nb: nr[m] += alpha * v / len(nb)
        r = {n: v + (1 - alpha) * p.get(n, 0) for n, v in nr.items()}
        for s in seeds: r.setdefault(s, (1 - alpha) * p[s])
    return r
cochange = defaultdict(Counter); evals = []
random.seed(7)
for i, (msg, ch) in enumerate(commits):
    ch_in = [f for f in ch if f in files]
    if i >= len(commits) - N and 2 <= len(ch_in) <= 12 and len(ch) <= 20:
        seed = random.choice(ch_in); target = set(ch_in) - {seed}
        mt = toks(msg)
        lex = {n: sum(math.log(NF / df[t]) for t in mt & ptoks[n]) for n in files}
        pr = ppr([seed]); cc = cochange[seed]
        scores = {"bfs1(by degree asc)": {n: 1 / (1 + len(und[n])) for n in und[seed]},
                  "ppr": pr, "lexical(msg)": {n: v for n, v in lex.items() if v > 0},
                  "cochange(history)": dict(cc)}
        rr = defaultdict(float)
        for name in ("ppr", "lexical(msg)", "cochange(history)"):
            for rank, n in enumerate(sorted((n for n in scores[name] if n != seed), key=lambda n: -scores[name][n])[:200]):
                rr[n] += 1 / (60 + rank)
        scores["hybrid RRF"] = rr
        res = {}
        for name, sc in scores.items():
            ranked = [n for n in sorted(sc, key=lambda n: -sc[n]) if n != seed]
            res[name] = {k: len(target & set(ranked[:k])) / len(target) for k in (5, 10, 20, 40)}
        b1 = walk({seed}, 1, [down, up]); b2 = walk({seed}, 2, [down, up])
        evals.append((res, len(b1), len(b2), len(target)))
    for a in ch:
        for b in ch:
            if a != b and len(ch) <= 30: cochange[a][b] += 1
print(f"{os.path.basename(repo)}: nodes={NF} edges={sum(len(v) for v in down.values())} max degree={deg[0]} p99 degree={deg[len(deg)//100]} median={deg[len(deg)//2]}; eval commits={len(evals)}")
b1 = sorted(e[1] for e in evals); b2 = sorted(e[2] for e in evals)
print(f"  BFS slice size (share of repo): 1-hop p50={b1[len(b1)//2]} ({100*b1[len(b1)//2]/NF:.0f}%) p90={b1[int(.9*len(b1))]}; 2-hop p50={b2[len(b2)//2]} ({100*b2[len(b2)//2]/NF:.0f}%) p90={b2[int(.9*len(b2))]} ({100*b2[int(.9*len(b2))]/NF:.0f}%)")
for name in evals[0][0]:
    print(f"  {name:22s} " + " ".join(f"R@{k}={100*sum(e[0][name][k] for e in evals)/len(evals):5.1f}%" for k in (5, 10, 20, 40)))
