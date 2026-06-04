package engine

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/enrichment"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
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
	redistributable := e.isRedistributable(name)
	meta := e.baseSetMetadata(name, entry)
	e.applyMetadataConfigFallbacks(name, &meta)
	e.applyMetadataArtifactFields(name, entry, outDir, redistributable, &meta)
	e.applyMetadataSourceFields(name, redistributable, enableAll, resolver, &meta)
	clearMetadataRawFieldsForPolicy(redistributable, &meta)
	return meta
}

func (e *Engine) baseSetMetadata(name string, entry *cache.Entry) setMetadata {
	category := entry.Category
	if src := e.lookupSource(name); src != nil && src.Category != "" {
		category = src.Category
	}
	return setMetadata{
		Name:               name,
		Entries:            entry.Entries,
		EntriesMin:         entry.EntriesMin,
		EntriesMax:         entry.EntriesMax,
		IPs:                entry.UniqueIPs,
		IPsMin:             entry.IPsMin,
		IPsMax:             entry.IPsMax,
		IPV:                entry.IPV,
		Hash:               entry.Hash,
		Frequency:          entry.FrequencyMinutes,
		Aggregation:        aggregationMinutesFromName(name),
		Started:            millis(entry.StartedDate),
		Updated:            millis(entry.SourceDate),
		Processed:          millis(entry.ProcessedDate),
		Checked:            millis(entry.CheckedDate),
		ClockSkew:          millis(entry.ClockSkewSeconds),
		Category:           category,
		Maintainer:         entry.Maintainer,
		MaintainerURL:      entry.MaintainerURL,
		License:            entry.License,
		Attribution:        entry.Attribution,
		Info:               markdownLinksToHTML(entry.Info),
		File:               entry.File,
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
}

func (e *Engine) applyMetadataConfigFallbacks(name string, meta *setMetadata) {
	// License + attribution are config-time facts, not runtime state.
	// finalize.go copies them into the cache.Entry on every successful
	// processing run, but cached entries written before that change
	// landed have empty values until the next refresh. Fall back to
	// the live config so users see the right answer immediately.
	if e.cfg == nil {
		return
	}
	src := e.cfg.SourceByName(name)
	if src == nil {
		return
	}
	if meta.License == "" {
		meta.License = src.License
	}
	if meta.Attribution == "" {
		meta.Attribution = src.Attribution
	}
}

func (e *Engine) applyMetadataArtifactFields(name string, entry *cache.Entry, outDir string, redistributable bool, meta *setMetadata) {
	if redistributable {
		meta.Source = entry.PublicURL
	}
	if metadataArtifactExists(e, outDir, name+"_comparison.json") {
		meta.Comparison = name + "_comparison.json"
	}
	e.applyMetadataGeoArtifacts(name, outDir, meta)
	if redistributable && entry.File != "" && e.runtime.LocalCopyURL != "" {
		meta.FileLocal = strings.TrimRight(e.runtime.LocalCopyURL, "/") + "/" + entry.File
	}
	if redistributable && entry.File != "" && e.runtime.GitHubChangesURL != "" {
		meta.CommitHistory = strings.TrimRight(e.runtime.GitHubChangesURL, "/") + "/" + entry.File
	}
}

func (e *Engine) applyMetadataGeoArtifacts(name, outDir string, meta *setMetadata) {
	if e.cfg == nil {
		return
	}
	for _, src := range e.cfg.SourcesWithUse(config.UseGeoIP) {
		file := name + "_" + src.Name + ".json"
		if !metadataArtifactExists(e, outDir, file) {
			continue
		}
		if meta.Geo == nil {
			meta.Geo = make(map[string]string)
		}
		meta.Geo[src.Name] = file
	}
}

func metadataArtifactExists(e *Engine, outDir, file string) bool {
	return fileExists(filepath.Join(outDir, file)) || fileExists(filepath.Join(e.outputDir(), file))
}

func (e *Engine) applyMetadataSourceFields(name string, redistributable, enableAll bool, resolver *effectiveEntryResolver, meta *setMetadata) {
	src := e.lookupSource(name)
	if src == nil {
		meta.DontRedistribute = !redistributable
		return
	}
	meta.Provenance = string(publicProvenance(src))
	applyMetadataEnrichment(src, meta)
	if len(src.Use) > 0 {
		meta.UsedFor = append([]string(nil), src.Use...)
	}
	meta.Hidden = src.Hidden
	meta.Format = src.Format
	meta.Output = src.Output
	meta.Processor = formatProcessorSteps(src.Processor)
	meta.DontRedistribute = !redistributable
	if src.Provenance == config.ProvenanceSecondaryMerge {
		e.applyMetadataMergeComposition(src, enableAll, resolver, meta)
	}
}

func applyMetadataEnrichment(src *config.Source, meta *setMetadata) {
	if src.Enrichment == nil {
		return
	}
	meta.OfficialName = enrichment.StringValue(src.Enrichment.OfficialName)
	meta.ShortDescription = enrichment.StringValue(src.Enrichment.ShortDescription)
	meta.CurrentStatus = &src.Enrichment.CurrentStatus
	meta.Enrichment = src.Enrichment
}

func (e *Engine) applyMetadataMergeComposition(src *config.Source, enableAll bool, resolver *effectiveEntryResolver, meta *setMetadata) {
	composition := e.mergeCompositionWithResolver(src, enableAll, resolver)
	meta.MergeIncluded = append([]MergeInputState(nil), composition.Included...)
	meta.MergeSubtracted = append([]MergeInputState(nil), composition.Subtracted...)
	meta.MergeExcluded = append([]MergeInputState(nil), composition.Excluded...)
}

func clearMetadataRawFieldsForPolicy(redistributable bool, meta *setMetadata) {
	if redistributable && meta.Health.Class != feedhealth.ClassArchived {
		return
	}
	meta.Source = ""
	meta.File = ""
	meta.FileLocal = ""
	meta.CommitHistory = ""
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
		parts = append(parts, formatProcessorStepWithArgs(step))
	}
	return strings.Join(parts, " | ")
}

func formatProcessorStepWithArgs(step config.ProcessorStep) string {
	keys := make([]string, 0, len(step.Args))
	for k := range step.Args {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	argParts := make([]string, 0, len(keys))
	for _, k := range keys {
		argParts = append(argParts, k+"="+step.Args[k])
	}
	return step.Name + "(" + strings.Join(argParts, ",") + ")"
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
