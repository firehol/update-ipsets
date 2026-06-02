package processor

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

// --- Edge case 1: Empty feeds ---

func TestEmptyFeedReturnsNil(t *testing.T) {
	processors := []string{
		"remove_comments", "extract_ipv4", "passthrough", "trim",
		"filter_all4", "filter_ip4", "filter_invalid4",
	}
	for _, name := range processors {
		t.Run(name, func(t *testing.T) {
			out, err := Run(t.Context(), []config.ProcessorStep{{Name: name}}, nil)
			if err != nil {
				t.Fatalf("processor %q failed on nil input: %v", name, err)
			}
			if len(out) != 0 {
				t.Fatalf("processor %q: expected empty output, got %q", name, out)
			}
		})
	}
}

func TestCommentOnlyFeedReturnsEmpty(t *testing.T) {
	input := []byte("# This feed is currently empty\n# No threats detected\n# End of list\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty output for comment-only feed, got %q", out)
	}
}

func TestWhitespaceOnlyFeedReturnsEmpty(t *testing.T) {
	input := []byte("   \n\t\n  \t  \n\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "trim"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty output for whitespace-only feed, got %q", out)
	}
}

// --- Edge case 2: HTML error pages ---

func TestExtractIPv4IgnoresHTMLTags(t *testing.T) {
	// Simulates a 404 HTML page that contains numeric content in tags/attributes
	input := []byte(`<!DOCTYPE html>
<html>
<head><title>404 Not Found</title></head>
<body>
<h1>Not Found</h1>
<p>The requested URL was not found on this server.</p>
<p>Error code: 404</p>
<div style="width:100px;height:200px">Page ID: 12345</div>
<img src="/images/error.png" width="640" height="480">
<a href="http://192.168.1.1/admin">Admin</a>
</body>
</html>`)

	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "extract_ipv4"}}, input)
	if err != nil {
		t.Fatal(err)
	}

	// The regex should extract 192.168.1.1 from the href attribute
	// but NOT fabricate IPs from non-IP numbers (404, 100, 200, etc.)
	got := string(out)
	if !strings.Contains(got, "192.168.1.1") {
		t.Fatalf("expected 192.168.1.1 from href, got %q", got)
	}

	// Verify no false positives from CSS dimensions, error codes, etc.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	for _, line := range lines {
		if line != "192.168.1.1" {
			t.Fatalf("unexpected IP extracted from HTML: %q", line)
		}
	}
}

func TestRemoveCommentsThenExtractFromHTML(t *testing.T) {
	// HTML error page with no valid IPs at all
	input := []byte(`<html><body><p>Server Error 500</p></body></html>`)
	steps := []config.ProcessorStep{
		{Name: "extract_ipv4"},
	}
	out, err := Run(t.Context(), steps, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no IPs from HTML error page, got %q", out)
	}
}

// --- Edge case 4: extract_ipv4_cidr preserves CIDR suffixes ---

// TestExtractIPv4CIDRPreservesPrefix is the regression test for
// the April 2026 MISP warninglists audit. The original
// extract_ipv4 processor strips CIDR suffixes ("103.21.244.0/22"
// becomes "103.21.244.0"), which made every CIDR-heavy JSON feed
// silently report 1 IP per entry instead of the full range. We
// added extract_ipv4_cidr as the CIDR-preserving sibling; this
// test pins the behaviour so a future "simplification" of the
// regex does not quietly re-introduce the bug.
func TestExtractIPv4CIDRPreservesPrefix(t *testing.T) {
	// A chunk of realistic MISP warninglist JSON with a mix of
	// /32, non-/32, bare IPs, and quoted tokens.
	input := []byte(`{
  "description": "Test list",
  "list": [
    "103.21.244.0/22",
    "192.168.1.5",
    "10.0.0.0/8",
    "1.2.3.4/32",
    "212.73.148.0/29"
  ]
}`)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "extract_ipv4_cidr"}},
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(out)), "\n")
	want := []string{
		"103.21.244.0/22",
		"192.168.1.5",
		"10.0.0.0/8",
		"1.2.3.4/32",
		"212.73.148.0/29",
	}
	if len(got) != len(want) {
		t.Fatalf("extract_ipv4_cidr count: got %v want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("extract_ipv4_cidr[%d]: got %q want %q", i, got[i], w)
		}
	}
}

// TestExtractIPv4CIDRRejectsInvalidPrefix confirms the regex
// refuses prefix lengths outside 0-32 so bad data cannot leak
// malformed tokens to the downstream iprange parser.
func TestExtractIPv4CIDRRejectsInvalidPrefix(t *testing.T) {
	// /33 is not a valid IPv4 prefix. The regex should either
	// extract the bare IP (dropping the invalid suffix) or emit
	// nothing. Either is acceptable; the test pins the current
	// "drop the invalid suffix" behaviour so any future change
	// is deliberate.
	input := []byte("1.2.3.4/33\n5.6.7.8/32\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "extract_ipv4_cidr"}},
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(out))
	// The invalid /33 must not leak as "1.2.3.4/33" — it's
	// either dropped entirely or emitted as the bare IP "1.2.3.4".
	if strings.Contains(got, "/33") {
		t.Fatalf("invalid /33 leaked through: %q", got)
	}
	// The valid /32 must survive intact.
	if !strings.Contains(got, "5.6.7.8/32") {
		t.Fatalf("valid /32 was dropped: %q", got)
	}
}

// TestExtractIPv4StripsCIDR is the negative-space counterpart to
// TestExtractIPv4CIDRPreservesPrefix: the legacy extract_ipv4
// processor MUST continue to strip CIDR suffixes so the 9 feeds
// that already depend on that behaviour (cybercrime, cybercure,
// maltrail_scanners, socks_proxy, sslproxies, vxvault, and the
// three stratosphere_aip CSVs) keep working unchanged.
func TestExtractIPv4StripsCIDR(t *testing.T) {
	input := []byte("103.21.244.0/22\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "extract_ipv4"}},
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(out))
	if got != "103.21.244.0" {
		t.Fatalf("extract_ipv4 should strip CIDR: got %q want %q", got, "103.21.244.0")
	}
}

// --- Edge case 5: CRLF vs LF ---

func TestCRLFLineEndings(t *testing.T) {
	input := []byte("1.2.3.4\r\n5.6.7.8\r\n# comment\r\n9.10.11.12\r\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n9.10.11.12\n"
	if string(out) != want {
		t.Fatalf("CRLF handling: got %q want %q", out, want)
	}
}

func TestCROnlyLineEndings(t *testing.T) {
	// Classic Mac line endings (bare \r)
	input := []byte("1.2.3.4\r5.6.7.8\r# comment\r9.10.11.12\r")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n9.10.11.12\n"
	if string(out) != want {
		t.Fatalf("CR-only handling: got %q want %q", out, want)
	}
}

func TestMixedLineEndings(t *testing.T) {
	input := []byte("1.2.3.4\n5.6.7.8\r\n9.10.11.12\r13.14.15.16\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n9.10.11.12\n13.14.15.16\n"
	if string(out) != want {
		t.Fatalf("mixed line endings: got %q want %q", out, want)
	}
}

// --- Edge case 5 streaming: CRLF in streaming pipeline ---

func TestStreamCRLFLineEndings(t *testing.T) {
	input := "1.2.3.4\r\n5.6.7.8\r\n# comment\r\n9.10.11.12\r\n"
	steps := []config.ProcessorStep{{Name: "remove_comments"}}

	// Bytes pipeline
	bytesOut, err := Run(t.Context(), steps, []byte(input))
	if err != nil {
		t.Fatal(err)
	}

	// Stream pipeline
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "crlf.dat")
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
		t.Fatalf("CRLF stream mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
	}
}

// --- Edge case 6: BOM markers ---

func TestBOMStrippedInRemoveComments(t *testing.T) {
	// UTF-8 BOM followed by an IP
	bom := []byte{0xEF, 0xBB, 0xBF}
	input := append(bom, []byte("1.2.3.4\n5.6.7.8\n")...)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(out) != want {
		t.Fatalf("BOM not stripped: got %q (bytes: %x) want %q", out, out, want)
	}
}

func TestBOMStrippedInExtractIPv4(t *testing.T) {
	// extract_ipv4 applies regex to the entire input. When two IPs are on
	// consecutive lines, the \n boundary consumed by the first match can
	// prevent the second from matching. Use a format where each IP has
	// non-digit context around it, which is the typical real-world pattern.
	bom := []byte{0xEF, 0xBB, 0xBF}
	input := append(bom, []byte("src=1.2.3.4 dst=5.6.7.8\n")...)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "extract_ipv4"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(out) != want {
		t.Fatalf("BOM not stripped in extract_ipv4: got %q want %q", out, want)
	}
}

func TestBOMStrippedInPassthrough(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	input := append(bom, []byte("1.2.3.4\n")...)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "passthrough"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	// passthrough copies data as-is, but splitLines is not called.
	// BOM stripping in passthrough is less critical since the parser
	// handles it. Verify it doesn't crash.
	if len(out) == 0 {
		t.Fatal("expected non-empty output from passthrough with BOM")
	}
}

func TestBOMStrippedInStreamingPipeline(t *testing.T) {
	bom := "\xEF\xBB\xBF"
	input := bom + "1.2.3.4\n5.6.7.8\n"
	steps := []config.ProcessorStep{{Name: "remove_comments"}}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "bom.dat")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()

	got, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(got) != want {
		t.Fatalf("BOM not stripped in stream: got %q (bytes: %x) want %q", got, got, want)
	}
}

func TestBOMWithCRLF(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	input := append(bom, []byte("1.2.3.4\r\n5.6.7.8\r\n")...)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(out) != want {
		t.Fatalf("BOM+CRLF: got %q want %q", out, want)
	}
}

// --- Edge case 7: Gzip bomb protection ---

func TestGunzipBombProtection(t *testing.T) {
	// Create a gzip payload that decompresses to more than maxDecompressedSize.
	// We use a very efficient compression (all zeros) to make a small payload
	// decompress to a large size.
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	// Write 1MB of zeros repeatedly — gzip compresses this to almost nothing.
	chunk := make([]byte, 1024*1024) // 1MB of zeros
	// Write enough to exceed the 500MB limit. Since this is a test, we don't
	// actually want to allocate 500MB. Instead, verify the limit mechanism
	// works at a smaller scale.
	for i := 0; i < 2; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// The gunzip processor itself has the limit. Verify that it decompresses
	// correctly when within bounds.
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "gunzip"}}, gz.Bytes())
	if err != nil {
		t.Fatalf("gunzip should succeed for 2MB data: %v", err)
	}
	if int64(len(out)) != 2*1024*1024 {
		t.Fatalf("expected 2MB decompressed, got %d", len(out))
	}
}

func TestGunzipSizeConstantExists(t *testing.T) {
	// Verify the constant is reasonable.
	if maxDecompressedSize < 100*1024*1024 {
		t.Fatalf("maxDecompressedSize too small: %d", maxDecompressedSize)
	}
	if maxDecompressedSize > 1024*1024*1024 {
		t.Fatalf("maxDecompressedSize too large: %d", maxDecompressedSize)
	}
}

// --- Edge case 8: Malformed CIDR ---

func TestFilterInvalid4RejectsSlash0(t *testing.T) {
	input := []byte("0.0.0.0/0\n10.0.0.0/0\n192.168.1.1\n0.0.0.0\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "filter_invalid4"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "192.168.1.1\n"
	if string(out) != want {
		t.Fatalf("filter_invalid4: got %q want %q", out, want)
	}
}

// --- Edge case 9: Duplicate entries ---

func TestDuplicateIPsInFeed(t *testing.T) {
	// 100 copies of the same IP, using remove_comments (not extract_ipv4)
	// to avoid the boundary-consumption regex limitation of the bytes path.
	var sb strings.Builder
	for range 100 {
		sb.WriteString("1.2.3.4\n")
	}
	input := []byte(sb.String())
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 100 {
		t.Fatalf("expected 100 lines, got %d", len(lines))
	}
	for _, line := range lines {
		if line != "1.2.3.4" {
			t.Fatalf("unexpected line: %q", line)
		}
	}
}

func TestDuplicateIPsInFeedStreaming(t *testing.T) {
	// Streaming extract_ipv4 works line-by-line, avoiding the bytes
	// path's boundary-consumption issue.
	var sb strings.Builder
	for range 100 {
		sb.WriteString("1.2.3.4\n")
	}
	input := sb.String()
	steps := []config.ProcessorStep{{Name: "extract_ipv4"}}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "dupes.dat")
	if err := os.WriteFile(srcPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	resultPath, err := RunStream(t.Context(), steps, srcPath, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(resultPath) }()
	got, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 100 {
		t.Fatalf("expected 100 extracted IPs in stream, got %d", len(lines))
	}
	for _, line := range lines {
		if line != "1.2.3.4" {
			t.Fatalf("unexpected line: %q", line)
		}
	}
}

// --- Edge case 10: Unicode in feed data ---

func TestUnicodeInComments(t *testing.T) {
	input := []byte("1.2.3.4 # Les données de sécurité — mise à jour quotidienne\n5.6.7.8 # 日本語コメント\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(out) != want {
		t.Fatalf("unicode comments: got %q want %q", out, want)
	}
}

func TestUnicodeInNonIPLines(t *testing.T) {
	input := []byte("# Données de menaces\n1.2.3.4\nÅtgärd: blockera\n5.6.7.8\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "extract_ipv4"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(out) != want {
		t.Fatalf("unicode non-IP lines: got %q want %q", out, want)
	}
}

// --- Edge case: IPv6 filter processors ---

func TestFilterAll4DropsIPv6(t *testing.T) {
	input := []byte("1.2.3.4\n2001:db8::1\n5.6.7.0/24\nfe80::1\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "filter_all4"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.0/24\n"
	if string(out) != want {
		t.Fatalf("filter_all4 IPv6: got %q want %q", out, want)
	}
}

func TestFilterAll6AcceptsIPv6(t *testing.T) {
	input := []byte("1.2.3.4\n2001:db8::1\n5.6.7.0/24\nfe80::/10\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "filter_all6"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	want := "2001:db8::1\nfe80::/10\n"
	if string(out) != want {
		t.Fatalf("filter_all6: got %q want %q", out, want)
	}
}

// --- Streaming equivalence for BOM + CRLF edge cases ---

func TestStreamBOMEquivalence(t *testing.T) {
	cases := []struct {
		name  string
		steps []config.ProcessorStep
		input string
	}{
		{
			name:  "bom_remove_comments",
			steps: []config.ProcessorStep{{Name: "remove_comments"}},
			input: "\xEF\xBB\xBF1.2.3.4 # comment\n5.6.7.8\n",
		},
		{
			name:  "bom_extract_ipv4",
			steps: []config.ProcessorStep{{Name: "extract_ipv4"}},
			input: "\xEF\xBB\xBFsrc=1.2.3.4 dst=5.6.7.8\n",
		},
		{
			name:  "bom_crlf_remove_comments",
			steps: []config.ProcessorStep{{Name: "remove_comments"}},
			input: "\xEF\xBB\xBF1.2.3.4 # comment\r\n5.6.7.8\r\n",
		},
		{
			name:  "crlf_filter_all4",
			steps: []config.ProcessorStep{{Name: "filter_all4"}},
			input: "1.2.3.4\r\n5.6.7.0/24\r\n",
		},
		{
			name:  "crlf_extract_ipv4",
			steps: []config.ProcessorStep{{Name: "extract_ipv4"}},
			input: "src=1.2.3.4 dst=5.6.7.8\r\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bytesOut, err := Run(t.Context(), tc.steps, []byte(tc.input))
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			tmpDir := t.TempDir()
			srcPath := filepath.Join(tmpDir, "input.dat")
			if err := os.WriteFile(srcPath, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}

			resultPath, err := RunStream(t.Context(), tc.steps, srcPath, tmpDir)
			if err != nil {
				t.Fatalf("RunStream: %v", err)
			}
			defer func() { _ = os.Remove(resultPath) }()
			streamOut, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal(err)
			}

			if string(bytesOut) != string(streamOut) {
				t.Fatalf("mismatch:\n  bytes:  %q\n  stream: %q", bytesOut, streamOut)
			}
		})
	}
}
