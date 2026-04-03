package processor

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestStreamP2PBlocklistEquivalence(t *testing.T) {
	content := "Proxy:1.2.3.4-1.2.3.6\nOther:8.8.8.8-8.8.8.9\nInvalid:999\n"
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		steps []config.ProcessorStep
	}{
		{
			name:  "all ranges",
			steps: []config.ProcessorStep{{Name: "p2p_blocklist"}},
		},
		{
			name:  "proxy ranges",
			steps: []config.ProcessorStep{{Name: "p2p_blocklist_proxy"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bytesOut, err := Run(t.Context(), tc.steps, gz.Bytes())
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			tmpDir := t.TempDir()
			srcPath := filepath.Join(tmpDir, "input.p2p.gz")
			if err := os.WriteFile(srcPath, gz.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}

			resultPath, err := RunStream(t.Context(), tc.steps, srcPath, tmpDir)
			if err != nil {
				t.Fatalf("RunStream: %v", err)
			}
			defer func() { _ = os.Remove(resultPath) }()

			streamOut, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(streamOut) != string(bytesOut) {
				t.Fatalf("p2p stream mismatch:\n  bytes: %q\n stream: %q", bytesOut, streamOut)
			}
		})
	}
}

func TestStreamNoStepsHonorsCanceledContext(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.dat")
	if err := os.WriteFile(srcPath, []byte("1.2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	resultPath, err := RunStream(ctx, nil, srcPath, tmpDir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunStream err=%v, want context.Canceled", err)
	}
	if resultPath != "" {
		t.Fatalf("RunStream returned result path %q after cancellation", resultPath)
	}
}

func TestStreamByteFallbackHonorsCanceledContext(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.json")
	if err := os.WriteFile(srcPath, []byte(`{"items":[{"ip":"1.2.3.4"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	steps := []config.ProcessorStep{{Name: "json_path", Args: map[string]string{"path": "$.items[*].ip"}}}
	resultPath, err := RunStream(ctx, steps, srcPath, tmpDir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunStream err=%v, want context.Canceled", err)
	}
	if resultPath != "" {
		t.Fatalf("RunStream returned result path %q after cancellation", resultPath)
	}
}

func TestStreamCleansUpIntermediateAfterByteFallbackCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stepName := "test_stream_fallback_cancel"
	oldStep, hadStep := registry[stepName]
	t.Cleanup(func() {
		if hadStep {
			registry[stepName] = oldStep
		} else {
			delete(registry, stepName)
		}
		cancel()
	})

	registry[stepName] = func(context.Context, []byte, map[string]string) ([]byte, error) {
		cancel()
		return []byte("1.2.3.4\n"), nil
	}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "input.dat")
	if err := os.WriteFile(srcPath, []byte("1.2.3.4 # comment\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	steps := []config.ProcessorStep{
		{Name: "remove_comments"},
		{Name: stepName},
		{Name: "filter_all4"},
	}
	resultPath, err := RunStream(ctx, steps, srcPath, tmpDir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunStream err=%v, want context.Canceled", err)
	}
	if resultPath != "" {
		t.Fatalf("RunStream returned result path %q after cancellation", resultPath)
	}
	matches, err := filepath.Glob(filepath.Join(tmpDir, "proc-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("RunStream left intermediate temp files after cancellation: %v", matches)
	}
}
