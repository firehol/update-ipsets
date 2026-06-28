package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestBuildFeedManifestRequiresConfiguredProviderFanOutArtifacts(t *testing.T) {
	eng, _ := testHandlerWithProviderCatalog(t, Options{EnableAll: true})
	cfg := eng.Config()
	src := cfg.Sources["sample"]
	if src == nil {
		t.Fatal("sample source missing")
	}

	resp := buildFeedManifest("sample", src, cfg, eng.Runtime(), eng, 0)

	assertManifestRequiredFile(t, resp, "geo", "geodb", "sample_geodb.json")
	assertManifestRequiredFile(t, resp, "asn", "asndb", "sample_asn_asndb.json")
}

func assertManifestRequiredFile(t *testing.T, resp ManifestResponse, kind, provider, suffix string) {
	t.Helper()

	for _, file := range resp.Files {
		if file.Kind != kind || file.Provider != provider {
			continue
		}
		if !file.Required {
			t.Fatalf("%s/%s required = false, want true", kind, provider)
		}
		if got, want := filepath.Base(file.Rel), suffix; got != want {
			t.Fatalf("%s/%s rel basename = %q, want %q", kind, provider, got, want)
		}
		if got, want := filepath.Base(file.Path), suffix; got != want {
			t.Fatalf("%s/%s path basename = %q, want %q", kind, provider, got, want)
		}
		return
	}
	t.Fatalf("manifest missing %s/%s provider artifact", kind, provider)
}

func TestBuildFeedManifestUsesProviderSourceForDatabaseFeeds(t *testing.T) {
	eng, _ := testHandlerWithProviderCatalog(t, Options{EnableAll: true})
	cfg := eng.Config()

	cases := []struct {
		name       string
		wantSuffix string
	}{
		{name: "geodb", wantSuffix: "/geolocation/geodb.source"},
		{name: "asndb", wantSuffix: "/asn/asndb/source"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			src := cfg.Sources[tc.name]
			if src == nil {
				t.Fatalf("%s source missing", tc.name)
			}
			resp := buildFeedManifest(tc.name, src, cfg, eng.Runtime(), eng, 0)
			providerSource := requireManifestKind(t, resp, "provider_source")
			if !providerSource.Required {
				t.Fatalf("provider_source required = false, want true")
			}
			if !strings.HasSuffix(filepath.ToSlash(providerSource.Path), tc.wantSuffix) {
				t.Fatalf("provider_source path = %q, want suffix %q", providerSource.Path, tc.wantSuffix)
			}
			for _, forbidden := range []string{"canonical", "metadata", "binary"} {
				if file, ok := manifestKind(resp, forbidden); ok {
					t.Fatalf("database manifest unexpectedly includes %s file: %+v", forbidden, file)
				}
			}
		})
	}
}

func TestAdminFeedManifestUsesCachedSchedulerProcessedDate(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	eng, _ := testHandlerWithProviderCatalog(t, opts)
	processedAt := time.Unix(1_700_000_150, 0).UTC()
	if err := scheduler.SaveSnapshot(filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"), scheduler.Snapshot{
		GeneratedAt: processedAt,
		Items: []scheduler.Item{{
			Name:        "sample",
			ProcessedAt: processedAt,
		}},
	}); err != nil {
		t.Fatalf("write scheduler snapshot: %v", err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	var resp ManifestResponse
	status, _ := server.getJSON(t, "/api/v1/admin/feeds/sample/manifest", &resp)
	if status != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200", status)
	}
	if got, want := resp.ProcessedDate, processedAt.Unix(); got != want {
		t.Fatalf("manifest processed_date = %d, want cached scheduler value %d", got, want)
	}
}

func TestAdminFeedManifestTimesOutWhenFilesystemInspectionStalls(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	eng, _ := testHandlerWithProviderCatalog(t, opts)
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	blockStat := make(chan struct{})
	restoreSettings := setAdminManifestTestSettings(10*time.Millisecond, manifestFS{
		stat: func(string) (os.FileInfo, error) {
			<-blockStat
			return nil, os.ErrNotExist
		},
	})
	t.Cleanup(func() {
		close(blockStat)
		waitForAdminManifestSlot(t)
		restoreSettings()
	})

	started := time.Now()
	var body map[string]string
	status, _ := server.getJSON(t, "/api/v1/admin/feeds/sample/manifest", &body)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("manifest status = %d, want 503; body=%v", status, body)
	}
	if !strings.Contains(body["error"], "timed out") {
		t.Fatalf("manifest error = %q, want timeout", body["error"])
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("manifest timeout response took %s, want bounded response", elapsed)
	}
}

func TestAdminFeedManifestReportsBusyWhenInspectionAlreadyRunning(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	eng, _ := testHandlerWithProviderCatalog(t, opts)
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newHandler(eng, opts, runner))

	adminManifestBuildSlots <- struct{}{}
	t.Cleanup(func() {
		<-adminManifestBuildSlots
	})

	var body map[string]string
	status, _ := server.getJSON(t, "/api/v1/admin/feeds/sample/manifest", &body)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("manifest status = %d, want 503; body=%v", status, body)
	}
	if !strings.Contains(body["error"], "busy") {
		t.Fatalf("manifest error = %q, want busy", body["error"])
	}
}

func waitForAdminManifestSlot(t *testing.T) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case adminManifestBuildSlots <- struct{}{}:
			<-adminManifestBuildSlots
			return
		case <-deadline:
			t.Fatal("admin manifest slot remained busy")
		case <-ticker.C:
		}
	}
}

func setAdminManifestTestSettings(timeout time.Duration, fs manifestFS) func() {
	adminManifestSettingsMu.Lock()
	oldTimeout := adminManifestBuildTimeout
	oldFS := adminManifestFS
	adminManifestBuildTimeout = timeout
	if fs.stat != nil {
		adminManifestFS.stat = fs.stat
	}
	if fs.readDir != nil {
		adminManifestFS.readDir = fs.readDir
	}
	adminManifestSettingsMu.Unlock()

	return func() {
		adminManifestSettingsMu.Lock()
		adminManifestBuildTimeout = oldTimeout
		adminManifestFS = oldFS
		adminManifestSettingsMu.Unlock()
	}
}

func requireManifestKind(t *testing.T, resp ManifestResponse, kind string) ManifestFile {
	t.Helper()
	file, ok := manifestKind(resp, kind)
	if !ok {
		t.Fatalf("manifest missing %s file", kind)
	}
	return file
}

func manifestKind(resp ManifestResponse, kind string) (ManifestFile, bool) {
	for _, file := range resp.Files {
		if file.Kind == kind {
			return file, true
		}
	}
	return ManifestFile{}, false
}
