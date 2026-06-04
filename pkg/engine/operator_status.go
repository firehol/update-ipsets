package engine

import (
	"strings"
)

type OperatorProblemClass string

const (
	OperatorProblemClassNone       OperatorProblemClass = ""
	OperatorProblemClassDownloader OperatorProblemClass = "downloader"
	OperatorProblemClassProcessing OperatorProblemClass = "processing"
)

type OperatorStatus struct {
	Label        string
	ProblemClass OperatorProblemClass
}

var staticOperatorStatusMeanings = map[string]OperatorStatus{
	"running":                                    {Label: "Run in progress"},
	DownloadStatusSkipped.String():               {Label: "No downloader work for this item"},
	DownloadStatusDisabled.String():              {Label: "Disabled"},
	DownloadStatusDownloading.String():           {Label: "Downloader working"},
	DownloadStatusDownloadFailed.String():        {Label: "Downloader fetch or materialization failed", ProblemClass: OperatorProblemClassDownloader},
	DownloadStatusMissingEnv.String():            {Label: "Downloader source URL is missing environment values", ProblemClass: OperatorProblemClassDownloader},
	DownloadStatusURLResolveFailed.String():      {Label: "Downloader could not resolve the source URL", ProblemClass: OperatorProblemClassDownloader},
	DownloadStatusNotModified.String():           {Label: "Upstream unchanged; kept existing local input"},
	DownloadStatusSame.String():                  {Label: "Local input unchanged after downloader work"},
	DownloadStatusDownloaded.String():            {Label: "Local input refreshed"},
	DownloadStatusPrepareFailed.String():         {Label: "Downloader could not prepare local input", ProblemClass: OperatorProblemClassDownloader},
	DownloadStatusHistorySnapshotFailed.String(): {Label: "Downloader could not update retained history snapshot", ProblemClass: OperatorProblemClassDownloader},
	DownloadStatusMaterializing.String():         {Label: "Materializing local child input"},
	ProcessingExceptionInvalidInput.String():     {Label: "Processing configuration is invalid", ProblemClass: OperatorProblemClassProcessing},
	ProcessingExceptionMissingInput.String():     {Label: "Processing local input is missing", ProblemClass: OperatorProblemClassProcessing},
	"processing":                                 {Label: "Processing local input"},
	ProcessingExceptionParse.String():            {Label: "Processing could not parse local input", ProblemClass: OperatorProblemClassProcessing},
	ProcessingExceptionFinalize.String():         {Label: "Processing could not publish outputs", ProblemClass: OperatorProblemClassProcessing},
	ProcessingExceptionRetention.String():        {Label: "Processing could not update retention artifacts", ProblemClass: OperatorProblemClassProcessing},
	ProcessingExceptionCancelled.String():        {Label: "Processing was cancelled before completion"},
	"updated":                                    {Label: "Local data refreshed"},
	"config_error":                               {Label: "Provider configuration is invalid", ProblemClass: OperatorProblemClassProcessing},
	"extract_failed":                             {Label: "Provider data extraction failed", ProblemClass: OperatorProblemClassProcessing},
	"open_failed":                                {Label: "Provider data could not be opened", ProblemClass: OperatorProblemClassProcessing},
	"unavailable":                                {Label: "Provider data is unavailable locally", ProblemClass: OperatorProblemClassProcessing},
	"stale":                                      {Label: "Using cached provider data after refresh failure", ProblemClass: OperatorProblemClassDownloader},
}

// OperatorStatusMeaning translates low-level cache/downloader/processing
// status codes into operator-facing meaning. This keeps the admin API and live
// queue surfaces from leaking implementation-specific status strings.
func OperatorStatusMeaning(raw string, downloadFailures int, isFeed bool) OperatorStatus {
	if raw == "" {
		return OperatorStatus{}
	}
	if raw == DownloadStatusFailed.String() {
		return failedOperatorStatus(downloadFailures, isFeed)
	}
	if raw == DownloadStatusEmpty.String() {
		return emptyOperatorStatus(isFeed)
	}
	if status, ok := staticOperatorStatusMeanings[raw]; ok {
		return status
	}
	return OperatorStatus{Label: fallbackOperatorStatusLabel(raw)}
}

func failedOperatorStatus(downloadFailures int, isFeed bool) OperatorStatus {
	if isFeed && downloadFailures == 0 {
		return OperatorStatus{
			Label:        "Processing local input failed",
			ProblemClass: OperatorProblemClassProcessing,
		}
	}
	return OperatorStatus{
		Label:        "Downloader local work failed",
		ProblemClass: OperatorProblemClassDownloader,
	}
}

func emptyOperatorStatus(isFeed bool) OperatorStatus {
	if isFeed {
		return OperatorStatus{Label: "Latest local result is empty"}
	}
	return OperatorStatus{Label: "Latest downloaded artifact is empty"}
}

func fallbackOperatorStatusLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Fields(strings.ReplaceAll(raw, "_", " "))
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}
