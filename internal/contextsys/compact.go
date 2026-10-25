package contextsys

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CompactTier is a compaction level keyed to context-fill fraction
// (spec 9.5: snip/micro/collapse/auto at 50/65/80/90%).
type CompactTier int

const (
	TierNone CompactTier = iota
	TierSnip
	TierMicro
	TierCollapse
	TierAuto
)

func (t CompactTier) String() string {
	return [...]string{"none", "snip", "micro", "collapse", "auto"}[t]
}

// CompactThresholds are context-fill fractions per tier (defaults per spec).
type CompactThresholds struct {
	Snip     float64 // default 0.50
	Micro    float64 // default 0.65
	Collapse float64 // default 0.80
	Auto     float64 // default 0.90
}

// DefaultThresholds returns the spec fractions.
func DefaultThresholds() CompactThresholds {
	return CompactThresholds{Snip: 0.50, Micro: 0.65, Collapse: 0.80, Auto: 0.90}
}

// Turn is a transcript entry the compactor can see.
type Turn struct {
	Role      string
	Content   string
	At        time.Time
	ToolBytes int // bytes of tool output inside Content (snippable)
}

// Summarizer makes the auto-tier model call. It returns the summary text.
// The hot path never calls it — snip/micro/collapse are deterministic.
type Summarizer func(ctx context.Context, prompt string) (string, error)

// AutoSummaryTemplate is the fixed spec template for the auto tier.
const AutoSummaryTemplate = "Summarize this session for continuation. Use exactly these headers:\nGoal\nConstraints\nProgress\nBlocked\nNext\n\nTranscript:\n%s"

// Compactor tracks tier crossings for one session. Each tier fires once
// per crossing (reset when fill drops back below the tier).
type Compactor struct {
	Thresholds CompactThresholds
	summarize  Summarizer
	lastTier   CompactTier
	autoFails  int
	autoDead   bool // 3 consecutive auto failures -> stop trying
	onEvent    func(InjectionEvent)
}

// NewCompactor returns a session compactor. onEvent receives an
// InjectionEvent for every compaction action (wired to the InjectionLog).
func NewCompactor(sum Summarizer, onEvent func(InjectionEvent)) *Compactor {
	return &Compactor{Thresholds: DefaultThresholds(), summarize: sum, onEvent: onEvent}
}

// TierFor maps a fill fraction (0..1) to the tier that should fire.
func (c *Compactor) TierFor(fill float64) CompactTier {
	t := c.Thresholds
	switch {
	case fill >= t.Auto && !c.autoDead:
		return TierAuto
	case fill >= t.Collapse:
		return TierCollapse
	case fill >= t.Micro:
		return TierMicro
	case fill >= t.Snip:
		return TierSnip
	}
	return TierNone
}

// MaybeCompact evaluates the current fill; when the tier advanced it
// compacts and returns the surviving turns. It returns nil when nothing
// fires, so callers can distinguish "no compaction" from "compacted".
// Each tier fires exactly once per crossing — dropping back below a tier
// re-arms it via lastTier tracking.
func (c *Compactor) MaybeCompact(ctx context.Context, turns []Turn, fill float64, contextBytes int) []Turn {
	tier := c.TierFor(fill)
	if tier <= c.lastTier {
		return nil
	}
	c.lastTier = tier
	if tier == TierNone {
		return nil
	}
	var out []Turn
	var note string
	switch tier {
	case TierSnip:
		out, note = compactSnip(turns)
	case TierMicro:
		out, note = compactMicro(turns)
	case TierCollapse:
		out, note = compactCollapse(turns)
	case TierAuto:
		summary, err := c.autoSummary(ctx, turns)
		if err != nil {
			return nil // auto failed; deterministic tiers remain in effect next pass
		}
		out = []Turn{{Role: "system", Content: summary, At: time.Now().UTC()}}
		note = "session auto-summarized into continuation brief"
	}
	if c.onEvent != nil {
		c.onEvent(InjectionEvent{
			Source: "compact." + tier.String(), Kind: "compaction",
			Bytes: contextBytes, At: time.Now().UTC(),
			Notice: fmt.Sprintf("[context: %s compaction at %.0f%% — %s]", tier.String(), fill*100, note),
		})
	}
	return out
}

// Reset drops the last-tier memory (new session/fresh transcript).
func (c *Compactor) Reset() { c.lastTier = TierNone }

// autoSummary invokes the model summarizer; three consecutive failures
// permanently disable the auto tier for this session.
func (c *Compactor) autoSummary(ctx context.Context, turns []Turn) (string, error) {
	if c.summarize == nil || c.autoDead {
		return "", fmt.Errorf("auto summarizer unavailable")
	}
	var b strings.Builder
	for _, t := range turns {
		fmt.Fprintf(&b, "[%s] %s\n", t.Role, t.Content)
	}
	s, err := c.summarize(ctx, fmt.Sprintf(AutoSummaryTemplate, b.String()))
	if err != nil {
		c.autoFails++
		if c.autoFails >= 3 {
			c.autoDead = true
		}
		return "", err
	}
	c.autoFails = 0
	return s, nil
}

// AutoDisabled reports whether the auto tier stopped after 3 failures.
func (c *Compactor) AutoDisabled() bool { return c.autoDead }

// --- deterministic tiers (no model calls on the hot path) ---

// tail keeps the most recent turns worth at least keepBytes of content.
func tail(turns []Turn, keepBytes int) []Turn {
	if keepBytes <= 0 {
		return nil
	}
	acc := 0
	start := len(turns)
	for i := len(turns) - 1; i >= 0; i-- {
		acc += len(turns[i].Content)
		start = i
		if acc >= keepBytes {
			break
		}
	}
	return append([]Turn(nil), turns[start:]...)
}

// snipToolOutputs truncates tool output inside each turn to head bytes.
func snipToolOutputs(turns []Turn, head int) []Turn {
	out := make([]Turn, 0, len(turns))
	for _, t := range turns {
		c := t
		if t.ToolBytes > 0 && len(t.Content) > head {
			c.Content = t.Content[:head] + fmt.Sprintf("\n[snip: %d bytes of tool output elided]", len(t.Content)-head)
		}
		out = append(out, c)
	}
	return out
}

func compactSnip(turns []Turn) ([]Turn, string) {
	return snipToolOutputs(tail(turns, 16384), 1024), "kept recent tail, tool outputs snipped"
}

func compactMicro(turns []Turn) ([]Turn, string) {
	return snipToolOutputs(tail(turns, 8192), 512), "kept short recent tail, tool outputs capped"
}

func compactCollapse(turns []Turn) ([]Turn, string) {
	return snipToolOutputs(tail(turns, 4096), 256), "collapsed to last turns only"
}
