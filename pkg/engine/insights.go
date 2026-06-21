package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/insights"
)

// insightsPayload is the on-disk JSON shape of <name>_insights.json.
// It is kept tiny by design: the frontend reads `items` verbatim and
// the catalog / audit trail lives inside each insight's Evidence map.
type insightsPayload struct {
	Name     string             `json:"name"`
	Computed int64              `json:"computed"`
	Items    []insights.Insight `json:"items"`
}

// insightsEngine is the process-wide cached insights.Engine. Derive is
// stateless and thread-safe so a single instance is reused across every
// feed and every run.
var getInsightsEngine = sync.OnceValue(func() *insights.Engine {
	return insights.NewEngine()
})

// writeInsights assembles the SignalSnapshot for the named feed, runs
// the deterministic insights engine, and writes the resulting JSON to
// the output dir. Insights errors are returned, but derived data is never critical.
func (e *Engine) writeInsights(name string, outDir string) error {
	snap, err := e.buildSignalSnapshot(name, outDir)
	if err != nil {
		return err
	}
	items := getInsightsEngine().Derive(snap)
	if items == nil {
		items = []insights.Insight{}
	}
	payload := insightsPayload{
		Name:     name,
		Computed: e.now().UTC().Unix(),
		Items:    items,
	}
	data, err := jsonMarshalTabIndent(payload)
	if err != nil {
		return err
	}
	return writeFileAtomicAt(
		filepath.Join(outDir, name+"_insights.json"),
		append(data, '\n'),
		generatedFileMode,
		e.feedProcessingTimestamp(name),
	)
}

// writeInsightsForFeeds regenerates insights for the same affected feed wave
// as the heavy comparison files, plus any public feed that is still missing an
// insights file. Insights read existing per-feed JSON artifacts; they must not
// become another whole-catalog sweep after an ordinary single-feed update.
//
// Errors are logged but not returned: one bad snapshot should not abort the heavy block.
func (e *Engine) writeInsightsForFeeds(ctx context.Context, updatedNames []string, outDir string) error {
	ctx = nonNilContext(ctx)
	targetNames := insightTargetNames(e.cfg, updatedNames, e.publicOutputNames(), outDir, e.outputDir())
	progress := e.beginActiveOperation("insights.write_feeds", "", "write", "feeds", int64(len(targetNames)))
	defer progress.Finish()
	for _, name := range targetNames {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if err := e.writeInsights(name, outDir); err != nil {
			e.logger.Warn("insights write failed", "set", name, "error", err)
		}
		progress.Add(1, int64(len(targetNames)), nil)
	}
	return nil
}

func insightTargetNames(cfg *config.Config, updatedNames []string, outputNames []string, stageDir string, liveDir string) []string {
	if len(outputNames) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	targets := targetFeedsForFanOut(cfg, updatedNames, outputNames)
	out := make([]string, 0, len(targets))
	for _, name := range targets {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, name := range outputNames {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		rel := name + "_insights.json"
		if insightFileExists(stageDir, rel) || insightFileExists(liveDir, rel) {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func insightFileExists(dir string, rel string) bool {
	if dir == "" || rel == "" {
		return false
	}
	return fileExists(filepath.Join(dir, rel))
}

// buildSignalSnapshot reads every per-feed artifact already produced by
// the engine and assembles the SignalSnapshot the insights package
// expects. It never touches the filesystem for anything other than
// files the engine itself writes during the same run.
func (e *Engine) buildSignalSnapshot(name string, outDir string) (insights.SignalSnapshot, error) {
	entry := e.EntrySnapshot(name)
	if entry == nil {
		return insights.SignalSnapshot{}, fmt.Errorf("no cache entry for %q", name)
	}
	now := e.now().UTC().Unix()
	snap := insights.SignalSnapshot{
		Name:              name,
		Category:          entry.Category,
		TrackedSinceTS:    entry.StartedDate,
		SnapshotTS:        now,
		TotalIPs:          entry.UniqueIPs,
		LastUpdatedTS:     entry.SourceDate,
		ConfiguredFreqMin: entry.FrequencyMinutes,
		DownloadFailures:  entry.DownloadFailures,
		ClockSkewSeconds:  entry.ClockSkewSeconds,
	}

	// Time series from the bash-compatible history ledgers. Readers clamp
	// to WebChartsEntries, the canonical public chart window. The insights
	// hot path prefers the already-bounded staged/live public artifacts so
	// it does not rescan full history snapshots for every feed.
	snap.SizeSeries = e.readInsightsSizeSeries(name, outDir)
	snap.ChurnSeries = e.readInsightsChurnSeries(name, outDir, snap.SizeSeries)

	// Retention histograms from the pre-built retention.json.
	past, current := e.readRetentionHistograms(name)
	snap.AgeOfRemoved = past
	snap.AgeOfListed = current

	// Geographic composition from the default country provider.
	snap.TopCountries = e.readTopCountries(name, outDir)

	// Bogon share across every configured bogon provider — we use the
	// MAX share so a feed that overlaps any one bogon baseline is
	// reported, not averaged down by providers that miss the range.
	snap.BogonShare = e.readBogonShare(name, outDir)

	// ASN composition from the default ASN provider. Top ASNs are not
	// used by the current rules but are assembled anyway so future rules
	// can read them without another engine change.
	asnFacts := e.readASNFacts(name, outDir)
	snap.TopASNs = asnFacts.Top

	// Critical-infrastructure facts come from the reference-feed overlap
	// artifact, not ASN-wide classification. This keeps the insight aligned
	// with the public feed page and avoids reintroducing broad ASN warnings.
	infraFacts := e.readCriticalInfrastructureFacts(name, outDir)
	snap.InfraIPs = infraFacts.IPs
	snap.InfraShare = infraFacts.Share
	snap.InfraTiers = infraFacts.Tiers

	// Pairwise overlaps from _comparison.json plus category aggregates.
	overlaps, byCat := e.readOverlapFacts(name, outDir)
	snap.Overlaps = overlaps
	snap.OverlapsByCat = byCat

	return snap, nil
}

// readRetentionHistograms converts the engine's existing retention
// data (past and current maps) into the AgeHistogram shape the
// insights package consumes. The source is <lib_dir>/<name>/retention.json
// which is always rebuilt by updateRetention on every successful run.
func (e *Engine) readRetentionHistograms(name string) (past insights.AgeHistogram, current insights.AgeHistogram) {
	data, err := readFileInRoot(e.runtime.LibDir, filepath.Join(name, "retention.json"))
	if err != nil {
		return
	}
	var ret RetentionData
	if err := json.Unmarshal(data, &ret); err != nil {
		return
	}
	past = seriesToHistogram(ret.Past)
	current = seriesToHistogram(ret.Current)
	return
}

func seriesToHistogram(s RetentionSeries) insights.AgeHistogram {
	h := insights.AgeHistogram{
		BucketsHours: append([]int(nil), s.Hours...),
		Counts:       append([]uint64(nil), s.IPs...),
		Total:        s.Total,
	}
	// RetentionSeries is already sorted ascending by hours in
	// retentionSeries(); we still guard against malformed files from
	// older builds by re-sorting defensively.
	if !slices.IsSorted(h.BucketsHours) {
		type kv struct {
			h int
			c uint64
		}
		pairs := make([]kv, len(h.BucketsHours))
		for i := range h.BucketsHours {
			pairs[i] = kv{h.BucketsHours[i], h.Counts[i]}
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].h < pairs[j].h })
		for i, p := range pairs {
			h.BucketsHours[i] = p.h
			h.Counts[i] = p.c
		}
	}
	return h
}

// readTopCountries loads the geographic composition from the first
// available geo provider, in a stable preference order. We deliberately
// do not aggregate across providers: they disagree about where IPs
// live, and picking one authoritative provider is less confusing than
// publishing an average.
func (e *Engine) readTopCountries(name string, outDir string) []insights.CountryShare {
	provider := e.preferredGeoProvider()
	if provider == "" {
		return nil
	}
	data, err := readFirstExisting(geoCountryCandidatePaths(outDir, name, provider), geoCountryCandidatePaths(e.outputDir(), name, provider))
	if err != nil {
		return nil
	}
	var rows []CountryValue
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil
	}
	var total uint64
	for _, r := range rows {
		total += r.Value
	}
	if total == 0 {
		return nil
	}
	out := make([]insights.CountryShare, 0, len(rows))
	for _, r := range rows {
		out = append(out, insights.CountryShare{
			Code:  r.Code,
			Name:  r.Code, // file format does not include country name
			IPs:   r.Value,
			Share: float64(r.Value) / float64(total),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Share > out[j].Share })
	return out
}

// preferredGeoProvider returns the configured geolocation provider used for
// canonical summaries, insights, IP context, and entity pages. It falls back to
// catalog order for programmatic/test configs that do not set defaults.
func (e *Engine) preferredGeoProvider() string {
	if e == nil || e.cfg == nil {
		return ""
	}
	if provider := e.cfg.DefaultProviderForRole(config.UseGeoIP); provider != "" {
		return provider
	}
	for _, src := range e.cfg.SourcesWithUse(config.UseGeoIP) {
		return src.Name
	}
	return ""
}

// preferredASNProvider returns the configured ASN provider used for canonical
// summaries, insights, IP context, and entity pages. It falls back to catalog
// order for programmatic/test configs that do not set defaults.
func (e *Engine) preferredASNProvider() string {
	if e == nil || e.cfg == nil {
		return ""
	}
	if provider := e.cfg.DefaultProviderForRole(config.UseASN); provider != "" {
		return provider
	}
	for _, src := range e.cfg.SourcesWithUse(config.UseASN) {
		return src.Name
	}
	return ""
}

// asnFacts is the subset of the ASN JSON file the insights engine
// consumes.
type asnFacts struct {
	Top []insights.ASNShare
}

// readASNFacts loads the top ASN list for the feed. Share values are computed
// against feed_ips (the JSON invariant from writeASNComparisonFiles).
func (e *Engine) readASNFacts(name string, outDir string) asnFacts {
	provider := e.preferredASNProvider()
	if provider == "" {
		return asnFacts{}
	}
	data, err := readFirstExisting(asnCandidatePaths(outDir, name, provider), asnCandidatePaths(e.outputDir(), name, provider))
	if err != nil {
		return asnFacts{}
	}
	var payload struct {
		FeedIPs uint64 `json:"feed_ips"`
		ByASN   []struct {
			ASN   uint32 `json:"asn"`
			Name  string `json:"name"`
			Count uint64 `json:"count"`
		} `json:"by_asn"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return asnFacts{}
	}
	if payload.FeedIPs == 0 {
		return asnFacts{}
	}
	facts := asnFacts{}
	facts.Top = make([]insights.ASNShare, 0, len(payload.ByASN))
	for _, row := range payload.ByASN {
		facts.Top = append(facts.Top, insights.ASNShare{
			Number: row.ASN,
			Name:   row.Name,
			IPs:    row.Count,
			Share:  float64(row.Count) / float64(payload.FeedIPs),
		})
	}
	sort.Slice(facts.Top, func(i, j int) bool { return facts.Top[i].Share > facts.Top[j].Share })
	return facts
}

type criticalInfrastructureFacts struct {
	IPs   uint64
	Share float64
	Tiers []insights.InfraTier
}

func (e *Engine) readCriticalInfrastructureFacts(name string, outDir string) criticalInfrastructureFacts {
	if !e.IsCriticalInfrastructureTarget(name) {
		return criticalInfrastructureFacts{}
	}
	data, err := readFirstExisting(criticalInfrastructureCandidatePaths(outDir, name), criticalInfrastructureCandidatePaths(e.outputDir(), name))
	if err != nil {
		return criticalInfrastructureFacts{}
	}
	var payload struct {
		FeedIPs       uint64  `json:"feed_ips"`
		CriticalIPs   uint64  `json:"critical_ips"`
		Percent       float64 `json:"percent"`
		ProviderSetID string  `json:"provider_set_id"`
		Tiers         []struct {
			Tier        string  `json:"tier"`
			CriticalIPs uint64  `json:"critical_ips"`
			Percent     float64 `json:"percent"`
			Providers   int     `json:"providers"`
		} `json:"tiers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return criticalInfrastructureFacts{}
	}
	currentProviderSetID := e.CriticalInfrastructureProviderSetID()
	if currentProviderSetID == "" || payload.ProviderSetID != currentProviderSetID {
		return criticalInfrastructureFacts{}
	}
	out := criticalInfrastructureFacts{IPs: payload.CriticalIPs}
	if payload.FeedIPs > 0 {
		out.Share = float64(payload.CriticalIPs) / float64(payload.FeedIPs)
	} else if payload.Percent > 0 {
		out.Share = payload.Percent / 100
	}
	out.Tiers = make([]insights.InfraTier, 0, len(payload.Tiers))
	for _, tier := range payload.Tiers {
		share := tier.Percent / 100
		if payload.FeedIPs > 0 {
			share = float64(tier.CriticalIPs) / float64(payload.FeedIPs)
		}
		out.Tiers = append(out.Tiers, insights.InfraTier{
			Tier:      tier.Tier,
			IPs:       tier.CriticalIPs,
			Share:     share,
			Providers: tier.Providers,
		})
	}
	return out
}

// readBogonShare loads every configured bogon provider's per-feed JSON
// and returns the highest share observed across providers. Using the
// max (rather than the union or an average) means a feed counts as
// "bogon-present" as long as at least one bogon dataset identifies an
// overlap, without double-counting IPs that appear in multiple bogon
// datasets.
func (e *Engine) readBogonShare(name string, outDir string) float64 {
	if e == nil || e.cfg == nil {
		return 0
	}
	var best float64
	for _, src := range e.cfg.SourcesWithUse(config.UseBogons) {
		data, err := readFirstExisting(bogonCandidatePaths(outDir, name, src.Name), bogonCandidatePaths(e.outputDir(), name, src.Name))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			continue
		}
		var payload struct {
			FeedIPs  uint64  `json:"feed_ips"`
			BogonIPs uint64  `json:"bogon_ips"`
			Percent  float64 `json:"percent"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}
		if payload.FeedIPs == 0 {
			continue
		}
		share := float64(payload.BogonIPs) / float64(payload.FeedIPs)
		if share > best {
			best = share
		}
	}
	return best
}

// readOverlapFacts reads <name>_comparison.json and converts it into
// the SignalSnapshot's Overlaps and OverlapsByCat maps. Entries whose
// counterpart feed has no cache entry are skipped — the insights
// package cannot evaluate them without knowing their size.
func (e *Engine) readOverlapFacts(name string, outDir string) ([]insights.FeedOverlap, map[string]insights.CategoryStat) {
	data, err := readFirstExisting(singleCandidatePath(outDir, name+"_comparison.json"), singleCandidatePath(e.outputDir(), name+"_comparison.json"))
	if err != nil {
		return nil, nil
	}
	var rows []CompareRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, nil
	}
	self := e.state.EntrySnapshot(name)
	if self == nil || self.UniqueIPs == 0 {
		return nil, nil
	}
	overlaps := make([]insights.FeedOverlap, 0, len(rows))
	type catAccum struct {
		maxShare float64
		count    int
		example  string
	}
	byCat := make(map[string]catAccum)
	for _, row := range rows {
		if row.IPs == 0 {
			continue
		}
		ourShare := float64(row.Common) / float64(self.UniqueIPs)
		theirShare := float64(row.Common) / float64(row.IPs)
		other := e.state.EntrySnapshot(row.Name)
		older := false
		if other != nil && other.StartedDate > 0 && self.StartedDate > 0 {
			older = other.StartedDate < self.StartedDate
		}
		overlaps = append(overlaps, insights.FeedOverlap{
			Other:         row.Name,
			Category:      row.Category,
			OurShare:      ourShare,
			TheirShare:    theirShare,
			OlderThanThis: older,
		})
		cat := row.Category
		if cat == "" {
			cat = "uncategorized"
		}
		acc := byCat[cat]
		acc.count++
		if ourShare > acc.maxShare {
			acc.maxShare = ourShare
			acc.example = row.Name
		}
		byCat[cat] = acc
	}
	out := make(map[string]insights.CategoryStat, len(byCat))
	for cat, acc := range byCat {
		out[cat] = insights.CategoryStat{
			MaxShare:    acc.maxShare,
			FeedCount:   acc.count,
			ExampleFeed: acc.example,
		}
	}
	return overlaps, out
}

// parseInt64 parses a base-10 signed integer.
func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// parseUint64 parses a base-10 unsigned integer.
func parseUint64(s string) (uint64, error) {
	var v uint64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// readFirstExisting walks candidate path groups in order and returns the first
// file that opens cleanly. The error from the LAST attempted path is returned
// when none exist, so callers can distinguish "missing" (fs.ErrNotExist) from
// "permission denied" or "I/O error". Groups MUST be ordered
// most-preferred-first so staged files win over live files during a batch.
type rootedCandidatePath struct {
	rootDir string
	rel     string
}

func readFirstExisting(pathGroups ...[]rootedCandidatePath) ([]byte, error) {
	if len(pathGroups) == 0 {
		return nil, fs.ErrNotExist
	}
	var lastErr error
	for _, paths := range pathGroups {
		for _, candidate := range paths {
			data, err := readFileInRoot(candidate.rootDir, candidate.rel)
			if err == nil {
				return data, nil
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fs.ErrNotExist
	}
	return nil, lastErr
}

// geoCountryCandidatePaths returns the on-disk path for the per-feed
// geographic attribution snapshot. The provider is always the full
// source name from the unified config (e.g. "geolite2_country") and
// the writer joins it directly, so there is exactly one canonical
// path. Returned as a slice to keep the call site symmetrical with
// the asn/bogon helpers.
func geoCountryCandidatePaths(dir, name, provider string) []rootedCandidatePath {
	if dir == "" {
		return nil
	}
	return []rootedCandidatePath{{rootDir: dir, rel: name + "_" + provider + ".json"}}
}

// asnCandidatePaths returns the on-disk path for the per-feed ASN
// attribution snapshot. The writer always uses
// <feed>_asn_<source_name>.json so there is exactly one canonical
// path; the slice return type matches the geo/bogon helpers.
func asnCandidatePaths(dir, name, provider string) []rootedCandidatePath {
	if dir == "" {
		return nil
	}
	return []rootedCandidatePath{{rootDir: dir, rel: name + "_asn_" + provider + ".json"}}
}

// bogonCandidatePaths returns the on-disk paths to try when reading
// the per-feed bogon attribution snapshot. The current naming scheme
// is consistent (<feed>_bogons_<source_name>.json) so only one path
// is attempted, but the helper exists symmetrically with the geo and
// ASN readers so a future rename can be absorbed in one place.
func bogonCandidatePaths(dir, name, sourceName string) []rootedCandidatePath {
	if dir == "" {
		return nil
	}
	return []rootedCandidatePath{{rootDir: dir, rel: name + "_bogons_" + sourceName + ".json"}}
}

func criticalInfrastructureCandidatePaths(dir, name string) []rootedCandidatePath {
	if dir == "" {
		return nil
	}
	return []rootedCandidatePath{{rootDir: dir, rel: name + "_critical_infrastructure.json"}}
}
