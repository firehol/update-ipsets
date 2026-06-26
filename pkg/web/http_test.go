package web

import (
	"path/filepath"
	"testing"
)

func TestSafePathAllowsRootDirectory(t *testing.T) {
	got, ok := safePath(string(filepath.Separator), "sample.json")
	if !ok {
		t.Fatal("safePath rejected a relative file under the filesystem root")
	}
	want := filepath.Join(string(filepath.Separator), "sample.json")
	if got != want {
		t.Fatalf("safePath root result = %q, want %q", got, want)
	}
}

func TestSafePathRejectsRootTraversal(t *testing.T) {
	for _, name := range []string{"../etc/passwd", "/etc/passwd"} {
		if got, ok := safePath(string(filepath.Separator), name); ok {
			t.Fatalf("safePath(%q) = %q, true; want traversal rejection", name, got)
		}
	}
}
