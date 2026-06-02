package processor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileWritesPrivateDestination(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")
	want := []byte("copied\n")
	if err := os.WriteFile(srcPath, want, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(t.Context(), srcPath, dstPath); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("copied body = %q, want %q", got, want)
	}
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o600 {
		t.Fatalf("copied file mode = %04o, want 0600", gotMode)
	}
}
