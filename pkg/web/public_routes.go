package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func classifyError(err error) (int, string) {
	if err == nil {
		return http.StatusBadRequest, ""
	}
	if engine.IsServerError(err) {
		return http.StatusInternalServerError, err.Error()
	}
	return http.StatusBadRequest, err.Error()
}

func (s *surfaceRoutes) registerPublicAPI(mux *http.ServeMux) {
	s.registerPublicServingReloadListener()
	mux.HandleFunc("POST /mcp", s.handleMCP())
	mux.HandleFunc("GET /mcp", s.handleMCP())
	mux.HandleFunc("DELETE /mcp", s.handleMCP())

	mux.HandleFunc("GET /api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		writeJSON(w, http.StatusOK, buildPublicStatus(s.eng))
	})
	mux.HandleFunc("GET /api/v1/categories", s.handlePublicCategories())
	mux.HandleFunc("GET /api/v1/home/globe", s.handleHomeGlobe())
	mux.HandleFunc("GET /api/v1/home/summary", s.handleHomeSummary())

	mux.HandleFunc("GET /api/v1/sets", s.handlePublicFeedList())
	mux.HandleFunc("GET /api/v1/ipsets", s.handlePublicFeedList())
	mux.HandleFunc("GET /api/v1/sets/", s.handlePublicSet("/api/v1/sets/"))
	mux.HandleFunc("GET /api/v1/ipsets/", s.handlePublicSet("/api/v1/ipsets/"))
	mux.HandleFunc("GET /api/v1/client-ip", handleClientIP(s.resolver))
	mux.HandleFunc("GET /api/v1/countries", s.handleCountryIndex())
	mux.HandleFunc("GET /api/v1/countries/", s.handleCountryDetail())
	mux.HandleFunc("GET /api/v1/asns", s.handleASNIndex())
	mux.HandleFunc("GET /api/v1/asns/", s.handleASNDetail())
	mux.HandleFunc("GET /api/v1/maintainers", handleMaintainerIndex(s.eng))
	mux.HandleFunc("GET /api/v1/maintainers/", handleMaintainerDetail(s.eng))
	mux.HandleFunc("GET /api/v1/query", s.handleGlobalSearch())
	mux.HandleFunc("GET /api/v1/search", s.handleGlobalSearch())
	mux.HandleFunc("GET /api/v1/compose", s.handlePublicCompose())
}

func (s *surfaceRoutes) servingStateOrUnavailable(w http.ResponseWriter) (*publicServingState, bool) {
	state := s.currentPublicServingState()
	if state == nil {
		jsonError(w, http.StatusServiceUnavailable, fmt.Errorf("public serving state is not available"))
		return nil, false
	}
	return state, true
}

func (s *surfaceRoutes) handleMCP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.mcpServer == nil {
			jsonError(w, http.StatusServiceUnavailable, fmt.Errorf("MCP server is not available"))
			return
		}
		s.mcpServer.ServeHTTP(w, r)
	}
}

func (s *surfaceRoutes) handleHomeGlobe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleHomeGlobe(s.eng, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) handleHomeSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleHomeSummary(s.eng, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) handleCountryIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleCountryIndex(s.eng, state.cache, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) handleCountryDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleCountryDetail(s.eng, state.cache, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) handleASNIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleASNIndex(s.eng, state.cache, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) handleASNDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleASNDetail(s.eng, state.cache, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) registerAdminPublicMetadataAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/categories", s.handlePublicCategories())
}

func (s *surfaceRoutes) handlePublicCategories() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, state.catalog.Categories)
	}
}

func (s *surfaceRoutes) handlePublicFeedList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, state.catalog.Feeds)
	}
}

func (s *surfaceRoutes) handleGlobalSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeGlobalSearch(w, r, s.eng, s.searchLimiter, s.resolver)
	}
}

func (s *surfaceRoutes) handlePublicCompose() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		include := parseList(r.URL.Query().Get("include"))
		exclude := parseList(r.URL.Query().Get("exclude"))
		data, err := s.eng.PublicCompose(r.Context(), include, exclude, r.URL.Query().Get("format"))
		if err != nil {
			observeAPIRecalculation(r, "public", "compose", "error", 0)
			status, msg := classifyError(err)
			plainError(w, status, msg)
			return
		}
		observeAPIRecalculation(r, "public", "compose", "ok", 0)
		writePlain(w, http.StatusOK, data)
	}
}

func (s *surfaceRoutes) handlePublicSet(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		path := strings.TrimPrefix(r.URL.Path, prefix)
		name, action, _ := strings.Cut(path, "/")
		if name == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "missing set name"})
			return
		}
		if !validFeedName(name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid set name"})
			return
		}
		state, stateOK := s.servingStateOrUnavailable(w)
		if !stateOK {
			return
		}
		if !state.isPublicFeedName(name) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown set %q", name)})
			return
		}
		if action == "" {
			s.servePublicSetMetadata(w, r, name)
			return
		}
		s.servePublicSetAction(w, r, name, action)
	}
}

func (s *surfaceRoutes) servePublicSetMetadata(w http.ResponseWriter, r *http.Request, name string) {
	rel := cacheSuffixRel(name, ".json")
	state, ok := s.servingStateOrUnavailable(w)
	if !ok {
		return
	}
	if state.cache.ServeRootedFile(w, r, state.outputDir, rel, "application/json") {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf("no metadata data for %q", name))
}

func (s *surfaceRoutes) servePublicSetAction(w http.ResponseWriter, r *http.Request, name, action string) {
	switch {
	case action == "search":
		writeFeedScopedSearch(w, r, s.eng, s.searchLimiter, s.resolver, name)
	case action == "data":
		s.servePublicSetRawData(w, r, name)
	case action == "history":
		s.servePublicSetFile(w, r, name, "_history.csv", "text/csv; charset=utf-8", "no history data for %q")
	case action == "changesets":
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		changes, err := s.eng.PublicChangesetSeriesInDir(name, state.outputDir)
		if err != nil {
			jsonError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, changes)
	case action == "compare", action == "comparison":
		s.servePublicSetFile(w, r, name, "_comparison.json", "application/json", "no comparison data for %q")
	case action == "retention":
		s.servePublicSetFile(w, r, name, "_retention.json", "application/json", "no retention data for %q")
	case action == "insights":
		s.servePublicSetInsights(w, r, name)
	case action == "countries":
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, state.catalog.GeoProviders)
	case strings.HasPrefix(action, "countries/"):
		s.servePublicSetCountryProvider(w, name, strings.TrimPrefix(action, "countries/"))
	case action == "asn":
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, state.catalog.ASNProviders)
	case strings.HasPrefix(action, "asn/"):
		provider := strings.TrimPrefix(action, "asn/")
		s.servePublicSetProviderFile(w, r, name, provider, "_asn_", "no ASN data for %q with provider %q")
	case action == "bogons":
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, state.catalog.BogonProviders)
	case strings.HasPrefix(action, "bogons/"):
		provider := strings.TrimPrefix(action, "bogons/")
		s.servePublicSetProviderFile(w, r, name, provider, "_bogons_", "no bogon data for %q with provider %q")
	case action == "infrastructure":
		s.servePublicSetCriticalAggregate(w, r, name)
	case action == "infrastructure/providers":
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, state.catalog.CriticalInfrastructureProviders)
	case strings.HasPrefix(action, "infrastructure/"):
		s.servePublicSetCriticalProvider(w, r, name, strings.TrimPrefix(action, "infrastructure/"))
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown set endpoint"})
	}
}

func (s *surfaceRoutes) servePublicSetRawData(w http.ResponseWriter, r *http.Request, name string) {
	state, stateOK := s.servingStateOrUnavailable(w)
	if !stateOK {
		return
	}
	rel, ok := state.rawFeedRel(name)
	if !ok {
		plainError(w, http.StatusNotFound, fmt.Sprintf("raw feed data for %q is not available", name))
		return
	}
	if serveRawFeedRel(w, r, rel, state.ipsetsDir, state.baseDir) {
		return
	}
	plainError(w, http.StatusNotFound, fmt.Sprintf("raw feed data for %q is not available", name))
}

func (s *surfaceRoutes) servePublicSetFile(w http.ResponseWriter, r *http.Request, name, suffix, contentType, notFoundFormat string) {
	rel := cacheSuffixRel(name, suffix)
	state, ok := s.servingStateOrUnavailable(w)
	if !ok {
		return
	}
	if state.cache.ServeRootedFile(w, r, state.outputDir, rel, contentType) {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf(notFoundFormat, name))
}

func (s *surfaceRoutes) servePublicSetInsights(w http.ResponseWriter, r *http.Request, name string) {
	rel := cacheSuffixRel(name, "_insights.json")
	state, ok := s.servingStateOrUnavailable(w)
	if !ok {
		return
	}
	if !state.isPublicFeedName(name) {
		jsonError(w, http.StatusNotFound, fmt.Errorf("unknown set %q", name))
		return
	}
	if state.cache.ServeRootedFile(w, r, state.outputDir, rel, "application/json") {
		return
	}
	http.NotFound(w, r)
}

func (s *surfaceRoutes) servePublicSetCountryProvider(w http.ResponseWriter, name, provider string) {
	if !validFeedName(provider) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider name"})
		return
	}
	state, ok := s.servingStateOrUnavailable(w)
	if !ok {
		return
	}
	data, err := s.eng.CountryComparisonInDir(name, provider, state.outputDir)
	if err != nil {
		jsonError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *surfaceRoutes) servePublicSetProviderFile(w http.ResponseWriter, r *http.Request, name, provider, suffixPrefix, notFoundFormat string) {
	if !validFeedName(provider) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider name"})
		return
	}
	rel := cacheSuffixRel(name, suffixPrefix+provider+".json")
	state, ok := s.servingStateOrUnavailable(w)
	if !ok {
		return
	}
	if state.cache.ServeRootedFile(w, r, state.outputDir, rel, "application/json") {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf(notFoundFormat, name, provider))
}

func (s *surfaceRoutes) servePublicSetCriticalAggregate(w http.ResponseWriter, r *http.Request, name string) {
	state, ok := s.servingStateOrUnavailable(w)
	if !ok {
		return
	}
	if len(state.catalog.CriticalInfrastructureProviders) == 0 {
		jsonError(w, http.StatusNotFound, fmt.Errorf("no critical infrastructure providers are configured"))
		return
	}
	if !state.criticalInfrastructureTarget(name) {
		jsonError(w, http.StatusNotFound, fmt.Errorf("critical infrastructure data is not generated for %q", name))
		return
	}
	// Cache-first: serve the published artifact as-is. The engine guarantees
	// (via the single-snapshot pipeline contract) that on-disk artifacts and
	// the runtime provider_set_id marker agree within a run. Admin integrity
	// is the operator-facing tripwire if that invariant ever breaks; the
	// public path MUST NOT surface that internal concept to end users.
	rel := cacheSuffixRel(name, "_critical_infrastructure.json")
	if state.cache.ServeRootedFile(w, r, state.outputDir, rel, "application/json") {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf("no critical infrastructure data for %q", name))
}

func (s *surfaceRoutes) servePublicSetCriticalProvider(w http.ResponseWriter, r *http.Request, name, provider string) {
	if !validFeedName(provider) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider name"})
		return
	}
	state, ok := s.servingStateOrUnavailable(w)
	if !ok {
		return
	}
	if !state.criticalInfrastructureTarget(name) {
		jsonError(w, http.StatusNotFound, fmt.Errorf("critical infrastructure data is not generated for %q", name))
		return
	}
	if !state.criticalInfrastructureProvider(provider) {
		jsonError(w, http.StatusNotFound, fmt.Errorf("unknown critical infrastructure provider %q", provider))
		return
	}
	rel := cacheSuffixRel(name, "_critical_"+provider+".json")
	if state.cache.ServeRootedFile(w, r, state.outputDir, rel, "application/json") {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf("no critical infrastructure data for %q with provider %q", name, provider))
}
