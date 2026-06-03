package asnloc

import (
	"errors"
	"net"
	"os"
	"testing"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestUint32ToIP(t *testing.T) {
	cases := []struct {
		in   uint32
		want string
	}{
		{0, "0.0.0.0"},
		{0x01020304, "1.2.3.4"},
		{0x08080808, "8.8.8.8"},
		{0xC0A80001, "192.168.0.1"},
		{0xFFFFFFFF, "255.255.255.255"},
	}
	for _, c := range cases {
		got := uint32ToIP(c.in).String()
		if got != c.want {
			t.Errorf("uint32ToIP(%#x) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIPNetToBounds(t *testing.T) {
	cases := []struct {
		cidr   string
		lo, hi uint32
		ok     bool
	}{
		{"0.0.0.0/0", 0, 0xFFFFFFFF, true},
		{"10.0.0.0/8", 0x0A000000, 0x0AFFFFFF, true},
		{"192.168.1.0/24", 0xC0A80100, 0xC0A801FF, true},
		{"1.2.3.4/32", 0x01020304, 0x01020304, true},
		{"172.16.0.0/12", 0xAC100000, 0xAC1FFFFF, true},
	}
	for _, c := range cases {
		_, ipnet, err := net.ParseCIDR(c.cidr)
		if err != nil {
			t.Fatalf("ParseCIDR(%q): %v", c.cidr, err)
		}
		lo, hi, ok := ipnetToBounds(ipnet)
		if ok != c.ok || lo != c.lo || hi != c.hi {
			t.Errorf("ipnetToBounds(%s) = (%#x, %#x, %v), want (%#x, %#x, %v)",
				c.cidr, lo, hi, ok, c.lo, c.hi, c.ok)
		}
	}
}

func TestIPNetToBoundsRejectsIPv6(t *testing.T) {
	_, ipnet, err := net.ParseCIDR("2001:db8::/32")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := ipnetToBounds(ipnet); ok {
		t.Errorf("ipnetToBounds should reject IPv6 network")
	}
}

func TestOpenRejectsUnknownType(t *testing.T) {
	if _, err := Open("not_a_real_provider", "/tmp/nonexistent"); err == nil {
		t.Error("Open should reject unknown provider types")
	}
}

func TestOpenTextBackedProviders(t *testing.T) {
	iptoasnPath := writeASNLocTempFile(t, "iptoasn.tsv", sampleIPToASNTSV)
	db, err := Open("iptoasn_combined_tsv", iptoasnPath)
	if err != nil {
		t.Fatalf("Open(iptoasn_combined_tsv) error = %v", err)
	}
	networks, covered, err := db.Stats()
	if err != nil {
		t.Fatalf("iptoasn Stats() error = %v", err)
	}
	if networks != 4 || covered != 1792 {
		t.Fatalf("iptoasn Stats() = (%d, %d), want (4, 1792)", networks, covered)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("iptoasn Close() error = %v", err)
	}

	caidaPath := writeASNLocTempFile(t, "caida.pfx2as", sampleCAIDAPrefix2AS)
	db, err = Open("caida_prefix2as", caidaPath)
	if err != nil {
		t.Fatalf("Open(caida_prefix2as) error = %v", err)
	}
	networks, covered, err = db.Stats()
	if err != nil {
		t.Fatalf("caida Stats() error = %v", err)
	}
	if networks != 6 || covered != 6144 {
		t.Fatalf("caida Stats() = (%d, %d), want (6, 6144)", networks, covered)
	}
}

func TestDatabaseNilAndWrapperBehavior(t *testing.T) {
	var nilDB *Database
	if err := nilDB.Close(); err != nil {
		t.Fatalf("nil Close() error = %v", err)
	}
	if _, _, err := nilDB.Lookup(0xC0000201); err == nil {
		t.Fatal("nil Lookup() error = nil, want error")
	}
	if _, _, err := nilDB.Stats(); err == nil {
		t.Fatal("nil Stats() error = nil, want error")
	}

	db := &Database{
		Provider: "test",
		be: newRangeTableBackend([]asnRange{
			{lo: 0xC0000200, hi: 0xC000027F, asn: 64500, name: "EXAMPLE-A"},
		}),
	}
	rec, network, err := db.Lookup(0xC0000201)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if rec != (Record{ASN: 64500, Name: "EXAMPLE-A"}) {
		t.Fatalf("Lookup() record = %#v", rec)
	}
	if network != (Network{Lo: 0xC0000200, Hi: 0xC000027F}) {
		t.Fatalf("Lookup() network = %#v", network)
	}
	networks, covered, err := db.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if networks != 1 || covered != 128 {
		t.Fatalf("Stats() = (%d, %d), want (1, 128)", networks, covered)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCountFeedRangeWalking(t *testing.T) {
	db := testASNDatabase()
	set := iprange.New("feed")
	addRange(t, set, 0xC0000200, 0xC00002FF) // 192.0.2.0/24
	addRange(t, set, 0xCB007100, 0xCB007103) // 203.0.113.0/30

	counts, names, err := db.CountFeed(set)
	if err != nil {
		t.Fatalf("CountFeed() error = %v", err)
	}
	if counts[64500] != 128 || counts[64501] != 128 || counts[0] != 4 {
		t.Fatalf("CountFeed() counts = %#v", counts)
	}
	if names[64500] != "EXAMPLE-A" || names[64501] != "EXAMPLE-B" {
		t.Fatalf("CountFeed() names = %#v", names)
	}
}

func TestCountFeedWithBogons(t *testing.T) {
	db := testASNDatabase()
	set := iprange.New("feed")
	addRange(t, set, 0xC0000200, 0xC00002FF) // 192.0.2.0/24
	addRange(t, set, 0xCB007100, 0xCB007103) // 203.0.113.0/30

	bogons := iprange.New("bogons")
	addRange(t, bogons, 0xC0000280, 0xC00002FF) // second half of 192.0.2.0/24
	addRange(t, bogons, 0xCB007102, 0xCB007103) // half of 203.0.113.0/30

	counts, names, bogonCount, err := db.CountFeedWithBogons(set, bogons)
	if err != nil {
		t.Fatalf("CountFeedWithBogons() error = %v", err)
	}
	if bogonCount != 130 {
		t.Fatalf("bogonCount = %d, want 130", bogonCount)
	}
	if counts[64500] != 128 || counts[64501] != 0 || counts[0] != 2 {
		t.Fatalf("CountFeedWithBogons() counts = %#v", counts)
	}
	if names[64500] != "EXAMPLE-A" {
		t.Fatalf("CountFeedWithBogons() names = %#v", names)
	}

	plainCounts, plainNames, plainBogons, err := db.CountFeedWithBogons(set, nil)
	if err != nil {
		t.Fatalf("CountFeedWithBogons(nil) error = %v", err)
	}
	if plainBogons != 0 || plainCounts[64500] != 128 || plainCounts[64501] != 128 || plainNames[64501] != "EXAMPLE-B" {
		t.Fatalf("CountFeedWithBogons(nil) = counts %#v names %#v bogons %d", plainCounts, plainNames, plainBogons)
	}
}

func TestCountFeedPropagatesLookupError(t *testing.T) {
	wantErr := errors.New("lookup failed")
	db := &Database{be: errorBackend{err: wantErr}}
	set := iprange.New("feed")
	addRange(t, set, 0xC0000200, 0xC0000200)

	if _, _, err := db.CountFeed(set); !errors.Is(err, wantErr) {
		t.Fatalf("CountFeed() error = %v, want %v", err, wantErr)
	}
}

func TestMMDBDecoderForKnownTypes(t *testing.T) {
	for _, providerType := range []string{"maxmind_geolite2_asn_mmdb", "dbip_asn_lite_mmdb"} {
		if _, err := mmdbDecoderFor(providerType); err != nil {
			t.Errorf("mmdbDecoderFor should accept %s: %v", providerType, err)
		}
	}
	if _, err := mmdbDecoderFor("not_a_real_provider"); err == nil {
		t.Error("mmdbDecoderFor should reject unknown provider types")
	}
}

func TestIPNetToBoundsHandlesIPv4MappedMasks(t *testing.T) {
	_, ipnet, err := net.ParseCIDR("::ffff:192.0.2.0/120")
	if err != nil {
		t.Fatal(err)
	}
	lo, hi, ok := ipnetToBounds(ipnet)
	if !ok {
		t.Fatal("ipnetToBounds() rejected IPv4-mapped network")
	}
	if lo != 0xC0000200 || hi != 0xC00002FF {
		t.Fatalf("ipnetToBounds() = (%#x, %#x), want 192.0.2.0/24", lo, hi)
	}
}

func testASNDatabase() *Database {
	return &Database{
		Provider: "test",
		be: newRangeTableBackend([]asnRange{
			{lo: 0xC0000200, hi: 0xC000027F, asn: 64500, name: "EXAMPLE-A"},
			{lo: 0xC0000280, hi: 0xC00002FF, asn: 64501, name: "EXAMPLE-B"},
			{lo: 0xC6336400, hi: 0xC633647F, asn: 64502, name: "EXAMPLE-C"},
		}),
	}
}

type errorBackend struct {
	err error
}

func (e errorBackend) lookup(uint32) (Record, Network, error) {
	return Record{}, Network{}, e.err
}

func (e errorBackend) stats() (int, uint64, error) {
	return 0, 0, e.err
}

func (e errorBackend) close() error {
	return e.err
}

func addRange(t *testing.T, set *iprange.IPSet, lo, hi uint32) {
	t.Helper()
	if err := set.Add(lo, hi); err != nil {
		t.Fatalf("add range %#x-%#x: %v", lo, hi, err)
	}
}

func writeASNLocTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := t.TempDir() + string(os.PathSeparator) + name
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// TestCountFeedWithRealMMDB exercises the full range-walking path against
// a real MaxMind GeoLite2-ASN database. We point this at the file the
// daemon downloads at runtime; if it's missing the test is skipped so
// CI without an MMDB still passes.
func TestCountFeedWithRealMMDB(t *testing.T) {
	// Look for an MMDB in well-known locations the project uses.
	candidates := []string{
		os.Getenv("GEOLITE2_ASN_MMDB"),
		"/var/cache/update-ipsets/maxmind_geolite2_asn/GeoLite2-ASN.mmdb",
		"/etc/firehol/asn/maxmind_geolite2.source",
	}
	var path string
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		t.Skip("no MaxMind GeoLite2-ASN MMDB available; set GEOLITE2_ASN_MMDB to enable")
	}

	db, err := Open("maxmind_geolite2_asn_mmdb", path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Build a small test set: Cloudflare DNS (1.1.1.1), Google DNS (8.8.8.8),
	// and a /24 inside Cloudflare's range.
	set := iprange.New("test")
	if err := set.Add(0x01010101, 0x01010101); err != nil { // 1.1.1.1
		t.Fatal(err)
	}
	if err := set.Add(0x08080808, 0x08080808); err != nil { // 8.8.8.8
		t.Fatal(err)
	}
	if err := set.Add(0x68F42400, 0x68F424FF); err != nil { // 104.244.36.0/24 (Twitter)
		t.Fatal(err)
	}

	counts, names, err := db.CountFeed(set)
	if err != nil {
		t.Fatalf("CountFeed: %v", err)
	}
	if len(counts) == 0 {
		t.Fatal("CountFeed returned no counts")
	}
	var totalIPs uint64
	for _, n := range counts {
		totalIPs += n
	}
	if totalIPs != 258 { // 1 + 1 + 256
		t.Errorf("total counted IPs = %d, want 258", totalIPs)
	}
	t.Logf("counts: %v", counts)
	t.Logf("names: %v", names)
}
