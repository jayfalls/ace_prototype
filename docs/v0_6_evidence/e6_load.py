"""E6: (a) DB write load per version, (b) write-set overlap between nearby L6 tasks, (c) what recon reads fetched in v0.3."""
import sqlite3, re
from collections import Counter, defaultdict
from datetime import datetime
A = "/home/jay/programming/ace_prototype/archive"
DBS = {"v0.1": f"{A}/v0_1_decommissioned_20261020/ace-sandbox-v0_1-full/ace.db",
       "v0.2": f"{A}/v0_2_decommissioned_20261021/ace-sandbox-v0_2-full/ace.db",
       "v0.3": f"{A}/v0_3_decommissioned_20261022/ace-sandbox-v0_3-full/ace.db"}
def sec(ts): return datetime.fromisoformat(ts[:19]).timestamp()
for v, db in DBS.items():
    c = sqlite3.connect(f"file:{db}?mode=ro", uri=True); c.text_factory = lambda b: b.decode("utf-8", "replace")
    per = Counter(); tot = 0
    for t in ("thought_traces", "telemetry_spans", "bus_events", "memories", "chat_messages", "telemetry_timeseries"):
        try:
            for (ts,) in c.execute(f"select created_at from {t}"):
                per[int(sec(ts))] += 1; tot += 1
        except Exception as e: pass
    span = max(per) - min(per)
    w10 = Counter(); 
    for s, n in per.items(): w10[s // 10] += n
    print(f"{v}: rows={tot} over {span/3600:.1f}h mean={tot/span:.2f}/s peak-1s={max(per.values())} peak-10s-avg={max(w10.values())/10:.1f}/s")
    # write-set overlap of consecutive L6 cycles
    ws = []
    cur = defaultdict(set); order = []
    for cid, at, tgt in c.execute("select cycle_id, action_type, action_target from thought_traces where layer_id=6 order by created_at"):
        if cid not in cur: order.append(cid)
        for k, part in zip(at.split("+"), (tgt or "").split(" ; ") + [""] * 9):
            if k in ("workspace_write", "workspace_edit", "wiki_write"):
                cur[cid] |= set(re.findall(r"[\w./-]+\.\w+", part)[:1])
    ws = [cur[x] for x in order if cur[x]]
    for W in (2, 4):
        pairs = conf = 0
        for i in range(len(ws) - W + 1):
            grp = ws[i:i + W]
            for a in range(W):
                for b in range(a + 1, W):
                    pairs += 1; conf += bool(grp[a] & grp[b])
        print(f"   if {W} consecutive writing tasks ran in parallel: {100*conf/max(1,pairs):.0f}% of task pairs write the same file (n={pairs})")
    shared = Counter(f for s in ws for f in s)
    print("   most-written files:", shared.most_common(4))
c = sqlite3.connect(f"file:{DBS['v0.3']}?mode=ro", uri=True); c.text_factory = lambda b: b.decode("utf-8", "replace")
kinds = Counter(); ex = []
for at, tgt in c.execute("select action_type, action_target from thought_traces where layer_id=6"):
    for k, part in zip(at.split("+"), (tgt or "").split(" ; ") + [""] * 9):
        if k in ("workspace_read", "wiki_read"): kinds["whole-file read: " + ("wiki" if k == "wiki_read" else "workspace")] += 1
        elif k == "sandbox_sh" and re.search(r"\b(cat|head|tail|sed -n|grep|ls|PRAGMA|json\.load|\.keys\(\)|sqlite_master|wc)\b", part):
            if re.search(r"sqlite_master|PRAGMA|\.schema|table_info", part): kinds["sh: DB schema"] += 1
            elif re.search(r"json\.load|\.keys\(\)|jq", part): kinds["sh: JSON keys/sample"] += 1
            elif re.search(r"\bgrep\b|rg ", part): kinds["sh: grep for symbol/text"] += 1
            elif re.search(r"\bsed -n|head|tail", part): kinds["sh: line range"] += 1
            elif re.search(r"\bls\b|find ", part): kinds["sh: list dir"] += 1
            else: kinds["sh: other read"] += 1; ex.append(part[:110])
print("v0.3 L6 reach-out kinds:", kinds.most_common())
