package engine

import "testing"

func TestOperatorStatusMeaningStaticStatuses(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want OperatorStatus
	}{
		{"running", "running", OperatorStatus{Label: "Run in progress"}},
		{"skipped", DownloadStatusSkipped.String(), OperatorStatus{Label: "No downloader work for this item"}},
		{"disabled", DownloadStatusDisabled.String(), OperatorStatus{Label: "Disabled"}},
		{"downloading", DownloadStatusDownloading.String(), OperatorStatus{Label: "Downloader working"}},
		{"download failed", DownloadStatusDownloadFailed.String(), OperatorStatus{Label: "Downloader fetch or materialization failed", ProblemClass: OperatorProblemClassDownloader}},
		{"missing env", DownloadStatusMissingEnv.String(), OperatorStatus{Label: "Downloader source URL is missing environment values", ProblemClass: OperatorProblemClassDownloader}},
		{"url resolve failed", DownloadStatusURLResolveFailed.String(), OperatorStatus{Label: "Downloader could not resolve the source URL", ProblemClass: OperatorProblemClassDownloader}},
		{"not modified", DownloadStatusNotModified.String(), OperatorStatus{Label: "Upstream unchanged; kept existing local input"}},
		{"same", DownloadStatusSame.String(), OperatorStatus{Label: "Local input unchanged after downloader work"}},
		{"downloaded", DownloadStatusDownloaded.String(), OperatorStatus{Label: "Local input refreshed"}},
		{"prepare failed", DownloadStatusPrepareFailed.String(), OperatorStatus{Label: "Downloader could not prepare local input", ProblemClass: OperatorProblemClassDownloader}},
		{"history snapshot failed", DownloadStatusHistorySnapshotFailed.String(), OperatorStatus{Label: "Downloader could not update retained history snapshot", ProblemClass: OperatorProblemClassDownloader}},
		{"materializing", DownloadStatusMaterializing.String(), OperatorStatus{Label: "Materializing local child input"}},
		{"invalid input", ProcessingExceptionInvalidInput.String(), OperatorStatus{Label: "Processing configuration is invalid", ProblemClass: OperatorProblemClassProcessing}},
		{"missing input", ProcessingExceptionMissingInput.String(), OperatorStatus{Label: "Processing local input is missing", ProblemClass: OperatorProblemClassProcessing}},
		{"processing", "processing", OperatorStatus{Label: "Processing local input"}},
		{"parse failed", ProcessingExceptionParse.String(), OperatorStatus{Label: "Processing could not parse local input", ProblemClass: OperatorProblemClassProcessing}},
		{"finalize failed", ProcessingExceptionFinalize.String(), OperatorStatus{Label: "Processing could not publish outputs", ProblemClass: OperatorProblemClassProcessing}},
		{"retention failed", ProcessingExceptionRetention.String(), OperatorStatus{Label: "Processing could not update retention artifacts", ProblemClass: OperatorProblemClassProcessing}},
		{"cancelled", ProcessingExceptionCancelled.String(), OperatorStatus{Label: "Processing was cancelled before completion"}},
		{"updated", "updated", OperatorStatus{Label: "Local data refreshed"}},
		{"config error", "config_error", OperatorStatus{Label: "Provider configuration is invalid", ProblemClass: OperatorProblemClassProcessing}},
		{"extract failed", "extract_failed", OperatorStatus{Label: "Provider data extraction failed", ProblemClass: OperatorProblemClassProcessing}},
		{"open failed", "open_failed", OperatorStatus{Label: "Provider data could not be opened", ProblemClass: OperatorProblemClassProcessing}},
		{"unavailable", "unavailable", OperatorStatus{Label: "Provider data is unavailable locally", ProblemClass: OperatorProblemClassProcessing}},
		{"stale", "stale", OperatorStatus{Label: "Using cached provider data after refresh failure", ProblemClass: OperatorProblemClassDownloader}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OperatorStatusMeaning(tt.raw, 0, true)
			assertOperatorStatus(t, got, tt.want)
		})
	}
}

func TestOperatorStatusMeaningContextSensitiveStatuses(t *testing.T) {
	tests := []struct {
		name             string
		raw              string
		downloadFailures int
		isFeed           bool
		want             OperatorStatus
	}{
		{
			name:             "failed feed with download failures",
			raw:              DownloadStatusFailed.String(),
			downloadFailures: 3,
			isFeed:           true,
			want:             OperatorStatus{Label: "Downloader local work failed", ProblemClass: OperatorProblemClassDownloader},
		},
		{
			name:   "failed feed without download failures",
			raw:    DownloadStatusFailed.String(),
			isFeed: true,
			want:   OperatorStatus{Label: "Processing local input failed", ProblemClass: OperatorProblemClassProcessing},
		},
		{
			name: "failed artifact without download failures",
			raw:  DownloadStatusFailed.String(),
			want: OperatorStatus{Label: "Downloader local work failed", ProblemClass: OperatorProblemClassDownloader},
		},
		{
			name:   "empty feed",
			raw:    DownloadStatusEmpty.String(),
			isFeed: true,
			want:   OperatorStatus{Label: "Latest local result is empty"},
		},
		{
			name: "empty artifact",
			raw:  DownloadStatusEmpty.String(),
			want: OperatorStatus{Label: "Latest downloaded artifact is empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OperatorStatusMeaning(tt.raw, tt.downloadFailures, tt.isFeed)
			assertOperatorStatus(t, got, tt.want)
		})
	}
}

func TestOperatorStatusMeaningFallbacks(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty", "", ""},
		{"spaces only", "   ", ""},
		{"trimmed title", "  custom_status  ", "Custom Status"},
		{"multiple spaces", "custom   raw", "Custom Raw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OperatorStatusMeaning(tt.raw, 0, true)
			if got.Label != tt.want {
				t.Fatalf("label = %q, want %q", got.Label, tt.want)
			}
			if got.ProblemClass != OperatorProblemClassNone {
				t.Fatalf("problem_class = %q, want empty", got.ProblemClass)
			}
		})
	}
}

func assertOperatorStatus(t *testing.T, got, want OperatorStatus) {
	t.Helper()
	if got.Label != want.Label {
		t.Fatalf("label = %q, want %q", got.Label, want.Label)
	}
	if got.ProblemClass != want.ProblemClass {
		t.Fatalf("problem_class = %q, want %q", got.ProblemClass, want.ProblemClass)
	}
}
