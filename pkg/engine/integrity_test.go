package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/insights"
)

// newIntegrityTestEngine builds an Engine stub pointing at temp
// directories with just enough state for CheckIntegrity to run.
// Populates BaseDir and WebDir; sets a minimal config with one
// plain source and one geoip provider so expectedSecondaryFiles
// produces a predictable list; seeds an empty cache.State so the
// check can populate ProcessedDate via markProcessed.
func newIntegrityTestEngine(t *testing.T) *Engine {
	t.Helper()
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "base")
	webDir := filepath.Join(tmp, "web")
	for _, dir := range []string{baseDir, webDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{
		Name:      "sample",
		URL:       "https://example.test/sample.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "ip",
	}
	cfg.Sources["geolite2_country"] = &config.Source{
		Name:   "geolite2_country",
		URL:    "https://example.test/geo.csv",
		Use:    []string{config.UseGeoIP},
		Format: "maxmind_country_csv",
	}
	cfg.Sources["iptoasn"] = &config.Source{
		Name:   "iptoasn",
		URL:    "https://example.test/asn.csv",
		Use:    []string{config.UseASN},
		Format: "iptoasn_csv",
	}
	cfg.Sources["fullbogons"] = &config.Source{
		Name:   "fullbogons",
		URL:    "https://example.test/bogons.txt",
		Use:    []string{config.UseBogons},
		Format: "ipset_plain",
	}
	return newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
	}))
}

// markProcessed seeds the engine's cache with a ProcessedDate for
// the given feed, simulating a prior finalize() run. The integrity
// check reads ProcessedDate as the reference "when was this last
// successfully processed" timestamp — writing a source file without
// updating the cache no longer counts as "processed" on its own.
func markProcessed(t *testing.T, eng *Engine, name string, processedAt time.Time) {
	t.Helper()
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.ProcessedDate = processedAt.Unix()
}

// writeFileForIntegrity writes a file with the given mtime.
func writeFileForIntegrity(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func writeJSONForIntegrity(t *testing.T, path string, payload any, mtime time.Time) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func mustMarshalForIntegrity(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeValidGeoPayloadForIntegrity(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	writeJSONForIntegrity(t, path, CountryComparisonPayload{
		TotalMapped: 4,
		Countries:   []CountryValue{{Code: "US", Value: 4}},
	}, mtime)
}

func seedIntegritySecondaries(t *testing.T, eng *Engine, name string, processedAt, mtime time.Time, invalid map[string][]byte) {
	t.Helper()
	baseDir := eng.runtime.BaseDir
	webDir := eng.runtime.WebDir

	writeFileForIntegrity(t, filepath.Join(baseDir, name+".ipset"), processedAt)
	markProcessed(t, eng, name, processedAt)

	for _, artifact := range eng.expectedSecondaryArtifacts(name) {
		path := filepath.Join(webDir, artifact.RelPath)
		if data, ok := invalid[artifact.RelPath]; ok {
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if artifact.Kind == secondaryArtifactGeo {
			writeValidGeoPayloadForIntegrity(t, path, mtime)
			continue
		}
		switch artifact.Kind {
		case secondaryArtifactMetadata:
			writeJSONForIntegrity(t, path, setMetadata{Name: name}, mtime)
		case secondaryArtifactRetention:
			writeJSONForIntegrity(t, path, RetentionData{Name: name}, mtime)
		case secondaryArtifactComparison:
			writeJSONForIntegrity(t, path, []CompareRow{{Name: "other", IPs: 1, Common: 1}}, mtime)
		case secondaryArtifactInsights:
			writeJSONForIntegrity(t, path, insightsPayload{Name: name}, mtime)
		case secondaryArtifactASN:
			writeJSONForIntegrity(t, path, asnFeedJSON{Provider: artifact.Provider}, mtime)
		case secondaryArtifactBogons:
			writeJSONForIntegrity(t, path, bogonFeedJSON{Provider: artifact.Provider}, mtime)
		case secondaryArtifactCriticalAggregate:
			writeJSONForIntegrity(t, path, criticalAggregateJSON{
				Feed:          name,
				ProviderSetID: eng.CriticalInfrastructureProviderSetID(),
				Complete:      true,
			}, mtime)
		case secondaryArtifactCriticalProvider:
			writeJSONForIntegrity(t, path, criticalProviderOverlapJSON{
				ProviderSetID: eng.CriticalInfrastructureProviderSetID(),
			}, mtime)
		default:
			writeFileForIntegrity(t, path, mtime)
		}
	}
}

// TestCheckIntegrityClean confirms a feed whose source file exists,
// cache reports it processed, and every secondary is at least as
// recent as ProcessedDate produces no findings.
func TestCheckIntegrityClean(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)

	// Source file mtime is irrelevant to the check now — write it
	// at a future time to prove the check ignores it.
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, nil)
	writeFileForIntegrity(t, filepath.Join(eng.runtime.BaseDir, "sample.ipset"), processedAt.Add(2*time.Hour))

	findings := eng.CheckIntegrity()
	if len(findings) != 0 {
		t.Fatalf("expected no findings on clean feed, got %d: %+v", len(findings), findings)
	}
}

func TestCheckIntegrityUsesWebDirOverride(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)
	defaultWebDir := eng.runtime.WebDir
	overrideWebDir := filepath.Join(t.TempDir(), "served-web")
	if err := os.MkdirAll(overrideWebDir, 0o700); err != nil {
		t.Fatal(err)
	}

	eng.runtime.WebDir = overrideWebDir
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, nil)
	eng.runtime.WebDir = defaultWebDir

	if findings := eng.CheckIntegrity(); len(findings) == 0 {
		t.Fatal("expected findings when default web dir lacks published secondaries")
	}
	if findings := eng.CheckIntegrityWithOptions(IntegrityOptions{WebDir: overrideWebDir}); len(findings) != 0 {
		t.Fatalf("expected override web dir to be clean, got %d findings: %+v", len(findings), findings)
	}
}

// TestCheckIntegrityDetectsMissing confirms every missing
// secondary file is enumerated in the finding.
func TestCheckIntegrityDetectsMissing(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	baseDir := eng.runtime.BaseDir
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	sourceMTime := processedAt.Add(2 * time.Hour)

	writeFileForIntegrity(t, filepath.Join(baseDir, "sample.ipset"), sourceMTime)
	markProcessed(t, eng, "sample", processedAt)
	// Do NOT write any secondary files.

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing secondaries, got %d", len(findings))
	}
	f := findings[0]
	if f.Feed != "sample" {
		t.Errorf("wrong feed: got %q", f.Feed)
	}
	expectedMissingCount := len(eng.expectedSecondaryFiles("sample"))
	if len(f.MissingFiles) != expectedMissingCount {
		t.Errorf("expected %d missing files, got %d (%v)", expectedMissingCount, len(f.MissingFiles), f.MissingFiles)
	}
	if !strings.Contains(f.Reason, "missing") {
		t.Errorf("reason should mention 'missing', got %q", f.Reason)
	}
	if !f.ProcessedAt.Equal(processedAt) {
		t.Errorf("expected processed_at %s, got %s", processedAt, f.ProcessedAt)
	}
	if !f.SourceFileMTime.Equal(sourceMTime) {
		t.Errorf("expected source_file_mtime %s, got %s", sourceMTime, f.SourceFileMTime)
	}
	if !f.SourceMTime.Equal(sourceMTime) {
		t.Errorf("expected source_mtime compatibility field %s, got %s", sourceMTime, f.SourceMTime)
	}
}

// TestCheckIntegrityDetectsStale confirms a secondary older than
// ProcessedDate is reported as stale.
func TestCheckIntegrityDetectsStale(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	baseDir := eng.runtime.BaseDir
	webDir := eng.runtime.WebDir
	processedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	older := processedAt.Add(-1 * time.Hour)

	writeFileForIntegrity(t, filepath.Join(baseDir, "sample.ipset"), processedAt)
	markProcessed(t, eng, "sample", processedAt)
	for _, f := range eng.expectedSecondaryFiles("sample") {
		// All files exist but their mtime is OLDER than ProcessedDate.
		writeFileForIntegrity(t, filepath.Join(webDir, f), older)
	}

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for stale secondaries, got %d", len(findings))
	}
	f := findings[0]
	if len(f.StaleFiles) == 0 {
		t.Errorf("expected stale files list populated, got %+v", f)
	}
	if len(f.MissingFiles) != 0 {
		t.Errorf("expected no missing files, got %v", f.MissingFiles)
	}
	if !strings.Contains(f.Reason, "stale") {
		t.Errorf("reason should mention 'stale', got %q", f.Reason)
	}
}

// TestCheckIntegrityMixedMissingAndStale confirms a feed with
// BOTH missing and stale files reports both categories and gets
// the combined reason string.
func TestCheckIntegrityMixedMissingAndStale(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	baseDir := eng.runtime.BaseDir
	webDir := eng.runtime.WebDir
	processedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	older := processedAt.Add(-1 * time.Hour)

	writeFileForIntegrity(t, filepath.Join(baseDir, "sample.ipset"), processedAt)
	markProcessed(t, eng, "sample", processedAt)
	expected := eng.expectedSecondaryFiles("sample")
	// Write the first half stale, leave the second half missing.
	for i, f := range expected {
		if i < len(expected)/2 {
			writeFileForIntegrity(t, filepath.Join(webDir, f), older)
		}
	}

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if len(f.MissingFiles) == 0 || len(f.StaleFiles) == 0 {
		t.Errorf("expected both missing and stale populated, got missing=%d stale=%d", len(f.MissingFiles), len(f.StaleFiles))
	}
	if !strings.Contains(f.Reason, "missing") || !strings.Contains(f.Reason, "stale") {
		t.Errorf("reason should mention both missing and stale, got %q", f.Reason)
	}
}

// TestCheckIntegritySkipsNeverProcessed confirms a feed with no
// cache entry (never successfully processed) and no enable
// marker is silently skipped.
func TestCheckIntegritySkipsNeverProcessed(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	// Do not write any source file and do not seed the cache.
	findings := eng.CheckIntegrity()
	if len(findings) != 0 {
		t.Fatalf("expected no findings when feed was never processed, got %d", len(findings))
	}
}

// TestCheckIntegrityFlagsEnabledButNeverProcessed confirms a feed
// with an enable marker but no cache ProcessedDate is reported
// as "enabled but never processed" — this catches feeds whose
// first run failed before finalize and never recovered.
func TestCheckIntegrityFlagsEnabledButNeverProcessed(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	baseDir := eng.runtime.BaseDir
	// Only the enable marker exists — no output, no cache entry.
	writeFileForIntegrity(t, filepath.Join(baseDir, "sample.enabled"), time.Now().UTC())

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for enabled-but-never-processed, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Reason, "never been successfully processed") {
		t.Errorf("unexpected reason: %q", findings[0].Reason)
	}
}

// TestCheckIntegrityFlagsMissingSourceWhenProcessed confirms a
// feed whose cache says it was processed but whose committed canonical feed body
// file is missing gets reported as a real inconsistency (someone
// deleted the output file from under us).
func TestCheckIntegrityFlagsMissingSourceWhenProcessed(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	markProcessed(t, eng, "sample", processedAt)
	// Do not write the committed canonical feed body. Also do not write
	// secondaries, so the output would otherwise be a flood of
	// missing-secondary findings too.

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing committed canonical feed body, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Reason, "committed canonical feed body missing") {
		t.Errorf("unexpected reason: %q", findings[0].Reason)
	}
}

func TestCheckIntegritySkipsMissingSourceWhenFeedIsUnavailableAndNoLocalRecoveryExists(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	eng.cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	eng.cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 120
	processedAt := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(time.Second)
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.Version = 2
	entry.ProcessedDate = processedAt.Unix()
	entry.SourceDate = processedAt.Unix()
	entry.CheckedDate = processedAt.Unix()
	entry.Entries = 1
	entry.FrequencyMinutes = 60
	entry.DownloadFailures = 4
	entry.FailureStartedDate = time.Now().UTC().Add(-7 * 24 * time.Hour).Unix()

	findings := eng.CheckIntegrity()
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unavailable feed without local recovery path, got %d: %+v", len(findings), findings)
	}
}

func TestIntegrityRecoveryPlanRechecksDownloadableFeedWhenCommittedSourceMissing(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	markProcessed(t, eng, "sample", processedAt)

	findings := eng.CheckIntegrity()
	recheck, reprocess := eng.IntegrityRecoveryPlan(findings)
	if len(recheck) != 1 || recheck[0] != "sample" {
		t.Fatalf("expected sample queued for recheck, got recheck=%v reprocess=%v", recheck, reprocess)
	}
	if len(reprocess) != 0 {
		t.Fatalf("expected no reprocess targets, got %v", reprocess)
	}
}

func TestIntegrityRecoveryPlanRebuildsDownloadableFeedWhenCommittedSourceExists(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	baseDir := eng.runtime.BaseDir
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	markProcessed(t, eng, "sample", processedAt)
	writeFileForIntegrity(t, filepath.Join(baseDir, "sample.ipset"), processedAt)

	findings := eng.CheckIntegrity()
	recheck, reprocess := eng.IntegrityRecoveryPlan(findings)
	if len(recheck) != 0 {
		t.Fatalf("expected no recheck targets, got %v", recheck)
	}
	if len(reprocess) != 1 || reprocess[0] != "sample" {
		t.Fatalf("expected sample queued for reprocess, got reprocess=%v", reprocess)
	}
}

func TestIntegrityRecoveryPlanRebuildsHistoryDerivativeWhenLocalStateExists(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.runtime.HistoryDir = filepath.Join(filepath.Dir(eng.runtime.BaseDir), "history")
	eng.runtime.LibDir = filepath.Join(filepath.Dir(eng.runtime.BaseDir), "lib")
	for _, dir := range []string{eng.runtime.HistoryDir, eng.runtime.LibDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	eng.cfg.Sources["sample_2d"] = &config.Source{
		Name:              "sample_2d",
		URL:               "internal://retention_window?parent=sample&minutes=2880",
		DerivedFrom:       []string{"sample"},
		Provenance:        config.ProvenanceSecondaryRetention,
		HistoryWindowDays: 2,
		Frequency:         0,
		IPV:               "ipv4",
		Output:            "ip",
	}

	processedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	parentUpdate := processedAt.Add(-30 * time.Minute)
	seedIntegritySecondaries(t, eng, "sample_2d", processedAt, processedAt.Add(10*time.Second), nil)
	seedIntegritySecondaries(t, eng, "sample", parentUpdate, parentUpdate.Add(10*time.Second), nil)
	writeFileForIntegrity(t, eng.feedBodyPath("sample_2d"), processedAt)
	writeFileForIntegrity(t, eng.feedBodyPath("sample"), parentUpdate)
	parentEntry := eng.state.Entry("sample")
	parentEntry.Name = "sample"
	parentEntry.SourceDate = parentUpdate.Unix()
	parentEntry.ProcessedDate = parentUpdate.Unix()
	snapshotPath := filepath.Join(eng.runtime.HistoryDir, "sample", fmt.Sprintf("%d.set", parentUpdate.Unix()))
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o700); err != nil {
		t.Fatal(err)
	}
	set := mustSet(t, "sample_history_snapshot")
	if err := writeBinaryPath(snapshotPath, set, parentUpdate); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(eng.runtime.WebDir, "sample_2d_insights.json")); err != nil {
		t.Fatal(err)
	}

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 derivative finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Feed != "sample_2d" {
		t.Fatalf("finding feed = %q, want sample_2d", findings[0].Feed)
	}

	recheck, reprocess := eng.IntegrityRecoveryPlan(findings)
	if len(recheck) != 0 {
		t.Fatalf("expected no parent recheck, got %v", recheck)
	}
	if len(reprocess) != 1 || reprocess[0] != "sample_2d" {
		t.Fatalf("expected sample_2d queued for reprocess, got recheck=%v reprocess=%v", recheck, reprocess)
	}
}

func TestIntegrityRecoveryPlanRechecksArtifactParentWhenChildSourceMissing(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg = config.New()
	eng.cfg.Artifacts["dronebl"] = &config.Artifact{
		Name:      "dronebl",
		Type:      config.ArtifactTypeDroneBLBuildzone,
		Frequency: 60,
	}
	eng.cfg.Sources["child"] = &config.Source{
		Name:           "child",
		URL:            "artifact://dronebl?parts=auto_botnets",
		ArtifactParent: "dronebl",
		Frequency:      0,
		IPV:            "ipv4",
		Output:         "ipset",
	}
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	markProcessed(t, eng, "child", processedAt)

	findings := eng.CheckIntegrity()
	recheck, reprocess := eng.IntegrityRecoveryPlan(findings)
	if len(recheck) != 1 || recheck[0] != "dronebl" {
		t.Fatalf("expected artifact parent queued for recheck, got recheck=%v reprocess=%v", recheck, reprocess)
	}
	if len(reprocess) != 0 {
		t.Fatalf("expected no direct reprocess targets, got %v", reprocess)
	}
}

func TestCheckIntegrityFlagsMissingHistoryDerivativeRollupAndRechecksParent(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.runtime.HistoryDir = filepath.Join(filepath.Dir(eng.runtime.BaseDir), "history")
	eng.runtime.LibDir = filepath.Join(filepath.Dir(eng.runtime.BaseDir), "lib")
	for _, dir := range []string{eng.runtime.HistoryDir, eng.runtime.LibDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	eng.cfg.Sources["sample_2d"] = &config.Source{
		Name:              "sample_2d",
		URL:               "internal://retention_window?parent=sample&minutes=2880",
		DerivedFrom:       []string{"sample"},
		Provenance:        config.ProvenanceSecondaryRetention,
		HistoryWindowDays: 2,
		Frequency:         0,
		IPV:               "ipv4",
		Output:            "ip",
	}
	processedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	parentUpdate := processedAt.Add(-30 * time.Minute)
	seedIntegritySecondaries(t, eng, "sample_2d", processedAt, processedAt.Add(10*time.Second), nil)
	seedIntegritySecondaries(t, eng, "sample", parentUpdate, parentUpdate.Add(10*time.Second), nil)
	writeFileForIntegrity(t, eng.feedBodyPath("sample"), parentUpdate)
	parentEntry := eng.state.Entry("sample")
	parentEntry.Name = "sample"
	parentEntry.SourceDate = parentUpdate.Unix()
	parentEntry.ProcessedDate = parentUpdate.Unix()

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 history-snapshot finding, got %d: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Feed != "sample_2d" {
		t.Fatalf("finding feed = %q, want sample_2d", finding.Feed)
	}
	if len(finding.BlockedFeeds) != 1 || finding.BlockedFeeds[0] != "sample" {
		t.Fatalf("blocked feeds = %v, want [sample]", finding.BlockedFeeds)
	}
	if len(finding.MissingFiles) != 1 || !strings.Contains(finding.MissingFiles[0], "history/sample/") {
		t.Fatalf("missing files = %v, want missing history snapshot", finding.MissingFiles)
	}
	recheck, reprocess := eng.IntegrityRecoveryPlan(findings)
	if len(recheck) != 1 || recheck[0] != "sample" {
		t.Fatalf("expected parent recheck, got recheck=%v reprocess=%v", recheck, reprocess)
	}
	if len(reprocess) != 0 {
		t.Fatalf("expected no reprocess targets, got %v", reprocess)
	}
}

func TestCheckIntegrityAllowsPartialHistoryDerivativeWindowWhenAvailableRollupExists(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.runtime.HistoryDir = filepath.Join(filepath.Dir(eng.runtime.BaseDir), "history")
	eng.runtime.LibDir = filepath.Join(filepath.Dir(eng.runtime.BaseDir), "lib")
	for _, dir := range []string{eng.runtime.HistoryDir, eng.runtime.LibDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	eng.cfg.Sources["sample_30d"] = &config.Source{
		Name:              "sample_30d",
		URL:               "internal://retention_window?parent=sample&minutes=43200",
		DerivedFrom:       []string{"sample"},
		Provenance:        config.ProvenanceSecondaryRetention,
		HistoryWindowDays: 30,
		Frequency:         0,
		IPV:               "ipv4",
		Output:            "ip",
	}
	processedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	parentUpdate := processedAt.Add(-30 * time.Minute)
	seedIntegritySecondaries(t, eng, "sample_30d", processedAt, processedAt.Add(10*time.Second), nil)
	seedIntegritySecondaries(t, eng, "sample", parentUpdate, parentUpdate.Add(10*time.Second), nil)
	writeFileForIntegrity(t, eng.feedBodyPath("sample"), parentUpdate)
	parentEntry := eng.state.Entry("sample")
	parentEntry.Name = "sample"
	parentEntry.SourceDate = parentUpdate.Unix()
	parentEntry.ProcessedDate = parentUpdate.Unix()
	snapshotPath := filepath.Join(eng.runtime.HistoryDir, "sample", fmt.Sprintf("%d.set", parentUpdate.Unix()))
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o700); err != nil {
		t.Fatal(err)
	}
	set := mustSet(t, "sample_history_snapshot")
	if err := writeBinaryPath(snapshotPath, set, parentUpdate); err != nil {
		t.Fatal(err)
	}

	findings := eng.CheckIntegrity()
	if len(findings) != 0 {
		t.Fatalf("expected no findings for partial but valid history window, got %+v", findings)
	}
}

// TestCheckIntegritySkipsDatabaseSources confirms geoip / asn
// databases are not included in the walk. They produce binary
// databases, not ipsets, and their secondary files follow a
// different layout.
func TestCheckIntegritySkipsDatabaseSources(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	baseDir := eng.runtime.BaseDir
	processedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	// Write ipset files for both the regular source AND the
	// geoip database source — the database source should still
	// be skipped because of its use:[geoip] role.
	writeFileForIntegrity(t, filepath.Join(baseDir, "sample.ipset"), processedAt)
	writeFileForIntegrity(t, filepath.Join(baseDir, "geolite2_country.ipset"), processedAt)
	markProcessed(t, eng, "sample", processedAt)
	markProcessed(t, eng, "geolite2_country", processedAt)

	findings := eng.CheckIntegrity()
	// Only sample should appear (with missing secondaries).
	for _, f := range findings {
		if f.Feed == "geolite2_country" {
			t.Errorf("database source should not be integrity-checked, found finding: %+v", f)
		}
	}
}

func TestExpectedSecondaryFilesIncludesMergeBogonProviders(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:      "cymru_unassigned",
		Frequency: 1440,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseBogons},
	}

	files := eng.expectedSecondaryFiles("sample")
	if !containsIntegrityFile(files, "sample_bogons_cymru_unassigned.json") {
		t.Fatalf("expected cymru_unassigned bogon artifact in %v", files)
	}
}

func TestExpectedSecondaryFilesSkipsCriticalArtifactsForIPv6Feeds(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["critical_dns"] = &config.Source{
		Name:   "critical_dns",
		IPV:    "ipv4",
		Output: "ip",
		Use:    []string{config.UseCriticalInfrastructure},
		Critical: &config.CriticalMetadata{
			Tier:          "hard",
			Role:          "public_dns_core",
			SourceType:    "curated_static",
			SourceQuality: "C",
			Rationale:     "test provider",
		},
	}
	eng.cfg.Sources["sample_v6"] = &config.Source{
		Name:   "sample_v6",
		IPV:    "ipv6",
		Output: "net",
	}

	v4Files := eng.expectedSecondaryFiles("sample")
	if !containsIntegrityFile(v4Files, "sample_critical_infrastructure.json") {
		t.Fatalf("expected IPv4 feed to require critical aggregate artifact in %v", v4Files)
	}

	v6Files := eng.expectedSecondaryFiles("sample_v6")
	for _, file := range v6Files {
		if strings.Contains(file, "_critical_") {
			t.Fatalf("IPv6 feed should not require critical artifacts in v1, got %v", v6Files)
		}
	}
}

func TestExpectedSecondaryFilesSkipsUnloadedCriticalProviderArtifacts(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["critical_dns"] = &config.Source{
		Name:   "critical_dns",
		IPV:    "ipv4",
		Output: "ip",
		Use:    []string{config.UseCriticalInfrastructure},
		Critical: &config.CriticalMetadata{
			Tier:          "hard",
			Role:          "public_dns_core",
			SourceType:    "curated_static",
			SourceQuality: "C",
			Rationale:     "test provider",
		},
	}

	files := eng.expectedSecondaryFiles("sample")
	if !containsIntegrityFile(files, "sample_critical_infrastructure.json") {
		t.Fatalf("expected critical aggregate artifact in %v", files)
	}
	if containsIntegrityFile(files, "sample_critical_critical_dns.json") {
		t.Fatalf("unloaded critical provider should not require per-provider artifact in %v", files)
	}

	path := filepath.Join(eng.runtime.BaseDir, "critical_dns.ipset")
	if err := os.WriteFile(path, []byte("1.1.1.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := eng.state.Entry("critical_dns")
	entry.Name = "critical_dns"
	entry.File = "critical_dns.ipset"

	files = eng.expectedSecondaryFiles("sample")
	if !containsIntegrityFile(files, "sample_critical_critical_dns.json") {
		t.Fatalf("loaded critical provider should require per-provider artifact in %v", files)
	}
}

func TestCheckIntegrityFlagsStaleCriticalProviderSetID(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["critical_dns"] = &config.Source{
		Name:   "critical_dns",
		IPV:    "ipv4",
		Output: "ip",
		Use:    []string{config.UseCriticalInfrastructure},
		Critical: &config.CriticalMetadata{
			Tier:          "hard",
			Role:          "public_dns_core",
			SourceType:    "curated_static",
			SourceQuality: "C",
			Rationale:     "test provider",
		},
	}
	providerEntry := eng.state.Entry("critical_dns")
	providerEntry.Name = "critical_dns"
	providerEntry.File = "critical_dns.ipset"
	providerEntry.Hash = "current-provider-body"
	providerEntry.Version = 7
	providerEntry.ProcessedDate = time.Now().UTC().Add(-30 * time.Minute).Unix()
	providerEntry.UniqueIPs = 2
	writeFileForIntegrity(t, filepath.Join(eng.runtime.BaseDir, "critical_dns.ipset"), time.Unix(providerEntry.ProcessedDate, 0))
	seedIntegritySecondaries(t, eng, "critical_dns", time.Unix(providerEntry.ProcessedDate, 0), time.Unix(providerEntry.ProcessedDate, 0).Add(10*time.Second), nil)

	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
		"sample_critical_infrastructure.json": []byte(`{"feed":"sample","provider_set_id":"stale","complete":true}`),
	})

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if len(findings[0].MalformedFiles) != 1 || findings[0].MalformedFiles[0] != "sample_critical_infrastructure.json" {
		t.Fatalf("expected stale critical aggregate to be malformed, got %+v", findings[0])
	}

	seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
		"sample_critical_critical_dns.json": []byte(`{"provider_set_id":"stale"}`),
	})
	findings = eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 provider finding, got %d: %+v", len(findings), findings)
	}
	if len(findings[0].MalformedFiles) != 1 || findings[0].MalformedFiles[0] != "sample_critical_critical_dns.json" {
		t.Fatalf("expected stale critical provider to be malformed, got %+v", findings[0])
	}
}

func TestValidateStructuredSecondaryDoesNotTreatCriticalSubstringAsProviderArtifact(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["orphan_critical_infrastructure"] = &config.Source{
		Name:   "orphan_critical_infrastructure",
		IPV:    "ipv4",
		Output: "ip",
	}
	eng.cfg.Sources["orphan_critical_critical_dns"] = &config.Source{
		Name:   "orphan_critical_critical_dns",
		IPV:    "ipv4",
		Output: "ip",
	}
	eng.cfg.Sources["critical_dns"] = &config.Source{
		Name:   "critical_dns",
		IPV:    "ipv4",
		Output: "ip",
		Use:    []string{config.UseCriticalInfrastructure},
		Critical: &config.CriticalMetadata{
			Tier:          "hard",
			Role:          "public_dns_core",
			SourceType:    "curated_static",
			SourceQuality: "C",
			Rationale:     "test provider",
		},
	}
	path := filepath.Join(t.TempDir(), "data_shield_critical_extra.json")
	if err := os.WriteFile(path, []byte(`{"name":"data_shield_critical_extra"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateStructuredSecondary("data_shield_critical_extra.json", path, map[string]struct{}{}, eng); err != nil {
		t.Fatalf("metadata file with _critical_ in feed name was treated as critical provider artifact: %v", err)
	}

	for _, name := range []string{"orphan_critical_infrastructure", "orphan_critical_critical_dns"} {
		path := filepath.Join(t.TempDir(), name+".json")
		if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"name":%q}`, name)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := validateStructuredSecondary(name+".json", path, map[string]struct{}{}, eng); err != nil {
			t.Fatalf("exact feed metadata %q was treated as generated critical artifact: %v", name, err)
		}
	}
}

func TestValidateStructuredSecondaryUsesExactCriticalProviderDescriptor(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	feedName := "cidr_report_bogons"
	providerName := "critical_as112"
	eng.cfg.Sources[feedName] = &config.Source{
		Name:   feedName,
		IPV:    "ipv4",
		Output: "ip",
	}
	eng.cfg.Sources[providerName] = &config.Source{
		Name:   providerName,
		IPV:    "ipv4",
		Output: "ip",
		Use:    []string{config.UseCriticalInfrastructure},
		Critical: &config.CriticalMetadata{
			Tier:          "hard",
			Role:          "dns_sink_infrastructure",
			SourceType:    "curated_static",
			SourceQuality: "A",
			Rationale:     "test provider",
		},
	}
	providerEntry := eng.state.Entry(providerName)
	providerEntry.Name = providerName
	providerEntry.File = providerName + ".ipset"
	providerEntry.Hash = "provider-body"
	providerEntry.Version = 1
	providerEntry.ProcessedDate = time.Now().UTC().Unix()
	providerEntry.UniqueIPs = 1

	rel := feedName + "_critical_" + providerName + ".json"
	path := filepath.Join(t.TempDir(), rel)
	payload := criticalProviderOverlapJSON{
		Provider:      CriticalInfrastructureProvider{Name: providerName, Tier: "hard"},
		ProviderSetID: eng.CriticalInfrastructureProviderSetID(),
	}
	writeJSONForIntegrity(t, path, payload, time.Now().UTC())

	if err := validateStructuredSecondary(rel, path, map[string]struct{}{}, eng); err != nil {
		t.Fatalf("valid critical provider artifact for feed containing _bogons was rejected: %v", err)
	}
}

func TestIntegrityRecoveryPlanRechecksMissingMergeBogonProvider(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:         "cymru_unassigned",
		URL:          "internal://merge?exclude=bogons&inputs=fullbogons",
		Frequency:    1440,
		IPV:          "ipv4",
		Output:       "netset",
		Use:          []string{config.UseBogons},
		DerivedFrom:  []string{"fullbogons", "bogons"},
		MergeSources: []string{"fullbogons"},
		MergeExclude: []string{"bogons"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}

	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	writeFileForIntegrity(t, filepath.Join(eng.runtime.BaseDir, "sample.ipset"), processedAt)
	markProcessed(t, eng, "sample", processedAt)

	findings := eng.CheckIntegrity()
	var sampleFinding *IntegrityFinding
	for i := range findings {
		if findings[i].Feed == "sample" {
			sampleFinding = &findings[i]
			break
		}
	}
	if sampleFinding == nil {
		t.Fatalf("expected sample finding, got %+v", findings)
	}
	if !containsString(sampleFinding.BlockedFeeds, "cymru_unassigned") {
		t.Fatalf("blocked feeds = %v, want cymru_unassigned", sampleFinding.BlockedFeeds)
	}

	recheck, reprocess := eng.IntegrityRecoveryPlan([]IntegrityFinding{*sampleFinding})
	if len(recheck) != 1 || recheck[0] != "cymru_unassigned" {
		t.Fatalf("expected cymru_unassigned queued for recheck, got recheck=%v reprocess=%v", recheck, reprocess)
	}
	if len(reprocess) != 1 || reprocess[0] != "sample" {
		t.Fatalf("expected sample queued for reprocess too, got %v", reprocess)
	}
}

func TestIntegrityBlockedFeedsIncludesUnavailableSubtractiveParent(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["bogons"] = &config.Source{
		Name:      "bogons",
		URL:       "https://example.test/bogons.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseBogons},
	}
	eng.cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:         "cymru_unassigned",
		URL:          "internal://merge?exclude=bogons&inputs=fullbogons",
		Frequency:    1440,
		IPV:          "ipv4",
		Output:       "netset",
		Use:          []string{config.UseBogons},
		DerivedFrom:  []string{"fullbogons", "bogons"},
		MergeSources: []string{"fullbogons"},
		MergeExclude: []string{"bogons"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}

	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.WriteFile(sourceEnablePathForRuntime(eng.runtime, "fullbogons"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeFileForIntegrity(t, eng.feedBodyPath("fullbogons"), processedAt)

	resolver := newEffectiveEntryResolver(eng.cfg, eng.state.SnapshotEntries())
	blocked := eng.integrityBlockedFeeds("cymru_unassigned", eng.cfg.Sources["cymru_unassigned"], resolver, false)
	if len(blocked) != 1 || blocked[0] != "bogons" {
		t.Fatalf("blocked feeds = %v, want [bogons]", blocked)
	}
}

func TestIntegrityBlockedFeedsSkipsSubtractiveParentsWhenNoAdditiveParentEligible(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["bogons"] = &config.Source{
		Name:      "bogons",
		URL:       "https://example.test/bogons.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseBogons},
	}
	eng.cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:         "cymru_unassigned",
		URL:          "internal://merge?exclude=bogons&inputs=fullbogons",
		Frequency:    1440,
		IPV:          "ipv4",
		Output:       "netset",
		Use:          []string{config.UseBogons},
		DerivedFrom:  []string{"fullbogons", "bogons"},
		MergeSources: []string{"fullbogons"},
		MergeExclude: []string{"bogons"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}

	resolver := newEffectiveEntryResolver(eng.cfg, eng.state.SnapshotEntries())
	blocked := eng.integrityBlockedFeeds("cymru_unassigned", eng.cfg.Sources["cymru_unassigned"], resolver, false)
	if len(blocked) != 0 {
		t.Fatalf("blocked feeds = %v, want none when the merge has no eligible additive parent", blocked)
	}
}

func TestIntegrityBlockedFeedsRespectsEnableAll(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["bogons"] = &config.Source{
		Name:      "bogons",
		URL:       "https://example.test/bogons.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseBogons},
	}
	eng.cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:         "cymru_unassigned",
		URL:          "internal://merge?exclude=bogons&inputs=fullbogons",
		Frequency:    1440,
		IPV:          "ipv4",
		Output:       "netset",
		Use:          []string{config.UseBogons},
		DerivedFrom:  []string{"fullbogons", "bogons"},
		MergeSources: []string{"fullbogons"},
		MergeExclude: []string{"bogons"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}

	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	writeFileForIntegrity(t, eng.feedBodyPath("fullbogons"), processedAt)
	resolver := newEffectiveEntryResolver(eng.cfg, eng.state.SnapshotEntries())

	blocked := eng.integrityBlockedFeeds("cymru_unassigned", eng.cfg.Sources["cymru_unassigned"], resolver, false)
	if len(blocked) != 0 {
		t.Fatalf("enableAll=false blocked feeds = %v, want none without enabled additive parents", blocked)
	}
	blocked = eng.integrityBlockedFeeds("cymru_unassigned", eng.cfg.Sources["cymru_unassigned"], resolver, true)
	if len(blocked) != 1 || blocked[0] != "bogons" {
		t.Fatalf("enableAll=true blocked feeds = %v, want [bogons]", blocked)
	}
}

func containsIntegrityFile(files []string, want string) bool {
	for _, file := range files {
		if file == want {
			return true
		}
	}
	return false
}

// TestCheckIntegritySkipsHiddenSources confirms hidden sources
// (rfc_reserved) are not included in the walk — they have no
// public outputs.
func TestCheckIntegritySkipsHiddenSources(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["hidden_source"] = &config.Source{
		Name:      "hidden_source",
		URL:       "https://example.test/hidden.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "ip",
		Hidden:    true,
	}
	baseDir := eng.runtime.BaseDir
	processedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	writeFileForIntegrity(t, filepath.Join(baseDir, "hidden_source.ipset"), processedAt)
	markProcessed(t, eng, "hidden_source", processedAt)

	findings := eng.CheckIntegrity()
	for _, f := range findings {
		if f.Feed == "hidden_source" {
			t.Errorf("hidden source should not be integrity-checked, found finding: %+v", f)
		}
	}
}

// TestCheckIntegrityIgnoresFutureSourceMTime is the regression
// test for the Last-Modified forward-stamping bug. Upstream feeds
// like dshield publish HTTP Last-Modified headers in the future
// relative to when we actually processed them; the pre-fix check
// trusted the source file mtime and reported every such feed as
// having stale secondaries forever. The check must use
// ProcessedDate (our wall clock), not the file mtime.
func TestCheckIntegrityIgnoresFutureSourceMTime(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	// Simulate the dshield scenario: we processed it an hour ago,
	// but the upstream's Last-Modified header is 2 hours in the
	// future, so the file mtime on disk is also 2h in the future.
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	futureMTime := time.Now().UTC().Add(2 * time.Hour)
	seedIntegritySecondaries(t, eng, "sample", processedAt, processedAt.Add(10*time.Second), nil)
	writeFileForIntegrity(t, filepath.Join(eng.runtime.BaseDir, "sample.ipset"), futureMTime)

	findings := eng.CheckIntegrity()
	if len(findings) != 0 {
		t.Fatalf("expected no findings — future file mtime must be ignored, got %d: %+v", len(findings), findings)
	}
}

func TestCheckIntegrityAcceptsLegacyCountryArray(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
		"sample_geolite2_country.json": mustMarshalForIntegrity(t, []CountryValue{{Code: "US", Value: 4}}),
	})

	findings := eng.CheckIntegrity()
	if len(findings) != 0 {
		t.Fatalf("expected no findings for accepted legacy country payload, got %d: %+v", len(findings), findings)
	}
}

func TestCheckIntegrityFlagsInvalidCountryPayload(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
		"sample_geolite2_country.json": []byte(`{"broken":true}`),
	})

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	found := false
	for _, rel := range findings[0].MalformedFiles {
		if rel == "sample_geolite2_country.json" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected invalid country payload to be reported as malformed, got %+v", findings[0])
	}
}

func TestCheckIntegrityFlagsMalformedStructuredSecondaries(t *testing.T) {
	tests := []struct {
		name string
		rel  string
		body string
	}{
		{name: "metadata", rel: "sample.json", body: `{"name":`},
		{name: "retention", rel: "sample_retention.json", body: `{"past":`},
		{name: "comparison", rel: "sample_comparison.json", body: `{"rows":`},
		{name: "insights", rel: "sample_insights.json", body: `{"items":`},
		{name: "asn", rel: "sample_asn_iptoasn.json", body: `{"provider":`},
		{name: "bogons", rel: "sample_bogons_fullbogons.json", body: `{"provider":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := newIntegrityTestEngine(t)
			processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
			later := processedAt.Add(10 * time.Second)
			seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
				tt.rel: []byte(tt.body),
			})

			findings := eng.CheckIntegrity()
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
			}
			if len(findings[0].MalformedFiles) != 1 || findings[0].MalformedFiles[0] != tt.rel {
				t.Fatalf("expected malformed file %q, got %+v", tt.rel, findings[0])
			}
			if !strings.Contains(findings[0].Reason, "malformed") {
				t.Fatalf("expected malformed reason, got %+v", findings[0])
			}
		})
	}
}

func TestCheckIntegrityFlagsNonRedistributableMetadataRawFields(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	redistributable := false
	eng.cfg.Sources["sample"].Redistributable = &redistributable
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
		"sample.json": []byte(`{
			"name": "sample",
			"file": "sample.ipset",
			"source": "https://example.test/source.txt",
			"file_local": "https://example.test/sample.ipset",
			"commit_history": "https://example.test/history/sample.ipset"
		}`),
	})

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if len(findings[0].MalformedFiles) != 1 || findings[0].MalformedFiles[0] != "sample.json" {
		t.Fatalf("expected sample.json malformed, got %+v", findings[0])
	}
}

func TestCheckIntegrityFlagsZeroOverlapComparisonRows(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
		"sample_comparison.json": []byte(`[{"name":"other","ips":10,"common":0}]` + "\n"),
	})

	findings := eng.CheckIntegrity()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if len(findings[0].MalformedFiles) != 1 || findings[0].MalformedFiles[0] != "sample_comparison.json" {
		t.Fatalf("expected sample_comparison.json malformed, got %+v", findings[0])
	}
}

func TestCheckIntegrityAcceptsNonEmptyInsightsPayload(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	later := processedAt.Add(10 * time.Second)
	seedIntegritySecondaries(t, eng, "sample", processedAt, later, map[string][]byte{
		"sample_insights.json": mustMarshalForIntegrity(t, insightsPayload{
			Name: "sample",
			Items: []insights.Insight{
				{
					Code:        "sample_rule",
					Section:     insights.SectionRetention,
					Headline:    "Sample insight",
					Methodology: "/methodology/sample",
				},
			},
		}),
	})

	findings := eng.CheckIntegrity()
	if len(findings) != 0 {
		t.Fatalf("expected no findings for valid non-empty insights payload, got %d: %+v", len(findings), findings)
	}
}
