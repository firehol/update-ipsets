package jsonbench

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	veloxjson "github.com/velox-io/json"
)

type feedEntityJointASNCompat struct {
	ASN   uint32 `json:"asn"`
	Name  string `json:"name,omitempty"`
	Count uint64 `json:"count"`
}

type feedEntityCountryContributionCompat struct {
	Code          string                     `json:"code"`
	AttributedIPs uint64                     `json:"attributed_ips"`
	ASNs          []feedEntityJointASNCompat `json:"asns,omitempty"`
}

type feedEntityASNContributionCompat struct {
	ASN           uint32 `json:"asn"`
	Name          string `json:"name,omitempty"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

type feedEntitySidecarCurrentCompat struct {
	Feed         string                                `json:"feed"`
	Category     string                                `json:"category,omitempty"`
	Provenance   string                                `json:"provenance,omitempty"`
	Maintainer   string                                `json:"maintainer,omitempty"`
	UniqueIPs    uint64                                `json:"unique_ips"`
	LastChangeTS int64                                 `json:"last_change_ts,omitempty"`
	GeoProvider  string                                `json:"geo_provider,omitempty"`
	ASNProvider  string                                `json:"asn_provider,omitempty"`
	Countries    []feedEntityCountryContributionCompat `json:"countries,omitempty"`
	ASNs         []feedEntityASNContributionCompat     `json:"asns,omitempty"`
}

type feedEntitySidecarLegacyCompat struct {
	Feed         string   `json:"feed"`
	Category     string   `json:"category,omitempty"`
	Provenance   string   `json:"provenance,omitempty"`
	Maintainer   string   `json:"maintainer,omitempty"`
	UniqueIPs    uint64   `json:"unique_ips"`
	LastChangeTS int64    `json:"last_change_ts,omitempty"`
	GeoProvider  string   `json:"geo_provider,omitempty"`
	ASNProvider  string   `json:"asn_provider,omitempty"`
	Countries    []string `json:"countries,omitempty"`
	ASNs         []uint32 `json:"asns,omitempty"`
}

type rawSidecarCompat struct {
	Feed      string             `json:"feed"`
	Countries stdjson.RawMessage `json:"countries,omitempty"`
	ASNs      stdjson.RawMessage `json:"asns,omitempty"`
}

type homeSummaryProviderCompat struct {
	Name      string `json:"name,omitempty"`
	Coverage  string `json:"coverage,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

type detailCategorySummaryCompat struct {
	Category      string `json:"category"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

type detailMaintainerSummaryCompat struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

type asnDetailFeedCompat struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Provenance    string `json:"provenance,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	AttributedIPs uint64 `json:"attributed_ips"`
	UniqueIPs     uint64 `json:"unique_ips"`
	HealthClass   string `json:"health_class"`
	LastChangeTS  int64  `json:"last_change_ts,omitempty"`
}

type asnDetailTotalsCompat struct {
	FeedsMatching int    `json:"feeds_matching"`
	AttributedIPs uint64 `json:"attributed_ips"`
	Categories    int    `json:"categories"`
	Maintainers   int    `json:"maintainers"`
	Countries     int    `json:"countries"`
}

type asnDetailCountryCompat struct {
	Code          string `json:"code"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

type countryComparisonPayloadCompat struct {
	Country  string                    `json:"country"`
	Provider homeSummaryProviderCompat `json:"provider"`
	Feeds    map[string]uint64         `json:"feeds,omitempty"`
}

type asnDetailPayloadCompat struct {
	ASN                 uint32                           `json:"asn"`
	Name                string                           `json:"name,omitempty"`
	Description         string                           `json:"description,omitempty"`
	Provider            homeSummaryProviderCompat        `json:"provider"`
	GeoProvider         homeSummaryProviderCompat        `json:"geo_provider,omitempty"`
	Totals              asnDetailTotalsCompat            `json:"totals"`
	Feeds               []asnDetailFeedCompat            `json:"feeds"`
	FeedsByCategory     map[string][]asnDetailFeedCompat `json:"feeds_by_category,omitempty"`
	TopCategories       []detailCategorySummaryCompat    `json:"top_categories,omitempty"`
	TopMaintainers      []detailMaintainerSummaryCompat  `json:"top_maintainers,omitempty"`
	TopCountries        []asnDetailCountryCompat         `json:"top_countries,omitempty"`
	CountryDistribution *countryComparisonPayloadCompat  `json:"country_distribution,omitempty"`
}

type cacheStateCompat struct {
	SavedAt time.Time                   `json:"saved_at"`
	Entries map[string]cacheEntryCompat `json:"entries"`
}

type cacheEntryCompat struct {
	Name                 string   `json:"name"`
	File                 string   `json:"file,omitempty"`
	Source               string   `json:"source,omitempty"`
	URL                  string   `json:"url,omitempty"`
	PublicURL            string   `json:"public_url,omitempty"`
	IPV                  string   `json:"ipv,omitempty"`
	Hash                 string   `json:"hash,omitempty"`
	ContentHash          string   `json:"content_hash,omitempty"`
	FrequencyMinutes     int      `json:"frequency_minutes,omitempty"`
	HistoryMinutes       []int    `json:"history_minutes,omitempty"`
	Entries              int      `json:"entries,omitempty"`
	UniqueIPs            uint64   `json:"unique_ips,omitempty"`
	SourceDate           int64    `json:"source_date,omitempty"`
	ProcessedDate        int64    `json:"processed_date,omitempty"`
	CheckedDate          int64    `json:"checked_date,omitempty"`
	StartedDate          int64    `json:"started_date,omitempty"`
	Category             string   `json:"category,omitempty"`
	Info                 string   `json:"info,omitempty"`
	Maintainer           string   `json:"maintainer,omitempty"`
	MaintainerURL        string   `json:"maintainer_url,omitempty"`
	EntriesMin           int      `json:"entries_min,omitempty"`
	EntriesMax           int      `json:"entries_max,omitempty"`
	IPsMin               uint64   `json:"ips_min,omitempty"`
	IPsMax               uint64   `json:"ips_max,omitempty"`
	ClockSkewSeconds     int64    `json:"clock_skew_seconds,omitempty"`
	DownloadFailures     int      `json:"download_failures,omitempty"`
	FailureStartedDate   int64    `json:"failure_started_date,omitempty"`
	Version              int      `json:"version,omitempty"`
	AverageUpdateMins    int      `json:"average_update_mins,omitempty"`
	MinUpdateMins        int      `json:"min_update_mins,omitempty"`
	MaxUpdateMins        int      `json:"max_update_mins,omitempty"`
	HistoryTotalGapSecs  int64    `json:"history_total_gap_secs,omitempty"`
	HistoryMinGapSecs    int64    `json:"history_min_gap_secs,omitempty"`
	HistoryMaxGapSecs    int64    `json:"history_max_gap_secs,omitempty"`
	RotationMedianPct    float64  `json:"rotation_median_pct,omitempty"`
	RotationP75Pct       float64  `json:"rotation_p75_pct,omitempty"`
	RotationSamples      int      `json:"rotation_samples,omitempty"`
	ChangeRatioMedianPct float64  `json:"change_ratio_median_pct,omitempty"`
	ChangeRatioP75Pct    float64  `json:"change_ratio_p75_pct,omitempty"`
	ChangeRatioSamples   int      `json:"change_ratio_samples,omitempty"`
	Downloader           string   `json:"downloader,omitempty"`
	DownloaderOptions    string   `json:"downloader_options,omitempty"`
	License              string   `json:"license,omitempty"`
	Attribution          string   `json:"attribution,omitempty"`
	LastError            string   `json:"last_error,omitempty"`
	LastStatus           string   `json:"last_status,omitempty"`
	LastRunReason        string   `json:"last_run_reason,omitempty"`
	LastProcessingMS     int64    `json:"last_processing_ms,omitempty"`
	UniqueSharePct       float64  `json:"unique_share_pct,omitempty"`
	UniqueShareSamples   int      `json:"unique_share_samples,omitempty"`
	CriticalOverlapTiers []string `json:"critical_overlap_tiers,omitempty"`
}

type schedulerSnapshotCompat struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Items       []schedulerItemCompat `json:"items"`
}

type schedulerItemCompat struct {
	Name             string    `json:"name"`
	Kind             string    `json:"kind"`
	Hidden           bool      `json:"hidden,omitempty"`
	Enabled          bool      `json:"enabled"`
	HealthClass      string    `json:"health_class,omitempty"`
	FrequencyMinutes int       `json:"frequency_minutes"`
	Failures         int       `json:"failures"`
	CheckedAt        time.Time `json:"checked_at,omitempty"`
	NextDue          time.Time `json:"next_due,omitempty"`
	Detail           string    `json:"detail,omitempty"`
}

type jsonProjectPayloadCase struct {
	name     string
	value    any
	newValue func() any
}

func TestVeloxSafeStringMatchesStdlibOnProjectPayloads(t *testing.T) {
	for _, tc := range projectPayloadCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "cache_state" {
				t.Skip("velox marshal currently crashes on this payload; covered by TestVeloxCacheStateMarshalCrashReproducer")
			}

			stdBytes, err := stdjson.Marshal(tc.value)
			if err != nil {
				t.Fatalf("encoding/json marshal: %v", err)
			}
			veloxBytes, err := veloxjson.Marshal(tc.value, veloxjson.WithStdCompat())
			if err != nil {
				t.Fatalf("velox marshal: %v", err)
			}
			if !stdjson.Valid(veloxBytes) {
				t.Fatalf("velox output is not valid JSON: %q", veloxBytes)
			}

			stdFromStd := tc.newValue()
			if err := stdjson.Unmarshal(stdBytes, stdFromStd); err != nil {
				t.Fatalf("encoding/json decode stdlib output: %v", err)
			}
			stdFromVelox := tc.newValue()
			if err := stdjson.Unmarshal(veloxBytes, stdFromVelox); err != nil {
				t.Fatalf("encoding/json decode velox output: %v", err)
			}
			if !reflect.DeepEqual(stdFromStd, stdFromVelox) {
				t.Fatalf("velox marshal changed decoded value\nstdlib: %#v\nvelox: %#v", stdFromStd, stdFromVelox)
			}

			veloxFromStd := tc.newValue()
			if err := veloxjson.Unmarshal(stdBytes, veloxFromStd, veloxjson.WithCopyString()); err != nil {
				t.Fatalf("velox decode stdlib output: %v", err)
			}
			if !reflect.DeepEqual(stdFromStd, veloxFromStd) {
				t.Fatalf("velox unmarshal changed decoded value\nstdlib: %#v\nvelox: %#v", stdFromStd, veloxFromStd)
			}
		})
	}
}

func TestVeloxCacheStateMarshalCrashReproducer(t *testing.T) {
	if os.Getenv("UPDATE_IPSETS_JSONBENCH_VELOX_CRASH_CHILD") == "1" {
		if _, err := veloxjson.Marshal(cacheStateBenchmarkPayload(12), veloxjson.WithStdCompat()); err != nil {
			fmt.Fprintf(os.Stderr, "velox marshal returned error instead of crashing: %v\n", err)
			os.Exit(2)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestVeloxCacheStateMarshalCrashReproducer$", "-test.v")
	cmd.Env = append(os.Environ(), "UPDATE_IPSETS_JSONBENCH_VELOX_CRASH_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("velox cache-state marshal unexpectedly passed; remove the cache_state compatibility skip and re-enable velox cache-state benchmarks")
	}
	output := string(out)
	if !strings.Contains(output, "fatal error: fault") && !strings.Contains(output, "SIGSEGV") && !strings.Contains(output, "segmentation violation") {
		t.Fatalf("velox cache-state child failed without the expected crash signature: %v\n%s", err, output)
	}
	t.Logf("velox cache-state marshal crash reproduced in child process: %v", err)
}

func TestVeloxSafeStringRejectsInvalidProjectPayloadsLikeStdlib(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		newValue func() any
	}{
		{
			name:     "malformed object",
			data:     []byte(`{"feed":`),
			newValue: func() any { return &feedEntitySidecarCurrentCompat{} },
		},
		{
			name:     "wrong integer type",
			data:     []byte(`{"feed":"example","unique_ips":"not-a-number"}`),
			newValue: func() any { return &feedEntitySidecarCurrentCompat{} },
		},
		{
			name:     "invalid raw message value",
			data:     []byte(`{"feed":"example","countries":[}`),
			newValue: func() any { return &rawSidecarCompat{} },
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stdErr := stdjson.Unmarshal(tc.data, tc.newValue())
			veloxErr := veloxjson.Unmarshal(tc.data, tc.newValue(), veloxjson.WithCopyString())
			if (stdErr == nil) != (veloxErr == nil) {
				t.Fatalf("error mismatch: encoding/json=%v velox=%v", stdErr, veloxErr)
			}
			if stdErr == nil {
				t.Fatalf("test case is not invalid for encoding/json")
			}
		})
	}
}

func TestVeloxSafeStringDoesNotAliasInputForRetainedValues(t *testing.T) {
	data := []byte(`{"feed":"alpha","countries":[{"code":"GR","asns":[{"asn":64512,"name":"bravo","count":7}]}],"asns":[{"asn":64513,"name":"charl","attributed_ips":9}]}`)
	var got feedEntitySidecarCurrentCompat
	if err := veloxjson.Unmarshal(data, &got, veloxjson.WithCopyString()); err != nil {
		t.Fatalf("velox unmarshal: %v", err)
	}

	mutateJSONStrings(data, map[string]string{
		"alpha": "omega",
		"bravo": "delta",
		"charl": "echo!",
	})

	if got.Feed != "alpha" {
		t.Fatalf("feed changed after input mutation: %q", got.Feed)
	}
	if got.Countries[0].ASNs[0].Name != "bravo" {
		t.Fatalf("nested country ASN name changed after input mutation: %q", got.Countries[0].ASNs[0].Name)
	}
	if got.ASNs[0].Name != "charl" {
		t.Fatalf("ASN name changed after input mutation: %q", got.ASNs[0].Name)
	}
}

func TestVeloxSafeStringDoesNotAliasInputForRawMessage(t *testing.T) {
	data := []byte(`{"feed":"alpha","countries":[{"code":"GR","name":"bravo"}],"asns":[64512]}`)
	var got rawSidecarCompat
	if err := veloxjson.Unmarshal(data, &got, veloxjson.WithCopyString()); err != nil {
		t.Fatalf("velox unmarshal: %v", err)
	}

	mutateJSONStrings(data, map[string]string{
		"alpha": "omega",
		"bravo": "delta",
	})

	if got.Feed != "alpha" {
		t.Fatalf("feed changed after input mutation: %q", got.Feed)
	}
	if strings.Contains(string(got.Countries), "delta") {
		t.Fatalf("raw message changed after input mutation: %s", got.Countries)
	}
	if !strings.Contains(string(got.Countries), "bravo") {
		t.Fatalf("raw message lost original value: %s", got.Countries)
	}
}

func BenchmarkProjectPayloadJSONMarshal(b *testing.B) {
	for _, payload := range projectPayloadBenchmarkCases() {
		payload := payload
		for _, codec := range comparisonPairLedgerJSONCodecs {
			codec := codec
			b.Run(payload.name+"/"+codec.name, func(b *testing.B) {
				if strings.HasPrefix(codec.name, "velox_") && strings.HasPrefix(payload.name, "cache_state_") {
					b.Skip("velox marshal currently crashes on cache-state-shaped payloads")
				}
				b.ReportAllocs()
				for b.Loop() {
					data, err := codec.marshal(payload.value)
					if err != nil {
						b.Fatal(err)
					}
					jsonBenchBytes = data
				}
			})
		}
	}
}

func BenchmarkProjectPayloadJSONUnmarshal(b *testing.B) {
	for _, payload := range projectPayloadBenchmarkCases() {
		payload := payload
		data, err := stdjson.Marshal(payload.value)
		if err != nil {
			b.Fatal(err)
		}
		for _, codec := range comparisonPairLedgerJSONCodecs {
			codec := codec
			b.Run(payload.name+"/"+codec.name, func(b *testing.B) {
				if strings.HasPrefix(codec.name, "velox_") && strings.HasPrefix(payload.name, "cache_state_") {
					b.Skip("velox cache-state marshal crash blocks this candidate for cache-state use")
				}
				b.ReportAllocs()
				for b.Loop() {
					got := payload.newValue()
					if err := codec.unmarshal(data, got); err != nil {
						b.Fatal(err)
					}
					jsonBenchAny = got
				}
			})
		}
	}
}

func projectPayloadCases() []jsonProjectPayloadCase {
	return []jsonProjectPayloadCase{
		{
			name:     "comparison_pair_ledger_v1",
			value:    comparisonPairLedgerBenchmarkPayload(8),
			newValue: func() any { return &comparisonPairLedgerFile{} },
		},
		{
			name:     "feed_entity_sidecar_current",
			value:    feedEntitySidecarCurrentBenchmarkPayload(8),
			newValue: func() any { return &feedEntitySidecarCurrentCompat{} },
		},
		{
			name:     "feed_entity_sidecar_legacy",
			value:    feedEntitySidecarLegacyBenchmarkPayload(),
			newValue: func() any { return &feedEntitySidecarLegacyCompat{} },
		},
		{
			name:     "feed_entity_sidecar_raw",
			value:    rawSidecarBenchmarkPayload(),
			newValue: func() any { return &rawSidecarCompat{} },
		},
		{
			name:     "asn_detail_public",
			value:    asnDetailBenchmarkPayload(12),
			newValue: func() any { return &asnDetailPayloadCompat{} },
		},
		{
			name:     "cache_state",
			value:    cacheStateBenchmarkPayload(12),
			newValue: func() any { return &cacheStateCompat{} },
		},
		{
			name:     "scheduler_snapshot",
			value:    schedulerSnapshotBenchmarkPayload(12),
			newValue: func() any { return &schedulerSnapshotCompat{} },
		},
	}
}

func projectPayloadBenchmarkCases() []jsonProjectPayloadCase {
	return []jsonProjectPayloadCase{
		{
			name:     "feed_entity_sidecar_current_250",
			value:    feedEntitySidecarCurrentBenchmarkPayload(250),
			newValue: func() any { return &feedEntitySidecarCurrentCompat{} },
		},
		{
			name:     "asn_detail_public_1000",
			value:    asnDetailBenchmarkPayload(1000),
			newValue: func() any { return &asnDetailPayloadCompat{} },
		},
		{
			name:     "cache_state_1000",
			value:    cacheStateBenchmarkPayload(1000),
			newValue: func() any { return &cacheStateCompat{} },
		},
		{
			name:     "scheduler_snapshot_1000",
			value:    schedulerSnapshotBenchmarkPayload(1000),
			newValue: func() any { return &schedulerSnapshotCompat{} },
		},
	}
}

func feedEntitySidecarCurrentBenchmarkPayload(rows int) feedEntitySidecarCurrentCompat {
	countries := make([]feedEntityCountryContributionCompat, 0, rows)
	asns := make([]feedEntityASNContributionCompat, 0, rows)
	for i := 0; i < rows; i++ {
		asn := uint32(64512 + i)
		countries = append(countries, feedEntityCountryContributionCompat{
			Code:          fmt.Sprintf("C%02d", i%99),
			AttributedIPs: uint64(1000 + i),
			ASNs: []feedEntityJointASNCompat{{
				ASN:   asn,
				Name:  fmt.Sprintf("AS%d <edge>&line\u2028", asn),
				Count: uint64(50 + i),
			}},
		})
		asns = append(asns, feedEntityASNContributionCompat{
			ASN:           asn,
			Name:          fmt.Sprintf("AS%d <edge>&line\u2028", asn),
			AttributedIPs: uint64(1000 + i),
		})
	}
	return feedEntitySidecarCurrentCompat{
		Feed:         "example_feed",
		Category:     "attacks",
		Provenance:   "community",
		Maintainer:   "Example Maintainer",
		UniqueIPs:    uint64(rows * 1000),
		LastChangeTS: 1_785_000_000,
		GeoProvider:  "geo-provider",
		ASNProvider:  "asn-provider",
		Countries:    countries,
		ASNs:         asns,
	}
}

func feedEntitySidecarLegacyBenchmarkPayload() feedEntitySidecarLegacyCompat {
	return feedEntitySidecarLegacyCompat{
		Feed:         "legacy_feed",
		Category:     "reputation",
		Provenance:   "community",
		Maintainer:   "Legacy Maintainer",
		UniqueIPs:    12345,
		LastChangeTS: 1_700_000_000,
		GeoProvider:  "geo-provider",
		ASNProvider:  "asn-provider",
		Countries:    []string{"US", "GR", "DE"},
		ASNs:         []uint32{64512, 64513, 64514},
	}
}

func rawSidecarBenchmarkPayload() rawSidecarCompat {
	return rawSidecarCompat{
		Feed:      "raw_feed",
		Countries: stdjson.RawMessage(`[{"code":"GR","attributed_ips":10},{"code":"US","attributed_ips":20}]`),
		ASNs:      stdjson.RawMessage(`[64512,64513]`),
	}
}

func asnDetailBenchmarkPayload(rows int) asnDetailPayloadCompat {
	feeds := make([]asnDetailFeedCompat, 0, rows)
	byCategory := make(map[string][]asnDetailFeedCompat, 8)
	for i := 0; i < rows; i++ {
		category := fmt.Sprintf("category-%02d", i%8)
		feed := asnDetailFeedCompat{
			Name:          fmt.Sprintf("feed_%04d", i),
			Category:      category,
			Provenance:    "community",
			Maintainer:    fmt.Sprintf("maintainer-%02d", i%16),
			AttributedIPs: uint64(100 + i),
			UniqueIPs:     uint64(1000 + i),
			HealthClass:   "healthy",
			LastChangeTS:  1_785_000_000 + int64(i),
		}
		feeds = append(feeds, feed)
		byCategory[category] = append(byCategory[category], feed)
	}
	return asnDetailPayloadCompat{
		ASN:         64512,
		Name:        "AS64512 Example <Network>&",
		Description: "Example ASN payload with project-shaped nested data.",
		Provider: homeSummaryProviderCompat{
			Name:      "asn-provider",
			Coverage:  "full",
			UpdatedAt: 1_785_000_000,
		},
		GeoProvider: homeSummaryProviderCompat{
			Name:      "geo-provider",
			Coverage:  "full",
			UpdatedAt: 1_785_000_000,
		},
		Totals: asnDetailTotalsCompat{
			FeedsMatching: rows,
			AttributedIPs: uint64(rows * 100),
			Categories:    8,
			Maintainers:   16,
			Countries:     4,
		},
		Feeds:           feeds,
		FeedsByCategory: byCategory,
		TopCategories: []detailCategorySummaryCompat{{
			Category:      "category-00",
			FeedCount:     rows / 8,
			AttributedIPs: uint64(rows * 10),
		}},
		TopMaintainers: []detailMaintainerSummaryCompat{{
			Slug:          "maintainer-00",
			Name:          "Maintainer 00",
			URL:           "https://example.invalid/maintainer",
			FeedCount:     rows / 16,
			AttributedIPs: uint64(rows * 5),
		}},
		TopCountries: []asnDetailCountryCompat{{
			Code:          "GR",
			FeedCount:     rows / 4,
			AttributedIPs: uint64(rows * 12),
		}},
		CountryDistribution: &countryComparisonPayloadCompat{
			Country: "GR",
			Provider: homeSummaryProviderCompat{
				Name:      "geo-provider",
				Coverage:  "full",
				UpdatedAt: 1_785_000_000,
			},
			Feeds: map[string]uint64{"feed_0000": 100, "feed_0001": 90},
		},
	}
}

func cacheStateBenchmarkPayload(rows int) cacheStateCompat {
	entries := make(map[string]cacheEntryCompat, rows)
	for i := 0; i < rows; i++ {
		name := fmt.Sprintf("feed_%04d", i)
		entries[name] = cacheEntryCompat{
			Name:                 name,
			File:                 name + ".ipset",
			Source:               name,
			URL:                  "https://example.invalid/" + name,
			PublicURL:            "https://iplists.example.invalid/files/" + name + ".ipset",
			IPV:                  "ipset",
			Hash:                 fmt.Sprintf("%064d", i),
			ContentHash:          fmt.Sprintf("%064x", i),
			FrequencyMinutes:     60,
			HistoryMinutes:       []int{60, 120, 180},
			Entries:              1000 + i,
			UniqueIPs:            uint64(900 + i),
			SourceDate:           1_785_000_000 + int64(i),
			ProcessedDate:        1_785_000_100 + int64(i),
			CheckedDate:          1_785_000_200 + int64(i),
			StartedDate:          1_785_000_050 + int64(i),
			Category:             "attacks",
			Info:                 "Example cache entry <info>&line\u2028",
			Maintainer:           "Example Maintainer",
			MaintainerURL:        "https://example.invalid",
			EntriesMin:           1,
			EntriesMax:           1_000_000,
			IPsMin:               1,
			IPsMax:               1_000_000,
			ClockSkewSeconds:     int64(i % 5),
			DownloadFailures:     i % 3,
			FailureStartedDate:   0,
			Version:              1 + i,
			AverageUpdateMins:    60,
			MinUpdateMins:        30,
			MaxUpdateMins:        90,
			HistoryTotalGapSecs:  int64(i),
			HistoryMinGapSecs:    int64(i % 7),
			HistoryMaxGapSecs:    int64(i % 11),
			RotationMedianPct:    3.5,
			RotationP75Pct:       7.5,
			RotationSamples:      30,
			ChangeRatioMedianPct: 1.5,
			ChangeRatioP75Pct:    4.5,
			ChangeRatioSamples:   30,
			Downloader:           "http",
			DownloaderOptions:    "default",
			License:              "public feed",
			Attribution:          "Example",
			LastStatus:           "success",
			LastRunReason:        "scheduled",
			LastProcessingMS:     int64(100 + i),
			UniqueSharePct:       25.5,
			UniqueShareSamples:   10,
			CriticalOverlapTiers: []string{"hard", "soft"},
		}
	}
	return cacheStateCompat{
		SavedAt: time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
		Entries: entries,
	}
}

func schedulerSnapshotBenchmarkPayload(rows int) schedulerSnapshotCompat {
	items := make([]schedulerItemCompat, 0, rows)
	base := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	for i := 0; i < rows; i++ {
		items = append(items, schedulerItemCompat{
			Name:             fmt.Sprintf("feed_%04d", i),
			Kind:             "source",
			Hidden:           i%10 == 0,
			Enabled:          i%7 != 0,
			HealthClass:      "healthy",
			FrequencyMinutes: 60,
			Failures:         i % 3,
			CheckedAt:        base.Add(time.Duration(i) * time.Minute),
			NextDue:          base.Add(time.Duration(i+60) * time.Minute),
			Detail:           "scheduled",
		})
	}
	return schedulerSnapshotCompat{
		GeneratedAt: base,
		Items:       items,
	}
}

func mutateJSONStrings(data []byte, replacements map[string]string) {
	for old, next := range replacements {
		if len(old) != len(next) {
			panic("replacement strings must have equal lengths")
		}
		oldBytes := []byte(old)
		nextBytes := []byte(next)
		for {
			idx := bytes.Index(data, oldBytes)
			if idx < 0 {
				break
			}
			copy(data[idx:idx+len(nextBytes)], nextBytes)
		}
	}
}

var jsonBenchAny any
