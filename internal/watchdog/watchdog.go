// Package watchdog implements the integrity overlay (spec 8.5 + 29cd3922):
// the `./ace` supervisor reads the atomic status snapshot the engine writes
// each tick — never through the bus — checks heartbeat, constitution sha,
// config inventory, gate fail-closed state, resource floors and stalls, and
// keeps a hash-chained event file plus rotating DB backups.
package watchdog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Snapshot is the atomic per-tick status file the engine writes (write tmp +
// rename). The watchdog never queries the engine directly — only this file
// and the SQLite DB.
type Snapshot struct {
	Heartbeat       string            `json:"heartbeat"` // RFC3339, written every cycle boundary
	TotalCycles     int               `json:"total_cycles"`
	Mode            string            `json:"mode"`
	Busy            bool              `json:"busy"`
	CurrentCycleID  string            `json:"current_cycle_id"`
	ConstitutionSHA string            `json:"constitution_sha"`
	MissionVersion  string            `json:"mission_version"`
	ActiveTaskID    string            `json:"active_task_id"`
	LastToolCallAt  string            `json:"last_tool_call_at"`
	LastActionAt    string            `json:"last_action_at"` // layer announced an action
	VerdictAllow    int               `json:"verdict_allow"`
	VerdictDeny     int               `json:"verdict_deny"`
	ConfigChecksums map[string]string `json:"config_checksums"` // owner file -> sha
}

// SnapshotPath is the state-dir file the engine writes atomically.
func SnapshotPath(sandboxDir string) string {
	return filepath.Join(sandboxDir, "watchdog_status.json")
}

// WriteSnapshot atomically publishes a snapshot (tmp + rename — readers never
// see a partial write).
func WriteSnapshot(sandboxDir string, s Snapshot) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := SnapshotPath(sandboxDir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, SnapshotPath(sandboxDir))
}

// ConfigChecksums hashes the owner-controlled config files the engine must
// never mutate (mission/personality/grants/models) — drift vs. the snapshot
// is a watchdog fault.
func ConfigChecksums(configDir string) map[string]string {
	out := map[string]string{}
	for _, name := range []string{"mission.md", "personality.txt", "grants.json", "models.json"} {
		if b, err := os.ReadFile(filepath.Join(configDir, name)); err == nil {
			sum := sha256.Sum256(b)
			out[name] = hex.EncodeToString(sum[:8])
		}
	}
	return out
}

// Check is one watchdog verdict line.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// RunChecks performs every spec-8.5 check against the snapshot + DB. sandboxDir
// holds ace.db + watchdog_status.json; intervalSec is the engine tick.
func RunChecks(sandboxDir string, intervalSec int) []Check {
	var checks []Check
	b, err := os.ReadFile(SnapshotPath(sandboxDir))
	if err != nil {
		return []Check{{Name: "snapshot", OK: false, Detail: "no status snapshot: " + err.Error()}}
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return []Check{{Name: "snapshot", OK: false, Detail: "corrupt snapshot: " + err.Error()}}
	}

	hb, _ := time.Parse(time.RFC3339, s.Heartbeat)
	age := time.Since(hb)
	checks = append(checks, Check{"heartbeat", age < time.Duration(3*intervalSec)*time.Second,
		fmt.Sprintf("last heartbeat %v ago", age.Round(time.Second))})

	checks = append(checks, Check{"constitution", s.ConstitutionSHA != "", "sha " + s.ConstitutionSHA})

	dbPath := filepath.Join(sandboxDir, "ace.db")
	db, derr := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(2000)")
	if derr == nil {
		defer db.Close()
		var n int
		gerr := db.QueryRow(`SELECT COUNT(*) FROM verdict_rows`).Scan(&n)
		checks = append(checks, Check{"gate_fail_closed", gerr == nil,
			fmt.Sprintf("verdict_rows reachable: %d rows", n)})

		// Runaway loop: cycles in the last hour beyond a sane bound.
		// sandboxDir/runaway_limit overrides the default 400 — written to
		// trip the pause live, deleted to restore (no restart needed).
		var recent int
		_ = db.QueryRow(`SELECT COUNT(*) FROM cycle_reports WHERE created_at > datetime('now','-1 hour')`).Scan(&recent)
		limit := 400
		if blob, rerr := os.ReadFile(filepath.Join(sandboxDir, "runaway_limit")); rerr == nil {
			if v, perr := strconv.Atoi(strings.TrimSpace(string(blob))); perr == nil && v > 0 {
				limit = v
			}
		}
		checks = append(checks, Check{"runaway_loop", recent < limit, fmt.Sprintf("%d cycles in last hour (limit %d)", recent, limit)})

		// Memory health (spec 9.6): episodic cap pressure + unverified
		// semantic facts are watchdog-visible.
		var episodic, unverified int
		_ = db.QueryRow(`SELECT
			COALESCE(SUM(CASE WHEN COALESCE(type,'episodic')='episodic' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN type='semantic' AND verified_at IS NULL THEN 1 ELSE 0 END),0)
			FROM memories`).Scan(&episodic, &unverified)
		checks = append(checks, Check{"memory_store", episodic <= 10000,
			fmt.Sprintf("episodic=%d/10000 unverified_semantic=%d", episodic, unverified)})

		// Stall: action announced >120s ago with no tool call while busy.
		if s.Busy && s.LastActionAt != "" {
			la, _ := time.Parse(time.RFC3339, s.LastActionAt)
			lt, _ := time.Parse(time.RFC3339, s.LastToolCallAt)
			stalled := time.Since(la) > 120*time.Second && lt.Before(la)
			checks = append(checks, Check{"stall_detection", !stalled,
				fmt.Sprintf("action at %s, last tool %s", la.Format("15:04:05"), lt.Format("15:04:05"))})
		}
	} else {
		checks = append(checks, Check{"gate_fail_closed", false, "db unreachable: " + derr.Error()})
	}

	// Disk floor 10GB + memory floor check.
	var stat syscall.Statfs_t
	if syscall.Statfs(sandboxDir, &stat) == nil {
		freeGB := float64(stat.Bavail) * float64(stat.Bsize) / 1e9
		checks = append(checks, Check{"disk", freeGB >= 10, fmt.Sprintf("%.1f GB free", freeGB)})
	}
	if meminfo, err := os.ReadFile("/proc/meminfo"); err == nil {
		if gb, ok := memAvailableGB(meminfo); ok {
			checks = append(checks, Check{"memory", gb >= 1, fmt.Sprintf("%.1f GB RAM available", gb)})
		}
	}
	return checks
}

var meminfoAvailRe = regexp.MustCompile(`(?m)^MemAvailable:\s*(\d+)\s*kB`)

// memAvailableGB parses MemAvailable (not Freeram — reclaimable cache is
// usable) from meminfo text. Host-independent: caller injects the content.
func memAvailableGB(meminfo []byte) (float64, bool) {
	m := meminfoAvailRe.FindSubmatch(meminfo)
	if m == nil {
		return 0, false
	}
	kb, err := strconv.ParseInt(string(m[1]), 10, 64)
	if err != nil {
		return 0, false
	}
	return float64(kb) * 1024 / 1e9, true
}

// Backups --------------------------------------------------------------------

// Backup rotates ace.db via VACUUM INTO into <sandboxDir>/backups/, keeping
// 24 hourly + 7 daily snapshots. Safe while the engine runs (SQLite WAL).
func Backup(ctx context.Context, sandboxDir string) (string, error) {
	dbPath := filepath.Join(sandboxDir, "ace.db")
	dir := filepath.Join(sandboxDir, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("ace-%s.db", time.Now().UTC().Format("20060102-1504"))
	dst := filepath.Join(dir, name)
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(8000)")
	if err != nil {
		return "", err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(dst, "'", "''"))); err != nil {
		return "", err
	}
	pruneBackups(dir)
	return dst, nil
}

func pruneBackups(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".db") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files) // name-ordered = time-ordered
	// Keep newest 24 hourly; of the rest keep one per day, max 7.
	if len(files) <= 24 {
		return
	}
	older := files[:len(files)-24]
	dailySeen := map[string]bool{}
	var drop []string
	for _, f := range older {
		day := f[4:12]
		if !dailySeen[day] && len(dailySeen) < 7 {
			dailySeen[day] = true
		} else {
			drop = append(drop, f)
		}
	}
	for _, f := range drop {
		_ = os.Remove(filepath.Join(dir, f))
	}
}
