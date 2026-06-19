package web

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
	"github.com/firehol/update-ipsets/pkg/scheduler"
	"github.com/firehol/update-ipsets/pkg/systemd"
)

const delayedPublishStageCleanupDelay = 5 * time.Minute

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

	runner := scheduler.New(eng, opts.EnableAll, opts.Logger)
	queueStartupIntegrityRecovery(runCtx, eng, opts, runner)

	waitForBackground := startRunBackgroundWork(runCtx, eng, opts, runner, startedAt)
	defer func() {
		cancel()
		waitForBackground()
	}()

	servers := buildRunServers(runCtx, eng, opts, runner)
	if err := listenRunServers(servers); err != nil {
		return err
	}

	startRunShutdownWatcher(runCtx, servers, opts.Logger)
	startRunWatchdog(runCtx)
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
	integrityWebDir := outputDirFromOptions(eng.Runtime().BaseDir, choose(opts.WebDir, eng.Runtime().WebDir))
	findings, err := eng.CheckIntegrityWithOptionsContext(ctx, engine.IntegrityOptions{EnableAll: opts.EnableAll, WebDir: integrityWebDir})
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
}

func startRunBackgroundWork(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner, startedAt time.Time) func() {
	startupEntityArtifactsDone := make(chan struct{})
	go func() {
		defer close(startupEntityArtifactsDone)
		if err := eng.EnsureEntityArtifactsCurrentWithTrigger(ctx, "startup"); err != nil {
			opts.Logger.Error("failed to ensure country and ASN entity artifacts at startup", "error", err)
		} else {
			opts.Logger.Info("country and ASN entity artifacts checked at startup")
		}
	}()

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
		stageCleanup, err := eng.CleanupPublishStagesBefore(cutoff)
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
		<-ctx.Done()
		if err := systemd.Stopping("update-ipsets stopping"); err != nil {
			logger.Error("systemd stopping notification failed", "error", err)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, srv := range servers {
			if err := srv.server.Shutdown(shutdownCtx); err != nil && !isServerClosedError(err) {
				logger.Error("http server shutdown error", "listener", srv.name, "listen", srv.addr, "error", err)
			}
		}
	}()
}

func startRunWatchdog(ctx context.Context) {
	interval := systemd.WatchdogInterval()
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = systemd.Watchdog("update-ipsets running")
			}
		}
	}()
}

func announceRunReady(servers []namedServer, opts Options) {
	for _, srv := range servers {
		opts.Logger.Info("update-ipsets daemon listening",
			"listener", srv.name,
			"listen", srv.addr,
			"tls", opts.CertFile != "" && opts.KeyFile != "")
	}
	if err := systemd.Ready(readyMessage(servers)); err != nil {
		opts.Logger.Error("systemd ready notification failed", "error", err)
	}
}

func serveRunServers(servers []namedServer, certFile, keyFile string, cancel context.CancelFunc) error {
	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		srv := srv
		go func() {
			err := serveServer(srv, certFile, keyFile)
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
