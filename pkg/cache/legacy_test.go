package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireLegacyEntry(t *testing.T, st *State, name string) *Entry {
	t.Helper()
	entry := st.EntrySnapshot(name)
	if entry == nil {
		t.Fatalf("expected migrated entry %q", name)
	}
	return entry
}

func TestLoadLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cache")
	content := `declare -A IPSET_INFO=([sample]="sample feed" )
declare -A IPSET_FILE=([sample]="sample.ipset" )
declare -A IPSET_MINS=([sample]="15" )
declare -A IPSET_HISTORY_MINS=([sample]="60 1440" )
declare -A IPSET_IPS=([sample]="42" )
declare -A IPSET_CLOCK_SKEW=([sample]="7" )
declare -A IPSET_DOWNLOADER=([sample]="copyfile" )
declare -A IPSET_DOWNLOADER_OPTIONS=([sample]="path=/tmp/file" )`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := LoadLegacy(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := requireLegacyEntry(t, st, "sample")
	if entry.Info != "sample feed" {
		t.Fatalf("unexpected info: %q", entry.Info)
	}
	if entry.File != "sample.ipset" {
		t.Fatalf("unexpected file: %q", entry.File)
	}
	if entry.FrequencyMinutes != 15 {
		t.Fatalf("unexpected frequency: %d", entry.FrequencyMinutes)
	}
	if got, want := len(entry.HistoryMinutes), 2; got != want {
		t.Fatalf("unexpected history count: got %d want %d", got, want)
	}
	if entry.UniqueIPs != 42 {
		t.Fatalf("unexpected unique IPs: %d", entry.UniqueIPs)
	}
	if entry.ClockSkewSeconds != 7 {
		t.Fatalf("unexpected clock skew: %d", entry.ClockSkewSeconds)
	}
	if entry.Downloader != "copyfile" {
		t.Fatalf("unexpected downloader: %q", entry.Downloader)
	}
	if entry.DownloaderOptions != "path=/tmp/file" {
		t.Fatalf("unexpected downloader options: %q", entry.DownloaderOptions)
	}
}

func TestLoadLegacyShellQuoting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cache")
	content := strings.Join([]string{
		`declare -A IPSET_INFO=([double]="quote \"slash\\ line\n tab\t dollar\$ tick\`,
		"`",
		` keep\q" [single]='single value' [ansi]=$'line\nhex\x41\x7a\x5A octal\101 quote\' slash\\ tab\t raw\z' [bare]=bare-value )`,
	}, "")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := LoadLegacy(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"double": "quote \"slash\\ line\n tab\t dollar$ tick` keep\\q",
		"single": "single value",
		"ansi":   "line\nhexAzZ octalA quote' slash\\ tab\t rawz",
		"bare":   "bare-value",
	}
	for name, want := range cases {
		entry := requireLegacyEntry(t, st, name)
		if entry.Info != want {
			t.Fatalf("%s info = %q, want %q", name, entry.Info, want)
		}
	}
}

func TestLoadLegacyMigratesKnownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cache")
	content := `# ordinary comments and non-declare lines are ignored
ignored=true
declare -A IPSET_SOURCE=([sample]="sample.source" )
declare -A IPSET_URL=([sample]="https://example.test/feed.txt" )
declare -A IPSET_IPV=([sample]="ipv4" )
declare -A IPSET_HASH=([sample]="sha256:abc" )
declare -A IPSET_ENTRIES=([sample]="7" )
declare -A IPSET_SOURCE_DATE=([sample]="100" )
declare -A IPSET_CHECKED_DATE=([sample]="101" )
declare -A IPSET_PROCESSED_DATE=([sample]="102" )
declare -A IPSET_STARTED_DATE=([sample]="103" )
declare -A IPSET_CATEGORY=([sample]="attacks" )
declare -A IPSET_MAINTAINER=([sample]="Feed Maintainer" )
declare -A IPSET_MAINTAINER_URL=([sample]="https://example.test/about" )
declare -A IPSET_ENTRIES_MIN=([sample]="3" )
declare -A IPSET_ENTRIES_MAX=([sample]="9" )
declare -A IPSET_IPS_MIN=([sample]="4" )
declare -A IPSET_IPS_MAX=([sample]="12" )
declare -A IPSET_DOWNLOAD_FAILURES=([sample]="2" )
declare -A IPSET_VERSION=([sample]="5" )
declare -A IPSET_AVERAGE_UPDATE_TIME=([sample]="60" )
declare -A IPSET_MIN_UPDATE_TIME=([sample]="30" )
declare -A IPSET_MAX_UPDATE_TIME=([sample]="120" )`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := LoadLegacy(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := requireLegacyEntry(t, st, "sample")

	if entry.Source != "sample.source" {
		t.Fatalf("source = %q", entry.Source)
	}
	if entry.URL != "https://example.test/feed.txt" {
		t.Fatalf("url = %q", entry.URL)
	}
	if entry.IPV != "ipv4" || entry.Hash != "sha256:abc" || entry.Category != "attacks" {
		t.Fatalf("unexpected identity fields: %+v", entry)
	}
	if entry.Entries != 7 || entry.UniqueIPs != 0 {
		t.Fatalf("unexpected count fields: %+v", entry)
	}
	if entry.SourceDate != 100 || entry.CheckedDate != 101 || entry.ProcessedDate != 102 || entry.StartedDate != 103 {
		t.Fatalf("unexpected timestamps: %+v", entry)
	}
	if entry.Maintainer != "Feed Maintainer" || entry.MaintainerURL != "https://example.test/about" {
		t.Fatalf("unexpected maintainer fields: %+v", entry)
	}
	if entry.EntriesMin != 3 || entry.EntriesMax != 9 || entry.IPsMin != 4 || entry.IPsMax != 12 {
		t.Fatalf("unexpected min/max fields: %+v", entry)
	}
	if entry.DownloadFailures != 2 || entry.Version != 5 {
		t.Fatalf("unexpected failure/version fields: %+v", entry)
	}
	if entry.AverageUpdateMins != 60 || entry.MinUpdateMins != 30 || entry.MaxUpdateMins != 120 {
		t.Fatalf("unexpected update timing fields: %+v", entry)
	}
}

func TestLoadLegacyReportsMalformedDeclarations(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing assignment",
			body: "declare -A IPSET_INFO",
			want: "invalid legacy cache line",
		},
		{
			name: "missing array wrapper",
			body: `declare -A IPSET_INFO=[sample]="x"`,
			want: "invalid associative array payload",
		},
		{
			name: "missing key terminator",
			body: `declare -A IPSET_INFO=([sample="x" )`,
			want: "unterminated associative array key",
		},
		{
			name: "missing key assignment",
			body: `declare -A IPSET_INFO=([sample] )`,
			want: "missing '='",
		},
		{
			name: "missing key opener",
			body: `declare -A IPSET_INFO=(sample="x" )`,
			want: "expected '['",
		},
		{
			name: "unterminated double quote",
			body: `declare -A IPSET_INFO=([sample]="unterminated )`,
			want: "unterminated double-quoted value",
		},
		{
			name: "unterminated single quote",
			body: `declare -A IPSET_INFO=([sample]='unterminated )`,
			want: "unterminated single-quoted value",
		},
		{
			name: "unterminated ansi quote",
			body: `declare -A IPSET_INFO=([sample]=$'unterminated )`,
			want: "unterminated ANSI-C quoted value",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".cache")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := LoadLegacy(path)
			if err == nil {
				t.Fatal("expected LoadLegacy error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadLegacy error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestLoadWithMigration(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".cache")
	jsonPath := filepath.Join(dir, ".cache.json")
	if err := os.WriteFile(legacyPath, []byte(`declare -A IPSET_FILE=([sample]="sample.ipset" )`), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := LoadWithMigration(jsonPath, legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := requireLegacyEntry(t, st, "sample")
	if entry.File != "sample.ipset" {
		t.Fatalf("unexpected migrated file: %q", entry.File)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("expected migrated json cache: %v", err)
	}
}

func TestLoadWithMigrationKeepsExistingJSONCache(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".cache")
	jsonPath := filepath.Join(dir, ".cache.json")

	existing := New()
	existing.Entry("sample").File = "json.ipset"
	if err := Save(jsonPath, existing); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`declare -A IPSET_FILE=([sample]="legacy.ipset" )`), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := LoadWithMigration(jsonPath, legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := requireLegacyEntry(t, st, "sample")
	if entry.File != "json.ipset" {
		t.Fatalf("legacy cache replaced existing json entry: %q", entry.File)
	}
}

func TestLoadWithMigrationWithoutLegacyReturnsEmptyState(t *testing.T) {
	dir := t.TempDir()
	st, err := LoadWithMigration(filepath.Join(dir, ".cache.json"), filepath.Join(dir, ".cache"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(st.Entries); got != 0 {
		t.Fatalf("entries = %d, want 0", got)
	}
}

func TestLoadWithMigrationReportsLegacyErrors(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".cache")
	jsonPath := filepath.Join(dir, ".cache.json")
	if err := os.WriteFile(legacyPath, []byte(`declare -A IPSET_INFO=([sample]="unterminated )`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadWithMigration(jsonPath, legacyPath)
	if err == nil {
		t.Fatal("expected migration error")
	}
	if !strings.Contains(err.Error(), "unterminated double-quoted value") {
		t.Fatalf("migration error = %q", err.Error())
	}
}
