package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGzipWriteHeaderStripsContentLength verifies that the gzipResponseWriter
// strips the Content-Length header when WriteHeader is called. This prevents
// the HTTP layer from sending an incorrect Content-Length (based on the
// uncompressed size) when the body is being gzip-compressed.
func TestGzipWriteHeaderStripsContentLength(t *testing.T) {
	// Backend handler that sets Content-Length explicitly.
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `{"name": "test", "value": 42}`
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "29") // pre-compression size
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})

	handler := gzipMiddleware(backend)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	// Content-Encoding should be gzip.
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", got)
	}

	// Content-Length MUST be stripped (or absent) — the compressed size
	// differs from the uncompressed size.
	if cl := rec.Header().Get("Content-Length"); cl != "" {
		t.Fatalf("Content-Length should be stripped for gzip responses, got %q", cl)
	}

	// Verify the gzip body decodes correctly.
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader error: %v", err)
	}
	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("reading gzip body: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing gzip body: %v", err)
	}

	if !strings.Contains(string(body), `"name": "test"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

// TestGzipSkipsNonMatchingPaths verifies that gzip middleware does not
// compress responses for paths that don't match the compressible list.
func TestGzipSkipsNonMatchingPaths(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("binary data"))
	})

	handler := gzipMiddleware(backend)

	req := httptest.NewRequest(http.MethodGet, "/files/data.ipset", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got == "gzip" {
		t.Fatal("expected non-gzip response for .ipset path")
	}
	if rec.Body.String() != "binary data" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

// TestGzipSkipsWithoutAcceptEncoding verifies that gzip middleware passes
// through when the client does not send Accept-Encoding: gzip.
func TestGzipSkipsWithoutAcceptEncoding(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain response"))
	})

	handler := gzipMiddleware(backend)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got == "gzip" {
		t.Fatal("should not gzip when Accept-Encoding is absent")
	}
	if rec.Body.String() != "plain response" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestGzipSkipsHeadRequests(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})

	handler := gzipMiddleware(backend)

	req := httptest.NewRequest(http.MethodHead, "/api/v1/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got == "gzip" {
		t.Fatal("HEAD requests should not be gzip-compressed")
	}
}

func TestGzipCompressesGetRequestsWithMatchingPath(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("compress me"))
	})

	handler := gzipMiddleware(backend)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", got)
	}
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer func() {
		if err := gz.Close(); err != nil {
			t.Fatalf("gzip.Close: %v", err)
		}
	}()
	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if string(body) != "compress me" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestGzipAcceptsUppercaseEncoding(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("compress me"))
	})

	handler := gzipMiddleware(backend)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Accept-Encoding", "GZIP")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip encoding for uppercase GZIP, got %q", got)
	}
}

func TestGzipNegotiatesAcceptEncodingQuality(t *testing.T) {
	tests := []struct {
		name       string
		headers    []string
		compressed bool
	}{
		{name: "zero quality", headers: []string{"gzip;q=0"}},
		{name: "uppercase zero quality", headers: []string{"GZIP;Q=0"}},
		{name: "positive quality", headers: []string{"br, gzip;q=0.001"}, compressed: true},
		{name: "maximum quality", headers: []string{"gzip;q=1.000"}, compressed: true},
		{name: "zero decimal quality", headers: []string{"gzip;q=0.000"}},
		{name: "quality whitespace", headers: []string{"gzip ; q = 0.5"}, compressed: true},
		{name: "positive wildcard", headers: []string{"br;q=1, *;q=0.5"}, compressed: true},
		{name: "zero wildcard", headers: []string{"br;q=1, *;q=0"}},
		{name: "explicit zero overrides wildcard", headers: []string{"gzip;q=0, *;q=1"}},
		{name: "malformed explicit overrides wildcard", headers: []string{"gzip;q=invalid, *;q=1"}},
		{name: "legacy alias", headers: []string{"x-gzip;q=0.5"}, compressed: true},
		{name: "legacy alias zero quality", headers: []string{"x-gzip;q=0"}},
		{name: "exact token", headers: []string{"notgzip"}},
		{name: "malformed quality", headers: []string{"gzip;q=invalid"}},
		{name: "quality over one", headers: []string{"gzip;q=2"}},
		{name: "invalid one fraction", headers: []string{"gzip;q=1.001"}},
		{name: "too many quality digits", headers: []string{"gzip;q=0.0001"}},
		{name: "unknown parameter", headers: []string{"gzip;level=1"}},
		{name: "duplicate quality", headers: []string{"gzip;q=1;q=0"}},
		{name: "multiple field values", headers: []string{"br", "gzip;q=1"}, compressed: true},
	}

	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("response"))
	})
	handler := gzipMiddleware(backend)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			for _, value := range test.headers {
				req.Header.Add("Accept-Encoding", value)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := rec.Header().Get("Content-Encoding") == "gzip"
			if got != test.compressed {
				t.Fatalf("Content-Encoding gzip = %v, want %v", got, test.compressed)
			}
		})
	}
}
