package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ace/internal/bus"
	"ace/internal/cognitive"
	"ace/internal/core"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/science"
	"ace/internal/telemetry"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type fakeLLM struct{}

func (f *fakeLLM) Name() string                { return "fake" }
func (f *fakeLLM) Models() []core.ModelInfo    { return []core.ModelInfo{{ID: "test"}} }
func (f *fakeLLM) ActiveModel() string         { return "test" }
func (f *fakeLLM) SetActiveModel(string) error { return nil }
func (f *fakeLLM) StreamComplete(ctx context.Context, req core.CompletionRequest, onDelta func(core.StreamDelta)) (core.CompletionResponse, error) {
	return core.CompletionResponse{Model: "test", Content: "Hello from ACE via Telegram!", LatencyMs: 10}, nil
}

func TestTelegramBridgeSlashCommandsAndChat(t *testing.T) {
	var sentMessages []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest_token_12345/getMe":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"result": map[string]string{"username": "ace_cog_bot", "first_name": "ACE"},
			})
		case "/bottest_token_12345/getUpdates":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"result": []interface{}{},
			})
		case "/bottest_token_12345/sendMessage":
			var body struct {
				Text string `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			sentMessages = append(sentMessages, body.Text)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer ts.Close()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, science.Migrate(db))

	jail, err := sandbox.NewJail(t.TempDir())
	require.NoError(t, err)
	defer jail.Close()

	telem, err := telemetry.NewSink(db)
	require.NoError(t, err)
	memBus, err := bus.NewMemoryBus(db, telem)
	require.NoError(t, err)
	memStore, err := memory.NewStore(db, jail.WikiRoot(), jail.WikiDir(), telem)
	require.NoError(t, err)

	eng := cognitive.NewEngine(&fakeLLM{}, memBus, memStore, jail, telem)
	eng.SetMode(core.ModeManualStep, 45)

	tg, err := NewTelegramBridge(db, eng, memStore, jail, telem, "", "")
	require.NoError(t, err)
	tg.baseURL = ts.URL

	st, err := tg.Configure(context.Background(), "test_token_12345", "999", true, true)
	require.NoError(t, err)
	require.True(t, st.Connected)
	require.Equal(t, "ace_cog_bot", st.BotUsername)

	// Human-voice invariant: proactive kinds enqueue as owner_offers for the
	// chat agent (never sent directly); chat replies still log sent rows.
	lastOfferRow := func() (kind, fact, status string) {
		_ = db.QueryRow(`SELECT kind, fact, status FROM owner_offers ORDER BY id DESC LIMIT 1`).Scan(&kind, &fact, &status)
		return
	}
	lastLogRow := func() (kind, text, status string) {
		_ = db.QueryRow(`SELECT kind, text, status FROM owner_msg_log ORDER BY id DESC LIMIT 1`).Scan(&kind, &text, &status)
		return
	}

	// 1. Unauthorized chat ID -> owner gets an offer about the stranger; stranger gets nothing
	tg.handleInboundMessage(context.Background(), 111, "stranger", "hi", 0)
	require.Empty(t, sentMessages)
	kind, fact, status := lastOfferRow()
	require.Equal(t, "reply", kind)
	require.Contains(t, fact, "unknown telegram account")
	require.Equal(t, "pending", status)

	// 2. Authorized /status command -> reply queued as an offer
	tg.handleInboundMessage(context.Background(), 999, "jay", "/status", 1)
	kind, fact, status = lastOfferRow()
	require.Equal(t, "reply", kind)
	require.Contains(t, fact, "/status")
	require.Equal(t, "pending", status)

	// 3. Authorized normal chat message -> model-written reply logged + sent
	tg.handleInboundMessage(context.Background(), 999, "jay", "Hello ACE", 2)
	kind, text, status := lastLogRow()
	require.Equal(t, "reply", kind)
	require.Equal(t, "sent", status)
	require.Equal(t, "Hello from ACE via Telegram!", text)
}

// An empty allowlist must deny every chat — v0.7 allowed all chats when unset.
func TestTelegramBridge_EmptyAllowlistDeniesAll(t *testing.T) {
	b := &TelegramBridge{}
	require.False(t, b.isChatAllowed(12345))
	require.False(t, b.isChatAllowed(0))

	b.allowedChatIDs = "  ,  "
	require.False(t, b.isChatAllowed(12345), "whitespace-only allowlist is still empty")

	b.allowedChatIDs = "777, 888"
	require.True(t, b.isChatAllowed(888))
	require.False(t, b.isChatAllowed(999))
}
