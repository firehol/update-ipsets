package iprange

import (
	"bytes"
	"testing"
)

func TestBinary6RoundTrip(t *testing.T) {
	s := makeTestSet6(
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(20)},
		Range6{Lo: u128FromHiLo(0x20010db800000000, 1), Hi: u128FromHiLo(0x20010db800000000, 0xff)},
	)
	s.Lines = 7
	s.Optimize()

	var buf bytes.Buffer
	if err := WriteBinary6(&buf, s); err != nil {
		t.Fatalf("WriteBinary6: %v", err)
	}
	got, err := ReadBinary6("test", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadBinary6: %v", err)
	}
	if len(got.Ranges) != len(s.Ranges) {
		t.Fatalf("ranges = %d, want %d", len(got.Ranges), len(s.Ranges))
	}
	if !got.UniqueIPs.Equals(s.UniqueIPs) {
		t.Fatalf("unique IPs = %s, want %s", got.UniqueIPs, s.UniqueIPs)
	}
}

func TestBinary6Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteBinary6(&buf, New6("empty")); err != nil {
		t.Fatalf("WriteBinary6: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty set should write 0 bytes, got %d", buf.Len())
	}
}
