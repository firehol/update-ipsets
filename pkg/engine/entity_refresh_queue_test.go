package engine

import (
	"context"
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
