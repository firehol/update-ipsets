package cache

import (
	"path/filepath"
	"testing"
)

func TestStateSnapshotIsDetachedFromLaterMutations(t *testing.T) {
	st := New()
	entry := st.Entry("sample")
	entry.UniqueIPs = 42
	entry.File = "sample.ipset"

	snapshot := st.SnapshotState()

	st.Entry("sample").UniqueIPs = 99
	st.Entry("later").UniqueIPs = 7

	path := filepath.Join(t.TempDir(), "cache.json")
	if err := Save(path, snapshot); err != nil {
		t.Fatalf("Save(snapshot) returned error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load(snapshot) returned error: %v", err)
	}
	got := loaded.EntrySnapshot("sample")
	if got == nil {
		t.Fatal("snapshot lost sample entry")
	}
	if got.UniqueIPs != 42 {
		t.Fatalf("snapshot sample unique_ips = %d, want 42", got.UniqueIPs)
	}
	if loaded.EntrySnapshot("later") != nil {
		t.Fatal("snapshot included entry created after snapshot")
	}
}
