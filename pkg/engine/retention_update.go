package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

type retentionUpdatePaths struct {
	dir    string
	newDir string
}

type retentionUpdateDiff struct {
	newSet  *iprange.IPSet
	added   uint64
	removed uint64
}

type retentionReconcileResult struct {
	cohorts        map[int64]uint64
	currentBuckets map[int]uint64
	incomplete     int
}

func (e *Engine) updateRetention(ctx context.Context, name string, previous, current *iprange.IPSet, updatedAt time.Time) error {
	ctx = nonNilContext(ctx)
	paths, err := e.prepareRetentionUpdatePaths(name)
	if err != nil {
		return err
	}

	updatedAtUnix := updatedAt.UTC().Unix()
	diff, err := e.retentionDiffFromSources(ctx, name, previous, current)
	if err != nil {
		return err
	}
	return e.updateRetentionWithDiff(ctx, name, paths, diff, current, updatedAt, updatedAtUnix)
}

func (e *Engine) updateRetentionFromDiff(ctx context.Context, name string, diff retentionUpdateDiff, current *iprange.IPSet, updatedAt time.Time) error {
	ctx = nonNilContext(ctx)
	paths, err := e.prepareRetentionUpdatePaths(name)
	if err != nil {
		return err
	}
	return e.updateRetentionWithDiff(ctx, name, paths, diff, current, updatedAt, updatedAt.UTC().Unix())
}

func (e *Engine) updateRetentionWithDiff(ctx context.Context, name string, paths retentionUpdatePaths, diff retentionUpdateDiff, current *iprange.IPSet, updatedAt time.Time, updatedAtUnix int64) error {
	if err := e.recordRetentionDelta(name, paths, diff, updatedAt, updatedAtUnix); err != nil {
		return err
	}
	if err := ensureCSVHeader(filepath.Join(paths.dir, "retention.csv"), "date_removed,date_added,hours,ips\n"); err != nil {
		return err
	}

	started := e.retentionStartedAt(name, updatedAtUnix)
	past := e.retentionPastFromRuntime(name, started)
	if diff.removed == 0 {
		return e.refreshRetentionWithoutRemovals(ctx, name, paths, started, updatedAtUnix, past)
	}

	result, err := e.reconcileRetentionCohorts(ctx, name, paths, started, updatedAtUnix, current, past)
	if err != nil {
		return err
	}
	e.replaceRetentionCohorts(name, result.cohorts)
	return writeRetentionOutputs(paths.dir, name, started, updatedAtUnix, result.incomplete, past, result.currentBuckets, result.cohorts)
}

func (e *Engine) prepareRetentionUpdatePaths(name string) (retentionUpdatePaths, error) {
	dir := filepath.Join(e.runtime.LibDir, name)
	paths := retentionUpdatePaths{
		dir:    dir,
		newDir: filepath.Join(dir, "new"),
	}
	if err := os.MkdirAll(paths.newDir, generatedDirMode); err != nil {
		return retentionUpdatePaths{}, err
	}
	return paths, nil
}

func retentionDiff(previous, current *iprange.IPSet) retentionUpdateDiff {
	newSet := iprange.Exclude(current, previous)
	removedSet := iprange.Exclude(previous, current)
	return retentionUpdateDiff{
		newSet:  newSet,
		added:   newSet.UniqueCount(),
		removed: removedSet.UniqueCount(),
	}
}

func (e *Engine) retentionDiffFromSources(ctx context.Context, name string, previous, current iprange.RangeSource) (retentionUpdateDiff, error) {
	ctx = nonNilContext(ctx)
	newSet, err := collectIter(ctx, name+"_new", iprange.ExcludeIter(current, previous))
	if err != nil {
		return retentionUpdateDiff{}, err
	}
	if err := checkFileSetErr(current, name, e.logger); err != nil {
		return retentionUpdateDiff{}, err
	}
	if err := checkFileSetErr(previous, name, e.logger); err != nil {
		return retentionUpdateDiff{}, err
	}

	removed, err := countUniqueIter(ctx, name+"_removed", iprange.ExcludeIter(previous, current))
	if err != nil {
		return retentionUpdateDiff{}, err
	}
	if err := checkFileSetErr(previous, name, e.logger); err != nil {
		return retentionUpdateDiff{}, err
	}
	if err := checkFileSetErr(current, name, e.logger); err != nil {
		return retentionUpdateDiff{}, err
	}
	return retentionUpdateDiff{
		newSet:  newSet,
		added:   newSet.UniqueCount(),
		removed: removed,
	}, nil
}

func (e *Engine) recordRetentionDelta(name string, paths retentionUpdatePaths, diff retentionUpdateDiff, updatedAt time.Time, updatedAtUnix int64) error {
	if diff.added > 0 || diff.removed > 0 {
		if err := normalizeChangesetLedgerHeader(e.runtime.LibDir, filepath.Join(name, "changesets.csv")); err != nil {
			return err
		}
		if err := appendCSV(filepath.Join(paths.dir, "changesets.csv"), changesetLedgerHeader,
			fmt.Sprintf("%d,%d,%d\n", updatedAtUnix, diff.added, diff.removed)); err != nil {
			return err
		}
		e.observeChangesetPoint(name, ChangesetPoint{
			Timestamp: updatedAtUnix,
			Added:     diff.added,
			Removed:   diff.removed,
		})
	}
	if diff.added == 0 {
		return nil
	}
	if err := writeBinaryPath(filepath.Join(paths.newDir, fmt.Sprintf("%d", updatedAtUnix)), diff.newSet, updatedAt); err != nil {
		return err
	}
	e.observeRetentionCohort(name, updatedAtUnix, diff.added)
	return nil
}

func (e *Engine) retentionStartedAt(name string, fallback int64) int64 {
	started := e.state.Entry(name).Snapshot().StartedDate
	if started == 0 {
		return fallback
	}
	return started
}

func (e *Engine) refreshRetentionWithoutRemovals(ctx context.Context, name string, paths retentionUpdatePaths, started, updatedAt int64, past map[int]uint64) error {
	cohorts := e.retentionCohortsFromRuntime(ctx, name)
	currentBuckets, incomplete := buildCurrentRetentionBuckets(cohorts, updatedAt, started)
	return writeRetentionOutputs(paths.dir, name, started, updatedAt, incomplete, past, currentBuckets, cohorts)
}

func (e *Engine) reconcileRetentionCohorts(ctx context.Context, name string, paths retentionUpdatePaths, started, updatedAt int64, current *iprange.IPSet, past map[int]uint64) (retentionReconcileResult, error) {
	result := retentionReconcileResult{
		cohorts:        make(map[int64]uint64),
		currentBuckets: map[int]uint64{},
	}
	files, err := os.ReadDir(paths.newDir)
	if err != nil {
		return retentionReconcileResult{}, err
	}
	for _, entry := range files {
		if shouldSkipRetentionCohortEntry(entry) {
			continue
		}
		addedAt, ok := e.retentionCohortTimestamp(name, entry.Name())
		if !ok {
			continue
		}
		if err := e.reconcileRetentionCohort(ctx, name, paths, started, updatedAt, current, past, entry.Name(), addedAt, &result); err != nil {
			return retentionReconcileResult{}, err
		}
	}
	return result, nil
}

func shouldSkipRetentionCohortEntry(entry os.DirEntry) bool {
	return entry.IsDir() || isIgnoredRetentionSnapshotName(entry.Name())
}

func (e *Engine) retentionCohortTimestamp(name, baseName string) (int64, bool) {
	tsStr := strings.TrimSuffix(baseName, ".set")
	addedAt, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		e.logger.Warn("retention: skipping malformed filename", "source", name, "file", baseName, "error", err)
		return 0, false
	}
	return addedAt, true
}

func (e *Engine) reconcileRetentionCohort(ctx context.Context, name string, paths retentionUpdatePaths, started, updatedAt int64, current *iprange.IPSet, past map[int]uint64, baseName string, addedAt int64, result *retentionReconcileResult) error {
	path := filepath.Join(paths.newDir, baseName)
	oldSource, err := openRetentionCohortSet(ctx, baseName, e.runtime.LibDir, filepath.Join(name, "new", baseName), path)
	if err != nil {
		return err
	}
	defer func() { _ = oldSource.Close() }()

	still, err := collectIter(ctx, baseName+"_still", iprange.IntersectIter(oldSource.RangeSource, current))
	if err != nil {
		return err
	}
	if err := checkFileSetErr(oldSource.RangeSource, name, e.logger); err != nil {
		return err
	}
	stillCount := still.UniqueCount()
	oldCount := oldSource.UniqueIPs()
	var removedCount uint64
	if oldCount > stillCount {
		removedCount = oldCount - stillCount
	}
	hours := retentionHours(updatedAt, addedAt)
	if removedCount > 0 {
		if err := e.recordRetentionRemoval(name, paths.dir, started, updatedAt, addedAt, hours, removedCount, past); err != nil {
			return err
		}
	}
	if stillCount == 0 {
		return removeRetentionCohortFile(path)
	}

	result.cohorts[addedAt] = stillCount
	result.currentBuckets[hours] += stillCount
	if addedAt <= started {
		result.incomplete = 1
	}
	if removedCount == 0 {
		return nil
	}
	return writeBinaryPath(path, still, time.Unix(addedAt, 0).UTC())
}

func retentionHours(updatedAt, addedAt int64) int {
	return int((updatedAt + 1800 - addedAt) / 3600)
}

func (e *Engine) recordRetentionRemoval(name, dir string, started, updatedAt, addedAt int64, hours int, removed uint64, past map[int]uint64) error {
	if err := appendCSV(filepath.Join(dir, "retention.csv"), "date_removed,date_added,hours,ips\n",
		fmt.Sprintf("%d,%d,%d,%d\n", updatedAt, addedAt, hours, removed)); err != nil {
		return err
	}
	if addedAt <= started {
		return nil
	}
	past[hours] += removed
	e.observeRetentionPast(name, started, hours, removed)
	return nil
}

func removeRetentionCohortFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func writeRetentionOutputs(dir, name string, started, updatedAt int64, incomplete int, past, current map[int]uint64, cohorts map[int64]uint64) error {
	retention := buildRetentionDataFromBuckets(name, started, updatedAt, incomplete, past, current)
	data, err := jsonMarshalTabIndent(retention)
	if err != nil {
		return err
	}
	if err := writeRetentionCohortIndex(filepath.Join(dir, "retention_cohorts.csv"), cohorts); err != nil {
		return err
	}
	if err := writeRetentionHistogramCache(filepath.Join(dir, "histogram"), retention); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "retention.json"), append(data, '\n'), generatedFileMode)
}
