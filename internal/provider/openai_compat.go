package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ace/internal/core"
)

// openaiCompatAdapter is the thin OpenAI-compatible streaming client moved
// out of internal/llm (spec 9.2): POST + SSE + provider-reported usage.
// Every impersonation piece is gone — no CLI user-agent, no gate preamble,
// no stub tools, no persisted session IDs. A fresh per-process session id is
// sent only because the opencode-go wire requires the routing header.
type openaiCompatAdapter struct {
	group      GroupCfg
	endpoint   string
	apiKey     string
	sessionID  string
	httpClient *http.Client
	timeout    time.Duration
}

type compatPayload struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens"`
	Messages      []core.ChatTurn `json:"messages"`
	Stream        bool            `json:"stream"`
	StreamOptions map[string]bool `json:"stream_options"`
}

type compatChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens         int `json:"prompt_tokens"`
		CompletionTokens     int `json:"completion_tokens"`
		PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
		PromptTokensDetails  *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

// NewOpenAICompatAdapter builds an adapter for a group's base_url + key_env.
// sessionHeader names the routing header the wire requires (opencode-go:
// x-opencode-session); empty = none sent.
func NewOpenAICompatAdapter(g GroupCfg, apiKey string, timeoutSec int, sessionHeader string) (Adapter, error) {
	if g.BaseURL == "" {
		return nil, fmt.Errorf("group %s: base_url required", g.Name)
	}
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	return &openaiCompatAdapter{
		group:     g,
		endpoint:  g.BaseURL,
		apiKey:    apiKey,
		sessionID: fmt.Sprintf("ace-%s-%d", g.Name, time.Now().Unix()),
		httpClient: &http.Client{
			Timeout:   time.Duration(timeoutSec) * time.Second,
			Transport: &http.Transport{MaxIdleConns: 4, IdleConnTimeout: 60 * time.Second},
		},
		timeout: time.Duration(timeoutSec) * time.Second,
	}, nil
}

func (a *openaiCompatAdapter) Name() string { return a.group.Name }

func (a *openaiCompatAdapter) Stream(ctx context.Context, modelID string, r Request) (core.CompletionResponse, error) {
	start := time.Now()
	maxTokens := r.Req.MaxTokens
	if maxTokens < 512 {
		maxTokens = 4096
	}
	msgs := make([]core.ChatTurn, 0, len(r.Req.Messages)+1)
	if strings.TrimSpace(r.Req.SystemPrompt) != "" {
		msgs = append(msgs, core.ChatTurn{Role: "system", Content: r.Req.SystemPrompt})
	}
	msgs = append(msgs, r.Req.Messages...)

	rawBody, err := json.Marshal(compatPayload{
		Model: modelID, MaxTokens: maxTokens, Messages: msgs,
		Stream: true, StreamOptions: map[string]bool{"include_usage": true},
	})
	if err != nil {
		return core.CompletionResponse{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, a.endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return core.CompletionResponse{}, err
	}
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// The opencode-go wire requires this routing header (protocol, not
	// impersonation): fresh per-process id, never persisted or replayed.
	httpReq.Header.Set("x-opencode-session", a.sessionID)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return core.CompletionResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return core.CompletionResponse{}, fmt.Errorf("model %s HTTP %d: %s", modelID, resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var contentBuf, reasoningBuf strings.Builder
	var promptToks, compToks, cachedToks int
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		if line == "data: [DONE]" {
			break
		}
		var chunk compatChunk
		if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			promptToks = chunk.Usage.PromptTokens
			compToks = chunk.Usage.CompletionTokens
			if chunk.Usage.PromptCacheHitTokens > 0 {
				cachedToks = chunk.Usage.PromptCacheHitTokens
			} else if chunk.Usage.PromptTokensDetails != nil && chunk.Usage.PromptTokensDetails.CachedTokens > 0 {
				cachedToks = chunk.Usage.PromptTokensDetails.CachedTokens
			}
		}
		for _, ch := range chunk.Choices {
			cDelta := ch.Delta.Content
			rDelta := ch.Delta.ReasoningContent
			if cDelta != "" {
				contentBuf.WriteString(cDelta)
			}
			if rDelta != "" {
				reasoningBuf.WriteString(rDelta)
				if contentBuf.Len() == 0 && reasoningBuf.Len() > 48000 {
					cancel() // reasoning runaway guard
					break
				}
			}
			if r.OnDelta != nil && (cDelta != "" || rDelta != "") {
				r.OnDelta(core.StreamDelta{ContentDelta: cDelta, ReasoningDelta: rDelta})
			}
		}
	}
	if scanErr := scanner.Err(); scanErr != nil && strings.TrimSpace(contentBuf.String()) == "" {
		return core.CompletionResponse{}, fmt.Errorf("model %s stream aborted: %w", modelID, scanErr)
	}
	return core.CompletionResponse{
		Model:            modelID,
		Content:          strings.TrimSpace(contentBuf.String()),
		Reasoning:        strings.TrimSpace(reasoningBuf.String()),
		PromptTokens:     promptToks,
		CachedTokens:     cachedToks,
		CompletionTokens: compToks,
		LatencyMs:        time.Since(start).Milliseconds(),
	}, scanner.Err()
}
