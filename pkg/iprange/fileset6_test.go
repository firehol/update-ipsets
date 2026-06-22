package iprange

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestBinary6File(t *testing.T, s *IPSet6) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test6.set")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := WriteBinary6(f, s); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileSet6OpenAndContains(t *testing.T) {
	s := makeTestSet6(
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(20)},
		Range6{Lo: u128FromHiLo(0x20010db800000000, 0), Hi: u128FromHiLo(0x20010db800000000, 0xffff)},
	)
	s.Optimize()

	fs, err := OpenFileSet6(writeTestBinary6File(t, s))
	if err != nil {
		t.Fatalf("OpenFileSet6: %v", err)
	}
	defer func() { _ = fs.Close() }()

	if !fs.Contains(u128FromUint64(15)) {
		t.Fatal("expected 15 to be contained")
	}
	if !fs.Contains(u128FromHiLo(0x20010db800000000, 1)) {
		t.Fatal("expected 2001:db8::1 to be contained")
	}
	if fs.Contains(u128FromUint64(25)) {
		t.Fatal("did not expect 25 to be contained")
	}
}

func TestFileSet6UniqueIPsAndEmpty(t *testing.T) {
	s := makeTestSet6(
		Range6{Lo: u128FromUint64(1), Hi: u128FromUint64(10)},
		Range6{Lo: u128FromUint64(20), Hi: u128FromUint64(29)},
	)
	s.Optimize()
	fs, err := OpenFileSet6(writeTestBinary6File(t, s))
	if err != nil {
		t.Fatalf("OpenFileSet6: %v", err)
	}
	defer func() { _ = fs.Close() }()
	if !fs.UniqueIPs().Equals(u128FromUint64(20)) {
		t.Fatalf("unique IPs = %s, want 20", fs.UniqueIPs())
	}

	emptyPath := filepath.Join(t.TempDir(), "empty6.set")
	if err := os.WriteFile(emptyPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	empty, err := OpenFileSet6(emptyPath)
	if err != nil {
		t.Fatalf("OpenFileSet6 empty: %v", err)
	}
	defer func() { _ = empty.Close() }()
	if empty.Len() != 0 {
		t.Fatalf("Len = %d, want 0", empty.Len())
	}
}

func TestFileSet6OpenAllocationShape(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("race detector instrumentation changes allocation counts")
	}

	s := makeTestSet6(
		Range6{Lo: u128FromUint64(1), Hi: u128FromUint64(10)},
		Range6{Lo: u128FromHiLo(0x20010db800000000, 0), Hi: u128FromHiLo(0x20010db800000000, 0xffff)},
	)
	s.Optimize()
	path := writeTestBinary6File(t, s)

	allocs := testing.AllocsPerRun(20, func() {
		fs, err := OpenFileSet6(path)
		if err != nil {
			panic(err)
		}
		if err := fs.Close(); err != nil {
			panic(err)
		}
	})
	if allocs > 9 {
		t.Fatalf("OpenFileSet6() allocations = %.0f, want <= 9", allocs)
	}
}
