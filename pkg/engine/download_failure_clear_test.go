package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const (
	rawFeedServerFail int32 = iota
	rawFeedServerOK
	rawFeedServerNotModified
)

func TestRawFeedSuccessClearsPriorDownloadFailure(t *testing.T) {
	t.Run("downloaded", func(t *testing.T) {
		sourceURL, mode := newSwitchingRawFeedServer(t, []byte("1.2.3.4\n"), time.Time{})
		eng := newRawFeedDownloadEngine(t, sourceURL)

		assertFetchAndStageFails(t, eng)
		mode.Store(rawFeedServerOK)
		assertFetchAndStageStatus(t, eng, DownloadStatusDownloaded)
		assertRawFeedFailureCleared(t, eng)
	})

	t.Run("empty", func(t *testing.T) {
		sourceURL, mode := newSwitchingRawFeedServer(t, nil, time.Time{})
		eng := newRawFeedDownloadEngine(t, sourceURL)

		assertFetchAndStageFails(t, eng)
		mode.Store(rawFeedServerOK)
		assertFetchAndStageStatus(t, eng, DownloadStatusEmpty)
		assertRawFeedFailureCleared(t, eng)
	})

	t.Run("same", func(t *testing.T) {
		sourceURL, mode := newSwitchingRawFeedServer(t, []byte("1.2.3.4\n"), time.Time{})
		mode.Store(rawFeedServerOK)
		eng := newRawFeedDownloadEngine(t, sourceURL)
		assertFetchAndStageStatus(t, eng, DownloadStatusDownloaded)

		mode.Store(rawFeedServerFail)
		assertFetchAndStageFails(t, eng)
		mode.Store(rawFeedServerOK)
		assertFetchAndStageStatus(t, eng, DownloadStatusSame)
		assertRawFeedFailureCleared(t, eng)
	})

	t.Run("not modified", func(t *testing.T) {
		modifiedAt := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
		sourceURL, mode := newSwitchingRawFeedServer(t, []byte("1.2.3.4\n"), modifiedAt)
		mode.Store(rawFeedServerOK)
		eng := newRawFeedDownloadEngine(t, sourceURL)
		assertFetchAndStageStatus(t, eng, DownloadStatusDownloaded)

		mode.Store(rawFeedServerFail)
		assertFetchAndStageFails(t, eng)
		mode.Store(rawFeedServerNotModified)
		assertFetchAndStageStatus(t, eng, DownloadStatusNotModified)
		assertRawFeedFailureCleared(t, eng)
	})
}

func newSwitchingRawFeedServer(t *testing.T, body []byte, modifiedAt time.Time) (string, *atomic.Int32) {
	t.Helper()

	var mode atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch mode.Load() {
		case rawFeedServerOK:
			if !modifiedAt.IsZero() {
				w.Header().Set("Last-Modified", modifiedAt.Format(http.TimeFormat))
			}
			_, _ = w.Write(body)
		case rawFeedServerNotModified:
			w.WriteHeader(http.StatusNotModified)
		default:
			http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL, &mode
}

func newRawFeedDownloadEngine(t *testing.T, sourceURL string) *Engine {
	t.Helper()

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  sample:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: tests
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceURL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func assertFetchAndStageStatus(t *testing.T, eng *Engine, want DownloadStatus) {
	t.Helper()

	decision, err := eng.FetchAndStage(t.Context(), "sample", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != want {
		t.Fatalf("decision status = %q, want %q", decision.Status, want)
	}
}

func assertFetchAndStageFails(t *testing.T, eng *Engine) {
	t.Helper()

	decision, err := eng.FetchAndStage(t.Context(), "sample", false, true)
	if err == nil {
		t.Fatalf("FetchAndStage succeeded with status %q, want failure", decision.Status)
	}
	if decision.Status != DownloadStatusDownloadFailed {
		t.Fatalf("decision status = %q, want %q", decision.Status, DownloadStatusDownloadFailed)
	}
}

func assertRawFeedFailureCleared(t *testing.T, eng *Engine) {
	t.Helper()

	entry := eng.state.EntrySnapshot("sample")
	if entry == nil {
		t.Fatal("missing sample cache entry")
	}
	if entry.DownloadFailures != 0 {
		t.Fatalf("download failures = %d, want 0", entry.DownloadFailures)
	}
	if entry.FailureStartedDate != 0 {
		t.Fatalf("failure started date = %d, want 0", entry.FailureStartedDate)
	}
}
