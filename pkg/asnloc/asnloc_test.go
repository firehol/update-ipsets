package asnloc

import (
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
