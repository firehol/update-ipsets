package engine

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReloadPublicationListenerReplacesByName(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	var firstCalls atomic.Int32
	var secondCalls atomic.Int32
	eng.RegisterReloadPublicationListener("test.listener", func(_ ReloadPublication) error {
		firstCalls.Add(1)
		return nil
	})

	reloadedWebDir := filepath.Join(root, "web-reloaded")
	eng.RegisterReloadPublicationListener("test.listener", func(pub ReloadPublication) error {
		secondCalls.Add(1)
		if pub.Runtime.WebDir != reloadedWebDir {
			t.Fatalf("listener runtime web dir = %q, want %q", pub.Runtime.WebDir, reloadedWebDir)
		}
		return nil
	})

	writeRuntimeReloadConfigWithWebDir(t, cfgPath, root, reloadedWebDir, 2)
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := firstCalls.Load(); got != 0 {
		t.Fatalf("replaced listener was called %d times, want 0", got)
	}
	if got := secondCalls.Load(); got != 1 {
		t.Fatalf("replacement listener was called %d times, want 1", got)
	}
	waitForEngineLaneIdle(t, eng)
}

func TestReloadPublicationListenerErrorSurvivesCleanupQueueFailure(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	reloadedWebDir := filepath.Join(root, "web-reloaded")
	eng.RegisterReloadPublicationListener("test.listener", func(ReloadPublication) error {
		return errors.New("listener diagnostics")
	})
	eng.engineLane.Shutdown(0)

	writeRuntimeReloadConfigWithWebDir(t, cfgPath, root, reloadedWebDir, 2)
	if err := eng.ReloadContext(t.Context()); !errors.Is(err, ErrLaneShuttingDown) {
		t.Fatalf("ReloadContext error = %v, want ErrLaneShuttingDown", err)
	}
	if got := eng.Runtime().WebDir; got != reloadedWebDir {
		t.Fatalf("runtime web dir = %q after cleanup queue failure, want %q", got, reloadedWebDir)
	}
	status := eng.StatusSnapshotLight()
	if status.LastConfigReload.IsZero() {
		t.Fatal("last config reload timestamp is zero after runtime publication")
	}
	if !strings.Contains(status.LastConfigReloadError, "listener diagnostics") ||
		!strings.Contains(status.LastConfigReloadError, ErrLaneShuttingDown.Error()) {
		t.Fatalf("last reload error = %q, want listener and cleanup diagnostics", status.LastConfigReloadError)
	}
}

func TestReloadPublicationListenerPanicIsRecorded(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	reloadedWebDir := filepath.Join(root, "web-reloaded")
	eng.RegisterReloadPublicationListener("test.panic", func(ReloadPublication) error {
		panic("listener boom")
	})

	writeRuntimeReloadConfigWithWebDir(t, cfgPath, root, reloadedWebDir, 2)
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatalf("ReloadContext returned error for recovered listener panic: %v", err)
	}
	if got := eng.Runtime().WebDir; got != reloadedWebDir {
		t.Fatalf("runtime web dir = %q after listener panic, want %q", got, reloadedWebDir)
	}
	status := eng.StatusSnapshotLight()
	if !strings.Contains(status.LastConfigReloadError, "test.panic") || !strings.Contains(status.LastConfigReloadError, "listener boom") {
		t.Fatalf("last reload error = %q, want recovered listener panic", status.LastConfigReloadError)
	}
	waitForEngineLaneIdle(t, eng)
}
