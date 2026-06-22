package engine

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestWorkLaneRunStartsFIFO(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	releaseFirst := make(chan struct{})
	firstStarted := make(chan struct{})
	var mu sync.Mutex
	var order []string

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- lane.Run(t.Context(), LaneWork{ID: "first", Kind: LaneWorkCleanup}, func(context.Context) error {
			close(firstStarted)
			<-releaseFirst
			mu.Lock()
			order = append(order, "first")
			mu.Unlock()
			return nil
		})
	}()
	<-firstStarted

	secondDone := make(chan error, 1)
	thirdDone := make(chan error, 1)
	go func() {
		secondDone <- lane.Run(t.Context(), LaneWork{ID: "second", Kind: LaneWorkCleanup}, func(context.Context) error {
			mu.Lock()
			order = append(order, "second")
			mu.Unlock()
			return nil
		})
	}()
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 1 && s.WaitingCount == 1
	})
	go func() {
		thirdDone <- lane.Run(t.Context(), LaneWork{ID: "third", Kind: LaneWorkCleanup}, func(context.Context) error {
			mu.Lock()
			order = append(order, "third")
			mu.Unlock()
			return nil
		})
	}()

	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 1 && s.WaitingCount == 2
	})
	close(releaseFirst)

	for name, ch := range map[string]<-chan error{
		"first":  firstDone,
		"second": secondDone,
		"third":  thirdDone,
	} {
		if err := <-ch; err != nil {
			t.Fatalf("%s run returned error: %v", name, err)
		}
	}

	mu.Lock()
	got := slices.Clone(order)
	mu.Unlock()
	want := []string{"first", "second", "third"}
	if !slices.Equal(got, want) {
		t.Fatalf("run order = %v, want %v", got, want)
	}
}

func TestWorkLaneLimitTwoRunsTwoJobs(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(2)
	release := make(chan struct{})
	started := make(chan string, 2)

	for _, id := range []string{"a", "b"} {
		id := id
		ticket, err := lane.Submit(t.Context(), LaneWork{
			ID:            id,
			Kind:          LaneWorkCleanup,
			CoalescingKey: "cleanup:" + id,
		}, func(context.Context) error {
			started <- id
			<-release
			return nil
		})
		if err != nil {
			t.Fatalf("Submit(%s): %v", id, err)
		}
		if ticket.State != LaneWorkActive {
			t.Fatalf("Submit(%s) state = %q, want active", id, ticket.State)
		}
	}

	seen := map[string]bool{<-started: true, <-started: true}
	if !seen["a"] || !seen["b"] {
		t.Fatalf("started = %v, want a and b", seen)
	}
	snap := lane.Snapshot()
	if snap.ActiveCount != 2 || snap.WaitingCount != 0 {
		t.Fatalf("snapshot active=%d waiting=%d, want 2/0", snap.ActiveCount, snap.WaitingCount)
	}
	close(release)
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 0
	})
}

func TestWorkLaneSetLimitLoweringDoesNotCancelActiveWork(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(2)
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	started := make(chan string, 2)
	for _, work := range []struct {
		id      string
		release chan struct{}
	}{
		{id: "a", release: releaseA},
		{id: "b", release: releaseB},
	} {
		work := work
		if _, err := lane.Submit(t.Context(), LaneWork{
			ID:            work.id,
			Kind:          LaneWorkCleanup,
			CoalescingKey: "cleanup:" + work.id,
		}, func(context.Context) error {
			started <- work.id
			<-work.release
			return nil
		}); err != nil {
			t.Fatalf("Submit(%s): %v", work.id, err)
		}
	}
	seen := map[string]bool{<-started: true, <-started: true}
	if !seen["a"] || !seen["b"] {
		t.Fatalf("started = %v, want a and b", seen)
	}

	lane.SetLimit(1)
	thirdStarted := make(chan struct{})
	if _, err := lane.Submit(t.Context(), LaneWork{
		ID:            "c",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:c",
	}, func(context.Context) error {
		close(thirdStarted)
		return nil
	}); err != nil {
		t.Fatalf("Submit(c): %v", err)
	}
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.Limit == 1 && s.ActiveCount == 2 && s.WaitingCount == 1
	})

	close(releaseA)
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.Limit == 1 && s.ActiveCount == 1 && s.WaitingCount == 1
	})
	select {
	case <-thirdStarted:
		t.Fatal("queued work started while active count still equaled lowered limit")
	default:
	}

	close(releaseB)
	select {
	case <-thirdStarted:
	case <-time.After(time.Second):
		t.Fatal("queued work did not start after active count dropped below lowered limit")
	}
}

func TestWorkLaneRunCanceledBeforeAdmission(t *testing.T) {
	t.Parallel()

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
	cancel()
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued Run error = %v, want context.Canceled", err)
	}
	select {
	case <-secondStarted:
		t.Fatal("canceled queued work started")
	default:
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first run returned error: %v", err)
	}
}

func TestWorkLaneSubmitCoalescesByKey(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	release := make(chan struct{})
	started := make(chan struct{})
	first, err := lane.Submit(t.Context(), LaneWork{
		ID:            "rebuild-1",
		Kind:          LaneWorkEntityRebuild,
		Component:     LaneComponentEntityArtifacts,
		CoalescingKey: "entity:rebuild:full",
	}, func(context.Context) error {
		close(started)
		<-release
		return nil
	})
	if err != nil {
		t.Fatalf("first Submit: %v", err)
	}
	<-started
	second, err := lane.Submit(t.Context(), LaneWork{
		ID:            "rebuild-2",
		Kind:          LaneWorkEntityRebuild,
		Component:     LaneComponentEntityArtifacts,
		CoalescingKey: "entity:rebuild:full",
	}, func(context.Context) error {
		t.Fatal("coalesced duplicate started")
		return nil
	})
	if err != nil {
		t.Fatalf("second Submit: %v", err)
	}
	if !second.Coalesced || second.ID != first.ID || second.State != LaneWorkActive {
		t.Fatalf("coalesced ticket = %+v, first = %+v", second, first)
	}
	close(release)
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 0
	})
}

func TestWorkLaneSubmitRejectsMissingCoalescingKey(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	_, err := lane.Submit(t.Context(), LaneWork{ID: "missing-key"}, func(context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrLaneMissingCoalescingKey) {
		t.Fatalf("Submit error = %v, want ErrLaneMissingCoalescingKey", err)
	}
}

func TestWorkLaneRunFromWorkerFailsFast(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	err := lane.Run(t.Context(), LaneWork{ID: "outer", Kind: LaneWorkCleanup}, func(ctx context.Context) error {
		return lane.Run(ctx, LaneWork{ID: "inner", Kind: LaneWorkCleanup}, func(context.Context) error {
			t.Fatal("inner run started")
			return nil
		})
	})
	if !errors.Is(err, ErrLaneReentrantRun) {
		t.Fatalf("Run error = %v, want ErrLaneReentrantRun", err)
	}
}

func TestWorkLaneTryRunDoesNotQueueWhenBusy(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	release := make(chan struct{})
	activeStarted := make(chan struct{})
	_, err := lane.Submit(t.Context(), LaneWork{
		ID:            "active",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:active",
	}, func(context.Context) error {
		close(activeStarted)
		<-release
		return nil
	})
	if err != nil {
		t.Fatalf("active Submit: %v", err)
	}
	<-activeStarted

	tryStarted := false
	ran, err := lane.TryRun(t.Context(), LaneWork{ID: "try", Kind: LaneWorkCleanup}, func(context.Context) error {
		tryStarted = true
		return nil
	})
	if err != nil {
		t.Fatalf("busy TryRun returned error: %v", err)
	}
	if ran || tryStarted {
		t.Fatalf("busy TryRun ran=%v tryStarted=%v, want false/false", ran, tryStarted)
	}

	close(release)
	waitForSnapshot(t, lane, func(s LaneSnapshot) bool {
		return s.ActiveCount == 0
	})
	ran, err = lane.TryRun(t.Context(), LaneWork{ID: "try-after", Kind: LaneWorkCleanup}, func(context.Context) error {
		tryStarted = true
		return nil
	})
	if err != nil {
		t.Fatalf("idle TryRun returned error: %v", err)
	}
	if !ran || !tryStarted {
		t.Fatalf("idle TryRun ran=%v tryStarted=%v, want true/true", ran, tryStarted)
	}
}

func TestWorkLaneSubmitFromWorkerQueuesWithoutDeadlock(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	innerStarted := make(chan struct{})
	err := lane.Run(t.Context(), LaneWork{ID: "outer", Kind: LaneWorkCleanup}, func(ctx context.Context) error {
		ticket, err := lane.Submit(ctx, LaneWork{
			ID:            "inner",
			Kind:          LaneWorkCleanup,
			CoalescingKey: "cleanup:inner",
		}, func(context.Context) error {
			close(innerStarted)
			return nil
		})
		if err != nil {
			return err
		}
		if !ticket.Queued || ticket.State != LaneWorkQueued {
			return errors.New("inner work did not queue behind active worker")
		}
		select {
		case <-innerStarted:
			return errors.New("inner work started before active worker returned")
		default:
		}
		return nil
	})
	if err != nil {
		t.Fatalf("outer Run returned error: %v", err)
	}
	select {
	case <-innerStarted:
	case <-time.After(time.Second):
		t.Fatal("inner work did not start after outer worker returned")
	}
}

func TestWorkLaneRunPanicReturnsErrorAndReleasesSlot(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	err := lane.Run(t.Context(), LaneWork{ID: "panic", Kind: LaneWorkCleanup}, func(context.Context) error {
		panic("boom")
	})
	if !errors.Is(err, ErrLanePanic) {
		t.Fatalf("Run error = %v, want ErrLanePanic", err)
	}

	started := false
	if err := lane.Run(t.Context(), LaneWork{ID: "after", Kind: LaneWorkCleanup}, func(context.Context) error {
		started = true
		return nil
	}); err != nil {
		t.Fatalf("Run after panic returned error: %v", err)
	}
	if !started {
		t.Fatal("work after panic did not start")
	}
}

func TestWorkLaneSubmitPanicReleasesSlotForQueuedWork(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	panicStarted := make(chan struct{})
	afterStarted := make(chan struct{})
	_, err := lane.Submit(t.Context(), LaneWork{
		ID:            "panic",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:panic",
	}, func(context.Context) error {
		close(panicStarted)
		panic("boom")
	})
	if err != nil {
		t.Fatalf("Submit panic work: %v", err)
	}
	<-panicStarted

	_, err = lane.Submit(t.Context(), LaneWork{
		ID:            "after",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:after",
	}, func(context.Context) error {
		close(afterStarted)
		return nil
	})
	if err != nil {
		t.Fatalf("Submit after panic: %v", err)
	}
	select {
	case <-afterStarted:
	case <-time.After(time.Second):
		t.Fatal("queued work did not start after panic")
	}
}

func TestWorkLaneShutdownCancelsQueuedAndActiveWork(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	activeCanceled := make(chan struct{})
	activeStarted := make(chan struct{})
	_, err := lane.Submit(t.Context(), LaneWork{
		ID:            "active",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:active",
	}, func(ctx context.Context) error {
		close(activeStarted)
		<-ctx.Done()
		close(activeCanceled)
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("active Submit: %v", err)
	}
	<-activeStarted

	queuedStarted := make(chan struct{})
	queued, err := lane.Submit(t.Context(), LaneWork{
		ID:            "queued",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:queued",
	}, func(context.Context) error {
		close(queuedStarted)
		return nil
	})
	if err != nil {
		t.Fatalf("queued Submit: %v", err)
	}
	if !queued.Queued {
		t.Fatalf("queued ticket = %+v, want queued", queued)
	}

	lane.Shutdown(time.Second)
	select {
	case <-activeCanceled:
	case <-time.After(time.Second):
		t.Fatal("active work was not canceled")
	}
	select {
	case <-queuedStarted:
		t.Fatal("queued work started during shutdown")
	default:
	}
	if _, err := lane.Submit(t.Context(), LaneWork{ID: "after", CoalescingKey: "after"}, func(context.Context) error {
		return nil
	}); !errors.Is(err, ErrLaneShuttingDown) {
		t.Fatalf("post-shutdown Submit error = %v, want ErrLaneShuttingDown", err)
	}
}

func TestWorkLaneShutdownIdleReturnsAndRejectsFutureWork(t *testing.T) {
	t.Parallel()

	lane := NewWorkLane(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		lane.Shutdown(time.Second)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle shutdown did not return")
	}

	_, err := lane.Submit(t.Context(), LaneWork{
		ID:            "after",
		Kind:          LaneWorkCleanup,
		CoalescingKey: "cleanup:after",
	}, func(context.Context) error {
		t.Fatal("post-shutdown work started")
		return nil
	})
	if !errors.Is(err, ErrLaneShuttingDown) {
		t.Fatalf("post-shutdown Submit error = %v, want ErrLaneShuttingDown", err)
	}
}

func waitForSnapshot(t *testing.T, lane *WorkLane, ok func(LaneSnapshot) bool) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if ok(lane.Snapshot()) {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("condition not met before timeout; snapshot=%+v", lane.Snapshot())
		case <-ticker.C:
		}
	}
}
