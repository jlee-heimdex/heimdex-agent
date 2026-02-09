package pipelines

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const defaultCacheTTL = 5 * time.Minute

// CachedDoctor wraps a Runner to cache doctor probe results with a configurable TTL.
// This avoids running the doctor subprocess on every index job.
type CachedDoctor struct {
	runner Runner
	ttl    time.Duration
	logger *slog.Logger

	mu     sync.RWMutex
	cached *Capabilities
}

// NewCachedDoctor creates a caching wrapper around doctor probes.
func NewCachedDoctor(runner Runner, logger *slog.Logger) *CachedDoctor {
	return &CachedDoctor{
		runner: runner,
		ttl:    defaultCacheTTL,
		logger: logger,
	}
}

// Get returns cached capabilities if fresh, otherwise re-probes.
func (d *CachedDoctor) Get(ctx context.Context) (*Capabilities, error) {
	d.mu.RLock()
	if d.cached != nil && time.Since(d.cached.ProbedAt) < d.ttl {
		caps := d.cached
		d.mu.RUnlock()
		return caps, nil
	}
	d.mu.RUnlock()

	return d.Refresh(ctx)
}

func (d *CachedDoctor) Peek() *Capabilities {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cached
}

// Refresh forces a new doctor probe regardless of cache freshness.
func (d *CachedDoctor) Refresh(ctx context.Context) (*Capabilities, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	caps, err := d.runner.RunDoctor(ctx)
	if err != nil {
		d.logger.Warn("doctor probe failed", "error", err)
		// Return stale cache if available
		if d.cached != nil {
			d.logger.Info("returning stale capabilities cache")
			return d.cached, nil
		}
		return nil, err
	}

	d.cached = caps
	return caps, nil
}

// Invalidate clears the cached capabilities.
func (d *CachedDoctor) Invalidate() {
	d.mu.Lock()
	d.cached = nil
	d.mu.Unlock()
}
