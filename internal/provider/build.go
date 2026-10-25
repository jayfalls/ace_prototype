package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigPath is provider.json inside the owner config dir — owner-set,
// read-only to the agent.
func ConfigPath(configDir string) string {
	return filepath.Join(configDir, "provider.json")
}

// Load reads provider.json from the owner config dir.
func Load(configDir string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(ConfigPath(configDir))
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(b, &cfg)
}

// DefaultConfig returns the working-config used when provider.json is
// absent: one opencode-go group on the bifrost adapter, ACE_LLM_API_KEY.
func DefaultConfig() Config {
	var c Config
	c.Groups = []GroupCfg{{
		Name: "opencode-go", Adapter: "bifrost", Provider: "opencode-go",
		Priority: 1, KeyEnv: "ACE_LLM_API_KEY", Billing: "subscription",
		Models: []string{"*"},
	}}
	c.Timeouts.Cloud, c.Timeouts.Local = 60, 300
	c.Breaker.Cooldown, c.Breaker.Max = 60, 300
	c.Retry.Base, c.Retry.Cap, c.Retry.Max = 1, 8, 2
	return c
}

// BuildAdapters constructs an adapter per group using env credentials.
// group.Adapter picks the implementation: "bifrost" (native provider impls)
// or "openai_compat" (raw OpenAI-compatible base_url). A group with a
// missing key is skipped with an error only when it's the highest-priority
// group — a missing credential on a fallback group just narrows the chain.
func BuildAdapters(ctx context.Context, cfg Config, cloudTimeout, localTimeout int) (map[string]Adapter, error) {
	out := map[string]Adapter{}
	var firstErr error
	for _, g := range cfg.Groups {
		timeout := g.Timeout
		if timeout <= 0 {
			timeout = cloudTimeout
			if g.Adapter == "local" {
				timeout = localTimeout
			}
		}
		key := ""
		if g.KeyEnv != "" {
			key = os.Getenv(g.KeyEnv)
		}
		var a Adapter
		var err error
		switch g.Adapter {
		case "bifrost":
			a, err = NewBifrostAdapter(ctx, g, key, timeout)
		case "openai_compat":
			a, err = NewOpenAICompatAdapter(g, key, timeout, "x-opencode-session")
		default:
			err = fmt.Errorf("group %s: unknown adapter %q", g.Name, g.Adapter)
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out[g.Name] = a
	}
	if len(out) == 0 {
		return nil, firstErr
	}
	return out, nil
}
