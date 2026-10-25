package contextsys

import (
	"fmt"
	"strings"
	"time"
)

// RecapLine is the one-line recap of a turn: time, role and a short
// content head. After compaction the last context.recap_n (default 5)
// turns return with time and stats; older turns are bucketed into counts.
type RecapLine struct {
	At      time.Time
	Role    string
	Summary string
	Bytes   int
}

// Recap renders the one-line recap for a turn (≤160 chars).
func Recap(t Turn) RecapLine {
	sum := strings.Join(strings.Fields(t.Content), " ")
	if len(sum) > 96 {
		sum = sum[:96] + "…"
	}
	return RecapLine{At: t.At, Role: t.Role, Summary: sum, Bytes: len(t.Content)}
}

// RenderRecaps formats recaps after a compaction: the last n turns with
// time and byte stats, older turns bucketed by role. Empty for no turns.
func RenderRecaps(turns []Turn, n int) string {
	if len(turns) == 0 {
		return ""
	}
	if n <= 0 {
		n = 5
	}
	bucketed := map[string]int{}
	older := len(turns) - n
	if older < 0 {
		older = 0
	}
	for _, t := range turns[:older] {
		bucketed[t.Role]++
	}
	var b strings.Builder
	b.WriteString("=== SESSION RECAP ===\n")
	if older > 0 {
		fmt.Fprintf(&b, "earlier: %d turns compacted", older)
		for role, count := range bucketed {
			fmt.Fprintf(&b, " (%s×%d)", role, count)
		}
		b.WriteString("\n")
	}
	for _, t := range turns[len(turns)-minInt(n, len(turns)):] {
		r := Recap(t)
		fmt.Fprintf(&b, "- %s [%s] (%dB) %s\n", r.At.Format("15:04:05"), r.Role, r.Bytes, r.Summary)
	}
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
