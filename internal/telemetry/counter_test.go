package telemetry

import (
	"reflect"
	"testing"
)

func TestCounterBookSnapshotSortsAndSanitizes(t *testing.T) {
	var nilBook *CounterBook
	if got := nilBook.Snapshot(); got != nil {
		t.Fatalf("nil Snapshot() = %#v, want nil", got)
	}

	var book CounterBook
	book.Add("", 10, 10)
	book.Add("ignored-negative", -1, -1)
	book.Add("beta", 2, 100)
	book.Add("alpha", 2, 100)
	book.Add("largest-bytes", 1, 200)
	book.Add("largest-bytes", 2, 0)

	want := []CounterStatSnapshot{
		{Name: "largest-bytes", Count: 3, Bytes: 200},
		{Name: "alpha", Count: 2, Bytes: 100},
		{Name: "beta", Count: 2, Bytes: 100},
	}
	if got := book.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestCounterBookTryAddDropsWhenLocked(t *testing.T) {
	var book CounterBook
	book.Add("kept", 1, 10)

	book.mu.Lock()
	if ok := book.TryAdd("dropped", 1, 10); ok {
		t.Fatal("TryAdd succeeded while book lock was held")
	}
	book.mu.Unlock()

	want := []CounterStatSnapshot{{Name: "kept", Count: 1, Bytes: 10}}
	if got := book.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() after dropped try-add = %#v, want %#v", got, want)
	}

	if ok := book.TryAdd("kept", 1, 10); !ok {
		t.Fatal("TryAdd failed after lock was released")
	}
	if got := book.Snapshot(); len(got) != 1 || got[0].Count != 2 || got[0].Bytes != 20 {
		t.Fatalf("Snapshot() after successful try-add = %#v, want kept count 2 bytes 20", got)
	}
}

func TestCounterBookTrySnapshotDropsWhenLocked(t *testing.T) {
	var book CounterBook
	book.Add("kept", 1, 10)

	book.mu.Lock()
	if got, ok := book.TrySnapshot(); ok || got != nil {
		t.Fatalf("TrySnapshot() while locked = %#v, %v; want nil, false", got, ok)
	}
	book.mu.Unlock()

	want := []CounterStatSnapshot{{Name: "kept", Count: 1, Bytes: 10}}
	if got, ok := book.TrySnapshot(); !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("TrySnapshot() after unlock = %#v, %v; want %#v, true", got, ok, want)
	}
}
