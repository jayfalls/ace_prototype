package plan

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// Spec 9.4: each plan/DAG node stores assumptions[] (claim ids). A new fact
// touching a claim marks only that node's subtree STALE — siblings keep
// their state — and SweepStale replans only that subtree.

// EnsureAssumptionsColumn adds the assumptions column (tolerated ALTER —
// same migration style as mission_dag.go).
func EnsureAssumptionsColumn(db *sql.DB) {
	_, _ = db.Exec(`ALTER TABLE mission_dag_nodes ADD COLUMN assumptions TEXT NOT NULL DEFAULT '[]'`)
}

// SetAssumptions replaces a node's claim-id list.
func SetAssumptions(ctx context.Context, db *sql.DB, nodeID string, claimIDs []string) error {
	b, err := json.Marshal(claimIDs)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`UPDATE mission_dag_nodes SET assumptions=? WHERE id=?`, string(b), nodeID)
	return err
}

// NodeAssumptions returns a node's claim ids.
func NodeAssumptions(ctx context.Context, db *sql.DB, nodeID string) ([]string, error) {
	var raw string
	if err := db.QueryRowContext(ctx,
		`SELECT assumptions FROM mission_dag_nodes WHERE id=?`, nodeID).Scan(&raw); err != nil {
		return nil, err
	}
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out, nil
}

// MarkStaleByClaim marks the subtree of every node whose assumptions
// contain claimID as STALE. Siblings keep their state — only descendants
// (recursive parent_id walk) of a touched node change.
func MarkStaleByClaim(ctx context.Context, db *sql.DB, claimID string) (int, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, assumptions FROM mission_dag_nodes WHERE state NOT IN ('COMPLETED','STALE','RETIRED','DUPLICATE_OF')`)
	if err != nil {
		return 0, err
	}
	type row struct{ id, assumptions string }
	var all []row
	for rows.Next() {
		var r row
		if rows.Scan(&r.id, &r.assumptions) == nil {
			all = append(all, r)
		}
	}
	rows.Close()

	// Roots to stale: nodes whose assumptions mention the claim.
	roots := map[string]bool{}
	for _, r := range all {
		if strings.Contains(r.assumptions, `"`+claimID+`"`) {
			roots[r.id] = true
		}
	}
	if len(roots) == 0 {
		return 0, nil
	}

	// Walk children transitively over the whole live set.
	children := map[string][]string{}
	if childRows, err := db.QueryContext(ctx,
		`SELECT id, parent_id FROM mission_dag_nodes WHERE state NOT IN ('COMPLETED','STALE','RETIRED','DUPLICATE_OF')`); err == nil {
		for childRows.Next() {
			var id, parent string
			if childRows.Scan(&id, &parent) == nil && parent != "" {
				children[parent] = append(children[parent], id)
			}
		}
		childRows.Close()
	}
	marked := map[string]bool{}
	stack := []string{}
	for r := range roots {
		stack = append(stack, r)
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if marked[id] {
			continue
		}
		marked[id] = true
		stack = append(stack, children[id]...)
	}

	n := 0
	for id := range marked {
		if _, err := db.ExecContext(ctx,
			`UPDATE mission_dag_nodes SET state='STALE', updated_at=datetime('now') WHERE id=?`, id); err == nil {
			n++
		}
	}
	return n, nil
}
