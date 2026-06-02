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
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock file mode = %04o, want 0600", got)
	}

	lockDir := filepath.Dir(path)
	if info, err := os.Stat(lockDir); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("lock dir mode = %04o, want 0700", got)
	}

	second, err := acquireLock(path)
	if err == nil {
		_ = second.Release()
		t.Fatal("expected second lock acquisition to fail")
	}
}
