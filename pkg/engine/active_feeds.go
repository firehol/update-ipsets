package engine

import "sort"

func (e *Engine) markFeedStart(name string, feed ActiveFeed) {
	if e == nil || name == "" {
		return
	}
	e.activeFeedsMu.Lock()
	defer e.activeFeedsMu.Unlock()
	if e.activeFeeds == nil {
		e.activeFeeds = make(map[string]ActiveFeed)
	}
	e.activeFeeds[name] = feed
}

func (e *Engine) markFeedEnd(name string) {
	if e == nil || name == "" {
		return
	}
	e.activeFeedsMu.Lock()
	defer e.activeFeedsMu.Unlock()
	if e.activeFeeds == nil {
		return
	}
	delete(e.activeFeeds, name)
}

func activeFeedsFromMap(in map[string]ActiveFeed) []ActiveFeed {
	if len(in) == 0 {
		return nil
	}
	out := make([]ActiveFeed, 0, len(in))
	for _, feed := range in {
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

func (e *Engine) snapshotActiveFeeds() []ActiveFeed {
	if e == nil {
		return nil
	}
	e.activeFeedsMu.RLock()
	defer e.activeFeedsMu.RUnlock()
	return activeFeedsFromMap(e.activeFeeds)
}

func (e *Engine) trySnapshotActiveFeeds() ([]ActiveFeed, bool) {
	if e == nil {
		return nil, true
	}
	if !e.activeFeedsMu.TryRLock() {
		return nil, false
	}
	defer e.activeFeedsMu.RUnlock()
	return activeFeedsFromMap(e.activeFeeds), true
}

func (e *Engine) ActiveFeedsSnapshot() []ActiveFeed {
	if e == nil {
		return nil
	}
	return e.snapshotActiveFeeds()
}
