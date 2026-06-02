package cache

import (
	"os"
	"path/filepath"
	"testing"
)

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
	entry := st.Entry("sample")
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
	if st.Entry("sample").File != "sample.ipset" {
		t.Fatalf("unexpected migrated file: %q", st.Entry("sample").File)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("expected migrated json cache: %v", err)
	}
}
