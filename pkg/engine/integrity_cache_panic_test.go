package engine

import (
	"errors"
	"strings"
	"testing"
)

func TestPipelineIntegrityRefreshPanicSettlesCache(t *testing.T) {
	eng := newEngineFixture(t)
	restore := setPipelineIntegrityAfterRunningHookForTest(func() {
		panic("forced pipeline integrity panic")
	})
	opts := IntegrityOptions{}
	if _, err := eng.QueuePipelineIntegrityRefresh(t.Context(), opts, "test"); err != nil {
		t.Fatalf("QueuePipelineIntegrityRefresh() error = %v", err)
	}
	waitForEngineLaneIdle(t, eng)
	restore()

	snap := eng.PipelineIntegrityCacheSnapshot(opts)
	if snap.CacheState == IntegrityCacheRefreshRunning || snap.Running {
		t.Fatalf("pipeline integrity cache stayed running after panic: %+v", snap)
	}
	if !strings.Contains(snap.LastError, "forced pipeline integrity panic") {
		t.Fatalf("pipeline integrity last error = %q, want panic detail", snap.LastError)
	}
}

func TestEntityIntegrityRefreshPanicSettlesCache(t *testing.T) {
	eng := newEngineFixture(t)
	restore := setEntityIntegrityAfterRunningHookForTest(func() {
		panic("forced entity integrity panic")
	})
	if _, err := eng.QueueEntityIntegrityRefresh(t.Context(), "test"); err != nil {
		t.Fatalf("QueueEntityIntegrityRefresh() error = %v", err)
	}
	waitForEngineLaneIdle(t, eng)
	restore()

	snap := eng.EntityIntegrityCacheSnapshot()
	if snap.CacheState == IntegrityCacheRefreshRunning || snap.Running {
		t.Fatalf("entity integrity cache stayed running after panic: %+v", snap)
	}
	if !strings.Contains(snap.LastError, "forced entity integrity panic") {
		t.Fatalf("entity integrity last error = %q, want panic detail", snap.LastError)
	}
}

func TestPipelineIntegrityLateQueuedStateDoesNotOverrideSettledWork(t *testing.T) {
	eng := newEngineFixture(t)
	opts := eng.normalizeIntegrityOptions(IntegrityOptions{})
	workID := "integrity_refresh:pipeline:test"
	eng.setPipelineIntegrityRunning(opts, workID, "test")
	eng.setPipelineIntegritySettled(opts, workID, nil, errors.New("forced pipeline integrity failure"))
	eng.setPipelineIntegrityQueued(opts, workID, LaneTicket{ID: workID, State: LaneWorkActive})

	snap := eng.PipelineIntegrityCacheSnapshot(opts)
	if snap.CacheState == IntegrityCacheRefreshRunning || snap.Running || snap.Ticket != nil {
		t.Fatalf("late queued state overrode settled pipeline integrity work: %+v", snap)
	}
	if !strings.Contains(snap.LastError, "forced pipeline integrity failure") {
		t.Fatalf("pipeline integrity last error = %q, want original failure", snap.LastError)
	}
}

func TestEntityIntegrityLateQueuedStateDoesNotOverrideSettledWork(t *testing.T) {
	eng := newEngineFixture(t)
	workID := "integrity_refresh:entity:test"
	eng.setEntityIntegrityRunning(workID, "test")
	eng.setEntityIntegritySettled(workID, nil, errors.New("forced entity integrity failure"))
	eng.setEntityIntegrityQueued(workID, LaneTicket{ID: workID, State: LaneWorkActive})

	snap := eng.EntityIntegrityCacheSnapshot()
	if snap.CacheState == IntegrityCacheRefreshRunning || snap.Running || snap.Ticket != nil {
		t.Fatalf("late queued state overrode settled entity integrity work: %+v", snap)
	}
	if !strings.Contains(snap.LastError, "forced entity integrity failure") {
		t.Fatalf("entity integrity last error = %q, want original failure", snap.LastError)
	}
}
