package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"
	"github.com/firehol/update-ipsets/pkg/systemd"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

//go:embed static
var embeddedStatic embed.FS

//go:embed static/index.html
var embeddedIndex string

type Options struct {
	Listen                    string
	AdminListen               string
	AdminAuthMode             AdminAuthMode
	AllowUnauthenticatedAdmin bool
	Interval                  time.Duration
	EnableAll                 bool
	Logger                    *slog.Logger
	MetricsHandler            http.Handler
	CertFile                  string
	KeyFile                   string
	WebDir                    string
	FilesDir                  string
	TrustProxyHeaders         bool
	TrustCloudflareHeaders    bool
}

type AdminAuthMode string

const (
	AdminAuthModeRequired AdminAuthMode = "required"
	AdminAuthModeDisabled AdminAuthMode = "disabled"
)

type listenerMode int

const (
	listenerModeShared listenerMode = iota
	listenerModePublicOnly
	listenerModeAdminOnly
)

type namedServer struct {
	name     string
	addr     string
	listener net.Listener
	server   *http.Server
}

func normalizeOptions(opts Options) Options {
	if opts.Listen == "" {
		opts.Listen = ":8080"
	}
	opts.AdminListen = strings.TrimSpace(opts.AdminListen)
	if opts.Interval <= 0 {
		opts.Interval = time.Minute
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	mode := strings.ToLower(strings.TrimSpace(string(opts.AdminAuthMode)))
	switch AdminAuthMode(mode) {
	case "", AdminAuthModeRequired:
		opts.AdminAuthMode = AdminAuthModeRequired
	case AdminAuthModeDisabled:
		opts.AdminAuthMode = AdminAuthModeDisabled
	default:
		opts.AdminAuthMode = AdminAuthMode(mode)
	}
	return opts
}

func validateRunOptions(eng *engine.Engine, opts Options) error {
	switch opts.AdminAuthMode {
	case AdminAuthModeRequired, AdminAuthModeDisabled:
	default:
		return fmt.Errorf("invalid admin auth mode %q (valid: required, disabled)", opts.AdminAuthMode)
	}
	if opts.AdminAuthMode == AdminAuthModeDisabled && !opts.AllowUnauthenticatedAdmin {
		return fmt.Errorf("admin auth mode %q requires --allow-unauthenticated-admin", opts.AdminAuthMode)
	}
	if opts.AdminListen != "" {
		if strings.TrimSpace(opts.AdminListen) == strings.TrimSpace(opts.Listen) {
			return fmt.Errorf("--admin-listen must differ from --listen; omit --admin-listen to share one listener")
		}
		if strings.TrimSpace(eng.Runtime().PublicBaseURL) == "" {
			return fmt.Errorf("runtime.public_base_url must be configured when --admin-listen is used")
		}
	}
	return nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	instrumented := otelhttp.NewHandler(withTelemetryRoutePattern(handler), "http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + telemetryRouteName(r.URL.Path)
		}),
	)
	return &http.Server{
		Addr:              addr,
		Handler:           instrumented,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}
}

func withTelemetryRoutePattern(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
		r.Pattern = telemetryRouteName(r.URL.Path)
	})
}

func telemetryRouteName(path string) string {
	switch {
	case path == "":
		return "/"
	case path == "/" || path == "/healthz":
		return path
	case path == "/mcp",
		path == "/metrics",
		path == "/admin",
		path == "/api/v1/status",
		path == "/api/v1/home/globe",
		path == "/api/v1/home/summary",
		path == "/api/v1/sets",
		path == "/api/v1/ipsets",
		path == "/api/v1/categories",
		path == "/api/v1/client-ip",
		path == "/api/v1/countries",
		path == "/api/v1/asns",
		path == "/api/v1/maintainers",
		path == "/api/v1/query",
		path == "/api/v1/search",
		path == "/api/v1/compose",
		path == "/api/v1/methodology",
		path == "/api/v1/admin/status",
		path == "/api/v1/admin/feeds",
		path == "/api/v1/admin/artifacts",
		path == "/api/v1/admin/schedule",
		path == "/api/v1/admin/integrity",
		path == "/api/v1/admin/integrity/entities",
		path == "/api/v1/admin/integrity/entities/rebuild",
		path == "/api/v1/admin/integrity/reprocess",
		path == "/api/v1/admin/run":
		return path
	case strings.HasPrefix(path, "/api/v1/sets/"):
		return telemetrySetRoute("/api/v1/sets/", path)
	case strings.HasPrefix(path, "/api/v1/ipsets/"):
		return telemetrySetRoute("/api/v1/ipsets/", path)
	case strings.HasPrefix(path, "/api/v1/admin/feeds/"):
		return telemetryAdminItemRoute("/api/v1/admin/feeds/", path)
	case strings.HasPrefix(path, "/api/v1/admin/artifacts/"):
		return telemetryAdminItemRoute("/api/v1/admin/artifacts/", path)
	case strings.HasPrefix(path, "/api/v1/countries/"):
		return "/api/v1/countries/{code}"
	case strings.HasPrefix(path, "/api/v1/asns/"):
		return "/api/v1/asns/{asn}"
	case strings.HasPrefix(path, "/api/v1/maintainers/"):
		return "/api/v1/maintainers/{slug}"
	case strings.HasPrefix(path, "/api/v1/methodology/"):
		return "/api/v1/methodology/{slug}"
	case strings.HasPrefix(path, "/api/v1/"):
		return "/api/v1/*"
	case strings.HasPrefix(path, "/files/"):
		return "/files/{name}"
	case strings.HasPrefix(path, "/ipsets/"):
		return "/ipsets/{name}"
	case strings.HasPrefix(path, "/countries/"):
		return "/countries/{code}"
	case strings.HasPrefix(path, "/asns/"):
		return "/asns/{asn}"
	case strings.HasPrefix(path, "/maintainers/"):
		return "/maintainers/{slug}"
	case strings.HasPrefix(path, "/methodology/"):
		return "/methodology/{slug}"
	case strings.HasPrefix(path, "/static/"):
		return "/static/*"
	case strings.HasPrefix(path, "/world/"):
		return "/world/*"
	case strings.HasPrefix(path, "/admin/"):
		return "/admin/*"
	default:
		return "/*"
	}
}

func telemetrySetRoute(prefix, path string) string {
	parts := telemetryPathParts(strings.TrimPrefix(path, prefix))
	if len(parts) == 0 {
		return strings.TrimSuffix(prefix, "/")
	}
	base := strings.TrimSuffix(prefix, "/") + "/{name}"
	if len(parts) == 1 {
		return base
	}

	switch parts[1] {
	case "search", "data", "history", "changesets", "retention", "insights", "countries", "asn", "bogons", "infrastructure":
		if len(parts) == 2 {
			return base + "/" + parts[1]
		}
	case "compare", "comparison":
		return base + "/comparison"
	default:
		return base + "/{action}"
	}

	switch parts[1] {
	case "countries", "asn", "bogons":
		return base + "/" + parts[1] + "/{provider}"
	case "infrastructure":
		if len(parts) >= 3 && parts[2] == "providers" {
			return base + "/infrastructure/providers"
		}
		return base + "/infrastructure/{provider}"
	default:
		return base + "/{action}"
	}
}

func telemetryAdminItemRoute(prefix, path string) string {
	parts := telemetryPathParts(strings.TrimPrefix(path, prefix))
	if len(parts) == 0 {
		return strings.TrimSuffix(prefix, "/")
	}
	base := strings.TrimSuffix(prefix, "/") + "/{name}"
	if len(parts) == 1 {
		return base
	}
	switch parts[1] {
	case "disable", "enable", "manifest", "recheck", "reprocess":
		return base + "/" + parts[1]
	default:
		return base + "/{action}"
	}
}

func telemetryPathParts(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := raw[:0]
	for _, part := range raw {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func serveServer(s namedServer, certFile, keyFile string) error {
	if certFile != "" && keyFile != "" {
		return s.server.ServeTLS(s.listener, certFile, keyFile)
	}
	return s.server.Serve(s.listener)
}

func isServerClosedError(err error) bool {
	return errors.Is(err, http.ErrServerClosed)
}

func readyMessage(servers []namedServer) string {
	parts := make([]string, 0, len(servers))
	for _, srv := range servers {
		parts = append(parts, srv.name+"="+srv.addr)
	}
	return "listening on " + strings.Join(parts, ", ")
}

func Run(ctx context.Context, eng *engine.Engine, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeOptions(opts)
	if err := validateRunOptions(eng, opts); err != nil {
		return err
	}
	if err := eng.ApplyRuntimeOverrides(opts.WebDir, opts.FilesDir); err != nil {
		return err
	}
	if err := eng.CleanupStaleCriticalInfrastructureArtifacts(); err != nil {
		opts.Logger.Warn("failed to cleanup stale critical infrastructure artifacts", "error", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner := scheduler.New(eng, opts.EnableAll, opts.Logger)
	// Startup integrity check: before the scheduler's main loop
	// starts, walk every feed and verify its secondary outputs
	// (geo / ASN / bogon / comparison / insights / history CSV /
	// metadata JSON) exist, remain readable, and are still
	// consistent with the last successful local publication.
	// Feeds whose pipeline broke mid-run get queued for the
	// split integrity recovery plan (recheck and/or reprocess)
	// in the first scheduler tick, with dynamic injection
	// pulling in their derivatives automatically.
	//
	// Errors here are logged, never fatal — a transient FS
	// hiccup should not prevent the daemon from starting.
	integrityWebDir := outputDirFromOptions(eng.Runtime().BaseDir, choose(opts.WebDir, eng.Runtime().WebDir))
	if findings := eng.CheckIntegrityWithOptions(engine.IntegrityOptions{EnableAll: opts.EnableAll, WebDir: integrityWebDir}); len(findings) > 0 {
		for _, f := range findings {
			opts.Logger.Warn("integrity finding",
				"feed", f.Feed,
				"reason", f.Reason,
				"missing", len(f.MissingFiles),
				"stale", len(f.StaleFiles),
				"processed_at", f.ProcessedAt.Format(time.RFC3339),
				"source_file_mtime", f.SourceFileMTime.Format(time.RFC3339),
			)
		}
		recheckNames, reprocessNames := eng.IntegrityRecoveryPlan(findings)
		opts.Logger.Warn("integrity check queued stale feeds for recovery",
			"findings", len(findings),
			"recheck", len(recheckNames),
			"reprocess", len(reprocessNames))
		if len(recheckNames) > 0 {
			runner.TriggerSources(scheduler.PendingAction{
				Names:   recheckNames,
				Recheck: true,
				Reason:  runreason.ReasonStartupIntegrityReprocess,
			})
		}
		if len(reprocessNames) > 0 {
			runner.TriggerSources(scheduler.PendingAction{
				Names:     reprocessNames,
				Reprocess: true,
				Reason:    runreason.ReasonStartupIntegrityReprocess,
			})
		}
	} else {
		opts.Logger.Info("integrity check passed — all feeds have up-to-date and readable secondary files")
	}
	startupEntityArtifactsDone := make(chan struct{})
	go func() {
		defer close(startupEntityArtifactsDone)
		if err := eng.EnsureEntityArtifactsCurrentWithTrigger(runCtx, "startup"); err != nil {
			opts.Logger.Error("failed to ensure country and ASN entity artifacts at startup", "error", err)
		} else {
			opts.Logger.Info("country and ASN entity artifacts checked at startup")
		}
	}()
	runnerDone := make(chan struct{})
	go func() {
		defer close(runnerDone)
		runner.Run(runCtx)
	}()
	defer func() {
		cancel()
		<-startupEntityArtifactsDone
		<-runnerDone
	}()

	publicHandler := newHandlerWithContext(runCtx, eng, opts, runner)
	if opts.AdminListen != "" {
		publicHandler = newPublicHandlerWithContext(runCtx, eng, opts, runner)
	}
	servers := []namedServer{{
		name:   "public",
		addr:   opts.Listen,
		server: newHTTPServer(opts.Listen, publicHandler),
	}}
	if opts.AdminListen != "" {
		servers = append(servers, namedServer{
			name:   "admin",
			addr:   opts.AdminListen,
			server: newHTTPServer(opts.AdminListen, newAdminHandlerWithContext(runCtx, eng, opts, runner)),
		})
	}
	for i := range servers {
		listener, err := net.Listen("tcp", servers[i].addr)
		if err != nil {
			for j := 0; j < i; j++ {
				if servers[j].listener != nil {
					_ = servers[j].listener.Close()
				}
			}
			return err
		}
		servers[i].listener = listener
	}

	go func() {
		<-runCtx.Done()
		if err := systemd.Stopping("update-ipsets stopping"); err != nil {
			opts.Logger.Error("systemd stopping notification failed", "error", err)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, srv := range servers {
			if err := srv.server.Shutdown(shutdownCtx); err != nil && !isServerClosedError(err) {
				opts.Logger.Error("http server shutdown error", "listener", srv.name, "listen", srv.addr, "error", err)
			}
		}
	}()

	if interval := systemd.WatchdogInterval(); interval > 0 {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
					_ = systemd.Watchdog("update-ipsets running")
				}
			}
		}()
	}

	for _, srv := range servers {
		opts.Logger.Info("update-ipsets daemon listening",
			"listener", srv.name,
			"listen", srv.addr,
			"tls", opts.CertFile != "" && opts.KeyFile != "")
	}
	if err := systemd.Ready(readyMessage(servers)); err != nil {
		opts.Logger.Error("systemd ready notification failed", "error", err)
	}

	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		srv := srv
		go func() {
			err := serveServer(srv, opts.CertFile, opts.KeyFile)
			if isServerClosedError(err) {
				errCh <- nil
				return
			}
			if err != nil {
				cancel()
				errCh <- fmt.Errorf("%s listener %s: %w", srv.name, srv.addr, err)
				return
			}
			errCh <- nil
		}()
	}

	var firstErr error
	for range servers {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func publicRawFeedRel(eng *engine.Engine, name string) (string, bool) {
	if eng == nil || !eng.IsPublicFeedName(name) || !eng.IsRedistributable(name) || !eng.PublicRawFeedAllowed(name) {
		return "", false
	}
	snap := eng.EntrySnapshot(name)
	if snap == nil || !rawFeedFileMatches(name, snap.File) {
		return "", false
	}
	return snap.File, true
}

func rawFeedFileMatches(name, file string) bool {
	return file == name+".ipset" || file == name+".netset"
}

func sanitizeSchedulerSnapshot(snap scheduler.Snapshot) scheduler.Snapshot {
	out := scheduler.Snapshot{
		GeneratedAt: sanitizeJSONTime(snap.GeneratedAt),
		Items:       make([]scheduler.Item, 0, len(snap.Items)),
	}
	for _, item := range snap.Items {
		item.CheckedAt = sanitizeJSONTime(item.CheckedAt)
		item.NextDue = sanitizeJSONTime(item.NextDue)
		out.Items = append(out.Items, item)
	}
	return out
}

func feedScopedPublicArtifactName(eng *engine.Engine, rel string) (string, bool) {
	rel = strings.TrimSpace(strings.TrimPrefix(rel, "/"))
	if rel == "" || strings.Contains(rel, "/") {
		return "", false
	}
	switch rel {
	case "index.json", "all-ipsets.json", "sitemap.xml", "robots.txt", "llms.txt":
		return "", false
	}
	switch {
	case strings.HasSuffix(rel, "_history.csv"):
		return strings.TrimSuffix(rel, "_history.csv"), true
	case strings.HasSuffix(rel, "_changesets.csv"):
		return strings.TrimSuffix(rel, "_changesets.csv"), true
	case strings.HasSuffix(rel, "_retention.json"):
		return strings.TrimSuffix(rel, "_retention.json"), true
	case strings.HasSuffix(rel, "_comparison.json"):
		return strings.TrimSuffix(rel, "_comparison.json"), true
	case strings.HasSuffix(rel, "_insights.json"):
		return strings.TrimSuffix(rel, "_insights.json"), true
	case strings.HasSuffix(rel, ".json"):
		base := strings.TrimSuffix(rel, ".json")
		if eng != nil && eng.IsPublicFeedName(base) {
			return base, true
		}
		if name, ok := providerScopedArtifactFeedName(eng, base); ok {
			return name, true
		}
		return base, true
	case strings.HasSuffix(rel, ".html"):
		return strings.TrimSuffix(rel, ".html"), true
	default:
		return "", false
	}
}

func knownCriticalInfrastructureProvider(eng *engine.Engine, provider string) bool {
	for _, item := range eng.CriticalInfrastructureProviders() {
		if item.Name == provider {
			return true
		}
	}
	return false
}

func providerScopedArtifactFeedName(eng *engine.Engine, base string) (string, bool) {
	if eng == nil {
		return "", false
	}
	for _, provider := range eng.GeoProviders() {
		suffix := "_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	for _, provider := range eng.ASNProviders() {
		suffix := "_asn_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	for _, provider := range eng.BogonProviders() {
		suffix := "_bogons_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	if strings.HasSuffix(base, "_critical_infrastructure") {
		return strings.TrimSuffix(base, "_critical_infrastructure"), true
	}
	for _, provider := range eng.CriticalInfrastructureProviders() {
		suffix := "_critical_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	return "", false
}

func sanitizeJSONTime(ts time.Time) time.Time {
	if ts.IsZero() {
		return time.Time{}
	}
	year := ts.Year()
	if year < 0 || year > 9999 {
		return time.Time{}
	}
	return ts
}

func writeJSON(w http.ResponseWriter, status int, value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 0
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := bytes.NewBuffer(data)
	body.WriteByte('\n')
	written, _ := body.WriteTo(w)
	return int(written)
}
