package cache

import (
	"sync"
	"testing"
)

func TestRenameEntryConcurrentSnapshot(t *testing.T) {
	st := New()
	entry := st.Entry("old_name")
	entry.Entries = 42

	start := make(chan struct{})
	errCh := make(chan string, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for range 10000 {
			st.RenameEntry("old_name", "new_name")
			st.RenameEntry("new_name", "old_name")
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for range 20000 {
			snap := entry.Snapshot()
			if snap.Name != "old_name" && snap.Name != "new_name" {
				select {
				case errCh <- snap.Name:
				default:
				}
				return
			}
			if snap.Entries != 42 {
				select {
				case errCh <- "lost entries":
				default:
				}
				return
			}
		}
	}()
	close(start)
	wg.Wait()

	select {
	case got := <-errCh:
		t.Fatalf("unexpected concurrent snapshot result: %q", got)
	default:
	}
	if snap := st.EntrySnapshot("old_name"); snap == nil {
		t.Fatal("expected old_name after paired renames")
	} else if snap.Name != "old_name" || snap.Entries != 42 {
		t.Fatalf("unexpected final old_name snapshot: %+v", snap)
	}
	if snap := st.EntrySnapshot("new_name"); snap != nil {
		t.Fatalf("unexpected new_name after paired renames: %+v", snap)
	}
}
