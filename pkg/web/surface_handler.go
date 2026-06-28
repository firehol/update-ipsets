package web

import (
	"context"
	"net/http"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func newHandler(eng *engine.Engine, opts Options, runner *scheduler.Runner) http.Handler {
	return newHandlerWithContext(context.Background(), eng, opts, runner)
}

func newHandlerWithContext(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) http.Handler {
	return newSurfaceHandlerWithContext(ctx, eng, opts, runner, listenerModeShared)
}

func newPublicHandler(eng *engine.Engine, opts Options, runner *scheduler.Runner) http.Handler {
	return newPublicHandlerWithContext(context.Background(), eng, opts, runner)
}

func newPublicHandlerWithContext(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) http.Handler {
	return newSurfaceHandlerWithContext(ctx, eng, opts, runner, listenerModePublicOnly)
}

func newAdminHandler(eng *engine.Engine, opts Options, runner *scheduler.Runner) http.Handler {
	return newAdminHandlerWithContext(context.Background(), eng, opts, runner)
}

func newAdminHandlerWithContext(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) http.Handler {
	return newSurfaceHandlerWithContext(ctx, eng, opts, runner, listenerModeAdminOnly)
}

func newSurfaceHandlerWithContext(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner, mode listenerMode) http.Handler {
	mux := http.NewServeMux()
	registerHealthz(mux)
	servePublic := mode != listenerModeAdminOnly
	serveAdmin := mode != listenerModePublicOnly
	routes := newSurfaceRoutesWithContext(ctx, eng, opts, runner)
	if mode == listenerModeAdminOnly {
		routes.registerAdminServingReloadListener()
	}

	if servePublic {
		routes.registerPublicAPI(mux)
	} else if mode == listenerModeAdminOnly {
		routes.registerAdminPublicMetadataAPI(mux)
	}

	if serveAdmin {
		routes.registerAdmin(mux)
	} else if mode == listenerModePublicOnly {
		registerPublicAdminBlock(mux)
	}

	routes.registerEmbeddedAssets(mux)
	if servePublic {
		routes.registerPublicArtifactsAndSPA(mux)
	}

	resolver := &clientIPResolver{
		trustProxy:      opts.TrustProxyHeaders,
		trustCloudflare: opts.TrustCloudflareHeaders,
	}
	requestLogger := requestPathLogger(opts.Logger)

	return logMiddleware(requestLogger, resolver, corsMiddleware(gzipMiddleware(recoverMiddleware(requestLogger, resolver, rateLimitMiddleware(resolver, mux)))))
}

func registerHealthz(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
}
