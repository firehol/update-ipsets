package web

import (
	"net/http"

	"github.com/firehol/update-ipsets/internal/observability"
)

func observeAPIRecalculation(r *http.Request, surface, action, result string, targets int) {
	observability.APIRecalculation(r.Context(), surface, action, result, int64(targets))
}
