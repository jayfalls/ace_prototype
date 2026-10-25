package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ace/internal/core"
)

var binaryGateClient = &http.Client{Timeout: 30 * time.Second}

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

// EvaluateBinaryGate runs a fast binary decision gate: the LiquidAI/d1-3B
// logprob readout when DECISION_MODEL_URL is set, else one "YES <prob> /
// NO <prob>" call through prov (sharedContext first for prefix caching).
func EvaluateBinaryGate(ctx context.Context, prov core.LLMProvider, sharedContext string, question string) (float64, bool, error) {
	if d1URL := strings.TrimSpace(os.Getenv("DECISION_MODEL_URL")); d1URL != "" {
		if pYes, ok := d1LogprobReadout(ctx, d1URL, sharedContext, question); ok {
			return pYes, pYes >= 0.5, nil
		}
	}
	resp, err := prov.StreamComplete(ctx, core.CompletionRequest{
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

func d1LogprobReadout(ctx context.Context, d1URL, sharedContext, question string) (float64, bool) {
	reqBody := map[string]interface{}{
		"model": "LiquidAI/d1-3B",
		"messages": []map[string]string{
			{"role": "user", "content": fmt.Sprintf("%s\n\n%s\n\nReply with yes or no only.", sharedContext, question)},
		},
		"max_tokens":   1,
		"temperature":  0.0,
		"logprobs":     true,
		"top_logprobs": 20,
	}
	raw, _ := json.Marshal(reqBody)
	hReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(d1URL, "/")+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return 0, false
	}
	hReq.Header.Set("Content-Type", "application/json")
	resp, err := binaryGateClient.Do(hReq)
	if err != nil {
		return 0, false
	}
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
	if json.NewDecoder(resp.Body).Decode(&payload) != nil || len(payload.Choices) == 0 || len(payload.Choices[0].Logprobs.Content) == 0 {
		return 0, false
	}
	lpMap := make(map[string]float64)
	for _, entry := range payload.Choices[0].Logprobs.Content[0].TopLogprobs {
		lpMap[entry.Token] = entry.Logprob
	}
	return ReadoutD1Logprobs(lpMap)
}
