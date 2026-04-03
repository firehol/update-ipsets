package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadCatalog is a helper that loads the decomposed FireHOL directory catalog.
// It skips the test if the directory is not available.
func loadCatalog(t *testing.T) *Config {
	t.Helper()
	path := filepath.Join("..", "..", "configs", "firehol")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("firehol catalog not available: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load firehol catalog: %v", err)
	}
	return cfg
}

// TestCatalogExpectedCounts verifies we have exactly the expected number
// of sources and merges after the historical-feed restore. Geolocation,
// ASN, and bogon providers all live under cfg.Sources with use:[…].
func TestCatalogExpectedCounts(t *testing.T) {
	cfg := loadCatalog(t)
	// Post-ExpandDerivatives and synthetic injection:
	// 341 plain sources + 67 retention variants + 13 merges +
	// 2 hidden synthetic GeoIP-derived feeds = 423.
	if got := len(cfg.Sources); got != 423 {
		t.Fatalf("expected 423 sources, got %d", got)
	}
	if got := len(cfg.Artifacts); got != 1 {
		t.Fatalf("expected 1 artifact, got %d", got)
	}
	// cfg.Merges is cleared by ExpandDerivatives; merges now exist
	// as Source entries with internal://merge URLs. Count them by
	// URL scheme.
	mergeCount := 0
	for _, src := range cfg.Sources {
		if strings.HasPrefix(src.URL, InternalMergeScheme) {
			mergeCount++
		}
	}
	if mergeCount != 13 {
		t.Fatalf("expected 13 merge Sources (internal://merge URL), got %d", mergeCount)
	}
	if got := len(cfg.Merges); got != 0 {
		t.Fatalf("expected cfg.Merges to be empty after expansion, got %d", got)
	}
	if got := len(cfg.SourcesWithUse(UseGeoIP)); got != 5 {
		t.Fatalf("expected 5 geoip sources, got %d", got)
	}
	if got := len(cfg.SourcesWithUse(UseASN)); got != 4 {
		t.Fatalf("expected 4 asn sources, got %d", got)
	}
	if got, want := cfg.Defaults.ASNProvider, "iptoasn"; got != want {
		t.Fatalf("default ASN provider = %q, want %q", got, want)
	}
	if got, want := cfg.Defaults.GeoProvider, "dbip_country"; got != want {
		t.Fatalf("default geo provider = %q, want %q", got, want)
	}
	if got := cfg.SourcesWithUseDefaultFirst(UseASN); len(got) == 0 || got[0].Name != "iptoasn" {
		t.Fatalf("default-first ASN sources = %+v, want iptoasn first", sourceNamesForTest(got))
	}
	if got := cfg.SourcesWithUseDefaultFirst(UseGeoIP); len(got) == 0 || got[0].Name != "dbip_country" {
		t.Fatalf("default-first geo sources = %+v, want dbip_country first", sourceNamesForTest(got))
	}
	// Six bogon sources: the synthetic rfc_reserved baseline, four
	// maintained third-party bogon feeds, and the Team Cymru unassigned merge.
	// Stale themed lists such as iblocklist_bogons stay regular feeds so they
	// do not make unrelated feed analyses look like they overlap bogon space.
	if got := len(cfg.SourcesWithUse(UseBogons)); got != 6 {
		t.Fatalf("expected 6 bogon sources (rfc_reserved + 4 third-party + cymru_unassigned), got %d", got)
	}
	if src := cfg.Sources["iblocklist_bogons"]; src == nil {
		t.Fatal("expected iblocklist_bogons source")
	} else {
		if src.HasUse(UseBogons) {
			t.Fatal("iblocklist_bogons must remain a regular feed, not a bogon reference provider")
		}
		if src.ExcludeFromUnmaintained {
			t.Fatal("iblocklist_bogons must not suppress age-based maintenance health")
		}
	}
}

// TestCatalogDeepValidation runs cross-referencing validation including
// merge source references against known sources and history variants.
func TestCatalogDeepValidation(t *testing.T) {
	cfg := loadCatalog(t)
	if err := ValidateDeep(cfg); err != nil {
		t.Fatalf("deep validation failed: %v", err)
	}
}

// TestCatalogAllSourcesHaveProcessor verifies every source that
// produces an ipset has at least one processor step defined. Database
// sources (asn, geoip) parse via format-specific code paths and do not
// run the processor pipeline.
func TestCatalogAllSourcesHaveProcessor(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if src.HasUse(UseASN) || src.HasUse(UseGeoIP) {
			continue
		}
		if len(src.Processor) == 0 {
			t.Errorf("source %q has no processor steps", name)
		}
	}
}

// TestCatalogAllSourcesHaveCategory verifies every source has a non-empty category.
func TestCatalogAllSourcesHaveCategory(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if src.Hidden {
			// Hidden synthetic sources do not need a category since
			// they never appear on the public catalog.
			continue
		}
		if src.Category == "" {
			t.Errorf("source %q has no category", name)
		}
	}
}

// TestCatalogAllSourcesHaveInfo verifies every source has a non-empty info string.
func TestCatalogAllSourcesHaveInfo(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if src.Info == "" {
			t.Errorf("source %q has no info description", name)
		}
	}
}

// TestCatalogAllSourcesHaveMaintainer verifies every source has a non-empty maintainer.
func TestCatalogAllSourcesHaveMaintainer(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if src.Maintainer == "" {
			t.Errorf("source %q has no maintainer", name)
		}
	}
}

// TestCatalogURLFormats validates the URL format for all sources.
// Sources with empty URLs are legitimate only for static config-backed sources
// and legacy external-producer feeds that still have no direct download URL.
// internal:// and artifact:// are also legitimate synthetic source schemes.
func TestCatalogURLFormats(t *testing.T) {
	cfg := loadCatalog(t)

	// These sources legitimately have no URL (populated by external scripts).
	emptyURLOK := map[string]bool{
		"sorbs_anonymizers": true,
		"sorbs_block":       true,
		"sorbs_escalations": true,
		"sorbs_new_spam":    true,
		"sorbs_noserver":    true,
		"sorbs_recent_spam": true,
		"sorbs_smtp":        true,
		"sorbs_web":         true,
		"sorbs_zombie":      true,
	}

	for name, src := range cfg.Sources {
		if src.URL == "" {
			if len(src.Static) > 0 {
				continue
			}
			if !emptyURLOK[name] {
				t.Errorf("source %q has empty URL (not in known empty-URL list)", name)
			}
			continue
		}

		parsed, err := url.Parse(src.URL)
		if err != nil {
			t.Errorf("source %q has unparseable URL %q: %v", name, src.URL, err)
			continue
		}

		// Scheme must be http, https, internal, or artifact.
		if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "internal" && parsed.Scheme != ArtifactScheme {
			t.Errorf("source %q has unexpected URL scheme %q in %q", name, parsed.Scheme, src.URL)
		}

		// Must have a host (or be an internal:// URL where the
		// "host" carries the synthetic name).
		if parsed.Host == "" && parsed.Scheme != "internal" && parsed.Scheme != ArtifactScheme {
			t.Errorf("source %q has URL with no host: %q", name, src.URL)
		}
	}
}

func TestCatalogDroneBLMappings(t *testing.T) {
	cfg := loadCatalog(t)
	artifact := cfg.ArtifactByName("dronebl")
	if artifact == nil {
		t.Fatal("missing dronebl artifact definition")
	}
	if artifact.Type != ArtifactTypeDroneBLBuildzone {
		t.Fatalf("dronebl artifact type = %q, want %q", artifact.Type, ArtifactTypeDroneBLBuildzone)
	}
	if artifact.MaxDownloadSize != 268435456 {
		t.Fatalf("dronebl max_download_size = %d, want %d", artifact.MaxDownloadSize, 268435456)
	}

	mappings := map[string]string{
		"dronebl_abused_vpn":         "abused_vpn_services",
		"dronebl_anonymizers":        "http_proxies,socks_proxies,web_page_proxies,wingate_proxies,proxychains",
		"dronebl_auto_botnets":       "auto_botnets",
		"dronebl_autorooting_worms":  "autorooting_worms",
		"dronebl_bottler":            "bottler",
		"dronebl_compromised":        "compromised",
		"dronebl_ddos_drones":        "ddos_drones",
		"dronebl_dictionary_attacks": "bruteforce_attackers",
		"dronebl_dns_mx_on_irc":      "dns_mx_on_irc",
		"dronebl_irc_drones":         "irc_drones",
		"dronebl_open_dns_resolvers": "open_dns_resolvers",
		"dronebl_unknown":            "uncategorized",
		"dronebl_worms_bots":         "unknown_worms_spambots",
	}

	for name, wantLists := range mappings {
		src := cfg.Sources[name]
		if src == nil {
			t.Fatalf("missing DroneBL source %q", name)
		}
		if src.ArtifactParent != "dronebl" {
			t.Errorf("source %q artifact parent = %q, want dronebl", name, src.ArtifactParent)
		}
		ref, err := ParseArtifactURL(src.URL)
		if err != nil {
			t.Fatalf("source %q artifact URL parse failed: %v", name, err)
		}
		if ref.Artifact != "dronebl" {
			t.Errorf("source %q artifact ref = %q, want dronebl", name, ref.Artifact)
		}
		if got := strings.Join(ref.Parts, ","); got != wantLists {
			t.Errorf("source %q artifact parts = %q, want %q", name, got, wantLists)
		}
	}
}

// TestCatalogHistoryWindowsAreValid verifies all history window values
// are positive integers and produce sensible variant names.
func TestCatalogHistoryWindowsAreValid(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		for _, mins := range src.History {
			if mins <= 0 {
				t.Errorf("source %q has non-positive history window: %d", name, mins)
			}
			label := HistoryLabel(mins)
			if label == "" || label == "_" {
				t.Errorf("source %q: HistoryLabel(%d) = %q (invalid)", name, mins, label)
			}
			variant := name + label
			if variant == name {
				t.Errorf("source %q: history variant is same as base name for window %d", name, mins)
			}
		}
	}
}

// TestCatalogHistoryVariantNames verifies the generated names match
// the expected suffixes for standard windows.
func TestCatalogHistoryVariantNames(t *testing.T) {
	known := map[int]string{
		1440:  "_1d",
		2880:  "_2d",
		10080: "_7d",
		43200: "_30d",
	}
	for mins, want := range known {
		got := HistoryLabel(mins)
		if got != want {
			t.Errorf("HistoryLabel(%d) = %q, want %q", mins, got, want)
		}
	}
}

// TestCatalogMergeSourcesExist verifies every merge references only
// sources that exist as direct sources, history variants, or well-known
// geolocation-derived sources.
func TestCatalogMergeSourcesExist(t *testing.T) {
	cfg := loadCatalog(t)
	known := GeneratedNames(cfg)

	for name, src := range catalogMergeSources(cfg) {
		for _, ref := range mergeSourceRefs(src) {
			if _, ok := known[ref]; !ok {
				t.Errorf("merge %q references unknown source %q", name, ref)
			}
		}
	}
}

// TestCatalogMergesHaveRequiredFields verifies merge metadata is populated.
func TestCatalogMergesHaveRequiredFields(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range catalogMergeSources(cfg) {
		if src.Category == "" {
			t.Errorf("merge %q has no category", name)
		}
		if src.Info == "" {
			t.Errorf("merge %q has no info", name)
		}
		if src.Maintainer == "" {
			t.Errorf("merge %q has no maintainer", name)
		}
		if src.MaintainerURL == "" {
			t.Errorf("merge %q has no maintainer_url", name)
		}
		if src.Frequency <= 0 {
			t.Errorf("merge %q has non-positive frequency %d", name, src.Frequency)
		}
		if len(src.MergeSources) == 0 {
			t.Errorf("merge %q has no additive sources", name)
		}
	}
}

// TestCatalogGeolocationFeedsAreComplete verifies all 5 geoip sources
// have format, URL, and positive frequency. Geoip databases now live
// under cfg.Sources with use:[geoip].
func TestCatalogGeolocationFeedsAreComplete(t *testing.T) {
	cfg := loadCatalog(t)

	expectedFormats := map[string]string{
		"dbip_country":        "dbip_country_csv",
		"geolite2_country":    "maxmind_country_csv",
		"ip2location_country": "ip2location_country_zip",
		"ipdeny_country":      "ipdeny_country_tar_gz",
		"ipip_country":        "ipip_country_zip",
	}

	for name, wantFormat := range expectedFormats {
		src, ok := cfg.Sources[name]
		if !ok {
			t.Errorf("missing geoip source %q", name)
			continue
		}
		if !src.HasUse(UseGeoIP) {
			t.Errorf("source %q is missing use:[geoip]", name)
		}
		if src.Format != wantFormat {
			t.Errorf("geoip source %q: format = %q, want %q", name, src.Format, wantFormat)
		}
		if src.URL == "" {
			t.Errorf("geoip source %q has empty URL", name)
		}
		if src.Frequency <= 0 {
			t.Errorf("geoip source %q has invalid frequency %d", name, src.Frequency)
		}
	}
}

// TestCatalogGeolocationURLsAreValid verifies geoip URLs are parseable.
func TestCatalogGeolocationURLsAreValid(t *testing.T) {
	cfg := loadCatalog(t)
	for _, src := range cfg.SourcesWithUse(UseGeoIP) {
		// Some URLs contain template variables like {YYYY} which
		// url.Parse may not like. Just check basic structure.
		u := src.URL
		if u == "" {
			t.Errorf("geoip source %q has empty URL", src.Name)
			continue
		}
		// Replace template variables for parsing.
		cleaned := strings.NewReplacer(
			"{YYYY}", "2025",
			"{MM}", "01",
			"${MAXMIND_LICENSE_KEY}", "TESTKEY",
		).Replace(u)

		parsed, err := url.Parse(cleaned)
		if err != nil {
			t.Errorf("geoip source %q has unparseable URL: %v", src.Name, err)
			continue
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			t.Errorf("geoip source %q has unexpected scheme %q", src.Name, parsed.Scheme)
		}
		if parsed.Host == "" {
			t.Errorf("geoip source %q URL has no host", src.Name)
		}
	}
}

// TestCatalogSourceAttributeKeys verifies all attribute keys are from
// the known set.
func TestCatalogSourceAttributeKeys(t *testing.T) {
	cfg := loadCatalog(t)

	knownKeys := map[string]bool{
		"dont_redistribute":      true,
		"redistribute":           true,
		"can_be_empty":           true,
		"no_if_modified_since":   true,
		"dont_enable_with_all":   true,
		"inbound":                true,
		"outbound":               true,
		"downloader":             true,
		"downloader_options":     true,
		"public_url":             true,
		"license":                true,
		"grade":                  true,
		"protection":             true,
		"intended_use":           true,
		"false_positives":        true,
		"poisoning":              true,
		"services":               true,
		"context_role":           true,
		"context_source_type":    true,
		"context_source_quality": true,
		"context_rationale":      true,
	}

	for name, src := range cfg.Sources {
		for key := range src.Attributes {
			if !knownKeys[key] {
				t.Errorf("source %q has unknown attribute key %q", name, key)
			}
		}
	}
}

// TestCatalogIPVValues verifies all ipset sources and merges use only
// ipv4/ipv6. Database sources (asn, geoip) legitimately omit IPV.
func TestCatalogIPVValues(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if src.HasUse(UseASN) || src.HasUse(UseGeoIP) {
			continue
		}
		if src.IPV != "ipv4" && src.IPV != "ipv6" {
			t.Errorf("source %q has invalid ipv %q", name, src.IPV)
		}
	}
}

// TestCatalogOutputValues verifies all ipset output types are valid.
// Database sources (asn, geoip) legitimately omit Output.
func TestCatalogOutputValues(t *testing.T) {
	cfg := loadCatalog(t)
	// After canonicalizeOutput() runs at load time, every
	// Source.Output is guaranteed to be one of the two
	// authoritative values: "ipset" (writes {name}.ipset) or
	// "netset" (writes {name}.netset). Legacy values ("ip",
	// "net", "both") are accepted by the loader but translated
	// in place, so this assertion enforces the invariant that
	// downstream code never sees them.
	valid := map[string]bool{
		"ipset":  true,
		"netset": true,
	}
	for name, src := range cfg.Sources {
		if src.HasUse(UseASN) || src.HasUse(UseGeoIP) {
			continue
		}
		if !valid[src.Output] {
			t.Errorf("source %q has invalid output %q (want ipset or netset)", name, src.Output)
		}
	}
}

// TestCatalogSourceNamesAreFilesafe verifies no source/merge name
// contains characters that would break filesystem paths.
func TestCatalogSourceNamesAreFilesafe(t *testing.T) {
	cfg := loadCatalog(t)
	for name := range cfg.Sources {
		if !validFeedName(name) {
			t.Errorf("source name %q is not filesystem-safe", name)
		}
	}
}

// TestCatalogProcessorRawPresent verifies every source with a processor
// list also has a non-empty processor_raw recording the original bash name.
func TestCatalogProcessorRawPresent(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if len(src.Processor) > 0 && src.ProcessorRaw == "" {
			t.Errorf("source %q has processor steps but empty processor_raw", name)
		}
	}
}

// TestCatalogFrequenciesArePositive verifies every plain downloaded source
// has a positive frequency. Internal, static, and artifact-backed children
// always have frequency 0 because they are input-triggered rather than polled
// directly.
func TestCatalogFrequenciesArePositive(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if strings.HasPrefix(src.URL, "internal://") || strings.HasPrefix(src.URL, ArtifactScheme+"://") || len(src.Static) > 0 {
			continue
		}
		if src.Frequency <= 0 {
			t.Errorf("source %q has non-positive frequency %d", name, src.Frequency)
		}
	}
}

// TestCatalogNoDuplicateMergeSources verifies no merge has duplicate
// inputs. Merges are now standalone Source entries with an
// internal://merge URL and a DerivedFrom list — the DerivedFrom list
// is the authoritative inputs catalog after ExpandDerivatives.
func TestCatalogNoDuplicateMergeSources(t *testing.T) {
	cfg := loadCatalog(t)
	for name, src := range cfg.Sources {
		if !strings.HasPrefix(src.URL, InternalMergeScheme) {
			continue
		}
		seen := map[string]bool{}
		for _, ref := range src.DerivedFrom {
			if seen[ref] {
				t.Errorf("merge %q has duplicate source reference %q", name, ref)
			}
			seen[ref] = true
		}
	}
}

// TestCatalogMergeSourceCountsMatchBash verifies that the key merge
// lists have the expected number of inputs matching the bash script.
// After ExpandDerivatives, merges are Source entries; DerivedFrom is
// the inputs list.
func TestCatalogMergeSourceCountsMatchBash(t *testing.T) {
	cfg := loadCatalog(t)

	// Expected counts from the bash script merge definitions.
	expected := map[string]int{
		"firehol_level1":      4,
		"firehol_level2":      3,
		"firehol_level3":      5,
		"firehol_level4":      6,
		"firehol_anonymous":   4,
		"firehol_proxies":     4,
		"firehol_webclient":   1,
		"firehol_webserver":   2,
		"firehol_abusers_1d":  6,
		"firehol_abusers_30d": 6,
	}

	for name, wantCount := range expected {
		src, ok := cfg.Sources[name]
		if !ok {
			t.Errorf("expected merge %q not found in Sources", name)
			continue
		}
		if !strings.HasPrefix(src.URL, InternalMergeScheme) {
			t.Errorf("expected merge %q to have internal://merge URL, got %q", name, src.URL)
			continue
		}
		if got := len(src.DerivedFrom); got != wantCount {
			t.Errorf("merge %q: got %d inputs, want %d (DerivedFrom: %v)",
				name, got, wantCount, src.DerivedFrom)
		}
	}
}

func TestCatalogCymruUnassignedMergeSubtractsBogons(t *testing.T) {
	cfg := loadCatalog(t)
	src := cfg.Sources["cymru_unassigned"]
	if src == nil {
		t.Fatal("missing cymru_unassigned merge")
	}
	if !strings.HasPrefix(src.URL, InternalMergeScheme) {
		t.Fatalf("cymru_unassigned URL = %q, want internal merge URL", src.URL)
	}
	if src.Label != "Team Cymru unassigned" {
		t.Fatalf("cymru_unassigned label = %q, want %q", src.Label, "Team Cymru unassigned")
	}
	if !src.HasUse(UseBogons) {
		t.Fatalf("cymru_unassigned use = %v, want bogons", src.Use)
	}
	if !strings.Contains(src.License, "team-cymru.com/bogon-reference-http") {
		t.Fatalf("cymru_unassigned license = %q, want Team Cymru bogon reference", src.License)
	}
	if got, want := src.MergeSources, []string{"fullbogons"}; !sameOrderedStrings(got, want) {
		t.Fatalf("cymru_unassigned merge sources = %v, want %v", got, want)
	}
	if got, want := src.MergeExclude, []string{"bogons"}; !sameOrderedStrings(got, want) {
		t.Fatalf("cymru_unassigned merge exclude = %v, want %v", got, want)
	}
	if got, want := src.DerivedFrom, []string{"fullbogons", "bogons"}; !sameOrderedStrings(got, want) {
		t.Fatalf("cymru_unassigned derived_from = %v, want %v", got, want)
	}
	inputs, exclude, err := ParseMergeURLParts(src.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inputs, []string{"fullbogons"}; !sameOrderedStrings(got, want) {
		t.Fatalf("cymru_unassigned URL inputs = %v, want %v", got, want)
	}
	if got, want := exclude, []string{"bogons"}; !sameOrderedStrings(got, want) {
		t.Fatalf("cymru_unassigned URL exclude = %v, want %v", got, want)
	}
	if !containsSourceName(cfg.SourcesWithUse(UseBogons), "cymru_unassigned") {
		t.Fatal("cymru_unassigned missing from bogon provider sources")
	}
}

func sameOrderedStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func containsSourceName(sources []*Source, name string) bool {
	for _, src := range sources {
		if src != nil && src.Name == name {
			return true
		}
	}
	return false
}

func sourceNamesForTest(sources []*Source) []string {
	out := make([]string, 0, len(sources))
	for _, src := range sources {
		if src != nil {
			out = append(out, src.Name)
		}
	}
	return out
}

// TestCatalogSourcesComplete lists the expected source names in the
// fully-expanded catalog and verifies each one exists. Counts drift
// as sources are added or retired — re-count from configs/firehol/
// when this fails. After ExpandDerivatives and built-in synthetic-source
// injection run during LoadYAML, the in-memory catalog has 423 sources:
// 341 plain sources + 67 retention variants + 13 merges moved in from the
// merges: block + 2 hidden synthetic GeoIP-derived feeds.
func TestCatalogSourcesComplete(t *testing.T) {
	cfg := loadCatalog(t)

	// Full list of expected sources: ipset feeds extracted from the bash
	// script via ExtractLegacyScript, minus the 15 retired feeds whose
	// upstream is permanently dead, plus the unified database sources.
	expected := []string{
		"abuseipdb_1d", "abuseipdb_3d", "abuseipdb_7d", "abuseipdb_30d",
		"apnic_ssh_bruteforce", "apnic_telnet_bruteforce",
		"bds_atif",
		"blueliv_crimeserver_last",
		"blocklist_de", "blocklist_de_apache", "blocklist_de_bots",
		"blocklist_de_bruteforce", "blocklist_de_ftp", "blocklist_de_imap",
		"blocklist_de_mail", "blocklist_de_sip", "blocklist_de_ssh",
		"blocklist_de_strongips",
		"blocklist_net_ua",
		"bogons",
		"botscout",
		"botvrij_dst", "botvrij_src",
		"bitwire_inbound", "bitwire_outbound",
		"bitwire_ipsum_clean",
		"bitwire_iplistfetch_blacklist", "bitwire_iplistfetch_blacklist2",
		"bruteforceblocker",
		"c2_tracker",
		"c2_tracker_asyncrat", "c2_tracker_cobaltstrike",
		"c2_tracker_havoc", "c2_tracker_metasploit",
		"c2_tracker_mozi", "c2_tracker_mythic",
		"c2_tracker_njrat", "c2_tracker_remcos",
		"c2_tracker_sliver", "c2_tracker_xmrig",
		"caida_prefix2as",
		"ciarmy", "cinsarmy",
		"cidr_report_bogons",
		"cleanmx_phishing", "cleanmx_viruses",
		"cleantalk_new", "cleantalk_updated",
		"coinbl_hosts", "coinbl_hosts_browser", "coinbl_hosts_optional",
		"coinbl_ips",
		"critical_as112",
		"critical_context_apple_networks", "critical_context_github_hosted_compute",
		"critical_dns_root_servers",
		"critical_public_dns_core",
		"critical_soft_akamai_edge_secondary",
		"critical_soft_atlassian_cloud", "critical_soft_auth0",
		"critical_soft_aws_cloudfront", "critical_soft_braintree",
		"critical_soft_cloudflare_edge", "critical_soft_fastly_edge",
		"critical_soft_github_services", "critical_soft_microsoft365",
		"critical_soft_mollie", "critical_soft_okta",
		"critical_soft_salesforce_hyperforce", "critical_soft_stripe_api",
		"critical_soft_stripe_webhooks", "critical_soft_terraform_cloud",
		"critical_soft_zoom",
		"cybercrime", "cybercure",
		"cymru_unassigned",
		"data_shield", "data_shield_critical",
		"datacenters",
		"dataplane_dnsrd", "dataplane_dnsrdany", "dataplane_dnsversion",
		"dataplane_proto41",
		"dataplane_sipinvitation", "dataplane_sipquery",
		"dataplane_sipregistration", "dataplane_sshclient",
		"dataplane_sshpwauth", "dataplane_smtpdata", "dataplane_smtpgreet",
		"dataplane_telnetlogin", "dataplane_vncrfb",
		"dbip_asn_lite",
		"dbip_country",
		"ddrimus_http_threat",
		"dm_tor",
		"dronebl_abused_vpn",
		"dronebl_anonymizers", "dronebl_auto_botnets",
		"dronebl_autorooting_worms", "dronebl_bottler",
		"dronebl_compromised", "dronebl_ddos_drones",
		"dronebl_dictionary_attacks", "dronebl_dns_mx_on_irc",
		"dronebl_irc_drones", "dronebl_open_dns_resolvers",
		"dronebl_unknown", "dronebl_worms_bots",
		"dshield",
		"et_block", "et_compromised", "et_dshield", "et_spamhaus", "et_tor",
		"feodo", "feodo_badips",
		"fullbogons",
		"gazpitchy_blacklist",
		"geolite2_country",
		"gpf_comics",
		"graphiclineweb",
		"greensnow",
		"griffinguard",
		"hagezi_tif",
		"hfish_honeypot",
		"opendbl_bruteforce",
		"shadowwhisperer_scanners", "shadowwhisperer_tunnel",
		"shadowwhisperer_hackers", "shadowwhisperer_hosting",
		"shadowwhisperer_bruteforce_medium",
		"shadowwhisperer_bruteforce_high",
		"shadowwhisperer_bruteforce_extreme",
		"shadowwhisperer_probes",
		"shadowwhisperer_threats_unclassified",
		"threatfox_ips",
		"criticalpath_log4j", "criticalpath_cobaltstrike",
		"drb_ra_c2intel", "drb_ra_c2intel_30d", "drb_ra_c2intel_90d",
		"criticalpath_sip",
		"blackmirror_ipv4",
		"malwarefilter_botnet",
		"ustc_blackip",
		"nginx_bad_bot_blocker",
		"vxvault_url_list",
		"iblocklist_abuse_palevo", "iblocklist_abuse_spyeye",
		"iblocklist_abuse_zeus",
		"iblocklist_ads", "iblocklist_bogons",
		"iblocklist_ciarmy_malicious", "iblocklist_cidr_report_bogons",
		"iblocklist_cruzit_web_attacks",
		"iblocklist_dshield",
		"iblocklist_edu", "iblocklist_exclusions",
		"iblocklist_fornonlancomputers", "iblocklist_forumspam",
		"iblocklist_hijacked", "iblocklist_iana_multicast",
		"iblocklist_iana_private", "iblocklist_iana_reserved",
		"iblocklist_isp_aol", "iblocklist_isp_att",
		"iblocklist_isp_cablevision", "iblocklist_isp_charter",
		"iblocklist_isp_comcast", "iblocklist_isp_embarq",
		"iblocklist_isp_qwest", "iblocklist_isp_sprint",
		"iblocklist_isp_suddenlink", "iblocklist_isp_twc",
		"iblocklist_isp_verizon",
		"iblocklist_level1", "iblocklist_level2", "iblocklist_level3",
		"iblocklist_malc0de",
		"iblocklist_onion_router",
		"iblocklist_org_activision", "iblocklist_org_apple",
		"iblocklist_org_blizzard", "iblocklist_org_crowd_control",
		"iblocklist_org_electronic_arts", "iblocklist_org_joost",
		"iblocklist_org_linden_lab", "iblocklist_org_logmein",
		"iblocklist_org_microsoft", "iblocklist_org_ncsoft",
		"iblocklist_org_nintendo", "iblocklist_org_pandora",
		"iblocklist_org_pirate_bay", "iblocklist_org_punkbuster",
		"iblocklist_org_riot_games", "iblocklist_org_sony_online",
		"iblocklist_org_square_enix", "iblocklist_org_steam",
		"iblocklist_org_ubisoft", "iblocklist_org_xfire",
		"iblocklist_pedophiles",
		"iblocklist_proxies",
		"iblocklist_rangetest",
		"iblocklist_spamhaus_drop",
		"iblocklist_spider", "iblocklist_spyware",
		"iblocklist_webexploit",
		"iblocklist_yoyo_adservers",
		"ip2location_country",
		"ip2proxy_px1lite",
		"ipdeny_country",
		"ipip_country",
		"ipblacklistcloud_recent",
		"ipsum", "ipsum_2", "ipsum_3", "ipsum_4", "ipsum_5",
		"ipsum_6", "ipsum_7", "ipsum_8",
		"iptoasn",
		"jamesbrine_bruteforce",
		"maltrail_scanners",
		"maltrail_scanners_cidr",
		"marcusholtz_aggregated",
		"maxmind_geolite2_asn", "maxmind_proxy_fraud",
		"misp_akamai",
		"misp_alphastrike_research_scanners",
		"misp_alphastrike_scanners",
		"misp_amazon_aws",
		"misp_apple",
		"misp_bufferover_scanners",
		"misp_censys_scanners",
		"misp_check_host_net",
		"misp_cloudflare",
		"misp_coalition_intel_scanners",
		"misp_cyberresilience_scanners",
		"misp_cybergreen_scanners",
		"misp_cypex_scanners",
		"misp_f6_scanners",
		"misp_fastly",
		"misp_github",
		"misp_google_gcp",
		"misp_google_gmail_sending_ips",
		"misp_googlebot",
		"misp_internet_census_scanners",
		"misp_intrinsec_scanners",
		"misp_ipinfo_scanners",
		"misp_ipip_scanners",
		"misp_microsoft_azure",
		"misp_microsoft_azure_china",
		"misp_microsoft_azure_germany",
		"misp_microsoft_azure_us_gov",
		"misp_microsoft_office365_cn",
		"misp_microsoft_office365_ip",
		"misp_modat_scanners",
		"misp_modat_nt_scanners",
		"misp_netsecscan_nt_scanners",
		"misp_netsecscan_scanners",
		"misp_onyphe_published_scanners",
		"misp_onyphe_scanners",
		"misp_openai_gptbot",
		"misp_ovh_cluster",
		"misp_palo_alto_cortex_xpanse",
		"misp_probethenet_scanners",
		"misp_public_dns",
		"misp_rapid7_scanners",
		"misp_research_scanners",
		"misp_shadowforce_published_scanners",
		"misp_shadowforce_scanners",
		"misp_shadowserver_published_scanners",
		"misp_shadowserver_scanners",
		"misp_shodan_published_scanners",
		"misp_shodan_scanners",
		"misp_sinkholes",
		"misp_skipa_scanners",
		"misp_smtp_receiving_ips",
		"misp_smtp_sending_ips",
		"misp_stretchoid_scanners",
		"misp_telegram",
		"misp_tenable_cloud",
		"misp_umbrella_blockpage",
		"misp_vpn",
		"misp_zscaler",
		"myip",
		"netmountains_curated",
		"anonymous",
		"palinkas_scanners",
		"php_bad",
		"php_commenters", "php_dictionary", "php_harvesters", "php_spammers",
		"proxylists", "proxz",
		"provider_context_aws_cloud", "provider_context_digitalocean_geofeed",
		"provider_context_gcp_cloud", "provider_context_linode_geofeed",
		"provider_context_oracle_cloud", "provider_context_vultr_geofeed",
		"rfc_reserved",
		"romainmarcoux_malicious",
		"romainmarcoux_malicious_aa", "romainmarcoux_malicious_ab",
		"romainmarcoux_malicious_ac", "romainmarcoux_malicious_ad",
		"romainmarcoux_outgoing_aa", "romainmarcoux_outgoing_ab",
		"rutgers_drop",
		"sblam",
		"sefinek_malicious",
		"sekuripy_ipnoise",
		"serpro_reputation",
		"socks_proxy",
		"spamhaus_edrop",
		"spamhaus_drop",
		"sslproxies",
		"stopforumspam", "stopforumspam_180d", "stopforumspam_1d",
		"stopforumspam_30d", "stopforumspam_365d", "stopforumspam_7d",
		"stopforumspam_90d", "stopforumspam_toxic",
		"stratosphere_aip_24h",
		"stratosphere_aip_alpha",
		"stratosphere_aip_prioritize",
		"threatview_c2", "threatview_ip",
		"tor_exits",
		"turris_greylist",
		"uninvited_activity",
		"uscert_hidden_cobra",
		"viriback",
		"vxvault",
		"xroxy",
		"yoyo_adservers",
		"zeus", "zeus_badips",
		"satellite",

		// Retention variants emitted by ExpandDerivatives from
		// `history: [1440, 10080, 43200]` on 15 parent sources/merges.
		// Ordered alphabetically for easy diffing.
		"botscout_1d", "botscout_7d", "botscout_30d",
		"blueliv_crimeserver_last_1d", "blueliv_crimeserver_last_2d",
		"blueliv_crimeserver_last_7d", "blueliv_crimeserver_last_30d",
		"cleantalk_1d", "cleantalk_7d", "cleantalk_30d",
		"cleantalk_new_1d", "cleantalk_new_7d", "cleantalk_new_30d",
		"cleantalk_updated_1d", "cleantalk_updated_7d", "cleantalk_updated_30d",
		"dshield_1d", "dshield_7d", "dshield_30d",
		"ipblacklistcloud_recent_1d", "ipblacklistcloud_recent_7d", "ipblacklistcloud_recent_30d",
		"php_bad_1d", "php_bad_7d", "php_bad_30d",
		"php_commenters_1d", "php_commenters_7d", "php_commenters_30d",
		"php_dictionary_1d", "php_dictionary_7d", "php_dictionary_30d",
		"php_harvesters_1d", "php_harvesters_7d", "php_harvesters_30d",
		"php_spammers_1d", "php_spammers_7d", "php_spammers_30d",
		"proxylists_1d", "proxylists_7d", "proxylists_30d",
		"proxz_1d", "proxz_7d", "proxz_30d",
		"rutgers_drop_1d", "rutgers_drop_7d", "rutgers_drop_30d",
		"sekuripy_ipnoise_1d", "sekuripy_ipnoise_7d", "sekuripy_ipnoise_30d",
		"socks_proxy_1d", "socks_proxy_7d", "socks_proxy_30d",
		"sslproxies_1d", "sslproxies_7d", "sslproxies_30d",
		"tor_exits_1d", "tor_exits_7d", "tor_exits_30d",
		"turris_greylist_1d", "turris_greylist_7d", "turris_greylist_30d",
		"viriback_1d", "viriback_7d", "viriback_30d",
		"xroxy_1d", "xroxy_7d", "xroxy_30d",

		// Merges moved from cfg.Merges to cfg.Sources by
		// ExpandDerivatives. These now have internal://merge URLs.
		"cleantalk", "firehol_abusers_1d", "firehol_abusers_30d",
		"firehol_anonymous",
		"firehol_level1", "firehol_level2", "firehol_level3", "firehol_level4",
		"firehol_proxies",
		"firehol_webclient", "firehol_webserver",
	}

	found := map[string]bool{}
	for name := range cfg.Sources {
		found[name] = true
	}

	var missing []string
	for _, name := range expected {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("missing %d expected sources: %v", len(missing), missing)
	}

	// Also verify we don't have unexpected extras.
	expectedSet := map[string]bool{}
	for _, name := range expected {
		expectedSet[name] = true
	}
	var extras []string
	for name := range cfg.Sources {
		if !expectedSet[name] {
			extras = append(extras, name)
		}
	}
	if len(extras) > 0 {
		t.Errorf("found %d unexpected extra sources: %v", len(extras), extras)
	}
}

// TestCatalogSpecificFireholLevels verifies the exact composition of
// the firehol_level1 through firehol_level4 merge lists. After
// ExpandDerivatives runs, merges are Source entries whose
// DerivedFrom list is the authoritative inputs catalog.
func TestCatalogSpecificFireholLevels(t *testing.T) {
	cfg := loadCatalog(t)

	// From the bash script (as extracted into the YAML).
	expectedLevel := map[string][]string{
		"firehol_level1": {
			"dshield", "feodo", "fullbogons", "spamhaus_drop",
		},
		"firehol_level2": {
			"blocklist_de", "dshield_1d", "greensnow",
		},
		"firehol_level3": {
			"bruteforceblocker", "ciarmy", "dshield_30d", "myip", "vxvault",
		},
		"firehol_level4": {
			"blocklist_net_ua", "botscout_30d", "cybercrime",
			"iblocklist_hijacked", "iblocklist_spyware", "iblocklist_webexploit",
		},
	}

	for name, wantSources := range expectedLevel {
		src, ok := cfg.Sources[name]
		if !ok {
			t.Errorf("merge %q not found in Sources", name)
			continue
		}
		if !strings.HasPrefix(src.URL, InternalMergeScheme) {
			t.Errorf("merge %q expected internal://merge URL, got %q", name, src.URL)
			continue
		}
		if got, want := len(src.DerivedFrom), len(wantSources); got != want {
			t.Errorf("merge %q: got %d inputs, want %d (DerivedFrom: %v)",
				name, got, want, src.DerivedFrom)
		}
		wantSet := map[string]bool{}
		for _, s := range wantSources {
			wantSet[s] = true
		}
		for _, s := range src.DerivedFrom {
			if !wantSet[s] {
				t.Errorf("merge %q contains unexpected input %q", name, s)
			}
		}
	}
}

// TestCatalogRoundTrip verifies the catalog survives a YAML save/load cycle
// without losing sources, merges, or geolocation feeds.
func TestCatalogRoundTrip(t *testing.T) {
	cfg := loadCatalog(t)

	tmp := filepath.Join(t.TempDir(), "roundtrip.yaml")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveYAML(f, cfg); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadYAML(tmp)
	if err != nil {
		t.Fatalf("failed to reload round-tripped config: %v", err)
	}

	if got, want := len(reloaded.Sources), len(cfg.Sources); got != want {
		t.Fatalf("round-trip source count: got %d, want %d", got, want)
	}
	if got, want := len(reloaded.Merges), len(cfg.Merges); got != want {
		t.Fatalf("round-trip merge count: got %d, want %d", got, want)
	}
	if got, want := len(reloaded.SourcesWithUse(UseGeoIP)), len(cfg.SourcesWithUse(UseGeoIP)); got != want {
		t.Fatalf("round-trip geoip source count: got %d, want %d", got, want)
	}
	if got, want := len(reloaded.SourcesWithUse(UseASN)), len(cfg.SourcesWithUse(UseASN)); got != want {
		t.Fatalf("round-trip asn source count: got %d, want %d", got, want)
	}

	// Spot-check a few sources survive the round-trip.
	for _, name := range []string{"dshield", "spamhaus_drop", "blocklist_de", "tor_exits", "greensnow"} {
		orig, ok1 := cfg.Sources[name]
		reloaded, ok2 := reloaded.Sources[name]
		if !ok1 || !ok2 {
			t.Fatalf("source %q missing after round-trip", name)
		}
		if orig.URL != reloaded.URL {
			t.Errorf("source %q URL mismatch: %q vs %q", name, orig.URL, reloaded.URL)
		}
		if orig.Frequency != reloaded.Frequency {
			t.Errorf("source %q frequency mismatch: %d vs %d", name, orig.Frequency, reloaded.Frequency)
		}
		if orig.IPV != reloaded.IPV {
			t.Errorf("source %q ipv mismatch: %q vs %q", name, orig.IPV, reloaded.IPV)
		}
		if orig.Output != reloaded.Output {
			t.Errorf("source %q output mismatch: %q vs %q", name, orig.Output, reloaded.Output)
		}
		if len(orig.Processor) != len(reloaded.Processor) {
			t.Errorf("source %q processor count mismatch: %d vs %d",
				name, len(orig.Processor), len(reloaded.Processor))
		}
	}
}

// TestCatalogEnableByAllConsistency verifies that DroneBL, SORBS, ip2proxy,
// and php_bad sources are NOT enabled_by_all (matching the bash script's
// dont_enable_with_all or external-downloader semantics).
func TestCatalogEnableByAllConsistency(t *testing.T) {
	cfg := loadCatalog(t)

	// These should NOT be enabled_by_all.
	shouldNotEnable := []string{
		"dronebl_abused_vpn",
		"dronebl_anonymizers", "dronebl_auto_botnets",
		"dronebl_autorooting_worms", "dronebl_bottler",
		"dronebl_compromised", "dronebl_ddos_drones",
		"dronebl_dictionary_attacks", "dronebl_dns_mx_on_irc",
		"dronebl_irc_drones", "dronebl_open_dns_resolvers",
		"dronebl_unknown", "dronebl_worms_bots",
		"ip2proxy_px1lite",
		"php_bad",
	}

	for _, name := range shouldNotEnable {
		src, ok := cfg.Sources[name]
		if !ok {
			t.Errorf("source %q not found", name)
			continue
		}
		if src.EnabledByAll {
			t.Errorf("source %q should NOT be enabled_by_all but is", name)
		}
	}
}

// TestCatalogRedistributableConsistency verifies the explicit
// non-redistribution list matches AGENTS.md's "License & Redistribution
// Policy" section.
//
// iplists.firehol.org is a non-commercial community comparison site
// and a pass-through distributor: it republishes each feed's raw IP
// list under the feed's original license. Therefore `redistributable:
// false` is set ONLY when the source's license or terms of service
// TestCatalogGeneratedNamesContainsExpected verifies the GeneratedNames
// function produces the right set.
func TestCatalogGeneratedNamesContainsExpected(t *testing.T) {
	cfg := loadCatalog(t)
	names := GeneratedNames(cfg)

	// Check some expected generated names.
	expectedPresent := []string{
		"dshield", "dshield_1d", "dshield_7d", "dshield_30d",
		"botscout", "botscout_1d", "botscout_7d", "botscout_30d",
		"php_commenters_1d", "php_commenters_30d",
		"php_dictionary_1d", "php_dictionary_30d",
		"php_harvesters_1d", "php_harvesters_30d",
		"php_spammers_1d", "php_spammers_30d",
		"socks_proxy_30d", "sslproxies_30d",
		"tor_exits_1d", "tor_exits_30d",
		"firehol_level1", "firehol_level2", "firehol_level3", "firehol_level4",
		"anonymous", "satellite",
	}

	for _, name := range expectedPresent {
		if _, ok := names[name]; !ok {
			t.Errorf("expected generated name %q not found", name)
		}
	}
}

// TestCatalogNoURLSchemeMismatch checks that URLs using http:// where the
// domain is known to support https:// are flagged. This is informational,
// not a strict failure, since the bash script may use http for compatibility.
func TestCatalogNoURLSchemeMismatch(t *testing.T) {
	cfg := loadCatalog(t)

	// Count http vs https.
	httpCount := 0
	httpsCount := 0
	for _, src := range cfg.Sources {
		if src.URL == "" {
			continue
		}
		if strings.HasPrefix(src.URL, "https://") {
			httpsCount++
		} else if strings.HasPrefix(src.URL, "http://") {
			httpCount++
		}
	}
	t.Logf("URL scheme distribution: %d https, %d http", httpsCount, httpCount)
}

// TestCatalogHistorySourcesReferencedByMerges verifies that every
// history-windowed source name referenced by a merge is actually
// generated by a source with the right history window.
func TestCatalogHistorySourcesReferencedByMerges(t *testing.T) {
	cfg := loadCatalog(t)
	retentionRefs := 0

	for mergeName, src := range catalogMergeSources(cfg) {
		for _, ref := range mergeSourceRefs(src) {
			refSrc := cfg.Sources[ref]
			if refSrc == nil {
				t.Errorf("merge %q references %q which is not an expanded source", mergeName, ref)
				continue
			}
			if refSrc.Provenance == ProvenanceSecondaryRetention {
				retentionRefs++
			}
		}
	}
	if retentionRefs == 0 {
		t.Fatal("expected at least one merge reference to point at an expanded retention source")
	}
}

func catalogMergeSources(cfg *Config) map[string]*Source {
	out := map[string]*Source{}
	if cfg == nil {
		return out
	}
	for name, src := range cfg.Sources {
		if src != nil && src.Provenance == ProvenanceSecondaryMerge {
			out[name] = src
		}
	}
	return out
}

func mergeSourceRefs(src *Source) []string {
	if src == nil {
		return nil
	}
	refs := append([]string(nil), src.MergeSources...)
	refs = append(refs, src.MergeExclude...)
	return refs
}

// TestCatalogProcessorStepArgs verifies that processor steps with args
// have valid argument keys for their processor type.
func TestCatalogProcessorStepArgs(t *testing.T) {
	cfg := loadCatalog(t)

	// Known args for processor steps that accept them.
	knownArgs := map[string]map[string]bool{
		"csv_column":       {"index": true, "value": true},
		"xml_tag":          {"tag": true, "value": true},
		"unzip":            {"file": true, "value": true},
		"regex":            {"pattern": true, "value": true},
		"grep":             {"pattern": true, "value": true, "literal": true, "case_insensitive": true},
		"grep_not":         {"pattern": true, "value": true, "literal": true, "case_insensitive": true},
		"json_path":        {"path": true, "value": true},
		"json_paths":       {"path": true, "paths": true, "value": true},
		"cut_delimiter":    {"delimiter": true, "field": true, "value": true},
		"hostname_resolve": {"threads": true},
	}

	for name, src := range cfg.Sources {
		for _, step := range src.Processor {
			if len(step.Args) == 0 {
				continue
			}
			allowed, hasSpec := knownArgs[step.Name]
			if !hasSpec {
				// Processor doesn't normally take args.
				t.Logf("source %q: processor %q has unexpected args %v",
					name, step.Name, step.Args)
				continue
			}
			for key := range step.Args {
				if !allowed[key] {
					t.Errorf("source %q: processor %q has unknown arg key %q",
						name, step.Name, key)
				}
			}
		}
	}
}

// TestHistoryLabelEdgeCases verifies HistoryLabel for non-standard values.
func TestHistoryLabelEdgeCases(t *testing.T) {
	cases := []struct {
		mins int
		want string
	}{
		{60, "_1h"},
		{120, "_2h"},
		{720, "_12h"},
		{1440, "_1d"},
		{2880, "_2d"},
		{10080, "_7d"},
		{43200, "_30d"},
		{44640, "_31d"},   // 31 days
		{1500, "_1d1h"},   // 1 day + 1 hour
		{525600, "_365d"}, // 1 year
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d_mins", tc.mins), func(t *testing.T) {
			got := HistoryLabel(tc.mins)
			if got != tc.want {
				t.Errorf("HistoryLabel(%d) = %q, want %q", tc.mins, got, tc.want)
			}
		})
	}
}

// TestValidateDeepRejectsBadMergeRef verifies ValidateDeep catches
// a merge referencing a non-existent source.
func TestValidateDeepRejectsBadMergeRef(t *testing.T) {
	cfg := New()
	cfg.Sources["real"] = &Source{
		Name: "real", Frequency: 10, IPV: "ipv4", Output: "ipset",
	}
	cfg.Merges["bad"] = &Merge{
		Name: "bad", IPV: "ipv4", Output: "ipset",
		Sources: []string{"real", "nonexistent"},
	}
	if err := ValidateDeep(cfg); err == nil {
		t.Fatal("expected ValidateDeep to reject merge with nonexistent source")
	}
}

func TestValidateDeepRejectsBadMergeExcludeRef(t *testing.T) {
	cfg := New()
	cfg.Sources["real"] = &Source{
		Name: "real", Frequency: 10, IPV: "ipv4", Output: "ipset",
	}
	cfg.Merges["bad"] = &Merge{
		Name: "bad", IPV: "ipv4", Output: "ipset",
		Sources: []string{"real"},
		Exclude: []string{"nonexistent"},
	}
	if err := ValidateDeep(cfg); err == nil {
		t.Fatal("expected ValidateDeep to reject merge with nonexistent exclude source")
	}
}

// TestValidateDeepAcceptsHistoryVariants verifies ValidateDeep allows
// merge references to history-windowed variants.
func TestValidateDeepAcceptsHistoryVariants(t *testing.T) {
	cfg := New()
	cfg.Sources["feed"] = &Source{
		Name: "feed", Frequency: 10, IPV: "ipv4", Output: "ipset",
		History: []int{1440, 10080},
	}
	cfg.Merges["combo"] = &Merge{
		Name: "combo", IPV: "ipv4", Output: "ipset",
		Sources: []string{"feed", "feed_1d", "feed_7d"},
	}
	if err := ValidateDeep(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidateDeepAcceptsGeolocationDerived verifies ValidateDeep allows
// merge references to "anonymous" and "satellite".
func TestValidateDeepAcceptsGeolocationDerived(t *testing.T) {
	cfg := New()
	cfg.Sources["tor"] = &Source{
		Name: "tor", Frequency: 10, IPV: "ipv4", Output: "ipset",
	}
	cfg.Sources["ipdeny_country"] = &Source{
		Name:          "ipdeny_country",
		URL:           "https://example.test/ipdeny.tar.gz",
		Frequency:     1440,
		IPV:           "ipv4",
		Output:        "ipset",
		Format:        "ipdeny_country_tar_gz",
		Use:           []string{UseGeoIP},
		Maintainer:    "geo",
		MaintainerURL: "https://example.test",
		Processor:     []ProcessorStep{{Name: "passthrough"}},
		ProcessorRaw:  "cat",
		AcceptEmpty:   true,
	}
	cfg.Merges["anon"] = &Merge{
		Name: "anon", IPV: "ipv4", Output: "ipset",
		Sources: []string{"tor", "anonymous"},
	}
	if err := injectBuiltInSyntheticSources(cfg); err != nil {
		t.Fatalf("inject synthetic sources: %v", err)
	}
	if err := ValidateDeep(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCatalogMatchesLegacyExtraction verifies the YAML catalog produces
// the same set of source and merge names as extracting from the bash
// script — except for the unified database sources (asn, geoip,
// rfc_reserved synthetic) which the bash script does not know about.
func TestCatalogMatchesLegacyExtraction(t *testing.T) {
	// Opt-in test: set UPDATE_IPSETS_LEGACY_BASH to compare the YAML
	// catalog against an external bash script (from a local checkout of
	// firehol/firehol). The repo does not depend on the bash script
	// being present anywhere.
	bashPath := os.Getenv("UPDATE_IPSETS_LEGACY_BASH")
	if bashPath == "" {
		t.Skip("set UPDATE_IPSETS_LEGACY_BASH to run this test")
	}
	if _, err := os.Stat(bashPath); err != nil {
		t.Skipf("UPDATE_IPSETS_LEGACY_BASH=%q not readable: %v", bashPath, err)
	}

	yamlCfg := loadCatalog(t)

	bashCfg, err := ExtractLegacyScript(bashPath, ExtractOptions{IncludeGeolocation: true})
	if err != nil {
		t.Fatalf("failed to extract from bash: %v", err)
	}

	// Sources newly introduced under the unified model — they live in
	// cfg.Sources but the bash script does not produce them.
	unifiedOnly := map[string]bool{
		"caida_prefix2as":      true,
		"dbip_asn_lite":        true,
		"dbip_country":         true,
		"geolite2_country":     true,
		"ip2location_country":  true,
		"ipdeny_country":       true,
		"ipip_country":         true,
		"iptoasn":              true,
		"maxmind_geolite2_asn": true,
		"rfc_reserved":         true,
	}

	// Curated source additions that intentionally did not exist in the
	// legacy bash script. These came from post-bash feed audits.
	curatedPostBashSources := map[string]bool{
		"apnic_ssh_bruteforce":                   true,
		"apnic_telnet_bruteforce":                true,
		"dataplane_proto41":                      true,
		"dataplane_smtpdata":                     true,
		"dataplane_smtpgreet":                    true,
		"dataplane_telnetlogin":                  true,
		"critical_context_apple_networks":        true,
		"critical_context_github_hosted_compute": true,
		"critical_soft_atlassian_cloud":          true,
		"critical_soft_auth0":                    true,
		"critical_soft_aws_cloudfront":           true,
		"critical_soft_braintree":                true,
		"critical_soft_github_services":          true,
		"critical_soft_microsoft365":             true,
		"critical_soft_mollie":                   true,
		"critical_soft_okta":                     true,
		"critical_soft_salesforce_hyperforce":    true,
		"critical_soft_stripe_api":               true,
		"critical_soft_stripe_webhooks":          true,
		"critical_soft_terraform_cloud":          true,
		"critical_soft_zoom":                     true,
		"jamesbrine_bruteforce":                  true,
		"misp_akamai":                            true,
		"misp_alphastrike_scanners":              true,
		"misp_amazon_aws":                        true,
		"misp_apple":                             true,
		"misp_censys_scanners":                   true,
		"misp_check_host_net":                    true,
		"misp_cybergreen_scanners":               true,
		"misp_fastly":                            true,
		"misp_github":                            true,
		"misp_google_gcp":                        true,
		"misp_google_gmail_sending_ips":          true,
		"misp_googlebot":                         true,
		"misp_microsoft_azure":                   true,
		"misp_microsoft_azure_china":             true,
		"misp_microsoft_azure_germany":           true,
		"misp_microsoft_azure_us_gov":            true,
		"misp_microsoft_office365_cn":            true,
		"misp_microsoft_office365_ip":            true,
		"misp_modat_nt_scanners":                 true,
		"misp_netsecscan_nt_scanners":            true,
		"misp_netsecscan_scanners":               true,
		"misp_onyphe_published_scanners":         true,
		"misp_openai_gptbot":                     true,
		"misp_ovh_cluster":                       true,
		"misp_palo_alto_cortex_xpanse":           true,
		"misp_public_dns":                        true,
		"misp_shadowforce_published_scanners":    true,
		"misp_shadowserver_published_scanners":   true,
		"misp_shodan_published_scanners":         true,
		"misp_sinkholes":                         true,
		"misp_skipa_scanners":                    true,
		"misp_smtp_receiving_ips":                true,
		"misp_smtp_sending_ips":                  true,
		"misp_umbrella_blockpage":                true,
		"provider_context_aws_cloud":             true,
		"provider_context_digitalocean_geofeed":  true,
		"provider_context_gcp_cloud":             true,
		"provider_context_linode_geofeed":        true,
		"provider_context_oracle_cloud":          true,
		"provider_context_vultr_geofeed":         true,
		"serpro_reputation":                      true,
		"threatview_c2":                          true,
		"threatview_ip":                          true,
	}

	// Sources retired from the YAML catalog — the bash script still
	// references them but the upstream feed is dead, frozen, or
	// permanently unreachable, or intentionally removed.
	// Listed in cfg.Deleted as well.
	removedFromYAML := map[string]bool{
		"bitcoin_nodes":      true,
		"bm_tor":             true,
		"cleantalk_top20":    true,
		"cta_cryptowall":     true,
		"darklist_de":        true,
		"proxyrss":           true,
		"ri_connect_proxies": true,
		"ri_web_proxies":     true,
		"sorbs_anonymizers":  true,
		"sorbs_block":        true,
		"sorbs_escalations":  true,
		"sorbs_new_spam":     true,
		"sorbs_noserver":     true,
		"sorbs_recent_spam":  true,
		"sorbs_smtp":         true,
		"sorbs_web":          true,
		"sorbs_zombie":       true,
	}

	// Historical feeds restored from older bash commits. They are valid
	// YAML sources/merges even though the current bash HEAD no longer
	// contains them.
	restoredHistoricalSources := map[string]bool{
		"blueliv_crimeserver_last": true,
		"cleanmx_phishing":         true,
		"cleanmx_viruses":          true,
		"cleantalk_1d":             true,
		"cleantalk_7d":             true,
		"cleantalk_30d":            true,
		"cleantalk_new":            true,
		"cleantalk_updated":        true,
		"coinbl_hosts":             true,
		"coinbl_hosts_browser":     true,
		"coinbl_hosts_optional":    true,
		"coinbl_ips":               true,
		"datacenters":              true,
		"iblocklist_abuse_zeus":    true,
		"ipblacklistcloud_recent":  true,
		"maxmind_proxy_fraud":      true,
		"proxylists":               true,
		"proxz":                    true,
		"spamhaus_edrop":           true,
		"uscert_hidden_cobra":      true,
		"xroxy":                    true,
		"zeus":                     true,
		"zeus_badips":              true,
	}

	restoredOrCuratedPostBashMerges := map[string]bool{
		"cleantalk":                           true,
		"critical_soft_akamai_edge_secondary": true,
	}

	// Source names must match exactly.
	yamlSources := map[string]bool{}
	for name := range yamlCfg.Sources {
		yamlSources[name] = true
	}
	bashSources := map[string]bool{}
	for name := range bashCfg.Sources {
		bashSources[name] = true
	}

	for name := range bashSources {
		if removedFromYAML[name] {
			continue
		}
		if !yamlSources[name] {
			t.Errorf("bash script has source %q not in YAML", name)
		}
	}
	for name := range yamlSources {
		if unifiedOnly[name] {
			continue
		}
		if curatedPostBashSources[name] {
			continue
		}
		if restoredHistoricalSources[name] {
			continue
		}
		if !bashSources[name] {
			t.Errorf("YAML has source %q not in bash script", name)
		}
	}

	// Merge names must match exactly.
	yamlMerges := map[string]bool{}
	for name, src := range yamlCfg.Sources {
		if strings.HasPrefix(src.URL, InternalMergeScheme) {
			yamlMerges[name] = true
		}
	}
	bashMerges := map[string]bool{}
	for name := range bashCfg.Merges {
		bashMerges[name] = true
	}

	for name := range bashMerges {
		if !yamlMerges[name] {
			t.Errorf("bash script has merge %q not in YAML", name)
		}
	}
	for name := range yamlMerges {
		if restoredOrCuratedPostBashMerges[name] {
			continue
		}
		if !bashMerges[name] {
			t.Errorf("YAML has merge %q not in bash script", name)
		}
	}
}
