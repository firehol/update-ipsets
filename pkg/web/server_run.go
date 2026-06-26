package web

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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
	startedAt := time.Now().UTC()
	if err := prepareEngineForRun(eng, opts); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	eng.AttachWorkLaneContext(runCtx, 30*time.Second)
	watchdogDone := startRunWatchdog(runCtx, opts.Logger)
	newRuntimeStatsSampler().Start(runCtx)

	runner := scheduler.New(eng, opts.EnableAll, opts.Logger)
	queueStartupIntegrityRecovery(runCtx, eng, opts, runner)

	waitForBackground := startRunBackgroundWork(runCtx, eng, opts, runner, startedAt)
	defer func() {
		cancel()
		waitForBackground()
		<-watchdogDone
		cachePersistenceCtx, cancelCachePersistence := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelCachePersistence()
		if err := eng.StopCachePersistence(cachePersistenceCtx); err != nil {
			opts.Logger.Warn("cache persistence shutdown timed out", "error", err)
		}
	}()

	servers := buildRunServers(runCtx, eng, opts, runner)
	if err := listenRunServers(servers); err != nil {
		return err
	}

	startRunShutdownWatcher(runCtx, servers, opts.Logger)
	announceRunReady(servers, opts)

	return serveRunServers(servers, opts.CertFile, opts.KeyFile, cancel)
}

func prepareEngineForRun(eng *engine.Engine, opts Options) error {
	if err := eng.ApplyRuntimeOverrides(opts.WebDir, opts.FilesDir); err != nil {
		return err
	}
	stageCleanup, err := eng.CleanupStalePublishStages()
	if err != nil {
		opts.Logger.Warn("failed to cleanup stale publish stages", "error", err)
	}
	if stageCleanup.TotalRemoved() > 0 {
		opts.Logger.Info("cleaned stale publish stages",
			"web_removed", stageCleanup.WebRemoved,
			"entity_removed", stageCleanup.EntityRemoved)
	}
	if err := eng.CleanupStaleCriticalInfrastructureArtifacts(); err != nil {
		opts.Logger.Warn("failed to cleanup stale critical infrastructure artifacts", "error", err)
	}
	return nil
}

// queueStartupIntegrityRecovery repairs split secondary artifacts from the
// first scheduler tick without making transient filesystem findings fatal.
func queueStartupIntegrityRecovery(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) {
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
		opts.Logger.Warn("startup integrity check cancelled", "error", err)
		return
	}
	if len(findings) == 0 {
		opts.Logger.Info("integrity check passed — all feeds have up-to-date and readable secondary files")
		return
	}

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
		if err := runner.TriggerSourcesWithin(ctx, scheduler.DefaultActionAdmissionTimeout, scheduler.PendingAction{
			Names:   recheckNames,
			Recheck: true,
			Reason:  runreason.ReasonStartupIntegrityReprocess,
		}); err != nil {
			opts.Logger.Error("failed to queue startup integrity recheck work", "targets", len(recheckNames), "error", err)
		}
	}
	if len(reprocessNames) > 0 {
		if err := runner.TriggerSourcesWithin(ctx, scheduler.DefaultActionAdmissionTimeout, scheduler.PendingAction{
			Names:     reprocessNames,
			Reprocess: true,
			Reason:    runreason.ReasonStartupIntegrityReprocess,
		}); err != nil {
			opts.Logger.Error("failed to queue startup integrity reprocess work", "targets", len(reprocessNames), "error", err)
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
	startupEntityArtifactsDone := startStartupEntityArtifacts(ctx, opts.Logger, func(ctx context.Context) error {
		return eng.EnsureEntityArtifactsCurrentWithTrigger(ctx, "startup")
	})

	runnerDone := make(chan struct{})
	go func() {
		defer close(runnerDone)
		runner.Run(ctx)
	}()

	delayedStageCleanupDone := startDelayedPublishStageCleanup(ctx, eng, opts, startedAt)

	return func() {
		<-startupEntityArtifactsDone
		<-runnerDone
		<-delayedStageCleanupDone
	}
}

func startStartupEntityArtifacts(ctx context.Context, logger *slog.Logger, ensure func(context.Context) error) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverDaemonControlPanic(logger, "startup_entity_artifacts")
		if ensure == nil {
			return
		}
		if err := ensure(ctx); err != nil {
			nonNilLogger(logger).Error("failed to ensure country and ASN entity artifacts at startup", "error", err)
		} else {
			nonNilLogger(logger).Info("country and ASN entity artifacts checked at startup")
		}
	}()
	return done
}

func startDelayedPublishStageCleanup(ctx context.Context, eng *engine.Engine, opts Options, cutoff time.Time) <-chan struct{} {
	done := make(chan struct{})
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
			opts.Logger.Warn("failed to cleanup pre-start publish stages", "error", err)
		}
		if stageCleanup.TotalRemoved() > 0 {
			opts.Logger.Info("cleaned pre-start publish stages",
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

func startRunShutdownWatcher(ctx context.Context, servers []namedServer, logger *slog.Logger) {
	go func() {
		defer recoverDaemonControlPanic(logger, "shutdown_watcher")
		<-ctx.Done()
		notifyRunStopping(ctx, logger)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, srv := range servers {
			if err := srv.server.Shutdown(shutdownCtx); err != nil && !isServerClosedError(err) {
				logger.Error("http server shutdown error", "listener", srv.name, "listen", srv.addr, "error", err)
			}
		}
	}()
}

func notifyRunStopping(ctx context.Context, logger *slog.Logger) {
	defer recoverDaemonControlPanic(logger, "systemd_stopping")
	if err := systemdStoppingNotify("update-ipsets stopping"); err != nil {
		reportSystemdNotifyError(ctx, logger, "stopping", err)
	}
}

func startRunWatchdog(ctx context.Context, logger *slog.Logger) <-chan struct{} {
	interval := systemd.WatchdogInterval()
	if interval <= 0 {
		done := make(chan struct{})
		close(done)
		return done
	}
	notifyDeadline := systemd.NotifyDeadline(interval)
	var lastBeat atomic.Int64
	lastBeat.Store(time.Now().UnixNano())
	_ = startWatchdogSelfHealth(ctx, logger, interval, notifyDeadline, &lastBeat)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverDaemonControlPanic(logger, "watchdog")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendRunWatchdogTick(ctx, logger, &lastBeat)
			}
		}
	}()
	return done
}

func sendRunWatchdogTick(ctx context.Context, logger *slog.Logger, lastBeat *atomic.Int64) {
	defer recoverDaemonControlPanic(logger, "watchdog")
	if err := systemdWatchdogNotify("update-ipsets running"); err != nil {
		reportSystemdNotifyError(ctx, logger, "watchdog", err)
		return
	}
	if lastBeat != nil {
		lastBeat.Store(time.Now().UnixNano())
	}
}

func announceRunReady(servers []namedServer, opts Options) {
	for _, srv := range servers {
		opts.Logger.Info("update-ipsets daemon listening",
			"listener", srv.name,
			"listen", srv.addr,
			"tls", opts.CertFile != "" && opts.KeyFile != "")
	}
	notifyRunReady(opts.Logger, readyMessage(servers))
}

func notifyRunReady(logger *slog.Logger, status string) {
	defer recoverDaemonControlPanic(logger, "systemd_ready")
	if err := systemdReadyNotify(status); err != nil {
		reportSystemdNotifyError(context.Background(), logger, "ready", err)
	}
}

func serveRunServers(servers []namedServer, certFile, keyFile string, cancel context.CancelFunc) error {
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

	var firstErr error
	for range servers {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
