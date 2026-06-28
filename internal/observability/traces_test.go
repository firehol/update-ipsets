package observability

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestTraceQueueDropsInsteadOfBlocking(t *testing.T) {
	resetMetricsForTest()
	configureTraceQueue(1)

	_, span := Start(context.Background(), "overflow.trace")
	End(span, nil)

	if got := counterValue(t, "telemetry.traces.dropped", nil); got == 0 {
		t.Fatal("telemetry.traces.dropped = 0, want a dropped trace event")
	}
	events := SnapshotTraceEvents()
	if len(events) != 1 {
		t.Fatalf("SnapshotTraceEvents() length = %d, want bounded ring length 1", len(events))
	}
}

func TestTraceCaptureDisabledByDefault(t *testing.T) {
	resetMetricsForTest()

	_, span := Start(context.Background(), "disabled.trace", String("status", "ok"))
	End(span, nil)

	if events := SnapshotTraceEvents(); len(events) != 0 {
		t.Fatalf("SnapshotTraceEvents() length = %d, want no events while traces are disabled", len(events))
	}
	if got := optionalCounterValue("telemetry.traces.dropped", nil); got != 0 {
		t.Fatalf("telemetry.traces.dropped = %d, want 0 while traces are disabled", got)
	}
}

func TestTraceQueueTruncatesOversizedStringPayloads(t *testing.T) {
	resetMetricsForTest()
	configureTraceQueue(1 << 20)

	_, span := Start(
		context.Background(),
		strings.Repeat("n", maxTraceStringBytes+1),
		String("oversized", strings.Repeat("v", maxTraceStringBytes+1)),
	)
	End(span, nil)
	events := SnapshotTraceEvents()
	if len(events) != 2 {
		t.Fatalf("SnapshotTraceEvents() length = %d, want 2", len(events))
	}
	if events[0].Name != truncatedTraceValue {
		t.Fatalf("trace name = %q, want truncated marker", events[0].Name)
	}
	if got := events[0].Attrs[0].s; got != truncatedTraceValue {
		t.Fatalf("trace attr value = %q, want truncated marker", got)
	}
}

func TestTraceQueueDoesNotDropBecauseProducersContend(t *testing.T) {
	resetMetricsForTest()
	configureTraceQueue(1 << 20)

	const goroutines = 8
	const perGoroutine = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for n := 0; n < perGoroutine; n++ {
				_, span := Start(context.Background(), "concurrent.trace", String("download.status", "ok"))
				End(span, nil)
			}
		}()
	}
	wg.Wait()
	events := SnapshotTraceEvents()
	wantEvents := goroutines * perGoroutine * 2
	if len(events) != wantEvents {
		t.Fatalf("SnapshotTraceEvents() length = %d, want %d", len(events), wantEvents)
	}
	if got := optionalCounterValue("telemetry.traces.dropped", nil); got != 0 {
		t.Fatalf("telemetry.traces.dropped = %d, want 0 with sufficient queue capacity", got)
	}
}
