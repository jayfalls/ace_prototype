"""Extract human-typed message timestamps (UTC) from Pi and Claude Code session logs."""
import json, glob, os, sys
out = []
for f in glob.glob(os.path.expanduser("~/.pi/agent/sessions/**/*.jsonl"), recursive=True):
    for line in open(f, errors="replace"):
        try: e = json.loads(line)
        except Exception: continue
        m = e.get("message") or {}
        if m.get("role") == "user" and e.get("timestamp"):
            c = m.get("content")
            txt = c if isinstance(c, str) else " ".join(x.get("text", "") for x in c or [] if isinstance(x, dict) and x.get("type") == "text")
            if txt.strip(): out.append(("pi", e["timestamp"]))
for f in glob.glob(os.path.expanduser("~/.claude/projects/**/*.jsonl"), recursive=True):
    for line in open(f, errors="replace"):
        try: e = json.loads(line)
        except Exception: continue
        if e.get("type") != "user" or e.get("isSidechain") or e.get("isMeta"): continue
        c = (e.get("message") or {}).get("content")
        if isinstance(c, list) and any(isinstance(x, dict) and x.get("type") == "tool_result" for x in c): continue
        if e.get("timestamp"): out.append(("claude", e["timestamp"]))
out = sorted(set(out), key=lambda x: x[1])
json.dump(out, open("human_ts.json", "w"))
from collections import Counter
print(len(out), Counter(s for s, _ in out), out[0][1], out[-1][1])
