package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectPublishedArtifactRejectsHiddenPathSegments(t *testing.T) {
	eng, handler := testHandler(t, Options{EnableAll: true})
	hiddenDir := filepath.Join(eng.Runtime().WebDir, ".update-ipsets-web-123")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "sample.json"), []byte(`{"hidden":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.update-ipsets-web-123/sample.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hidden direct artifact status = %d, want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublicTopLevelArtifactRejectsSymlinkEscape(t *testing.T) {
	eng, handler := testHandler(t, Options{EnableAll: true})
	outside := filepath.Join(t.TempDir(), "robots.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	robots := filepath.Join(eng.Runtime().WebDir, "robots.txt")
	if err := os.Remove(robots); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, robots); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("symlinked robots status = %d, want 404 body=%q", rec.Code, rec.Body.String())
	}
}

func TestRawFeedRouteRejectsSymlinkEscape(t *testing.T) {
	eng, handler := testHandler(t, Options{EnableAll: true})
	outside := filepath.Join(t.TempDir(), "sample.ipset")
	if err := os.WriteFile(outside, []byte("203.0.113.10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(eng.Runtime().BaseDir, "sample.ipset")
	if err := os.Remove(raw); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, raw); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/sample.ipset", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("symlinked raw feed status = %d, want 404 body=%q", rec.Code, rec.Body.String())
	}
}
