package cache

import (
	"time"

	"github.com/firehol/update-ipsets/pkg/runreason"
)

// MarkSourceProcessingDisabled records an ordinary source skipped because it is
// disabled for this run.
func (entry *Entry) MarkSourceProcessingDisabled(checkedUnix int64) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "disabled"
	entry.LastError = ""
	entry.CheckedDate = checkedUnix
}

// MarkSourceProcessingMissingInput records a missing prepared feed body.
func (entry *Entry) MarkSourceProcessingMissingInput(path string) string {
	if entry == nil {
		return ""
	}
	unlock := entry.lockEntry()
	defer unlock()
	message := "feed body does not exist at " + path
	entry.LastStatus = "missing_input"
	entry.LastError = message
	return message
}

// MarkSourceProcessingStarted records that ordinary source parsing/finalization
// has started.
func (entry *Entry) MarkSourceProcessingStarted() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "processing"
}

// MarkSourceParseFailed records a failed ordinary source parse.
func (entry *Entry) MarkSourceParseFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "parse_failed"
	entry.LastError = message
}

// MarkSourceFinalizeFailed records a failed ordinary source finalization.
func (entry *Entry) MarkSourceFinalizeFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "finalize_failed"
	entry.LastError = message
}

// MarkSourceRetentionFailed records a failed retention update after source
// finalization.
func (entry *Entry) MarkSourceRetentionFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = "retention_failed"
	entry.LastError = message
}

// ApplyFinalizedSourceSet stores finalized ordinary source set evidence.
func (entry *Entry) ApplyFinalizedSourceSet(snapshot FinalizedSourceSetSnapshot) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.File = snapshot.File
	entry.Source = snapshot.Source
	entry.IPV = snapshot.IPV
	entry.Hash = snapshot.Hash
	entry.ContentHash = snapshot.ContentHash
	entry.SourceDate = snapshot.SourceUnix
	entry.ProcessedDate = snapshot.ProcessedUnix
	if entry.StartedDate == 0 {
		entry.StartedDate = entry.SourceDate
	}
	entry.Entries = snapshot.Entries
	entry.UniqueIPs = snapshot.UniqueIPs
}

// ApplyFinalizedSourceMetadata stores authored metadata and clock-skew evidence
// after finalized-source history bookkeeping.
func (entry *Entry) ApplyFinalizedSourceMetadata(snapshot FinalizedSourceMetadataSnapshot) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.Category = snapshot.Category
	entry.Info = snapshot.Info
	entry.Maintainer = snapshot.Maintainer
	entry.MaintainerURL = snapshot.MaintainerURL
	entry.License = snapshot.License
	entry.Attribution = snapshot.Attribution
	entry.ClockSkewSeconds = snapshot.ClockSkewSeconds
}

// MarkSourceProcessingComplete records successful ordinary source completion.
func (entry *Entry) MarkSourceProcessingComplete(empty bool) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastError = ""
	if empty {
		entry.LastStatus = "empty"
		return
	}
	entry.LastStatus = "updated"
}

// ApplyHistoryLedgerStats stores the current aggregate state derived from the
// feed's append-only history ledger.
func (entry *Entry) ApplyHistoryLedgerStats(snapshot HistoryLedgerStatsSnapshot) bool {
	if entry == nil || snapshot.Version <= 0 {
		return false
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.Version = snapshot.Version
	entry.StartedDate = snapshot.StartedUnix
	entry.Entries = snapshot.Entries
	entry.UniqueIPs = snapshot.UniqueIPs
	entry.EntriesMin = snapshot.EntriesMin
	entry.EntriesMax = snapshot.EntriesMax
	entry.IPsMin = snapshot.IPsMin
	entry.IPsMax = snapshot.IPsMax
	entry.HistoryTotalGapSecs = snapshot.HistoryTotalGapSecs
	entry.HistoryMinGapSecs = snapshot.HistoryMinGapSecs
	entry.HistoryMaxGapSecs = snapshot.HistoryMaxGapSecs
	entry.AverageUpdateMins = snapshot.AverageUpdateMinutes
	entry.MinUpdateMins = snapshot.MinUpdateMinutes
	entry.MaxUpdateMins = snapshot.MaxUpdateMinutes
	return true
}

// ValidJSONUnixSeconds reports whether ts can be serialized and interpreted as
// a sane JSON Unix timestamp by update-ipsets cache readers.
func ValidJSONUnixSeconds(ts int64) bool {
	return ts > 0 && ts <= maxJSONUnixSeconds
}

// InvalidJSONUnixSeconds reports whether ts must be repaired before the cache
// entry is safely serialized.
func InvalidJSONUnixSeconds(ts int64) bool {
	return ts < 0 || ts > maxJSONUnixSeconds
}

// RepairInvalidTimestamps repairs invalid persisted timestamps using available
// disk/history evidence. It leaves clean timestamps unchanged.
func (entry *Entry) RepairInvalidTimestamps(evidence TimestampRepairEvidence) bool {
	if entry == nil {
		return false
	}
	unlock := entry.lockEntry()
	defer unlock()
	haveLatest := evidence.HaveLatest && ValidJSONUnixSeconds(evidence.LatestUnix)
	haveFirst := evidence.HaveFirst && ValidJSONUnixSeconds(evidence.FirstUnix)
	changed := false
	if InvalidJSONUnixSeconds(entry.SourceDate) {
		if haveLatest {
			entry.SourceDate = evidence.LatestUnix
		} else {
			entry.SourceDate = 0
		}
		changed = true
	}
	if InvalidJSONUnixSeconds(entry.ProcessedDate) {
		switch {
		case haveLatest:
			entry.ProcessedDate = evidence.LatestUnix
		case ValidJSONUnixSeconds(entry.SourceDate):
			entry.ProcessedDate = entry.SourceDate
		default:
			entry.ProcessedDate = 0
		}
		changed = true
	}
	if InvalidJSONUnixSeconds(entry.CheckedDate) {
		switch {
		case ValidJSONUnixSeconds(entry.FailureStartedDate):
			entry.CheckedDate = entry.FailureStartedDate
		case ValidJSONUnixSeconds(entry.ProcessedDate):
			entry.CheckedDate = entry.ProcessedDate
		case ValidJSONUnixSeconds(entry.SourceDate):
			entry.CheckedDate = entry.SourceDate
		case haveLatest:
			entry.CheckedDate = evidence.LatestUnix
		default:
			entry.CheckedDate = 0
		}
		changed = true
	}
	if InvalidJSONUnixSeconds(entry.StartedDate) {
		switch {
		case haveFirst:
			entry.StartedDate = evidence.FirstUnix
		case ValidJSONUnixSeconds(entry.SourceDate):
			entry.StartedDate = entry.SourceDate
		case ValidJSONUnixSeconds(entry.ProcessedDate):
			entry.StartedDate = entry.ProcessedDate
		default:
			entry.StartedDate = 0
		}
		changed = true
	}
	if InvalidJSONUnixSeconds(entry.FailureStartedDate) {
		entry.FailureStartedDate = 0
		changed = true
	}
	return changed
}

// ApplyRotationStats stores the current rotation/change-ratio summary.
func (entry *Entry) ApplyRotationStats(snapshot RotationStatsSnapshot) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.RotationMedianPct = snapshot.RotationMedianPct
	entry.RotationP75Pct = snapshot.RotationP75Pct
	entry.RotationSamples = snapshot.RotationSamples
	entry.ChangeRatioMedianPct = snapshot.ChangeRatioMedianPct
	entry.ChangeRatioP75Pct = snapshot.ChangeRatioP75Pct
	entry.ChangeRatioSamples = snapshot.ChangeRatioSamples
}

// ClearRotationStats clears rotation/change-ratio summary fields when there is
// not enough history to compute them.
func (entry *Entry) ClearRotationStats() {
	entry.ApplyRotationStats(RotationStatsSnapshot{})
}

// RecordDownloadFailure advances the failure lifecycle for a failed fetch.
func (entry *Entry) RecordDownloadFailure(nowUnix int64) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	if entry.DownloadFailures == 0 || entry.FailureStartedDate == 0 {
		entry.FailureStartedDate = nowUnix
	}
	entry.DownloadFailures++
}

// RecordLegacyFailureStart stores an imported legacy failure start timestamp
// without changing the current failure counter.
func (entry *Entry) RecordLegacyFailureStart(legacyCheckedUnix int64) bool {
	if entry == nil || legacyCheckedUnix <= 0 {
		return false
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.FailureStartedDate = legacyCheckedUnix
	return true
}

// ClearDownloadFailure resets the failure lifecycle after a successful fetch.
func (entry *Entry) ClearDownloadFailure() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.DownloadFailures = 0
	entry.FailureStartedDate = 0
}

// MarkRunStarted records the visible lifecycle state for a feed attempt.
func (entry *Entry) MarkRunStarted(reason runreason.Reason) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastRunReason = reason
	entry.LastStatus = "running"
}

// MarkDownloadStarted records the beginning of a download-stage attempt.
func (entry *Entry) MarkDownloadStarted(checkedUnix int64) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.CheckedDate = checkedUnix
	entry.LastStatus = DownloadStatusDownloading.String()
	entry.LastError = ""
}

// MarkArtifactChildMaterializing records an artifact-derived child source being
// regenerated from its parent artifact.
func (entry *Entry) MarkArtifactChildMaterializing(checkedUnix int64) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.CheckedDate = checkedUnix
	entry.LastStatus = DownloadStatusMaterializing.String()
	entry.LastError = ""
}

// MarkDownloadDisabled records a skipped attempt because the source is disabled.
func (entry *Entry) MarkDownloadDisabled(checkedUnix int64) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.CheckedDate = checkedUnix
	entry.LastStatus = DownloadStatusDisabled.String()
	entry.LastError = ""
}

// MarkDownloadMissingEnv records an unset environment variable in a URL template.
func (entry *Entry) MarkDownloadMissingEnv(template string) string {
	if entry == nil {
		return ""
	}
	unlock := entry.lockEntry()
	defer unlock()
	message := "URL template references an unset environment variable: " + template
	entry.LastStatus = DownloadStatusMissingEnv.String()
	entry.LastError = message
	return message
}

// RecordResolvedDownloadURL stores the concrete URL after provider indirection.
func (entry *Entry) RecordResolvedDownloadURL(resolved string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.URL = resolved
}

// RecordDownloadSourceDate stores the source-observed timestamp for a download.
func (entry *Entry) RecordDownloadSourceDate(modifiedAt time.Time) {
	if entry == nil || modifiedAt.IsZero() {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.SourceDate = modifiedAt.Unix()
}

// MarkDownloadFetchFailed records a failed upstream fetch result.
func (entry *Entry) MarkDownloadFetchFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusDownloadFailed.String()
	entry.LastError = message
}

// MarkDownloadURLResolveFailed records a failed provider URL resolution.
func (entry *Entry) MarkDownloadURLResolveFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusURLResolveFailed.String()
	entry.LastError = message
}

// MarkDownloadOperationFailed records a local download-stage operation failure.
func (entry *Entry) MarkDownloadOperationFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusFailed.String()
	entry.LastError = message
}

// MarkDownloadPrepareFailed records a failed canonical body preparation.
func (entry *Entry) MarkDownloadPrepareFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusPrepareFailed.String()
	entry.LastError = message
}

// MarkDownloadHistorySnapshotFailed records a failed history snapshot append.
func (entry *Entry) MarkDownloadHistorySnapshotFailed(message string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusHistorySnapshotFailed.String()
	entry.LastError = message
}

// MarkDownloadNotModified records an upstream not-modified result.
func (entry *Entry) MarkDownloadNotModified() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusNotModified.String()
	entry.LastError = ""
}

// MarkDownloadSame records a same-content download-stage result.
func (entry *Entry) MarkDownloadSame() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusSame.String()
	entry.LastError = ""
}

// MarkDownloadDownloaded records a staged non-empty download result.
func (entry *Entry) MarkDownloadDownloaded() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusDownloaded.String()
	entry.LastError = ""
}

// MarkDownloadEmpty records a staged empty download result.
func (entry *Entry) MarkDownloadEmpty() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastStatus = DownloadStatusEmpty.String()
	entry.LastError = ""
}

// RecordProcessingDuration stores the observed processing duration in millis.
func (entry *Entry) RecordProcessingDuration(ms int64) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.LastProcessingMS = ms
}

// SetCriticalOverlapTiers stores the current critical-overlap tier summary.
func (entry *Entry) SetCriticalOverlapTiers(tiers []string) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.CriticalOverlapTiers = append([]string(nil), tiers...)
}

// ClearCriticalOverlapTiers clears the current critical-overlap tier summary.
func (entry *Entry) ClearCriticalOverlapTiers() {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.CriticalOverlapTiers = nil
}

// SetUniqueShare stores the bounded unique-share comparison summary.
func (entry *Entry) SetUniqueShare(pct float64, samples int) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	entry.UniqueSharePct = pct
	entry.UniqueShareSamples = samples
}

// RecordStatsUpdate updates min/max bounds, version, and first-run cadence
// stats after Entries and UniqueIPs have been populated.
func (entry *Entry) RecordStatsUpdate(frequency int) {
	if entry == nil {
		return
	}
	unlock := entry.lockEntry()
	defer unlock()
	entry.recordStatsUpdateLocked(frequency)
}

func (entry *Entry) recordStatsUpdateLocked(frequency int) {
	if entry.EntriesMin == 0 || entry.Entries < entry.EntriesMin {
		entry.EntriesMin = entry.Entries
	}
	if entry.Entries > entry.EntriesMax {
		entry.EntriesMax = entry.Entries
	}
	if entry.IPsMin == 0 || entry.UniqueIPs < entry.IPsMin {
		entry.IPsMin = entry.UniqueIPs
	}
	if entry.UniqueIPs > entry.IPsMax {
		entry.IPsMax = entry.UniqueIPs
	}
	entry.Version++
	if frequency > 0 && entry.AverageUpdateMins == 0 {
		entry.AverageUpdateMins = frequency
		entry.MinUpdateMins = frequency
		entry.MaxUpdateMins = frequency
	}
}
