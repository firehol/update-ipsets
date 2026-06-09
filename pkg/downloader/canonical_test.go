package downloader

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestCanonicalOutputFamilyAliases(t *testing.T) {
	tests := []struct {
		output string
		want   string
	}{
		{output: "ip", want: "ipset"},
		{output: "ips", want: "ipset"},
		{output: "ipset", want: "ipset"},
		{output: "net", want: "netset"},
		{output: "nets", want: "netset"},
		{output: "both", want: "netset"},
		{output: "all", want: "netset"},
		{output: "netset", want: "netset"},
		{output: " NetSet ", want: "netset"},
		{output: "json", want: "json"},
	}
	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			if got := canonicalOutputFamily(tt.output); got != tt.want {
				t.Fatalf("canonicalOutputFamily(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}

func TestRenderCanonicalFeedBody(t *testing.T) {
	ipSet := parseCanonicalSetForTest(t, "ipset-sample", "5.6.7.8\n1.2.3.4\n")
	body, err := RenderCanonicalFeedBody(ipSet, "ips")
	if err != nil {
		t.Fatalf("RenderCanonicalFeedBody(ipset): %v", err)
	}
	if got, want := string(body), "1.2.3.4\n5.6.7.8\n"; got != want {
		t.Fatalf("ipset body = %q, want %q", got, want)
	}

	netSet := parseCanonicalSetForTest(t, "netset-sample", "5.6.7.0/24\n1.2.3.4\n")
	body, err = RenderCanonicalFeedBody(netSet, "both")
	if err != nil {
		t.Fatalf("RenderCanonicalFeedBody(netset): %v", err)
	}
	if got, want := string(body), "1.2.3.4\n5.6.7.0/24\n"; got != want {
		t.Fatalf("netset body = %q, want %q", got, want)
	}
}

func TestRenderCanonicalFeedBodyErrors(t *testing.T) {
	if _, err := RenderCanonicalFeedBody(nil, "ipset"); err == nil {
		t.Fatal("RenderCanonicalFeedBody(nil) error = nil, want error")
	}
	set := parseCanonicalSetForTest(t, "sample", "1.2.3.4\n")
	if _, err := RenderCanonicalFeedBody(set, "json"); err == nil {
		t.Fatal("RenderCanonicalFeedBody(unsupported) error = nil, want error")
	}
}

func TestParseCanonicalFeedReaderAndFile(t *testing.T) {
	opts := iprange.DefaultParseOptions()
	opts.DefaultPrefix = 32
	readerSet, err := ParseCanonicalFeedReader(t.Context(), "reader-sample", strings.NewReader("1.2.3.4\n5.6.7.0/31\n"), opts)
	if err != nil {
		t.Fatalf("ParseCanonicalFeedReader: %v", err)
	}
	assertCanonicalSetContains(t, readerSet, "1.2.3.4", "5.6.7.0", "5.6.7.1")
	if got, want := readerSet.UniqueCount(), uint64(3); got != want {
		t.Fatalf("reader unique count = %d, want %d", got, want)
	}

	path := filepath.Join(t.TempDir(), "feed.txt")
	if err := os.WriteFile(path, []byte("8.8.8.8\n9.9.9.0/31\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileSet, err := ParseCanonicalFeedFile(t.Context(), "file-sample", path, 0)
	if err != nil {
		t.Fatalf("ParseCanonicalFeedFile: %v", err)
	}
	assertCanonicalSetContains(t, fileSet, "8.8.8.8", "9.9.9.0", "9.9.9.1")
	if got, want := fileSet.UniqueCount(), uint64(3); got != want {
		t.Fatalf("file unique count = %d, want %d", got, want)
	}
}

func TestParseProcessedFeedFileSanitizesInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "processed.txt")
	input := "\ufeff 1.2.3.4\r\n\n0.0.0.0\n5.6.7.0/0\n8.8.8.8\r\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := ParseProcessedFeedFile(t.Context(), "processed-sample", path, 0)
	if err != nil {
		t.Fatalf("ParseProcessedFeedFile: %v", err)
	}
	assertCanonicalSetContains(t, set, "1.2.3.4", "8.8.8.8")
	assertCanonicalSetOmits(t, set, "0.0.0.0", "5.6.7.1")
	if got, want := set.UniqueCount(), uint64(2); got != want {
		t.Fatalf("processed unique count = %d, want %d", got, want)
	}
}

func TestPrepareCanonicalFeedBody(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "source.txt")
	input := "# header\n1.2.3.4 # first\n0.0.0.0\n5.6.7.8\r\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	body, set, err := PrepareCanonicalFeedBody(t.Context(), "prepared-sample", "ipset", inputPath, []config.ProcessorStep{{Name: "remove_comments"}}, filepath.Join(root, "tmp"), 0)
	if err != nil {
		t.Fatalf("PrepareCanonicalFeedBody: %v", err)
	}
	if got, want := string(body), "1.2.3.4\n5.6.7.8\n"; got != want {
		t.Fatalf("prepared body = %q, want %q", got, want)
	}
	assertCanonicalSetContains(t, set, "1.2.3.4", "5.6.7.8")
	if got, want := set.UniqueCount(), uint64(2); got != want {
		t.Fatalf("prepared unique count = %d, want %d", got, want)
	}
}

func TestSanitizeReaderNormalizesProcessedStream(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normalizes processed stream",
			input: "\ufeff 1.2.3.4\r\n\n0.0.0.0\n5.6.7.0/0\n  8.8.8.8\t\r\n9.9.9.9   10.10.10.10\r\n",
			want:  "1.2.3.4\n8.8.8.8\n9.9.9.9 10.10.10.10\n",
		},
		{
			name:  "empty input returns empty stream",
			input: "",
			want:  "",
		},
		{
			name:  "bom only is stripped to empty stream",
			input: "\ufeff",
			want:  "",
		},
		{
			name:  "bom mid stream is preserved not stripped",
			input: "1.1.1.1\n\ufeff2.2.2.2\n",
			want:  "1.1.1.1\n\ufeff2.2.2.2\n",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := io.ReadAll(newSanitizeReader(strings.NewReader(tc.input)))
			if err != nil {
				t.Fatalf("ReadAll(newSanitizeReader): %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("sanitized stream = %q, want %q", got, tc.want)
			}
		})
	}
}

func parseCanonicalSetForTest(t *testing.T, name, body string) *iprange.IPSet {
	t.Helper()
	opts := iprange.DefaultParseOptions()
	opts.DefaultPrefix = 32
	set, err := ParseCanonicalFeedReader(t.Context(), name, strings.NewReader(body), opts)
	if err != nil {
		t.Fatalf("ParseCanonicalFeedReader(%s): %v", name, err)
	}
	return set
}

func assertCanonicalSetContains(t *testing.T, set *iprange.IPSet, addrs ...string) {
	t.Helper()
	for _, addr := range addrs {
		ip, err := iprange.ParseIPv4Token(addr)
		if err != nil {
			t.Fatalf("ParseIPv4Token(%q): %v", addr, err)
		}
		if !set.Contains(ip) {
			t.Fatalf("set does not contain %s", addr)
		}
	}
}

func assertCanonicalSetOmits(t *testing.T, set *iprange.IPSet, addrs ...string) {
	t.Helper()
	for _, addr := range addrs {
		ip, err := iprange.ParseIPv4Token(addr)
		if err != nil {
			t.Fatalf("ParseIPv4Token(%q): %v", addr, err)
		}
		if set.Contains(ip) {
			t.Fatalf("set contains %s, want omitted", addr)
		}
	}
}
