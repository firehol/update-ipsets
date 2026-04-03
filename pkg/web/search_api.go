package web

import (
	"net/http"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

type ipSearchResponse struct {
	IP           string              `json:"ip"`
	Scope        string              `json:"scope,omitempty"`
	SearchedFeed string              `json:"searched_feed,omitempty"`
	Matches      []engine.QueryMatch `json:"matches"`
	Context      *engine.IPContext   `json:"context,omitempty"`
}

func writeGlobalSearch(w http.ResponseWriter, r *http.Request, eng *engine.Engine, limiter *clientRateLimiter, resolver *clientIPResolver) {
	if !allowSearch(w, r, limiter, resolver) {
		return
	}
	apiNoCache(w)
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing ip query parameter"})
		return
	}
	matches, err := eng.QueryIP(r.Context(), ip)
	if err != nil {
		status := http.StatusBadRequest
		if engine.IsServerError(err) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	if r.URL.Query().Get("details") != "true" {
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, match.Name)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ip": ip, "matches": names})
		return
	}
	resp := ipSearchResponse{
		IP:      ip,
		Scope:   "global",
		Matches: matches,
	}
	// Best-effort enrichment: when the preferred geo and/or ASN providers
	// are loadable from disk, attach country, ASN, and infrastructure-role
	// context for the looked-up IP. Errors are non-fatal — the matches
	// list is the primary payload.
	if ctx, err := eng.LookupIPContext(ip); err == nil {
		resp.Context = ctx
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeFeedScopedSearch(w http.ResponseWriter, r *http.Request, eng *engine.Engine, limiter *clientRateLimiter, resolver *clientIPResolver, name string) {
	if !allowSearch(w, r, limiter, resolver) {
		return
	}
	apiNoCache(w)
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing ip query parameter"})
		return
	}
	match, found, err := eng.QueryFeedIP(r.Context(), name, ip)
	if err != nil {
		status := http.StatusBadRequest
		if engine.IsServerError(err) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	resp := ipSearchResponse{
		IP:           ip,
		Scope:        "feed",
		SearchedFeed: name,
		Matches:      []engine.QueryMatch{},
	}
	if found && match != nil {
		resp.Matches = append(resp.Matches, *match)
	}
	if r.URL.Query().Get("details") == "true" {
		if ctx, err := eng.LookupIPContext(ip); err == nil {
			resp.Context = ctx
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func allowSearch(w http.ResponseWriter, r *http.Request, limiter *clientRateLimiter, resolver *clientIPResolver) bool {
	if limiter == nil {
		return true
	}
	if limiter.Allow(resolver.clientIP(r), time.Now()) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "IP search rate limit exceeded (10/min)"})
	return false
}
