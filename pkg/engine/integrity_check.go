package engine

import (
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
	check, ok := e.newIntegrityCheck(opts)
	if !ok {
		return nil
	}
	return check.findings()
}

type integrityCheck struct {
	e        *Engine
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
	if e == nil || e.cfg == nil {
		return integrityCheck{}, false
	}
	webDir := opts.WebDir
	if webDir == "" {
		webDir = e.runtime.WebDir
	}
	baseDir := e.runtime.BaseDir
	if webDir == "" || baseDir == "" {
		return integrityCheck{}, false
	}
	return integrityCheck{
		e:        e,
		opts:     opts,
		webDir:   webDir,
		baseDir:  baseDir,
		resolver: newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries()),
	}, true
}

func (c integrityCheck) findings() []IntegrityFinding {
	var findings []IntegrityFinding
	for _, name := range config.SortedSourceNames(c.e.cfg) {
		source, ok := c.sourceContext(name)
		if !ok {
			continue
		}
		finding, ok := c.findingForSource(source)
		if ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func (c integrityCheck) sourceContext(name string) (integritySourceContext, bool) {
	src := c.e.cfg.Sources[name]
	if src == nil || src.Hidden {
		return integritySourceContext{}, false
	}
	if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
		return integritySourceContext{}, false
	}
	health := c.e.classifyEffectiveEntryHealth(name, c.resolver.entryFromSnapshot(name))
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

func (c integrityCheck) findingForSource(source integritySourceContext) (IntegrityFinding, bool) {
	if blockedFeeds := c.e.integrityBlockedFeeds(source.name, source.src, c.resolver, c.opts.EnableAll); len(blockedFeeds) > 0 {
		return c.blockedFeedFinding(source, blockedFeeds), true
	}
	if source.entry == nil || source.entry.ProcessedDate == 0 {
		return c.neverProcessedFinding(source.name)
	}

	processedTime := time.Unix(source.entry.ProcessedDate, 0).UTC()
	sourcePath, sourceMTime, sourceOK := findSourceOutputFile(c.baseDir, source.name)
	if !sourceOK {
		return c.missingSourceFinding(source, processedTime)
	}
	if c.e.now().UTC().Sub(processedTime) < integrityInFlightTolerance {
		return IntegrityFinding{}, false
	}
	if finding, ok := c.e.integrityHistoryDerivativeFinding(source.name, source.src, processedTime, sourcePath, sourceMTime); ok {
		return finding, true
	}
	return c.secondaryArtifactFinding(source, processedTime, sourcePath, sourceMTime)
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
	if _, err := os.Stat(c.e.sourceEnablePath(name)); err != nil {
		return IntegrityFinding{}, false
	}
	return IntegrityFinding{
		Feed:         name,
		MissingFiles: []string{name + ".ipset or " + name + ".netset"},
		Reason:       "feed is enabled but has never been successfully processed",
	}, true
}

func (c integrityCheck) missingSourceFinding(source integritySourceContext, processedTime time.Time) (IntegrityFinding, bool) {
	if c.e.shouldSuppressMissingSourceIntegrity(source.name, source.entry, source.src) {
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

func (c integrityCheck) secondaryArtifactFinding(source integritySourceContext, processedTime time.Time, sourcePath string, sourceMTime time.Time) (IntegrityFinding, bool) {
	finding := IntegrityFinding{
		Feed:            source.name,
		SourcePath:      sourcePath,
		SourceMTime:     sourceMTime,
		SourceFileMTime: sourceMTime,
		ProcessedAt:     processedTime,
	}
	artifactByRelPath := c.scanSecondaryArtifacts(&finding, source, processedTime)
	finding.BlockedFeeds = appendUniqueStrings(finding.BlockedFeeds, c.e.integrityBlockedBogonProviderArtifacts(finding, artifactByRelPath))
	if integrityFindingClean(finding) {
		return IntegrityFinding{}, false
	}
	finalizeIntegrityFinding(&finding)
	return finding, true
}

func (c integrityCheck) scanSecondaryArtifacts(finding *IntegrityFinding, source integritySourceContext, processedTime time.Time) map[string]secondaryArtifactDescriptor {
	artifacts := c.e.expectedSecondaryArtifacts(source.name)
	artifactByRelPath := make(map[string]secondaryArtifactDescriptor, len(artifacts))
	for _, artifact := range artifacts {
		artifactByRelPath[artifact.RelPath] = artifact
		c.scanSecondaryArtifact(finding, source, artifact, processedTime)
	}
	return artifactByRelPath
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
	if err := validateStructuredSecondaryArtifact(artifact, path, c.e); err != nil {
		finding.MalformedFiles = append(finding.MalformedFiles, artifact.RelPath)
		return
	}
	if artifact.Kind == secondaryArtifactMetadata && validatePublicMetadataArtifactPolicy(path, c.e.isRedistributable(source.name) && source.health.Class != feedhealth.ClassArchived) != nil {
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
