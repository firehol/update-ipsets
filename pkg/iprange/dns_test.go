package iprange

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type blockingIPv4Resolver struct {
	once    sync.Once
	started chan struct{}
}

func (r *blockingIPv4Resolver) LookupIPv4(ctx context.Context, _ string) ([]uint32, error) {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestResolveHostnamesStopsSchedulingAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	resolver := &blockingIPv4Resolver{started: make(chan struct{})}

	errCh := make(chan error, 1)
	go func() {
		_, err := ResolveHostnames(ctx, []string{"one.example", "two.example", "three.example"}, 1, resolver)
		errCh <- err
	}()

	select {
	case <-resolver.started:
	case <-time.After(2 * time.Second):
		t.Fatal("resolver did not receive the first host")
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ResolveHostnames err=%v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ResolveHostnames did not return after cancellation")
	}
}

func TestResolveHostnames6CanceledBeforeScheduling(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := ResolveHostnames6(ctx, []string{"one.example", "two.example"}, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveHostnames6 err=%v, want context.Canceled", err)
	}
}
