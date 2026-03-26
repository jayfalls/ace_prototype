package caching

import (
	"golang.org/x/sync/singleflight"
)

// singleFlightImpl wraps golang.org/x/sync/singleflight.Group to implement SingleFlight.
type singleFlightImpl struct {
	group singleflight.Group
}

// NewSingleFlight creates a new SingleFlight instance for stampede protection.
func NewSingleFlight() SingleFlight {
	return &singleFlightImpl{}
}

// Do executes fn for the given key, coalescing concurrent calls.
// If multiple goroutines call Do with the same key simultaneously,
// only one executes fn while others wait for the result.
// The returned bool indicates whether the result was shared with other callers.
func (sf *singleFlightImpl) Do(key string, fn func() (interface{}, error)) (interface{}, error, bool) {
	return sf.group.Do(key, fn)
}

// DoChan is like Do but returns a channel that receives the result.
// The channel is closed after sending, allowing the caller to
// process the result asynchronously.
func (sf *singleFlightImpl) DoChan(key string, fn func() (interface{}, error)) <-chan SingleFlightResult {
	ch := make(chan SingleFlightResult, 1)
	result := sf.group.DoChan(key, fn)

	go func() {
		r := <-result
		ch <- SingleFlightResult{
			Val:    r.Val,
			Err:    r.Err,
			Shared: r.Shared,
		}
	}()

	return ch
}
