package engine

import (
	"time"
)

func (e *Engine) currentRunMetrics() *runMetrics {
	if e == nil {
		return nil
	}
	return e.currentMetricsPtr.Load()
}

func (e *Engine) observeRunOperation(name string, dur time.Duration) {
	if e == nil || name == "" {
		return
	}
	e.observeRunOperationLocal(name, dur)
}

func (e *Engine) observeRunOperationLocal(name string, dur time.Duration) {
	if e == nil || name == "" {
		return
	}
	e.lifetimeOperations.Observe(name, dur)
	if current := e.currentRunMetrics(); current != nil {
		current.observeOperation(name, dur)
	}
}

func (e *Engine) ObserveOperation(name string, dur time.Duration) {
	if e == nil || name == "" {
		return
	}
	e.observeRunOperationLocal(name, dur)
}

func (e *Engine) TryObserveOperation(name string, dur time.Duration) {
	if e == nil || name == "" {
		return
	}
	e.tryObserveRunOperationLocal(name, dur)
}

func (e *Engine) observeRunOperationAggregate(name string, count int64, total, max time.Duration) {
	if e == nil || name == "" {
		return
	}
	e.lifetimeOperations.ObserveAggregate(name, count, total, max)
	if current := e.currentRunMetrics(); current != nil {
		current.observeOperationAggregate(name, count, total, max)
	}
}

func (e *Engine) tryObserveRunOperationLocal(name string, dur time.Duration) {
	if e == nil || name == "" {
		return
	}
	e.lifetimeOperations.TryObserve(name, dur)
	if current := e.currentRunMetrics(); current != nil {
		current.tryObserveOperation(name, dur)
	}
}

func (e *Engine) observeFeedOperation(feedName, operation string, dur time.Duration) {
	if e == nil || feedName == "" || operation == "" {
		return
	}
	if current := e.currentRunMetrics(); current != nil {
		current.observeFeedOperation(feedName, operation, dur)
	}
}

func (e *Engine) observeRunCounter(name string, count, bytes int64) {
	if e == nil || name == "" {
		return
	}
	e.observeRunCounterLocal(name, count, bytes)
}

func (e *Engine) observeRunCounterLocal(name string, count, bytes int64) {
	if e == nil || name == "" {
		return
	}
	e.lifetimeCounters.Add(name, count, bytes)
	if current := e.currentRunMetrics(); current != nil {
		current.observeCounter(name, count, bytes)
	}
}

func (e *Engine) ObserveCounter(name string, count, bytes int64) {
	if e == nil || name == "" {
		return
	}
	e.observeRunCounterLocal(name, count, bytes)
}

func (e *Engine) TryObserveCounter(name string, count, bytes int64) {
	if e == nil || name == "" {
		return
	}
	e.tryObserveRunCounterLocal(name, count, bytes)
}

func (e *Engine) tryObserveRunCounterLocal(name string, count, bytes int64) {
	if e == nil || name == "" {
		return
	}
	e.lifetimeCounters.TryAdd(name, count, bytes)
	if current := e.currentRunMetrics(); current != nil {
		current.tryObserveCounter(name, count, bytes)
	}
}

func (e *Engine) feedMetricsSnapshot(name string) (FeedTimingSnapshot, bool) {
	if e == nil || name == "" {
		return FeedTimingSnapshot{}, false
	}
	current := e.currentRunMetrics()
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
	if current := e.currentRunMetrics(); current != nil {
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

func (e *Engine) tryLifetimeMetricsSnapshot() *LifetimeMetricsSnapshot {
	if e == nil {
		return nil
	}
	ops, _ := e.lifetimeOperations.TrySnapshot()
	counters, _ := e.lifetimeCounters.TrySnapshot()
	if len(ops) == 0 && len(counters) == 0 {
		return nil
	}
	return &LifetimeMetricsSnapshot{
		Operations: ops,
		Counters:   counters,
	}
}
