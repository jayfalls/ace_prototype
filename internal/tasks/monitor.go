package tasks

import "context"

// MonitorFunc evaluates a monitor's condition; fires when it returns true.
type MonitorFunc func(ctx context.Context) (fired bool, detail string)

// Monitor is a Sense-plugin condition that fires an event (spec 9.4). It is
// persisted as a kind='monitor' task so a restart re-arms it; last_fired_at
// rides in the task payload's heartbeat (edge-triggered: only a false→true
// transition emits).
type Monitor struct {
	Name      string
	TaskID    string
	Condition MonitorFunc
	Event     func(detail string)
}

type monitorState struct {
	taskID string
	fn     MonitorFunc
	ev     func(string)
	last   bool
}

// RegisterMonitor arms a condition under a name; the backing task row makes
// it durable — a Monitor registered again after restart resumes with the
// same TaskID.
func (m *Manager) RegisterMonitor(ctx context.Context, name string, cond MonitorFunc, ev func(string)) (Monitor, error) {
	t, err := m.Submit(ctx, Task{Kind: "monitor", Layer: 5, State: StateRunning,
		IdempotencyKey: "monitor:" + name, Payload: name})
	if err != nil {
		return Monitor{}, err
	}
	if m.monitors == nil {
		m.monitors = map[string]*monitorState{}
	}
	m.monMu.Lock()
	m.monitors[t.IdempotencyKey] = &monitorState{taskID: t.ID, fn: cond, ev: ev}
	m.monMu.Unlock()
	return Monitor{Name: name, TaskID: t.ID, Condition: cond, Event: ev}, nil
}

// TickMonitors evaluates every live monitor once (the heartbeat loop calls
// this per tick; it skips and logs nothing when there are no monitors —
// spec's "skips with reason when nothing to do" lives in heartbeat.go).
func (m *Manager) TickMonitors(ctx context.Context) {
	m.monMu.Lock()
	defer m.monMu.Unlock()
	for _, mon := range m.monitors {
		fired, detail := mon.fn(ctx)
		if fired && !mon.last {
			mon.ev(detail)
			_ = m.Heartbeat(ctx, mon.taskID, 1)
		}
		mon.last = fired
	}
}

// MonitorCount reports how many monitors are armed.
func (m *Manager) MonitorCount() int {
	m.monMu.Lock()
	defer m.monMu.Unlock()
	return len(m.monitors)
}
