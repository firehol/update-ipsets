package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"
	"github.com/firehol/update-ipsets/pkg/systemd"
)

const delayedPublishStageCleanupDelay = 5 * time.Minute

var serveRunServer = serveServer

var startupIntegrityRecoveryHookMu sync.Mutex
var startupIntegrityRecoveryBeforeCheckHook func()

func Run(ctx context.Context, eng *engine.Engine, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeOptions(opts)
	if err := validateRunOptions(eng, opts); err != nil {
		return err
	}
	controlLogger := plainLivenessLogger()
	startedAt := time.Now().UTC()
	if err := prepareEngineForRun(eng, opts); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	eng.AttachWorkLaneContext(runCtx, 30*time.Second)
	watchdogDone := closedRunLifecycleDone()
	newRuntimeStatsSampler().Start(runCtx)

	runner := scheduler.New(eng, opts.EnableAll, opts.Logger)
	waitForBackground := func() {}
	defer func() {
		cancel()
		waitForBackground()
		<-watchdogDone
		cachePersistenceCtx, cancelCachePersistence := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelCachePersistence()
		if err := eng.StopCachePersistence(cachePersistenceCtx); err != nil {
			controlLogger.Warn("cache persistence shutdown timed out", "error", err)
		}
	}()

	servers := buildRunServers(runCtx, eng, opts, runner)
	if err := listenRunServers(servers); err != nil {
		return err
	}

	startRunShutdownWatcher(runCtx, servers)

	return serveRunServers(servers, opts.CertFile, opts.KeyFile, cancel, func() {
		watchdogDone = startRunWatchdog(runCtx, webServingWatchdogProbe(servers, opts.CertFile != "" && opts.KeyFile != ""))
		announceRunReady(servers, opts)
		waitForBackground = startRunBackgroundWork(runCtx, eng, opts, runner, startedAt)
	})
}

func closedRunLifecycleDone() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func prepareEngineForRun(eng *engine.Engine, opts Options) error {
	return eng.ApplyRuntimeOverrides(opts.WebDir, opts.FilesDir)
}

// queueStartupIntegrityRecovery repairs split secondary artifacts from the
// first scheduler tick without making transient filesystem findings fatal.
func queueStartupIntegrityRecovery(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) {
	logger := plainLivenessLogger()
	_, rt := eng.ConfigRuntimeSnapshot()
	integrityWebDir := outputDirFromOptions(rt.BaseDir, choose(opts.WebDir, rt.WebDir))
	integrityOpts := engine.IntegrityOptions{EnableAll: opts.EnableAll, WebDir: integrityWebDir}
	eng.MarkPipelineIntegrityStartupScanRunning(integrityOpts)
	if hook := startupIntegrityRecoveryBeforeCheckHookForTest(); hook != nil {
		hook()
	}
	findings, err := eng.CheckIntegrityWithOptionsContext(ctx, integrityOpts)
	eng.StorePipelineIntegrityFindings(integrityOpts, findings, err)
	if err != nil {
		logger.Warn("startup integrity check cancelled", "error", err)
		return
	}
	if len(findings) == 0 {
		logger.Info("integrity check passed — all feeds have up-to-date and readable secondary files")
		return
	}

	for _, f := range findings {
		logger.Warn("integrity finding",
			"feed", f.Feed,
			"reason", f.Reason,
			"missing", len(f.MissingFiles),
			"stale", len(f.StaleFiles),
			"processed_at", f.ProcessedAt.Format(time.RFC3339),
			"source_file_mtime", f.SourceFileMTime.Format(time.RFC3339),
		)
	}
	recheckNames, reprocessNames := eng.IntegrityRecoveryPlan(findings)
	logger.Warn("integrity check queued stale feeds for recovery",
		"findings", len(findings),
		"recheck", len(recheckNames),
		"reprocess", len(reprocessNames))
	if len(recheckNames) > 0 {
		if err := runner.TriggerSourcesWithin(ctx, scheduler.DefaultActionAdmissionTimeout, scheduler.PendingAction{
			Names:   recheckNames,
			Recheck: true,
			Reason:  runreason.ReasonStartupIntegrityReprocess,
		}); err != nil {
			logger.Error("failed to queue startup integrity recheck work", "targets", len(recheckNames), "error", err)
		}
	}
	if len(reprocessNames) > 0 {
		if err := runner.TriggerSourcesWithin(ctx, scheduler.DefaultActionAdmissionTimeout, scheduler.PendingAction{
			Names:     reprocessNames,
			Reprocess: true,
			Reason:    runreason.ReasonStartupIntegrityReprocess,
		}); err != nil {
			logger.Error("failed to queue startup integrity reprocess work", "targets", len(reprocessNames), "error", err)
		}
	}
}

func startupIntegrityRecoveryBeforeCheckHookForTest() func() {
	startupIntegrityRecoveryHookMu.Lock()
	defer startupIntegrityRecoveryHookMu.Unlock()
	return startupIntegrityRecoveryBeforeCheckHook
}

func setStartupIntegrityRecoveryBeforeCheckHookForTest(fn func()) func() {
	startupIntegrityRecoveryHookMu.Lock()
	old := startupIntegrityRecoveryBeforeCheckHook
	startupIntegrityRecoveryBeforeCheckHook = fn
	startupIntegrityRecoveryHookMu.Unlock()
	return func() {
		startupIntegrityRecoveryHookMu.Lock()
		startupIntegrityRecoveryBeforeCheckHook = old
		startupIntegrityRecoveryHookMu.Unlock()
	}
}

func startRunBackgroundWork(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner, startedAt time.Time) func() {
	startupIntegrityRecoveryDone := startStartupIntegrityRecovery(ctx, eng, opts, runner)
	startupEntityArtifactsDone := startStartupEntityArtifacts(ctx, func(ctx context.Context) error {
		return eng.EnsureEntityArtifactsCurrentWithTrigger(ctx, "startup")
	})
	startupCriticalInfrastructureCleanupDone := startStartupCriticalInfrastructureCleanup(ctx, eng)

	runnerDone := make(chan struct{})
	go func() {
		defer close(runnerDone)
		runner.Run(ctx)
	}()

	delayedStageCleanupDone := startDelayedPublishStageCleanup(ctx, eng, startedAt)

	return func() {
		<-startupIntegrityRecoveryDone
		<-startupEntityArtifactsDone
		<-startupCriticalInfrastructureCleanupDone
		<-runnerDone
		<-delayedStageCleanupDone
	}
}

func startStartupIntegrityRecovery(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverDaemonControlPanic("startup_integrity_recovery")
		queueStartupIntegrityRecovery(ctx, eng, opts, runner)
	}()
	return done
}

func startStartupEntityArtifacts(ctx context.Context, ensure func(context.Context) error) <-chan struct{} {
	done := make(chan struct{})
	logger := plainLivenessLogger()
	go func() {
		defer close(done)
		defer recoverDaemonControlPanic("startup_entity_artifacts")
		if ensure == nil {
			return
		}
		if err := ensure(ctx); err != nil {
			logger.Error("failed to ensure country and ASN entity artifacts at startup", "error", err)
		} else {
			logger.Info("country and ASN entity artifacts checked at startup")
		}
	}()
	return done
}

func startStartupCriticalInfrastructureCleanup(ctx context.Context, eng *engine.Engine) <-chan struct{} {
	done := make(chan struct{})
	logger := plainLivenessLogger()
	go func() {
		defer close(done)
		defer recoverDaemonControlPanic("startup_critical_infrastructure_cleanup")
		if err := eng.CleanupStaleCriticalInfrastructureArtifactsWithTrigger(ctx, "startup"); err != nil {
			logger.Warn("failed to queue startup critical infrastructure cleanup", "error", err)
		}
	}()
	return done
}

func startDelayedPublishStageCleanup(ctx context.Context, eng *engine.Engine, cutoff time.Time) <-chan struct{} {
	done := make(chan struct{})
	logger := plainLivenessLogger()
	go func() {
		defer close(done)
		timer := time.NewTimer(delayedPublishStageCleanupDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		stageCleanup, err := eng.CleanupPublishStagesBeforeWithTrigger(ctx, cutoff, "delayed_startup_cleanup")
		if err != nil {
			logger.Warn("failed to cleanup pre-start publish stages", "error", err)
		}
		if stageCleanup.TotalRemoved() > 0 {
			logger.Info("cleaned pre-start publish stages",
				"web_removed", stageCleanup.WebRemoved,
				"entity_removed", stageCleanup.EntityRemoved)
		}
	}()
	return done
}

func buildRunServers(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) []namedServer {
	publicHandler := newHandlerWithContext(ctx, eng, opts, runner)
	if opts.AdminListen != "" {
		publicHandler = newPublicHandlerWithContext(ctx, eng, opts, runner)
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
			server: newHTTPServer(opts.AdminListen, newAdminHandlerWithContext(ctx, eng, opts, runner)),
		})
	}
	return servers
}

func listenRunServers(servers []namedServer) error {
	for i := range servers {
		listener, err := net.Listen("tcp", servers[i].addr)
		if err != nil {
			closeRunListeners(servers[:i])
			return err
		}
		servers[i].listener = listener
	}
	return nil
}

func closeRunListeners(servers []namedServer) {
	for _, srv := range servers {
		if srv.listener != nil {
			_ = srv.listener.Close()
		}
	}
}

func startRunShutdownWatcher(ctx context.Context, servers []namedServer) {
	logger := plainLivenessLogger()
	go func() {
		defer recoverDaemonControlPanic("shutdown_watcher")
		<-ctx.Done()
		notifyRunStopping()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, srv := range servers {
			if err := srv.server.Shutdown(shutdownCtx); err != nil && !isServerClosedError(err) {
				logger.Error("http server shutdown error", "listener", srv.name, "listen", srv.addr, "error", err)
			}
		}
	}()
}

func notifyRunStopping() {
	defer recoverDaemonControlPanic("systemd_stopping")
	if err := systemdStoppingNotify("update-ipsets stopping"); err != nil {
		reportSystemdNotifyError("stopping", err)
	}
}

type watchdogProbe func(context.Context) error

func startRunWatchdog(ctx context.Context, probes ...watchdogProbe) <-chan struct{} {
	interval := systemd.WatchdogInterval()
	if interval <= 0 {
		done := make(chan struct{})
		close(done)
		return done
	}
	probe := firstWatchdogProbe(probes...)
	notifyDeadline := systemd.NotifyDeadline(interval)
	var lastBeat atomic.Int64
	lastBeat.Store(time.Now().UnixNano())
	_ = startWatchdogSelfHealth(ctx, interval, notifyDeadline, &lastBeat)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverDaemonControlPanic("watchdog")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendRunWatchdogTick(ctx, &lastBeat, probe)
			}
		}
	}()
	return done
}

func firstWatchdogProbe(probes ...watchdogProbe) watchdogProbe {
	for _, probe := range probes {
		if probe != nil {
			return probe
		}
	}
	return func(context.Context) error { return nil }
}

func sendRunWatchdogTick(ctx context.Context, lastBeat *atomic.Int64, probes ...watchdogProbe) {
	logger := plainLivenessLogger()
	defer recoverDaemonControlPanic("watchdog")
	probe := firstWatchdogProbe(probes...)
	if err := probe(ctx); err != nil {
		logger.Warn("watchdog web-serving probe failed", "error", err)
		return
	}
	if err := systemdWatchdogNotify("update-ipsets running"); err != nil {
		reportSystemdNotifyError("watchdog", err)
		return
	}
	if lastBeat != nil {
		lastBeat.Store(time.Now().UnixNano())
	}
}

func webServingWatchdogProbe(servers []namedServer, tlsEnabled bool) watchdogProbe {
	if len(servers) == 0 {
		return func(context.Context) error { return errors.New("no HTTP listeners configured") }
	}
	client := &http.Client{
		Transport: watchdogProbeTransport(tlsEnabled),
	}
	return func(ctx context.Context) error {
		timeout := watchdogProbeTimeout(systemd.WatchdogInterval())
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		for _, srv := range servers {
			if err := probeRunServerHealth(probeCtx, client, srv, tlsEnabled); err != nil {
				return err
			}
		}
		return nil
	}
}

func watchdogProbeTransport(tlsEnabled bool) http.RoundTripper {
	if !tlsEnabled {
		return http.DefaultTransport
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local self-probe of the configured listener
	}
}

func watchdogProbeTimeout(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 2 * time.Second
	}
	timeout := systemd.NotifyDeadline(interval)
	if timeout < 50*time.Millisecond {
		return 50 * time.Millisecond
	}
	return timeout
}

func probeRunServerHealth(ctx context.Context, client *http.Client, srv namedServer, tlsEnabled bool) error {
	if srv.listener == nil {
		return fmt.Errorf("%s listener is not ready", srv.name)
	}
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+watchdogProbeHostPort(srv.listener.Addr())+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("%s listener health request: %w", srv.name, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s listener health request: %w", srv.name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s listener health status %d", srv.name, resp.StatusCode)
	}
	return nil
}

func watchdogProbeHostPort(addr net.Addr) string {
	if addr == nil {
		return "127.0.0.1"
	}
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		host = "127.0.0.1"
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func announceRunReady(servers []namedServer, opts Options) {
	logger := plainLivenessLogger()
	for _, srv := range servers {
		logger.Info("update-ipsets daemon listening",
			"listener", srv.name,
			"listen", srv.addr,
			"tls", opts.CertFile != "" && opts.KeyFile != "")
	}
	notifyRunReady(readyMessage(servers))
}

func notifyRunReady(status string) {
	defer recoverDaemonControlPanic("systemd_ready")
	if err := systemdReadyNotify(status); err != nil {
		reportSystemdNotifyError("ready", err)
	}
}

func serveRunServers(servers []namedServer, certFile, keyFile string, cancel context.CancelFunc, afterStart ...func()) error {
	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		srv := srv
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					cancel()
					errCh <- fmt.Errorf("%s listener %s panicked: %v", srv.name, srv.addr, recovered)
				}
			}()
			err := serveRunServer(srv, certFile, keyFile)
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
	for _, fn := range afterStart {
		if fn != nil {
			fn()
		}
	}

	var firstErr error
	for range servers {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
