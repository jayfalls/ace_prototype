package plan

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ace/internal/science"
)

// resetState returns the queue-entry state for a stage so a re-anchored node
// re-enters its own stage's pipeline.
var resetState = map[string]string{
	"L2_STRATEGY":   "NEEDS_L3_MODEL",
	"L3_CAPABILITY": "NEEDS_L4_SPEC",
	"L4_TASK_SPEC":  "READY_FOR_L5",
}

// Verdict is the L1 ruling on whether a stale node still serves the re-anchored
// mission — wired to gate.Ruling by the engine, stubbed in tests.
type Verdict func(ctx context.Context, nodeID, title string) (allow bool, reason string)

// SweepStale resolves every STALE dag node (spec 9.0): a node under a
// completed or duplicate parent is RETIRED with a registry row; otherwise it
// re-anchors once — new soul_hash, state reset, an L1 verdict — and a node
// that goes STALE a second time retires instead of oscillating.
func SweepStale(ctx context.Context, db *sql.DB, soulHash string, verdict Verdict) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, parent_id, stage, title, reanchor_count FROM mission_dag_nodes WHERE state='STALE'`)
	if err != nil {
		return 0, err
	}
	type staleNode struct {
		id, parent, stage, title string
		reanchors                int
	}
	var stale []staleNode
	for rows.Next() {
		var n staleNode
		if err := rows.Scan(&n.id, &n.parent, &n.stage, &n.title, &n.reanchors); err != nil {
			rows.Close()
			return 0, err
		}
		stale = append(stale, n)
	}
	rows.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	retire := func(n staleNode, reason string) error {
		if _, err := db.ExecContext(ctx, `UPDATE mission_dag_nodes SET state='RETIRED', retired_reason=?, updated_at=? WHERE id=?`, reason, now, n.id); err != nil {
			return err
		}
		return science.RegisterConcept(ctx, db, science.Concept{
			Name: n.id, Kind: "dag_node", Owner: "L1",
			Purpose: n.title, SupersededBy: "retired",
		})
	}

	for _, n := range stale {
		var parentState string
		_ = db.QueryRowContext(ctx, `SELECT state FROM mission_dag_nodes WHERE id=?`, n.parent).Scan(&parentState)
		switch {
		case parentState == "COMPLETED" || parentState == "DUPLICATE_OF" || parentState == "RETIRED":
			if err := retire(n, "parent "+n.parent+" "+parentState); err != nil {
				return 0, err
			}
		case n.reanchors >= 1:
			if err := retire(n, "stale after re-anchor"); err != nil {
				return 0, err
			}
		default:
			allow, reason := true, "no verdict"
			if verdict != nil {
				allow, reason = verdict(ctx, n.id, n.title)
			}
			if !allow {
				if err := retire(n, "re-anchor denied: "+reason); err != nil {
					return 0, err
				}
				continue
			}
			state := resetState[n.stage]
			if state == "" {
				return 0, fmt.Errorf("no reset state for stage %s", n.stage)
			}
			if _, err := db.ExecContext(ctx, `UPDATE mission_dag_nodes SET state=?, soul_hash=?, reanchor_count=reanchor_count+1, retired_reason=?, updated_at=? WHERE id=?`,
				state, soulHash, "re-anchored: "+reason, now, n.id); err != nil {
				return 0, err
			}
		}
	}
	return len(stale), nil
}
