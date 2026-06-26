package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/iprange"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

var ensureRuntimeDirectoryHook func(string)

func (e *Engine) ensureDirectories() error {
	if e == nil {
		return nil
	}
	return ensureDirectoriesForRuntime(e.Runtime())
}

func ensureDirectoriesForRuntime(rt Runtime) error {
	dirs := []string{
		rt.BaseDir,
		rt.CacheDir,
		rt.LibDir,
		rt.HistoryDir,
		rt.ErrorsDir,
		rt.TmpDir,
	}
	if rt.WebDir != "" {
		dirs = append(dirs, rt.WebDir)
	}
	if rt.WebDirForIPSets != "" {
		dirs = append(dirs, rt.WebDirForIPSets)
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if ensureRuntimeDirectoryHook != nil {
			ensureRuntimeDirectoryHook(dir)
		}
		if err := os.MkdirAll(dir, generatedDirMode); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) processSource(ctx context.Context, src *config.Source, opts RunOptions, reason runreason.Reason) FeedProcessingResult {
	return e.processSourceWithSnapshot(ctx, e.operationSnapshot(), src, opts, reason)
}

func (e *Engine) processSourceWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source, opts RunOptions, reason runreason.Reason) FeedProcessingResult {
	if src == nil {
		return processingException(ProcessingExceptionInvalidInput, "nil source", fmt.Errorf("nil source"))
	}
	// asn and geoip sources are databases, not ipsets. They are
	// handled in the heavy block by processASNDatabases /
	// processGeoIPDatabases. Skip them here so the regular ipset
	// pipeline does not try to parse a binary archive as text.
	//
	// Sources with use:[bogons] still produce ipsets and fall through
	// to the normal pipeline; the bogon role only adds participation
	// in the bogon union.
	if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
		return processingOK("database source handled by heavy block", false)
	}
	return e.processConcreteSourceWithSnapshot(ctx, snap, src.Name, src, src.Output, opts, reason)
}

func (e *Engine) processConcreteSource(ctx context.Context, runName string, src *config.Source, output string, opts RunOptions, reason runreason.Reason) FeedProcessingResult {
	return e.processConcreteSourceWithSnapshot(ctx, e.operationSnapshot(), runName, src, output, opts, reason)
}

func (e *Engine) processConcreteSourceWithSnapshot(ctx context.Context, snap operationSnapshot, runName string, src *config.Source, output string, opts RunOptions, reason runreason.Reason) FeedProcessingResult {
	entry := e.state.Entry(runName)
	entry.ApplyProcessingSourceConfig(cache.ProcessingSourceConfigSnapshot{
		Name:              runName,
		Info:              src.Info,
		Category:          src.Category,
		Maintainer:        src.Maintainer,
		MaintainerURL:     src.MaintainerURL,
		Frequency:         src.Frequency,
		History:           src.History,
		Downloader:        src.Attributes["downloader"],
		DownloaderOptions: src.Attributes["downloader_options"],
		URL:               src.URL,
		PublicURL:         publicURL(src),
	})
	attempt := e.beginFeedAttempt(entry, reason)
	defer attempt.finish()

	if !snap.isEnabled(runName, opts) {
		entry.MarkSourceProcessingDisabled(e.now().UTC().Unix())
		return processingOK("disabled", false)
	}

	bodyPath := snap.feedBodyPath(runName)
	processingBodyPath, err := claimProcessingFeedBody(bodyPath)
	if err != nil {
		entry.MarkSourceProcessingMissingInput(bodyPath)
		e.logger.Error("feed body does not exist", "source", runName, "path", bodyPath, "error", err)
		return processingException(ProcessingExceptionMissingInput, "feed body does not exist", err)
	}
	info, err := os.Stat(processingBodyPath)
	if err != nil {
		entry.MarkSourceProcessingMissingInput(processingBodyPath)
		e.logger.Error("feed body does not exist", "source", runName, "path", processingBodyPath, "error", err)
		return processingException(ProcessingExceptionMissingInput, "feed body does not exist", err)
	}
	sourceMTime := info.ModTime().UTC()
	observedAt := e.now().UTC()
	entry.MarkSourceProcessingStarted()
	return e.processAndCommitWithSnapshot(ctx, snap, runName, src, output, entry, processingBodyPath, info.Size(), sourceMTime, observedAt)
}

// processAndCommit parses an already-prepared canonical feed body and commits
// the downstream artifacts derived from it.
func (e *Engine) processAndCommit(ctx context.Context, runName string, src *config.Source, output string, entry *cache.Entry, sourcePath string, sourceBytes int64, sourceMTime, observedAt time.Time) FeedProcessingResult {
	return e.processAndCommitWithSnapshot(ctx, e.operationSnapshot(), runName, src, output, entry, sourcePath, sourceBytes, sourceMTime, observedAt)
}

func (e *Engine) processAndCommitWithSnapshot(ctx context.Context, snap operationSnapshot, runName string, src *config.Source, output string, entry *cache.Entry, sourcePath string, sourceBytes int64, sourceMTime, observedAt time.Time) FeedProcessingResult {
	work := FeedProcessingWork{InputBytes: sourceBytes}
	started := time.Now()
	parseOp := e.beginActiveOperation("sources.parse_feed_body", runName, "read", "bytes", sourceBytes)
	var resolveOp *activeOperationHandle
	parseOpts := iprange.DefaultParseOptions()
	parseOpts.DefaultPrefix = 32
	parseOpts.DNSThreads = snap.runtime.ParallelDNSQueries
	parseOpts.Progress = func(progress iprange.ParseProgress) {
		switch progress.Stage {
		case "resolve":
			if resolveOp == nil {
				if parseOp != nil {
					parseOp.Update(sourceBytes, sourceBytes, parseProgressCounters(progress))
					parseOp.Finish()
					parseOp = nil
				}
				resolveOp = e.beginActiveOperation("sources.resolve_hostnames", runName, "resolve", "hostnames", progress.HostnamesQueued)
			}
			if resolveOp != nil {
				resolveOp.Update(progress.HostnamesCompleted, progress.HostnamesQueued, parseProgressCounters(progress))
			}
		default:
			if parseOp != nil {
				parseOp.Update(progress.BytesRead, sourceBytes, parseProgressCounters(progress))
			}
		}
	}
	initialSet, err := downloader.ParseCanonicalFeedFileWithOptions(ctx, runName, sourcePath, parseOpts)
	if parseOp != nil {
		parseOp.Update(sourceBytes, sourceBytes, nil)
		parseOp.Finish()
	}
	if resolveOp != nil {
		resolveOp.Finish()
	}
	parseDur := time.Since(started)
	e.observeRunOperation("sources.parse_feed_body", parseDur)
	e.observeFeedOperation(runName, "sources.parse_feed_body", parseDur)
	if err != nil {
		entry.MarkSourceParseFailed(err.Error())
		e.logger.Error("source processing failed", "source", runName, "stage", "parse", "error", err)
		return processingException(ProcessingExceptionParse, err.Error(), err).withWork(work)
	}
	finalSet := initialSet
	work.Entries = int64(finalSet.Entries())
	work.UniqueIPs = int64Clamp(finalSet.UniqueCount())
	diffTotal := int64(finalSet.Entries())
	if entrySnapshot := entry.Snapshot(); entrySnapshot.Entries > 0 {
		diffTotal += int64(entrySnapshot.Entries)
	}
	diffOp := e.beginActiveOperation("sources.diff_previous_latest", runName, "diff", "ranges", diffTotal)
	retentionDiff := e.retentionDiffWithPreviousLatestWithSnapshot(ctx, snap, runName, finalSet)
	if diffOp != nil {
		diffOp.Update(diffTotal, diffTotal, map[string]int64{
			"added_ips":   int64Clamp(retentionDiff.added),
			"removed_ips": int64Clamp(retentionDiff.removed),
		})
		diffOp.Finish()
	}

	started = time.Now()
	finalizeOp := e.beginActiveOperation("sources.finalize", runName, "write", "operation", 1)
	if err := e.finalizeWithSnapshot(ctx, snap, runName, src, output, sourcePath, finalSet, sourceMTime, observedAt); err != nil {
		if finalizeOp != nil {
			finalizeOp.Finish()
		}
		finalizeDur := time.Since(started)
		e.observeRunOperation("sources.finalize", finalizeDur)
		e.observeFeedOperation(runName, "sources.finalize", finalizeDur)
		entry.MarkSourceFinalizeFailed(err.Error())
		e.logger.Error("source processing failed", "source", runName, "stage", "finalize", "error", err)
		return processingException(ProcessingExceptionFinalize, err.Error(), err).withWork(work)
	}
	if finalizeOp != nil {
		finalizeOp.Update(1, 1, nil)
		finalizeOp.Finish()
	}
	finalizeDur := time.Since(started)
	e.observeRunOperation("sources.finalize", finalizeDur)
	e.observeFeedOperation(runName, "sources.finalize", finalizeDur)

	started = time.Now()
	retentionOp := e.beginActiveOperation("sources.update_retention", runName, "update", "operation", 1)
	if err := e.updateRetentionFromDiffWithSnapshot(ctx, snap, runName, retentionDiff, finalSet, sourceMTime); err != nil {
		if retentionOp != nil {
			retentionOp.Finish()
		}
		retentionDur := time.Since(started)
		e.observeRunOperation("sources.update_retention", retentionDur)
		e.observeFeedOperation(runName, "sources.update_retention", retentionDur)
		entry.MarkSourceRetentionFailed(err.Error())
		e.logger.Error("source processing failed", "source", runName, "stage", "retention", "error", err)
		return processingException(ProcessingExceptionRetention, err.Error(), err).withWork(work)
	}
	if retentionOp != nil {
		retentionOp.Finish()
	}
	retentionDur := time.Since(started)
	e.observeRunOperation("sources.update_retention", retentionDur)
	e.observeFeedOperation(runName, "sources.update_retention", retentionDur)

	started = time.Now()
	rotationOp := e.beginActiveOperation("sources.refresh_rotation", runName, "refresh", "operation", 1)
	e.refreshRotationStatsFromLedger(runName, entry)
	if rotationOp != nil {
		rotationOp.Update(1, 1, nil)
		rotationOp.Finish()
	}
	rotationDur := time.Since(started)
	e.observeRunOperation("sources.refresh_rotation", rotationDur)
	e.observeFeedOperation(runName, "sources.refresh_rotation", rotationDur)

	if finalSet.UniqueCount() == 0 {
		entry.MarkSourceProcessingComplete(true)
		e.logger.Info("source updated to empty set", "source", runName)
		return processingOK("updated successfully with empty set", true).withWork(work)
	}
	entry.MarkSourceProcessingComplete(false)
	e.logger.Info("source updated", "source", runName, "entries", finalSet.Entries(), "unique_ips", finalSet.UniqueCount())
	return processingOK("updated successfully", true).withWork(work)
}

func (e *Engine) retentionDiffWithPreviousLatest(ctx context.Context, name string, current *iprange.IPSet) retentionUpdateDiff {
	return e.retentionDiffWithPreviousLatestWithSnapshot(ctx, e.operationSnapshot(), name, current)
}

func (e *Engine) retentionDiffWithPreviousLatestWithSnapshot(ctx context.Context, snap operationSnapshot, name string, current *iprange.IPSet) retentionUpdateDiff {
	previous, err := e.openPreviousLatestSetWithRuntime(ctx, snap.runtime, name)
	if err != nil {
		e.logger.Warn("could not open previous binary set, treating as first run", "source", name, "error", err)
		return retentionDiff(iprange.New(name), current)
	}
	defer func() {
		if err := previous.Close(); err != nil {
			e.logger.Warn("could not close previous binary set", "source", name, "error", err)
		}
	}()
	diff, err := e.retentionDiffFromSources(ctx, name, previous.RangeSource, current)
	if err != nil {
		e.logger.Warn("could not diff previous binary set, treating as first run", "source", name, "error", err)
		return retentionDiff(iprange.New(name), current)
	}
	return diff
}

func parseProgressCounters(progress iprange.ParseProgress) map[string]int64 {
	return map[string]int64{
		"bytes_read":          progress.BytesRead,
		"lines_read":          progress.LinesRead,
		"ranges_accepted":     progress.RangesAccepted,
		"hostnames_queued":    progress.HostnamesQueued,
		"hostnames_completed": progress.HostnamesCompleted,
		"hostnames_resolved":  progress.HostnamesResolved,
	}
}
