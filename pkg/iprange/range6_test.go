package iprange

import (
	"math"
	"strconv"
	"testing"
)

func TestUint128Math(t *testing.T) {
	a := u128FromUint64(100)
	b := u128FromUint64(50)
	if got := a.Add(b); !got.Equals(u128FromUint64(150)) {
		t.Fatalf("100 + 50 = %s", got)
	}
	if got := a.Sub(b); !got.Equals(u128FromUint64(50)) {
		t.Fatalf("100 - 50 = %s", got)
	}
	overflow := uint128{Lo: math.MaxUint64}.Add64(1)
	if overflow.Hi != 1 || overflow.Lo != 0 {
		t.Fatalf("overflow add64 = {%d,%d}", overflow.Hi, overflow.Lo)
	}
}

func TestUint128DecimalRoundTrip(t *testing.T) {
	tests := []Uint128{
		uint128Zero,
		u128FromUint64(42),
		u128FromHiLo(0, math.MaxUint64),
		u128FromHiLo(math.MaxUint64, math.MaxUint64),
	}
	for _, tc := range tests {
		got, err := ParseUint128(tc.String())
		if err != nil {
			t.Fatalf("ParseUint128(%q): %v", tc.String(), err)
		}
		if !got.Equals(tc) {
			t.Fatalf("roundtrip mismatch: got %s want %s", got, tc)
		}
	}
}

func TestUint128ToIPv6Formatting(t *testing.T) {
	tests := []struct {
		addr Uint128
		want string
	}{
		{uint128Zero, "::"},
		{u128FromHiLo(0x20010db800000000, 0), "2001:db8::"},
		{u128FromHiLo(0x20010db800000000, 1), "2001:db8::1"},
		{IPv4ToMapped6(0x0a000001), "::ffff:a00:1"},
	}
	for _, tc := range tests {
		if got := Uint128ToIPv6(tc.addr); got != tc.want {
			t.Fatalf("Uint128ToIPv6(%v) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestParseIPv6Token(t *testing.T) {
	got, err := ParseIPv6Token("2001:db8::1")
	if err != nil {
		t.Fatalf("ParseIPv6Token: %v", err)
	}
	if want := u128FromHiLo(0x20010db800000000, 1); !got.Equals(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestIPv4MappedHelpers(t *testing.T) {
	addr := IPv4ToMapped6(0x0a000001)
	if !IsIPv4Mapped6(addr) {
		t.Fatal("expected mapped IPv4 address")
	}
	v4, ok := Mapped6ToIPv4(addr)
	if !ok || v4 != 0x0a000001 {
		t.Fatalf("Mapped6ToIPv4 = %08x, %v", v4, ok)
	}
}

func TestSplitRange6CIDR(t *testing.T) {
	var got []string
	var enabled [129]bool
	for i := range enabled {
		enabled[i] = true
	}
	err := splitRange6(uint128Zero, 0, u128FromHiLo(0x20010db800000000, 0), u128FromHiLo(0x20010db800000000, 0xff), enabled, func(addr Uint128, prefix int) error {
		got = append(got, Uint128ToIPv6(addr)+"/"+strconv.Itoa(prefix))
		return nil
	})
	if err != nil {
		t.Fatalf("splitRange6: %v", err)
	}
	if len(got) != 1 || got[0] != "2001:db8::/120" {
		t.Fatalf("got %v", got)
	}
}
