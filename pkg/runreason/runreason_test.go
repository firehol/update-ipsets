package runreason

import "testing"

func TestReasonValid(t *testing.T) {
	valid := []Reason{
		ReasonUnknown,
		ReasonScheduledDue,
		ReasonManualRun,
		ReasonManualRecheck,
		ReasonManualReprocess,
		ReasonStartupIntegrityReprocess,
		ReasonIntegrityReprocess,
		ReasonDependencyUpdate,
		ReasonProviderDefaults,
	}
	for _, reason := range valid {
		if !reason.Valid() {
			t.Fatalf("%q should be valid", reason)
		}
		if got := reason.String(); got != string(reason) {
			t.Fatalf("%q String() = %q, want %q", reason, got, string(reason))
		}
	}

	if Reason("unexpected").Valid() {
		t.Fatal("unexpected reason should be invalid")
	}
}
