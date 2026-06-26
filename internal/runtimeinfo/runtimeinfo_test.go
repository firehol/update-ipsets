package runtimeinfo_test

import (
	"testing"

	"github.com/firehol/update-ipsets/internal/runtimeinfo"
)

func TestCaptureReturnsRuntimeAndProcessCounters(t *testing.T) {
	snap := runtimeinfo.Capture()
	if snap.Goroutines <= 0 {
		t.Fatalf("goroutines = %d, want positive runtime count", snap.Goroutines)
	}
	if snap.HeapAlloc == 0 && snap.HeapSys == 0 {
		t.Fatalf("heap counters are empty: %+v", snap)
	}
	if snap.GoMemLimit == 0 {
		t.Fatalf("go memory limit = 0, want -1 or a positive limit")
	}
}

func TestDiffClampsUnsignedCounterResets(t *testing.T) {
	start := runtimeinfo.Snapshot{NumGC: 10, PauseTotalMS: 20, ProcReadBytes: 100, ProcWriteBytes: 100}
	end := runtimeinfo.Snapshot{NumGC: 12, PauseTotalMS: 25, ProcReadBytes: 90, ProcWriteBytes: 150}

	delta := runtimeinfo.Diff(start, end)
	if delta.NumGC != 2 || delta.PauseTotalMS != 5 {
		t.Fatalf("runtime delta = %+v, want gc=2 pause=5", delta)
	}
	if delta.ProcReadBytes != 0 || delta.ProcWriteBytes != 50 {
		t.Fatalf("io delta = %+v, want read clamp 0 and write 50", delta)
	}
}
