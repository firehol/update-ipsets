package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type FileLock struct {
	path string
	file *os.File
}

func acquireLock(path string) (*FileLock, error) {
	if path == "" {
		return nil, fmt.Errorf("lock file path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, generatedFileMode)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(generatedFileMode); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("another update-ipsets instance already holds %s", path)
		}
		return nil, err
	}
	// Write PID so operators can identify the lock holder.
	_ = file.Truncate(0)
	_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
	return &FileLock{path: path, file: file}, nil
}

func (l *FileLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	// Truncate the PID before releasing so stale files are clearly empty.
	_ = l.file.Truncate(0)
	errUnlock := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	errClose := l.file.Close()
	errRemove := os.Remove(l.path)
	l.file = nil
	if errUnlock != nil {
		return errUnlock
	}
	if errClose != nil {
		return errClose
	}
	// Ignore ENOENT from Remove (already cleaned up).
	if errRemove != nil && !os.IsNotExist(errRemove) {
		return errRemove
	}
	return nil
}

func (l *FileLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
