package iprange

import (
	"bytes"
	"testing"
)

func TestBinaryRoundTrip(t *testing.T) {
	set, err := ParseReader(t.Context(), "roundtrip", bytes.NewBufferString("10.0.0.0/24\n10.0.1.1\n"), DefaultParseOptions())
	if err != nil {
		t.Fatal(err)
	}
	set.Optimize()

	var buf bytes.Buffer
	if err := WriteBinary(&buf, set); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadBinary("roundtrip", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	loaded.Optimize()

	expectRanges(t, loaded, set.Ranges)
	if loaded.UniqueIPs != set.UniqueIPs {
		t.Fatalf("unique IP mismatch: got %d want %d", loaded.UniqueIPs, set.UniqueIPs)
	}
}

func TestReadBinaryRejectsTrailingBytes(t *testing.T) {
	set := newOptimizedSet("binary", Range{Lo: 1, Hi: 2})
	var buf bytes.Buffer
	if err := WriteBinary(&buf, set); err != nil {
		t.Fatal(err)
	}
	buf.WriteByte(0xff)

	if _, err := ReadBinary("binary", bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected trailing data error")
	}
}
