package web

import (
	"bytes"
	"compress/gzip"
	"context"
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
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	storePipelineIntegrityCache(t, eng, engine.IntegrityOptions{})

	report, err := buildIntegrityReport(context.Background(), eng, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
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

	storePipelineIntegrityCache(t, eng, engine.IntegrityOptions{})
	report, err := buildIntegrityReport(context.Background(), eng, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Count != 0 {
		t.Fatalf("archived feeds should be excluded by default, got %d findings", report.Count)
	}

	storePipelineIntegrityCache(t, eng, engine.IntegrityOptions{IncludeArchived: true})
	report, err = buildIntegrityReport(context.Background(), eng, true, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Count == 0 {
		t.Fatalf("include archived should surface findings, got none")
	}
}

func TestHandleAdminEntityIntegrityReturnsEntityFindings(t *testing.T) {
	eng := newEntityIntegrityTestEngine(t)
	if err := os.Remove(eng.PublicCountryIndexPath()); err != nil {
		t.Fatal(err)
	}
	storeEntityIntegrityCache(t, eng)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/integrity/entities", nil)
	rec := httptest.NewRecorder()
	handleAdminEntityIntegrity(context.Background(), eng).ServeHTTP(rec, req)
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

func TestAdminIntegrityGetReturnsColdPipelineCacheWithoutQueueingRefresh(t *testing.T) {
	eng, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{Feed: "sample"}}, nil)
	blockIntegrityEngineLane(t, eng)

	server := newWebHTTPTestServer(t, handler)
	var report integrityReport
	status, _ := server.getJSON(t, "/api/v1/admin/integrity?include_archived=true", &report)
	if status != http.StatusOK {
		t.Fatalf("pipeline integrity GET status = %d, want 200", status)
	}
	if report.Status != integrityStatusInProgress {
		t.Fatalf("status = %q, want %q: %+v", report.Status, integrityStatusInProgress, report)
	}
	if report.CacheState != engine.IntegrityCacheCold {
		t.Fatalf("cache_state = %q, want %q: %+v", report.CacheState, engine.IntegrityCacheCold, report)
	}
	if report.Queued || report.Running || report.Ticket != nil {
		t.Fatalf("GET must not queue refresh work, got %+v", report)
	}
	if report.Count != 0 || len(report.Findings) != 0 {
		t.Fatalf("cold cache must not report findings, got count=%d findings=%+v", report.Count, report.Findings)
	}
	assertNoQueuedEngineLaneWork(t, eng)
}

func TestAdminIntegrityGetReturnsStalePipelineCacheWithoutQueueingRefresh(t *testing.T) {
	eng, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{Feed: "sample"}}, nil)
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{EnableAll: true}, []engine.IntegrityFinding{{Feed: "sample"}}, nil)
	blockIntegrityEngineLane(t, eng)
	eng.MarkIntegrityCachesStale()

	server := newWebHTTPTestServer(t, handler)
	var report integrityReport
	status, _ := server.getJSON(t, "/api/v1/admin/integrity", &report)
	if status != http.StatusOK {
		t.Fatalf("pipeline integrity GET status = %d, want 200", status)
	}
	if report.Status != integrityStatusInProgress {
		t.Fatalf("status = %q, want %q: %+v", report.Status, integrityStatusInProgress, report)
	}
	if report.CacheState != engine.IntegrityCacheStale {
		t.Fatalf("cache_state = %q, want %q: %+v", report.CacheState, engine.IntegrityCacheStale, report)
	}
	if report.Queued || report.Running || report.Ticket != nil {
		t.Fatalf("GET must not queue refresh work, got %+v", report)
	}
	if report.Count != 0 || len(report.Findings) != 0 {
		t.Fatalf("stale cache findings must not be reported as current issues, got count=%d findings=%+v", report.Count, report.Findings)
	}
	assertNoQueuedEngineLaneWork(t, eng)
}

func TestHandleAdminEntityIntegrityGetReturnsColdCacheWithoutQueueingRefresh(t *testing.T) {
	eng := newEntityIntegrityTestEngine(t)
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{Feed: "sample"}}, nil)
	blockIntegrityEngineLane(t, eng)

	handler := newTestAdminHandler(t, eng)
	server := newWebHTTPTestServer(t, handler)

	var report entityIntegrityReport
	status, _ := server.getJSON(t, "/api/v1/admin/integrity/entities", &report)
	if status != http.StatusOK {
		t.Fatalf("entity integrity GET status = %d, want 200", status)
	}
	if report.Status != integrityStatusInProgress {
		t.Fatalf("status = %q, want %q: %+v", report.Status, integrityStatusInProgress, report)
	}
	if report.CacheState != engine.IntegrityCacheCold {
		t.Fatalf("cache_state = %q, want %q: %+v", report.CacheState, engine.IntegrityCacheCold, report)
	}
	if report.Queued || report.Running || report.Ticket != nil {
		t.Fatalf("GET must not queue refresh work, got %+v", report)
	}
	if report.Count != 0 || len(report.Findings) != 0 {
		t.Fatalf("cold cache must not report findings, got count=%d findings=%+v", report.Count, report.Findings)
	}
	assertNoQueuedEngineLaneWork(t, eng)
}

func TestAdminEntityIntegrityGetReturnsStaleCacheWithoutQueueingRefresh(t *testing.T) {
	eng := newEntityIntegrityTestEngine(t)
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{Feed: "sample"}}, nil)
	eng.StoreEntityIntegrityFindings([]engine.EntityIntegrityFinding{{
		Scope:   "global",
		Kind:    "config_newer",
		Subject: "entity_artifacts",
		Reason:  "test stale cache finding",
	}}, nil)
	blockIntegrityEngineLane(t, eng)
	eng.MarkIntegrityCachesStale()

	handler := newTestAdminHandler(t, eng)
	server := newWebHTTPTestServer(t, handler)

	var report entityIntegrityReport
	status, _ := server.getJSON(t, "/api/v1/admin/integrity/entities", &report)
	if status != http.StatusOK {
		t.Fatalf("entity integrity GET status = %d, want 200", status)
	}
	if report.Status != integrityStatusInProgress {
		t.Fatalf("status = %q, want %q: %+v", report.Status, integrityStatusInProgress, report)
	}
	if report.CacheState != engine.IntegrityCacheStale {
		t.Fatalf("cache_state = %q, want %q: %+v", report.CacheState, engine.IntegrityCacheStale, report)
	}
	if report.Queued || report.Running || report.Ticket != nil {
		t.Fatalf("GET must not queue refresh work, got %+v", report)
	}
	if report.Count != 0 || len(report.Findings) != 0 {
		t.Fatalf("stale cache findings must not be reported as current issues, got count=%d findings=%+v", report.Count, report.Findings)
	}
	assertNoQueuedEngineLaneWork(t, eng)
}

func TestHandleAdminEntityIntegrityRebuildSchedulesBackgroundRebuild(t *testing.T) {
	eng := newEntityIntegrityTestEngine(t)
	countryIndexPath := eng.PublicCountryIndexPath()
	if err := os.Remove(countryIndexPath); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/integrity/entities/rebuild", nil)
	rec := httptest.NewRecorder()
	handleAdminEntityIntegrityRebuildWithContext(t.Context(), eng).ServeHTTP(rec, req)
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

func TestAdminIntegrityRefreshRoutesQueueEngineLaneWork(t *testing.T) {
	eng, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{Feed: "sample"}}, nil)

	releaseLane := make(chan struct{})
	laneStarted := make(chan struct{})
	_, err := eng.QueuePipelineIntegrityReprocess(t.Context(), engine.IntegrityOptions{}, "test_block", func(ctx context.Context, _ []engine.IntegrityFinding) error {
		close(laneStarted)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseLane:
			return nil
		}
	})
	if err != nil {
		t.Fatalf("queue blocking integrity reprocess: %v", err)
	}
	<-laneStarted
	t.Cleanup(func() {
		close(releaseLane)
		waitForEngineLaneClear(t, eng, 2*time.Second)
	})

	server := newWebHTTPTestServer(t, handler)
	status, _, body := server.do(t, http.MethodPost, "/api/v1/admin/integrity/refresh", nil)
	if status != http.StatusAccepted {
		t.Fatalf("POST pipeline integrity refresh status = %d body=%s, want 202", status, body)
	}
	var pipelineResult integrityReprocessResult
	if err := json.Unmarshal(body, &pipelineResult); err != nil {
		t.Fatalf("decode pipeline refresh response: %v", err)
	}
	if pipelineResult.Status != integrityStatusScheduled || pipelineResult.Ticket == nil {
		t.Fatalf("pipeline refresh result = %+v, want scheduled lane ticket", pipelineResult)
	}
	if pipelineResult.Ticket.Kind != engine.LaneWorkIntegrityRefresh || pipelineResult.Ticket.Component != engine.LaneComponentPipelineIntegrity {
		t.Fatalf("pipeline refresh ticket = %+v, want pipeline integrity refresh", pipelineResult.Ticket)
	}

	status, _, body = server.do(t, http.MethodPost, "/api/v1/admin/integrity/entities/refresh", nil)
	if status != http.StatusAccepted {
		t.Fatalf("POST entity integrity refresh status = %d body=%s, want 202", status, body)
	}
	var entityResult entityIntegrityActionResult
	if err := json.Unmarshal(body, &entityResult); err != nil {
		t.Fatalf("decode entity refresh response: %v", err)
	}
	if entityResult.Status != integrityStatusScheduled || entityResult.Ticket == nil {
		t.Fatalf("entity refresh result = %+v, want scheduled lane ticket", entityResult)
	}
	if entityResult.Ticket.Kind != engine.LaneWorkIntegrityRefresh || entityResult.Ticket.Component != engine.LaneComponentEntityIntegrity {
		t.Fatalf("entity refresh ticket = %+v, want entity integrity refresh", entityResult.Ticket)
	}
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

func storePipelineIntegrityCache(t *testing.T, eng *engine.Engine, opts engine.IntegrityOptions) {
	t.Helper()
	findings, err := eng.CheckIntegrityWithOptionsContext(t.Context(), opts)
	if err != nil {
		t.Fatalf("pipeline integrity scan: %v", err)
	}
	eng.StorePipelineIntegrityFindings(opts, findings, nil)
}

func storeEntityIntegrityCache(t *testing.T, eng *engine.Engine) {
	t.Helper()
	findings, _, err := eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		t.Fatalf("entity integrity scan: %v", err)
	}
	eng.StoreEntityIntegrityFindings(findings, nil)
}

func blockIntegrityEngineLane(t *testing.T, eng *engine.Engine) {
	t.Helper()

	releaseLane := make(chan struct{})
	laneStarted := make(chan struct{})
	_, err := eng.QueuePipelineIntegrityReprocess(t.Context(), engine.IntegrityOptions{}, "test_block", func(ctx context.Context, _ []engine.IntegrityFinding) error {
		close(laneStarted)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseLane:
			return nil
		}
	})
	if err != nil {
		t.Fatalf("queue blocking integrity reprocess: %v", err)
	}
	<-laneStarted
	t.Cleanup(func() {
		close(releaseLane)
		waitForEngineLaneClear(t, eng, 2*time.Second)
	})
}

func assertNoQueuedEngineLaneWork(t *testing.T, eng *engine.Engine) {
	t.Helper()

	lane := eng.StatusSnapshotLight().EngineLane
	if lane.WaitingCount != 0 || len(lane.Waiting) != 0 {
		t.Fatalf("GET queued engine-lane work: waiting_count=%d waiting=%+v", lane.WaitingCount, lane.Waiting)
	}
}

func newTestAdminHandler(t *testing.T, eng *engine.Engine) http.Handler {
	t.Helper()

	return newHandler(eng, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}, scheduler.New(eng, true, nil))
}

func TestHandleAdminIntegrityReprocessReturnsSplitTargets(t *testing.T) {
	eng := newSettledIntegrityTestEngine(t)
	if err := os.Remove(filepath.Join(eng.Runtime().BaseDir, "sample.ipset")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(eng.Runtime().WebDir, "sample.json")); err != nil {
		t.Fatal(err)
	}
	storePipelineIntegrityCache(t, eng, engine.IntegrityOptions{EnableAll: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/integrity/reprocess", nil)
	rec := httptest.NewRecorder()
	handler := handleAdminIntegrityReprocess(context.Background(), eng, scheduler.New(eng, true, nil), "")
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
	if result.Ticket == nil {
		t.Fatal("expected integrity reprocess lane ticket")
	}
	if result.Ticket.Kind != engine.LaneWorkIntegrityReprocess {
		t.Fatalf("ticket kind = %q, want %q", result.Ticket.Kind, engine.LaneWorkIntegrityReprocess)
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
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
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
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(webDir, "sample_dbip_country.json"), []byte(`{"total_mapped":256,"countries":[{"code":"US","value":256}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "sample_asn_iptoasn.json"), []byte(`{"by_asn":[{"asn":13335,"name":"CLOUDFLARENET","count":256}]}`), 0o600); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
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
	return os.WriteFile(path, payload.Bytes(), 0o600)
}

func writeEntityASNProviderFixture(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("1.1.1.0\t1.1.1.255\t13335\tUS\tCLOUDFLARENET\n"), 0o600)
}
