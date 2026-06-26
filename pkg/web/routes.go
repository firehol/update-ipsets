package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

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
	mux.HandleFunc("GET /api/v1/admin/integrity", wrapAdminAuth(s.opts, s.handleAdminIntegrity()))
	mux.HandleFunc("GET /api/v1/admin/integrity/refresh", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/integrity/refresh", wrapAdminAuth(s.opts, s.handleAdminIntegrityRefresh()))
	mux.HandleFunc("GET /api/v1/admin/integrity/entities", wrapAdminAuth(s.opts, handleAdminEntityIntegrity(s.baseCtx, s.eng)))
	mux.HandleFunc("GET /api/v1/admin/integrity/entities/refresh", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/integrity/entities/refresh", wrapAdminAuth(s.opts, handleAdminEntityIntegrityRefresh(s.baseCtx, s.eng)))
	mux.HandleFunc("GET /api/v1/admin/integrity/entities/rebuild", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/integrity/entities/rebuild", wrapAdminAuth(s.opts, handleAdminEntityIntegrityRebuildWithContext(s.baseCtx, s.eng)))
	mux.HandleFunc("GET /api/v1/admin/integrity/reprocess", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/integrity/reprocess", wrapAdminAuth(s.opts, s.handleAdminIntegrityReprocess()))
	mux.HandleFunc("GET /api/v1/admin/run", methodNotAllowedHandler(http.MethodPost))
	mux.HandleFunc("POST /api/v1/admin/run", wrapAdminAuth(s.opts, s.handleAdminRun()))
	mux.HandleFunc("POST /api/v1/admin/run/", notFoundHandler)
	mux.HandleFunc("POST /api/v1/admin/enable/", notFoundHandler)
	mux.HandleFunc("POST /api/v1/admin/disable/", notFoundHandler)
}

func (s *surfaceRoutes) handleAdminIntegrity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleAdminIntegrity(s.baseCtx, s.eng, s.opts.EnableAll, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) handleAdminIntegrityRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleAdminIntegrityRefresh(s.baseCtx, s.eng, s.opts.EnableAll, state.outputDir)(w, r)
	}
}

func (s *surfaceRoutes) handleAdminIntegrityReprocess() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := s.servingStateOrUnavailable(w)
		if !ok {
			return
		}
		handleAdminIntegrityReprocess(s.baseCtx, s.eng, s.runner, state.outputDir)(w, r)
	}
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
			if !s.runner.TryTriggerSources(scheduler.PendingAction{
				Recheck:   recheck,
				Reprocess: reprocess,
				Reason:    reason,
			}) {
				observeAPIRecalculation(r, "admin", "run_reprocess", "conflict", 0)
				writeJSON(w, http.StatusConflict, map[string]string{"error": "scheduler action queue is full"})
				return
			}
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
			state, ok := s.servingStateOrUnavailable(w)
			if !ok {
				return
			}
			if state.cache.ServeRootedFile(w, r, state.outputDir, strings.TrimPrefix(requestPath, "/"), "") {
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
		state, stateOK := s.servingStateOrUnavailable(w)
		if !stateOK {
			return
		}
		if serveRawFeedRel(w, r, rel, state.ipsetsDir, state.baseDir) {
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
			if validFeedName(ipset) {
				target := "/ipsets/" + url.PathEscape(ipset)
				// validFeedName rejects URL/path separators; target is a local path.
				http.Redirect(w, r, target, http.StatusMovedPermanently) // nosemgrep
				return
			}
			http.NotFound(w, r)
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
	state, stateOK := s.servingStateOrUnavailable(w)
	if !stateOK {
		return
	}
	if _, ok := safePath(state.outputDir, rel); !ok {
		http.NotFound(w, r)
		return
	}
	if state.cache.ServeRootedFile(w, r, state.outputDir, rel, "") {
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
	state, stateOK := s.servingStateOrUnavailable(w)
	if !stateOK {
		return
	}
	if serveRawFeedRel(w, r, rel, state.ipsetsDir, state.baseDir) {
		return
	}
	http.NotFound(w, r)
}
