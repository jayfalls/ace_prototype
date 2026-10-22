"""Simulate a stuck detector on the real Mode-3 veto sequences: escalate after k consecutive identical veto reasons."""
import sqlite3, re
from collections import defaultdict
A = "/home/jay/programming/ace_prototype/archive"
DBS = [f"{A}/v0_2_decommissioned_20261021/ace-sandbox-v0_2-full/ace.db", f"{A}/v0_3_decommissioned_20261022/ace-sandbox-v0_3-full/ace.db"]
seqs = defaultdict(list)
for db in DBS:
    c = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
    for st, det, ts in c.execute("select status, detail, created_at from telemetry_spans where operation='gate.mode3' order by created_at"):
        m = re.search(r"\*\*(.+?)(\.\.\.|\*\*)", det); mil = (db[-30:], m.group(1)[:40] if m else "?")
        reason = det.split("passed=false", 1)[-1] if st != "ok" else "PASS"
        norm = re.sub(r"\d+(\.\d+)?", "#", reason)[:160]   # normalize numbers
        seqs[mil].append(norm)
total = sum(len(s) for s in seqs.values()); passes = sum(s.count("PASS") for s in seqs.values())
print(f"milestones={len(seqs)} gated cycles={total} passes={passes}")
for mode in ("identical reason", "any veto"):
    for k in (2, 3, 4, 5, 8):
        used = esc = lost = 0
        for mil, s in seqs.items():
            run, prev = 0, None
            for i, r in enumerate(s):
                used += 1
                if r == "PASS": run = 0; prev = None; break
                run = run + 1 if (mode == "any veto" or r == prev) else 1; prev = r
                if run >= k:
                    esc += 1
                    if "PASS" in s[i + 1:]: lost += 1          # would have passed later without escalation
                    break
        print(f"{mode:16s} k={k}: cycles used={used:3d} ({100*used/total:4.1f}%), escalations={esc:2d}, escalations on milestones that later passed={lost}")
