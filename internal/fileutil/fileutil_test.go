package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistsWithExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exists.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Exists(path) {
		t.Fatal("expected Exists to return true for existing file")
	}
}

func TestExistsWithNonExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.txt")
	if Exists(path) {
		t.Fatal("expected Exists to return false for non-existing file")
	}
}

func TestExistsWithDirectory(t *testing.T) {
	dir := t.TempDir()
	// Exists uses os.Stat, which succeeds for directories too.
	// The function says "file at path exists" but the implementation
	// returns true for directories as well — verify current behavior.
	if !Exists(dir) {
		t.Fatal("expected Exists to return true for a directory (os.Stat succeeds)")
	}
}

func TestWriteAtomicBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")
	data := []byte("atomic write content")
	mode := os.FileMode(0o640)

	if err := WriteAtomic(path, data, mode); err != nil {
		t.Fatalf("WriteAtomic returned error: %v", err)
	}

	// Verify content.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("content mismatch: got %q want %q", got, data)
	}

	// Verify permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("permission mismatch: got %04o want %04o", info.Mode().Perm(), mode)
	}
}

func TestWriteAtomicCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "file.txt")
	data := []byte("nested write")

	if err := WriteAtomic(path, data, 0o644); err != nil {
		t.Fatalf("WriteAtomic returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("content mismatch: got %q want %q", got, data)
	}
}

func TestWriteAtomicNoSyncBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "staged.json")
	data := []byte(`{"ok":true}`)

	if err := WriteAtomicNoSync(path, data, 0o644); err != nil {
		t.Fatalf("WriteAtomicNoSync returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("content mismatch: got %q want %q", got, data)
	}
}

func TestWriteAtomicNoLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.txt")

	if err := WriteAtomic(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteAtomic returned error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "clean.txt" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}

func TestWriteAtomicOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.txt")

	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteAtomic returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("expected overwritten content, got %q", got)
	}
}

func TestWriteAtomicReadOnlyDirectory(t *testing.T) {
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Restore permissions so cleanup can remove it.
		_ = os.Chmod(readOnly, 0o755)
	})

	path := filepath.Join(readOnly, "fail.txt")
	err := WriteAtomic(path, []byte("should fail"), 0o644)
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
}

func TestWriteAtomicEmptyData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")

	if err := WriteAtomic(path, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteAtomic returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file, got size %d", info.Size())
	}
}
