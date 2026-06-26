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

func (e *Engine) FetchAndStage(ctx context.Context, name string, force, enableAll bool) (DownloadDecision, error) {
	snap := e.operationSnapshot()
	return e.fetchAndStageWithSnapshot(ctx, snap, name, force, enableAll)
}

func (e *Engine) fetchAndStageWithSnapshot(ctx context.Context, snap operationSnapshot, name string, force, enableAll bool) (DownloadDecision, error) {
	if snap.cfg != nil && snap.cfg.ArtifactByName(name) != nil {
		return e.fetchAndStageArtifactWithSnapshot(ctx, snap, name, force, enableAll)
	}
	if snap.cfg == nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "unknown source"}, fmt.Errorf("unknown source %q", name)
	}
	src := snap.cfg.Sources[name]
	if src == nil {
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: "unknown source"}, fmt.Errorf("unknown source %q", name)
	}
	if !snap.isDownloadable(name) {
		return DownloadDecision{Name: name, Status: DownloadStatusSkipped, Message: "source is not fetched by the download loop"}, nil
	}
	switch {
	case src.ArtifactParent != "":
		return e.fetchAndStageArtifactChildWithSnapshot(ctx, snap, src, force, enableAll)
	case snap.isHistoryDerivative(name):
		return e.fetchAndStageHistoryDerivativeWithSnapshot(ctx, snap, src, force, enableAll)
	case snap.isMerge(name):
		return e.fetchAndStageMergeWithSnapshot(ctx, snap, src, force, enableAll)
	case src.HasUse(config.UseASN):
		return e.fetchAndStageProviderWithSnapshot(ctx, snap, src, force, enableAll, true)
	case src.HasUse(config.UseGeoIP):
		return e.fetchAndStageProviderWithSnapshot(ctx, snap, src, force, enableAll, false)
	default:
		return e.fetchAndStagePlainSourceWithSnapshot(ctx, snap, src, force, enableAll)
	}
}

func (e *Engine) fetchAndStagePlainSource(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	return e.fetchAndStagePlainSourceWithSnapshot(ctx, e.operationSnapshot(), src, force, enableAll)
}

func (e *Engine) fetchAndStagePlainSourceWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(snap.cfg, snap.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	expandedURL := e.expandURLWithRuntime(snap.runtime, src.URL)
	if expandedURL == "" && src.URL != "" {
		message := entry.MarkDownloadMissingEnv(src.URL)
		return DownloadDecision{Name: name, Status: DownloadStatusMissingEnv, Message: message}, nil
	}
	rawPath := snap.sourcePath(name)
	bodyPath := snap.feedBodyPath(name)
	result, err := e.fetchStaticSourceWithSnapshot(src, snap, rawPath)
	if result == nil && err == nil {
		result, err = snap.downloads.Fetch(ctx, downloader.Request{
			Name:              name,
			URL:               expandedURL,
			ReferencePath:     rawPath,
			UserAgent:         snap.runtime.UserAgent,
			MaxConnectTime:    snap.runtime.MaxConnectTime,
			MaxDownloadTime:   snap.runtime.MaxDownloadTime,
			NoIfModifiedSince: src.Attributes["no_if_modified_since"] != "",
			Downloader:        src.Attributes["downloader"],
			DownloaderOptions: src.Attributes["downloader_options"],
			Referer:           "https://iplists.firehol.org/",
			AcceptEmpty:       true,
			MaxDownloadSize:   snap.runtime.MaxDownloadSize,
			TmpDir:            snap.runtime.TmpDir,
		})
	}
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	return e.applyRawFeedDownloadResultWithSnapshot(ctx, snap, entry, src, result, rawPath, bodyPath, force, enableAll)
}

func (e *Engine) fetchStaticSource(src *config.Source, rawPath string) (*downloader.Result, error) {
	return e.fetchStaticSourceWithSnapshot(src, e.operationSnapshot(), rawPath)
}

func (e *Engine) fetchStaticSourceWithSnapshot(src *config.Source, snap operationSnapshot, rawPath string) (*downloader.Result, error) {
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
	tmpDir := snap.runtime.TmpDir
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
	return e.applyRawFeedDownloadResultWithSnapshot(ctx, e.operationSnapshot(), entry, src, result, rawPath, bodyPath, force, enableAll)
}

func (e *Engine) applyRawFeedDownloadResultWithSnapshot(ctx context.Context, snap operationSnapshot, entry *cache.Entry, src *config.Source, result *downloader.Result, rawPath, bodyPath string, force, enableAll bool) (DownloadDecision, error) {
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
			decision, rebuilt, err := e.rebuildCanonicalFeedBodyFromRetainedRawWithSnapshot(ctx, snap, entry, src, rawPath, bodyPath, modifiedAt, force, enableAll)
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
			decision, rebuilt, err := e.rebuildCanonicalFeedBodyFromRetainedRawWithSnapshot(ctx, snap, entry, src, rawPath, bodyPath, modifiedAt, force, enableAll)
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
		body, set, err := e.prepareCanonicalFeedBodyWithSnapshot(ctx, snap, name, src, rawPath)
		if err != nil {
			e.incrementFailure(entry)
			entry.MarkDownloadPrepareFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusPrepareFailed, Message: err.Error()}, err
		}
		snapshotChanged, err := e.appendHistorySnapshotWithSnapshot(ctx, snap, name, set, modifiedAt)
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
			e.extendWithHistoryDerivativeDecisionsWithSnapshot(ctx, snap, &decision, name, enableAll)
		}
		return decision, nil
	default:
		result.CleanUp()
		entry.MarkDownloadOperationFailed(result.Message)
		return DownloadDecision{Name: name, Status: DownloadStatusFailed, Message: result.Message}, errors.New(result.Message)
	}
}

func (e *Engine) rebuildCanonicalFeedBodyFromRetainedRaw(ctx context.Context, entry *cache.Entry, src *config.Source, rawPath, bodyPath string, modifiedAt time.Time, force, enableAll bool) (DownloadDecision, bool, error) {
	return e.rebuildCanonicalFeedBodyFromRetainedRawWithSnapshot(ctx, e.operationSnapshot(), entry, src, rawPath, bodyPath, modifiedAt, force, enableAll)
}

func (e *Engine) rebuildCanonicalFeedBodyFromRetainedRawWithSnapshot(ctx context.Context, snap operationSnapshot, entry *cache.Entry, src *config.Source, rawPath, bodyPath string, modifiedAt time.Time, force, enableAll bool) (DownloadDecision, bool, error) {
	name := entry.Snapshot().Name
	if rawPath == "" || !fileExists(rawPath) {
		return DownloadDecision{}, false, nil
	}
	body, set, err := e.prepareCanonicalFeedBodyWithSnapshot(ctx, snap, name, src, rawPath)
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadPrepareFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusPrepareFailed, Message: err.Error()}, true, err
	}
	snapshotChanged, err := e.appendHistorySnapshotWithSnapshot(ctx, snap, name, set, modifiedAt)
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
		e.extendWithHistoryDerivativeDecisionsWithSnapshot(ctx, snap, &decision, name, enableAll)
	}
	return decision, true, nil
}

func (e *Engine) fetchAndStageProvider(ctx context.Context, src *config.Source, force, enableAll, resolveASNURL bool) (DownloadDecision, error) {
	return e.fetchAndStageProviderWithSnapshot(ctx, e.operationSnapshot(), src, force, enableAll, resolveASNURL)
}

func (e *Engine) fetchAndStageProviderWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source, force, enableAll, resolveASNURL bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(snap.cfg, snap.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	expandedURL := e.expandURLWithRuntime(snap.runtime, src.URL)
	if expandedURL == "" && src.URL != "" {
		message := entry.MarkDownloadMissingEnv(src.URL)
		return DownloadDecision{Name: name, Status: DownloadStatusMissingEnv, Message: message}, nil
	}
	if resolveASNURL {
		resolved, err := e.resolveASNDownloadURLWithRuntime(ctx, snap.runtime, src.Format, expandedURL)
		if err != nil {
			e.incrementFailure(entry)
			entry.MarkDownloadURLResolveFailed(err.Error())
			return DownloadDecision{Name: name, Status: DownloadStatusURLResolveFailed, Message: err.Error()}, nil
		}
		expandedURL = resolved
		entry.RecordResolvedDownloadURL(resolved)
	}

	archivePath := providerArchivePathForRuntime(snap.runtime, name, src)
	result, err := snap.downloads.Fetch(ctx, downloader.Request{
		Name:              name,
		URL:               expandedURL,
		ReferencePath:     archivePath,
		UserAgent:         snap.runtime.UserAgent,
		MaxConnectTime:    snap.runtime.MaxConnectTime,
		MaxDownloadTime:   snap.runtime.MaxDownloadTime,
		Downloader:        src.Downloader,
		DownloaderOptions: src.DownloaderOptions,
		Referer:           "https://iplists.firehol.org/",
		MaxDownloadSize:   snap.runtime.MaxDownloadSize,
		TmpDir:            snap.runtime.TmpDir,
	})
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	return e.applyStagedDownloadResultWithSnapshot(entry, snap, archivePath, result, force, enableAll)
}

func (e *Engine) fetchAndStageArtifactChild(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	return e.fetchAndStageArtifactChildWithSnapshot(ctx, e.operationSnapshot(), src, force, enableAll)
}

func (e *Engine) fetchAndStageArtifactChildWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(snap.cfg, snap.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	sourcePath := snap.sourcePath(name)
	localInputPath := preferStagedPath(sourcePath)
	info, err := os.Stat(localInputPath)
	if err != nil {
		e.incrementFailure(entry)
		message := "local materialized input does not exist at " + localInputPath
		entry.MarkDownloadFetchFailed(message)
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: message}, err
	}

	body, set, err := e.prepareCanonicalFeedBodyWithSnapshot(ctx, snap, name, src, localInputPath)
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadPrepareFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusPrepareFailed, Message: err.Error()}, err
	}

	clearFailure(entry)
	decision, err := e.applyPreparedFeedBodyResult(entry, snap.feedBodyPath(name), body, info.ModTime().UTC(), force)
	if err != nil {
		return decision, err
	}
	snapshotChanged, err := e.appendHistorySnapshotWithSnapshot(ctx, snap, name, set, info.ModTime().UTC())
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadHistorySnapshotFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, err
	}
	if snapshotChanged {
		e.extendWithHistoryDerivativeDecisionsWithSnapshot(ctx, snap, &decision, name, enableAll)
	}
	return decision, nil
}

func (e *Engine) fetchAndStageHistoryDerivative(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	return e.fetchAndStageHistoryDerivativeWithSnapshot(ctx, e.operationSnapshot(), src, force, enableAll)
}

func (e *Engine) fetchAndStageHistoryDerivativeWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(snap.cfg, snap.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	body, set, err := e.composeHistoryDerivativeBodyWithSnapshot(ctx, snap, src)
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadFetchFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusDownloadFailed, Message: err.Error()}, err
	}
	clearFailure(entry)
	decision, err := e.applyPreparedFeedBodyResult(entry, snap.feedBodyPath(name), body, e.now().UTC(), force)
	if err != nil {
		return decision, err
	}
	snapshotChanged, err := e.appendHistorySnapshotWithSnapshot(ctx, snap, name, set, e.now().UTC())
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadHistorySnapshotFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, err
	}
	if snapshotChanged {
		e.extendWithHistoryDerivativeDecisionsWithSnapshot(ctx, snap, &decision, name, enableAll)
	}
	return decision, nil
}

func (e *Engine) fetchAndStageMerge(ctx context.Context, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	return e.fetchAndStageMergeWithSnapshot(ctx, e.operationSnapshot(), src, force, enableAll)
}

func (e *Engine) fetchAndStageMergeWithSnapshot(ctx context.Context, snap operationSnapshot, src *config.Source, force, enableAll bool) (DownloadDecision, error) {
	name := src.Name
	entry := e.state.Entry(name)
	e.seedEntryFromSourceConfig(entry, name, src)
	checkedAt := e.now().UTC().Unix()
	if !EffectiveSourceEnabledForRun(snap.cfg, snap.runtime, name, enableAll, force) {
		entry.MarkDownloadDisabled(checkedAt)
		return DownloadDecision{Name: name, Status: DownloadStatusDisabled, Message: "feed is disabled"}, nil
	}
	entry.MarkDownloadStarted(checkedAt)

	body, set, disabledMsg, err := e.composeMergeBodyWithSnapshot(ctx, snap, src, enableAll)
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
	decision, err := e.applyPreparedFeedBodyResult(entry, snap.feedBodyPath(name), body, e.now().UTC(), force)
	if err != nil {
		return decision, err
	}
	snapshotChanged, err := e.appendHistorySnapshotWithSnapshot(ctx, snap, name, set, e.now().UTC())
	if err != nil {
		e.incrementFailure(entry)
		entry.MarkDownloadHistorySnapshotFailed(err.Error())
		return DownloadDecision{Name: name, Status: DownloadStatusHistorySnapshotFailed, Message: err.Error()}, err
	}
	if snapshotChanged {
		e.extendWithHistoryDerivativeDecisionsWithSnapshot(ctx, snap, &decision, name, enableAll)
	}
	return decision, nil
}

func (e *Engine) applyStagedDownloadResult(entry *cache.Entry, finalPath string, result *downloader.Result, force, enableAll bool) (DownloadDecision, error) {
	return e.applyStagedDownloadResultWithSnapshot(entry, e.operationSnapshot(), finalPath, result, force, enableAll)
}

func (e *Engine) applyStagedDownloadResultWithSnapshot(entry *cache.Entry, snap operationSnapshot, finalPath string, result *downloader.Result, force, enableAll bool) (DownloadDecision, error) {
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
			ProcessingNames: e.downloadProcessingNamesWithSnapshot(name, snap, enableAll, force, false),
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
			ProcessingNames: e.downloadProcessingNamesWithSnapshot(name, snap, enableAll, force, false),
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
			ProcessingNames: e.downloadProcessingNamesWithSnapshot(name, snap, enableAll, force, true),
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
	return e.downloadProcessingNamesWithSnapshot(name, e.operationSnapshot(), enableAll, force, admitted)
}

func (e *Engine) downloadProcessingNamesWithSnapshot(name string, snap operationSnapshot, enableAll, force, admitted bool) []string {
	if e == nil {
		return nil
	}
	src := (*config.Source)(nil)
	if snap.cfg != nil {
		src = snap.cfg.Sources[name]
	}
	if src != nil && (src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP)) {
		if !admitted && !force {
			return nil
		}
		return e.fullFeedReprocessTargetsWithSnapshot(snap, enableAll)
	}
	if admitted {
		return []string{name}
	}
	return forcedProcessingNames(name, force)
}

func (e *Engine) FullFeedReprocessTargets(enableAll bool) []string {
	return e.fullFeedReprocessTargetsWithSnapshot(e.operationSnapshot(), enableAll)
}

func (e *Engine) fullFeedReprocessTargetsWithSnapshot(snap operationSnapshot, enableAll bool) []string {
	if e == nil || snap.cfg == nil {
		return nil
	}
	targets := make([]string, 0, len(snap.cfg.Sources))
	for _, name := range config.SortedSourceNames(snap.cfg) {
		src := snap.cfg.Sources[name]
		if src == nil {
			continue
		}
		if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
			continue
		}
		if !EffectiveSourceEnabled(snap.cfg, snap.runtime, name, enableAll) {
			continue
		}
		if !fileExists(latestFeedBodyPath(snap.feedBodyPath(name))) {
			continue
		}
		targets = append(targets, name)
	}
	return targets
}

func (e *Engine) extendWithHistoryDerivativeDecisions(ctx context.Context, decision *DownloadDecision, parent string, enableAll bool) {
	e.extendWithHistoryDerivativeDecisionsWithSnapshot(ctx, e.operationSnapshot(), decision, parent, enableAll)
}

func (e *Engine) extendWithHistoryDerivativeDecisionsWithSnapshot(ctx context.Context, snap operationSnapshot, decision *DownloadDecision, parent string, enableAll bool) {
	if e == nil || snap.cfg == nil || decision == nil {
		return
	}
	dependents := snap.cfg.Dependents()[parent]
	for _, dep := range dependents {
		if err := contextErr(ctx); err != nil {
			e.logger.Error("history derivative recomposition cancelled", "parent", parent, "derivative", dep, "error", err)
			return
		}
		if !snap.isHistoryDerivative(dep) {
			continue
		}
		src := snap.cfg.Sources[dep]
		if src == nil {
			continue
		}
		depDecision, err := e.fetchAndStageHistoryDerivativeWithSnapshot(ctx, snap, src, false, enableAll)
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
