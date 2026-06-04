package engine

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

const asnProcessingSampleTSV = `1.0.0.0	1.0.0.255	13335	US	CLOUDFLARENET
8.8.8.0	8.8.8.255	15169	US	GOOGLE
`

func TestProcessASNDatabasesNoSources(t *testing.T) {
	eng := newEngineFixture(t)

	datasets, err := eng.processASNDatabases(t.Context(), RunOptions{})
	if err != nil {
		t.Fatalf("processASNDatabases() error = %v", err)
	}
	if datasets != nil {
		t.Fatalf("datasets = %#v, want nil without ASN sources", datasets)
	}
}

func TestProcessASNDatabasesRejectsInvalidFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{name: "unknown format", format: "not_registered"},
		{name: "wrong role format", format: "dbip_country_csv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := asnProcessingSource("provider", tt.format)
			eng := newASNProcessingEngine(t, src)

			datasets, err := eng.processASNDatabases(t.Context(), RunOptions{})
			if err != nil {
				t.Fatalf("processASNDatabases() error = %v", err)
			}
			if len(datasets) != 0 {
				t.Fatalf("datasets len = %d, want 0", len(datasets))
			}
			entry := eng.state.Entry(src.Name)
			if entry.LastStatus != "config_error" {
				t.Fatalf("last status = %q, want config_error", entry.LastStatus)
			}
			if entry.LastError != "unknown ASN format "+tt.format {
				t.Fatalf("last error = %q", entry.LastError)
			}
			if entry.Category != "asn" || entry.Info != "ASN provider" || entry.MaintainerURL != "https://example.test/provider" {
				t.Fatalf("provider source metadata was not applied: %+v", entry)
			}
		})
	}
}

func TestProcessASNDatabasesMarksUnavailableWhenDatabaseMissing(t *testing.T) {
	format := "test_asn_no_extract"
	withASNProcessingFormat(t, format, formatSpec{role: formatRoleASN, dataFile: "database.tsv"})
	src := asnProcessingSource("provider", format)
	eng := newASNProcessingEngine(t, src)

	datasets, err := eng.processASNDatabases(t.Context(), RunOptions{})
	if err != nil {
		t.Fatalf("processASNDatabases() error = %v", err)
	}
	if len(datasets) != 0 {
		t.Fatalf("datasets len = %d, want 0", len(datasets))
	}
	entry := eng.state.Entry(src.Name)
	if entry.LastStatus != "unavailable" {
		t.Fatalf("last status = %q, want unavailable", entry.LastStatus)
	}
	if !strings.Contains(entry.LastError, "database file not found") {
		t.Fatalf("last error = %q, want database file not found", entry.LastError)
	}
}

func TestProcessASNDatabasesLoadsExistingDatabase(t *testing.T) {
	src := asnProcessingSource("provider", "iptoasn_combined_tsv")
	eng := newASNProcessingEngine(t, src)
	writeASNProcessingDatabase(t, eng, src.Name, "database.tsv", asnProcessingSampleTSV)

	datasets, err := eng.processASNDatabases(t.Context(), RunOptions{})
	if err != nil {
		t.Fatalf("processASNDatabases() error = %v", err)
	}
	defer datasets.closeAll(eng.logger)
	if len(datasets) != 1 || datasets[src.Name] == nil {
		t.Fatalf("datasets = %#v, want one loaded provider", datasets)
	}
	entry := eng.state.Entry(src.Name)
	if entry.LastStatus != "updated" || entry.LastError != "" {
		t.Fatalf("provider status = %q error %q, want updated without error", entry.LastStatus, entry.LastError)
	}
	if entry.Entries != 2 || entry.UniqueIPs != 512 {
		t.Fatalf("provider stats = entries %d unique %d, want 2 and 512", entry.Entries, entry.UniqueIPs)
	}
	if entry.FrequencyMinutes != src.Frequency || entry.PublicURL != src.URL {
		t.Fatalf("provider metadata = %+v, want configured frequency and URL", entry)
	}
}

func TestProcessASNDatabasesUsesStagedArchiveAsFreshLoad(t *testing.T) {
	src := asnProcessingSource("provider", "iptoasn_combined_tsv")
	eng := newASNProcessingEngine(t, src)
	entry := eng.state.Entry(src.Name)
	entry.DownloadFailures = 3

	providerDir := filepath.Join(eng.runtime.LibDir, "asn", src.Name)
	if err := os.MkdirAll(providerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	stagedArchive := stagedPath(filepath.Join(providerDir, "source"))
	writeGzipASNProcessingFixture(t, stagedArchive, asnProcessingSampleTSV)
	archiveTime := time.Unix(2500, 0).UTC()
	if err := os.Chtimes(stagedArchive, archiveTime, archiveTime); err != nil {
		t.Fatal(err)
	}

	datasets, err := eng.processASNDatabases(t.Context(), RunOptions{})
	if err != nil {
		t.Fatalf("processASNDatabases() error = %v", err)
	}
	defer datasets.closeAll(eng.logger)
	if len(datasets) != 1 {
		t.Fatalf("datasets len = %d, want 1", len(datasets))
	}
	if entry.LastStatus != "updated" || entry.LastError != "" {
		t.Fatalf("provider status = %q error %q, want staged load to clear stale failure", entry.LastStatus, entry.LastError)
	}
	if entry.SourceDate != archiveTime.Unix() {
		t.Fatalf("source date = %d, want %d", entry.SourceDate, archiveTime.Unix())
	}
	if !fileExists(filepath.Join(providerDir, "database.tsv")) {
		t.Fatal("staged archive was not extracted to database.tsv")
	}
}

func TestProcessASNDatabasesReturnsOpenFailure(t *testing.T) {
	good := asnProcessingSource("aa_good", "iptoasn_combined_tsv")
	bad := asnProcessingSource("zz_bad", "iptoasn_combined_tsv")
	eng := newASNProcessingEngine(t, good, bad)
	writeASNProcessingDatabase(t, eng, good.Name, "database.tsv", asnProcessingSampleTSV)
	writeASNProcessingDatabase(t, eng, bad.Name, "database.tsv", "1.2.3.0\t1.2.3.255\t13335\n")

	datasets, err := eng.processASNDatabases(t.Context(), RunOptions{})
	if err == nil {
		t.Fatal("processASNDatabases() error = nil, want open failure")
	}
	if !strings.Contains(err.Error(), "asn open zz_bad") {
		t.Fatalf("error = %v, want zz_bad open context", err)
	}
	if datasets != nil {
		t.Fatalf("datasets = %#v, want nil on fatal open error", datasets)
	}
	if got := eng.state.Entry(good.Name).LastStatus; got != "updated" {
		t.Fatalf("good provider status = %q, want updated before later failure", got)
	}
	if got := eng.state.Entry(bad.Name).LastStatus; got != "open_failed" {
		t.Fatalf("bad provider status = %q, want open_failed", got)
	}
}

func newASNProcessingEngine(t *testing.T, sources ...*config.Source) *Engine {
	t.Helper()
	cfg := config.New()
	cfg.SourceOrder = make([]string, 0, len(sources))
	for _, src := range sources {
		cfg.Sources[src.Name] = src
		cfg.SourceOrder = append(cfg.SourceOrder, src.Name)
	}
	return newEngineFixture(t, withConfig(cfg), withNow(func() time.Time {
		return time.Unix(2000, 0).UTC()
	}))
}

func asnProcessingSource(name, format string) *config.Source {
	return &config.Source{
		Name:          name,
		URL:           "https://example.test/" + name,
		Use:           []string{config.UseASN},
		Format:        format,
		Frequency:     1440,
		Info:          "ASN provider",
		Maintainer:    "Example maintainer",
		MaintainerURL: "https://example.test/provider",
	}
}

func writeASNProcessingDatabase(t *testing.T, eng *Engine, provider, file, body string) {
	t.Helper()
	providerDir := filepath.Join(eng.runtime.LibDir, "asn", provider)
	if err := os.MkdirAll(providerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providerDir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeGzipASNProcessingFixture(t *testing.T, path, body string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(body)); err != nil {
		_ = gz.Close()
		_ = f.Close()
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func withASNProcessingFormat(t *testing.T, name string, spec formatSpec) {
	t.Helper()
	old, existed := formatRegistry[name]
	formatRegistry[name] = spec
	t.Cleanup(func() {
		if existed {
			formatRegistry[name] = old
			return
		}
		delete(formatRegistry, name)
	})
}
