// Package tools hosts ACE's built-in tools as plugins (spec 9.1): the web,
// fetch, publish, mail and launch capabilities extracted from the old
// prosecutor dispatch switch, plus the FnTool adapter engine-internal
// actions register through.
package tools

import (
	"context"
	"encoding/json"

	"ace/internal/core"
)

// Args is the canonical tool-call payload: the LayerAction fields the old
// dispatch switch consumed.
type Args struct {
	Target  string `json:"target"`
	Content string `json:"content"`
	OldText string `json:"old_text,omitempty"`
}

// Meta carries per-call context the Tool signature cannot: the emitting
// cycle, the model under test and the proactive-update sink.
type Meta struct {
	CycleID string
	Model   string
	Layer   int
	Emit    func(text string)
}

type metaKey struct{}

// WithMeta attaches call metadata for a tool invocation.
func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// MetaFrom reads the metadata (zero Meta when absent).
func MetaFrom(ctx context.Context) Meta {
	if v, ok := ctx.Value(metaKey{}).(Meta); ok {
		return v
	}
	return Meta{}
}

// FnTool adapts a plain function to core.Tool.
type FnTool struct {
	NameV     string
	SchemaV   json.RawMessage
	ReadOnlyV bool
	LayersV   []int // prompt-visible layers (empty = dispatchable but never advertised)
	Fn        func(ctx context.Context, a Args) (string, error)
}

func (t *FnTool) Name() string { return t.NameV }

// Layers reports which layers see this tool in the prompt action list —
// dispatch permission itself is enforced inside each Fn.
func (t *FnTool) Layers() []int { return t.LayersV }
func (t *FnTool) Schema() json.RawMessage {
	if len(t.SchemaV) == 0 {
		return json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"content":{"type":"string"}}}`)
	}
	return t.SchemaV
}
func (t *FnTool) ReadOnly() bool { return t.ReadOnlyV }

func (t *FnTool) Call(ctx context.Context, args json.RawMessage) (core.ToolResult, error) {
	var a Args
	if len(args) > 0 {
		if err := json.Unmarshal(args, &a); err != nil {
			return core.ToolResult{}, err
		}
	}
	out, err := t.Fn(ctx, a)
	if err != nil {
		return core.ToolResult{Content: out, Error: err.Error()}, err
	}
	return core.ToolResult{Content: out}, nil
}

// Helpers the moved web/launch tools reference (mirrors of the cognitive
// package's unexported versions — kept local so tools has no engine dep).
func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
