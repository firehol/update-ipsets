package processor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/internal/fileutil"
)

func TestCopyFileWritesGroupReadableDestination(t *testing.T) {
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
	if gotMode := info.Mode().Perm(); gotMode != fileutil.GeneratedFileMode {
		t.Fatalf("copied file mode = %04o, want %04o", gotMode, fileutil.GeneratedFileMode)
	}
}
