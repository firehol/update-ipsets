package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"
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
	mux.HandleFunc("POST /mcp", s.mcpServer.ServeHTTP)
	mux.HandleFunc("GET /mcp", s.mcpServer.ServeHTTP)
	mux.HandleFunc("DELETE /mcp", s.mcpServer.ServeHTTP)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		writeJSON(w, http.StatusOK, buildPublicStatus(s.eng))
	})
	mux.HandleFunc("GET /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		writeJSON(w, http.StatusOK, s.eng.PublicCategories())
	})
	mux.HandleFunc("GET /api/v1/home/globe", handleHomeGlobe(s.eng, s.outputDir))
	mux.HandleFunc("GET /api/v1/home/summary", handleHomeSummary(s.eng, s.outputDir))

	mux.HandleFunc("GET /api/v1/sets", s.handlePublicFeedList())
	mux.HandleFunc("GET /api/v1/ipsets", s.handlePublicFeedList())
	mux.HandleFunc("GET /api/v1/sets/", s.handlePublicSet("/api/v1/sets/"))
	mux.HandleFunc("GET /api/v1/ipsets/", s.handlePublicSet("/api/v1/ipsets/"))
	mux.HandleFunc("GET /api/v1/client-ip", handleClientIP(s.resolver))
	mux.HandleFunc("GET /api/v1/countries", handleCountryIndex(s.eng, s.cache, s.outputDir))
	mux.HandleFunc("GET /api/v1/countries/", handleCountryDetail(s.eng, s.cache, s.outputDir))
	mux.HandleFunc("GET /api/v1/asns", handleASNIndex(s.eng, s.cache, s.outputDir))
	mux.HandleFunc("GET /api/v1/asns/", handleASNDetail(s.eng, s.cache, s.outputDir))
	mux.HandleFunc("GET /api/v1/maintainers", handleMaintainerIndex(s.eng))
	mux.HandleFunc("GET /api/v1/maintainers/", handleMaintainerDetail(s.eng))
	mux.HandleFunc("GET /api/v1/query", s.handleGlobalSearch())
	mux.HandleFunc("GET /api/v1/search", s.handleGlobalSearch())
	mux.HandleFunc("GET /api/v1/compose", s.handlePublicCompose())
}

func (s *surfaceRoutes) handlePublicFeedList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		writeJSON(w, http.StatusOK, s.eng.PublicFeedSummaries())
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
		if !s.eng.IsPublicFeedName(name) {
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
	if s.cache.ServeRootedFile(w, r, s.outputDir, rel, "application/json") {
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
		changes, err := s.eng.PublicChangesetSeriesInDir(name, s.outputDir)
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
		writeJSON(w, http.StatusOK, s.eng.GeoProviders())
	case strings.HasPrefix(action, "countries/"):
		s.servePublicSetCountryProvider(w, name, strings.TrimPrefix(action, "countries/"))
	case action == "asn":
		writeJSON(w, http.StatusOK, s.eng.ASNProviders())
	case strings.HasPrefix(action, "asn/"):
		provider := strings.TrimPrefix(action, "asn/")
		s.servePublicSetProviderFile(w, r, name, provider, "_asn_", "no ASN data for %q with provider %q")
	case action == "bogons":
		writeJSON(w, http.StatusOK, s.eng.BogonProviders())
	case strings.HasPrefix(action, "bogons/"):
		provider := strings.TrimPrefix(action, "bogons/")
		s.servePublicSetProviderFile(w, r, name, provider, "_bogons_", "no bogon data for %q with provider %q")
	case action == "infrastructure":
		s.servePublicSetCriticalAggregate(w, r, name)
	case action == "infrastructure/providers":
		writeJSON(w, http.StatusOK, s.eng.CriticalInfrastructureProviders())
	case strings.HasPrefix(action, "infrastructure/"):
		s.servePublicSetCriticalProvider(w, r, name, strings.TrimPrefix(action, "infrastructure/"))
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown set endpoint"})
	}
}

func (s *surfaceRoutes) servePublicSetRawData(w http.ResponseWriter, r *http.Request, name string) {
	rel, ok := publicRawFeedRel(s.eng, name)
	if !ok {
		plainError(w, http.StatusNotFound, fmt.Sprintf("raw feed data for %q is not available", name))
		return
	}
	if serveRawFeedRel(w, r, rel, s.ipsetsDir, s.baseDir) {
		return
	}
	plainError(w, http.StatusNotFound, fmt.Sprintf("raw feed data for %q is not available", name))
}

func (s *surfaceRoutes) servePublicSetFile(w http.ResponseWriter, r *http.Request, name, suffix, contentType, notFoundFormat string) {
	rel := cacheSuffixRel(name, suffix)
	if s.cache.ServeRootedFile(w, r, s.outputDir, rel, contentType) {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf(notFoundFormat, name))
}

func (s *surfaceRoutes) servePublicSetInsights(w http.ResponseWriter, r *http.Request, name string) {
	if _, err := s.eng.Entry(name); err != nil {
		jsonError(w, http.StatusNotFound, err)
		return
	}
	rel := cacheSuffixRel(name, "_insights.json")
	if s.cache.ServeRootedFile(w, r, s.outputDir, rel, "application/json") {
		return
	}
	http.NotFound(w, r)
}

func (s *surfaceRoutes) servePublicSetCountryProvider(w http.ResponseWriter, name, provider string) {
	if !validFeedName(provider) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider name"})
		return
	}
	data, err := s.eng.CountryComparisonInDir(name, provider, s.outputDir)
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
	if s.cache.ServeRootedFile(w, r, s.outputDir, rel, "application/json") {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf(notFoundFormat, name, provider))
}

func (s *surfaceRoutes) servePublicSetCriticalAggregate(w http.ResponseWriter, r *http.Request, name string) {
	if len(s.eng.CriticalInfrastructureProviders()) == 0 {
		jsonError(w, http.StatusNotFound, fmt.Errorf("no critical infrastructure providers are configured"))
		return
	}
	if !s.eng.IsCriticalInfrastructureTarget(name) {
		jsonError(w, http.StatusNotFound, fmt.Errorf("critical infrastructure data is not generated for %q", name))
		return
	}
	// Cache-first: serve the published artifact as-is. The engine guarantees
	// (via the single-snapshot pipeline contract) that on-disk artifacts and
	// the runtime provider_set_id marker agree within a run. Admin integrity
	// is the operator-facing tripwire if that invariant ever breaks; the
	// public path MUST NOT surface that internal concept to end users.
	rel := cacheSuffixRel(name, "_critical_infrastructure.json")
	if s.cache.ServeRootedFile(w, r, s.outputDir, rel, "application/json") {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf("no critical infrastructure data for %q", name))
}

func (s *surfaceRoutes) servePublicSetCriticalProvider(w http.ResponseWriter, r *http.Request, name, provider string) {
	if !validFeedName(provider) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider name"})
		return
	}
	if !s.eng.IsCriticalInfrastructureTarget(name) {
		jsonError(w, http.StatusNotFound, fmt.Errorf("critical infrastructure data is not generated for %q", name))
		return
	}
	if !knownCriticalInfrastructureProvider(s.eng, provider) {
		jsonError(w, http.StatusNotFound, fmt.Errorf("unknown critical infrastructure provider %q", provider))
		return
	}
	rel := cacheSuffixRel(name, "_critical_"+provider+".json")
	if s.cache.ServeRootedFile(w, r, s.outputDir, rel, "application/json") {
		return
	}
	jsonError(w, http.StatusNotFound, fmt.Errorf("no critical infrastructure data for %q with provider %q", name, provider))
}

func (s *surfaceRoutes) registerAdmin(mux *http.ServeMux) {
	adminArtifactsRouter := wrapAdminAuth(s.opts, handleAdminArtifactsRouter(s.eng, s.runner))
	adminFeedsRouter := wrapAdminAuth(s.opts, handleAdminFeedsRouter(s.eng, s.runner, s.opts))

	if s.opts.MetricsHandler != nil {
		mux.Handle("GET /metrics", s.opts.MetricsHandler)
	}
	mux.HandleFunc("GET /admin", wrapAdminAuth(s.opts, handleAdminPage))
	mux.HandleFunc("GET /admin/", wrapAdminAuth(s.opts, handleAdminPage))
	mux.HandleFunc("GET /api/v1/admin/status", wrapAdminAuth(s.opts, handleAdminStatus(s.eng, s.runner)))
	mux.HandleFunc("GET /api/v1/admin/artifacts", wrapAdminAuth(s.opts, handleAdminArtifacts(s.eng, s.runner)))
	mux.HandleFunc("GET /api/v1/admin/artifacts/", adminArtifactsRouter)
	mux.HandleFunc("POST /api/v1/admin/artifacts/", adminArtifactsRouter)
	mux.HandleFunc("GET /api/v1/admin/feeds", wrapAdminAuth(s.opts, handleAdminFeeds(s.eng, s.runner)))
	mux.HandleFunc("GET /api/v1/admin/feeds/", adminFeedsRouter)
	mux.HandleFunc("POST /api/v1/admin/feeds/", adminFeedsRouter)
	mux.HandleFunc("GET /api/v1/admin/schedule", wrapAdminAuth(s.opts, handleAdminSchedule(s.eng, s.runner)))
	mux.HandleFunc("GET /api/v1/admin/integrity", wrapAdminAuth(s.opts, handleAdminIntegrity(s.eng, s.opts.EnableAll, s.outputDir)))
	mux.HandleFunc("GET /api/v1/admin/integrity/entities", wrapAdminAuth(s.opts, handleAdminEntityIntegrity(s.eng)))
	mux.HandleFunc("GET /api/v1/admin/integrity/entities/rebuild", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/integrity/entities/rebuild", wrapAdminAuth(s.opts, handleAdminEntityIntegrityRebuildWithContext(s.baseCtx, s.eng)))
	mux.HandleFunc("GET /api/v1/admin/integrity/reprocess", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/integrity/reprocess", wrapAdminAuth(s.opts, handleAdminIntegrityReprocess(s.eng, s.runner, s.outputDir)))
	mux.HandleFunc("GET /api/v1/admin/run", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/run", wrapAdminAuth(s.opts, s.handleAdminRun()))
	mux.HandleFunc("POST /api/v1/admin/run/", notFoundHandler)
	mux.HandleFunc("POST /api/v1/admin/enable/", notFoundHandler)
	mux.HandleFunc("POST /api/v1/admin/disable/", notFoundHandler)
}

func methodNotAllowedHandler(allowed string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allowed)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use " + allowed})
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (s *surfaceRoutes) handleAdminRun() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/run" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		recheck := r.URL.Query().Get("recheck") == "true"
		reprocess := r.URL.Query().Get("reprocess") == "true"
		if recheck {
			observeAPIRecalculation(r, "admin", "run_recheck", "rejected", 0)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "global recheck is not supported; use feed-level recheck or run due work now"})
			return
		}
		if recheck || reprocess {
			reason := runreason.ReasonManualRecheck
			if reprocess {
				reason = runreason.ReasonManualReprocess
			}
			s.runner.TriggerSources(scheduler.PendingAction{
				Recheck:   recheck,
				Reprocess: reprocess,
				Reason:    reason,
			})
			observeAPIRecalculation(r, "admin", "run_reprocess", "scheduled", 0)
			writeJSON(w, http.StatusAccepted, map[string]string{
				"status":    "scheduled",
				"recheck":   fmt.Sprintf("%t", recheck),
				"reprocess": fmt.Sprintf("%t", reprocess),
			})
			return
		}
		if s.runner.TriggerQueuedAction(scheduler.PendingAction{
			RunDue: true,
			Reason: runreason.ReasonManualRun,
		}) {
			observeAPIRecalculation(r, "admin", "run_due", "scheduled", 0)
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "scheduled"})
			return
		}
		observeAPIRecalculation(r, "admin", "run_due", "conflict", 0)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "run already queued"})
	}
}

func registerPublicAdminBlock(mux *http.ServeMux) {
	notFound := func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}
	mux.HandleFunc("GET /metrics", notFound)
	mux.HandleFunc("GET /admin", notFound)
	mux.HandleFunc("GET /admin/", notFound)
}

func (s *surfaceRoutes) registerEmbeddedAssets(mux *http.ServeMux) {
	staticFS, _ := fs.Sub(embeddedStatic, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.Handle("GET /world/", http.StripPrefix("/", http.FileServer(http.FS(staticFS))))
}

func (s *surfaceRoutes) registerPublicArtifactsAndSPA(mux *http.ServeMux) {
	mux.HandleFunc("GET /files/", s.handleRawFeedFile())
	mux.HandleFunc("GET /api/v1/methodology", handleMethodologyIndex)
	mux.HandleFunc("GET /api/v1/methodology/", handleMethodologyPage)

	for _, path := range []string{"/all-ipsets.json", "/sitemap.xml", "/robots.txt", "/llms.txt"} {
		requestPath := path
		mux.HandleFunc(http.MethodGet+" "+requestPath, func(w http.ResponseWriter, r *http.Request) {
			if s.cache.ServeRootedFile(w, r, s.outputDir, strings.TrimPrefix(requestPath, "/"), "") {
				return
			}
			http.NotFound(w, r)
		})
	}

	mux.HandleFunc("GET /", s.handleSPAFallback())
	mux.HandleFunc("GET /ipsets/", func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedIndex(w, embeddedIndex)
	})
	mux.HandleFunc("GET /methodology", func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedIndex(w, embeddedIndex)
	})
	mux.HandleFunc("GET /methodology/", func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedIndex(w, embeddedIndex)
	})
}

func (s *surfaceRoutes) handleRawFeedFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/files/")
		if rel == "" || strings.Contains(rel, "/") {
			http.NotFound(w, r)
			return
		}
		if !strings.HasSuffix(rel, ".ipset") && !strings.HasSuffix(rel, ".netset") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimSuffix(strings.TrimSuffix(rel, ".ipset"), ".netset")
		expected, ok := publicRawFeedRel(s.eng, name)
		if !ok || expected != rel {
			http.NotFound(w, r)
			return
		}
		if serveRawFeedRel(w, r, rel, s.ipsetsDir, s.baseDir) {
			return
		}
		http.NotFound(w, r)
	}
}

func (s *surfaceRoutes) handleSPAFallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			http.NotFound(w, r)
			return
		}
		if ipset := r.URL.Query().Get("ipset"); ipset != "" && r.URL.Path == "/" {
			target := "/ipsets/" + url.PathEscape(ipset)
			if r.URL.Fragment != "" {
				target += "#" + url.QueryEscape(r.URL.Fragment)
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		if r.URL.Path == "/" {
			serveEmbeddedIndex(w, embeddedIndex)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasSuffix(trimmed, ".json") || strings.HasSuffix(trimmed, ".csv") || strings.HasSuffix(trimmed, ".xml") || strings.HasSuffix(trimmed, ".txt") || strings.HasSuffix(trimmed, ".html") || strings.HasSuffix(trimmed, ".md") {
			s.serveDirectPublishedArtifact(w, r, trimmed)
			return
		}
		if strings.HasSuffix(trimmed, ".ipset") || strings.HasSuffix(trimmed, ".netset") {
			s.serveDirectRawFeed(w, r, trimmed)
			return
		}
		serveEmbeddedIndex(w, embeddedIndex)
	}
}

func (s *surfaceRoutes) serveDirectPublishedArtifact(w http.ResponseWriter, r *http.Request, rel string) {
	if hasHiddenPathSegment(rel) {
		http.NotFound(w, r)
		return
	}
	if feedName, ok := feedScopedPublicArtifactName(s.eng, rel); ok && !s.eng.IsPublicFeedName(feedName) {
		http.NotFound(w, r)
		return
	}
	if _, ok := safePath(s.outputDir, rel); !ok {
		http.NotFound(w, r)
		return
	}
	if s.cache.ServeRootedFile(w, r, s.outputDir, rel, "") {
		return
	}
	http.NotFound(w, r)
}

func hasHiddenPathSegment(rel string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func (s *surfaceRoutes) serveDirectRawFeed(w http.ResponseWriter, r *http.Request, rel string) {
	name := strings.TrimSuffix(strings.TrimSuffix(rel, ".ipset"), ".netset")
	expected, ok := publicRawFeedRel(s.eng, name)
	if !ok || expected != rel {
		http.NotFound(w, r)
		return
	}
	if serveRawFeedRel(w, r, rel, s.ipsetsDir, s.baseDir) {
		return
	}
	http.NotFound(w, r)
}
