package engine

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEngineLaneDiagnosticsLogsLongHeldSlot(t *testing.T) {
	var logs bytes.Buffer
	eng := newEngineFixture(t)
	eng.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	eng.engineLane = NewWorkLane(1)

	block := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- eng.engineLane.Run(t.Context(), LaneWork{
			ID:        "slow-work",
			Kind:      LaneWorkEntityRefresh,
			Component: LaneComponentEntityArtifacts,
			Name:      "entity.refresh",
			Stage:     "patching",
			Detail:    "testing long held lane work",
		}, func(context.Context) error {
			<-block
			return nil
		})
	}()

	waitForSnapshot(t, eng.engineLane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 1 && len(s.Active) == 1
	})
	active := eng.engineLane.Snapshot().Active[0]
	lastLogged := map[string]time.Time{}
	eng.logLongRunningEngineLaneWork(t.Context(), active.StartedAt.Add(engineLaneLongHoldThreshold), lastLogged)
	eng.logLongRunningEngineLaneWork(t.Context(), active.StartedAt.Add(engineLaneLongHoldThreshold+engineLaneLongHoldThreshold/2), lastLogged)

	close(block)
	if err := <-done; err != nil {
		t.Fatalf("lane Run() error = %v", err)
	}

	entry := findJSONLogByMessage(t, logs.String(), "engine lane work running longer than expected")
	if got := entry["id"]; got != "slow-work" {
		t.Fatalf("diagnostic id = %v, want slow-work", got)
	}
	if got := entry["elapsed_ms"].(float64); got < float64(engineLaneLongHoldThreshold.Milliseconds()) {
		t.Fatalf("diagnostic elapsed_ms = %v, want at least %d", got, engineLaneLongHoldThreshold.Milliseconds())
	}
	if got := strings.Count(logs.String(), "engine lane work running longer than expected"); got != 1 {
		t.Fatalf("long-held lane diagnostic log count = %d, want 1; logs:\n%s", got, logs.String())
	}
	warning := eng.StatusSnapshotLight().EngineLane.LongHoldWarning
	if warning == nil {
		t.Fatal("light status engine lane long-hold warning is nil")
	}
	if warning.ID != "slow-work" {
		t.Fatalf("light status long-hold warning id = %q, want slow-work", warning.ID)
	}
	if warning.ThresholdMS != engineLaneLongHoldThreshold.Milliseconds() {
		t.Fatalf("light status long-hold threshold = %d, want %d", warning.ThresholdMS, engineLaneLongHoldThreshold.Milliseconds())
	}
}

func TestEngineLaneDiagnosticsRecoversPanic(t *testing.T) {
	var logs bytes.Buffer
	var panicNext atomic.Bool
	panicNext.Store(true)
	eng := newEngineFixture(t)
	eng.logger = slog.New(&panicOnceHandler{
		inner:     slog.NewJSONHandler(&logs, nil),
		panicNext: &panicNext,
	})
	eng.engineLane = NewWorkLane(1)

	block := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- eng.engineLane.Run(t.Context(), LaneWork{
			ID:        "slow-work",
			Kind:      LaneWorkEntityRefresh,
			Component: LaneComponentEntityArtifacts,
			Name:      "entity.refresh",
			Stage:     "patching",
		}, func(context.Context) error {
			<-block
			return nil
		})
	}()

	waitForSnapshot(t, eng.engineLane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 1 && len(s.Active) == 1
	})
	active := eng.engineLane.Snapshot().Active[0]
	lastLogged := map[string]time.Time{}
	eng.logLongRunningEngineLaneWorkSafely(t.Context(), active.StartedAt.Add(engineLaneLongHoldThreshold), lastLogged)
	eng.logLongRunningEngineLaneWorkSafely(t.Context(), active.StartedAt.Add(2*engineLaneLongHoldThreshold), lastLogged)

	close(block)
	if err := <-done; err != nil {
		t.Fatalf("lane Run() error = %v", err)
	}
	if !strings.Contains(logs.String(), "engine lane diagnostics panic recovered") {
		t.Fatalf("panic recovery log missing: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "engine lane work running longer than expected") {
		t.Fatalf("diagnostics did not continue after panic: %s", logs.String())
	}
}

type panicOnceHandler struct {
	inner     slog.Handler
	panicNext *atomic.Bool
}

func (h *panicOnceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *panicOnceHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.panicNext != nil && h.panicNext.CompareAndSwap(true, false) {
		panic("forced engine lane diagnostics panic")
	}
	return h.inner.Handle(ctx, record)
}

func (h *panicOnceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &panicOnceHandler{inner: h.inner.WithAttrs(attrs), panicNext: h.panicNext}
}

func (h *panicOnceHandler) WithGroup(name string) slog.Handler {
	return &panicOnceHandler{inner: h.inner.WithGroup(name), panicNext: h.panicNext}
}
