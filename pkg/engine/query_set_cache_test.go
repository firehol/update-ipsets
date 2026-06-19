package engine

import (
	"context"
	"errors"
	"testing"
)

func TestOpenLatestSetForQueryHonorsCancelledContext(t *testing.T) {
	eng := newEngineFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	src, release, err := eng.openLatestSetForQuery(ctx, "alpha")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("openLatestSetForQuery() error = %v, want context.Canceled", err)
	}
	if src != nil {
		t.Fatalf("openLatestSetForQuery() src = %#v, want nil", src)
	}
	if release != nil {
		t.Fatal("openLatestSetForQuery() release should be nil on cancellation")
	}
}
