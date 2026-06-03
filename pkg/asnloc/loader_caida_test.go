package asnloc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleCAIDAPrefix2AS = `# header that should be ignored
1.0.0.0	24	13335
1.0.4.0	22	38803
1.0.16.0	24	2519
1.1.8.0	24	149511_138421
1.51.32.0	20	24361_4538
8.8.8.0	24	15169
2001:200::	32	2500
`

func TestParseCAIDAPrefix2ASHappy(t *testing.T) {
	ranges, err := parseCAIDAPrefix2ASStream(strings.NewReader(sampleCAIDAPrefix2AS))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	// 6 IPv4 lines, IPv6 line skipped → 6.
	if len(ranges) != 6 {
		t.Fatalf("got %d ranges, want 6: %+v", len(ranges), ranges)
	}
	// 1.0.0.0/24 → 1.0.0.0..1.0.0.255
	if ranges[0].lo != 0x01000000 || ranges[0].hi != 0x010000FF || ranges[0].asn != 13335 {
		t.Errorf("first range wrong: %+v", ranges[0])
	}
	// 1.0.4.0/22 → 1.0.4.0..1.0.7.255
	if ranges[1].lo != 0x01000400 || ranges[1].hi != 0x010007FF || ranges[1].asn != 38803 {
		t.Errorf("second range wrong: %+v", ranges[1])
	}
	// MOAS line: 1.1.8.0/24 → primary ASN is 149511 (first in underscore list).
	var found bool
	for _, r := range ranges {
		if r.lo == 0x01010800 && r.hi == 0x010108FF {
			if r.asn != 149511 {
				t.Errorf("MOAS primary ASN wrong: got %d want 149511", r.asn)
			}
			found = true
		}
	}
	if !found {
		t.Error("MOAS row not in parsed output")
	}
}

func TestLoadCAIDAPrefix2ASPlainAndGzip(t *testing.T) {
	plainPath := filepath.Join(t.TempDir(), "routeviews.pfx2as")
	if err := os.WriteFile(plainPath, []byte(sampleCAIDAPrefix2AS), 0o600); err != nil {
		t.Fatal(err)
	}
	be, err := loadCAIDAPrefix2AS(plainPath)
	if err != nil {
		t.Fatalf("loadCAIDAPrefix2AS(plain) error = %v", err)
	}
	networks, covered, err := be.stats()
	if err != nil {
		t.Fatalf("plain stats error = %v", err)
	}
	if networks != 6 || covered != 6144 {
		t.Fatalf("plain stats = (%d, %d), want (6, 6144)", networks, covered)
	}

	gzipPath := filepath.Join(t.TempDir(), "routeviews.pfx2as.gz")
	writeGzipASNLocFixture(t, gzipPath, sampleCAIDAPrefix2AS)
	be, err = loadCAIDAPrefix2AS(gzipPath)
	if err != nil {
		t.Fatalf("loadCAIDAPrefix2AS(gzip) error = %v", err)
	}
	networks, covered, err = be.stats()
	if err != nil {
		t.Fatalf("gzip stats error = %v", err)
	}
	if networks != 6 || covered != 6144 {
		t.Fatalf("gzip stats = (%d, %d), want (6, 6144)", networks, covered)
	}
}

func TestLoadCAIDAPrefix2ASErrors(t *testing.T) {
	if _, err := loadCAIDAPrefix2AS(filepath.Join(t.TempDir(), "missing.pfx2as")); err == nil {
		t.Fatal("loadCAIDAPrefix2AS(missing) error = nil, want error")
	}

	path := filepath.Join(t.TempDir(), "bad.pfx2as")
	if err := os.WriteFile(path, []byte("1.0.0.0\t99\t13335\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCAIDAPrefix2AS(path); err == nil {
		t.Fatal("loadCAIDAPrefix2AS(bad) error = nil, want parse error")
	}
}

func TestParseCAIDAPrefix2ASNamesAreEmpty(t *testing.T) {
	ranges, err := parseCAIDAPrefix2ASStream(strings.NewReader(sampleCAIDAPrefix2AS))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	for _, r := range ranges {
		if r.name != "" {
			t.Errorf("CAIDA pfx2as has no names — got %q", r.name)
		}
	}
}

func TestParseCAIDAPrefix2ASRejectsBadPrefixLength(t *testing.T) {
	in := "1.0.0.0\t99\t13335\n"
	if _, err := parseCAIDAPrefix2ASStream(strings.NewReader(in)); err == nil {
		t.Error("expected error on out-of-range prefix length")
	}
}

func TestParseCAIDAPrefix2ASRejectsBadASN(t *testing.T) {
	in := "1.0.0.0\t24\tnot-a-number\n"
	if _, err := parseCAIDAPrefix2ASStream(strings.NewReader(in)); err == nil {
		t.Error("expected error on non-numeric ASN")
	}
}

func TestParseCAIDAPrefix2ASRejectsTooFewColumns(t *testing.T) {
	in := "1.0.0.0\t24\n"
	if _, err := parseCAIDAPrefix2ASStream(strings.NewReader(in)); err == nil {
		t.Error("expected error on truncated line")
	}
}

func TestParseCAIDAPrefix2ASAcceptsCommaMOAS(t *testing.T) {
	// Defensive: some historical files used commas instead of underscores.
	in := "1.0.0.0\t24\t13335,38803\n"
	ranges, err := parseCAIDAPrefix2ASStream(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	if len(ranges) != 1 || ranges[0].asn != 13335 {
		t.Errorf("comma MOAS not handled: %+v", ranges)
	}
}
