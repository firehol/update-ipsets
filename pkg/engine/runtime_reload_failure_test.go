package engine

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestReloadContextDoesNotInstallRuntimeWhenDirectoryCreationFails(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	beforeRuntime := eng.Runtime()
	beforeReloads := eng.StatusSnapshotLight().ConfigReloadCount

	blockedParent := filepath.Join(root, "blocked-parent")
	if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeRuntimeReloadConfigWithWebDir(t, cfgPath, root, filepath.Join(blockedParent, "web"), 2)

	err = eng.ReloadContext(t.Context())
	if err == nil {
		t.Fatal("ReloadContext() error = nil, want directory creation error")
	}
	afterRuntime := eng.Runtime()
	if afterRuntime.WebDir != beforeRuntime.WebDir {
		t.Fatalf("runtime web dir = %q after failed reload, want previous %q", afterRuntime.WebDir, beforeRuntime.WebDir)
	}
	if afterRuntime.BaseDir != beforeRuntime.BaseDir {
		t.Fatalf("runtime base dir = %q after failed reload, want previous %q", afterRuntime.BaseDir, beforeRuntime.BaseDir)
	}
	afterReloads := eng.StatusSnapshotLight().ConfigReloadCount
	if afterReloads != beforeReloads+1 {
		t.Fatalf("reload count = %d after failed reload, want %d", afterReloads, beforeReloads+1)
	}
	status := eng.StatusSnapshotLight()
	if status.LastConfigReload.IsZero() {
		t.Fatal("last config reload timestamp is zero after failed reload")
	}
	if status.LastConfigReloadError == "" {
		t.Fatal("last config reload error is empty after failed reload")
	}
	waitForEngineLaneIdle(t, eng)
}
