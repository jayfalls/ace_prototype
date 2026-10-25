package tasks

import (
	"context"
	"time"
)

// RunJob executes work for a task with heartbeat + checkpoint plumbing:
// the task row gains heartbeats, work checkpoints via the cp callback, and
// the run row records the outcome. On restart, Resume returns the task and
// LatestCheckpoint hands back the last blob — the job continues from there
// and idempotency keys keep side effects single-run (spec 9.4 test).
func (m *Manager) RunJob(
	ctx context.Context,
	taskID string,
	work func(ctx context.Context, cp func(seq int, blob []byte)) error,
) error {
	runID, err := m.startRun(ctx, taskID)
	if err != nil {
		return err
	}
	if err := m.setState(ctx, taskID, StateRunning, 0, ""); err != nil {
		return err
	}
	cp := func(seq int, blob []byte) {
		_ = m.Checkpoint(ctx, taskID, seq, blob)
		_ = m.Heartbeat(ctx, taskID, float64(seq)/100.0)
	}
	workErr := work(ctx, cp)
	if workErr != nil {
		_ = m.Fail(ctx, taskID, workErr)
		m.endRun(ctx, runID, "failed", workErr.Error())
		return workErr
	}
	_ = m.Complete(ctx, taskID)
	m.endRun(ctx, runID, "done", "")
	return nil
}

// DeadlineCheck is the spec-9.4 deadline governor: given the task's measured
// progress it projects the finish time; if the projection passes the
// deadline it fires alert (the engine routes it to L4).
func (m *Manager) DeadlineCheck(ctx context.Context, taskID string, alert func(projected time.Time)) {
	t, err := m.Poll(ctx, taskID)
	if err != nil || t.State != StateRunning || t.Deadline.IsZero() {
		return
	}
	elapsed := m.now().Sub(t.CreatedAt)
	var projected time.Time
	if t.Progress > 0.01 {
		total := time.Duration(float64(elapsed) / t.Progress)
		projected = t.CreatedAt.Add(total)
	} else {
		projected = m.now().Add(elapsed) // no signal: double elapsed
	}
	if projected.After(t.Deadline) {
		alert(projected)
	}
}
