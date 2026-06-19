package scheduler

import (
	"testing"

	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestQueuedProcessingReasonReprocess(t *testing.T) {
	cases := []struct {
		name   string
		reason runreason.Reason
		want   bool
	}{
		{"scheduled due", runreason.ReasonScheduledDue, false},
		{"manual run", runreason.ReasonManualRun, false},
		{"manual recheck", runreason.ReasonManualRecheck, false},
		{"dependency update", runreason.ReasonDependencyUpdate, false},
		{"manual reprocess", runreason.ReasonManualReprocess, true},
		{"integrity reprocess", runreason.ReasonIntegrityReprocess, true},
		{"startup integrity reprocess", runreason.ReasonStartupIntegrityReprocess, true},
		{"provider defaults", runreason.ReasonProviderDefaults, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := queuedProcessingReasonReprocess(tc.reason); got != tc.want {
				t.Fatalf("queuedProcessingReasonReprocess(%q) = %v, want %v", tc.reason, got, tc.want)
			}
		})
	}
}

func TestQueuedProcessingReprocessScansBatchReasons(t *testing.T) {
	cases := []struct {
		name  string
		items []queuedWork
		want  bool
	}{
		{
			name: "ordinary scheduled batch",
			items: []queuedWork{
				{Name: "one", Reason: runreason.ReasonScheduledDue},
				{Name: "two", Reason: runreason.ReasonScheduledDue},
			},
			want: false,
		},
		{
			name: "integrity item preserves reprocess intent in mixed batch",
			items: []queuedWork{
				{Name: "repair", Reason: runreason.ReasonIntegrityReprocess},
				{Name: "scheduled", Reason: runreason.ReasonScheduledDue},
			},
			want: true,
		},
		{
			name: "provider-default item preserves reprocess intent in mixed batch",
			items: []queuedWork{
				{Name: "provider", Reason: runreason.ReasonProviderDefaults},
				{Name: "manual", Reason: runreason.ReasonManualRun},
			},
			want: true,
		},
		{
			name: "manual recheck remains non-reprocess",
			items: []queuedWork{
				{Name: "recheck", Reason: runreason.ReasonManualRecheck},
				{Name: "scheduled", Reason: runreason.ReasonScheduledDue},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := queuedProcessingReprocess(tc.items); got != tc.want {
				t.Fatalf("queuedProcessingReprocess(%v) = %v, want %v", tc.items, got, tc.want)
			}
		})
	}
}

func TestCombineReasons(t *testing.T) {
	cases := []struct {
		name  string
		items []queuedWork
		want  runreason.Reason
	}{
		{name: "empty batch", want: runreason.ReasonScheduledDue},
		{
			name:  "empty reason defaults to scheduled",
			items: []queuedWork{{Name: "sample"}},
			want:  runreason.ReasonScheduledDue,
		},
		{
			name: "same reason passes through",
			items: []queuedWork{
				{Name: "one", Reason: runreason.ReasonScheduledDue},
				{Name: "two", Reason: runreason.ReasonScheduledDue},
			},
			want: runreason.ReasonScheduledDue,
		},
		{
			name: "manual reprocess wins",
			items: []queuedWork{
				{Name: "one", Reason: runreason.ReasonScheduledDue},
				{Name: "two", Reason: runreason.ReasonManualReprocess},
			},
			want: runreason.ReasonManualReprocess,
		},
		{
			name: "manual recheck wins over scheduled",
			items: []queuedWork{
				{Name: "one", Reason: runreason.ReasonManualRecheck},
				{Name: "two", Reason: runreason.ReasonScheduledDue},
			},
			want: runreason.ReasonManualRecheck,
		},
		{
			name: "mixed non-display reasons collapse to manual run",
			items: []queuedWork{
				{Name: "one", Reason: runreason.ReasonIntegrityReprocess},
				{Name: "two", Reason: runreason.ReasonScheduledDue},
			},
			want: runreason.ReasonManualRun,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := combineReasons(tc.items); got != tc.want {
				t.Fatalf("combineReasons(%v) = %q, want %q", tc.items, got, tc.want)
			}
		})
	}
}
