package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const ticketCLIScript = `#!/usr/bin/env python3
"""ACE Sandbox Pi Ticket CLI — JSON ticket, checklist, log, and evidence tracker."""
import json, os, sys, time

STORE_DIR = "/workspace/.tickets"
STORE_FILE = os.path.join(STORE_DIR, "tickets.json")
PAD_FILE = os.path.join(STORE_DIR, "pad.md")

def load_db():
    os.makedirs(STORE_DIR, exist_ok=True)
    if not os.path.exists(STORE_FILE):
        return {"next_id": 1, "tickets": []}
    with open(STORE_FILE, "r") as f:
        return json.load(f)

def save_db(db):
    os.makedirs(STORE_DIR, exist_ok=True)
    with open(STORE_FILE, "w") as f:
        json.dump(db, f, indent=2)

def find_ticket(db, tid):
    for t in db["tickets"]:
        if str(t["id"]) == str(tid):
            return t
    return None

def main():
    args = sys.argv[1:]
    if not args or args[0] in ("-h", "--help", "help"):
        print(json.dumps({"usage": "ticket [list | add <title> --goal <g> | show <id> | log <id> --did .. --doing .. --learned .. --new-item .. --pad | check <id> <idx> | evidence <id> <idx> <proof>]"}))
        return
    cmd = args[0]
    db = load_db()
    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())

    if cmd == "list":
        print(json.dumps({"tickets": db["tickets"], "count": len(db["tickets"])}, indent=2))
        return

    if cmd == "add":
        title = args[1] if len(args) > 1 else "Untitled Task"
        goal = ""
        if "--goal" in args:
            idx = args.index("--goal")
            if idx + 1 < len(args):
                goal = args[idx + 1]
        t = {
            "id": db["next_id"],
            "title": title,
            "goal": goal,
            "status": "in_progress",
            "items": [],
            "logs": [],
            "created_at": now,
            "updated_at": now,
        }
        db["next_id"] += 1
        db["tickets"].append(t)
        save_db(db)
        print(json.dumps(t, indent=2))
        return

    if cmd == "show" and len(args) > 1:
        t = find_ticket(db, args[1])
        print(json.dumps(t or {"error": "not found"}, indent=2))
        return

    if cmd == "log" and len(args) > 1:
        t = find_ticket(db, args[1])
        if not t:
            t = {"id": db["next_id"], "title": f"Auto Ticket {db['next_id']}", "goal": "", "status": "in_progress", "items": [], "logs": [], "created_at": now, "updated_at": now}
            db["next_id"] += 1
            db["tickets"].append(t)
        entry = {"at": now}
        i = 2
        pad_flag = False
        while i < len(args):
            k = args[i]
            if k in ("--did", "--doing", "--learned") and i + 1 < len(args):
                entry[k[2:]] = args[i + 1]
                i += 2
            elif k == "--new-item" and i + 1 < len(args):
                t["items"].append({"idx": len(t["items"]) + 1, "text": args[i + 1], "checked": False, "evidence": ""})
                i += 2
            elif k == "--pad":
                pad_flag = True
                i += 1
            else:
                i += 1
        t["logs"].append(entry)
        t["updated_at"] = now
        if pad_flag and entry.get("learned"):
            with open(PAD_FILE, "a") as pf:
                pf.write(f"- [{now}] (Ticket #{t['id']}): {entry['learned']}\n")
        save_db(db)
        print(json.dumps(t, indent=2))
        return

    if cmd == "check" and len(args) > 2:
        t = find_ticket(db, args[1])
        if not t:
            print(json.dumps({"error": "ticket not found"}))
            return
        idx = int(args[2])
        for item in t["items"]:
            if item["idx"] == idx:
                item["checked"] = True
        if t["items"] and all(it["checked"] for it in t["items"]):
            t["status"] = "completed"
        t["updated_at"] = now
        save_db(db)
        print(json.dumps(t, indent=2))
        return

    if cmd == "evidence" and len(args) > 3:
        t = find_ticket(db, args[1])
        if not t:
            print(json.dumps({"error": "ticket not found"}))
            return
        idx = int(args[2])
        proof = " ".join(args[3:])
        for item in t["items"]:
            if item["idx"] == idx:
                item["evidence"] = proof
                item["checked"] = True
        t["updated_at"] = now
        save_db(db)
        print(json.dumps(t, indent=2))
        return

    print(json.dumps({"error": f"unknown command {cmd}"}))

if __name__ == "__main__":
    main()
`

const kbCLIScript = `#!/usr/bin/env python3
"""ACE Sandbox Pi Knowledge Base CLI (kb search / kb note) across /workspace/.kb and /wiki."""
import json, os, sys, time

KB_DIR = "/workspace/.kb"
KB_FILE = os.path.join(KB_DIR, "notes.json")
WIKI_DIR = "/wiki"

def load_notes():
    os.makedirs(KB_DIR, exist_ok=True)
    if not os.path.exists(KB_FILE):
        return []
    with open(KB_FILE, "r") as f:
        return json.load(f)

def save_notes(notes):
    os.makedirs(KB_DIR, exist_ok=True)
    with open(KB_FILE, "w") as f:
        json.dump(notes, f, indent=2)

def main():
    args = sys.argv[1:]
    if not args or args[0] in ("-h", "--help", "help"):
        print(json.dumps({"usage": "kb [search <query> | note <title> --body <text> --tags <t>]"}))
        return
    cmd = args[0]
    notes = load_notes()

    if cmd == "search":
        q = (" ".join(args[1:])).lower().strip()
        tokens = [t for t in q.split() if t]
        hits = []
        for n in notes:
            hay = (n.get("title", "") + " " + n.get("body", "") + " " + n.get("tags", "")).lower()
            score = sum(1 for tok in tokens if tok in hay) if tokens else 1
            if score > 0:
                hits.append({"source": "kb", "score": score, "title": n["title"], "body": n["body"][:300], "created_at": n["created_at"]})
        if os.path.exists(WIKI_DIR):
            for root, _, files in os.walk(WIKI_DIR):
                for fn in files:
                    if fn.endswith(".md"):
                        p = os.path.join(root, fn)
                        rel = os.path.relpath(p, WIKI_DIR)
                        try:
                            txt = open(p, "r", errors="ignore").read()
                            hay = (rel + " " + txt).lower()
                            score = sum(1 for tok in tokens if tok in hay) if tokens else 1
                            if score > 0:
                                hits.append({"source": f"wiki:{rel}", "score": score, "title": rel, "body": txt[:300]})
                        except Exception:
                            pass
        hits.sort(key=lambda x: x["score"], reverse=True)
        print(json.dumps({"query": q, "hits": hits[:8]}, indent=2))
        return

    if cmd == "note":
        title = args[1] if len(args) > 1 else "Untitled Note"
        body, tags = "", ""
        i = 2
        while i < len(args):
            if args[i] == "--body" and i + 1 < len(args):
                body = args[i + 1]
                i += 2
            elif args[i] == "--tags" and i + 1 < len(args):
                tags = args[i + 1]
                i += 2
            else:
                i += 1
        entry = {
            "id": len(notes) + 1,
            "title": title,
            "body": body,
            "tags": tags,
            "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        notes.append(entry)
        save_notes(notes)
        print(json.dumps(entry, indent=2))
        return

    print(json.dumps({"error": f"unknown kb subcommand {cmd}"}))

if __name__ == "__main__":
    main()
`

func (j *Jail) seedPiTools() {
	_ = mkdirAllRoot(j.wsRoot, "bin")
	_ = j.writeExecutable("bin/ticket", ticketCLIScript)
	_ = j.writeExecutable("bin/kb", kbCLIScript)
}

func (j *Jail) writeExecutable(relPath string, content string) error {
	f, err := j.wsRoot.OpenFile(relPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

var (
	hashRangeRe  = regexp.MustCompile(`^\s*(\d+)#([0-9a-fA-F]{3})(?:\s*\.\.\s*(\d+)#([0-9a-fA-F]{3}))?\s*$`)
	hashPrefixRe = regexp.MustCompile(`(?m)^\s*\d+#[0-9a-fA-F]{3}\s*\|\s?`)
)

func lineHash3(line string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimRight(line, "\r\n")))
	return fmt.Sprintf("%03x", h.Sum32()&0xfff)
}

// FormatWithLineHashes renders file content with Pi-style LINE#HASH | prefixes so L6 can perform hash-anchored edits.
func FormatWithLineHashes(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(fmt.Sprintf("%d#%s | %s\n", i+1, lineHash3(line), line))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func stripHashPrefixes(s string) string {
	return hashPrefixRe.ReplaceAllString(s, "")
}

// EditFile performs Pi-style surgical replacement inside /workspace, supporting:
// 1) Hashline anchors ("12#a3f..18#b02" or "12#a3f")
// 2) Exact old_text replacement (automatically stripping any accidental LINE#HASH | prefixes)
// 3) Trimmed-line fallback when only indentation/trailing spaces differ.
func (j *Jail) EditFile(relPath string, oldText string, newText string) error {
	if strings.TrimSpace(oldText) == "" {
		return errors.New("old_text (or line#hash range) must not be empty for workspace_edit")
	}
	cur, err := j.ReadFile(relPath)
	if err != nil {
		return fmt.Errorf("read %s for edit: %w", relPath, err)
	}

	cleanNew := stripHashPrefixes(newText)

	// Mode 1: Check if oldText is a Pi Hashline range like "12#a4f..15#c91" or single line "12#a4f"
	if m := hashRangeRe.FindStringSubmatch(strings.TrimSpace(oldText)); m != nil {
		startLine, _ := strconv.Atoi(m[1])
		startHash := strings.ToLower(m[2])
		endLine := startLine
		endHash := startHash
		if m[3] != "" {
			endLine, _ = strconv.Atoi(m[3])
			endHash = strings.ToLower(m[4])
		}
		lines := strings.Split(cur, "\n")
		if startLine < 1 || endLine < startLine || endLine > len(lines) {
			return fmt.Errorf("hashline range %d..%d out of bounds (file has %d lines)", startLine, endLine, len(lines))
		}
		actualStartHash := lineHash3(lines[startLine-1])
		if actualStartHash != startHash {
			return fmt.Errorf("hash mismatch on line %d: expected #%s, got #%s (re-read file first)", startLine, startHash, actualStartHash)
		}
		actualEndHash := lineHash3(lines[endLine-1])
		if actualEndHash != endHash {
			return fmt.Errorf("hash mismatch on line %d: expected #%s, got #%s (re-read file first)", endLine, endHash, actualEndHash)
		}
		var outLines []string
		outLines = append(outLines, lines[:startLine-1]...)
		outLines = append(outLines, strings.Split(cleanNew, "\n")...)
		outLines = append(outLines, lines[endLine:]...)
		return j.WriteFile(relPath, strings.Join(outLines, "\n"))
	}

	// Mode 2: Exact text match (with or without accidental hash prefixes)
	cleanOld := stripHashPrefixes(oldText)
	if count := strings.Count(cur, cleanOld); count == 1 {
		return j.WriteFile(relPath, strings.Replace(cur, cleanOld, cleanNew, 1))
	} else if count > 1 {
		return fmt.Errorf("old_text matched %d locations in %s; use a more specific block or LINE#HASH..LINE#HASH range", count, relPath)
	}

	// Mode 3: Line-by-line trimmed whitespace fallback
	curLines := strings.Split(cur, "\n")
	oldLines := strings.Split(strings.TrimSpace(cleanOld), "\n")
	if len(oldLines) > 0 && len(oldLines) <= len(curLines) {
		matchIdx := -1
		for i := 0; i <= len(curLines)-len(oldLines); i++ {
			allMatch := true
			for k := 0; k < len(oldLines); k++ {
				if strings.TrimSpace(curLines[i+k]) != strings.TrimSpace(oldLines[k]) {
					allMatch = false
					break
				}
			}
			if allMatch {
				if matchIdx != -1 {
					matchIdx = -2 // ambiguous
					break
				}
				matchIdx = i
			}
		}
		if matchIdx >= 0 {
			var outLines []string
			outLines = append(outLines, curLines[:matchIdx]...)
			outLines = append(outLines, strings.Split(cleanNew, "\n")...)
			outLines = append(outLines, curLines[matchIdx+len(oldLines):]...)
			return j.WriteFile(relPath, strings.Join(outLines, "\n"))
		}
	}

	return fmt.Errorf("old_text not found in %s (tip: use LINE#HASH..LINE#HASH range from workspace_read)", relPath)
}

// DiagnoseFile runs fast Pi-Lens AST/syntax/LSP diagnostics on a file inside /workspace after write/edit.
func (j *Jail) DiagnoseFile(ctx context.Context, relPath string) string {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return ""
	}
	switch {
	case strings.HasSuffix(clean, ".py"):
		cmd := fmt.Sprintf(`python3 -c '
import ast, py_compile, sys
p = %q
try:
    py_compile.compile(p, doraise=True)
    tree = ast.parse(open(p).read(), filename=p)
    syms = [f"class {n.name}" if isinstance(n, ast.ClassDef) else f"def {n.name}" for n in tree.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef))]
    print("[Pi-Lens Python AST OK] lines=" + str(len(open(p).readlines())) + " symbols=" + (", ".join(syms[:12]) or "script"))
except Exception as e:
    print("[Pi-Lens Syntax ERROR] " + str(e))
    sys.exit(1)
'`, "/workspace/"+clean)
		res, _ := j.runBwrap(ctx, cmd)
		return strings.TrimSpace(res.Stdout + " " + res.Stderr)

	case strings.HasSuffix(clean, ".sh"):
		res, _ := j.runBwrap(ctx, fmt.Sprintf("sh -n /workspace/%s && echo '[Pi-Lens Shell OK] syntax valid'", clean))
		return strings.TrimSpace(res.Stdout + " " + res.Stderr)

	case strings.HasSuffix(clean, ".json"):
		data, err := j.ReadFile(clean)
		if err != nil {
			return ""
		}
		var v interface{}
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return "[Pi-Lens JSON ERROR] " + err.Error()
		}
		return "[Pi-Lens JSON OK] valid JSON"
	}
	return ""
}
