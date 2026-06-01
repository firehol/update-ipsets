package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/web"
)

func runDaemon(args []string) int {
	args = compactCLIArgs(args)

	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config path")
	listen := fs.String("listen", ":8080", "listen address")
	adminListen := fs.String("admin-listen", "", "optional separate admin listen address")
	adminAuthMode := fs.String("admin-auth-mode", "required", "admin auth mode: required or disabled")
	allowUnauthenticatedAdmin := fs.Bool("allow-unauthenticated-admin", false, "acknowledge unsafe unauthenticated admin mode")
	interval := fs.Duration("interval", time.Minute, "scheduler interval")
	enableAll := fs.Bool("enable-all", false, "treat all sources as enabled")
	pushGit := fs.Bool("push-git", false, "push generated changes to git")
	silent := fs.Bool("silent", false, "errors only")
	verbose := fs.Bool("verbose", false, "verbose logging")
	tlsCert := fs.String("tls-cert", "", "TLS certificate path")
	tlsKey := fs.String("tls-key", "", "TLS private key path")
	webDir := fs.String("web-dir", "", "override web output directory")
	webFilesDir := fs.String("web-files-dir", "", "override served ipset files directory")
	trustProxyHeaders := fs.Bool("trust-proxy-headers", false, "trust X-Forwarded-For and X-Real-IP headers for client IP detection")
	trustCloudflareHeaders := fs.Bool("trust-cloudflare-headers", false, "trust CF-Connecting-IP header for client IP detection")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	logger := newLogger(*silent, *verbose)
	otelSetup, err := observability.Init(context.Background(), "update-ipsets", version, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := otelSetup.Shutdown(shutdownCtx); err != nil {
			logger.Error("opentelemetry shutdown failed", "error", err)
		}
	}()
	logger = otelSetup.Logger
	if otelSetup.Enabled {
		logger.Info("opentelemetry enabled")
	}
	eng, err := engine.New(*configPath, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *pushGit {
		eng.SetPushToGit(true)
	}
	lock, err := eng.AcquireLock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		if err := lock.Release(); err != nil {
			logger.Warn("failed to release process lock", "error", err)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hup:
				if err := eng.Reload(); err != nil {
					logger.Error("config reload failed", "error", err)
				} else {
					logger.Info("config reloaded", "config_path", eng.Runtime().ConfigPath)
					go func() {
						if err := eng.EnsureEntityArtifactsCurrentWithTrigger(ctx, "reload"); err != nil {
							logger.Error("entity artifact ensure after reload failed", "error", err)
						} else {
							logger.Info("country and ASN entity artifacts checked after reload")
						}
					}()
				}
			}
		}
	}()
	if err := web.Run(ctx, eng, web.Options{
		Listen:                    *listen,
		AdminListen:               *adminListen,
		AdminAuthMode:             web.AdminAuthMode(*adminAuthMode),
		AllowUnauthenticatedAdmin: *allowUnauthenticatedAdmin,
		Interval:                  *interval,
		EnableAll:                 *enableAll,
		Logger:                    logger,
		MetricsHandler:            otelSetup.PrometheusHandler,
		CertFile:                  *tlsCert,
		KeyFile:                   *tlsKey,
		WebDir:                    *webDir,
		FilesDir:                  *webFilesDir,
		TrustProxyHeaders:         *trustProxyHeaders || eng.Runtime().TrustProxyHeaders,
		TrustCloudflareHeaders:    *trustCloudflareHeaders || eng.Runtime().TrustCloudflareHeaders,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func compactCLIArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	compacted := args[:0]
	for _, arg := range args {
		if arg == "" {
			continue
		}
		compacted = append(compacted, arg)
	}
	return compacted
}
