package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJailIsolationAndOperations(t *testing.T) {
	tmp := t.TempDir()
	jail, err := NewJail(tmp)
	require.NoError(t, err)
	defer jail.Close()

	st := jail.Status()
	require.True(t, st.Healthy, "sandbox verification must be healthy: %s", st.VerificationLog)
	require.True(t, st.OSRootActive)
	require.True(t, st.TraversalBlocked)
	require.True(t, st.BwrapAvailable)
	require.True(t, st.HomeHidden)
	require.True(t, st.NetworkUnshared)

	// 1. Verify path traversal is blocked on ReadFile and WriteFile
	_, err = jail.ReadFile("../../etc/passwd")
	require.Error(t, err)
	err = jail.WriteFile("../escape.txt", "nope")
	require.Error(t, err)

	// 2. Verify legitimate workspace read/write works
	err = jail.WriteFile("notes/test.txt", "hello sandbox")
	require.NoError(t, err)
	got, err := jail.ReadFile("notes/test.txt")
	require.NoError(t, err)
	require.Equal(t, "hello sandbox", got)

	files, err := jail.ListFiles()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 2)

	// 4. Verify Pi EditFile surgical replacement, Hashline editing, DiagnoseFile, and ticket/kb CLIs
	err = jail.EditFile("notes/test.txt", "hello sandbox", "line1\nline2")
	require.NoError(t, err)
	hashed := FormatWithLineHashes("line1\nline2")
	require.Contains(t, hashed, "1#")
	parts := strings.Split(hashed, "\n")
	firstAnchor := strings.TrimSpace(strings.Split(parts[0], "|")[0])
	err = jail.EditFile("notes/test.txt", firstAnchor, "line1_hash_edited")
	require.NoError(t, err)
	gotEdited, err := jail.ReadFile("notes/test.txt")
	require.NoError(t, err)
	require.Equal(t, "line1_hash_edited\nline2", gotEdited)

	require.NoError(t, jail.WriteFile("diag_test.py", "def hello():\n    return 42\n"))
	diag := jail.DiagnoseFile(context.Background(), "diag_test.py")
	require.Contains(t, diag, "[Pi-Lens Python AST OK]")
	require.Contains(t, diag, "def hello")

	piRes, err := jail.Exec(context.Background(), `ticket add "Test Ticket" --goal "Verify Pi CLI" && kb note "Test KB" --body "Verified in bwrap"`)
	require.NoError(t, err)
	require.Contains(t, piRes.Stdout, "Test Ticket")
	require.Contains(t, piRes.Stdout, "Test KB")

	// 5. Verify bash -o pipefail catches failing left-hand pipeline commands (Fix S1/B1)
	pipeRes, err := jail.Exec(context.Background(), `false | head -n 5`)
	require.Error(t, err)
	require.NotEqual(t, 0, pipeRes.ExitCode, "pipefail must propagate non-zero exit code from left side of pipe")

	// 6. Verify pre-seeded workspace_audit.py AST-PDG module imports and runs inside bwrap
	sdkRes, err := jail.Exec(context.Background(), `python3 -c 'import sys; sys.path.insert(0, "/workspace"); import workspace_audit; print(workspace_audit.audit_conceptual_flows("/workspace/workspace_audit.py")["clean"])'`)
	require.NoError(t, err, "workspace_audit import stderr: %s", sdkRes.Stderr)
	require.Equal(t, 0, sdkRes.ExitCode)
	require.Contains(t, sdkRes.Stdout, `True`)
}

// A hung selftest must die at tools.selftest_timeout — the whole process
// group, not just bwrap. Run `sleep 300` with a 3s bound and verify the
// deadline kills it fast instead of running the full command.
func TestExecWithTimeoutKillsHungProcessGroup(t *testing.T) {
	tmp := t.TempDir()
	jail, err := NewJail(tmp)
	require.NoError(t, err)
	defer jail.Close()

	start := time.Now()
	// Spawn children so the group-kill actually has something to reap.
	res, err := jail.ExecWithTimeout(context.Background(), `(sleep 300 & sleep 300 & sleep 300) ; wait`, 3*time.Second)
	elapsed := time.Since(start)

	require.Error(t, err, "hung command must surface a deadline error")
	require.Less(t, elapsed, 15*time.Second, "deadline must kill fast, took %s", elapsed)
	require.NotEqual(t, 0, res.ExitCode)
}
