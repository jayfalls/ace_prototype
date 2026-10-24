package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ace/internal/core"
)

const (
	defaultOpenCodeEndpoint   = "https://opencode.ai/inference/openai/v1/chat/completions"
	defaultOpenCodeGoEndpoint = "https://opencode.ai/inference/go/openai/v1/chat/completions"
	openCodeUserAgent         = "opencode/1.18.21 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14"
	openCodeGatePreamble      = "You are opencode, an interactive CLI tool that helps users with software engineering tasks. Use the following instructions. Respond ONLY with a single raw JSON object in assistant text content. Never invoke tool_calls."
	base62Chars               = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// resolveOpenCodeCredential checks env vars first, then ~/.local/share/opencode/opencode.db (populated by `opencode auth login`).
func resolveOpenCodeCredential() (apiKey string, isGoSub bool) {
	for _, k := range []string{"ACE_LLM_API_KEY", "OPENCODE_API_KEY", "OPENAI_API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" && v != "public" {
			return v, strings.HasPrefix(v, "st_") || strings.HasPrefix(v, "oc_")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "public", false
	}
	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if _, err := os.Stat(dbPath); err != nil {
		return "public", false
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return "public", false
	}
	defer db.Close()
	var raw string
	if err := db.QueryRow(`SELECT value FROM credential WHERE active = 1 ORDER BY time_updated DESC LIMIT 1`).Scan(&raw); err != nil {
		return "public", false
	}
	var cred struct {
		Access string `json:"access"`
	}
	if json.Unmarshal([]byte(raw), &cred) == nil && strings.TrimSpace(cred.Access) != "" {
		return strings.TrimSpace(cred.Access), true
	}
	return "public", false
}

var stubToolNames = []string{
	"bash", "edit", "glob", "grep", "read", "skill", "task", "todowrite", "webfetch", "websearch", "write",
}

type wireToolStub struct {
	Type     string           `json:"type"`
	Function wireFunctionStub `json:"function"`
}

type wireFunctionStub struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

func buildStubTools() []wireToolStub {
	out := make([]wireToolStub, 0, len(stubToolNames))
	for _, name := range stubToolNames {
		out = append(out, wireToolStub{
			Type: "function",
			Function: wireFunctionStub{
				Name:        name,
				Description: "DISABLED STUB — NEVER CALL THIS FUNCTION OR YOUR STREAM WILL ABORT. Write all actions as JSON text inside assistant message content.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		})
	}
	return out
}

type modelHealth struct {
	successCount            int64
	failureCount            int64
	consecutiveFailures     int64
	consecutiveTaskFailures int64
	taskAlpha               float64
	taskBeta                float64
	ewmaCacheRatio          float64
	totalLatencyMs          int64
	cooldownUntil           time.Time
}

type cachedCompletion struct {
	resp      core.CompletionResponse
	expiresAt time.Time
}

// OpenCodeFreeProvider calls OpenCode's free inference tier directly via HTTPS
// without spawning or depending on the OpenCode CLI/server harness.
type OpenCodeFreeProvider struct {
	mu                    sync.RWMutex
	endpoint              string
	httpClient            *http.Client
	models                []core.ModelInfo
	health                map[string]*modelHealth
	activeModel           string
	sessionID             string
	compassSessionID      string
	layerSessionIDs       map[string]string
	idCounter             atomic.Uint64
	recentPrefixesByModel map[string][]string
	exactCache            map[string]cachedCompletion
	providerCooldownUntil time.Time
}

func DefaultModels() []core.ModelInfo {
	_, isGoSub := resolveOpenCodeCredential()
	if isGoSub {
		return []core.ModelInfo{
			{
				ID:          "deepseek-v4.1-flash",
				Name:        "DeepSeek V4.1 Flash (OpenCode Go 1M)",
				Provider:    "opencode-go",
				Modalities:  []string{"text", "image"},
				Reasoning:   true,
				ContextSize: 1000000,
				OutputSize:  384000,
			},
			{
				ID:          "beam-free",
				Name:        "Beam Fast (OpenCode Go Free)",
				Provider:    "opencode-go-free",
				Modalities:  []string{"text"},
				Reasoning:   true,
				ContextSize: 1048576,
				OutputSize:  131072,
			},
			{
				ID:          "cat-jam-free",
				Name:        "Cat Jam (OpenCode Go Free)",
				Provider:    "opencode-go-free",
				Modalities:  []string{"text"},
				Reasoning:   true,
				ContextSize: 1048576,
				OutputSize:  131072,
			},
		}
	}
	return []core.ModelInfo{
		{
			ID:          "deepseek-v4.1-flash-free",
			Name:        "DeepSeek V4.1 Flash (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text", "image"},
			Reasoning:   true,
			ContextSize: 262144,
			OutputSize:  65536,
		},
		{
			ID:          "glm-5.3-flash-free",
			Name:        "GLM 5.3 Flash (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text"},
			Reasoning:   true,
			ContextSize: 262144,
			OutputSize:  65536,
		},
		{
			ID:          "beam-free",
			Name:        "Beam Fast (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text"},
			Reasoning:   true,
			ContextSize: 1048576,
			OutputSize:  131072,
		},
		{
			ID:          "minimax-m3.1-flash-free",
			Name:        "MiniMax M3.1 Flash (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text"},
			Reasoning:   true,
			ContextSize: 262144,
			OutputSize:  65536,
		},
		{
			ID:          "nemotron-3.5-super-vl-free",
			Name:        "Nemotron 3.5 Super VL (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text", "image"},
			Reasoning:   true,
			ContextSize: 1048576,
			OutputSize:  131072,
		},
		{
			ID:          "longcat-2.5-preview-free",
			Name:        "LongCat 2.5 Preview (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text", "image"},
			Reasoning:   true,
			ContextSize: 1000000,
			OutputSize:  131072,
		},
		{
			ID:          "big-pickle",
			Name:        "Big Pickle (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text"},
			Reasoning:   true,
			ContextSize: 262144,
			OutputSize:  65536,
		},
		{
			ID:          "nemotron-3.5-lightning-free",
			Name:        "Nemotron 3.5 Lightning (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text"},
			Reasoning:   false,
			ContextSize: 262144,
			OutputSize:  65536,
		},
		{
			ID:          "nemotron-3-ultra-free",
			Name:        "Nemotron 3 Ultra (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text"},
			Reasoning:   false,
			ContextSize: 1000000,
			OutputSize:  128000,
		},
		{
			ID:          "mimo-v2.6-flash-free",
			Name:        "MiMo V2.6 Flash Multimodal (Free)",
			Provider:    "opencode-free",
			Modalities:  []string{"text", "image", "audio", "video"},
			Reasoning:   false,
			ContextSize: 262144,
			OutputSize:  65536,
		},
	}
}

func NewOpenCodeFreeProvider(endpoint string) *OpenCodeFreeProvider {
	if strings.TrimSpace(endpoint) == "" {
		if envEP := strings.TrimSpace(os.Getenv("ACE_LLM_ENDPOINT")); envEP != "" {
			endpoint = envEP
		} else if envBase := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); envBase != "" {
			endpoint = strings.TrimRight(envBase, "/") + "/chat/completions"
		} else if _, isGo := resolveOpenCodeCredential(); isGo {
			endpoint = defaultOpenCodeGoEndpoint
		} else {
			endpoint = defaultOpenCodeEndpoint
		}
	}
	models := DefaultModels()
	health := make(map[string]*modelHealth, len(models))
	for i := range models {
		models[i].CircuitState = "closed"
		priorAlpha := 4.0
		if !models[i].Reasoning {
			priorAlpha = 2.0
		}
		health[models[i].ID] = &modelHealth{
			taskAlpha:      priorAlpha,
			taskBeta:       1.0,
			ewmaCacheRatio: 0.75,
		}
	}
	p := &OpenCodeFreeProvider{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 190 * time.Second,
		},
		models:                models,
		health:                health,
		activeModel:           models[0].ID,
		recentPrefixesByModel: make(map[string][]string),
		exactCache:            make(map[string]cachedCompletion),
	}
	p.sessionID = p.generateOpenCodeID("ses", true)
	p.compassSessionID = p.generateOpenCodeID("ses", true)
	p.layerSessionIDs = map[string]string{
		"L1":     p.generateOpenCodeID("ses", true),
		"L2":     p.generateOpenCodeID("ses", true),
		"L3":     p.generateOpenCodeID("ses", true),
		"L4":     p.generateOpenCodeID("ses", true),
		"L5":     p.generateOpenCodeID("ses", true),
		"L6":     p.sessionID,
		"TURN05": p.generateOpenCodeID("ses", true),
	}
	return p
}

func (p *OpenCodeFreeProvider) SessionID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionID
}

func (p *OpenCodeFreeProvider) SetSessionID(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	p.mu.Lock()
	p.sessionID = id
	if p.compassSessionID == "" || p.compassSessionID == id {
		if len(id) > 6 {
			last := id[len(id)-1]
			rep := byte('A')
			if last == 'A' {
				rep = 'B'
			}
			p.compassSessionID = id[:len(id)-1] + string(rep)
		} else {
			p.compassSessionID = p.generateOpenCodeID("ses", true)
		}
	}
	p.mu.Unlock()
}

func (p *OpenCodeFreeProvider) Name() string {
	return "opencode-free"
}

func (p *OpenCodeFreeProvider) Models() []core.ModelInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	now := time.Now()
	out := make([]core.ModelInfo, len(p.models))
	copy(out, p.models)
	for i := range out {
		if h, ok := p.health[out[i].ID]; ok {
			out[i].SuccessCount = h.successCount
			out[i].FailureCount = h.failureCount
			if h.successCount > 0 {
				out[i].AvgLatencyMs = h.totalLatencyMs / h.successCount
			}
			if now.Before(h.cooldownUntil) {
				out[i].CircuitState = "open"
			} else {
				out[i].CircuitState = "closed"
			}
		}
	}
	return out
}

func (p *OpenCodeFreeProvider) ActiveModel() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activeModel
}

func (p *OpenCodeFreeProvider) SetActiveModel(modelID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, m := range p.models {
		if m.ID == modelID {
			p.activeModel = modelID
			return nil
		}
	}
	return fmt.Errorf("unknown model: %s", modelID)
}

// BetaLower95 computes the 95% credible lower bound of Beta(alpha, beta).
func BetaLower95(alpha, betaParam float64) float64 {
	a := math.Max(0.5, alpha)
	b := math.Max(0.5, betaParam)
	tot := a + b
	mean := a / tot
	variance := (a * b) / (tot * tot * (tot + 1.0))
	low := mean - 1.96*math.Sqrt(math.Max(0, variance))
	return math.Max(0.05, math.Min(0.99, low))
}

// ExpectedCostOfPass computes the expected cost-of-pass for a candidate model (Rule A8/S19).
func (p *OpenCodeFreeProvider) ExpectedCostOfPass(modelID string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.expectedCostOfPassLocked(modelID)
}

func (p *OpenCodeFreeProvider) expectedCostOfPassLocked(modelID string) float64 {
	h := p.health[modelID]
	if h == nil {
		return 10000.0
	}
	pLow := BetaLower95(h.taskAlpha, h.taskBeta)
	warmDiscount := 0.0
	if modelID == p.activeModel {
		warmDiscount = h.ewmaCacheRatio
	}
	basePromptCost := (1.0 - 0.70*warmDiscount) * 6000.0
	latencyPenalty := 0.0
	if h.successCount > 0 {
		avgMs := float64(h.totalLatencyMs) / float64(h.successCount)
		// Penalize slow models (e.g. >20s/step) so Bayesian routing strongly favors low-latency models (<12s/step)
		latencyPenalty = (avgMs / 1000.0) * 180.0
	}
	reasoningPenalty := 0.0
	for _, m := range p.models {
		if m.ID == modelID && !m.Reasoning {
			reasoningPenalty = 3500.0
			break
		}
	}
	return (basePromptCost + latencyPenalty + reasoningPenalty) / math.Max(0.05, pLow)
}

// RecordTaskOutcome feeds empirical L6 task pass/fail outcomes back into the Bayesian Beta(alpha, beta) router (Fix S19).
func (p *OpenCodeFreeProvider) RecordTaskOutcome(modelID string, passed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h, ok := p.health[modelID]
	if !ok {
		h = &modelHealth{taskAlpha: 2.0, taskBeta: 1.0, ewmaCacheRatio: 0.75}
		p.health[modelID] = h
	}
	if passed {
		h.taskAlpha += 1.0
		h.consecutiveTaskFailures = 0
		return
	}
	h.taskBeta += 1.0
	h.consecutiveTaskFailures++
	// If the main model is a subscription model (e.g. deepseek-v4.1-flash), NEVER switch activeModel to a free model!
	if len(p.models) > 0 && !isFreeTierModelID(p.models[0].ID) {
		p.activeModel = p.models[0].ID
		return
	}
	// If a model fails >= 2 consecutive L6 task steps, put it on a 3-minute task cooldown and promote the best healthy Reasoning model!
	if h.consecutiveTaskFailures >= 2 {
		h.cooldownUntil = time.Now().Add(3 * time.Minute)
		now := time.Now()
		bestID := ""
		bestCoP := math.MaxFloat64
		for _, m := range p.models {
			if m.ID == modelID {
				continue
			}
			mh := p.health[m.ID]
			if mh != nil && now.Before(mh.cooldownUntil) {
				continue
			}
			cop := p.expectedCostOfPassLocked(m.ID)
			if cop < bestCoP {
				bestCoP = cop
				bestID = m.ID
			}
		}
		if bestID != "" {
			p.activeModel = bestID
		}
	}
}

func isFreeTierModelID(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return strings.HasSuffix(id, "-free") || id == "big-pickle" || id == "grok-code"
}

func (p *OpenCodeFreeProvider) FallbackChain(preferred string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	primary := strings.TrimSpace(preferred)
	if primary == "" {
		primary = p.activeModel
	}

	// STRICT TWO-WAY TIER ISOLATION LAW (SPEC_ACE_V0_6_FINAL.md §8):
	// 1. If the caller requested the main OpenCode Go subscription model (`deepseek-v4.1-flash`),
	//    NEVER fall back to a free model! Use `deepseek-v4.1-flash` until subscription usage is done.
	if !isFreeTierModelID(primary) {
		return []string{primary}
	}

	// 2. If the caller requested a Free-tier model (e.g. `beam-free` for Turn 0.5 router),
	//    ONLY fall back to other `$0` Free-tier models (`beam-free` -> `cat-jam-free`), NEVER into a paid model!
	now := time.Now()
	var closedChain []string
	var openChain []string

	addCandidate := func(id string) {
		if !isFreeTierModelID(id) {
			return
		}
		h := p.health[id]
		if h != nil && now.Before(h.cooldownUntil) {
			openChain = append(openChain, id)
		} else {
			closedChain = append(closedChain, id)
		}
	}
	addCandidate(primary)
	for _, m := range p.models {
		if m.ID != primary && isFreeTierModelID(m.ID) {
			addCandidate(m.ID)
		}
	}
	return append(closedChain, openChain...)
}

func (p *OpenCodeFreeProvider) recordModelOutcome(modelID string, ok bool, latencyMs int64, isRateLimit bool, isDailyQuota bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h, exists := p.health[modelID]
	if !exists {
		h = &modelHealth{taskAlpha: 2.0, taskBeta: 1.0, ewmaCacheRatio: 0.75}
		p.health[modelID] = h
	}
	if ok {
		h.successCount++
		h.consecutiveFailures = 0
		h.totalLatencyMs += latencyMs
		h.cooldownUntil = time.Time{}
		// If primary model is a subscription model (deepseek-v4.1-flash), never let a free router call (beam-free) hijack activeModel!
		if len(p.models) > 0 && !isFreeTierModelID(p.models[0].ID) {
			p.activeModel = p.models[0].ID
			return
		}
		// Only sticky-promote a model if it is a Reasoning model OR no Reasoning model is currently healthy (Fix S19)
		isReasoning := false
		for _, m := range p.models {
			if m.ID == modelID && m.Reasoning {
				isReasoning = true
				break
			}
		}
		if isReasoning {
			// Only sticky-keep a fallback reasoning model as activeModel if its latency was <= 28s OR primary model is on cooldown
			if modelID == p.models[0].ID || latencyMs <= 28000 {
				p.activeModel = modelID
			} else if topH := p.health[p.models[0].ID]; topH != nil && time.Now().Before(topH.cooldownUntil) {
				p.activeModel = modelID
			} else {
				p.activeModel = p.models[0].ID
			}
		} else {
			now := time.Now()
			anyReasoningHealthy := false
			for _, m := range p.models {
				if m.Reasoning {
					if mh := p.health[m.ID]; mh == nil || !now.Before(mh.cooldownUntil) {
						anyReasoningHealthy = true
						break
					}
				}
			}
			if !anyReasoningHealthy {
				p.activeModel = modelID
			}
		}
	} else {
		h.failureCount++
		h.consecutiveFailures++
		if isDailyQuota {
			h.cooldownUntil = time.Now().Add(30 * time.Minute)
		} else if isRateLimit {
			h.cooldownUntil = time.Now().Add(90 * time.Second)
		} else if h.consecutiveFailures >= 2 {
			h.cooldownUntil = time.Now().Add(45 * time.Second)
		}
	}
}

// StreamComplete executes a streaming completion with circuit-breaker failover across OpenCode Free models.
func (p *OpenCodeFreeProvider) StreamComplete(
	ctx context.Context,
	req core.CompletionRequest,
	onDelta func(core.StreamDelta),
) (core.CompletionResponse, error) {
	candidates := p.FallbackChain(req.Model)

	expectJSON := strings.Contains(req.SystemPrompt, "OUTPUT CONTRACT") ||
		(len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "OUTPUT CONTRACT"))
	isValidDecisionJSON := func(s string) bool {
		s = strings.TrimSpace(s)
		if !strings.Contains(s, "{") {
			return false
		}
		hasField := strings.Contains(s, "\"action\"") || strings.Contains(s, "\"actions\"") ||
			strings.Contains(s, "\"southbound_directive\"") || strings.Contains(s, "\"state_summary\"") ||
			strings.Contains(s, "DSML")
		return hasField
	}
	extractValidJSONFromReasoning := func(r string) string {
		end := strings.LastIndex(r, "}")
		if end <= 0 {
			return ""
		}
		for pos := end - 1; pos >= 0 && end-pos <= 48000; pos-- {
			if r[pos] == '{' {
				cand := strings.TrimSpace(r[pos : end+1])
				if isValidDecisionJSON(cand) && json.Valid([]byte(cand)) {
					return cand
				}
			}
		}
		return ""
	}

	p.mu.RLock()
	cdUntil := p.providerCooldownUntil
	p.mu.RUnlock()
	if time.Now().Before(cdUntil) {
		rem := int(time.Until(cdUntil).Seconds())
		if rem < 1 {
			rem = 1
		}
		return core.CompletionResponse{}, fmt.Errorf("opencode provider cooling down (%ds remaining) to clear sliding-window rate limit", rem)
	}

	var lastErr error
	streamStart := time.Now()
	rlHitsInTurn := 0
	for idx, modelID := range candidates {
		// Fast-Failover Tail Guard: if candidate #0 already spent >= 28s, skip slow cold-cache reasoning models
		// and jump directly to a fast non-reasoning lightning model so a single step never cascades past ~60s!
		if idx >= 1 && time.Since(streamStart) >= 28*time.Second {
			isFastFallback := modelID == "nemotron-3.5-lightning-free" || modelID == "nemotron-3-ultra-free"
			if !isFastFallback && idx < len(candidates)-1 {
				continue
			}
		}
		callReq := req
		if idx >= 1 && len(req.Messages) > 0 {
			msgsCopy := make([]core.ChatTurn, len(req.Messages))
			copy(msgsCopy, req.Messages)
			lastIdx := len(msgsCopy) - 1
			msgsCopy[lastIdx].Content += "\n\n[FAST FALLBACK MODE: Keep internal reasoning under 60 words and emit the required JSON object immediately.]"
			callReq.Messages = msgsCopy
		}
		attemptStart := time.Now()
		resp, err := p.callSingleModel(ctx, modelID, callReq, onDelta)
		if err == nil && (!isValidDecisionJSON(resp.Content) || strings.TrimSpace(resp.Content) == "") && strings.TrimSpace(resp.Reasoning) != "" {
			if candJSON := extractValidJSONFromReasoning(resp.Reasoning); candJSON != "" {
				resp.Content = candJSON
			}
		}
		if err == nil && expectJSON && !isValidDecisionJSON(resp.Content) {
			err = fmt.Errorf("model %s truncated before action/decision JSON fields (len=%d)", modelID, len(resp.Content))
		}
		// Sticky 1-Retry Jitter: retry once if the primary model failed fast (<8s) OR if there is only 1 candidate in the tier (subscription mode) and parent ctx is still live!
		if (err != nil || strings.TrimSpace(resp.Content) == "") && idx == 0 && ctx.Err() == nil && (time.Since(attemptStart) < 8*time.Second || len(candidates) == 1) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			if !strings.Contains(errStr, "FreeUsageLimitError") && !strings.Contains(errStr, "HTTP 429") {
				time.Sleep(150 * time.Millisecond)
				resp, err = p.callSingleModel(ctx, modelID, req, onDelta)
				if err == nil && (!isValidDecisionJSON(resp.Content) || strings.TrimSpace(resp.Content) == "") && strings.TrimSpace(resp.Reasoning) != "" {
					if candJSON := extractValidJSONFromReasoning(resp.Reasoning); candJSON != "" {
						resp.Content = candJSON
					}
				}
				if err == nil && expectJSON && !isValidDecisionJSON(resp.Content) {
					err = fmt.Errorf("model %s truncated before action/decision JSON fields (len=%d)", modelID, len(resp.Content))
				}
			}
		}
		if err == nil && strings.TrimSpace(resp.Content) != "" {
			p.recordModelOutcome(modelID, true, resp.LatencyMs, false, false)
			return resp, nil
		}
		if err == nil {
			err = fmt.Errorf("model %s returned empty content (reasoning_len=%d)", modelID, len(resp.Reasoning))
		}
		errLower := ""
		if err != nil {
			errLower = strings.ToLower(err.Error())
		}
		isQuota := err != nil && strings.Contains(errLower, "daily")
		isRL := err != nil && (isQuota || strings.Contains(err.Error(), "FreeUsageLimitError") || strings.Contains(err.Error(), "HTTP 429"))
		p.recordModelOutcome(modelID, false, 0, isRL, isQuota)
		if err != nil {
			lastErr = err
		}
		if isRL {
			rlHitsInTurn++
			// Endpoint-wide rate limit on opencode.ai: do NOT hammer 8 more models in a row!
			// Honor Retry-After header (capped at 5m per poll so we re-probe periodically) so 0 requests are wasted while rate-limited.
			if rlHitsInTurn >= 2 {
				waitDur := 65 * time.Second
				if err != nil {
					if idxRA := strings.Index(err.Error(), "Retry-After="); idxRA >= 0 {
						rest := err.Error()[idxRA+len("Retry-After="):]
						if end := strings.IndexAny(rest, " )]"); end > 0 {
							rest = rest[:end]
						}
						if sec, pErr := strconv.Atoi(strings.TrimSpace(rest)); pErr == nil && sec > 0 {
							if sec > 300 {
								sec = 300
							}
							waitDur = time.Duration(sec) * time.Second
						}
					}
				}
				p.mu.Lock()
				p.providerCooldownUntil = time.Now().Add(waitDur)
				p.mu.Unlock()
				break
			}
			time.Sleep(600 * time.Millisecond)
		}
		if ctx.Err() != nil {
			return core.CompletionResponse{}, ctx.Err()
		}
	}
	if lastErr == nil {
		lastErr = errors.New("all candidate models returned empty response")
	}
	return core.CompletionResponse{}, lastErr
}

// ReadoutD1Logprobs implements LiquidAI/d1-3B 1-token logprob readout:
// P(yes) = 1.0 / (1.0 + exp(max(logprob_no) - max(logprob_yes)))
// Matches ("yes", "Yes", "YES") vs ("no", "No", "NO") without leading-space forms.
func ReadoutD1Logprobs(topLogprobs map[string]float64) (float64, bool) {
	var yesMax, noMax float64
	hasYes, hasNo := false, false
	for _, tok := range []string{"yes", "Yes", "YES"} {
		if lp, ok := topLogprobs[tok]; ok {
			if !hasYes || lp > yesMax {
				yesMax = lp
				hasYes = true
			}
		}
	}
	for _, tok := range []string{"no", "No", "NO"} {
		if lp, ok := topLogprobs[tok]; ok {
			if !hasNo || lp > noMax {
				noMax = lp
				hasNo = true
			}
		}
	}
	if !hasYes || !hasNo {
		return 0, false
	}
	pYes := 1.0 / (1.0 + math.Exp(noMax-yesMax))
	return pYes, true
}

// EvaluateBinaryGate runs a fast binary decision gate (LiquidAI/d1-3B vLLM if DECISION_MODEL_URL is set,
// or non-reasoning OpenCode fast model nemotron-3-ultra-free with prefix-cached context first).
func (p *OpenCodeFreeProvider) EvaluateBinaryGate(ctx context.Context, sharedContext string, question string) (float64, bool, error) {
	if d1URL := strings.TrimSpace(os.Getenv("DECISION_MODEL_URL")); d1URL != "" {
		reqBody := map[string]interface{}{
			"model": "LiquidAI/d1-3B",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": fmt.Sprintf("%s\n\n%s\n\nReply with yes or no only.", sharedContext, question),
				},
			},
			"max_tokens":   1,
			"temperature":  0.0,
			"logprobs":     true,
			"top_logprobs": 20,
		}
		raw, _ := json.Marshal(reqBody)
		hReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(d1URL, "/")+"/chat/completions", bytes.NewReader(raw))
		if err == nil {
			hReq.Header.Set("Content-Type", "application/json")
			if resp, err := p.httpClient.Do(hReq); err == nil {
				defer resp.Body.Close()
				var payload struct {
					Choices []struct {
						Logprobs struct {
							Content []struct {
								TopLogprobs []struct {
									Token   string  `json:"token"`
									Logprob float64 `json:"logprob"`
								} `json:"top_logprobs"`
							} `json:"content"`
						} `json:"logprobs"`
					} `json:"choices"`
				}
				if json.NewDecoder(resp.Body).Decode(&payload) == nil && len(payload.Choices) > 0 && len(payload.Choices[0].Logprobs.Content) > 0 {
					lpMap := make(map[string]float64)
					for _, entry := range payload.Choices[0].Logprobs.Content[0].TopLogprobs {
						lpMap[entry.Token] = entry.Logprob
					}
					if pYes, ok := ReadoutD1Logprobs(lpMap); ok {
						return pYes, pYes >= 0.5, nil
					}
				}
			}
		}
	}

	// Fast non-reasoning gate on OpenCode (sharedContext placed first for KV prefix cache hit)
	resp, err := p.callSingleModel(ctx, "nemotron-3-ultra-free", core.CompletionRequest{
		Model:        "nemotron-3-ultra-free",
		SystemPrompt: "You are a LiquidAI/d1-3B style binary decision gate. Output ONLY 'YES <prob>' or 'NO <prob>' (0.00 to 1.00) on a single line.",
		Messages: []core.ChatTurn{
			{Role: "user", Content: fmt.Sprintf("%s\n\nGate Question: %s\nReply with YES <prob> or NO <prob> only.", sharedContext, question)},
		},
		MaxTokens: 2048,
	}, nil)
	if err != nil {
		return 0.5, true, err
	}
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(resp.Content)))
	if len(fields) == 0 {
		return 0.5, true, nil
	}
	isYes := strings.HasPrefix(fields[0], "Y")
	prob := 0.85
	if len(fields) > 1 {
		if parsed, err := strconv.ParseFloat(fields[1], 64); err == nil && parsed >= 0 && parsed <= 1 {
			prob = parsed
		}
	}
	if !isYes && prob > 0.5 {
		prob = 1.0 - prob
	}
	return prob, isYes, nil
}

type wirePayload struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens"`
	Messages      []core.ChatTurn `json:"messages"`
	Tools         []wireToolStub  `json:"tools"`
	ToolChoice    string          `json:"tool_choice"`
	Stream        bool            `json:"stream"`
	StreamOptions map[string]bool `json:"stream_options"`
}

type sseChunk struct {
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

func detectPromptLayerZone(promptKey string, maxTokens int) string {
	switch {
	case strings.Contains(promptKey, "Layer 1 (") || strings.Contains(promptKey, "ROLE — L1"):
		return "L1"
	case strings.Contains(promptKey, "Layer 2 (") || strings.Contains(promptKey, "ROLE — L2"):
		return "L2"
	case strings.Contains(promptKey, "Layer 3 (") || strings.Contains(promptKey, "ROLE — L3"):
		return "L3"
	case strings.Contains(promptKey, "Layer 4 (") || strings.Contains(promptKey, "ROLE — L4"):
		return "L4"
	case strings.Contains(promptKey, "Layer 6 (") || strings.Contains(promptKey, "ROLE — L6") || maxTokens > 8192:
		return "L6"
	default:
		return "TURN05"
	}
}

func commonPrefixLen(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func (p *OpenCodeFreeProvider) callSingleModel(
	ctx context.Context,
	modelID string,
	req core.CompletionRequest,
	onDelta func(core.StreamDelta),
) (core.CompletionResponse, error) {
	start := time.Now()
	maxTokens := req.MaxTokens
	if maxTokens < 512 {
		maxTokens = 4096
	}

	msgs := make([]core.ChatTurn, 0, len(req.Messages)+1)
	mergedSys := openCodeGatePreamble
	if strings.TrimSpace(req.SystemPrompt) != "" {
		mergedSys = openCodeGatePreamble + "\n\n" + req.SystemPrompt
	}
	msgs = append(msgs, core.ChatTurn{
		Role:    "system",
		Content: mergedSys,
	})
	msgs = append(msgs, req.Messages...)

	bodyStruct := wirePayload{
		Model:         modelID,
		MaxTokens:     maxTokens,
		Messages:      msgs,
		Tools:         buildStubTools(),
		ToolChoice:    "none",
		Stream:        true,
		StreamOptions: map[string]bool{"include_usage": true},
	}

	rawBody, err := json.Marshal(bodyStruct)
	if err != nil {
		return core.CompletionResponse{}, err
	}

	// Build serialized prefix string for KV prefix cache tracking & exact deduplication
	var prefixBuilder strings.Builder
	for _, m := range msgs {
		prefixBuilder.WriteString(m.Role)
		prefixBuilder.WriteString(":")
		prefixBuilder.WriteString(m.Content)
		prefixBuilder.WriteString("\n")
	}
	fullPromptKey := prefixBuilder.String()
	cacheKey := modelID + "::" + fullPromptKey

	p.mu.RLock()
	if hit, ok := p.exactCache[cacheKey]; ok && time.Now().Before(hit.expiresAt) {
		p.mu.RUnlock()
		cachedResp := hit.resp
		cachedResp.CachedTokens = cachedResp.PromptTokens
		cachedResp.LatencyMs = 1
		return cachedResp, nil
	}
	sessID := p.sessionID
	layerZone := detectPromptLayerZone(fullPromptKey, maxTokens)
	radixBucketKey := modelID + ":" + layerZone
	if ls, ok := p.layerSessionIDs[layerZone]; ok && ls != "" {
		sessID = ls
	} else if maxTokens <= 8192 && p.compassSessionID != "" {
		sessID = p.compassSessionID
	}
	p.mu.RUnlock()

	callTimeout := 75 * time.Second
	maxReasoningChars := 18000
	if maxTokens > 8192 {
		if strings.HasPrefix(modelID, "deepseek-v4.1-flash") {
			callTimeout = 150 * time.Second
			maxReasoningChars = 42000
		} else {
			callTimeout = 60 * time.Second
			maxReasoningChars = 18000
		}
	}
	callCtx, cancelCall := context.WithTimeout(ctx, callTimeout)
	defer cancelCall()
	idleTimer := time.AfterFunc(65*time.Second, func() {
		cancelCall()
	})
	defer idleTimer.Stop()

	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, p.endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return core.CompletionResponse{}, err
	}

	apiKey, _ := resolveOpenCodeCredential()
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", openCodeUserAgent)
	httpReq.Header.Set("x-opencode-client", "cli")
	httpReq.Header.Set("x-opencode-project", "global")
	httpReq.Header.Set("x-opencode-session", sessID)
	httpReq.Header.Set("x-opencode-request", p.generateOpenCodeID("msg", false))

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return core.CompletionResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		ra := strings.TrimSpace(resp.Header.Get("Retry-After"))
		if ra != "" {
			return core.CompletionResponse{}, fmt.Errorf("model %s HTTP %d (Retry-After=%s): %s", modelID, resp.StatusCode, ra, strings.TrimSpace(string(errBody)))
		}
		return core.CompletionResponse{}, fmt.Errorf("model %s HTTP %d: %s", modelID, resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var contentBuf, reasoningBuf strings.Builder
	var promptToks, compToks, cachedToks int

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	for scanner.Scan() {
		idleTimer.Reset(35 * time.Second)
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		if line == "data: [DONE]" {
			break
		}
		var chunk sseChunk
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
				// Reasoning Runaway Guard: allow up to 48,000 chars on L6 code-writing turns (20,000 chars on L1-L5) before cutting runaway loops
				if contentBuf.Len() == 0 && reasoningBuf.Len() > maxReasoningChars {
					cancelCall()
					break
				}
			}
			if onDelta != nil && (cDelta != "" || rDelta != "") {
				onDelta(core.StreamDelta{
					ContentDelta:   cDelta,
					ReasoningDelta: rDelta,
				})
			}
		}
	}
	if scanErr := scanner.Err(); scanErr != nil && strings.TrimSpace(contentBuf.String()) == "" {
		return core.CompletionResponse{}, fmt.Errorf("model %s stream aborted: %w", modelID, scanErr)
	}

	// Track Radix-style prefix cache overlap across the last 16 calls on this model in this session zone
	p.mu.Lock()
	recentList := p.recentPrefixesByModel[radixBucketKey]
	bestSharedBytes := 0
	for _, prev := range recentList {
		if sb := commonPrefixLen(prev, fullPromptKey); sb > bestSharedBytes {
			bestSharedBytes = sb
		}
	}
	// Append current prompt (plus assistant response so next turn in multi-turn loop matches full prefix!)
	withAssistantKey := fullPromptKey + "assistant:" + strings.TrimSpace(contentBuf.String()) + "\n"
	recentList = append(recentList, fullPromptKey, withAssistantKey)
	if len(recentList) > 24 {
		recentList = recentList[len(recentList)-24:]
	}
	p.recentPrefixesByModel[radixBucketKey] = recentList
	p.UnlockOrCache(cacheKey, core.CompletionResponse{
		Model:            modelID,
		Content:          strings.TrimSpace(contentBuf.String()),
		Reasoning:        strings.TrimSpace(reasoningBuf.String()),
		PromptTokens:     promptToks,
		CachedTokens:     cachedToks,
		CompletionTokens: compToks,
		LatencyMs:        time.Since(start).Milliseconds(),
	}, bestSharedBytes, &cachedToks, promptToks, len(fullPromptKey))

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

func (p *OpenCodeFreeProvider) UnlockOrCache(
	cacheKey string,
	resp core.CompletionResponse,
	bestSharedBytes int,
	cachedToks *int,
	promptToks int,
	totalBytes int,
) {
	defer p.mu.Unlock()
	if bestSharedBytes >= 256 && totalBytes > 0 {
		var radixCached int
		if promptToks > 0 {
			// Scale cached token count proportionally by shared byte ratio of the serialized prompt
			ratio := float64(bestSharedBytes) / float64(totalBytes)
			if ratio > 0.96 {
				ratio = 0.96
			}
			radixCached = int(math.Round(float64(promptToks) * ratio))
		} else {
			radixCached = bestSharedBytes / 4
		}
		if radixCached > *cachedToks {
			*cachedToks = radixCached
		}
	}
	resp.CachedTokens = *cachedToks
	if resp.Content != "" {
		p.exactCache[cacheKey] = cachedCompletion{
			resp:      resp,
			expiresAt: time.Now().Add(45 * time.Second),
		}
	}
}

// generateOpenCodeID implements OpenCode's exact 26-char ID encoding (prefix + "_" + 12 hex + 14 base62).
func (p *OpenCodeFreeProvider) generateOpenCodeID(prefix string, descending bool) string {
	ms := uint64(time.Now().UnixMilli())
	cnt := (p.idCounter.Add(1) % 0x0fff) + 1
	val := (ms * 0x1000) + cnt
	if descending {
		val = ^val
	}

	var buf8 [8]byte
	binary.BigEndian.PutUint64(buf8[:], val)
	hexPart := hex.EncodeToString(buf8[2:8]) // 6 bytes = 12 hex chars

	var randBytes [14]byte
	_, _ = rand.Read(randBytes[:])
	var b62 [14]byte
	for i := 0; i < 14; i++ {
		b62[i] = base62Chars[int(randBytes[i])%len(base62Chars)]
	}

	return prefix + "_" + hexPart + string(b62[:])
}
