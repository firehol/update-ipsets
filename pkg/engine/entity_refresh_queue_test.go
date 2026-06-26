package engine

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestEntityArtifactRefreshQueueCoalescesFeedNames(t *testing.T) {
	eng := newEngineFixture(t)

	shouldStart, pending, coalescingKey := eng.enqueueEntityArtifactRefresh([]string{"beta", "alpha", "alpha", ""})
	if !shouldStart {
		t.Fatal("expected first enqueue to start the worker")
	}
	if pending != 2 {
		t.Fatalf("expected 2 pending feeds after first enqueue, got %d", pending)
	}
	if coalescingKey == "" {
		t.Fatal("expected first enqueue to reserve a finite coalescing key")
	}

	shouldStart, pending, coalescingKey = eng.enqueueEntityArtifactRefresh([]string{"beta", "gamma"})
	if shouldStart {
		t.Fatal("expected second enqueue to coalesce into the running worker")
	}
	if pending != 3 {
		t.Fatalf("expected 3 pending feeds after coalescing, got %d", pending)
	}
	if coalescingKey != "" {
		t.Fatalf("coalesced enqueue returned coalescing key %q, want empty", coalescingKey)
	}

	got := eng.drainEntityArtifactRefreshPending()
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected drained feeds: got %#v want %#v", got, want)
	}
}

func TestEntityHealthRefreshQueueCoalescesFeedNames(t *testing.T) {
	eng := newEngineFixture(t)

	shouldStart, pending, coalescingKey := eng.enqueueEntityHealthRefresh([]string{"beta", "alpha", "alpha", ""})
	if !shouldStart {
		t.Fatal("expected first health enqueue to start the worker")
	}
	if pending != 2 {
		t.Fatalf("expected 2 pending health feeds after first enqueue, got %d", pending)
	}
	if coalescingKey == "" {
		t.Fatal("expected first health enqueue to reserve a finite coalescing key")
	}

	shouldStart, pending, coalescingKey = eng.enqueueEntityHealthRefresh([]string{"beta", "gamma"})
	if shouldStart {
		t.Fatal("expected second health enqueue to coalesce into the running worker")
	}
	if pending != 3 {
		t.Fatalf("expected 3 pending health feeds after coalescing, got %d", pending)
	}
	if coalescingKey != "" {
		t.Fatalf("coalesced health enqueue returned coalescing key %q, want empty", coalescingKey)
	}

	got := eng.drainEntityHealthRefreshPending()
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected drained health feeds: got %#v want %#v", got, want)
	}
}

func TestQueueEntityArtifactRefreshDoesNotCoalesceWithCompletingLaneItem(t *testing.T) {
	t.Parallel()

	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	startCompletingRefreshLaneItem(t, eng, "entity:refresh:feed_updates")
	eng.mu.Lock()
	eng.entityRefreshRunning = false
	eng.mu.Unlock()

	result, err := eng.QueueEntityArtifactsRefreshForFeedUpdates(t.Context(), []string{"alpha"}, "feed_update")
	if err != nil {
		t.Fatalf("QueueEntityArtifactsRefreshForFeedUpdates() error = %v", err)
	}
	if result.Coalesced || !result.Queued || result.State != LaneWorkQueued {
		t.Fatalf("queue result = %+v, want newly queued continuation-safe work", result)
	}
	snapshot := eng.engineLane.Snapshot()
	if snapshot.ActiveCount != 1 || snapshot.WaitingCount != 1 {
		t.Fatalf("engine lane active=%d waiting=%d, want 1/1", snapshot.ActiveCount, snapshot.WaitingCount)
	}
	eng.mu.RLock()
	running := eng.entityRefreshRunning
	pending := len(eng.entityRefreshPending)
	eng.mu.RUnlock()
	if !running || pending != 1 {
		t.Fatalf("entity refresh running=%v pending=%d, want true/1", running, pending)
	}

	eng.engineLane.Shutdown(time.Second)
}

func TestQueueEntityHealthRefreshDoesNotCoalesceWithCompletingLaneItem(t *testing.T) {
	t.Parallel()

	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	startCompletingRefreshLaneItem(t, eng, "entity:refresh:health")
	eng.mu.Lock()
	eng.entityHealthRunning = false
	eng.mu.Unlock()

	result, err := eng.QueueEntityArtifactsRefreshForHealthTransitions(t.Context(), []string{"alpha"})
	if err != nil {
		t.Fatalf("QueueEntityArtifactsRefreshForHealthTransitions() error = %v", err)
	}
	if result.Coalesced || !result.Queued || result.State != LaneWorkQueued {
		t.Fatalf("queue result = %+v, want newly queued continuation-safe work", result)
	}
	snapshot := eng.engineLane.Snapshot()
	if snapshot.ActiveCount != 1 || snapshot.WaitingCount != 1 {
		t.Fatalf("engine lane active=%d waiting=%d, want 1/1", snapshot.ActiveCount, snapshot.WaitingCount)
	}
	eng.mu.RLock()
	running := eng.entityHealthRunning
	pending := len(eng.entityHealthPending)
	eng.mu.RUnlock()
	if !running || pending != 1 {
		t.Fatalf("entity health refresh running=%v pending=%d, want true/1", running, pending)
	}

	eng.engineLane.Shutdown(time.Second)
}

func TestEntityRefreshContinuationUsesCallerContext(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	eng.mu.Lock()
	eng.entityRefreshRunning = true
	eng.entityRefreshPending = map[string]struct{}{"alpha": {}}
	eng.mu.Unlock()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	eng.submitEntityArtifactRefreshContinuation(ctx, "feed_update", 1, "entity:refresh:test:canceled")

	eng.mu.RLock()
	running := eng.entityRefreshRunning
	eng.mu.RUnlock()
	if running {
		t.Fatal("entity refresh continuation kept running state after canceled context")
	}
	if snap := eng.engineLane.Snapshot(); snap.ActiveCount != 0 || snap.WaitingCount != 0 {
		t.Fatalf("canceled continuation queued lane work: %+v", snap)
	}
}

func TestEntityHealthContinuationUsesCallerContext(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	eng.mu.Lock()
	eng.entityHealthRunning = true
	eng.entityHealthPending = map[string]struct{}{"alpha": {}}
	eng.mu.Unlock()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	eng.submitEntityHealthRefreshContinuation(ctx, 1, "entity:health:test:canceled")

	eng.mu.RLock()
	running := eng.entityHealthRunning
	eng.mu.RUnlock()
	if running {
		t.Fatal("entity health continuation kept running state after canceled context")
	}
	if snap := eng.engineLane.Snapshot(); snap.ActiveCount != 0 || snap.WaitingCount != 0 {
		t.Fatalf("canceled health continuation queued lane work: %+v", snap)
	}
}

func TestEntityArtifactRefreshPanicClearsRunningFlag(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	restore := setEntityArtifactRefreshAfterDrainHookForTest(func() {
		panic("forced entity artifact refresh panic")
	})
	result, err := eng.QueueEntityArtifactsRefreshForFeedUpdates(t.Context(), []string{"alpha"}, "feed_update")
	if err != nil {
		t.Fatalf("QueueEntityArtifactsRefreshForFeedUpdates() error = %v", err)
	}
	if result.State != LaneWorkQueued && result.State != LaneWorkActive {
		t.Fatalf("queue result = %+v, want admitted work", result)
	}
	waitForEngineLaneIdle(t, eng)
	restore()

	eng.mu.RLock()
	running := eng.entityRefreshRunning
	pending := len(eng.entityRefreshPending)
	eng.mu.RUnlock()
	if running || pending != 0 {
		t.Fatalf("entity refresh running=%v pending=%d after panic, want false/0", running, pending)
	}
}

func TestEntityArtifactRefreshPostTaskPanicClearsRunningFlag(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	restore := setEntityArtifactRefreshAfterTaskHookForTest(func() {
		panic("forced entity artifact refresh post-task panic")
	})
	result, err := eng.QueueEntityArtifactsRefreshForFeedUpdates(t.Context(), []string{"alpha"}, "feed_update")
	if err != nil {
		t.Fatalf("QueueEntityArtifactsRefreshForFeedUpdates() error = %v", err)
	}
	if result.State != LaneWorkQueued && result.State != LaneWorkActive {
		t.Fatalf("queue result = %+v, want admitted work", result)
	}
	waitForEngineLaneIdle(t, eng)
	restore()

	eng.mu.RLock()
	running := eng.entityRefreshRunning
	eng.mu.RUnlock()
	if running {
		t.Fatal("entity refresh remained running after post-task panic")
	}
}

func TestQueuedEntityArtifactRefreshReturnsTaskErrorToLane(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	blockEntityArtifactPublishDir(t, eng)
	eng.mu.Lock()
	eng.entityRefreshRunning = true
	eng.entityRefreshPending = map[string]struct{}{"alpha": {}}
	eng.mu.Unlock()

	err := eng.engineLane.Run(t.Context(), LaneWork{
		Kind:          LaneWorkEntityRefresh,
		Component:     LaneComponentEntityArtifacts,
		Name:          "entity.refresh",
		CoalescingKey: "test:entity:artifact:error",
	}, func(laneCtx context.Context) error {
		return eng.runQueuedEntityArtifactRefresh(laneCtx, "feed_update")
	})
	if err == nil {
		t.Fatal("queued entity artifact refresh returned nil after publish setup failed")
	}

	eng.mu.RLock()
	running := eng.entityRefreshRunning
	pending := len(eng.entityRefreshPending)
	eng.mu.RUnlock()
	if running || pending != 0 {
		t.Fatalf("entity refresh running=%v pending=%d after task error, want false/0", running, pending)
	}
}

func TestEntityHealthRefreshPanicClearsRunningFlag(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	restore := setEntityHealthRefreshAfterDrainHookForTest(func() {
		panic("forced entity health refresh panic")
	})
	result, err := eng.QueueEntityArtifactsRefreshForHealthTransitions(t.Context(), []string{"alpha"})
	if err != nil {
		t.Fatalf("QueueEntityArtifactsRefreshForHealthTransitions() error = %v", err)
	}
	if result.State != LaneWorkQueued && result.State != LaneWorkActive {
		t.Fatalf("queue result = %+v, want admitted work", result)
	}
	waitForEngineLaneIdle(t, eng)
	restore()

	eng.mu.RLock()
	running := eng.entityHealthRunning
	pending := len(eng.entityHealthPending)
	eng.mu.RUnlock()
	if running || pending != 0 {
		t.Fatalf("entity health running=%v pending=%d after panic, want false/0", running, pending)
	}
}

func TestEntityHealthRefreshPostTaskPanicClearsRunningFlag(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	restore := setEntityHealthRefreshAfterTaskHookForTest(func() {
		panic("forced entity health refresh post-task panic")
	})
	result, err := eng.QueueEntityArtifactsRefreshForHealthTransitions(t.Context(), []string{"alpha"})
	if err != nil {
		t.Fatalf("QueueEntityArtifactsRefreshForHealthTransitions() error = %v", err)
	}
	if result.State != LaneWorkQueued && result.State != LaneWorkActive {
		t.Fatalf("queue result = %+v, want admitted work", result)
	}
	waitForEngineLaneIdle(t, eng)
	restore()

	eng.mu.RLock()
	running := eng.entityHealthRunning
	eng.mu.RUnlock()
	if running {
		t.Fatal("entity health refresh remained running after post-task panic")
	}
}

func TestQueuedEntityHealthRefreshReturnsTaskErrorToLane(t *testing.T) {
	eng := newEngineFixture(t, withConfig(entityRefreshQueueTestConfig()))
	blockEntityArtifactPublishDir(t, eng)
	eng.mu.Lock()
	eng.entityHealthRunning = true
	eng.entityHealthPending = map[string]struct{}{"alpha": {}}
	eng.mu.Unlock()

	err := eng.engineLane.Run(t.Context(), LaneWork{
		Kind:          LaneWorkEntityRefresh,
		Component:     LaneComponentEntityArtifactsHealth,
		Name:          "entity.health_refresh",
		CoalescingKey: "test:entity:health:error",
	}, func(laneCtx context.Context) error {
		return eng.runQueuedEntityHealthRefresh(laneCtx)
	})
	if err == nil {
		t.Fatal("queued entity health refresh returned nil after publish setup failed")
	}

	eng.mu.RLock()
	running := eng.entityHealthRunning
	pending := len(eng.entityHealthPending)
	eng.mu.RUnlock()
	if running || pending != 0 {
		t.Fatalf("entity health running=%v pending=%d after task error, want false/0", running, pending)
	}
}

func startCompletingRefreshLaneItem(t *testing.T, eng *Engine, coalescingKey string) {
	t.Helper()
	started := make(chan struct{})
	_, err := eng.engineLane.Submit(t.Context(), LaneWork{
		ID:            "completing-refresh",
		Kind:          LaneWorkEntityRefresh,
		Component:     LaneComponentEntityArtifacts,
		CoalescingKey: coalescingKey,
	}, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("submit completing refresh lane item: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("completing refresh lane item did not start")
	}
}

func blockEntityArtifactPublishDir(t *testing.T, eng *Engine) {
	t.Helper()
	if err := os.WriteFile(eng.runtime.WebDir, []byte("not a directory\n"), 0o600); err != nil {
		t.Fatalf("block web publish dir: %v", err)
	}
}

func entityRefreshQueueTestConfig() *config.Config {
	cfg := config.New()
	cfg.Sources = map[string]*config.Source{
		"alpha": {
			Name:      "alpha",
			URL:       "https://example.test/alpha.txt",
			Frequency: 60,
			IPV:       "ipv4",
			Output:    "ip",
		},
		"geo": {
			Name:      "geo",
			URL:       "https://example.test/geo.csv",
			Frequency: 1440,
			Use:       []string{config.UseGeoIP},
			Format:    "dbip_country_csv",
		},
	}
	return cfg
}
