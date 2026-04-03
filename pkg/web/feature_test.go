package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestAdminAuthAndActions(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")

	eng, handler := testHandler(t, Options{EnableAll: true})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected unauthenticated admin SPA status: got %d", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Basic") {
		t.Fatalf("expected basic auth challenge, got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected authenticated admin SPA status: got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	text := string(body)
	// Verify the React index.html shell loaded.
	for _, want := range []string{"FireHOL IP Lists", "/static/assets/", "<div id=\"root\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in admin SPA body", want)
		}
	}

	// /admin/sets/* is part of the same authenticated shell surface.
	req = httptest.NewRequest(http.MethodGet, "/admin/sets/sample", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected unauthenticated nested admin status: got %d", rec.Code)
	}

	// /admin/sets/* (and any nested admin route) also serves the SPA once
	// the operator is authenticated.
	req = httptest.NewRequest(http.MethodGet, "/admin/sets/sample", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin set detail status: got %d", rec.Code)
	}

	// Admin API endpoints still REQUIRE basic auth — without it, 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected admin API status without auth: got %d", rec.Code)
	}

	// Test the new admin status API.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin status API code: got %d", rec.Code)
	}
	body, _ = io.ReadAll(rec.Result().Body)
	status := decodeTestJSON[adminStatus](t, body)
	if status.System.Goroutines <= 0 {
		t.Fatalf("admin status goroutines = %d, want positive", status.System.Goroutines)
	}
	if got := string(status.Engine.LastReason); got != "manual_run" {
		t.Fatalf("admin status last reason = %q, want manual_run", got)
	}

	// Test the feeds list API.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/feeds", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin feeds API code: got %d", rec.Code)
	}
	body, _ = io.ReadAll(rec.Result().Body)
	feeds := decodeTestJSON[[]adminFeed](t, body)
	sampleFeed, ok := findAdminFeed(feeds, "sample")
	if !ok {
		t.Fatalf("admin feeds missing sample: %+v", feeds)
	}
	if sampleFeed.LastRunReason != "manual_run" {
		t.Fatalf("sample last_run_reason = %q, want manual_run", sampleFeed.LastRunReason)
	}

	// Test feed detail API.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/feeds/sample", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin feed detail API code: got %d", rec.Code)
	}
	body, _ = io.ReadAll(rec.Result().Body)
	detail := decodeTestJSON[adminFeed](t, body)
	if detail.Name != "sample" || detail.Category == "" {
		t.Fatalf("unexpected admin feed detail: %+v", detail)
	}
	if !strings.Contains(string(body), `"last_processing_ms"`) {
		t.Fatalf("expected last_processing_ms in feed detail body: %s", body)
	}

	// Test admin schedule API.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/schedule", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin schedule API code: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/schedule", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected public schedule API code: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/run", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected admin trigger status: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/run", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unexpected admin trigger conflict status: got %d", rec.Code)
	}

	sourcePath := filepath.Join(eng.Runtime().BaseDir, "sample.enabled")
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/feeds/sample/disable", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected disable status: got %d", rec.Code)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected enable marker removed, stat err=%v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/feeds/sample/enable", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected enable status: got %d", rec.Code)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("expected enable marker restored: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/run/sample", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected single-set run status: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/feeds/sample/run", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected feed run action status: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/disable/sample", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected legacy disable route status: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/enable/sample", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected legacy enable route status: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/run?recheck=true", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected global recheck status: got %d", rec.Code)
	}

	if err := os.Remove(filepath.Join(eng.Runtime().BaseDir, "sample.ipset")); err != nil {
		t.Fatalf("remove sample.ipset: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/feeds/sample/reprocess", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unexpected bodyless reprocess status: got %d", rec.Code)
	}
}

func TestAdminFailsClosedWithoutConfiguredCredentials(t *testing.T) {
	_, handler := testHandler(t, Options{EnableAll: true})

	for _, path := range []string{"/admin", "/admin/sets/sample", "/api/v1/admin/status"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: expected 503 when admin auth is unconfigured, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "not configured") {
			t.Fatalf("%s: expected misconfiguration message, got %q", path, rec.Body.String())
		}
	}
}

func TestMiddlewareFeaturesAndOverrides(t *testing.T) {
	eng, handler := testHandler(t, Options{EnableAll: true})

	req := httptest.NewRequest(http.MethodGet, "/sample.json", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected gzip status: got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", got)
	}
	gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	gzMetadata := decodeTestJSON[map[string]any](t, body)
	if gzMetadata["name"] != "sample" {
		t.Fatalf("gzip metadata name = %v, want sample", gzMetadata["name"])
	}

	req = httptest.NewRequest(http.MethodGet, "/sample.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected cached file status: got %d", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	lastModified := rec.Header().Get("Last-Modified")
	if etag == "" || lastModified == "" {
		t.Fatalf("expected caching headers, got ETag=%q Last-Modified=%q", etag, lastModified)
	}

	req = httptest.NewRequest(http.MethodGet, "/sample.json", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("unexpected If-None-Match status: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/sample.json", nil)
	req.Header.Set("If-Modified-Since", lastModified)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("unexpected If-Modified-Since status: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected published metadata response: status=%d body=%s", rec.Code, rec.Body.String())
	}
	metadata := decodeTestJSON[map[string]any](t, rec.Body.Bytes())
	if metadata["name"] != "sample" {
		t.Fatalf("metadata name = %v, want sample", metadata["name"])
	}
	metadataPath := filepath.Join(eng.Runtime().WebDir, "sample.json")
	if err := os.Remove(metadataPath); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing metadata artifact status = %d, want 404", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/ipsets/sample/history", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected published history status: got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "DateTime,Entries,UniqueIPs") {
		t.Fatalf("unexpected history body: %s", rec.Body.String())
	}
	historyPath := filepath.Join(eng.Runtime().WebDir, "sample_history.csv")
	if err := os.Remove(historyPath); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ipsets/sample/history", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing history artifact status = %d, want 404", rec.Code)
	}

	changesetsPath := filepath.Join(eng.Runtime().WebDir, "sample_changesets.csv")
	if err := os.WriteFile(changesetsPath, []byte("DateTime,AddedIPs,RemovedIPs\n1700000000,2,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/changesets", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected published changesets response: status=%d body=%s", rec.Code, rec.Body.String())
	}
	changesets := decodeTestJSON[[]map[string]any](t, rec.Body.Bytes())
	if len(changesets) != 1 || changesets[0]["added"] != float64(2) {
		t.Fatalf("changesets = %+v, want one row with added=2", changesets)
	}
	if err := os.Remove(changesetsPath); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/changesets", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing changesets artifact status = %d, want 404", rec.Code)
	}

	retentionPath := filepath.Join(eng.Runtime().WebDir, "sample_retention.json")
	if err := os.WriteFile(retentionPath, []byte(`{"ipset":"sample"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/retention", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected published retention response: status=%d body=%s", rec.Code, rec.Body.String())
	}
	retention := decodeTestJSON[map[string]any](t, rec.Body.Bytes())
	if retention["ipset"] != "sample" {
		t.Fatalf("retention ipset = %v, want sample", retention["ipset"])
	}
	if err := os.Remove(retentionPath); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/retention", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing retention artifact status = %d, want 404", rec.Code)
	}

	comparisonPath := filepath.Join(eng.Runtime().WebDir, "sample_comparison.json")
	if err := os.WriteFile(comparisonPath, []byte(`[{"name":"other","category":"tests","ips":1,"common":1}]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/compare", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected published comparison response: status=%d body=%s", rec.Code, rec.Body.String())
	}
	comparison := decodeTestJSON[[]map[string]any](t, rec.Body.Bytes())
	if len(comparison) != 1 || comparison[0]["name"] != "other" {
		t.Fatalf("comparison = %+v, want one row named other", comparison)
	}
	if err := os.Remove(comparisonPath); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/compare", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing comparison artifact status = %d, want 404", rec.Code)
	}

	_, rlHandler := testHandler(t, Options{EnableAll: true, TrustCloudflareHeaders: true})
	for i := 0; i < 240; i++ {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
		req.Header.Set("CF-Connecting-IP", "198.51.100.7")
		rec = httptest.NewRecorder()
		rlHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected rate-limit warmup status on iteration %d: got %d", i, rec.Code)
		}
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("CF-Connecting-IP", "198.51.100.7")
	rec = httptest.NewRecorder()
	rlHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected rate-limit status: got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("unexpected Retry-After header: %q", got)
	}

	if got := (&clientIPResolver{trustProxy: true, trustCloudflare: true}).clientIP(proxyRequest("198.51.100.9", "203.0.113.9, 203.0.113.10", "192.0.2.9")); got != "198.51.100.9" {
		t.Fatalf("unexpected CF-Connecting-IP precedence: %q", got)
	}
	if got := (&clientIPResolver{trustProxy: true, trustCloudflare: true}).clientIP(proxyRequest("", "203.0.113.9, 203.0.113.10", "192.0.2.9")); got != "203.0.113.9" {
		t.Fatalf("unexpected X-Forwarded-For precedence: %q", got)
	}
	if got := (&clientIPResolver{trustProxy: true, trustCloudflare: true}).clientIP(proxyRequest("", "", "192.0.2.9")); got != "192.0.2.9" {
		t.Fatalf("unexpected X-Real-IP precedence: %q", got)
	}
	if got := (&clientIPResolver{}).clientIP(proxyRequest("198.51.100.9", "203.0.113.9, 203.0.113.10", "192.0.2.9")); got != "127.0.0.1" {
		t.Fatalf("expected RemoteAddr when no trust configured: %q", got)
	}
	if got := (&clientIPResolver{trustProxy: true}).clientIP(proxyRequest("198.51.100.9", "203.0.113.9, 203.0.113.10", "192.0.2.9")); got != "203.0.113.9" {
		t.Fatalf("expected X-Forwarded-For over ignored CF-Connecting-IP when only proxy trusted: %q", got)
	}
	if got := (&clientIPResolver{trustCloudflare: true}).clientIP(proxyRequest("", "203.0.113.9, 203.0.113.10", "192.0.2.9")); got != "127.0.0.1" {
		t.Fatalf("expected RemoteAddr when only Cloudflare trusted and no CF header: %q", got)
	}

	// Root always serves the embedded SPA, never a disk index.html.
	rootDir := t.TempDir()
	customDir := filepath.Join(rootDir, "custom-web")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "index.html"), []byte("custom index"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, overrideHandler := testHandler(t, Options{EnableAll: true, WebDir: customDir})
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	overrideHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected override index status: got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "custom index") {
		t.Fatalf("root should serve embedded SPA, not disk index.html")
	}

	if err := os.WriteFile(filepath.Join(customDir, "sample_changesets.csv"), []byte("DateTime,AddedIPs,RemovedIPs\n1700000000,4,3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/changesets", nil)
	rec = httptest.NewRecorder()
	overrideHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"added": 4`) {
		t.Fatalf("unexpected override changesets response: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRawFeedBodyRoutesRejectNonRedistributableFeed(t *testing.T) {
	_, handler := testHandlerWithRuntimeAndSourceExtra(t, Options{EnableAll: true}, "", "    redistributable: false\n")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata route status = %d, want 200", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public list status = %d, want 200", rec.Code)
	}
	listBody := rec.Body.String()
	if !strings.Contains(listBody, `"redistributable": false`) {
		t.Fatalf("public list should expose redistributable=false, got %s", listBody)
	}
	for _, field := range []string{`"url":`, `"public_url":`, `"file":`, `"source":`} {
		if strings.Contains(listBody, field) {
			t.Fatalf("public list exposed non-redistributable raw/source field %s in %s", field, listBody)
		}
	}

	for _, path := range []string{
		"/api/v1/sets/sample/data",
		"/files/sample.ipset",
		"/sample.ipset",
	} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/compose?include=sample&format=single", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("compose status = %d, want 400", rec.Code)
	}
}

func TestRawFeedRoutesReturn404WhenMaterializedFileMissing(t *testing.T) {
	eng, handler := testHandler(t, Options{EnableAll: true})
	if err := os.Remove(filepath.Join(eng.Runtime().BaseDir, "sample.ipset")); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		"/api/v1/sets/sample/data",
		"/files/sample.ipset",
		"/sample.ipset",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404; body=%q", path, rec.Code, rec.Body.String())
		}
	}
}

func TestBogonProviderRouteServesOnlyPublishedArtifacts(t *testing.T) {
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
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ipset
  bogon_provider:
    url: https://example.test/bogons.txt
    frequency: 60
    ipv: ipv4
    output: netset
    use: [bogons]
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(eng, Options{EnableAll: true}, scheduler.New(eng, true, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/bogons", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bogon provider list status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "bogon_provider") {
		t.Fatalf("provider list did not expose configured provider: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/bogons/bogon_provider", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing bogon artifact status = %d, want 404", rec.Code)
	}

	artifactPath := filepath.Join(eng.Runtime().WebDir, "sample_bogons_bogon_provider.json")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte(`{"provider":"bogon_provider"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/bogons/bogon_provider", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("published bogon artifact status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"provider":"bogon_provider"`) {
		t.Fatalf("unexpected bogon artifact body: %s", rec.Body.String())
	}
}

func TestCriticalInfrastructureRouteServesOnlyPublishedArtifacts(t *testing.T) {
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
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ipset
  sample_v6:
    url: https://example.test/sample-v6.txt
    frequency: 60
    ipv: ipv6
    output: netset
  data_shield:
    url: https://example.test/data-shield.txt
    frequency: 60
    ipv: ipv4
    output: ipset
  data_shield_critical:
    url: https://example.test/data-shield-critical.txt
    frequency: 60
    ipv: ipv4
    output: ipset
  orphan_critical_infrastructure:
    url: https://example.test/orphan-critical-infrastructure.txt
    frequency: 60
    ipv: ipv4
    output: ipset
  orphan_critical_critical_dns:
    url: https://example.test/orphan-critical-provider-looking.txt
    frequency: 60
    ipv: ipv4
    output: ipset
  critical_dns:
    url: https://example.test/critical-dns.txt
    redistributable: false
    frequency: 60
    ipv: ipv4
    output: netset
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: A
      rationale: test public DNS reference feed
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(eng, Options{EnableAll: true}, scheduler.New(eng, true, nil))

	for _, name := range []string{"orphan_critical_infrastructure", "orphan_critical_critical_dns"} {
		path := filepath.Join(eng.Runtime().WebDir, name+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"name":%q}`+"\n", name)), 0o644); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/"+name+".json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("exact public feed %q direct JSON status = %d, want 200", name, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/infrastructure/providers", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("critical provider list status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"name": "critical_dns"`) || !strings.Contains(body, `"tier": "hard"`) || !strings.Contains(body, `"redistributable": false`) {
		t.Fatalf("provider list did not expose typed critical provider: %s", body)
	}

	for _, path := range []string{
		"/api/v1/sets/critical_dns/data",
		"/files/critical_dns.netset",
		"/critical_dns.netset",
	} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("non-redistributable critical reference raw route %s status = %d, want 404", path, rec.Code)
		}
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/compose?include=critical_dns&format=single", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-redistributable critical reference compose status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/infrastructure", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing critical aggregate status = %d, want 404", rec.Code)
	}

	aggregatePath := filepath.Join(eng.Runtime().WebDir, "sample_critical_infrastructure.json")
	if err := os.MkdirAll(filepath.Dir(aggregatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Public surfaces are cache-first: a published critical-infrastructure
	// artifact MUST be served regardless of its provider_set_id. Drift
	// detection is the admin integrity path's concern (which still flags
	// such an artifact as malformed); the public path MUST NOT surface that
	// internal contract to end users.
	if err := os.WriteFile(aggregatePath, []byte(`{"feed":"sample","critical_ips":1,"provider_set_id":"stale"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/infrastructure", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("critical aggregate with non-current provider_set_id status = %d, want 200 (cache-first)", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/sample_critical_infrastructure.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("direct critical aggregate with non-current provider_set_id status = %d, want 200 (cache-first)", rec.Code)
	}

	aggregateBody := fmt.Sprintf(`{"feed":"sample","critical_ips":1,"complete":true,"provider_set_id":%q}`+"\n", eng.CriticalInfrastructureProviderSetID())
	if err := os.WriteFile(aggregatePath, []byte(aggregateBody), 0o644); err != nil {
		t.Fatal(err)
	}
	providerPath := filepath.Join(eng.Runtime().WebDir, "sample_critical_critical_dns.json")
	if err := os.WriteFile(providerPath, []byte(`{"provider":{"name":"critical_dns"},"critical_ips":1,"provider_set_id":"stale"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/infrastructure", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("published critical aggregate status = %d, want 200", rec.Code)
	}
	criticalAggregate := decodeTestJSON[map[string]any](t, rec.Body.Bytes())
	if criticalAggregate["critical_ips"] != float64(1) {
		t.Fatalf("critical aggregate critical_ips = %v, want 1", criticalAggregate["critical_ips"])
	}
	req = httptest.NewRequest(http.MethodGet, "/sample_critical_infrastructure.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("published direct critical aggregate status = %d, want 200", rec.Code)
	}

	v6AggregatePath := filepath.Join(eng.Runtime().WebDir, "sample_v6_critical_infrastructure.json")
	if err := os.WriteFile(v6AggregatePath, []byte(fmt.Sprintf(`{"feed":"sample_v6","critical_ips":1,"provider_set_id":%q}`+"\n", eng.CriticalInfrastructureProviderSetID())), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample_v6/infrastructure", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("non-comparable IPv6 critical aggregate API status = %d, want 404 (route enforces target eligibility)", rec.Code)
	}
	// Direct path is cache-first. An IPv6 feed's critical-overlap artifact
	// must never be produced in the first place; if one slips onto disk
	// (e.g. older binary), CleanupStaleCriticalInfrastructureArtifacts
	// removes it on the next engine pass. The HTTP path serves the file.
	req = httptest.NewRequest(http.MethodGet, "/sample_v6_critical_infrastructure.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("direct critical aggregate artifact status = %d, want 200 (cache-first; cleanup is admin)", rec.Code)
	}

	// Same cache-first contract for the per-provider artifact: the stale
	// provider_set_id we wrote at line ~771 above must still be served.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/infrastructure/critical_dns", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("critical provider with non-current provider_set_id status = %d, want 200 (cache-first)", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/sample_critical_critical_dns.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("direct critical provider with non-current provider_set_id status = %d, want 200 (cache-first)", rec.Code)
	}

	providerBody := fmt.Sprintf(`{"provider":{"name":"critical_dns"},"critical_ips":1,"provider_set_id":%q}`+"\n", eng.CriticalInfrastructureProviderSetID())
	if err := os.WriteFile(providerPath, []byte(providerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/infrastructure/critical_dns", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("published critical provider status = %d, want 200", rec.Code)
	}
	criticalProvider := decodeTestJSON[criticalProviderPayload](t, rec.Body.Bytes())
	if criticalProvider.Provider.Name != "critical_dns" {
		t.Fatalf("critical provider name = %q, want critical_dns", criticalProvider.Provider.Name)
	}
	req = httptest.NewRequest(http.MethodGet, "/sample_critical_critical_dns.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("published direct critical provider status = %d, want 200", rec.Code)
	}

	shorterFeedCriticalProviderPath := filepath.Join(eng.Runtime().WebDir, "data_shield_critical_critical_dns.json")
	if err := os.WriteFile(shorterFeedCriticalProviderPath, []byte(providerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/data_shield_critical_critical_dns.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("direct critical provider artifact for shorter feed with provider starting in critical_ status = %d, want 200", rec.Code)
	}

	criticalNamedProviderPath := filepath.Join(eng.Runtime().WebDir, "data_shield_critical_critical_critical_dns.json")
	if err := os.WriteFile(criticalNamedProviderPath, []byte(providerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/data_shield_critical_critical_critical_dns.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("direct critical provider artifact for feed ending in _critical status = %d, want 200", rec.Code)
	}

	criticalNamedRetentionPath := filepath.Join(eng.Runtime().WebDir, "data_shield_critical_retention.json")
	if err := os.WriteFile(criticalNamedRetentionPath, []byte(`{"ipset":"data_shield_critical"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/data_shield_critical_retention.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("normal retention artifact for feed ending in _critical status = %d, want 200", rec.Code)
	}

	// Orphan artifacts for providers no longer in the configured catalog
	// (the filename does not decompose to a configured provider scope):
	//   - The API route enforces configured-provider membership and 404s.
	//   - The direct artifact path also 404s because feedScopedPublicArtifactName
	//     reaches the default branch and the synthesized feed name is not a
	//     public feed. This is a filename-shape check, not a provider_set_id
	//     equality check, and it stays.
	criticalNamedStaleProviderPath := filepath.Join(eng.Runtime().WebDir, "data_shield_critical_critical_removed_provider.json")
	if err := os.WriteFile(criticalNamedStaleProviderPath, []byte(`{"provider":{"name":"removed_provider"},"critical_ips":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/data_shield_critical_critical_removed_provider.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("direct orphan critical provider artifact for feed ending in _critical status = %d, want 404 (filename does not parse as configured provider scope)", rec.Code)
	}

	staleProviderPath := filepath.Join(eng.Runtime().WebDir, "sample_critical_removed_provider.json")
	if err := os.WriteFile(staleProviderPath, []byte(`{"provider":{"name":"removed_provider"},"critical_ips":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/infrastructure/removed_provider", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("API critical provider for unknown provider status = %d, want 404 (route enforces configured catalog)", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/sample_critical_removed_provider.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("direct orphan critical provider artifact status = %d, want 404 (filename does not parse as configured provider scope)", rec.Code)
	}
}

func TestAdminAuthDisabledAllowsUnauthenticatedAccess(t *testing.T) {
	eng, _ := testHandler(t, Options{EnableAll: true})
	handler := newHandler(eng, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}, scheduler.New(eng, true, nil))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin SPA status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status code = %d, want 200", rec.Code)
	}
}

func testHandler(t *testing.T, opts Options) (*engine.Engine, http.Handler) {
	t.Helper()
	return testHandlerWithRuntime(t, opts, "")
}

func testHandlerWithRuntime(t *testing.T, opts Options, runtimeExtra string) (*engine.Engine, http.Handler) {
	t.Helper()
	return testHandlerWithRuntimeAndSourceExtra(t, opts, runtimeExtra, "")
}

func testHandlerWithRuntimeAndSourceExtra(t *testing.T, opts Options, runtimeExtra, sourceExtra string) (*engine.Engine, http.Handler) {
	t.Helper()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n5.6.7.0/30\n"))
	}))
	t.Cleanup(sourceServer.Close)

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
%s
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
%s
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), runtimeExtra, sourceServer.URL, sourceExtra)
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
	runner := scheduler.New(eng, true, nil)
	return eng, newHandler(eng, opts, runner)
}

func proxyRequest(cfIP, forwardedFor, realIP string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if cfIP != "" {
		req.Header.Set("CF-Connecting-IP", cfIP)
	}
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	return req
}
