package config

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/firehol/update-ipsets/pkg/enrichment"
)

// Internal URL schemes used by the loader to emit first-class sources
// from curator-facing sugar (history windows, merges). These constants
// keep that loader detail centralized even though runtime behavior now
// keys primarily off provenance and DerivedFrom metadata.
const (
	// InternalRetentionWindowScheme is the URL scheme prefix for a
	// retention-window derivative source. The URL form is
	// "internal://retention_window?parent=<name>&minutes=<n>".
	InternalRetentionWindowScheme = "internal://retention_window"

	// InternalMergeScheme is the URL scheme prefix for a merge
	// derivative source. The URL form is
	// "internal://merge?inputs=<a>,<b>,<c>[&exclude=<d>,<e>]".
	InternalMergeScheme = "internal://merge"
)

// ExpandDerivatives rewrites curator-facing sugar into first-class
// Source entries:
//
//   - Every source or merge with a non-empty History field emits N additional
//     Source entries named "{parent}_{label}" (e.g. viriback_1d) whose
//     URL is an internal://retention_window query, whose DerivedFrom
//     lists the parent, and whose frequency is 0 (static — only
//     re-runs when the parent updates).
//   - Every entry in the legacy Merges map is moved into Sources as a
//     standalone entry with an internal://merge URL and DerivedFrom
//     listing every additive and subtractive dependency. MergeSources
//     and MergeExclude preserve the signed composition. The Merges map
//     is cleared.
//
// After expansion, the in-memory config contains exactly one kind of
// feed (Source). Downstream code (scheduler, engine pipeline, admin
// API) no longer iterates a separate merges namespace; it reasons over
// a unified source map and uses provenance / DerivedFrom to recognize
// plain feeds, history derivatives, and merges.
//
// Name collisions are fatal errors — if a curator manually declared
// both `viriback_1d` as a source AND `viriback` with `history: [1440]`,
// the loader cannot decide which to keep. Similarly, DerivedFrom
// cycles (a merge whose transitive inputs include itself) are fatal;
// see DetectCycles for the enumeration logic.
//
// Validation of the expanded config (valid fields, known use roles,
// etc.) is the caller's responsibility — ExpandDerivatives only
// expands and checks for collisions / cycles.
func ExpandDerivatives(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if cfg.Sources == nil {
		cfg.Sources = map[string]*Source{}
	}

	// Step 1: source retention window expansion. Must happen before merge
	// expansion so merges can reference retention variants as inputs
	// (e.g. a "last-week union" merge that takes `viriback_7d` as an
	// input only sees it after this step populates Sources).
	retentionNames := make([]string, 0, len(cfg.Sources))
	for name := range cfg.Sources {
		retentionNames = append(retentionNames, name)
	}
	slices.Sort(retentionNames) // deterministic iteration for tests / diffs
	if err := expandRetentionWindows(cfg, retentionNames); err != nil {
		return err
	}

	// Step 2: merge expansion. Every entry in cfg.Merges becomes a
	// standalone Source entry. The Merges map is cleared so downstream
	// code has exactly one iteration space.
	mergeNames := make([]string, 0, len(cfg.Merges))
	for name := range cfg.Merges {
		mergeNames = append(mergeNames, name)
	}
	slices.Sort(mergeNames)
	for _, mergeName := range mergeNames {
		m := cfg.Merges[mergeName]
		if m == nil {
			return fmt.Errorf("expand: merge %q is nil", mergeName)
		}
		if _, exists := cfg.Sources[mergeName]; exists {
			return fmt.Errorf(
				"expand: merge name collision — source %q already declared, "+
					"cannot also create it as a merge of %v",
				mergeName, m.Sources)
		}
		sources := cleanMergeRefs(m.Sources)
		exclude := cleanMergeRefs(m.Exclude)
		if len(sources) == 0 {
			return fmt.Errorf("expand: merge %q has no sources", mergeName)
		}
		if err := validateSignedMergeRefs(mergeName, sources, exclude); err != nil {
			return err
		}
		redistributable, err := effectiveMergeRedistributable(cfg, mergeName, m)
		if err != nil {
			return err
		}
		derived := append(append([]string(nil), sources...), exclude...)
		frequency := m.Frequency
		if frequency <= 0 {
			frequency = cfg.Runtime.ProcessingIntervalMinutes
		}
		src := &Source{
			Name:                    mergeName,
			Label:                   m.Label,
			URL:                     buildMergeURL(sources, exclude),
			Frequency:               frequency,
			IPV:                     m.IPV,
			Output:                  m.Output,
			Category:                m.Category,
			Info:                    m.Info,
			Maintainer:              m.Maintainer,
			MaintainerURL:           m.MaintainerURL,
			License:                 m.License,
			Attribution:             m.Attribution,
			Enrichment:              enrichment.Clone(m.Enrichment),
			Redistributable:         redistributable,
			ExcludeFromUnmaintained: m.ExcludeFromUnmaintained,
			History:                 append([]int(nil), m.History...),
			Processor: []ProcessorStep{
				{Name: "passthrough"},
			},
			ProcessorRaw: "cat",
			EnabledByAll: true,
			Use:          append([]string(nil), m.Use...),
			Critical:     cloneCriticalMetadata(m.Critical),
			DerivedFrom:  derived,
			MergeSources: sources,
			MergeExclude: exclude,
			Provenance:   ProvenanceSecondaryMerge,
			// Empty output stays valid for explicitly empty input sets.
			// Missing or unavailable signed inputs fail composition before
			// rendering so a merge cannot silently broaden itself.
			AcceptEmpty: true,
		}
		cfg.Sources[mergeName] = src
	}
	cfg.Merges = nil

	// Step 3: merge retention window expansion. Merge parents only exist in
	// Sources after step 2, so their history derivatives must be emitted here.
	if err := expandRetentionWindows(cfg, mergeNames); err != nil {
		return err
	}

	// Step 4: cycle detection across the fully-expanded DerivedFrom
	// graph. A cycle at this point can only come from a curator who
	// hand-wrote internal:// URLs or a malformed merges: block —
	// retention_window derivatives only list their parent and the
	// parent never lists them back, so they cannot form cycles on
	// their own.
	if err := cfg.DetectCycles(); err != nil {
		return fmt.Errorf("expand: %w", err)
	}
	return nil
}

func expandRetentionWindows(cfg *Config, parentNames []string) error {
	for _, parentName := range parentNames {
		parent := cfg.Sources[parentName]
		if parent == nil || len(parent.History) == 0 {
			continue
		}
		if !sourceProducesIPSet(parent) {
			return fmt.Errorf("expand: feed %q declares history windows but does not produce a feed body", parentName)
		}
		windows := append([]int(nil), parent.History...)
		slices.Sort(windows)
		for _, minutes := range windows {
			if minutes <= 0 {
				continue
			}
			variantName := parentName + historyLabelForExpand(minutes)
			if _, exists := cfg.Sources[variantName]; exists {
				return fmt.Errorf(
					"expand: retention variant name collision — "+
						"source %q already declared, cannot also derive from %q history: [%d]",
					variantName, parentName, minutes)
			}
			variant := *parent // shallow-clone
			variant.Name = variantName
			variant.Enrichment = enrichment.Clone(parent.Enrichment)
			windowDays := normalizeHistoryWindowDays(minutes)
			variant.URL = buildRetentionWindowURL(parentName, minutes)
			variant.Frequency = 0
			variant.History = nil
			variant.DerivedFrom = []string{parentName}
			variant.MergeSources = nil
			variant.MergeExclude = nil
			variant.Provenance = ProvenanceSecondaryRetention
			variant.HistoryWindowDays = windowDays
			variant.Info = appendWindowNote(parent.Info, minutes)
			variant.ExcludeFromUnmaintained = parent.ExcludeFromUnmaintained
			// A retention variant is always a plain ipset. Parents that
			// do not produce feed bodies are rejected above.
			variant.Use = nil
			// History derivatives are downloader-composed plain CIDR
			// feed bodies built from parent feed bodies and retained
			// timestamped history snapshots. Inheriting the parent's processor pipeline
			// (e.g. csv_column for a CSV parent like viriback) would
			// strip that canonical CIDR body to zero entries, so the
			// derivative always uses passthrough.
			variant.Processor = []ProcessorStep{{Name: "passthrough"}}
			variant.ProcessorRaw = "cat"
			// Empty output is legitimate for retention variants:
			// freshly-added parents may not yet have enough retained
			// history snapshots to populate the window, so
			// the first run of the variant produces an empty ipset.
			// The downloader's default "reject empty" guard would
			// otherwise mark the variant as failed on the first tick.
			variant.AcceptEmpty = true
			variant.Static = nil
			variant.Critical = nil
			variant.ArtifactParent = ""
			cfg.Sources[variantName] = &variant
		}
		// Consume the sugar field so the rest of the engine does not
		// attempt to re-expand it. The parent itself stays intact as
		// a normal source.
		parent.History = nil
	}
	return nil
}

// buildRetentionWindowURL renders the canonical internal URL for a
// retention window derivative. The query string is sorted so two
// calls with the same inputs always produce the same URL (important
// for hashing / caching / diffs).
func buildRetentionWindowURL(parent string, minutes int) string {
	q := url.Values{}
	q.Set("parent", parent)
	q.Set("minutes", strconv.Itoa(minutes))
	return InternalRetentionWindowScheme + "?" + q.Encode()
}

// buildMergeURL renders the canonical internal URL for a merge
// derivative. Inputs are joined with commas and URL-encoded once;
// the parser reverses this by splitting on commas after URL decoding.
func buildMergeURL(inputs, exclude []string) string {
	q := url.Values{}
	q.Set("inputs", strings.Join(inputs, ","))
	if len(exclude) > 0 {
		q.Set("exclude", strings.Join(exclude, ","))
	}
	return InternalMergeScheme + "?" + q.Encode()
}

func cleanMergeRefs(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			out = append(out, ref)
		}
	}
	return out
}

func effectiveMergeRedistributable(cfg *Config, mergeName string, merge *Merge) (*bool, error) {
	if merge == nil {
		return nil, fmt.Errorf("expand: merge %q is nil", mergeName)
	}
	allowed := merge.Redistributable == nil || *merge.Redistributable

	visiting := map[string]struct{}{mergeName: {}}
	for _, ref := range append(cleanMergeRefs(merge.Sources), cleanMergeRefs(merge.Exclude)...) {
		refAllowed, err := mergeRefRedistributable(cfg, ref, visiting)
		if err != nil {
			return nil, err
		}
		if !refAllowed {
			allowed = false
		}
	}

	if !allowed {
		return boolPtr(false), nil
	}
	if merge.Redistributable != nil {
		return boolPtr(*merge.Redistributable), nil
	}
	return nil, nil
}

func mergeRefRedistributable(cfg *Config, ref string, visiting map[string]struct{}) (bool, error) {
	if src := cfg.Sources[ref]; src != nil {
		return src.IsRedistributable(), nil
	}
	m := cfg.Merges[ref]
	if m == nil {
		return true, nil
	}
	if _, ok := visiting[ref]; ok {
		return false, fmt.Errorf("expand: redistributability cycle through merge %q", ref)
	}
	visiting[ref] = struct{}{}
	defer delete(visiting, ref)

	if m.Redistributable != nil && !*m.Redistributable {
		return false, nil
	}
	for _, parent := range append(cleanMergeRefs(m.Sources), cleanMergeRefs(m.Exclude)...) {
		allowed, err := mergeRefRedistributable(cfg, parent, visiting)
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, nil
		}
	}
	return true, nil
}

func validateSignedMergeRefs(name string, sources, exclude []string) error {
	seenSources := map[string]struct{}{}
	for _, ref := range sources {
		if _, ok := seenSources[ref]; ok {
			return fmt.Errorf("merge %q has duplicate source reference %q", name, ref)
		}
		seenSources[ref] = struct{}{}
	}

	seenExclude := map[string]struct{}{}
	for _, ref := range exclude {
		if _, ok := seenExclude[ref]; ok {
			return fmt.Errorf("merge %q has duplicate exclude reference %q", name, ref)
		}
		if _, ok := seenSources[ref]; ok {
			return fmt.Errorf("merge %q references %q in both sources and exclude", name, ref)
		}
		seenExclude[ref] = struct{}{}
	}
	return nil
}

func normalizeHistoryWindowDays(value int) int {
	if value <= 0 {
		return 0
	}
	// Accept the current catalog's legacy minute values while treating
	// day-sized integers as the semantic source of truth.
	if value >= 1440 && value%1440 == 0 {
		return value / 1440
	}
	return value
}

// ParseRetentionWindowURL extracts (parent, minutes) from an
// internal://retention_window?parent=X&minutes=Y URL. Returns an
// error when the URL is malformed or uses the wrong scheme. Used by
// the engine at startup to build provider closures.
func ParseRetentionWindowURL(rawURL string) (parent string, minutes int, err error) {
	if !strings.HasPrefix(rawURL, InternalRetentionWindowScheme+"?") {
		return "", 0, fmt.Errorf("not a retention_window URL: %q", rawURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, fmt.Errorf("parse retention_window URL: %w", err)
	}
	q := u.Query()
	parent = q.Get("parent")
	if parent == "" {
		return "", 0, fmt.Errorf("retention_window URL missing parent: %q", rawURL)
	}
	rawMinutes := q.Get("minutes")
	if rawMinutes == "" {
		return "", 0, fmt.Errorf("retention_window URL missing minutes: %q", rawURL)
	}
	minutes, err = strconv.Atoi(rawMinutes)
	if err != nil {
		return "", 0, fmt.Errorf("retention_window URL invalid minutes %q: %w", rawMinutes, err)
	}
	if minutes <= 0 {
		return "", 0, fmt.Errorf("retention_window URL requires positive minutes, got %d", minutes)
	}
	return parent, minutes, nil
}

// ParseMergeURL extracts the additive input source names from an
// internal://merge?inputs=a,b,c URL. Empty names are filtered out.
//
// Deprecated: callers that need signed merge composition must use
// ParseMergeURLParts. This compatibility shim intentionally returns only
// additive inputs.
func ParseMergeURL(rawURL string) ([]string, error) {
	inputs, _, err := ParseMergeURLParts(rawURL)
	return inputs, err
}

// ParseMergeURLParts extracts additive and subtractive source names from an
// internal://merge?inputs=a,b,c&exclude=d,e URL. Older merge URLs without an
// exclude query remain valid and return an empty exclude list.
func ParseMergeURLParts(rawURL string) (inputs []string, exclude []string, err error) {
	if !strings.HasPrefix(rawURL, InternalMergeScheme+"?") {
		return nil, nil, fmt.Errorf("not a merge URL: %q", rawURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse merge URL: %w", err)
	}
	q := u.Query()
	inputs = splitMergeURLList(q.Get("inputs"))
	if len(inputs) == 0 {
		return nil, nil, fmt.Errorf("merge URL inputs list is empty: %q", rawURL)
	}
	exclude = splitMergeURLList(q.Get("exclude"))
	if err := validateSignedMergeRefs("url", inputs, exclude); err != nil {
		return nil, nil, err
	}
	return inputs, exclude, nil
}

func splitMergeURLList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// historyLabelForExpand renders the suffix for a retention window
// name, matching the legacy inline cloning convention used by the
// old updateHistoryVariants code path. Windows that are an exact
// number of days become "_Nd"; windows that are an exact number of
// hours (under 24h) become "_Nh"; mixed day/hour windows are
// rendered as "_NdMh". Keeping the output identical to the legacy
// labels means the public URLs and blocklist-ipsets git history
// for existing variants (dshield_1d, viriback_7d, …) stay valid
// across the refactor.
func historyLabelForExpand(minutes int) string {
	if minutes >= 24*60 {
		days := minutes / (24 * 60)
		rem := minutes - days*(24*60)
		if rem == 0 {
			return fmt.Sprintf("_%dd", days)
		}
		return fmt.Sprintf("_%dd%dh", days, rem/60)
	}
	return fmt.Sprintf("_%dh", minutes/60)
}

// appendWindowNote augments the parent source's info text with a
// short trailing note describing the retention window so users
// browsing the public page understand what the variant represents
// without having to parse the URL.
func appendWindowNote(parentInfo string, minutes int) string {
	label := humanizeMinutes(minutes)
	note := fmt.Sprintf("Retention window: union of retained history snapshots observed in the last %s.", label)
	if strings.Contains(parentInfo, note) {
		return parentInfo
	}
	if parentInfo == "" {
		return note
	}
	return parentInfo + " " + note
}

// humanizeMinutes converts a minute count into a compact human
// label: 1440 → "1 day", 10080 → "7 days", 60 → "1 hour".
func humanizeMinutes(minutes int) string {
	if minutes%(24*60) == 0 {
		days := minutes / (24 * 60)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if minutes%60 == 0 {
		hours := minutes / 60
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d minutes", minutes)
}
