package contextsys

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MapNode is one problem-space node: a file, doc, person, source, open
// question, spec or dataset (spec 9.5 map.go). Uncertainty-ledger and
// living-spec (9.6) entries are nodes with kind question/spec.
type MapNode struct {
	ID      string
	Kind    string // file | doc | person | source | question | spec | dataset
	Ref     string // path, url or record id
	Summary string
	Layers  int // bitmask: bit n => visible to layer n
}

// MapEdge links two nodes (depends_on, mentions, blocks, answers...).
type MapEdge struct {
	FromID string
	ToID   string
	Kind   string
}

// Map is the SQLite-backed problem-space map shared cluster-wide; each
// layer reads its own view through the Layers bitmask.
type Map struct {
	db *sql.DB
}

// NewMap binds the map to the engine DB.
func NewMap(db *sql.DB) *Map { return &Map{db: db} }

// MigrateMap creates the map tables.
func MigrateMap(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS map_nodes (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		ref TEXT NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		layers INTEGER NOT NULL DEFAULT 63,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS map_edges (
		from_id TEXT NOT NULL,
		to_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		PRIMARY KEY (from_id, to_id, kind)
	);
	CREATE INDEX IF NOT EXISTS idx_map_nodes_kind ON map_nodes(kind)`)
	return err
}

// LayerMask sets the visibility bit for layer n (1..6).
func LayerMask(layer int) int { return 1 << uint(layer-1) }

// AllLayers is every layer's bits.
const AllLayers = 63

// AddNode upserts a node.
func (m *Map) AddNode(ctx context.Context, n MapNode) error {
	if n.Layers == 0 {
		n.Layers = AllLayers
	}
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO map_nodes (id, kind, ref, summary, layers, created_at, updated_at)
		 VALUES (?,?,?,?,?,datetime('now'),datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET kind=excluded.kind, ref=excluded.ref,
		 summary=excluded.summary, layers=excluded.layers, updated_at=datetime('now')`,
		n.ID, n.Kind, n.Ref, n.Summary, n.Layers)
	return err
}

// AddEdge links two nodes (idempotent via PK).
func (m *Map) AddEdge(ctx context.Context, e MapEdge) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO map_edges (from_id, to_id, kind) VALUES (?,?,?)`,
		e.FromID, e.ToID, e.Kind)
	return err
}

// Node fetches one node.
func (m *Map) Node(ctx context.Context, id string) (MapNode, error) {
	var n MapNode
	err := m.db.QueryRowContext(ctx,
		`SELECT id, kind, ref, summary, layers FROM map_nodes WHERE id=?`, id).
		Scan(&n.ID, &n.Kind, &n.Ref, &n.Summary, &n.Layers)
	return n, err
}

// Frontier is the spec's deterministic tree search: from a region of
// interest it follows edges breadth-first and returns every downstream
// node in deterministic order (depth first, then id) that the given
// layer may see. The frontier feeds turn-0 injection.
func (m *Map) Frontier(ctx context.Context, roiID string, layer int, limit int) ([]MapNode, error) {
	if limit <= 0 {
		limit = 64
	}
	mask := LayerMask(layer)
	seen := map[string]bool{roiID: true}
	queue := []string{roiID}
	var out []MapNode
	for depth := 0; len(queue) > 0 && len(out) < limit; depth++ {
		var next []string
		var ids []string
		for _, id := range queue {
			ids = append(ids, id)
		}
		// Pull all children of this level in one query, deterministic order.
		ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		rows, err := m.db.QueryContext(ctx, fmt.Sprintf(
			`SELECT DISTINCT to_id FROM map_edges WHERE from_id IN (%s) ORDER BY to_id`, ph), anys(ids)...)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil && !seen[id] {
				seen[id] = true
				next = append(next, id)
			}
		}
		rows.Close()
		// Resolve this level's nodes, honoring the layer view.
		for _, id := range next {
			if len(out) >= limit {
				break
			}
			n, err := m.Node(ctx, id)
			if err != nil {
				continue
			}
			if n.Layers&mask != 0 {
				out = append(out, n)
			}
		}
		queue = next
	}
	return out, nil
}

// OpenItems returns nodes of the given kinds a layer can see — the
// closure ranking input (unanswered questions, untested assumptions).
func (m *Map) OpenItems(ctx context.Context, layer int, kinds ...string) ([]MapNode, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",")
	args := anys(kinds)
	args = append(args, LayerMask(layer))
	rows, err := m.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT id, kind, ref, summary, layers FROM map_nodes
		 WHERE kind IN (%s) AND (layers & ?) != 0 ORDER BY id`, ph), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MapNode
	for rows.Next() {
		var n MapNode
		if rows.Scan(&n.ID, &n.Kind, &n.Ref, &n.Summary, &n.Layers) == nil {
			out = append(out, n)
		}
	}
	return out, rows.Err()
}

func anys(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
