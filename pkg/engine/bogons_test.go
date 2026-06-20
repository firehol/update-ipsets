package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func newTestConfigForFanOut() *config.Config {
	cfg := config.New()
	// A bogons-marked source whose name is "bogons" — this stands in
	// for the real Cymru bogons feed referenced by the targetFeedsForFanOut
	// underlying-feed test.
	cfg.Sources["bogons"] = &config.Source{
		Name:      "bogons",
		IPV:       "ipv4",
		Output:    "net",
		Frequency: 1440,
		Use:       []string{config.UseBogons},
	}
	cfg.Sources["maxmind_geolite2"] = &config.Source{
		Name:      "maxmind_geolite2",
		Frequency: 10080,
		Use:       []string{config.UseGeoIP},
		Format:    "maxmind_country_csv",
	}
	cfg.Sources["maxmind_asn"] = &config.Source{
		Name:      "maxmind_asn",
		Frequency: 10080,
		Use:       []string{config.UseASN},
		Format:    "maxmind_asn_mmdb_tar_gz",
	}
	cfg.Sources["rfc_reserved"] = &config.Source{
		Name:      "rfc_reserved",
		IPV:       "ipv4",
		Output:    "net",
		Frequency: 0,
		Use:       []string{config.UseBogons},
		Format:    "rfc_reserved_baseline",
		Hidden:    true,
	}
	return cfg
}

func TestBogonProvidersIncludesMergeDerivedProvider(t *testing.T) {
	cfg := config.New()
	cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:       "cymru_unassigned",
		Label:      "Team Cymru unassigned",
		Use:        []string{config.UseBogons},
		Provenance: config.ProvenanceSecondaryMerge,
	}
	eng := newEngineFixture(t, withConfig(cfg))

	providers := eng.BogonProviders()
	if len(providers) != 1 {
		t.Fatalf("providers = %+v, want one merge-derived provider", providers)
	}
	if providers[0].Name != "cymru_unassigned" || providers[0].Label != "Team Cymru unassigned" {
		t.Fatalf("provider = %+v, want cymru_unassigned with label", providers[0])
	}
}

// TestBuildASNFeedJSONThreeBucketInvariant verifies that the three
// buckets attributed_ips + bogon_ips + unknown_ips always equal
// feed_ips, regardless of which inputs are zero. This is the core
// invariant the bogon split exists to maintain.
func TestBuildASNFeedJSONThreeBucketInvariant(t *testing.T) {
	cases := []struct {
		name   string
		counts map[uint32]uint64 // ASN -> count, ASN 0 means unknown
		bogon  uint64
	}{
		{
			name:   "all attributed",
			counts: map[uint32]uint64{16509: 100, 13335: 200},
			bogon:  0,
		},
		{
			name:   "attributed plus unknown",
			counts: map[uint32]uint64{16509: 100, 0: 50},
			bogon:  0,
		},
		{
			name:   "attributed plus unknown plus bogon",
			counts: map[uint32]uint64{16509: 100, 0: 50},
			bogon:  25,
		},
		{
			name:   "only bogon",
			counts: map[uint32]uint64{},
			bogon:  77,
		},
		{
			name:   "only unknown",
			counts: map[uint32]uint64{0: 42},
			bogon:  0,
		},
		{
			name:   "empty",
			counts: map[uint32]uint64{},
			bogon:  0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := buildASNFeedJSON("test", tc.counts, map[uint32]string{}, tc.bogon)
			sum := payload.AttributedIPs + payload.BogonIPs + payload.UnknownIPs
			if sum != payload.FeedIPs {
				t.Fatalf("invariant broken: attributed(%d) + bogon(%d) + unknown(%d) = %d, want feed_ips=%d",
					payload.AttributedIPs, payload.BogonIPs, payload.UnknownIPs, sum, payload.FeedIPs)
			}
			// ASN 0 must NEVER appear in by_asn — unknown is reported via UnknownIPs.
			for _, row := range payload.ByASN {
				if row.ASN == 0 {
					t.Fatalf("ASN 0 must not appear in by_asn rows")
				}
			}
		})
	}
}

// TestBuildBogonUnionEmpty verifies that buildBogonUnion returns nil
// for an empty dataset, so callers can use the nil check as the
// "no bogon split" signal.
func TestBuildBogonUnionEmpty(t *testing.T) {
	union, err := buildBogonUnion(t.Context(), nil)
	if err != nil {
		t.Fatalf("nil dataset: %v", err)
	}
	if union != nil {
		t.Fatalf("nil dataset must return nil union, got %v", union)
	}
	empty := &bogonDatasets{Providers: map[string]*bogonProviderSet{}}
	union, err = buildBogonUnion(t.Context(), empty)
	if err != nil {
		t.Fatalf("empty dataset: %v", err)
	}
	if union != nil {
		t.Fatalf("empty dataset must return nil union, got %v", union)
	}
}

// TestBuildBogonUnionMerges verifies that buildBogonUnion correctly
// unions overlapping provider sets without double-counting and
// preserves the sum of unique IPs.
func TestBuildBogonUnionMerges(t *testing.T) {
	a := iprange.New("a")
	if err := a.AddRange(iprange.Range{Lo: 100, Hi: 200}); err != nil {
		t.Fatalf("add range a: %v", err)
	}
	b := iprange.New("b")
	if err := b.AddRange(iprange.Range{Lo: 150, Hi: 300}); err != nil {
		t.Fatalf("add range b: %v", err)
	}
	ds := &bogonDatasets{
		Providers: map[string]*bogonProviderSet{
			"a": {Name: "a", Format: "ipset", Set: a},
			"b": {Name: "b", Format: "ipset", Set: b},
		},
		Names: []string{"a", "b"},
	}
	union, err := buildBogonUnion(t.Context(), ds)
	if err != nil {
		t.Fatalf("buildBogonUnion: %v", err)
	}
	if union == nil {
		t.Fatalf("expected non-nil union")
	}
	got := union.UniqueCount()
	want := uint64(201) // 100..300 inclusive
	if got != want {
		t.Fatalf("union unique count = %d, want %d", got, want)
	}
}

func TestWriteASNComparisonFilesReusesPrecomputedBogonSplit(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["iptoasn"] = &config.Source{Name: "iptoasn", Frequency: 60, Use: []string{config.UseASN}, Format: "iptoasn_combined_tsv"}
	cfg.Sources["caida"] = &config.Source{Name: "caida", Frequency: 60, Use: []string{config.UseASN}, Format: "caida_prefix2as"}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
		rt.LibDir = filepath.Join(root, "lib")
		rt.MaxHeavyPhaseWorkers = 1
	}))
	if err := os.WriteFile(filepath.Join(baseDir, "sample.ipset"), []byte("192.0.2.0/24\n203.0.113.0/30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.File = "sample.ipset"
	entry.Version = 1

	iptoasnPath := filepath.Join(root, "iptoasn.tsv")
	if err := os.WriteFile(iptoasnPath, []byte("192.0.2.0\t192.0.2.127\t64500\tZZ\tEXAMPLE-A\n192.0.2.128\t192.0.2.255\t64501\tZZ\tEXAMPLE-B\n203.0.113.0\t203.0.113.1\t64502\tZZ\tEXAMPLE-C\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	caidaPath := filepath.Join(root, "caida.pfx2as")
	if err := os.WriteFile(caidaPath, []byte("192.0.2.0\t25\t64500\n192.0.2.128\t25\t64501\n203.0.113.0\t31\t64502\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	iptoasnDB, err := asnloc.Open("iptoasn_combined_tsv", iptoasnPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = iptoasnDB.Close() }()
	caidaDB, err := asnloc.Open("caida_prefix2as", caidaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = caidaDB.Close() }()

	bogons := iprange.New("bogons")
	if err := bogons.AddRange(iprange.Range{Lo: 0xC0000280, Hi: 0xC00002FF}); err != nil { // 192.0.2.128-255
		t.Fatal(err)
	}
	if err := bogons.AddRange(iprange.Range{Lo: 0xCB007102, Hi: 0xCB007103}); err != nil { // 203.0.113.2-3
		t.Fatal(err)
	}
	bogons.Optimize()

	if err := eng.writeASNComparisonFiles(t.Context(), asnDatasets{"iptoasn": iptoasnDB, "caida": caidaDB}, bogons, []string{"sample"}, webDir, nil); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"iptoasn", "caida"} {
		data, err := os.ReadFile(filepath.Join(webDir, "sample_asn_"+provider+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var payload asnFeedJSON
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.BogonIPs != 130 || payload.UnknownIPs != 0 || payload.AttributedIPs != 130 || payload.FeedIPs != 260 {
			t.Fatalf("%s payload = %+v, want bogon=130 unknown=0 attributed=130 feed=260", provider, payload)
		}
	}
}

func TestCountASNFeedWithBogonSplitFallsBackWhenSplitMissing(t *testing.T) {
	root := t.TempDir()
	iptoasnPath := filepath.Join(root, "iptoasn.tsv")
	if err := os.WriteFile(iptoasnPath, []byte("192.0.2.0\t192.0.2.127\t64500\tZZ\tEXAMPLE-A\n192.0.2.128\t192.0.2.255\t64501\tZZ\tEXAMPLE-B\n203.0.113.0\t203.0.113.1\t64502\tZZ\tEXAMPLE-C\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := asnloc.Open("iptoasn_combined_tsv", iptoasnPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	feed := iprange.New("sample")
	if err := feed.AddRange(iprange.Range{Lo: 0xC0000200, Hi: 0xC00002FF}); err != nil { // 192.0.2.0/24
		t.Fatal(err)
	}
	if err := feed.AddRange(iprange.Range{Lo: 0xCB007100, Hi: 0xCB007103}); err != nil { // 203.0.113.0/30
		t.Fatal(err)
	}
	feed.Optimize()

	bogons := iprange.New("bogons")
	if err := bogons.AddRange(iprange.Range{Lo: 0xC0000280, Hi: 0xC00002FF}); err != nil { // 192.0.2.128-255
		t.Fatal(err)
	}
	if err := bogons.AddRange(iprange.Range{Lo: 0xCB007102, Hi: 0xCB007103}); err != nil { // 203.0.113.2-3
		t.Fatal(err)
	}
	bogons.Optimize()

	counts, _, bogonIPs, err := countASNFeedWithBogonSplit(db, feed, bogons, map[string]uint64{}, "sample")
	if err != nil {
		t.Fatal(err)
	}
	if bogonIPs != 130 || counts[64500] != 128 || counts[64502] != 2 {
		t.Fatalf("fallback counts = %#v bogonIPs=%d, want attributed residual and 130 bogons", counts, bogonIPs)
	}
}

func TestPrecomputeASNBogonSplitsRecordsDisjointZero(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.LibDir = filepath.Join(root, "lib")
	}))
	if err := os.WriteFile(filepath.Join(baseDir, "sample.ipset"), []byte("198.51.100.0/24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.File = "sample.ipset"
	entry.Version = 1

	bogons := iprange.New("bogons")
	if err := bogons.AddRange(iprange.Range{Lo: 0x0A000000, Hi: 0x0AFFFFFF}); err != nil { // 10.0.0.0/8
		t.Fatal(err)
	}
	bogons.Optimize()

	setCache := newLatestSetCache(eng)
	defer setCache.CloseAll(eng.logger)
	splits := eng.precomputeASNBogonSplits(t.Context(), []string{"sample"}, asnDatasets{"first": nil, "second": nil}, bogons, setCache)
	got, ok := splits["sample"]
	if !ok {
		t.Fatal("expected disjoint feed to have an explicit precomputed split")
	}
	if got != 0 {
		t.Fatalf("disjoint split = %d, want 0", got)
	}
}

func TestWriteBogonComparisonFilesIncludesMergeDerivedProvider(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:       "cymru_unassigned",
		Frequency:  60,
		IPV:        "ipv4",
		Output:     "netset",
		Use:        []string{config.UseBogons},
		Provenance: config.ProvenanceSecondaryMerge,
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
	}))
	if err := os.WriteFile(filepath.Join(baseDir, "sample.ipset"), []byte("10.0.0.1\n10.0.0.2\n192.0.2.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng.state.Entry("sample").Name = "sample"
	eng.state.Entry("sample").File = "sample.ipset"
	eng.state.Entry("sample").Version = 1
	providerSet := iprange.New("cymru_unassigned")
	if err := providerSet.AddRange(iprange.Range{Lo: 0x0A000001, Hi: 0x0A000002}); err != nil {
		t.Fatal(err)
	}
	providerSet.Optimize()
	datasets := &bogonDatasets{
		Providers: map[string]*bogonProviderSet{
			"cymru_unassigned": {Name: "cymru_unassigned", Set: providerSet},
		},
		Names: []string{"cymru_unassigned"},
	}

	if err := eng.writeBogonComparisonFiles(t.Context(), datasets, []string{"cymru_unassigned"}, webDir, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(webDir, "sample_bogons_cymru_unassigned.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"provider": "cymru_unassigned"`) || !strings.Contains(text, `"bogon_ips": 2`) {
		t.Fatalf("unexpected bogon comparison payload: %s", text)
	}
}

// TestRFCReservedBaselineParses verifies that every hardcoded RFC
// reserved entry parses correctly and the resulting set has a stable
// total IP count. The exact total is documented here so any future
// edit to rfcReservedBogons forces an explicit update of the test.
func TestRFCReservedBaselineParses(t *testing.T) {
	// Reset cache to ensure parsing happens fresh in this test.
	rfcReservedRangesCache = nil
	rfcReservedSetCache = nil
	defer func() {
		rfcReservedRangesCache = nil
		rfcReservedSetCache = nil
	}()
	set, err := buildRFCReservedSet()
	if err != nil {
		t.Fatalf("buildRFCReservedSet: %v", err)
	}
	if set == nil {
		t.Fatal("nil set")
	}
	got := set.UniqueCount()
	// Sum of the 15 reserved ranges, computed by hand and verified
	// against the optimized set's UniqueCount(). Any future edit to
	// rfcReservedBogons must update this constant explicitly.
	//
	//   0.0.0.0/8        16777216
	//   10.0.0.0/8       16777216
	//   100.64.0.0/10     4194304
	//   127.0.0.0/8      16777216
	//   169.254.0.0/16      65536
	//   172.16.0.0/12     1048576
	//   192.0.0.0/24          256
	//   192.0.2.0/24          256
	//   192.88.99.0/24        256
	//   192.168.0.0/16      65536
	//   198.18.0.0/15      131072
	//   198.51.100.0/24       256
	//   203.0.113.0/24        256
	//   224.0.0.0/4     268435456
	//   240.0.0.0/4     268435456
	//   total           592708864
	const want uint64 = 592708864
	if got != want {
		t.Fatalf("rfc reserved unique count = %d, want %d", got, want)
	}
}

// TestRFCReservedBytesRegisteredWithDownloader verifies the package
// init wired the synthetic source into the downloader registry. This
// is the contract every internal:// source relies on.
func TestRFCReservedBytesRegisteredWithDownloader(t *testing.T) {
	provider, ok := downloaderLookupInternal(RFCReservedSourceName)
	if !ok {
		t.Fatalf("rfc_reserved provider not registered")
	}
	body, err := provider("")
	if err != nil {
		t.Fatalf("provider error: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("provider returned empty body")
	}
	// The bytes should contain at least one CIDR token from the
	// hardcoded baseline so callers can verify they got the right
	// content.
	if got := string(body); !contains(got, "10.0.0.0/8") || !contains(got, "127.0.0.0/8") {
		t.Fatalf("provider body missing expected RFC entries: %s", got)
	}
}

// downloaderLookupInternal forwards to the real registry. Captured as
// a var so test mocks can swap it if ever needed.
var downloaderLookupInternal = func(name string) (downloader.InternalProvider, bool) {
	return downloader.LookupInternal(name)
}

// contains is a tiny strings.Contains alias to keep the import list of
// this test file minimal.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestComputeRFCByRangeBreakdown verifies the per-range breakdown is
// computed correctly and only non-zero entries are returned.
func TestComputeRFCByRangeBreakdown(t *testing.T) {
	// Build a tiny feed with two known IPs in different RFC ranges:
	//   10.0.0.5      -> RFC 1918 private (10/8)
	//   192.168.1.1   -> RFC 1918 private (192.168/16)
	feed := iprange.New("feed")
	if err := feed.AddRange(iprange.Range{Lo: 0x0a000005, Hi: 0x0a000005}); err != nil {
		t.Fatal(err)
	}
	if err := feed.AddRange(iprange.Range{Lo: 0xc0a80101, Hi: 0xc0a80101}); err != nil {
		t.Fatal(err)
	}
	rfcReservedRangesCache = nil
	defer func() { rfcReservedRangesCache = nil }()
	ranges, err := getRFCReservedRanges()
	if err != nil {
		t.Fatal(err)
	}
	out, err := computeRFCByRangeBreakdown(t.Context(), feed, ranges)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 non-zero range entries, got %d", len(out))
	}
	for _, row := range out {
		if row.Count != 1 {
			t.Errorf("range %s: expected count 1, got %d", row.CIDR, row.Count)
		}
	}
}

// TestTargetFeedsForFanOut verifies the provider-aware fan-out
// selection logic that fixes the latent bug where a provider-only
// update would skip every comparison.
func TestTargetFeedsForFanOut(t *testing.T) {
	// Build a minimal config with one geo provider, one ASN provider,
	// one bogon provider, and a couple of source names.
	cfg := newTestConfigForFanOut()
	output := []string{"feed_a", "feed_b"}

	t.Run("empty updated returns all", func(t *testing.T) {
		got := targetFeedsForFanOut(cfg, nil, output)
		if len(got) != 2 {
			t.Fatalf("got %v, want all output names", got)
		}
	})

	t.Run("only feed updates filters", func(t *testing.T) {
		got := targetFeedsForFanOut(cfg, []string{"feed_a"}, output)
		if len(got) != 1 || got[0] != "feed_a" {
			t.Fatalf("got %v, want [feed_a]", got)
		}
	})

	t.Run("provider update forces full fan-out", func(t *testing.T) {
		got := targetFeedsForFanOut(cfg, []string{"maxmind_geolite2"}, output)
		if len(got) != 2 {
			t.Fatalf("got %v, want all output names when provider updates", got)
		}
	})

	t.Run("asn provider update forces full fan-out", func(t *testing.T) {
		got := targetFeedsForFanOut(cfg, []string{"maxmind_asn"}, output)
		if len(got) != 2 {
			t.Fatalf("got %v, want all output names when ASN provider updates", got)
		}
	})

	t.Run("bogon provider update forces full fan-out", func(t *testing.T) {
		got := targetFeedsForFanOut(cfg, []string{"rfc_reserved"}, output)
		if len(got) != 2 {
			t.Fatalf("got %v, want all output names when bogon provider updates", got)
		}
	})

	t.Run("bogon provider underlying feed update forces full fan-out", func(t *testing.T) {
		// "bogons" is the source name referenced by the cymru_bogons
		// bogon provider in the test config; updating it should
		// force the full fan-out so the new bogon union takes effect.
		got := targetFeedsForFanOut(cfg, []string{"bogons"}, output)
		if len(got) != 2 {
			t.Fatalf("got %v, want all output names when bogon underlying feed updates", got)
		}
	})

	t.Run("mixed provider and feed update forces full fan-out", func(t *testing.T) {
		got := targetFeedsForFanOut(cfg, []string{"feed_a", "maxmind_geolite2"}, output)
		if len(got) != 2 {
			t.Fatalf("got %v, want all output names when mix includes a provider", got)
		}
	})

	t.Run("unknown name in updated names is ignored", func(t *testing.T) {
		got := targetFeedsForFanOut(cfg, []string{"never_existed"}, output)
		if len(got) != 0 {
			t.Fatalf("got %v, want empty for unknown-only updated names", got)
		}
	})
}
