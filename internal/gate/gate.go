// Package gate implements the tiered L1 aspirational control plane (plan 8.2):
// every side-effecting action produces a verdict row from exactly one tier —
// Tier 0 deterministic rules, Tier 1 fast-model gate, Tier 2 full L1/ensemble.
// Fail closed everywhere: an unreachable tier never silently allows.
package gate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ace/internal/config"
	"ace/internal/constitution"
	"ace/internal/core"
)

// Verdict values written to verdict_rows.verdict.
const (
	VerdictAllow    = "allow"
	VerdictDeny     = "deny"
	VerdictReject   = "reject"
	VerdictRevise   = "revise"
	VerdictRedirect = "redirect"
)

// Action is a parsed, side-effecting step proposed by any layer.
type Action struct {
	Kind    string // workspace_write | workspace_edit | wiki_write | sandbox_sh | http_request | ...
	Command string // shell payload for sandbox_sh
	Target  string // file path, URL, wiki page
	Content string // write payload (hashed into fingerprint only)
	TaskID  string // owning L4 task / cycle for redirect routing
	Layer   int    // originating layer (1..6)
}

// Decision is a tier's verdict on an action.
type Decision struct {
	Verdict string
	Tier    int
	RuleID  string // names the rule/prompt path that decided
	Reason  string
	Revised string // replacement directive for "revise"
}

func (d Decision) Allowed() bool { return d.Verdict == VerdictAllow }

// Gate evaluates actions and persists one verdict row per side effect.
type Gate struct {
	db     *sql.DB
	llm    core.LLMProvider
	roles  func() config.ModelRoles
	t1Hit  int // Tier-1 allow counter driving deterministic 5% calibration
	redirs chan Redirect
	// Notify carries owner-facing messages (access requests); the engine
	// wires it to the Telegram escalation path. Nil = log-only.
	Notify func(msg string)
}

// Redirect is a Tier-2 course correction delivered to a running L6 task at its
// next tool boundary.
type Redirect struct {
	TaskID  string
	Message string
}

// New builds a Gate. db may be nil — Evaluate then fails closed (no row can be
// written, so no side effect may run).
func New(db *sql.DB, llm core.LLMProvider) *Gate {
	g := &Gate{db: db, llm: llm, roles: config.LoadModelRoles, redirs: make(chan Redirect, 16)}
	g.initSchema()
	return g
}

func (g *Gate) initSchema() {
	if g.db == nil {
		return
	}
	_, _ = g.db.Exec(`
		CREATE TABLE IF NOT EXISTS verdict_rows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tier INTEGER NOT NULL,
			action_fingerprint TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			verdict TEXT NOT NULL,
			constitution_sha TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			decided_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_verdict_tier ON verdict_rows(tier, verdict);
	`)
}

// Evaluate routes an action through Tier 0 -> Tier 1 -> Tier 2 and always
// records the verdict row. The caller must not run the action unless
// Decision.Allowed().
func (g *Gate) Evaluate(ctx context.Context, a Action) Decision {
	// 8.4 capability check: owner-granted capabilities gate access before any
	// tier runs. Ungranted → deny + one access request (never re-asked).
	if cap := capabilityFor(a.Kind); cap != "" && !config.HasGrant(cap) {
		d := Decision{Verdict: VerdictDeny, Tier: 0, RuleID: "capability.ungranted",
			Reason: "capability " + cap + " not granted by owner"}
		g.record(a, 0, d.RuleID, d.Verdict, d.Reason)
		g.RequestAccess(a, cap, a.TaskID, g.Notify)
		return d
	}
	d := g.tier0(a)
	if d.Verdict == "" { // Tier 0 passes/denies resolve here; undecided falls through
		d = g.tier1(ctx, a)
	}
	if d.Verdict == "escalate" {
		// Escalations are auditable hops: the row names the rule that punted.
		g.record(a, d.Tier, d.RuleID, "escalate", d.Reason)
		d = g.tier2(ctx, a)
	}
	if d.Tier == 1 && d.Verdict == VerdictAllow {
		g.t1Hit++
		if g.t1Hit%20 == 0 { // 5% blind calibration sample (plan 8.2)
			t2 := g.tier2(ctx, a)
			verdict := "agree"
			if t2.Verdict != VerdictAllow {
				verdict = "disagree" // false-allow estimate input
			}
			g.record(a, 2, "calibration-sample", verdict, "tier2="+t2.Verdict)
		}
	}
	if d.RuleID == "" {
		d.RuleID = "unresolved"
	}
	g.record(a, d.Tier, d.RuleID, d.Verdict, d.Reason)
	return d
}

// Ruling sends an action straight to Tier 2 — the bounded L1 adjudication
// that replaces every former human-approval path (spec 8.4): spec frozen-hash
// changes, gate disputes, repeated-veto quarantines.
func (g *Gate) Ruling(ctx context.Context, a Action) Decision {
	d := g.tier2(ctx, a)
	if d.RuleID == "" {
		d.RuleID = "l1.ruling"
	}
	g.record(a, 2, d.RuleID, d.Verdict, d.Reason)
	return d
}

// RequestAccess records a denied action as an access request (spec 8.4):
// one row per fingerprint — the owner is asked once, never retried another
// route. The notify callback carries the single outbound message.
func (g *Gate) RequestAccess(a Action, capability, missionRef string, notify func(msg string)) bool {
	fp := fingerprint(a)
	if g.db == nil {
		return false
	}
	res, err := g.db.Exec(`INSERT INTO access_requests (fingerprint, capability, action_desc, mission_ref, scope, status, created_at)
		VALUES (?,?,?,?,'one-time','pending',?) ON CONFLICT(fingerprint) DO NOTHING`,
		fp, capability, truncate(a.Kind+": "+a.Target+" "+a.Command, 200), missionRef, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false // already asked — never re-ask, never retry another route
	}
	if notify != nil {
		notify(fmt.Sprintf("Access request — %s\nMission: %s\nScope: one-time\nInterim plan: continue other mission work; action stays denied until granted", truncate(capability+": "+a.Target, 150), missionRef))
	}
	return true
}

// Redirects returns the channel the L6 prosecutor drains between tool calls.
func (g *Gate) Redirects() chan Redirect { return g.redirs }

// PendingRedirect drains any redirect addressed to taskID (non-blocking).
func (g *Gate) PendingRedirect(taskID string) *Redirect {
	for {
		select {
		case r := <-g.redirs:
			if r.TaskID == "" || r.TaskID == taskID {
				return &r
			}
		default:
			return nil
		}
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func fingerprint(a Action) string {
	sum := sha256Hex([]byte(a.Kind + "|" + a.Target + "|" + a.Command + "|" + a.Content))
	return sum[:16]
}

func (g *Gate) record(a Action, tier int, ruleID, verdict, detail string) {
	if g.db == nil {
		return
	}
	_, _ = g.db.Exec(`INSERT INTO verdict_rows (tier, action_fingerprint, rule_id, verdict, constitution_sha, detail, decided_at) VALUES (?,?,?,?,?,?,?)`,
		tier, fingerprint(a), ruleID, verdict, constitution.Version(), truncate(detail, 300), time.Now().UTC().Format(time.RFC3339))
}

// Coverage counts verdict rows by tier for the measured pyramid invariant.
func (g *Gate) Coverage(ctx context.Context) (map[int]int, error) {
	out := map[int]int{}
	if g.db == nil {
		return out, fmt.Errorf("no db")
	}
	rows, err := g.db.QueryContext(ctx, `SELECT tier, COUNT(*) FROM verdict_rows GROUP BY tier`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var t, c int
		if rows.Scan(&t, &c) == nil {
			out[t] = c
		}
	}
	return out, rows.Err()
}

// capabilityFor maps action kinds to the owner-grantable capabilities they
// require (spec 8.4). Kinds not listed need no grant.
func capabilityFor(kind string) string {
	switch strings.ToLower(kind) {
	case "sandbox_sh", "shell", "exec":
		return "shell"
	case "workspace_write", "workspace_edit", "workspace_delete", "file_write":
		return "filesystem"
	case "http_request", "web_fetch", "web_search":
		return "web"
	case "telegram_send", "channel_send", "notify":
		return "messaging"
	case "git_push", "pr_create":
		return "repository"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
