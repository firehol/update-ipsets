package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const stalePublishStageMinAge = 5 * time.Minute

type StalePublishStageCleanupResult struct {
	WebRemoved    int `json:"web_removed"`
	EntityRemoved int `json:"entity_removed"`
}

func (r StalePublishStageCleanupResult) TotalRemoved() int {
	return r.WebRemoved + r.EntityRemoved
}

func (e *Engine) CleanupStalePublishStages() (StalePublishStageCleanupResult, error) {
	if e == nil {
		return StalePublishStageCleanupResult{}, nil
	}
	cutoff := time.Now().UTC().Add(-stalePublishStageMinAge)
	if e.now != nil {
		cutoff = e.now().UTC().Add(-stalePublishStageMinAge)
	}
	return e.CleanupPublishStagesBefore(cutoff)
}

func (e *Engine) CleanupPublishStagesBefore(cutoff time.Time) (StalePublishStageCleanupResult, error) {
	if e == nil {
		return StalePublishStageCleanupResult{}, nil
	}
	cutoff = cutoff.UTC()
	webRemoved, webErr := cleanupStalePublishStageDirs(e.outputDir(), webPublishStagePrefix, cutoff)
	entityRemoved, entityErr := cleanupStalePublishStageDirs(e.entitiesDir(), entityPublishStagePrefix, cutoff)
	return StalePublishStageCleanupResult{
		WebRemoved:    webRemoved,
		EntityRemoved: entityRemoved,
	}, errors.Join(webErr, entityErr)
}

func cleanupStalePublishStageDirs(root, prefix string, cutoff time.Time) (int, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("list stale publish stages in %s: %w", root, err)
	}
	var errs []error
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			errs = append(errs, fmt.Errorf("stat stale publish stage %s: %w", filepath.Join(root, entry.Name()), err))
			continue
		}
		if !cutoff.IsZero() && info.ModTime().After(cutoff) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, fmt.Errorf("remove stale publish stage %s: %w", path, err))
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}
