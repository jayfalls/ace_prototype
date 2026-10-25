package gate

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ace/internal/constitution"
	"ace/internal/core"
	"ace/internal/provider"
)

// tier2 is full L1 (models.main): allow / revise / reject / redirect.
// Rejections and self-changes require the ensemble vote (models.ensemble);
// fewer than three responders means the rejection stands and any self-change
// is deferred (plan 8.2). A Tier-2 error rejects — fail closed.
func (g *Gate) tier2(ctx context.Context, a Action) Decision {
	model := g.roles().Main
	if model == "" {
		model = g.llm.ActiveModel()
	}
	verdict, reason, revised := g.askTier2(ctx, model, a)
	if verdict == "" {
		return Decision{Verdict: VerdictReject, Tier: 2, RuleID: "t2.error-reject", Reason: "tier-2 error: " + reason}
	}
	if verdict == VerdictReject || verdict == VerdictRevise {
		// Ensemble check: the rejection/revision only overturns when >=3
		// ensemble models respond and a majority votes allow.
		votes := g.ensembleVote(ctx, a)
		if votes.responded >= 3 && votes.allowVotes*2 > votes.responded {
			return Decision{Verdict: VerdictAllow, Tier: 2, RuleID: "t2.ensemble-override", Reason: "ensemble overturned rejection"}
		}
		return Decision{Verdict: VerdictReject, Tier: 2, RuleID: "t2.reject", Reason: reason}
	}
	if verdict == VerdictRedirect {
		g.queueRedirect(Redirect{TaskID: a.TaskID, Message: revised})
	}
	return Decision{Verdict: verdict, Tier: 2, RuleID: "t2." + verdict, Reason: reason, Revised: revised}
}

func (g *Gate) askTier2(ctx context.Context, model string, a Action) (verdict, reason, revised string) {
	ctx, cancel := context.WithTimeout(provider.WithLayer(ctx, "L1"), 90*time.Second)
	defer cancel()
	resp, err := g.llm.StreamComplete(ctx, core.CompletionRequest{
		Model: model,
		SystemPrompt: "You are ACE's Tier-2 L1 Aspirational gate. Judge the proposed action against the pinned constitution. " +
			"Reply with JSON only: {\"verdict\":\"allow|revise|reject|redirect\",\"reason\":\"...\",\"revised\":\"replacement directive for revise/redirect\"}.\n\n" + constitution.Text(),
		Messages:    []core.ChatTurn{{Role: "user", Content: "Action kind: " + a.Kind + "\nTarget: " + a.Target + "\nCommand: " + truncate(a.Command, 600) + "\nLayer: L" + itoa(a.Layer)}},
		MaxTokens:   256,
		Temperature: 0,
	}, nil)
	if err != nil {
		g.recordTrace(a, model, "error", resp, err)
		return "", err.Error(), ""
	}
	var parsed struct {
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
		Revised string `json:"revised"`
	}
	body := resp.Content
	if i := strings.Index(body, "{"); i >= 0 {
		if j := strings.LastIndex(body, "}"); j > i {
			body = body[i : j+1]
		}
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		g.recordTrace(a, model, "unparseable", resp, err)
		return "", "unparseable tier-2 verdict: " + truncate(resp.Content, 80), ""
	}
	g.recordTrace(a, model, parsed.Verdict, resp, nil)
	switch parsed.Verdict {
	case VerdictAllow, VerdictRevise, VerdictReject, VerdictRedirect:
		return parsed.Verdict, parsed.Reason, parsed.Revised
	}
	return "", "invalid tier-2 verdict: " + truncate(parsed.Verdict, 40), ""
}

type ensembleResult struct {
	responded  int
	allowVotes int
}

// ensembleVote asks each configured ensemble model whether the action should be
// allowed. Fewer than three responders means no quorum — caller fails closed.
func (g *Gate) ensembleVote(ctx context.Context, a Action) ensembleResult {
	var res ensembleResult
	for _, m := range g.roles().Ensemble {
		ctx2, cancel := context.WithTimeout(provider.WithLayer(ctx, "L1"), 45*time.Second)
		resp, err := g.llm.StreamComplete(ctx2, core.CompletionRequest{
			Model:        m,
			SystemPrompt: "Vote on whether this action should be allowed under the ACE constitution. Reply with exactly ALLOW or REJECT.\n\n" + constitution.Text(),
			Messages:     []core.ChatTurn{{Role: "user", Content: "Action: " + a.Kind + " " + a.Target + " " + truncate(a.Command, 300)}},
			MaxTokens:    8,
			Temperature:  0,
		}, nil)
		cancel()
		g.recordTrace(a, m, "ensemble-vote", resp, err)
		if err != nil {
			continue
		}
		res.responded++
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(resp.Content)), "ALLOW") {
			res.allowVotes++
		}
	}
	return res
}
