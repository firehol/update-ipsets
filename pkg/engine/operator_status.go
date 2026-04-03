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

// OperatorStatusMeaning translates low-level cache/downloader/processing
// status codes into operator-facing meaning. This keeps the admin API and live
// queue surfaces from leaking implementation-specific status strings.
func OperatorStatusMeaning(raw string, downloadFailures int, isFeed bool) OperatorStatus {
	switch raw {
	case "":
		return OperatorStatus{}
	case "running":
		return OperatorStatus{Label: "Run in progress"}
	case DownloadStatusSkipped.String():
		return OperatorStatus{Label: "No downloader work for this item"}
	case DownloadStatusDisabled.String():
		return OperatorStatus{Label: "Disabled"}
	case DownloadStatusFailed.String():
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
	case DownloadStatusDownloading.String():
		return OperatorStatus{Label: "Downloader working"}
	case DownloadStatusDownloadFailed.String():
		return OperatorStatus{
			Label:        "Downloader fetch or materialization failed",
			ProblemClass: OperatorProblemClassDownloader,
		}
	case DownloadStatusMissingEnv.String():
		return OperatorStatus{
			Label:        "Downloader source URL is missing environment values",
			ProblemClass: OperatorProblemClassDownloader,
		}
	case DownloadStatusURLResolveFailed.String():
		return OperatorStatus{
			Label:        "Downloader could not resolve the source URL",
			ProblemClass: OperatorProblemClassDownloader,
		}
	case DownloadStatusNotModified.String():
		return OperatorStatus{Label: "Upstream unchanged; kept existing local input"}
	case DownloadStatusSame.String():
		return OperatorStatus{Label: "Local input unchanged after downloader work"}
	case DownloadStatusDownloaded.String():
		return OperatorStatus{Label: "Local input refreshed"}
	case DownloadStatusEmpty.String():
		if isFeed {
			return OperatorStatus{Label: "Latest local result is empty"}
		}
		return OperatorStatus{Label: "Latest downloaded artifact is empty"}
	case DownloadStatusPrepareFailed.String():
		return OperatorStatus{
			Label:        "Downloader could not prepare local input",
			ProblemClass: OperatorProblemClassDownloader,
		}
	case DownloadStatusHistorySnapshotFailed.String():
		return OperatorStatus{
			Label:        "Downloader could not update retained history snapshot",
			ProblemClass: OperatorProblemClassDownloader,
		}
	case DownloadStatusMaterializing.String():
		return OperatorStatus{Label: "Materializing local child input"}
	case ProcessingExceptionInvalidInput.String():
		return OperatorStatus{
			Label:        "Processing configuration is invalid",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case ProcessingExceptionMissingInput.String():
		return OperatorStatus{
			Label:        "Processing local input is missing",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case "processing":
		return OperatorStatus{Label: "Processing local input"}
	case ProcessingExceptionParse.String():
		return OperatorStatus{
			Label:        "Processing could not parse local input",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case ProcessingExceptionFinalize.String():
		return OperatorStatus{
			Label:        "Processing could not publish outputs",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case ProcessingExceptionRetention.String():
		return OperatorStatus{
			Label:        "Processing could not update retention artifacts",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case ProcessingExceptionCancelled.String():
		return OperatorStatus{Label: "Processing was cancelled before completion"}
	case "updated":
		return OperatorStatus{Label: "Local data refreshed"}
	case "empty":
		return OperatorStatus{Label: "Latest local result is empty"}
	case "config_error":
		return OperatorStatus{
			Label:        "Provider configuration is invalid",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case "extract_failed":
		return OperatorStatus{
			Label:        "Provider data extraction failed",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case "open_failed":
		return OperatorStatus{
			Label:        "Provider data could not be opened",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case "unavailable":
		return OperatorStatus{
			Label:        "Provider data is unavailable locally",
			ProblemClass: OperatorProblemClassProcessing,
		}
	case "stale":
		return OperatorStatus{
			Label:        "Using cached provider data after refresh failure",
			ProblemClass: OperatorProblemClassDownloader,
		}
	default:
		return OperatorStatus{Label: fallbackOperatorStatusLabel(raw)}
	}
}

func fallbackOperatorStatusLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Fields(strings.ReplaceAll(raw, "_", " "))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}
