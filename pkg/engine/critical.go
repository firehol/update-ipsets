package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type criticalProviderSet struct {
	Name          string
	Meta          CriticalInfrastructureProvider
	Set           iprange.RangeSource
	overlapFilter iprange.RangeOverlapFilter
	sources       []*closableSource
}

type criticalDatasets struct {
	Providers     map[string]*criticalProviderSet
	Names         []string
	Configured    []string
	Missing       []criticalMissingProviderJSON
	ProviderSetID string
}

func (d *criticalDatasets) closeAll() {
	if d == nil {
		return
	}
	for _, p := range d.Providers {
		if p == nil {
			continue
		}
		for _, s := range p.sources {
			if s != nil {
				_ = s.Close()
			}
		}
	}
}

// CriticalInfrastructureProvider describes one configured critical-infrastructure
// reference source as exposed to the public API. These are reference feeds used
// for overlap analysis, not threat feeds.
type CriticalInfrastructureProvider struct {
	Name            string `json:"name"`
	Label           string `json:"label,omitempty"`
	Type            string `json:"type,omitempty"`
	Tier            string `json:"tier"`
	Role            string `json:"role"`
	SourceType      string `json:"source_type"`
	SourceQuality   string `json:"source_quality"`
	Rationale       string `json:"rationale"`
	Info            string `json:"info,omitempty"`
	License         string `json:"license,omitempty"`
	Attribution     string `json:"attribution,omitempty"`
	Redistributable bool   `json:"redistributable"`
	Maintainer      string `json:"maintainer,omitempty"`
	MaintainerURL   string `json:"maintainer_url,omitempty"`
}

type criticalProviderOverlapJSON struct {
	Provider      CriticalInfrastructureProvider `json:"provider"`
	ProviderSetID string                         `json:"provider_set_id"`
	FeedIPs       uint64                         `json:"feed_ips"`
	CriticalIPs   uint64                         `json:"critical_ips"`
	Percent       float64                        `json:"percent"`
}

type criticalTierSummaryJSON struct {
	Tier        string  `json:"tier"`
	CriticalIPs uint64  `json:"critical_ips"`
	Percent     float64 `json:"percent"`
	Providers   int     `json:"providers"`
}

type criticalMissingProviderJSON struct {
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
}

type criticalASNContextMatchJSON struct {
	ASN           uint32  `json:"asn"`
	Name          string  `json:"name"`
	Tier          string  `json:"tier"`
	Role          string  `json:"role"`
	SourceQuality string  `json:"source_quality"`
	Rationale     string  `json:"rationale"`
	IPs           uint64  `json:"ips"`
	Percent       float64 `json:"percent"`
}

type criticalASNContextJSON struct {
	Provider string                        `json:"provider,omitempty"`
	FeedIPs  uint64                        `json:"feed_ips"`
	IPs      uint64                        `json:"ips"`
	Percent  float64                       `json:"percent"`
	Matches  []criticalASNContextMatchJSON `json:"matches,omitempty"`
}

type criticalAggregateJSON struct {
	Feed                string                        `json:"feed"`
	Family              string                        `json:"family"`
	FeedIPs             uint64                        `json:"feed_ips"`
	CriticalIPs         uint64                        `json:"critical_ips"`
	Percent             float64                       `json:"percent"`
	Complete            bool                          `json:"complete"`
	ProviderSetID       string                        `json:"provider_set_id"`
	ConfiguredProviders []string                      `json:"configured_providers,omitempty"`
	MissingProviders    []criticalMissingProviderJSON `json:"missing_providers,omitempty"`
	Tiers               []criticalTierSummaryJSON     `json:"tiers,omitempty"`
	Providers           []criticalProviderOverlapJSON `json:"providers,omitempty"`
	ASNContext          *criticalASNContextJSON       `json:"asn_context,omitempty"`
}

// CriticalInfrastructureProviders returns configured critical-infrastructure
// reference sources in YAML declaration order. Hidden sources are included
// because the endpoint describes comparison providers, not navigable feed pages.
func (e *Engine) CriticalInfrastructureProviders() []CriticalInfrastructureProvider {
	return criticalInfrastructureProvidersForConfig(e.Config())
}

func criticalInfrastructureProvidersForConfig(cfg *config.Config) []CriticalInfrastructureProvider {
	if cfg == nil {
		return nil
	}
	sources := cfg.SourcesWithUse(config.UseCriticalInfrastructure)
	out := make([]CriticalInfrastructureProvider, 0, len(sources))
	for _, src := range sources {
		if src == nil || src.Critical == nil {
			continue
		}
		out = append(out, criticalProviderFromSource(src))
	}
	return out
}

func (e *Engine) CriticalInfrastructureProviderSetID() string {
	if e == nil {
		return ""
	}
	e.criticalProviderSetMu.RLock()
	if e.criticalProviderSetCached {
		id := e.criticalProviderSetID
		e.criticalProviderSetMu.RUnlock()
		return id
	}
	e.criticalProviderSetMu.RUnlock()
	return e.refreshCriticalInfrastructureProviderSetID()
}

func (e *Engine) refreshCriticalInfrastructureProviderSetID() string {
	if e == nil {
		return ""
	}
	id := CriticalInfrastructureProviderSetIDForSnapshot(e.Config())
	e.criticalProviderSetMu.Lock()
	e.criticalProviderSetID = id
	e.criticalProviderSetCached = true
	e.criticalProviderSetMu.Unlock()
	return id
}

// CriticalInfrastructureProviderSetIDForSnapshot returns the stable identity
// of the currently configured critical reference set. The identity is derived
// only from configured catalog membership and per-provider configuration
// fingerprints (and the configured critical_asn_context list when present);
// it deliberately excludes materialized cache state such as ContentHash,
// Entries, and UniqueIPs so the identity does not drift while the pipeline is
// running. Per-feed overlap freshness is enforced separately through the
// processing-time mtime contract on the secondary artifacts.
func CriticalInfrastructureProviderSetIDForSnapshot(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	providers := cfg.SourcesWithUse(config.UseCriticalInfrastructure)
	if len(providers) == 0 && len(cfg.CriticalASNContext) == 0 {
		return ""
	}
	out := make([]criticalProviderSetRuntimeIdentity, 0, len(providers))
	for _, src := range providers {
		if src == nil || src.Critical == nil {
			continue
		}
		item := criticalProviderSetRuntimeIdentity{CriticalInfrastructureProvider: criticalProviderFromSource(src)}
		item.SourceConfig = criticalProviderSourceFingerprint(src)
		out = append(out, item)
	}
	if len(cfg.CriticalASNContext) > 0 {
		for _, src := range cfg.SourcesWithUse(config.UseASN) {
			if src == nil {
				continue
			}
			item := criticalProviderSetRuntimeIdentity{
				CriticalInfrastructureProvider: CriticalInfrastructureProvider{
					Name:            src.Name,
					Label:           src.Label,
					Type:            "critical_asn_context_provider",
					Info:            src.Info,
					License:         src.License,
					Attribution:     src.Attribution,
					Redistributable: src.IsRedistributable(),
					Maintainer:      src.Maintainer,
					MaintainerURL:   src.MaintainerURL,
				},
				SourceConfig: criticalProviderSourceFingerprint(src),
			}
			out = append(out, item)
		}
	}
	return criticalProviderSetRuntimeID(out, cfg.CriticalASNContext)
}

func CriticalInfrastructureProviderSetMarkerPath(rt Runtime) string {
	if rt.LibDir == "" {
		return ""
	}
	return filepath.Join(rt.LibDir, "critical_infrastructure", "provider_set_id")
}

func CriticalInfrastructureProviderSetChangedForSnapshot(cfg *config.Config, rt Runtime) bool {
	current := CriticalInfrastructureProviderSetIDForSnapshot(cfg)
	path := CriticalInfrastructureProviderSetMarkerPath(rt)
	if path == "" {
		return false
	}
	previous := readCriticalInfrastructureProviderSetMarker(rt)
	return previous != current
}

// writeCriticalInfrastructureProviderSetMarkerValue writes the supplied
// identity to the runtime marker. An empty identity removes the marker so a
// catalog that drops all critical providers stops claiming a non-empty
// provider set.
func (e *Engine) writeCriticalInfrastructureProviderSetMarkerValue(id string) error {
	if e == nil {
		return nil
	}
	path := CriticalInfrastructureProviderSetMarkerPath(e.Runtime())
	return writeCriticalInfrastructureProviderSetMarkerValueAtPath(path, id)
}

func writeCriticalInfrastructureProviderSetMarkerValueAtPath(path, id string) error {
	if path == "" {
		return nil
	}
	if id == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeFileAtomic(path, []byte(id+"\n"), generatedFileMode)
}

func readCriticalInfrastructureProviderSetMarker(rt Runtime) string {
	data, err := readFileInRoot(rt.LibDir, filepath.Join("critical_infrastructure", "provider_set_id"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func criticalProviderFromSource(src *config.Source) CriticalInfrastructureProvider {
	if src == nil || src.Critical == nil {
		return CriticalInfrastructureProvider{}
	}
	meta := src.Critical
	return CriticalInfrastructureProvider{
		Name:            src.Name,
		Label:           src.Label,
		Type:            src.Format,
		Tier:            meta.Tier,
		Role:            meta.Role,
		SourceType:      meta.SourceType,
		SourceQuality:   meta.SourceQuality,
		Rationale:       meta.Rationale,
		Info:            src.Info,
		License:         src.License,
		Attribution:     src.Attribution,
		Redistributable: src.IsRedistributable(),
		Maintainer:      src.Maintainer,
		MaintainerURL:   src.MaintainerURL,
	}
}

type criticalProviderSetRuntimeIdentity struct {
	CriticalInfrastructureProvider
	SourceConfig string
}

func criticalProviderSetRuntimeID(providers []criticalProviderSetRuntimeIdentity, asnContext []config.CriticalASNContext) string {
	if len(providers) == 0 && len(asnContext) == 0 {
		return ""
	}
	items := append([]criticalProviderSetRuntimeIdentity(nil), providers...)
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	asns := append([]config.CriticalASNContext(nil), asnContext...)
	sort.Slice(asns, func(i, j int) bool { return asns[i].ASN < asns[j].ASN })
	h := sha256.New()
	for _, p := range items {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%t\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00",
			p.Name, p.Label, p.Type, p.Tier, p.Role, p.SourceType, p.SourceQuality, p.Rationale,
			p.Redistributable, p.Info, p.License, p.Attribution, p.Maintainer, p.MaintainerURL,
			p.SourceConfig)
	}
	_, _ = fmt.Fprint(h, "critical_asn_context\x00")
	for _, entry := range asns {
		_, _ = fmt.Fprintf(h, "%d\x00%s\x00%s\x00%s\x00%s\x00%s\x00",
			entry.ASN, entry.Name, entry.Tier, entry.Role, entry.SourceQuality, entry.Rationale)
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func criticalProviderSourceFingerprint(src *config.Source) string {
	if src == nil {
		return ""
	}
	payload := struct {
		URL               string                   `json:"url,omitempty"`
		Static            []string                 `json:"static,omitempty"`
		IPV               string                   `json:"ipv,omitempty"`
		Output            string                   `json:"output,omitempty"`
		Format            string                   `json:"format,omitempty"`
		Processor         []config.ProcessorStep   `json:"processor,omitempty"`
		ProcessorRaw      string                   `json:"processor_raw,omitempty"`
		Downloader        string                   `json:"downloader,omitempty"`
		DownloaderOptions string                   `json:"downloader_options,omitempty"`
		Attributes        map[string]string        `json:"attributes,omitempty"`
		Critical          *config.CriticalMetadata `json:"critical,omitempty"`
	}{
		URL:               src.URL,
		Static:            append([]string(nil), src.Static...),
		IPV:               src.IPV,
		Output:            src.Output,
		Format:            src.Format,
		Processor:         append([]config.ProcessorStep(nil), src.Processor...),
		ProcessorRaw:      src.ProcessorRaw,
		Downloader:        src.Downloader,
		DownloaderOptions: src.DownloaderOptions,
		Attributes:        cloneStringMap(src.Attributes),
		Critical:          cloneCriticalMetadata(src.Critical),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func cloneCriticalMetadata(in *config.CriticalMetadata) *config.CriticalMetadata {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// loadCriticalInfrastructureSources opens the latest set for each configured
// critical-infrastructure reference source and stamps the resulting dataset
// with providerSetID. providerSetID MUST be the value captured once at
// pipeline plan time (pipelineRunPlan.criticalProviderSetID); callers MUST NOT
// re-read it from engine state, so all per-feed artifacts in this run carry
// the exact same identity as the runtime marker written at the end of the
// run.
func (e *Engine) loadCriticalInfrastructureSources(ctx context.Context, providerSetID string) (*criticalDatasets, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	sources := e.cfg.SourcesWithUse(config.UseCriticalInfrastructure)
	if len(sources) == 0 {
		return &criticalDatasets{Providers: map[string]*criticalProviderSet{}}, nil
	}

	out := &criticalDatasets{
		Providers:     make(map[string]*criticalProviderSet, len(sources)),
		ProviderSetID: providerSetID,
	}
	loadOp := e.beginActiveOperation("critical.load_providers", "", "load", "providers", int64(len(sources)))
	defer loadOp.Finish()
	for _, src := range sources {
		if err := contextErr(ctx); err != nil {
			out.closeAll()
			return nil, err
		}
		func() {
			defer loadOp.Add(1, int64(len(sources)), nil)
			if src == nil || src.Critical == nil {
				return
			}
			name := src.Name
			out.Configured = append(out.Configured, name)
			latest, err := e.openLatestSet(ctx, name)
			if err != nil {
				e.logger.Warn("critical infrastructure source skipped: latest set not available",
					"source", name, "error", err)
				out.Missing = append(out.Missing, criticalMissingProviderJSON{Name: name, Reason: err.Error()})
				return
			}
			filter, err := iprange.BuildRangeOverlapFilterContext(ctx, latest.RangeSource)
			if err != nil {
				e.logger.Warn("critical infrastructure source skipped: overlap filter failed",
					"source", name, "error", err)
				out.Missing = append(out.Missing, criticalMissingProviderJSON{Name: name, Reason: err.Error()})
				_ = latest.Close()
				return
			}
			out.Providers[name] = &criticalProviderSet{
				Name:          name,
				Meta:          criticalProviderFromSource(src),
				Set:           latest.RangeSource,
				overlapFilter: filter,
				sources:       []*closableSource{latest},
			}
			out.Names = append(out.Names, name)
		}()
	}
	return out, nil
}

func (e *Engine) writeCriticalInfrastructureFiles(ctx context.Context, datasets *criticalDatasets, updatedNames []string, outDir string, setCache *latestSetCache) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if datasets == nil || len(datasets.Configured) == 0 {
		return nil
	}
	if setCache == nil {
		setCache = newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
	}

	roles := []string{config.UseCriticalInfrastructure}
	if len(e.cfg.CriticalASNContext) > 0 {
		roles = append(roles, config.UseASN)
	}
	targetNames := criticalTargetNames(e.cfg, targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), roles...))
	if len(targetNames) == 0 {
		return nil
	}
	compareOp := e.beginActiveOperation("critical.write_comparisons", "", "compare", "feeds", int64(len(targetNames)))
	defer compareOp.Finish()

	numWorkers := e.runtime.HeavyPhaseWorkers()
	if numWorkers < 1 {
		numWorkers = 1
	}

	var mu sync.Mutex
	overlapTiers := make(map[string][]string, len(targetNames))

	if err := runBoundedNameJobs(ctx, numWorkers, targetNames, func(ctx context.Context, name string) error {
		if err := contextErr(ctx); err != nil {
			return err
		}
		defer compareOp.Add(1, int64(len(targetNames)), nil)
		tiers, err := e.writeCriticalInfrastructureForFeed(ctx, name, datasets, outDir, setCache)
		if err != nil {
			return err
		}
		if err := contextErr(ctx); err != nil {
			return err
		}
		mu.Lock()
		overlapTiers[name] = tiers
		mu.Unlock()
		return nil
	}); err != nil {
		return err
	}
	for _, name := range targetNames {
		if err := contextErr(ctx); err != nil {
			return err
		}
		e.setCriticalOverlapSummary(name, overlapTiers[name])
	}
	return nil
}

func criticalOverlapTiersFromAggregate(payload criticalAggregateJSON) []string {
	tiers := make([]string, 0, len(payload.Tiers))
	for _, tier := range payload.Tiers {
		if tier.CriticalIPs > 0 && tier.Tier != "" {
			tiers = append(tiers, tier.Tier)
		}
	}
	return tiers
}

func (e *Engine) setCriticalOverlapSummary(name string, tiers []string) {
	if e == nil || e.state == nil || name == "" {
		return
	}
	entry := e.state.Entry(name)
	entry.SetCriticalOverlapTiers(tiers)
}

func (e *Engine) clearCriticalOverlapSummaries() {
	if e == nil || e.state == nil {
		return
	}
	for _, entry := range e.EntriesSnapshot() {
		if len(entry.CriticalOverlapTiers) == 0 {
			continue
		}
		e.state.Entry(entry.Name).ClearCriticalOverlapTiers()
	}
}

func (e *Engine) writeCriticalProviderPayload(outDir, feedName, providerName string, payload criticalProviderOverlapJSON) error {
	data, err := jsonMarshalTabIndent(payload)
	if err != nil {
		return err
	}
	outPath := filepath.Join(outDir, feedName+"_critical_"+providerName+".json")
	return writeFileAtomicAt(outPath, append(data, '\n'), generatedFileMode, e.feedProcessingTimestamp(feedName))
}

func (e *Engine) criticalASNContextForFeed(feedName string, feedIPs uint64, outDir string) (*criticalASNContextJSON, error) {
	if e == nil || e.cfg == nil || len(e.cfg.CriticalASNContext) == 0 {
		return nil, nil
	}
	provider, payload, err := e.readPreferredASNPayload(feedName, outDir)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, nil
	}
	contextByASN := make(map[uint32]config.CriticalASNContext, len(e.cfg.CriticalASNContext))
	for _, entry := range e.cfg.CriticalASNContext {
		contextByASN[entry.ASN] = entry
	}
	out := &criticalASNContextJSON{
		Provider: provider,
		FeedIPs:  feedIPs,
	}
	for _, row := range payload.ByASN {
		context, ok := contextByASN[row.ASN]
		if !ok || row.Count == 0 {
			continue
		}
		match := criticalASNContextMatchJSON{
			ASN:           row.ASN,
			Name:          context.Name,
			Tier:          context.Tier,
			Role:          context.Role,
			SourceQuality: context.SourceQuality,
			Rationale:     context.Rationale,
			IPs:           row.Count,
		}
		if feedIPs > 0 {
			match.Percent = 100 * float64(row.Count) / float64(feedIPs)
		}
		out.IPs += row.Count
		out.Matches = append(out.Matches, match)
	}
	if len(out.Matches) == 0 {
		return nil, nil
	}
	if feedIPs > 0 {
		out.Percent = 100 * float64(out.IPs) / float64(feedIPs)
	}
	sort.Slice(out.Matches, func(i, j int) bool {
		leftTier, rightTier := criticalTierRank(out.Matches[i].Tier), criticalTierRank(out.Matches[j].Tier)
		if leftTier != rightTier {
			return leftTier < rightTier
		}
		if out.Matches[i].IPs != out.Matches[j].IPs {
			return out.Matches[i].IPs > out.Matches[j].IPs
		}
		return out.Matches[i].ASN < out.Matches[j].ASN
	})
	return out, nil
}

func (e *Engine) readPreferredASNPayload(feedName, outDir string) (string, *asnFeedJSON, error) {
	// Respect the configured default ASN provider (defaults.asn_provider).
	// Plain SourcesWithUse() iterates in YAML insertion order, which means
	// the first alphabetically-named ASN provider wins regardless of the
	// curator's choice — caida_prefix2as gets picked over iptoasn even
	// though iptoasn is the configured default. SourcesWithUseDefaultFirst
	// moves the default to the front and keeps the remaining catalog
	// order as a fallback when the default has no payload for this feed.
	for _, provider := range e.cfg.SourcesWithUseDefaultFirst(config.UseASN) {
		if provider == nil {
			continue
		}
		rel := feedName + "_asn_" + provider.Name + ".json"
		data, err := readFirstExisting(singleCandidatePath(outDir, rel), singleCandidatePath(e.outputDir(), rel))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return provider.Name, nil, err
		}
		var payload asnFeedJSON
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Name, nil, fmt.Errorf("read ASN context payload %s: %w", rel, err)
		}
		return provider.Name, &payload, nil
	}
	return "", nil, nil
}

func criticalTierRank(tier string) int {
	for i, known := range config.CriticalTiers() {
		if tier == known {
			return i
		}
	}
	return len(config.CriticalTiers())
}

func criticalTargetNames(cfg *config.Config, names []string) []string {
	if cfg == nil || len(names) == 0 {
		return names
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		if isCriticalInfrastructureOutputName(cfg, name) {
			continue
		}
		if !isCriticalInfrastructureComparableName(cfg, name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (e *Engine) IsCriticalInfrastructureTarget(name string) bool {
	if e == nil {
		return false
	}
	cfg := e.Config()
	return !isCriticalInfrastructureOutputName(cfg, name) && isCriticalInfrastructureComparableName(cfg, name)
}

func (e *Engine) markStaleCriticalInfrastructureArtifactDeletes(batch *stagedPublishBatch) error {
	cfg, rt := e.configRuntimeSnapshot()
	return markStaleCriticalInfrastructureArtifactDeletesForConfig(batch, cfg, outputDirForRuntime(rt))
}

func markStaleCriticalInfrastructureArtifactDeletesForConfig(batch *stagedPublishBatch, cfg *config.Config, liveDir string) error {
	if cfg == nil || batch == nil {
		return nil
	}
	if liveDir == "" {
		return nil
	}
	entries, err := os.ReadDir(liveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	currentProviders := make(map[string]struct{})
	for _, provider := range criticalInfrastructureProvidersForConfig(cfg) {
		currentProviders[provider.Name] = struct{}{}
	}
	hasProviders := len(currentProviders) > 0
	publicNames := configuredPublicFeedNames(cfg)
	currentPublic := stringExactSet(publicNames)
	currentTargets := stringExactSet(criticalTargetNames(cfg, publicNames))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		base := strings.TrimSuffix(name, ".json")
		if _, ok := currentPublic[base]; ok {
			continue
		}
		if strings.HasSuffix(base, "_critical_infrastructure") {
			feed := strings.TrimSuffix(base, "_critical_infrastructure")
			if !hasProviders {
				batch.markDelete(name)
				continue
			}
			if _, ok := currentTargets[feed]; !ok {
				batch.markDelete(name)
			}
			continue
		}
		feed, provider, ok := criticalProviderArtifactPartsForTargets(base, currentPublic, currentProviders)
		if !ok {
			continue
		}
		if _, ok := currentTargets[feed]; !ok {
			batch.markDelete(name)
			continue
		}
		if _, ok := currentProviders[provider]; !ok {
			batch.markDelete(name)
		}
	}
	return nil
}

func (e *Engine) CleanupStaleCriticalInfrastructureArtifacts() error {
	return e.CleanupStaleCriticalInfrastructureArtifactsContext(context.Background())
}

func (e *Engine) CleanupStaleCriticalInfrastructureArtifactsContext(ctx context.Context) error {
	if e == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	cfg, rt := e.configRuntimeSnapshot()
	if cfg == nil {
		return nil
	}
	providers := criticalInfrastructureProvidersForConfig(cfg)
	if len(providers) == 0 {
		e.clearCriticalOverlapSummaries()
	}
	batch, err := newWebPublishBatchForRuntime(rt)
	if err != nil {
		return err
	}
	defer batch.cleanup()
	if err := markStaleCriticalInfrastructureArtifactDeletesForConfig(batch.stagedPublishBatch, cfg, outputDirForRuntime(rt)); err != nil {
		return err
	}
	if _, err := batch.publishContext(ctx); err != nil {
		return err
	}
	if len(providers) == 0 {
		return writeCriticalInfrastructureProviderSetMarkerValueAtPath(CriticalInfrastructureProviderSetMarkerPath(rt), CriticalInfrastructureProviderSetIDForSnapshot(cfg))
	}
	return nil
}

func (e *Engine) QueueCriticalInfrastructureCleanup(ctx context.Context, trigger string) (LaneTicket, error) {
	if e == nil || e.engineLane == nil {
		return LaneTicket{}, nil
	}
	ctx = nonNilContext(ctx)
	if trigger == "" {
		trigger = "background"
	}
	return e.engineLane.Submit(ctx, LaneWork{
		Kind:          LaneWorkCleanup,
		Component:     LaneComponentCriticalInfrastructure,
		Name:          "cleanup.critical_infrastructure",
		Trigger:       trigger,
		Stage:         "cleanup",
		Detail:        "removing stale critical infrastructure artifacts",
		CoalescingKey: criticalInfrastructureCleanupCoalescingKey(trigger),
	}, func(laneCtx context.Context) error {
		return e.cleanupStaleCriticalInfrastructureArtifactsAdmitted(laneCtx, trigger)
	})
}

func criticalInfrastructureCleanupCoalescingKey(trigger string) string {
	switch trigger {
	case "reload":
		return "cleanup:critical_infrastructure:reload"
	case "startup":
		return "cleanup:critical_infrastructure:startup"
	default:
		return "cleanup:critical_infrastructure:background"
	}
}

func (e *Engine) CleanupStaleCriticalInfrastructureArtifactsWithTrigger(ctx context.Context, trigger string) error {
	if e == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if trigger == "" {
		trigger = "background"
	}
	if e.engineLane == nil {
		return e.cleanupStaleCriticalInfrastructureArtifactsAdmitted(ctx, trigger)
	}
	return e.engineLane.Run(ctx, LaneWork{
		Kind:      LaneWorkCleanup,
		Component: LaneComponentCriticalInfrastructure,
		Name:      "cleanup.critical_infrastructure",
		Trigger:   trigger,
		Stage:     "cleanup",
		Detail:    "removing stale critical infrastructure artifacts",
	}, func(laneCtx context.Context) error {
		return e.cleanupStaleCriticalInfrastructureArtifactsAdmitted(laneCtx, trigger)
	})
}

func (e *Engine) cleanupStaleCriticalInfrastructureArtifactsAdmitted(ctx context.Context, trigger string) error {
	err := e.CleanupStaleCriticalInfrastructureArtifactsContext(ctx)
	if err != nil && e.logger != nil {
		e.logger.Warn("failed to cleanup stale critical infrastructure artifacts", "trigger", trigger, "error", err)
	}
	return err
}

func (e *Engine) CleanupCriticalInfrastructureArtifactsIfUnconfigured() error {
	if e == nil {
		return nil
	}
	cfg := e.Config()
	if cfg == nil || len(criticalInfrastructureProvidersForConfig(cfg)) > 0 {
		return nil
	}
	e.clearCriticalOverlapSummaries()
	return e.CleanupStaleCriticalInfrastructureArtifacts()
}

func configuredPublicFeedNames(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Sources))
	for name, src := range cfg.Sources {
		if isPublicFeedSource(src) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func criticalProviderArtifactPartsForTargets(base string, targets, currentProviders map[string]struct{}) (string, string, bool) {
	if len(targets) == 0 {
		return "", "", false
	}
	names := make([]string, 0, len(targets))
	for target := range targets {
		names = append(names, target)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })

	for _, feed := range names {
		prefix := feed + "_critical_"
		provider, ok := strings.CutPrefix(base, prefix)
		if !ok || provider == "" {
			continue
		}
		if _, ok := currentProviders[provider]; ok {
			return feed, provider, true
		}
	}

	for _, feed := range names {
		if feed == "" || !strings.HasPrefix(base, feed+"_") {
			continue
		}
		prefix := feed + "_critical_"
		if provider, ok := strings.CutPrefix(base, prefix); ok && provider != "" {
			return feed, provider, true
		}
		return "", "", false
	}
	return "", "", false
}

func criticalFeedFamily(cfg *config.Config, name string) string {
	if cfg != nil {
		if src := cfg.Sources[name]; src != nil && src.IPV != "" {
			return src.IPV
		}
	}
	return "ipv4"
}

func isCriticalInfrastructureComparableName(cfg *config.Config, name string) bool {
	if cfg == nil || name == "" {
		return false
	}
	src := cfg.Sources[name]
	if src == nil {
		return false
	}
	if src.HasUse(config.UseProviderContext) {
		return false
	}
	return src.IPV == "" || src.IPV == "ipv4"
}

func isCriticalInfrastructureOutputName(cfg *config.Config, name string) bool {
	if cfg == nil || name == "" {
		return false
	}
	if src := cfg.Sources[name]; src != nil {
		return src.HasUse(config.UseCriticalInfrastructure)
	}
	for sourceName, src := range cfg.Sources {
		if src == nil || !src.HasUse(config.UseCriticalInfrastructure) {
			continue
		}
		for _, minutes := range src.History {
			if name == sourceName+historyLabel(minutes) {
				return true
			}
		}
	}
	return false
}
