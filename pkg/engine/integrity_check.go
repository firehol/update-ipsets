package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

// CheckIntegrity walks every live feed in the catalog and verifies
// that its committed canonical feed body exists and every per-feed secondary
// file (geo / asn / bogon / critical-infrastructure providers,
// comparison matrix, insights,
// metadata JSON, history CSV) is present and not older than the
// feed's last ProcessedDate.
//
// A file newer than ProcessedDate is fine. A file older than
// ProcessedDate signals a pipeline interruption: finalize updated
// the cache and committed canonical feed body, but one or more heavy-block writers
// did not run or failed midway.
//
// Runs at daemon startup (logged, non-fatal) and on demand via
// the admin API (GET /api/v1/admin/integrity). The caller can
// then reprocess the affected feeds via the standard
// runner.TriggerSources path — see
// POST /api/v1/admin/integrity/reprocess which does both in one
// request.
//
// Hidden sources and database sources (use:[asn]/[geoip]) are
// skipped: they have no ipset output and their secondary files
// follow a different layout.
func (e *Engine) CheckIntegrity() []IntegrityFinding {
	return e.CheckIntegrityWithOptions(IntegrityOptions{})
}

func (e *Engine) CheckIntegrityWithOptions(opts IntegrityOptions) []IntegrityFinding {
	findings, _ := e.CheckIntegrityWithOptionsContext(context.Background(), opts)
	return findings
}

func (e *Engine) CheckIntegrityContext(ctx context.Context) ([]IntegrityFinding, error) {
	return e.CheckIntegrityWithOptionsContext(ctx, IntegrityOptions{})
}

func (e *Engine) CheckIntegrityWithOptionsContext(ctx context.Context, opts IntegrityOptions) ([]IntegrityFinding, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	check, ok := e.newIntegrityCheck(opts)
	if !ok {
		return nil, nil
	}
	return check.findings(ctx)
}

type integrityCheck struct {
	e        *Engine
	snapshot operationSnapshot
	opts     IntegrityOptions
	webDir   string
	baseDir  string
	resolver *effectiveEntryResolver
}

type integritySourceContext struct {
	name   string
	src    *config.Source
	entry  *cache.Entry
	health feedhealth.Snapshot
}

func (e *Engine) newIntegrityCheck(opts IntegrityOptions) (integrityCheck, bool) {
	snap := e.operationSnapshot()
	if e == nil || snap.cfg == nil {
		return integrityCheck{}, false
	}
	webDir := opts.WebDir
	if webDir == "" {
		webDir = snap.runtime.WebDir
	}
	baseDir := snap.runtime.BaseDir
	if webDir == "" || baseDir == "" {
		return integrityCheck{}, false
	}
	return integrityCheck{
		e:        e,
		snapshot: snap,
		opts:     opts,
		webDir:   webDir,
		baseDir:  baseDir,
		resolver: newEffectiveEntryResolver(snap.cfg, e.state.SnapshotEntries()),
	}, true
}

func (c integrityCheck) findings(ctx context.Context) ([]IntegrityFinding, error) {
	var findings []IntegrityFinding
	for _, name := range config.SortedSourceNames(c.snapshot.cfg) {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		source, ok := c.sourceContext(name)
		if !ok {
			continue
		}
		finding, ok, err := c.findingForSource(ctx, source)
		if err != nil {
			return nil, err
		}
		if ok {
			findings = append(findings, finding)
		}
	}
	return findings, nil
}

func (c integrityCheck) sourceContext(name string) (integritySourceContext, bool) {
	src := c.snapshot.cfg.Sources[name]
	if src == nil || src.Hidden {
		return integritySourceContext{}, false
	}
	if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
		return integritySourceContext{}, false
	}
	health := feedhealth.Classify(c.resolver.entryFromSnapshot(name), src, c.snapshot.feedHealthPolicy, c.e.now().UTC())
	if health.Class == feedhealth.ClassArchived && !c.opts.IncludeArchived {
		return integritySourceContext{}, false
	}
	return integritySourceContext{
		name:   name,
		src:    src,
		entry:  c.e.state.Entry(name),
		health: health,
	}, true
}

func (c integrityCheck) findingForSource(ctx context.Context, source integritySourceContext) (IntegrityFinding, bool, error) {
	if err := contextErr(ctx); err != nil {
		return IntegrityFinding{}, false, err
	}
	if blockedFeeds := c.e.integrityBlockedFeedsWithSnapshot(c.snapshot, source.name, source.src, c.resolver, c.opts.EnableAll); len(blockedFeeds) > 0 {
		return c.blockedFeedFinding(source, blockedFeeds), true, nil
	}
	if source.entry == nil || source.entry.ProcessedDate == 0 {
		finding, ok := c.neverProcessedFinding(source.name)
		return finding, ok, nil
	}

	processedTime := time.Unix(source.entry.ProcessedDate, 0).UTC()
	sourcePath, sourceMTime, sourceOK := findSourceOutputFile(c.baseDir, source.name)
	if !sourceOK {
		finding, ok := c.missingSourceFinding(source, processedTime)
		return finding, ok, nil
	}
	if c.e.now().UTC().Sub(processedTime) < integrityInFlightTolerance {
		return IntegrityFinding{}, false, nil
	}
	if finding, ok := c.e.integrityHistoryDerivativeFindingWithSnapshot(c.snapshot, source.name, source.src, processedTime, sourcePath, sourceMTime); ok {
		return finding, true, nil
	}
	finding, ok, err := c.secondaryArtifactFinding(ctx, source, processedTime, sourcePath, sourceMTime)
	return finding, ok, err
}

func (c integrityCheck) blockedFeedFinding(source integritySourceContext, blockedFeeds []string) IntegrityFinding {
	finding := IntegrityFinding{
		Feed:         source.name,
		BlockedFeeds: blockedFeeds,
	}
	if source.entry != nil && source.entry.ProcessedDate > 0 {
		finding.ProcessedAt = time.Unix(source.entry.ProcessedDate, 0).UTC()
	}
	if path, mtime, ok := findSourceOutputFile(c.baseDir, source.name); ok {
		finding.SourcePath = path
		finding.SourceMTime = mtime
		finding.SourceFileMTime = mtime
	}
	finding.Reason = integrityFindingReason(finding)
	return finding
}

func (c integrityCheck) neverProcessedFinding(name string) (IntegrityFinding, bool) {
	if _, err := os.Stat(sourceEnablePathForRuntime(c.snapshot.runtime, name)); err != nil {
		return IntegrityFinding{}, false
	}
	return IntegrityFinding{
		Feed:         name,
		MissingFiles: []string{name + ".ipset or " + name + ".netset"},
		Reason:       "feed is enabled but has never been successfully processed",
	}, true
}

func (c integrityCheck) missingSourceFinding(source integritySourceContext, processedTime time.Time) (IntegrityFinding, bool) {
	if c.e.shouldSuppressMissingSourceIntegrityWithSnapshot(c.snapshot, source.name, source.entry, source.src) {
		return IntegrityFinding{}, false
	}
	return IntegrityFinding{
		Feed:         source.name,
		SourcePath:   "",
		ProcessedAt:  processedTime,
		MissingFiles: []string{source.name + ".ipset or " + source.name + ".netset"},
		Reason:       "committed canonical feed body missing (cache says processed)",
	}, true
}

func (c integrityCheck) secondaryArtifactFinding(ctx context.Context, source integritySourceContext, processedTime time.Time, sourcePath string, sourceMTime time.Time) (IntegrityFinding, bool, error) {
	finding := IntegrityFinding{
		Feed:            source.name,
		SourcePath:      sourcePath,
		SourceMTime:     sourceMTime,
		SourceFileMTime: sourceMTime,
		ProcessedAt:     processedTime,
	}
	artifactByRelPath, err := c.scanSecondaryArtifacts(ctx, &finding, source, processedTime)
	if err != nil {
		return IntegrityFinding{}, false, err
	}
	finding.BlockedFeeds = appendUniqueStrings(finding.BlockedFeeds, c.e.integrityBlockedBogonProviderArtifactsWithSnapshot(c.snapshot, finding, artifactByRelPath))
	if integrityFindingClean(finding) {
		return IntegrityFinding{}, false, nil
	}
	finalizeIntegrityFinding(&finding)
	return finding, true, nil
}

func (c integrityCheck) scanSecondaryArtifacts(ctx context.Context, finding *IntegrityFinding, source integritySourceContext, processedTime time.Time) (map[string]secondaryArtifactDescriptor, error) {
	artifacts := c.e.expectedSecondaryArtifactsWithSnapshot(c.snapshot, source.name)
	artifactByRelPath := make(map[string]secondaryArtifactDescriptor, len(artifacts))
	for _, artifact := range artifacts {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		artifactByRelPath[artifact.RelPath] = artifact
		c.scanSecondaryArtifact(finding, source, artifact, processedTime)
	}
	return artifactByRelPath, nil
}

func (c integrityCheck) scanSecondaryArtifact(finding *IntegrityFinding, source integritySourceContext, artifact secondaryArtifactDescriptor, processedTime time.Time) {
	path := filepath.Join(c.webDir, artifact.RelPath)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			finding.MissingFiles = append(finding.MissingFiles, artifact.RelPath)
			return
		}
		finding.StaleFiles = append(finding.StaleFiles, artifact.RelPath)
		return
	}
	if info.ModTime().UTC().Before(processedTime) {
		finding.StaleFiles = append(finding.StaleFiles, artifact.RelPath)
		return
	}
	if err := validateStructuredSecondaryArtifactWithSnapshot(artifact, path, c.e, c.snapshot); err != nil {
		finding.MalformedFiles = append(finding.MalformedFiles, artifact.RelPath)
		return
	}
	if artifact.Kind == secondaryArtifactMetadata && validatePublicMetadataArtifactPolicy(path, isRedistributableForConfig(c.snapshot.cfg, source.name) && source.health.Class != feedhealth.ClassArchived) != nil {
		finding.MalformedFiles = append(finding.MalformedFiles, artifact.RelPath)
	}
}

func integrityFindingClean(finding IntegrityFinding) bool {
	return len(finding.MissingFiles) == 0 &&
		len(finding.StaleFiles) == 0 &&
		len(finding.MalformedFiles) == 0 &&
		len(finding.BlockedFeeds) == 0
}

func finalizeIntegrityFinding(finding *IntegrityFinding) {
	finding.Reason = integrityFindingReason(*finding)
	slices.Sort(finding.MissingFiles)
	slices.Sort(finding.StaleFiles)
	slices.Sort(finding.MalformedFiles)
	slices.Sort(finding.BlockedFeeds)
}
