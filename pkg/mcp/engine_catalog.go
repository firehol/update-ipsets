package mcp

import (
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

type EngineFeedCatalog struct {
	eng *engine.Engine
}

func NewEngineFeedCatalog(eng *engine.Engine) *EngineFeedCatalog {
	return &EngineFeedCatalog{eng: eng}
}

func (c *EngineFeedCatalog) FeedFilterOptions() FeedFilterOptions {
	if c == nil || c.eng == nil {
		return FeedFilterOptions{}
	}
	summaries := c.eng.PublicFeedSummaries()
	categories := make(map[string]struct{}, len(summaries))
	maintainers := make(map[string]struct{}, len(summaries))
	licenses := make(map[string]struct{}, len(summaries))
	for i := range summaries {
		s := &summaries[i]
		if value := strings.TrimSpace(s.Category); value != "" {
			categories[value] = struct{}{}
		}
		if value := strings.TrimSpace(s.Maintainer); value != "" {
			maintainers[value] = struct{}{}
		}
		if value := strings.TrimSpace(s.License); value != "" {
			licenses[value] = struct{}{}
		}
	}
	return FeedFilterOptions{
		Categories:  sortedStringOptions(categories),
		Maintainers: sortedStringOptions(maintainers),
		Licenses:    sortedStringOptions(licenses),
	}
}

func (c *EngineFeedCatalog) FindFeeds(filters FeedFilters) ([]FeedHit, error) {
	summaries := c.eng.PublicFeedSummaries()
	now := time.Now().UTC()
	hits := make([]FeedHit, 0, len(summaries))
	for i := range summaries {
		s := &summaries[i]
		hit := FeedHit{
			Name:             s.Name,
			Category:         s.Category,
			Maintainer:       s.Maintainer,
			MaintainerURL:    s.MaintainerURL,
			Provenance:       s.Provenance,
			OfficialName:     s.OfficialName,
			ShortDescription: s.ShortDescription,
			Info:             s.Info,
			UniqueIPs:        s.UniqueIPs,
			Entries:          s.Entries,
			IPV:              s.IPV,
			License:          s.License,
			Redistributable:  s.Redistributable,
			Health:           strings.ToLower(string(s.Health.Class)),
			Freshness:        freshnessBucket(s.ProcessedDate, now),
			Cadence:          cadenceBucket(s.FrequencyMinutes, s.AverageUpdateMins),
			UniqueSharePct:   s.UniqueSharePct,
			ProcessedDate:    s.ProcessedDate,
			CheckedDate:      s.CheckedDate,
		}
		if s.Critical != nil {
			hit.CriticalTier = s.Critical.Tier
		}
		if !matchFeed(hit, filters) {
			continue
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func freshnessBucket(processedDate int64, now time.Time) string {
	if processedDate == 0 {
		return "older"
	}
	age := now.Unix() - processedDate
	switch {
	case age <= 3600:
		return "hour"
	case age <= 86400:
		return "day"
	case age <= 7*86400:
		return "week"
	case age <= 30*86400:
		return "month"
	default:
		return "older"
	}
}

func cadenceBucket(freqMin, avgMin int) string {
	if avgMin == 0 && freqMin == 0 {
		return "unknown"
	}
	m := avgMin
	if m == 0 {
		m = freqMin
	}
	switch {
	case m <= 90:
		return "hourly"
	case m <= 36*60:
		return "daily"
	case m <= 10*24*60:
		return "weekly"
	case m <= 45*24*60:
		return "monthly"
	default:
		return "slower"
	}
}
