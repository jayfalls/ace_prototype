package tasks

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule is an L2 commitment (spec 9.4: L2 writes schedules rows).
type Schedule struct {
	ID       string
	Name     string
	RRule    string
	NextDue  time.Time
	LastRun  string
	Enabled  bool
}

// NextDue computes the next fire time for a tiny RRULE subset:
// FREQ=MINUTELY|HOURLY|DAILY|WEEKLY[;INTERVAL=n][;BYDAY=MO,TU,...].
// It is deterministic off `from`, so a weekly schedule fires once per week
// across restarts.
func NextDue(rrule string, from time.Time) (time.Time, error) {
	freq, interval := "", 1
	var byDays []time.Weekday
	for _, part := range strings.Split(rrule, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.ToUpper(kv[0]) {
		case "FREQ":
			freq = strings.ToUpper(kv[1])
		case "INTERVAL":
			if n, err := strconv.Atoi(kv[1]); err == nil && n > 0 {
				interval = n
			}
		case "BYDAY":
			for _, d := range strings.Split(kv[1], ",") {
				if wd, ok := weekdayOf(strings.ToUpper(strings.TrimSpace(d))); ok {
					byDays = append(byDays, wd)
				}
			}
		}
	}
	step := func() (time.Duration, error) {
		switch freq {
		case "MINUTELY":
			return time.Duration(interval) * time.Minute, nil
		case "HOURLY":
			return time.Duration(interval) * time.Hour, nil
		case "DAILY":
			return time.Duration(interval) * 24 * time.Hour, nil
		case "WEEKLY":
			return time.Duration(interval) * 7 * 24 * time.Hour, nil
		}
		return 0, fmt.Errorf("tasks: unsupported RRULE freq %q", freq)
	}
	d, err := step()
	if err != nil {
		return time.Time{}, err
	}
	next := from.Add(d)
	if len(byDays) > 0 && freq == "WEEKLY" {
		// Walk forward to the next listed weekday after `from`.
		for i := 0; i < 7*interval; i++ {
			cand := from.Add(time.Duration(i+1) * 24 * time.Hour)
			for _, wd := range byDays {
				if cand.Weekday() == wd {
					return cand, nil
				}
			}
		}
	}
	return next, nil
}

func weekdayOf(s string) (time.Weekday, bool) {
	days := map[string]time.Weekday{
		"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday,
		"WE": time.Wednesday, "TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday,
	}
	wd, ok := days[s]
	return wd, ok
}

// UpsertSchedule creates or replaces a named schedule. First next_due is
// computed from now.
func (m *Manager) UpsertSchedule(ctx context.Context, name, rrule string) (Schedule, error) {
	next, err := NextDue(rrule, m.now())
	if err != nil {
		return Schedule{}, err
	}
	s := Schedule{ID: "sch_" + strings.ReplaceAll(name, " ", "_"), Name: name, RRule: rrule, NextDue: next, Enabled: true}
	_, err = m.db.ExecContext(ctx,
		`INSERT INTO schedules (id, name, rrule, next_due, enabled) VALUES (?,?,?,?,1)
		 ON CONFLICT(name) DO UPDATE SET rrule=excluded.rrule, enabled=1`,
		s.ID, name, rrule, next.Format(tsFmt))
	return s, err
}

// DueSchedules returns enabled schedules whose next_due <= now.
func (m *Manager) DueSchedules(ctx context.Context, now time.Time) ([]Schedule, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, name, rrule, next_due, last_run_id, enabled FROM schedules WHERE enabled=1 AND next_due<=?`, now.Format(tsFmt))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var s Schedule
		var nd, lr string
		var en int
		if rows.Scan(&s.ID, &s.Name, &s.RRule, &nd, &lr, &en) == nil {
			s.NextDue, _ = time.Parse(tsFmt, nd)
			s.LastRun = lr
			s.Enabled = en == 1
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

// AdvanceSchedule computes the schedule's next_due from its previous
// next_due (not from now) so missed runs still dedupe by (name, due), and
// persists it ONLY when the run finished — spec: next_due advances only
// after a run row reaches done.
func (m *Manager) AdvanceSchedule(ctx context.Context, schedID string, runDone bool) error {
	if !runDone {
		return nil
	}
	var rrule, ndStr string
	if err := m.db.QueryRowContext(ctx,
		`SELECT rrule, next_due FROM schedules WHERE id=?`, schedID).Scan(&rrule, &ndStr); err != nil {
		return err
	}
	prev, err := time.Parse(tsFmt, ndStr)
	if err != nil {
		return err
	}
	next, err := NextDue(rrule, prev)
	if err != nil {
		return err
	}
	// If the computed next is still in the past (long outage), skip forward
	// to the first future slot without emitting the missed ones — dedupe.
	for !next.After(m.now()) {
		next, err = NextDue(rrule, next)
		if err != nil {
			return err
		}
	}
	_, err = m.db.ExecContext(ctx,
		`UPDATE schedules SET next_due=? WHERE id=?`, next.Format(tsFmt), schedID)
	return err
}

// RecordScheduleRun stamps last_run_id on a schedule.
func (m *Manager) RecordScheduleRun(ctx context.Context, schedID, runID string) {
	_, _ = m.db.ExecContext(ctx, `UPDATE schedules SET last_run_id=? WHERE id=?`, runID, schedID)
}

// ReconcileSchedules advances next_due for every schedule whose last run
// row reached DONE — spec: next_due advances only after a run reaches
// done. Failed runs leave next_due untouched (the due stays eligible).
func (m *Manager) ReconcileSchedules(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx,
		`SELECT s.id FROM schedules s JOIN tasks t ON t.id = s.last_run_id
		 WHERE t.state = 'DONE' AND s.next_due <= ?`, m.now().Format(tsFmt))
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := m.AdvanceSchedule(ctx, id, true); err != nil {
			return err
		}
	}
	return nil
}
