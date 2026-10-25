package provider

import (
	"sync"
	"time"
)

// breaker tracks per group+model circuit state: cooldown doubling on each
// trip up to max (spec 9.2: 60 s doubling, 300 s cap; open → next group).
type breaker struct {
	mu       sync.Mutex
	cooldown time.Duration
	max      time.Duration
	state    map[string]*breakerCell
}

type breakerCell struct {
	until    time.Time
	current  time.Duration // cooldown applied on the NEXT trip
	failures int
}

func newBreaker(cooldownSec, maxSec int) *breaker {
	if cooldownSec <= 0 {
		cooldownSec = 60
	}
	if maxSec <= 0 {
		maxSec = 300
	}
	return &breaker{
		cooldown: time.Duration(cooldownSec) * time.Second,
		max:      time.Duration(maxSec) * time.Second,
		state:    map[string]*breakerCell{},
	}
}

func (b *breaker) cell(group, model string) *breakerCell {
	k := group + "|" + model
	c, ok := b.state[k]
	if !ok {
		c = &breakerCell{current: b.cooldown}
		b.state[k] = c
	}
	return c
}

// Open reports whether the circuit is cooling down — the registry skips the
// group+model while open.
func (b *breaker) Open(group, model string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cell(group, model).until.After(time.Now())
}

// Trip opens the circuit: first trip applies cooldown, each subsequent trip
// doubles it up to max.
func (b *breaker) Trip(group, model string) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.cell(group, model)
	c.until = time.Now().Add(c.current)
	c.failures++
	d := c.current
	c.current *= 2
	if c.current > b.max {
		c.current = b.max
	}
	return d
}

// Reset closes the circuit after a success.
func (b *breaker) Reset(group, model string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.cell(group, model)
	c.until = time.Time{}
	c.current = b.cooldown
	c.failures = 0
}
