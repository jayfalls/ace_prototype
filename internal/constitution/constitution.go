// Package constitution holds the compiled-in ACE constitution (plan 8.1).
// The text is go:embed'ed — it cannot change at runtime — and its sha256 is
// pinned in this file; startup aborts if the two ever diverge, so a changed
// constitution always ships with a deliberate pin bump in the same commit.
package constitution

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

//go:embed constitution.md
var text string

// PinnedSHA256 is the sha256 of constitution.md, bumped in the same commit as
// any change to the text. constitution_test.go fails the build pipeline on drift.
const PinnedSHA256 = "ae0c281958752587d7a38b5134fe40d8559f352baf7964d50ed5b30c20227f05"

var sha = sha256.Sum256([]byte(text))
var version = hex.EncodeToString(sha[:])[:12]

// Text returns the full constitutional text (immutable at runtime).
func Text() string { return text }

// Version returns the short sha256 recorded on every cycle and verdict.
func Version() string { return version }

// Check is called once at startup: fail closed on any embed/pin divergence.
func Check() error {
	got := hex.EncodeToString(sha[:])
	if got != PinnedSHA256 {
		return fmt.Errorf("constitution sha256 %s does not match pinned %s — bump PinnedSHA256 in the same commit as constitution.md changes", got, PinnedSHA256)
	}
	return nil
}
