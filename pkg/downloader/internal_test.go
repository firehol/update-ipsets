package downloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFetchInternalReturnsRegisteredBytes verifies that an internal://
// URL routes through the in-process registry and yields a Result with
// a body file containing the registered bytes.
func TestFetchInternalReturnsRegisteredBytes(t *testing.T) {
	const name = "test-internal-source"
	const payload = "1.2.3.0/24\n5.6.7.0/24\n"
	RegisterInternal(name, func(_ string) ([]byte, error) {
		return []byte(payload), nil
	})
	defer UnregisterInternal(name)

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:    "internal://" + name,
		TmpDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.CleanUp()
	if result.Status != StatusOK {
		t.Fatalf("status = %q, want %q", result.Status, StatusOK)
	}
	if result.BodyPath == "" {
		t.Fatal("expected a body file path for internal source")
	}
	// ModifiedTime must be a real timestamp, not the legacy
	// InternalSentinelTime. The integrity check and finalize's
	// touchFileAt depend on real mtimes to compare source-vs-
	// secondary file freshness.
	if result.ModifiedTime.Equal(InternalSentinelTime) {
		t.Fatalf("modified time = %v, got sentinel (should be real time)", result.ModifiedTime)
	}
	if result.ModifiedTime.IsZero() {
		t.Fatalf("modified time is zero")
	}
	body, err := os.ReadFile(result.BodyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != payload {
		t.Fatalf("body = %q, want %q", body, payload)
	}
}

// TestFetchInternalUnknownNameFails ensures the validator can rely on
// the registry returning a clear failed Result when no provider is
// registered for the requested name.
func TestFetchInternalUnknownNameFails(t *testing.T) {
	UnregisterInternal("definitely-not-registered")
	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:    "internal://definitely-not-registered",
		TmpDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
}

// TestFetchInternalNotModifiedSentinel verifies that a provider
// returning ErrInternalNotModified short-circuits the pipeline with
// StatusSame and does not produce a body file. This is the cheap
// path expensive providers (retention_window, merge) use to skip
// regeneration when their inputs have not changed since the
// reference file's mtime.
func TestFetchInternalNotModifiedSentinel(t *testing.T) {
	const name = "test-internal-not-modified"
	callCount := 0
	RegisterInternal(name, func(referencePath string) ([]byte, error) {
		callCount++
		if referencePath == "" {
			t.Errorf("provider expected a reference path, got empty")
		}
		return nil, ErrInternalNotModified
	})
	defer UnregisterInternal(name)

	tmpDir := t.TempDir()
	refPath := filepath.Join(tmpDir, "ref")
	if err := os.WriteFile(refPath, []byte("stale-reference-body"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Set a distinct mtime on the reference so we can assert the
	// Result carries it through.
	refMTime := time.Now().UTC().Truncate(time.Second).Add(-42 * time.Minute)
	if err := os.Chtimes(refPath, refMTime, refMTime); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:           "internal://" + name,
		ReferencePath: refPath,
		TmpDir:        tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSame {
		t.Fatalf("status = %q, want %q", result.Status, StatusSame)
	}
	if result.BodyPath != "" {
		t.Fatalf("expected empty body path for not-modified result, got %q", result.BodyPath)
	}
	if !result.ModifiedTime.Equal(refMTime) {
		t.Fatalf("ModifiedTime = %v, want %v (reference file mtime)", result.ModifiedTime, refMTime)
	}
	if callCount != 1 {
		t.Fatalf("provider called %d times, want 1", callCount)
	}
}

// TestFetchInternalSameAsReference verifies that a previously written
// reference file with the same content makes Fetch return StatusSame
// without rewriting the temp file. This is the cache-hit path the
// engine relies on for synthetic sources whose content rarely changes.
func TestFetchInternalSameAsReference(t *testing.T) {
	const name = "test-internal-same"
	const payload = "9.9.9.0/24\n"
	RegisterInternal(name, func(_ string) ([]byte, error) {
		return []byte(payload), nil
	})
	defer UnregisterInternal(name)

	tmpDir := t.TempDir()
	refPath := filepath.Join(tmpDir, "ref")
	if err := os.WriteFile(refPath, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}

	client := New(5*time.Second, 5*time.Second)
	result, err := client.Fetch(t.Context(), Request{
		URL:           "internal://" + name,
		ReferencePath: refPath,
		TmpDir:        tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSame {
		t.Fatalf("status = %q, want %q", result.Status, StatusSame)
	}
	if result.BodyPath != "" {
		t.Fatal("same-body internal source should not produce a body file")
	}
}
