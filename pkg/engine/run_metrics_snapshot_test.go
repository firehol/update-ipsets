package engine

import (
	"context"
	"testing"
	"time"
)

func TestRunMetricsTrySnapshotDoesNotWaitForMetricsLock(t *testing.T) {
	metrics := newRunMetrics(time.Now().UTC(), RunPhaseMetadata)
	metrics.observeOperation("metadata.write_indexes", time.Millisecond)

	metrics.mu.Lock()
	if got, ok := metrics.trySnapshot(true); ok || got.Current {
		t.Fatalf("trySnapshot while locked = %+v, %v; want empty snapshot, false", got, ok)
	}
	metrics.mu.Unlock()

	got, ok := metrics.trySnapshot(true)
	if !ok {
		t.Fatal("trySnapshot after unlock returned ok=false")
	}
	if len(got.Operations) != 1 || got.Operations[0].Name != "metadata.write_indexes" {
		t.Fatalf("trySnapshot after unlock operations = %+v, want metadata.write_indexes", got.Operations)
	}
}

func TestStatusSnapshotDoesNotWaitForCurrentMetricsLock(t *testing.T) {
	eng := newEngineFixture(t)
	metrics := newRunMetrics(time.Now().UTC(), RunPhaseMetadata)
	metrics.observeOperation("metadata.write_indexes", time.Millisecond)
	eng.currentMetricsPtr.Store(metrics)
	t.Cleanup(func() { eng.currentMetricsPtr.Store(nil) })

	metrics.mu.Lock()
	status := eng.StatusSnapshot()
	metrics.mu.Unlock()
	if status.CurrentMetrics != nil {
		t.Fatalf("StatusSnapshot current_metrics while locked = %+v, want omitted best-effort metrics", status.CurrentMetrics)
	}

	status = eng.StatusSnapshot()
	if status.CurrentMetrics == nil {
		t.Fatal("StatusSnapshot current_metrics after unlock = nil, want metrics")
	}
}

func TestTrySnapshotsDoNotWaitForEngineMutex(t *testing.T) {
	eng := newEngineFixture(t)

	eng.mu.Lock()
	defer eng.mu.Unlock()
	configDone := make(chan bool, 1)
	policyDone := make(chan bool, 1)
	schedulerConfigDone := make(chan bool, 1)
	recheckTargetDone := make(chan bool, 1)
	reprocessStateDone := make(chan bool, 1)
	enableDone := make(chan bool, 1)
	disableDone := make(chan bool, 1)
	enableArtifactDone := make(chan bool, 1)
	disableArtifactDone := make(chan bool, 1)
	lightDone := make(chan bool, 1)
	fullDone := make(chan bool, 1)
	go func() {
		_, _, ok := eng.TryConfigRuntimeSnapshot()
		configDone <- ok
	}()
	go func() {
		_, _, _, ok := eng.TryConfigRuntimePolicySnapshot()
		policyDone <- ok
	}()
	go func() {
		_, ok := eng.TrySchedulerConfigSnapshot()
		schedulerConfigDone <- ok
	}()
	go func() {
		_, ok := eng.TryResolveRecheckTarget("alpha")
		recheckTargetDone <- ok
	}()
	go func() {
		_, ok := eng.TryHasLocalReprocessState("alpha")
		reprocessStateDone <- ok
	}()
	go func() {
		ok, _ := eng.TryEnable([]string{"alpha"}, false)
		enableDone <- ok
	}()
	go func() {
		ok, _ := eng.TryDisable([]string{"alpha"}, false)
		disableDone <- ok
	}()
	go func() {
		ok, _ := eng.TryEnableArtifacts([]string{"alpha"}, false)
		enableArtifactDone <- ok
	}()
	go func() {
		ok, _ := eng.TryDisableArtifacts([]string{"alpha"}, false)
		disableArtifactDone <- ok
	}()
	go func() {
		_, ok := eng.TryStatusSnapshotLight()
		lightDone <- ok
	}()
	go func() {
		_, ok := eng.TryStatusSnapshot()
		fullDone <- ok
	}()

	for name, ch := range map[string]<-chan bool{
		"config":           configDone,
		"policy":           policyDone,
		"scheduler_config": schedulerConfigDone,
		"recheck_target":   recheckTargetDone,
		"reprocess_state":  reprocessStateDone,
		"enable":           enableDone,
		"disable":          disableDone,
		"enable_artifact":  enableArtifactDone,
		"disable_artifact": disableArtifactDone,
		"light":            lightDone,
		"full":             fullDone,
	} {
		select {
		case ok := <-ch:
			if ok {
				t.Fatalf("%s try snapshot returned ok=true while engine mutex was held", name)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s try snapshot waited for engine mutex", name)
		}
	}
}

func TestTryStatusSnapshotsDoNotWaitForStatusComponentLocks(t *testing.T) {
	lockers := map[string]func(*Engine) func(){
		"engine lane": func(eng *Engine) func() {
			eng.engineLane.mu.Lock()
			return eng.engineLane.mu.Unlock
		},
		"git lane": func(eng *Engine) func() {
			eng.gitLane.mu.Lock()
			return eng.gitLane.mu.Unlock
		},
		"cache persistence pointer": func(eng *Engine) func() {
			eng.cachePersistenceMu.Lock()
			return eng.cachePersistenceMu.Unlock
		},
		"cache persistence worker": func(eng *Engine) func() {
			worker := eng.ensureCachePersistenceWorker()
			worker.mu.Lock()
			return worker.mu.Unlock
		},
		"active feeds": func(eng *Engine) func() {
			eng.activeFeedsMu.Lock()
			return eng.activeFeedsMu.Unlock
		},
		"active operations": func(eng *Engine) func() {
			eng.activeOperationsMu.Lock()
			return eng.activeOperationsMu.Unlock
		},
		"background tasks": func(eng *Engine) func() {
			eng.backgroundTasksMu.Lock()
			return eng.backgroundTasksMu.Unlock
		},
		"engine lane warning": func(eng *Engine) func() {
			eng.engineLaneLongHoldWarningMu.Lock()
			return eng.engineLaneLongHoldWarningMu.Unlock
		},
		"pipeline integrity cache": func(eng *Engine) func() {
			eng.pipelineIntegrityCacheMu.Lock()
			return eng.pipelineIntegrityCacheMu.Unlock
		},
		"entity integrity cache": func(eng *Engine) func() {
			eng.entityIntegrityCacheMu.Lock()
			return eng.entityIntegrityCacheMu.Unlock
		},
	}
	for name, lock := range lockers {
		t.Run(name, func(t *testing.T) {
			eng := newEngineFixture(t)
			assertTryStatusSnapshotsDoNotWait(t, eng, lock)
		})
	}
}

func assertTryStatusSnapshotsDoNotWait(t *testing.T, eng *Engine, lock func(*Engine) func()) {
	t.Helper()
	unlock := lock(eng)
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()

	type result struct {
		name string
		ok   bool
	}
	done := make(chan result, 2)
	go func() {
		_, ok := eng.TryStatusSnapshotLight()
		done <- result{name: "light", ok: ok}
	}()
	go func() {
		_, ok := eng.TryStatusSnapshot()
		done <- result{name: "full", ok: ok}
	}()

	for range 2 {
		select {
		case got := <-done:
			if got.ok {
				t.Fatalf("%s try status snapshot returned ok=true while dependency lock was held", got.name)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("try status snapshot waited for dependency lock")
		}
	}
	unlock()
	locked = false
}

func TestTryIntegrityCacheSnapshotsDoNotWaitForCacheLocks(t *testing.T) {
	eng := newEngineFixture(t)
	opts := IntegrityOptions{WebDir: eng.runtime.WebDir}

	eng.pipelineIntegrityCacheMu.Lock()
	pipelineDone := make(chan bool, 1)
	go func() {
		_, ok := eng.TryPipelineIntegrityCacheSnapshot(opts)
		pipelineDone <- ok
	}()
	select {
	case ok := <-pipelineDone:
		if ok {
			t.Fatal("TryPipelineIntegrityCacheSnapshot returned ok=true while cache lock was held")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TryPipelineIntegrityCacheSnapshot waited for cache lock")
	}
	eng.pipelineIntegrityCacheMu.Unlock()

	eng.entityIntegrityCacheMu.Lock()
	entityDone := make(chan bool, 1)
	go func() {
		_, ok := eng.TryEntityIntegrityCacheSnapshot()
		entityDone <- ok
	}()
	select {
	case ok := <-entityDone:
		if ok {
			t.Fatal("TryEntityIntegrityCacheSnapshot returned ok=true while cache lock was held")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TryEntityIntegrityCacheSnapshot waited for cache lock")
	}
	eng.entityIntegrityCacheMu.Unlock()
}

func TestIntegrityQueueAPIsDoNotWaitForCacheLocks(t *testing.T) {
	eng := newEngineFixture(t)
	opts := IntegrityOptions{WebDir: eng.runtime.WebDir}

	eng.pipelineIntegrityCacheMu.Lock()
	refreshDone := make(chan error, 1)
	go func() {
		_, err := eng.QueuePipelineIntegrityRefresh(t.Context(), opts, "test")
		refreshDone <- err
	}()
	select {
	case err := <-refreshDone:
		if err != nil {
			t.Fatalf("QueuePipelineIntegrityRefresh() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("QueuePipelineIntegrityRefresh waited for cache lock")
	}

	reprocessDone := make(chan error, 1)
	go func() {
		_, err := eng.QueuePipelineIntegrityReprocess(t.Context(), opts, "test", func(context.Context, []IntegrityFinding) error {
			return nil
		})
		reprocessDone <- err
	}()
	select {
	case err := <-reprocessDone:
		if err != ErrIntegrityCacheBusy {
			t.Fatalf("QueuePipelineIntegrityReprocess() error = %v, want ErrIntegrityCacheBusy", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("QueuePipelineIntegrityReprocess waited for cache lock")
	}
	eng.pipelineIntegrityCacheMu.Unlock()

	eng.entityIntegrityCacheMu.Lock()
	entityDone := make(chan error, 1)
	go func() {
		_, err := eng.QueueEntityIntegrityRefresh(t.Context(), "test")
		entityDone <- err
	}()
	select {
	case err := <-entityDone:
		if err != nil {
			t.Fatalf("QueueEntityIntegrityRefresh() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("QueueEntityIntegrityRefresh waited for cache lock")
	}
	eng.entityIntegrityCacheMu.Unlock()

	waitForEngineLaneIdle(t, eng)
}
