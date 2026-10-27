package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ace/internal/config"
	"ace/internal/constitution"
	"ace/internal/core"

	"github.com/google/uuid"
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
	ownerReTaskID   = regexp.MustCompile(`\bT_[A-Z0-9_]+|\bS\d+\b|\bc_s\d+\b|cyc_[A-Za-z0-9]+|contradiction:|\b[0-9a-f]{12,}\b`)
	ownerRePath     = regexp.MustCompile(`/workspace/|/wiki/|\.ace-sandbox`)
	ownerReJargon   = regexp.MustCompile(`(?i)\bL[1-6]\b|Mode-?3\b|Tier\b|quarantine[ds]?\b|veto(es)?\b|mission_dag|veto_ledger|selftest`)
	ownerReTable    = regexp.MustCompile(`\n\s*\|.*\|.*\||^\s*\|.*\|.*\|`)
	ownerReCodeSpan = regexp.MustCompile("`{1,3}")

	// The check keeps ONLY mechanical rules — ID, path, jargon, table,
	// code formatting. Whether a reply should carry numbers or status is
	// the chat agent's judgment, not a regex's.
)

// ownerReplyViolations: replies run the same mechanical check as any
// owner-bound text; the ownerMsg param is kept for call-site stability.
func ownerReplyViolations(text, ownerMsg string) []string {
	return ownerSendViolations(text)
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
- Only reference earlier conversation when the subject actually matches — same result file, same statistic, same thing the owner named. NEVER claim the owner asked about a result unless the earlier message names that same result or its file.
- Output ONLY the message text.`

// NotifyOwner hands an event to the chat agent as an offer (plan "Human
// voice"): no composing, no sending — drainOwnerOffers is the only
// proactive writer. kind decides sendability; fact carries the numbers.
func (e *Engine) NotifyOwner(ctx context.Context, kind, fact string) {
	e.NotifyOwnerSourced(ctx, kind, fact, "")
}

// NotifyOwnerSourced is NotifyOwner with an explicit provenance string:
// the owner message or ACE record the fact rests on.
func (e *Engine) NotifyOwnerSourced(ctx context.Context, kind, fact, source string) {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return
	}
	if !ownerSendKinds[kind] {
		e.appendOwnerDigest(ctx, kind, fact)
		return
	}
	if db := e.getDB(); db != nil {
		_, _ = db.ExecContext(ctx,
			`INSERT INTO owner_offers (kind, fact, source, status, created_at) VALUES (?,?,?,'pending',?)`,
			kind, truncateText(fact, 800), truncateText(source, 300),
			time.Now().UTC().Format(time.RFC3339))
	}
}

// drainOwnerOffers is the proactive half of the one chat agent: pending
// offers go into the same conversation history the owner sees, the model
// decides HOLD or ONE folded message, and the result lands in
// chat_messages, owner_msg_log and the channel — identical everywhere.
func (e *Engine) drainOwnerOffers(ctx context.Context) {
	db := e.getDB()
	if db == nil {
		return
	}
	// Offers the agent never picked become digest rows — a stale offer
	// is history, not news.
	_, _ = db.ExecContext(ctx, `UPDATE owner_offers SET status='digested', handled_at=?
		WHERE status='pending' AND replace(substr(created_at,1,19),'T',' ') < datetime('now','-2 hours')`,
		time.Now().UTC().Format(time.RFC3339))

	rows, err := db.QueryContext(ctx,
		`SELECT id, kind, fact, source FROM owner_offers WHERE status='pending' ORDER BY id LIMIT 10`)
	if err != nil {
		return
	}
	type offer struct {
		id                 int
		kind, fact, source string
	}
	var pending []offer
	for rows.Next() {
		var o offer
		if rows.Scan(&o.id, &o.kind, &o.fact, &o.source) == nil {
			pending = append(pending, o)
		}
	}
	rows.Close()
	if len(pending) == 0 {
		return
	}

	// Keeps quiet when it has just spoken: a proactive send must be at
	// least 10 minutes old unless a question or access request is waiting.
	urgent := false
	for _, o := range pending {
		if o.kind == "question" || o.kind == "access" {
			urgent = true
		}
	}
	if !urgent {
		var last string
		if err := db.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(created_at),'') FROM owner_msg_log WHERE status='sent' AND kind!='reply'`).Scan(&last); err == nil && last != "" {
			if t, perr := time.Parse(time.RFC3339, last); perr == nil && time.Since(t) < 10*time.Minute {
				return
			}
		}
	}

	var facts string
	for _, o := range pending {
		facts += fmt.Sprintf("- [%s] %s\n", o.kind, o.fact)
	}
	text := e.composeOwnerOffer(ctx, facts)
	if strings.EqualFold(strings.TrimSpace(text), "hold") || strings.TrimSpace(text) == "" {
		// A friend wouldn't interrupt for this — the offers stay
		// readable in the digest, not on the owner's phone.
		_, _ = db.ExecContext(ctx, `UPDATE owner_offers SET status='digested', handled_at=? WHERE status='pending'`,
			time.Now().UTC().Format(time.RFC3339))
		return
	}
	if violations := ownerSendViolations(text); len(violations) > 0 {
		_, _ = db.ExecContext(ctx, `UPDATE owner_offers SET status='digested', handled_at=? WHERE status='pending'`,
			time.Now().UTC().Format(time.RFC3339))
		e.logOwnerMessage(ctx, "offer", facts, "", "rejected", strings.Join(violations, "; "))
		return
	}

	msgID := "cht_" + uuid.NewString()[:12]
	_ = e.memory.SaveChatMessage(ctx, core.ChatMessage{
		ID:        msgID,
		AgentID:   e.agentID,
		Role:      "assistant",
		Source:    "proactive",
		Content:   text,
		CreatedAt: time.Now().UTC(),
	})
	e.sendTelegramTextEscalation(ctx, text)
	e.logOwnerMessage(ctx, "offer", facts, text, "sent", "")
	_, _ = db.ExecContext(ctx, `UPDATE owner_offers SET status='mentioned', handled_at=? WHERE status='pending'`,
		time.Now().UTC().Format(time.RFC3339))
}

// composeOwnerOffer is one turn of the real chat agent — same system
// prompt, same personality, same conversation history — deciding whether
// pending offers deserve a message. Returns "HOLD" or the message text.
func (e *Engine) composeOwnerOffer(ctx context.Context, facts string) string {
	soul := constitution.Text()
	index := e.buildCanonicalIndexWithSkills()
	goals, _ := e.memory.ReadWiki("goals.md")
	wikiPages, _ := e.memory.ListWiki()
	wsFiles, _ := e.sandbox.ListFiles()
	history, _ := e.memory.RecentChatMessages(ctx, 10)
	sysPrompt := FastChatSystemPrompt(soul, index, extractActiveGoals(goals), wikiPages, wsFiles)

	turns := make([]core.ChatTurn, 0, len(history)+1)
	for _, h := range history {
		if h.Role != "user" && h.Role != "assistant" {
			continue
		}
		if strings.Contains(h.Content, "⚡ Executed in `bwrap` sandbox") || strings.HasPrefix(h.Content, "Ran `workspace_read") {
			continue
		}
		turns = append(turns, core.ChatTurn{Role: h.Role, Content: h.Content})
	}
	turns = append(turns, core.ChatTurn{Role: "user", Content: `Things that happened inside ACE since you last spoke (internal facts — never show the owner raw IDs, paths or jargon):

` + facts + `

Decide as a friend: if any of this is worth interrupting Jay for, write ONE short message covering it — fold related items together, give the real numbers and name the result by its file's plain name (e.g. "the c_s31 data check"). Never write S-numbers, T_-codes, backticks, or file paths — the checker rejects them and the message dies unsent. If nothing is worth interrupting for, reply with exactly: HOLD`})

	resp, err := e.llm.StreamComplete(ctx, core.CompletionRequest{
		SystemPrompt: sysPrompt,
		Messages:     turns,
		MaxTokens:    512,
		Temperature:  0.5,
	}, nil)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(resp.Content)
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
		 WHERE kind IN ('UNCERTAINTY','CONTRADICTION') AND state IN ('OPEN','QUARANTINED') AND needs_owner=1 AND asked_at=''
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
	// The question cites the ACE record it rests on — without evidence the
	// item was never owner-grounded, so it stays in the digest.
	var evidence string
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(owner_evidence,'') FROM connectome_nodes WHERE id=?`, id).Scan(&evidence)
	if strings.TrimSpace(evidence) == "" {
		return
	}
	e.NotifyOwnerSourced(ctx, "question",
		"an open question only the owner can close (they can take their time — I keep working on my stated assumption): "+summary+
			"\nresting on: "+evidence, id)
	_, _ = db.ExecContext(ctx, `UPDATE connectome_nodes SET asked_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), id)
}

// maybeNotifyOwnerResults turns verified completions that moved their
// branch's primary statistic into result offers for the chat agent. A
// result offer must carry the full fact — stat name, before→after, n,
// p-value and the result file; anything missing is digest, not a message.
func (e *Engine) maybeNotifyOwnerResults(ctx context.Context) {
	db := e.getDB()
	if db == nil {
		return
	}
	// Expire every stale completion in one write — a deep backlog must not
	// push fresh news past the 15-minute bound one row per tick.
	_, _ = db.ExecContext(ctx, `
		UPDATE branch_progress SET notified=1
		WHERE notified = 0
		AND replace(substr(created_at,1,19),'T',' ') < datetime('now','-15 minutes')
		AND task_id IN (SELECT id FROM mission_dag_nodes WHERE state='COMPLETED')`)
	rows, err := db.QueryContext(ctx, `
		SELECT bp.id, bp.task_id, bp.moved, bp.created_at, COALESCE(n.title,'')
		FROM branch_progress bp
		JOIN mission_dag_nodes n ON n.id = bp.task_id
		WHERE bp.notified = 0 AND n.state = 'COMPLETED'
		ORDER BY bp.id LIMIT 5`)
	if err != nil {
		return
	}
	type rowT struct {
		bpID, moved    int
		taskID, createdAt, title string
	}
	var pending []rowT
	for rows.Next() {
		var r rowT
		if rows.Scan(&r.bpID, &r.taskID, &r.moved, &r.createdAt, &r.title) == nil {
			pending = append(pending, r)
		}
	}
	rows.Close() // close before any write — SQLite's single conn deadlocks otherwise

	for _, r := range pending {
		stale := false
		for _, layout := range []string{"2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05", time.RFC3339} {
			if t, perr := time.Parse(layout, r.createdAt); perr == nil {
				stale = time.Since(t) > 15*time.Minute
				break
			}
		}
		fact, reason := e.resultFactForTask(ctx, r.taskID, r.bpID)
		if fact == "" || stale {
			// Incomplete fact, no clear effect, or old news → digest.
			if stale || fact == "" {
				_, _ = db.ExecContext(ctx, `UPDATE branch_progress SET notified=1 WHERE id=?`, r.bpID)
			}
			if fact == "" && !stale {
				if reason == "" {
					reason = "result fact incomplete (stat/before/after/n/p/file)"
				}
				e.appendOwnerDigest(ctx, "result",
					reason+": "+truncateText(r.title, 150))
			}
			continue
		}
		e.NotifyOwnerSourced(ctx, "result", fact, r.taskID)
		_, _ = db.ExecContext(ctx, `UPDATE branch_progress SET notified=1 WHERE id=?`, r.bpID)
	}
}

// resultAlpha is the significance bound for a result message. The
// pre-registered alpha wins when a question exists; prereg is empty on
// this mission so the conservative default applies.
func (e *Engine) resultAlpha(ctx context.Context, taskID string) float64 {
	if db := e.getDB(); db != nil {
		stem := strings.ToLower(strings.TrimPrefix(taskID, "T_"))
		var a float64
		if err := db.QueryRowContext(ctx,
			`SELECT q.alpha FROM prereg_questions q
				JOIN prereg_attempts a2 ON a2.question_id = q.id
				WHERE a2.result_ref LIKE '%' || ? || '%'
				ORDER BY a2.created_at DESC LIMIT 1`,
			stem).Scan(&a); err == nil && a > 0 && a < 1 {
			return a
		}
	}
	return 0.05
}

// resultFactForTask builds the complete result fact from the artifact and
// the branch's own history: stat name, branch's previous primary value →
// new value, n, exact binomial p vs the branch baseline, and the result
// filename. Returns ("", reason) when the fact is incomplete or the
// effect is not significant — p must beat the pre-registered alpha.
func (e *Engine) resultFactForTask(ctx context.Context, taskID string, bpID int) (string, string) {
	dbg := os.Getenv("ACE_DEBUG_RESULT_FACT") != ""
	db := e.getDB()
	wsDir, _ := e.resolveSandboxDirs()
	if db == nil || wsDir == "" {
		if dbg {
			fmt.Fprintf(os.Stderr, "[resultfact] early: db=%v wsDir=%q\n", db != nil, wsDir)
		}
		return "", "no database or workspace"
	}
	stem := strings.ToLower(strings.TrimPrefix(taskID, "T_"))
	var blob []byte
	var rel string
	for _, cand := range []string{"results/" + stem + ".json", "results/" + strings.TrimSuffix(stem, "_data") + ".json"} {
		if b, err := os.ReadFile(filepath.Join(wsDir, ResolveWorkspaceArtifactPath(wsDir, cand))); err == nil {
			blob, rel = b, cand
			break
		}
	}
	if len(blob) == 0 {
		if dbg {
			fmt.Fprintf(os.Stderr, "[resultfact] %s: no artifact\n", taskID)
		}
		return "", "no result artifact"
	}
	var res struct {
		HeldoutAcc   *float64 `json:"heldout_accuracy"`
		HeldoutScore *float64 `json:"heldout_score"`
		Heldout      *float64 `json:"heldout"`
		BaselineAcc  *float64 `json:"baseline_accuracy"`
		Baseline     *float64 `json:"baseline"`
		NHeldout     *int     `json:"n_heldout"`
		N            *int     `json:"n"`
		PrimaryStat  *string  `json:"primary_statistic"`
	}
	if json.Unmarshal(blob, &res) != nil {
		return "", "result artifact unreadable"
	}
	// Stat name comes from the key the artifact actually reports.
	stat, after := "", (*float64)(nil)
	switch {
	case res.HeldoutAcc != nil:
		stat, after = "held-out accuracy", res.HeldoutAcc
	case res.HeldoutScore != nil:
		stat, after = "held-out score", res.HeldoutScore
	case res.Heldout != nil:
		stat, after = "held-out value", res.Heldout
	}
	if stat == "" {
		return "", "no held-out statistic in result artifact"
	}
	if res.PrimaryStat != nil && strings.TrimSpace(*res.PrimaryStat) != "" {
		stat = strings.TrimSpace(*res.PrimaryStat)
	}
	n := 0
	if res.NHeldout != nil {
		n = *res.NHeldout
	} else if res.N != nil {
		n = *res.N
	}
	if n <= 0 {
		return "", "no held-out n in result artifact"
	}
	// The branch's previous primary statistic: the last after_val recorded
	// for this branch before this row — not the task's own baseline.
	branchID := ""
	_ = db.QueryRowContext(ctx, `SELECT branch_id FROM branch_progress WHERE id=?`, bpID).Scan(&branchID)
	before, beforeOK := 0.0, false
	if branchID != "" {
		var prev string
		if err := db.QueryRowContext(ctx,
			`SELECT after_val FROM branch_progress WHERE branch_id=? AND id<? AND after_val!='' ORDER BY id DESC LIMIT 1`,
			branchID, bpID).Scan(&prev); err == nil {
			if v, perr := strconv.ParseFloat(prev, 64); perr == nil {
				before, beforeOK = v, true
			}
		}
	}
	if !beforeOK {
		// First measurement on the branch: the artifact's baseline is the
		// number the branch started from.
		if res.BaselineAcc != nil {
			before, beforeOK = *res.BaselineAcc, true
		} else if res.Baseline != nil {
			before, beforeOK = *res.Baseline, true
		}
	}
	if !beforeOK {
		return "", "no branch baseline to compare against"
	}
	// Exact one-sided binomial test: correct = after*n, null = before rate.
	alpha := e.resultAlpha(ctx, taskID)
	if *after == before {
		_, _ = db.ExecContext(ctx,
			`UPDATE branch_progress SET stat_name=?, before_val=?, after_val=?, n=?, result_file=? WHERE id=?`,
			stat, fmt.Sprintf("%.4f", before), fmt.Sprintf("%.4f", *after), n, rel, bpID)
		return "", fmt.Sprintf("result equals baseline — likely predicting the most common answer (%s %.3f, n=%d)",
			stat, *after, n)
	}
	if *after < before {
		pDown := 1.0
		if before > 0 && before < 1 && n <= 100000 {
			k := int(math.Round(*after * float64(n)))
			pDown = 1 - binomTail(k+1, n, before)
		}
		_, _ = db.ExecContext(ctx,
			`UPDATE branch_progress SET stat_name=?, before_val=?, after_val=?, n=?, pval=?, result_file=? WHERE id=?`,
			stat, fmt.Sprintf("%.4f", before), fmt.Sprintf("%.4f", *after), n, fmt.Sprintf("p=%.4g", pDown), rel, bpID)
		if pDown < alpha {
			return "", fmt.Sprintf("worse than baseline (%s %.3f → %.3f, n=%d, p=%.4g < α=%.3g)",
				stat, before, *after, n, pDown, alpha)
		}
		return "", fmt.Sprintf("no clear effect yet (%s %.3f → %.3f, n=%d, p=%.4g ≥ α=%.3g)",
			stat, before, *after, n, pDown, alpha)
	}
	p := 1.0
	if before > 0 && before < 1 && n <= 100000 {
		k := int(math.Round(*after * float64(n)))
		p = binomTail(k, n, before)
	}
	_, _ = db.ExecContext(ctx,
		`UPDATE branch_progress SET stat_name=?, before_val=?, after_val=?, n=?, pval=?, result_file=? WHERE id=?`,
		stat, fmt.Sprintf("%.4f", before), fmt.Sprintf("%.4f", *after), n, fmt.Sprintf("p=%.4g", p), rel, bpID)
	if p >= alpha {
		return "", fmt.Sprintf("no clear effect yet (%s %.3f → %.3f, n=%d, p=%.4g ≥ α=%.3g)",
			stat, before, *after, n, p, alpha)
	}
	fact := fmt.Sprintf(
		"verified completion: %s moved %.3f → %.3f (n=%d, p=%.4g vs the branch's previous level, α=%.3g), result file %s",
		stat, before, *after, n, p, alpha, rel)
	return fact, ""
}

// binomTail is P(X >= k) under Binomial(n, p) — the one-sided exact test
// for "the held-out rate beats the branch's previous level".
func binomTail(k, n int, p float64) float64 {
	if k <= 0 {
		return 1
	}
	if k > n {
		return 0
	}
	sum := 0.0
	for i := k; i <= n; i++ {
		sum += math.Exp(lgamma(float64(n)+1) - lgamma(float64(i)+1) - lgamma(float64(n-i)+1) +
			float64(i)*math.Log(p) + float64(n-i)*math.Log(1-p))
	}
	if sum > 1 {
		sum = 1
	}
	return sum
}

func lgamma(x float64) float64 { r, _ := math.Lgamma(x); return r }

func (e *Engine) logOwnerMessage(ctx context.Context, kind, fact, text, status, reason string) {
	if db := e.getDB(); db != nil {
		_, _ = db.ExecContext(ctx, `INSERT INTO owner_msg_log (kind, fact, text, status, reason, created_at) VALUES (?,?,?,?,?,?)`,
			kind, truncateText(fact, 500), truncateText(text, 500), status, truncateText(reason, 200),
			time.Now().UTC().Format(time.RFC3339))
	}
}
