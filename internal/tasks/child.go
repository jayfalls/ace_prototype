package tasks

import (
	"context"
	"fmt"
	"time"
)

// ChildMaxDepth is the spec-9.4 nesting cap: children may not spawn.
const ChildMaxDepth = 1

// ChildBlocklist is the tool set a child task may never invoke. It is
// enforced at the registry boundary (ChildToolAllowed), not by prompt —
// spec 9.4.
var ChildBlocklist = map[string]bool{
	"delegate":     true,
	"memory_write": true,
	"send_message": true,
}

// ChildToolAllowed reports whether a child task may call the tool.
func ChildToolAllowed(tool string) bool { return !ChildBlocklist[tool] }

// ChildConcurrency is the per-provider cap on concurrent child tasks.
const ChildConcurrency = 2

// ChildTimeout is the spec default for a child's total lifetime.
const ChildTimeout = 5 * time.Minute

// SpawnChild creates a depth-1 child task. It fails for parents already at
// depth >= ChildMaxDepth and when the provider's concurrency cap is full.
// Cancelling the parent kills the child via Manager.Cancel's cascade.
func (m *Manager) SpawnChild(ctx context.Context, parent Task, kind, payload, provider string) (Task, error) {
	if parent.Depth >= ChildMaxDepth {
		return Task{}, errChildDepth
	}
	sem, ok := m.providerSems[provider]
	if !ok {
		sem = make(chan struct{}, ChildConcurrency)
		m.providerSems[provider] = sem
	}
	select {
	case sem <- struct{}{}:
	default:
		return Task{}, fmt.Errorf("tasks: provider %s child concurrency cap reached", provider)
	}
	// The semaphore slot is released by ReleaseChild.
	return m.Submit(ctx, Task{
		Kind:     kind,
		ParentID: parent.ID,
		Layer:    parent.Layer,
		State:    StateQueued,
		Depth:    parent.Depth + 1,
		Payload:  payload,
		Deadline: m.now().Add(ChildTimeout),
	})
}

// ReleaseChild frees the provider's concurrency slot after a child ends.
func (m *Manager) ReleaseChild(provider string) {
	if sem, ok := m.providerSems[provider]; ok {
		select {
		case <-sem:
		default:
		}
	}
}
