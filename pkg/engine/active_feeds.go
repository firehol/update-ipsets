package engine

import "sort"

func (e *Engine) markFeedStart(name string, feed ActiveFeed) {
	if e == nil || name == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeFeeds == nil {
		e.activeFeeds = make(map[string]ActiveFeed)
	}
	e.activeFeeds[name] = feed
}

func (e *Engine) markFeedEnd(name string) {
	if e == nil || name == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeFeeds == nil {
		return
	}
	delete(e.activeFeeds, name)
}

func (e *Engine) snapshotActiveFeedsLocked() []ActiveFeed {
	if len(e.activeFeeds) == 0 {
		return nil
	}
	out := make([]ActiveFeed, 0, len(e.activeFeeds))
	for _, feed := range e.activeFeeds {
		out = append(out, feed)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].Name < out[j].Name
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func (e *Engine) ActiveFeedsSnapshot() []ActiveFeed {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.snapshotActiveFeedsLocked()
}
