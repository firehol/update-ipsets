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

type retentionReconcileStats struct {
	totalEntries     int
	scannedEntries   int
	skippedEntries   int
	malformedEntries int
	processedCohorts int
	rewrittenCohorts int
	deletedCohorts   int
	inputIPs         uint64
	keptIPs          uint64
	removedIPs       uint64
}

type retentionCohortUpdate struct {
	oldIPs     uint64
	keptIPs    uint64
	removedIPs uint64
	rewritten  bool
	deleted    bool
}

const retentionReconcileCompareBatchSize = 256

type retentionCohortCandidate struct {
	baseName string
	path     string
	addedAt  int64
}

type retentionOpenCohort struct {
	candidate retentionCohortCandidate
	source    *closableSource
}

func (s *retentionReconcileStats) record(update retentionCohortUpdate) {
	if s == nil {
		return
	}
	s.processedCohorts++
	if update.rewritten {
		s.rewrittenCohorts++
	}
	if update.deleted {
		s.deletedCohorts++
	}
	s.inputIPs += update.oldIPs
	s.keptIPs += update.keptIPs
	s.removedIPs += update.removedIPs
}

func (s retentionReconcileStats) progressCounters() map[string]int64 {
	return map[string]int64{
		"total_entries":     int64(s.totalEntries),
		"scanned_entries":   int64(s.scannedEntries),
		"skipped_entries":   int64(s.skippedEntries),
		"malformed_entries": int64(s.malformedEntries),
		"processed_cohorts": int64(s.processedCohorts),
		"rewritten_cohorts": int64(s.rewrittenCohorts),
		"deleted_cohorts":   int64(s.deletedCohorts),
		"input_ips":         int64Clamp(s.inputIPs),
		"kept_ips":          int64Clamp(s.keptIPs),
		"removed_ips":       int64Clamp(s.removedIPs),
	}
}

func shouldReportRetentionProgress(idx, total int, _ retentionReconcileStats) bool {
	processedEntries := idx + 1
	return processedEntries == total || processedEntries%256 == 0
}

func int64Clamp(value uint64) int64 {
	const maxInt64 = uint64(1<<63 - 1)
	if value > maxInt64 {
		return int64(maxInt64)
	}
	return int64(value)
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
	past := e.retentionPastFromRuntimeContext(ctx, name, started)
	if diff.removed == 0 {
		return e.refreshRetentionWithoutRemovals(ctx, name, paths, started, updatedAtUnix, past)
	}

	result, err := e.reconcileRetentionCohorts(ctx, name, paths, started, updatedAtUnix, current, past)
	if err != nil {
		return err
	}
	e.replaceRetentionCohorts(name, result.cohorts)
	return writeRetentionOutputs(ctx, paths.dir, name, started, updatedAtUnix, result.incomplete, past, result.currentBuckets, result.cohorts)
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
	newSet, err := iprange.ExcludeSourcesContext(ctx, name+"_new", current, previous)
	if err != nil {
		return retentionUpdateDiff{}, err
	}
	if err := checkFileSetErr(current, name, e.logger); err != nil {
		return retentionUpdateDiff{}, err
	}
	if err := checkFileSetErr(previous, name, e.logger); err != nil {
		return retentionUpdateDiff{}, err
	}

	removed, err := iprange.ExcludeCountContext(ctx, previous, current)
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

func (e *Engine) refreshRetentionWithoutRemovals(ctx context.Context, name string, paths retentionUpdatePaths, started, updatedAt int64, past map[int]uint64) (err error) {
	opStarted := time.Now()
	cohorts := e.retentionCohortsFromRuntime(ctx, name)
	currentBuckets, incomplete := buildCurrentRetentionBuckets(cohorts, updatedAt, started)
	defer func() {
		elapsedMS := telemetryDurationMillis(time.Since(opStarted))
		status := "ok"
		if err != nil {
			status = "error"
		}
		e.observeRunCounter("retention.refresh_without_removals.cohorts", int64(len(cohorts)), 0)
		e.logger.Info("retention refresh summary",
			"source", name,
			"status", status,
			"work_unit", "cohorts",
			"work_size", len(cohorts),
			"work_completed", len(cohorts),
			"completion_pct", completionPct(int64(len(cohorts)), int64(len(cohorts))),
			"rate_per_second", ratePerSecond(int64(len(cohorts)), elapsedMS),
			"cohorts", len(cohorts),
			"current_buckets", len(currentBuckets),
			"incomplete", incomplete,
			"elapsed_ms", elapsedMS,
		)
	}()
	return writeRetentionOutputs(ctx, paths.dir, name, started, updatedAt, incomplete, past, currentBuckets, cohorts)
}

func (e *Engine) reconcileRetentionCohorts(ctx context.Context, name string, paths retentionUpdatePaths, started, updatedAt int64, current *iprange.IPSet, past map[int]uint64) (result retentionReconcileResult, err error) {
	opStarted := time.Now()
	stats := retentionReconcileStats{}
	result = retentionReconcileResult{
		cohorts:        make(map[int64]uint64),
		currentBuckets: map[int]uint64{},
	}
	currentSource := iprange.CompareSource{Name: name, Source: current}
	files, err := os.ReadDir(paths.newDir)
	if err != nil {
		return retentionReconcileResult{}, err
	}
	stats.totalEntries = len(files)
	batch := make([]retentionCohortCandidate, 0, retentionReconcileCompareBatchSize)
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		updates, err := e.reconcileRetentionCohortBatch(ctx, name, paths, started, updatedAt, current, currentSource, past, batch, &result)
		if err != nil {
			return err
		}
		for _, update := range updates {
			stats.record(update)
		}
		batch = batch[:0]
		return nil
	}
	progress := e.beginActiveOperation("retention.reconcile_cohorts", name, "scan", "files", int64(len(files)))
	defer func() {
		elapsedMS := telemetryDurationMillis(time.Since(opStarted))
		if progress != nil {
			progress.Update(int64(stats.scannedEntries), int64(stats.totalEntries), stats.progressCounters())
			progress.Finish()
		}
		status := "ok"
		if err != nil {
			status = "error"
		}
		e.observeRunCounter("retention.reconcile.total_entries", int64(stats.totalEntries), 0)
		e.observeRunCounter("retention.reconcile.scanned_entries", int64(stats.scannedEntries), 0)
		e.observeRunCounter("retention.reconcile.skipped_entries", int64(stats.skippedEntries), 0)
		e.observeRunCounter("retention.reconcile.malformed_entries", int64(stats.malformedEntries), 0)
		e.observeRunCounter("retention.reconcile.cohorts_processed", int64(stats.processedCohorts), 0)
		e.observeRunCounter("retention.reconcile.cohorts_rewritten", int64(stats.rewrittenCohorts), 0)
		e.observeRunCounter("retention.reconcile.cohorts_deleted", int64(stats.deletedCohorts), 0)
		e.observeRunCounter("retention.reconcile.input_ips", int64Clamp(stats.inputIPs), 0)
		e.observeRunCounter("retention.reconcile.kept_ips", int64Clamp(stats.keptIPs), 0)
		e.observeRunCounter("retention.reconcile.removed_ips", int64Clamp(stats.removedIPs), 0)
		e.logger.Info("retention reconcile summary",
			"source", name,
			"status", status,
			"work_unit", "files",
			"work_size", stats.totalEntries,
			"work_completed", stats.scannedEntries,
			"completion_pct", completionPct(int64(stats.scannedEntries), int64(stats.totalEntries)),
			"rate_per_second", ratePerSecond(int64(stats.scannedEntries), elapsedMS),
			"total_entries", stats.totalEntries,
			"scanned_entries", stats.scannedEntries,
			"skipped_entries", stats.skippedEntries,
			"malformed_entries", stats.malformedEntries,
			"processed_cohorts", stats.processedCohorts,
			"rewritten_cohorts", stats.rewrittenCohorts,
			"deleted_cohorts", stats.deletedCohorts,
			"input_ips", stats.inputIPs,
			"kept_ips", stats.keptIPs,
			"removed_ips", stats.removedIPs,
			"elapsed_ms", elapsedMS,
		)
	}()
	for idx, entry := range files {
		if err := contextErr(ctx); err != nil {
			return retentionReconcileResult{}, err
		}
		stats.scannedEntries = idx + 1
		if shouldSkipRetentionCohortEntry(entry) {
			stats.skippedEntries++
			if progress != nil && shouldReportRetentionProgress(idx, len(files), stats) {
				progress.Update(int64(idx+1), int64(len(files)), stats.progressCounters())
			}
			continue
		}
		addedAt, ok := e.retentionCohortTimestamp(name, entry.Name())
		if !ok {
			stats.skippedEntries++
			stats.malformedEntries++
			if progress != nil && shouldReportRetentionProgress(idx, len(files), stats) {
				progress.Update(int64(idx+1), int64(len(files)), stats.progressCounters())
			}
			continue
		}
		batch = append(batch, retentionCohortCandidate{
			baseName: entry.Name(),
			path:     filepath.Join(paths.newDir, entry.Name()),
			addedAt:  addedAt,
		})
		if len(batch) >= retentionReconcileCompareBatchSize {
			if err := flushBatch(); err != nil {
				return retentionReconcileResult{}, err
			}
		}
		if progress != nil && shouldReportRetentionProgress(idx, len(files), stats) {
			progress.Update(int64(idx+1), int64(len(files)), stats.progressCounters())
		}
	}
	if err := flushBatch(); err != nil {
		return retentionReconcileResult{}, err
	}
	return result, nil
}

func (e *Engine) reconcileRetentionCohortBatch(ctx context.Context, name string, paths retentionUpdatePaths, started, updatedAt int64, current *iprange.IPSet, currentSource iprange.CompareSource, past map[int]uint64, batch []retentionCohortCandidate, result *retentionReconcileResult) ([]retentionCohortUpdate, error) {
	if len(batch) == 0 {
		return nil, nil
	}
	opened := make([]retentionOpenCohort, 0, len(batch))
	sources := make([]iprange.CompareSource, 0, len(batch)+1)
	pairs := make([]iprange.ComparePair, 0, len(batch))
	sources = append(sources, currentSource)
	for _, candidate := range batch {
		oldSource, err := openRetentionCohortSet(ctx, candidate.baseName, e.runtime.LibDir, filepath.Join(name, "new", candidate.baseName), candidate.path)
		if err != nil {
			_ = closeRetentionOpenCohorts(opened)
			return nil, err
		}
		opened = append(opened, retentionOpenCohort{
			candidate: candidate,
			source:    oldSource,
		})
		sources = append(sources, iprange.CompareSource{Name: candidate.baseName, Source: oldSource.RangeSource})
		pairs = append(pairs, iprange.ComparePair{Left: 0, Right: len(sources) - 1})
	}

	rows, err := iprange.CompareSourcePairs(ctx, sources, pairs)
	if err != nil {
		_ = closeRetentionOpenCohorts(opened)
		return nil, err
	}
	if len(rows) != len(opened) {
		_ = closeRetentionOpenCohorts(opened)
		return nil, fmt.Errorf("retention: compare pairs returned %d rows for %d cohorts", len(rows), len(opened))
	}

	updates := make([]retentionCohortUpdate, 0, len(opened))
	for i, cohort := range opened {
		update, err := e.applyRetentionCohortCompare(ctx, name, paths, started, updatedAt, current, cohort.source, cohort.candidate.baseName, cohort.candidate.path, cohort.candidate.addedAt, rows[i], past, result)
		if err != nil {
			_ = closeRetentionOpenCohorts(opened[i+1:])
			return nil, err
		}
		updates = append(updates, update)
	}
	return updates, nil
}

func closeRetentionOpenCohorts(opened []retentionOpenCohort) error {
	var errs []error
	for _, cohort := range opened {
		if cohort.source == nil {
			continue
		}
		if err := cohort.source.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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

func (e *Engine) applyRetentionCohortCompare(ctx context.Context, name string, paths retentionUpdatePaths, started, updatedAt int64, current *iprange.IPSet, oldSource *closableSource, baseName, path string, addedAt int64, row iprange.CompareRow, past map[int]uint64, result *retentionReconcileResult) (retentionCohortUpdate, error) {
	oldCount := row.Unique2
	stillCount := row.CommonIPs
	if stillCount > oldCount {
		_ = oldSource.Close()
		return retentionCohortUpdate{}, fmt.Errorf("retention: compare-next common count exceeds cohort size for %s: common=%d cohort=%d", baseName, stillCount, oldCount)
	}
	var removedCount uint64
	if oldCount > stillCount {
		removedCount = oldCount - stillCount
	}
	update := retentionCohortUpdate{
		oldIPs:     oldCount,
		keptIPs:    stillCount,
		removedIPs: removedCount,
	}
	hours := retentionHours(updatedAt, addedAt)
	if oldCount == 0 {
		if err := oldSource.Close(); err != nil {
			return retentionCohortUpdate{}, err
		}
		update.deleted = true
		return update, removeRetentionCohortFile(path)
	}
	if removedCount == 0 {
		if err := oldSource.Close(); err != nil {
			return retentionCohortUpdate{}, err
		}
		result.cohorts[addedAt] = stillCount
		result.currentBuckets[hours] += stillCount
		if addedAt <= started {
			result.incomplete = 1
		}
		return update, nil
	}

	still, err := iprange.IntersectSourcesContext(ctx, baseName+"_still", oldSource.RangeSource, current)
	if err != nil {
		_ = oldSource.Close()
		return retentionCohortUpdate{}, err
	}
	if err := checkFileSetErr(oldSource.RangeSource, name, e.logger); err != nil {
		_ = oldSource.Close()
		return retentionCohortUpdate{}, err
	}
	materializedCount := still.UniqueCount()
	if materializedCount != row.CommonIPs {
		_ = oldSource.Close()
		return retentionCohortUpdate{}, fmt.Errorf("retention: compare-next common count mismatch for %s: row=%d materialized=%d", baseName, row.CommonIPs, materializedCount)
	}
	removedCount = 0
	if oldCount > materializedCount {
		removedCount = oldCount - materializedCount
	}
	update.keptIPs = materializedCount
	update.removedIPs = removedCount
	if err := oldSource.Close(); err != nil {
		return retentionCohortUpdate{}, err
	}
	if removedCount > 0 {
		if err := e.recordRetentionRemoval(name, paths.dir, started, updatedAt, addedAt, hours, removedCount, past); err != nil {
			return retentionCohortUpdate{}, err
		}
	}
	if stillCount == 0 {
		update.deleted = true
		return update, removeRetentionCohortFile(path)
	}

	result.cohorts[addedAt] = stillCount
	result.currentBuckets[hours] += stillCount
	if addedAt <= started {
		result.incomplete = 1
	}
	if removedCount == 0 {
		return update, nil
	}
	update.rewritten = true
	return update, writeBinaryPath(path, still, time.Unix(addedAt, 0).UTC())
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

func writeRetentionOutputs(ctx context.Context, dir, name string, started, updatedAt int64, incomplete int, past, current map[int]uint64, cohorts map[int64]uint64) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	retention := buildRetentionDataFromBuckets(name, started, updatedAt, incomplete, past, current)
	data, err := jsonMarshalTabIndent(retention)
	if err != nil {
		return err
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := writeRetentionCohortIndex(filepath.Join(dir, "retention_cohorts.csv"), cohorts); err != nil {
		return err
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := writeRetentionHistogramCache(filepath.Join(dir, "histogram"), retention); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "retention.json"), append(data, '\n'), generatedFileMode)
}
