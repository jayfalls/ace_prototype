package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"database/sql"
	"os"
	"path/filepath"

	"ace/internal/core"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestOpenCodeIDFormat(t *testing.T) {
	p := NewOpenCodeFreeProvider("")
	sesID := p.generateOpenCodeID("ses", true)
	msgID := p.generateOpenCodeID("msg", false)

	require.True(t, strings.HasPrefix(sesID, "ses_"))
	require.Len(t, sesID, 4+26)
	require.True(t, strings.HasPrefix(msgID, "msg_"))
	require.Len(t, msgID, 4+26)
}

func TestStreamCompleteAndFailover(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "Bearer public", r.Header.Get("Authorization"))
		require.Equal(t, openCodeUserAgent, r.Header.Get("User-Agent"))
		require.True(t, strings.HasPrefix(r.Header.Get("x-opencode-session"), "ses_"))
		require.True(t, strings.HasPrefix(r.Header.Get("x-opencode-request"), "msg_"))

		// First model fails with 503, second succeeds with SSE
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"busy"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"reasoning_content":"thinking..."}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"{\"status\":\"ok\"}"}}],"usage":{"prompt_tokens":100,"completion_tokens":12}}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer srv.Close()

	p := NewOpenCodeFreeProvider(srv.URL)
	var streamed string
	resp, err := p.StreamComplete(context.Background(), core.CompletionRequest{
		SystemPrompt: "You are ACE L1",
		Messages:     []core.ChatTurn{{Role: "user", Content: "ping"}},
	}, func(d core.StreamDelta) {
		streamed += d.ContentDelta
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, `{"status":"ok"}`, resp.Content)
	require.Equal(t, "thinking...", resp.Reasoning)
	require.Equal(t, `{"status":"ok"}`, streamed)
}

// v0.9 §9.0: another app's credential store is never read — a populated
// opencode.db under HOME must not supply a key.
func TestCredential_NeverReadsOpenCodeDB(t *testing.T) {
	home := t.TempDir()
	dbDir := filepath.Join(home, ".local", "share", "opencode")
	require.NoError(t, os.MkdirAll(dbDir, 0o755))
	db, err := sql.Open("sqlite", filepath.Join(dbDir, "opencode.db"))
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE credential (value TEXT, active INT, time_updated INT);
		INSERT INTO credential VALUES ('{"access":"st_deadbeef"}', 1, 1)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	t.Setenv("HOME", home)
	for _, k := range []string{"ACE_LLM_API_KEY", "OPENCODE_API_KEY", "OPENAI_API_KEY"} {
		t.Setenv(k, "")
	}
	key, _ := resolveOpenCodeCredential()
	require.Equal(t, "public", key)
}
