package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func handleHomeGlobe(eng *engine.Engine, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		raw := strings.TrimSpace(r.URL.Query().Get("categories"))
		if raw == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing categories query parameter"})
			return
		}
		payload, err := eng.HomeGlobeInDir(parseList(raw), outputDir)
		if err != nil {
			if errors.Is(err, engine.ErrHomeAggregatesNotReady) {
				jsonError(w, http.StatusServiceUnavailable, err)
				return
			}
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

// handleHomeSummary serves /api/v1/home/summary. The categories query
// parameter is optional — when omitted or empty, every public category
// participates. The limit query parameter bounds the size of each
// top-N ranking (countries, ASNs, maintainers); the engine clamps it
// to a sane ceiling.
func handleHomeSummary(eng *engine.Engine, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		raw := strings.TrimSpace(r.URL.Query().Get("categories"))
		var categories []string
		if raw != "" {
			categories = parseList(raw)
		}
		limit := 0
		if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
			parsed, err := strconv.Atoi(v)
			if err != nil || parsed < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be a non-negative integer"})
				return
			}
			limit = parsed
		}
		payload, err := eng.HomeSummaryInDir(categories, limit, outputDir)
		if err != nil {
			if errors.Is(err, engine.ErrHomeAggregatesNotReady) {
				jsonError(w, http.StatusServiceUnavailable, err)
				return
			}
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}
