package layers

import (
	"sync"
	"time"

	"ace/internal/core"
)

// SchedulerConfig carries the spec 9.3 cadence knobs.
type SchedulerConfig struct {
	// L1ReviewInterval is layers.l1.review_interval (default 15m): L1 fires
	// on bus events plus this slow supervisory review.
	L1ReviewInterval time.Duration
	// L5Period is L5's own clock — cognitive control evaluates on this
	// cadence regardless of mailbox state.
	L5Period time.Duration
	// L5MinDwell is layers.l5.min_dwell (default 60s): minimum time between
	// L5 dispatches; suppresses thrash (rapid task switching).
	L5MinDwell time.Duration
}

func (c SchedulerConfig) withDefaults() SchedulerConfig {
	if c.L1ReviewInterval <= 0 {
		c.L1ReviewInterval = 15 * time.Minute
	}
	if c.L5Period <= 0 {
		c.L5Period = 30 * time.Second
	}
	if c.L5MinDwell <= 0 {
		c.L5MinDwell = 60 * time.Second
	}
	return c
}

// Scheduler is the spec-9.3 mailbox/cadence dispatcher that replaces the
// 2-second polling ticker. Every layer owns a mailbox; a layer is runnable
// only under its cadence rule:
//
//	L1: bus events plus the slow review interval.
//	L2–L4: guidance from above AND data from below (a northbound envelope or
//	       a mission-DAG node whose parents are done, via DataBelow).
//	L5: its own clock, gated by min_dwell against thrash.
//	L6: continuous — always runnable; the engine gates it on idle.
type Scheduler struct {
	mu           sync.Mutex
	cfg          SchedulerConfig
	mail         [7][]core.BusEnvelope
	lastDispatch [7]time.Time
	l1LastReview time.Time
	l5LastTick   time.Time
	wake         chan struct{}
	// DataBelow reports a non-bus "data from below" signal for L2–L4 — in
	// the engine, a mission-DAG node whose parents are done. Nil = none.
	DataBelow func(layer int) bool
}

func NewScheduler(cfg SchedulerConfig) *Scheduler {
	return &Scheduler{
		cfg:  cfg.withDefaults(),
		wake: make(chan struct{}, 1),
	}
}

// Wake returns a channel that receives a tick whenever new mail arrives.
func (s *Scheduler) Wake() <-chan struct{} { return s.wake }

// SetConfig hot-swaps the cadence knobs (config.d reload, spec 9.3).
func (s *Scheduler) SetConfig(cfg SchedulerConfig) {
	s.mu.Lock()
	s.cfg = cfg.withDefaults()
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Config returns the active cadence knobs.
func (s *Scheduler) Config() SchedulerConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Deliver routes an envelope into the destination layer's mailbox.
func (s *Scheduler) Deliver(env core.BusEnvelope) {
	if env.ToLayer < 1 || env.ToLayer > 6 {
		return
	}
	s.mu.Lock()
	s.mail[env.ToLayer] = append(s.mail[env.ToLayer], env)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// guidance counts envelopes in layer n's mailbox arriving from above.
func (s *Scheduler) guidance(n int) int {
	c := 0
	for _, e := range s.mail[n] {
		if e.FromLayer < n {
			c++
		}
	}
	return c
}

// data counts envelopes arriving from below.
func (s *Scheduler) data(n int) int {
	c := 0
	for _, e := range s.mail[n] {
		if e.FromLayer > n {
			c++
		}
	}
	return c
}

// Ready reports whether layer n may run under its cadence rule.
func (s *Scheduler) Ready(n int, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch n {
	case 1:
		if len(s.mail[1]) > 0 {
			return true
		}
		return now.Sub(s.l1LastReview) >= s.cfg.L1ReviewInterval
	case 2, 3, 4:
		if s.guidance(n) == 0 {
			return false
		}
		if s.data(n) > 0 {
			return true
		}
		return s.DataBelow != nil && s.DataBelow(n)
	case 5:
		if now.Sub(s.l5LastTick) < s.cfg.L5Period {
			return false
		}
		return now.Sub(s.lastDispatch[5]) >= s.cfg.L5MinDwell
	case 6:
		return true
	}
	return false
}

// ReadyNow returns the highest-priority runnable layer (lowest number), or 0.
func (s *Scheduler) ReadyNow(now time.Time) int {
	for n := 1; n <= 6; n++ {
		if s.Ready(n, now) {
			return n
		}
	}
	return 0
}

// Dispatch drains layer n's mailbox, stamps its dispatch times, and returns
// the envelopes that triggered the run (the stimulus source).
func (s *Scheduler) Dispatch(n int, now time.Time) []core.BusEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	envs := s.mail[n]
	s.mail[n] = nil
	s.lastDispatch[n] = now
	switch n {
	case 1:
		s.l1LastReview = now
	case 5:
		s.l5LastTick = now
	}
	return envs
}

// NextWake returns how long until the next timer-driven readiness change
// (L1 review, L5 clock). Zero means a layer may already be ready.
func (s *Scheduler) NextWake(now time.Time) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := time.Hour
	if d := s.cfg.L1ReviewInterval - now.Sub(s.l1LastReview); d < next {
		next = d
	}
	if d := s.cfg.L5Period - now.Sub(s.l5LastTick); d < next {
		next = d
	}
	if next < 0 {
		return 0
	}
	return next
}
