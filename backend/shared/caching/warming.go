package caching

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// WarmingManager Implementation
// =============================================================================

// warmingManagerImpl implements WarmingManager.
type warmingManagerImpl struct {
	configs  map[string]WarmingConfig
	cache    Cache
	observer CacheObserver
	progress sync.Map // map[string]*WarmingProgress
}

// Compile-time interface check.
var _ WarmingManager = (*warmingManagerImpl)(nil)

// NewWarmingManager creates a new WarmingManager with the given configs, cache, and observer.
// If observer is nil, a no-op observer is used.
func NewWarmingManager(configs []WarmingConfig, cache Cache, observer CacheObserver) WarmingManager {
	cfgMap := make(map[string]WarmingConfig, len(configs))
	for _, cfg := range configs {
		cfgMap[cfg.Namespace] = cfg
	}
	if observer == nil {
		observer = NewNoopObserver()
	}
	return &warmingManagerImpl{
		configs:  cfgMap,
		cache:    cache,
		observer: observer,
	}
}

// Warm warms the cache for the given namespace.
// It looks up the WarmingConfig, creates a context with deadline, calls WarmFunc,
// and tracks progress. Returns ErrWarmingTimeout if the deadline is exceeded.
func (w *warmingManagerImpl) Warm(ctx context.Context, namespace string) error {
	cfg, ok := w.configs[namespace]
	if !ok {
		return ErrInvalidKey
	}

	if !cfg.Enabled {
		return nil
	}

	deadline := cfg.Deadline
	if deadline == 0 {
		deadline = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	// Scope cache to namespace
	nsCache := w.cache.WithNamespace(namespace)

	// Initialize progress
	prog := &WarmingProgress{
		Namespace: namespace,
	}
	w.progress.Store(namespace, prog)

	start := time.Now()

	// Call WarmFunc
	err := cfg.WarmFunc(ctx, nsCache)

	elapsed := time.Since(start).Seconds() * 1000
	prog.ElapsedMs = elapsed

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ErrWarmingTimeout
		}
		prog.FailureCount++
		w.observer.ObserveWarming(ctx, namespace, *prog)
		return err
	}

	prog.SuccessCount++
	w.observer.ObserveWarming(ctx, namespace, *prog)
	return nil
}

// WarmOnStartup warms all namespaces that have OnStartup=true.
// Sequential configs run one at a time; parallel configs run concurrently.
// Returns an aggregated error if any namespace fails.
func (w *warmingManagerImpl) WarmOnStartup(ctx context.Context) error {
	var (
		mu   sync.Mutex
		errs []error
	)

	// Collect sequential and parallel configs
	var sequential []WarmingConfig
	var parallel []WarmingConfig

	for _, cfg := range w.configs {
		if !cfg.Enabled {
			continue
		}
		if !cfg.OnStartup {
			continue
		}
		if cfg.Parallel {
			parallel = append(parallel, cfg)
		} else {
			sequential = append(sequential, cfg)
		}
	}

	// Run sequential configs one at a time
	for _, cfg := range sequential {
		if err := w.Warm(ctx, cfg.Namespace); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}

	// Run parallel configs concurrently
	var wg sync.WaitGroup
	for _, cfg := range parallel {
		wg.Add(1)
		go func(c WarmingConfig) {
			defer wg.Done()
			if err := w.Warm(ctx, c.Namespace); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(cfg)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("warming failed for %d namespace(s): %v", len(errs), errs)
	}
	return nil
}

// TrackProgress returns the current warming progress for the given namespace.
// Returns a zero-value WarmingProgress if no warming has been tracked for the namespace.
// Returns a copy of the progress struct to prevent external mutation of shared state.
func (w *warmingManagerImpl) TrackProgress(namespace string) *WarmingProgress {
	val, ok := w.progress.Load(namespace)
	if !ok {
		return &WarmingProgress{Namespace: namespace}
	}
	p := val.(*WarmingProgress)
	// Return a copy to prevent external mutation of shared state
	return &WarmingProgress{
		Namespace:        p.Namespace,
		EntriesPopulated: p.EntriesPopulated,
		EntriesRemaining: p.EntriesRemaining,
		ElapsedMs:        p.ElapsedMs,
		SuccessCount:     p.SuccessCount,
		FailureCount:     p.FailureCount,
	}
}
