package mcp

import (
	"slices"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

type FeedFilterOptions struct {
	Categories      []string
	Maintainers     []string
	Provenance      []string
	Health          []string
	Freshness       []string
	Cadence         []string
	Uniqueness      []string
	Licenses        []string
	Redistributable []string
	Critical        []string
}

type FeedFilterOptionsProvider interface {
	FeedFilterOptions() FeedFilterOptions
}

func feedFilterOptionsFromCatalog(catalog FeedCatalog) FeedFilterOptions {
	options := defaultFeedFilterOptions()
	provider, ok := catalog.(FeedFilterOptionsProvider)
	if !ok || provider == nil {
		return options
	}
	return options.Merge(provider.FeedFilterOptions())
}

func defaultFeedFilterOptions() FeedFilterOptions {
	return FeedFilterOptions{
		Provenance: []string{
			string(config.ProvenancePrimary),
			string(config.ProvenanceSecondaryUpstream),
			string(config.ProvenanceSecondaryMerge),
			string(config.ProvenanceSecondaryRetention),
		},
		Health: []string{
			string(feedhealth.ClassHealthy),
			string(feedhealth.ClassDelayed),
			string(feedhealth.ClassRisky),
			string(feedhealth.ClassArchived),
			string(feedhealth.ClassUnmaintained),
			string(feedhealth.ClassEmpty),
			string(feedhealth.ClassUnavailable),
		},
		Freshness:       []string{"hour", "day", "week", "month", "older"},
		Cadence:         []string{"hourly", "daily", "weekly", "monthly", "slower", "unknown"},
		Uniqueness:      []string{"very_high", "high", "medium", "low", "unknown"},
		Redistributable: []string{"true", "false"},
		Critical:        config.CriticalTiers(),
	}
}

func (o FeedFilterOptions) Merge(other FeedFilterOptions) FeedFilterOptions {
	return FeedFilterOptions{
		Categories:      mergeStringOptions(o.Categories, other.Categories),
		Maintainers:     mergeStringOptions(o.Maintainers, other.Maintainers),
		Provenance:      mergeStringOptions(o.Provenance, other.Provenance),
		Health:          mergeStringOptions(o.Health, other.Health),
		Freshness:       mergeStringOptions(o.Freshness, other.Freshness),
		Cadence:         mergeStringOptions(o.Cadence, other.Cadence),
		Uniqueness:      mergeStringOptions(o.Uniqueness, other.Uniqueness),
		Licenses:        mergeStringOptions(o.Licenses, other.Licenses),
		Redistributable: mergeStringOptions(o.Redistributable, other.Redistributable),
		Critical:        mergeStringOptions(o.Critical, other.Critical),
	}
}

func mergeStringOptions(first, second []string) []string {
	seen := make(map[string]struct{}, len(first)+len(second))
	out := make([]string, 0, len(first)+len(second))
	for _, values := range [][]string{first, second} {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			key := strings.ToLower(value)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func sortedStringOptions(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}
