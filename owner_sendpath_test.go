package ace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSingleOwnerSendPath enforces the Human-voice invariant (plan b59dabaf):
// every owner-bound message is written by the chat model through
// Engine.NotifyOwner — nothing else may invoke a channel send. A stray
// SendMessage / sendTelegramTextEscalation call site is a bug.
func TestSingleOwnerSendPath(t *testing.T) {
	// pattern -> files permitted to contain a *call* (defs are filtered below)
	allowed := map[string][]string{
		"sendTelegramTextEscalation(": {
			"internal/cognitive/owner_voice.go",
			"internal/cognitive/sensorium.go", // definition site
		},
		".SendMessage(": {
			"internal/channels/telegram.go", // channel transport impl
		},
	}
	var violations []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, "node_modules") || strings.HasPrefix(path, ".git") {
			return nil
		}
		blob, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, "./"))
		for pat, allowlist := range allowed {
			if !strings.Contains(string(blob), pat) {
				continue
			}
			permitted := false
			for _, a := range allowlist {
				if rel == a {
					permitted = true
					break
				}
			}
			if permitted {
				continue
			}
			for i, line := range strings.Split(string(blob), "\n") {
				if strings.Contains(line, pat) && !strings.Contains(line, "func (") {
					violations = append(violations, rel+":"+itoa(i+1)+" contains "+pat)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("owner-bound sends must go through Engine.NotifyOwner:\n%s", strings.Join(violations, "\n"))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
