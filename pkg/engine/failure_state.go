package engine

import "github.com/firehol/update-ipsets/pkg/cache"

func (e *Engine) incrementFailure(entry *cache.Entry) {
	if entry == nil {
		return
	}
	entry.RecordDownloadFailure(e.now().UTC().Unix())
}

func clearFailure(entry *cache.Entry) {
	if entry == nil {
		return
	}
	entry.ClearDownloadFailure()
}
