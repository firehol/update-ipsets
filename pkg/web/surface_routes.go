package web

import (
	"context"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	mcppkg "github.com/firehol/update-ipsets/pkg/mcp"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

type surfaceRoutes struct {
	eng           *engine.Engine
	opts          Options
	runner        *scheduler.Runner
	baseCtx       context.Context
	outputDir     string
	ipsetsDir     string
	baseDir       string
	cache         *fileCache
	searchLimiter *clientRateLimiter
	resolver      *clientIPResolver
	mcpServer     *mcppkg.Server
}

func newSurfaceRoutesWithContext(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) *surfaceRoutes {
	if ctx == nil {
		ctx = context.Background()
	}
	outputDir := outputDirFromOptions(eng.Runtime().BaseDir, choose(opts.WebDir, eng.Runtime().WebDir))
	ipsetsDir := filesDir(eng.Runtime().BaseDir, choose(opts.FilesDir, eng.Runtime().WebDirForIPSets))
	runtime := eng.Runtime()
	// Raw ipset/netset files are written to BaseDir by the engine; the
	// optional ipsetsDir (WEB_DIR_FOR_IPSETS) is a curated mirror used for
	// bash-compatible or migrated mirror layouts. Locally the mirror is empty,
	// so raw serving falls back to BaseDir.
	return &surfaceRoutes{
		eng:       eng,
		opts:      opts,
		runner:    runner,
		baseCtx:   ctx,
		outputDir: outputDir,
		ipsetsDir: ipsetsDir,
		baseDir:   eng.Runtime().BaseDir,
		cache: newFileCacheWithLimits(fileCacheLimits{
			MaxEntries:   runtime.WebArtifactCacheMaxEntries,
			MaxBytes:     runtime.WebArtifactCacheMaxBytes,
			MaxFileBytes: runtime.WebArtifactCacheMaxFileBytes,
		}),
		searchLimiter: newClientRateLimiter(10, time.Minute),
		resolver: &clientIPResolver{
			trustProxy:      opts.TrustProxyHeaders,
			trustCloudflare: opts.TrustCloudflareHeaders,
		},
		mcpServer: mcppkg.NewServer(
			mcppkg.NewEngineFeedCatalog(eng),
			mcppkg.NewFileMarkdownStore(outputDir),
		),
	}
}
