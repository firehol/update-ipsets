package engine

import (
	"context"
	"testing"
	"time"
)

func TestWorkLaneDuplicateExplicitIDsDoNotShareActiveSlot(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(2)
	release := make(chan struct{})
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	work := LaneWork{ID: "duplicate", Kind: LaneWorkCleanup}

	go func() {
		firstDone <- lane.Run(t.Context(), work, func(context.Context) error {
			close(firstStarted)
			<-release
			return nil
		})
	}()
	go func() {
		secondDone <- lane.Run(t.Context(), work, func(context.Context) error {
			close(secondStarted)
			<-release
			return nil
		})
	}()

	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 2
	})
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first duplicate-ID work did not start")
	}
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second duplicate-ID work did not start")
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first duplicate-ID work returned error: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second duplicate-ID work returned error: %v", err)
	}
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 0 && s.WaitingCount == 0
	})
}
