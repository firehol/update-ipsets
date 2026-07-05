package engine

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteJSONCompactMatchesStdlibForEntityPayloads(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{
			name: "public ASN detail",
			value: &ASNDetailPayload{
				ASN:         15169,
				Name:        "GOOGLE",
				Description: "example",
				Provider:    HomeSummaryProvider{Name: "iptoasn", Label: "IPtoASN"},
				GeoProvider: HomeSummaryProvider{Name: "dbip_country", Label: "DB-IP Country"},
				Totals: ASNDetailTotals{
					FeedsMatching: 2,
					AttributedIPs: 768,
					Categories:    1,
					Maintainers:   2,
					Countries:     2,
				},
				Feeds: []ASNDetailFeed{
					{Name: "alpha", Category: "intrusion", Provenance: "primary", Maintainer: "Alpha", AttributedIPs: 512, UniqueIPs: 512, HealthClass: "healthy", LastChangeTS: 1770000000},
					{Name: "beta", Category: "intrusion", Provenance: "secondary_upstream", AttributedIPs: 256, UniqueIPs: 256, HealthClass: "delayed"},
				},
				FeedsByCategory: map[string][]ASNDetailFeed{
					"intrusion": {
						{Name: "alpha", Category: "intrusion", Provenance: "primary", Maintainer: "Alpha", AttributedIPs: 512, UniqueIPs: 512, HealthClass: "healthy", LastChangeTS: 1770000000},
					},
					"malware": {
						{Name: "beta", Category: "malware", Provenance: "secondary_upstream", AttributedIPs: 256, UniqueIPs: 256, HealthClass: "delayed"},
					},
				},
				TopCategories:  []DetailCategorySummary{{Category: "intrusion", FeedCount: 2, AttributedIPs: 768}},
				TopMaintainers: []DetailMaintainerSummary{{Slug: "alpha", Name: "Alpha", URL: "https://example.test", FeedCount: 1, AttributedIPs: 512}},
				TopCountries:   []ASNDetailCountry{{Code: "DE", FeedCount: 1, AttributedIPs: 512}, {Code: "FR", FeedCount: 1, AttributedIPs: 256}},
				CountryDistribution: &CountryComparisonPayload{
					TotalMapped: 768,
					Countries:   []CountryValue{{Code: "DE", Value: 512}, {Code: "FR", Name: "France", Value: 256}},
				},
			},
		},
		{
			name: "private country sidecar",
			value: &countryDetailSidecar{
				Code:        "US",
				Provider:    HomeSummaryProvider{Name: "dbip_country"},
				ASNProvider: HomeSummaryProvider{},
				Totals: CountryDetailTotals{
					FeedsMatching:       1,
					AttributedIPsInFeed: 256,
					Categories:          1,
					Maintainers:         1,
					ASNs:                1,
				},
				Feeds: []countryDetailFeedBase{
					{Name: "alpha", Category: "intrusion", Provenance: "primary", AttributedIPs: 256, UniqueIPs: 256},
				},
			},
		},
		{
			name: "feed sidecar",
			value: &feedEntitySidecar{
				Feed:         "alpha",
				Category:     "intrusion",
				Provenance:   "primary",
				Maintainer:   "Alpha",
				UniqueIPs:    768,
				LastChangeTS: 1770000000,
				GeoProvider:  "dbip_country",
				ASNProvider:  "iptoasn",
				Countries: []feedEntityCountryContribution{
					{
						Code:          "DE",
						AttributedIPs: 512,
						ASNs:          []feedEntityJointASN{{ASN: 15169, Name: "GOOGLE", Count: 512}},
					},
				},
				ASNs: []feedEntityASNContribution{
					{ASN: 15169, Name: "GOOGLE", AttributedIPs: 512},
					{ASN: 13335, Name: "CLOUDFLARENET", AttributedIPs: 256},
				},
			},
		},
		{
			name: "ASN index",
			value: ASNIndexPayload{
				Provider: HomeSummaryProvider{Name: "iptoasn"},
				ASNs:     []ASNIndexEntry{{ASN: 13335, Name: "CLOUDFLARENET", FeedCount: 1, AttributedIPs: 256}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			want = append(want, '\n')

			var got bytes.Buffer
			if err := writeJSONCompact(&got, tc.value); err != nil {
				t.Fatalf("writeJSONCompact: %v", err)
			}
			if !bytes.Equal(got.Bytes(), want) {
				t.Fatalf("streamed JSON mismatch\n got: %s\nwant: %s", got.Bytes(), want)
			}
		})
	}
}

func TestWriteObservedJSONFileAtPreservesLogicalMTimeAndValidJSON(t *testing.T) {
	eng := newEngineFixture(t)
	path := filepath.Join(t.TempDir(), "asns", "index.json")
	mod := time.Date(2026, 7, 4, 12, 30, 0, 0, time.UTC)
	payload := ASNIndexPayload{
		Provider: HomeSummaryProvider{Name: "iptoasn"},
		ASNs:     []ASNIndexEntry{{ASN: 15169, FeedCount: 2, AttributedIPs: 768}},
	}

	if err := eng.writeObservedJSONFileAt(path, payload, mod, "test.entity_json_write"); err != nil {
		t.Fatalf("writeObservedJSONFileAt: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("JSON file should end with newline, got %q", data)
	}
	var got ASNIndexPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("written JSON is invalid: %v", err)
	}
	if got.Provider.Name != "iptoasn" || len(got.ASNs) != 1 || got.ASNs[0].ASN != 15169 {
		t.Fatalf("unexpected decoded payload: %+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(mod) {
		t.Fatalf("mtime = %s, want %s", info.ModTime(), mod)
	}
}

func TestWriteJSONCompactStreamsLargeEntityPayloadIncrementally(t *testing.T) {
	payload := ASNIndexPayload{
		Provider: HomeSummaryProvider{Name: "iptoasn"},
		ASNs:     make([]ASNIndexEntry, 40_000),
	}
	for i := range payload.ASNs {
		payload.ASNs[i] = ASNIndexEntry{
			ASN:           uint32(64_512 + i),
			FeedCount:     i%19 + 1,
			AttributedIPs: uint64(i+1) * 257,
		}
	}
	tracker := &chunkTrackingWriter{}
	if err := writeJSONCompact(tracker, payload); err != nil {
		t.Fatalf("writeJSONCompact: %v", err)
	}
	if tracker.total < 2*1024*1024 {
		t.Fatalf("test payload too small to prove streaming shape: total bytes = %d", tracker.total)
	}
	if tracker.maxChunk > 128*1024 {
		t.Fatalf("stream emitted a large contiguous chunk of %d bytes; want incremental writes", tracker.maxChunk)
	}
}

type chunkTrackingWriter struct {
	total    int64
	maxChunk int
}

func (w *chunkTrackingWriter) Write(p []byte) (int, error) {
	if len(p) > w.maxChunk {
		w.maxChunk = len(p)
	}
	w.total += int64(len(p))
	return len(p), nil
}

var _ io.Writer = (*chunkTrackingWriter)(nil)
