//go:build spike
// +build spike

// Bifrost spike (spec 9.2): one live streaming call to OpenCode Go through
// Bifrost's native opencode-go provider. Run: ACE_BIFROST_SPIKE=1 go test
// -tags spike ./internal/provider/ -run TestBifrostSpike -v
package provider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
)

type spikeAccount struct{ key, session string }

func (a spikeAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpencodeGo}, nil
}
func (a spikeAccount) GetKeysForProvider(_ context.Context, p schemas.ModelProvider) ([]schemas.Key, error) {
	return []schemas.Key{{ID: "env", Value: *schemas.NewSecretVar(a.key), Models: schemas.WhiteList{"*"}, Weight: 1}}, nil
}
func (a spikeAccount) GetConfigForProvider(p schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	return &schemas.ProviderConfig{NetworkConfig: schemas.NetworkConfig{
		ExtraHeaders: map[string]schemas.SecretVar{"x-opencode-session": *schemas.NewSecretVar(a.session)},
	}}, nil
}

func TestBifrostSpike(t *testing.T) {
	if os.Getenv("ACE_BIFROST_SPIKE") == "" {
		t.Skip("spike off")
	}
	key := os.Getenv("ACE_LLM_API_KEY")
	if key == "" {
		t.Fatal("no ACE_LLM_API_KEY")
	}
	client, err := bifrost.Init(context.Background(), schemas.BifrostConfig{Account: spikeAccount{key: key, session: "ace-spike-" + time.Now().Format("150405")}})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	bctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	stream, berr := client.ChatCompletionStreamRequest(bctx, &schemas.BifrostChatRequest{
		Provider: schemas.OpencodeGo,
		Model:    "deepseek-v4.1-flash",
		Input: []schemas.ChatMessage{{
			Role:    schemas.ChatMessageRoleUser,
			Content: &schemas.ChatMessageContent{ContentStr: ptr("Reply with the word ok.")},
		}},
	})
	if berr != nil {
		t.Fatalf("stream: %+v", berr)
	}
	deadline := time.After(90 * time.Second)
	var text string
	var usage *schemas.BifrostLLMUsage
	for {
		select {
		case chunk, ok := <-stream:
			if !ok {
				fmt.Printf("text=%q usage=%+v\n", text, usage)
				if usage == nil || usage.TotalTokens == 0 {
					t.Fatal("no provider-reported usage")
				}
				return
			}
			if chunk == nil {
				continue
			}
			if chunk.BifrostChatResponse != nil {
				for _, c := range chunk.BifrostChatResponse.Choices {
					if c.ChatStreamResponseChoice != nil && c.ChatStreamResponseChoice.Delta != nil && c.ChatStreamResponseChoice.Delta.Content != nil {
						text += *c.ChatStreamResponseChoice.Delta.Content
					}
				}
				if chunk.BifrostChatResponse.Usage != nil {
					usage = chunk.BifrostChatResponse.Usage
				}
			}
		case <-deadline:
			t.Fatal("stream timeout")
		}
	}
}

func ptr(s string) *string { return &s }
