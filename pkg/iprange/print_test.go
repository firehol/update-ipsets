package iprange

import (
	"bytes"
	"testing"
)

func TestPrintCIDR(t *testing.T) {
	set := newOptimizedSet("cidr", Range{Lo: mustIP(t, "192.0.2.0"), Hi: mustIP(t, "192.0.2.255")})
	var buf bytes.Buffer
	if err := set.Write(&buf, DefaultPrintOptions()); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "192.0.2.0/24\n"; got != want {
		t.Fatalf("unexpected CIDR output %q", got)
	}
}

func TestPrintCIDRWithRestrictedPrefixes(t *testing.T) {
	set := newOptimizedSet("cidr", Range{Lo: mustIP(t, "192.0.2.0"), Hi: mustIP(t, "192.0.2.255")})
	opts := DefaultPrintOptions()
	opts.PrefixesEnabled[24] = false

	var buf bytes.Buffer
	if err := set.Write(&buf, opts); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "192.0.2.0/25\n192.0.2.128/25\n"; got != want {
		t.Fatalf("unexpected CIDR output %q", got)
	}
}

func TestPrintRangesAndSingleIPs(t *testing.T) {
	set := newOptimizedSet("ranges", Range{Lo: mustIP(t, "192.0.2.1"), Hi: mustIP(t, "192.0.2.3")})

	var ranges bytes.Buffer
	opts := DefaultPrintOptions()
	opts.Format = PrintRanges
	if err := set.Write(&ranges, opts); err != nil {
		t.Fatal(err)
	}
	if got, want := ranges.String(), "192.0.2.1-192.0.2.3\n"; got != want {
		t.Fatalf("unexpected range output %q", got)
	}

	var singles bytes.Buffer
	opts.Format = PrintSingleIPs
	if err := set.Write(&singles, opts); err != nil {
		t.Fatal(err)
	}
	if got, want := singles.String(), "192.0.2.1\n192.0.2.2\n192.0.2.3\n"; got != want {
		t.Fatalf("unexpected single IP output %q", got)
	}
}

func TestPrintCIDRSkipsDisabledHostPrefixes(t *testing.T) {
	set := newOptimizedSet("hosts-and-nets",
		Range{Lo: mustIP(t, "192.0.2.1"), Hi: mustIP(t, "192.0.2.1")},
		Range{Lo: mustIP(t, "192.0.2.128"), Hi: mustIP(t, "192.0.2.255")},
	)
	opts := DefaultPrintOptions()
	opts.PrefixesEnabled[32] = false

	var buf bytes.Buffer
	if err := set.Write(&buf, opts); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "192.0.2.128/25\n"; got != want {
		t.Fatalf("unexpected CIDR output %q", got)
	}
}
