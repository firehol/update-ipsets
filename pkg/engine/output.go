package engine

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/enrichment"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

var markdownLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)

// setMetadata matches the per-ipset JSON schema produced by the bash
// script. All fields are always present (no omitempty) because the
// bash template unconditionally includes every field, even when empty.
//
// The trailing block (UsedFor through Format) extends the schema with
// fields the Phase 3 frontend reads but the bash template never knew
// about. They are emitted with omitempty so a missing source-config
// entry produces no extra noise on the disk file.
type setMetadata struct {
	Name             string                    `json:"name"`
	Entries          int                       `json:"entries"`
	EntriesMin       int                       `json:"entries_min"`
	EntriesMax       int                       `json:"entries_max"`
	IPs              uint64                    `json:"ips"`
	IPsMin           uint64                    `json:"ips_min"`
	IPsMax           uint64                    `json:"ips_max"`
	IPV              string                    `json:"ipv"`
	Hash             string                    `json:"hash"`
	Frequency        int                       `json:"frequency"`
	Aggregation      int                       `json:"aggregation"`
	Started          int64                     `json:"started"`
	Updated          int64                     `json:"updated"`
	Processed        int64                     `json:"processed"`
	Checked          int64                     `json:"checked"`
	ClockSkew        int64                     `json:"clock_skew"`
	Category         string                    `json:"category"`
	Provenance       string                    `json:"provenance,omitempty"`
	Maintainer       string                    `json:"maintainer"`
	MaintainerURL    string                    `json:"maintainer_url"`
	License          string                    `json:"license,omitempty"`
	Attribution      string                    `json:"attribution,omitempty"`
	OfficialName     string                    `json:"official_name,omitempty"`
	ShortDescription string                    `json:"short_description,omitempty"`
	CurrentStatus    *enrichment.CurrentStatus `json:"current_status,omitempty"`
	Enrichment       *enrichment.Feed          `json:"enrichment,omitempty"`
	Info             string                    `json:"info"`
	Source           string                    `json:"source"`
	File             string                    `json:"file"`
	History          string                    `json:"history"`
	// Geo holds the per-feed-per-geo-provider country JSON file paths,
	// keyed by the source name (e.g. "geolite2_country" →
	// "dshield_geolite2_country.json"). Walked by the frontend in
	// /api/v1/sets/{name}/countries order; entries are only present
	// when the file actually exists on disk. Replaces the legacy
	// hardcoded geolite2/ipdeny/ip2location/ipip/dbip top-level fields
	// — adding a new geo provider is now a YAML-only operation.
	Geo                map[string]string   `json:"geo,omitempty"`
	Comparison         string              `json:"comparison"`
	FileLocal          string              `json:"file_local"`
	CommitHistory      string              `json:"commit_history"`
	Errors             int                 `json:"errors"`
	Version            int                 `json:"version"`
	AverageUpdate      int                 `json:"average_update"`
	MinUpdate          int                 `json:"min_update"`
	MaxUpdate          int                 `json:"max_update"`
	RotationMedian     float64             `json:"rotation_median_pct,omitempty"`
	RotationP75        float64             `json:"rotation_p75_pct,omitempty"`
	RotationSamples    int                 `json:"rotation_samples,omitempty"`
	ChangeRatioMedian  float64             `json:"change_ratio_median_pct,omitempty"`
	ChangeRatioP75     float64             `json:"change_ratio_p75_pct,omitempty"`
	ChangeRatioSamples int                 `json:"change_ratio_samples,omitempty"`
	Health             feedhealth.Snapshot `json:"health"`
	Downloader         string              `json:"downloader"`
	UsedFor            []string            `json:"used_for,omitempty"`
	Hidden             bool                `json:"hidden,omitempty"`
	MergeIncluded      []MergeInputState   `json:"merge_included,omitempty"`
	MergeSubtracted    []MergeInputState   `json:"merge_subtracted,omitempty"`
	MergeExcluded      []MergeInputState   `json:"merge_excluded,omitempty"`
	Processor          string              `json:"processor,omitempty"`
	PreProcessor       string              `json:"pre_processor,omitempty"`
	DontRedistribute   bool                `json:"dont_redistribute,omitempty"`
	Format             string              `json:"format,omitempty"`
	Output             string              `json:"output,omitempty"`
}

// allIPSetsItem matches the all-ipsets.json item schema. All fields
// are always present (no omitempty) to match bash output.
type allIPSetsItem struct {
	IPSet      string `json:"ipset"`
	Category   string `json:"category"`
	Maintainer string `json:"maintainer"`
	Started    int64  `json:"started"`
	Updated    int64  `json:"updated"`
	Checked    int64  `json:"checked"`
	ClockSkew  int64  `json:"clock_skew"`
	IPs        uint64 `json:"ips"`
	Errors     int    `json:"errors"`
}

// Metadata builds the public per-feed metadata payload for name, the
// same shape that is written to <name>.json on disk. Used by the public
// API setHandler so the cache fallback path returns the same fields the
// frontend already reads from the static file (eliminating the
// per-field normalization shim that grew up around the divergence).
//
// Returns an error if the feed is unknown or hidden.
func (e *Engine) Metadata(name string) (any, error) {
	return e.MetadataWithEnableAll(name, false)
}

func (e *Engine) MetadataWithEnableAll(name string, enableAll bool) (any, error) {
	entry, err := e.Entry(name)
	if err != nil {
		return nil, err
	}
	return e.buildSetMetadataWithEnableAll(name, entry, enableAll), nil
}

func (e *Engine) buildSetMetadataWithEnableAll(name string, entry *cache.Entry, enableAll bool) setMetadata {
	return e.buildSetMetadataInDirWithEnableAll(name, entry, e.outputDir(), enableAll)
}

func (e *Engine) buildSetMetadataInDirWithEnableAll(name string, entry *cache.Entry, outDir string, enableAll bool) setMetadata {
	return e.buildSetMetadataFromEffectiveEntryInDir(name, e.entryViewFromFreshStateSnapshot(name, entry), outDir, enableAll)
}

func (e *Engine) buildSetMetadataFromEffectiveEntryInDir(name string, entry *cache.Entry, outDir string, enableAll bool) setMetadata {
	return e.buildSetMetadataFromEffectiveEntryInDirWithResolver(name, entry, outDir, enableAll, nil)
}

func (e *Engine) buildSetMetadataFromEffectiveEntryInDirWithResolver(name string, entry *cache.Entry, outDir string, enableAll bool, resolver *effectiveEntryResolver) setMetadata {
	// Bash per-ipset JSON uses IPSET_CHECKED_DATE directly without
	// the max comparison (that's only done for all-ipsets.json).
	redistributable := e.isRedistributable(name)
	category := entry.Category
	if src := e.lookupSource(name); src != nil && src.Category != "" {
		category = src.Category
	}
	meta := setMetadata{
		Name:          name,
		Entries:       entry.Entries,
		EntriesMin:    entry.EntriesMin,
		EntriesMax:    entry.EntriesMax,
		IPs:           entry.UniqueIPs,
		IPsMin:        entry.IPsMin,
		IPsMax:        entry.IPsMax,
		IPV:           entry.IPV,
		Hash:          entry.Hash,
		Frequency:     entry.FrequencyMinutes,
		Aggregation:   aggregationMinutesFromName(name),
		Started:       millis(entry.StartedDate),
		Updated:       millis(entry.SourceDate),
		Processed:     millis(entry.ProcessedDate),
		Checked:       millis(entry.CheckedDate),
		ClockSkew:     millis(entry.ClockSkewSeconds),
		Category:      category,
		Maintainer:    entry.Maintainer,
		MaintainerURL: entry.MaintainerURL,
		License:       entry.License,
		Attribution:   entry.Attribution,
		Info:          markdownLinksToHTML(entry.Info),
		File:          entry.File,
		// Bash always sets "history": "<name>_history.csv" unconditionally.
		History:            name + "_history.csv",
		Errors:             entry.DownloadFailures,
		Version:            entry.Version,
		AverageUpdate:      entry.AverageUpdateMins,
		MinUpdate:          entry.MinUpdateMins,
		MaxUpdate:          entry.MaxUpdateMins,
		RotationMedian:     entry.RotationMedianPct,
		RotationP75:        entry.RotationP75Pct,
		RotationSamples:    entry.RotationSamples,
		ChangeRatioMedian:  entry.ChangeRatioMedianPct,
		ChangeRatioP75:     entry.ChangeRatioP75Pct,
		ChangeRatioSamples: entry.ChangeRatioSamples,
		Health:             e.classifyEffectiveEntryHealth(name, entry),
		Downloader:         entry.Downloader,
	}
	// License + attribution are config-time facts, not runtime state.
	// finalize.go copies them into the cache.Entry on every successful
	// processing run, but cached entries written before that change
	// landed have empty values until the next refresh. Fall back to
	// the live config so users see the right answer immediately —
	// editing the YAML and restarting the daemon updates the spec page
	// without waiting for every feed to re-process.
	if e.cfg != nil {
		if src := e.cfg.SourceByName(name); src != nil {
			if meta.License == "" {
				meta.License = src.License
			}
			if meta.Attribution == "" {
				meta.Attribution = src.Attribution
			}
		}
	}
	if redistributable {
		meta.Source = entry.PublicURL
	}
	if fileExists(filepath.Join(outDir, name+"_comparison.json")) || fileExists(filepath.Join(e.outputDir(), name+"_comparison.json")) {
		meta.Comparison = name + "_comparison.json"
	}
	// Geo file fan-out: walk every configured geolocation source in
	// YAML order and record the per-feed JSON file path when it
	// exists on disk. The map is keyed by the source name so the
	// frontend can correlate entries with /api/v1/sets/{name}/countries
	// without any frontend-side knowledge of which providers exist.
	if e.cfg != nil {
		for _, src := range e.cfg.SourcesWithUse(config.UseGeoIP) {
			file := name + "_" + src.Name + ".json"
			if fileExists(filepath.Join(outDir, file)) || fileExists(filepath.Join(e.outputDir(), file)) {
				if meta.Geo == nil {
					meta.Geo = make(map[string]string)
				}
				meta.Geo[src.Name] = file
			}
		}
	}
	if redistributable && entry.File != "" && e.runtime.LocalCopyURL != "" {
		meta.FileLocal = strings.TrimRight(e.runtime.LocalCopyURL, "/") + "/" + entry.File
	}
	if redistributable && entry.File != "" && e.runtime.GitHubChangesURL != "" {
		meta.CommitHistory = strings.TrimRight(e.runtime.GitHubChangesURL, "/") + "/" + entry.File
	}
	// Phase 3 spec/provenance fields. They live on the source config,
	// not the cache entry, so we look them up here. The lookup must
	// also handle split _ip/_net derivative names that resolve to a
	// shared parent source.
	if src := e.lookupSource(name); src != nil {
		meta.Provenance = string(publicProvenance(src))
		if src.Enrichment != nil {
			meta.OfficialName = enrichment.StringValue(src.Enrichment.OfficialName)
			meta.ShortDescription = enrichment.StringValue(src.Enrichment.ShortDescription)
			meta.CurrentStatus = &src.Enrichment.CurrentStatus
			meta.Enrichment = src.Enrichment
		}
		if len(src.Use) > 0 {
			meta.UsedFor = append([]string(nil), src.Use...)
		}
		meta.Hidden = src.Hidden
		meta.Format = src.Format
		meta.Output = src.Output
		meta.Processor = formatProcessorSteps(src.Processor)
		meta.DontRedistribute = !redistributable
		if src.Provenance == config.ProvenanceSecondaryMerge {
			composition := e.mergeCompositionWithResolver(src, enableAll, resolver)
			meta.MergeIncluded = append([]MergeInputState(nil), composition.Included...)
			meta.MergeSubtracted = append([]MergeInputState(nil), composition.Subtracted...)
			meta.MergeExcluded = append([]MergeInputState(nil), composition.Excluded...)
		}
	} else {
		// No source config (merged sets, legacy). Fall back to the
		// engine's redistributability resolver so the dont_redistribute
		// flag still reflects reality.
		meta.DontRedistribute = !redistributable
	}
	if !redistributable || meta.Health.Class == feedhealth.ClassArchived {
		meta.Source = ""
		meta.File = ""
		meta.FileLocal = ""
		meta.CommitHistory = ""
	}
	return meta
}

// lookupSource returns the *config.Source that backs the given output
// name, accounting for split _ip / _net derivatives that share a parent.
// Returns nil if no source matches (merges and unconfigured names).
func (e *Engine) lookupSource(name string) *config.Source {
	if e == nil || e.cfg == nil {
		return nil
	}
	if src := e.cfg.Sources[name]; src != nil {
		return src
	}
	if strings.HasSuffix(name, "_ip") || strings.HasSuffix(name, "_net") {
		base := strings.TrimSuffix(strings.TrimSuffix(name, "_ip"), "_net")
		if src := e.cfg.Sources[base]; src != nil {
			return src
		}
	}
	return nil
}

// formatProcessorSteps renders a Source.Processor pipeline as a single
// human-readable string suitable for the spec/provenance row that the
// Phase 3 page displays. Steps with arguments are rendered as
// "name(k=v,k=v)" so the operator sees the full configuration.
func formatProcessorSteps(steps []config.ProcessorStep) string {
	if len(steps) == 0 {
		return ""
	}
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		if step.Name == "" {
			continue
		}
		if len(step.Args) == 0 {
			parts = append(parts, step.Name)
			continue
		}
		keys := make([]string, 0, len(step.Args))
		for k := range step.Args {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		argParts := make([]string, 0, len(keys))
		for _, k := range keys {
			argParts = append(argParts, k+"="+step.Args[k])
		}
		parts = append(parts, step.Name+"("+strings.Join(argParts, ",")+")")
	}
	return strings.Join(parts, " | ")
}

// writeComparisonFiles computes pairwise overlap between ipsets.
// When updatedNames is non-empty, only those sources are compared against
// all others — the rest keep their existing _comparison.json files on disk.
// When updatedNames is empty, all sources are compared (initial run).
func (e *Engine) writeComparisonFiles(ctx context.Context, updatedNames []string, outDir string, setCache *latestSetCache) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	names := e.publicOutputNames()
	if len(names) < 2 {
		return nil
	}
	if setCache == nil {
		setCache = newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
	}

	type setInfo struct {
		name         string
		ips          uint64
		category     string
		lo           uint32
		hi           uint32
		hasRange     bool
		prefixBitmap *comparisonPrefixBitmap
	}
	prepareStarted := time.Now()
	infos := make([]setInfo, 0, len(names))
	for _, name := range names {
		snap := e.state.EntrySnapshot(name)
		if snap == nil || !e.hasUsableSet(name) {
			continue
		}
		src, err := setCache.Open(name)
		if err != nil {
			e.logger.Warn("comparison skipped: cannot open set", "set", name, "error", err)
			continue
		}
		prefixBitmap := buildComparisonPrefixBitmap(src.RangeSource)
		if ioErr := checkFileSetErr(src.RangeSource, name, e.logger); ioErr != nil {
			continue
		}
		lo, hi, hasRange := rangeBounds(src)
		infos = append(infos, setInfo{
			name:         name,
			ips:          src.UniqueIPs(),
			category:     snap.Category,
			lo:           lo,
			hi:           hi,
			hasRange:     hasRange,
			prefixBitmap: prefixBitmap,
		})
	}
	e.observeRunOperation("metadata.comparison_prepare_sets", time.Since(prepareStarted))
	if len(infos) < 2 {
		return nil
	}

	// Determine which sources need comparison. If updatedNames is
	// provided, only compare those against all others. Otherwise
	// compare everything (initial run or full rebuild).
	updatedSet := make(map[string]struct{}, len(updatedNames))
	for _, name := range updatedNames {
		updatedSet[name] = struct{}{}
	}
	isUpdated := func(name string) bool {
		if len(updatedSet) == 0 {
			return true // full comparison when no filter
		}
		_, ok := updatedSet[name]
		return ok
	}

	type pair struct{ i, j int }
	type pairResult struct {
		i, j   int
		common uint64
	}
	numWorkers := e.runtime.HeavyPhaseWorkers()
	if numWorkers < 1 {
		numWorkers = 1
	}

	pairCh := make(chan pair)
	var pairMu sync.Mutex
	var pairResults []pairResult
	var wg sync.WaitGroup
	var pairCount atomic.Int64
	var pairTotalNS atomic.Int64
	var pairMaxNS atomic.Int64
	var pairSkipped atomic.Int64
	var pairSkippedEmpty atomic.Int64
	var pairSkippedRange atomic.Int64
	var pairSkippedPrefix atomic.Int64

	for range numWorkers {
		wg.Go(func() {
			for p := range pairCh {
				if err := contextErr(ctx); err != nil {
					return
				}
				if infos[p.i].ips == 0 || infos[p.j].ips == 0 {
					pairSkipped.Add(1)
					pairSkippedEmpty.Add(1)
					pairMu.Lock()
					pairResults = append(pairResults, pairResult{i: p.i, j: p.j, common: 0})
					pairMu.Unlock()
					continue
				}
				if infos[p.i].hasRange && infos[p.j].hasRange && (infos[p.i].hi < infos[p.j].lo || infos[p.j].hi < infos[p.i].lo) {
					pairSkipped.Add(1)
					pairSkippedRange.Add(1)
					pairMu.Lock()
					pairResults = append(pairResults, pairResult{i: p.i, j: p.j, common: 0})
					pairMu.Unlock()
					continue
				}
				if !comparisonPrefixOverlap(infos[p.i].prefixBitmap, infos[p.j].prefixBitmap) {
					pairSkipped.Add(1)
					pairSkippedPrefix.Add(1)
					pairMu.Lock()
					pairResults = append(pairResults, pairResult{i: p.i, j: p.j, common: 0})
					pairMu.Unlock()
					continue
				}
				srcA, err := setCache.Open(infos[p.i].name)
				if err != nil {
					e.logger.Warn("pairwise comparison skipped: cannot open set", "set", infos[p.i].name, "error", err)
					continue
				}
				srcB, err := setCache.Open(infos[p.j].name)
				if err != nil {
					e.logger.Warn("pairwise comparison skipped: cannot open set", "set", infos[p.j].name, "error", err)
					continue
				}
				started := time.Now()
				common := iprange.OverlapCountIter(srcA, srcB)
				recordAtomicDuration(&pairCount, &pairTotalNS, &pairMaxNS, time.Since(started))
				ioErrA := checkFileSetErr(srcA.RangeSource, infos[p.i].name, e.logger)
				ioErrB := checkFileSetErr(srcB.RangeSource, infos[p.j].name, e.logger)
				if ioErrA != nil || ioErrB != nil {
					continue
				}
				pairMu.Lock()
				pairResults = append(pairResults, pairResult{i: p.i, j: p.j, common: common})
				pairMu.Unlock()
			}
		})
	}

	// Only generate pairs where at least one side was updated.
	var pairCandidates int64
sendPairs:
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			if isUpdated(infos[i].name) || isUpdated(infos[j].name) {
				pairCandidates++
				select {
				case <-ctx.Done():
					break sendPairs
				case pairCh <- pair{i, j}:
				}
			}
		}
	}
	close(pairCh)
	wg.Wait()
	if err := contextErr(ctx); err != nil {
		return err
	}
	if pairCandidates > 0 {
		e.observeRunCounter("metadata.comparison_pair_candidates", pairCandidates, 0)
	}
	if count := pairCount.Load(); count > 0 {
		e.observeRunCounter("metadata.comparison_pair_overlap", count, 0)
		e.observeRunOperationAggregate(
			"metadata.comparison_pair_overlap",
			count,
			time.Duration(pairTotalNS.Load()),
			time.Duration(pairMaxNS.Load()),
		)
	}
	if skipped := pairSkipped.Load(); skipped > 0 {
		e.observeRunOperationAggregate("metadata.comparison_pair_skipped", skipped, 0, 0)
		e.observeRunCounter("metadata.comparison_pair_skipped", skipped, 0)
	}
	if skipped := pairSkippedEmpty.Load(); skipped > 0 {
		e.observeRunCounter("metadata.comparison_pair_skipped_empty", skipped, 0)
	}
	if skipped := pairSkippedRange.Load(); skipped > 0 {
		e.observeRunCounter("metadata.comparison_pair_skipped_range", skipped, 0)
	}
	if skipped := pairSkippedPrefix.Load(); skipped > 0 {
		e.observeRunCounter("metadata.comparison_pair_skipped_prefix", skipped, 0)
	}

	// Precompute the positive leaf-ancestor set for every feed so we can
	// mark "related" pairs in O(|family|) per row. See
	// leafAncestors() below for the definition: two feeds are related iff
	// their positive leaf-ancestor sets intersect. For signed merges,
	// subtractive parents are dependencies, not positive ancestry.
	familyCache := make(map[string]map[string]bool, len(e.cfg.Sources))
	familyFor := func(name string) map[string]bool {
		if cached, ok := familyCache[name]; ok {
			return cached
		}
		f := leafAncestors(e.cfg, name)
		familyCache[name] = f
		return f
	}
	areRelated := func(a, b string) bool {
		fa := familyFor(a)
		fb := familyFor(b)
		// Iterate the smaller set for cheapness.
		if len(fb) < len(fa) {
			fa, fb = fb, fa
		}
		for k := range fa {
			if fb[k] {
				return true
			}
		}
		return false
	}

	// Group fresh pairwise results by feed name. Each pair contributes
	// one row to each side of the pair so both feeds learn about the
	// overlap during this run.
	grouped := map[string][]CompareRow{}
	for _, pr := range pairResults {
		related := areRelated(infos[pr.i].name, infos[pr.j].name)
		grouped[infos[pr.i].name] = append(grouped[infos[pr.i].name], CompareRow{
			Name:     infos[pr.j].name,
			Category: infos[pr.j].category,
			IPs:      infos[pr.j].ips,
			Common:   pr.common,
			Related:  related,
		})
		grouped[infos[pr.j].name] = append(grouped[infos[pr.j].name], CompareRow{
			Name:     infos[pr.i].name,
			Category: infos[pr.i].category,
			IPs:      infos[pr.i].ips,
			Common:   pr.common,
			Related:  related,
		})
	}

	// Merge the fresh rows into each feed's existing _comparison.json
	// before writing. The previous implementation overwrote the file
	// with the partial group from the current scheduler tick, gutting
	// every comparison file every time a sub-set of feeds updated.
	// Reading the existing file, replacing matching rows by Name, and
	// appending new ones preserves the rest of the catalog overlap
	// across runs while still surfacing today's recomputed values.
	var mergeCount int64
	var mergeTotal time.Duration
	var mergeMax time.Duration
	for name, group := range grouped {
		started := time.Now()
		livePath := filepath.Join(e.outputDir(), name+"_comparison.json")
		path := filepath.Join(outDir, name+"_comparison.json")
		var existing []CompareRow
		if data, err := os.ReadFile(livePath); err == nil {
			if err := json.Unmarshal(data, &existing); err != nil {
				// A corrupt comparison file should not poison the
				// merge — fall back to "no existing rows" so the
				// next write replaces it cleanly.
				e.logger.Warn("comparison merge: ignoring unreadable existing file", "set", name, "error", err)
				existing = nil
			}
		} else if !os.IsNotExist(err) {
			e.logger.Warn("comparison merge: cannot read existing file", "set", name, "error", err)
		}
		merged := mergeCompareRows(existing, group)
		sort.Slice(merged, func(i, j int) bool {
			return merged[i].Name < merged[j].Name
		})
		data, err := jsonMarshalTabIndent(merged)
		if err != nil {
			return err
		}
		if err := writeFileAtomicAt(path, append(data, '\n'), generatedFileMode, e.feedProcessingTimestamp(name)); err != nil {
			return err
		}
		mergeCount++
		mergeDur := time.Since(started)
		mergeTotal += mergeDur
		if mergeDur > mergeMax {
			mergeMax = mergeDur
		}
	}
	if mergeCount > 0 {
		e.observeRunOperationAggregate("metadata.comparison_merge_rows", mergeCount, mergeTotal, mergeMax)
	}
	if err := e.sanitizeComparisonArtifacts(outDir); err != nil {
		return err
	}
	return nil
}

func recordAtomicDuration(count, totalNS, maxNS *atomic.Int64, dur time.Duration) {
	if count == nil || totalNS == nil || maxNS == nil {
		return
	}
	if dur < 0 {
		dur = 0
	}
	ns := dur.Nanoseconds()
	count.Add(1)
	totalNS.Add(ns)
	for {
		current := maxNS.Load()
		if ns <= current {
			return
		}
		if maxNS.CompareAndSwap(current, ns) {
			return
		}
	}
}

func rangeBounds(src *closableSource) (uint32, uint32, bool) {
	if src == nil || src.RangeSource == nil {
		return 0, 0, false
	}
	switch set := src.RangeSource.(type) {
	case iprange.FileSet:
		if set.Len() == 0 {
			return 0, 0, false
		}
		first, err := set.Range(0)
		if err != nil {
			return 0, 0, false
		}
		last, err := set.Range(set.Len() - 1)
		if err != nil {
			return 0, 0, false
		}
		return first.Lo, last.Hi, true
	case *iprange.IPSet:
		set.Optimize()
		if len(set.Ranges) == 0 {
			return 0, 0, false
		}
		return set.Ranges[0].Lo, set.Ranges[len(set.Ranges)-1].Hi, true
	default:
		return 0, 0, false
	}
}

const (
	comparisonPrefixBits  = 20
	comparisonPrefixShift = 32 - comparisonPrefixBits
	comparisonPrefixWords = 1 << (comparisonPrefixBits - 6)
)

type comparisonPrefixBitmap [comparisonPrefixWords]uint64

func buildComparisonPrefixBitmap(src iprange.RangeSource) *comparisonPrefixBitmap {
	if src == nil {
		return nil
	}
	var bitmap comparisonPrefixBitmap
	hasRanges := false
	for r := range src.Iter() {
		hasRanges = true
		start := r.Lo >> comparisonPrefixShift
		end := r.Hi >> comparisonPrefixShift
		for prefix := start; prefix <= end; prefix++ {
			bitmap[prefix>>6] |= uint64(1) << (prefix & 63)
		}
	}
	if !hasRanges {
		return nil
	}
	return &bitmap
}

func comparisonPrefixOverlap(a, b *comparisonPrefixBitmap) bool {
	if a == nil || b == nil {
		return true
	}
	for i := range a {
		if a[i]&b[i] != 0 {
			return true
		}
	}
	return false
}

// leafAncestors returns the set of positive leaf (primary, non-derivative)
// source names that a given feed ultimately derives from. It walks positive
// ancestry until it hits sources with no positive parents and collects their
// names. For signed merges, only additive parents are positive ancestry;
// subtractive parents remain dependencies but do not make comparison rows
// related.
//
// For a plain HTTP source X:               leafAncestors(X) = {X}
// For a retention variant X_1d of X:       leafAncestors(X_1d) = {X}
// For a merge M with sources [A, B] - [C]: leafAncestors(M) = {A, B}
//
//	(or the leaves of A and B if either is itself a
//	derivative — merges-of-merges are legal in the DAG model
//	and this walks through transitively)
//
// Used by writeComparisonFiles to mark "related" pairs: two feeds whose
// positive leaf-ancestor sets intersect share at least one primary ancestor,
// which means any overlap between them is trivially explained by shared positive
// ancestry rather than by genuine agreement between independent sources. The
// public UI's
// uniqueness / inclusion tiles skip these rows so the numbers
// reflect real cross-feed signal instead of derivative echo.
func leafAncestors(cfg *config.Config, name string) map[string]bool {
	result := map[string]bool{}
	if cfg == nil {
		return result
	}
	visited := map[string]bool{}
	var walk func(n string)
	walk = func(n string) {
		if visited[n] {
			return
		}
		visited[n] = true
		src := cfg.Sources[n]
		if src == nil {
			return
		}
		parents := positiveLineageParents(src)
		if len(parents) == 0 {
			result[n] = true
			return
		}
		for _, parent := range parents {
			walk(parent)
		}
	}
	walk(name)
	return result
}

func positiveLineageParents(src *config.Source) []string {
	if src == nil {
		return nil
	}
	if src.Provenance == config.ProvenanceSecondaryMerge || len(src.MergeSources) > 0 || len(src.MergeExclude) > 0 {
		return mergeSourceNames(src)
	}
	return src.DerivedFrom
}

// mergeCompareRows merges fresh comparison rows into the existing rows
// loaded from disk. Fresh positive-overlap rows replace any existing entry with
// the same counterpart Name; rows that only exist on disk are preserved. Fresh
// zero-overlap rows delete matching existing rows because absence is the public
// representation of "no overlap".
func mergeCompareRows(existing, fresh []CompareRow) []CompareRow {
	byName := make(map[string]CompareRow, len(existing)+len(fresh))
	for _, r := range existing {
		if r.Common == 0 {
			continue
		}
		byName[r.Name] = r
	}
	for _, r := range fresh {
		if r.Common == 0 {
			delete(byName, r.Name)
			continue
		}
		byName[r.Name] = r
	}
	out := make([]CompareRow, 0, len(byName))
	for _, r := range byName {
		out = append(out, r)
	}
	return out
}

func (e *Engine) sanitizeComparisonArtifacts(outDir string) error {
	liveOutDir := e.outputDir()
	paths := map[string]string{}
	collect := func(dir string) {
		if dir == "" {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) && e.logger != nil {
				e.logger.Warn("comparison sanitize: cannot read directory", "dir", dir, "error", err)
			}
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, "_comparison.json") {
				continue
			}
			paths[name] = filepath.Join(dir, name)
		}
	}
	collect(liveOutDir)
	// Staged output wins over live output when both contain the same artifact.
	collect(outDir)

	var cleaned int64
	for rel, srcPath := range paths {
		data, changed, err := sanitizedComparisonArtifactData(srcPath)
		if err != nil {
			if e.logger != nil {
				e.logger.Warn("comparison sanitize: cannot parse artifact", "file", srcPath, "error", err)
			}
			continue
		}
		if !changed {
			continue
		}
		if err := writeFileAtomic(filepath.Join(outDir, rel), data, generatedFileMode); err != nil {
			return err
		}
		cleaned++
	}
	if cleaned > 0 {
		e.observeRunCounter("metadata.comparison_zero_rows_removed", cleaned, 0)
	}
	return nil
}

func sanitizedComparisonArtifactData(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var rows []CompareRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, false, err
	}
	filtered := rows[:0]
	changed := false
	for _, row := range rows {
		if row.Common == 0 {
			changed = true
			continue
		}
		filtered = append(filtered, row)
	}
	if !changed {
		return nil, false, nil
	}
	out, err := jsonMarshalTabIndent(filtered)
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

const (
	sitemapXMLNS   = "http://www.sitemaps.org/schemas/sitemap/0.9"
	maxSitemapURLs = 45000
)

type sitemapURLEntry struct {
	Loc string `xml:"loc"`
}

type sitemapURLSet struct {
	XMLName xml.Name          `xml:"urlset"`
	XMLNS   string            `xml:"xmlns,attr"`
	URLs    []sitemapURLEntry `xml:"url"`
}

type sitemapIndexEntry struct {
	Loc string `xml:"loc"`
}

type sitemapIndex struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	XMLNS    string              `xml:"xmlns,attr"`
	Sitemaps []sitemapIndexEntry `xml:"sitemap"`
}

func (e *Engine) writePublicMetadataFiles(outDir string, outputNames []string) ([]string, error) {
	siteBase := e.publicSiteBaseURL()
	feedPrefix := e.publicFeedURLPrefix(siteBase)
	files, err := e.writeSitemapFiles(outDir, siteBase, feedPrefix, outputNames)
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(outDir, "robots.txt"), []byte(renderRobotsTXT(siteBase)), generatedFileMode); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(outDir, "llms.txt"), []byte(renderLLMSTXT(siteBase, feedPrefix, outputNames)), generatedFileMode); err != nil {
		return nil, err
	}
	files = append(files, "robots.txt", "llms.txt")
	return files, nil
}

func (e *Engine) writeSitemapFiles(outDir, siteBase, feedPrefix string, outputNames []string) ([]string, error) {
	const indexName = "sitemap.xml"
	files := []string{indexName}
	if siteBase == "" {
		payload, err := marshalSitemapIndex(nil)
		if err != nil {
			return nil, err
		}
		if err := writeFileAtomic(filepath.Join(outDir, indexName), payload, generatedFileMode); err != nil {
			return nil, err
		}
		if err := removeStaleSitemapShards(outDir, files); err != nil {
			return nil, err
		}
		return files, nil
	}

	shards := []struct {
		name string
		urls []string
	}{
		{name: "sitemap-pages.xml", urls: publicPageSitemapURLs(siteBase)},
		{name: "sitemap-feeds.xml", urls: publicFeedSitemapURLs(feedPrefix, outputNames)},
		{name: "sitemap-countries.xml", urls: e.publicCountrySitemapURLs(siteBase, outDir)},
		{name: "sitemap-maintainers.xml", urls: e.publicMaintainerSitemapURLs(siteBase)},
	}
	for _, shard := range shards {
		if err := writeSitemapURLSet(filepath.Join(outDir, shard.name), shard.urls); err != nil {
			return nil, err
		}
		files = append(files, shard.name)
	}
	for i, urls := range chunkStrings(e.publicASNSitemapURLs(siteBase, outDir), maxSitemapURLs) {
		name := fmt.Sprintf("sitemap-asns-%04d.xml", i+1)
		if err := writeSitemapURLSet(filepath.Join(outDir, name), urls); err != nil {
			return nil, err
		}
		files = append(files, name)
	}

	indexEntries := make([]string, 0, len(files)-1)
	for _, name := range files[1:] {
		indexEntries = append(indexEntries, joinPublicURL(siteBase, name))
	}
	payload, err := marshalSitemapIndex(indexEntries)
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(outDir, indexName), payload, generatedFileMode); err != nil {
		return nil, err
	}
	if err := removeStaleSitemapShards(outDir, files); err != nil {
		return nil, err
	}
	return files, nil
}

func publicPageSitemapURLs(siteBase string) []string {
	urls := make([]string, 0, 5)
	for _, path := range []string{"", "countries", "asns", "maintainers", "methodology"} {
		urls = append(urls, joinPublicURL(siteBase, path))
	}
	return urls
}

func publicFeedSitemapURLs(feedPrefix string, outputNames []string) []string {
	if feedPrefix == "" {
		return nil
	}
	urls := make([]string, 0, len(outputNames))
	for _, name := range outputNames {
		urls = append(urls, joinPublicURL(feedPrefix, name))
	}
	slices.Sort(urls)
	return urls
}

func (e *Engine) publicCountrySitemapURLs(siteBase, outDir string) []string {
	if index := e.loadCountryIndexForSitemap(outDir); index != nil {
		return countrySitemapURLsFromIndex(siteBase, index)
	}
	index, err := e.buildCountryIndex(newEntityOutputView(e, outDir))
	if err != nil || index == nil {
		return nil
	}
	return countrySitemapURLsFromIndex(siteBase, index)
}

func countrySitemapURLsFromIndex(siteBase string, index *CountryIndexPayload) []string {
	if index == nil {
		return nil
	}
	urls := make([]string, 0, len(index.Countries))
	for _, country := range index.Countries {
		code := strings.ToUpper(strings.TrimSpace(country.Code))
		if code == "" {
			continue
		}
		urls = append(urls, joinPublicURL(siteBase, "countries/"+code))
	}
	slices.Sort(urls)
	return urls
}

func (e *Engine) publicASNSitemapURLs(siteBase, outDir string) []string {
	if index := e.loadASNIndexForSitemap(outDir); index != nil {
		return asnSitemapURLsFromIndex(siteBase, index)
	}
	index, err := e.buildASNIndex(newEntityOutputView(e, outDir))
	if err != nil || index == nil {
		return nil
	}
	return asnSitemapURLsFromIndex(siteBase, index)
}

func asnSitemapURLsFromIndex(siteBase string, index *ASNIndexPayload) []string {
	if index == nil {
		return nil
	}
	urls := make([]string, 0, len(index.ASNs))
	for _, asn := range index.ASNs {
		if asn.ASN == 0 {
			continue
		}
		urls = append(urls, joinPublicURL(siteBase, fmt.Sprintf("asns/%d", asn.ASN)))
	}
	slices.Sort(urls)
	return urls
}

func (e *Engine) loadCountryIndexForSitemap(outDir string) *CountryIndexPayload {
	for _, path := range sitemapIndexCandidatePaths(outDir, e.outputDir(), e.publicCountryIndexRelPath()) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload CountryIndexPayload
		if err := json.Unmarshal(data, &payload); err == nil {
			return &payload
		}
	}
	return nil
}

func (e *Engine) loadASNIndexForSitemap(outDir string) *ASNIndexPayload {
	for _, path := range sitemapIndexCandidatePaths(outDir, e.outputDir(), e.publicASNIndexRelPath()) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload ASNIndexPayload
		if err := json.Unmarshal(data, &payload); err == nil {
			return &payload
		}
	}
	return nil
}

func sitemapIndexCandidatePaths(stageDir, liveDir, rel string) []string {
	paths := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, dir := range []string{stageDir, liveDir} {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, rel)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func (e *Engine) publicMaintainerSitemapURLs(siteBase string) []string {
	index, err := e.MaintainerIndex(nil)
	if err != nil || index == nil {
		return nil
	}
	urls := make([]string, 0, len(index.Maintainers))
	for _, maintainer := range index.Maintainers {
		slug := strings.TrimSpace(maintainer.Slug)
		if slug == "" {
			continue
		}
		urls = append(urls, joinPublicURL(siteBase, "maintainers/"+slug))
	}
	slices.Sort(urls)
	return urls
}

func writeSitemapURLSet(path string, urls []string) error {
	payload, err := marshalSitemapURLSet(urls)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, payload, generatedFileMode)
}

func marshalSitemapURLSet(urls []string) ([]byte, error) {
	entries := make([]sitemapURLEntry, 0, len(urls))
	for _, u := range urls {
		if strings.TrimSpace(u) == "" {
			continue
		}
		entries = append(entries, sitemapURLEntry{Loc: u})
	}
	payload, err := xml.MarshalIndent(sitemapURLSet{XMLNS: sitemapXMLNS, URLs: entries}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(payload, '\n')...), nil
}

func marshalSitemapIndex(urls []string) ([]byte, error) {
	entries := make([]sitemapIndexEntry, 0, len(urls))
	for _, u := range urls {
		if strings.TrimSpace(u) == "" {
			continue
		}
		entries = append(entries, sitemapIndexEntry{Loc: u})
	}
	payload, err := xml.MarshalIndent(sitemapIndex{XMLNS: sitemapXMLNS, Sitemaps: entries}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(payload, '\n')...), nil
}

func chunkStrings(items []string, size int) [][]string {
	if size <= 0 || len(items) == 0 {
		return nil
	}
	chunks := make([][]string, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

func removeStaleSitemapShards(outDir string, generated []string) error {
	stale, err := staleSitemapShardNames(outDir, generated)
	if err != nil {
		return err
	}
	for _, name := range stale {
		path := filepath.Join(outDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func staleSitemapShardNames(outDir string, generated []string) ([]string, error) {
	keep := make(map[string]struct{}, len(generated))
	for _, name := range generated {
		keep[name] = struct{}{}
	}
	matches, err := filepath.Glob(filepath.Join(outDir, "sitemap-*.xml"))
	if err != nil {
		return nil, err
	}
	stale := make([]string, 0)
	for _, path := range matches {
		name := filepath.Base(path)
		if _, ok := keep[name]; ok {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		stale = append(stale, name)
	}
	slices.Sort(stale)
	return stale, nil
}

func (e *Engine) publicSiteBaseURL() string {
	if base := normalizeAbsolutePublicURL(e.runtime.PublicBaseURL); base != "" {
		return base
	}
	return derivePublicSiteBaseFromWebURL(e.runtime.WebURL)
}

func (e *Engine) publicFeedURLPrefix(siteBase string) string {
	if prefix := normalizeAbsolutePublicURL(e.runtime.WebURL); prefix != "" {
		return prefix
	}
	if siteBase == "" {
		return ""
	}
	return joinPublicURL(siteBase, "ipsets")
}

func normalizeAbsolutePublicURL(raw string) string {
	parsed, ok := parseAbsolutePublicURL(raw)
	if !ok {
		return ""
	}
	normalized := *parsed
	normalized.Path = strings.TrimRight(normalized.Path, "/")
	return strings.TrimRight(normalized.String(), "/")
}

func derivePublicSiteBaseFromWebURL(raw string) string {
	parsed, ok := parseAbsolutePublicURL(raw)
	if !ok {
		return ""
	}
	derived := *parsed
	path := strings.TrimRight(derived.Path, "/")
	switch {
	case path == "" || path == "/":
		derived.Path = ""
	case strings.HasSuffix(path, "/ipsets"):
		derived.Path = strings.TrimSuffix(path, "/ipsets")
		if derived.Path == "/" {
			derived.Path = ""
		}
	default:
		derived.Path = path
	}
	return strings.TrimRight(derived.String(), "/")
}

func parseAbsolutePublicURL(raw string) (*url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, false
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, true
}

func joinPublicURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		if base == "" {
			return "/"
		}
		return base
	}
	if base == "" {
		return "/" + path
	}
	return base + "/" + path
}

func renderRobotsTXT(siteBase string) string {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, path := range []string{
		"/api/v1/search",
		"/api/v1/query",
		"/api/v1/compose",
		"/api/v1/client-ip",
		"/api/v1/sets/*/search",
		"/api/v1/ipsets/*/search",
	} {
		b.WriteString("Disallow: ")
		b.WriteString(path)
		b.WriteString("\n")
	}
	b.WriteString("Allow: /\n")
	if siteBase != "" {
		b.WriteString("Sitemap: ")
		b.WriteString(joinPublicURL(siteBase, "sitemap.xml"))
		b.WriteString("\n")
	}
	return b.String()
}

func renderLLMSTXT(siteBase, feedPrefix string, outputNames []string) string {
	link := func(path string) string {
		return joinPublicURL(siteBase, path)
	}

	var b strings.Builder
	b.WriteString("# FireHOL IP Lists\n\n")
	b.WriteString("> Public cybercrime IP feed observatory for discovering, comparing, and consuming maintained IP blocklists.\n\n")
	b.WriteString("This file is a concise, public-only map of the site for AI agents. It links to human pages, public APIs, methodology, and feed artifacts. It does not describe private or operator-only surfaces.\n\n")
	b.WriteString("## Primary Pages\n\n")
	b.WriteString("- [Homepage and feed explorer](" + link("") + "): Main public surface for IP lookup and feed discovery.\n")
	b.WriteString("- [Countries](" + link("countries") + "): Country index for public feed matches.\n")
	b.WriteString("- [ASNs](" + link("asns") + "): ASN index for public feed matches.\n")
	b.WriteString("- [Maintainers](" + link("maintainers") + "): Maintainer index for public feeds.\n")
	b.WriteString("- [Methodology](" + link("methodology") + "): Explanations of feed metrics, health, overlap, retention, geography, ASN attribution, and insights.\n\n")
	b.WriteString("## Public APIs\n\n")
	b.WriteString("- [Service status](" + link("api/v1/status") + "): High-level public service state.\n")
	b.WriteString("- [Categories](" + link("api/v1/categories") + "): Public category registry.\n")
	b.WriteString("- [Feed catalog](" + link("api/v1/sets") + "): Public feed inventory.\n")
	b.WriteString("- [Global IP search](" + link("api/v1/search?ip=1.1.1.1") + "): Query public feed membership for one IP address.\n")
	b.WriteString("- [Countries API](" + link("api/v1/countries") + "): Published country summaries.\n")
	b.WriteString("- [ASNs API](" + link("api/v1/asns") + "): Published ASN summaries.\n")
	b.WriteString("- [Maintainers API](" + link("api/v1/maintainers") + "): Public maintainer summaries.\n")
	b.WriteString("- [Methodology API](" + link("api/v1/methodology") + "): Machine-readable methodology index.\n")
	if len(outputNames) > 0 {
		b.WriteString("- [Compose API example](" + link("api/v1/compose?include="+url.QueryEscape(outputNames[0])+"&format=single") + "): Public feed composition endpoint using include/exclude query parameters.\n")
	}
	b.WriteString("\n")
	b.WriteString("## Feed Surfaces\n\n")
	b.WriteString("- [Legacy feed catalog JSON](" + link("all-ipsets.json") + "): Bash-compatible public feed catalog.\n")
	b.WriteString("- [Public feed API index](" + link("api/v1/sets") + "): Canonical API entry point for feed metadata.\n")
	if len(outputNames) > 0 && feedPrefix != "" {
		name := outputNames[0]
		b.WriteString("- [Example feed detail](" + joinPublicURL(feedPrefix, name) + "): Example public feed page; use the feed catalog for the full list.\n")
	}
	b.WriteString("\n")
	b.WriteString("## Optional\n\n")
	b.WriteString("- [Sitemap](" + link("sitemap.xml") + "): XML sitemap for public pages.\n")
	b.WriteString("- [robots.txt](" + link("robots.txt") + "): Crawler policy and sitemap pointer.\n")
	return b.String()
}

// markdownLinksToHTML converts markdown links to HTML anchors using the
// same pipeline as the bash script: insert newlines after ")", convert
// links, then replace newlines/tabs with spaces. This deliberately
// produces double-space artifacts that the bash version creates.
func markdownLinksToHTML(input string) string {
	// Step 1: insert newline after every ")" — same as sed "s/)/)\n/g"
	input = strings.ReplaceAll(input, ")", ")\n")
	// Step 2: convert markdown links — same as sed on each line
	input = markdownLinkRE.ReplaceAllString(input, `<a href="$2">$1</a>`)
	// Step 3: replace newlines/tabs with spaces — same as tr "\n\t" "  "
	r := strings.NewReplacer("\n", " ", "\t", " ")
	return r.Replace(input)
}

// jsonMarshalTabIndent produces JSON with tab indentation matching the
// bash script's printf-based output.
func jsonMarshalTabIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "\t")
}

func millis(seconds int64) int64 {
	if seconds <= 0 {
		return 0
	}
	return seconds * 1000
}
