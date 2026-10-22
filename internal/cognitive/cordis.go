package cognitive

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// FileSnapshot captures the pre-mutation state of a workspace file for LIFO rollback.
type FileSnapshot struct {
	RelPath string
	Existed bool
	Content []byte
}

// CordisFiber implements the Cordis Spatiotemporal Runtime (P8):
// - Revertible Effects: LIFO file snapshot stack before any workspace mutation, with automatic RevertAll() on broken AST or failing --selftest.
// - Reactive Coeffects: SHA-256 content-digest map of workspace files so unchanged files are never re-read or re-verified.
type CordisFiber struct {
	workspaceDir string
	undoStack    []FileSnapshot
	digests      map[string]string
	mutatedPaths map[string]bool
}

func NewCordisFiber(workspaceDir string) *CordisFiber {
	f := &CordisFiber{
		workspaceDir: workspaceDir,
		undoStack:    make([]FileSnapshot, 0, 8),
		digests:      make(map[string]string),
		mutatedPaths: make(map[string]bool),
	}
	f.RefreshDigests()
	return f
}

func normalizeRelPath(relPath string) string {
	p := strings.TrimSpace(relPath)
	p = strings.TrimPrefix(p, "/workspace/")
	p = strings.TrimPrefix(p, "workspace/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return filepath.ToSlash(p)
}

// SnapshotFile records the pre-image of a file before mutation (once per unique path in a fiber).
func (f *CordisFiber) SnapshotFile(relPath string) {
	clean := normalizeRelPath(relPath)
	if clean == "" || strings.Contains(clean, "..") {
		return
	}
	for _, existing := range f.undoStack {
		if existing.RelPath == clean {
			f.mutatedPaths[clean] = true
			return
		}
	}
	abs := filepath.Join(f.workspaceDir, clean)
	data, err := os.ReadFile(abs)
	snap := FileSnapshot{
		RelPath: clean,
		Existed: err == nil,
		Content: data,
	}
	f.undoStack = append(f.undoStack, snap)
	f.mutatedPaths[clean] = true
}

// RevertAll rolls back all snapshotted mutations in strict LIFO order.
func (f *CordisFiber) RevertAll() []string {
	var reverted []string
	for i := len(f.undoStack) - 1; i >= 0; i-- {
		snap := f.undoStack[i]
		abs := filepath.Join(f.workspaceDir, snap.RelPath)
		if snap.Existed {
			_ = os.MkdirAll(filepath.Dir(abs), 0o755)
			_ = os.WriteFile(abs, snap.Content, 0o644)
		} else {
			_ = os.Remove(abs)
		}
		reverted = append(reverted, snap.RelPath)
	}
	f.undoStack = f.undoStack[:0]
	f.mutatedPaths = make(map[string]bool)
	f.RefreshDigests()
	return reverted
}

// Commit clears the LIFO rollback stack and updates coeffect SHA-256 digests.
func (f *CordisFiber) Commit() {
	f.undoStack = f.undoStack[:0]
	f.RefreshDigests()
}

// HasEmpiricalDiff returns true if any snapshotted file's current SHA-256 differs from its pre-snapshot state.
func (f *CordisFiber) HasEmpiricalDiff() bool {
	for _, snap := range f.undoStack {
		abs := filepath.Join(f.workspaceDir, snap.RelPath)
		curr, err := os.ReadFile(abs)
		if !snap.Existed && err == nil {
			return true
		}
		if snap.Existed && err != nil {
			return true
		}
		if snap.Existed && err == nil && sha256Hex(curr) != sha256Hex(snap.Content) {
			return true
		}
	}
	return false
}

// RefreshDigests scans active workspace files and updates the coeffect SHA-256 digest table.
func (f *CordisFiber) RefreshDigests() map[string]string {
	entries, err := os.ReadDir(f.workspaceDir)
	if err != nil {
		return f.digests
	}
	next := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		abs := filepath.Join(f.workspaceDir, e.Name())
		if data, err := os.ReadFile(abs); err == nil {
			next[e.Name()] = sha256Hex(data)[:16]
		}
	}
	f.digests = next
	return f.digests
}

func (f *CordisFiber) Digests() map[string]string {
	out := make(map[string]string, len(f.digests))
	for k, v := range f.digests {
		out[k] = v
	}
	return out
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
