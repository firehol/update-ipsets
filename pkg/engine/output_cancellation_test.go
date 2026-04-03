package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
)

func TestWriteComparisonFilesReturnsCanceledContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root := t.TempDir()
		webDir := filepath.Join(root, "web")
		if err := os.MkdirAll(webDir, 0o755); err != nil {
			t.Fatal(err)
		}
		eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
			rt.BaseDir = filepath.Join(root, "base")
			rt.WebDir = webDir
			rt.MaxHeavyPhaseWorkers = 1
		}))

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := eng.writeComparisonFiles(ctx, nil, webDir, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("writeComparisonFiles err=%v, want context.Canceled", err)
		}
		entries, err := os.ReadDir(webDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("canceled comparison wrote %d files, want none", len(entries))
		}
	})
}
