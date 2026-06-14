package dronebl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ErrMissingPassword = errors.New("missing DroneBL rsync password")

type FetchOptions struct {
	RsyncURL    string
	DataDir     string
	RsyncBinary string
	Timeout     time.Duration
}

func FetchBuildzone(ctx context.Context, opts FetchOptions) error {
	if opts.RsyncURL == "" {
		return fmt.Errorf("rsync URL is required")
	}
	if opts.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}
	if err := os.MkdirAll(opts.DataDir, generatedDirMode); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	if err := os.Chmod(opts.DataDir, generatedDirMode); err != nil {
		return fmt.Errorf("chmod data directory: %w", err)
	}

	password := os.Getenv("DRONEBL_RSYNC_PASSWORD")
	if password == "" {
		password = os.Getenv("RSYNC_PASSWORD")
	}
	if password == "" {
		return ErrMissingPassword
	}

	if err := cleanFetchScratch(opts.DataDir); err != nil {
		return err
	}
	stageDir, err := os.MkdirTemp(opts.DataDir, ".fetch-")
	if err != nil {
		return fmt.Errorf("create temporary fetch directory: %w", err)
	}
	if err := os.Chmod(stageDir, generatedDirMode); err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("chmod temporary fetch directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()

	runCtx := ctx
	cancel := func() {}
	if opts.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}
	defer cancel()

	rsyncBinary := opts.RsyncBinary
	if rsyncBinary == "" {
		rsyncBinary = "rsync"
	}
	args := []string{"-HaSz"}
	if opts.Timeout > 0 {
		args = append(args, fmt.Sprintf("--timeout=%d", max(1, int(opts.Timeout/time.Second))))
	}
	args = append(args, buildzoneRsyncURL(opts.RsyncURL), stageDir+"/")
	cmd := exec.CommandContext(runCtx, rsyncBinary, args...)
	cmd.Env = withRsyncPassword(os.Environ(), password)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync DroneBL buildzone: %w", err)
	}
	if err := promoteFetchedBuildzone(stageDir, opts.DataDir); err != nil {
		return err
	}
	if err := cleanFetchScratch(opts.DataDir); err != nil {
		return err
	}
	return nil
}

func withRsyncPassword(env []string, password string) []string {
	out := env[:0]
	for _, item := range env {
		if len(item) >= len("RSYNC_PASSWORD=") && item[:len("RSYNC_PASSWORD=")] == "RSYNC_PASSWORD=" {
			continue
		}
		out = append(out, item)
	}
	return append(out, "RSYNC_PASSWORD="+password)
}

func buildzoneRsyncURL(raw string) string {
	trimmed := strings.TrimRight(raw, "/")
	if trimmed == "" {
		return raw
	}
	// Catalog values normally point at the module root; direct buildzone paths
	// are accepted for operator overrides and tests.
	if trimmed == "buildzone" || strings.HasSuffix(trimmed, "/buildzone") {
		return trimmed
	}
	return trimmed + "/buildzone"
}

func promoteFetchedBuildzone(stageDir, dataDir string) error {
	src := filepath.Join(stageDir, "buildzone")
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat fetched DroneBL buildzone: %w", err)
	}
	tmp, err := os.CreateTemp(dataDir, ".buildzone.*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary buildzone: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary buildzone: %w", err)
	}
	if err := os.Remove(tmpPath); err != nil {
		return fmt.Errorf("remove temporary buildzone placeholder: %w", err)
	}
	if err := os.Rename(src, tmpPath); err != nil {
		return fmt.Errorf("stage fetched DroneBL buildzone: %w", err)
	}
	if err := os.Chmod(tmpPath, generatedFileMode); err != nil {
		return fmt.Errorf("chmod fetched DroneBL buildzone: %w", err)
	}
	if !info.ModTime().IsZero() {
		if err := os.Chtimes(tmpPath, info.ModTime(), info.ModTime()); err != nil {
			return fmt.Errorf("preserve DroneBL buildzone mtime: %w", err)
		}
	}
	if err := os.Rename(tmpPath, filepath.Join(dataDir, "buildzone")); err != nil {
		return fmt.Errorf("install fetched DroneBL buildzone: %w", err)
	}
	cleanup = false
	return nil
}

func cleanFetchScratch(dataDir string) error {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return fmt.Errorf("read DroneBL fetch directory: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "buildzone" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dataDir, name)); err != nil {
			return fmt.Errorf("remove stale DroneBL fetch path %s: %w", name, err)
		}
	}
	return nil
}
