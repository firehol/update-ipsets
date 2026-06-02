package downloader

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFetchLocalCopyMaxDownloadSizeEnforced verifies that fetchLocalCopy
// (the "copyfile" downloader) enforces the MaxDownloadSize limit.
func TestFetchLocalCopyMaxDownloadSizeEnforced(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "large.txt")

	// Create a file larger than our limit.
	content := strings.Repeat("x", 500)
	if err := os.WriteFile(srcFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)

	// Limit to 100 bytes — should fail.
	result, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: srcFile,
		MaxDownloadSize:   100,
		TmpDir:            tmpDir,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer result.CleanUp()

	if result.Status != StatusFailed {
		t.Fatalf("expected failed status for oversized local file, got %q: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "exceeds max download size") {
		t.Fatalf("unexpected error message: %s", result.Message)
	}
}

// TestFetchLocalCopyMaxDownloadSizeAllowsUnderLimit verifies that
// files within the size limit are copied successfully.
func TestFetchLocalCopyMaxDownloadSizeAllowsUnderLimit(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "small.txt")

	content := "10.0.0.1\n10.0.0.2\n"
	if err := os.WriteFile(srcFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)

	result, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: srcFile,
		MaxDownloadSize:   1000,
		TmpDir:            tmpDir,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer result.CleanUp()

	if result.Status != StatusOK {
		t.Fatalf("expected OK status, got %q: %s", result.Status, result.Message)
	}
	if result.BodySize != int64(len(content)) {
		t.Fatalf("size mismatch: got %d want %d", result.BodySize, len(content))
	}
}

// TestFetchLocalCopyDisabledLimit verifies that MaxDownloadSize=-1
// disables the size limit for local copies.
func TestFetchLocalCopyDisabledLimit(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "any_size.txt")

	content := strings.Repeat("line\n", 200)
	if err := os.WriteFile(srcFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)

	result, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: srcFile,
		MaxDownloadSize:   -1, // disabled
		TmpDir:            tmpDir,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer result.CleanUp()

	if result.Status != StatusOK {
		t.Fatalf("expected OK with disabled limit, got %q: %s", result.Status, result.Message)
	}
}

// TestFetchLocalCopyEmptyFileRejects verifies that empty local files
// are rejected unless AcceptEmpty is set.
func TestFetchLocalCopyEmptyFileRejects(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(srcFile, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)

	// Without AcceptEmpty.
	result, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: srcFile,
		TmpDir:            tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusFailed {
		t.Fatalf("expected failed for empty file, got %q", result.Status)
	}

	// With AcceptEmpty.
	result2, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: srcFile,
		AcceptEmpty:       true,
		TmpDir:            tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result2.CleanUp()
	if result2.Status != StatusOK {
		t.Fatalf("expected OK with AcceptEmpty, got %q", result2.Status)
	}
}

// TestFetchLocalCopyMissingSourcePath verifies that a missing source
// path option is properly rejected.
func TestFetchLocalCopyMissingSourcePath(t *testing.T) {
	client := New(5*time.Second, 5*time.Second)

	result, err := client.Fetch(t.Context(), Request{
		Downloader:        "copyfile",
		DownloaderOptions: "", // empty
		TmpDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()

	if result.Status != StatusFailed {
		t.Fatalf("expected failed for missing source path, got %q", result.Status)
	}
}

func TestFetchFileURLUsesLocalCopyPath(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "sample.txt")
	content := "10.0.0.1\n10.0.0.2\n"
	if err := os.WriteFile(srcFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:             (&url.URL{Scheme: "file", Path: srcFile}).String(),
		MaxDownloadSize: 1000,
		TmpDir:          tmpDir,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer result.CleanUp()

	if result.Status != StatusOK {
		t.Fatalf("expected OK status, got %q: %s", result.Status, result.Message)
	}
	if result.BodySize != int64(len(content)) {
		t.Fatalf("size mismatch: got %d want %d", result.BodySize, len(content))
	}
}

func TestFetchFileURLRejectsHostComponent(t *testing.T) {
	client := New(5*time.Second, 5*time.Second)
	_, err := client.Fetch(t.Context(), Request{
		URL:    "file://example.test/tmp/sample.txt",
		TmpDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected file URL with host to fail")
	}
	if !strings.Contains(err.Error(), "file url host component is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
