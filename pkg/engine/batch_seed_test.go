package engine

import (
	"testing"
)

func TestBatchSeedHelpersAreNoLongerPartOfStatusSnapshot(t *testing.T) {
	eng := newEngineFixture(t)
	status := eng.StatusSnapshot()
	if len(status.ActiveFeeds) != 0 {
		t.Fatalf("expected no active feeds by default, got %#v", status.ActiveFeeds)
	}
}
