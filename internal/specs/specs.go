package specs

// specs.go holds the living-spec machinery moved from
// internal/cognitive/spec_gate.go (spec 9.6): budgets, veto hashing,
// human summaries, section extraction.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"ace/internal/contextsys"
	"strconv"
	"strings"
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
	paths := contextsys.ConnectomePathTokenRe.FindAllString(vetoReason, -1)
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
// and implements the 2-stage Stuck Detector (§6.1):
//   - At count == 3: NEVER parks or escalates to Telegram; triggers a local L4 strategy revision.
func CleanHumanMilestoneTitle(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "- [ ]")
	s = strings.TrimPrefix(s, "- [!]")
	s = strings.TrimPrefix(s, "- [x]")
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, ":"); idx > 0 {
		s = s[:idx]
	}
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(TruncateString(s, 80))
}

func PlainEnglishBlockerSummary(vetoKind string, targetPath string, vetoReason string) string {
	lower := strings.ToLower(vetoReason)
	switch {
	case strings.Contains(lower, "scientific falsification") || strings.Contains(lower, "h1_accepted"):
		return fmt.Sprintf("The benchmark experiment in %s ran cleanly, but the new method has not yet beaten the baseline on the held-out test split.", filepath.Base(targetPath))
	case vetoKind == "missing_artifact":
		return fmt.Sprintf("The expected output file (%s) was not produced where the milestone check looks for it.", filepath.Base(targetPath))
	case vetoKind == "selftest_failure":
		return fmt.Sprintf("The self-test in %s is still failing after multiple code fixes.", filepath.Base(targetPath))
	case vetoKind == "hygiene_violation":
		return fmt.Sprintf("Code quality check flagged unused imports or dead code in %s.", filepath.Base(targetPath))
	default:
		return fmt.Sprintf("The verification gate blocked %s after repeated attempts.", filepath.Base(targetPath))
	}
}

// EscalateGateDisputeOrStuck opens a CONTRADICTION node in the shared Connectome and routes a plain-English
var SpecSectionRe = regexp.MustCompile(`(?m)^##\s*(?:Section\s*)?([1-5])\b[^\n]*\n`)

// ParseAndValidateLivingSpec validates that a Living Spec at `/wiki/specs/<milestone_id>.md`:
// 1. Is <= SpecBudgetMaxBytes() (default 2,800B),
// 2. Contains all 5 mandatory sections (§1 Scope, §2 Verified Facts, §3 Target Interfaces, §4 Invariants/Edge Cases, §5 Falsification/Acceptance),
func ExtractSpecSections(spec string) map[int]string {
	out := make(map[int]string)
	locs := SpecSectionRe.FindAllStringSubmatchIndex(spec, -1)
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

func ExtractMilestoneShortID(milestone string) string {
	re := regexp.MustCompile(`(?i)\b(?:axis|milestone|step|task)\s+([0-9]+(?:\.[0-9]+)*)`)
	if m := re.FindStringSubmatch(milestone); len(m) >= 2 {
		return "ms_" + strings.ReplaceAll(m[1], ".", "_")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(milestone)))
	return "ms_" + hex.EncodeToString(sum[:6])
}

func TruncateString(s string, maxLen int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
