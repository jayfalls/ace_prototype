package cognitive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ace/internal/core"
	"ace/internal/gate"
	"ace/internal/specs"
)

// Spec 9.6: living-spec machinery lives in internal/specs. Aliases keep
// the engine's gate methods readable.
const DefaultSpecMaxBytes = specs.DefaultSpecMaxBytes

var (
	SpecBudgetMaxBytes         = specs.SpecBudgetMaxBytes
	ComputeVetoReasonHash      = specs.ComputeVetoReasonHash
	specSectionRe              = specs.SpecSectionRe
	extractSpecSections        = specs.ExtractSpecSections
	extractMilestoneShortID    = specs.ExtractMilestoneShortID
	truncateString             = specs.TruncateString
	cleanHumanMilestoneTitle   = specs.CleanHumanMilestoneTitle
	plainEnglishBlockerSummary = specs.PlainEnglishBlockerSummary
)

// trigger the Stuck Detector after 3 occurrences (§6.1).
//   - At count >= 5 (or >= 8 for active code/hypothesis debugging): opens a CONTRADICTION node and escalates via L1.5 -> Telegram once.
func (e *Engine) RecordGateVetoAndCheckStuck(ctx context.Context, cycleID string, milestoneID string, vetoReason string, workerThought string) (bool, int) {
	vhash, vkind, tpath := ComputeVetoReasonHash(milestoneID, vetoReason)
	if e.telem != nil {
		_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
			TraceID:   cycleID,
			AgentID:   e.agentID,
			LayerCode: "L5",
			Operation: "veto_reason",
			Status:    vkind,
			Detail:    fmt.Sprintf("hash=%s kind=%s target=%s reason=%s", vhash, vkind, tpath, truncateString(vetoReason, 160)),
		})
	}

	db := e.getDB()
	if db == nil {
		return false, 1
	}

	now := time.Now().UTC()
	_, _ = db.ExecContext(ctx, `
		INSERT INTO veto_ledger (veto_hash, milestone_id, veto_kind, target_path, count, last_reason, last_cycle_id, disputed, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?, 0, ?)
		ON CONFLICT(veto_hash) DO UPDATE SET
			count = veto_ledger.count + 1,
			last_reason = excluded.last_reason,
			last_cycle_id = excluded.last_cycle_id,
			updated_at = excluded.updated_at
	`, vhash, milestoneID, vkind, tpath, truncateString(vetoReason, 300), cycleID, now)

	var count, disputed, ruledCount int
	_ = db.QueryRowContext(ctx, `SELECT count, disputed, COALESCE(ruled_count,0) FROM veto_ledger WHERE veto_hash = ?`, vhash).Scan(&count, &disputed, &ruledCount)

	// Bounded retries after a ruling (owner rule): once a dispute has been
	// ruled, the next 3 identical vetoes park the node — no unbounded
	// retry loop. ruled_count==0 on a disputed row means the ruling
	// predates the column; those rows are already far past +3, so park on
	// the next occurrence and stamp the audit baseline.
	if disputed == 1 && (ruledCount == 0 || count >= ruledCount+3) {
		if ruledCount == 0 {
			_, _ = db.ExecContext(ctx, `UPDATE veto_ledger SET ruled_count = ? WHERE veto_hash = ?`, count, vhash)
		}
		if parked := e.ParkDAGTaskForMilestone(ctx, milestoneID); parked > 0 && e.telem != nil {
			_ = e.telem.RecordSpan(ctx, core.TelemetrySpan{
				TraceID: cycleID, AgentID: e.agentID, LayerCode: "L5",
				Operation: "veto_park", Status: "QUARANTINED",
				Detail: fmt.Sprintf("hash=%s count=%d ruled_count=%d parked=%d", vhash, count, ruledCount, parked),
			})
		}
		return true, count
	}

	// Per §6.1: 3 vetoes only forces local L4 strategy reflection and NEVER parks or escalates to the human.
	// Normal code/hypothesis debugging (selftest_failure or scientific falsification) is given up to 8 attempts before human escalation.
	threshold := 5
	lowerReason := strings.ToLower(vetoReason)
	if vkind == "selftest_failure" || strings.Contains(lowerReason, "scientific falsification") || strings.Contains(lowerReason, "h1_accepted") {
		threshold = 8
	}

	if count >= threshold {
		if disputed == 0 {
			_ = e.EscalateGateDisputeOrStuck(ctx, cycleID, milestoneID, vhash, vkind, tpath, vetoReason, workerThought)
		}
		return true, count
	}
	return false, count
}

// human summary (or queues it in the overnight digest if the human is likely offline).
func (e *Engine) EscalateGateDisputeOrStuck(ctx context.Context, cycleID string, milestoneID string, vetoHash string, vetoKind string, targetPath string, vetoReason string, workerDiagnosis string) error {
	nodeID := "contradiction:gate_" + vetoHash
	title := cleanHumanMilestoneTitle(milestoneID)
	blocker := plainEnglishBlockerSummary(vetoKind, targetPath, vetoReason)

	if db := e.getDB(); db != nil {
		var existingState string
		_ = db.QueryRowContext(ctx, `SELECT state FROM connectome_nodes WHERE id = ?`, nodeID).Scan(&existingState)
		if existingState == "QUARANTINED" {
			return nil
		}
		_, _ = db.ExecContext(ctx, `UPDATE veto_ledger SET disputed = 1, ruled_count = count, updated_at = ? WHERE veto_hash = ?`, time.Now().UTC(), vetoHash)
	}

	_ = e.UpsertUncertaintyNode(ctx, ConnectomeNode{
		ID:             nodeID,
		Kind:           "CONTRADICTION",
		Path:           targetPath,
		Summary:        fmt.Sprintf("%s — %s", title, blocker),
		State:          "OPEN",
		OwnerLayer:     5,
		BlockingWeight: 2.0,
		InfoValue:      2.0,
		ExpectedCost:   0.5,
		EvidenceRef:    "cyc_" + cycleID,
		CycleEpoch:     time.Now().Unix(),
	})

	// 8.4: L1 adjudicates — no human permission ask. One bounded Tier-2
	// ruling; denied or unresolved means the node quarantines and L3/L5
	// pick other work. The owner still gets a *report*.
	if e.gate != nil {
		if gd := e.gate.Ruling(ctx, gate.Action{
			Kind: "gate_dispute", Target: targetPath,
			Command: fmt.Sprintf("veto %s (%s) on %s: %s — worker diagnosis: %s. Allow = retry task, reject = quarantine",
				vetoKind, vetoHash, targetPath, vetoReason, workerDiagnosis),
			TaskID: milestoneID, Layer: 5,
		}); gd.Allowed() {
			if db := e.getDB(); db != nil {
				_, _ = db.ExecContext(ctx, `UPDATE connectome_nodes SET state = 'OPEN', updated_at = ? WHERE id = ?`, time.Now().UTC(), nodeID)
			}
			return nil
		}
	}
	// A denied ruling is a quarantine decision — park the DAG task too, not
	// just the connectome node, so selection never re-dispatches it.
	e.ParkDAGTaskForMilestone(ctx, milestoneID)
	// Parks/quarantines are digest news, not a Telegram ping (Human voice).
	e.NotifyOwner(ctx, "digest", fmt.Sprintf(
		"Task parked after ruling: %s — %s (ruling: %s). Work continues on the next item.",
		title, blocker, vetoReason))
	return nil
}

// RunL6PreGateSelfCheck runs the cheap deterministic part of Mode-3 (target existence, non-empty size, syntax)
// inside L6 BEFORE calling the full Mode-3 gate so L6 can fix trivial mistakes in the same turn (§6.1).
func (e *Engine) RunL6PreGateSelfCheck(milestone string) (bool, string) {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" {
		return false, "[L6 Self-Check] workspace root unresolved — cannot verify declared artifacts"
	}
	paths, _ := ExtractMilestoneArtifacts(milestone)
	for _, rel := range paths {
		if strings.Contains(rel, "*") || strings.HasSuffix(rel, "/") {
			continue
		}
		resolved := ResolveWorkspaceArtifactPath(wsDir, rel)
		info, err := os.Stat(resolved)
		if err != nil {
			return false, fmt.Sprintf("[L6 Self-Check] Missing declared target artifact `/workspace/%s`", rel)
		}
		if info.IsDir() {
			continue
		}
		if info.Size() == 0 {
			return false, fmt.Sprintf("[L6 Self-Check] Target artifact `/workspace/%s` is 0 bytes", rel)
		}
	}
	return true, "ok"
}

// 3. Enforces the Frozen SHA-256 Hash on §1 + §5 so LLMs cannot weaken acceptance criteria without an L1 ruling (§4).
func (e *Engine) ParseAndValidateLivingSpec(ctx context.Context, milestoneID string, specContent string) (frozenHash string, err error) {
	maxBytes := SpecBudgetMaxBytes()
	if len(specContent) > maxBytes {
		return "", fmt.Errorf("spec exceeds %dB budget (got %dB)", maxBytes, len(specContent))
	}

	sections := extractSpecSections(specContent)
	for s := 1; s <= 5; s++ {
		if strings.TrimSpace(sections[s]) == "" {
			return "", fmt.Errorf("spec is missing mandatory Section %d", s)
		}
	}

	sum := sha256.Sum256([]byte(strings.TrimSpace(sections[1]) + "\n---\n" + strings.TrimSpace(sections[5])))
	newFrozenHash := hex.EncodeToString(sum[:16])

	db := e.getDB()
	if db == nil {
		return newFrozenHash, nil
	}

	var existingHash string
	qerr := db.QueryRowContext(ctx, `SELECT frozen_hash FROM spec_registry WHERE milestone_id = ?`, milestoneID).Scan(&existingHash)
	if qerr == nil && existingHash != "" && existingHash != newFrozenHash {
		// 8.4: frozen-hash changes are high-stakes and require an L1 ruling —
		// there is no human approval path.
		if e.gate != nil {
			gd := e.gate.Ruling(ctx, gate.Action{
				Kind: "spec_mutation", Target: milestoneID,
				Command: fmt.Sprintf("frozen spec hash change %s -> %s (sections 1/5)", existingHash, newFrozenHash),
			})
			if !gd.Allowed() {
				return "", fmt.Errorf("[SPEC GATE VETO — L1 Rejected Frozen-Hash Change] %s (rule %s)", gd.Reason, gd.RuleID)
			}
		} else {
			return "", fmt.Errorf("[SPEC GATE VETO — Frozen Hash Mismatch] Cannot modify Section 1 (Scope) or Section 5 (Falsification Criteria) of `/wiki/specs/%s.md` without an L1 ruling (expected hash %s, got %s)", milestoneID, existingHash, newFrozenHash)
		}
	}

	_, _ = db.ExecContext(ctx, `
		INSERT INTO spec_registry (milestone_id, spec_path, frozen_hash, size_bytes, status, updated_at)
		VALUES (?, ?, ?, ?, 'LIVING', ?)
		ON CONFLICT(milestone_id) DO UPDATE SET
			size_bytes = excluded.size_bytes,
			frozen_hash = excluded.frozen_hash,
			updated_at = excluded.updated_at
	`, milestoneID, "/wiki/specs/"+milestoneID+".md", newFrozenHash, len(specContent), time.Now().UTC())

	return newFrozenHash, nil
}

// EnsureLivingSpecForMilestone loads `/wiki/specs/<milestone_id>.md` or deterministically synthesizes
// an initial <=2,800B 5-section Living Spec from the active milestone and Connectome anchors.
func (e *Engine) EnsureLivingSpecForMilestone(ctx context.Context, milestone string) (specID string, specText string) {
	specID = sanitizeTaskID(extractMilestoneShortID(milestone))
	_, wikiDir := e.resolveSandboxDirs()
	if wikiDir == "" {
		return specID, ""
	}
	specRel := filepath.Join("specs", specID+".md")
	specHost := filepath.Join(wikiDir, specRel)
	if raw, err := os.ReadFile(specHost); err == nil && len(raw) > 0 {
		s := string(raw)
		// Spec 9.6: over the cap is rejected — consolidate, never truncate.
		if err := specs.ValidateSpec(s, 0); err != nil {
			return specID, ""
		}
		_, _ = e.ParseAndValidateLivingSpec(ctx, specID, s)
		return specID, s
	}

	paths, _ := ExtractMilestoneArtifacts(milestone)
	var targetBullets strings.Builder
	if len(paths) == 0 {
		targetBullets.WriteString("- `file:/workspace/` (discover & bind target module nodes)\n")
	} else {
		for _, p := range paths {
			targetBullets.WriteString(fmt.Sprintf("- `file:/workspace/%s`\n", strings.TrimPrefix(p, "/")))
		}
	}

	initial := fmt.Sprintf(`# Living Spec: %s
## Section 1: Scope & Non-Goals (Frozen)
- **In-Scope**: %s
- **Out-of-Scope**: Unrelated workspace files, synthetic/random mock data, or weakening existing tests.

## Section 2: Verified Facts & Baselines
- Grounded in real workspace datasets and empirical execution traces.

## Section 3: Target Connectome & Interfaces
%s
## Section 4: Nuances, Invariants & Edge Cases
- Preserve AST-PDG conceptual hygiene (zero dead branches or zombie flows).
- Maintain bounded runtime and deterministic exit codes.

## Section 5: Falsification & Acceptance Criteria (Frozen)
- Every target script in Section 3 must exist, be non-empty, pass `+"`--selftest`"+` with exit=0, and satisfy Mode-3 substance verification.
`, specID, strings.TrimSpace(milestone), targetBullets.String())

	if len(initial) > SpecBudgetMaxBytes() {
		initial = initial[:SpecBudgetMaxBytes()]
	}
	_ = os.MkdirAll(filepath.Dir(specHost), 0o755)
	_ = os.WriteFile(specHost, []byte(initial), 0o644)
	_, _ = e.ParseAndValidateLivingSpec(ctx, specID, initial)
	return specID, initial
}
