package engine

import (
	"context"
	"log/slog"
	"sync"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// latestSetCache keeps already-open feed sets alive for the duration of
// a single heavy processing block. The heavy phases repeatedly touch the
// same target feeds, so reopening the same binary/text set thousands of
// times is pure overhead.
type latestSetCache struct {
	engine *Engine

	mu   sync.Mutex
	sets map[string]*closableSource
	errs map[string]error
}

func newLatestSetCache(engine *Engine) *latestSetCache {
	return &latestSetCache{
		engine: engine,
		sets:   make(map[string]*closableSource),
		errs:   make(map[string]error),
	}
}

func (c *latestSetCache) Open(name string) (*closableSource, error) {
	if c == nil || c.engine == nil {
		return nil, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if src, ok := c.sets[name]; ok {
		return src, nil
	}
	if err, ok := c.errs[name]; ok {
		return nil, err
	}

	src, err := c.engine.openLatestSet(context.Background(), name)
	if err != nil {
		c.errs[name] = err
		return nil, err
	}
	if !latestSetCacheable(src) {
		return src, nil
	}
	c.sets[name] = src
	return src, nil
}

func latestSetCacheable(src *closableSource) bool {
	if src == nil {
		return false
	}
	_, ok := src.RangeSource.(iprange.FileSet)
	return ok
}

func (c *latestSetCache) CloseAll(logger *slog.Logger) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for name, src := range c.sets {
		if src == nil {
			continue
		}
		if err := src.Close(); err != nil && logger != nil {
			logger.Warn("heavy phase set cache close failed", "set", name, "error", err)
		}
	}
	c.sets = make(map[string]*closableSource)
	c.errs = make(map[string]error)
}
