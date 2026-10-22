package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ace/internal/core"
	"github.com/stretchr/testify/require"
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
