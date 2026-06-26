package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestWorkLaneRunCanceledAtActivationDoesNotStart(t *testing.T) {
	lane := NewWorkLane(1)
	releaseFirst := make(chan struct{})
	firstStarted := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- lane.Run(t.Context(), LaneWork{ID: "first", Kind: LaneWorkCleanup}, func(context.Context) error {
			close(firstStarted)
			<-releaseFirst
			return nil
		})
	}()
	<-firstStarted

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	secondDone := make(chan error, 1)
	secondStarted := make(chan struct{}, 1)
	go func() {
		secondDone <- lane.Run(ctx, LaneWork{ID: "second", Kind: LaneWorkCleanup}, func(context.Context) error {
			secondStarted <- struct{}{}
			return nil
		})
	}()
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.WaitingCount == 1
	})

	var cancelOnActivation sync.Once
	lane.startNotificationHook = func() {
		cancelOnActivation.Do(cancel)
	}
	close(releaseFirst)

	if err := <-firstDone; err != nil {
		t.Fatalf("first run returned error: %v", err)
	}
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("activated canceled Run error = %v, want context.Canceled", err)
	}
	select {
	case <-secondStarted:
		t.Fatal("canceled work started after activation")
	default:
	}
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 0 && s.WaitingCount == 0
	})
}

func TestWorkLaneSubmitUsesAttachedContextAfterAdmission(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	daemonCtx, daemonCancel := context.WithCancel(t.Context())
	defer daemonCancel()
	lane.AttachContext(daemonCtx, time.Second)

	releaseFirst := make(chan struct{})
	firstStarted := make(chan struct{})
	if _, err := lane.Submit(t.Context(), LaneWork{
		ID:            "active",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:active",
	}, func(context.Context) error {
		close(firstStarted)
		<-releaseFirst
		return nil
	}); err != nil {
		t.Fatalf("active Submit: %v", err)
	}
	<-firstStarted

	requestCtx, requestCancel := context.WithCancel(t.Context())
	startCtxErr := make(chan error, 1)
	done := make(chan struct{})
	ticket, err := lane.Submit(requestCtx, LaneWork{
		ID:            "accepted",
		Kind:          LaneWorkEntityRebuild,
		Component:     LaneComponentEntityArtifacts,
		CoalescingKey: "entity:rebuild:accepted",
	}, func(ctx context.Context) error {
		startCtxErr <- ctx.Err()
		close(done)
		return nil
	})
	if err != nil {
		t.Fatalf("accepted Submit: %v", err)
	}
	if !ticket.Queued {
		t.Fatalf("accepted Submit ticket = %+v, want queued", ticket)
	}
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 1 && s.WaitingCount == 1
	})

	requestCancel()
	close(releaseFirst)

	select {
	case err := <-startCtxErr:
		if err != nil {
			t.Fatalf("accepted work context error at start = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted work did not start")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("accepted work did not complete")
	}
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 0 && s.WaitingCount == 0
	})
}
