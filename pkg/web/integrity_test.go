package web

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestSanitizeIntegrityReportClampsOutOfRangeTimes(t *testing.T) {
	report := integrityReport{
		Status:      integrityStatusIssues,
		LastStarted: time.Date(12000, time.January, 1, 0, 0, 0, 0, time.UTC),
		LastEnded:   time.Date(12001, time.January, 1, 0, 0, 0, 0, time.UTC),
		Findings: []engine.IntegrityFinding{{
			Feed:            "sample",
			SourceMTime:     time.Date(12002, time.January, 1, 0, 0, 0, 0, time.UTC),
			SourceFileMTime: time.Date(12003, time.January, 1, 0, 0, 0, 0, time.UTC),
			ProcessedAt:     time.Date(12004, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
	}

	sanitized := sanitizeIntegrityReport(report)
	if !sanitized.LastStarted.IsZero() || !sanitized.LastEnded.IsZero() {
		t.Fatalf("expected top-level integrity times to be zero, got %+v", sanitized)
	}
	if !sanitized.Findings[0].SourceMTime.IsZero() || !sanitized.Findings[0].SourceFileMTime.IsZero() || !sanitized.Findings[0].ProcessedAt.IsZero() {
		t.Fatalf("expected finding times to be zero, got %+v", sanitized.Findings[0])
	}
	if _, err := json.Marshal(sanitized); err != nil {
		t.Fatalf("expected sanitized integrity report to marshal, got %v", err)
	}
}

func TestBuildIntegrityReportAnnotatesRecoveryMetadata(t *testing.T) {
	eng := newSettledIntegrityTestEngine(t)
	if err := os.WriteFile(
		filepath.Join(eng.Runtime().WebDir, "sample.json"),
		[]byte(`{"name":`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	report := buildIntegrityReport(eng, false, false, "")
	if report.Status != integrityStatusIssues {
		t.Fatalf("report status = %q, want %q", report.Status, integrityStatusIssues)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(report.Findings), report.Findings)
	}
	finding := report.Findings[0]
	if len(finding.MalformedFiles) != 1 || finding.MalformedFiles[0] != "sample.json" {
		t.Fatalf("malformed files = %v, want [sample.json]", finding.MalformedFiles)
	}
	if finding.RecoveryAction != engine.IntegrityRecoveryActionReprocess {
		t.Fatalf("recovery action = %q, want %q", finding.RecoveryAction, engine.IntegrityRecoveryActionReprocess)
	}
	if len(finding.RecoveryTargets) != 1 || finding.RecoveryTargets[0] != "sample" {
		t.Fatalf("recovery targets = %v, want [sample]", finding.RecoveryTargets)
	}
}

func TestBuildIntegrityReportExcludesArchivedFeedsUnlessRequested(t *testing.T) {
	eng := newSettledIntegrityTestEngine(t)
	now := time.Now().UTC()
	eng.Config().Runtime.FeedHealthSingleObservationGraceMins = 60
	eng.Config().Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	eng.Config().Runtime.FeedHealthDefaultRiskyCadenceMins = 60
	eng.Config().Runtime.FeedHealthArchivalThresholdMins = 60

	cachePath := filepath.Join(eng.Runtime().BaseDir, ".cache.json")
	state, err := cache.Load(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	entry := state.Entry("sample")
	entry.Name = "sample"
	entry.ProcessedDate = now.Add(-200 * time.Minute).Unix()
	entry.SourceDate = now.Add(-200 * time.Minute).Unix()
	entry.CheckedDate = now.Unix()
	entry.DownloadFailures = 5
	entry.FailureStartedDate = now.Add(-200 * time.Minute).Unix()
	entry.LastStatus = "download_failed"
	entry.Version = 3
	if err := cache.Save(cachePath, state); err != nil {
		t.Fatal(err)
	}
	eng, err = engine.New(eng.Runtime().ConfigPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.Config().Runtime.FeedHealthSingleObservationGraceMins = 60
	eng.Config().Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	eng.Config().Runtime.FeedHealthDefaultRiskyCadenceMins = 60
	eng.Config().Runtime.FeedHealthArchivalThresholdMins = 60
	if err := os.Remove(filepath.Join(eng.Runtime().WebDir, "sample.json")); err != nil {
		t.Fatal(err)
	}

	report := buildIntegrityReport(eng, false, false, "")
	if report.Count != 0 {
		t.Fatalf("archived feeds should be excluded by default, got %d findings", report.Count)
	}

	report = buildIntegrityReport(eng, true, false, "")
	if report.Count == 0 {
		t.Fatalf("include archived should surface findings, got none")
	}
}

func TestHandleAdminEntityIntegrityReturnsEntityFindings(t *testing.T) {
	eng := newEntityIntegrityTestEngine(t)
	if err := os.Remove(eng.PublicCountryIndexPath()); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/integrity/entities", nil)
	rec := httptest.NewRecorder()
	handleAdminEntityIntegrity(eng).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var report entityIntegrityReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if report.Status != integrityStatusIssues {
		t.Fatalf("status = %q, want %q", report.Status, integrityStatusIssues)
	}
	if report.Count == 0 {
		t.Fatalf("expected entity integrity findings, got %+v", report)
	}
	if report.Findings[0].Kind != "country_index_missing" {
		t.Fatalf("first finding kind = %q, want country_index_missing", report.Findings[0].Kind)
	}
}

func TestEntityIntegrityBusyDuringEngineRunOrEntityBackgroundTask(t *testing.T) {
	if !entityIntegrityBusy(engine.StatusSnapshot{Running: true}) {
		t.Fatal("entity integrity must be busy while the engine run is active")
	}
	if !entityIntegrityBusy(engine.StatusSnapshot{BackgroundTasks: []engine.BackgroundTaskSnapshot{{Name: "Entity artifacts refresh"}}}) {
		t.Fatal("entity integrity must be busy while entity background work is active")
	}
	if entityIntegrityBusy(engine.StatusSnapshot{BackgroundTasks: []engine.BackgroundTaskSnapshot{{Name: "Other maintenance"}}}) {
		t.Fatal("unrelated background work must not suppress entity integrity checks")
	}
}

func TestHandleAdminEntityIntegrityRebuildSchedulesBackgroundRebuild(t *testing.T) {
	eng := newEntityIntegrityTestEngine(t)
	countryIndexPath := eng.PublicCountryIndexPath()
	if err := os.Remove(countryIndexPath); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/integrity/entities/rebuild", nil)
	rec := httptest.NewRecorder()
	handleAdminEntityIntegrityRebuild(eng).ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var result entityIntegrityActionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Status != integrityStatusScheduled {
		t.Fatalf("status = %q, want %q", result.Status, integrityStatusScheduled)
	}

	waitForEntityRebuildOutput(t, eng, countryIndexPath, 2*time.Second)
}

func waitForEntityRebuildOutput(t *testing.T, eng *engine.Engine, path string, timeout time.Duration) {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		_, statErr := os.Stat(path)
		if statErr == nil && len(eng.StatusSnapshot().BackgroundTasks) == 0 {
			return
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			t.Fatalf("stat rebuilt entity artifact: %v", statErr)
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for queued entity rebuild to recreate %s", path)
		case <-ticker.C:
		}
	}
}

func TestHandleAdminIntegrityReprocessReturnsSplitTargets(t *testing.T) {
	eng := newSettledIntegrityTestEngine(t)
	if err := os.Remove(filepath.Join(eng.Runtime().BaseDir, "sample.ipset")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(eng.Runtime().WebDir, "sample.json")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/integrity/reprocess", nil)
	rec := httptest.NewRecorder()
	handler := handleAdminIntegrityReprocess(eng, scheduler.New(eng, true, nil), "")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var result integrityReprocessResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Status != integrityStatusScheduled {
		t.Fatalf("result status = %q, want %q", result.Status, integrityStatusScheduled)
	}
	if result.Count != 1 {
		t.Fatalf("result count = %d, want 1", result.Count)
	}
	if len(result.RecheckNames) != 1 || result.RecheckNames[0] != "sample" {
		t.Fatalf("recheck names = %v, want [sample]", result.RecheckNames)
	}
	if len(result.ReprocessNames) != 0 {
		t.Fatalf("reprocess names = %v, want none", result.ReprocessNames)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected returned findings")
	}
	var sampleFinding *engine.IntegrityFinding
	for i := range result.Findings {
		if result.Findings[i].Feed == "sample" {
			sampleFinding = &result.Findings[i]
			break
		}
	}
	if sampleFinding == nil {
		t.Fatalf("expected sample finding in %+v", result.Findings)
	}
	if sampleFinding.RecoveryAction != engine.IntegrityRecoveryActionRecheck {
		t.Fatalf("finding recovery action = %q, want %q", sampleFinding.RecoveryAction, engine.IntegrityRecoveryActionRecheck)
	}
	if len(sampleFinding.RecoveryTargets) != 1 || sampleFinding.RecoveryTargets[0] != "sample" {
		t.Fatalf("finding recovery targets = %v, want [sample]", sampleFinding.RecoveryTargets)
	}
}

func newSettledIntegrityTestEngine(t *testing.T) *engine.Engine {
	t.Helper()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n5.6.7.0/30\n"))
	}))
	t.Cleanup(sourceServer.Close)

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  sample:
    url: %q
    frequency: 1
    history: [60]
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}

	cachePath := filepath.Join(root, "base", ".cache.json")
	state, err := cache.Load(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	settledAt := time.Now().UTC().Add(-2 * time.Hour).Unix()
	for _, name := range []string{"sample", "sample_1h"} {
		entry := state.Entry(name)
		if entry.ProcessedDate == 0 {
			continue
		}
		entry.ProcessedDate = settledAt
		if entry.SourceDate == 0 {
			entry.SourceDate = settledAt
		}
		if entry.CheckedDate == 0 {
			entry.CheckedDate = settledAt
		}
	}
	if err := cache.Save(cachePath, state); err != nil {
		t.Fatal(err)
	}

	eng, err = engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func newEntityIntegrityTestEngine(t *testing.T) *engine.Engine {
	t.Helper()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.1.1.0/24\n"))
	}))
	t.Cleanup(sourceServer.Close)

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  sample:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
  dbip_country:
    url: https://example.test/geo.csv
    frequency: 1440
    hidden: true
    use: [geoip]
    format: dbip_country_csv
  iptoasn:
    url: https://example.test/asn.tsv
    frequency: 1440
    hidden: true
    use: [asn]
    format: iptoasn_combined_tsv
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "sample_dbip_country.json"), []byte(`{"total_mapped":256,"countries":[{"code":"US","value":256}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "sample_asn_iptoasn.json"), []byte(`{"by_asn":[{"asn":13335,"name":"CLOUDFLARENET","count":256}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeEntityGeoProviderFixture(filepath.Join(root, "lib", "geolocation", "dbip_country.source")); err != nil {
		t.Fatal(err)
	}
	if err := writeEntityASNProviderFixture(filepath.Join(root, "lib", "asn", "iptoasn", "database.tsv")); err != nil {
		t.Fatal(err)
	}
	if err := eng.RebuildEntityArtifacts(); err != nil {
		t.Fatal(err)
	}
	return eng
}

func writeEntityGeoProviderFixture(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var payload bytes.Buffer
	gw := gzip.NewWriter(&payload)
	if _, err := gw.Write([]byte("1.1.1.0,1.1.1.255,US\n")); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, payload.Bytes(), 0o644)
}

func writeEntityASNProviderFixture(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("1.1.1.0\t1.1.1.255\t13335\tUS\tCLOUDFLARENET\n"), 0o644)
}
