package asnloc

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleIPToASNTSV = `1.0.0.0	1.0.0.255	13335	US	CLOUDFLARENET
1.0.1.0	1.0.3.255	0	None	Not routed
1.0.4.0	1.0.7.255	38803	AU	GTELECOM-AS-AP Gtelecom Pty Ltd
1.0.16.0	1.0.16.255	2519	JP	VECTANT ARTERIA Networks Corporation
8.8.8.0	8.8.8.255	15169	US	GOOGLE
2001:200::	2001:200:5ff:ffff:ffff:ffff:ffff:ffff	2500	JP	WIDE-BB WIDE Project
# this is a comment line


`

func TestParseIPToASNTSVHappy(t *testing.T) {
	ranges, err := parseIPToASNTSVStream(strings.NewReader(sampleIPToASNTSV))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	// 5 IPv4 lines, but AS=0 line is skipped, IPv6 line is skipped → 4 expected.
	if len(ranges) != 4 {
		t.Fatalf("got %d ranges, want 4: %+v", len(ranges), ranges)
	}
	// Spot-check Cloudflare 1.0.0.0/24.
	if ranges[0].lo != 0x01000000 || ranges[0].hi != 0x010000FF {
		t.Errorf("first range bounds wrong: %x..%x", ranges[0].lo, ranges[0].hi)
	}
	if ranges[0].asn != 13335 || ranges[0].name != "CLOUDFLARENET" {
		t.Errorf("first range attribution wrong: asn=%d name=%q", ranges[0].asn, ranges[0].name)
	}
}

func TestLoadIPToASNTSVPlainAndGzip(t *testing.T) {
	plainPath := filepath.Join(t.TempDir(), "iptoasn.tsv")
	if err := os.WriteFile(plainPath, []byte(sampleIPToASNTSV), 0o600); err != nil {
		t.Fatal(err)
	}
	be, err := loadIPToASNTSV(plainPath)
	if err != nil {
		t.Fatalf("loadIPToASNTSV(plain) error = %v", err)
	}
	networks, covered, err := be.stats()
	if err != nil {
		t.Fatalf("plain stats error = %v", err)
	}
	if networks != 4 || covered != 1792 {
		t.Fatalf("plain stats = (%d, %d), want (4, 1792)", networks, covered)
	}

	gzipPath := filepath.Join(t.TempDir(), "iptoasn.tsv.gz")
	writeGzipASNLocFixture(t, gzipPath, sampleIPToASNTSV)
	be, err = loadIPToASNTSV(gzipPath)
	if err != nil {
		t.Fatalf("loadIPToASNTSV(gzip) error = %v", err)
	}
	networks, covered, err = be.stats()
	if err != nil {
		t.Fatalf("gzip stats error = %v", err)
	}
	if networks != 4 || covered != 1792 {
		t.Fatalf("gzip stats = (%d, %d), want (4, 1792)", networks, covered)
	}
}

func TestLoadIPToASNTSVErrors(t *testing.T) {
	if _, err := loadIPToASNTSV(filepath.Join(t.TempDir(), "missing.tsv")); err == nil {
		t.Fatal("loadIPToASNTSV(missing) error = nil, want error")
	}

	path := filepath.Join(t.TempDir(), "bad.tsv")
	if err := os.WriteFile(path, []byte("1.2.3.0\t1.2.3.255\t13335\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadIPToASNTSV(path); err == nil {
		t.Fatal("loadIPToASNTSV(bad) error = nil, want parse error")
	}
}

func TestParseIPToASNTSVSkipsAS0(t *testing.T) {
	in := "1.2.3.0\t1.2.3.255\t0\tNone\tNot routed\n"
	ranges, err := parseIPToASNTSVStream(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(ranges) != 0 {
		t.Errorf("AS0 line should be skipped, got %+v", ranges)
	}
}

func TestParseIPToASNTSVRejectsTooFewColumns(t *testing.T) {
	in := "1.2.3.0\t1.2.3.255\t13335\n"
	if _, err := parseIPToASNTSVStream(strings.NewReader(in)); err == nil {
		t.Error("expected error on short line, got nil")
	}
}

func TestParseIPToASNTSVRejectsBadIP(t *testing.T) {
	in := "not.an.ip.0\t1.2.3.255\t13335\tUS\tCLOUDFLARENET\n"
	if _, err := parseIPToASNTSVStream(strings.NewReader(in)); err == nil {
		t.Error("expected error on malformed start IP, got nil")
	}
}

func TestParseIPToASNTSVRejectsReversedRange(t *testing.T) {
	in := "1.2.3.255\t1.2.3.0\t13335\tUS\tCLOUDFLARENET\n"
	if _, err := parseIPToASNTSVStream(strings.NewReader(in)); err == nil {
		t.Error("expected error on hi<lo, got nil")
	}
}

func TestParseIPv4DecimalRejectsInvalidInputs(t *testing.T) {
	for _, raw := range []string{"", "not.an.ip", "2001:db8::1"} {
		t.Run(raw, func(t *testing.T) {
			if _, ok := parseIPv4Decimal(raw); ok {
				t.Fatalf("parseIPv4Decimal(%q) ok = true, want false", raw)
			}
		})
	}
}

func TestRangeTableLookupHitMissAdjacent(t *testing.T) {
	// Build a backend manually so the test does not depend on file IO.
	be := newRangeTableBackend([]asnRange{
		{lo: 0x01000000, hi: 0x010000FF, asn: 13335, name: "CLOUDFLARENET"}, // 1.0.0.0/24
		{lo: 0x08080800, hi: 0x080808FF, asn: 15169, name: "GOOGLE"},        // 8.8.8.0/24
	})

	// Hit inside the first range.
	rec, nw, err := be.lookup(0x01000010) // 1.0.0.16
	if err != nil {
		t.Fatalf("lookup err: %v", err)
	}
	if rec.ASN != 13335 || rec.Name != "CLOUDFLARENET" {
		t.Errorf("hit attribution wrong: %+v", rec)
	}
	if nw.Lo != 0x01000000 || nw.Hi != 0x010000FF {
		t.Errorf("hit bounds wrong: %x..%x", nw.Lo, nw.Hi)
	}

	// Miss between the two ranges — gap should span 1.0.1.0..8.8.7.255.
	rec, nw, err = be.lookup(0x01000100) // 1.0.1.0
	if err != nil {
		t.Fatalf("miss err: %v", err)
	}
	if rec.ASN != 0 {
		t.Errorf("miss should report ASN 0, got %d", rec.ASN)
	}
	if nw.Lo != 0x01000100 || nw.Hi != 0x080807FF {
		t.Errorf("miss gap bounds wrong: %x..%x", nw.Lo, nw.Hi)
	}

	// Miss before any known range — gap should span [0, first_range_lo - 1].
	_, nw, _ = be.lookup(0x00000001)
	if nw.Lo != 0x00000000 || nw.Hi != 0x00FFFFFF {
		t.Errorf("leading gap bounds wrong: %x..%x", nw.Lo, nw.Hi)
	}

	// Miss after the last known range — gap should span [last_range_hi + 1, 2^32-1].
	_, nw, _ = be.lookup(0x09000000)
	if nw.Lo != 0x08080900 || nw.Hi != ^uint32(0) {
		t.Errorf("trailing gap bounds wrong: %x..%x", nw.Lo, nw.Hi)
	}
}

func TestRangeTableMergesAdjacentSameASN(t *testing.T) {
	// Two touching /24s with the same ASN should merge into one /23.
	be := newRangeTableBackend([]asnRange{
		{lo: 0x01000000, hi: 0x010000FF, asn: 13335, name: "CLOUDFLARENET"},
		{lo: 0x01000100, hi: 0x010001FF, asn: 13335, name: "CLOUDFLARENET"},
	})
	if len(be.ranges) != 1 {
		t.Fatalf("expected 1 merged range, got %d", len(be.ranges))
	}
	if be.ranges[0].lo != 0x01000000 || be.ranges[0].hi != 0x010001FF {
		t.Errorf("merged range bounds wrong: %x..%x", be.ranges[0].lo, be.ranges[0].hi)
	}
}

func TestRangeTableStats(t *testing.T) {
	be := newRangeTableBackend([]asnRange{
		{lo: 0x01000000, hi: 0x010000FF, asn: 13335}, // 256 IPs
		{lo: 0x08080800, hi: 0x080808FF, asn: 15169}, // 256 IPs
	})
	n, ips, err := be.stats()
	if err != nil {
		t.Fatalf("stats err: %v", err)
	}
	if n != 2 {
		t.Errorf("network count = %d, want 2", n)
	}
	if ips != 512 {
		t.Errorf("ip count = %d, want 512", ips)
	}
}

func TestRangeTableNilAndOverlapBehavior(t *testing.T) {
	var nilBackend *rangeTableBackend
	rec, network, err := nilBackend.lookup(0xC0000201)
	if err != nil {
		t.Fatalf("nil lookup error = %v", err)
	}
	if rec.ASN != 0 || network.Lo != 0xC0000201 || network.Hi != ^uint32(0) {
		t.Fatalf("nil lookup = record %#v network %#v", rec, network)
	}
	if _, _, err := nilBackend.stats(); err == nil {
		t.Fatal("nil stats error = nil, want error")
	}
	if err := nilBackend.close(); err != nil {
		t.Fatalf("nil close error = %v", err)
	}

	be := newRangeTableBackend([]asnRange{
		{lo: 0xC0000200, hi: 0xC00002FF, asn: 64500},
		{lo: 0xC0000280, hi: 0xC00003FF, asn: 64501},
	})
	if len(be.ranges) != 2 {
		t.Fatalf("range count = %d, want 2", len(be.ranges))
	}
	if be.ranges[1].lo != 0xC0000300 || be.ranges[1].hi != 0xC00003FF {
		t.Fatalf("overlap adjusted range = %#v", be.ranges[1])
	}
}

func writeGzipASNLocFixture(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(content)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
