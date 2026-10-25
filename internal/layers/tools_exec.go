package layers

import (
	"context"
	"sync"
)

// Call is one tool invocation scheduled through Executor.
type Call struct {
	Name     string
	Path     string // filesystem/workspace target; "" uses a global lane
	ReadOnly bool
	Run      func(ctx context.Context) (string, error)
}

// Executor runs tool calls under the spec-9.3 concurrency policy:
//
//   - Read-only calls overlap, capped at ReadCap (layers.parallel_reads, 4).
//   - Mutating calls are serialized globally.
//   - Two calls touching the same path are never in flight together —
//     reads take a per-path shared lock, writes the exclusive lock.
type Executor struct {
	sem     chan struct{}
	writeMu sync.Mutex
	paths   sync.Map // string -> *sync.RWMutex
}

// NewExecutor builds an Executor; readCap <= 0 defaults to 4.
func NewExecutor(readCap int) *Executor {
	if readCap <= 0 {
		readCap = 4
	}
	return &Executor{sem: make(chan struct{}, readCap)}
}

func (x *Executor) lockFor(path string) *sync.RWMutex {
	if path == "" {
		path = "\x00global"
	}
	v, _ := x.paths.LoadOrStore(path, &sync.RWMutex{})
	return v.(*sync.RWMutex)
}

// RunOne executes one call under the concurrency policy.
func (x *Executor) RunOne(ctx context.Context, c Call) (string, error) {
	lk := x.lockFor(c.Path)
	if c.ReadOnly {
		select {
		case x.sem <- struct{}{}:
			defer func() { <-x.sem }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
		lk.RLock()
		defer lk.RUnlock()
		return c.Run(ctx)
	}
	x.writeMu.Lock()
	defer x.writeMu.Unlock()
	lk.Lock()
	defer lk.Unlock()
	return c.Run(ctx)
}

// RunBatch executes calls preserving input order. Maximal runs of
// consecutive read-only calls overlap (capped); mutating calls run
// serially. results[i] corresponds to calls[i].
func (x *Executor) RunBatch(ctx context.Context, calls []Call) []string {
	results := make([]string, len(calls))
	i := 0
	for i < len(calls) {
		if !calls[i].ReadOnly {
			r, err := x.RunOne(ctx, calls[i])
			results[i] = outcome(r, err)
			i++
			continue
		}
		j := i
		var wg sync.WaitGroup
		for j < len(calls) && calls[j].ReadOnly {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				r, err := x.RunOne(ctx, calls[k])
				results[k] = outcome(r, err)
			}(j)
			j++
		}
		wg.Wait()
		i = j
	}
	return results
}

func outcome(r string, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return r
}
