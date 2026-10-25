package contextsys

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Overflow writes over-cap input to a file and returns a head plus size,
// path and a read-by-range instruction (spec 9.5: replaces
// SpilloverTruncate and smartFormatHotFile).
type Overflow struct {
	Dir      string // writable dir for spill files (inside the sandbox)
	HeadSize int    // bytes of head retained in the prompt (default 4096)
	rel      func(path string) string
}

// NewOverflow creates a writer rooted at dir (usually the workspace dir).
func NewOverflow(dir string) *Overflow {
	return &Overflow{Dir: dir, HeadSize: 4096}
}

var overflowNameRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Handle returns data unchanged when under cap. Over cap it writes the
// full payload to Dir/spill-<name> and returns head + size + path +
// read-by-range instruction. A failed write degrades to a truncated head
// with the byte count — never fabricated content.
func (o *Overflow) Handle(name, data string, cap int) string {
	if len(data) <= cap {
		return data
	}
	head := o.HeadSize
	if head <= 0 {
		head = 4096
	}
	if head > len(data) {
		head = len(data)
	}
	safe := overflowNameRe.ReplaceAllString(name, "_")
	if safe == "" {
		safe = "payload"
	}
	path := filepath.Join(o.Dir, "spill-"+safe)
	werr := os.WriteFile(path, []byte(data), 0o600)
	if werr == nil {
		return fmt.Sprintf("%s\n\n[OVERFLOW: %d bytes total, head %d shown; full content at %s — read by range, e.g. `sed -n 'START,ENDp' %s`]",
			data[:head], len(data), head, path, path)
	}
	return fmt.Sprintf("%s\n\n[OVERFLOW: %d bytes total, head %d shown; spill write failed: %v]",
		data[:head], len(data), head, werr)
}

// Cap returns the smaller of cap and the raw length — callers use it to
// decide Handle vs passthrough without scanning twice.
func (o *Overflow) Cap(data string, cap int) int {
	if len(data) < cap {
		return len(data)
	}
	return cap
}

// HeadOrMeta is the spec helper for hot-file formatting: under cap the
// whole body; over cap head + meta line (no file write — hot files are
// already on disk; the path is the source path itself).
func HeadOrMeta(path, data string, cap int) string {
	if len(data) <= cap {
		return data
	}
	return fmt.Sprintf("%s\n\n[OVERFLOW: %d bytes total, head %d shown; full file at %s — read by range]",
		data[:cap], len(data), cap, strings.TrimSpace(path))
}
