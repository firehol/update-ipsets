package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_acquireLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "update-ipsets.lock")
	first, err := acquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Release() }()
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != generatedFileMode {
		t.Fatalf("lock file mode = %04o, want %04o", got, generatedFileMode)
	}

	lockDir := filepath.Dir(path)
	if info, err := os.Stat(lockDir); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != generatedDirMode {
		t.Fatalf("lock dir mode = %04o, want %04o", got, generatedDirMode)
	}

	second, err := acquireLock(path)
	if err == nil {
		_ = second.Release()
		t.Fatal("expected second lock acquisition to fail")
	}
}
