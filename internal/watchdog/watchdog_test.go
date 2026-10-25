package watchdog

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
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
	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()
	db.SetMaxOpenConns(1)
	InitEvents(db)
	for i := 0; i < 5; i++ {
		if err := AppendEvent(db, "tick", "ok"); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyChain(db); err != nil {
		t.Fatalf("clean chain failed verify: %v", err)
	}
	db.Exec(`UPDATE watchdog_events SET detail='tampered' WHERE id=3`)
	if err := VerifyChain(db); err == nil {
		t.Fatal("tampered row must break the chain")
	}
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
