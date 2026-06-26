package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOptimisticEntityArtifactMutationRestagesAfterGenerationChange(t *testing.T) {
	eng := newEngineFixture(t)
	attempts := 0

	err := eng.runOptimisticEntityArtifactMutation(t.Context(), nil, "test mutation", func() (*entityArtifactMutationPlan, error) {
		attempts++
		entityBatch, err := eng.newEntityPublishBatch()
		if err != nil {
			return nil, err
		}
		body := []byte("attempt-1\n")
		if attempts == 2 {
			body = []byte("attempt-2\n")
		}
		if err := os.WriteFile(filepath.Join(entityBatch.stageDir, "version"), body, generatedFileMode); err != nil {
			entityBatch.cleanup()
			return nil, err
		}
		if attempts == 1 {
			eng.entityArtifactsMu.Lock()
			eng.bumpEntityArtifactGenerationLocked()
			eng.entityArtifactsMu.Unlock()
		}
		return &entityArtifactMutationPlan{entity: entityBatch}, nil
	})
	if err != nil {
		t.Fatalf("runOptimisticEntityArtifactMutation() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("stage attempts = %d, want 2", attempts)
	}
	body, err := os.ReadFile(eng.entityVersionPath())
	if err != nil {
		t.Fatalf("ReadFile(version) error = %v", err)
	}
	if got, want := string(body), "attempt-2\n"; got != want {
		t.Fatalf("published body = %q, want %q", got, want)
	}
}

func TestEntityArtifactPublishDoesNotBlockLaterStaging(t *testing.T) {
	eng := newEngineFixture(t)
	entityBatch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer entityBatch.cleanup()
	if err := os.WriteFile(filepath.Join(entityBatch.stageDir, "version"), []byte("first\n"), generatedFileMode); err != nil {
		t.Fatal(err)
	}

	publishPaused := make(chan struct{})
	releasePublish := make(chan struct{})
	restore := setEntityArtifactPublishAfterLeaseHookForTest(func() {
		close(publishPaused)
		<-releasePublish
	})
	defer restore()

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- eng.publishEntityArtifactMutationPlan(
			t.Context(),
			nil,
			"first publish",
			eng.entityArtifactGenerationSnapshot(),
			&entityArtifactMutationPlan{entity: entityBatch},
		)
	}()

	select {
	case <-publishPaused:
	case <-time.After(2 * time.Second):
		t.Fatal("first entity artifact publish did not reach publish hook")
	}

	secondStageStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- eng.runOptimisticEntityArtifactMutation(t.Context(), nil, "second mutation", func() (*entityArtifactMutationPlan, error) {
			close(secondStageStarted)
			return nil, nil
		})
	}()

	select {
	case <-secondStageStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second entity artifact mutation did not reach staging while first publish was paused")
	}

	close(releasePublish)
	if err := <-firstDone; err != nil {
		t.Fatalf("first publish error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second mutation error = %v", err)
	}
}

func TestEntityArtifactPublishMarksIntegrityStaleAfterReleasingPublishLease(t *testing.T) {
	eng := newEngineFixture(t)
	entityBatch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer entityBatch.cleanup()
	if err := os.WriteFile(filepath.Join(entityBatch.stageDir, "version"), []byte("published\n"), generatedFileMode); err != nil {
		t.Fatal(err)
	}
	leaseAcquired := make(chan struct{})
	restore := setEntityArtifactPublishAfterLeaseHookForTest(func() {
		close(leaseAcquired)
	})
	defer restore()

	eng.pipelineIntegrityCacheMu.Lock()
	publishDone := make(chan error, 1)
	go func() {
		publishDone <- eng.publishEntityArtifactMutationPlan(
			t.Context(),
			nil,
			"publish",
			eng.entityArtifactGenerationSnapshot(),
			&entityArtifactMutationPlan{entity: entityBatch},
		)
	}()

	select {
	case <-leaseAcquired:
	case <-time.After(time.Second):
		eng.pipelineIntegrityCacheMu.Unlock()
		t.Fatal("publish did not acquire entity artifact lease")
	}

	leaseAvailable := make(chan struct{})
	go func() {
		eng.entityArtifactPublishMu.Lock()
		close(leaseAvailable)
		eng.entityArtifactPublishMu.Unlock()
	}()

	select {
	case <-leaseAvailable:
	case <-time.After(time.Second):
		eng.pipelineIntegrityCacheMu.Unlock()
		t.Fatal("entity publish lease stayed locked while integrity stale marking was blocked")
	}

	eng.pipelineIntegrityCacheMu.Unlock()
	select {
	case err := <-publishDone:
		if err != nil {
			t.Fatalf("publish error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("publish did not finish after integrity cache lock was released")
	}
}

func TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease(t *testing.T) {
	eng := newEngineFixture(t)
	entityBatch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer entityBatch.cleanup()
	if err := os.WriteFile(filepath.Join(entityBatch.stageDir, "version"), []byte("published\n"), generatedFileMode); err != nil {
		t.Fatal(err)
	}

	syncStarted := make(chan struct{})
	releaseSync := make(chan struct{})
	restore := setSyncGeneratedFilesBeforeHookForTest(func() {
		close(syncStarted)
		<-releaseSync
	})
	defer restore()

	publishDone := make(chan error, 1)
	go func() {
		publishDone <- eng.publishEntityArtifactMutationPlan(
			t.Context(),
			nil,
			"publish",
			eng.entityArtifactGenerationSnapshot(),
			&entityArtifactMutationPlan{entity: entityBatch},
		)
	}()

	select {
	case <-syncStarted:
	case <-time.After(time.Second):
		t.Fatal("publish did not reach generated-file sync")
	}
	select {
	case err := <-publishDone:
		close(releaseSync)
		t.Fatalf("publish returned before generated-file sync was released: %v", err)
	default:
	}

	leaseAvailable := make(chan struct{})
	go func() {
		eng.entityArtifactPublishMu.Lock()
		close(leaseAvailable)
		eng.entityArtifactPublishMu.Unlock()
	}()

	select {
	case <-leaseAvailable:
	case <-time.After(time.Second):
		close(releaseSync)
		t.Fatal("entity publish lease stayed locked while generated-file sync was blocked")
	}

	close(releaseSync)
	select {
	case err := <-publishDone:
		if err != nil {
			t.Fatalf("publish error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("publish did not finish after generated-file sync was released")
	}
}

func TestPublishRunArtifactsReleasesEntityLeaseAfterPanic(t *testing.T) {
	eng := newEngineFixture(t)
	webBatch, err := eng.newWebPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer webBatch.cleanup()
	entityBatch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer entityBatch.cleanup()
	if err := os.WriteFile(filepath.Join(entityBatch.stageDir, "version"), []byte("published\n"), generatedFileMode); err != nil {
		t.Fatal(err)
	}
	restore := setEntityArtifactPublishAfterLeaseHookForTest(func() {
		panic("forced entity publish panic")
	})
	defer restore()

	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("publishRunArtifacts did not panic")
			}
		}()
		_ = eng.publishRunArtifacts(
			context.Background(),
			RunOptions{},
			&Report{},
			pipelineRunPlan{},
			nil,
			webBatch,
			entityBatch,
		)
	}()

	leaseAvailable := make(chan struct{})
	go func() {
		eng.entityArtifactPublishMu.Lock()
		close(leaseAvailable)
		eng.entityArtifactPublishMu.Unlock()
	}()

	select {
	case <-leaseAvailable:
	case <-time.After(time.Second):
		t.Fatal("entity publish lease stayed locked after panic")
	}
}
