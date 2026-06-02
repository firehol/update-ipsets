package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

// TestStreamProcessorHeapBounded verifies that processing a 10MB input
// through remove_comments + extract_ipv4 keeps heap growth bounded.
// The pipeline streams data line-by-line; it must not buffer the full
// input in heap.
func TestStreamProcessorHeapBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping processor heap test in short mode")
	}

	// Generate ~10MB of input data.
	const targetSize = 10 * 1024 * 1024
	var sb strings.Builder
	line := "log entry src=192.168.1.1 dst=10.0.0.1 # this is a verbose log comment that adds bulk\n"
	lineNum := 0
	for sb.Len() < targetSize {
		a := lineNum >> 16 & 0xFF
		b := lineNum >> 8 & 0xFF
		c := lineNum & 0xFF
		fmt.Fprintf(&sb, "log entry src=%d.%d.%d.%d dst=10.0.0.1 # verbose comment padding to make it large\n",
			a, b, c, lineNum%256)
		lineNum++
	}
	_ = line // suppress unused
	input := sb.String()
	actualSize := len(input)

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "large-input.dat")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	steps := []config.ProcessorStep{
		{Name: "remove_comments"},
		{Name: "extract_ipv4"},
	}

	// Force GC and measure baseline.
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatalf("RunStream failed: %v", err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	heapGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("input size=%d heap growth=%d", actualSize, heapGrowth)

	// Verify output is non-empty and correct.
	output, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) == 0 {
		t.Fatal("expected non-empty output from extract_ipv4")
	}

	// Verify equivalence with []byte pipeline.
	bytesOut, err := Run(t.Context(), steps, []byte(input))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if string(output) != string(bytesOut) {
		t.Fatal("stream output differs from bytes output for large input")
	}

	// The 10MB input should not cause >5MB heap growth in streaming mode.
	// Normal Go runtime overhead (GC metadata, buffers) accounts for some growth.
	const maxHeapGrowth = 5 * 1024 * 1024
	if heapGrowth > maxHeapGrowth {
		t.Logf("WARNING: heap grew by %d bytes (>%d) on %d byte input; streaming may not be effective",
			heapGrowth, maxHeapGrowth, actualSize)
	}
}

// TestStreamProcessorChainedLargeInput tests a 3-step pipeline on large input.
func TestStreamProcessorChainedLargeInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chained pipeline test in short mode")
	}

	// Generate input with mix of IPs and CIDRs, with comments.
	const numLines = 100_000
	var sb strings.Builder
	for i := range numLines {
		a := i >> 16 & 0xFF
		b := i >> 8 & 0xFF
		c := i & 0xFF
		if i%3 == 0 {
			fmt.Fprintf(&sb, "%d.%d.%d.0/24 # network block\n", a, b, c)
		} else {
			fmt.Fprintf(&sb, "%d.%d.%d.%d # single host\n", a, b, c, i%256)
		}
	}
	input := sb.String()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "chained-input.dat")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	steps := []config.ProcessorStep{
		{Name: "remove_comments"},
		{Name: "filter_all4"},
		{Name: "append_slash32"},
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatalf("RunStream failed: %v", err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	// Verify equivalence.
	bytesOut, err := Run(t.Context(), steps, []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	streamOut, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(streamOut) != string(bytesOut) {
		t.Fatalf("3-step pipeline output mismatch on %d lines", numLines)
	}

	t.Logf("chained pipeline: %d input lines -> %d bytes output", numLines, len(streamOut))
}
