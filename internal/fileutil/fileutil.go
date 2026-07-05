// Package fileutil provides shared file utility functions used across
// multiple packages to avoid duplication.
package fileutil

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
)

const (
	// GeneratedDirMode is the default mode for daemon-generated directories.
	GeneratedDirMode os.FileMode = 0o700
	// GeneratedFileMode is the default mode for daemon-generated non-executable files.
	GeneratedFileMode os.FileMode = 0o600
)

// Exists returns true if the file at path exists and is not a directory.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadFileUnderRoot reads a regular file addressed relative to rootDir without
// allowing absolute paths, parent traversal, or symlink escapes outside rootDir.
func ReadFileUnderRoot(rootDir, relPath string) ([]byte, error) {
	file, err := OpenFileUnderRoot(rootDir, relPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

// OpenFileUnderRoot opens a file addressed relative to rootDir without allowing
// absolute paths, parent traversal, or symlink escapes outside rootDir.
func OpenFileUnderRoot(rootDir, relPath string) (*os.File, error) {
	return OpenFileWithFlagsUnderRoot(rootDir, relPath, os.O_RDONLY, 0)
}

// OpenFileWithFlagsUnderRoot opens a file addressed relative to rootDir with
// caller-supplied flags without allowing absolute paths, parent traversal, or
// symlink escapes outside rootDir.
func OpenFileWithFlagsUnderRoot(rootDir, relPath string, flag int, perm os.FileMode) (*os.File, error) {
	cleanRel, err := cleanRootRelativePath(relPath)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.OpenFile(cleanRel, flag, perm)
}

func cleanRootRelativePath(relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("relative path is required")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute path %q is not allowed", relPath)
	}
	clean := filepath.Clean(relPath)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes root", relPath)
	}
	return clean, nil
}

// WriteAtomic writes data to path atomically by writing to a temporary file
// in the same directory and then renaming it into place.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	return writeAtomic(path, data, mode, true)
}

// WriteAtomicNoSync is the same atomic temp-file replacement, but skips
// syncing the temporary file before rename. It is intended for staging
// directories whose contents are regenerated and then published as a batch.
func WriteAtomicNoSync(path string, data []byte, mode os.FileMode) error {
	return writeAtomic(path, data, mode, false)
}

// WriteAtomicWithWriter writes data produced by write to path atomically,
// returning the number of bytes written to the temporary file before rename.
func WriteAtomicWithWriter(path string, mode os.FileMode, write func(io.Writer) error) (int64, error) {
	return writeAtomicWithWriter(path, mode, true, write)
}

// WriteAtomicNoSyncWithWriter is the streaming equivalent of WriteAtomicNoSync.
func WriteAtomicNoSyncWithWriter(path string, mode os.FileMode, write func(io.Writer) error) (int64, error) {
	return writeAtomicWithWriter(path, mode, false, write)
}

func writeAtomic(path string, data []byte, mode os.FileMode, syncFile bool) error {
	started := time.Now()
	_, span := observability.Start(context.Background(), "file.write_atomic",
		observability.String("file.path", path),
		observability.Bool("file.sync", syncFile),
		observability.Int("file.bytes", len(data)),
	)
	var opErr error
	defer func() {
		attrs := []observability.Attr{
			observability.Bool("file.sync", syncFile),
		}
		observability.TryObserve("file.write_atomic", 1, int64(len(data)), time.Since(started), attrs...)
		observability.End(span, opErr)
	}()
	if opErr = os.MkdirAll(filepath.Dir(path), GeneratedDirMode); opErr != nil { // nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission - 0700 is the restrictive usable directory mode.
		return opErr
	}
	tmp, opErr := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if opErr != nil {
		return opErr
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, opErr = tmp.Write(data); opErr != nil {
		_ = tmp.Close()
		return opErr
	}
	if opErr = tmp.Chmod(mode); opErr != nil {
		_ = tmp.Close()
		return opErr
	}
	if syncFile {
		if opErr = tmp.Sync(); opErr != nil {
			_ = tmp.Close()
			return opErr
		}
	}
	if opErr = tmp.Close(); opErr != nil {
		return opErr
	}
	opErr = os.Rename(tmpName, path)
	return opErr
}

func writeAtomicWithWriter(path string, mode os.FileMode, syncFile bool, write func(io.Writer) error) (int64, error) {
	if write == nil {
		return 0, fmt.Errorf("write callback is required")
	}
	started := time.Now()
	_, span := observability.Start(context.Background(), "file.write_atomic",
		observability.String("file.path", path),
		observability.Bool("file.sync", syncFile),
	)
	var opErr error
	var written int64
	defer func() {
		attrs := []observability.Attr{
			observability.Bool("file.sync", syncFile),
		}
		observability.TryObserve("file.write_atomic", 1, written, time.Since(started), attrs...)
		observability.End(span, opErr)
	}()
	if opErr = os.MkdirAll(filepath.Dir(path), GeneratedDirMode); opErr != nil { // nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission - 0700 is the restrictive usable directory mode.
		return 0, opErr
	}
	tmp, opErr := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if opErr != nil {
		return 0, opErr
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	counter := &countingWriter{w: tmp}
	buffered := bufio.NewWriterSize(counter, 64*1024)
	if opErr = write(buffered); opErr != nil {
		written = counter.n
		_ = tmp.Close()
		return counter.n, opErr
	}
	if opErr = buffered.Flush(); opErr != nil {
		written = counter.n
		_ = tmp.Close()
		return counter.n, opErr
	}
	written = counter.n
	if opErr = tmp.Chmod(mode); opErr != nil {
		_ = tmp.Close()
		return written, opErr
	}
	if syncFile {
		if opErr = tmp.Sync(); opErr != nil {
			_ = tmp.Close()
			return written, opErr
		}
	}
	if opErr = tmp.Close(); opErr != nil {
		return written, opErr
	}
	opErr = os.Rename(tmpName, path)
	return written, opErr
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}
