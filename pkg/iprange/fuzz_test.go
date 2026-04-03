package iprange

import (
	"bytes"
	"testing"
)

func FuzzParseReader(f *testing.F) {
	f.Add([]byte("1.2.3.4\n"))
	f.Add([]byte("10.0.0.0/24\n"))
	f.Add([]byte("192.0.2.1 - 192.0.2.9\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		set, err := ParseReader(t.Context(), "fuzz", bytes.NewReader(data), DefaultParseOptions())
		if err != nil {
			return
		}
		set.Optimize()
		var out bytes.Buffer
		if err := WriteBinary(&out, set); err != nil {
			t.Fatalf("write parsed set: %v", err)
		}
		roundTrip, err := ReadBinary("fuzz", bytes.NewReader(out.Bytes()))
		if err != nil {
			t.Fatalf("read parsed set round-trip: %v", err)
		}
		if !sameIPSetMembership(set, roundTrip) {
			t.Fatalf("parsed set changed after binary round-trip")
		}
	})
}

func FuzzReadBinary(f *testing.F) {
	var sample bytes.Buffer
	set := newOptimizedSet("binary", Range{Lo: 1, Hi: 1})
	if err := WriteBinary(&sample, set); err != nil {
		f.Fatalf("seed write failed: %v", err)
	}
	f.Add(sample.Bytes())
	f.Add([]byte(BinaryHeaderV10))

	f.Fuzz(func(t *testing.T, data []byte) {
		set, err := ReadBinary("binary", bytes.NewReader(data))
		if err != nil {
			return
		}
		var out bytes.Buffer
		if err := WriteBinary(&out, set); err != nil {
			t.Fatalf("write binary set: %v", err)
		}
		roundTrip, err := ReadBinary("binary", bytes.NewReader(out.Bytes()))
		if err != nil {
			t.Fatalf("read binary set round-trip: %v", err)
		}
		if !sameIPSetMembership(set, roundTrip) {
			t.Fatalf("binary set changed after round-trip")
		}
	})
}

func sameIPSetMembership(left, right *IPSet) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.UniqueCount() != right.UniqueCount() {
		return false
	}
	for r := range left.Iter() {
		if !right.Contains(r.Lo) || !right.Contains(r.Hi) {
			return false
		}
	}
	for r := range right.Iter() {
		if !left.Contains(r.Lo) || !left.Contains(r.Hi) {
			return false
		}
	}
	return true
}
