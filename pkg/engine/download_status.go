package engine

import "github.com/firehol/update-ipsets/pkg/cache"

type DownloadStatus = cache.DownloadStatus

const (
	DownloadStatusUnknown               = cache.DownloadStatusUnknown
	DownloadStatusSkipped               = cache.DownloadStatusSkipped
	DownloadStatusDisabled              = cache.DownloadStatusDisabled
	DownloadStatusFailed                = cache.DownloadStatusFailed
	DownloadStatusDownloading           = cache.DownloadStatusDownloading
	DownloadStatusDownloadFailed        = cache.DownloadStatusDownloadFailed
	DownloadStatusMissingEnv            = cache.DownloadStatusMissingEnv
	DownloadStatusURLResolveFailed      = cache.DownloadStatusURLResolveFailed
	DownloadStatusNotModified           = cache.DownloadStatusNotModified
	DownloadStatusSame                  = cache.DownloadStatusSame
	DownloadStatusDownloaded            = cache.DownloadStatusDownloaded
	DownloadStatusEmpty                 = cache.DownloadStatusEmpty
	DownloadStatusPrepareFailed         = cache.DownloadStatusPrepareFailed
	DownloadStatusHistorySnapshotFailed = cache.DownloadStatusHistorySnapshotFailed
	DownloadStatusMaterializing         = cache.DownloadStatusMaterializing
)
