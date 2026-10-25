// events.go — hash-chained watchdog event log as an append-only file OUTSIDE
// ace.db, so a database rollback can never erase it (v0.9 §9.0). One JSON
// object per line: {"ts","kind","detail","prev_sha","sha"} with
// sha = sha256(prev|ts|kind|detail) — identical to the retired
// watchdog_events table, so migrated rows verify unchanged.
package watchdog

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EventPath is the sandbox file the chain lives in.
func EventPath(sandboxDir string) string {
	return filepath.Join(sandboxDir, "watchdog_events.log")
}

type eventLine struct {
	TS      string `json:"ts"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
	PrevSHA string `json:"prev_sha"`
	SHA     string `json:"sha"`
}

func chainSHA(prev, ts, kind, detail string) string {
	sum := sha256.Sum256([]byte(prev + "|" + ts + "|" + kind + "|" + detail))
	return hex.EncodeToString(sum[:])
}

func errChain(n int, msg string) error {
	return fmt.Errorf("event %d: %s", n, msg)
}

// lastSHA returns the chain head — the sha of the final line, "" if none.
func lastSHA(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var sha string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var e eventLine
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			sha = e.SHA
		}
	}
	return sha
}

// AppendEventFile appends one hash-chained event. Safe for the supervisor and
// one-shot control processes alike: the tail is re-read on every call.
func AppendEventFile(path, kind, detail string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	prev := lastSHA(path)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	e := eventLine{TS: ts, Kind: kind, Detail: detail, PrevSHA: prev}
	e.SHA = chainSHA(prev, ts, kind, detail)
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// VerifyChainFile recomputes every event hash — returns the first broken link.
func VerifyChainFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var prev string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		var e eventLine
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return errChain(n, "unparseable line")
		}
		if e.PrevSHA != prev {
			return errChain(n, "prev_sha mismatch (chain broken)")
		}
		if chainSHA(e.PrevSHA, e.TS, e.Kind, e.Detail) != e.SHA {
			return errChain(n, "sha mismatch (tampered event)")
		}
		prev = e.SHA
	}
	return sc.Err()
}

// MigrateEvents exports the retired watchdog_events table into the file once —
// a no-op when the file already has lines. Missing table is not an error.
func MigrateEvents(db *sql.DB, path string) {
	if lastSHA(path) != "" || db == nil {
		return
	}
	rows, err := db.Query(`SELECT ts, kind, detail, prev_sha, sha FROM watchdog_events ORDER BY id`)
	if err != nil {
		return
	}
	defer rows.Close()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	for rows.Next() {
		var e eventLine
		if rows.Scan(&e.TS, &e.Kind, &e.Detail, &e.PrevSHA, &e.SHA) == nil {
			if b, err := json.Marshal(e); err == nil {
				_, _ = f.Write(append(b, '\n'))
			}
		}
	}
}
