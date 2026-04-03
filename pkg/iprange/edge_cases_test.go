package iprange

import (
	"strings"
	"testing"
)

func TestParseReaderBOM(t *testing.T) {
	// UTF-8 BOM before first IP
	input := "\xEF\xBB\xBF1.2.3.4\n5.6.7.8\n"
	set, err := ParseReader(t.Context(), "bom", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("BOM input should parse: %v", err)
	}
	set.Optimize()
	if set.UniqueCount() != 2 {
		t.Fatalf("expected 2 unique IPs, got %d", set.UniqueCount())
	}
	if !set.Contains(mustIP(t, "1.2.3.4")) {
		t.Fatal("should contain 1.2.3.4")
	}
	if !set.Contains(mustIP(t, "5.6.7.8")) {
		t.Fatal("should contain 5.6.7.8")
	}
}

func TestParseReaderBOMWithComment(t *testing.T) {
	input := "\xEF\xBB\xBF# comment\n1.2.3.4\n"
	set, err := ParseReader(t.Context(), "bom-comment", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("BOM+comment should parse: %v", err)
	}
	set.Optimize()
	if set.UniqueCount() != 1 {
		t.Fatalf("expected 1 unique IP, got %d", set.UniqueCount())
	}
}

func TestParseReaderBOMWithCRLF(t *testing.T) {
	input := "\xEF\xBB\xBF1.2.3.4\r\n5.6.7.8\r\n"
	set, err := ParseReader(t.Context(), "bom-crlf", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("BOM+CRLF should parse: %v", err)
	}
	set.Optimize()
	if set.UniqueCount() != 2 {
		t.Fatalf("expected 2 unique IPs, got %d", set.UniqueCount())
	}
}

func TestParsePrefixInvalid(t *testing.T) {
	cases := []struct {
		input string
	}{
		{"33"},
		{"-1"},
		{"256.0.0.0"},
	}
	for _, tc := range cases {
		_, err := ParsePrefix(tc.input)
		if err == nil {
			t.Fatalf("ParsePrefix(%q) should fail", tc.input)
		}
	}
}

func TestParsePrefixValid(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"1", 1},
		{"24", 24},
		{"32", 32},
		{"255.255.255.0", 24},
		{"255.255.0.0", 16},
		{"255.0.0.0", 8},
	}
	for _, tc := range cases {
		got, err := ParsePrefix(tc.input)
		if err != nil {
			t.Fatalf("ParsePrefix(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ParsePrefix(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseReaderMalformedCIDRPrefix33(t *testing.T) {
	// 1.2.3.4/33 is an invalid prefix; the lenient parser SKIPS the
	// line rather than failing the whole load (matches the C iprange
	// behaviour the bash version has used in production for years).
	// The result is a valid empty set.
	input := "1.2.3.4/33\n"
	set, err := ParseReader(t.Context(), "bad-cidr", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("invalid prefix should be skipped, not error: %v", err)
	}
	if set.UniqueCount() != 0 {
		t.Fatalf("expected empty set, got %d IPs", set.UniqueCount())
	}
}

func TestParseReaderSlash0(t *testing.T) {
	// /0 is valid CIDR (covers all IPs); the parser should handle it
	input := "0.0.0.0/0\n"
	set, err := ParseReader(t.Context(), "slash0", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("/0 should parse: %v", err)
	}
	set.Optimize()
	// 0.0.0.0/0 covers all 2^32 IPs
	if set.UniqueCount() != 1<<32 {
		t.Fatalf("expected 2^32 IPs, got %d", set.UniqueCount())
	}
}

func TestParseReaderSlash32(t *testing.T) {
	input := "255.255.255.255/32\n"
	set, err := ParseReader(t.Context(), "slash32", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("/32 should parse: %v", err)
	}
	set.Optimize()
	if set.UniqueCount() != 1 {
		t.Fatalf("expected 1 IP, got %d", set.UniqueCount())
	}
	if !set.Contains(mustIP(t, "255.255.255.255")) {
		t.Fatal("should contain 255.255.255.255")
	}
}

func TestParseReaderDuplicateIPs(t *testing.T) {
	var sb strings.Builder
	for range 100 {
		sb.WriteString("1.2.3.4\n")
	}
	set, err := ParseReader(t.Context(), "dupes", strings.NewReader(sb.String()), DefaultParseOptions())
	if err != nil {
		t.Fatal(err)
	}
	set.Optimize()
	if set.UniqueCount() != 1 {
		t.Fatalf("100 duplicates should optimize to 1 unique IP, got %d", set.UniqueCount())
	}
	if set.Entries() != 1 {
		t.Fatalf("100 duplicates should optimize to 1 range entry, got %d", set.Entries())
	}
}

func TestParseReaderOverlappingRanges(t *testing.T) {
	input := "1.2.3.0/24\n1.2.3.128/25\n1.2.3.4\n"
	set, err := ParseReader(t.Context(), "overlapping", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatal(err)
	}
	set.Optimize()
	// The /24 already contains both the /25 and the single IP.
	if set.Entries() != 1 {
		t.Fatalf("overlapping ranges should optimize to 1 entry, got %d", set.Entries())
	}
	if set.UniqueCount() != 256 {
		t.Fatalf("expected 256 unique IPs, got %d", set.UniqueCount())
	}
}

func TestParseReaderEmptyInput(t *testing.T) {
	set, err := ParseReader(t.Context(), "empty", strings.NewReader(""), DefaultParseOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.UniqueCount() != 0 {
		t.Fatalf("empty input should produce 0 IPs, got %d", set.UniqueCount())
	}
}

func TestParseReaderCRLFInput(t *testing.T) {
	input := "1.2.3.4\r\n5.6.7.8\r\n"
	set, err := ParseReader(t.Context(), "crlf", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("CRLF input should parse: %v", err)
	}
	set.Optimize()
	if set.UniqueCount() != 2 {
		t.Fatalf("expected 2 IPs, got %d", set.UniqueCount())
	}
}

func TestParseReaderCommentOnlyInput(t *testing.T) {
	input := "# empty feed\n; also empty\n# end\n"
	set, err := ParseReader(t.Context(), "comments", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.UniqueCount() != 0 {
		t.Fatalf("comment-only input should produce 0 IPs, got %d", set.UniqueCount())
	}
}

// Lenient parsing: IPv6 lines mixed into an IPv4 feed are silently
// skipped, the IPv4 lines are returned. This matches the C iprange
// behaviour the bash version of update-ipsets uses in production.
// Many real-world feeds (blocklist_de, dataplane_*, c2_tracker, …)
// publish both families to the same URL.
func TestParseReaderSkipsIPv6InIPv4Mode(t *testing.T) {
	input := strings.Join([]string{
		"# mixed-family feed",
		"1.2.3.4",
		"2001:16a2:ea06:300:d96c:f0f4:51e2:625e",
		"5.6.7.8",
		"2001:0418:8006:0000:0000:0000:0000:0009",
		"9.10.11.12",
	}, "\n") + "\n"
	set, err := ParseReader(t.Context(), "mixed", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("IPv6 lines should be skipped, not error: %v", err)
	}
	if got := set.UniqueCount(); got != 3 {
		t.Fatalf("expected 3 IPv4 IPs, got %d", got)
	}
}

// Lenient parsing: arbitrary garbage lines (random text, section
// headings, leftover prose that escaped the comment filter) are
// silently skipped instead of aborting the whole feed. The valid
// IPs that surround them must still be returned.
func TestParseReaderSkipsGarbageLines(t *testing.T) {
	input := strings.Join([]string{
		"1.2.3.4",
		"Затронутые скрипты:", // Russian section header (blocklist_net_ua case)
		"5.6.7.8",
		"<?xml version=\"1.0\"?>", // stray HTML/XML tag
		"9.10.11.12",
		"not-an-ip-at-all",
		"13.14.15.16",
	}, "\n") + "\n"
	set, err := ParseReader(t.Context(), "garbage", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("garbage lines should be skipped, not error: %v", err)
	}
	if got := set.UniqueCount(); got != 4 {
		t.Fatalf("expected 4 IPv4 IPs, got %d", got)
	}
}

// Edge case: a file that is ENTIRELY garbage parses to an empty set,
// not an error. The engine surfaces this as `last_status: empty`
// downstream, which is more accurate than `parse_failed`. The
// gpf_comics feed (which started returning XML) is the canonical
// real-world example.
func TestParseReaderAllGarbageReturnsEmpty(t *testing.T) {
	input := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root><error>404</error></root>\n"
	set, err := ParseReader(t.Context(), "html", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("all-garbage input should return empty set, not error: %v", err)
	}
	if set.UniqueCount() != 0 {
		t.Fatalf("expected empty set, got %d IPs", set.UniqueCount())
	}
}

func TestParseReaderAcceptsVeryLongGarbageLine(t *testing.T) {
	input := strings.Repeat("x", 3*1024*1024) + "\n1.2.3.4\n"
	set, err := ParseReader(t.Context(), "long-garbage", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("long garbage line should be skipped without parser failure: %v", err)
	}
	set.Optimize()
	if set.UniqueCount() != 1 {
		t.Fatalf("expected 1 IP after long garbage line, got %d", set.UniqueCount())
	}
	if !set.Contains(mustIP(t, "1.2.3.4")) {
		t.Fatal("should contain 1.2.3.4")
	}
}

// Single-word ASCII tokens (CSV header cells like "IP", "ENABLED",
// "total") are NOT plausible hostnames and must not be sent to DNS
// resolution. The blocklist_net_ua CSV starts with `IP;BAN_DATE;…`
// and the literal "IP" header was hitting `lookup IP: no such host`
// before the dot-required check landed.
func TestParseReaderSkipsSingleWordTokens(t *testing.T) {
	input := strings.Join([]string{
		"IP", // CSV header cell — not a hostname
		"1.2.3.4",
		"ENABLED",
		"5.6.7.8",
		"total",
		"9.10.11.12",
	}, "\n") + "\n"
	set, err := ParseReader(t.Context(), "csv-header", strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("single-word tokens should be skipped, not error: %v", err)
	}
	if got := set.UniqueCount(); got != 3 {
		t.Fatalf("expected 3 IPv4 IPs, got %d", got)
	}
}

// Hostnames WITH dots are still resolved as before — the dot
// requirement is the discriminator between identifiers and DNS
// names. Uses a static stub resolver so the test does not hit
// the network.
func TestParseReaderResolvesDottedHostnames(t *testing.T) {
	opts := DefaultParseOptions()
	opts.Resolver = staticResolver{
		"example.net": {mustIP(t, "203.0.113.10")},
	}
	input := "example.net\n1.2.3.4\nIP\n"
	set, err := ParseReader(t.Context(), "mixed-host", strings.NewReader(input), opts)
	if err != nil {
		t.Fatalf("mixed input should parse: %v", err)
	}
	if got := set.UniqueCount(); got != 2 {
		t.Fatalf("expected 2 IPs (1.2.3.4 + resolved example.net), got %d", got)
	}
}

func TestOptimizeDuplicatesAndAdjacent(t *testing.T) {
	set := New("test")
	// Add 1.2.3.4 three times
	for range 3 {
		_ = set.Add(mustIP(t, "1.2.3.4"), mustIP(t, "1.2.3.4"))
	}
	// Add adjacent 1.2.3.5
	_ = set.Add(mustIP(t, "1.2.3.5"), mustIP(t, "1.2.3.5"))

	set.Optimize()
	if set.Entries() != 1 {
		t.Fatalf("expected 1 merged range, got %d", set.Entries())
	}
	if set.UniqueCount() != 2 {
		t.Fatalf("expected 2 unique IPs, got %d", set.UniqueCount())
	}
}
