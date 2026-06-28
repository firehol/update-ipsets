package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
)

var ErrCachePersistenceStopped = errors.New("cache persistence worker stopped")

var cachePersistenceSave = cache.Save
var cachePersistenceHookMu sync.Mutex
var cachePersistenceWorkerLoopHook func()

type cachePersistenceWorker struct {
	path   string
	logger *slog.Logger

	mu             sync.Mutex
	cond           *sync.Cond
	pending        *cache.State
	pendingSeq     uint64
	inFlightSeq    uint64
	acceptedSeq    uint64
	completedSeq   uint64
	failedSeq      uint64
	completedSaves uint64
	failedSaves    uint64
	saving         bool
	stopping       bool
	stopped        bool
	lastStarted    time.Time
	lastSaved      time.Time
	lastError      string
	done           chan struct{}
}

func newCachePersistenceWorker(path string, logger *slog.Logger) *cachePersistenceWorker {
	w := &cachePersistenceWorker{
		path:   path,
		logger: logger,
		done:   make(chan struct{}),
	}
	w.cond = sync.NewCond(&w.mu)
	go w.run()
	return w
}

func (w *cachePersistenceWorker) Submit(snapshot *cache.State) (uint64, error) {
	if w == nil {
		return 0, ErrCachePersistenceStopped
	}
	if snapshot == nil {
		snapshot = cache.New()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopping || w.stopped {
		return 0, ErrCachePersistenceStopped
	}
	w.acceptedSeq++
	seq := w.acceptedSeq
	w.pending = snapshot
	w.pendingSeq = seq
	w.cond.Broadcast()
	return seq, nil
}

func (w *cachePersistenceWorker) Snapshot() CachePersistenceSnapshot {
	if w == nil {
		return CachePersistenceSnapshot{State: CachePersistenceIdle}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshotLocked()
}

func (w *cachePersistenceWorker) TrySnapshot() (CachePersistenceSnapshot, bool) {
	if w == nil {
		return CachePersistenceSnapshot{State: CachePersistenceIdle}, true
	}
	if !w.mu.TryLock() {
		return CachePersistenceSnapshot{}, false
	}
	defer w.mu.Unlock()
	return w.snapshotLocked(), true
}

func (w *cachePersistenceWorker) snapshotLocked() CachePersistenceSnapshot {
	state := CachePersistenceIdle
	pending := w.pending != nil
	switch {
	case w.stopped:
		state = CachePersistenceStopped
	case w.saving:
		state = CachePersistenceSaving
	case pending:
		state = CachePersistencePending
	case w.lastError != "":
		state = CachePersistenceFailed
	}
	return CachePersistenceSnapshot{
		State:       state,
		Pending:     pending,
		Saving:      w.saving,
		LastStarted: w.lastStarted,
		LastSaved:   w.lastSaved,
		LastError:   w.lastError,
		Accepted:    w.acceptedSeq,
		Completed:   w.completedSaves,
		Failed:      w.failedSaves,
	}
}

func (w *cachePersistenceWorker) Stopped() bool {
	if w == nil {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

func (w *cachePersistenceWorker) Wait(ctx context.Context, seq uint64) error {
	if w == nil || seq == 0 {
		return nil
	}
	ctx = nonNilContext(ctx)
	stopContextWake := context.AfterFunc(ctx, func() {
		w.mu.Lock()
		w.cond.Broadcast()
		w.mu.Unlock()
	})
	defer stopContextWake()
	w.mu.Lock()
	defer w.mu.Unlock()
	for {
		completed := w.completedSeq >= seq
		failed := w.failedSeq >= seq
		lastErr := w.lastError
		stopped := w.stopped
		if completed {
			return nil
		}
		if failed && lastErr != "" {
			return errors.New(lastErr)
		}
		if stopped {
			return ErrCachePersistenceStopped
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		w.cond.Wait()
	}
}

func (w *cachePersistenceWorker) Stop(ctx context.Context) error {
	if w == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	w.mu.Lock()
	if !w.stopping {
		w.stopping = true
		w.cond.Broadcast()
	}
	w.mu.Unlock()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *cachePersistenceWorker) run() {
	defer close(w.done)
	defer w.recoverRunPanic()
	for {
		if hook := cachePersistenceWorkerLoopHookForTest(); hook != nil {
			hook()
		}
		w.mu.Lock()
		for w.pending == nil && !w.stopping {
			w.cond.Wait()
		}
		if w.pending == nil && w.stopping {
			w.stopped = true
			w.cond.Broadcast()
			w.mu.Unlock()
			return
		}
		state := w.pending
		seq := w.pendingSeq
		w.pending = nil
		w.pendingSeq = 0
		w.inFlightSeq = seq
		w.saving = true
		w.lastStarted = time.Now().UTC()
		w.mu.Unlock()

		err := saveCachePersistenceState(w.path, state)

		w.mu.Lock()
		w.saving = false
		w.inFlightSeq = 0
		if err != nil {
			w.failedSeq = seq
			w.failedSaves++
			w.lastError = err.Error()
			if w.logger != nil {
				w.logger.Error("failed to persist cache", "error", err)
			}
		} else {
			w.completedSeq = seq
			w.completedSaves++
			w.lastSaved = time.Now().UTC()
			w.lastError = ""
		}
		w.cond.Broadcast()
		w.mu.Unlock()
	}
}

func (w *cachePersistenceWorker) recoverRunPanic() {
	if w == nil {
		return
	}
	recovered := recover()
	if recovered == nil {
		return
	}
	err := fmt.Errorf("cache persistence worker panicked: %v", recovered)
	w.mu.Lock()
	failedSeq := w.inFlightSeq
	if failedSeq == 0 {
		failedSeq = w.pendingSeq
	}
	w.pending = nil
	w.pendingSeq = 0
	w.inFlightSeq = 0
	w.saving = false
	w.stopping = true
	w.stopped = true
	if failedSeq != 0 {
		w.failedSeq = failedSeq
	}
	w.failedSaves++
	w.lastError = err.Error()
	w.cond.Broadcast()
	w.mu.Unlock()
	if w.logger != nil {
		w.logger.Error("cache persistence worker stopped after panic", "error", err)
	}
}

func saveCachePersistenceState(path string, state *cache.State) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("cache persistence save panicked: %v", recovered)
		}
	}()
	cachePersistenceHookMu.Lock()
	save := cachePersistenceSave
	cachePersistenceHookMu.Unlock()
	return save(path, state)
}

func cachePersistenceWorkerLoopHookForTest() func() {
	cachePersistenceHookMu.Lock()
	defer cachePersistenceHookMu.Unlock()
	return cachePersistenceWorkerLoopHook
}

func setCachePersistenceSaveForTest(fn func(string, *cache.State) error) func() {
	cachePersistenceHookMu.Lock()
	old := cachePersistenceSave
	cachePersistenceSave = fn
	cachePersistenceHookMu.Unlock()
	return func() {
		cachePersistenceHookMu.Lock()
		cachePersistenceSave = old
		cachePersistenceHookMu.Unlock()
	}
}

func setCachePersistenceWorkerLoopHookForTest(fn func()) func() {
	cachePersistenceHookMu.Lock()
	old := cachePersistenceWorkerLoopHook
	cachePersistenceWorkerLoopHook = fn
	cachePersistenceHookMu.Unlock()
	return func() {
		cachePersistenceHookMu.Lock()
		cachePersistenceWorkerLoopHook = old
		cachePersistenceHookMu.Unlock()
	}
}
