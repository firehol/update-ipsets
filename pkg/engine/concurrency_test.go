package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestRunBoundedNameJobsStopsSchedulingAfterContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		firstStarted := make(chan struct{})
		releaseFirst := make(chan struct{})
		var processed atomic.Int64

		errCh := make(chan error, 1)
		go func() {
			errCh <- runBoundedNameJobs(ctx, 1, []string{"first", "second", "third"}, func(ctx context.Context, name string) error {
				processed.Add(1)
				if name == "first" {
					close(firstStarted)
					<-releaseFirst
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return nil
			})
		}()

		<-firstStarted

		cancel()
		close(releaseFirst)
		synctest.Wait()

		err := <-errCh
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runBoundedNameJobs err=%v, want context.Canceled", err)
		}

		if got := processed.Load(); got != 1 {
			t.Fatalf("processed jobs=%d, want only the in-flight job to run", got)
		}
	})
}

func TestRunBoundedNameJobsReturnsJoinedWorkerErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		release := make(chan struct{})
		started := make(chan string, 3)
		workerErrs := map[string]error{
			"first":  errors.New("first failed"),
			"second": errors.New("second failed"),
			"third":  errors.New("third failed"),
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- runBoundedNameJobs(ctx, 3, []string{"first", "second", "third"}, func(_ context.Context, name string) error {
				started <- name
				<-release
				return fmt.Errorf("worker %s: %w", name, workerErrs[name])
			})
		}()

		seen := map[string]struct{}{}
		for len(seen) < len(workerErrs) {
			name := <-started
			seen[name] = struct{}{}
		}
		close(release)
		synctest.Wait()

		err := <-errCh
		if err == nil {
			t.Fatal("runBoundedNameJobs returned nil, want joined error")
		}
		for name, want := range workerErrs {
			if !errors.Is(err, want) {
				t.Fatalf("joined error %q does not wrap %s", err, want)
			}
			if !strings.Contains(err.Error(), "worker "+name) {
				t.Fatalf("joined error %q does not include worker %s context", err, name)
			}
		}
	})
}
