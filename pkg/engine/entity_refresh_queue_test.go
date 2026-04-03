package engine

import (
	"slices"
	"testing"
)

func TestEntityArtifactRefreshQueueCoalescesFeedNames(t *testing.T) {
	eng := newEngineFixture(t)

	shouldStart, pending := eng.enqueueEntityArtifactRefresh([]string{"beta", "alpha", "alpha", ""})
	if !shouldStart {
		t.Fatal("expected first enqueue to start the worker")
	}
	if pending != 2 {
		t.Fatalf("expected 2 pending feeds after first enqueue, got %d", pending)
	}

	shouldStart, pending = eng.enqueueEntityArtifactRefresh([]string{"beta", "gamma"})
	if shouldStart {
		t.Fatal("expected second enqueue to coalesce into the running worker")
	}
	if pending != 3 {
		t.Fatalf("expected 3 pending feeds after coalescing, got %d", pending)
	}

	got := eng.drainEntityArtifactRefreshPending()
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected drained feeds: got %#v want %#v", got, want)
	}
}

func TestEntityHealthRefreshQueueCoalescesFeedNames(t *testing.T) {
	eng := newEngineFixture(t)

	shouldStart, pending := eng.enqueueEntityHealthRefresh([]string{"beta", "alpha", "alpha", ""})
	if !shouldStart {
		t.Fatal("expected first health enqueue to start the worker")
	}
	if pending != 2 {
		t.Fatalf("expected 2 pending health feeds after first enqueue, got %d", pending)
	}

	shouldStart, pending = eng.enqueueEntityHealthRefresh([]string{"beta", "gamma"})
	if shouldStart {
		t.Fatal("expected second health enqueue to coalesce into the running worker")
	}
	if pending != 3 {
		t.Fatalf("expected 3 pending health feeds after coalescing, got %d", pending)
	}

	got := eng.drainEntityHealthRefreshPending()
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected drained health feeds: got %#v want %#v", got, want)
	}
}
