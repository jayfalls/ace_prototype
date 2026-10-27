package cognitive

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The check keeps ONLY mechanical rules — internal IDs, paths, jargon,
// tables, code formatting. Whether a reply carries numbers or status is
// the chat agent's judgment now (owner directive: delete ownerReStatusAsk).
func TestOwnerReplyViolations(t *testing.T) {
	cases := []struct {
		name     string
		ownerMsg string
		reply    string
		wantRej  bool
	}{
		{"ack gets ack", "I'm working", "Got it — good luck!", false},
		{"ack may carry numbers", "I'm working", "Cool, 36 records came back clean.", false},
		{"ack may carry status", "I'm working", "Nice — three tasks completed while you were out.", false},
		{"long ack is fine", "thanks", strings.Repeat("word ", 30), false},
		{"status asked, numbers ok", "how's it going?", "Making progress — 12 things done, nothing on fire.", false},
		{"status asked, status words ok", "what broke last night?", "Two tasks failed overnight but the queue recovered.", false},
		{"jargon always rejected", "how's it going?", "L4 vetoed the task again.", true},
		{"id always rejected", "how's it going?", "Finished T_C_S22 just now.", true},
		{"path always rejected", "what broke?", "Something in /workspace/modules blew up.", true},
		{"numbers unprompted still ok", "still busy?", "Yep — 3 things in flight.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ownerReplyViolations(c.reply, c.ownerMsg)
			require.Equal(t, c.wantRej, len(got) > 0, "violations: %v", got)
		})
	}
}
