package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/runreason"

	"go.opentelemetry.io/otel/attribute"
)

const maxJSONUnixSeconds int64 = 253402300799

type State struct {
	SavedAt time.Time         `json:"saved_at"`
	Entries map[string]*Entry `json:"entries"`
	mu      sync.RWMutex
}

type Entry struct {
	mu *sync.RWMutex

	Name      string `json:"name"`
	File      string `json:"file,omitempty"`
	Source    string `json:"source,omitempty"`
	URL       string `json:"url,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
	IPV       string `json:"ipv,omitempty"`
	Hash      string `json:"hash,omitempty"`
	// ContentHash is populated only for reference sets that need stable
	// processed-content identity across local reprocessing timestamps.
	ContentHash          string           `json:"content_hash,omitempty"`
	FrequencyMinutes     int              `json:"frequency_minutes,omitempty"`
	HistoryMinutes       []int            `json:"history_minutes,omitempty"`
	Entries              int              `json:"entries,omitempty"`
	UniqueIPs            uint64           `json:"unique_ips,omitempty"`
	SourceDate           int64            `json:"source_date,omitempty"`
	ProcessedDate        int64            `json:"processed_date,omitempty"`
	CheckedDate          int64            `json:"checked_date,omitempty"`
	StartedDate          int64            `json:"started_date,omitempty"`
	Category             string           `json:"category,omitempty"`
	Info                 string           `json:"info,omitempty"`
	Maintainer           string           `json:"maintainer,omitempty"`
	MaintainerURL        string           `json:"maintainer_url,omitempty"`
	EntriesMin           int              `json:"entries_min,omitempty"`
	EntriesMax           int              `json:"entries_max,omitempty"`
	IPsMin               uint64           `json:"ips_min,omitempty"`
	IPsMax               uint64           `json:"ips_max,omitempty"`
	ClockSkewSeconds     int64            `json:"clock_skew_seconds,omitempty"`
	DownloadFailures     int              `json:"download_failures,omitempty"`
	FailureStartedDate   int64            `json:"failure_started_date,omitempty"`
	Version              int              `json:"version,omitempty"`
	AverageUpdateMins    int              `json:"average_update_mins,omitempty"`
	MinUpdateMins        int              `json:"min_update_mins,omitempty"`
	MaxUpdateMins        int              `json:"max_update_mins,omitempty"`
	HistoryTotalGapSecs  int64            `json:"history_total_gap_secs,omitempty"`
	HistoryMinGapSecs    int64            `json:"history_min_gap_secs,omitempty"`
	HistoryMaxGapSecs    int64            `json:"history_max_gap_secs,omitempty"`
	RotationMedianPct    float64          `json:"rotation_median_pct,omitempty"`
	RotationP75Pct       float64          `json:"rotation_p75_pct,omitempty"`
	RotationSamples      int              `json:"rotation_samples,omitempty"`
	ChangeRatioMedianPct float64          `json:"change_ratio_median_pct,omitempty"`
	ChangeRatioP75Pct    float64          `json:"change_ratio_p75_pct,omitempty"`
	ChangeRatioSamples   int              `json:"change_ratio_samples,omitempty"`
	Downloader           string           `json:"downloader,omitempty"`
	DownloaderOptions    string           `json:"downloader_options,omitempty"`
	License              string           `json:"license,omitempty"`
	Attribution          string           `json:"attribution,omitempty"`
	LastError            string           `json:"last_error,omitempty"`
	LastStatus           string           `json:"last_status,omitempty"`
	LastRunReason        runreason.Reason `json:"last_run_reason,omitempty"`
	LastProcessingMS     int64            `json:"last_processing_ms,omitempty"`
	UniqueSharePct       float64          `json:"unique_share_pct,omitempty"`
	UniqueShareSamples   int              `json:"unique_share_samples,omitempty"`
	CriticalOverlapTiers []string         `json:"critical_overlap_tiers,omitempty"`
}

func newEntry(name string) *Entry {
	return &Entry{Name: name, mu: new(sync.RWMutex)}
}

func (entry *Entry) entryMu() *sync.RWMutex {
	if entry.mu == nil {
		entry.mu = new(sync.RWMutex)
	}
	return entry.mu
}

func (entry *Entry) lockEntry() func() {
	mu := entry.entryMu()
	mu.Lock()
	return mu.Unlock
}

// ArtifactConfigSnapshot is the cache-relevant authored metadata for an artifact.
type ArtifactConfigSnapshot struct {
	Name          string
	URL           string
	Frequency     int
	Info          string
	Maintainer    string
	MaintainerURL string
	Downloader    string
	SourceFile    string
}

// SourceConfigSnapshot is the cache-relevant authored metadata for a source.
type SourceConfigSnapshot struct {
	Name                      string
	URL                       string
	PublicURL                 string
	IPV                       string
	Hash                      string
	Frequency                 int
	History                   []int
	Category                  string
	Info                      string
	Maintainer                string
	MaintainerURL             string
	Downloader                string
	DownloaderOptions         string
	FallbackDownloader        string
	FallbackDownloaderOptions string
	License                   string
	Attribution               string
	SourceFile                string
	FinalFile                 string
}

// ProviderSourceConfigSnapshot is the cache-relevant authored metadata for a
// supporting ASN/geolocation provider source.
type ProviderSourceConfigSnapshot struct {
	Name              string
	Category          string
	DefaultCategory   string
	Info              string
	Maintainer        string
	MaintainerURL     string
	Frequency         int
	URL               string
	Downloader        string
	DownloaderOptions string
}

// ProcessingSourceConfigSnapshot is the cache-relevant authored metadata for a
// source entering the ordinary processing pipeline.
type ProcessingSourceConfigSnapshot struct {
	Name              string
	Category          string
	Info              string
	Maintainer        string
	MaintainerURL     string
	Frequency         int
	History           []int
	Downloader        string
	DownloaderOptions string
	URL               string
	PublicURL         string
}

// DiskSetStats is the cache-relevant evidence from a restored local set.
type DiskSetStats struct {
	Entries     int
	UniqueIPs   uint64
	ModifiedAt  int64
	ContentHash string
}

// ProviderLoadStats is the cache-relevant evidence from a loaded supporting
// ASN/geolocation provider.
type ProviderLoadStats struct {
	SourceUnix       int64
	ProcessedUnix    int64
	ClockSkewSeconds int64
	Entries          int
	UniqueIPs        uint64
}

// FinalizedSourceSetSnapshot is the cache-relevant evidence from a finalized
// ordinary source set.
type FinalizedSourceSetSnapshot struct {
	File          string
	Source        string
	IPV           string
	Hash          string
	ContentHash   string
	SourceUnix    int64
	ProcessedUnix int64
	Entries       int
	UniqueIPs     uint64
}

// FinalizedSourceMetadataSnapshot is the authored metadata and timing evidence
// applied after finalized-source history bookkeeping.
type FinalizedSourceMetadataSnapshot struct {
	Category         string
	Info             string
	Maintainer       string
	MaintainerURL    string
	License          string
	Attribution      string
	ClockSkewSeconds int64
}

// HistoryLedgerStatsSnapshot is the cache-relevant state derived from a feed's
// append-only history ledger.
type HistoryLedgerStatsSnapshot struct {
	Version              int
	StartedUnix          int64
	Entries              int
	UniqueIPs            uint64
	EntriesMin           int
	EntriesMax           int
	IPsMin               uint64
	IPsMax               uint64
	HistoryTotalGapSecs  int64
	HistoryMinGapSecs    int64
	HistoryMaxGapSecs    int64
	AverageUpdateMinutes int
	MinUpdateMinutes     int
	MaxUpdateMinutes     int
}

// TimestampRepairEvidence is the disk/history evidence available when repairing
// invalid persisted entry timestamps.
type TimestampRepairEvidence struct {
	LatestUnix int64
	HaveLatest bool
	FirstUnix  int64
	HaveFirst  bool
}

// RotationStatsSnapshot is the cache-relevant rotation and change-ratio summary
// computed from feed size/churn history.
type RotationStatsSnapshot struct {
	RotationMedianPct    float64
	RotationP75Pct       float64
	RotationSamples      int
	ChangeRatioMedianPct float64
	ChangeRatioP75Pct    float64
	ChangeRatioSamples   int
}

func New() *State {
	return &State{
		Entries: map[string]*Entry{},
	}
}

func Load(path string) (*State, error) {
	started := time.Now()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			observeRuntimeCacheOperation("load", "empty", time.Since(started))
			return New(), nil
		}
		observeRuntimeCacheOperation("load", "error", time.Since(started))
		return nil, err
	}
	defer func() {
		observeRuntimeCacheOperation("load", "ok", time.Since(started))
	}()

	st := New()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, err
	}
	if st.Entries == nil {
		st.Entries = map[string]*Entry{}
	}
	for _, entry := range st.Entries {
		if entry != nil && entry.mu == nil {
			entry.mu = new(sync.RWMutex)
		}
	}
	return st, nil
}

func (st *State) MarshalJSON() ([]byte, error) {
	type stateJSON struct {
		SavedAt time.Time        `json:"saved_at"`
		Entries map[string]Entry `json:"entries"`
	}
	if st == nil {
		return json.Marshal(stateJSON{Entries: map[string]Entry{}})
	}
	st.mu.RLock()
	savedAt := st.SavedAt
	entries := make(map[string]Entry, len(st.Entries))
	for name, entry := range st.Entries {
		if entry != nil {
			entries[name] = cloneEntry(entry)
		}
	}
	st.mu.RUnlock()
	return json.Marshal(stateJSON{SavedAt: savedAt, Entries: entries})
}

func Save(path string, st *State) error {
	started := time.Now()
	var opErr error
	defer func() {
		result := "ok"
		if opErr != nil {
			result = "error"
		}
		observeRuntimeCacheOperation("save", result, time.Since(started))
	}()
	if st == nil {
		st = New()
	}
	if st.Entries == nil {
		st.Entries = map[string]*Entry{}
	}
	st.mu.Lock()
	st.SavedAt = time.Now().UTC()
	st.mu.Unlock()

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		opErr = err
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		opErr = err
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cache-*.json")
	if err != nil {
		opErr = err
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		opErr = err
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		opErr = err
		return err
	}
	if err := tmp.Close(); err != nil {
		opErr = err
		return err
	}
	opErr = os.Rename(tmpName, path)
	return opErr
}

func observeRuntimeCacheOperation(operation, result string, dur time.Duration) {
	if operation == "" {
		operation = "unknown"
	}
	if result == "" {
		result = "unknown"
	}
	attrs := []attribute.KeyValue{
		attribute.String("cache.operation", operation),
		attribute.String("cache.result", result),
	}
	ctx := observability.BackgroundContext()
	observability.Count(ctx, "runtime.cache.operations", 1, attrs...)
	observability.Duration(ctx, "runtime.cache.operation", dur, attrs...)
}

// Entry returns the entry for name, creating it if absent.
// The returned pointer carries entry-level locking for lifecycle methods.
// Direct field writes bypass that lock and must remain caller-serialized.
func (st *State) Entry(name string) *Entry {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Entries == nil {
		st.Entries = map[string]*Entry{}
	}
	entry := st.Entries[name]
	if entry == nil {
		entry = newEntry(name)
		st.Entries[name] = entry
	}
	return entry
}

// ReplaceEntry atomically stores entry under name.
//
// This is for cache lifecycle paths that need to replace a complete synthesized
// entry, such as disk bootstrap or timestamp repair. Field-level lifecycle
// updates should use narrower semantic transitions, not full replacement.
func (st *State) ReplaceEntry(name string, entry Entry) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Entries == nil {
		st.Entries = map[string]*Entry{}
	}
	clone := entry
	clone.mu = new(sync.RWMutex)
	if entry.HistoryMinutes != nil {
		clone.HistoryMinutes = append([]int(nil), entry.HistoryMinutes...)
	}
	if entry.CriticalOverlapTiers != nil {
		clone.CriticalOverlapTiers = append([]string(nil), entry.CriticalOverlapTiers...)
	}
	clone.Name = name
	st.Entries[name] = &clone
}

func cloneEntry(entry *Entry) Entry {
	if entry == nil {
		return Entry{}
	}
	entryMu := entry.entryMu()
	entryMu.RLock()
	defer entryMu.RUnlock()
	clone := *entry
	clone.mu = new(sync.RWMutex)
	if entry.HistoryMinutes != nil {
		clone.HistoryMinutes = append([]int(nil), entry.HistoryMinutes...)
	}
	if entry.CriticalOverlapTiers != nil {
		clone.CriticalOverlapTiers = append([]string(nil), entry.CriticalOverlapTiers...)
	}
	return clone
}

// Snapshot returns a detached copy of the entry. It is safe for concurrent
// readers while lifecycle methods update the entry.
func (entry *Entry) Snapshot() Entry {
	return cloneEntry(entry)
}

// Remove deletes the entry for name.
func (st *State) Remove(name string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Entries == nil {
		return
	}
	delete(st.Entries, name)
}

// EntrySnapshot returns a copy of the entry for name, or nil
// if not found. Safe for concurrent read access.
func (st *State) EntrySnapshot(name string) *Entry {
	st.mu.RLock()
	defer st.mu.RUnlock()
	entry := st.Entries[name]
	if entry == nil {
		return nil
	}
	clone := cloneEntry(entry)
	return &clone
}

// Names returns a sorted list of all entry names. Safe for concurrent access.
func (st *State) Names() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	names := make([]string, 0, len(st.Entries))
	for name := range st.Entries {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// SnapshotEntries returns copies of all entries. Safe for concurrent access.
func (st *State) SnapshotEntries() map[string]Entry {
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make(map[string]Entry, len(st.Entries))
	for name, entry := range st.Entries {
		if entry != nil {
			out[name] = cloneEntry(entry)
		}
	}
	return out
}

// RenameEntry atomically moves an entry from oldName to newName.
func (st *State) RenameEntry(oldName, newName string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if entry := st.Entries[oldName]; entry != nil {
		entry.Name = newName
		st.Entries[newName] = entry
		delete(st.Entries, oldName)
	}
}
