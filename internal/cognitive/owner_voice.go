package cognitive

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ace/internal/config"
	"ace/internal/core"
)

// owner_voice.go — the single owner-send path (plan "Human voice", b59dabaf).
//
// Every owner-bound message goes through NotifyOwner: the chat model writes
// the actual text from a short fact summary, in the voice from
// ~/.config/ace/personality.txt, then a deterministic friend-check runs
// before anything is sent. Internal churn (parks, vetoes, restarts, the
// runaway pause) never reaches Telegram — it lands in the digest and ACE
// talks about it when asked.
//
// Kinds worth sending: question, access, pr_review, result, reply.
// Any other kind is digest-only by construction.

var ownerSendKinds = map[string]bool{
	"question":  true,
	"access":    true,
	"pr_review": true,
	"result":    true,
	"reply":     true,
}

// Internal identifiers and jargon that must never reach the owner.
var (
	ownerReTaskID   = regexp.MustCompile(`\bT_[A-Z0-9_]+|\bS\d+\b|cyc_[A-Za-z0-9]+|contradiction:|\b[0-9a-f]{12,}\b`)
	ownerRePath     = regexp.MustCompile(`/workspace/|/wiki/|\.ace-sandbox`)
	ownerReJargon   = regexp.MustCompile(`(?i)\bL[1-6]\b|Mode-?3\b|Tier\b|quarantine[ds]?\b|veto(es)?\b|mission_dag|veto_ledger|selftest`)
	ownerReTable    = regexp.MustCompile(`\n\s*\|.*\|.*\||^\s*\|.*\|.*\|`)
	ownerReCodeSpan = regexp.MustCompile("`{1,3}")
)

// ownerSendViolations returns the deterministic-check failures for a
// candidate message. Empty means sendable.
func ownerSendViolations(text string) []string {
	var out []string
	if m := ownerReTaskID.FindString(text); m != "" {
		out = append(out, "internal id "+m)
	}
	if m := ownerRePath.FindString(text); m != "" {
		out = append(out, "internal path "+m)
	}
	if m := ownerReJargon.FindString(text); m != "" {
		out = append(out, "jargon "+m)
	}
	if ownerReTable.MatchString(text) {
		out = append(out, "table")
	}
	if ownerReCodeSpan.MatchString(text) {
		out = append(out, "code formatting")
	}
	return out
}

const ownerVoiceSystem = `You are ACE writing a short message to your owner — a friend and collaborator, not a user of a dashboard.

Your voice: %s

You get a short fact summary and write ONE short message (under 400 characters) about it. Rules:
- Sound like a human texting a friend. First person. Light, direct, honest.
- NEVER include internal identifiers, task or branch codes, file paths, hashes, layer numbers, gate names, or any jargon. No tables, no code formatting, no bullet-point dumps.
- If the facts are bad news, say so plainly and say what happens next.
- Output ONLY the message text.`

// NotifyOwner is the only way anything reaches the owner. kind decides
// sendability; fact is a short internal summary (IDs and paths are fine
// here — the model translates, the check verifies).
func (e *Engine) NotifyOwner(ctx context.Context, kind, fact string) {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return
	}
	if !ownerSendKinds[kind] {
		e.appendOwnerDigest(ctx, kind, fact)
		return
	}

	text, err := e.writeOwnerMessage(ctx, fact, nil)
	status, reason := "sent", ""
	violations := ownerSendViolations(text)
	if err != nil || len(violations) > 0 {
		// One rewrite with the violations spelled out.
		text, err = e.writeOwnerMessage(ctx, fact, violations)
		violations = ownerSendViolations(text)
	}
	if err != nil || len(violations) > 0 {
		status, reason = "rejected", "compose failed"
		if len(violations) > 0 {
			reason = strings.Join(violations, "; ")
		}
		text = ""
		e.appendOwnerDigest(ctx, kind, fact)
	} else {
		e.sendTelegramTextEscalation(ctx, text)
	}
	e.logOwnerMessage(ctx, kind, fact, text, status, reason)
}

// writeOwnerMessage asks the chat model to render the fact in the owner
// voice. violations non-nil requests a jargon-free rewrite.
func (e *Engine) writeOwnerMessage(ctx context.Context, fact string, violations []string) (string, error) {
	personality := config.LoadPersonality()
	if personality == "" {
		personality = "warm, direct, plain-spoken"
	}
	user := "Facts to share (internal — translate for a friend, strip every identifier):\n" + fact
	if len(violations) > 0 {
		user += fmt.Sprintf("\n\nYour previous attempt failed the friend-check on: %s. Rewrite without any of that — plainer, shorter.", strings.Join(violations, "; "))
	}
	resp, err := e.llm.StreamComplete(ctx, core.CompletionRequest{
		SystemPrompt: fmt.Sprintf(ownerVoiceSystem, personality),
		Messages:     []core.ChatTurn{{Role: "user", Content: user}},
		MaxTokens:    512,
		Temperature:  0.6,
	}, nil)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(resp.Content)
	if len(text) > 400 {
		text = strings.TrimSpace(text[:400])
	}
	return text, nil
}

// appendOwnerDigest files non-sendable news where the owner will see it on
// request: the owner_msg_log table with status='digest' (surfaced by the
// overnight digest and readable on demand).
func (e *Engine) appendOwnerDigest(ctx context.Context, kind, fact string) {
	e.logOwnerMessage(ctx, kind, fact, "", "digest", "")
}

func (e *Engine) logOwnerMessage(ctx context.Context, kind, fact, text, status, reason string) {
	if db := e.getDB(); db != nil {
		_, _ = db.ExecContext(ctx, `INSERT INTO owner_msg_log (kind, fact, text, status, reason, created_at) VALUES (?,?,?,?,?,?)`,
			kind, truncateText(fact, 500), truncateText(text, 500), status, truncateText(reason, 200),
			time.Now().UTC().Format(time.RFC3339))
	}
}
