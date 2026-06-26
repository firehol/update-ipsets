package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
)

type DownloadDecision struct {
	Name            string
	Status          DownloadStatus
	Message         string
	HTTPCode        int
	BodySize        int64
	ProcessingNames []string
	PromoteNames    []string
}

func (e *Engine) IsDownloadable(name string) bool {
	if e.isArtifact(name) {
		return true
	}
	src := e.cfg.Sources[name]
	if src == nil {
		return false
	}
	if src.ArtifactParent != "" {
		return true
	}
	return src.URL != "" || len(src.Static) > 0
}

func (e *Engine) IsProviderDatabase(name string) bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.cfg == nil {
		return false
	}
	src := e.cfg.Sources[name]
	return src != nil && (src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP))
}

func (e *Engine) IsMerge(name string) bool {
	if e == nil || e.cfg == nil {
		return false
	}
	src := e.cfg.Sources[name]
	return src != nil && src.Provenance == config.ProvenanceSecondaryMerge
}

func (e *Engine) IsHistoryDerivative(name string) bool {
	if e == nil || e.cfg == nil {
		return false
	}
	src := e.cfg.Sources[name]
	return src != nil && src.Provenance == config.ProvenanceSecondaryRetention
}

func (e *Engine) EffectiveScheduleMinutes(name string) int {
	src := e.cfg.Sources[name]
	if src == nil {
		return 0
	}
	return src.Frequency
}

func (e *Engine) FetchAndStage(ctx context.Context, name string, force, enableAll bool) (DownloadDecision, error) {
	if e.isArtifact(name) {
		return e.fetchAndStageArtifact(ctx, name, force, enableAll)
	}
	src := e.cfg.Sources[name]
	if src == nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "unknown source"}, fmt.Errorf("unknown source %q", name)
	}
	if !e.IsDownloadable(name) {
		return DownloadDecision{Name: name, Status: DownloadStatusSkipped, Message: "source is not fetched by the download loop"}, nil
	}
	switch {
	case src.ArtifactParent != "":
		return e.fetchAndStageArtifactChild(ctx, src, force, enableAll)
	case e.IsHistoryDerivative(name):
		return e.fetchAndStageHistoryDerivative(ctx, src, force, enableAll)
	case e.IsMerge(name):
		return e.fetchAndStageMerge(ctx, src, force, enableAll)
	case src.HasUse(config.UseASN):
		return e.fetchAndStageProvider(ctx, src, force, enableAll, true)
	case src.HasUse(config.UseGeoIP):
		return e.fetchAndStageProvider(ctx, src, force, enableAll, false)
	default:
		return e.fetchAndStagePlainSource(ctx, src, force, enableAll)
	}
}

func (e *Engine) RecoverStagedSources() ([]string, error) {
	names := make([]string, 0)
	for _, name := range config.SortedSourceNames(e.cfg) {
		src := e.cfg.Sources[name]
		if src == nil || !e.IsDownloadable(name) {
			continue
		}
		if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
			finalPath := e.providerArchivePath(name, src)
			_ = os.Remove(pendingTempPath(finalPath))
			if fileExists(stagedPath(finalPath)) {
				names = append(names, name)
			}
			continue
		}
		bodyPath := e.feedBodyPath(name)
		_ = os.Remove(pendingTempPath(bodyPath))
		_ = os.Remove(pendingTempPath(e.sourcePath(name)))
		if fileExists(stagedPath(bodyPath)) {
			if _, err := claimProcessingFeedBody(bodyPath); err != nil {
				return nil, err
			}
			names = append(names, name)
			continue
		}
		if fileExists(processingPath(bodyPath)) {
			names = append(names, name)
		}
	}
	return names, nil
}

func (e *Engine) PromoteCommittedDownloads(names []string) error {
	promoted := false
	for _, name := range names {
		if !e.IsDownloadable(name) {
			continue
		}
		finalPath := ""
		if e.isArtifact(name) {
			finalPath = e.artifactSourcePath(name)
		} else {
			src := e.cfg.Sources[name]
			if src == nil {
				continue
			}
			if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
				finalPath = e.providerArchivePath(name, src)
			} else {
				continue
			}
		}
		if err := promoteStagedFile(finalPath); err != nil {
			return err
		}
		promoted = true
	}
	if promoted {
		e.MarkIntegrityCachesStale()
	}
	return nil
}

func (e *Engine) HasStagedDownload(name string) bool {
	if !e.IsDownloadable(name) {
		return false
	}
	finalPath := ""
	if e.isArtifact(name) {
		finalPath = e.artifactSourcePath(name)
	} else {
		src := e.cfg.Sources[name]
		if src == nil {
			return false
		}
		if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
			finalPath = e.providerArchivePath(name, src)
		} else {
			finalPath = e.feedBodyPath(name)
		}
	}
	return fileExists(stagedPath(finalPath))
}

func (e *Engine) HasLocalFeedBody(name string) bool {
	if e == nil || e.cfg == nil || e.cfg.Sources[name] == nil {
		return false
	}
	return fileExists(latestFeedBodyPath(e.feedBodyPath(name)))
}

func (e *Engine) HasLocalReprocessState(name string) bool {
	if e == nil || e.cfg == nil {
		return false
	}
	if e.isArtifact(name) {
		return fileExists(preferStagedPath(e.artifactSourcePath(name)))
	}
	src := e.cfg.Sources[name]
	if src == nil {
		return false
	}
	if e.IsProviderDatabase(name) {
		return fileExists(preferStagedPath(e.providerArchivePath(name, src)))
	}
	return e.HasLocalFeedBody(name)
}

func (e *Engine) ResolveRecheckTarget(ctx context.Context, name string) string {
	ctx = nonNilContext(ctx)
	if e == nil || e.cfg == nil {
		return name
	}
	src := e.cfg.Sources[name]
	if src == nil {
		return name
	}
	if e.IsHistoryDerivative(name) {
		if _, _, err := e.composeHistoryDerivativeBody(ctx, src); err == nil {
			return name
		}
		if len(src.DerivedFrom) > 0 {
			return e.ResolveRecheckTarget(ctx, src.DerivedFrom[0])
		}
		return name
	}
	if src.ArtifactParent == "" {
		return name
	}
	if fileExists(preferStagedPath(e.sourcePath(name))) || e.HasLocalFeedBody(name) {
		return name
	}
	if e.cfg.ArtifactByName(src.ArtifactParent) != nil {
		return src.ArtifactParent
	}
	return name
}

func (e *Engine) fetchAndStagePlainSource(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	expandedURL := e.expandURL(src.URL)
	if expandedURL == "" && src.URL != "" {
		message := entry.MarkDownloadMissingEnv(src.URL)
		return DownloadDecision{Name: name, Status: DownloadStatusMissingEnv, Message: message}, nil
	}
	rawPath := e.sourcePath(name)
	bodyPath := e.feedBodyPath(name)
	result, err := e.fetchStaticSource(src, rawPath)
	if result == nil && err == nil {
		result, err = e.downloads.Fetch(ctx, downloader.Request{
			Name:              name,
			URL:               expandedURL,
			ReferencePath:     rawPath,
			UserAgent:         e.runtime.UserAgent,
			MaxConnectTime:    e.runtime.MaxConnectTime,
			MaxDownloadTime:   e.runtime.MaxDownloadTime,
			NoIfModifiedSince: src.Attributes["no_if_modified_since"] != "",
			Downloader:        src.Attributes["downloader"],
			DownloaderOptions: src.Attributes["downloader_options"],
			Referer:           "https://iplists.firehol.org/",
			AcceptEmpty:       true,
			MaxDownloadSize:   e.runtime.MaxDownloadSize,
			TmpDir:            e.runtime.TmpDir,
		})
	}
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	return e.applyRawFeedDownloadResult(ctx, entry, src, result, rawPath, bodyPath, force, enableAll)
}

func (e *Engine) fetchStaticSource(src *config.Source, rawPath string) (*downloader.Result, error) {
	if src == nil || len(src.Static) == 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	if e != nil && e.now != nil {
		now = e.now().UTC()
	}
	body := []byte(strings.Join(src.Static, "\n") + "\n")
	if existing, err := readFilePathUnderRoot(filepath.Dir(rawPath), rawPath); err == nil && bytes.Equal(existing, body) {
		modifiedAt := now
		if info, statErr := os.Stat(rawPath); statErr == nil {
			modifiedAt = info.ModTime().UTC()
		}
		return &downloader.Result{
			Status:       downloader.StatusSame,
			Message:      "static config source unchanged",
			BodySize:     int64(len(body)),
			ModifiedTime: modifiedAt,
			CheckedAt:    now,
		}, nil
	}
	tmpDir := e.runtime.TmpDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	} else if err := os.MkdirAll(tmpDir, generatedDirMode); err != nil {
		return nil, fmt.Errorf("create static source temp dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(tmpDir, "dl-static-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create static source temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmpFile.Write(body); err != nil {
		return nil, fmt.Errorf("write static source temp file: %w", err)
	}
	if err := tmpFile.Chmod(generatedFileMode); err != nil {
		return nil, fmt.Errorf("chmod static source temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close static source temp file: %w", err)
	}
	cleanup = false
	return &downloader.Result{
		Status:       downloader.StatusOK,
		Message:      "static config source updated",
		BodyPath:     tmpPath,
		BodySize:     int64(len(body)),
		ModifiedTime: now,
		CheckedAt:    now,
	}, nil
}

func (e *Engine) applyRawFeedDownloadResult(ctx context.Context, entry *cache.Entry, src *config.Source, result *downloader.Result, rawPath, bodyPath string, force, enableAll bool) (DownloadDecision, error) {
	name := entry.Snapshot().Name
	switch result.Status {
	case downloader.StatusFailed:
		result.CleanUp()
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(result.Message)
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: result.Message}, errors.New(result.Message)
	case downloader.StatusNotModified:
		result.CleanUp()
		modifiedAt := retainedRawObservedTime(rawPath, result.ModifiedTime, e.now().UTC())
		_, ok := existingLatestFeedBodyPath(bodyPath)
		if force || !ok {
			decision, rebuilt, err := e.rebuildCanonicalFeedBodyFromRetainedRaw(ctx, entry, src, rawPath, bodyPath, modifiedAt, force, enableAll)
			if rebuilt || err != nil {
				return decision, err
			}
		}
		clearFailure(entry)
		entry.MarkDownloadNotModified()
		return DownloadDecision{
			Name:            name,
			Status:          DownloadStatusNotModified,
			Message:         result.Message,
			ProcessingNames: forcedProcessingNames(name, force),
		}, nil
	case downloader.StatusSame:
		result.CleanUp()
		modifiedAt := feedBodyObservedTime(result.ModifiedTime, e.now().UTC())
		comparePath, ok := existingLatestFeedBodyPath(bodyPath)
		if force || !ok {
			decision, rebuilt, err := e.rebuildCanonicalFeedBodyFromRetainedRaw(ctx, entry, src, rawPath, bodyPath, modifiedAt, force, enableAll)
			if rebuilt || err != nil {
				return decision, err
			}
		}
		decision, err := e.applyExistingFeedBodySameResult(entry, comparePath, modifiedAt, force)
		if err != nil {
			return decision, err
		}
		return decision, nil
	case downloader.StatusOK:
		modifiedAt := feedBodyObservedTime(result.ModifiedTime, e.now().UTC())
		if err := moveDownloadedBody(result, rawPath); err != nil {
			result.CleanUp()
			e.incrementFailure(entry)
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		if err := touchFileAt(rawPath, modifiedAt); err != nil {
			e.incrementFailure(entry)
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		body, set, err := e.prepareCanonicalFeedBody(ctx, name, src, rawPath)
		if err != nil {
			e.incrementFailure(entry)
			entry.MarkDownloadPrepareFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusPrepareFailed, Message: err.Error()}, err
		}
		snapshotChanged, err := e.appendHistorySnapshot(ctx, name, set, modifiedAt)
		if err != nil {
			e.incrementFailure(entry)
			entry.MarkDownloadHistorySnapshotFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, err
		}
		decision, err := e.applyPreparedFeedBodyResult(entry, bodyPath, body, modifiedAt, force)
		if err != nil {
			return decision, err
		}
		if snapshotChanged {
			e.extendWithHistoryDerivativeDecisions(ctx, &decision, name, enableAll)
		}
		return decision, nil
	default:
		result.CleanUp()
		entry.MarkDownloadOperationFailed(result.Message)
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: result.Message}, errors.New(result.Message)
	}
}

func (e *Engine) rebuildCanonicalFeedBodyFromRetainedRaw(ctx context.Context, entry *cache.Entry, src *config.Source, rawPath, bodyPath string, modifiedAt time.Time, force, enableAll bool) (DownloadDecision, bool, error) {
	name := entry.Snapshot().Name
	if rawPath == "" || !fileExists(rawPath) {
		return DownloadDecision{}, false, nil
	}
	body, set, err := e.prepareCanonicalFeedBody(ctx, name, src, rawPath)
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadPrepareFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusPrepareFailed, Message: err.Error()}, true, err
	}
	snapshotChanged, err := e.appendHistorySnapshot(ctx, name, set, modifiedAt)
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadHistorySnapshotFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, true, err
	}
	decision, err := e.applyPreparedFeedBodyResult(entry, bodyPath, body, modifiedAt, force)
	if err != nil {
		return decision, true, err
	}
	if snapshotChanged {
		e.extendWithHistoryDerivativeDecisions(ctx, &decision, name, enableAll)
	}
	return decision, true, nil
}

func (e *Engine) fetchAndStageProvider(ctx context.Context, src *config.Source, force, enableAll, resolveASNURL bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	expandedURL := e.expandURL(src.URL)
	if expandedURL == "" && src.URL != "" {
		message := entry.MarkDownloadMissingEnv(src.URL)
		return DownloadDecision{Name: name, Status: DownloadStatusMissingEnv, Message: message}, nil
	}
	if resolveASNURL {
		resolved, err := e.resolveASNDownloadURL(ctx, src.Format, expandedURL)
		if err != nil {
			e.incrementFailure(entry)
			entry.MarkDownloadURLResolveFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusURLResolveFailed, Message: err.Error()}, nil
		}
		expandedURL = resolved
		entry.RecordResolvedDownloadURL(resolved)
	}

	archivePath := e.providerArchivePath(name, src)
	result, err := e.downloads.Fetch(ctx, downloader.Request{
		Name:              name,
		URL:               expandedURL,
		ReferencePath:     archivePath,
		UserAgent:         e.runtime.UserAgent,
		MaxConnectTime:    e.runtime.MaxConnectTime,
		MaxDownloadTime:   e.runtime.MaxDownloadTime,
		Downloader:        src.Downloader,
		DownloaderOptions: src.DownloaderOptions,
		Referer:           "https://iplists.firehol.org/",
		MaxDownloadSize:   e.runtime.MaxDownloadSize,
		TmpDir:            e.runtime.TmpDir,
	})
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	return e.applyStagedDownloadResult(entry, archivePath, result, force, enableAll)
}

func (e *Engine) fetchAndStageArtifactChild(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	sourcePath := e.sourcePath(name)
	localInputPath := preferStagedPath(sourcePath)
	info, err := os.Stat(localInputPath)
	if err != nil {
		e.incrementFailure(entry)
		message := "local materialized input does not exist at " + localInputPath
		entry.MarkDownloadFetchFailed(message)
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: message}, err
	}

	body, set, err := e.prepareCanonicalFeedBody(ctx, name, src, localInputPath)
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadPrepareFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusPrepareFailed, Message: err.Error()}, err
	}

	clearFailure(entry)
	decision, err := e.applyPreparedFeedBodyResult(entry, e.feedBodyPath(name), body, info.ModTime().UTC(), force)
	if err != nil {
		return decision, err
	}
	snapshotChanged, err := e.appendHistorySnapshot(ctx, name, set, info.ModTime().UTC())
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadHistorySnapshotFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, err
	}
	if snapshotChanged {
		e.extendWithHistoryDerivativeDecisions(ctx, &decision, name, enableAll)
	}
	return decision, nil
}

func (e *Engine) fetchAndStageHistoryDerivative(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	body, set, err := e.composeHistoryDerivativeBody(ctx, src)
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	clearFailure(entry)
	decision, err := e.applyPreparedFeedBodyResult(entry, e.feedBodyPath(name), body, e.now().UTC(), force)
	if err != nil {
		return decision, err
	}
	snapshotChanged, err := e.appendHistorySnapshot(ctx, name, set, e.now().UTC())
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadHistorySnapshotFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, err
	}
	if snapshotChanged {
		e.extendWithHistoryDerivativeDecisions(ctx, &decision, name, enableAll)
	}
	return decision, nil
}

func (e *Engine) fetchAndStageMerge(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	body, set, disabledMsg, err := e.composeMergeBody(ctx, src, enableAll)
	if disabledMsg != "" {
		clearFailure(entry)
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: disabledMsg}, nil
	}
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	clearFailure(entry)
	decision, err := e.applyPreparedFeedBodyResult(entry, e.feedBodyPath(name), body, e.now().UTC(), force)
	if err != nil {
		return decision, err
	}
	snapshotChanged, err := e.appendHistorySnapshot(ctx, name, set, e.now().UTC())
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadHistorySnapshotFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, err
	}
	if snapshotChanged {
		e.extendWithHistoryDerivativeDecisions(ctx, &decision, name, enableAll)
	}
	return decision, nil
}

func (e *Engine) applyStagedDownloadResult(entry *cache.Entry, finalPath string, result *downloader.Result, force, enableAll bool) (DownloadDecision, error) {
	name := entry.Snapshot().Name
	if result == nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "empty downloader result"}, fmt.Errorf("empty downloader result for %s", name)
	}
	entry.RecordDownloadSourceDate(result.ModifiedTime)
	switch result.Status {
	case downloader.StatusFailed:
		result.CleanUp()
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(result.Message)
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: result.Message}, errors.New(result.Message)
	case downloader.StatusNotModified:
		result.CleanUp()
		clearFailure(entry)
		entry.MarkDownloadNotModified()
		return DownloadDecision{
			Name:            name,
			Status:          DownloadStatusNotModified,
			Message:         result.Message,
			HTTPCode:        result.HTTPCode,
			BodySize:        result.BodySize,
			ProcessingNames: e.downloadProcessingNames(name, enableAll, force, false),
		}, nil
	case downloader.StatusSame:
		result.CleanUp()
		clearFailure(entry)
		if err := touchFileAt(finalPath, result.ModifiedTime); err != nil {
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		entry.MarkDownloadSame()
		return DownloadDecision{
			Name:            name,
			Status:          DownloadStatusSame,
			Message:         "downloaded file is the same as the previous source",
			HTTPCode:        result.HTTPCode,
			BodySize:        result.BodySize,
			ProcessingNames: e.downloadProcessingNames(name, enableAll, force, false),
		}, nil
	case downloader.StatusOK:
		clearFailure(entry)
		if err := stageDownloadedBody(result, finalPath); err != nil {
			result.CleanUp()
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		if err := touchFileAt(stagedPath(finalPath), result.ModifiedTime); err != nil {
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		entry.MarkDownloadDownloaded()
		return DownloadDecision{
			Name:            name,
			Status:          DownloadStatusDownloaded,
			Message:         result.Message,
			HTTPCode:        result.HTTPCode,
			BodySize:        result.BodySize,
			ProcessingNames: e.downloadProcessingNames(name, enableAll, force, true),
			PromoteNames:    []string{name},
		}, nil
	default:
		result.CleanUp()
		entry.MarkDownloadOperationFailed(result.Message)
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: result.Message}, errors.New(result.Message)
	}
}

func (e *Engine) applyPreparedFeedBodyResult(entry *cache.Entry, finalPath string, body []byte, modifiedAt time.Time, force bool) (DownloadDecision, error) {
	name := entry.Snapshot().Name
	entry.RecordDownloadSourceDate(modifiedAt)
	comparePath := latestFeedBodyPath(finalPath)
	same, err := canonicalFeedBodySame(comparePath, body)
	if err != nil {
		entry.MarkDownloadOperationFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
	}
	if same {
		if err := touchFileAt(comparePath, modifiedAt); err != nil {
			entry.MarkDownloadOperationFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
		}
		clearFailure(entry)
		entry.MarkDownloadSame()
		return DownloadDecision{
			Name:            name,
			Status:          DownloadStatusSame,
			Message:         "prepared feed body is the same as the latest local canonical feed body",
			ProcessingNames: forcedProcessingNames(name, force),
		}, nil
	}
	if err := stageFeedBodyBytes(finalPath, body); err != nil {
		entry.MarkDownloadOperationFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
	}
	if err := touchFileAt(stagedPath(finalPath), modifiedAt); err != nil {
		entry.MarkDownloadOperationFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
	}
	if len(body) == 0 {
		clearFailure(entry)
		entry.MarkDownloadEmpty()
		return DownloadDecision{
			Name:            name,
			Status:          DownloadStatusEmpty,
			Message:         "prepared feed body is empty",
			ProcessingNames: []string{name},
		}, nil
	}
	clearFailure(entry)
	entry.MarkDownloadDownloaded()
	return DownloadDecision{
		Name:            name,
		Status:          DownloadStatusDownloaded,
		Message:         "prepared feed body staged",
		ProcessingNames: []string{name},
	}, nil
}

func (e *Engine) applyExistingFeedBodySameResult(entry *cache.Entry, finalPath string, modifiedAt time.Time, force bool) (DownloadDecision, error) {
	name := entry.Snapshot().Name
	entry.RecordDownloadSourceDate(modifiedAt)
	if err := touchFileAt(finalPath, modifiedAt); err != nil {
		entry.MarkDownloadOperationFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: err.Error()}, err
	}
	clearFailure(entry)
	entry.MarkDownloadSame()
	return DownloadDecision{
		Name:            name,
		Status:          DownloadStatusSame,
		Message:         "prepared feed body is the same as the latest local canonical feed body",
		ProcessingNames: forcedProcessingNames(name, force),
	}, nil
}

func (e *Engine) downloadProcessingNames(name string, enableAll, force, admitted bool) []string {
	if e == nil {
		return nil
	}
	if e.IsProviderDatabase(name) {
		if !admitted && !force {
			return nil
		}
		return e.FullFeedReprocessTargets(enableAll)
	}
	if admitted {
		return []string{name}
	}
	return forcedProcessingNames(name, force)
}

func (e *Engine) FullFeedReprocessTargets(enableAll bool) []string {
	if e == nil || e.cfg == nil {
		return nil
	}
	targets := make([]string, 0, len(e.cfg.Sources))
	for _, name := range config.SortedSourceNames(e.cfg) {
		src := e.cfg.Sources[name]
		if src == nil {
			continue
		}
		if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
			continue
		}
		if !EffectiveSourceEnabled(e.cfg, e.runtime, name, enableAll) {
			continue
		}
		if !fileExists(latestFeedBodyPath(e.feedBodyPath(name))) {
			continue
		}
		targets = append(targets, name)
	}
	return targets
}

func (e *Engine) extendWithHistoryDerivativeDecisions(ctx context.Context, decision *DownloadDecision, parent string, enableAll bool) {
	if e == nil || e.cfg == nil || decision == nil {
		return
	}
	dependents := e.cfg.Dependents()[parent]
	for _, dep := range dependents {
		if err := contextErr(ctx); err != nil {
			e.logger.Error("history derivative recomposition cancelled", "parent", parent, "derivative", dep, "error", err)
			return
		}
		if !e.IsHistoryDerivative(dep) {
			continue
		}
		src := e.cfg.Sources[dep]
		if src == nil {
			continue
		}
		depDecision, err := e.fetchAndStageHistoryDerivative(ctx, src, false, enableAll)
		if err != nil {
			e.logger.Error("history derivative recomposition failed", "parent", parent, "derivative", dep, "error", err)
			continue
		}
		decision.ProcessingNames = append(decision.ProcessingNames, depDecision.ProcessingNames...)
		decision.PromoteNames = append(decision.PromoteNames, depDecision.PromoteNames...)
	}
}

func feedBodyObservedTime(modifiedAt, fallback time.Time) time.Time {
	if !modifiedAt.IsZero() {
		return modifiedAt.UTC()
	}
	return fallback.UTC()
}

func retainedRawObservedTime(rawPath string, modifiedAt, fallback time.Time) time.Time {
	if !modifiedAt.IsZero() {
		return modifiedAt.UTC()
	}
	if rawPath != "" {
		if info, err := os.Stat(rawPath); err == nil && !info.ModTime().IsZero() {
			return info.ModTime().UTC()
		}
	}
	return fallback.UTC()
}

func forcedProcessingNames(name string, force bool) []string {
	if !force {
		return nil
	}
	return []string{name}
}
