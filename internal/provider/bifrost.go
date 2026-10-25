package provider

import (
	"context"
	"fmt"
	"time"

	"ace/internal/core"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
)

// bifrostAdapter wraps a bifrost client for one provider group whose adapter
// is "bifrost" — the group's Provider field names the bifrost provider
// (e.g. "opencode-go").
type bifrostAdapter struct {
	group    GroupCfg
	provider schemas.ModelProvider
	client   *bifrost.Bifrost
	timeout  time.Duration
}

type bifrostAccount struct {
	group GroupCfg
	key   string
	sess  string
}

func (a bifrostAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.ModelProvider(a.group.Provider)}, nil
}
func (a bifrostAccount) GetKeysForProvider(_ context.Context, _ schemas.ModelProvider) ([]schemas.Key, error) {
	return []schemas.Key{{ID: "env", Value: *schemas.NewSecretVar(a.key), Models: schemas.WhiteList{"*"}, Weight: 1}}, nil
}
func (a bifrostAccount) GetConfigForProvider(_ schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	// x-opencode-session is a required routing header on the opencode-go wire
	// (a fresh per-process id — session impersonation is gone with the hook).
	return &schemas.ProviderConfig{NetworkConfig: schemas.NetworkConfig{
		ExtraHeaders: map[string]schemas.SecretVar{"x-opencode-session": *schemas.NewSecretVar(a.sess)},
	}}, nil
}

// NewBifrostAdapter initialises the bifrost client for a group. The
// credential comes from the group's key_env; timeout is cloud/local.
func NewBifrostAdapter(ctx context.Context, g GroupCfg, apiKey string, timeoutSec int) (Adapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("group %s: no credential in %s", g.Name, g.KeyEnv)
	}
	sess := fmt.Sprintf("ace-%s-%d", g.Name, time.Now().Unix())
	client, err := bifrost.Init(ctx, schemas.BifrostConfig{
		Account: bifrostAccount{group: g, key: apiKey, sess: sess},
	})
	if err != nil {
		return nil, fmt.Errorf("bifrost init %s: %w", g.Name, err)
	}
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	return &bifrostAdapter{
		group:    g,
		provider: schemas.ModelProvider(g.Provider),
		client:   client,
		timeout:  time.Duration(timeoutSec) * time.Second,
	}, nil
}

func (a *bifrostAdapter) Name() string { return a.group.Name }

func (a *bifrostAdapter) Stream(ctx context.Context, modelID string, r Request) (core.CompletionResponse, error) {
	start := time.Now()
	msgs := make([]schemas.ChatMessage, 0, len(r.Req.Messages)+1)
	if r.Req.SystemPrompt != "" {
		msgs = append(msgs, schemas.ChatMessage{
			Role:    schemas.ChatMessageRoleSystem,
			Content: &schemas.ChatMessageContent{ContentStr: &r.Req.SystemPrompt},
		})
	}
	for _, m := range r.Req.Messages {
		role := schemas.ChatMessageRoleUser
		switch m.Role {
		case "system":
			role = schemas.ChatMessageRoleSystem
		case "assistant":
			role = schemas.ChatMessageRoleAssistant
		}
		msgs = append(msgs, schemas.ChatMessage{Role: role, Content: &schemas.ChatMessageContent{ContentStr: strptr(m.Content)}})
	}
	callCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	stream, berr := a.client.ChatCompletionStreamRequest(schemas.NewBifrostContext(callCtx, schemas.NoDeadline), &schemas.BifrostChatRequest{
		Provider: a.provider,
		Model:    modelID,
		Input:    msgs,
	})
	if berr != nil {
		return core.CompletionResponse{}, fmt.Errorf("%s: HTTP %d: %s", a.group.Name, berr.StatusCode, berr.Error.Message)
	}
	var content, reasoning string
	var in, cached, out int
	for {
		select {
		case chunk, ok := <-stream:
			if !ok {
				return core.CompletionResponse{
					Model: modelID, Content: content, Reasoning: reasoning,
					PromptTokens: in, CachedTokens: cached, CompletionTokens: out,
					LatencyMs: time.Since(start).Milliseconds(),
				}, nil
			}
			if chunk == nil {
				continue
			}
			if berr := chunk.BifrostError; berr != nil {
				return core.CompletionResponse{}, fmt.Errorf("%s: HTTP %d: %s", a.group.Name, berr.StatusCode, berr.Error.Message)
			}
			resp := chunk.BifrostChatResponse
			if resp == nil {
				continue
			}
			for _, c := range resp.Choices {
				if c.ChatStreamResponseChoice != nil && c.ChatStreamResponseChoice.Delta != nil {
					d := c.ChatStreamResponseChoice.Delta
					if d.Content != nil && *d.Content != "" {
						content += *d.Content
						if r.OnDelta != nil {
							r.OnDelta(core.StreamDelta{ContentDelta: *d.Content})
						}
					}
					if d.Reasoning != nil && *d.Reasoning != "" {
						reasoning += *d.Reasoning
						if r.OnDelta != nil {
							r.OnDelta(core.StreamDelta{ReasoningDelta: *d.Reasoning})
						}
					}
				}
			}
			if resp.Usage != nil {
				in = resp.Usage.PromptTokens
				out = resp.Usage.CompletionTokens
				if resp.Usage.PromptTokensDetails != nil {
					cached = resp.Usage.PromptTokensDetails.CachedReadTokens
				}
			}
		case <-callCtx.Done():
			return core.CompletionResponse{}, fmt.Errorf("%s: stream timeout", a.group.Name)
		}
	}
}

func strptr(s string) *string { return &s }
