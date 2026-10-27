package cognitive

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"ace/internal/bus"
	"ace/internal/core"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/science"
	"ace/internal/telemetry"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// replayProvider scripts the two seams the live model owns: (a) folding
// pending offers into ONE human message, built by extracting the numbers
// straight out of the facts it was handed — so the sent text provably
// carries what the DB computed; (b) answering the owner by calling
// ace_read("results") first and quoting the tool output back, so the
// reply provably came from a lookup, not from memory or invention.
type replayProvider struct{}

func (replayProvider) Name() string           { return "replay" }
func (replayProvider) ActiveModel() string    { return "replay-model" }
func (replayProvider) SetActiveModel(string) error { return nil }
func (replayProvider) Models() []core.ModelInfo {
	return []core.ModelInfo{{ID: "replay-model", Name: "Replay"}}
}

var reFactNumbers = regexp.MustCompile(`([0-9.]+) → ([0-9.]+) \(n=(\d+), (p=[0-9.e+-]+)[^)]*\), result file (\S+)`)

func (replayProvider) StreamComplete(ctx context.Context, req core.CompletionRequest, onDelta func(core.StreamDelta)) (core.CompletionResponse, error) {
	if strings.Contains(req.SystemPrompt, "Tier-1 constitutional fast gate") {
		return core.CompletionResponse{Model: "replay", Content: "ALLOW", LatencyMs: 1}, nil
	}
	if strings.Contains(req.SystemPrompt, "Tier-2 L1 Aspirational gate") {
		return core.CompletionResponse{Model: "replay", Content: `{"verdict":"allow","reason":"replay"}`, LatencyMs: 1}, nil
	}
	if os.Getenv("ACE_DEBUG_REPLAY") != "" {
		fmt.Fprintf(os.Stderr, "[replay] msgs=%d last=%q\n", len(req.Messages), truncateText(lastOf(req.Messages), 140))
	}
	last := ""
	sawResults := false
	for _, m := range req.Messages {
		if strings.Contains(m.Content, "BRANCH RESULTS") {
			sawResults = true
			last = m.Content
		}
	}
	if len(req.Messages) > 0 && !sawResults {
		last = req.Messages[len(req.Messages)-1].Content
	}
	switch {
	// The proactive turn: fold every offered fact into one plain message.
	case strings.Contains(last, "Things that happened inside ACE"):
		matches := reFactNumbers.FindAllStringSubmatch(last, -1)
		if len(matches) == 0 {
			return core.CompletionResponse{Model: "replay", Content: "HOLD"}, nil
		}
		var parts []string
		for _, m := range matches {
			parts = append(parts, fmt.Sprintf("held-out accuracy %s → %s (n=%s, %s, file %s)", m[1], m[2], m[3], m[4], m[5]))
		}
		msg := "Two checks verified and the numbers actually moved: " + strings.Join(parts, " — and ") + "."
		return core.CompletionResponse{Model: "replay", Content: msg}, nil

	// First chat pass: the model looks it up before answering.
	case !strings.Contains(last, "BRANCH RESULTS"):
		return core.CompletionResponse{Model: "replay", Content: `<action>{"type":"ace_read","target":"results"}</action>`}, nil

	// Synthesis pass: quote the lookup back to the owner.
	default:
		row := reReplayRow(last)
		if row == "" {
			return core.CompletionResponse{Model: "replay", Content: "I couldn't find the numbers — checking again."}, nil
		}
		return core.CompletionResponse{Model: "replay", Content: "Here's what actually landed: " + row}, nil
	}
}

func lastOf(ms []core.ChatTurn) string {
	if len(ms) == 0 {
		return ""
	}
	return ms[len(ms)-1].Content
}

var reRow = regexp.MustCompile(`held-out accuracy \| [0-9.]+ \| [0-9.]+ \| \d+ \| p=[0-9.e+-]+ \| \S+\.json`)

func reReplayRow(s string) string {
	return reRow.FindString(s)
}

// TestOwnerChannelReplay reruns the real 13:46–13:50 window on a copied
// database: three verified completions + the owner's two follow-ups. It
// must produce ONE combined proactive message carrying real numbers and
// file names, then replies that quote the same rows — looked up via
// ace_read, not invented. Run with:
//
//	ACE_REPLAY_DB=/tmp/ace_replay.db ACE_REPLAY_SANDBOX=.ace-sandbox \
//	  go test ./internal/cognitive -run TestOwnerChannelReplay -v
func TestOwnerChannelReplay(t *testing.T) {
	src := os.Getenv("ACE_REPLAY_DB")
	wsDir := os.Getenv("ACE_REPLAY_SANDBOX")
	if src == "" || wsDir == "" {
		t.Skip("set ACE_REPLAY_DB (a VACUUM INTO copy) and ACE_REPLAY_SANDBOX (the live sandbox dir)")
	}
	db, err := sql.Open("sqlite", src+"?_pragma=busy_timeout(2000)")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, science.Migrate(db)) // the copy predates owner_offers

	// Replay state: the three completions become unread news; the channel
	// is disabled so nothing can reach real Telegram. created_at is moved
	// to now — the 15-minute news bound would (correctly) expire rows that
	// really are a day old.
	_, err = db.Exec(`UPDATE branch_progress SET notified=0, created_at=? WHERE id IN (30,31,32)`,
		time.Now().UTC().Format("2006-01-02 15:04:05"))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE channel_settings SET enabled=0`)
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM owner_offers`)
	require.NoError(t, err)
	// The copy persists between runs — clear prior-run log rows so
	// assertions count only this replay's output.
	_, err = db.Exec(`DELETE FROM owner_msg_log`)
	require.NoError(t, err)

	wsAbs, err := filepath.Abs(wsDir) // NewJail keeps the given path — reads
	require.NoError(t, err)           // resolve against the test's CWD otherwise
	jail, err := sandbox.NewJail(wsAbs)
	require.NoError(t, err)
	defer jail.Close()
	telem, err := telemetry.NewSink(db)
	require.NoError(t, err)
	memBus, err := bus.NewMemoryBus(db, telem)
	require.NoError(t, err)
	memStore, err := memory.NewStore(db, jail.WikiRoot(), jail.WikiDir(), telem)
	require.NoError(t, err)

	eng := NewEngine(replayProvider{}, memBus, memStore, jail, telem)
	ctx := context.Background()

	// --- the completions become offers with full facts -------------------
	eng.maybeNotifyOwnerResults(ctx)
	var offerCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM owner_offers WHERE status='pending'`).Scan(&offerCount))
	t.Logf("offers queued: %d", offerCount)
	var facts []string
	rows, err := db.Query(`SELECT fact FROM owner_offers WHERE status='pending' ORDER BY id`)
	require.NoError(t, err)
	for rows.Next() {
		var f string
		_ = rows.Scan(&f)
		facts = append(facts, f)
	}
	rows.Close()
	for _, f := range facts {
		t.Logf("offer fact: %s", f)
	}

	// --- one combined proactive message ----------------------------------
	var chatBefore int
	_ = db.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE role='assistant' AND source='proactive'`).Scan(&chatBefore)
	eng.drainOwnerOffers(ctx)
	var sent int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM owner_msg_log WHERE kind='offer' AND status='sent'`).Scan(&sent))
	require.Equal(t, 1, sent, "three completions in four minutes must fold into ONE send")
	var msg string
	require.NoError(t, db.QueryRow(`SELECT text FROM owner_msg_log WHERE kind='offer' AND status='sent'`).Scan(&msg))
	t.Logf("proactive message: %s", msg)
	require.Contains(t, msg, "1.000")
	require.Contains(t, msg, "c_s31_data.json")
	// S32's p=0.1094 ≥ α=0.05 is "no clear effect yet" — digest, not a win.
	require.NotContains(t, msg, "c_s32_data.json")
	var digestRow int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM owner_msg_log WHERE status='digest' AND fact LIKE '%no clear effect%'`).Scan(&digestRow))
	require.Equal(t, 2, digestRow,
		"S32 (p=0.109≥α) and S31_HARNESS (no movement vs branch's new 1.0) must both digest")

	// The sent text is in the SAME history the chat agent reads.
	var chatRow int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM chat_messages WHERE role='assistant' AND source='proactive' AND content=?`, msg).Scan(&chatRow))
	require.GreaterOrEqual(t, chatRow, 1)
	var chatAfter int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE role='assistant' AND source='proactive'`).Scan(&chatAfter))
	require.Equal(t, chatBefore+1, chatAfter, "exactly one proactive row lands in chat_messages")

	// --- the owner's two follow-ups --------------------------------------
	m1, err := eng.Chat(ctx, "How many pts?")
	require.NoError(t, err)
	t.Logf("reply 1: %s", m1.Content)
	require.Contains(t, m1.Content, "0.5")

	m2, err := eng.Chat(ctx, "You said it moved the main number for this branch")
	require.NoError(t, err)
	t.Logf("reply 2: %s", m2.Content)
	require.True(t,
		strings.Contains(m2.Content, ".json") || strings.Contains(m2.Content, "held-out"),
		"reply must quote the result file or the statistic, got: %s", m2.Content)
}
