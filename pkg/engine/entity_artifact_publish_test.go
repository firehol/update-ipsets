package engine

import (
	"os"
	"path/filepath"
	"testing"
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
