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

	summaries map[string]latestSetSummary
}

func newLatestSetCache(engine *Engine) *latestSetCache {
	return &latestSetCache{
		engine:    engine,
		sets:      make(map[string]*closableSource),
		errs:      make(map[string]error),
		summaries: make(map[string]latestSetSummary),
	}
}

type latestSetSummary struct {
	summary iprange.RangeSourceSummary
	err     error
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

func (c *latestSetCache) Summary(ctx context.Context, name string) (iprange.RangeSourceSummary, error) {
	if c == nil {
		return iprange.RangeSourceSummary{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	if cached, ok := c.summaries[name]; ok {
		c.mu.Unlock()
		return cached.summary, cached.err
	}
	c.mu.Unlock()

	src, err := c.Open(name)
	if err != nil {
		c.mu.Lock()
		c.summaries[name] = latestSetSummary{err: err}
		c.mu.Unlock()
		return iprange.RangeSourceSummary{}, err
	}
	if src == nil || src.RangeSource == nil {
		return iprange.RangeSourceSummary{}, nil
	}
	if !latestSetCacheable(src) {
		defer src.Close()
	}
	summary, err := iprange.BuildRangeSourceSummaryContext(ctx, src.RangeSource)
	if err == nil {
		err = checkRangeSourceErr(src.RangeSource)
	}

	c.mu.Lock()
	c.summaries[name] = latestSetSummary{summary: summary, err: err}
	c.mu.Unlock()
	return summary, err
}

func (c *latestSetCache) OverlapFilter(ctx context.Context, name string) (iprange.RangeOverlapFilter, error) {
	summary, err := c.Summary(ctx, name)
	if err != nil {
		return iprange.RangeOverlapFilter{}, err
	}
	return summary.OverlapFilter(), nil
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
	c.summaries = make(map[string]latestSetSummary)
}

func checkRangeSourceErr(src iprange.RangeSource) error {
	return iprange.RangeSourceErr(src)
}
