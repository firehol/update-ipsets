package web

import (
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientRateLimiterEnforcesLimitAndRecoversAfterWindow(t *testing.T) {
	limiter := newClientRateLimiter(2, time.Minute)
	now := time.Unix(1_700_000_000, 0)

	if !limiter.Allow("198.51.100.10", now) {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow("198.51.100.10", now) {
		t.Fatal("second request should be allowed")
	}
	if limiter.Allow("198.51.100.10", now) {
		t.Fatal("third request in the same window should be rejected")
	}
	if !limiter.Allow("198.51.100.10", now.Add(2*time.Minute)) {
		t.Fatal("request after the limiter window should be allowed")
	}
	if !limiter.Allow("198.51.100.11", now) {
		t.Fatal("different client IP should have an independent rate-limit bucket")
	}
}

func TestRecoverMiddlewareWritesGzippedServerError(t *testing.T) {
	resolver := &clientIPResolver{}
	handler := gzipMiddleware(recoverMiddleware(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		resolver,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}),
	))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("content encoding = %q, want gzip", got)
	}
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip body: %v", err)
	}
	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip body: %v", err)
	}
	if !strings.Contains(string(body), "internal server error") {
		t.Fatalf("body = %q, want server error", body)
	}
}
