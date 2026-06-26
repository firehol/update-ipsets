package engine

import (
	"context"
	"log/slog"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func (e *Engine) submitCachePersistence(snapshot *cache.State) (uint64, error) {
	if e == nil {
		return 0, ErrCachePersistenceStopped
	}
	worker := e.ensureCachePersistenceWorker()
	return worker.Submit(snapshot)
}

func (e *Engine) ensureCachePersistenceWorker() *cachePersistenceWorker {
	e.cachePersistenceMu.Lock()
	defer e.cachePersistenceMu.Unlock()
	if e.cachePersistence == nil || e.cachePersistence.Stopped() {
		e.cachePersistence = newCachePersistenceWorker(e.cachePath, nonNilSlogLogger(e.logger))
	}
	return e.cachePersistence
}

func (e *Engine) cachePersistenceSnapshot() CachePersistenceSnapshot {
	if e == nil {
		return CachePersistenceSnapshot{State: CachePersistenceIdle}
	}
	e.cachePersistenceMu.Lock()
	worker := e.cachePersistence
	e.cachePersistenceMu.Unlock()
	if worker == nil {
		return CachePersistenceSnapshot{State: CachePersistenceIdle}
	}
	return worker.Snapshot()
}

func (e *Engine) StopCachePersistence(ctx context.Context) error {
	if e == nil {
		return nil
	}
	e.cachePersistenceMu.Lock()
	worker := e.cachePersistence
	e.cachePersistenceMu.Unlock()
	if worker == nil {
		return nil
	}
	return worker.Stop(ctx)
}

func nonNilSlogLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.DiscardHandler)
}
