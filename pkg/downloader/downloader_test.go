package downloader

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchNotModified(t *testing.T) {
	modified := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ims := r.Header.Get("If-Modified-Since"); ims != "" {
			if parsed, err := http.ParseTime(ims); err == nil && !modified.After(parsed) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()

	ref := filepath.Join(t.TempDir(), "source.ref")
	if err := os.WriteFile(ref, []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(ref, modified, modified); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:           server.URL,
		ReferencePath: ref,
		UserAgent:     "test-agent",
		TmpDir:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusNotModified {
		t.Fatalf("unexpected status: got %q want %q", result.Status, StatusNotModified)
	}
	if result.BodyPath != "" {
		t.Fatal("304 should not produce a body file")
	}
}

func TestFetchSameBody(t *testing.T) {
	modified := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("5.6.7.8\n"))
	}))
	defer server.Close()

	ref := filepath.Join(t.TempDir(), "source.ref")
	if err := os.WriteFile(ref, []byte("5.6.7.8\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		Name:          "same",
		URL:           server.URL,
		ReferencePath: ref,
		UserAgent:     fmt.Sprintf("test-%d", time.Now().UnixNano()),
		TmpDir:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusSame {
		t.Fatalf("unexpected status: got %q want %q", result.Status, StatusSame)
	}
	if result.BodyPath != "" {
		t.Fatal("same-body should not produce a body file (temp cleaned up)")
	}
	if result.BodyHash == "" {
		t.Fatal("same-body result should have hash")
	}
}

func TestFetchStreamingCreatesFile(t *testing.T) {
	payload := "1.2.3.4\n5.6.7.8\n9.10.11.12\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       server.URL,
		UserAgent: "test-agent",
		TmpDir:    tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	if result.Status != StatusOK {
		t.Fatalf("unexpected status: %q", result.Status)
	}
	if result.BodyPath == "" {
		t.Fatal("expected body path for OK result")
	}
	data, err := os.ReadFile(result.BodyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != payload {
		t.Fatalf("body mismatch: got %q want %q", data, payload)
	}
	if result.BodySize != int64(len(payload)) {
		t.Fatalf("size mismatch: got %d want %d", result.BodySize, len(payload))
	}
	if result.BodyHash == "" {
		t.Fatal("expected hash")
	}
}

func TestFetchDownloaderOptionsPOSTAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %q", r.Method)
		}
		if got := string(body); got != "ipv6=0&export_type=text" {
			t.Fatalf("unexpected body %q", got)
		}
		if got := r.Header.Get("X-Test"); got != "yes" {
			t.Fatalf("unexpected header %q", got)
		}
		if got := r.Referer(); got != "https://example.test/ref" {
			t.Fatalf("unexpected referer %q", got)
		}
		// Regression: curl sets Content-Type:
		// application/x-www-form-urlencoded automatically when
		// --data is used, and gpf_comics' form-submission
		// endpoint (the canonical example) silently returns its
		// HTML login page instead of the data when the header
		// is absent. The downloader must match curl's default.
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("expected auto Content-Type: application/x-www-form-urlencoded with --data, got %q", got)
		}
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()

	client := New(time.Second, time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:               server.URL,
		UserAgent:         "test-agent",
		DownloaderOptions: `--data 'ipv6=0&export_type=text' -H 'X-Test: yes' --referer 'https://example.test/ref'`,
		TmpDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer result.CleanUp()
	if result.Status != StatusOK {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

// TestFetchDownloaderOptionsExplicitContentTypeWins pins the
// "caller override" half of the auto-Content-Type logic: when
// the caller passes their own -H 'Content-Type: ...', the
// downloader must NOT replace it with the form-urlencoded
// default. Use case: a feed whose server wants
// application/json body.
func TestFetchDownloaderOptionsExplicitContentTypeWins(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected caller-supplied Content-Type to win, got %q", got)
		}
		_, _ = w.Write([]byte("9.9.9.9\n"))
	}))
	defer server.Close()
	client := New(time.Second, time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:               server.URL,
		UserAgent:         "test-agent",
		DownloaderOptions: `--data '{"k":"v"}' -H 'Content-Type: application/json'`,
		TmpDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer result.CleanUp()
	if result.Status != StatusOK {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

func TestFetchDownloaderOptionsMethodAndBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "alice" || pass != "secret" {
			t.Fatalf("unexpected auth %q/%q", user, pass)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %q", r.Method)
		}
		_, _ = w.Write([]byte("2.2.2.2\n"))
	}))
	defer server.Close()

	client := New(time.Second, time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:               server.URL,
		UserAgent:         "test-agent",
		DownloaderOptions: `-X POST -u 'alice:secret'`,
		TmpDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer result.CleanUp()
	if result.Status != StatusOK {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

func TestFetchDownloadSizeLimit(t *testing.T) {
	// Server sends 200 bytes.
	payload := strings.Repeat("x", 200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)

	// Limit to 100 bytes: should fail.
	result, err := client.Fetch(t.Context(), Request{
		URL:             server.URL,
		UserAgent:       "test-agent",
		MaxDownloadSize: 100,
		TmpDir:          t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusFailed {
		t.Fatalf("expected failed status for oversized download, got %q: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "exceeds max download size") {
		t.Fatalf("unexpected message: %s", result.Message)
	}

	// Limit to 300: should succeed.
	result2, err := client.Fetch(t.Context(), Request{
		URL:             server.URL,
		UserAgent:       "test-agent",
		MaxDownloadSize: 300,
		TmpDir:          t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result2.CleanUp()
	if result2.Status != StatusOK {
		t.Fatalf("expected OK status, got %q", result2.Status)
	}

	// Disabled limit (-1): should succeed.
	result3, err := client.Fetch(t.Context(), Request{
		URL:             server.URL,
		UserAgent:       "test-agent",
		MaxDownloadSize: -1,
		TmpDir:          t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result3.CleanUp()
	if result3.Status != StatusOK {
		t.Fatalf("expected OK with disabled limit, got %q", result3.Status)
	}
}

func TestFetchTempFileCleanupOnError(t *testing.T) {
	// Server drops connection mid-body.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10000")
		_, _ = w.Write([]byte("partial"))
		// Close connection without sending the rest.
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		if conn != nil {
			_ = conn.Close()
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	client := New(2*time.Second, 2*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:       server.URL,
		UserAgent: "test-agent",
		TmpDir:    tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %q", result.Status)
	}
	// The temp directory should be empty (cleanup on error).
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		t.Errorf("leftover temp file: %s", e.Name())
	}
}

func TestFetchEmptyBodyRejectsUnlessAcceptEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(5*time.Second, 5*time.Second)

	// Without AcceptEmpty: should fail.
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
		t.Fatalf("expected failed for empty, got %q", result.Status)
	}

	// With AcceptEmpty: should succeed.
	result2, err := client.Fetch(t.Context(), Request{
		URL:         server.URL,
		UserAgent:   "test-agent",
		AcceptEmpty: true,
		TmpDir:      t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result2.CleanUp()
	if result2.Status != StatusOK {
		t.Fatalf("expected OK with AcceptEmpty, got %q", result2.Status)
	}
}

func TestFetchCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(srcFile, []byte("10.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: srcFile,
		UserAgent:         "test-agent",
		TmpDir:            tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusOK {
		t.Fatalf("expected OK, got %q: %s", result.Status, result.Message)
	}
	if result.BodyPath == "" {
		t.Fatal("expected body path")
	}
	data, err := os.ReadFile(result.BodyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "10.0.0.1\n" {
		t.Fatalf("body mismatch: %q", data)
	}
}

func TestFetchCopyFileSameBody(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "input.txt")
	refFile := filepath.Join(tmpDir, "ref.txt")
	content := "same-content\n"
	if err := os.WriteFile(srcFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: srcFile,
		ReferencePath:     refFile,
		UserAgent:         "test-agent",
		TmpDir:            tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusSame {
		t.Fatalf("expected same, got %q", result.Status)
	}
	if result.BodyPath != "" {
		t.Fatal("same-body copyfile should not produce a body file")
	}
}
