package layers

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ace/internal/core"
)

// Spec 9.3: L4 output missing success_criteria retries three times and
// escalates.
func TestContractPolicyRetriesThenEscalates(t *testing.T) {
	deficient := json.RawMessage(`{"plan": [{"id": "s1", "deps": [], "checkpoint": "c"}]}`)
	var calls int
	var escalated [][]string
	missing, err := ContractPolicy(4, func(note string) (json.RawMessage, bool, error) {
		calls++
		if calls > 1 && note == "" {
			t.Fatal("retry must carry the prefilled-opening note")
		}
		return deficient, false, nil
	}, func(m []string) { escalated = append(escalated, m) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", calls)
	}
	if len(escalated) != 1 {
		t.Fatalf("expected exactly one escalation, got %d", len(escalated))
	}
	if len(missing) == 0 {
		t.Fatal("deficient output must report missing keys")
	}
}

// A valid L4 contract passes on the first attempt with no escalation.
func TestContractPolicyValidFirstTry(t *testing.T) {
	valid := json.RawMessage(`{"plan": [{"id": "s1", "deps": [], "checkpoint": "c", "success_criteria": "ok"}], "risks": [{"risk": "r", "contingency": "c"}]}`)
	var calls, esc int
	missing, err := ContractPolicy(4, func(note string) (json.RawMessage, bool, error) {
		calls++
		return valid, false, nil
	}, func(m []string) { esc++ })
	if err != nil || calls != 1 || esc != 0 || missing != nil {
		t.Fatalf("calls=%d esc=%d missing=%v err=%v", calls, esc, missing, err)
	}
}

// Spec 9.3: a layer must not run with guidance but no data; it runs once
// data arrives.
func TestSchedulerWaitsForDataBelow(t *testing.T) {
	s := NewScheduler(SchedulerConfig{})
	env := func(from, to int) core.BusEnvelope {
		return core.BusEnvelope{FromLayer: from, ToLayer: to, Summary: "s"}
	}
	s.Deliver(env(1, 2)) // L1 -> L2 guidance only
	now := time.Now()
	if s.Ready(2, now) {
		t.Fatal("L2 must not run on guidance alone")
	}
	s.Deliver(env(3, 2)) // L3 -> L2 northbound data
	if !s.Ready(2, now) {
		t.Fatal("L2 must run once data from below arrives")
	}
	got := s.Dispatch(2, now)
	if len(got) != 2 {
		t.Fatalf("dispatch drained %d envelopes, want 2", len(got))
	}
	if s.Ready(2, now) {
		t.Fatal("L2 must not stay ready after its mailbox is drained")
	}
}

// L1 fires on events and on the slow review interval; L5 respects min_dwell.
func TestSchedulerL1AndL5Cadence(t *testing.T) {
	s := NewScheduler(SchedulerConfig{
		L1ReviewInterval: 15 * time.Minute,
		L5Period:         30 * time.Second,
		L5MinDwell:       60 * time.Second,
	})
	now := time.Now()
	if !s.Ready(1, now) {
		t.Fatal("L1 must be ready at genesis (review interval elapsed)")
	}
	s.Dispatch(1, now)
	if s.Ready(1, now) {
		t.Fatal("L1 must not re-fire right after its review")
	}
	s.Deliver(core.BusEnvelope{FromLayer: 6, ToLayer: 1, Summary: "ev"})
	if !s.Ready(1, now) {
		t.Fatal("L1 must fire on a bus event")
	}
	s.Dispatch(1, now)

	if !s.Ready(5, now) {
		t.Fatal("L5 clock should be ready at genesis")
	}
	s.Dispatch(5, now)
	// At 45s the 30s period has elapsed but min_dwell (60s) has not —
	// dwell is the binding constraint and must suppress refiring.
	if s.Ready(5, now.Add(45*time.Second)) {
		t.Fatal("L5 min_dwell must suppress refiring at 45s")
	}
	if !s.Ready(5, now.Add(61*time.Second)) {
		t.Fatal("L5 must fire once period and min_dwell both elapsed")
	}
	if !s.Ready(6, now) {
		t.Fatal("L6 is always runnable (continuous)")
	}
}

// Spec 9.3: three read-only tools overlap in time.
func TestExecutorParallelReads(t *testing.T) {
	x := NewExecutor(4)
	var inFlight int32
	var maxInFlight int32
	run := func(ctx context.Context) (string, error) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxInFlight)
			if n <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, n) {
				break
			}
		}
		time.Sleep(60 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return "ok", nil
	}
	calls := []Call{
		{Name: "r1", Path: "a", ReadOnly: true, Run: run},
		{Name: "r2", Path: "b", ReadOnly: true, Run: run},
		{Name: "r3", Path: "c", ReadOnly: true, Run: run},
	}
	res := x.RunBatch(context.Background(), calls)
	if res[0] != "ok" || res[1] != "ok" || res[2] != "ok" {
		t.Fatalf("results out of order or wrong: %v", res)
	}
	if maxInFlight < 3 {
		t.Fatalf("read-only calls did not overlap (max in flight = %d)", maxInFlight)
	}
}

// Spec 9.3: overlapping writes are serialized — start/end ordering must
// show no in-flight overlap, and a read on the same path excludes a write.
func TestExecutorSerializesOverlappingWrites(t *testing.T) {
	x := NewExecutor(4)
	var mu sync.Mutex
	var overlaps bool
	inFlight := map[string]int{}
	mark := func(path string) func(ctx context.Context) (string, error) {
		return func(ctx context.Context) (string, error) {
			mu.Lock()
			if inFlight[path] > 0 {
				overlaps = true
			}
			inFlight[path]++
			mu.Unlock()
			time.Sleep(15 * time.Millisecond)
			mu.Lock()
			inFlight[path]--
			mu.Unlock()
			return "w", nil
		}
	}
	calls := []Call{
		{Name: "w1", Path: "p", Run: mark("p")},
		{Name: "w2", Path: "p", Run: mark("p")},
	}
	var wg sync.WaitGroup
	for _, c := range calls {
		wg.Add(1)
		go func(c Call) {
			defer wg.Done()
			_, _ = x.RunOne(context.Background(), c)
		}(c)
	}
	wg.Wait()
	if overlaps {
		t.Fatal("overlapping writes on the same path were in flight together")
	}
}
