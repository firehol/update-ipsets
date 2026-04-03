package cache

type DownloadStatus string

const (
	DownloadStatusUnknown               DownloadStatus = ""
	DownloadStatusSkipped               DownloadStatus = "skipped"
	DownloadStatusDisabled              DownloadStatus = "disabled"
	DownloadStatusFailed                DownloadStatus = "failed"
	DownloadStatusDownloading           DownloadStatus = "downloading"
	DownloadStatusDownloadFailed        DownloadStatus = "download_failed"
	DownloadStatusMissingEnv            DownloadStatus = "missing_env"
	DownloadStatusURLResolveFailed      DownloadStatus = "url_resolve_failed"
	DownloadStatusNotModified           DownloadStatus = "not_modified"
	DownloadStatusSame                  DownloadStatus = "same"
	DownloadStatusDownloaded            DownloadStatus = "downloaded"
	DownloadStatusEmpty                 DownloadStatus = "empty"
	DownloadStatusPrepareFailed         DownloadStatus = "prepare_failed"
	DownloadStatusHistorySnapshotFailed DownloadStatus = "history_snapshot_failed"
	DownloadStatusMaterializing         DownloadStatus = "materializing"
)

func (s DownloadStatus) String() string {
	return string(s)
}
