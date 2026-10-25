package bus

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ace/internal/core"
)

// Spec 9.1/9.3 topic contract: every envelope rides a layer mailbox slot —
// ace.engine.<agent>.layer.<n>.input | .output. From/To must be adjacent
// hops; routing multi-hop is the scheduler's job, not the envelope's.
var topicRe = regexp.MustCompile(`^ace\.engine\.[a-z0-9_-]+\.layer\.[1-6]\.(input|output)$`)

// Validate enforces the envelope contract (spec 9.3). A violation returns a
// descriptive error; Publish records it to bus_rejects before returning.
func Validate(env core.BusEnvelope) error {
	if strings.TrimSpace(env.AgentID) == "" {
		return fmt.Errorf("bus: empty agent_id")
	}
	if strings.TrimSpace(env.CycleID) == "" {
		return fmt.Errorf("bus: empty cycle_id")
	}
	if strings.TrimSpace(env.Summary) == "" {
		return fmt.Errorf("bus: empty summary")
	}
	if !topicRe.MatchString(env.Topic) {
		return fmt.Errorf("bus: malformed topic %q (want ace.engine.<agent>.layer.<1-6>.(input|output))", env.Topic)
	}
	if env.FromLayer < 1 || env.FromLayer > 6 || env.ToLayer < 1 || env.ToLayer > 6 {
		return fmt.Errorf("bus: layer out of range %d->%d", env.FromLayer, env.ToLayer)
	}
	d := env.FromLayer - env.ToLayer
	if d < 0 {
		d = -d
	}
	if d != 1 {
		return fmt.Errorf("bus: non-adjacent hop %d->%d (|from-to| must be 1)", env.FromLayer, env.ToLayer)
	}
	// A .input topic names the destination layer's mailbox.
	if strings.HasSuffix(env.Topic, ".input") && env.ToLayer != intDigit(env.Topic) {
		return fmt.Errorf("bus: .input topic must name the destination layer")
	}
	return nil
}

// intDigit extracts the layer number from a contract topic.
func intDigit(topic string) int {
	// ace.engine.<agent>.layer.<n>.<slot> — <n> is the second-to-last field.
	parts := strings.Split(topic, ".")
	if len(parts) < 2 {
		return 0
	}
	n := parts[len(parts)-2]
	if len(n) == 1 && n[0] >= '1' && n[0] <= '6' {
		return int(n[0] - '0')
	}
	return 0
}

// InputTopic returns the contract topic for an envelope landing in layer
// toLayer's mailbox.
func InputTopic(agentID string, toLayer int) string {
	return fmt.Sprintf("ace.engine.%s.layer.%d.input", agentID, toLayer)
}

// recordReject logs a rejected publish to bus_rejects (spec live check:
// count by reason is reported).
func (b *MemoryBus) recordReject(ctx context.Context, env core.BusEnvelope, reason string) {
	if b.db == nil {
		return
	}
	_, _ = b.db.ExecContext(ctx,
		`INSERT INTO bus_rejects (topic, from_layer, to_layer, reason, created_at) VALUES (?,?,?,?,?)`,
		env.Topic, env.FromLayer, env.ToLayer, reason, time.Now().UTC())
}
