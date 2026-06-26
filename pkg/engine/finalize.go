package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func (e *Engine) finalize(ctx context.Context, name string, src *config.Source, output string, bodyPath string, finalSet *iprange.IPSet, sourceMTime time.Time, observedAt time.Time) error {
	path := e.finalPath(name, output)
	hash := hashForOutput(output)
	entry := e.state.Entry(name)
	baseline := entry.Snapshot()
	rawPath := e.sourcePath(name)

	if e.runtime.IPSetsApply {
		started := time.Now()
		if err := e.applyKernelSet(name, hash, bodyPath); err != nil {
			if e.runtime.ErrorsDir != "" {
				if writeErr := writeFeedBodyAtomic(filepath.Join(e.runtime.ErrorsDir, filepath.Base(path)), nil, bodyPath, time.Time{}); writeErr != nil {
					e.logger.Warn("failed to write error snapshot", "source", name, "error", writeErr)
				}
			}
			return err
		}
		dur := time.Since(started)
		e.observeRunOperation("sources.finalize.kernel_apply", dur)
		e.observeFeedOperation(name, "sources.finalize.kernel_apply", dur)
	}

	// Write the binary latest before promoting the canonical text file so
	// FileSet-based readers and the committed feed body stay aligned.
	latestDir := filepath.Join(e.runtime.LibDir, name)
	if err := os.MkdirAll(latestDir, generatedDirMode); err != nil {
		return err
	}
	writeLatestStarted := time.Now()
	if err := writeBinaryPath(filepath.Join(latestDir, "latest"), finalSet, sourceMTime); err != nil {
		return fmt.Errorf("write latest for %s: %w", name, err)
	}
	if e.querySetCache != nil {
		e.querySetCache.Invalidate(name)
	}
	writeLatestDur := time.Since(writeLatestStarted)
	e.observeRunOperation("sources.finalize.write_latest", writeLatestDur)
	e.observeFeedOperation(name, "sources.finalize.write_latest", writeLatestDur)

	writeTextStarted := time.Now()
	header := e.renderHeader(name, src, hash, finalSet, sourceMTime)
	if err := writeFeedBodyAtomic(path, header, bodyPath, sourceMTime); err != nil {
		return fmt.Errorf("write final file for %s: %w", name, err)
	}
	// Clean up the staging/processing body file so it is not mistaken for
	// reprocessable state.
	if bodyPath != path {
		if err := os.Remove(bodyPath); err != nil && !os.IsNotExist(err) {
			e.logger.Warn("failed to remove staging body", "source", name, "path", bodyPath, "error", err)
		}
	}
	writeTextDur := time.Since(writeTextStarted)
	e.observeRunOperation("sources.finalize.write_text", writeTextDur)
	e.observeFeedOperation(name, "sources.finalize.write_text", writeTextDur)

	sourceFile := filepath.Base(path)
	if fileExists(rawPath) {
		sourceFile = filepath.Base(rawPath)
	}
	// Critical reference feeds need stable processed-content identity for
	// provider_set_id; other feeds do not consume this field today.
	contentHash := ""
	if src.HasUse(config.UseCriticalInfrastructure) {
		hash, err := iprange.RangeSourceContentHashContext(ctx, finalSet)
		if err != nil {
			return fmt.Errorf("content hash for %s: %w", name, err)
		}
		contentHash = hash.Hex()
	}
	entry.ApplyFinalizedSourceSet(cache.FinalizedSourceSetSnapshot{
		File:          filepath.Base(path),
		Source:        sourceFile,
		IPV:           src.IPV,
		Hash:          hash,
		ContentHash:   contentHash,
		SourceUnix:    sourceMTime.Unix(),
		ProcessedUnix: observedAt.UTC().Unix(),
		Entries:       finalSet.Entries(),
		UniqueIPs:     finalSet.UniqueCount(),
	})
	// Always track evolution data in the internal full ledger. The public
	// _history.csv is generated later from the last WebChartsEntries rows,
	// matching the bash pipeline's lib/history split.
	appendHistoryStarted := time.Now()
	if err := appendCSV(filepath.Join(latestDir, "history.csv"), "DateTime,Entries,UniqueIPs\n",
		fmt.Sprintf("%d,%d,%d\n", sourceMTime.UTC().Unix(), finalSet.Entries(), finalSet.UniqueCount())); err != nil {
		e.logger.Warn("failed to append history data", "source", name, "error", err)
	}
	appendHistoryDur := time.Since(appendHistoryStarted)
	e.observeRunOperation("sources.finalize.append_history", appendHistoryDur)
	e.observeFeedOperation(name, "sources.finalize.append_history", appendHistoryDur)
	observeHistoryStarted := time.Now()
	if !e.observeHistoryPointContext(ctx, name, HistoryPoint{
		Timestamp: sourceMTime.UTC().Unix(),
		Name:      name,
		Entries:   finalSet.Entries(),
		UniqueIPs: finalSet.UniqueCount(),
	}, entry, &baseline, src.Frequency) {
		applyEntryStatsUpdate(entry, src.Frequency)
	}
	observeHistoryDur := time.Since(observeHistoryStarted)
	e.observeRunOperation("sources.finalize.observe_history", observeHistoryDur)
	e.observeFeedOperation(name, "sources.finalize.observe_history", observeHistoryDur)

	now := e.now().UTC()
	clockSkewSeconds := int64(0)
	if sourceMTime.After(now) {
		clockSkewSeconds = int64(sourceMTime.Sub(now).Seconds())
	}
	entry.ApplyFinalizedSourceMetadata(cache.FinalizedSourceMetadataSnapshot{
		Category:         src.Category,
		Info:             src.Info,
		Maintainer:       src.Maintainer,
		MaintainerURL:    src.MaintainerURL,
		License:          src.License,
		Attribution:      src.Attribution,
		ClockSkewSeconds: clockSkewSeconds,
	})

	return nil
}
