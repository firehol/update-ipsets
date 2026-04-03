package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

// integrityInFlightTolerance is the minimum age of a feed's last
// ProcessedDate before the integrity check considers it settled. A
// source that finished processing in the last N seconds is almost
// certainly still in the middle of the heavy block's fan-out — the
// processed set lands first, then geo/ASN/bogon/comparison/insights
// writers fan out over the next few seconds. Flagging during that
// window would produce a stream of transient false positives on
// every scheduler tick. Feeds whose ProcessedDate is older than
// this tolerance AND whose secondaries are older than it are
// genuinely stuck.
const integrityInFlightTolerance = 60 * time.Second

// IntegrityFinding describes one feed whose on-disk outputs are
// inconsistent with its last successful processing. The check
// produces one finding per affected feed, listing every missing or
// stale secondary file so operators can see exactly what broke.
//
// Semantics: a feed is "clean" when it has a cache entry with a
// non-zero ProcessedDate (meaning it was successfully processed at
// least once) AND every secondary JSON / CSV under WebDir (geo,
// asn, bogon, comparison, insights, metadata, history) exists AND
// is at least as recent as ProcessedDate.
//
// The reference timestamp is cache.Entry.ProcessedDate, not the
// committed canonical feed body's mtime. File mtimes on .ipset/.netset are
// taken from the upstream HTTP Last-Modified header (so the source
// file is always in sync with what the remote reports), and many
// feeds publish a Last-Modified that is in the future relative to
// when we actually processed the body (server clock skew, or
// forward-stamped publication headers). Trusting the file mtime
// would report every such feed as having stale secondaries forever,
// because the secondaries are stamped with the wall-clock time of
// the fan-out run (always < the feed's Last-Modified header).
// ProcessedDate is our own authoritative "we ran it at T" and is
// the only timestamp that can safely be compared against secondary
// mtimes.
type IntegrityFinding struct {
	Feed       string `json:"feed"`
	SourcePath string `json:"source_path"`
	// SourceMTime is the historical JSON field. It now carries the
	// actual committed canonical feed body mtime; SourceFileMTime is the clearer
	// alias new clients should read.
	SourceMTime     time.Time               `json:"source_mtime"`
	SourceFileMTime time.Time               `json:"source_file_mtime"`
	ProcessedAt     time.Time               `json:"processed_at"`
	MissingFiles    []string                `json:"missing_files,omitempty"`
	StaleFiles      []string                `json:"stale_files,omitempty"`
	MalformedFiles  []string                `json:"malformed_files,omitempty"`
	BlockedFeeds    []string                `json:"blocked_feeds,omitempty"`
	RecoveryAction  IntegrityRecoveryAction `json:"recovery_action,omitempty"`
	RecoveryTargets []string                `json:"recovery_targets,omitempty"`
	Reason          string                  `json:"reason"`
}

type IntegrityRecoveryAction string

const (
	IntegrityRecoveryActionRecheck   IntegrityRecoveryAction = "recheck"
	IntegrityRecoveryActionReprocess IntegrityRecoveryAction = "reprocess"
)

type IntegrityOptions struct {
	IncludeArchived bool
	EnableAll       bool
	WebDir          string
}

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
	if e == nil || e.cfg == nil {
		return nil
	}
	webDir := opts.WebDir
	if webDir == "" {
		webDir = e.runtime.WebDir
	}
	baseDir := e.runtime.BaseDir
	if webDir == "" || baseDir == "" {
		return nil
	}

	var findings []IntegrityFinding
	resolver := newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries())
	for _, name := range config.SortedSourceNames(e.cfg) {
		src := e.cfg.Sources[name]
		if src == nil || src.Hidden {
			continue
		}
		// Database sources produce a binary .mmdb / .csv, not an
		// ipset; their secondary layout is different and they are
		// checked elsewhere (the public API walks them separately).
		if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
			continue
		}
		health := e.classifyEffectiveEntryHealth(name, resolver.entryFromSnapshot(name))
		if health.Class == feedhealth.ClassArchived && !opts.IncludeArchived {
			continue
		}

		// ProcessedDate is our reference: it's the wall-clock time
		// at which finalize() last ran for this feed. Secondary
		// files written by the heavy block's fan-out are stamped
		// with the wall-clock time of that same run, so anything
		// strictly earlier than ProcessedDate is stale.
		entry := e.state.Entry(name)
		if blockedFeeds := e.integrityBlockedFeeds(name, src, resolver, opts.EnableAll); len(blockedFeeds) > 0 {
			finding := IntegrityFinding{
				Feed:         name,
				BlockedFeeds: blockedFeeds,
			}
			if entry != nil && entry.ProcessedDate > 0 {
				finding.ProcessedAt = time.Unix(entry.ProcessedDate, 0).UTC()
			}
			if path, mtime, ok := findSourceOutputFile(baseDir, name); ok {
				finding.SourcePath = path
				finding.SourceMTime = mtime
				finding.SourceFileMTime = mtime
			}
			finding.Reason = integrityFindingReason(finding)
			findings = append(findings, finding)
			continue
		}
		if entry == nil || entry.ProcessedDate == 0 {
			// Never successfully processed. If the enable marker
			// exists the feed is enabled but broken — flag it. If
			// there's no marker either, the feed is disabled and
			// silence is correct.
			triggerPath := e.sourceEnablePath(name)
			if _, err := os.Stat(triggerPath); err != nil {
				continue
			}
			findings = append(findings, IntegrityFinding{
				Feed:         name,
				MissingFiles: []string{name + ".ipset or " + name + ".netset"},
				Reason:       "feed is enabled but has never been successfully processed",
			})
			continue
		}
		processedTime := time.Unix(entry.ProcessedDate, 0).UTC()

		sourcePath, sourceMTime, sourceOK := findSourceOutputFile(baseDir, name)
		if !sourceOK {
			if e.shouldSuppressMissingSourceIntegrity(name, entry, src) {
				continue
			}
			// Cache says processed, but the committed canonical feed body is gone.
			// That's a real inconsistency regardless of trigger
			// state, and reprocess is the right remedy.
			findings = append(findings, IntegrityFinding{
				Feed:         name,
				SourcePath:   "",
				ProcessedAt:  processedTime,
				MissingFiles: []string{name + ".ipset or " + name + ".netset"},
				Reason:       "committed canonical feed body missing (cache says processed)",
			})
			continue
		}

		// In-flight tolerance: a feed that finished processing in
		// the last few seconds is almost certainly still in the
		// middle of the heavy block's fan-out. Skip comparison so
		// we do not flag every tick's fresh updates during the
		// few-second window between processSource and the geo /
		// ASN / bogon / insights writers.
		if e.now().UTC().Sub(processedTime) < integrityInFlightTolerance {
			continue
		}

		if finding, ok := e.integrityHistoryDerivativeFinding(name, src, processedTime, sourcePath, sourceMTime); ok {
			findings = append(findings, finding)
			continue
		}

		finding := IntegrityFinding{
			Feed:            name,
			SourcePath:      sourcePath,
			SourceMTime:     sourceMTime,
			SourceFileMTime: sourceMTime,
			ProcessedAt:     processedTime,
		}
		artifacts := e.expectedSecondaryArtifacts(name)
		artifactByRelPath := make(map[string]secondaryArtifactDescriptor, len(artifacts))
		for _, artifact := range artifacts {
			artifactByRelPath[artifact.RelPath] = artifact
			path := filepath.Join(webDir, artifact.RelPath)
			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					finding.MissingFiles = append(finding.MissingFiles, artifact.RelPath)
					continue
				}
				// Stat errors on the web tree (permissions, I/O)
				// get filed as "stale" so the operator sees them
				// in the same list as mtime violations — there is
				// no clean disposition for a file we can't read.
				finding.StaleFiles = append(finding.StaleFiles, artifact.RelPath)
				continue
			}
			if info.ModTime().UTC().Before(processedTime) {
				finding.StaleFiles = append(finding.StaleFiles, artifact.RelPath)
				continue
			}
			if err := validateStructuredSecondaryArtifact(artifact, path, e); err != nil {
				finding.MalformedFiles = append(finding.MalformedFiles, artifact.RelPath)
				continue
			}
			if artifact.Kind == secondaryArtifactMetadata && validatePublicMetadataArtifactPolicy(path, e.isRedistributable(name) && health.Class != feedhealth.ClassArchived) != nil {
				finding.MalformedFiles = append(finding.MalformedFiles, artifact.RelPath)
			}
		}
		finding.BlockedFeeds = appendUniqueStrings(finding.BlockedFeeds, e.integrityBlockedBogonProviderArtifacts(finding, artifactByRelPath))
		if len(finding.MissingFiles) == 0 && len(finding.StaleFiles) == 0 && len(finding.MalformedFiles) == 0 && len(finding.BlockedFeeds) == 0 {
			continue
		}
		finding.Reason = integrityFindingReason(finding)
		// Sort the file lists for deterministic output.
		slices.Sort(finding.MissingFiles)
		slices.Sort(finding.StaleFiles)
		slices.Sort(finding.MalformedFiles)
		slices.Sort(finding.BlockedFeeds)
		findings = append(findings, finding)
	}
	return findings
}

func integrityFindingReason(f IntegrityFinding) string {
	var parts []string
	if len(f.MissingFiles) > 0 {
		parts = append(parts, "missing")
	}
	if len(f.StaleFiles) > 0 {
		parts = append(parts, "stale")
	}
	if len(f.MalformedFiles) > 0 {
		parts = append(parts, "malformed")
	}
	if len(f.BlockedFeeds) > 0 {
		parts = append(parts, "blocked")
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 && parts[0] == "blocked" {
		return "blocked by unavailable input feeds: " + strings.Join(f.BlockedFeeds, ", ")
	}
	if len(parts) == 1 {
		switch parts[0] {
		case "missing":
			return "missing secondary files"
		case "stale":
			return "stale secondary files (older than last processing)"
		case "malformed":
			return "malformed secondary files"
		}
	}
	return joinReasonParts(parts) + " secondary files"
}

func joinReasonParts(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		head := strings.Join(parts[:len(parts)-1], ", ")
		return head + ", and " + parts[len(parts)-1]
	}
}

func (e *Engine) integrityBlockedFeeds(name string, src *config.Source, resolver *effectiveEntryResolver, enableAll bool) []string {
	if e == nil || e.cfg == nil || src == nil {
		return nil
	}
	if src.Provenance != config.ProvenanceSecondaryMerge {
		return nil
	}
	composition := e.mergeCompositionWithResolver(src, enableAll, resolver)
	if composition.eligibleSourceCount == 0 {
		return nil
	}
	blocked := append([]string(nil), composition.missingFeedBodies...)
	blocked = appendUniqueStrings(blocked, composition.unavailableSubtractiveFeeds)
	slices.Sort(blocked)
	return blocked
}

func (e *Engine) integrityBlockedBogonProviderArtifacts(finding IntegrityFinding, artifacts map[string]secondaryArtifactDescriptor) []string {
	if e == nil || e.cfg == nil {
		return nil
	}
	relPaths := make([]string, 0, len(finding.MissingFiles)+len(finding.StaleFiles)+len(finding.MalformedFiles))
	relPaths = append(relPaths, finding.MissingFiles...)
	relPaths = append(relPaths, finding.StaleFiles...)
	relPaths = append(relPaths, finding.MalformedFiles...)
	if len(relPaths) == 0 {
		return nil
	}

	// The bogon provider list is intentionally small today. If many
	// merge-derived bogon providers are added later, cache this suffix scan per
	// integrity run instead of doing it per finding.
	blocked := map[string]struct{}{}
	for _, relPath := range relPaths {
		artifact, ok := artifacts[relPath]
		if !ok {
			continue
		}
		if artifact.Kind != secondaryArtifactBogons || artifact.Provider == "" {
			continue
		}
		provider := e.cfg.Sources[artifact.Provider]
		if provider == nil || provider.Provenance != config.ProvenanceSecondaryMerge {
			continue
		}
		if !e.hasUsableSet(provider.Name) {
			blocked[provider.Name] = struct{}{}
		}
	}
	return sortedNames(blocked)
}

func appendUniqueStrings(dst []string, values []string) []string {
	if len(values) == 0 {
		return dst
	}
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}

func (e *Engine) integrityHistoryDerivativeFinding(name string, src *config.Source, processedTime time.Time, sourcePath string, sourceMTime time.Time) (IntegrityFinding, bool) {
	if e == nil || e.cfg == nil || src == nil || src.Provenance != config.ProvenanceSecondaryRetention || len(src.DerivedFrom) == 0 {
		return IntegrityFinding{}, false
	}
	parent := src.DerivedFrom[0]
	missing := make([]string, 0)
	parentPath := latestFeedBodyPath(e.feedBodyPath(parent))
	if !fileExists(parentPath) {
		missing = append(missing, filepath.Join("base", filepath.Base(e.feedBodyPath(parent))))
	} else {
		window := e.historyDerivativeWindowDuration(src)
		referenceTime := historyDerivativeReferenceTime(parentPath, e.now().UTC())
		cutoff := referenceTime.Add(-window)
		expectedCurrent := filepath.Join("history", parent, fmt.Sprintf("%d.set", referenceTime.Unix()))
		snapshots, err := readHistorySnapshots(filepath.Join(e.runtime.HistoryDir, parent))
		if err != nil && !os.IsNotExist(err) {
			missing = append(missing, filepath.Join("history", parent)+" (unreadable)")
		} else {
			usable := 0
			for _, snapshot := range snapshots {
				if !historySnapshotWithinWindow(snapshot, cutoff) {
					continue
				}
				fileSet, err := iprange.OpenFileSet(snapshot.path)
				if err != nil {
					missing = append(missing, filepath.Join("history", parent, snapshot.name)+" (corrupt)")
					continue
				}
				if err := fileSet.Close(); err != nil {
					missing = append(missing, filepath.Join("history", parent, snapshot.name)+" (unreadable)")
					continue
				}
				usable++
			}
			if usable == 0 {
				missing = append(missing, expectedCurrent)
			}
		}
	}
	if len(missing) == 0 {
		return IntegrityFinding{}, false
	}
	slices.Sort(missing)
	return IntegrityFinding{
		Feed:            name,
		SourcePath:      sourcePath,
		SourceMTime:     sourceMTime,
		SourceFileMTime: sourceMTime,
		ProcessedAt:     processedTime,
		MissingFiles:    missing,
		BlockedFeeds:    []string{parent},
		Reason:          "missing or corrupt downloader-owned history snapshots; parent recheck required",
	}, true
}

func (e *Engine) shouldSuppressMissingSourceIntegrity(name string, entry *cache.Entry, src *config.Source) bool {
	if e == nil || e.cfg == nil || entry == nil || src == nil {
		return false
	}
	if e.integrityHasCommittedOrStagedSource(name) {
		return false
	}
	health := feedhealth.Classify(entry, src, feedhealth.PolicyFromRuntime(e.cfg.Runtime), e.now().UTC())
	return health.Class == feedhealth.ClassUnavailable
}

// findSourceOutputFile locates the current committed canonical feed body
// (.netset preferred over .ipset, matching the legacy processMerge
// precedence and the merge provider's file-picking order). Returns
// ok=false when neither file exists.
func findSourceOutputFile(baseDir, name string) (string, time.Time, bool) {
	for _, suffix := range []string{".netset", ".ipset"} {
		path := filepath.Join(baseDir, name+suffix)
		if info, err := os.Stat(path); err == nil {
			return path, info.ModTime().UTC(), true
		}
	}
	return "", time.Time{}, false
}

func (e *Engine) expectedSecondaryArtifacts(name string) []secondaryArtifactDescriptor {
	artifacts := []secondaryArtifactDescriptor{
		{RelPath: name + ".json", Kind: secondaryArtifactMetadata, Feed: name},
		{RelPath: name + "_history.csv", Kind: secondaryArtifactHistory, Feed: name},
		{RelPath: name + "_changesets.csv", Kind: secondaryArtifactChangesets, Feed: name},
		{RelPath: name + "_retention.json", Kind: secondaryArtifactRetention, Feed: name},
		{RelPath: name + "_comparison.json", Kind: secondaryArtifactComparison, Feed: name},
		{RelPath: name + "_insights.json", Kind: secondaryArtifactInsights, Feed: name},
	}
	if e.cfg == nil {
		return artifacts
	}
	for _, p := range e.cfg.SourcesWithUse(config.UseGeoIP) {
		artifacts = append(artifacts, secondaryArtifactDescriptor{
			RelPath:  name + "_" + p.Name + ".json",
			Kind:     secondaryArtifactGeo,
			Feed:     name,
			Provider: p.Name,
		})
	}
	for _, p := range e.cfg.SourcesWithUse(config.UseASN) {
		artifacts = append(artifacts, secondaryArtifactDescriptor{
			RelPath:  name + "_asn_" + p.Name + ".json",
			Kind:     secondaryArtifactASN,
			Feed:     name,
			Provider: p.Name,
		})
	}
	for _, p := range e.cfg.SourcesWithUse(config.UseBogons) {
		artifacts = append(artifacts, secondaryArtifactDescriptor{
			RelPath:  name + "_bogons_" + p.Name + ".json",
			Kind:     secondaryArtifactBogons,
			Feed:     name,
			Provider: p.Name,
		})
	}
	criticalProviders := e.cfg.SourcesWithUse(config.UseCriticalInfrastructure)
	if len(criticalProviders) > 0 && !isCriticalInfrastructureOutputName(e.cfg, name) && isCriticalInfrastructureComparableName(e.cfg, name) {
		artifacts = append(artifacts, secondaryArtifactDescriptor{
			RelPath: name + "_critical_infrastructure.json",
			Kind:    secondaryArtifactCriticalAggregate,
			Feed:    name,
		})
		for _, p := range criticalProviders {
			if !e.hasMaterializedLatestSetFile(p.Name) {
				continue
			}
			artifacts = append(artifacts, secondaryArtifactDescriptor{
				RelPath:  name + "_critical_" + p.Name + ".json",
				Kind:     secondaryArtifactCriticalProvider,
				Feed:     name,
				Provider: p.Name,
			})
		}
	}
	return artifacts
}

// expectedSecondaryFiles returns the relative paths (under
// WebDir) of every per-feed file the heavy block should produce
// for the given source name. Must match what writeMetadataFiles +
// writeCountryComparisonFiles + writeASNComparisonFiles +
// writeBogonComparisonFiles + writeCriticalInfrastructureFiles +
// writeInsightsForFeeds actually
// write — any divergence here makes CheckIntegrity either falsely
// flag intact feeds or silently skip real breakage.
//
// Derivative sources (internal://*) produce exactly the same
// secondary files as plain sources. The pipeline treats them
// uniformly and so does this function.
func (e *Engine) expectedSecondaryFiles(name string) []string {
	artifacts := e.expectedSecondaryArtifacts(name)
	files := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		files = append(files, artifact.RelPath)
	}
	return files
}

func (e *Engine) hasMaterializedLatestSetFile(name string) bool {
	if e == nil {
		return false
	}
	if e.hasBinaryLatestSet(name) {
		return true
	}
	entry := e.EntrySnapshot(name)
	if entry == nil || entry.File == "" || !rawFeedFileMatches(name, entry.File) {
		return false
	}
	path, ok := safeRuntimeFilePath(e.runtime.BaseDir, entry.File)
	return ok && fileExists(path)
}
