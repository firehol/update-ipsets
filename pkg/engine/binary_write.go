package engine

import (
	"os"
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

func writeBinaryPath(path string, set *iprange.IPSet, mod time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), generatedDirMode); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	closed := false
	closeTmp := func() {
		if !closed {
			_ = tmp.Close()
			closed = true
		}
	}
	defer func() {
		closeTmp()
		_ = os.Remove(tmpName)
	}()
	if err := iprange.WriteBinary(tmp, set); err != nil {
		closeTmp()
		return err
	}
	if err := tmp.Chmod(generatedFileMode); err != nil {
		closeTmp()
		return err
	}
	if err := tmp.Sync(); err != nil {
		closeTmp()
		return err
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return err
	}
	closed = true
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return touchFileAt(path, mod)
}
