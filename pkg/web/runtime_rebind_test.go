package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestPublicArtifactRoutesRebindWebDirAfterRuntimeReload(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldWebDir, "sample.json", `{"version":"old"}`+"\n")
	writePublicServingFile(t, newWebDir, "sample.json", `{"version":"new"}`+"\n")

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK {
		t.Fatalf("initial /sample.json status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != `{"version":"old"}`+"\n" {
		t.Fatalf("initial /sample.json body = %q, want old artifact", got)
	}

	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusOK {
		t.Fatalf("reloaded /sample.json status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != `{"version":"new"}`+"\n" {
		t.Fatalf("reloaded /sample.json body = %q, want new artifact", got)
	}
}

func TestPublicArtifactRoutesReloadToEmptyWebDirDoesNotFallback(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldWebDir, "sample.json", `{"version":"old"}`+"\n")

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"old"}`+"\n" {
		t.Fatalf("initial /sample.json status=%d body=%q, want old artifact", status, body)
	}

	if err := os.MkdirAll(newWebDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusNotFound {
		t.Fatalf("reloaded empty-root /sample.json status=%d body=%q, want 404 without old-root fallback", status, body)
	}
}

func TestPublicArtifactRoutesRebindBaseDirWhenWebDirIsEmpty(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldBaseDir := filepath.Join(root, "base-old")
	newBaseDir := filepath.Join(root, "base-new")
	writePublicServingReloadConfigWithBase(t, cfgPath, root, oldBaseDir, "", "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldBaseDir, "sample.json", `{"version":"old-base"}`+"\n")
	writePublicServingFile(t, newBaseDir, "sample.json", `{"version":"new-base"}`+"\n")

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"old-base"}`+"\n" {
		t.Fatalf("initial base-root artifact status=%d body=%q, want old base artifact", status, body)
	}

	writePublicServingReloadConfigWithBase(t, cfgPath, root, newBaseDir, "", "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"new-base"}`+"\n" {
		t.Fatalf("reloaded base-root artifact status=%d body=%q, want new base artifact", status, body)
	}
}

func TestPublicArtifactRoutesReloadWhileRequestsAreInFlight(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	oldBody := `{"version":"old"}` + "\n"
	newBody := `{"version":"new"}` + "\n"
	writePublicServingFile(t, oldWebDir, "sample.json", oldBody)
	writePublicServingFile(t, newWebDir, "sample.json", newBody)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	oldSeen := make(chan struct{}, 1)
	newSeen := make(chan struct{}, 1)
	errs := make(chan error, 1)

	reportErr := func(err error) {
		select {
		case errs <- err:
		default:
		}
		cancel()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.server.URL+"/sample.json", nil)
			if err != nil {
				reportErr(err)
				return
			}
			resp, err := server.client.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				reportErr(err)
				return
			}
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil {
				reportErr(readErr)
				return
			}
			if closeErr != nil {
				reportErr(closeErr)
				return
			}
			if resp.StatusCode != http.StatusOK {
				reportErr(fmt.Errorf("GET /sample.json status=%d body=%q", resp.StatusCode, body))
				return
			}
			switch got := string(body); got {
			case oldBody:
				select {
				case oldSeen <- struct{}{}:
				default:
				}
			case newBody:
				select {
				case newSeen <- struct{}{}:
				default:
				}
			default:
				reportErr(fmt.Errorf("GET /sample.json body=%q, want old or new artifact", got))
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	waitForReloadServingObservation(t, oldSeen, errs, "old artifact before reload")

	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	waitForReloadServingObservation(t, newSeen, errs, "new artifact after reload")

	cancel()
	wg.Wait()
	select {
	case err := <-errs:
		t.Fatal(err)
	default:
	}
}

func TestRawFeedRoutesRebindIPSetsDirAfterRuntimeReload(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldFilesDir := filepath.Join(root, "files-old")
	newFilesDir := filepath.Join(root, "files-new")
	writePublicServingReloadConfig(t, cfgPath, root, filepath.Join(root, "web"), oldFilesDir, "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldFilesDir, "sample.ipset", "old raw\n")
	writePublicServingFile(t, newFilesDir, "sample.ipset", "new raw\n")

	status, _, body := server.get(t, "/files/sample.ipset")
	if status != http.StatusOK {
		t.Fatalf("initial raw status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != "old raw\n" {
		t.Fatalf("initial raw body = %q, want old mirror", got)
	}

	writePublicServingReloadConfig(t, cfgPath, root, filepath.Join(root, "web"), newFilesDir, "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/files/sample.ipset")
	if status != http.StatusOK {
		t.Fatalf("reloaded raw status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != "new raw\n" {
		t.Fatalf("reloaded raw body = %q, want new mirror", got)
	}
}

func TestRawFeedRoutesRebindBaseDirFallbackAfterRuntimeReload(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldBaseDir := filepath.Join(root, "base-old")
	newBaseDir := filepath.Join(root, "base-new")
	writePublicServingReloadConfigWithBase(t, cfgPath, root, oldBaseDir, filepath.Join(root, "web"), "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldBaseDir, "sample.ipset", "old base raw\n")
	writePublicServingFile(t, newBaseDir, "sample.ipset", "new base raw\n")

	status, _, body := server.get(t, "/files/sample.ipset")
	if status != http.StatusOK {
		t.Fatalf("initial base fallback raw status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != "old base raw\n" {
		t.Fatalf("initial base fallback raw body = %q, want old base", got)
	}

	writePublicServingReloadConfigWithBase(t, cfgPath, root, newBaseDir, filepath.Join(root, "web"), "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/files/sample.ipset")
	if status != http.StatusOK {
		t.Fatalf("reloaded base fallback raw status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != "new base raw\n" {
		t.Fatalf("reloaded base fallback raw body = %q, want new base", got)
	}
}

func TestMCPFetchAnalysisRebindsWebDirAfterRuntimeReload(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldWebDir, "sample.md", "# old analysis\n")
	writePublicServingFile(t, newWebDir, "sample.md", "# new analysis\n")

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	mcpClient := newMCPHTTPClient(t, ctx, server.server.URL+"/mcp")

	if got := fetchAnalysisText(t, ctx, mcpClient, "sample"); got != "# old analysis\n" {
		t.Fatalf("initial MCP analysis = %q, want old markdown", got)
	}

	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := fetchAnalysisText(t, ctx, mcpClient, "sample"); got != "# new analysis\n" {
		t.Fatalf("reloaded MCP analysis = %q, want new markdown", got)
	}
}

func TestPublicRuntimeOverrideKeepsServedWebDirOnReload(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	configuredOldWebDir := filepath.Join(root, "configured-web-old")
	configuredNewWebDir := filepath.Join(root, "configured-web-new")
	overrideWebDir := filepath.Join(root, "served-web")
	writePublicServingReloadConfig(t, cfgPath, root, configuredOldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true, WebDir: overrideWebDir})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, configuredOldWebDir, "sample.json", `{"version":"configured-old"}`+"\n")
	writePublicServingFile(t, configuredNewWebDir, "sample.json", `{"version":"configured-new"}`+"\n")
	writePublicServingFile(t, overrideWebDir, "sample.json", `{"version":"override"}`+"\n")

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK {
		t.Fatalf("initial override /sample.json status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != `{"version":"override"}`+"\n" {
		t.Fatalf("initial override /sample.json body = %q, want override artifact", got)
	}

	writePublicServingReloadConfig(t, cfgPath, root, configuredNewWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusOK {
		t.Fatalf("reloaded override /sample.json status = %d, want 200 body=%s", status, body)
	}
	if got := string(body); got != `{"version":"override"}`+"\n" {
		t.Fatalf("reloaded override /sample.json body = %q, want override artifact", got)
	}
}

func TestAdminIntegrityRouteRebindsWebDirAfterRuntimeReload(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, err := engine.New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newAdminHandler(eng, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}, runner))

	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{EnableAll: true, WebDir: oldWebDir}, []engine.IntegrityFinding{{Feed: "old-root"}}, nil)
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{EnableAll: true, WebDir: newWebDir}, []engine.IntegrityFinding{{Feed: "new-root"}}, nil)

	status, _, body := server.get(t, "/api/v1/admin/integrity")
	if status != http.StatusOK {
		t.Fatalf("initial admin integrity status=%d body=%s, want 200", status, body)
	}
	assertIntegrityReportFeed(t, body, "old-root")

	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{EnableAll: true, WebDir: newWebDir}, []engine.IntegrityFinding{{Feed: "new-root"}}, nil)

	status, _, body = server.get(t, "/api/v1/admin/integrity")
	if status != http.StatusOK {
		t.Fatalf("reloaded admin integrity status=%d body=%s, want 200", status, body)
	}
	assertIntegrityReportFeed(t, body, "new-root")
}

func TestPublicFileCacheLimitReloadPublishesFreshCache(t *testing.T) {
	assertPublicFileCacheLimitReloadPublishesFreshCache(
		t,
		"  web_artifact_cache_max_file_bytes: 1024\n",
		"  web_artifact_cache_max_file_bytes: 1\n",
	)
}

func TestPublicFileCacheEntryLimitReloadPublishesFreshCache(t *testing.T) {
	assertPublicFileCacheLimitReloadPublishesFreshCache(
		t,
		"  web_artifact_cache_max_entries: 32\n",
		"  web_artifact_cache_max_entries: 1\n",
	)
}

func assertPublicFileCacheLimitReloadPublishesFreshCache(t *testing.T, initialRuntimeExtra, reloadRuntimeExtra string) {
	t.Helper()
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	webDir := filepath.Join(root, "web")
	writePublicServingReloadConfig(t, cfgPath, root, webDir, "", initialRuntimeExtra)

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	stamp := time.Unix(1_700_000_000, 0)
	oldBody := `{"version":"old"}` + "\n"
	newBody := `{"version":"new"}` + "\n"
	writePublicServingFileAt(t, webDir, "sample.json", oldBody, stamp)

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != oldBody {
		t.Fatalf("initial cache-limit artifact status=%d body=%q", status, body)
	}

	// Same path, size, and mtime proves the old cache generation would be
	// stale if reload did not publish a fresh cache after the limit changed.
	writePublicServingFileAt(t, webDir, "sample.json", newBody, stamp)
	writePublicServingReloadConfig(t, cfgPath, root, webDir, "", reloadRuntimeExtra)
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != newBody {
		t.Fatalf("reloaded cache-limit artifact status=%d body=%q, want disk body after fresh cache", status, body)
	}
}

func assertIntegrityReportFeed(t *testing.T, body []byte, want string) {
	t.Helper()
	report := decodeTestJSON[integrityReport](t, body)
	if report.Count != 1 || len(report.Findings) != 1 {
		t.Fatalf("integrity report findings = %+v, want exactly one finding for %q", report.Findings, want)
	}
	if got := report.Findings[0].Feed; got != want {
		t.Fatalf("integrity report feed = %q, want %q", got, want)
	}
}

func TestPublicServingStateFollowsReloadAfterListenerFailure(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldWebDir, "sample.json", `{"version":"old"}`+"\n")
	writePublicServingFile(t, newWebDir, "sample.json", `{"version":"new"}`+"\n")
	eng.RegisterReloadPublicationListener("test.post_publication_failure", func(engine.ReloadPublication) error {
		return errors.New("post-publication listener failed")
	})

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"old"}`+"\n" {
		t.Fatalf("initial listener-failure artifact status=%d body=%q, want old artifact", status, body)
	}

	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"new"}`+"\n" {
		t.Fatalf("reloaded listener-failure artifact status=%d body=%q, want new artifact", status, body)
	}
	if got := eng.StatusSnapshotLight().LastConfigReloadError; !strings.Contains(got, "post-publication listener failed") {
		t.Fatalf("last config reload error = %q, want listener diagnostic", got)
	}
}

func TestPublicServingStateFollowsReloadWhenCleanupQueueFails(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldWebDir, "sample.json", `{"version":"old"}`+"\n")
	writePublicServingFile(t, newWebDir, "sample.json", `{"version":"new"}`+"\n")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	eng.RegisterReloadPublicationListener("zzz.cancel_after_publication", func(engine.ReloadPublication) error {
		cancel()
		return nil
	})

	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReloadContext error = %v, want context canceled from post-publication cleanup queue", err)
	}

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"new"}`+"\n" {
		t.Fatalf("artifact after post-publication cleanup failure status=%d body=%q, want new artifact", status, body)
	}
	if got := eng.StatusSnapshotLight().LastConfigReloadError; !strings.Contains(got, context.Canceled.Error()) {
		t.Fatalf("last config reload error = %q, want cleanup queue diagnostic", got)
	}
}

func newPublicServingReloadHandler(t *testing.T, cfgPath string, opts Options) (*engine.Engine, http.Handler) {
	t.Helper()
	eng, err := engine.New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	runner := scheduler.New(eng, true, nil)
	return eng, newHandler(eng, opts, runner)
}

func writePublicServingReloadConfig(t *testing.T, cfgPath, root, webDir, filesDir, runtimeExtra string) {
	t.Helper()
	writePublicServingReloadConfigWithBase(t, cfgPath, root, filepath.Join(root, "base"), webDir, filesDir, runtimeExtra)
}

func writePublicServingReloadConfigWithBase(t *testing.T, cfgPath, root, baseDir, webDir, filesDir, runtimeExtra string) {
	t.Helper()
	filesLine := ""
	if filesDir != "" {
		filesLine = fmt.Sprintf("  web_dir_for_ipsets: %q\n", filesDir)
	}
	cfg := fmt.Sprintf(`runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
%s  cache_dir: %q
  tmp_dir: %q
  ipsets_apply: false
%ssources:
  sample:
    static:
      - 192.0.2.1
    frequency: 1
    history: [60]
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: tests
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filesLine, filepath.Join(root, "cache"), filepath.Join(root, "tmp"), runtimeExtra)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writePublicServingFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writePublicServingFileAt(t *testing.T, root, rel, body string, modTime time.Time) {
	t.Helper()
	writePublicServingFile(t, root, rel, body)
	path := filepath.Join(root, rel)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func waitForReloadServingObservation(t *testing.T, observations <-chan struct{}, errs <-chan error, label string) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-observations:
	case err := <-errs:
		t.Fatal(err)
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", label)
	}
}

func newMCPHTTPClient(t *testing.T, ctx context.Context, endpoint string) *mcpclient.Client {
	t.Helper()
	transport, err := mcptransport.NewStreamableHTTP(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Close() })

	client := mcpclient.NewClient(transport)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	_, err = client.Initialize(ctx, mcpgo.InitializeRequest{
		Params: mcpgo.InitializeParams{
			ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcpgo.ClientCapabilities{},
			ClientInfo: mcpgo.Implementation{
				Name:    "update-ipsets-test",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func fetchAnalysisText(t *testing.T, ctx context.Context, client *mcpclient.Client, name string) string {
	t.Helper()
	result, err := client.CallTool(ctx, mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "fetch_analysis",
			Arguments: map[string]any{
				"type": "feed",
				"name": name,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("fetch_analysis returned tool error: %+v", result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("fetch_analysis returned %d content blocks, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(mcpgo.TextContent)
	if !ok {
		t.Fatalf("fetch_analysis content type = %T, want mcp.TextContent", result.Content[0])
	}
	return strings.TrimPrefix(text.Text, "\ufeff")
}

func TestPublicOnlyHandlerRegistersReloadListener(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, err := engine.New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	runner := scheduler.New(eng, true, nil)
	server := newWebHTTPTestServer(t, newPublicHandler(eng, Options{EnableAll: true}, runner))

	writePublicServingFile(t, oldWebDir, "sample.json", `{"version":"old"}`+"\n")
	writePublicServingFile(t, newWebDir, "sample.json", `{"version":"new"}`+"\n")

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"old"}`+"\n" {
		t.Fatalf("initial public-only artifact status=%d body=%q", status, body)
	}
	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"new"}`+"\n" {
		t.Fatalf("reloaded public-only artifact status=%d body=%q", status, body)
	}
}

func TestAdminOnlyHandlerDoesNotReplacePublicReloadListener(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	newWebDir := filepath.Join(root, "web-new")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, err := engine.New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	runner := scheduler.New(eng, true, nil)
	publicServer := newWebHTTPTestServer(t, newPublicHandler(eng, Options{EnableAll: true}, runner))
	_ = newAdminHandler(eng, Options{EnableAll: true}, runner)

	writePublicServingFile(t, oldWebDir, "sample.json", `{"version":"old"}`+"\n")
	writePublicServingFile(t, newWebDir, "sample.json", `{"version":"new"}`+"\n")

	status, _, body := publicServer.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"old"}`+"\n" {
		t.Fatalf("initial split-listener artifact status=%d body=%q", status, body)
	}
	writePublicServingReloadConfig(t, cfgPath, root, newWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, _, body = publicServer.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"new"}`+"\n" {
		t.Fatalf("reloaded split-listener artifact status=%d body=%q", status, body)
	}
}

func TestReloadFailureKeepsPreviousPublicServingState(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	oldWebDir := filepath.Join(root, "web-old")
	blockedParent := filepath.Join(root, "blocked-parent")
	brokenWebDir := filepath.Join(blockedParent, "web")
	writePublicServingReloadConfig(t, cfgPath, root, oldWebDir, "", "")

	eng, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, oldWebDir, "sample.json", `{"version":"old"}`+"\n")
	if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	writePublicServingReloadConfig(t, cfgPath, root, brokenWebDir, "", "")
	if err := eng.ReloadContext(t.Context()); err == nil {
		t.Fatal("ReloadContext returned nil for blocked web dir, want error")
	}

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK || string(body) != `{"version":"old"}`+"\n" {
		t.Fatalf("artifact after failed reload status=%d body=%q, want previous state", status, body)
	}
}

func TestNoPublicRequestFallbackToRuntimeWebDir(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	servedWebDir := filepath.Join(root, "served-web")
	runtimeWebDir := filepath.Join(root, "runtime-web")
	writePublicServingReloadConfig(t, cfgPath, root, runtimeWebDir, "", "")

	_, handler := newPublicServingReloadHandler(t, cfgPath, Options{EnableAll: true, WebDir: servedWebDir})
	server := newWebHTTPTestServer(t, handler)

	writePublicServingFile(t, runtimeWebDir, "sample.json", `{"version":"runtime"}`+"\n")

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusNotFound {
		t.Fatalf("override route status = %d body=%s, want 404 without fallback to runtime web dir", status, body)
	}
}
