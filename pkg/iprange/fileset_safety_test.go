package iprange

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// --- Reject non-optimized headers ---

// writeNonOptimizedBinary writes a binary file with "non-optimized" in the
// header, simulating a file that was written without optimization.
func writeNonOptimizedBinary(t *testing.T, ranges []Range) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nonopt.set")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	bw := bufio.NewWriter(f)
	_, _ = fmt.Fprint(bw, BinaryHeaderV10)
	_, _ = fmt.Fprintln(bw, "non-optimized")
	_, _ = fmt.Fprintf(bw, "record size %d\n", 8)
	_, _ = fmt.Fprintf(bw, "records %d\n", len(ranges))
	_, _ = fmt.Fprintf(bw, "bytes %d\n", len(ranges)*8+4)
	_, _ = fmt.Fprintf(bw, "lines %d\n", len(ranges))
	var uniqueIPs uint64
	for _, r := range ranges {
		uniqueIPs += r.Size()
	}
	_, _ = fmt.Fprintf(bw, "unique ips %d\n", uniqueIPs)
	_ = binary.Write(bw, nativeEndian, binaryEndiannessMarker)
	for _, r := range ranges {
		_ = binary.Write(bw, nativeEndian, r.Lo)
		_ = binary.Write(bw, nativeEndian, r.Hi)
	}
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileSetRejectsNonOptimized(t *testing.T) {
	// A valid binary file with non-optimized header must be rejected.
	path := writeNonOptimizedBinary(t, []Range{
		{Lo: 500, Hi: 600},
		{Lo: 100, Hi: 200}, // deliberately unsorted
	})

	_, err := OpenFileSet(path)
	if err == nil {
		t.Fatal("expected error for non-optimized binary file")
	}
	if !errors.Is(err, ErrNotOptimized) {
		t.Fatalf("expected ErrNotOptimized, got: %v", err)
	}
}

func TestFileSetRejectsNonOptimizedSorted(t *testing.T) {
	// Even if the ranges happen to be sorted, the non-optimized flag
	// means we can't trust them — reject.
	path := writeNonOptimizedBinary(t, []Range{
		{Lo: 100, Hi: 200},
		{Lo: 300, Hi: 400},
	})

	_, err := OpenFileSet(path)
	if err == nil {
		t.Fatal("expected error for non-optimized binary file (even if sorted)")
	}
	if !errors.Is(err, ErrNotOptimized) {
		t.Fatalf("expected ErrNotOptimized, got: %v", err)
	}
}

// --- Use-after-close safety ---

func TestFileSetUseAfterClose(t *testing.T) {
	set := newOptimizedSet("uac",
		Range{Lo: 100, Hi: 200},
		Range{Lo: 300, Hi: 400},
	)
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := fs.Close(); err != nil {
		t.Fatal(err)
	}

	// Range should return ErrFileSetClosed.
	_, err = fs.Range(0)
	if !errors.Is(err, ErrFileSetClosed) {
		t.Fatalf("Range after close: expected ErrFileSetClosed, got %v", err)
	}

	// Contains should return false (not panic).
	if fs.Contains(150) {
		t.Fatal("Contains after close should return false")
	}

	// Iter should yield nothing (not panic).
	count := 0
	for range fs.Iter() {
		count++
	}
	if count != 0 {
		t.Fatalf("Iter after close yielded %d ranges, want 0", count)
	}

	// Err should return ErrFileSetClosed.
	if !errors.Is(fs.Err(), ErrFileSetClosed) {
		t.Fatalf("Err after close: expected ErrFileSetClosed, got %v", fs.Err())
	}

	// Len still returns the count (metadata, not a read operation).
	if fs.Len() != 2 {
		t.Fatalf("Len after close: got %d, want 2", fs.Len())
	}
}

// --- Concurrent close/read must not panic (run with -race) ---

func TestFileSetConcurrentCloseRead(t *testing.T) {
	set := newOptimizedSet("concurrent",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 30, Hi: 40},
		Range{Lo: 50, Hi: 60},
	)
	path := writeTempSet(t, set)

	// Run multiple rounds to increase the chance of hitting the race window.
	for round := 0; round < 10; round++ {
		fs, err := OpenFileSet(path)
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})

		// Readers: hammer Contains and Range concurrently.
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(seed uint64) {
				defer wg.Done()
				<-start
				rng := rand.New(rand.NewPCG(seed, 0))
				for j := 0; j < 2000; j++ {
					fs.Contains(rng.Uint32N(100))
					_, _ = fs.Range(int(rng.Uint32N(3)))
				}
			}(uint64(round*100 + i))
		}

		// Iterator goroutines.
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for j := 0; j < 200; j++ {
					for range fs.Iter() {
					}
				}
			}()
		}

		// Closer: close mid-flight.
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			runtime.Gosched()
			_ = fs.Close()
		}()

		// Release all goroutines at once for maximum contention.
		close(start)
		wg.Wait()
	}
	// If we get here without a panic or race detector complaint, the test passes.
}

// --- Err() method ---

func TestFileSetErrNilBeforeClose(t *testing.T) {
	set := newOptimizedSet("errcheck", Range{Lo: 1, Hi: 100})
	path := writeTempSet(t, set)

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	// Normal operations should leave Err() nil.
	fs.Contains(50)
	for range fs.Iter() {
	}
	if fs.Err() != nil {
		t.Fatalf("Err: got %v, want nil", fs.Err())
	}
}

// --- Empty set Err() and close behavior ---

func TestFileSetEmptySetErr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.set")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}

	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}

	if fs.Err() != nil {
		t.Fatalf("Err before close on empty set: %v", fs.Err())
	}
	_ = fs.Close()
	// emptyFileSet.Err() returns ErrFileSetClosed after Close.
	if !errors.Is(fs.Err(), ErrFileSetClosed) {
		t.Fatalf("Err after close on empty set: got %v, want ErrFileSetClosed", fs.Err())
	}
}

// --- Invalid ranges (Lo > Hi) in payload ---

func writeInvalidRangesBinary(t *testing.T) string {
	t.Helper()
	ranges := []Range{
		{Lo: 100, Hi: 200},
		{Lo: 500, Hi: 300}, // invalid: Lo > Hi
	}
	path := filepath.Join(t.TempDir(), "invalid_ranges.set")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	bw := bufio.NewWriter(f)
	_, _ = fmt.Fprint(bw, BinaryHeaderV10)
	_, _ = fmt.Fprintln(bw, "optimized")
	_, _ = fmt.Fprintf(bw, "record size %d\n", 8)
	_, _ = fmt.Fprintf(bw, "records %d\n", len(ranges))
	_, _ = fmt.Fprintf(bw, "bytes %d\n", len(ranges)*8+4)
	_, _ = fmt.Fprintf(bw, "lines %d\n", len(ranges))
	// uniqueIPs >= records (fake it to pass header validation)
	_, _ = fmt.Fprintf(bw, "unique ips %d\n", 500)
	_ = binary.Write(bw, nativeEndian, binaryEndiannessMarker)
	for _, r := range ranges {
		_ = binary.Write(bw, nativeEndian, r.Lo)
		_ = binary.Write(bw, nativeEndian, r.Hi)
	}
	_ = bw.Flush()
	return path
}

func TestFileSetInvalidRangesInPayload(t *testing.T) {
	// A binary file with Lo > Hi ranges is now rejected during
	// OpenFileSet — payload validation catches the invalid range.
	path := writeInvalidRangesBinary(t)

	_, err := OpenFileSet(path)
	if err == nil {
		t.Fatal("expected error for file with Lo > Hi range")
	}
	if !strings.Contains(err.Error(), "Lo") || !strings.Contains(err.Error(), "Hi") {
		t.Fatalf("error should mention Lo > Hi, got: %v", err)
	}
}

// --- Payload validation: unsorted ranges ---

func writeUnsortedRangesBinary(t *testing.T) string {
	t.Helper()
	ranges := []Range{
		{Lo: 300, Hi: 400},
		{Lo: 100, Hi: 200}, // unsorted: appears after a higher range
	}
	path := filepath.Join(t.TempDir(), "unsorted_ranges.set")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	bw := bufio.NewWriter(f)
	_, _ = fmt.Fprint(bw, BinaryHeaderV10)
	_, _ = fmt.Fprintln(bw, "optimized")
	_, _ = fmt.Fprintf(bw, "record size %d\n", 8)
	_, _ = fmt.Fprintf(bw, "records %d\n", len(ranges))
	_, _ = fmt.Fprintf(bw, "bytes %d\n", len(ranges)*8+4)
	_, _ = fmt.Fprintf(bw, "lines %d\n", len(ranges))
	_, _ = fmt.Fprintf(bw, "unique ips %d\n", 500)
	_ = binary.Write(bw, nativeEndian, binaryEndiannessMarker)
	for _, r := range ranges {
		_ = binary.Write(bw, nativeEndian, r.Lo)
		_ = binary.Write(bw, nativeEndian, r.Hi)
	}
	_ = bw.Flush()
	return path
}

func TestFileSetRejectsUnsortedPayload(t *testing.T) {
	path := writeUnsortedRangesBinary(t)

	_, err := OpenFileSet(path)
	if err == nil {
		t.Fatal("expected error for file with unsorted ranges")
	}
	if !strings.Contains(err.Error(), "not sorted") {
		t.Fatalf("error should mention sorting, got: %v", err)
	}
}

// --- Payload validation: overlapping ranges ---

func writeOverlappingRangesBinary(t *testing.T) string {
	t.Helper()
	ranges := []Range{
		{Lo: 100, Hi: 300},
		{Lo: 200, Hi: 400}, // overlaps with previous
	}
	path := filepath.Join(t.TempDir(), "overlapping_ranges.set")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	bw := bufio.NewWriter(f)
	_, _ = fmt.Fprint(bw, BinaryHeaderV10)
	_, _ = fmt.Fprintln(bw, "optimized")
	_, _ = fmt.Fprintf(bw, "record size %d\n", 8)
	_, _ = fmt.Fprintf(bw, "records %d\n", len(ranges))
	_, _ = fmt.Fprintf(bw, "bytes %d\n", len(ranges)*8+4)
	_, _ = fmt.Fprintf(bw, "lines %d\n", len(ranges))
	_, _ = fmt.Fprintf(bw, "unique ips %d\n", 500)
	_ = binary.Write(bw, nativeEndian, binaryEndiannessMarker)
	for _, r := range ranges {
		_ = binary.Write(bw, nativeEndian, r.Lo)
		_ = binary.Write(bw, nativeEndian, r.Hi)
	}
	_ = bw.Flush()
	return path
}

func TestFileSetRejectsOverlappingPayload(t *testing.T) {
	path := writeOverlappingRangesBinary(t)

	_, err := OpenFileSet(path)
	if err == nil {
		t.Fatal("expected error for file with overlapping ranges")
	}
	if !strings.Contains(err.Error(), "not sorted") {
		t.Fatalf("error should mention overlap, got: %v", err)
	}
}

// --- Pread backend direct test ---

func TestFileSetPreadBackendRoundTrip(t *testing.T) {
	// Exercise the pread backend directly to verify it produces the same
	// results as the default (mmap) backend.
	set := newOptimizedSet("pread",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 30, Hi: 40},
		Range{Lo: 50, Hi: 60},
	)
	path := writeTempSet(t, set)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	hdr, err := parseBinaryHeader(f)
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}

	fs, err := openFileSetPread(f, path, fi.Size(), hdr, FileSetOpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	if fs.Len() != 3 {
		t.Fatalf("pread Len: got %d, want 3", fs.Len())
	}
	for i, want := range set.Ranges {
		got, err := fs.Range(i)
		if err != nil {
			t.Fatalf("pread Range(%d): %v", i, err)
		}
		if got != want {
			t.Fatalf("pread Range(%d): got %v, want %v", i, got, want)
		}
	}
	if !fs.Contains(15) {
		t.Fatal("pread should contain 15")
	}
	if fs.Contains(25) {
		t.Fatal("pread should not contain 25")
	}

	var collected []Range
	for r := range fs.Iter() {
		collected = append(collected, r)
	}
	if len(collected) != 3 {
		t.Fatalf("pread iter: got %d ranges, want 3", len(collected))
	}
	if fs.Err() != nil {
		t.Fatalf("pread Err: %v", fs.Err())
	}
}

// --- Pread backend concurrent close/read ---

func TestFileSetPreadConcurrentCloseRead(t *testing.T) {
	set := newOptimizedSet("pread_conc",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 30, Hi: 40},
		Range{Lo: 50, Hi: 60},
	)
	path := writeTempSet(t, set)

	for round := 0; round < 5; round++ {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		fi, err := f.Stat()
		if err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
		hdr, err := parseBinaryHeader(f)
		if err != nil {
			_ = f.Close()
			t.Fatal(err)
		}

		fs, err := openFileSetPread(f, path, fi.Size(), hdr, FileSetOpenOptions{})
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})

		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func(seed uint64) {
				defer wg.Done()
				<-start
				rng := rand.New(rand.NewPCG(seed, 0))
				for j := 0; j < 1000; j++ {
					fs.Contains(rng.Uint32N(100))
					_, _ = fs.Range(int(rng.Uint32N(3)))
				}
			}(uint64(round*100 + i))
		}

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for j := 0; j < 100; j++ {
					for range fs.Iter() {
					}
				}
			}()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			runtime.Gosched()
			_ = fs.Close()
		}()

		close(start)
		wg.Wait()
	}
}
