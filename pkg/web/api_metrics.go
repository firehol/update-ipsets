package web

import (
	"net/http"

	"github.com/firehol/update-ipsets/internal/observability"
)

func observeAPIRecalculation(_ *http.Request, surface, action, result string, targets int) {
	observability.TryAPIRecalculation(surface, action, result, int64(targets))
}
