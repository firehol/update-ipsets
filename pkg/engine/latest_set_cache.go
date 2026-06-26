package engine

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// latestSetCache keeps already-open feed sets alive for the duration of
// a single heavy processing block. The heavy phases repeatedly touch the
// same target feeds, so reopening the same binary/text set thousands of
// times is pure overhead.
type latestSetCache struct {
	engine   *Engine
	snapshot operationSnapshot

	mu     sync.Mutex
	closed bool
	sets   map[string]*closableSource
	errs   map[string]error
	loads  map[string]*latestSetLoad

	summaries map[string]latestSetSummary
	filters   map[string]latestSetFilter
}

func newLatestSetCache(engine *Engine) *latestSetCache {
	return &latestSetCache{
		engine:    engine,
		sets:      make(map[string]*closableSource),
		errs:      make(map[string]error),
		loads:     make(map[string]*latestSetLoad),
		summaries: make(map[string]latestSetSummary),
		filters:   make(map[string]latestSetFilter),
	}
}

func newLatestSetCacheForSnapshot(engine *Engine, snap operationSnapshot) *latestSetCache {
	cache := newLatestSetCache(engine)
	cache.snapshot = snap
	return cache
}

type latestSetSummary struct {
	summary iprange.RangeSourceSummary
	err     error
}

type latestSetFilter struct {
	filter iprange.RangeOverlapFilter
	err    error
}

type latestSetLoad struct {
	done     chan struct{}
	finished bool
}

var errLatestSetCacheClosed = errors.New("latest set cache closed")

func finishLatestSetLoad(load *latestSetLoad) {
	if load == nil || load.finished {
		return
	}
	close(load.done)
	load.finished = true
}

func (c *latestSetCache) Open(name string) (*closableSource, error) {
	return c.OpenContext(context.Background(), name)
}

func (c *latestSetCache) OpenContext(ctx context.Context, name string) (*closableSource, error) {
	ctx = nonNilContext(ctx)
	if c == nil || c.engine == nil {
		return nil, nil
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	for {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}

		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, errLatestSetCacheClosed
		}
		if src, ok := c.sets[name]; ok {
			c.mu.Unlock()
			return src, nil
		}
		if err, ok := c.errs[name]; ok {
			c.mu.Unlock()
			return nil, err
		}
		if load := c.loads[name]; load != nil {
			done := load.done
			c.mu.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		load := &latestSetLoad{done: make(chan struct{})}
		c.loads[name] = load
		c.mu.Unlock()

		src, err := c.openLatestSet(ctx, name)
		ctxErr := contextErr(ctx)
		c.mu.Lock()
		closed := c.closed
		if c.loads[name] == load {
			delete(c.loads, name)
		}
		finishLatestSetLoad(load)
		if closed {
			c.mu.Unlock()
			if src != nil {
				_ = src.Close()
			}
			return nil, errLatestSetCacheClosed
		}
		if ctxErr != nil {
			c.mu.Unlock()
			if src != nil {
				_ = src.Close()
			}
			return nil, ctxErr
		}
		if err != nil {
			c.errs[name] = err
			c.mu.Unlock()
			return nil, err
		}
		if !latestSetCacheable(src) {
			c.mu.Unlock()
			return src, nil
		}
		if cached, ok := c.sets[name]; ok {
			c.mu.Unlock()
			if src != nil {
				_ = src.Close()
			}
			return cached, nil
		}
		c.sets[name] = src
		c.mu.Unlock()
		return src, nil
	}
}

func (c *latestSetCache) openLatestSet(ctx context.Context, name string) (*closableSource, error) {
	if c.snapshot.cfg != nil {
		return c.engine.openLatestSetWithSnapshot(ctx, c.snapshot, name)
	}
	return c.engine.openLatestSet(ctx, name)
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

	src, err := c.OpenContext(ctx, name)
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
		defer func() { _ = src.Close() }()
	}
	summary, err := iprange.BuildRangeSourceSummaryContext(ctx, src.RangeSource)
	if err == nil {
		err = checkRangeSourceErr(src.RangeSource)
	}

	c.mu.Lock()
	c.summaries[name] = latestSetSummary{summary: summary, err: err}
	c.filters[name] = latestSetFilter{filter: summary.OverlapFilter(), err: err}
	c.mu.Unlock()
	return summary, err
}

func (c *latestSetCache) OverlapFilter(ctx context.Context, name string) (iprange.RangeOverlapFilter, error) {
	if c == nil {
		return iprange.RangeOverlapFilter{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	if cached, ok := c.summaries[name]; ok {
		c.mu.Unlock()
		if cached.err != nil {
			return iprange.RangeOverlapFilter{}, cached.err
		}
		return cached.summary.OverlapFilter(), nil
	}
	if cached, ok := c.filters[name]; ok {
		c.mu.Unlock()
		return cached.filter, cached.err
	}
	c.mu.Unlock()

	src, err := c.OpenContext(ctx, name)
	if err != nil {
		c.mu.Lock()
		c.filters[name] = latestSetFilter{err: err}
		c.mu.Unlock()
		return iprange.RangeOverlapFilter{}, err
	}
	if src == nil || src.RangeSource == nil {
		return iprange.RangeOverlapFilter{}, nil
	}
	if !latestSetCacheable(src) {
		defer func() { _ = src.Close() }()
	}
	filter, err := iprange.BuildRangeOverlapFilterContext(ctx, src.RangeSource)
	if err == nil {
		err = checkRangeSourceErr(src.RangeSource)
	}

	c.mu.Lock()
	c.filters[name] = latestSetFilter{filter: filter, err: err}
	c.mu.Unlock()
	return filter, err
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
	c.closed = true
	sets := c.sets
	for _, load := range c.loads {
		finishLatestSetLoad(load)
	}
	c.sets = make(map[string]*closableSource)
	c.errs = make(map[string]error)
	c.loads = make(map[string]*latestSetLoad)
	c.summaries = make(map[string]latestSetSummary)
	c.filters = make(map[string]latestSetFilter)
	c.mu.Unlock()

	for name, src := range sets {
		if src == nil {
			continue
		}
		if err := src.Close(); err != nil && logger != nil {
			logger.Warn("heavy phase set cache close failed", "set", name, "error", err)
		}
	}
}

func checkRangeSourceErr(src iprange.RangeSource) error {
	return iprange.RangeSourceErr(src)
}
