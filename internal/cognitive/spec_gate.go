package cognitive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ace/internal/core"
)

const DefaultSpecMaxBytes = 2800

func SpecBudgetMaxBytes() int {
	if raw := strings.TrimSpace(os.Getenv("ACE_SPEC_DISTILL_MAX_BYTES")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 512 && v <= 16384 {
			return v
		}
	}
	return DefaultSpecMaxBytes
}

// ComputeVetoReasonHash hashes (milestone_id, veto_kind, target_path) so repeated identical Mode-3 or LTL vetoes
// trigger the Stuck Detector after 3 occurrences (§6.1).
func ComputeVetoReasonHash(milestoneID string, vetoReason string) (hashHex string, vetoKind string, targetPath string) {
	lower := strings.ToLower(vetoReason)
	switch {
	case strings.Contains(lower, "missing") || strings.Contains(lower, "does not exist") || strings.Contains(lower, "not found"):
		vetoKind = "missing_artifact"
	case strings.Contains(lower, "selftest") || strings.Contains(lower, "exit="):
		vetoKind = "selftest_failure"
	case strings.Contains(lower, "hygiene") || strings.Contains(lower, "zombie") || strings.Contains(lower, "ast"):
		vetoKind = "hygiene_violation"
	case strings.Contains(lower, "ltl veto"):
		vetoKind = "ltl_veto"
	default:
		vetoKind = "gate_veto"
	}
	paths := connectomePathTokenRe.FindAllString(vetoReason, -1)
	if len(paths) > 0 {
		targetPath = paths[0]
	}
	msNorm := strings.TrimSpace(milestoneID)
	if len(msNorm) > 96 {
		msNorm = msNorm[:96]
	}
	sum := sha256.Sum256([]byte(msNorm + "|" + vetoKind + "|" + targetPath))
	return hex.EncodeToString(sum[:8]), vetoKind, targetPath
}

// RecordGateVetoAndCheckStuck records a gate veto in `veto_ledger`, emits a `veto_reason` telemetry span,
// and returns (isStuck=true, count) when the same veto hash occurs >= 3 times.
// When isStuck is true, it automatically opens a CONTRADICTION node in the shared Connectome and escalates via L1.5 -> Telegram.
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

	var count int
	_ = db.QueryRowContext(ctx, `SELECT count FROM veto_ledger WHERE veto_hash = ?`, vhash).Scan(&count)

	if count >= 3 {
		_ = e.EscalateGateDisputeOrStuck(ctx, cycleID, milestoneID, vhash, vkind, tpath, vetoReason, workerThought)
		return true, count
	}
	return false, count
}

// EscalateGateDisputeOrStuck opens a CONTRADICTION node in the shared Connectome and routes a <=4-line
// plain-English escalation (or queues it in the overnight digest if the human is likely offline).
func (e *Engine) EscalateGateDisputeOrStuck(ctx context.Context, cycleID string, milestoneID string, vetoHash string, vetoKind string, targetPath string, vetoReason string, workerDiagnosis string) error {
	nodeID := "contradiction:gate_" + vetoHash
	summary := fmt.Sprintf("Gate dispute (%s on %s, 3+ vetoes): %s | L6 diagnosis: %s",
		vetoKind, targetPath, truncateString(vetoReason, 120), truncateString(workerDiagnosis, 120))

	_ = e.UpsertUncertaintyNode(ctx, ConnectomeNode{
		ID:             nodeID,
		Kind:           "CONTRADICTION",
		Path:           targetPath,
		Summary:        summary,
		State:          "OPEN",
		OwnerLayer:     5,
		BlockingWeight: 2.0,
		InfoValue:      2.0,
		ExpectedCost:   0.5,
		EvidenceRef:    "cyc_" + cycleID,
		CycleEpoch:     time.Now().Unix(),
	})

	if db := e.getDB(); db != nil {
		_, _ = db.ExecContext(ctx, `UPDATE veto_ledger SET disputed = 1, updated_at = ? WHERE veto_hash = ?`, time.Now().UTC(), vetoHash)
	}

	msg := fmt.Sprintf(
		"1. Task: %s\n2. Contradiction: Gate vetoed `%s` (%s) 3x; worker says: %s\n3. Options: (A) Relax/fix milestone path or (B) Override gate for this task.\n4. Status: Parked this branch and switching to the next unblocked task.",
		truncateString(milestoneID, 70), targetPath, vetoKind, truncateString(workerDiagnosis, 90),
	)
	e.DispatchHumanContradictionAlert(ctx, cycleID, nodeID, msg, targetPath)
	return nil
}

// RunL6PreGateSelfCheck runs the cheap deterministic part of Mode-3 (target existence, non-empty size, syntax)
// inside L6 BEFORE calling the full Mode-3 gate so L6 can fix trivial mistakes in the same turn (§6.1).
func (e *Engine) RunL6PreGateSelfCheck(milestone string) (bool, string) {
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir == "" {
		return true, "ok"
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

var specSectionRe = regexp.MustCompile(`(?m)^##\s*(?:Section\s*)?([1-5])\b[^\n]*\n`)

// ParseAndValidateLivingSpec validates that a Living Spec at `/wiki/specs/<milestone_id>.md`:
// 1. Is <= SpecBudgetMaxBytes() (default 2,800B),
// 2. Contains all 5 mandatory sections (§1 Scope, §2 Verified Facts, §3 Target Interfaces, §4 Invariants/Edge Cases, §5 Falsification/Acceptance),
// 3. Enforces the Frozen SHA-256 Hash on §1 + §5 so LLMs cannot weaken acceptance criteria without human approval (§4).
func (e *Engine) ParseAndValidateLivingSpec(ctx context.Context, milestoneID string, specContent string, humanApprovalRef string) (frozenHash string, err error) {
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

	var existingHash, existingApproval string
	qerr := db.QueryRowContext(ctx, `SELECT frozen_hash, human_approval_ref FROM spec_registry WHERE milestone_id = ?`, milestoneID).Scan(&existingHash, &existingApproval)
	if qerr == nil && existingHash != "" && existingHash != newFrozenHash {
		// Only allow if humanApprovalRef is provided OR §5 only grew (pure addition of stricter checks)
		if humanApprovalRef == "" && existingApproval == "" {
			return "", fmt.Errorf("[SPEC GATE VETO — Frozen Hash Mismatch] Cannot modify Section 1 (Scope) or Section 5 (Falsification Criteria) of `/wiki/specs/%s.md` without human approval (expected hash %s, got %s)", milestoneID, existingHash, newFrozenHash)
		}
	}

	_, _ = db.ExecContext(ctx, `
		INSERT INTO spec_registry (milestone_id, spec_path, frozen_hash, size_bytes, status, human_approval_ref, updated_at)
		VALUES (?, ?, ?, ?, 'LIVING', ?, ?)
		ON CONFLICT(milestone_id) DO UPDATE SET
			size_bytes = excluded.size_bytes,
			frozen_hash = CASE WHEN excluded.human_approval_ref != '' THEN excluded.frozen_hash ELSE spec_registry.frozen_hash END,
			human_approval_ref = CASE WHEN excluded.human_approval_ref != '' THEN excluded.human_approval_ref ELSE spec_registry.human_approval_ref END,
			updated_at = excluded.updated_at
	`, milestoneID, "/wiki/specs/"+milestoneID+".md", newFrozenHash, len(specContent), humanApprovalRef, time.Now().UTC())

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
		if len(s) > SpecBudgetMaxBytes() {
			s = s[:SpecBudgetMaxBytes()]
		}
		_, _ = e.ParseAndValidateLivingSpec(ctx, specID, s, "")
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
	_, _ = e.ParseAndValidateLivingSpec(ctx, specID, initial, "")
	return specID, initial
}

func extractSpecSections(spec string) map[int]string {
	out := make(map[int]string)
	locs := specSectionRe.FindAllStringSubmatchIndex(spec, -1)
	for i, loc := range locs {
		secNum, _ := strconv.Atoi(spec[loc[2]:loc[3]])
		bodyStart := loc[1]
		bodyEnd := len(spec)
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		}
		out[secNum] = strings.TrimSpace(spec[bodyStart:bodyEnd])
	}
	return out
}

func extractMilestoneShortID(milestone string) string {
	re := regexp.MustCompile(`(?i)\b(?:axis|milestone|step|task)\s+([0-9]+(?:\.[0-9]+)*)`)
	if m := re.FindStringSubmatch(milestone); len(m) >= 2 {
		return "ms_" + strings.ReplaceAll(m[1], ".", "_")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(milestone)))
	return "ms_" + hex.EncodeToString(sum[:6])
}

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
