package gate

import (
	"context"
	"strings"
	"time"

	"ace/internal/constitution"
	"ace/internal/core"
)

// tier1 is the fast-model gate (models.fast): it may only allow or escalate.
// A Tier-1 error escalates — it never silently passes (plan 8.2).
func (g *Gate) tier1(ctx context.Context, a Action) Decision {
	model := g.roles().Fast
	if model == "" {
		model = g.llm.ActiveModel()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := g.llm.StreamComplete(ctx, core.CompletionRequest{
		Model: model,
		SystemPrompt: "You are ACE's Tier-1 constitutional fast gate. The pinned constitution (Heuristic Imperatives, UDHR, autonomy triad, epistemic law) governs. " +
			"Decide whether the proposed action is routine and safe to ALLOW, or must ESCALATE to full L1 review. Reply with exactly one token: ALLOW or ESCALATE.\n\n" + constitution.Text(),
		Messages:    []core.ChatTurn{{Role: "user", Content: "Action kind: " + a.Kind + "\nTarget: " + a.Target + "\nCommand: " + truncate(a.Command, 400) + "\nLayer: L" + itoa(a.Layer)}},
		MaxTokens:   8,
		Temperature: 0,
	}, nil)
	if err != nil {
		return Decision{Verdict: "escalate", Tier: 1, RuleID: "t1.error-escalate", Reason: err.Error()}
	}
	verdict := strings.ToUpper(strings.TrimSpace(resp.Content))
	if strings.HasPrefix(verdict, "ALLOW") {
		return Decision{Verdict: VerdictAllow, Tier: 1, RuleID: "t1.allow", Reason: model}
	}
	return Decision{Verdict: "escalate", Tier: 1, RuleID: "t1.escalate", Reason: truncate(verdict, 60)}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}
