package memory

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"ace/internal/core"
	"ace/internal/sandbox"

	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	jail, err := sandbox.NewJail(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { jail.Close() })
	s, err := NewStore(db, jail.WikiRoot(), jail.WikiDir(), nil)
	require.NoError(t, err)
	return s
}

// Spec test: an L3 write to L1-layer memory is rejected at the storage
// boundary — not in prompts.
func TestL3WriteToL1MemoryRejected(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Remember(context.Background(), "L3", core.MemoryEntry{
		LayerCode: "L1", Category: "insight", Title: "unauthorized", Content: "x",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "write denied")

	// Same-layer, L1-anywhere, and any-layer-to-GLOBAL writes still work.
	_, err = s.Remember(context.Background(), "L3", core.MemoryEntry{
		LayerCode: "L3", Category: "insight", Title: "own layer", Content: "x",
	})
	require.NoError(t, err)
	_, err = s.Remember(context.Background(), "L1", core.MemoryEntry{
		LayerCode: "L3", Category: "insight", Title: "l1 anywhere", Content: "x",
	})
	require.NoError(t, err)
	_, err = s.Remember(context.Background(), "L6", core.MemoryEntry{
		LayerCode: "GLOBAL", Category: "episodic", Title: "l6 shared", Content: "x",
	})
	require.NoError(t, err)
}

// Spec test: a caller-stamped updated_at older than the stored row is
// rejected as stale — monotonic writes in storage.
func TestStaleWriteRejected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, err := s.Remember(ctx, "L1", core.MemoryEntry{
		LayerCode: "L2", Category: "goal", Title: "fact X", Content: "v1",
	})
	require.NoError(t, err)

	// An update stamped BEFORE the stored row must be rejected.
	_, err = s.Remember(ctx, "L1", core.MemoryEntry{
		LayerCode: "L2", Category: "goal", Title: "fact X", Content: "v0-stale",
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "stale write")
}

// Spec test: over the episodic cap, the lowest retention score is
// evicted — importance × 0.5^(age/30d).
func TestLowestRetentionEvictedOverCap(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	old := EpisodicCapEntries
	EpisodicCapEntries = 3
	defer func() { EpisodicCapEntries = old }()

	// 3 entries: two fresh high-importance, one old low-importance.
	low, err := s.Remember(ctx, "L1", core.MemoryEntry{
		LayerCode: "L1", Type: "episodic", Category: "episodic",
		Title: "ancient low-value", Content: "x", Importance: 0.01,
		CreatedAt: time.Now().UTC().Add(-90 * 24 * time.Hour),
	})
	require.NoError(t, err)
	for _, title := range []string{"fresh one", "fresh two"} {
		_, err = s.Remember(ctx, "L1", core.MemoryEntry{
			LayerCode: "L1", Type: "episodic", Category: "episodic",
			Title: title, Content: "x", Importance: 5,
		})
		require.NoError(t, err)
	}

	// The fourth write pushes over the cap → the ancient low-importance
	// row has the lowest retention score and is evicted.
	_, err = s.Remember(ctx, "L1", core.MemoryEntry{
		LayerCode: "L1", Type: "episodic", Category: "episodic",
		Title: "fresh three", Content: "x", Importance: 5,
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories WHERE COALESCE(type,'episodic')='episodic'`).Scan(&n))
	require.Equal(t, 3, n)
	var exists int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE id = ?`, low.ID).Scan(&exists)
	require.Equal(t, 0, exists, "lowest-retention entry must be evicted")
}

// Semantic cap: over 2,200 B rejected.
func TestSemanticCapRejected(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Remember(context.Background(), "L1", core.MemoryEntry{
		LayerCode: "L1", Type: "semantic", Category: "fact",
		Title: "too long", Content: strings.Repeat("x", 2201),
	})
	require.Error(t, err)
}
