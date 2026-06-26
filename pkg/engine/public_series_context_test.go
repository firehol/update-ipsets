package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWritePublicRetentionJSONHonorsCancelledContext(t *testing.T) {
	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = filepath.Join(root, "lib")
		rt.WebDir = filepath.Join(root, "web")
	}))
	if err := os.MkdirAll(filepath.Join(eng.runtime.LibDir, "sample", "new"), generatedDirMode); err != nil {
		t.Fatalf("mkdir retention dir: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := eng.writePublicRetentionJSONContext(ctx, "sample", eng.runtime.WebDir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("writePublicRetentionJSONContext() error = %v, want context.Canceled", err)
	}
	if fileExists(filepath.Join(eng.runtime.WebDir, "sample_retention.json")) {
		t.Fatal("writePublicRetentionJSONContext() wrote retention artifact after context cancellation")
	}
}

func TestWritePerFeedDerivativeArtifactsHonorsCancelledContext(t *testing.T) {
	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = filepath.Join(root, "lib")
		rt.WebDir = filepath.Join(root, "web")
	}))
	run := eng.newMetadataWriteRun(t.Context(), eng.runtime.WebDir, []string{"sample"}, true)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := run.writePerFeedDerivativeArtifacts(ctx, "sample", time.Unix(1_700_000_000, 0).UTC(), true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("writePerFeedDerivativeArtifacts() error = %v, want context.Canceled", err)
	}
	if len(run.generated) != 0 {
		t.Fatalf("writePerFeedDerivativeArtifacts() generated %d files after cancellation, want 0", len(run.generated))
	}
}
