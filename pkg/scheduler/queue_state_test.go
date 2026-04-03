package scheduler

import (
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestQueueSnapshotCarriesOperatorFacingStatusMetadata(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	items := map[string]queuedWork{
		"sample": {
			Name:     "sample",
			Reason:   runreason.ReasonManualReprocess,
			QueuedAt: now,
		},
	}

	snapshot := queueSnapshotFromMap(items, func(name string) queueStatusView {
		return queueStatusView{
			Status:       "parse_failed",
			StatusLabel:  "Processing could not parse local input",
			ProblemClass: engine.OperatorProblemClassProcessing,
			Detail:       "invalid CIDR at line 1",
		}
	})

	if len(snapshot) != 1 {
		t.Fatalf("expected 1 queued item, got %#v", snapshot)
	}
	if got := snapshot[0].StatusLabel; got != "Processing could not parse local input" {
		t.Fatalf("status_label = %q, want operator-facing label", got)
	}
	if got := snapshot[0].ProblemClass; got != engine.OperatorProblemClassProcessing {
		t.Fatalf("problem_class = %q, want %q", got, engine.OperatorProblemClassProcessing)
	}
	if got := snapshot[0].Detail; got != "invalid CIDR at line 1" {
		t.Fatalf("detail = %q, want preserved detail", got)
	}
}
