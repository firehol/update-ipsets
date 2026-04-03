package downloader

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchHTMLErrorPage(t *testing.T) {
	// Server returns HTML 404 instead of IP list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<html><body><h1>404 Not Found</h1></body></html>`))
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       server.URL,
		UserAgent: "test-agent",
		TmpDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	if result.Status != StatusFailed {
		t.Fatalf("expected failed for 404, got %q: %s", result.Status, result.Message)
	}
	if result.HTTPCode != 404 {
		t.Fatalf("expected HTTP 404, got %d", result.HTTPCode)
	}
}

func TestFetchHTMLErrorPage200(t *testing.T) {
	// Server returns HTML with 200 status — this is a soft error that the
	// downloader cannot detect. The processor layer must handle it.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><h1>Service Unavailable</h1><p>Please try again later.</p></body></html>`))
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       server.URL,
		UserAgent: "test-agent",
		TmpDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	// Downloader treats any 200 with body as OK — content validation is
	// the responsibility of the processor and engine layers.
	if result.Status != StatusOK {
		t.Fatalf("expected OK for 200 HTML, got %q", result.Status)
	}
}

func TestFetchFollowsRedirects(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer final.Close()

	// Redirect chain: 301 -> 302 -> final
	middle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer middle.Close()

	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, middle.URL, http.StatusMovedPermanently)
	}))
	defer entry.Close()

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       entry.URL,
		UserAgent: "test-agent",
		TmpDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	if result.Status != StatusOK {
		t.Fatalf("expected OK after redirects, got %q: %s", result.Status, result.Message)
	}
}

func TestFetchTooManyRedirects(t *testing.T) {
	// Create a server that always redirects to itself
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server.URL+"/loop", http.StatusFound)
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       server.URL,
		UserAgent: "test-agent",
		TmpDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	// Go's default redirect limit is 10 — after that it returns an error
	// which the downloader maps to StatusFailed.
	if result.Status != StatusFailed {
		t.Fatalf("expected failed for redirect loop, got %q", result.Status)
	}
}

func TestFetchAuthHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("X-API-Key"); got != "secret123" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:               server.URL,
		UserAgent:         "test-agent",
		DownloaderOptions: `-u 'user:pass' -H 'X-API-Key: secret123'`,
		TmpDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	if result.Status != StatusOK {
		t.Fatalf("expected OK with auth, got %q: %s", result.Status, result.Message)
	}
}

func TestFetchBOMInBody(t *testing.T) {
	// Server returns body with UTF-8 BOM prefix
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\xEF\xBB\xBF1.2.3.4\n5.6.7.8\n"))
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       server.URL,
		UserAgent: "test-agent",
		TmpDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	// Downloader delivers raw bytes — BOM stripping is the processor's job.
	if result.Status != StatusOK {
		t.Fatalf("expected OK, got %q", result.Status)
	}
	if result.BodySize != int64(len("\xEF\xBB\xBF1.2.3.4\n5.6.7.8\n")) {
		t.Fatalf("expected BOM in body size, got %d", result.BodySize)
	}
}

func TestFetchCRLFBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\r\n5.6.7.8\r\n"))
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       server.URL,
		UserAgent: "test-agent",
		TmpDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	// Downloader delivers raw bytes — CRLF normalization is the processor's job.
	if result.Status != StatusOK {
		t.Fatalf("expected OK, got %q", result.Status)
	}
}

func TestFetchVeryLargeBody(t *testing.T) {
	// Test that MaxDownloadSize correctly rejects large responses
	bigPayload := strings.Repeat("1.2.3.4\n", 10000) // ~80KB
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(bigPayload))
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)

	// With a very small limit, it should fail
	result, err := client.Fetch(t.Context(), Request{
		URL:             server.URL,
		UserAgent:       "test-agent",
		MaxDownloadSize: 1024, // 1KB limit
		TmpDir:          t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusFailed {
		t.Fatalf("expected failed for oversized body, got %q", result.Status)
	}

	// With a generous limit, it should succeed
	result2, err := client.Fetch(t.Context(), Request{
		URL:             server.URL,
		UserAgent:       "test-agent",
		MaxDownloadSize: 200 * 1024, // 200KB
		TmpDir:          t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result2.CleanUp()
	if result2.Status != StatusOK {
		t.Fatalf("expected OK, got %q", result2.Status)
	}
}

func TestSplitShellWordsEdgeCases(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		// Empty
		{"", nil},
		// Simple
		{"--data foo", []string{"--data", "foo"}},
		// Single quotes
		{"--data 'hello world'", []string{"--data", "hello world"}},
		// Double quotes
		{`--data "hello world"`, []string{"--data", "hello world"}},
		// Escaped space
		{`--data hello\ world`, []string{"--data", "hello world"}},
		// Mixed
		{`-u 'user:pass' -H "X-Key: val"`, []string{"-u", "user:pass", "-H", "X-Key: val"}},
		// Equals form
		{"--data=value", []string{"--data=value"}},
	}
	for _, tc := range cases {
		got := splitShellWords(tc.input)
		if len(got) != len(tc.want) {
			t.Fatalf("splitShellWords(%q): got %q want %q", tc.input, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("splitShellWords(%q)[%d]: got %q want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}
