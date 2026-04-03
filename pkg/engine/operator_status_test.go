package engine

import "testing"

func TestOperatorStatusMeaningProcessingFailure(t *testing.T) {
	got := OperatorStatusMeaning("parse_failed", 0, true)
	if got.Label != "Processing could not parse local input" {
		t.Fatalf("label = %q, want processing parse label", got.Label)
	}
	if got.ProblemClass != OperatorProblemClassProcessing {
		t.Fatalf("problem_class = %q, want %q", got.ProblemClass, OperatorProblemClassProcessing)
	}
}

func TestOperatorStatusMeaningDownloaderFailure(t *testing.T) {
	got := OperatorStatusMeaning("history_snapshot_failed", 2, true)
	if got.Label != "Downloader could not update retained history snapshot" {
		t.Fatalf("label = %q, want downloader snapshot label", got.Label)
	}
	if got.ProblemClass != OperatorProblemClassDownloader {
		t.Fatalf("problem_class = %q, want %q", got.ProblemClass, OperatorProblemClassDownloader)
	}
}

func TestOperatorStatusMeaningResolvesAmbiguousFailedStatus(t *testing.T) {
	got := OperatorStatusMeaning("failed", 3, true)
	if got.ProblemClass != OperatorProblemClassDownloader {
		t.Fatalf("downloader failed problem_class = %q, want %q", got.ProblemClass, OperatorProblemClassDownloader)
	}

	got = OperatorStatusMeaning("failed", 0, true)
	if got.ProblemClass != OperatorProblemClassProcessing {
		t.Fatalf("processing failed problem_class = %q, want %q", got.ProblemClass, OperatorProblemClassProcessing)
	}
}
