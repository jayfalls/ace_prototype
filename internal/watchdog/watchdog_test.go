package watchdog

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSnapshotAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s := Snapshot{Heartbeat: time.Now().UTC().Format(time.RFC3339), TotalCycles: 7, Mode: "autonomous", ConstitutionSHA: "abc123"}
	if err := WriteSnapshot(dir, s); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(SnapshotPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 || !filepath.IsAbs(SnapshotPath(dir)) {
		t.Fatal("snapshot not written")
	}
}

func TestRunChecksHealthy(t *testing.T) {
	dir := t.TempDir()
	db, _ := sql.Open("sqlite", filepath.Join(dir, "ace.db"))
	db.Exec(`CREATE TABLE verdict_rows (id INTEGER PRIMARY KEY, tier INT, action_fingerprint TEXT, rule_id TEXT, verdict TEXT, constitution_sha TEXT, detail TEXT, decided_at DATETIME)`)
	db.Exec(`CREATE TABLE cycle_reports (id INTEGER PRIMARY KEY, cycle_id TEXT, done TEXT, measured TEXT, goals TEXT, uncertainties TEXT, created_at DATETIME)`)
	db.Close()
	s := Snapshot{Heartbeat: time.Now().UTC().Format(time.RFC3339), Busy: false, ConstitutionSHA: "abc"}
	_ = WriteSnapshot(dir, s)
	checks := RunChecks(dir, 20)
	for _, c := range checks {
		if !c.OK {
			t.Fatalf("check %s failed on healthy fixture: %s", c.Name, c.Detail)
		}
	}
}

func TestRunChecksStaleHeartbeat(t *testing.T) {
	dir := t.TempDir()
	s := Snapshot{Heartbeat: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
	_ = WriteSnapshot(dir, s)
	heartbeatOK := true
	for _, c := range RunChecks(dir, 20) {
		if c.Name == "heartbeat" {
			heartbeatOK = c.OK
		}
	}
	if heartbeatOK {
		t.Fatal("stale heartbeat must fault")
	}
}

func TestEventChainAndTamper(t *testing.T) {
	dir := t.TempDir()
	path := EventPath(dir)
	for i := 0; i < 5; i++ {
		if err := AppendEventFile(path, "tick", "ok"); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyChainFile(path); err != nil {
		t.Fatalf("clean chain failed verify: %v", err)
	}
	// Tamper with line 3 in the file — the chain must break.
	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	lines[2] = strings.Replace(lines[2], `"ok"`, `"tampered"`, 1)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChainFile(path); err == nil {
		t.Fatal("tampered event must break the chain")
	}
}

func TestMigrateEventsOnce(t *testing.T) {
	dir := t.TempDir()
	db, _ := sql.Open("sqlite", filepath.Join(dir, "ace.db"))
	defer db.Close()
	db.Exec(`CREATE TABLE watchdog_events (id INTEGER PRIMARY KEY AUTOINCREMENT, ts DATETIME, kind TEXT, detail TEXT, prev_sha TEXT, sha TEXT)`)
	sha := chainSHA("", "2026-01-01T00:00:00Z", "boot", "genesis")
	db.Exec(`INSERT INTO watchdog_events (ts,kind,detail,prev_sha,sha) VALUES ('2026-01-01T00:00:00Z','boot','genesis','',?)`, sha)

	path := EventPath(dir)
	MigrateEvents(db, path)
	if err := VerifyChainFile(path); err != nil {
		t.Fatalf("migrated chain failed verify: %v", err)
	}
	MigrateEvents(db, path) // second call must not duplicate
	if n := strings.Count(mustRead(t, path), "\n"); n != 1 {
		t.Fatalf("migration ran twice: %d lines", n)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	db, _ := sql.Open("sqlite", filepath.Join(dir, "ace.db"))
	db.Exec(`CREATE TABLE t (x INT); INSERT INTO t VALUES (1)`)
	db.Close()
	dst, err := Backup(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal("backup file missing")
	}
	bdb, err := sql.Open("sqlite", dst)
	if err != nil {
		t.Fatal(err)
	}
	defer bdb.Close()
	var n int
	if err := bdb.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("backup unreadable or wrong content: %v %d", err, n)
	}
}

func TestMemAvailableGB(t *testing.T) {
	mem := []byte("MemTotal:       16384000 kB\nMemFree:          204800 kB\nMemAvailable:    8388608 kB\nBuffers:           10240 kB\n")
	gb, ok := memAvailableGB(mem)
	if !ok {
		t.Fatal("no MemAvailable parsed")
	}
	if gb < 8.5 || gb > 8.6 {
		t.Fatalf("MemAvailable GB=%.3f want ~8.59", gb)
	}
	if _, ok := memAvailableGB([]byte("MemFree: 100 kB\n")); ok {
		t.Fatal("missing MemAvailable must not pass")
	}
}
