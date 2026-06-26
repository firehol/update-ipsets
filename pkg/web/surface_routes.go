package web

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	mcppkg "github.com/firehol/update-ipsets/pkg/mcp"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

const (
	publicServingReloadListenerName = "web.public_serving_state"
	adminServingReloadListenerName  = "web.admin_serving_state"
)

type publicServingTuple struct {
	outputDir     string
	ipsetsDir     string
	baseDir       string
	cacheEntries  int
	cacheBytes    int64
	cacheFileSize int64
}

// publicServingState is published as one immutable generation. The path fields
// intentionally duplicate tuple values so handlers do not unpack the cache key.
type publicServingState struct {
	tuple     publicServingTuple
	outputDir string
	ipsetsDir string
	baseDir   string
	cache     *fileCache
}

type surfaceRoutes struct {
	eng           *engine.Engine
	opts          Options
	runner        *scheduler.Runner
	baseCtx       context.Context
	servingMu     sync.Mutex
	serving       atomic.Pointer[publicServingState]
	searchLimiter *clientRateLimiter
	resolver      *clientIPResolver
	mcpServer     *mcppkg.Server
}

func newSurfaceRoutesWithContext(ctx context.Context, eng *engine.Engine, opts Options, runner *scheduler.Runner) *surfaceRoutes {
	if ctx == nil {
		ctx = context.Background()
	}
	routes := &surfaceRoutes{
		eng:           eng,
		opts:          opts,
		runner:        runner,
		baseCtx:       ctx,
		searchLimiter: newClientRateLimiter(10, time.Minute),
		resolver: &clientIPResolver{
			trustProxy:      opts.TrustProxyHeaders,
			trustCloudflare: opts.TrustCloudflareHeaders,
		},
	}
	routes.refreshPublicServingState()
	routes.mcpServer = mcppkg.NewServer(
		mcppkg.NewEngineFeedCatalog(eng),
		publicServingMarkdownStore{routes: routes},
	)
	return routes
}

func (s *surfaceRoutes) registerPublicServingReloadListener() {
	s.registerServingReloadListener(publicServingReloadListenerName)
}

func (s *surfaceRoutes) registerAdminServingReloadListener() {
	s.registerServingReloadListener(adminServingReloadListenerName)
}

func (s *surfaceRoutes) registerServingReloadListener(name string) {
	if s == nil || s.eng == nil {
		return
	}
	s.eng.RegisterReloadPublicationListener(name, func(engine.ReloadPublication) error {
		s.refreshPublicServingState()
		return nil
	})
}

func (s *surfaceRoutes) currentPublicServingState() *publicServingState {
	if s == nil {
		return nil
	}
	state := s.serving.Load()
	if state != nil {
		return state
	}
	s.refreshPublicServingState()
	return s.serving.Load()
}

func (s *surfaceRoutes) refreshPublicServingState() {
	if s == nil || s.eng == nil {
		return
	}
	_, runtime := s.eng.ConfigRuntimeSnapshot()
	outputDir := outputDirFromOptions(runtime.BaseDir, choose(s.opts.WebDir, runtime.WebDir))
	ipsetsDir := filesDir(runtime.BaseDir, choose(s.opts.FilesDir, runtime.WebDirForIPSets))
	tuple := publicServingTuple{
		outputDir:     outputDir,
		ipsetsDir:     ipsetsDir,
		baseDir:       runtime.BaseDir,
		cacheEntries:  runtime.WebArtifactCacheMaxEntries,
		cacheBytes:    runtime.WebArtifactCacheMaxBytes,
		cacheFileSize: runtime.WebArtifactCacheMaxFileBytes,
	}

	s.servingMu.Lock()
	defer s.servingMu.Unlock()
	if current := s.serving.Load(); current != nil && current.tuple == tuple {
		return
	}
	// Raw ipset/netset files are written to BaseDir by the engine; the
	// optional ipsetsDir (WEB_DIR_FOR_IPSETS) is a curated mirror used for
	// bash-compatible or migrated mirror layouts. Locally the mirror can be
	// empty, so raw serving falls back to BaseDir.
	s.serving.Store(&publicServingState{
		tuple:     tuple,
		outputDir: outputDir,
		ipsetsDir: ipsetsDir,
		baseDir:   runtime.BaseDir,
		cache: newFileCacheWithLimits(fileCacheLimits{
			MaxEntries:   runtime.WebArtifactCacheMaxEntries,
			MaxBytes:     runtime.WebArtifactCacheMaxBytes,
			MaxFileBytes: runtime.WebArtifactCacheMaxFileBytes,
		}),
	})
}

type publicServingMarkdownStore struct {
	routes *surfaceRoutes
}

func (s publicServingMarkdownStore) ReadMarkdown(entityType, name string) ([]byte, error) {
	if s.routes == nil {
		return nil, fmt.Errorf("public serving state is not available")
	}
	state := s.routes.currentPublicServingState()
	if state == nil {
		return nil, fmt.Errorf("public serving state is not available")
	}
	return mcppkg.NewFileMarkdownStore(state.outputDir).ReadMarkdown(entityType, name)
}
