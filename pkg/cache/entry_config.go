package cache

import "fmt"

// ApplyArtifactConfig stores authored artifact metadata on the entry.
func (entry *Entry) ApplyArtifactConfig(snapshot ArtifactConfigSnapshot) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.Name = snapshot.Name
	entry.URL = snapshot.URL
	entry.PublicURL = ""
	entry.IPV = ""
	entry.Hash = ""
	entry.FrequencyMinutes = snapshot.Frequency
	entry.HistoryMinutes = nil
	entry.Category = "artifact"
	entry.Info = snapshot.Info
	entry.Maintainer = snapshot.Maintainer
	entry.MaintainerURL = snapshot.MaintainerURL
	entry.Downloader = snapshot.Downloader
	entry.DownloaderOptions = ""
	entry.License = ""
	entry.Attribution = ""
	if snapshot.SourceFile != "" {
		entry.Source = snapshot.SourceFile
		entry.File = snapshot.SourceFile
	}
}

// ApplySourceConfig stores authored source metadata on the entry.
func (entry *Entry) ApplySourceConfig(snapshot SourceConfigSnapshot) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.Name = snapshot.Name
	entry.URL = snapshot.URL
	entry.PublicURL = snapshot.PublicURL
	entry.IPV = snapshot.IPV
	entry.Hash = snapshot.Hash
	entry.FrequencyMinutes = snapshot.Frequency
	entry.HistoryMinutes = append([]int(nil), snapshot.History...)
	entry.Category = snapshot.Category
	entry.Info = snapshot.Info
	entry.Maintainer = snapshot.Maintainer
	entry.MaintainerURL = snapshot.MaintainerURL
	entry.Downloader = snapshot.Downloader
	entry.DownloaderOptions = snapshot.DownloaderOptions
	if entry.Downloader == "" {
		entry.Downloader = snapshot.FallbackDownloader
	}
	if entry.DownloaderOptions == "" {
		entry.DownloaderOptions = snapshot.FallbackDownloaderOptions
	}
	entry.License = snapshot.License
	entry.Attribution = snapshot.Attribution
	if snapshot.SourceFile != "" {
		entry.Source = snapshot.SourceFile
	}
	if snapshot.FinalFile != "" {
		entry.File = snapshot.FinalFile
	}
}

// ApplyProviderSourceConfig stores authored metadata for an ASN/geolocation
// provider entry.
func (entry *Entry) ApplyProviderSourceConfig(snapshot ProviderSourceConfigSnapshot) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.Name = snapshot.Name
	entry.Category = snapshot.Category
	if entry.Category == "" {
		entry.Category = snapshot.DefaultCategory
	}
	entry.Info = snapshot.Info
	entry.Maintainer = snapshot.Maintainer
	entry.MaintainerURL = snapshot.MaintainerURL
	entry.FrequencyMinutes = snapshot.Frequency
	entry.URL = snapshot.URL
	entry.PublicURL = snapshot.URL
	if snapshot.Downloader != "" {
		entry.Downloader = snapshot.Downloader
	}
	if snapshot.DownloaderOptions != "" {
		entry.DownloaderOptions = snapshot.DownloaderOptions
	}
}

// ApplyProcessingSourceConfig stores source metadata at the start of ordinary
// processing.
func (entry *Entry) ApplyProcessingSourceConfig(snapshot ProcessingSourceConfigSnapshot) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.Name = snapshot.Name
	entry.Info = snapshot.Info
	entry.Category = snapshot.Category
	entry.Maintainer = snapshot.Maintainer
	entry.MaintainerURL = snapshot.MaintainerURL
	entry.FrequencyMinutes = snapshot.Frequency
	entry.HistoryMinutes = append([]int(nil), snapshot.History...)
	entry.Downloader = snapshot.Downloader
	entry.DownloaderOptions = snapshot.DownloaderOptions
	entry.URL = snapshot.URL
	entry.PublicURL = snapshot.PublicURL
}

// ApplyArtifactDiskBootstrap stores filesystem evidence for a restored artifact.
func (entry *Entry) ApplyArtifactDiskBootstrap(sourceFile string, modifiedUnix int64) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.SourceDate = modifiedUnix
	entry.ProcessedDate = modifiedUnix
	entry.CheckedDate = modifiedUnix
	entry.StartedDate = modifiedUnix
	entry.Version = 1
	entry.Source = sourceFile
	entry.File = sourceFile
}

// ApplyHistoryBootstrapTimestamp stores timestamp evidence from restored history.
func (entry *Entry) ApplyHistoryBootstrapTimestamp(timestamp int64) {
	if entry == nil || timestamp <= 0 {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.SourceDate = timestamp
	entry.ProcessedDate = timestamp
	entry.CheckedDate = timestamp
}

// ApplyDiskSetStats stores stats and freshness evidence from a restored set.
func (entry *Entry) ApplyDiskSetStats(stats DiskSetStats) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.Entries = stats.Entries
	entry.UniqueIPs = stats.UniqueIPs
	entry.ContentHash = stats.ContentHash
	if entry.EntriesMin == 0 || stats.Entries < entry.EntriesMin {
		entry.EntriesMin = stats.Entries
	}
	if stats.Entries > entry.EntriesMax {
		entry.EntriesMax = stats.Entries
	}
	if entry.IPsMin == 0 || stats.UniqueIPs < entry.IPsMin {
		entry.IPsMin = stats.UniqueIPs
	}
	if stats.UniqueIPs > entry.IPsMax {
		entry.IPsMax = stats.UniqueIPs
	}
	if entry.SourceDate == 0 || stats.ModifiedAt > entry.SourceDate {
		entry.SourceDate = stats.ModifiedAt
	}
	if entry.ProcessedDate == 0 || stats.ModifiedAt > entry.ProcessedDate {
		entry.ProcessedDate = stats.ModifiedAt
	}
	if entry.CheckedDate == 0 || stats.ModifiedAt > entry.CheckedDate {
		entry.CheckedDate = stats.ModifiedAt
	}
}

// FinalizeDiskBootstrap fills derived bootstrap defaults and reports evidence.
func (entry *Entry) FinalizeDiskBootstrap(frequency int) bool {
	if entry == nil {
		return false
	}
	unlock := entry.lockEntry()
	defer unlock()
	if entry.StartedDate == 0 {
		entry.StartedDate = entry.SourceDate
	}
	if entry.Version == 0 && (entry.SourceDate > 0 || entry.ProcessedDate > 0 || entry.CheckedDate > 0) {
		entry.Version = 1
		if frequency > 0 {
			entry.AverageUpdateMins = frequency
			entry.MinUpdateMins = frequency
			entry.MaxUpdateMins = frequency
		}
	}
	return entry.File != "" ||
		entry.SourceDate != 0 ||
		entry.ProcessedDate != 0 ||
		entry.CheckedDate != 0 ||
		entry.Version != 0
}

// ClearContentHash removes stale content-hash state when a source is not critical.
func (entry *Entry) ClearContentHash() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.ContentHash = ""
}

// RefreshCriticalContentHashStats stores restored critical content-hash stats.
func (entry *Entry) RefreshCriticalContentHashStats(stats DiskSetStats) {
	if entry == nil || stats.ContentHash == "" {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.ContentHash = stats.ContentHash
	entry.Entries = stats.Entries
	entry.UniqueIPs = stats.UniqueIPs
}

// MarkProviderConfigError records invalid supporting-provider configuration.
func (entry *Entry) MarkProviderConfigError(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "config_error"
	entry.LastError = message
}

// MarkProviderFilesystemFailure records a local filesystem failure before a
// supporting provider can be loaded.
func (entry *Entry) MarkProviderFilesystemFailure(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "failed"
	entry.LastError = message
}

// MarkProviderProcessing records that supporting-provider data is being
// extracted, parsed, or opened.
func (entry *Entry) MarkProviderProcessing() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "processing"
}

// MarkProviderExtractFailed records a failed ASN provider archive extraction.
func (entry *Entry) MarkProviderExtractFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "extract_failed"
	entry.LastError = message
}

// MarkProviderUnavailable records missing supporting-provider data on disk.
func (entry *Entry) MarkProviderUnavailable(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "unavailable"
	entry.LastError = message
}

// MarkProviderParseFailed records a failed geolocation provider parse.
func (entry *Entry) MarkProviderParseFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "parse_failed"
	entry.LastError = message
}

// MarkProviderOpenFailed records a failed ASN provider database open.
func (entry *Entry) MarkProviderOpenFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "open_failed"
	entry.LastError = message
}

// RecordProviderLoaded stores freshness/stats evidence for a loaded
// supporting provider and returns true when the loaded provider is stale cached
// data after a download failure.
func (entry *Entry) RecordProviderLoaded(stats ProviderLoadStats, frequency int, stagedSource bool) bool {
	if entry == nil {
		return false
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.SourceDate = stats.SourceUnix
	entry.ProcessedDate = stats.ProcessedUnix
	if entry.StartedDate == 0 {
		entry.StartedDate = entry.SourceDate
	}
	entry.ClockSkewSeconds = stats.ClockSkewSeconds
	entry.Entries = stats.Entries
	entry.UniqueIPs = stats.UniqueIPs
	entry.recordStatsUpdateLocked(frequency)
	if stagedSource || entry.DownloadFailures == 0 {
		entry.LastStatus = "updated"
		entry.LastError = ""
		return false
	}
	entry.LastStatus = "stale"
	entry.LastError = fmt.Sprintf("download failed %d time(s); using cached data", entry.DownloadFailures)
	return true
}
