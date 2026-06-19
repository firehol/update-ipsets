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

func (e *Engine) ensureDirectories() error {
	dirs := []string{
		e.runtime.BaseDir,
		e.runtime.CacheDir,
		e.runtime.LibDir,
		e.runtime.HistoryDir,
		e.runtime.ErrorsDir,
		e.runtime.TmpDir,
	}
	if e.runtime.WebDir != "" {
		dirs = append(dirs, e.runtime.WebDir)
	}
	if e.runtime.WebDirForIPSets != "" {
		dirs = append(dirs, e.runtime.WebDirForIPSets)
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, generatedDirMode); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) processSource(ctx context.Context, src *config.Source, opts RunOptions, reason runreason.Reason) FeedProcessingResult {
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
	return e.processConcreteSource(ctx, src.Name, src, src.Output, opts, reason)
}

func (e *Engine) processConcreteSource(ctx context.Context, runName string, src *config.Source, output string, opts RunOptions, reason runreason.Reason) FeedProcessingResult {
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

	if !e.isEnabled(runName, opts) {
		entry.MarkSourceProcessingDisabled(e.now().UTC().Unix())
		return processingOK("disabled", false)
	}

	bodyPath := e.feedBodyPath(runName)
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
	return e.processAndCommit(ctx, runName, src, output, entry, processingBodyPath, info.Size(), sourceMTime, observedAt)
}

// processAndCommit parses an already-prepared canonical feed body and commits
// the downstream artifacts derived from it.
func (e *Engine) processAndCommit(ctx context.Context, runName string, src *config.Source, output string, entry *cache.Entry, sourcePath string, sourceBytes int64, sourceMTime, observedAt time.Time) FeedProcessingResult {
	work := FeedProcessingWork{InputBytes: sourceBytes}
	started := time.Now()
	initialSet, err := downloader.ParseCanonicalFeedFile(ctx, runName, sourcePath, e.runtime.ParallelDNSQueries)
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
	retentionDiff := e.retentionDiffWithPreviousLatest(ctx, runName, finalSet)

	started = time.Now()
	if err := e.finalize(runName, src, output, sourcePath, finalSet, sourceMTime, observedAt); err != nil {
		finalizeDur := time.Since(started)
		e.observeRunOperation("sources.finalize", finalizeDur)
		e.observeFeedOperation(runName, "sources.finalize", finalizeDur)
		entry.MarkSourceFinalizeFailed(err.Error())
		e.logger.Error("source processing failed", "source", runName, "stage", "finalize", "error", err)
		return processingException(ProcessingExceptionFinalize, err.Error(), err).withWork(work)
	}
	finalizeDur := time.Since(started)
	e.observeRunOperation("sources.finalize", finalizeDur)
	e.observeFeedOperation(runName, "sources.finalize", finalizeDur)

	started = time.Now()
	retentionOp := e.beginActiveOperation("sources.update_retention", runName, "update", "operation", 1)
	if err := e.updateRetentionFromDiff(ctx, runName, retentionDiff, finalSet, sourceMTime); err != nil {
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
	e.refreshRotationStatsFromLedger(runName, entry)
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
	previous, err := e.openPreviousLatestSet(ctx, name)
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
