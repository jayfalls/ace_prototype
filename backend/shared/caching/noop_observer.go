package caching

import "context"

// =============================================================================
// No-Op Observer
// =============================================================================

// noOpObserver implements CacheObserver with no-op methods.
// Used as the default observer when none is provided.
type noOpObserver struct{}

func (n *noOpObserver) ObserveGet(ctx context.Context, namespace, key string, hit bool, latencyMs float64) {
}

func (n *noOpObserver) ObserveSet(ctx context.Context, namespace, key string, sizeBytes int64, latencyMs float64) {
}

func (n *noOpObserver) ObserveDelete(ctx context.Context, namespace, key, reason string) {}

func (n *noOpObserver) ObserveEviction(ctx context.Context, namespace, key, reason string) {}

func (n *noOpObserver) ObserveWarming(ctx context.Context, namespace string, progress WarmingProgress) {
}

// NewNoopObserver returns a CacheObserver that performs no operations.
// This is the default observer when none is explicitly configured.
func NewNoopObserver() CacheObserver {
	return &noOpObserver{}
}
