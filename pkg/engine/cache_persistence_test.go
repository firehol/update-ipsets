package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func TestRunFinalizationReleasesLaneBeforeCachePersistenceSubmit(t *testing.T) {
	eng := newEngineFixture(t)
	finalizing := make(chan struct{})
	releaseFinalization := make(chan struct{})
	var hookCalls atomic.Int32
	restore := setRunFinalizationBeforeCacheSubmitHookForTest(func() {
		if hookCalls.Add(1) == 1 {
			close(finalizing)
			<-releaseFinalization
		}
	})
	t.Cleanup(restore)

	done := make(chan error, 1)
	go func() {
		_, err := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
		done <- err
	}()

	select {
	case <-finalizing:
	case <-time.After(time.Second):
		t.Fatal("run did not reach finalizing state")
	}

	status := eng.StatusSnapshotLight()
	if status.RunState != RunStateFinalizing {
		t.Fatalf("run_state = %q, want %q", status.RunState, RunStateFinalizing)
	}
	if !status.Running {
		t.Fatal("legacy running field should stay true while run_state is finalizing")
	}
	if status.EngineLane.ActiveCount != 0 {
		t.Fatalf("engine lane active count = %d, want 0 during finalization", status.EngineLane.ActiveCount)
	}

	laneDone := make(chan error, 1)
	go func() {
		ok, err := eng.engineLane.TryRun(context.Background(), LaneWork{
			Kind:      LaneWorkCleanup,
			Component: LaneComponentCriticalInfrastructure,
			Name:      "test.cleanup",
		}, func(context.Context) error { return nil })
		if err != nil {
			laneDone <- err
			return
		}
		if !ok {
			laneDone <- errors.New("engine lane rejected work while finalization was outside the lane")
			return
		}
		laneDone <- nil
	}()
	select {
	case err := <-laneDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("engine lane work blocked behind run finalization")
	}

	close(releaseFinalization)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunOnce returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunOnce did not finish after finalization was released")
	}
	stopCachePersistenceForTest(t, eng)
}

func TestRunFinalizationRejectsSecondRunUntilReleased(t *testing.T) {
	eng := newEngineFixture(t)
	finalizing := make(chan struct{})
	releaseFinalization := make(chan struct{})
	var hookCalls atomic.Int32
	restore := setRunFinalizationBeforeCacheSubmitHookForTest(func() {
		if hookCalls.Add(1) == 1 {
			close(finalizing)
			<-releaseFinalization
		}
	})
	t.Cleanup(restore)

	firstDone := make(chan error, 1)
	go func() {
		_, err := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
		firstDone <- err
	}()

	select {
	case <-finalizing:
	case <-time.After(time.Second):
		t.Fatal("first run did not reach finalization")
	}

	secondDone := make(chan error, 1)
	go func() {
		_, err := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
		secondDone <- err
	}()
	select {
	case err := <-secondDone:
		if err == nil || !strings.Contains(err.Error(), "run already in progress") {
			t.Fatalf("second RunOnce error = %v, want run already in progress", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second RunOnce did not return while first run was finalizing")
	}

	close(releaseFinalization)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first RunOnce returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first RunOnce did not finish after finalization was released")
	}
	stopCachePersistenceForTest(t, eng)
}

func TestSynchronousCachePersistenceErrorIsRunOnceError(t *testing.T) {
	eng := newEngineFixture(t)
	saveErr := errors.New("forced cache save failure")
	restore := setCachePersistenceSaveForTest(func(string, *cache.State) error {
		return saveErr
	})
	t.Cleanup(restore)

	_, err := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true})
	if err == nil || !strings.Contains(err.Error(), saveErr.Error()) {
		t.Fatalf("RunOnce error = %v, want cache save failure", err)
	}
}

func TestCachePersistenceWorkerRecoversPanicAndAcceptsNextSave(t *testing.T) {
	var saves atomic.Int32
	restore := setCachePersistenceSaveForTest(func(path string, st *cache.State) error {
		if saves.Add(1) == 1 {
			panic("forced cache save panic")
		}
		return nil
	})
	t.Cleanup(restore)

	worker := newCachePersistenceWorker(filepath.Join(t.TempDir(), ".cache.json"), slog.New(slog.DiscardHandler))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = worker.Stop(ctx)
	})

	firstSeq, err := worker.Submit(cache.New())
	if err != nil {
		t.Fatalf("first Submit returned error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	firstErr := worker.Wait(ctx, firstSeq)
	cancel()
	if firstErr == nil || !strings.Contains(firstErr.Error(), "forced cache save panic") {
		t.Fatalf("first Wait error = %v, want recovered panic", firstErr)
	}

	secondSeq, err := worker.Submit(cache.New())
	if err != nil {
		t.Fatalf("second Submit returned error after recovered panic: %v", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Wait(ctx, secondSeq); err != nil {
		t.Fatalf("second Wait returned error: %v", err)
	}
	snapshot := worker.Snapshot()
	if snapshot.Completed != 1 || snapshot.Failed != 1 || snapshot.LastError != "" {
		t.Fatalf("worker snapshot after recovered panic and save = %#v", snapshot)
	}
}

func TestCachePersistenceWorkerLoopPanicStopsWorkerAndWakesCallers(t *testing.T) {
	restore := setCachePersistenceWorkerLoopHookForTest(func() {
		panic("forced worker loop panic")
	})
	t.Cleanup(restore)

	worker := newCachePersistenceWorker(filepath.Join(t.TempDir(), ".cache.json"), slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Stop(ctx); err != nil {
		t.Fatalf("Stop after worker loop panic returned error: %v", err)
	}

	snapshot := worker.Snapshot()
	if snapshot.State != CachePersistenceStopped || snapshot.Failed != 1 {
		t.Fatalf("worker snapshot after loop panic = %#v, want stopped with one failed save", snapshot)
	}
	if !strings.Contains(snapshot.LastError, "forced worker loop panic") {
		t.Fatalf("worker last error = %q, want recovered loop panic", snapshot.LastError)
	}
	if _, err := worker.Submit(cache.New()); !errors.Is(err, ErrCachePersistenceStopped) {
		t.Fatalf("Submit after worker loop panic error = %v, want stopped", err)
	}
}

func TestPublishFinalizationDoesNotHoldEngineLane(t *testing.T) {
	modified := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  sample:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: tests
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return modified.Add(time.Hour) }

	syncStarted := make(chan struct{})
	releaseSync := make(chan struct{})
	var once sync.Once
	restore := setSyncGeneratedFilesBeforeHookForTest(func() {
		once.Do(func() { close(syncStarted) })
		<-releaseSync
	})
	t.Cleanup(restore)

	done := make(chan error, 1)
	go func() {
		_, err := runSchedulerStyleOnce(t, eng, RunOptions{
			Selected:              []string{"sample"},
			EnableAll:             true,
			Manual:                true,
			CleanupOld:            true,
			AsyncCachePersistence: true,
		})
		done <- err
	}()

	select {
	case <-syncStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not reach publish finalization sync")
	}

	status := eng.StatusSnapshotLight()
	if status.RunState != RunStateFinalizing {
		t.Fatalf("run_state = %q, want %q while publish finalization is blocked", status.RunState, RunStateFinalizing)
	}
	if status.EngineLane.ActiveCount != 0 {
		t.Fatalf("engine lane active count = %d, want 0 while publish finalization is blocked", status.EngineLane.ActiveCount)
	}
	ok, err := eng.engineLane.TryRun(context.Background(), LaneWork{
		Kind:      LaneWorkCleanup,
		Component: LaneComponentCriticalInfrastructure,
		Name:      "test.cleanup",
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("engine lane returned error while publish finalization was blocked: %v", err)
	}
	if !ok {
		t.Fatal("engine lane rejected work while publish finalization was blocked")
	}
	_, secondErr := eng.RunOnce(context.Background(), RunOptions{
		Selected:              []string{"sample"},
		EnableAll:             true,
		Manual:                true,
		CleanupOld:            true,
		AsyncCachePersistence: true,
	})
	if secondErr == nil || !strings.Contains(secondErr.Error(), "run already in progress") {
		t.Fatalf("second RunOnce error = %v, want run already in progress", secondErr)
	}

	close(releaseSync)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunOnce returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunOnce did not finish after publish finalization was released")
	}
	stopCachePersistenceForTest(t, eng)
}

func TestCompleteRunPublicationRecoversPanicAndCleansStage(t *testing.T) {
	eng := newEngineFixture(t)
	webBatch, err := eng.newWebPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	stageDir := webBatch.stageDir
	err = eng.completeRunPublication(&runFinalization{
		report:   &Report{},
		webBatch: webBatch,
		opts: RunOptions{
			BeforePublish: func(*Report) error {
				panic("forced final publication panic")
			},
		},
	})
	if !errors.Is(err, ErrLanePanic) {
		t.Fatalf("completeRunPublication error = %v, want ErrLanePanic", err)
	}
	if _, statErr := os.Stat(stageDir); !os.IsNotExist(statErr) {
		t.Fatalf("stage dir still exists after finalization panic: stat err=%v", statErr)
	}
}

func TestBlockedCachePersistenceDoesNotBlockRunOrEngineLane(t *testing.T) {
	eng := newEngineFixture(t)
	saveStarted := make(chan struct{})
	releaseSave := make(chan struct{})
	var once sync.Once
	restore := setCachePersistenceSaveForTest(func(path string, st *cache.State) error {
		once.Do(func() { close(saveStarted) })
		<-releaseSave
		return nil
	})
	t.Cleanup(restore)

	done := make(chan error, 1)
	go func() {
		_, err := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
		done <- err
	}()

	select {
	case <-saveStarted:
	case <-time.After(time.Second):
		t.Fatal("cache persistence worker did not start saving")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunOnce returned error: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("RunOnce blocked behind cache persistence")
	}

	status := eng.StatusSnapshotLight()
	if status.RunState != RunStateIdle {
		t.Fatalf("run_state = %q, want %q after save was accepted", status.RunState, RunStateIdle)
	}
	if status.CachePersistence.State != CachePersistenceSaving {
		t.Fatalf("cache persistence state = %q, want %q", status.CachePersistence.State, CachePersistenceSaving)
	}

	ok, err := eng.engineLane.TryRun(context.Background(), LaneWork{
		Kind:      LaneWorkCleanup,
		Component: LaneComponentCriticalInfrastructure,
		Name:      "test.cleanup",
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("engine lane returned error: %v", err)
	}
	if !ok {
		t.Fatal("engine lane rejected work while cache persistence was blocked")
	}

	close(releaseSave)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := eng.StopCachePersistence(ctx); err != nil {
		t.Fatalf("StopCachePersistence returned error: %v", err)
	}
}

func TestCachePersistenceWorkerCoalescesNewestAcceptedSnapshot(t *testing.T) {
	var (
		mu          sync.Mutex
		saved       []uint64
		saveStarted = make(chan struct{})
		releaseSave = make(chan struct{})
		once        sync.Once
	)
	restore := setCachePersistenceSaveForTest(func(path string, st *cache.State) error {
		entry := st.EntrySnapshot("sample")
		if entry == nil {
			t.Fatal("missing sample entry in submitted snapshot")
		}
		mu.Lock()
		saved = append(saved, entry.UniqueIPs)
		mu.Unlock()
		once.Do(func() {
			close(saveStarted)
			<-releaseSave
		})
		return nil
	})
	t.Cleanup(restore)

	worker := newCachePersistenceWorker("unused", slog.New(slog.DiscardHandler))
	firstSeq, err := worker.Submit(cacheStateWithUniqueIPs(1))
	if err != nil {
		t.Fatalf("Submit(first) returned error: %v", err)
	}
	select {
	case <-saveStarted:
	case <-time.After(time.Second):
		t.Fatal("first save did not start")
	}
	if _, err := worker.Submit(cacheStateWithUniqueIPs(2)); err != nil {
		t.Fatalf("Submit(second) returned error: %v", err)
	}
	latestSeq, err := worker.Submit(cacheStateWithUniqueIPs(3))
	if err != nil {
		t.Fatalf("Submit(third) returned error: %v", err)
	}
	if latestSeq <= firstSeq {
		t.Fatalf("latest sequence %d did not advance beyond first %d", latestSeq, firstSeq)
	}
	close(releaseSave)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Wait(ctx, latestSeq); err != nil {
		t.Fatalf("Wait(latest) returned error: %v", err)
	}
	if err := worker.Stop(ctx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	mu.Lock()
	got := append([]uint64(nil), saved...)
	mu.Unlock()
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("saved snapshots = %v, want [1 3]", got)
	}
}

func cacheStateWithUniqueIPs(uniqueIPs uint64) *cache.State {
	st := cache.New()
	entry := st.Entry("sample")
	entry.UniqueIPs = uniqueIPs
	return st
}
