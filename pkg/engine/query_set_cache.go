package engine

import (
	"context"
	"sync"
)

// sharedLatestSetCache keeps binary latest sets alive across concurrent public
// lookup requests. Unlike latestSetCache, this cache must survive across
// requests and invalidate safely when finalize republishes a feed's latest
// binary file.
type sharedLatestSetCache struct {
	engine *Engine

	mu      sync.Mutex
	entries map[string]*sharedLatestSetCacheEntry
}

type sharedLatestSetCacheEntry struct {
	src   *closableSource
	refs  int
	stale bool
}

func newSharedLatestSetCache(engine *Engine) *sharedLatestSetCache {
	return &sharedLatestSetCache{
		engine:  engine,
		entries: make(map[string]*sharedLatestSetCacheEntry),
	}
}

func (c *sharedLatestSetCache) Acquire(name string) (*closableSource, func(), error) {
	if c == nil || c.engine == nil {
		return nil, nil, nil
	}

	c.mu.Lock()
	if entry, ok := c.entries[name]; ok && !entry.stale && entry.src != nil {
		entry.refs++
		c.mu.Unlock()
		return entry.src, c.releaseFunc(name, entry), nil
	}

	src, err := c.engine.openLatestSet(context.Background(), name)
	if err != nil {
		c.mu.Unlock()
		return nil, nil, err
	}
	if !latestSetCacheable(src) {
		c.mu.Unlock()
		return src, func() {
			if src != nil {
				_ = src.Close()
			}
		}, nil
	}

	entry := &sharedLatestSetCacheEntry{
		src:  src,
		refs: 1,
	}
	c.entries[name] = entry
	c.mu.Unlock()

	return src, c.releaseFunc(name, entry), nil
}

func (c *sharedLatestSetCache) releaseFunc(name string, entry *sharedLatestSetCacheEntry) func() {
	return sync.OnceFunc(func() {
		c.release(name, entry)
	})
}

func (c *sharedLatestSetCache) release(name string, entry *sharedLatestSetCacheEntry) {
	if c == nil || entry == nil {
		return
	}
	var toClose *closableSource

	c.mu.Lock()
	if entry.refs > 0 {
		entry.refs--
	}
	if entry.refs == 0 && entry.stale && entry.src != nil {
		toClose = entry.src
		entry.src = nil
	}
	c.mu.Unlock()

	if toClose != nil {
		if err := toClose.Close(); err != nil && c.engine != nil && c.engine.logger != nil {
			c.engine.logger.Warn("query latest-set cache close failed", "set", name, "error", err)
		}
	}
}

func (c *sharedLatestSetCache) Invalidate(name string) {
	if c == nil {
		return
	}
	var toClose *closableSource

	c.mu.Lock()
	entry, ok := c.entries[name]
	if !ok {
		c.mu.Unlock()
		return
	}
	delete(c.entries, name)
	entry.stale = true
	if entry.refs == 0 && entry.src != nil {
		toClose = entry.src
		entry.src = nil
	}
	c.mu.Unlock()

	if toClose != nil {
		if err := toClose.Close(); err != nil && c.engine != nil && c.engine.logger != nil {
			c.engine.logger.Warn("query latest-set cache close failed", "set", name, "error", err)
		}
	}
}
