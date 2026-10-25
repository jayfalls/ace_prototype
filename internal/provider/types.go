// Package provider is the v0.9 provider registry (spec 9.2): a role-routed
// fan-out over adapters (bifrost, openai_compat) with per-group breakers,
// classified retries, provider-reported usage accounting and quota windows.
package provider

import (
	"context"

	"ace/internal/core"
)

// Role is a routing lane: main drives L1–L6 reasoning, fast covers cheap
// side calls; per-layer roles (l1..l6) come from config.
type Role string

const (
	RoleMain      Role = "main"
	RoleFast      Role = "fast"
	RoleReasoning Role = "reasoning" // L1–L2
	RoleGeneral   Role = "general"   // L3–L4
	RoleEfficient Role = "efficient" // L5
	RoleTool      Role = "tool"      // L6
)

// Request wraps the wire request with routing metadata the registry needs —
// the port (core.CompletionRequest) stays unchanged.
type Request struct {
	Req     core.CompletionRequest
	Role    Role
	OnDelta func(core.StreamDelta)
}

// Adapter is one provider-group client (spec 9.2).
type Adapter interface {
	Name() string
	Stream(ctx context.Context, modelID string, r Request) (core.CompletionResponse, error)
}

// ModelRef is one routable model on one group.
type ModelRef struct {
	Group   string `json:"group"`
	Model   string `json:"model"`
	Billing string `json:"billing"` // subscription | metered | free
}

// GroupCfg is one provider.groups[] entry.
type GroupCfg struct {
	Name     string   `json:"name"`
	Adapter  string   `json:"adapter"`  // bifrost | openai_compat
	Provider string   `json:"provider"` // bifrost provider id (e.g. opencode-go); openai_compat ignores
	Priority int      `json:"priority"` // lower = tried first
	BaseURL  string   `json:"base_url"` // openai_compat endpoint
	KeyEnv   string   `json:"key_env"`  // env var holding the credential
	Billing  string   `json:"billing"`  // subscription | metered | free
	Models   []string `json:"models"`   // served ids; ["*"] = the adapter's whole catalogue
	Timeout  int      `json:"timeout"`  // seconds; 0 = group default
}

// ModelsCfg is provider.json's models block: which model each role uses.
type ModelsCfg struct {
	Main        string          `json:"main"`
	Fast        []string        `json:"fast"`
	Roles       map[Role]string `json:"roles"`
	Temperature struct {
		Efficient float64 `json:"efficient"` // logic roles, default 0.2
		Creative  float64 `json:"creative"`  // main, default 0.7
	} `json:"temperature"`
}

// Config is provider.json: groups + models + timeouts + breaker + retry + quota.
type Config struct {
	Groups   []GroupCfg `json:"groups"`
	Models   ModelsCfg  `json:"models"`
	Timeouts struct {
		Cloud int `json:"cloud"` // seconds, default 60
		Local int `json:"local"` // seconds, default 300
	} `json:"timeout"`
	Breaker struct {
		Cooldown int `json:"cooldown"` // seconds, default 60
		Max      int `json:"max"`      // seconds, default 300
	} `json:"breaker"`
	Retry struct {
		Base float64 `json:"base"` // seconds, default 1
		Cap  float64 `json:"cap"`  // seconds, default 8
		Max  int     `json:"max"`  // retries, default 2
	} `json:"retry"`
	Quota map[string]map[string]float64 `json:"quota"` // group -> window -> % limit
}
