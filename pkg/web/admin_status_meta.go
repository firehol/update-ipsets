package web

import "github.com/firehol/update-ipsets/pkg/engine"

type adminProblemClass = engine.OperatorProblemClass

const (
	adminProblemClassNone       = engine.OperatorProblemClassNone
	adminProblemClassDownloader = engine.OperatorProblemClassDownloader
	adminProblemClassProcessing = engine.OperatorProblemClassProcessing
)

func populateAdminFeedStatusMeta(feed *adminFeed) {
	if feed == nil {
		return
	}
	status := engine.OperatorStatusMeaning(feed.LastStatus, feed.DownloadFailures, true)
	feed.LastStatusLabel = status.Label
	feed.LastProblemClass = status.ProblemClass
}

func populateAdminArtifactStatusMeta(artifact *adminArtifact) {
	if artifact == nil {
		return
	}
	status := engine.OperatorStatusMeaning(artifact.LastStatus, artifact.DownloadFailures, false)
	artifact.LastStatusLabel = status.Label
	artifact.LastProblemClass = status.ProblemClass
}
