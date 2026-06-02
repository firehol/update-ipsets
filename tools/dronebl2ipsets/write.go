package dronebl

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func WriteSourceFile(outputDir, filename string, set *RangeSet, mtime time.Time) error {
	if outputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	dst := filepath.Join(outputDir, filename)
	tmp, err := os.CreateTemp(outputDir, "."+filename+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := set.WriteCIDR(tmp); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", filename, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filename, err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", filename, err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("install %s: %w", filename, err)
	}
	cleanup = false
	if !mtime.IsZero() {
		if err := os.Chtimes(dst, mtime, mtime); err != nil {
			return fmt.Errorf("preserve mtime for %s: %w", filename, err)
		}
	}
	return nil
}
