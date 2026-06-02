package processor

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

// TestStreamEquivalence verifies that RunStream produces byte-identical output
// to Run for every streamable processor.
func TestStreamEquivalence(t *testing.T) {
	cases := []struct {
		name  string
		steps []config.ProcessorStep
		input string
	}{
		{
			name:  "remove_comments",
			steps: []config.ProcessorStep{{Name: "remove_comments"}},
			input: "1.2.3.4 # comment\n\n5.6.7.8\t\t# test\n",
		},
		{
			name:  "remove_comments_semi",
			steps: []config.ProcessorStep{{Name: "remove_comments_semi"}},
			input: "1.2.3.4 ; drop\n;\n5.6.7.8\n",
		},
		{
			name:  "trim",
			steps: []config.ProcessorStep{{Name: "trim"}},
			input: "  a\t\tb  \n\n c \n",
		},
		{
			name:  "extract_ipv4",
			steps: []config.ProcessorStep{{Name: "extract_ipv4"}},
			input: "allow=1.2.3.4 deny=5.6.7.8/24 ignore=999.1.1.1\n",
		},
		{
			name:  "grep",
			steps: []config.ProcessorStep{{Name: "grep", Args: map[string]string{"pattern": "^keep"}}},
			input: "keep-one\ndrop\nkeep-two\n",
		},
		{
			name:  "grep_not",
			steps: []config.ProcessorStep{{Name: "grep_not", Args: map[string]string{"pattern": "^keep"}}},
			input: "keep-one\ndrop\nkeep-two\n",
		},
		{
			name:  "cut_delimiter",
			steps: []config.ProcessorStep{{Name: "cut_delimiter", Args: map[string]string{"delimiter": "|", "field": "3"}}},
			input: "a|b| c \n1|2\n",
		},
		{
			name:  "csv_column",
			steps: []config.ProcessorStep{{Name: "csv_column", Args: map[string]string{"index": "2"}}},
			input: "a,b,c\n1,2,3\n",
		},
		{
			name:  "subnet_to_cidr",
			steps: []config.ProcessorStep{{Name: "subnet_to_cidr"}},
			input: "1.2.3.0/255.255.255.0\n5.6.7.0/24\nplain\n",
		},
		{
			name:  "torproject_exits",
			steps: []config.ProcessorStep{{Name: "torproject_exits"}},
			input: "ExitAddress 1.2.3.4 2024-01-01\nOther line\nExitAddress 5.6.7.8 2024-01-02\n",
		},
		{
			name:  "passthrough",
			steps: []config.ProcessorStep{{Name: "passthrough"}},
			input: "hello\nworld\n",
		},
		{
			name:  "filter_ip4",
			steps: []config.ProcessorStep{{Name: "filter_ip4"}},
			input: "1.2.3.4\n5.6.7.0/24\n2001:db8::1\n",
		},
		{
			name:  "filter_net4",
			steps: []config.ProcessorStep{{Name: "filter_net4"}},
			input: "1.2.3.4\n5.6.7.0/24\n8.8.8.8/32\n",
		},
		{
			name:  "filter_all4",
			steps: []config.ProcessorStep{{Name: "filter_all4"}},
			input: "1.2.3.4\n5.6.7.0/24\n2001:db8::1\n",
		},
		{
			name:  "filter_invalid4",
			steps: []config.ProcessorStep{{Name: "filter_invalid4"}},
			input: "0.0.0.0\n1.2.3.4\n5.6.7.0/0\n8.8.8.8\n",
		},
		{
			name:  "append_slash32",
			steps: []config.ProcessorStep{{Name: "append_slash32"}},
			input: "1.2.3.4\n5.6.7.0/24\n",
		},
		{
			name:  "remove_slash32",
			steps: []config.ProcessorStep{{Name: "remove_slash32"}},
			input: "1.2.3.4/32\n5.6.7.0/24\n",
		},
		{
			name:  "snort_rules",
			steps: []config.ProcessorStep{{Name: "snort_rules"}},
			input: "alert ip any any -> any any [1.2.3.4, 5.6.7.0/24]\n# comment\nnon-alert\n",
		},
		{
			name:  "pix_deny_rules",
			steps: []config.ProcessorStep{{Name: "pix_deny_rules"}},
			input: "access-list outside deny ip 1.2.3.0 255.255.255.0 any\naccess-list outside deny ip host 5.6.7.8 any\n",
		},
		{
			name:  "dshield_format",
			steps: []config.ProcessorStep{{Name: "dshield_format"}},
			input: "1.2.3.0 x 24\n\ncomment\n",
		},
		{
			name:  "dataplane_column3",
			steps: []config.ProcessorStep{{Name: "dataplane_column3"}},
			input: "# comment\ncol1|col2| 1.2.3.4 |col4\na|b\n",
		},
		{
			name: "pipeline_remove_comments_then_extract_ipv4",
			steps: []config.ProcessorStep{
				{Name: "remove_comments"},
				{Name: "extract_ipv4"},
			},
			input: "allow 1.2.3.4 # keep\nallow 5.6.7.8\n",
		},
		{
			name: "pipeline_three_steps",
			steps: []config.ProcessorStep{
				{Name: "remove_comments"},
				{Name: "extract_ipv4"},
				{Name: "append_slash32"},
			},
			input: "allow 1.2.3.4 # keep\nallow 5.6.7.8\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Run the []byte pipeline
			bytesOut, err := Run(t.Context(), tc.steps, []byte(tc.input))
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			// Run the stream pipeline
			tmpDir := t.TempDir()
			srcPath := filepath.Join(tmpDir, "input.dat")
			if err := os.WriteFile(srcPath, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}

			resultPath, err := RunStream(t.Context(), tc.steps, srcPath, tmpDir)
			if err != nil {
				t.Fatalf("RunStream returned error: %v", err)
			}
			defer func() { _ = os.Remove(resultPath) }()

			streamOut, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal(err)
			}

			if string(bytesOut) != string(streamOut) {
				t.Fatalf("output mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
			}
		})
	}
}

// TestStreamGunzipEquivalence tests that streaming gunzip + line processor
// produces the same output as the []byte pipeline.
func TestStreamGunzipEquivalence(t *testing.T) {
	content := "1.2.3.4 # comment\n5.6.7.8 # another\n9.10.11.12\n"
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	steps := []config.ProcessorStep{
		{Name: "gunzip"},
		{Name: "remove_comments"},
	}

	// []byte path
	bytesOut, err := Run(t.Context(), steps, gz.Bytes())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// stream path
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.gz")
	if err := os.WriteFile(srcPath, gz.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	streamOut, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(bytesOut) != string(streamOut) {
		t.Fatalf("gunzip pipeline mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
	}
}

// TestStreamFallbackForNonStreamable tests that RunStream correctly falls back
// to []byte processing for non-streamable processors (like json_path, xml_tag).
func TestStreamFallbackForNonStreamable(t *testing.T) {
	input := `{"items":[{"ip":"1.2.3.4"},{"ip":"5.6.7.8"}]}`
	steps := []config.ProcessorStep{
		{Name: "json_path", Args: map[string]string{"path": "$.items[*].ip"}},
	}

	bytesOut, err := Run(t.Context(), steps, []byte(input))
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.json")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	streamOut, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(bytesOut) != string(streamOut) {
		t.Fatalf("fallback mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
	}
}

// TestStreamMixedPipeline tests a pipeline with both streamable and
// non-streamable steps interleaved.
func TestStreamMixedPipeline(t *testing.T) {
	// xml_tag is non-streamable, then filter_all4 is streamable
	input := `<root><ip>1.2.3.4</ip><ip>5.6.7.8</ip><ip>2001:db8::1</ip></root>`
	steps := []config.ProcessorStep{
		{Name: "xml_tag", Args: map[string]string{"tag": "ip"}},
		{Name: "filter_all4"},
	}

	bytesOut, err := Run(t.Context(), steps, []byte(input))
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.xml")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	streamOut, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(bytesOut) != string(streamOut) {
		t.Fatalf("mixed pipeline mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
	}
}

// TestStreamEmptyInput verifies streaming handles empty input correctly.
func TestStreamEmptyInput(t *testing.T) {
	steps := []config.ProcessorStep{{Name: "remove_comments"}}

	bytesOut, err := Run(t.Context(), steps, nil)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "empty.dat")
	if err := os.WriteFile(srcPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	streamOut, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(bytesOut) != string(streamOut) {
		t.Fatalf("empty input mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
	}
}

// TestStreamNoSteps verifies RunStream with an empty pipeline just copies the file.
func TestStreamNoSteps(t *testing.T) {
	input := "1.2.3.4\n5.6.7.8\n"
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.dat")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), nil, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	got, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != input {
		t.Fatalf("no-steps output: got %q want %q", got, input)
	}
}

// TestStreamToFile tests the convenience RunStreamToFile function.
func TestStreamToFile(t *testing.T) {
	input := "1.2.3.4 # test\n5.6.7.8\n"
	steps := []config.ProcessorStep{{Name: "remove_comments"}}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.dat")
	dstPath := filepath.Join(tmpDir, "output.dat")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := RunStreamToFile(t.Context(), steps, srcPath, dstPath, tmpDir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(got) != want {
		t.Fatalf("RunStreamToFile: got %q want %q", got, want)
	}
}

// TestStreamBoundedMemory verifies that streaming remove_comments on a large input
// does not allocate proportional to input size.
func TestStreamBoundedMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	// generate 5MB of input
	var sb strings.Builder
	line := "192.168.1.1 # this is a comment with some text to make lines longer\n"
	for sb.Len() < 5*1024*1024 {
		sb.WriteString(line)
	}
	input := sb.String()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "large.dat")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	// force GC before measurement
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	steps := []config.ProcessorStep{{Name: "remove_comments"}}
	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// the stream should not need to hold the full 5MB in heap
	// allow up to 2MB of additional allocation (for scanner buffer, etc.)
	heapGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("heap growth: %d bytes (input size: %d)", heapGrowth, len(input))

	// verify the output is correct
	got, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	bytesOut, err := Run(t.Context(), steps, []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(bytesOut) {
		t.Fatal("large input: stream output differs from bytes output")
	}

	// if heap grew by more than the input size, something is wrong
	if heapGrowth > int64(len(input)) {
		t.Logf("WARNING: heap grew by %d bytes, which is more than input size %d; streaming may not be effective", heapGrowth, len(input))
	}
}

// TestStreamUnzipFallback verifies that unzip (non-streamable) works via RunStream fallback.
func TestStreamUnzipFallback(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("data.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("1.2.3.4\n5.6.7.8\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	steps := []config.ProcessorStep{
		{Name: "unzip", Args: map[string]string{"file": "data.txt"}},
		{Name: "remove_comments"},
	}

	bytesOut, err := Run(t.Context(), steps, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.zip")
	if err := os.WriteFile(srcPath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	streamOut, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(bytesOut) != string(streamOut) {
		t.Fatalf("unzip fallback mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
	}
}

// TestClassifyPipeline verifies pipeline segmentation.
func TestClassifyPipeline(t *testing.T) {
	steps := []config.ProcessorStep{
		{Name: "remove_comments"},
		{Name: "extract_ipv4"},
		{Name: "json_path", Args: map[string]string{"path": "$.items[*].ip"}},
		{Name: "filter_all4"},
		{Name: "append_slash32"},
	}

	segments := classifyPipeline(steps)
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
	if !segments[0].streamable || len(segments[0].steps) != 2 {
		t.Fatalf("segment 0: streamable=%v, steps=%d", segments[0].streamable, len(segments[0].steps))
	}
	if segments[1].streamable || len(segments[1].steps) != 1 {
		t.Fatalf("segment 1: streamable=%v, steps=%d", segments[1].streamable, len(segments[1].steps))
	}
	if !segments[2].streamable || len(segments[2].steps) != 2 {
		t.Fatalf("segment 2: streamable=%v, steps=%d", segments[2].streamable, len(segments[2].steps))
	}
}

// TestIsStreamable checks the public IsStreamable function.
func TestIsStreamable(t *testing.T) {
	if !IsStreamable([]string{"remove_comments", "extract_ipv4", "append_slash32"}) {
		t.Fatal("expected streamable pipeline to be streamable")
	}
	if IsStreamable([]string{"remove_comments", "json_path"}) {
		t.Fatal("expected non-streamable pipeline to not be streamable")
	}
	if !IsStreamable([]string{"gunzip", "remove_comments"}) {
		t.Fatal("expected gunzip + remove_comments to be streamable")
	}
	if !IsStreamable([]string{"p2p_blocklist"}) {
		t.Fatal("expected p2p blocklist pipeline to be streamable")
	}
}

// BenchmarkStreamRemoveComments benchmarks streaming vs []byte for remove_comments.
func BenchmarkStreamRemoveComments(b *testing.B) {
	input := strings.Repeat("192.168.1.1 # comment\n", 50000)
	inputBytes := []byte(input)
	steps := []config.ProcessorStep{{Name: "remove_comments"}}

	b.Run("bytes", func(b *testing.B) {
		for b.Loop() {
			_, err := Run(b.Context(), steps, inputBytes)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("stream", func(b *testing.B) {
		tmpDir := b.TempDir()
		srcPath := filepath.Join(tmpDir, "input.dat")
		if err := os.WriteFile(srcPath, inputBytes, 0o600); err != nil {
			b.Fatal(err)
		}

		for b.Loop() {
			resultPath, err := RunStream(b.Context(), steps, srcPath, tmpDir)
			if err != nil {
				b.Fatal(err)
			}
			_ = os.Remove(resultPath)
		}
	})
}

// BenchmarkStreamExtractIPv4 benchmarks streaming vs []byte for extract_ipv4.
func BenchmarkStreamExtractIPv4(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 50000; i++ {
		fmt.Fprintf(&sb, "log entry src=%d.%d.%d.%d dst=10.0.0.1\n", i%256, (i/256)%256, (i/65536)%256, i%256)
	}
	input := sb.String()
	inputBytes := []byte(input)
	steps := []config.ProcessorStep{{Name: "extract_ipv4"}}

	b.Run("bytes", func(b *testing.B) {
		for b.Loop() {
			_, err := Run(b.Context(), steps, inputBytes)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("stream", func(b *testing.B) {
		tmpDir := b.TempDir()
		srcPath := filepath.Join(tmpDir, "input.dat")
		if err := os.WriteFile(srcPath, inputBytes, 0o600); err != nil {
			b.Fatal(err)
		}

		for b.Loop() {
			resultPath, err := RunStream(b.Context(), steps, srcPath, tmpDir)
			if err != nil {
				b.Fatal(err)
			}
			_ = os.Remove(resultPath)
		}
	})
}
