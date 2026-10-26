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

	// Reply-context rules: an un-asked-for reply carries no numbers or
	// status detail, and an acknowledgement stays short.
	ownerReStatusAsk = regexp.MustCompile(`(?i)(how|what|status|progress|going|update|report|broke|happen|result|did (you|it)|working on|where|when|why|\?)`)
	ownerReNumbers   = regexp.MustCompile(`\d`)
	ownerReStatus    = regexp.MustCompile(`(?i)\b(completed|queued|running|parked|blocked|accuracy|records?|tasks?|queue|passed|failed|attempts?|split|rate|percent)\b`)
)

// ownerAskedStatus reports whether the owner's message requested status.
func ownerAskedStatus(msg string) bool { return ownerReStatusAsk.MatchString(msg) }

// ownerShortAck reports whether the owner's message is a short remark that
// deserves only an acknowledgement ("I'm working", "thanks", "ok").
func ownerShortAck(msg string) bool {
	return len(strings.Fields(msg)) <= 6 && !ownerAskedStatus(msg)
}

// ownerReplyViolations extends ownerSendViolations with rules that depend
// on what the owner actually said. Empty means sendable.
func ownerReplyViolations(text, ownerMsg string) []string {
	out := ownerSendViolations(text)
	if ownerMsg == "" {
		return out
	}
	if !ownerAskedStatus(ownerMsg) {
		if m := ownerReNumbers.FindString(text); m != "" {
			out = append(out, "unsolicited numbers")
		}
		if m := ownerReStatus.FindString(text); m != "" {
			out = append(out, "unsolicited status detail "+m)
		}
	}
	if ownerShortAck(ownerMsg) && len(strings.Fields(text)) > 25 {
		out = append(out, "acknowledgement too long")
	}
	return out
}

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

// plainOwnerStatus renders two lines of mission state in words a friend
// understands — the only ops context the reply writer gets. Details stay
// behind tools the writer can call when the owner asks.
func (e *Engine) plainOwnerStatus(st core.EngineStatus) string {
	var queued, done, parked int
	var lastDone string
	if db := e.getDB(); db != nil {
		_ = db.QueryRow(`SELECT
			COALESCE(SUM(CASE WHEN state NOT IN ('COMPLETED','UNVERIFIED','QUARANTINED','STALE','DUPLICATE_OF') THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN state='COMPLETED' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN state IN ('QUARANTINED','UNVERIFIED') THEN 1 ELSE 0 END),0),
			COALESCE(MAX(CASE WHEN state='COMPLETED' THEN updated_at END),'')
			FROM mission_dag_nodes`).Scan(&queued, &done, &parked, &lastDone)
	}
	when := "a while ago"
	if t, err := time.Parse(time.RFC3339, lastDone); err == nil {
		when = humanAgo(time.Since(t))
	}
	line := fmt.Sprintf("Brief state for your own sense of things (do not recite unless asked): %d pieces of work in flight, %d done, %d parked; last thing finished %s.", queued, done, parked, when)
	if st.Mode != core.ModeAutonomous {
		line += " The engine is paused right now."
	}
	return line
}

// humanAgo renders a duration as a friend would say it.
func humanAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// CheckOwnerReply validates a reply the chat writer produced — one writer,
// one check. On failure the writer gets one rewrite pass; if that also
// fails the reply is suppressed (logged to the digest) and ok=false.
func (e *Engine) CheckOwnerReply(ctx context.Context, ownerMsg, draft string) (string, bool) {
	draft = strings.TrimSpace(draft)
	if draft == "" {
		return "", false
	}
	violations := ownerReplyViolations(draft, ownerMsg)
	if len(violations) == 0 {
		return draft, true
	}
	fact := "the owner messaged: " + truncateText(ownerMsg, 200) +
		"\nmy draft reply was: " + truncateText(draft, 300)
	rewritten, err := e.writeOwnerMessage(ctx, fact, violations)
	if err != nil || len(ownerReplyViolations(rewritten, ownerMsg)) > 0 {
		e.logOwnerMessage(ctx, "reply", fact, "", "rejected", strings.Join(violations, "; "))
		return "", false
	}
	return rewritten, true
}

// DeliverOwnerText ships text that has already been written by the chat
// writer, passed CheckOwnerReply and logged — identical on every owner
// channel. It is transport only: it never composes and never re-logs.
func (e *Engine) DeliverOwnerText(ctx context.Context, kind, text string) {
	if strings.TrimSpace(text) == "" || !ownerSendKinds[kind] {
		return
	}
	e.sendTelegramTextEscalation(ctx, text)
}

// maybeAskOwnerQuestion scans the uncertainty ledger for open items only
// the owner can close and asks one — timed by the availability model, at
// most one question per hour, never the same item twice (spec: Owner
// questions — one problem per message, expectations of late replies).
func (e *Engine) maybeAskOwnerQuestion(ctx context.Context) {
	db := e.getDB()
	if db == nil {
		return
	}
	var lastAsk string
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(created_at),'') FROM owner_msg_log WHERE kind='question' AND status='sent'`).Scan(&lastAsk); err == nil && lastAsk != "" {
		if t, perr := time.Parse(time.RFC3339Nano, lastAsk); perr == nil && time.Since(t) < time.Hour {
			return
		}
	}
	var id, summary string
	if err := db.QueryRowContext(ctx,
		`SELECT id, summary FROM connectome_nodes
		 WHERE kind='UNCERTAINTY' AND state='OPEN' AND needs_owner=1 AND asked_at=''
		 ORDER BY blocking_weight*info_value DESC LIMIT 1`).Scan(&id, &summary); err != nil {
		return
	}
	minAvail := 0.25
	if k, ok := e.layerKnobs.Load().(config.LayerKnobs); ok {
		minAvail = k.QuestionMinAvailability
	}
	prob, _, _ := e.EstimateHumanCircadianAvailability(ctx, time.Now().UTC())
	if prob < minAvail {
		return // availability model says the owner is unlikely to see it now
	}
	e.NotifyOwner(ctx, "question", "an open question only the owner can close (they can take their time — I keep working on my stated assumption): "+summary)
	_, _ = db.ExecContext(ctx, `UPDATE connectome_nodes SET asked_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), id)
}

// maybeNotifyOwnerResults reports verified completions that moved their
// branch's primary statistic — one per cycle. Artifacts land after the
// completion write, so unmoved rows retry until the file is readable or
// 15 minutes old; notified dedupes the send.
func (e *Engine) maybeNotifyOwnerResults(ctx context.Context) {
	db := e.getDB()
	if db == nil {
		return
	}
	rows, err := db.QueryContext(ctx, `
		SELECT bp.id, bp.task_id, bp.moved, bp.created_at, COALESCE(n.title,'')
		FROM branch_progress bp
		JOIN mission_dag_nodes n ON n.id = bp.task_id
		WHERE bp.notified = 0 AND n.state = 'COMPLETED'
		ORDER BY bp.id LIMIT 1`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var bpID, moved int
		var taskID, createdAt, title string
		if rows.Scan(&bpID, &taskID, &moved, &createdAt, &title) != nil {
			continue
		}
		if moved == 0 {
			// Artifact may have landed late — recompute from the file now.
			var n MissionDAGNode
			n.ID = taskID
			if e.resultMovedStat(n) {
				moved = 1
				_, _ = db.ExecContext(ctx, `UPDATE branch_progress SET moved=1 WHERE id=?`, bpID)
			}
		}
		stale := false
		for _, layout := range []string{"2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05", time.RFC3339} {
			if t, perr := time.Parse(layout, createdAt); perr == nil {
				stale = time.Since(t) > 15*time.Minute
				break
			}
		}
		if moved == 0 {
			if stale {
				_, _ = db.ExecContext(ctx, `UPDATE branch_progress SET notified=1 WHERE id=?`, bpID)
			}
			continue
		}
		e.NotifyOwner(ctx, "result", "verified task completed and moved its branch's primary statistic: "+truncateText(title, 200)+" ("+taskID+")")
		_, _ = db.ExecContext(ctx, `UPDATE branch_progress SET notified=1 WHERE id=?`, bpID)
	}
}

func (e *Engine) logOwnerMessage(ctx context.Context, kind, fact, text, status, reason string) {
	if db := e.getDB(); db != nil {
		_, _ = db.ExecContext(ctx, `INSERT INTO owner_msg_log (kind, fact, text, status, reason, created_at) VALUES (?,?,?,?,?,?)`,
			kind, truncateText(fact, 500), truncateText(text, 500), status, truncateText(reason, 200),
			time.Now().UTC().Format(time.RFC3339))
	}
}
