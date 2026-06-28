package web

import (
	"context"
	"fmt"
	"strings"
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
	catalog   engine.PublicServingCatalogSnapshot
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
	routes.mcpServer = mcppkg.NewServer(publicServingFeedCatalog{routes: routes}, publicServingMarkdownStore{routes: routes})
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
	s.eng.RegisterReloadPublicationListener(name, func(pub engine.ReloadPublication) error {
		s.refreshPublicServingStateForRuntime(pub.Runtime)
		return nil
	})
}

func (s *surfaceRoutes) currentPublicServingState() *publicServingState {
	if s == nil {
		return nil
	}
	return s.serving.Load()
}

func (s *surfaceRoutes) refreshPublicServingState() {
	if s == nil || s.eng == nil {
		return
	}
	_, runtime, ok := s.eng.TryConfigRuntimeSnapshot()
	if !ok {
		return
	}
	s.refreshPublicServingStateForRuntime(runtime)
}

func (s *surfaceRoutes) refreshPublicServingStateForRuntime(runtime engine.Runtime) {
	catalog, ok := s.eng.TryPublicServingCatalogSnapshot()
	if !ok {
		if current := s.currentPublicServingState(); current != nil {
			catalog = current.catalog
		} else {
			return
		}
	}
	s.storePublicServingState(runtime, catalog)
}

func (s *surfaceRoutes) storePublicServingState(runtime engine.Runtime, catalog engine.PublicServingCatalogSnapshot) {
	if s == nil {
		return
	}
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
		s.serving.Store(&publicServingState{
			tuple:     tuple,
			outputDir: outputDir,
			ipsetsDir: ipsetsDir,
			baseDir:   runtime.BaseDir,
			cache:     current.cache,
			catalog:   catalog,
		})
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
		catalog: catalog,
	})
}

func (s *publicServingState) isPublicFeedName(name string) bool {
	if s == nil {
		return false
	}
	_, ok := s.catalog.PublicFeedNames[name]
	return ok
}

func (s *publicServingState) rawFeedRel(name string) (string, bool) {
	if s == nil {
		return "", false
	}
	rel, ok := s.catalog.RawFeedFiles[name]
	return rel, ok
}

func (s *publicServingState) criticalInfrastructureTarget(name string) bool {
	if s == nil {
		return false
	}
	_, ok := s.catalog.CriticalInfrastructureTargets[name]
	return ok
}

func (s *publicServingState) criticalInfrastructureProvider(name string) bool {
	if s == nil {
		return false
	}
	for _, provider := range s.catalog.CriticalInfrastructureProviders {
		if provider.Name == name {
			return true
		}
	}
	return false
}

func (s *publicServingState) providerScopedArtifactFeedName(base string) (string, bool) {
	if s == nil {
		return "", false
	}
	for _, provider := range s.catalog.GeoProviders {
		suffix := "_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	for _, provider := range s.catalog.ASNProviders {
		suffix := "_asn_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	for _, provider := range s.catalog.BogonProviders {
		suffix := "_bogons_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	if strings.HasSuffix(base, "_critical_infrastructure") {
		return strings.TrimSuffix(base, "_critical_infrastructure"), true
	}
	for _, provider := range s.catalog.CriticalInfrastructureProviders {
		suffix := "_critical_" + provider.Name
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix), true
		}
	}
	return "", false
}

type publicServingFeedCatalog struct {
	routes *surfaceRoutes
}

func (c publicServingFeedCatalog) summaries() []engine.PublicFeedSummary {
	if c.routes == nil {
		return nil
	}
	state := c.routes.currentPublicServingState()
	if state == nil {
		return nil
	}
	return state.catalog.Feeds
}

func (c publicServingFeedCatalog) FeedFilterOptions() mcppkg.FeedFilterOptions {
	return mcppkg.FeedFilterOptionsFromSummaries(c.summaries())
}

func (c publicServingFeedCatalog) FindFeeds(filters mcppkg.FeedFilters) ([]mcppkg.FeedHit, error) {
	return mcppkg.FindFeedsInSummaries(c.summaries(), filters, time.Now().UTC()), nil
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
