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
}

func (e *Engine) ObserveCounter(name string, count, bytes int64) {
	e.observeRunCounter(name, count, bytes)
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
