package iprange

import (
	"bytes"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeTempSet writes an IPSet to a temporary .set file and returns its path.
func writeTempSet(t *testing.T, set *IPSet) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.set")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBinary(f, set); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileSetRoundTrip(t *testing.T) {
	set := newOptimizedSet("rt",
		Range{Lo: 100, Hi: 200},
		Range{Lo: 300, Hi: 400},
		Range{Lo: 1000, Hi: 2000},
	)
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	if fs.Len() != len(set.Ranges) {
		t.Fatalf("Len: got %d, want %d", fs.Len(), len(set.Ranges))
	}
	if fs.UniqueIPs() != set.UniqueIPs {
		t.Fatalf("UniqueIPs: got %d, want %d", fs.UniqueIPs(), set.UniqueIPs)
	}

	for i, want := range set.Ranges {
		got, err := fs.Range(i)
		if err != nil {
			t.Fatalf("Range(%d): %v", i, err)
		}
		if got != want {
			t.Fatalf("Range(%d): got %v, want %v", i, got, want)
		}
	}
}

func TestReadFileSetMetadata(t *testing.T) {
	set := newOptimizedSet("metadata",
		Range{Lo: 100, Hi: 200},
		Range{Lo: 300, Hi: 400},
		Range{Lo: 1000, Hi: 2000},
	)
	path := writeTempSet(t, set)

	meta, err := ReadFileSetMetadata(path)
	if err != nil {
		t.Fatalf("ReadFileSetMetadata() error = %v", err)
	}
	if !meta.Optimized {
		t.Fatal("metadata should report optimized payload")
	}
	if meta.RecordSize != 8 {
		t.Fatalf("RecordSize = %d, want 8", meta.RecordSize)
	}
	if meta.Records != len(set.Ranges) {
		t.Fatalf("Records = %d, want %d", meta.Records, len(set.Ranges))
	}
	if meta.PayloadBytes != len(set.Ranges)*8+4 {
		t.Fatalf("PayloadBytes = %d, want %d", meta.PayloadBytes, len(set.Ranges)*8+4)
	}
	if meta.Lines != set.Lines {
		t.Fatalf("Lines = %d, want %d", meta.Lines, set.Lines)
	}
	if meta.UniqueIPs != set.UniqueIPs {
		t.Fatalf("UniqueIPs = %d, want %d", meta.UniqueIPs, set.UniqueIPs)
	}

	emptyPath := filepath.Join(t.TempDir(), "empty.set")
	if err := os.WriteFile(emptyPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	emptyMeta, err := ReadFileSetMetadata(emptyPath)
	if err != nil {
		t.Fatalf("ReadFileSetMetadata(empty) error = %v", err)
	}
	if emptyMeta.Records != 0 || emptyMeta.UniqueIPs != 0 || emptyMeta.PayloadBytes != 0 {
		t.Fatalf("empty metadata = %+v, want zero counters", emptyMeta)
	}
}

func TestFileSetHeaderOpenAllocationShape(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("race detector instrumentation changes allocation counts")
	}

	set := newOptimizedSet("metadata-allocs",
		Range{Lo: 100, Hi: 200},
		Range{Lo: 300, Hi: 400},
		Range{Lo: 1000, Hi: 2000},
	)
	path := writeTempSet(t, set)

	metadataAllocs := testing.AllocsPerRun(20, func() {
		if _, err := ReadFileSetMetadata(path); err != nil {
			panic(err)
		}
	})
	if metadataAllocs > 9 {
		t.Fatalf("ReadFileSetMetadata() allocations = %.0f, want <= 9", metadataAllocs)
	}

	trustedOpenAllocs := testing.AllocsPerRun(20, func() {
		fs, err := OpenFileSetWithOptions(path, FileSetOpenOptions{TrustOptimizedPayload: true})
		if err != nil {
			panic(err)
		}
		if err := fs.Close(); err != nil {
			panic(err)
		}
	})
	if trustedOpenAllocs > 10 {
		t.Fatalf("OpenFileSetWithOptions(trusted) allocations = %.0f, want <= 10", trustedOpenAllocs)
	}
}

func TestFileSetTrustedOpenSkipsSortedValidationOnlyWhenRequested(t *testing.T) {
	set := newOptimizedSet("trusted",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 30, Hi: 40},
		Range{Lo: 50, Hi: 60},
	)
	path := writeUnsortedBinaryPayload(t, writeTempSet(t, set))

	if _, err := OpenFileSet(path); err == nil {
		t.Fatal("strict OpenFileSet should reject unsorted optimized payload")
	}

	meta, err := ReadFileSetMetadata(path)
	if err != nil {
		t.Fatalf("ReadFileSetMetadata() should accept structurally valid payload metadata: %v", err)
	}
	if meta.Records != len(set.Ranges) || meta.UniqueIPs != set.UniqueIPs {
		t.Fatalf("metadata = %+v, want records=%d unique=%d", meta, len(set.Ranges), set.UniqueIPs)
	}

	fs, err := OpenFileSetWithOptions(path, FileSetOpenOptions{TrustOptimizedPayload: true})
	if err != nil {
		t.Fatalf("trusted OpenFileSetWithOptions() error = %v", err)
	}
	defer func() { _ = fs.Close() }()
	if fs.Len() != len(set.Ranges) {
		t.Fatalf("Len = %d, want %d", fs.Len(), len(set.Ranges))
	}
	if fs.UniqueIPs() != set.UniqueIPs {
		t.Fatalf("UniqueIPs = %d, want %d", fs.UniqueIPs(), set.UniqueIPs)
	}
	first, err := fs.Range(0)
	if err != nil {
		t.Fatalf("Range(0): %v", err)
	}
	if first != set.Ranges[1] {
		t.Fatalf("trusted open should expose payload as stored: first range = %v, want %v", first, set.Ranges[1])
	}
}

func TestFileSetContains(t *testing.T) {
	set := newOptimizedSet("contains",
		Range{Lo: 100, Hi: 120},
		Range{Lo: 200, Hi: 220},
		Range{Lo: 500, Hi: 600},
	)
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	// IPs that must be found
	for _, ip := range []uint32{100, 110, 120, 200, 210, 220, 500, 550, 600} {
		if !fs.Contains(ip) {
			t.Errorf("Contains(%d) = false, want true", ip)
		}
	}

	// IPs that must NOT be found
	for _, ip := range []uint32{0, 99, 121, 199, 221, 499, 601, 1000, ^uint32(0)} {
		if fs.Contains(ip) {
			t.Errorf("Contains(%d) = true, want false", ip)
		}
	}

	// Cross-check against in-memory Contains
	rng := rand.New(rand.NewPCG(42, 0))
	for i := 0; i < 10000; i++ {
		ip := rng.Uint32()
		if fs.Contains(ip) != set.Contains(ip) {
			t.Fatalf("Contains(%d): fileset=%v inmemory=%v", ip, fs.Contains(ip), set.Contains(ip))
		}
	}
}

func TestFileSetContainsSingleRange(t *testing.T) {
	set := newOptimizedSet("single", Range{Lo: 42, Hi: 42})
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	if fs.Len() != 1 {
		t.Fatalf("Len: got %d, want 1", fs.Len())
	}
	if !fs.Contains(42) {
		t.Fatal("should contain 42")
	}
	if fs.Contains(41) || fs.Contains(43) {
		t.Fatal("should not contain 41 or 43")
	}
}

func TestFileSetIter(t *testing.T) {
	set := newOptimizedSet("iter",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 30, Hi: 40},
		Range{Lo: 50, Hi: 60},
		Range{Lo: 70, Hi: 80},
	)
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	var collected []Range
	for r := range fs.Iter() {
		collected = append(collected, r)
	}

	if len(collected) != len(set.Ranges) {
		t.Fatalf("iterator returned %d ranges, want %d", len(collected), len(set.Ranges))
	}
	for i, got := range collected {
		if got != set.Ranges[i] {
			t.Fatalf("iter range %d: got %v, want %v", i, got, set.Ranges[i])
		}
	}
}

func TestFileSetIterEarlyBreak(t *testing.T) {
	set := newOptimizedSet("earlybreak",
		Range{Lo: 1, Hi: 1},
		Range{Lo: 2, Hi: 2},
		Range{Lo: 3, Hi: 3},
	)
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	count := 0
	for range fs.Iter() {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Fatalf("expected early break after 1 iteration, got %d", count)
	}
}

func TestFileSetRangeOutOfBounds(t *testing.T) {
	set := newOptimizedSet("oob", Range{Lo: 1, Hi: 10})
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	if _, err := fs.Range(-1); err == nil {
		t.Fatal("expected error for negative index")
	}
	if _, err := fs.Range(1); err == nil {
		t.Fatal("expected error for index == Len()")
	}
	if _, err := fs.Range(100); err == nil {
		t.Fatal("expected error for index >> Len()")
	}
}

func TestFileSetLargeRoundTrip(t *testing.T) {
	// 100K ranges — verify the full round trip without loading into memory.
	const n = 100_000
	set := New("large")
	for i := 0; i < n; i++ {
		lo := uint32(i * 10)
		hi := lo + 5
		if err := set.Add(lo, hi); err != nil {
			t.Fatal(err)
		}
	}
	set.Optimize()
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	if fs.Len() != len(set.Ranges) {
		t.Fatalf("Len: got %d, want %d", fs.Len(), len(set.Ranges))
	}
	if fs.UniqueIPs() != set.UniqueIPs {
		t.Fatalf("UniqueIPs: got %d, want %d", fs.UniqueIPs(), set.UniqueIPs)
	}

	// Spot-check some ranges
	for _, i := range []int{0, 1, n/2 - 1, n / 2, n - 1} {
		if i >= fs.Len() {
			continue
		}
		got, err := fs.Range(i)
		if err != nil {
			t.Fatalf("Range(%d): %v", i, err)
		}
		if got != set.Ranges[i] {
			t.Fatalf("Range(%d): got %v, want %v", i, got, set.Ranges[i])
		}
	}

	// Spot-check Contains
	rng := rand.New(rand.NewPCG(99, 0))
	for i := 0; i < 1000; i++ {
		ip := rng.Uint32N(uint32(n * 10))
		if fs.Contains(ip) != set.Contains(ip) {
			t.Fatalf("Contains(%d): fileset=%v inmemory=%v", ip, fs.Contains(ip), set.Contains(ip))
		}
	}
}

func TestFileSetCorruptedTruncated(t *testing.T) {
	set := newOptimizedSet("trunc", Range{Lo: 1, Hi: 100})
	path := writeTempSet(t, set)

	// Read the file, truncate it by chopping off the last 4 bytes.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	truncPath := filepath.Join(t.TempDir(), "truncated.set")
	if err := os.WriteFile(truncPath, data[:len(data)-4], 0600); err != nil {
		t.Fatal(err)
	}

	_, err = OpenFileSet(truncPath)
	if err == nil {
		t.Fatal("expected error for truncated file")
	}
	if _, err = ReadFileSetMetadata(truncPath); err == nil {
		t.Fatal("expected metadata error for truncated file")
	}
}

func TestFileSetCorruptedBadMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad_magic.set")
	if err := os.WriteFile(path, []byte("not a valid header\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := OpenFileSet(path)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestFileSetCorruptedWrongEndianness(t *testing.T) {
	set := newOptimizedSet("endian", Range{Lo: 1, Hi: 10})
	path := writeTempSet(t, set)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Find the endianness marker (0x1A2B3C4D) and flip it.
	// The marker is the first 4 bytes of binary payload after the text header.
	marker := make([]byte, 4)
	nativeEndian.PutUint32(marker, binaryEndiannessMarker)
	idx := bytes.Index(data, marker)
	if idx < 0 {
		t.Fatal("could not find endianness marker in binary data")
	}
	// Reverse the marker bytes
	data[idx], data[idx+3] = data[idx+3], data[idx]
	data[idx+1], data[idx+2] = data[idx+2], data[idx+1]

	flipPath := filepath.Join(t.TempDir(), "flipped_endian.set")
	if err := os.WriteFile(flipPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = OpenFileSet(flipPath)
	if err == nil {
		t.Fatal("expected error for wrong endianness")
	}
	if _, err = ReadFileSetMetadata(flipPath); err == nil {
		t.Fatal("expected metadata error for wrong endianness")
	}
}

func TestFileSetCorruptedZeroLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.set")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}

	// Zero-byte files are treated as empty sets (not errors).
	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatalf("unexpected error for zero-length file: %v", err)
	}
	defer func() { _ = fs.Close() }()

	if fs.Len() != 0 {
		t.Fatalf("Len: got %d, want 0", fs.Len())
	}
	if fs.UniqueIPs() != 0 {
		t.Fatalf("UniqueIPs: got %d, want 0", fs.UniqueIPs())
	}
	if fs.Contains(42) {
		t.Fatal("empty set should not contain anything")
	}
	count := 0
	for range fs.Iter() {
		count++
	}
	if count != 0 {
		t.Fatalf("iter returned %d ranges, want 0", count)
	}
}

func TestFileSetEmptySet(t *testing.T) {
	// WriteBinary writes nothing for empty sets, producing a zero-byte file.
	// OpenFileSet treats zero-byte files as valid empty sets.
	set := newOptimizedSet("empty")
	path := filepath.Join(t.TempDir(), "empty.set")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBinary(f, set); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 0 {
		t.Fatal("unexpected: WriteBinary wrote data for empty set")
	}

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatalf("unexpected error for empty set file: %v", err)
	}
	defer func() { _ = fs.Close() }()

	if fs.Len() != 0 {
		t.Fatalf("Len: got %d, want 0", fs.Len())
	}
	if fs.Contains(1) {
		t.Fatal("empty set should not contain anything")
	}
	if fs.Err() != nil {
		t.Fatalf("unexpected Err: %v", fs.Err())
	}
}

func TestFileSetCloseIdempotent(t *testing.T) {
	set := newOptimizedSet("close", Range{Lo: 1, Hi: 1})
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := fs.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic or return an unexpected error.
	if err := fs.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestFileSetMemoryBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	// Create a 1M-range set = 8MB of range data on disk.
	const numRanges = 1_000_000
	set := New("memtest")
	for i := 0; i < numRanges; i++ {
		lo := uint32(i * 4)
		hi := lo + 1
		if err := set.Add(lo, hi); err != nil {
			t.Fatal(err)
		}
	}
	set.Optimize()
	path := writeTempSet(t, set)

	// Force GC and take baseline heap measurement.
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	// Perform lookups and iteration without materializing all ranges.
	rng := rand.New(rand.NewPCG(77, 0))
	for i := 0; i < 10000; i++ {
		ip := rng.Uint32N(uint32(numRanges * 4))
		fs.Contains(ip)
	}

	// Iterate all ranges but don't store them.
	count := 0
	for range fs.Iter() {
		count++
	}
	if count != fs.Len() {
		t.Fatalf("iter count %d != Len() %d", count, fs.Len())
	}

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// The 1M ranges would be 8MB in a []Range slice. With mmap, Go heap
	// allocation should be much less. Allow up to 2MB of heap growth to
	// account for GC noise, test framework overhead, etc.
	const maxHeapGrowth = 2 * 1024 * 1024
	heapGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("heap before=%d after=%d growth=%d (max allowed=%d)",
		before.HeapAlloc, after.HeapAlloc, heapGrowth, maxHeapGrowth)

	if heapGrowth > maxHeapGrowth {
		t.Fatalf("heap grew by %d bytes (>%d), data was likely loaded into memory",
			heapGrowth, maxHeapGrowth)
	}
}

func TestFileSetCorruptedExtraBytes(t *testing.T) {
	set := newOptimizedSet("extra", Range{Lo: 1, Hi: 10})
	path := writeTempSet(t, set)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Append extra garbage bytes.
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF)
	extraPath := filepath.Join(t.TempDir(), "extra.set")
	if err := os.WriteFile(extraPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = OpenFileSet(extraPath)
	if err == nil {
		t.Fatal("expected error for file with extra trailing bytes")
	}
}

func writeUnsortedBinaryPayload(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var marker [4]byte
	nativeEndian.PutUint32(marker[:], binaryEndiannessMarker)
	idx := bytes.Index(data, marker[:])
	if idx < 0 {
		t.Fatal("could not find endianness marker in binary data")
	}
	firstRange := idx + len(marker)
	if len(data) < firstRange+16 {
		t.Fatalf("binary payload too short to swap ranges: %d bytes", len(data)-firstRange)
	}
	var tmp [8]byte
	copy(tmp[:], data[firstRange:firstRange+8])
	copy(data[firstRange:firstRange+8], data[firstRange+8:firstRange+16])
	copy(data[firstRange+8:firstRange+16], tmp[:])

	out := filepath.Join(t.TempDir(), "unsorted.set")
	if err := os.WriteFile(out, data, 0600); err != nil {
		t.Fatal(err)
	}
	return out
}
