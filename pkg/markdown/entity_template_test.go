package markdown_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/markdown"
)

func TestCountryTemplateRenders(t *testing.T) {
	t.Parallel()
	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Skipf("template dir not available: %v", err)
	}
	ctx := &markdown.CountryPageContext{
		Code:     "US",
		Provider: "RIPE NCC",
		Totals: markdown.CountryTotals{
			Feeds: 42, IPs: 1_500_000, Categories: 5, Maintainers: 12, ASNs: 800,
		},
		TopCategories: []markdown.CategorySummary{
			{Category: "attack", Feeds: 20, IPs: 800_000},
			{Category: "malware", Feeds: 15, IPs: 500_000},
		},
		TopMaintainers: []markdown.MaintainerSummary{
			{Slug: "emerging-threats", Name: "Emerging Threats", URL: "https://example.com", Feeds: 10, IPs: 400_000},
			{Slug: "spamhaus", Name: "Spamhaus", Feeds: 8, IPs: 300_000},
		},
		TopASNs: []markdown.CountryASN{
			{ASN: 13335, Name: "Cloudflare", Feeds: 5, IPs: 200_000},
			{ASN: 15169, Name: "Google", Feeds: 3, IPs: 150_000},
		},
		FeedsByCategory: map[string][]markdown.FeedInEntity{
			"attack": {
				{Name: "et-compromised", Category: "attack", IPs: 50_000, Health: "healthy"},
				{Name: "dshield", Category: "attack", IPs: 100_000, Health: "stale"},
			},
		},
	}

	out, err := s.Execute("country.md.tmpl", ctx)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertContains(t, out, "United States")
	assertContains(t, out, "RIPE NCC")
	assertContains(t, out, "1,500,000")
	assertContains(t, out, "attack")
	assertContains(t, out, "Emerging Threats")
	assertContains(t, out, "Cloudflare")
	assertContains(t, out, "et-compromised")
	assertContains(t, out, "healthy")
	assertContains(t, out, "stale")
}

func TestCountryTemplateMinimal(t *testing.T) {
	t.Parallel()
	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Skipf("template dir not available: %v", err)
	}
	ctx := &markdown.CountryPageContext{
		Code:     "XX",
		Provider: "Test",
		Totals:   markdown.CountryTotals{Feeds: 0, IPs: 0},
	}

	out, err := s.Execute("country.md.tmpl", ctx)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertContains(t, out, "XX")
	assertContains(t, out, "Test")
	assertNotContains(t, out, "## Top Categories")
	assertNotContains(t, out, "## Top Maintainers")
	assertNotContains(t, out, "## Top ASNs")
	assertNotContains(t, out, "## Feeds by Category")
}

func TestASNTemplateRenders(t *testing.T) {
	t.Parallel()
	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Skipf("template dir not available: %v", err)
	}
	ctx := &markdown.ASNPageContext{
		ASN:         13335,
		Name:        "Cloudflare",
		Description: "A global cloud services provider",
		Provider:    "RIPE NCC",
		Totals: markdown.ASNTotals{
			Feeds: 30, IPs: 2_000_000, Categories: 4, Maintainers: 8, Countries: 45,
		},
		TopCountries: []markdown.ASNCountry{
			{Code: "US", Feeds: 15, IPs: 800_000},
			{Code: "DE", Feeds: 8, IPs: 400_000},
		},
		TopCategories: []markdown.CategorySummary{
			{Category: "attack", Feeds: 15, IPs: 1_000_000},
		},
		TopMaintainers: []markdown.MaintainerSummary{
			{Slug: "emerging-threats", Name: "Emerging Threats", Feeds: 10, IPs: 500_000},
		},
		FeedsByCategory: map[string][]markdown.FeedInEntity{
			"attack": {
				{Name: "et-compromised", Category: "attack", IPs: 200_000, Health: "healthy"},
			},
		},
	}

	out, err := s.Execute("asn.md.tmpl", ctx)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertContains(t, out, "AS13335 (Cloudflare)")
	assertContains(t, out, "A global cloud services provider")
	assertContains(t, out, "RIPE NCC")
	assertContains(t, out, "2,000,000")
	assertContains(t, out, "United States")
	assertContains(t, out, "Germany")
	assertContains(t, out, "attack")
	assertContains(t, out, "Emerging Threats")
	assertContains(t, out, "et-compromised")
}

func TestASNTemplateMinimal(t *testing.T) {
	t.Parallel()
	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Skipf("template dir not available: %v", err)
	}
	ctx := &markdown.ASNPageContext{
		ASN:      99999,
		Provider: "Test",
		Totals:   markdown.ASNTotals{Feeds: 0, IPs: 0},
	}

	out, err := s.Execute("asn.md.tmpl", ctx)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertContains(t, out, "AS99999")
	assertNotContains(t, out, "## Top Countries")
	assertNotContains(t, out, "## Top Categories")
}

func TestMaintainerTemplateRenders(t *testing.T) {
	t.Parallel()
	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Skipf("template dir not available: %v", err)
	}
	ctx := &markdown.MaintainerPageContext{
		Slug: "emerging-threats",
		Name: "Emerging Threats",
		URL:  "https://example.com",
		Totals: markdown.MaintainerTotals{
			Feeds: 25, IPs: 3_000_000, Categories: 4,
		},
		FeedsByCategory: map[string][]markdown.FeedInEntity{
			"attack": {
				{Name: "et-compromised", Category: "attack", IPs: 500_000, Health: "healthy"},
			},
			"malware": {
				{Name: "et-malware", Category: "malware", IPs: 200_000, Health: "healthy"},
			},
		},
	}

	out, err := s.Execute("maintainer.md.tmpl", ctx)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertContains(t, out, "Emerging Threats")
	assertContains(t, out, "https://example.com")
	assertContains(t, out, "3,000,000")
	assertContains(t, out, "et-compromised")
	assertContains(t, out, "et-malware")
}

func TestMaintainerTemplateMinimal(t *testing.T) {
	t.Parallel()
	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Skipf("template dir not available: %v", err)
	}
	ctx := &markdown.MaintainerPageContext{
		Slug:   "test",
		Name:   "Test",
		Totals: markdown.MaintainerTotals{Feeds: 0, IPs: 0},
	}

	out, err := s.Execute("maintainer.md.tmpl", ctx)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertContains(t, out, "Test")
	assertNotContains(t, out, "## Feeds by Category")
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("output missing %q\nfull output:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("output unexpectedly contains %q", needle)
	}
}
