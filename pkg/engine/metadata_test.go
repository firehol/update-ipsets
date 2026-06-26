package engine

import "testing"

func TestGitSyncWorkUsesStableCoalescingKey(t *testing.T) {
	eng := newEngineFixture(t)

	first := eng.gitSyncWork("publish.sync_generated_files")
	second := eng.gitSyncWork("publish.sync_generated_files")

	if first.ID == "" || second.ID == "" || first.ID == second.ID {
		t.Fatalf("git sync IDs = %q and %q, want unique non-empty IDs", first.ID, second.ID)
	}
	if first.CoalescingKey == "" {
		t.Fatal("git sync coalescing key is empty")
	}
	if first.CoalescingKey != second.CoalescingKey {
		t.Fatalf("git sync coalescing keys = %q and %q, want stable key", first.CoalescingKey, second.CoalescingKey)
	}
}
