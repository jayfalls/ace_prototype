package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ace/internal/core"
)

const HandoffMetaKey = "l6_handoff_packet_v1"

// FailedAttempt records a specific failed action or command across cycles so L6 never repeats it.
type FailedAttempt struct {
	CycleID        string `json:"cycle_id"`
	Tool           string `json:"tool"`
	Target         string `json:"target"`
	NormalizedHash string `json:"normalized_hash"`
	ExitCode       int    `json:"exit_code"`
	ErrorSnippet   string `json:"error_snippet"`
	Timestamp      string `json:"timestamp"`
}

// SomaticValence models affective homeostatic control signals for L5/L6 (inspired by OpenClaw/Soul specs).
type SomaticValence struct {
	Frustration float64 `json:"frustration"` // [0, 1] rises on repeated failure or LTL veto
	Boredom     float64 `json:"boredom"`     // [0, 1] rises on zero-diff cycles
	Reward      float64 `json:"reward"`      // [0, 1] rises on verified Mode-3 milestone checkoff
}

// CrossCycleHandoffPacket is persisted in engine_meta (`l6_handoff_packet_v1`) to eliminate cross-cycle amnesia.
type CrossCycleHandoffPacket struct {
	ActiveMilestone   string            `json:"active_milestone"`
	AttemptCount      int               `json:"attempt_count"`
	FailedAttempts    []FailedAttempt   `json:"failed_attempts"`
	VerifiedFiles     []string          `json:"verified_files"`
	FileDigests       map[string]string `json:"file_digests"`
	NextHypothesis    string            `json:"next_hypothesis"`
	ConsecutiveNoOp   int               `json:"consecutive_noop"`
	Valence           SomaticValence    `json:"valence"`
	LastUpdated       string            `json:"last_updated"`
}

var offlineNetInstallRe = regexp.MustCompile(`(?i)\b(pip3?\s+install|python3?\s+-m\s+pip\s+install|npm\s+install|npx\s+|curl\s+https?://|wget\s+https?://|git\s+clone\s+https?://)`)

func NormalizeCommandSignature(tool string, target string, content string) string {
	raw := strings.ToLower(strings.TrimSpace(tool + "|" + target + "|" + content))
	fields := strings.Fields(raw)
	joined := strings.Join(fields, " ")
	if len(joined) > 180 {
		joined = joined[:180]
	}
	return joined
}

// EvaluateLTLAction enforces temporal safety invariants (G1, G2, G3, G4) before executing an L6 action.
// Returns (allowed, violationReason).
func EvaluateLTLAction(action core.LayerAction, handoff CrossCycleHandoffPacket, alreadyReadInCycle map[string]bool) (bool, string) {
	tool := strings.ToLower(strings.TrimSpace(action.Type))
	target := strings.TrimSpace(action.Target)
	content := strings.TrimSpace(action.Content)
	cmdText := strings.TrimSpace(target + " " + content)

	// Invariant G1: Offline bwrap sandbox ban on network package managers (pip install, npm install, curl, wget)
	if tool == "sandbox_sh" && offlineNetInstallRe.MatchString(cmdText) {
		return false, "[LTL VETO G1 — Offline Sandbox Invariant] Network package installation (`pip install`, `npm install`, `curl`, `wget`) is impossible inside `bwrap --unshare-all` (no internet). Use Python 3.14 stdlib (`math`, `statistics`, `sqlite3`, `json`, `ast`, `re`, `collections`) or `import ace_rlm` (`/workspace/ace_rlm.py`), and remove any dead third-party imports (like unused `import numpy as np`)."
	}

	// Invariant G2: Anti-Cross-Cycle-Amnesia (block repeating an identical command that already failed with exit != 0)
	sig := NormalizeCommandSignature(tool, target, content)
	for _, fa := range handoff.FailedAttempts {
		if fa.NormalizedHash == sig && fa.ExitCode != 0 {
			return false, fmt.Sprintf("[LTL VETO G2 — Cross-Cycle Amnesia Guard] Command `%s` already failed in cycle %s with exit=%d (%s). Do NOT repeat the failed command; change your approach or fix the underlying code.", truncateForVeto(cmdText, 90), fa.CycleID, fa.ExitCode, fa.ErrorSnippet)
		}
	}

	// Invariant G3: Coeffect No-Redundant-Read within same cycle if unchanged
	if tool == "workspace_read" {
		clean := normalizeRelPath(target)
		if alreadyReadInCycle != nil && alreadyReadInCycle[clean] {
			return false, fmt.Sprintf("[LTL VETO G3 — Coeffect Read-Once Guard] `/workspace/%s` was already read or pre-injected in this cycle with unchanged SHA-256 digest. Proceed directly to `workspace_edit`, `workspace_write`, or `sandbox_sh`.", clean)
		}
	}

	// Invariant G4: Anti-Analysis-Paralysis (Max 1 read-only inspection turn before mandatory artifact write!)
	if handoff.ConsecutiveNoOp >= 1 && (tool == "workspace_read" || tool == "kb") {
		return false, fmt.Sprintf("[LTL VETO G4 — Analysis Paralysis Guard] You have already performed %d consecutive read-only inspection turn(s) without creating any target artifact. Further read-only inspection is BLOCKED. Emit `workspace_write` and/or `wiki_write` NOW to write the milestone artifact!", handoff.ConsecutiveNoOp)
	}

	return true, ""
}

func RecordHandoffAttempt(handoff *CrossCycleHandoffPacket, cycleID string, action core.LayerAction, exitCode int, errSnippet string, hasEmpiricalDiff bool, verifiedFiles []string, digests map[string]string) {
	handoff.AttemptCount++
	handoff.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	if digests != nil {
		handoff.FileDigests = digests
	}
	for _, vf := range verifiedFiles {
		clean := normalizeRelPath(vf)
		found := false
		for _, ex := range handoff.VerifiedFiles {
			if ex == clean {
				found = true
				break
			}
		}
		if !found && clean != "" {
			handoff.VerifiedFiles = append(handoff.VerifiedFiles, clean)
		}
	}

	if hasEmpiricalDiff {
		handoff.ConsecutiveNoOp = 0
		// Clear prior FailedAttempts when a file has been newly written/edited so re-running --selftest is never blocked by G2!
		handoff.FailedAttempts = nil
	}

	if exitCode != 0 {
		// Do not record G4 sequencing vetoes as G2 command-signature failures
		if !strings.Contains(errSnippet, "[LTL VETO G4") {
			sig := NormalizeCommandSignature(action.Type, action.Target, action.Content)
			snippet := strings.TrimSpace(errSnippet)
			if len(snippet) > 220 {
				snippet = "..." + snippet[len(snippet)-217:]
			}
			// Deduplicate by NormalizedHash
			updated := false
			for i := range handoff.FailedAttempts {
				if handoff.FailedAttempts[i].NormalizedHash == sig {
					handoff.FailedAttempts[i].CycleID = cycleID
					handoff.FailedAttempts[i].ExitCode = exitCode
					handoff.FailedAttempts[i].ErrorSnippet = snippet
					handoff.FailedAttempts[i].Timestamp = handoff.LastUpdated
					updated = true
					break
				}
			}
			if !updated {
				handoff.FailedAttempts = append(handoff.FailedAttempts, FailedAttempt{
					CycleID:        cycleID,
					Tool:           action.Type,
					Target:         action.Target,
					NormalizedHash: sig,
					ExitCode:       exitCode,
					ErrorSnippet:   snippet,
					Timestamp:      handoff.LastUpdated,
				})
				if len(handoff.FailedAttempts) > 6 {
					handoff.FailedAttempts = handoff.FailedAttempts[len(handoff.FailedAttempts)-6:]
				}
			}
		}
		handoff.Valence.Frustration = clamp01(handoff.Valence.Frustration + 0.30)
		handoff.Valence.Reward = clamp01(handoff.Valence.Reward - 0.15)
	} else if hasEmpiricalDiff {
		// Anti-Identification-as-Completion Law: only an empirical workspace diff + exit=0 resets frustration/noop counters!
		handoff.ConsecutiveNoOp = 0
		handoff.Valence.Frustration = clamp01(handoff.Valence.Frustration - 0.40)
		handoff.Valence.Boredom = 0
		handoff.Valence.Reward = clamp01(handoff.Valence.Reward + 0.35)
	} else if strings.Contains(strings.ToLower(action.Type), "web_search") || strings.Contains(strings.ToLower(action.Type), "web_fetch") {
		// Proactive external web research / data gathering: do not penalize the first active web-research turn as an idle G4 no-op!
		if handoff.ConsecutiveNoOp > 0 {
			handoff.ConsecutiveNoOp++
		}
		handoff.Valence.Reward = clamp01(handoff.Valence.Reward + 0.10)
	} else {
		handoff.ConsecutiveNoOp++
		handoff.Valence.Boredom = clamp01(handoff.Valence.Boredom + 0.25)
	}
}

func LoadHandoffPacket(ctx context.Context, mem core.MemoryStore, currentMilestone string) CrossCycleHandoffPacket {
	var pkt CrossCycleHandoffPacket
	if mem != nil {
		if raw, err := mem.LoadEngineMeta(ctx, HandoffMetaKey); err == nil && strings.TrimSpace(raw) != "" {
			_ = json.Unmarshal([]byte(raw), &pkt)
		}
	}
	if pkt.FileDigests == nil {
		pkt.FileDigests = make(map[string]string)
	}
	// If milestone changed, reset attempt counters and failed attempts for the new milestone while keeping VerifiedFiles
	if currentMilestone != "" && pkt.ActiveMilestone != "" && pkt.ActiveMilestone != currentMilestone {
		pkt.ActiveMilestone = currentMilestone
		pkt.AttemptCount = 0
		pkt.FailedAttempts = nil
		pkt.ConsecutiveNoOp = 0
		pkt.Valence = SomaticValence{Reward: 0.5}
	} else if currentMilestone != "" {
		pkt.ActiveMilestone = currentMilestone
	}
	return pkt
}

func SaveHandoffPacket(ctx context.Context, mem core.MemoryStore, pkt CrossCycleHandoffPacket) {
	if mem == nil {
		return
	}
	pkt.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	if b, err := json.Marshal(pkt); err == nil {
		_ = mem.SaveEngineMeta(ctx, HandoffMetaKey, string(b))
	}
}

func FormatHandoffBucket(pkt CrossCycleHandoffPacket, zoneCtx string) string {
	var b strings.Builder
	// Place Handoff, LTL G4 Lock, and FailedAttempts/Tracebacks FIRST so they are NEVER truncated by layer status summaries!
	b.WriteString(fmt.Sprintf("Handoff[attempts=%d, noop=%d, frust=%.2f, reward=%.2f", pkt.AttemptCount, pkt.ConsecutiveNoOp, pkt.Valence.Frustration, pkt.Valence.Reward))
	if len(pkt.VerifiedFiles) > 0 {
		b.WriteString(", verified=" + strings.Join(pkt.VerifiedFiles, ","))
	}
	b.WriteString("]")
	if pkt.ConsecutiveNoOp >= 1 {
		b.WriteString(fmt.Sprintf("\n⚠️ LTL G4 LOCK ACTIVE (noop=%d): Read-only sandbox_sh/workspace_read is VETOED! You MUST emit action.type=\"workspace_write\" or \"workspace_edit\" (and/or \"wiki_write\" in actions:[...]) on THIS turn!", pkt.ConsecutiveNoOp))
	}
	if len(pkt.FailedAttempts) > 0 {
		b.WriteString("\n🚨 LAST FAILED ATTEMPTS / TRACEBACK TO FIX IN CODE NOW (DO NOT REPEAT COMMAND WITHOUT EDITING CODE):")
		start := 0
		if len(pkt.FailedAttempts) > 3 {
			start = len(pkt.FailedAttempts) - 3
		}
		for _, fa := range pkt.FailedAttempts[start:] {
			b.WriteString(fmt.Sprintf("\n- FAILED(exit=%d) `%s` -> %s", fa.ExitCode, truncateForVeto(fa.Target, 85), truncateForVeto(fa.ErrorSnippet, 420)))
		}
	}
	if strings.TrimSpace(zoneCtx) != "" {
		b.WriteString("\n")
		b.WriteString(zoneCtx)
	}
	out := b.String()
	// Calibrated SHERLOC LTL Counterexample Handoff Slice Ceiling (2,854 bytes for failed attempt slices + zone tail)
	if len(out) > SLABucketZoneContextMaxBytes {
		out = out[:SLABucketZoneContextMaxBytes]
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func truncateForVeto(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
