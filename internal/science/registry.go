// registry.go — §8.8 concept registry: every live flow, module, doc and
// skill listed with owner, purpose and supersedes/superseded_by so agents
// stop reading dead code and stale docs as live.
package science

import (
	"context"
	"database/sql"
)

// Concept is one registry row: a live flow, module, doc or skill.
type Concept struct {
	Name         string
	Kind         string // flow | module | doc | skill
	Owner        string
	Purpose      string
	Supersedes   string
	SupersededBy string
}

// RegisterConcept upserts a live concept; MarkSuperseded points it at its
// replacement. Registry rows are facts, not suggestions.
func RegisterConcept(ctx context.Context, db *sql.DB, c Concept) error {
	if db == nil || c.Name == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, `INSERT INTO concept_registry (name, kind, owner, purpose, supersedes, superseded_by)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET kind=excluded.kind, owner=excluded.owner, purpose=excluded.purpose,
			supersedes=excluded.supersedes, superseded_by=excluded.superseded_by`,
		c.Name, c.Kind, c.Owner, c.Purpose, c.Supersedes, c.SupersededBy)
	return err
}

func MarkSuperseded(ctx context.Context, db *sql.DB, name, by string) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx, `UPDATE concept_registry SET superseded_by=? WHERE name=?`, by, name)
	return err
}

// Concepts returns the full registry ordered by kind then name.
func Concepts(ctx context.Context, db *sql.DB) ([]Concept, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, kind, owner, purpose, supersedes, superseded_by FROM concept_registry ORDER BY kind, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Concept
	for rows.Next() {
		var c Concept
		if rows.Scan(&c.Name, &c.Kind, &c.Owner, &c.Purpose, &c.Supersedes, &c.SupersededBy) == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// StaleReferenceCount is the §8.8 release metric: registry rows marked
// superseded whose name still resolves to a live path (exists(name) true).
func StaleReferenceCount(db *sql.DB, exists func(string) bool) int {
	if db == nil {
		return -1
	}
	rows, err := db.Query(`SELECT name FROM concept_registry WHERE superseded_by != ''`)
	if err != nil {
		return -1
	}
	defer rows.Close()
	stale := 0
	for rows.Next() {
		var name string
		if rows.Scan(&name) != nil {
			continue
		}
		if exists != nil && exists(name) {
			stale++
		}
	}
	return stale
}

// SeedRegistry registers the known live components once (INSERT OR IGNORE —
// owner edits and runtime registrations are preserved).
func SeedRegistry(ctx context.Context, db *sql.DB) {
	if db == nil {
		return
	}
	seeds := []Concept{
		{"internal/constitution", "module", "ace", "code-owned constitution, sha-pinned at boot", "", ""},
		{"internal/gate", "module", "ace", "tiered L1 verdict gate (T0/T1/T2)", "internal/cognitive L1 inline veto", ""},
		{"internal/science", "module", "ace", "scientist gate, prereg ledger, stats, metrics, registry", "", ""},
		{"internal/watchdog", "module", "ace", "out-of-process liveness/integrity watchdog", "", ""},
		{"internal/cognitive", "module", "ace", "six-layer engine, mission DAG, sensorium, connectome", "", ""},
		{"internal/channels/telegram.go", "module", "ace", "owner channel: directives, grants, reports", "", ""},
		{"internal/config", "module", "ace", "owner-set config: mission, grants, model roles", "", ""},
		{"docs/SPEC_ACE_V0_8.md", "doc", "owner", "v0.8 build spec", "PLAN_ACE_V0_8_TO_V0_12.md §8", ""},
		{"docs/PLAN_ACE_V0_8_TO_V0_12.md", "doc", "owner", "version roadmap", "", ""},
		{"docs/AUDIT_2026_10_24_GAP_PLAN.md", "doc", "owner", "v0.7.1 audit findings", "", ""},
		{"archive/v0_7_decommissioned_20261024", "module", "owner", "retired v0.7 experiment harness", "v0.7 live code", ""},
	}
	for _, c := range seeds {
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO concept_registry (name, kind, owner, purpose, supersedes, superseded_by) VALUES (?,?,?,?,?,?)`,
			c.Name, c.Kind, c.Owner, c.Purpose, c.Supersedes, c.SupersededBy)
	}
}
