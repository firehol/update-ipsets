package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func TestAdminAuthAndActions(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")
	setDetailedStatusCacheForTest(t, time.Now().UTC(), detailedSystemInfo{
		Goroutines:    7,
		HeapAlloc:     1,
		HeapSys:       1,
		Uptime:        "1s",
		UptimeSeconds: 1,
		DiskFree:      "test",
	})

	eng, handler := testHandler(t, Options{EnableAll: true})

	assertAdminShellAuth(t, handler)
	assertAdminAPIs(t, handler)
	assertAdminRunQueueActions(t, handler)
	assertAdminFeedEnablement(t, eng, handler)
	assertRemovedAdminActionsStayMissing(t, handler)
	assertBodylessReprocessConflict(t, eng, handler)
}

func assertAdminShellAuth(t *testing.T, handler http.Handler) {
	t.Helper()

	rec := adminTestRequest(handler, http.MethodGet, "/admin", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected unauthenticated admin SPA status: got %d", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Basic") {
		t.Fatalf("expected basic auth challenge, got %q", got)
	}

	rec = adminTestRequest(handler, http.MethodGet, "/admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected authenticated admin SPA status: got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	for _, want := range []string{"FireHOL IP Lists", "/static/assets/", "<div id=\"root\""} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("expected %q in admin SPA body", want)
		}
	}

	rec = adminTestRequest(handler, http.MethodGet, "/admin/sets/sample", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected unauthenticated nested admin status: got %d", rec.Code)
	}
	rec = adminTestRequest(handler, http.MethodGet, "/admin/sets/sample", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin set detail status: got %d", rec.Code)
	}
}

func assertAdminAPIs(t *testing.T, handler http.Handler) {
	t.Helper()

	rec := adminTestRequest(handler, http.MethodGet, "/api/v1/admin/status", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected admin API status without auth: got %d", rec.Code)
	}

	rec = adminTestRequest(handler, http.MethodGet, "/api/v1/admin/status", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin status API code: got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	status := decodeTestJSON[adminStatus](t, body)
	if status.System.Goroutines <= 0 {
		t.Fatalf("admin status goroutines = %d, want positive", status.System.Goroutines)
	}
	if got := string(status.Engine.LastReason); got != "manual_run" {
		t.Fatalf("admin status last reason = %q, want manual_run", got)
	}

	rec = adminTestRequest(handler, http.MethodGet, "/api/v1/admin/feeds", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin feeds API code: got %d", rec.Code)
	}
	body, _ = io.ReadAll(rec.Result().Body)
	feeds := decodeTestJSON[[]adminFeed](t, body)
	sampleFeed, ok := findAdminFeed(feeds, "sample")
	if !ok || sampleFeed.LastRunReason != "manual_run" {
		t.Fatalf("unexpected sample admin feed: found=%v feed=%+v", ok, sampleFeed)
	}

	rec = adminTestRequest(handler, http.MethodGet, "/api/v1/admin/feeds/sample", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin feed detail API code: got %d", rec.Code)
	}
	body, _ = io.ReadAll(rec.Result().Body)
	detail := decodeTestJSON[adminFeed](t, body)
	if detail.Name != "sample" || detail.Category == "" || !strings.Contains(string(body), `"last_processing_ms"`) {
		t.Fatalf("unexpected admin feed detail body: %s", body)
	}

	rec = adminTestRequest(handler, http.MethodGet, "/api/v1/admin/schedule", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin schedule API code: got %d", rec.Code)
	}
	rec = adminTestRequest(handler, http.MethodGet, "/api/v1/schedule", false)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected public schedule API code: got %d", rec.Code)
	}
}

func assertAdminRunQueueActions(t *testing.T, handler http.Handler) {
	t.Helper()

	rec := adminTestRequest(handler, http.MethodPost, "/api/v1/admin/run", true)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected admin trigger status: got %d", rec.Code)
	}
	rec = adminTestRequest(handler, http.MethodPost, "/api/v1/admin/run", true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unexpected admin trigger conflict status: got %d", rec.Code)
	}
	rec = adminTestRequest(handler, http.MethodPost, "/api/v1/admin/run?recheck=true", true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected global recheck status: got %d", rec.Code)
	}
}

func assertAdminFeedEnablement(t *testing.T, eng *engine.Engine, handler http.Handler) {
	t.Helper()

	sourcePath := filepath.Join(eng.Runtime().BaseDir, "sample.enabled")
	rec := adminTestRequest(handler, http.MethodPost, "/api/v1/admin/feeds/sample/disable", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected disable status: got %d", rec.Code)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected enable marker removed, stat err=%v", err)
	}

	rec = adminTestRequest(handler, http.MethodPost, "/api/v1/admin/feeds/sample/enable", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected enable status: got %d", rec.Code)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("expected enable marker restored: %v", err)
	}
}

func assertRemovedAdminActionsStayMissing(t *testing.T, handler http.Handler) {
	t.Helper()

	for _, path := range []string{
		"/api/v1/admin/run/sample",
		"/api/v1/admin/feeds/sample/run",
		"/api/v1/admin/disable/sample",
		"/api/v1/admin/enable/sample",
	} {
		rec := adminTestRequest(handler, http.MethodPost, path, true)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: unexpected status: got %d", path, rec.Code)
		}
	}
}

func assertBodylessReprocessConflict(t *testing.T, eng *engine.Engine, handler http.Handler) {
	t.Helper()

	if err := os.Remove(filepath.Join(eng.Runtime().BaseDir, "sample.ipset")); err != nil {
		t.Fatalf("remove sample.ipset: %v", err)
	}
	rec := adminTestRequest(handler, http.MethodPost, "/api/v1/admin/feeds/sample/reprocess", true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unexpected bodyless reprocess status: got %d", rec.Code)
	}
}

func adminTestRequest(handler http.Handler, method, path string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authenticated {
		req.SetBasicAuth("admin", "secret")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
