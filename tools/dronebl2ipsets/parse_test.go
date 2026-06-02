package dronebl

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const buildzoneFixture = `
10.0.0.0/24
!10.0.0.128/25
10.1.1.1 : direct assignment ignored by legacy script
@ metadata
$ metadata
127.0.0.2
:5:
5.5.5.5
:8:
8.8.8.8
:9:
9.9.9.0/24
!9.9.9.128/25
:12:
12.12.12.0/30
:13:
13.13.13.13
:19:
19.19.19.0/31
:255:
25.25.25.25
:99:
99.99.99.99
`

func TestParseBuildzoneRecognizesMissingClasses(t *testing.T) {
	parsed, err := ParseBuildzone(strings.NewReader(buildzoneFixture))
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]uint64{
		"bottler":              1,
		"open_dns_resolvers":   4,
		"bruteforce_attackers": 1,
		"abused_vpn_services":  2,
		"uncategorized":        1,
		"unknown":              1,
	}
	for name, want := range cases {
		data := parsed.Lists[name]
		if data == nil {
			t.Fatalf("expected list %q to exist", name)
		}
		if got := data.Include.UniqueCount(); got != want {
			t.Fatalf("list %q unique IPs: got %d want %d", name, got, want)
		}
	}
}

func TestBuildOutputsMatchesLegacyGroupingAndNewClasses(t *testing.T) {
	parsed, err := ParseBuildzone(strings.NewReader(buildzoneFixture))
	if err != nil {
		t.Fatal(err)
	}
	outputs := BuildOutputs(parsed, testOutputSpecs())

	expectOutput(t, outputs["dronebl_anonymizers"], strings.Join([]string{
		"8.8.8.8",
		"9.9.9.0/25",
		"10.0.0.0/25",
	}, "\n")+"\n")
	expectOutput(t, outputs["dronebl_bottler"], strings.Join([]string{
		"5.5.5.5",
		"10.0.0.0/25",
	}, "\n")+"\n")
	expectOutput(t, outputs["dronebl_open_dns_resolvers"], strings.Join([]string{
		"10.0.0.0/25",
		"12.12.12.0/30",
	}, "\n")+"\n")
	expectOutput(t, outputs["dronebl_dictionary_attacks"], strings.Join([]string{
		"10.0.0.0/25",
		"13.13.13.13",
	}, "\n")+"\n")
	expectOutput(t, outputs["dronebl_abused_vpn"], strings.Join([]string{
		"10.0.0.0/25",
		"19.19.19.0/31",
	}, "\n")+"\n")
	expectOutput(t, outputs["dronebl_unknown"], strings.Join([]string{
		"10.0.0.0/25",
		"25.25.25.25",
	}, "\n")+"\n")
}

func TestUpdateParsesBuildzoneAndWritesConfiguredSpecs(t *testing.T) {
	dir := t.TempDir()
	buildzone := filepath.Join(dir, "buildzone")
	if err := os.WriteFile(buildzone, []byte(buildzoneFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "out")

	report, err := Update(t.Context(), Options{
		WorkDir:       filepath.Join(dir, "work"),
		OutputDir:     outDir,
		BuildzonePath: buildzone,
		SkipFetch:     true,
		Specs: []OutputSpec{
			{Name: "dronebl_bottler", Lists: []string{"bottler"}},
			{Name: "dronebl_abused_vpn", Lists: []string{"abused_vpn_services"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Outputs) != 2 {
		t.Fatalf("outputs = %d, want 2", len(report.Outputs))
	}
	body, err := os.ReadFile(filepath.Join(outDir, "dronebl_bottler.source"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "5.5.5.5\n10.0.0.0/25\n"; got != want {
		t.Fatalf("unexpected source body: got %q want %q", got, want)
	}
}

func TestWriteSourceFileWritesCIDRAndPreservesMtime(t *testing.T) {
	set := NewRangeSet()
	if err := set.AddRange(Range{
		Lo: Network(mustIP(t, "203.0.113.1"), 24),
		Hi: Broadcast(mustIP(t, "203.0.113.1"), 24),
	}); err != nil {
		t.Fatal(err)
	}

	mtime := time.Date(2026, 4, 10, 7, 30, 0, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "out")
	if err := WriteSourceFile(dir, "dronebl_sample.source", set, mtime); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "dronebl_sample.source")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "203.0.113.0/24\n"; got != want {
		t.Fatalf("unexpected body: got %q want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode: got %04o want 0600", got)
	}
	if !info.ModTime().Equal(mtime) {
		t.Fatalf("mtime: got %s want %s", info.ModTime(), mtime)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("output dir mode: got %04o want 0700", got)
	}
}

func expectOutput(t *testing.T, set *RangeSet, want string) {
	t.Helper()
	var buf bytes.Buffer
	if err := set.WriteCIDR(&buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != want {
		t.Fatalf("output mismatch:\ngot:\n%swant:\n%s", got, want)
	}
}

func mustIP(t *testing.T, value string) uint32 {
	t.Helper()
	ip, err := ParseIPv4Token(value)
	if err != nil {
		t.Fatal(err)
	}
	return ip
}

func testOutputSpecs() []OutputSpec {
	return []OutputSpec{
		{Name: "dronebl_anonymizers", Lists: []string{
			"http_proxies",
			"socks_proxies",
			"web_page_proxies",
			"wingate_proxies",
			"proxychains",
		}},
		{Name: "dronebl_bottler", Lists: []string{"bottler"}},
		{Name: "dronebl_open_dns_resolvers", Lists: []string{"open_dns_resolvers"}},
		{Name: "dronebl_dictionary_attacks", Lists: []string{"bruteforce_attackers"}},
		{Name: "dronebl_abused_vpn", Lists: []string{"abused_vpn_services"}},
		{Name: "dronebl_unknown", Lists: []string{"uncategorized"}},
	}
}
