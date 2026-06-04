package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

type asnProviderProcessor struct {
	e                     *Engine
	src                   *config.Source
	entry                 *cache.Entry
	reason                runreason.Reason
	asnDir                string
	spec                  formatSpec
	archivePath           string
	processingArchivePath string
	dataPath              string
	archiveTime           time.Time
}

func (e *Engine) processASNProvider(src *config.Source, asnDir string, reason runreason.Reason) (*asnloc.Database, error) {
	processor := &asnProviderProcessor{
		e:      e,
		src:    src,
		entry:  e.state.Entry(src.Name),
		reason: reason,
		asnDir: asnDir,
	}
	return processor.process()
}

func (p *asnProviderProcessor) process() (*asnloc.Database, error) {
	attempt := p.e.beginFeedAttempt(p.entry, p.reason)
	defer attempt.finish()

	p.applySourceConfig()
	if !p.validateFormat() {
		return nil, nil
	}
	if err := p.preparePaths(); err != nil {
		return nil, err
	}
	if !p.extractDatabaseIfNeeded() || !p.databaseAvailable() {
		return nil, nil
	}
	db, err := p.openDatabase()
	if err != nil {
		return nil, err
	}
	p.recordLoaded(db)
	return db, nil
}

func (p *asnProviderProcessor) applySourceConfig() {
	p.entry.ApplyProviderSourceConfig(cache.ProviderSourceConfigSnapshot{
		Name:              p.src.Name,
		Category:          p.src.Category,
		DefaultCategory:   "asn",
		Info:              p.src.Info,
		Maintainer:        p.src.Maintainer,
		MaintainerURL:     p.src.MaintainerURL,
		Frequency:         p.src.Frequency,
		URL:               p.src.URL,
		Downloader:        p.src.Downloader,
		DownloaderOptions: p.src.DownloaderOptions,
	})
}

func (p *asnProviderProcessor) validateFormat() bool {
	spec, ok := lookupFormat(p.src.Format)
	if !ok || spec.role != formatRoleASN {
		p.e.logger.Error("ASN source has unknown or wrong-role format", "name", p.src.Name, "format", p.src.Format)
		p.entry.MarkProviderConfigError("unknown ASN format " + p.src.Format)
		return false
	}
	p.spec = spec
	return true
}

func (p *asnProviderProcessor) preparePaths() error {
	providerDir := filepath.Join(p.asnDir, p.src.Name)
	if err := os.MkdirAll(providerDir, generatedDirMode); err != nil {
		p.entry.MarkProviderFilesystemFailure(err.Error())
		return err
	}
	p.archivePath = filepath.Join(providerDir, "source")
	p.processingArchivePath = preferStagedPath(p.archivePath)
	p.dataPath = filepath.Join(providerDir, p.spec.dataFile)
	if info, err := os.Stat(p.processingArchivePath); err == nil {
		p.archiveTime = info.ModTime().UTC()
	}
	return nil
}

func (p *asnProviderProcessor) extractDatabaseIfNeeded() bool {
	if p.spec.extract == nil {
		return true
	}
	switch {
	case p.processingArchivePath != p.archivePath:
		return p.extractDatabase("ASN staged extract failed")
	case !fileExists(p.dataPath):
		return p.extractDatabase("ASN extract failed")
	default:
		return true
	}
}

func (p *asnProviderProcessor) extractDatabase(logMessage string) bool {
	p.entry.MarkProviderProcessing()
	if err := p.spec.extract(p.processingArchivePath, p.dataPath); err != nil {
		p.e.logger.Error(logMessage, "name", p.src.Name, "error", err)
		p.entry.MarkProviderExtractFailed(err.Error())
		return false
	}
	return true
}

func (p *asnProviderProcessor) databaseAvailable() bool {
	if fileExists(p.dataPath) {
		return true
	}
	p.e.logger.Warn("ASN database not available, skipping source", "name", p.src.Name, "path", p.dataPath)
	p.entry.MarkProviderUnavailable("database file not found at " + p.dataPath)
	return false
}

func (p *asnProviderProcessor) openDatabase() (*asnloc.Database, error) {
	p.entry.MarkProviderProcessing()
	db, err := asnloc.Open(p.src.Format, p.dataPath)
	if err != nil {
		p.e.logger.Error("ASN open failed", "name", p.src.Name, "format", p.src.Format, "path", p.dataPath, "error", err)
		p.entry.MarkProviderOpenFailed(err.Error())
		return nil, fmt.Errorf("asn open %s: %w", p.src.Name, err)
	}
	return db, nil
}

func (p *asnProviderProcessor) recordLoaded(db *asnloc.Database) {
	entries, uniqueIPs := p.providerStats(db)
	processedAt := p.e.now().UTC()
	now := p.e.now().UTC()
	clockSkewSeconds := int64(0)
	if p.archiveTime.After(now) {
		clockSkewSeconds = int64(p.archiveTime.Sub(now).Seconds())
	}
	stale := p.entry.RecordProviderLoaded(cache.ProviderLoadStats{
		SourceUnix:       p.archiveTime.Unix(),
		ProcessedUnix:    processedAt.Unix(),
		ClockSkewSeconds: clockSkewSeconds,
		Entries:          entries,
		UniqueIPs:        uniqueIPs,
	}, p.src.Frequency, p.processingArchivePath != p.archivePath)
	p.e.logger.Info("ASN source loaded", "name", p.src.Name, "networks", p.entry.Entries, "ipv4_covered", p.entry.UniqueIPs)
	if stale {
		p.e.logger.Warn("ASN using stale data after download failure", "name", p.src.Name, "failures", p.entry.DownloadFailures)
	}
}

func (p *asnProviderProcessor) providerStats(db *asnloc.Database) (int, uint64) {
	entries := p.entry.Entries
	uniqueIPs := p.entry.UniqueIPs
	networks, ipv4Covered, err := db.Stats()
	if err != nil {
		p.e.logger.Warn("ASN stats failed", "name", p.src.Name, "error", err)
		return entries, uniqueIPs
	}
	return networks, ipv4Covered
}
