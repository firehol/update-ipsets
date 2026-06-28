package engine

import (
	"sort"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/enrichment"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"go.opentelemetry.io/otel/attribute"
)

type PublicFeedSummary struct {
	Name                 string              `json:"name"`
	Category             string              `json:"category"`
	Provenance           string              `json:"provenance,omitempty"`
	Maintainer           string              `json:"maintainer"`
	MaintainerURL        string              `json:"maintainer_url,omitempty"`
	License              string              `json:"license,omitempty"`
	Redistributable      bool                `json:"redistributable"`
	OfficialName         string              `json:"official_name,omitempty"`
	ShortDescription     string              `json:"short_description,omitempty"`
	CurrentStatusState   string              `json:"current_status_state,omitempty"`
	Info                 string              `json:"info,omitempty"`
	IPV                  string              `json:"ipv,omitempty"`
	Hash                 string              `json:"hash,omitempty"`
	URL                  string              `json:"url,omitempty"`
	PublicURL            string              `json:"public_url,omitempty"`
	File                 string              `json:"file,omitempty"`
	Source               string              `json:"source,omitempty"`
	StartedDate          int64               `json:"started_date"`
	SourceDate           int64               `json:"source_date"`
	ProcessedDate        int64               `json:"processed_date"`
	CheckedDate          int64               `json:"checked_date"`
	UniqueIPs            uint64              `json:"unique_ips"`
	Entries              int                 `json:"entries"`
	EntriesMin           int                 `json:"entries_min,omitempty"`
	EntriesMax           int                 `json:"entries_max,omitempty"`
	IPsMin               uint64              `json:"ips_min,omitempty"`
	IPsMax               uint64              `json:"ips_max,omitempty"`
	FrequencyMinutes     int                 `json:"frequency_minutes"`
	AverageUpdateMins    int                 `json:"average_update_mins,omitempty"`
	MinUpdateMins        int                 `json:"min_update_mins,omitempty"`
	MaxUpdateMins        int                 `json:"max_update_mins,omitempty"`
	RotationMedianPct    float64             `json:"rotation_median_pct,omitempty"`
	RotationP75Pct       float64             `json:"rotation_p75_pct,omitempty"`
	RotationSamples      int                 `json:"rotation_samples,omitempty"`
	ChangeRatioMedianPct float64             `json:"change_ratio_median_pct,omitempty"`
	ChangeRatioP75Pct    float64             `json:"change_ratio_p75_pct,omitempty"`
	ChangeRatioSamples   int                 `json:"change_ratio_samples,omitempty"`
	Version              int                 `json:"version,omitempty"`
	LastStatus           string              `json:"last_status,omitempty"`
	LastError            string              `json:"last_error,omitempty"`
	DownloadFailures     int                 `json:"download_failures,omitempty"`
	UniqueSharePct       float64             `json:"unique_share_pct,omitempty"`
	UniqueShareSamples   int                 `json:"unique_share_samples,omitempty"`
	Critical             *PublicCriticalFeed `json:"critical,omitempty"`
	CriticalOverlapTiers []string            `json:"critical_overlap_tiers,omitempty"`
	Health               feedhealth.Snapshot `json:"health"`
}

type PublicCriticalFeed struct {
	Tier string `json:"tier,omitempty"`
	Role string `json:"role,omitempty"`
}

type PublicServingCatalogSnapshot struct {
	Categories                      []PublicCategory
	Feeds                           []PublicFeedSummary
	PublicFeedNames                 map[string]struct{}
	RawFeedFiles                    map[string]string
	GeoProviders                    []GeoProvider
	ASNProviders                    []ASNProvider
	BogonProviders                  []BogonProvider
	CriticalInfrastructureProviders []CriticalInfrastructureProvider
	CriticalInfrastructureTargets   map[string]struct{}
}

func (e *Engine) TryPublicServingCatalogSnapshot() (PublicServingCatalogSnapshot, bool) {
	var out PublicServingCatalogSnapshot
	snap, ok := e.tryOperationSnapshot()
	if !ok {
		return out, false
	}
	if snap.cfg == nil {
		return out, true
	}
	entries, ok := e.tryEntryMapSnapshot()
	if !ok {
		return out, false
	}

	out.Categories = publicCategoriesForConfig(snap.cfg)
	out.GeoProviders = geoProvidersForConfig(snap.cfg)
	out.ASNProviders = asnProvidersForConfig(snap.cfg)
	out.BogonProviders = bogonProvidersForConfig(snap.cfg)
	out.CriticalInfrastructureProviders = criticalInfrastructureProvidersForConfig(snap.cfg)
	out.PublicFeedNames = map[string]struct{}{}
	out.RawFeedFiles = map[string]string{}
	out.CriticalInfrastructureTargets = map[string]struct{}{}

	now := e.now().UTC()
	configured := configuredNamesForConfig(snap.cfg)
	configuredNames := make([]string, 0, len(configured))
	for name := range configured {
		configuredNames = append(configuredNames, name)
	}
	sort.Strings(configuredNames)
	for _, name := range configuredNames {
		if isPublicFeedNameForConfig(snap.cfg, name) {
			out.PublicFeedNames[name] = struct{}{}
		}
		if !isCriticalInfrastructureOutputName(snap.cfg, name) && isCriticalInfrastructureComparableName(snap.cfg, name) {
			out.CriticalInfrastructureTargets[name] = struct{}{}
		}
	}

	resolver := newEffectiveEntryResolver(snap.cfg, entries)
	names := make([]string, 0, len(entries))
	for name := range entries {
		if configured[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		raw := entries[name]
		entry := resolver.entry(name, &raw)
		if entry == nil {
			continue
		}
		if isPublicFeedNameForConfig(snap.cfg, name) {
			src := lookupSourceForConfig(snap.cfg, name)
			out.Feeds = append(out.Feeds, buildPublicFeedSummary(entry, src, snap.feedHealthPolicy, now, isRedistributableForConfig(snap.cfg, name)))
			if isPublicRawFeedAvailableForSnapshot(snap, name, entry, now) {
				out.RawFeedFiles[name] = entry.File
			}
		}
	}
	observePublicFeedSummaries(out.Feeds, now)
	return out, true
}

func (e *Engine) tryEntryMapSnapshot() (map[string]cache.Entry, bool) {
	if e == nil || e.state == nil {
		return map[string]cache.Entry{}, true
	}
	return e.state.TrySnapshotEntries()
}

func isPublicRawFeedAvailableForSnapshot(snap operationSnapshot, name string, entry *cache.Entry, now time.Time) bool {
	if entry == nil || entry.File == "" {
		return false
	}
	if !isRedistributableForConfig(snap.cfg, name) {
		return false
	}
	src := lookupSourceForConfig(snap.cfg, name)
	if feedhealth.Classify(entry, src, snap.feedHealthPolicy, now).Class == feedhealth.ClassArchived {
		return false
	}
	if !rawFeedFileMatches(name, entry.File) {
		return false
	}
	_, ok := safeRuntimeFilePath(snap.runtime.BaseDir, entry.File)
	return ok
}

func (e *Engine) PublicFeedSummaries() []PublicFeedSummary {
	if e == nil {
		return nil
	}
	cfg, _, policy := e.configRuntimePolicySnapshot()
	if cfg == nil {
		return nil
	}
	entries := e.entriesSnapshot(cfg, configuredNamesForConfig(cfg))
	now := e.now().UTC()
	out := make([]PublicFeedSummary, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		if !isPublicFeedNameForConfig(cfg, entry.Name) {
			continue
		}
		src := lookupSourceForConfig(cfg, entry.Name)
		out = append(out, buildPublicFeedSummary(entry, src, policy, now, isRedistributableForConfig(cfg, entry.Name)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	observePublicFeedSummaries(out, now)
	return out
}

func buildPublicFeedSummary(entry *cache.Entry, src *config.Source, policy feedhealth.Policy, now time.Time, redistributable bool) PublicFeedSummary {
	category := entry.Category
	if src != nil && src.Category != "" {
		category = src.Category
	}
	summary := PublicFeedSummary{
		Name:                 entry.Name,
		Category:             category,
		Provenance:           string(publicProvenance(src)),
		Maintainer:           entry.Maintainer,
		MaintainerURL:        entry.MaintainerURL,
		License:              entry.License,
		Redistributable:      redistributable,
		Info:                 entry.Info,
		IPV:                  entry.IPV,
		Hash:                 entry.Hash,
		URL:                  entry.URL,
		PublicURL:            entry.PublicURL,
		File:                 entry.File,
		Source:               entry.Source,
		StartedDate:          entry.StartedDate,
		SourceDate:           entry.SourceDate,
		ProcessedDate:        entry.ProcessedDate,
		CheckedDate:          entry.CheckedDate,
		UniqueIPs:            entry.UniqueIPs,
		Entries:              entry.Entries,
		EntriesMin:           entry.EntriesMin,
		EntriesMax:           entry.EntriesMax,
		IPsMin:               entry.IPsMin,
		IPsMax:               entry.IPsMax,
		FrequencyMinutes:     entry.FrequencyMinutes,
		AverageUpdateMins:    entry.AverageUpdateMins,
		MinUpdateMins:        entry.MinUpdateMins,
		MaxUpdateMins:        entry.MaxUpdateMins,
		RotationMedianPct:    entry.RotationMedianPct,
		RotationP75Pct:       entry.RotationP75Pct,
		RotationSamples:      entry.RotationSamples,
		ChangeRatioMedianPct: entry.ChangeRatioMedianPct,
		ChangeRatioP75Pct:    entry.ChangeRatioP75Pct,
		ChangeRatioSamples:   entry.ChangeRatioSamples,
		Version:              entry.Version,
		LastStatus:           entry.LastStatus,
		LastError:            entry.LastError,
		DownloadFailures:     entry.DownloadFailures,
		UniqueSharePct:       entry.UniqueSharePct,
		UniqueShareSamples:   entry.UniqueShareSamples,
	}
	if src != nil && src.Critical != nil && src.HasUse(config.UseCriticalInfrastructure) {
		summary.Critical = &PublicCriticalFeed{
			Tier: src.Critical.Tier,
			Role: src.Critical.Role,
		}
	}
	if src != nil && src.Enrichment != nil {
		summary.OfficialName = enrichment.StringValue(src.Enrichment.OfficialName)
		summary.ShortDescription = enrichment.StringValue(src.Enrichment.ShortDescription)
		summary.CurrentStatusState = src.Enrichment.CurrentStatus.State
	}
	if src == nil || (!src.HasUse(config.UseCriticalInfrastructure) && !src.HasUse(config.UseProviderContext)) {
		summary.CriticalOverlapTiers = append([]string(nil), entry.CriticalOverlapTiers...)
	}
	if src != nil && summary.License == "" {
		summary.License = src.License
	}
	summary.Health = feedhealth.Classify(entry, src, policy, now)
	if !redistributable || summary.Health.Class == feedhealth.ClassArchived {
		summary.URL = ""
		summary.PublicURL = ""
		summary.File = ""
		summary.Source = ""
	}
	return summary
}

func observePublicFeedSummaries(summaries []PublicFeedSummary, now time.Time) {
	for i := range summaries {
		summary := summaries[i]
		attrs := []attribute.KeyValue{attribute.String("feed.name", summary.Name)}
		observability.TryGauge("feed.state", feedStateCode(summary), attrs...)
		observability.TryGauge("feed.health.state", feedHealthCode(summary.Health.Class), attrs...)
		observability.TryGauge("feed.entries", int64(summary.Entries), attrs...)
		observability.TryGauge("feed.unique_ips", uint64ToInt64(summary.UniqueIPs), attrs...)
		observability.TryGauge("feed.errors", int64(summary.DownloadFailures), attrs...)
		observability.TryGauge("feed.freshness.seconds", feedFreshnessSeconds(summary, now), attrs...)
		observability.TryGauge("feed.last_success.timestamp", summary.ProcessedDate, attrs...)
	}
}

func feedStateCode(summary PublicFeedSummary) int64 {
	status := strings.ToLower(strings.TrimSpace(summary.LastStatus))
	switch {
	case status == "disabled":
		return 1
	case summary.ProcessedDate == 0 && summary.CheckedDate == 0:
		return 2
	case status == "running" || status == "downloading" || status == "processing" || status == "materializing":
		return 3
	case summary.LastError != "" || summary.Health.Class == feedhealth.ClassUnavailable:
		return 6
	case summary.Health.Class == feedhealth.ClassDelayed ||
		summary.Health.Class == feedhealth.ClassRisky ||
		summary.Health.Class == feedhealth.ClassUnmaintained ||
		summary.Health.Class == feedhealth.ClassArchived ||
		summary.Health.Class == feedhealth.ClassEmpty:
		return 5
	case status == "":
		return 0
	default:
		return 4
	}
}

func feedHealthCode(class feedhealth.Class) int64 {
	switch class {
	case feedhealth.ClassHealthy:
		return 1
	case feedhealth.ClassDelayed:
		return 2
	case feedhealth.ClassRisky:
		return 3
	case feedhealth.ClassUnavailable:
		return 4
	case feedhealth.ClassArchived:
		return 5
	case feedhealth.ClassEmpty:
		return 6
	case feedhealth.ClassUnmaintained:
		return 7
	default:
		return 0
	}
}

func feedFreshnessSeconds(summary PublicFeedSummary, now time.Time) int64 {
	if summary.ProcessedDate <= 0 {
		return 0
	}
	freshness := now.UTC().Unix() - summary.ProcessedDate
	if freshness < 0 {
		return 0
	}
	return freshness
}

func uint64ToInt64(value uint64) int64 {
	const maxInt64 = uint64(^uint64(0) >> 1)
	if value > maxInt64 {
		return int64(maxInt64)
	}
	return int64(value)
}
