package engine

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

type HomeGlobeCountry struct {
	Code          string `json:"code"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

type HomeGlobePayload struct {
	Provider          string             `json:"provider"`
	ProviderLabel     string             `json:"provider_label,omitempty"`
	Categories        []string           `json:"categories"`
	EligibleFeeds     int                `json:"eligible_feeds"`
	ContributingFeeds int                `json:"contributing_feeds"`
	Countries         []HomeGlobeCountry `json:"countries"`
}

func (e *Engine) HomeGlobe(categories []string) (*HomeGlobePayload, error) {
	outputDir := ""
	if e != nil {
		outputDir = e.outputDir()
	}
	return e.HomeGlobeInDir(categories, outputDir)
}

// HomeGlobeInDir composes homepage globe data from the precomputed homepage
// aggregate artifact in a specific published artifact directory. Public web
// handlers pass the resolved Options.WebDir so requests only read the artifact
// tree they serve.
func (e *Engine) HomeGlobeInDir(categories []string, outputDir string) (*HomeGlobePayload, error) {
	snap := e.operationSnapshot()
	if e == nil || snap.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	start := time.Now()
	e.observeRunCounter("http.home_globe.requests", 1, 0)
	defer func() {
		e.observeRunOperation("http.home_globe.request", time.Since(start))
	}()
	filter := normalizeCategoryFilter(categories)
	if len(filter) == 0 {
		return nil, fmt.Errorf("at least one category is required")
	}
	aggregates, err := e.loadHomeAggregatesInDir(outputDir)
	if err != nil {
		return nil, err
	}
	if aggregates.Providers.Geo.Name == "" {
		return nil, fmt.Errorf("no geolocation provider configured")
	}
	payload := composeHomeGlobeFromAggregates(aggregates, filter)
	e.observeRunCounter("http.home_globe.eligible_feeds", int64(payload.EligibleFeeds), 0)
	e.observeRunCounter("http.home_globe.contributing_feeds", int64(payload.ContributingFeeds), 0)
	return payload, nil
}

func normalizeCategoryFilter(categories []string) map[string]struct{} {
	out := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		normalized := strings.TrimSpace(category)
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

func sortedFilterKeys(filter map[string]struct{}) []string {
	out := make([]string, 0, len(filter))
	for category := range filter {
		out = append(out, category)
	}
	slices.Sort(out)
	return out
}

func homeGlobeHealthEligible(class feedhealth.Class) bool {
	return class == feedhealth.ClassHealthy || class == feedhealth.ClassDelayed
}

func providerDisplayLabel(src *config.Source) string {
	if src == nil {
		return ""
	}
	if src.Label != "" {
		return src.Label
	}
	return src.Name
}
