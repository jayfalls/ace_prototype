package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func testManager(t *testing.T) (*Manager, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	return New(db), db
}

// Spec 9.4: kill mid-job → on resume the job continues from the last
// checkpoint; side-effect counter equals the planned count, no duplicates.
func TestJobResumeNoDuplicateSideEffects(t *testing.T) {
	m, _ := testManager(t)
	ctx := context.Background()
	task, _ := m.Submit(ctx, Task{Kind: "job", Layer: 6})

	var effects int
	work := func(ctx context.Context, cp func(seq int, blob []byte)) error {
		for i := 1; i <= 3; i++ {
			ran, err := m.SideEffectOnce(ctx, fmt.Sprintf("%s:step%d", task.ID, i), func() error {
				effects++
				return nil
			})
			if err != nil {
				return err
			}
			if ran {
				cp(i, []byte(fmt.Sprintf("step %d done", i)))
			}
			if ran && i == 2 {
				return errors.New("simulated kill")
			}
		}
		return nil
	}
	// First attempt dies after step 2 — a real kill never reaches Fail, so
	// drive work() directly with the task in RUNNING (the crash shape).
	_ = m.setState(ctx, task.ID, StateRunning, 0, "")
	cp := func(seq int, blob []byte) { _ = m.Checkpoint(ctx, task.ID, seq, blob) }
	if err := work(ctx, cp); err == nil {
		t.Fatal("expected simulated kill error")
	}
	if effects != 2 {
		t.Fatalf("expected 2 side effects before kill, got %d", effects)
	}
	// Restart: resume the task, continue from checkpoint 2.
	live, err := m.Resume(ctx)
	if err != nil || len(live) == 0 {
		t.Fatalf("resume found no live tasks: %v", err)
	}
	seq, _, _ := m.LatestCheckpoint(ctx, task.ID)
	if seq != 2 {
		t.Fatalf("expected checkpoint seq 2, got %d", seq)
	}
	if err := m.RunJob(ctx, task.ID, work); err != nil {
		t.Fatalf("resumed run failed: %v", err)
	}
	if effects != 3 {
		t.Fatalf("side-effect counter %d, want planned 3 (no duplicates)", effects)
	}
	done, _ := m.Poll(ctx, task.ID)
	if done.State != StateDone {
		t.Fatalf("task state %s, want DONE", done.State)
	}
}

// Spec 9.4: a weekly schedule fires once per week across a restart and
// next_due does not advance when the run fails.
func TestWeeklyScheduleAcrossRestart(t *testing.T) {
	m, db := testManager(t)
	ctx := context.Background()
	if _, err := m.UpsertSchedule(ctx, "weekly-digest", "FREQ=WEEKLY;INTERVAL=1"); err != nil {
		t.Fatal(err)
	}
	var beforeStr string
	_ = db.QueryRowContext(ctx, `SELECT next_due FROM schedules WHERE name='weekly-digest'`).Scan(&beforeStr)
	before, _ := time.Parse(tsFmt, beforeStr)

	// Failed run must not advance next_due.
	if err := m.AdvanceSchedule(ctx, "sch_weekly-digest", false); err != nil {
		t.Fatal(err)
	}
	var afterStr string
	_ = db.QueryRowContext(ctx, `SELECT next_due FROM schedules WHERE name='weekly-digest'`).Scan(&afterStr)
	after, _ := time.Parse(tsFmt, afterStr)
	if !after.Equal(before) {
		t.Fatal("next_due advanced on a failed run")
	}

	// Done run advances exactly one week.
	if err := m.AdvanceSchedule(ctx, "sch_weekly-digest", true); err != nil {
		t.Fatal(err)
	}
	var advStr string
	_ = db.QueryRowContext(ctx, `SELECT next_due FROM schedules WHERE name='weekly-digest'`).Scan(&advStr)
	advanced, _ := time.Parse(tsFmt, advStr)
	if d := advanced.Sub(before); d != 7*24*time.Hour {
		t.Fatalf("weekly advance = %v, want 168h", d)
	}

	// "Restart": a fresh Manager on the same DB sees the same due times.
	m2 := New(db)
	due, err := m2.DueSchedules(ctx, time.Now().UTC().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range due {
		if s.Name == "weekly-digest" {
			t.Fatal("future weekly schedule must not be due now")
		}
	}
}

// Spec 9.4: a monitor fires after a restart (re-armed on the same task row).
func TestMonitorFiresAfterRestart(t *testing.T) {
	m, db := testManager(t)
	ctx := context.Background()
	cond := func(ctx context.Context) (bool, string) { return true, "fired!" }
	if _, err := m.RegisterMonitor(ctx, "cpu-hot", cond, func(d string) {}); err != nil {
		t.Fatal(err)
	}
	// "Restart" — new Manager, same DB; re-registering re-arms the same task.
	m2 := New(db)
	var fired2 []string
	if _, err := m2.RegisterMonitor(ctx, "cpu-hot", cond, func(d string) { fired2 = append(fired2, d) }); err != nil {
		t.Fatal(err)
	}
	m2.TickMonitors(ctx)
	if len(fired2) != 1 || fired2[0] != "fired!" {
		t.Fatalf("monitor did not fire after restart: %v", fired2)
	}
	// Edge-triggered: a second tick with condition still true does NOT refire.
	m2.TickMonitors(ctx)
	if len(fired2) != 1 {
		t.Fatal("monitor refired on a steady-true condition")
	}
	var n int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE kind='monitor'`).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 durable monitor row, got %d", n)
	}
}

// Spec 9.4: a child calling send_message is refused at the registry.
func TestChildBlocklist(t *testing.T) {
	if ChildToolAllowed("send_message") {
		t.Fatal("send_message must be refused for children")
	}
	if ChildToolAllowed("delegate") || ChildToolAllowed("memory_write") {
		t.Fatal("delegate and memory_write must be refused")
	}
	if !ChildToolAllowed("workspace_read") {
		t.Fatal("workspace_read must be allowed")
	}
	// Depth cap: a child cannot spawn a grandchild.
	m, _ := testManager(t)
	ctx := context.Background()
	parent, _ := m.Submit(ctx, Task{Kind: "job", Layer: 6, Depth: 1})
	if _, err := m.SpawnChild(ctx, parent, "job", "x", "prov"); err != errChildDepth {
		t.Fatalf("grandchild spawn err = %v, want depth cap", err)
	}
	// Parent kill switch cascades.
	root, _ := m.Submit(ctx, Task{Kind: "job", Layer: 6})
	child, err := m.SpawnChild(ctx, root, "job", "x", "prov2")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(ctx, root.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Poll(ctx, child.ID)
	if got.State != StateCanceled {
		t.Fatalf("parent cancel did not kill child (state %s)", got.State)
	}
}

// Spec 9.4: a tool running past 120s (10ms in test) returns a handle, not
// an error; the handle is pollable, readable, cancelable.
func TestExecDetachesToHandle(t *testing.T) {
	m, _ := testManager(t)
	ctx := context.Background()
	exec := func(ctx context.Context, cmd string) (string, error) {
		time.Sleep(80 * time.Millisecond)
		return "slow output", nil
	}
	res, h, err := m.ExecOrHandle(ctx, exec, "sleep-forever", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("ExecOrHandle errored: %v", err)
	}
	if res != "" || h == nil {
		t.Fatal("past-timeout exec must return a handle, not a result")
	}
	time.Sleep(120 * time.Millisecond)
	st, err := m.HandleStatus(ctx, h.TaskID)
	if err != nil || st != StateDone {
		t.Fatalf("handle state = %s err=%v, want DONE", st, err)
	}
	out, err := m.HandleRead(ctx, h.TaskID)
	if err != nil || out != "slow output" {
		t.Fatalf("handle read = %q err=%v", out, err)
	}
}
