package iprange

import (
	"context"
	"strings"
	"testing"
)

func TestParseReaderMixedInput(t *testing.T) {
	input := `
# comment
1.2.3.4
10.0.0.0/24
192.0.2.10 - 192.0.2.12
192.168.0.0/255.255.255.0
`

	set, err := ParseReader(t.Context(), "mixed", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatal(err)
	}
	set.Optimize()

	expectRanges(t, set, []Range{
		{Lo: mustIP(t, "1.2.3.4"), Hi: mustIP(t, "1.2.3.4")},
		{Lo: mustIP(t, "10.0.0.0"), Hi: mustIP(t, "10.0.0.255")},
		{Lo: mustIP(t, "192.0.2.10"), Hi: mustIP(t, "192.0.2.12")},
		{Lo: mustIP(t, "192.168.0.0"), Hi: mustIP(t, "192.168.0.255")},
	})
}

func TestParseReaderWithoutFixNetwork(t *testing.T) {
	opts := DefaultParseOptions()
	opts.UseCIDRNetwork = false

	set, err := ParseReader(t.Context(), "nofix", strings.NewReader("1.2.3.5/30\n"), opts)
	if err != nil {
		t.Fatal(err)
	}
	set.Optimize()

	expectRanges(t, set, []Range{{Lo: mustIP(t, "1.2.3.5"), Hi: mustIP(t, "1.2.3.7")}})
}

func TestParseReaderHostname(t *testing.T) {
	opts := DefaultParseOptions()
	opts.Resolver = staticResolver{
		"example.net": {mustIP(t, "203.0.113.10"), mustIP(t, "203.0.113.20")},
	}

	set, err := ParseReader(t.Context(), "hosts", strings.NewReader("example.net\n"), opts)
	if err != nil {
		t.Fatal(err)
	}
	set.Optimize()

	expectRanges(t, set, []Range{
		{Lo: mustIP(t, "203.0.113.10"), Hi: mustIP(t, "203.0.113.10")},
		{Lo: mustIP(t, "203.0.113.20"), Hi: mustIP(t, "203.0.113.20")},
	})
}

type staticResolver map[string][]uint32

func (s staticResolver) LookupIPv4(_ context.Context, host string) ([]uint32, error) {
	return s[host], nil
}

func mustIP(t *testing.T, ip string) uint32 {
	t.Helper()
	value, err := ParseIPv4Token(ip)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
