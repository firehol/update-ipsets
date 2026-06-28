package web

import (
	"compress/gzip"
	"context"
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

func TestRequestPathLoggerDoesNotUseCallerLogger(t *testing.T) {
	blocking := newReleasableBlockingHandler()
	defer blocking.releaseNow()

	logger := requestPathLogger(slog.New(blocking))
	logger.Warn("request path warning")

	select {
	case <-blocking.entered:
		t.Fatal("request-path logger used caller logger")
	default:
	}
}

func TestAsyncSlogHandlerDropsBeforeBlocking(t *testing.T) {
	blocking := newReleasableBlockingHandler()
	handler := newAsyncSlogHandler(blocking, 1)
	defer func() {
		blocking.releaseNow()
		handler.Close()
	}()
	logger := slog.New(handler)

	logger.Warn("first log blocks sink")
	select {
	case <-blocking.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking sink did not receive first log")
	}

	done := make(chan struct{})
	go func() {
		logger.Warn("queued while sink is blocked")
		logger.Warn("dropped while sink is blocked")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async slog handler blocked caller while sink was blocked")
	}
}

func TestSurfaceHandlerClientErrorLoggingDoesNotUseCallerLogger(t *testing.T) {
	blocking := newReleasableBlockingHandler()
	defer blocking.releaseNow()

	_, handler := testHandler(t, Options{
		EnableAll: true,
		Logger:    slog.New(blocking),
	})
	server := newWebHTTPTestServer(t, handler)

	done := make(chan error, 1)
	go func() {
		resp, err := server.client.Get(server.server.URL + "/api/v1/no-such-route")
		if err != nil {
			done <- err
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		err = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			done <- &unexpectedStatusError{got: resp.StatusCode, want: http.StatusNotFound}
			return
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
	case <-blocking.entered:
		t.Fatal("surface handler client-error logging used caller logger")
	case <-time.After(2 * time.Second):
		t.Fatal("request did not complete")
	}
}

type unexpectedStatusError struct {
	got  int
	want int
}

func (e *unexpectedStatusError) Error() string {
	return "status = " + http.StatusText(e.got) + ", want " + http.StatusText(e.want)
}

type releasableBlockingHandler struct {
	entered chan struct{}
	release chan struct{}
	once    chan struct{}
}

func newReleasableBlockingHandler() *releasableBlockingHandler {
	return &releasableBlockingHandler{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		once:    make(chan struct{}, 1),
	}
}

func (h *releasableBlockingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *releasableBlockingHandler) Handle(context.Context, slog.Record) error {
	select {
	case h.once <- struct{}{}:
		close(h.entered)
	default:
	}
	<-h.release
	return nil
}

func (h *releasableBlockingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *releasableBlockingHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *releasableBlockingHandler) releaseNow() {
	select {
	case <-h.release:
	default:
		close(h.release)
	}
}
