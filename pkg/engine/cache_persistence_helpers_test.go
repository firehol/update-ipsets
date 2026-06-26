package engine

import (
	"context"
	"testing"
	"time"
)

func stopCachePersistenceForTest(t *testing.T, eng *Engine) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := eng.StopCachePersistence(ctx); err != nil {
		t.Fatalf("StopCachePersistence returned error: %v", err)
	}
}
