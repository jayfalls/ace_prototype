package tasks

import "context"

// HeartbeatTick is the L5-owned time loop (spec 9.4 heartbeat.go): each
// tick it evaluates monitors and fires due schedules via the fire
// callback. When there is nothing to do it reports the skip reason through
// the skip callback and returns false.
func (m *Manager) HeartbeatTick(
	ctx context.Context,
	fireSchedule func(s Schedule),
	skip func(reason string),
) bool {
	did := false
	if m.MonitorCount() > 0 {
		m.TickMonitors(ctx)
		did = true
	}
	due, err := m.DueSchedules(ctx, m.now())
	if err == nil && len(due) > 0 && fireSchedule != nil {
		for _, s := range due {
			fireSchedule(s)
		}
		did = true
	}
	if !did && skip != nil {
		skip("no monitors, no due schedules")
	}
	return did
}
