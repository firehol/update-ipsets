package engine

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

var currentSetStatsHookMu sync.Mutex
var currentSetStatsBeforeOpenHook func(name string)

func setCurrentSetStatsBeforeOpenHookForTest(fn func(name string)) func() {
	currentSetStatsHookMu.Lock()
	old := currentSetStatsBeforeOpenHook
	currentSetStatsBeforeOpenHook = fn
	currentSetStatsHookMu.Unlock()
	return func() {
		currentSetStatsHookMu.Lock()
		currentSetStatsBeforeOpenHook = old
		currentSetStatsHookMu.Unlock()
	}
}

func currentSetStatsBeforeOpenHookForTest() func(name string) {
	currentSetStatsHookMu.Lock()
	defer currentSetStatsHookMu.Unlock()
	return currentSetStatsBeforeOpenHook
}

func (e *Engine) bootstrapMissingEntriesFromDisk() error {
	snap := e.operationSnapshot()
	if e == nil || snap.cfg == nil || e.state == nil {
		return nil
	}

	bootstrapped := 0
	for _, name := range config.SortedArtifactNames(snap.cfg) {
		if e.state.EntrySnapshot(name) != nil {
			continue
		}
		artifact := snap.cfg.ArtifactByName(name)
		if artifact == nil {
			continue
		}
		entry, ok := e.bootstrapArtifactEntryFromDiskWithRuntime(snap.runtime, name, artifact)
		if !ok {
			continue
		}
		e.state.ReplaceEntry(name, *entry)
		bootstrapped++
	}
	for _, name := range config.SortedSourceNames(snap.cfg) {
		if e.state.EntrySnapshot(name) != nil {
			continue
		}
		src := snap.cfg.Sources[name]
		if src == nil {
			continue
		}
		entry, ok := e.bootstrapEntryFromDiskWithSnapshot(snap, name, src)
		if !ok {
			continue
		}
		e.state.ReplaceEntry(name, *entry)
		bootstrapped++
	}
	if bootstrapped == 0 {
		return nil
	}
	if err := cache.Save(e.cachePath, e.state); err != nil {
		e.logger.Warn("bootstrap: failed to persist synthesized cache entries", "count", bootstrapped, "path", e.cachePath, "error", err)
		return nil
	}
	e.logger.Info("bootstrapped missing cache entries from disk", "count", bootstrapped, "path", e.cachePath)
	return nil
}

func (e *Engine) reconcileEntriesFromSourceConfig() {
	cfg, rt := e.configRuntimeSnapshot()
	e.reconcileEntriesFromSourceConfigForSnapshot(cfg, rt)
}

func (e *Engine) reconcileEntriesFromSourceConfigForSnapshot(cfg *config.Config, rt Runtime) {
	if e == nil || cfg == nil || e.state == nil {
		return
	}
	for _, name := range config.SortedArtifactNames(cfg) {
		artifact := cfg.ArtifactByName(name)
		if artifact == nil {
			continue
		}
		if e.state.EntrySnapshot(name) == nil {
			continue
		}
		entry := e.state.Entry(name)
		if entry == nil {
			continue
		}
		seedEntryFromArtifactConfigForRuntime(rt, entry, name, artifact)
	}
	for _, name := range config.SortedSourceNames(cfg) {
		src := cfg.Sources[name]
		if src == nil {
			continue
		}
		if e.state.EntrySnapshot(name) == nil {
			continue
		}
		entry := e.state.Entry(name)
		if entry == nil {
			continue
		}
		seedEntryFromSourceConfigForRuntime(rt, entry, name, src)
		e.refreshCriticalEntryContentHashFromDiskForRuntime(rt, entry, name, src)
	}
}

func (e *Engine) bootstrapArtifactEntryFromDisk(name string, artifact *config.Artifact) (*cache.Entry, bool) {
	return e.bootstrapArtifactEntryFromDiskWithRuntime(e.Runtime(), name, artifact)
}

func (e *Engine) bootstrapArtifactEntryFromDiskWithRuntime(rt Runtime, name string, artifact *config.Artifact) (*cache.Entry, bool) {
	if e == nil || artifact == nil {
		return nil, false
	}

	entry := &cache.Entry{Name: name}
	seedEntryFromArtifactConfigForRuntime(rt, entry, name, artifact)

	sourcePath := artifactSourcePathForRuntime(rt, name)
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, false
	}
	mtime := info.ModTime().UTC().Unix()
	entry.ApplyArtifactDiskBootstrap(filepath.Base(sourcePath), mtime)
	return entry, true
}

func (e *Engine) bootstrapEntryFromDisk(name string, src *config.Source) (*cache.Entry, bool) {
	return e.bootstrapEntryFromDiskWithSnapshot(e.operationSnapshot(), name, src)
}

func (e *Engine) bootstrapEntryFromDiskWithRuntime(rt Runtime, name string, src *config.Source) (*cache.Entry, bool) {
	return e.bootstrapEntryFromDiskWithSnapshot(operationSnapshot{runtime: rt}, name, src)
}

func (e *Engine) bootstrapEntryFromDiskWithSnapshot(snap operationSnapshot, name string, src *config.Source) (*cache.Entry, bool) {
	if e == nil || src == nil {
		return nil, false
	}

	entry := &cache.Entry{Name: name}
	seedEntryFromSourceConfigForRuntime(snap.runtime, entry, name, src)

	points := e.bootstrapHistoryPointsWithRuntime(snap.runtime, name)
	if len(points) > 0 {
		applyHistoryPointsToEntry(entry, points, src.Frequency)
		last := points[len(points)-1]
		entry.ApplyHistoryBootstrapTimestamp(last.Timestamp)
	}

	if stats, ok := e.currentSetStatsForRuntime(snap.runtime, name, src); ok {
		entry.ApplyDiskSetStats(cacheDiskSetStats(stats))
	}
	e.refreshRotationStatsFromLedgerWithSnapshot(snap, name, entry)

	if !entry.FinalizeDiskBootstrap(src.Frequency) {
		return nil, false
	}
	return entry, true
}

func (e *Engine) seedEntryFromArtifactConfig(entry *cache.Entry, name string, artifact *config.Artifact) {
	seedEntryFromArtifactConfigForRuntime(e.Runtime(), entry, name, artifact)
}

func seedEntryFromArtifactConfigForRuntime(rt Runtime, entry *cache.Entry, name string, artifact *config.Artifact) {
	sourceFile := ""
	sourcePath := artifactSourcePathForRuntime(rt, name)
	if fileExists(sourcePath) {
		sourceFile = filepath.Base(sourcePath)
	}
	entry.ApplyArtifactConfig(cache.ArtifactConfigSnapshot{
		Name:          name,
		URL:           artifact.RSyncURL,
		Frequency:     artifact.Frequency,
		Info:          artifact.Info,
		Maintainer:    artifact.Maintainer,
		MaintainerURL: artifact.MaintainerURL,
		Downloader:    artifact.Type,
		SourceFile:    sourceFile,
	})
}

func (e *Engine) seedEntryFromSourceConfig(entry *cache.Entry, name string, src *config.Source) {
	seedEntryFromSourceConfigForRuntime(e.Runtime(), entry, name, src)
}

func seedEntryFromSourceConfigForRuntime(rt Runtime, entry *cache.Entry, name string, src *config.Source) {
	sourceFile := ""
	sourcePath := sourcePathForRuntime(rt, name)
	if fileExists(sourcePath) {
		sourceFile = filepath.Base(sourcePath)
	}
	finalFile := ""
	finalPath := finalPathForRuntime(rt, name, src.Output)
	if fileExists(finalPath) {
		finalFile = filepath.Base(finalPath)
	}
	entry.ApplySourceConfig(cache.SourceConfigSnapshot{
		Name:                      name,
		URL:                       src.URL,
		PublicURL:                 publicURL(src),
		IPV:                       src.IPV,
		Hash:                      hashForOutput(src.Output),
		Frequency:                 src.Frequency,
		History:                   src.History,
		Category:                  src.Category,
		Info:                      src.Info,
		Maintainer:                src.Maintainer,
		MaintainerURL:             src.MaintainerURL,
		Downloader:                src.Attributes["downloader"],
		DownloaderOptions:         src.Attributes["downloader_options"],
		FallbackDownloader:        src.Downloader,
		FallbackDownloaderOptions: src.DownloaderOptions,
		License:                   src.License,
		Attribution:               src.Attribution,
		SourceFile:                sourceFile,
		FinalFile:                 finalFile,
	})
}

func (e *Engine) bootstrapHistoryPoints(name string) []HistoryPoint {
	return e.bootstrapHistoryPointsWithRuntime(e.Runtime(), name)
}

func (e *Engine) bootstrapHistoryPointsWithRuntime(rt Runtime, name string) []HistoryPoint {
	return e.historyFromLedgerCSVContextWithRuntime(context.Background(), rt, name)
}

type setStats struct {
	entries     int
	uniqueIPs   uint64
	mtime       int64
	contentHash string
}

func (e *Engine) currentSetStats(name string, src *config.Source) (setStats, bool) {
	return e.currentSetStatsForRuntime(e.Runtime(), name, src)
}

func (e *Engine) currentSetStatsForRuntime(rt Runtime, name string, src *config.Source) (setStats, bool) {
	if hook := currentSetStatsBeforeOpenHookForTest(); hook != nil {
		hook(name)
	}
	needsContentHash := src != nil && src.HasUse(config.UseCriticalInfrastructure)
	for _, latestPath := range []string{
		filepath.Join(rt.LibDir, name, "latest"),
		filepath.Join(rt.LibDir, name, "latest.set"),
	} {
		if !fileExists(latestPath) {
			continue
		}
		info, err := os.Stat(latestPath)
		if err != nil {
			continue
		}
		fs, err := iprange.OpenFileSet(latestPath)
		if err != nil {
			continue
		}
		stats := setStats{
			entries:   fs.Len(),
			uniqueIPs: fs.UniqueIPs(),
			mtime:     info.ModTime().UTC().Unix(),
		}
		if needsContentHash {
			hash, err := iprange.RangeSourceContentHashContext(context.Background(), fs)
			if err != nil || !hash.Valid {
				_ = fs.Close()
				continue
			}
			stats.contentHash = hash.Hex()
		}
		_ = fs.Close()
		return stats, true
	}

	finalPath := finalPathForRuntime(rt, name, src.Output)
	if !fileExists(finalPath) {
		return setStats{}, false
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		return setStats{}, false
	}
	set, err := iprange.LoadPath(context.Background(), finalPath, iprange.DefaultParseOptions())
	if err != nil {
		return setStats{}, false
	}
	contentHash := ""
	if needsContentHash {
		hash, err := iprange.RangeSourceContentHashContext(context.Background(), set)
		if err != nil || !hash.Valid {
			return setStats{}, false
		}
		contentHash = hash.Hex()
	}
	return setStats{
		entries:     set.Entries(),
		uniqueIPs:   set.UniqueCount(),
		mtime:       info.ModTime().UTC().Unix(),
		contentHash: contentHash,
	}, true
}

func (e *Engine) refreshCriticalEntryContentHashFromDiskForRuntime(rt Runtime, entry *cache.Entry, name string, src *config.Source) {
	if entry == nil || src == nil || !src.HasUse(config.UseCriticalInfrastructure) {
		if entry != nil {
			entry.ClearContentHash()
		}
		return
	}
	stats, ok := e.currentSetStatsForRuntime(rt, name, src)
	if !ok || stats.contentHash == "" {
		return
	}
	entry.RefreshCriticalContentHashStats(cacheDiskSetStats(stats))
}

func cacheDiskSetStats(stats setStats) cache.DiskSetStats {
	return cache.DiskSetStats{
		Entries:     stats.entries,
		UniqueIPs:   stats.uniqueIPs,
		ModifiedAt:  stats.mtime,
		ContentHash: stats.contentHash,
	}
}
