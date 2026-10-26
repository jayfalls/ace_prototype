package cognitive

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnerReplyViolations(t *testing.T) {
	cases := []struct {
		name     string
		ownerMsg string
		reply    string
		wantRej  bool
	}{
		{"ack gets ack", "I'm working", "Got it — good luck!", false},
		{"ack with numbers", "I'm working", "Cool, 36 records came back clean.", true},
		{"ack with status words", "I'm working", "Nice — three tasks completed while you were out.", true},
		{"ack too long", "thanks", strings.Repeat("word ", 30), true},
		{"status asked, numbers ok", "how's it going?", "Making progress — 12 things done, nothing on fire.", false},
		{"status asked, status words ok", "what broke last night?", "Two tasks failed overnight but the queue recovered.", false},
		{"jargon always rejected", "how's it going?", "L4 vetoed the task again.", true},
		{"id always rejected", "how's it going?", "Finished T_C_S22 just now.", true},
		{"path always rejected", "what broke?", "Something in /workspace/modules blew up.", true},
		{"question mark counts as asked", "still busy?", "Yep — 3 things in flight.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ownerReplyViolations(c.reply, c.ownerMsg)
			require.Equal(t, c.wantRej, len(got) > 0, "violations: %v", got)
		})
	}
}

func TestOwnerShortAck(t *testing.T) {
	require.True(t, ownerShortAck("I'm working"))
	require.True(t, ownerShortAck("thanks"))
	require.False(t, ownerShortAck("how's it going?"))
	require.False(t, ownerShortAck("tell me everything that happened while I was asleep"))
}
