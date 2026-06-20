package iprange

import (
	"bytes"
	"strings"
	"testing"
)

func parse6Str(t *testing.T, input string, opts ParseOptions) *IPSet6 {
	t.Helper()
	set, err := ParseReader6(t.Context(), "test", strings.NewReader(input), opts)
	if err != nil {
		t.Fatalf("ParseReader6: %v", err)
	}
	return set
}

func TestParseReader6IPv6AndRanges(t *testing.T) {
	set := parse6Str(t, "2001:db8::1\n2001:db8::10-2001:db8::20\n", DefaultParseOptions6())
	set.Optimize()
	if len(set.Ranges) != 2 {
		t.Fatalf("ranges = %d, want 2", len(set.Ranges))
	}
}

func TestParseReader6CIDR(t *testing.T) {
	set := parse6Str(t, "2001:db8::/64\n", DefaultParseOptions6())
	set.Optimize()
	if len(set.Ranges) != 1 {
		t.Fatalf("ranges = %d, want 1", len(set.Ranges))
	}
	if set.Ranges[0].Size().IsZero() {
		t.Fatal("expected non-zero /64 size")
	}
}

func TestParseReader6MixedIPv4IPv6(t *testing.T) {
	set := parse6Str(t, "192.168.0.1\n2001:db8::1\n", DefaultParseOptions6())
	set.Optimize()
	if len(set.Ranges) != 2 {
		t.Fatalf("ranges = %d, want 2", len(set.Ranges))
	}
}

func TestParseReader6CommentsBOMRangesAndCR(t *testing.T) {
	input := "\xEF\xBB\xBF# header\r\n" +
		" 2001:db8::1 ; comment\r\n" +
		"2001:db8::10 - 2001:db8::12 # comment\r\n" +
		"::ffff:192.0.2.1/128\r\n"
	set := parse6Str(t, input, DefaultParseOptions6())
	set.Optimize()
	if len(set.Ranges) != 3 {
		t.Fatalf("ranges = %d, want 3", len(set.Ranges))
	}
	if got := Uint128ToIPv6(set.Ranges[0].Lo); got != "::ffff:c000:201" {
		t.Fatalf("first range lo = %s, want mapped IPv4 address", got)
	}
	if got := Uint128ToIPv6(set.Ranges[1].Lo); got != "2001:db8::1" {
		t.Fatalf("second range lo = %s, want 2001:db8::1", got)
	}
	if got := set.Ranges[2].Size(); got != u128FromUint64(3) {
		t.Fatalf("third range size = %s, want 3", got.String())
	}
}

func TestParseReader6AcceptsVeryLongGarbageLine(t *testing.T) {
	input := strings.Repeat("x", 3*1024*1024) + "\n2001:db8::1\n"
	set := parse6Str(t, input, DefaultParseOptions6())
	set.Optimize()
	if len(set.Ranges) != 1 {
		t.Fatalf("ranges = %d, want 1", len(set.Ranges))
	}
}

func TestParseReader6BareIPv4UsesIPv4PrefixingBeforeMapping(t *testing.T) {
	opts := DefaultParseOptions6()
	opts.DefaultPrefix = 24
	set := parse6Str(t, "10.0.0.7\n", opts)
	set.Optimize()
	if len(set.Ranges) != 1 {
		t.Fatalf("ranges = %d, want 1", len(set.Ranges))
	}
	if got := Uint128ToIPv6(set.Ranges[0].Lo); got != "::ffff:a00:0" {
		t.Fatalf("lo = %s", got)
	}
	if got := Uint128ToIPv6(set.Ranges[0].Hi); got != "::ffff:a00:ff" {
		t.Fatalf("hi = %s", got)
	}
}

func TestParseReaderWrongBinaryFamilyErrors(t *testing.T) {
	v6 := makeTestSet6(Range6{Lo: u128FromUint64(1), Hi: u128FromUint64(2)})
	v6.Optimize()
	var v6buf bytes.Buffer
	if err := WriteBinary6(&v6buf, v6); err != nil {
		t.Fatalf("WriteBinary6: %v", err)
	}
	if _, err := ParseReader(t.Context(), "v6", bytes.NewReader(v6buf.Bytes()), DefaultParseOptions()); err == nil {
		t.Fatal("expected IPv6 binary in IPv4 mode to fail")
	}

	v4 := New("v4")
	if err := v4.Add(1, 2); err != nil {
		t.Fatalf("Add: %v", err)
	}
	v4.Optimize()
	var v4buf bytes.Buffer
	if err := WriteBinary(&v4buf, v4); err != nil {
		t.Fatalf("WriteBinary: %v", err)
	}
	if _, err := ParseReader6(t.Context(), "v4", bytes.NewReader(v4buf.Bytes()), DefaultParseOptions6()); err == nil {
		t.Fatal("expected IPv4 binary in IPv6 mode to fail")
	}
}
