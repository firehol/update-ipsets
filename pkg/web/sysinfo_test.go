package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestDetailedStatusFields(t *testing.T) {
	info := detailedStatus()

	if info.Goroutines <= 0 {
		t.Fatal("expected positive goroutine count")
	}
	if info.HeapAlloc == 0 {
		t.Fatal("expected non-zero HeapAlloc")
	}
	if info.HeapSys == 0 {
		t.Fatal("expected non-zero HeapSys")
	}
	if info.Uptime == "" {
		t.Fatal("expected non-empty uptime string")
	}
	if info.UptimeSeconds <= 0 {
		t.Fatal("expected positive uptime seconds")
	}
}

func TestDetailedStatusRSSOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("RSS reading is Linux-only")
	}
	// Verify /proc/self/status exists.
	if _, err := os.Stat("/proc/self/status"); err != nil {
		t.Skip("/proc/self/status not available")
	}

	info := detailedStatus()
	if info.RSSKB == 0 {
		t.Fatal("expected non-zero RSS on Linux")
	}
	if info.VMSKB == 0 {
		t.Fatal("expected non-zero VMS on Linux")
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, tt := range tests {
		got := humanBytes(tt.input)
		if got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStatusEndpointReturnsPublicFieldsOnly(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer sourceServer.Close()

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
  status_test:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: test feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"),
		filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"),
		sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		EnableAll: true, Manual: true, CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	runner := scheduler.New(eng, true, nil)
	handler := newHandler(eng, Options{EnableAll: true}, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Result().Body)
	var result map[string]json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse status JSON: %v", err)
	}
	if _, ok := result["system"]; !ok {
		t.Fatal("missing 'system' key in status response")
	}
	if _, ok := result["scheduler"]; ok {
		t.Fatal("public status must not expose scheduler state")
	}

	var sys publicSystemStatus
	if err := json.Unmarshal(result["system"], &sys); err != nil {
		t.Fatalf("failed to parse system info: %v", err)
	}
	if sys.UptimeSeconds <= 0 {
		t.Fatal("expected positive uptime_seconds")
	}

	var engStatus publicEngineStatus
	if err := json.Unmarshal(result["engine"], &engStatus); err != nil {
		t.Fatalf("failed to parse engine info: %v", err)
	}
	if !engStatus.Running && engStatus.LastEnded.IsZero() {
		t.Fatal("expected public engine status to report coarse runtime state")
	}
	if engStatus.SourceCount <= 0 {
		t.Fatal("expected positive source_count")
	}

	var rawSystem map[string]any
	if err := json.Unmarshal(result["system"], &rawSystem); err != nil {
		t.Fatalf("failed to parse raw system info: %v", err)
	}
	for _, forbidden := range []string{"heap_alloc", "heap_sys", "goroutines", "disk_free", "rss_kb"} {
		if _, ok := rawSystem[forbidden]; ok {
			t.Fatalf("public status leaked operator-only field %q", forbidden)
		}
	}

	var rawEngine map[string]any
	if err := json.Unmarshal(result["engine"], &rawEngine); err != nil {
		t.Fatalf("failed to parse raw engine info: %v", err)
	}
	for _, forbidden := range []string{"active_feeds", "current_phase", "config_path", "base_dir", "last_report", "last_error"} {
		if _, ok := rawEngine[forbidden]; ok {
			t.Fatalf("public status leaked operator-only field %q", forbidden)
		}
	}
}
