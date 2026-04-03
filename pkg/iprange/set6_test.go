package iprange

import "testing"

func makeTestSet6(ranges ...Range6) *IPSet6 {
	s := New6("test")
	for _, r := range ranges {
		if err := s.AddRange6(r); err != nil {
			panic(err)
		}
	}
	return s
}

func TestIPSet6Optimize(t *testing.T) {
	s := makeTestSet6(
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(20)},
		Range6{Lo: u128FromUint64(5), Hi: u128FromUint64(8)},
		Range6{Lo: u128FromUint64(15), Hi: u128FromUint64(25)},
	)
	s.Optimize()

	if len(s.Ranges) != 2 {
		t.Fatalf("ranges = %d, want 2", len(s.Ranges))
	}
	if !s.UniqueIPs.Equals(u128FromUint64(20)) {
		t.Fatalf("unique IPs = %s, want 20", s.UniqueIPs)
	}
}

func TestIPSet6Contains(t *testing.T) {
	s := makeTestSet6(
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(20)},
		Range6{Lo: u128FromUint64(50), Hi: u128FromUint64(60)},
	)
	s.Optimize()

	if !s.Contains(u128FromUint64(15)) {
		t.Fatal("expected 15 to be contained")
	}
	if s.Contains(u128FromUint64(21)) {
		t.Fatal("did not expect 21 to be contained")
	}
}

func TestIPSet6Merge(t *testing.T) {
	a := makeTestSet6(Range6{Lo: u128FromUint64(1), Hi: u128FromUint64(10)})
	b := makeTestSet6(Range6{Lo: u128FromUint64(20), Hi: u128FromUint64(30)})
	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	a.Optimize()
	if len(a.Ranges) != 2 {
		t.Fatalf("ranges = %d, want 2", len(a.Ranges))
	}
}

func TestIPSet6Family(t *testing.T) {
	if New6("x").Family() != FamilyIPv6 {
		t.Fatal("expected IPv6 family")
	}
}
