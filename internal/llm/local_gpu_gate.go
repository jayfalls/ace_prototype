package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// LocalGPUGateResult represents a 1-token P(yes) logprob + Laya ensemble decision from the local RTX 3050 4-bit GPU gate.
type LocalGPUGateResult struct {
	PYes          float64  `json:"p_yes"`
	PYesD14Bit    float64  `json:"p_yes_d1_4bit"`
	PYesLaya      *float64 `json:"p_yes_laya"`
	Decision      bool     `json:"decision"`
	Threshold     float64  `json:"threshold"`
	LogprobYes    float64  `json:"yes_logprob"`
	LogprobNo     float64  `json:"no_logprob"`
	TrunkCacheHit bool     `json:"trunk_cache_hit"`
	LatencyMs     float64  `json:"latency_ms"`
	InputTokens   int      `json:"input_tokens"`
	VRAMMB        float64  `json:"vram_mb"`
	Model         string   `json:"model"`
}

// LocalGPUChoiceResult represents a zero-output-token multi-option choice decision from `/system_one`.
type LocalGPUChoiceResult struct {
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	TrunkCacheHit bool               `json:"trunk_cache_hit"`
	LatencyMs     float64            `json:"latency_ms"`
	Model         string             `json:"model"`
}

var gpuGateClient = &http.Client{Timeout: 1800 * time.Millisecond}

func gpuGateURL() string {
	if u := os.Getenv("ACE_GPU_GATE_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:8091"
}

// EvaluateLocalGPUGate calls the local 4-bit RTX 3050 System-1 gate (0 remote API tokens).
func EvaluateLocalGPUGate(ctx context.Context, question string, threshold float64) (LocalGPUGateResult, error) {
	return EvaluateLocalGPUGateWithState(ctx, "", question, threshold)
}

// EvaluateLocalGPUGateWithState passes a shared `state` prefix so the RTX 3050 caches its KV trunk across calls.
func EvaluateLocalGPUGateWithState(ctx context.Context, state string, question string, threshold float64) (LocalGPUGateResult, error) {
	if threshold <= 0 || threshold >= 1 {
		threshold = 0.5
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"state":     state,
		"question":  question,
		"threshold": threshold,
		"engine":    "d1_4bit",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/gate", gpuGateURL()), bytes.NewReader(payload))
	if err != nil {
		return LocalGPUGateResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := gpuGateClient.Do(req)
	if err != nil {
		return LocalGPUGateResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LocalGPUGateResult{}, fmt.Errorf("local gpu gate status %d", resp.StatusCode)
	}
	var out LocalGPUGateResult
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16*1024)).Decode(&out); err != nil {
		return LocalGPUGateResult{}, err
	}
	out.Threshold = threshold
	return out, nil
}

// EvaluateLocalGPUChoice runs a LiquidAI/d1 `system_one` multi-option choice query on the local 4-bit GPU server.
func EvaluateLocalGPUChoice(ctx context.Context, state string, instructions string, criteria map[string]string) (LocalGPUChoiceResult, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"state":  state,
		"engine": "d1_4bit",
		"questions": map[string]interface{}{
			"route": map[string]interface{}{
				"type":         "choice",
				"instructions": instructions,
				"criteria":     criteria,
			},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/system_one", gpuGateURL()), bytes.NewReader(payload))
	if err != nil {
		return LocalGPUChoiceResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := gpuGateClient.Do(req)
	if err != nil {
		return LocalGPUChoiceResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LocalGPUChoiceResult{}, fmt.Errorf("local gpu system_one status %d", resp.StatusCode)
	}
	var raw struct {
		Model         string  `json:"model"`
		TrunkCacheHit bool    `json:"trunk_cache_hit"`
		LatencyMs     float64 `json:"latency_ms"`
		Answers       map[string]struct {
			Choice        string             `json:"choice"`
			Confidence    float64            `json:"confidence"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16*1024)).Decode(&raw); err != nil {
		return LocalGPUChoiceResult{}, err
	}
	ans, ok := raw.Answers["route"]
	if !ok {
		return LocalGPUChoiceResult{}, fmt.Errorf("missing route answer in system_one response")
	}
	return LocalGPUChoiceResult{
		Choice:        ans.Choice,
		Confidence:    ans.Confidence,
		Probabilities: ans.Probabilities,
		TrunkCacheHit: raw.TrunkCacheHit,
		LatencyMs:     raw.LatencyMs,
		Model:         raw.Model,
	}, nil
}
