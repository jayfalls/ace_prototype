package contextsys

import (
	"sort"
	"strings"
)

// tsFmt is the package timestamp format (RFC3339Nano, matches internal/tasks).
const tsFmt = "2006-01-02T15:04:05.999999999Z07:00"

// StageID is a context-assembly stage (spec 9.5 stages.go).
type StageID int

const (
	// StageTurn0 is deterministic injection: map ROI, memory, focused
	// spec, task, files, self-stats — all before the model sees anything.
	StageTurn0 StageID = iota
	// StageTurn05 is the router stage (context.turn05, off by default).
	StageTurn05
	// StageTurn1 is the worker's own tools — tool outputs appended as
	// turns, not part of turn-0 injection.
	StageTurn1
)

// Bucket is one byte-budgeted region inside a stage. Fill appends body
// under Cap; the returned tail is what did not fit (caller decides:
// drop, or route through Overflow).
type Bucket struct {
	Name string
	Cap  int
	body strings.Builder
}

// Fill appends content to the bucket, honoring Cap.
func (b *Bucket) Fill(content string) (dropped string) {
	room := b.Cap - b.body.Len()
	if room <= 0 {
		return content
	}
	if len(content) <= room {
		b.body.WriteString(content)
		return ""
	}
	b.body.WriteString(content[:room])
	return content[room:]
}

// Body returns the assembled bucket content.
func (b *Bucket) Body() string { return b.body.String() }

// BucketSpec declares one bucket (name, byte cap).
type BucketSpec struct {
	Name string
	Cap  int
}

// Stage is a composable assembly stage: ordered byte-budgeted buckets.
type Stage struct {
	ID      StageID
	Name    string
	Buckets []*Bucket
}

// NewStage builds a stage from ordered bucket specs.
func NewStage(id StageID, name string, specs ...BucketSpec) *Stage {
	s := &Stage{ID: id, Name: name}
	for _, spec := range specs {
		s.Buckets = append(s.Buckets, &Bucket{Name: spec.Name, Cap: spec.Cap})
	}
	return s
}

// Bucket returns the named bucket (nil when absent).
func (s *Stage) Bucket(name string) *Bucket {
	for _, b := range s.Buckets {
		if b.Name == name {
			return b
		}
	}
	return nil
}

// Render concatenates bucket bodies in declared order.
func (s *Stage) Render() string {
	var b strings.Builder
	for _, bk := range s.Buckets {
		b.WriteString(bk.Body())
	}
	return b.String()
}

// DefaultSliceBudget is the spec's turn-0 slice budget (8 KB).
const DefaultSliceBudget = 8192

// DefaultSpecBudget is the spec's focused-spec budget (2,800 B).
const DefaultSpecBudget = 2800

// PromptTokenTarget is the spec's L1–L5 prompt target (4k tokens; the
// live check is provider-reported, this is only the budget constant).
const PromptTokenTarget = 4000

// L6MaxTokens is the spec's L6 prompt ceiling (256k).
const L6MaxTokens = 256 * 1024

// StableOrder sorts bucket names so the prompt prefix is stable-first
// (spec: cache-friendly, byte-identical for identical inputs). Callers
// sort their fill inputs once; repeated builds then match byte-for-byte.
func StableOrder(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}
