package cache

import (
	"testing"
	"time"
)

func TestTrySnapshotEntriesDoesNotWaitForStateLock(t *testing.T) {
	st := New()
	st.Entry("first").Entries = 10
	st.mu.Lock()
	defer st.mu.Unlock()

	done := make(chan bool, 1)
	go func() {
		_, ok := st.TrySnapshotEntries()
		done <- ok
	}()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("TrySnapshotEntries returned ok=true while state lock was held")
		}
	case <-time.After(time.Second):
		t.Fatal("TrySnapshotEntries waited for state lock")
	}
}

func TestTrySnapshotEntriesDoesNotWaitForEntryLock(t *testing.T) {
	st := New()
	entry := st.Entry("first")
	entry.Entries = 10
	entry.entryMu().Lock()
	defer entry.entryMu().Unlock()

	done := make(chan bool, 1)
	go func() {
		_, ok := st.TrySnapshotEntries()
		done <- ok
	}()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("TrySnapshotEntries returned ok=true while entry lock was held")
		}
	case <-time.After(time.Second):
		t.Fatal("TrySnapshotEntries waited for entry lock")
	}
}
