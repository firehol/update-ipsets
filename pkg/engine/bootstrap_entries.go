package engine

import (
	"context"
	"os"
	"path/filepath"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func (e *Engine) bootstrapMissingEntriesFromDisk() error {
	if e == nil || e.cfg == nil || e.state == nil {
		return nil
	}

	bootstrapped := 0
	for _, name := range config.SortedArtifactNames(e.cfg) {
		if e.state.EntrySnapshot(name) != nil {
			continue
		}
		artifact := e.cfg.ArtifactByName(name)
		if artifact == nil {
			continue
		}
		entry, ok := e.bootstrapArtifactEntryFromDisk(name, artifact)
		if !ok {
			continue
		}
		e.state.ReplaceEntry(name, *entry)
		bootstrapped++
	}
	for _, name := range config.SortedSourceNames(e.cfg) {
		if e.state.EntrySnapshot(name) != nil {
			continue
		}
		src := e.cfg.Sources[name]
		if src == nil {
			continue
		}
		entry, ok := e.bootstrapEntryFromDisk(name, src)
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
	if e == nil || e.cfg == nil || e.state == nil {
		return
	}
	for _, name := range config.SortedArtifactNames(e.cfg) {
		artifact := e.cfg.ArtifactByName(name)
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
		e.seedEntryFromArtifactConfig(entry, name, artifact)
	}
	for _, name := range config.SortedSourceNames(e.cfg) {
		src := e.cfg.Sources[name]
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
		e.seedEntryFromSourceConfig(entry, name, src)
		e.refreshCriticalEntryContentHashFromDisk(entry, name, src)
	}
}

func (e *Engine) bootstrapArtifactEntryFromDisk(name string, artifact *config.Artifact) (*cache.Entry, bool) {
	if e == nil || artifact == nil {
		return nil, false
	}

	entry := &cache.Entry{Name: name}
	e.seedEntryFromArtifactConfig(entry, name, artifact)

	sourcePath := e.artifactSourcePath(name)
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, false
	}
	mtime := info.ModTime().UTC().Unix()
	entry.ApplyArtifactDiskBootstrap(filepath.Base(sourcePath), mtime)
	return entry, true
}

func (e *Engine) bootstrapEntryFromDisk(name string, src *config.Source) (*cache.Entry, bool) {
	if e == nil || src == nil {
		return nil, false
	}

	entry := &cache.Entry{Name: name}
	e.seedEntryFromSourceConfig(entry, name, src)

	points := e.bootstrapHistoryPoints(name)
	if len(points) > 0 {
		applyHistoryPointsToEntry(entry, points, src.Frequency)
		last := points[len(points)-1]
		entry.ApplyHistoryBootstrapTimestamp(last.Timestamp)
	}

	if stats, ok := e.currentSetStats(name, src); ok {
		entry.ApplyDiskSetStats(cacheDiskSetStats(stats))
	}
	e.refreshRotationStatsFromLedger(name, entry)

	if !entry.FinalizeDiskBootstrap(src.Frequency) {
		return nil, false
	}
	return entry, true
}

func (e *Engine) seedEntryFromArtifactConfig(entry *cache.Entry, name string, artifact *config.Artifact) {
	sourceFile := ""
	sourcePath := e.artifactSourcePath(name)
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
	sourceFile := ""
	sourcePath := e.sourcePath(name)
	if fileExists(sourcePath) {
		sourceFile = filepath.Base(sourcePath)
	}
	finalFile := ""
	finalPath := e.finalPath(name, src.Output)
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
	return e.historyFromLedgerCSV(name)
}

type setStats struct {
	entries     int
	uniqueIPs   uint64
	mtime       int64
	contentHash string
}

func (e *Engine) currentSetStats(name string, src *config.Source) (setStats, bool) {
	needsContentHash := src != nil && src.HasUse(config.UseCriticalInfrastructure)
	for _, latestPath := range []string{
		filepath.Join(e.runtime.LibDir, name, "latest"),
		filepath.Join(e.runtime.LibDir, name, "latest.set"),
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

	finalPath := e.finalPath(name, src.Output)
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

func (e *Engine) refreshCriticalEntryContentHashFromDisk(entry *cache.Entry, name string, src *config.Source) {
	if entry == nil || src == nil || !src.HasUse(config.UseCriticalInfrastructure) {
		if entry != nil {
			entry.ClearContentHash()
		}
		return
	}
	stats, ok := e.currentSetStats(name, src)
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
