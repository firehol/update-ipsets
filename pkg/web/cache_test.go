package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileCacheEvictsLeastRecentlyUsedEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	a := writeCacheTestFile(t, root, "a.json", "a1\n", time.Unix(100, 0))
	b := writeCacheTestFile(t, root, "b.json", "b1\n", time.Unix(100, 0))
	c := writeCacheTestFile(t, root, "c.json", "c1\n", time.Unix(100, 0))
	cache := newFileCacheWithLimits(fileCacheLimits{MaxEntries: 2, MaxBytes: 1024, MaxFileBytes: 1024})

	assertCachedServeBody(t, cache, a, "a1\n")
	assertCachedServeBody(t, cache, b, "b1\n")
	assertCachedServeBody(t, cache, c, "c1\n")

	// Same size and same mtime proves the response came from disk after LRU
	// eviction, not from the stale in-memory body.
	writeCacheTestFile(t, root, "a.json", "a2\n", time.Unix(100, 0))
	assertCachedServeBody(t, cache, a, "a2\n")
}

func TestFileCacheInvalidatesCachedBodyWhenMTimeChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeCacheTestFile(t, root, "fresh.json", "v1\n", time.Unix(150, 0))
	cache := newFileCacheWithLimits(fileCacheLimits{MaxEntries: 2, MaxBytes: 1024, MaxFileBytes: 1024})

	assertCachedServeBody(t, cache, path, "v1\n")
	writeCacheTestFile(t, root, "fresh.json", "v2\n", time.Unix(151, 0))
	assertCachedServeBody(t, cache, path, "v2\n")
}

func TestFileCacheBypassesOversizedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeCacheTestFile(t, root, "large.txt", "large-v1\n", time.Unix(200, 0))
	cache := newFileCacheWithLimits(fileCacheLimits{MaxEntries: 10, MaxBytes: 1024, MaxFileBytes: 4})

	assertCachedServeBody(t, cache, path, "large-v1\n")
	writeCacheTestFile(t, root, "large.txt", "large-v2\n", time.Unix(200, 0))
	assertCachedServeBody(t, cache, path, "large-v2\n")
}

func TestFileCacheHonorsByteLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	a := writeCacheTestFile(t, root, "a.json", "aaaa\n", time.Unix(300, 0))
	b := writeCacheTestFile(t, root, "b.json", "bbbb\n", time.Unix(300, 0))
	cache := newFileCacheWithLimits(fileCacheLimits{MaxEntries: 10, MaxBytes: 5, MaxFileBytes: 5})

	assertCachedServeBody(t, cache, a, "aaaa\n")
	assertCachedServeBody(t, cache, b, "bbbb\n")
	assertFileCacheContainsOnly(t, cache, b, int64(len("bbbb\n")))

	writeCacheTestFile(t, root, "b.json", "BBBB\n", time.Unix(300, 0))
	assertCachedServeBody(t, cache, b, "bbbb\n")

	writeCacheTestFile(t, root, "a.json", "zzzz\n", time.Unix(300, 0))
	assertCachedServeBody(t, cache, a, "zzzz\n")
}

func TestFileCacheInsertRecheckKeepsLRUStateConsistent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeCacheTestFile(t, root, "same.json", "v1\n", time.Unix(350, 0))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	cache := newFileCacheWithLimits(fileCacheLimits{MaxEntries: 10, MaxBytes: 1024, MaxFileBytes: 1024})

	cache.mu.Lock()
	out1, inserted1 := cache.insertLoadedLocked(path, info, &cachedFile{
		modTime: info.ModTime(),
		size:    info.Size(),
		data:    []byte("v1\n"),
	})
	out2, inserted2 := cache.insertLoadedLocked(path, info, &cachedFile{
		modTime: info.ModTime(),
		size:    info.Size(),
		data:    []byte("v2\n"),
	})
	entries := len(cache.files)
	lruEntries := cache.order.Len()
	bytesCached := cache.bytes
	cache.mu.Unlock()

	if !inserted1 {
		t.Fatal("first insert reused an existing entry")
	}
	if inserted2 {
		t.Fatal("second insert duplicated an already fresh cache entry")
	}
	if got, want := string(out1.data), "v1\n"; got != want {
		t.Fatalf("first insert body = %q, want %q", got, want)
	}
	if got, want := string(out2.data), "v1\n"; got != want {
		t.Fatalf("second insert body = %q, want %q", got, want)
	}
	if entries != 1 || lruEntries != 1 {
		t.Fatalf("cache entries=%d lru entries=%d, want 1 and 1", entries, lruEntries)
	}
	if bytesCached != info.Size() {
		t.Fatalf("cache bytes = %d, want %d", bytesCached, info.Size())
	}
}

func TestFileCacheRootedServingRejectsSymlinkEscapeAndKeepsServeContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	cache := newFileCacheWithLimits(fileCacheLimits{MaxEntries: 2, MaxBytes: 1024, MaxFileBytes: 1024})
	req := httptest.NewRequest(http.MethodGet, "/escape.txt", nil)
	rec := httptest.NewRecorder()
	if cache.ServeRootedFile(rec, req, root, "escape.txt", "text/plain; charset=utf-8") {
		t.Fatalf("ServeRootedFile served symlink escape with status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("ServeRootedFile symlink escape wrote status=%d body=%q", rec.Code, rec.Body.String())
	}

	path := writeCacheTestFile(t, root, "range.txt", "abcdef\n", time.Unix(400, 0))
	req = httptest.NewRequest(http.MethodGet, "/range.txt", nil)
	req.Header.Set("Range", "bytes=1-3")
	rec = httptest.NewRecorder()
	if !cache.ServeRootedFile(rec, req, root, filepath.Base(path), "text/plain; charset=utf-8") {
		t.Fatal("ServeRootedFile returned false for ordinary file")
	}
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", rec.Code)
	}
	if got, want := rec.Body.String(), "bcd"; got != want {
		t.Fatalf("range body = %q, want %q", got, want)
	}
}

func TestRawFeedRoutesDoNotEnterArtifactCache(t *testing.T) {
	eng, handler := testHandlerWithRuntime(t, Options{EnableAll: true}, "  web_artifact_cache_max_entries: 1\n")
	server := newWebHTTPTestServer(t, handler)

	status, _, body := server.get(t, "/sample.json")
	if status != http.StatusOK {
		t.Fatalf("metadata status = %d, want 200 body=%s", status, body)
	}

	status, _, body = server.get(t, "/files/sample.ipset")
	if status != http.StatusOK {
		t.Fatalf("raw feed status = %d, want 200 body=%s", status, body)
	}

	metadataPath := filepath.Join(eng.Runtime().WebDir, "sample.json")
	originalInfo, err := os.Stat(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	originalBody, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	replacement := bytes.Repeat([]byte("x"), len(originalBody))
	if err := os.WriteFile(metadataPath, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(metadataPath, originalInfo.ModTime(), originalInfo.ModTime()); err != nil {
		t.Fatal(err)
	}

	status, _, body = server.get(t, "/sample.json")
	if status != http.StatusOK {
		t.Fatalf("metadata after raw route status = %d, want 200 body=%s", status, body)
	}
	if got := body; !bytes.Equal(got, originalBody) {
		t.Fatalf("metadata cache was evicted by raw route: got %q, want cached original body", got)
	}
}

func writeCacheTestFile(t *testing.T, root, name, body string, mtime time.Time) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertCachedServeBody(t *testing.T, cache *fileCache, path, want string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+filepath.Base(path), nil)
	rec := httptest.NewRecorder()
	if !cache.ServeFile(rec, req, path, "") {
		t.Fatalf("ServeFile(%s) returned false", path)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ServeFile(%s) status = %d, want 200", path, rec.Code)
	}
	if got := rec.Body.String(); got != want {
		t.Fatalf("ServeFile(%s) body = %q, want %q", path, got, want)
	}
}

func assertFileCacheContainsOnly(t *testing.T, cache *fileCache, wantPath string, wantBytes int64) {
	t.Helper()
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if len(cache.files) != 1 {
		t.Fatalf("cache file count = %d, want 1", len(cache.files))
	}
	if cache.order.Len() != 1 {
		t.Fatalf("cache LRU entry count = %d, want 1", cache.order.Len())
	}
	if cache.bytes != wantBytes {
		t.Fatalf("cache bytes = %d, want %d", cache.bytes, wantBytes)
	}
	if _, ok := cache.files[wantPath]; !ok {
		t.Fatalf("cache is missing expected path %s", wantPath)
	}
	if gotPath, _ := cache.order.Front().Value.(string); gotPath != wantPath {
		t.Fatalf("cache LRU front = %s, want %s", gotPath, wantPath)
	}
}
