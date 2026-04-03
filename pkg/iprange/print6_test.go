package iprange

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrint6CIDRAndRanges(t *testing.T) {
	s := makeTestSet6(
		Range6{Lo: u128FromHiLo(0x20010db800000000, 0), Hi: u128FromHiLo(0x20010db800000000, 0xff)},
	)
	s.Optimize()

	var cidr bytes.Buffer
	if err := s.Write6(&cidr, DefaultPrintOptions6()); err != nil {
		t.Fatalf("Write6 CIDR: %v", err)
	}
	if !strings.Contains(cidr.String(), "2001:db8::/120") {
		t.Fatalf("unexpected CIDR output: %s", cidr.String())
	}

	opts := DefaultPrintOptions6()
	opts.Format = PrintRanges
	var ranges bytes.Buffer
	if err := s.Write6(&ranges, opts); err != nil {
		t.Fatalf("Write6 ranges: %v", err)
	}
	if !strings.Contains(ranges.String(), "-") {
		t.Fatalf("unexpected ranges output: %s", ranges.String())
	}
}

func TestPrint6SingleIPsAndBinary(t *testing.T) {
	s := makeTestSet6(Range6{Lo: u128FromUint64(1), Hi: u128FromUint64(3)})
	s.Optimize()

	opts := DefaultPrintOptions6()
	opts.Format = PrintSingleIPs
	var singles bytes.Buffer
	if err := s.Write6(&singles, opts); err != nil {
		t.Fatalf("Write6 single IPs: %v", err)
	}
	if got := len(strings.Split(strings.TrimSpace(singles.String()), "\n")); got != 3 {
		t.Fatalf("single IP lines = %d, want 3", got)
	}

	opts = DefaultPrintOptions6()
	opts.Format = PrintBinary
	var binary bytes.Buffer
	if err := s.Write6(&binary, opts); err != nil {
		t.Fatalf("Write6 binary: %v", err)
	}
	if !strings.HasPrefix(binary.String(), BinaryHeaderV20IPv6) {
		t.Fatalf("missing IPv6 binary header")
	}
}

func TestPrint6SingleIPsCap(t *testing.T) {
	s := makeTestSet6(Range6{Lo: uint128Zero, Hi: u128FromUint64(256 * 256 * 256)})
	s.Optimize()

	opts := DefaultPrintOptions6()
	opts.Format = PrintSingleIPs
	if err := s.Write6(&bytes.Buffer{}, opts); err != ErrSingleIPsRangeTooBig {
		t.Fatalf("expected ErrSingleIPsRangeTooBig, got %v", err)
	}
}
