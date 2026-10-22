package memory

import (
	"context"
	"database/sql"
	"testing"

	"ace/internal/core"
	"ace/internal/sandbox"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/require"
)

func TestStoreFTS5AndWikiAndChat(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	jail, err := sandbox.NewJail(t.TempDir())
	require.NoError(t, err)
	defer jail.Close()

	store, err := NewStore(db, jail.WikiRoot(), jail.WikiDir(), nil)
	require.NoError(t, err)

	// 1. Seeded wiki files exist
	soul, err := store.ReadWiki("soul.md")
	require.NoError(t, err)
	require.Contains(t, soul, "Scientist & Systems Researcher Constitution")

	// 2. Traversal in wiki is rejected
	_, err = store.ReadWiki("../../etc/passwd")
	require.Error(t, err)

	// 3. FTS5 search works
	_, err = store.Remember(context.Background(), core.MemoryEntry{
		LayerCode: "L2",
		Category:  "goal",
		Title:     "December Independence Goal",
		Content:   "Build a working single-binary ACE prototype with visual cockpit.",
		Tags:      "ace,milestone,december",
	})
	require.NoError(t, err)

	hits, err := store.Search(context.Background(), "December prototype", 5)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "December Independence Goal", hits[0].Title)

	// 4. Chat message persistence works
	err = store.SaveChatMessage(context.Background(), core.ChatMessage{
		Role:    "user",
		Source:  "user",
		Content: "Hello ACE!",
	})
	require.NoError(t, err)

	msgs, err := store.RecentChatMessages(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, "Hello ACE!", msgs[0].Content)
}
