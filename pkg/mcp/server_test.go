package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type stubCatalog struct {
	feeds []FeedHit
	err   error
}

type stubOptionsCatalog struct {
	stubCatalog
	options FeedFilterOptions
}

func (c *stubCatalog) FindFeeds(filters FeedFilters) ([]FeedHit, error) {
	if c.err != nil {
		return nil, c.err
	}
	var out []FeedHit
	for _, h := range c.feeds {
		if matchFeed(h, filters) {
			out = append(out, h)
		}
	}
	return out, nil
}

func (c *stubOptionsCatalog) FeedFilterOptions() FeedFilterOptions {
	return c.options
}

func TestHandleFindFeedsEmpty(t *testing.T) {
	s := NewServer(&stubCatalog{}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
		},
	}
	result, err := s.handleFindFeeds(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	if text != "No feeds matched the given filters." {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestHandleFindFeedsReturnsMarkdown(t *testing.T) {
	processed := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC).Unix()
	checked := time.Date(2026, 5, 16, 10, 30, 0, 0, time.UTC).Unix()
	feeds := []FeedHit{
		{
			Name:             "firehol_level1",
			Category:         "intrusion",
			Maintainer:       "FireHOL",
			MaintainerURL:    "https://firehol.org/",
			Provenance:       "primary",
			OfficialName:     "FireHOL Level 1",
			ShortDescription: "Primary blocking set.",
			Info:             "Curated intrusion feed.",
			UniqueIPs:        5000,
			Entries:          5100,
			IPV:              "ipv4",
			License:          "public feed",
			Redistributable:  true,
			Health:           "healthy",
			Freshness:        "day",
			Cadence:          "daily",
			UniqueSharePct:   12.345,
			CriticalTier:     "soft",
			ProcessedDate:    processed,
			CheckedDate:      checked,
		},
		{Name: "tor_exits", Category: "anonymizers", Maintainer: "TorProject", UniqueIPs: 2000},
	}
	s := NewServer(&stubCatalog{feeds: feeds}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
			Arguments: map[string]any{
				"category": "intrusion",
			},
		},
	}
	result, err := s.handleFindFeeds(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	assertContains(t, text, "feed|description|category|provenance|unique_ips|entries|ipv|license|redistributable|health|freshness|cadence|unique_share_pct|critical|processed|checked")
	assertContains(t, text, "firehol_level1 - FireHOL Level 1|Primary blocking set.|intrusion|primary|5000|5100|ipv4|public feed|true|healthy|day|daily|12.3%|soft|2026-05-04T12:00:00Z")
	assertContains(t, text, "2026-05-16T10:30:00Z")
	assertContains(t, text, "# firehol_level1")
	assertContains(t, text, "FireHOL Level 1")
	assertContains(t, text, "Primary blocking set.")
	assertContains(t, text, "by [FireHOL](https://firehol.org/)")
	assertContains(t, text, "Curated intrusion feed.")
	if strings.Contains(text, "tor_exits") {
		t.Fatalf("filtered markdown includes non-matching feed:\n%s", text)
	}
}

func TestHandleFindFeedsFilterByMaintainer(t *testing.T) {
	feeds := []FeedHit{
		{Name: "firehol_level1", Maintainer: "FireHOL"},
		{Name: "tor_exits", Maintainer: "TorProject"},
	}
	s := NewServer(&stubCatalog{feeds: feeds}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
			Arguments: map[string]any{
				"maintainer": "fire",
			},
		},
	}
	result, _ := s.handleFindFeeds(context.Background(), req)
	text := result.Content[0].(mcpgo.TextContent).Text
	assertContains(t, text, "firehol_level1")
	assertNotContains(t, text, "tor_exits")
}

func TestHandleFindFeedsFullTextSearch(t *testing.T) {
	feeds := []FeedHit{
		{Name: "apnic_ssh_bruteforce", Maintainer: "APNIC", ShortDescription: "SSH brute force attackers"},
		{Name: "tor_exits", Maintainer: "TorProject", Info: "Tor exit nodes"},
	}
	s := NewServer(&stubCatalog{feeds: feeds}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
			Arguments: map[string]any{
				"search": "ssh brute",
			},
		},
	}
	result, _ := s.handleFindFeeds(context.Background(), req)
	text := result.Content[0].(mcpgo.TextContent).Text
	assertContains(t, text, "apnic_ssh_bruteforce")
	assertContains(t, text, "SSH brute force attackers")
	assertNotContains(t, text, "tor_exits")
}

func TestHandleFindFeedsFilterByHealth(t *testing.T) {
	feeds := []FeedHit{
		{Name: "good_feed", Health: "healthy"},
		{Name: "bad_feed", Health: "unavailable"},
	}
	s := NewServer(&stubCatalog{feeds: feeds}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
			Arguments: map[string]any{
				"health": "healthy",
			},
		},
	}
	result, _ := s.handleFindFeeds(context.Background(), req)
	text := result.Content[0].(mcpgo.TextContent).Text
	assertContains(t, text, "good_feed")
	assertNotContains(t, text, "bad_feed")
}

func TestHandleFindFeedsFilterBySize(t *testing.T) {
	feeds := []FeedHit{
		{Name: "small", UniqueIPs: 100},
		{Name: "large", UniqueIPs: 100000},
	}
	s := NewServer(&stubCatalog{feeds: feeds}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
			Arguments: map[string]any{
				"size_min": float64(1000),
			},
		},
	}
	result, _ := s.handleFindFeeds(context.Background(), req)
	text := result.Content[0].(mcpgo.TextContent).Text
	assertContains(t, text, "large")
	assertNotContains(t, text, "small")
}

func TestHandleFindFeedsFilterByProvenance(t *testing.T) {
	feeds := []FeedHit{
		{Name: "primary_feed", Provenance: "primary"},
		{Name: "derived_feed", Provenance: "secondary_merge"},
	}
	s := NewServer(&stubCatalog{feeds: feeds}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
			Arguments: map[string]any{
				"provenance": "primary",
			},
		},
	}
	result, _ := s.handleFindFeeds(context.Background(), req)
	text := result.Content[0].(mcpgo.TextContent).Text
	assertContains(t, text, "primary_feed")
	assertNotContains(t, text, "derived_feed")
}

func TestHandleFindFeedsFilterByRedistributable(t *testing.T) {
	feeds := []FeedHit{
		{Name: "open_feed", Redistributable: true},
		{Name: "closed_feed", Redistributable: false},
	}
	s := NewServer(&stubCatalog{feeds: feeds}, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "find_feeds",
			Arguments: map[string]any{
				"redistributable": "true",
			},
		},
	}
	result, _ := s.handleFindFeeds(context.Background(), req)
	text := result.Content[0].(mcpgo.TextContent).Text
	assertContains(t, text, "open_feed")
	assertNotContains(t, text, "closed_feed")
}

func TestFindFeedsToolSchemaDeclaresFilterEnums(t *testing.T) {
	catalog := &stubOptionsCatalog{
		options: FeedFilterOptions{
			Categories:  []string{"anonymizers", "intrusion"},
			Maintainers: []string{"FireHOL", "TorProject"},
			Licenses:    []string{"CC-BY-SA-4.0", "public feed"},
		},
	}
	tool := NewServer(catalog, nil).findFeedsTool()

	assertToolEnumContains(t, tool, "category", "anonymizers", "intrusion")
	assertToolEnumContains(t, tool, "maintainer", "FireHOL", "TorProject")
	assertToolEnumContains(t, tool, "license", "CC-BY-SA-4.0", "public feed")
	assertToolEnumContains(t, tool, "provenance", "primary", "secondary_upstream", "secondary_merge", "secondary_retention")
	assertToolEnumContains(t, tool, "health", "healthy", "delayed", "risky", "archived", "unmaintained", "empty", "unavailable")
	assertToolEnumContains(t, tool, "freshness", "hour", "day", "week", "month", "older")
	assertToolEnumContains(t, tool, "cadence", "hourly", "daily", "weekly", "monthly", "slower", "unknown")
	assertToolEnumContains(t, tool, "uniqueness", "very_high", "high", "medium", "low", "unknown")
	assertToolEnumContains(t, tool, "redistributable", "true", "false")
	assertToolEnumContains(t, tool, "critical", "hard", "soft", "contextual")
}

func TestRenderFindFeedsMarkdownFormatsDatesAndPercent(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	text := renderFindFeedsMarkdown([]FeedHit{
		{
			Name:           "sample",
			UniqueSharePct: 12.345,
			ProcessedDate:  time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC).Unix(),
		},
	}, now)

	assertContains(t, text, "12.3%")
	assertContains(t, text, "2026-05-04T12:00:00Z (12d ago)")
}

func TestHandleFetchAnalysis(t *testing.T) {
	dir := t.TempDir()
	// Feed markdown lives at web/{feed}.md (sibling of web/{feed}.json).
	if err := os.WriteFile(filepath.Join(dir, "test_feed.md"), []byte("# Test Feed\n\nSome content."), 0o600); err != nil {
		t.Fatalf("write analysis markdown: %v", err)
	}

	store := NewFileMarkdownStore(dir)
	s := NewServer(nil, store)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "fetch_analysis",
			Arguments: map[string]any{
				"type": "feed",
				"name": "test_feed",
			},
		},
	}
	result, err := s.handleFetchAnalysis(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	if text != "# Test Feed\n\nSome content." {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestHandleFetchAnalysisEntityLayout(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		entityType string
		rel        string
		name       string
		body       string
	}{
		{"feed", "test_feed.md", "test_feed", "# feed"},
		{"country", filepath.Join("countries", "US.md"), "US", "# country"},
		{"asn", filepath.Join("asns", "13335.md"), "13335", "# asn"},
		{"maintainer", filepath.Join("maintainers", "firehol.md"), "firehol", "# maintainer"},
	} {
		path := filepath.Join(dir, c.rel)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(c.body), 0600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	store := NewFileMarkdownStore(dir)
	s := NewServer(nil, store)

	for _, c := range []struct {
		entityType string
		name       string
		want       string
	}{
		{"feed", "test_feed", "# feed"},
		{"country", "US", "# country"},
		{"asn", "13335", "# asn"},
		{"maintainer", "firehol", "# maintainer"},
	} {
		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name: "fetch_analysis",
				Arguments: map[string]any{
					"type": c.entityType,
					"name": c.name,
				},
			},
		}
		result, err := s.handleFetchAnalysis(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", c.entityType, err)
		}
		if result.IsError {
			t.Fatalf("%s: unexpected tool error: %v", c.entityType, result.Content)
		}
		got := result.Content[0].(mcpgo.TextContent).Text
		if got != c.want {
			t.Fatalf("%s: got %q want %q", c.entityType, got, c.want)
		}
	}
}

func TestHandleFetchAnalysisInvalidType(t *testing.T) {
	s := NewServer(nil, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "fetch_analysis",
			Arguments: map[string]any{
				"type": "invalid",
				"name": "test",
			},
		},
	}
	result, _ := s.handleFetchAnalysis(context.Background(), req)
	if !result.IsError {
		t.Fatal("expected error result")
	}
}

func TestHandleFetchAnalysisMissingName(t *testing.T) {
	s := NewServer(nil, nil)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "fetch_analysis",
			Arguments: map[string]any{
				"type": "feed",
			},
		},
	}
	result, _ := s.handleFetchAnalysis(context.Background(), req)
	if !result.IsError {
		t.Fatal("expected error result for missing name")
	}
}

func TestHandleFetchAnalysisNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMarkdownStore(dir)
	s := NewServer(nil, store)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "fetch_analysis",
			Arguments: map[string]any{
				"type": "feed",
				"name": "nonexistent",
			},
		},
	}
	result, _ := s.handleFetchAnalysis(context.Background(), req)
	if !result.IsError {
		t.Fatal("expected error result for missing entity")
	}
}

func TestFileMarkdownStorePathTraversal(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMarkdownStore(dir)

	_, err := store.ReadMarkdown("feed", "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestMatchFeedAllFilters(t *testing.T) {
	h := FeedHit{
		Name:            "firehol_level1",
		Category:        "intrusion",
		Maintainer:      "FireHOL",
		Provenance:      "primary",
		Health:          "healthy",
		Freshness:       "day",
		Cadence:         "daily",
		License:         "CC-BY-SA-4.0",
		Redistributable: true,
		CriticalTier:    "hard",
		UniqueSharePct:  60,
		UniqueIPs:       5000,
	}

	tests := []struct {
		name    string
		filters FeedFilters
		match   bool
	}{
		{"no filters", FeedFilters{}, true},
		{"category match", FeedFilters{Category: "intrusion"}, true},
		{"category mismatch", FeedFilters{Category: "geolocation"}, false},
		{"maintainer match", FeedFilters{Maintainer: "fire"}, true},
		{"maintainer mismatch", FeedFilters{Maintainer: "tor"}, false},
		{"provenance match", FeedFilters{Provenance: "primary"}, true},
		{"provenance mismatch", FeedFilters{Provenance: "secondary_merge"}, false},
		{"health match", FeedFilters{Health: "healthy"}, true},
		{"health mismatch", FeedFilters{Health: "unavailable"}, false},
		{"freshness match", FeedFilters{Freshness: "day"}, true},
		{"freshness mismatch", FeedFilters{Freshness: "month"}, false},
		{"cadence match", FeedFilters{Cadence: "daily"}, true},
		{"cadence mismatch", FeedFilters{Cadence: "hourly"}, false},
		{"license match", FeedFilters{License: "CC-BY-SA-4.0"}, true},
		{"license mismatch", FeedFilters{License: "GPL"}, false},
		{"redistributable true", FeedFilters{Redistributable: "true"}, true},
		{"redistributable false", FeedFilters{Redistributable: "false"}, false},
		{"critical match", FeedFilters{Critical: "hard"}, true},
		{"critical mismatch", FeedFilters{Critical: "soft"}, false},
		{"uniqueness very_high", FeedFilters{Uniqueness: "very_high"}, true},
		{"uniqueness low", FeedFilters{Uniqueness: "low"}, false},
		{"size_min ok", FeedFilters{SizeMin: 1000}, true},
		{"size_min too high", FeedFilters{SizeMin: 9999}, false},
		{"size_max ok", FeedFilters{SizeMax: 6000}, true},
		{"size_max too low", FeedFilters{SizeMax: 1000}, false},
		{"combined match", FeedFilters{Category: "intrusion", Health: "healthy"}, true},
		{"combined mismatch", FeedFilters{Category: "intrusion", Health: "unavailable"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchFeed(h, tt.filters)
			if got != tt.match {
				t.Errorf("matchFeed() = %v, want %v", got, tt.match)
			}
		})
	}
}

func TestUniquenessBucket(t *testing.T) {
	tests := []struct {
		pct    float64
		expect string
	}{
		{60, "very_high"},
		{50, "very_high"},
		{30, "high"},
		{20, "high"},
		{10, "medium"},
		{5, "medium"},
		{2, "low"},
		{0, "unknown"},
	}
	for _, tt := range tests {
		got := uniquenessBucket(tt.pct)
		if got != tt.expect {
			t.Errorf("uniquenessBucket(%v) = %q, want %q", tt.pct, got, tt.expect)
		}
	}
}

func TestCadenceBucket(t *testing.T) {
	tests := []struct {
		freq   int
		avg    int
		expect string
	}{
		{0, 0, "unknown"},
		{60, 0, "hourly"},
		{0, 60, "hourly"},
		{1440, 0, "daily"},
		{0, 1440, "daily"},
		{10080, 0, "weekly"},
		{43200, 0, "monthly"},
		{100000, 0, "slower"},
	}
	for _, tt := range tests {
		got := cadenceBucket(tt.freq, tt.avg)
		if got != tt.expect {
			t.Errorf("cadenceBucket(%d, %d) = %q, want %q", tt.freq, tt.avg, got, tt.expect)
		}
	}
}

func assertToolEnumContains(t *testing.T, tool mcpgo.Tool, field string, wants ...string) {
	t.Helper()
	prop, ok := tool.InputSchema.Properties[field].(map[string]any)
	if !ok {
		t.Fatalf("%s property missing or wrong type: %#v", field, tool.InputSchema.Properties[field])
	}
	raw, ok := prop["enum"].([]string)
	if !ok {
		t.Fatalf("%s enum missing or wrong type: %#v", field, prop["enum"])
	}
	got := make(map[string]struct{}, len(raw))
	for _, value := range raw {
		got[value] = struct{}{}
	}
	for _, want := range wants {
		if _, ok := got[want]; !ok {
			t.Fatalf("%s enum = %#v, want it to contain %q", field, raw, want)
		}
	}
}

func assertContains(t *testing.T, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected output to contain %q:\n%s", want, text)
	}
}

func assertNotContains(t *testing.T, text, want string) {
	t.Helper()
	if strings.Contains(text, want) {
		t.Fatalf("expected output not to contain %q:\n%s", want, text)
	}
}
