package engine

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type cancelAfterRead struct {
	cancel context.CancelFunc
	data   []byte
	read   bool
}

func (r *cancelAfterRead) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	n := copy(p, r.data)
	r.cancel()
	return n, nil
}

func TestCopyFileViaNewTouchesIdenticalDestinationInPlace(t *testing.T) {
	root := t.TempDir()
	srcRoot := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	for _, dir := range []string{srcRoot, dstDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	body := []byte("1.2.3.4\n")
	srcRel := "sample.ipset"
	srcPath := filepath.Join(srcRoot, srcRel)
	if err := os.WriteFile(srcPath, body, 0o600); err != nil {
		t.Fatalf("WriteFile(src) error = %v", err)
	}
	sourceMod := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(srcPath, sourceMod, sourceMod); err != nil {
		t.Fatalf("Chtimes(src) error = %v", err)
	}

	dst := filepath.Join(dstDir, srcRel)
	if err := os.WriteFile(dst, body, 0o600); err != nil {
		t.Fatalf("WriteFile(dst) error = %v", err)
	}
	oldMod := sourceMod.Add(-time.Hour)
	if err := os.Chtimes(dst, oldMod, oldMod); err != nil {
		t.Fatalf("Chtimes(dst) error = %v", err)
	}
	before, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat(dst before) error = %v", err)
	}

	gotMod, err := copyFileViaNew(srcRoot, srcRel, dst, "")
	if err != nil {
		t.Fatalf("copyFileViaNew() error = %v", err)
	}
	if !gotMod.Equal(sourceMod) {
		t.Fatalf("copyFileViaNew() mod = %s, want %s", gotMod, sourceMod)
	}
	after, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat(dst after) error = %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("identical destination was replaced instead of touched in place")
	}
	if got := after.Mode().Perm(); got != generatedFileMode {
		t.Fatalf("dst mode = %04o, want %04o", got, generatedFileMode)
	}
	if got := after.ModTime().UTC(); !got.Equal(sourceMod) {
		t.Fatalf("dst mtime = %s, want %s", got, sourceMod)
	}
}

func TestCopyFileViaNewReplacesChangedDestination(t *testing.T) {
	root := t.TempDir()
	srcRoot := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	for _, dir := range []string{srcRoot, dstDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	srcRel := "sample.ipset"
	srcPath := filepath.Join(srcRoot, srcRel)
	if err := os.WriteFile(srcPath, []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(src) error = %v", err)
	}
	sourceMod := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	if err := os.Chtimes(srcPath, sourceMod, sourceMod); err != nil {
		t.Fatalf("Chtimes(src) error = %v", err)
	}

	dst := filepath.Join(dstDir, srcRel)
	if err := os.WriteFile(dst, []byte("5.6.7.8\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(dst) error = %v", err)
	}

	gotMod, err := copyFileViaNew(srcRoot, srcRel, dst, "")
	if err != nil {
		t.Fatalf("copyFileViaNew() error = %v", err)
	}
	if !gotMod.Equal(sourceMod) {
		t.Fatalf("copyFileViaNew() mod = %s, want %s", gotMod, sourceMod)
	}
	body, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(dst) error = %v", err)
	}
	if got, want := string(body), "1.2.3.4\n"; got != want {
		t.Fatalf("dst body = %q, want %q", got, want)
	}
	if got, want := fileMode(t, dst), generatedFileMode; got != want {
		t.Fatalf("dst mode = %04o, want %04o", got, want)
	}
	if leftovers, err := filepath.Glob(filepath.Join(dstDir, ".new-*")); err != nil {
		t.Fatalf("Glob(.new-*) error = %v", err)
	} else if len(leftovers) != 0 {
		t.Fatalf("temporary copy files left behind: %v", leftovers)
	}
}

func TestCopyWithContextStopsBeforeWritingAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	reader := &cancelAfterRead{cancel: cancel, data: []byte("payload")}
	var dst bytes.Buffer
	written, err := copyWithContext(ctx, &dst, reader)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copyWithContext() error = %v, want context.Canceled", err)
	}
	if written != 0 {
		t.Fatalf("copyWithContext() written = %d, want 0", written)
	}
	if dst.Len() != 0 {
		t.Fatalf("destination length = %d, want 0", dst.Len())
	}
}

func TestCopyFileViaNewContextCancelledBeforeCopyLeavesDestinationUntouched(t *testing.T) {
	root := t.TempDir()
	srcRoot := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	for _, dir := range []string{srcRoot, dstDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	srcRel := "sample.ipset"
	if err := os.WriteFile(filepath.Join(srcRoot, srcRel), []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(src) error = %v", err)
	}
	dst := filepath.Join(dstDir, srcRel)
	if err := os.WriteFile(dst, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(dst) error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := copyFileViaNewContext(ctx, srcRoot, srcRel, dst, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("copyFileViaNewContext() error = %v, want context.Canceled", err)
	}
	body, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(dst) error = %v", err)
	}
	if got, want := string(body), "old\n"; got != want {
		t.Fatalf("dst body = %q, want %q", got, want)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	return info.Mode().Perm()
}
