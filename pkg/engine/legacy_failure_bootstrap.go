package engine

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func (e *Engine) bootstrapLegacyFailureStarts() error {
	if e == nil || e.state == nil {
		return nil
	}

	legacyPath := e.legacyImportedCachePath()
	if legacyPath == "" {
		return nil
	}

	legacyState, err := cache.Load(legacyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return nil
		}
		e.logger.Warn("legacy failure bootstrap: failed to load imported cache", "path", legacyPath, "error", err)
		return nil
	}

	updated := 0
	for name, legacy := range legacyState.SnapshotEntries() {
		if legacy.DownloadFailures <= 0 || legacy.CheckedDate <= 0 {
			continue
		}

		current := e.state.EntrySnapshot(name)
		if current == nil || current.DownloadFailures <= 0 {
			continue
		}
		if recoveredAfterLegacyFailure(current, legacy.CheckedDate) {
			continue
		}
		if current.FailureStartedDate > 0 && current.FailureStartedDate <= legacy.CheckedDate {
			continue
		}

		if e.state.Entry(name).RecordLegacyFailureStart(legacy.CheckedDate) {
			updated++
		}
	}

	if updated == 0 {
		return nil
	}
	if err := cache.Save(e.cachePath, e.state); err != nil {
		e.logger.Warn("legacy failure bootstrap: failed to persist reconstructed failure dates", "count", updated, "path", e.cachePath, "error", err)
		return nil
	}
	e.logger.Info("bootstrapped failure_started_date from imported legacy cache", "count", updated, "path", legacyPath)
	return nil
}

func (e *Engine) legacyImportedCachePath() string {
	if e == nil || e.runtime.BaseDir == "" {
		return ""
	}
	root := filepath.Dir(e.runtime.BaseDir)
	for _, path := range []string{
		filepath.Join(root, "import-bash-version", "merged-cache.json"),
		filepath.Join(root, "import-d1", "merged-cache.json"),
	} {
		if fileExists(path) {
			return path
		}
	}
	return filepath.Join(root, "import-bash-version", "merged-cache.json")
}

func recoveredAfterLegacyFailure(entry *cache.Entry, legacyCheckedDate int64) bool {
	if entry == nil || legacyCheckedDate <= 0 {
		return false
	}
	return entry.SourceDate > legacyCheckedDate || entry.ProcessedDate > legacyCheckedDate
}
