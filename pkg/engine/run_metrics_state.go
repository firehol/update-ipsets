package engine

import (
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
)

func (e *Engine) observeRunOperation(name string, dur time.Duration) {
	if e == nil || name == "" {
		return
	}
	observability.Duration(observability.BackgroundContext(), name, dur)
	e.lifetimeOperations.Observe(name, dur)
	e.mu.RLock()
	current := e.currentMetrics
	e.mu.RUnlock()
	if current != nil {
		current.observeOperation(name, dur)
	}
}

func (e *Engine) ObserveOperation(name string, dur time.Duration) {
	e.observeRunOperation(name, dur)
}

func (e *Engine) observeRunOperationAggregate(name string, count int64, total, max time.Duration) {
	if e == nil || name == "" {
		return
	}
	observability.Count(observability.BackgroundContext(), name, count)
	observability.Duration(observability.BackgroundContext(), name+".aggregate", total)
	e.lifetimeOperations.ObserveAggregate(name, count, total, max)
	e.mu.RLock()
	current := e.currentMetrics
	e.mu.RUnlock()
	if current != nil {
		current.observeOperationAggregate(name, count, total, max)
	}
}

func (e *Engine) observeFeedOperation(feedName, operation string, dur time.Duration) {
	if e == nil || feedName == "" || operation == "" {
		return
	}
	e.mu.RLock()
	current := e.currentMetrics
	e.mu.RUnlock()
	if current != nil {
		current.observeFeedOperation(feedName, operation, dur)
	}
}

func (e *Engine) observeRunCounter(name string, count, bytes int64) {
	if e == nil || name == "" {
		return
	}
	observability.Observe(observability.BackgroundContext(), name, count, bytes, 0)
	e.lifetimeCounters.Add(name, count, bytes)
	e.mu.RLock()
	current := e.currentMetrics
	e.mu.RUnlock()
	if current != nil {
		current.observeCounter(name, count, bytes)
	}
}

func (e *Engine) ObserveCounter(name string, count, bytes int64) {
	e.observeRunCounter(name, count, bytes)
}

func (e *Engine) feedMetricsSnapshot(name string) (FeedTimingSnapshot, bool) {
	if e == nil || name == "" {
		return FeedTimingSnapshot{}, false
	}
	e.mu.RLock()
	current := e.currentMetrics
	e.mu.RUnlock()
	if current == nil {
		return FeedTimingSnapshot{}, false
	}
	return current.feedSnapshot(name)
}

func (e *Engine) observeFeedWork(name string, result FeedProcessingResult, elapsed time.Duration) {
	if e == nil || name == "" {
		return
	}
	e.observeRunCounter("sources.feeds_processed", 1, result.Work.InputBytes)
	if result.Work.Entries > 0 {
		e.observeRunCounter("sources.entries_processed", result.Work.Entries, 0)
	}
	if result.Work.UniqueIPs > 0 {
		e.observeRunCounter("sources.unique_ips_processed", result.Work.UniqueIPs, 0)
	}
	e.mu.RLock()
	current := e.currentMetrics
	e.mu.RUnlock()
	if current != nil {
		current.observeFeedWork(name, result, elapsed)
	}
}

func (e *Engine) lifetimeMetricsSnapshot() *LifetimeMetricsSnapshot {
	if e == nil {
		return nil
	}
	ops := e.lifetimeOperations.Snapshot()
	counters := e.lifetimeCounters.Snapshot()
	if len(ops) == 0 && len(counters) == 0 {
		return nil
	}
	return &LifetimeMetricsSnapshot{
		Operations: ops,
		Counters:   counters,
	}
}
