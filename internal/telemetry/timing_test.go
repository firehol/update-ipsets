package telemetry

import (
	"reflect"
	"testing"
	"time"
)

func TestTimingBookSnapshotSortsAndRoundsDurations(t *testing.T) {
	var nilBook *TimingBook
	if got := nilBook.Snapshot(); got != nil {
		t.Fatalf("nil Snapshot() = %#v, want nil", got)
	}

	var book TimingBook
	book.Observe("", time.Second)
	book.ObserveAggregate("ignored", 0, time.Second, time.Second)
	book.ObserveAggregate("clamped", 2, -time.Second, -time.Second)
	book.Observe("fast", 500*time.Microsecond)
	book.Observe("slow", 4*time.Millisecond)
	book.ObserveAggregate("slow", 2, 6*time.Millisecond, 5*time.Millisecond)

	want := []TimingStatSnapshot{
		{Name: "slow", Count: 3, TotalMS: 10, AvgMS: 3, MaxMS: 5},
		{Name: "fast", Count: 1, TotalMS: 1, AvgMS: 1, MaxMS: 1},
		{Name: "clamped", Count: 2, TotalMS: 0, AvgMS: 0, MaxMS: 0},
	}
	if got := book.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestTimingBookTryObserveDropsWhenLocked(t *testing.T) {
	var book TimingBook
	book.Observe("kept", time.Millisecond)

	book.mu.Lock()
	if ok := book.TryObserve("dropped", time.Millisecond); ok {
		t.Fatal("TryObserve succeeded while book lock was held")
	}
	book.mu.Unlock()

	want := []TimingStatSnapshot{{Name: "kept", Count: 1, TotalMS: 1, AvgMS: 1, MaxMS: 1}}
	if got := book.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() after dropped try-observe = %#v, want %#v", got, want)
	}

	if ok := book.TryObserve("kept", time.Millisecond); !ok {
		t.Fatal("TryObserve failed after lock was released")
	}
	if got := book.Snapshot(); len(got) != 1 || got[0].Count != 2 {
		t.Fatalf("Snapshot() after successful try-observe = %#v, want kept count 2", got)
	}
}

func TestTimingBookTrySnapshotDropsWhenLocked(t *testing.T) {
	var book TimingBook
	book.Observe("kept", time.Millisecond)

	book.mu.Lock()
	if got, ok := book.TrySnapshot(); ok || got != nil {
		t.Fatalf("TrySnapshot() while locked = %#v, %v; want nil, false", got, ok)
	}
	book.mu.Unlock()

	want := []TimingStatSnapshot{{Name: "kept", Count: 1, TotalMS: 1, AvgMS: 1, MaxMS: 1}}
	if got, ok := book.TrySnapshot(); !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("TrySnapshot() after unlock = %#v, %v; want %#v, true", got, ok, want)
	}
}
