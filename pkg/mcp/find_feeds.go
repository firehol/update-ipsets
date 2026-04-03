package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type FeedFilters struct {
	Search          string `json:"search,omitempty"`
	Category        string `json:"category,omitempty"`
	Maintainer      string `json:"maintainer,omitempty"`
	Provenance      string `json:"provenance,omitempty"`
	Health          string `json:"health,omitempty"`
	Freshness       string `json:"freshness,omitempty"`
	Cadence         string `json:"cadence,omitempty"`
	Uniqueness      string `json:"uniqueness,omitempty"`
	License         string `json:"license,omitempty"`
	Redistributable string `json:"redistributable,omitempty"`
	Critical        string `json:"critical,omitempty"`
	SizeMin         int64  `json:"size_min,omitempty"`
	SizeMax         int64  `json:"size_max,omitempty"`
}

type FeedHit struct {
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	Maintainer       string  `json:"maintainer"`
	MaintainerURL    string  `json:"maintainer_url,omitempty"`
	Provenance       string  `json:"provenance,omitempty"`
	OfficialName     string  `json:"official_name,omitempty"`
	ShortDescription string  `json:"short_description,omitempty"`
	Info             string  `json:"info,omitempty"`
	UniqueIPs        uint64  `json:"unique_ips"`
	Entries          int     `json:"entries"`
	IPV              string  `json:"ipv,omitempty"`
	License          string  `json:"license,omitempty"`
	Redistributable  bool    `json:"redistributable"`
	Health           string  `json:"health,omitempty"`
	Freshness        string  `json:"freshness,omitempty"`
	Cadence          string  `json:"cadence,omitempty"`
	UniqueSharePct   float64 `json:"unique_share_pct,omitempty"`
	CriticalTier     string  `json:"critical_tier,omitempty"`
	ProcessedDate    int64   `json:"processed_date"`
	CheckedDate      int64   `json:"checked_date"`
}

func (s *Server) handleFindFeeds(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	filters := FeedFilters{
		Search:          req.GetString("search", ""),
		Category:        req.GetString("category", ""),
		Maintainer:      req.GetString("maintainer", ""),
		Provenance:      req.GetString("provenance", ""),
		Health:          req.GetString("health", ""),
		Freshness:       req.GetString("freshness", ""),
		Cadence:         req.GetString("cadence", ""),
		Uniqueness:      req.GetString("uniqueness", ""),
		License:         req.GetString("license", ""),
		Redistributable: req.GetString("redistributable", ""),
		Critical:        req.GetString("critical", ""),
		SizeMin:         int64(req.GetInt("size_min", 0)),
		SizeMax:         int64(req.GetInt("size_max", 0)),
	}

	hits, err := s.catalog.FindFeeds(filters)
	if err != nil {
		return mcpgo.NewToolResultErrorFromErr("find_feeds failed", err), nil
	}

	if len(hits) == 0 {
		return mcpgo.NewToolResultText("No feeds matched the given filters."), nil
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Name < hits[j].Name })

	return mcpgo.NewToolResultText(renderFindFeedsMarkdown(hits, time.Now().UTC())), nil
}

func matchFeed(h FeedHit, f FeedFilters) bool {
	if f.Search != "" && !matchFeedSearch(h, f.Search) {
		return false
	}
	if f.Category != "" && !strings.EqualFold(h.Category, f.Category) {
		return false
	}
	if f.Maintainer != "" && !strings.Contains(strings.ToLower(h.Maintainer), strings.ToLower(f.Maintainer)) {
		return false
	}
	if f.Provenance != "" && !strings.EqualFold(h.Provenance, f.Provenance) {
		return false
	}
	if f.Health != "" && !strings.EqualFold(h.Health, f.Health) {
		return false
	}
	if f.Freshness != "" && !strings.EqualFold(h.Freshness, f.Freshness) {
		return false
	}
	if f.Cadence != "" && !strings.EqualFold(h.Cadence, f.Cadence) {
		return false
	}
	if f.License != "" && !strings.EqualFold(h.License, f.License) {
		return false
	}
	if f.Critical != "" && !strings.EqualFold(h.CriticalTier, f.Critical) {
		return false
	}
	if f.Redistributable != "" {
		rb := strings.EqualFold(f.Redistributable, "true") || strings.EqualFold(f.Redistributable, "yes") || f.Redistributable == "1"
		if h.Redistributable != rb {
			return false
		}
	}
	if f.Uniqueness != "" {
		bucket := uniquenessBucket(h.UniqueSharePct)
		if !strings.EqualFold(bucket, f.Uniqueness) {
			return false
		}
	}
	if f.SizeMin > 0 && int64(h.UniqueIPs) < f.SizeMin {
		return false
	}
	if f.SizeMax > 0 && int64(h.UniqueIPs) > f.SizeMax {
		return false
	}
	return true
}

func matchFeedSearch(h FeedHit, query string) bool {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		h.Name,
		h.OfficialName,
		h.ShortDescription,
		h.Maintainer,
		h.Info,
	}, " "))
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func uniquenessBucket(pct float64) string {
	switch {
	case pct >= 50:
		return "very_high"
	case pct >= 20:
		return "high"
	case pct >= 5:
		return "medium"
	case pct > 0:
		return "low"
	default:
		return "unknown"
	}
}

func renderFindFeedsMarkdown(hits []FeedHit, now time.Time) string {
	var b strings.Builder
	b.WriteString("feed|description|category|provenance|unique_ips|entries|ipv|license|redistributable|health|freshness|cadence|unique_share_pct|critical|processed|checked\n")
	b.WriteString("----|-----------|--------|----------|----------|-------|---|-------|---------------|------|---------|-------|----------------|--------|---------|-------\n")
	for _, h := range hits {
		row := []string{
			markdownFeedLabel(h),
			feedDescription(h),
			h.Category,
			h.Provenance,
			fmt.Sprintf("%d", h.UniqueIPs),
			fmt.Sprintf("%d", h.Entries),
			h.IPV,
			h.License,
			fmt.Sprintf("%t", h.Redistributable),
			h.Health,
			h.Freshness,
			h.Cadence,
			formatPercent(h.UniqueSharePct),
			h.CriticalTier,
			formatUTCDateWithAge(h.ProcessedDate, now),
			formatUTCDateWithAge(h.CheckedDate, now),
		}
		for i, value := range row {
			if i > 0 {
				b.WriteByte('|')
			}
			b.WriteString(markdownTableCell(value))
		}
		b.WriteByte('\n')
	}

	for _, h := range hits {
		b.WriteString("\n# ")
		b.WriteString(h.Name)
		b.WriteString("\n")
		if official := strings.TrimSpace(h.OfficialName); official != "" && official != strings.TrimSpace(h.Name) {
			b.WriteString(official)
			b.WriteString("\n")
		}
		if short := strings.TrimSpace(h.ShortDescription); short != "" {
			b.WriteString("\n")
			b.WriteString(short)
			b.WriteString("\n")
		}
		if strings.TrimSpace(h.Maintainer) != "" {
			b.WriteString("by ")
			b.WriteString(markdownMaintainer(h.Maintainer, h.MaintainerURL))
			b.WriteString("\n")
		}
		if info := strings.TrimSpace(h.Info); info != "" {
			b.WriteString("\n")
			b.WriteString(info)
			if !strings.HasSuffix(info, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func markdownFeedLabel(h FeedHit) string {
	name := strings.TrimSpace(h.Name)
	official := strings.TrimSpace(h.OfficialName)
	if official == "" || official == name {
		return name
	}
	return name + " - " + official
}

func feedDescription(h FeedHit) string {
	if short := strings.TrimSpace(h.ShortDescription); short != "" {
		return short
	}
	return strings.TrimSpace(h.Info)
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", `\|`)
	return strings.TrimSpace(value)
}

func markdownMaintainer(name, url string) string {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if url == "" {
		return name
	}
	label := strings.ReplaceAll(name, "]", `\]`)
	target := strings.ReplaceAll(url, ")", "%29")
	return "[" + label + "](" + target + ")"
}

func formatPercent(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}

func formatUTCDateWithAge(unixSeconds int64, now time.Time) string {
	if unixSeconds == 0 {
		return ""
	}
	t := time.Unix(unixSeconds, 0).UTC()
	return fmt.Sprintf("%s (%s)", t.Format(time.RFC3339), relativeAge(t, now.UTC()))
}

func relativeAge(t, now time.Time) string {
	if t.After(now) {
		return "in the future"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}
