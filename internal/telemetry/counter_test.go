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
