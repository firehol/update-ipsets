package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/cache"
)

func TestRunCacheMergePreservesLocalOnlyEntries(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".cache")
	localJSONPath := filepath.Join(dir, ".cache.json")
	localOnlyPath := filepath.Join(dir, "local-only.txt")
	outPath := filepath.Join(dir, "merged.json")

	legacy := `declare -A IPSET_FILE=([production]="production.ipset" )
declare -A IPSET_IPS=([production]="42" )`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	local := cache.New()
	local.Entry("production").UniqueIPs = 1
	local.Entry("local_only").File = "local_only.ipset"
	local.Entry("local_only").UniqueIPs = 7
	if err := cache.Save(localJSONPath, local); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localOnlyPath, []byte("local_only\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := runCacheMerge([]string{
		"--legacy", legacyPath,
		"--local-json", localJSONPath,
		"--local-only", localOnlyPath,
		"--out", outPath,
	}); code != 0 {
		t.Fatalf("runCacheMerge returned %d", code)
	}

	merged, err := cache.Load(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Entry("production").UniqueIPs; got != 42 {
		t.Fatalf("production entry should come from legacy cache, got %d", got)
	}
	if got := merged.Entry("local_only").UniqueIPs; got != 7 {
		t.Fatalf("local-only entry should be preserved, got %d", got)
	}
}
