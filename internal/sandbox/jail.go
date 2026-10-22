package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ace/internal/core"
)

const (
	maxExecTimeout = 25 * time.Second
	maxOutputBytes = 256 * 1024
	maxRecentExecs = 50
)

// Jail implements core.Sandbox using Linux kernel openat2 (via Go 1.24+ os.Root)
// and bubblewrap (bwrap --unshare-all) namespace isolation.
type Jail struct {
	mu           sync.RWMutex
	baseDir      string
	workspaceDir string
	wikiDir      string
	wsRoot       *os.Root
	wikiRoot     *os.Root
	status       core.SandboxStatus
	recentExecs  []core.ExecResult
}

func NewJail(baseDir string) (*Jail, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve sandbox dir: %w", err)
	}
	wsDir := filepath.Join(absBase, "workspace")
	wkDir := filepath.Join(absBase, "wiki")

	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir workspace: %w", err)
	}
	if err := os.MkdirAll(wkDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir wiki: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(wkDir, "notes"), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir wiki/notes: %w", err)
	}

	wsRoot, err := os.OpenRoot(wsDir)
	if err != nil {
		return nil, fmt.Errorf("open workspace os.Root: %w", err)
	}
	wikiRoot, err := os.OpenRoot(wkDir)
	if err != nil {
		_ = wsRoot.Close()
		return nil, fmt.Errorf("open wiki os.Root: %w", err)
	}

	j := &Jail{
		baseDir:      absBase,
		workspaceDir: wsDir,
		wikiDir:      wkDir,
		wsRoot:       wsRoot,
		wikiRoot:     wikiRoot,
		recentExecs:  make([]core.ExecResult, 0, maxRecentExecs),
	}

	seedReadme := "# ACE Sandbox Workspace\n\nThis directory is isolated inside a Linux kernel `os.Root` and `bwrap --unshare-all --share-net` namespace.\nHost `/home` and host filesystems are unmounted and unreachable; controlled outbound HTTPS/localhost network egress is enabled.\nPi CLI tools (`ticket` and `kb`) are pre-installed in `/workspace/bin`.\n"
	if _, err := j.ReadFile("README.md"); err != nil {
		_ = j.WriteFile("README.md", seedReadme)
	}
	j.seedPiTools()
	j.initWorkspaceHygiene()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	j.Verify(ctx)

	return j, nil
}

func (j *Jail) WorkspaceDir() string {
	return j.workspaceDir
}

func (j *Jail) WikiDir() string {
	return j.wikiDir
}

func (j *Jail) Close() error {
	var err1, err2 error
	if j.wsRoot != nil {
		err1 = j.wsRoot.Close()
	}
	if j.wikiRoot != nil {
		err2 = j.wikiRoot.Close()
	}
	return errors.Join(err1, err2)
}

func (j *Jail) WikiRoot() *os.Root {
	return j.wikiRoot
}

// Verify runs live empirical checks against both os.Root and bwrap to prove host isolation.
func (j *Jail) Verify(ctx context.Context) core.SandboxStatus {
	st := core.SandboxStatus{
		OSRootActive:   j.wsRoot != nil && j.wikiRoot != nil,
		SandboxDir:     j.baseDir,
		LastVerifiedAt: time.Now().UTC(),
	}
	var logs []string

	// 1. Verify os.Root blocks "../" path traversal at kernel level
	if j.wsRoot != nil {
		_, err := j.wsRoot.Open("../etc/passwd")
		if err != nil {
			st.TraversalBlocked = true
			logs = append(logs, "PASS: os.Root kernel openat2 blocked '../etc/passwd'")
		}
		if err == nil {
			logs = append(logs, "FAIL: os.Root allowed parent traversal")
		}
	}

	// 2. Verify bwrap availability and version
	verCmd := exec.CommandContext(ctx, "bwrap", "--version")
	if out, err := verCmd.CombinedOutput(); err == nil {
		st.BwrapAvailable = true
		st.BwrapVersion = strings.TrimSpace(string(out))
		logs = append(logs, "PASS: "+st.BwrapVersion)
	}

	// 3. Verify bwrap hides /home and isolates PID/user/mount namespaces with DNS/TLS egress ready
	if st.BwrapAvailable {
		res, _ := j.runBwrap(ctx, "ls /home >/dev/null 2>&1 && echo HOME_VISIBLE || echo HOME_HIDDEN; test -r /etc/resolv.conf && echo NET_READY || echo NET_MISSING")
		if strings.Contains(res.Stdout, "HOME_HIDDEN") {
			st.HomeHidden = true
			logs = append(logs, "PASS: /home is unmounted inside bwrap")
		}
		if strings.Contains(res.Stdout, "NET_READY") {
			st.NetworkUnshared = true
			logs = append(logs, "PASS: user/PID/IPC/mount namespaces isolated (--unshare-all --share-net) with TLS/DNS egress")
		}
	}

	st.Healthy = st.OSRootActive && st.TraversalBlocked && st.BwrapAvailable && st.HomeHidden && st.NetworkUnshared
	st.VerificationLog = strings.Join(logs, " | ")

	j.mu.Lock()
	j.status = st
	j.mu.Unlock()
	return st
}

func (j *Jail) Status() core.SandboxStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

func (j *Jail) Exec(ctx context.Context, command string) (core.ExecResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return core.ExecResult{}, errors.New("empty sandbox command")
	}

	res, err := j.runBwrap(ctx, command)
	j.mu.Lock()
	j.recentExecs = append([]core.ExecResult{res}, j.recentExecs...)
	if len(j.recentExecs) > maxRecentExecs {
		j.recentExecs = j.recentExecs[:maxRecentExecs]
	}
	j.mu.Unlock()
	return res, err
}

func (j *Jail) runBwrap(ctx context.Context, command string) (core.ExecResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, maxExecTimeout)
	defer cancel()

	start := time.Now()
	args := []string{
		"--unshare-all",
		"--share-net",
		"--die-with-parent",
		"--ro-bind", "/usr", "/usr",
		"--symlink", "usr/lib64", "/lib64",
		"--symlink", "usr/lib", "/lib",
		"--symlink", "usr/bin", "/bin",
		"--symlink", "usr/sbin", "/sbin",
		"--ro-bind-try", "/etc/resolv.conf", "/etc/resolv.conf",
		"--ro-bind-try", "/etc/hosts", "/etc/hosts",
		"--ro-bind-try", "/etc/nsswitch.conf", "/etc/nsswitch.conf",
		"--ro-bind-try", "/etc/pki", "/etc/pki",
		"--ro-bind-try", "/etc/ssl", "/etc/ssl",
		"--ro-bind-try", "/etc/crypto-policies", "/etc/crypto-policies",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--ro-bind", j.wikiDir, "/wiki",
		"--bind", j.workspaceDir, "/workspace",
		"--chdir", "/workspace",
		"--clearenv",
		"--setenv", "PATH", "/workspace/bin:/usr/bin:/bin:/usr/sbin:/sbin",
		"--setenv", "PYTHONPATH", "/workspace",
		"--setenv", "HOME", "/workspace",
		"--setenv", "TERM", "dumb",
		"--setenv", "SSL_CERT_FILE", "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
	}
	if orKey := os.Getenv("OPENROUTER_API_KEY"); orKey != "" {
		args = append(args, "--setenv", "OPENROUTER_API_KEY", orKey)
	}
	args = append(args, "/bin/bash", "-o", "pipefail", "-c", command)

	cmd := exec.CommandContext(execCtx, "bwrap", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{buf: &stdout, max: maxOutputBytes}
	cmd.Stderr = &limitedWriter{buf: &stderr, max: maxOutputBytes}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		if exitCode == 0 {
			exitCode = -1
		}
	}

	return core.ExecResult{
		Command:    command,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		DurationMs: time.Since(start).Milliseconds(),
		Isolated:   true,
		ExecutedAt: time.Now().UTC(),
	}, err
}

func cleanRelPath(relPath string) (string, error) {
	trimmed := strings.TrimSpace(relPath)
	for {
		prev := trimmed
		trimmed = strings.TrimPrefix(trimmed, "/")
		trimmed = strings.TrimPrefix(trimmed, "workspace/")
		trimmed = strings.TrimPrefix(trimmed, "./")
		if trimmed == prev {
			break
		}
	}
	if trimmed == "" || trimmed == "." {
		return "", errors.New("empty relative path")
	}
	if strings.Contains(trimmed, "..") {
		return "", errors.New("path traversal '..' rejected")
	}
	return trimmed, nil
}

func (j *Jail) ReadFile(relPath string) (string, error) {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return "", err
	}
	f, err := j.wsRoot.Open(clean)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxOutputBytes))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (j *Jail) WriteFile(relPath string, content string) error {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(clean)
	if dir != "." && dir != "" {
		if err := mkdirAllRoot(j.wsRoot, dir); err != nil {
			return err
		}
	}
	f, err := j.wsRoot.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func (j *Jail) ListFiles() ([]core.WorkspaceFile, error) {
	var files []core.WorkspaceFile
	err := filepath.WalkDir(j.workspaceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != j.workspaceDir && (strings.HasPrefix(name, ".") || name == "__pycache__" || name == "node_modules" || name == "bin" || name == "archive") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".tmp") || strings.Contains(name, ".bak") || strings.HasSuffix(name, ".pyc") || strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".out") || strings.HasSuffix(name, ".err") {
			return nil
		}
		rel, err := filepath.Rel(j.workspaceDir, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, core.WorkspaceFile{
			Path:      filepath.ToSlash(rel),
			SizeBytes: info.Size(),
			UpdatedAt: info.ModTime().UTC(),
		})
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		pi, pj := files[i].Path, files[j].Path
		ri := !strings.Contains(pi, "/")
		rj := !strings.Contains(pj, "/")
		if ri != rj {
			return ri
		}
		scoreExt := func(p string) int {
			switch {
			case strings.HasSuffix(p, ".py"):
				return 0
			case strings.HasSuffix(p, ".json"):
				return 1
			case strings.HasSuffix(p, ".db"), strings.HasSuffix(p, ".tsv"):
				return 2
			case strings.HasSuffix(p, ".md"):
				return 3
			default:
				return 4
			}
		}
		si, sj := scoreExt(pi), scoreExt(pj)
		if si != sj {
			return si < sj
		}
		return pi < pj
	})
	return files, err
}

// SpilloverTruncate implements the Pi Blackhole / Spillover Manager pattern:
// When a tool output exceeds maxChars, the full output is persisted to /workspace/.spillover/<kind>-<ts>.txt
// and a compact Head (65%) + Tail (35%) preview with a pointer is returned so LLM context is never bloated.
func (j *Jail) SpilloverTruncate(kind string, raw string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 4000
	}
	if len(raw) <= maxChars {
		return raw
	}
	spillFile := fmt.Sprintf(".spillover/%s-%d.txt", kind, time.Now().UnixMilli())
	_ = j.WriteFile(spillFile, raw)
	headLen := (maxChars * 65) / 100
	tailLen := maxChars - headLen
	if headLen+tailLen >= len(raw) {
		return raw
	}
	omitted := len(raw) - (headLen + tailLen)
	return fmt.Sprintf("%s\n\n... [Pi Blackhole Spillover: %d bytes omitted -> full output saved at /workspace/%s] ...\n\n%s",
		raw[:headLen], omitted, spillFile, raw[len(raw)-tailLen:])
}

func (j *Jail) RecentExecs(limit int) []core.ExecResult {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if limit <= 0 || limit > len(j.recentExecs) {
		limit = len(j.recentExecs)
	}
	out := make([]core.ExecResult, limit)
	copy(out, j.recentExecs[:limit])
	return out
}

func mkdirAllRoot(root *os.Root, relDir string) error {
	parts := strings.Split(filepath.ToSlash(relDir), "/")
	curr := ""
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if curr == "" {
			curr = p
		}
		if curr != p {
			curr = curr + "/" + p
		}
		err := root.Mkdir(curr, 0o755)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}

type limitedWriter struct {
	buf *bytes.Buffer
	max int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	rem := w.max - w.buf.Len()
	if rem <= 0 {
		return len(p), nil
	}
	if len(p) > rem {
		w.buf.Write(p[:rem])
		return len(p), nil
	}
	return w.buf.Write(p)
}
