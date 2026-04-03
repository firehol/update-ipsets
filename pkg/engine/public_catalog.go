package engine

import (
	"sort"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/enrichment"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
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

func (e *Engine) PublicFeedSummaries() []PublicFeedSummary {
	if e == nil || e.cfg == nil {
		return nil
	}
	entries := e.EntriesSnapshot()
	now := e.now().UTC()
	out := make([]PublicFeedSummary, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		if !e.isPublicFeedName(entry.Name) {
			continue
		}
		src := e.lookupSource(entry.Name)
		out = append(out, buildPublicFeedSummary(entry, src, feedhealth.PolicyFromRuntime(e.cfg.Runtime), now, e.isRedistributable(entry.Name)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
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
