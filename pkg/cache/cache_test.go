package cache

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/internal/fileutil"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")

	st := New()
	entry := st.Entry("sample")
	entry.File = "sample.ipset"
	entry.Entries = 42
	entry.UniqueIPs = 100
	entry.Category = "attacks"
	entry.Hash = "abc123"

	if err := Save(path, st); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatalf("Stat returned error: %v", err)
	} else if got := info.Mode().Perm(); got != fileutil.GeneratedFileMode {
		t.Fatalf("cache file mode = %04o, want %04o", got, fileutil.GeneratedFileMode)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	e := loaded.Entries["sample"]
	if e == nil {
		t.Fatal("expected sample entry after load")
	}
	if e.File != "sample.ipset" {
		t.Fatalf("unexpected file: %q", e.File)
	}
	if e.Entries != 42 {
		t.Fatalf("unexpected entries: %d", e.Entries)
	}
	if e.UniqueIPs != 100 {
		t.Fatalf("unexpected unique IPs: %d", e.UniqueIPs)
	}
	if e.Category != "attacks" {
		t.Fatalf("unexpected category: %q", e.Category)
	}
	if e.Hash != "abc123" {
		t.Fatalf("unexpected hash: %q", e.Hash)
	}

	// SavedAt should be populated.
	if loaded.SavedAt.IsZero() {
		t.Fatal("expected non-zero SavedAt after load")
	}
}

func TestLoadNonExistentReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(st.Entries) != 0 {
		t.Fatalf("expected empty entries, got %d", len(st.Entries))
	}
}

func TestSaveNilState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nil.json")
	if err := Save(path, nil); err != nil {
		t.Fatalf("Save nil returned error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Entries) != 0 {
		t.Fatalf("expected empty entries, got %d", len(loaded.Entries))
	}
}

func TestEntryCreatesOnAbsent(t *testing.T) {
	st := New()
	entry := st.Entry("new_entry")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Name != "new_entry" {
		t.Fatalf("unexpected name: %q", entry.Name)
	}

	// Second call should return the same pointer.
	entry2 := st.Entry("new_entry")
	if entry != entry2 {
		t.Fatal("expected same entry pointer on second call")
	}
}

func TestEntryInitializesNilMap(t *testing.T) {
	st := &State{}
	entry := st.Entry("init")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if st.Entries == nil {
		t.Fatal("expected Entries map to be initialized")
	}
}

func TestReplaceEntryStoresCopyWithConfiguredName(t *testing.T) {
	st := New()
	replacement := Entry{
		Name:                 "wrong",
		File:                 "sample.ipset",
		Entries:              42,
		HistoryMinutes:       []int{60, 1440},
		CriticalOverlapTiers: []string{"hard", "soft"},
	}

	st.ReplaceEntry("sample", replacement)

	replacement.File = "changed.ipset"
	replacement.HistoryMinutes[0] = 30
	replacement.CriticalOverlapTiers[0] = "contextual"

	got := st.EntrySnapshot("sample")
	if got == nil {
		t.Fatal("expected replacement entry")
	}
	if got.Name != "sample" {
		t.Fatalf("replacement did not normalize name: %q", got.Name)
	}
	if got.File != "sample.ipset" {
		t.Fatalf("replacement entry aliased scalar data: %q", got.File)
	}
	if got.HistoryMinutes[0] != 60 {
		t.Fatalf("replacement entry aliased history slice: %v", got.HistoryMinutes)
	}
	if got.CriticalOverlapTiers[0] != "hard" {
		t.Fatalf("replacement entry aliased overlap tiers: %v", got.CriticalOverlapTiers)
	}
}

func TestReplaceEntryInitializesNilMap(t *testing.T) {
	st := &State{}
	st.ReplaceEntry("init", Entry{Entries: 7})

	got := st.EntrySnapshot("init")
	if got == nil {
		t.Fatal("expected replacement entry")
	}
	if got.Name != "init" || got.Entries != 7 {
		t.Fatalf("unexpected replacement entry: %+v", got)
	}
}

func TestEntryApplyArtifactConfig(t *testing.T) {
	entry := &Entry{Source: "old.source", File: "old.source", HistoryMinutes: []int{60}}
	entry.ApplyArtifactConfig(ArtifactConfigSnapshot{
		Name:          "artifact",
		URL:           "rsync://example.test/buildzone",
		Frequency:     120,
		Info:          "artifact info",
		Maintainer:    "artifact maintainer",
		MaintainerURL: "https://maintainer.test",
		Downloader:    "dronebl_buildzone",
		SourceFile:    "artifact.source",
	})

	if entry.Name != "artifact" {
		t.Fatalf("name = %q", entry.Name)
	}
	if entry.URL != "rsync://example.test/buildzone" {
		t.Fatalf("url = %q", entry.URL)
	}
	if entry.PublicURL != "" || entry.IPV != "" || entry.Hash != "" {
		t.Fatalf("artifact config should clear public URL/IPV/hash: %+v", entry)
	}
	if entry.FrequencyMinutes != 120 || entry.Category != "artifact" {
		t.Fatalf("unexpected artifact config fields: %+v", entry)
	}
	if entry.Downloader != "dronebl_buildzone" || entry.DownloaderOptions != "" {
		t.Fatalf("unexpected downloader fields: %+v", entry)
	}
	if entry.Source != "artifact.source" || entry.File != "artifact.source" {
		t.Fatalf("unexpected artifact files: source=%q file=%q", entry.Source, entry.File)
	}
	if entry.HistoryMinutes != nil {
		t.Fatalf("artifact history should be nil: %v", entry.HistoryMinutes)
	}
}

func TestEntryApplySourceConfigStoresCopyAndFallbackDownloader(t *testing.T) {
	history := []int{60, 1440}
	entry := &Entry{Source: "old.source", File: "old.netset"}
	entry.ApplySourceConfig(SourceConfigSnapshot{
		Name:                      "source",
		URL:                       "https://example.test/source.txt",
		PublicURL:                 "https://public.example.test/source.txt",
		IPV:                       "ipv4",
		Hash:                      "md5",
		Frequency:                 30,
		History:                   history,
		Category:                  "attacks",
		Info:                      "source info",
		Maintainer:                "source maintainer",
		MaintainerURL:             "https://maintainer.test",
		FallbackDownloader:        "curl",
		FallbackDownloaderOptions: "--compressed",
		License:                   "license",
		Attribution:               "attribution",
		SourceFile:                "source.source",
		FinalFile:                 "source.ipset",
	})
	history[0] = 5

	if entry.Name != "source" || entry.URL == "" || entry.PublicURL == "" {
		t.Fatalf("unexpected source identity fields: %+v", entry)
	}
	if entry.Downloader != "curl" || entry.DownloaderOptions != "--compressed" {
		t.Fatalf("fallback downloader not applied: %+v", entry)
	}
	if entry.HistoryMinutes[0] != 60 {
		t.Fatalf("history slice was aliased: %v", entry.HistoryMinutes)
	}
	if entry.Source != "source.source" || entry.File != "source.ipset" {
		t.Fatalf("unexpected source files: source=%q file=%q", entry.Source, entry.File)
	}
}

func TestEntryApplyProviderSourceConfig(t *testing.T) {
	entry := &Entry{Downloader: "existing-downloader", DownloaderOptions: "existing-options"}
	entry.ApplyProviderSourceConfig(ProviderSourceConfigSnapshot{
		Name:              "dbip_country",
		DefaultCategory:   "geolocation",
		Info:              "provider info",
		Maintainer:        "provider maintainer",
		MaintainerURL:     "https://provider.test",
		Frequency:         1440,
		URL:               "https://provider.test/db.csv.gz",
		Downloader:        "",
		DownloaderOptions: "",
	})

	if entry.Name != "dbip_country" || entry.Category != "geolocation" {
		t.Fatalf("unexpected provider identity fields: %+v", entry)
	}
	if entry.URL != "https://provider.test/db.csv.gz" || entry.PublicURL != "https://provider.test/db.csv.gz" {
		t.Fatalf("unexpected provider URLs: %+v", entry)
	}
	if entry.Downloader != "existing-downloader" || entry.DownloaderOptions != "existing-options" {
		t.Fatalf("empty provider downloader fields should preserve existing values: %+v", entry)
	}

	entry.ApplyProviderSourceConfig(ProviderSourceConfigSnapshot{
		Name:              "iptoasn",
		Category:          "asn_data",
		DefaultCategory:   "asn",
		Downloader:        "curl",
		DownloaderOptions: "--compressed",
	})
	if entry.Category != "asn_data" || entry.Downloader != "curl" || entry.DownloaderOptions != "--compressed" {
		t.Fatalf("explicit provider config not applied: %+v", entry)
	}
}

func TestEntryApplyArtifactDiskBootstrap(t *testing.T) {
	entry := &Entry{}
	entry.ApplyArtifactDiskBootstrap("artifact.source", 1700000000)

	if entry.SourceDate != 1700000000 || entry.ProcessedDate != 1700000000 || entry.CheckedDate != 1700000000 {
		t.Fatalf("unexpected artifact bootstrap dates: %+v", entry)
	}
	if entry.StartedDate != 1700000000 || entry.Version != 1 {
		t.Fatalf("unexpected artifact bootstrap lifecycle: %+v", entry)
	}
	if entry.Source != "artifact.source" || entry.File != "artifact.source" {
		t.Fatalf("unexpected artifact bootstrap files: %+v", entry)
	}
}

func TestEntryApplyHistoryBootstrapTimestamp(t *testing.T) {
	entry := &Entry{}
	entry.ApplyHistoryBootstrapTimestamp(1700000000)

	if entry.SourceDate != 1700000000 || entry.ProcessedDate != 1700000000 || entry.CheckedDate != 1700000000 {
		t.Fatalf("unexpected history bootstrap dates: %+v", entry)
	}
}

func TestEntryApplyDiskSetStatsAndFinalizeBootstrap(t *testing.T) {
	entry := &Entry{File: "source.ipset", SourceDate: 100, EntriesMin: 50, IPsMin: 50}
	entry.ApplyDiskSetStats(DiskSetStats{
		Entries:     10,
		UniqueIPs:   20,
		ModifiedAt:  200,
		ContentHash: "hash",
	})

	if entry.Entries != 10 || entry.UniqueIPs != 20 || entry.ContentHash != "hash" {
		t.Fatalf("unexpected set stats: %+v", entry)
	}
	if entry.EntriesMin != 10 || entry.EntriesMax != 10 || entry.IPsMin != 20 || entry.IPsMax != 20 {
		t.Fatalf("unexpected min/max stats: %+v", entry)
	}
	if entry.SourceDate != 200 || entry.ProcessedDate != 200 || entry.CheckedDate != 200 {
		t.Fatalf("unexpected freshness dates: %+v", entry)
	}
	if !entry.FinalizeDiskBootstrap(30) {
		t.Fatal("expected bootstrap evidence")
	}
	if entry.StartedDate != 200 || entry.Version != 1 {
		t.Fatalf("unexpected finalized lifecycle: %+v", entry)
	}
	if entry.AverageUpdateMins != 30 || entry.MinUpdateMins != 30 || entry.MaxUpdateMins != 30 {
		t.Fatalf("unexpected finalized frequency stats: %+v", entry)
	}
}

func TestEntryCriticalContentHashStats(t *testing.T) {
	entry := &Entry{ContentHash: "old", Entries: 1, UniqueIPs: 1}
	entry.ClearContentHash()
	if entry.ContentHash != "" {
		t.Fatalf("content hash not cleared: %q", entry.ContentHash)
	}

	entry.RefreshCriticalContentHashStats(DiskSetStats{Entries: 7, UniqueIPs: 6, ContentHash: "new"})
	if entry.ContentHash != "new" || entry.Entries != 7 || entry.UniqueIPs != 6 {
		t.Fatalf("critical content hash stats not refreshed: %+v", entry)
	}
}

func TestEntryProviderStatusTransitions(t *testing.T) {
	tests := []struct {
		name   string
		mark   func(*Entry, string)
		status string
	}{
		{
			name:   "config error",
			mark:   (*Entry).MarkProviderConfigError,
			status: "config_error",
		},
		{
			name:   "filesystem failure",
			mark:   (*Entry).MarkProviderFilesystemFailure,
			status: "failed",
		},
		{
			name:   "extract failed",
			mark:   (*Entry).MarkProviderExtractFailed,
			status: "extract_failed",
		},
		{
			name:   "unavailable",
			mark:   (*Entry).MarkProviderUnavailable,
			status: "unavailable",
		},
		{
			name:   "parse failed",
			mark:   (*Entry).MarkProviderParseFailed,
			status: "parse_failed",
		},
		{
			name:   "open failed",
			mark:   (*Entry).MarkProviderOpenFailed,
			status: "open_failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &Entry{LastStatus: "old", LastError: "old error"}
			tt.mark(entry, "provider failed")

			if entry.LastStatus != tt.status {
				t.Fatalf("last status = %q, want %q", entry.LastStatus, tt.status)
			}
			if entry.LastError != "provider failed" {
				t.Fatalf("last error = %q", entry.LastError)
			}
		})
	}

	entry := &Entry{LastStatus: "old", LastError: "old error"}
	entry.MarkProviderProcessing()
	if entry.LastStatus != "processing" {
		t.Fatalf("last status = %q, want processing", entry.LastStatus)
	}
	if entry.LastError != "old error" {
		t.Fatalf("processing should preserve existing last error, got %q", entry.LastError)
	}
}

func TestEntryRecordProviderLoadedUpdated(t *testing.T) {
	entry := &Entry{DownloadFailures: 0}
	stale := entry.RecordProviderLoaded(ProviderLoadStats{
		SourceUnix:       1700000300,
		ProcessedUnix:    1700000200,
		ClockSkewSeconds: 300,
		Entries:          12,
		UniqueIPs:        256,
	}, 1440, false)

	if stale {
		t.Fatal("fresh provider load should not be stale")
	}
	if entry.SourceDate != 1700000300 || entry.ProcessedDate != 1700000200 || entry.StartedDate != 1700000300 {
		t.Fatalf("unexpected provider dates: %+v", entry)
	}
	if entry.ClockSkewSeconds != 300 {
		t.Fatalf("clock skew = %d, want 300", entry.ClockSkewSeconds)
	}
	if entry.Entries != 12 || entry.UniqueIPs != 256 {
		t.Fatalf("unexpected provider stats: %+v", entry)
	}
	if entry.EntriesMin != 12 || entry.EntriesMax != 12 || entry.IPsMin != 256 || entry.IPsMax != 256 {
		t.Fatalf("unexpected provider min/max stats: %+v", entry)
	}
	if entry.Version != 1 || entry.AverageUpdateMins != 1440 || entry.MinUpdateMins != 1440 || entry.MaxUpdateMins != 1440 {
		t.Fatalf("unexpected provider update stats: %+v", entry)
	}
	if entry.LastStatus != "updated" || entry.LastError != "" {
		t.Fatalf("unexpected provider updated status: %+v", entry)
	}
}

func TestEntryRecordProviderLoadedStaleAfterFailure(t *testing.T) {
	entry := &Entry{DownloadFailures: 2, StartedDate: 100, LastError: "old error"}
	stale := entry.RecordProviderLoaded(ProviderLoadStats{
		SourceUnix:    1700000000,
		ProcessedUnix: 1700000100,
		Entries:       3,
		UniqueIPs:     4,
	}, 0, false)

	if !stale {
		t.Fatal("provider load from cached source after download failure should be stale")
	}
	if entry.StartedDate != 100 {
		t.Fatalf("existing started date should be preserved, got %d", entry.StartedDate)
	}
	if entry.ClockSkewSeconds != 0 {
		t.Fatalf("clock skew = %d, want 0", entry.ClockSkewSeconds)
	}
	if entry.LastStatus != "stale" {
		t.Fatalf("last status = %q, want stale", entry.LastStatus)
	}
	if entry.LastError != "download failed 2 time(s); using cached data" {
		t.Fatalf("last error = %q", entry.LastError)
	}
}

func TestEntryRecordProviderLoadedStagedSourceIsUpdatedAfterFailure(t *testing.T) {
	entry := &Entry{DownloadFailures: 2, LastError: "old error"}
	stale := entry.RecordProviderLoaded(ProviderLoadStats{
		SourceUnix:    1700000000,
		ProcessedUnix: 1700000100,
	}, 0, true)

	if stale {
		t.Fatal("staged provider source should be considered updated")
	}
	if entry.LastStatus != "updated" || entry.LastError != "" {
		t.Fatalf("unexpected staged provider status: %+v", entry)
	}
}

func TestEntryApplyProcessingSourceConfigStoresCopy(t *testing.T) {
	history := []int{60, 1440}
	entry := &Entry{}
	entry.ApplyProcessingSourceConfig(ProcessingSourceConfigSnapshot{
		Name:              "sample",
		Category:          "attacks",
		Info:              "source info",
		Maintainer:        "source maintainer",
		MaintainerURL:     "https://source.test",
		Frequency:         30,
		History:           history,
		Downloader:        "curl",
		DownloaderOptions: "--compressed",
		URL:               "https://source.test/feed.txt",
		PublicURL:         "https://public.test/feed.txt",
	})
	history[0] = 5

	if entry.Name != "sample" || entry.Category != "attacks" || entry.Info != "source info" {
		t.Fatalf("unexpected processing source metadata: %+v", entry)
	}
	if entry.FrequencyMinutes != 30 || entry.HistoryMinutes[0] != 60 {
		t.Fatalf("unexpected processing source schedule: %+v", entry)
	}
	if entry.Downloader != "curl" || entry.DownloaderOptions != "--compressed" {
		t.Fatalf("unexpected processing source downloader: %+v", entry)
	}
	if entry.URL != "https://source.test/feed.txt" || entry.PublicURL != "https://public.test/feed.txt" {
		t.Fatalf("unexpected processing source URLs: %+v", entry)
	}
}

func TestEntrySourceProcessingStatusTransitions(t *testing.T) {
	entry := &Entry{
		LastStatus:    "old",
		LastError:     "old error",
		SourceDate:    1699999900,
		ProcessedDate: 1699999950,
		StartedDate:   1699999900,
	}
	wantSourceDate := entry.SourceDate
	wantProcessedDate := entry.ProcessedDate
	wantStartedDate := entry.StartedDate
	entry.MarkSourceProcessingDisabled(1700000000)
	if entry.LastStatus != "disabled" || entry.LastError != "" || entry.CheckedDate != 1700000000 {
		t.Fatalf("unexpected disabled processing status: %+v", entry)
	}

	message := entry.MarkSourceProcessingMissingInput("/tmp/missing.ipset")
	if entry.LastStatus != "missing_input" || entry.LastError != message {
		t.Fatalf("unexpected missing-input status: %+v", entry)
	}
	if message != "feed body does not exist at /tmp/missing.ipset" {
		t.Fatalf("missing-input message = %q", message)
	}

	entry.MarkSourceProcessingStarted()
	if entry.LastStatus != "processing" {
		t.Fatalf("last status = %q, want processing", entry.LastStatus)
	}

	entry.MarkSourceParseFailed("parse failed")
	if entry.LastStatus != "parse_failed" || entry.LastError != "parse failed" {
		t.Fatalf("unexpected parse status: %+v", entry)
	}
	entry.MarkSourceFinalizeFailed("finalize failed")
	if entry.LastStatus != "finalize_failed" || entry.LastError != "finalize failed" {
		t.Fatalf("unexpected finalize status: %+v", entry)
	}
	entry.MarkSourceRetentionFailed("retention failed")
	if entry.LastStatus != "retention_failed" || entry.LastError != "retention failed" {
		t.Fatalf("unexpected retention status: %+v", entry)
	}
	if entry.SourceDate != wantSourceDate || entry.ProcessedDate != wantProcessedDate || entry.StartedDate != wantStartedDate {
		t.Fatalf("failure status transitions changed source dates: %+v", entry)
	}
}

func TestEntryApplyFinalizedSourceSetAndMetadata(t *testing.T) {
	entry := &Entry{}
	entry.ApplyFinalizedSourceSet(FinalizedSourceSetSnapshot{
		File:          "sample.ipset",
		Source:        "sample.source",
		IPV:           "ipv4",
		Hash:          "md5",
		ContentHash:   "content-hash",
		SourceUnix:    1700000000,
		ProcessedUnix: 1700000100,
		Entries:       5,
		UniqueIPs:     20,
	})

	if entry.File != "sample.ipset" || entry.Source != "sample.source" || entry.IPV != "ipv4" || entry.Hash != "md5" {
		t.Fatalf("unexpected finalized source identity: %+v", entry)
	}
	if entry.ContentHash != "content-hash" {
		t.Fatalf("content hash = %q", entry.ContentHash)
	}
	if entry.SourceDate != 1700000000 || entry.ProcessedDate != 1700000100 || entry.StartedDate != 1700000000 {
		t.Fatalf("unexpected finalized source dates: %+v", entry)
	}
	if entry.Entries != 5 || entry.UniqueIPs != 20 {
		t.Fatalf("unexpected finalized source stats: %+v", entry)
	}

	entry.ApplyFinalizedSourceMetadata(FinalizedSourceMetadataSnapshot{
		Category:         "attacks",
		Info:             "source info",
		Maintainer:       "source maintainer",
		MaintainerURL:    "https://source.test",
		License:          "license",
		Attribution:      "attribution",
		ClockSkewSeconds: 17,
	})
	if entry.Category != "attacks" || entry.License != "license" || entry.ClockSkewSeconds != 17 {
		t.Fatalf("unexpected finalized source metadata: %+v", entry)
	}
}

func TestEntryMarkSourceProcessingComplete(t *testing.T) {
	entry := &Entry{LastError: "old error"}
	entry.MarkSourceProcessingComplete(true)
	if entry.LastStatus != "empty" || entry.LastError != "" {
		t.Fatalf("unexpected empty completion status: %+v", entry)
	}

	entry.LastError = "old error"
	entry.MarkSourceProcessingComplete(false)
	if entry.LastStatus != "updated" || entry.LastError != "" {
		t.Fatalf("unexpected updated completion status: %+v", entry)
	}
}

func TestEntryRecordDownloadFailureStartsAndIncrements(t *testing.T) {
	entry := &Entry{}
	entry.RecordDownloadFailure(1700000000)

	if entry.DownloadFailures != 1 {
		t.Fatalf("download failures = %d, want 1", entry.DownloadFailures)
	}
	if entry.FailureStartedDate != 1700000000 {
		t.Fatalf("failure started date = %d", entry.FailureStartedDate)
	}

	entry.RecordDownloadFailure(1700000300)
	if entry.DownloadFailures != 2 {
		t.Fatalf("download failures = %d, want 2", entry.DownloadFailures)
	}
	if entry.FailureStartedDate != 1700000000 {
		t.Fatalf("failure started date changed: %d", entry.FailureStartedDate)
	}
}

func TestEntryRecordDownloadFailureRepairsMissingStart(t *testing.T) {
	entry := &Entry{DownloadFailures: 3}
	entry.RecordDownloadFailure(1700000600)

	if entry.DownloadFailures != 4 {
		t.Fatalf("download failures = %d, want 4", entry.DownloadFailures)
	}
	if entry.FailureStartedDate != 1700000600 {
		t.Fatalf("failure started date = %d", entry.FailureStartedDate)
	}
}

func TestEntryClearDownloadFailure(t *testing.T) {
	entry := &Entry{DownloadFailures: 3, FailureStartedDate: 1700000000}
	entry.ClearDownloadFailure()

	if entry.DownloadFailures != 0 {
		t.Fatalf("download failures = %d, want 0", entry.DownloadFailures)
	}
	if entry.FailureStartedDate != 0 {
		t.Fatalf("failure started date = %d, want 0", entry.FailureStartedDate)
	}
}

func TestEntryRecordLegacyFailureStart(t *testing.T) {
	entry := &Entry{DownloadFailures: 3}
	ok := entry.RecordLegacyFailureStart(1700000000)

	if !ok {
		t.Fatal("expected legacy failure start to be recorded")
	}
	if entry.DownloadFailures != 3 {
		t.Fatalf("download failures = %d, want unchanged 3", entry.DownloadFailures)
	}
	if entry.FailureStartedDate != 1700000000 {
		t.Fatalf("failure started date = %d, want 1700000000", entry.FailureStartedDate)
	}
}

func TestEntryRecordLegacyFailureStartRejectsEmptyTimestamp(t *testing.T) {
	entry := &Entry{FailureStartedDate: 1700000000}
	ok := entry.RecordLegacyFailureStart(0)

	if ok {
		t.Fatal("expected empty legacy failure start to be rejected")
	}
	if entry.FailureStartedDate != 1700000000 {
		t.Fatalf("failure started date changed: %d", entry.FailureStartedDate)
	}
}

func TestEntryMarkRunStarted(t *testing.T) {
	entry := &Entry{}
	entry.MarkRunStarted(runreason.ReasonManualRecheck)

	if entry.LastRunReason != runreason.ReasonManualRecheck {
		t.Fatalf("last run reason = %q", entry.LastRunReason)
	}
	if entry.LastStatus != "running" {
		t.Fatalf("last status = %q", entry.LastStatus)
	}
}

func TestEntryMarkDownloadStarted(t *testing.T) {
	entry := &Entry{LastStatus: "old", LastError: "old error"}
	entry.MarkDownloadStarted(1700000000)

	if entry.CheckedDate != 1700000000 {
		t.Fatalf("checked date = %d, want 1700000000", entry.CheckedDate)
	}
	if entry.LastStatus != DownloadStatusDownloading.String() {
		t.Fatalf("last status = %q", entry.LastStatus)
	}
	if entry.LastError != "" {
		t.Fatalf("last error = %q, want empty", entry.LastError)
	}
}

func TestEntryMarkArtifactChildMaterializing(t *testing.T) {
	entry := &Entry{LastStatus: "old", LastError: "old error"}
	entry.MarkArtifactChildMaterializing(1700000000)

	if entry.CheckedDate != 1700000000 {
		t.Fatalf("checked date = %d, want 1700000000", entry.CheckedDate)
	}
	if entry.LastStatus != DownloadStatusMaterializing.String() {
		t.Fatalf("last status = %q", entry.LastStatus)
	}
	if entry.LastError != "" {
		t.Fatalf("last error = %q, want empty", entry.LastError)
	}
}

func TestEntryMarkDownloadDisabled(t *testing.T) {
	entry := &Entry{LastStatus: "old", LastError: "old error"}
	entry.MarkDownloadDisabled(1700000000)

	if entry.CheckedDate != 1700000000 {
		t.Fatalf("checked date = %d, want 1700000000", entry.CheckedDate)
	}
	if entry.LastStatus != DownloadStatusDisabled.String() {
		t.Fatalf("last status = %q", entry.LastStatus)
	}
	if entry.LastError != "" {
		t.Fatalf("last error = %q, want empty", entry.LastError)
	}
}

func TestEntryMarkDownloadMissingEnv(t *testing.T) {
	entry := &Entry{}
	message := entry.MarkDownloadMissingEnv("https://example.test/${TOKEN}")

	if entry.LastStatus != DownloadStatusMissingEnv.String() {
		t.Fatalf("last status = %q", entry.LastStatus)
	}
	if entry.LastError != message {
		t.Fatalf("last error = %q, want %q", entry.LastError, message)
	}
	if message == "" {
		t.Fatal("expected missing-env message")
	}
}

func TestEntryRecordResolvedDownloadURL(t *testing.T) {
	entry := &Entry{URL: "old", PublicURL: "old-public"}
	entry.RecordResolvedDownloadURL("https://resolved.example/source")

	if entry.URL != "https://resolved.example/source" {
		t.Fatalf("url = %q", entry.URL)
	}
	if entry.PublicURL != "old-public" {
		t.Fatalf("public url = %q, want old-public", entry.PublicURL)
	}
}

func TestEntryRecordDownloadSourceDate(t *testing.T) {
	entry := &Entry{}
	modifiedAt := time.Unix(1700000000, 123).UTC()
	entry.RecordDownloadSourceDate(modifiedAt)

	if entry.SourceDate != 1700000000 {
		t.Fatalf("source date = %d, want 1700000000", entry.SourceDate)
	}

	entry.RecordDownloadSourceDate(time.Time{})
	if entry.SourceDate != 1700000000 {
		t.Fatalf("zero source date should not overwrite existing timestamp: %d", entry.SourceDate)
	}
}

func TestEntryMarkDownloadFailureStatuses(t *testing.T) {
	tests := []struct {
		name   string
		mark   func(*Entry, string)
		status DownloadStatus
	}{
		{
			name:   "fetch failed",
			mark:   (*Entry).MarkDownloadFetchFailed,
			status: DownloadStatusDownloadFailed,
		},
		{
			name:   "url resolve failed",
			mark:   (*Entry).MarkDownloadURLResolveFailed,
			status: DownloadStatusURLResolveFailed,
		},
		{
			name:   "operation failed",
			mark:   (*Entry).MarkDownloadOperationFailed,
			status: DownloadStatusFailed,
		},
		{
			name:   "prepare failed",
			mark:   (*Entry).MarkDownloadPrepareFailed,
			status: DownloadStatusPrepareFailed,
		},
		{
			name:   "history snapshot failed",
			mark:   (*Entry).MarkDownloadHistorySnapshotFailed,
			status: DownloadStatusHistorySnapshotFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &Entry{LastStatus: "old", LastError: "old error"}
			tt.mark(entry, "failed for test")

			if entry.LastStatus != tt.status.String() {
				t.Fatalf("last status = %q, want %q", entry.LastStatus, tt.status.String())
			}
			if entry.LastError != "failed for test" {
				t.Fatalf("last error = %q", entry.LastError)
			}
		})
	}
}

func TestEntryMarkDownloadSuccessStatuses(t *testing.T) {
	tests := []struct {
		name   string
		mark   func(*Entry)
		status DownloadStatus
	}{
		{
			name:   "not modified",
			mark:   (*Entry).MarkDownloadNotModified,
			status: DownloadStatusNotModified,
		},
		{
			name:   "same",
			mark:   (*Entry).MarkDownloadSame,
			status: DownloadStatusSame,
		},
		{
			name:   "downloaded",
			mark:   (*Entry).MarkDownloadDownloaded,
			status: DownloadStatusDownloaded,
		},
		{
			name:   "empty",
			mark:   (*Entry).MarkDownloadEmpty,
			status: DownloadStatusEmpty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &Entry{LastStatus: "old", LastError: "old error"}
			tt.mark(entry)

			if entry.LastStatus != tt.status.String() {
				t.Fatalf("last status = %q, want %q", entry.LastStatus, tt.status.String())
			}
			if entry.LastError != "" {
				t.Fatalf("last error = %q, want empty", entry.LastError)
			}
		})
	}
}

func TestEntryRecordProcessingDuration(t *testing.T) {
	entry := &Entry{}
	entry.RecordProcessingDuration(17)

	if entry.LastProcessingMS != 17 {
		t.Fatalf("last processing ms = %d", entry.LastProcessingMS)
	}
}

func TestEntrySetCriticalOverlapTiersStoresCopy(t *testing.T) {
	entry := &Entry{}
	tiers := []string{"hard", "soft"}

	entry.SetCriticalOverlapTiers(tiers)
	tiers[0] = "contextual"

	if entry.CriticalOverlapTiers[0] != "hard" {
		t.Fatalf("critical overlap tiers aliased input: %v", entry.CriticalOverlapTiers)
	}
}

func TestEntryClearCriticalOverlapTiers(t *testing.T) {
	entry := &Entry{CriticalOverlapTiers: []string{"hard"}}
	entry.ClearCriticalOverlapTiers()

	if entry.CriticalOverlapTiers != nil {
		t.Fatalf("critical overlap tiers = %v, want nil", entry.CriticalOverlapTiers)
	}
}

func TestEntrySetUniqueShareClampsPct(t *testing.T) {
	entry := &Entry{}
	entry.SetUniqueShare(120, 3)

	if entry.UniqueSharePct != 100 {
		t.Fatalf("unique share pct = %f, want 100", entry.UniqueSharePct)
	}
	if entry.UniqueShareSamples != 3 {
		t.Fatalf("unique share samples = %d, want 3", entry.UniqueShareSamples)
	}

	entry.SetUniqueShare(-7, 4)
	if entry.UniqueSharePct != 0 {
		t.Fatalf("unique share pct = %f, want 0", entry.UniqueSharePct)
	}
	if entry.UniqueShareSamples != 4 {
		t.Fatalf("unique share samples = %d, want 4", entry.UniqueShareSamples)
	}
}

func TestEntryRecordStatsUpdate(t *testing.T) {
	entry := &Entry{Entries: 10, UniqueIPs: 20}
	entry.RecordStatsUpdate(60)

	if entry.EntriesMin != 10 || entry.EntriesMax != 10 || entry.IPsMin != 20 || entry.IPsMax != 20 {
		t.Fatalf("unexpected first stats update: %+v", entry)
	}
	if entry.Version != 1 || entry.AverageUpdateMins != 60 || entry.MinUpdateMins != 60 || entry.MaxUpdateMins != 60 {
		t.Fatalf("unexpected first cadence update: %+v", entry)
	}

	entry.Entries = 15
	entry.UniqueIPs = 5
	entry.RecordStatsUpdate(30)
	if entry.EntriesMin != 10 || entry.EntriesMax != 15 || entry.IPsMin != 5 || entry.IPsMax != 20 {
		t.Fatalf("unexpected second stats update: %+v", entry)
	}
	if entry.Version != 2 || entry.AverageUpdateMins != 60 {
		t.Fatalf("unexpected second version/cadence update: %+v", entry)
	}
}

func TestEntryApplyHistoryLedgerStats(t *testing.T) {
	entry := &Entry{}
	ok := entry.ApplyHistoryLedgerStats(HistoryLedgerStatsSnapshot{
		Version:              3,
		StartedUnix:          1700000000,
		Entries:              15,
		UniqueIPs:            25,
		EntriesMin:           10,
		EntriesMax:           20,
		IPsMin:               5,
		IPsMax:               30,
		HistoryTotalGapSecs:  7200,
		HistoryMinGapSecs:    1800,
		HistoryMaxGapSecs:    5400,
		AverageUpdateMinutes: 60,
		MinUpdateMinutes:     30,
		MaxUpdateMinutes:     90,
	})

	if !ok {
		t.Fatal("expected history ledger stats apply to succeed")
	}
	if entry.Version != 3 || entry.StartedDate != 1700000000 {
		t.Fatalf("unexpected version/start: %+v", entry)
	}
	if entry.Entries != 15 || entry.UniqueIPs != 25 {
		t.Fatalf("unexpected current stats: %+v", entry)
	}
	if entry.EntriesMin != 10 || entry.EntriesMax != 20 || entry.IPsMin != 5 || entry.IPsMax != 30 {
		t.Fatalf("unexpected min/max stats: %+v", entry)
	}
	if entry.HistoryTotalGapSecs != 7200 || entry.HistoryMinGapSecs != 1800 || entry.HistoryMaxGapSecs != 5400 {
		t.Fatalf("unexpected history gaps: %+v", entry)
	}
	if entry.AverageUpdateMins != 60 || entry.MinUpdateMins != 30 || entry.MaxUpdateMins != 90 {
		t.Fatalf("unexpected cadence stats: %+v", entry)
	}
}

func TestEntryApplyHistoryLedgerStatsRejectsEmptySnapshot(t *testing.T) {
	entry := &Entry{Version: 2}
	ok := entry.ApplyHistoryLedgerStats(HistoryLedgerStatsSnapshot{})

	if ok {
		t.Fatal("expected empty history ledger snapshot to be rejected")
	}
	if entry.Version != 2 {
		t.Fatalf("entry was mutated by rejected snapshot: %+v", entry)
	}
}

func TestEntryRepairInvalidTimestampsUsesEvidence(t *testing.T) {
	invalid := int64(1_521_527_945_506)
	entry := &Entry{
		SourceDate:         invalid,
		ProcessedDate:      invalid,
		CheckedDate:        -1,
		StartedDate:        invalid,
		FailureStartedDate: invalid,
	}

	changed := entry.RepairInvalidTimestamps(TimestampRepairEvidence{
		LatestUnix: 1709584328,
		HaveLatest: true,
		FirstUnix:  1709549889,
		HaveFirst:  true,
	})

	if !changed {
		t.Fatal("expected invalid timestamps to be repaired")
	}
	if entry.SourceDate != 1709584328 || entry.ProcessedDate != 1709584328 {
		t.Fatalf("latest evidence not applied: %+v", entry)
	}
	if entry.CheckedDate != 1709584328 {
		t.Fatalf("checked date = %d, want repaired processed/latest timestamp", entry.CheckedDate)
	}
	if entry.StartedDate != 1709549889 {
		t.Fatalf("started date = %d, want first evidence timestamp", entry.StartedDate)
	}
	if entry.FailureStartedDate != 0 {
		t.Fatalf("failure started date = %d, want 0", entry.FailureStartedDate)
	}
}

func TestEntryRepairInvalidTimestampsFallsBackToEntryTimestamps(t *testing.T) {
	entry := &Entry{
		SourceDate:    1709584328,
		ProcessedDate: -1,
		CheckedDate:   -1,
		StartedDate:   -1,
	}

	changed := entry.RepairInvalidTimestamps(TimestampRepairEvidence{})

	if !changed {
		t.Fatal("expected invalid timestamps to be repaired")
	}
	if entry.ProcessedDate != 1709584328 || entry.CheckedDate != 1709584328 || entry.StartedDate != 1709584328 {
		t.Fatalf("expected repair fallback to source timestamp, got %+v", entry)
	}
}

func TestEntryRepairInvalidTimestampsRejectsInvalidEvidence(t *testing.T) {
	entry := &Entry{
		SourceDate:    -1,
		ProcessedDate: -1,
		CheckedDate:   -1,
		StartedDate:   -1,
	}

	changed := entry.RepairInvalidTimestamps(TimestampRepairEvidence{
		LatestUnix: 1_521_527_945_506,
		HaveLatest: true,
		FirstUnix:  -1,
		HaveFirst:  true,
	})

	if !changed {
		t.Fatal("expected invalid timestamps to be repaired")
	}
	if entry.SourceDate != 0 || entry.ProcessedDate != 0 || entry.CheckedDate != 0 || entry.StartedDate != 0 {
		t.Fatalf("invalid evidence was applied: %+v", entry)
	}
}

func TestEntryRepairInvalidTimestampsSkipsCleanEntry(t *testing.T) {
	entry := &Entry{
		SourceDate:         1709584328,
		ProcessedDate:      1709584328,
		CheckedDate:        1709590000,
		StartedDate:        1709549889,
		FailureStartedDate: 0,
	}

	changed := entry.RepairInvalidTimestamps(TimestampRepairEvidence{})

	if changed {
		t.Fatalf("clean entry was mutated: %+v", entry)
	}
}

func TestEntryApplyRotationStats(t *testing.T) {
	entry := &Entry{}
	entry.ApplyRotationStats(RotationStatsSnapshot{
		RotationMedianPct:    12.5,
		RotationP75Pct:       25,
		RotationSamples:      3,
		ChangeRatioMedianPct: 7.5,
		ChangeRatioP75Pct:    10,
		ChangeRatioSamples:   2,
	})

	if entry.RotationMedianPct != 12.5 || entry.RotationP75Pct != 25 || entry.RotationSamples != 3 {
		t.Fatalf("unexpected rotation stats: %+v", entry)
	}
	if entry.ChangeRatioMedianPct != 7.5 || entry.ChangeRatioP75Pct != 10 || entry.ChangeRatioSamples != 2 {
		t.Fatalf("unexpected change-ratio stats: %+v", entry)
	}
}

func TestEntryClearRotationStats(t *testing.T) {
	entry := &Entry{
		RotationMedianPct:    12.5,
		RotationP75Pct:       25,
		RotationSamples:      3,
		ChangeRatioMedianPct: 7.5,
		ChangeRatioP75Pct:    10,
		ChangeRatioSamples:   2,
	}
	entry.ClearRotationStats()

	if entry.RotationMedianPct != 0 || entry.RotationP75Pct != 0 || entry.RotationSamples != 0 {
		t.Fatalf("rotation stats not cleared: %+v", entry)
	}
	if entry.ChangeRatioMedianPct != 0 || entry.ChangeRatioP75Pct != 0 || entry.ChangeRatioSamples != 0 {
		t.Fatalf("change-ratio stats not cleared: %+v", entry)
	}
}

func TestRemove(t *testing.T) {
	st := New()
	st.Entry("to_remove")
	st.Remove("to_remove")

	if snap := st.EntrySnapshot("to_remove"); snap != nil {
		t.Fatal("expected nil snapshot after removal")
	}
}

func TestRemoveNonExistent(t *testing.T) {
	st := New()
	st.Remove("does_not_exist") // should not panic
}

func TestEntrySnapshotReturnsCopy(t *testing.T) {
	st := New()
	entry := st.Entry("original")
	entry.Entries = 100
	entry.Category = "malware"
	entry.HistoryMinutes = []int{60, 1440}
	entry.CriticalOverlapTiers = []string{"hard", "soft"}

	snap := st.EntrySnapshot("original")
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.Entries != 100 {
		t.Fatalf("snapshot entries mismatch: %d", snap.Entries)
	}

	// Modify the original — snapshot should be unaffected.
	entry.Entries = 999
	entry.Category = "changed"
	entry.HistoryMinutes[0] = 30
	entry.CriticalOverlapTiers[0] = "contextual"

	if snap.Entries != 100 {
		t.Fatalf("snapshot was mutated: entries=%d", snap.Entries)
	}
	if snap.Category != "malware" {
		t.Fatalf("snapshot was mutated: category=%q", snap.Category)
	}
	if snap.HistoryMinutes[0] != 60 {
		t.Fatalf("snapshot history slice was mutated: %v", snap.HistoryMinutes)
	}
	if snap.CriticalOverlapTiers[0] != "hard" {
		t.Fatalf("snapshot overlap tiers slice was mutated: %v", snap.CriticalOverlapTiers)
	}
}

func TestEntrySnapshotNonExistent(t *testing.T) {
	st := New()
	snap := st.EntrySnapshot("missing")
	if snap != nil {
		t.Fatal("expected nil snapshot for missing entry")
	}
}

func TestNames(t *testing.T) {
	st := New()
	st.Entry("bravo")
	st.Entry("alpha")
	st.Entry("charlie")

	names := st.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	// Names should be sorted.
	if names[0] != "alpha" || names[1] != "bravo" || names[2] != "charlie" {
		t.Fatalf("unexpected name order: %v", names)
	}
}

func TestNamesEmpty(t *testing.T) {
	st := New()
	names := st.Names()
	if len(names) != 0 {
		t.Fatalf("expected empty names, got %v", names)
	}
}

func TestSnapshotEntries(t *testing.T) {
	st := New()
	e1 := st.Entry("first")
	e1.Entries = 10
	e2 := st.Entry("second")
	e2.Entries = 20

	snap := st.SnapshotEntries()
	if len(snap) != 2 {
		t.Fatalf("expected 2 snapshot entries, got %d", len(snap))
	}
	if snap["first"].Entries != 10 {
		t.Fatalf("unexpected first entries: %d", snap["first"].Entries)
	}
	if snap["second"].Entries != 20 {
		t.Fatalf("unexpected second entries: %d", snap["second"].Entries)
	}

	// Modify original — snapshot should be unaffected.
	e1.Entries = 999
	if snap["first"].Entries != 10 {
		t.Fatal("snapshot entries were mutated")
	}
}

func TestSnapshotEntriesEmpty(t *testing.T) {
	st := New()
	snap := st.SnapshotEntries()
	if len(snap) != 0 {
		t.Fatalf("expected empty snapshot, got %d entries", len(snap))
	}
}

func TestRenameEntry(t *testing.T) {
	st := New()
	entry := st.Entry("old_name")
	entry.Entries = 42

	st.RenameEntry("old_name", "new_name")

	if snap := st.EntrySnapshot("old_name"); snap != nil {
		t.Fatal("old name should be removed after rename")
	}
	snap := st.EntrySnapshot("new_name")
	if snap == nil {
		t.Fatal("expected new_name to exist after rename")
	}
	if snap.Name != "new_name" {
		t.Fatalf("renamed entry Name field not updated: %q", snap.Name)
	}
	if snap.Entries != 42 {
		t.Fatalf("renamed entry lost data: entries=%d", snap.Entries)
	}
}

func TestRenameEntryNonExistent(t *testing.T) {
	st := New()
	st.RenameEntry("missing", "also_missing") // should not panic
}

func TestConcurrentEntryAndNames(t *testing.T) {
	st := New()
	const numGoroutines = 50
	const numOps = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	// Concurrent Entry() calls — each creates a unique entry.
	// Note: per the cache contract, concurrent mutations to the SAME entry
	// must be serialized by the caller. We use unique names per goroutine+op
	// to avoid that — the race we test here is concurrent map access via Entry().
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range numOps {
				name := "entry_" + itoa(id) + "_" + itoa(j)
				_ = st.Entry(name)
			}
		}(i)
	}

	// Concurrent Names() calls.
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range numOps {
				_ = st.Names()
			}
		}()
	}

	// Concurrent SnapshotEntries() calls.
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range numOps {
				_ = st.SnapshotEntries()
			}
		}()
	}

	wg.Wait()

	// After all goroutines finish, set the entry values sequentially.
	for i := range numGoroutines {
		for j := range numOps {
			name := "entry_" + itoa(i) + "_" + itoa(j)
			st.Entry(name).Entries = i*numOps + j
		}
	}

	names := st.Names()
	expected := numGoroutines * numOps
	if len(names) != expected {
		t.Fatalf("expected %d entries, got %d", expected, len(names))
	}
}

func TestConcurrentEntryCreationAndSnapshot(t *testing.T) {
	// This test exercises the mutex-protected operations concurrently:
	// Entry() (write lock) vs EntrySnapshot() and Names() (read lock).
	// Per the cache contract, mutations to the same entry's fields must be
	// serialized by the caller. So this test only exercises the map
	// operations, not field-level writes.
	st := New()

	var wg sync.WaitGroup
	const numWriters = 20
	const numReaders = 20
	const numOps = 100

	// Writer goroutines create new entries.
	for i := range numWriters {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numOps {
				name := "writer_" + itoa(id) + "_" + itoa(j)
				_ = st.Entry(name)
			}
		}(i)
	}

	// Reader goroutines call EntrySnapshot and Names.
	for range numReaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range numOps {
				_ = st.EntrySnapshot("writer_0_0")
				_ = st.Names()
			}
		}()
	}

	wg.Wait()

	expected := numWriters * numOps
	if got := len(st.Names()); got != expected {
		t.Fatalf("expected %d entries, got %d", expected, got)
	}
}

func TestVeryLongEntryName(t *testing.T) {
	st := New()
	longName := ""
	for range 1000 {
		longName += "a"
	}
	entry := st.Entry(longName)
	if entry.Name != longName {
		t.Fatal("long name not preserved")
	}
	snap := st.EntrySnapshot(longName)
	if snap == nil || snap.Name != longName {
		t.Fatal("snapshot failed for long name")
	}
}

func TestSaveLoadRoundTripPreservesAllFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "full.json")

	st := New()
	entry := st.Entry("full")
	entry.File = "full.ipset"
	entry.Source = "src"
	entry.URL = "https://example.test"
	entry.PublicURL = "https://public.test"
	entry.IPV = "ipv4"
	entry.Hash = "deadbeef"
	entry.FrequencyMinutes = 30
	entry.HistoryMinutes = []int{60, 1440}
	entry.Entries = 500
	entry.UniqueIPs = 250
	entry.SourceDate = 1700000000
	entry.ProcessedDate = 1700000100
	entry.CheckedDate = 1700000200
	entry.StartedDate = 1700000300
	entry.Category = "attacks"
	entry.Info = "test info"
	entry.Maintainer = "tester"
	entry.MaintainerURL = "https://tester.test"
	entry.EntriesMin = 10
	entry.EntriesMax = 1000
	entry.IPsMin = 5
	entry.IPsMax = 500
	entry.ClockSkewSeconds = 3
	entry.DownloadFailures = 1
	entry.Version = 2
	entry.AverageUpdateMins = 15
	entry.MinUpdateMins = 5
	entry.MaxUpdateMins = 60
	entry.Downloader = "curl"
	entry.DownloaderOptions = "--compressed"
	entry.LastError = ""
	entry.LastStatus = "ok"

	if err := Save(path, st); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	got := loaded.Entries["full"]
	if got == nil {
		t.Fatal("expected full entry after load")
	}
	if got.File != "full.ipset" {
		t.Fatalf("File mismatch: %q", got.File)
	}
	if got.FrequencyMinutes != 30 {
		t.Fatalf("FrequencyMinutes mismatch: %d", got.FrequencyMinutes)
	}
	if len(got.HistoryMinutes) != 2 || got.HistoryMinutes[0] != 60 || got.HistoryMinutes[1] != 1440 {
		t.Fatalf("HistoryMinutes mismatch: %v", got.HistoryMinutes)
	}
	if got.Downloader != "curl" {
		t.Fatalf("Downloader mismatch: %q", got.Downloader)
	}
	if got.LastStatus != "ok" {
		t.Fatalf("LastStatus mismatch: %q", got.LastStatus)
	}
}

// itoa is a minimal int-to-string for test use, avoiding strconv import.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
