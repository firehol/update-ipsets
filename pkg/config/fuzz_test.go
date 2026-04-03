package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoadYAML(f *testing.F) {
	f.Add([]byte(`
sources:
  sample:
    url: https://example.test/feed.txt
    frequency: 60
    ipv: ipv4
    output: ipset
`))
	f.Add([]byte(`
runtime:
  parallel_downloads: 4
sources: {}
`))
	f.Add([]byte("sources:\n  bad:\n    url: http://[\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip("input exceeds bounded config fuzz size")
		}
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadYAML(path)
		if err != nil {
			return
		}
		var out bytes.Buffer
		if err := SaveYAML(&out, cfg); err != nil {
			t.Fatalf("save parsed config: %v", err)
		}
		roundTripPath := filepath.Join(t.TempDir(), "roundtrip.yaml")
		if err := os.WriteFile(roundTripPath, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadYAML(roundTripPath); err != nil {
			t.Fatalf("reload saved config: %v\nsaved=%s", err, out.String())
		}
	})
}
