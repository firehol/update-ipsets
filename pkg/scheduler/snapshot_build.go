package scheduler

import (
	"container/heap"
	"fmt"
	"github.com/firehol/update-ipsets/internal/fileutil"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Item struct {
	Name             string    `json:"name"`
	Kind             string    `json:"kind"`
	Hidden           bool      `json:"hidden,omitempty"`
	Enabled          bool      `json:"enabled"`
	HealthClass      string    `json:"health_class,omitempty"`
	FrequencyMinutes int       `json:"frequency_minutes"`
	Failures         int       `json:"failures"`
	CheckedAt        time.Time `json:"checked_at,omitempty"`
	NextDue          time.Time `json:"next_due,omitempty"`
	Detail           string    `json:"detail,omitempty"`
}

type Snapshot struct {
	GeneratedAt time.Time `json:"generated_at"`
	Items       []Item    `json:"items"`
}

func BuildSnapshot(cfg *config.Config, rt engine.Runtime, entries []cache.Entry, enableAll bool, now time.Time) Snapshot {
	index := make(map[string]cache.Entry, len(entries))
	for _, entry := range entries {
		index[entry.Name] = entry
	}
	criticalProviderSetChanged := engine.CriticalInfrastructureProviderSetChangedForSnapshot(cfg, rt)

	items := make([]Item, 0, len(cfg.Sources))
	for _, name := range config.SortedSourceNames(cfg) {
		src := cfg.Sources[name]
		entry := snapshotEntryForSource(name, src, index)
		kind := sourceKind(src)
		sourcePathForCheck := sourcePathForSnapshot(rt, name, src)
		enabled := engine.EffectiveSourceEnabled(cfg, rt, name, enableAll)
		// Hidden sources are still tracked by the scheduler (the admin
		// endpoint surfaces them) but the public schedule endpoint
		// filters them out at serve time. Surfacing them in BuildSnapshot
		// keeps the admin view as a strict superset of the public view.
		frequency := scheduledFrequencyMinutes(src)
		health := feedhealth.Classify(entryViewForHealth(entry), src, feedhealth.PolicyFromRuntime(cfg.Runtime), now)
		next, detail := nextDue(entry, frequency, sourcePathForCheck, now, rt.IgnoreRepeatingDownloadErrors, health.Class == feedhealth.ClassUnmaintained, health.Class == feedhealth.ClassArchived)
		if staticSourceMaterializationChanged(src, sourcePathForCheck) {
			next = now
			detail = "static source config changed"
		}
		if criticalProviderSetChanged && src.HasUse(config.UseCriticalInfrastructure) {
			next = now
			detail = detailCriticalProviderSetChanged
		}
		items = append(items, Item{
			Name:             name,
			Kind:             kind,
			Hidden:           src.Hidden,
			Enabled:          enabled,
			HealthClass:      string(health.Class),
			FrequencyMinutes: frequency,
			Failures:         entry.DownloadFailures,
			CheckedAt:        unixTime(entry.CheckedDate),
			NextDue:          next,
			Detail:           detail,
		})
	}
	pq := make(itemHeap, 0, len(items))
	for _, item := range items {
		heap.Push(&pq, item)
	}
	sorted := make([]Item, 0, len(items))
	for pq.Len() > 0 {
		sorted = append(sorted, heap.Pop(&pq).(Item))
	}
	return Snapshot{GeneratedAt: now, Items: sorted}
}

func BuildArtifactItems(cfg *config.Config, rt engine.Runtime, entries []cache.Entry, enableAll bool, now time.Time) []Item {
	if cfg == nil || len(cfg.Artifacts) == 0 {
		return nil
	}
	index := make(map[string]cache.Entry, len(entries))
	for _, entry := range entries {
		index[entry.Name] = entry
	}
	items := make([]Item, 0, len(cfg.Artifacts))
	for _, name := range config.SortedArtifactNames(cfg) {
		artifact := cfg.ArtifactByName(name)
		if artifact == nil {
			continue
		}
		sourcePath := filepath.Join(rt.LibDir, "artifacts", name, "source")
		health := feedhealth.Classify(entryViewForHealth(index[name]), &config.Source{Frequency: artifact.Frequency}, feedhealth.PolicyFromRuntime(cfg.Runtime), now)
		next, detail := nextDue(index[name], artifact.Frequency, sourcePath, now, rt.IgnoreRepeatingDownloadErrors, health.Class == feedhealth.ClassUnmaintained, false)
		items = append(items, Item{
			Name:             name,
			Kind:             "artifact",
			Enabled:          engine.EffectiveArtifactEnabled(cfg, rt, name, enableAll),
			FrequencyMinutes: artifact.Frequency,
			Failures:         index[name].DownloadFailures,
			CheckedAt:        unixTime(index[name].CheckedDate),
			NextDue:          next,
			Detail:           detail,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Enabled != items[j].Enabled {
			return items[i].Enabled
		}
		if items[i].NextDue.IsZero() != items[j].NextDue.IsZero() {
			return items[i].NextDue.IsZero()
		}
		if !items[i].NextDue.Equal(items[j].NextDue) {
			return items[i].NextDue.Before(items[j].NextDue)
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func DueNames(snapshot Snapshot, now time.Time) []string {
	out := make([]string, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		if !item.Enabled {
			continue
		}
		if item.NextDue.IsZero() || !item.NextDue.After(now) {
			out = append(out, item.Name)
		}
	}
	return out
}

type itemHeap []Item

func (h itemHeap) Len() int      { return len(h) }
func (h itemHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h itemHeap) Less(i, j int) bool {
	if h[i].Enabled != h[j].Enabled {
		return h[i].Enabled
	}
	if h[i].NextDue.IsZero() != h[j].NextDue.IsZero() {
		return h[i].NextDue.IsZero()
	}
	if !h[i].NextDue.Equal(h[j].NextDue) {
		return h[i].NextDue.Before(h[j].NextDue)
	}
	return h[i].Name < h[j].Name
}

func (h *itemHeap) Push(x any) {
	*h = append(*h, x.(Item))
}

func (h *itemHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func nextDue(entry cache.Entry, minutes int, sourcePath string, now time.Time, ignoreRepeating int, reachedUnmaintained bool, archived bool) (time.Time, string) {
	// frequency:0 marks items that do not own an independent cadence.
	// Return a far-future deadline so DueNames never picks them up; if
	// they have never been checked at all the scheduler still runs them
	// once so the first committed body can materialize.
	if archived {
		return now.Add(365 * 24 * time.Hour), "archived (automatic retries disabled)"
	}
	if minutes == 0 {
		if entry.CheckedDate == 0 {
			return now, "never checked (static source)"
		}
		return now.Add(365 * 24 * time.Hour), "static source (never expires)"
	}
	if minutes < 1 {
		minutes = 1
	}
	origMinutes := minutes
	inc := (minutes + 50) / 100
	if minutes <= 30 {
		inc = 0
	}
	if inc > 10 {
		inc = 10
	}
	minutes += inc

	base := entry.CheckedDate
	if !validJSONUnixSeconds(base) {
		base = 0
	}
	if base == 0 && fileExists(sourcePath) {
		if info, err := os.Stat(sourcePath); err == nil {
			base = info.ModTime().UTC().Unix()
		}
	}
	fails := entry.DownloadFailures
	if fails > 0 {
		minutes = failureRetryDelayMinutes(origMinutes, fails, reachedUnmaintained)
	}
	if base == 0 {
		return now, "never checked"
	}
	next := time.Unix(base, 0).UTC().Add(time.Duration(minutes) * time.Minute)
	if next.Year() < 0 || next.Year() > 9999 {
		return now, "due now"
	}
	if now.Before(next) {
		if fails > 0 {
			state := "pre-unmaintained"
			if reachedUnmaintained {
				state = "unmaintained"
			}
			return next, fmt.Sprintf("retry in %d mins after %d hard failures (%s, base %d mins)", int(next.Sub(now).Minutes()), fails, state, origMinutes)
		}
		return next, fmt.Sprintf("next check in %d mins (base %d mins)", int(next.Sub(now).Minutes()), origMinutes)
	}
	if fails > 0 {
		return now, "retry due now"
	}
	return now, "due now"
}

func failureRetryDelayMinutes(configuredMinutes, failures int, reachedUnmaintained bool) int {
	if configuredMinutes < 1 {
		configuredMinutes = 1
	}
	if failures < 1 {
		return configuredMinutes
	}
	delay := (configuredMinutes + 15) / 16
	if delay < 1 {
		delay = 1
	}
	for i := 1; i < failures; i++ {
		if delay >= 43200 {
			return 43200
		}
		delay *= 2
		if !reachedUnmaintained && delay >= configuredMinutes {
			return configuredMinutes
		}
	}
	if !reachedUnmaintained && delay > configuredMinutes {
		return configuredMinutes
	}
	if delay > 43200 {
		return 43200
	}
	return delay
}

func entryViewForHealth(entry cache.Entry) *cache.Entry {
	clone := entry
	return &clone
}

func nextWait(now time.Time, groups ...[]Item) time.Duration {
	found := false
	best := time.Minute
	for _, items := range groups {
		for _, item := range items {
			if !item.Enabled {
				continue
			}
			if item.NextDue.IsZero() || !item.NextDue.After(now) {
				return 0
			}
			wait := item.NextDue.Sub(now)
			if wait < time.Second {
				return time.Second
			}
			if !found || wait < best {
				best = wait
				found = true
			}
		}
	}
	if !found {
		return time.Minute
	}
	return best
}
func (r *Runner) storeSnapshot(snapshot Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := !snapshotItemsEqual(r.snapshot.Items, snapshot.Items)
	r.snapshot = snapshot
	if !changed {
		return
	}
	if err := SaveSnapshot(r.statePath, snapshot); err != nil {
		r.logger.Error("failed to persist scheduler snapshot", "error", err)
		r.metrics.recordSnapshotPersistError()
	}
}

func (r *Runner) currentSnapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return Snapshot{
		GeneratedAt: r.snapshot.GeneratedAt,
		Items:       append([]Item(nil), r.snapshot.Items...),
	}
}

func snapshotItemsEqual(left, right []Item) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func healthTransitionNames(prev, next Snapshot) []string {
	names := healthTransitionDetails(prev, next)
	if len(names) == 0 {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, t := range names {
		out = append(out, t.Feed)
	}
	return out
}

func healthTransitionDetails(prev, next Snapshot) []HealthTransition {
	if len(next.Items) == 0 {
		return nil
	}
	if len(prev.Items) == 0 {
		return nil
	}
	prevHealth := make(map[string]string, len(prev.Items))
	for _, item := range prev.Items {
		prevHealth[item.Name] = item.HealthClass
	}
	now := next.GeneratedAt
	out := make([]HealthTransition, 0)
	for _, item := range next.Items {
		if item.Kind == "artifact" || item.Name == "" {
			continue
		}
		previous, ok := prevHealth[item.Name]
		if ok && previous == item.HealthClass {
			continue
		}
		out = append(out, HealthTransition{
			Feed:      item.Name,
			FromClass: previous,
			ToClass:   item.HealthClass,
			At:        now,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Feed < out[j].Feed
	})
	return out
}

func unixTime(value int64) time.Time {
	if !validJSONUnixSeconds(value) {
		return time.Time{}
	}
	return time.Unix(value, 0).UTC()
}

func validJSONUnixSeconds(value int64) bool {
	return value > 0 && value <= maxJSONUnixSeconds
}

func fileExists(path string) bool {
	return fileutil.Exists(path)
}

func scheduledFrequencyMinutes(src *config.Source) int {
	if src == nil {
		return 0
	}
	return src.Frequency
}

func sourcePathForSnapshot(rt engine.Runtime, name string, src *config.Source) string {
	if src != nil && src.HasUse(config.UseASN) {
		return filepath.Join(rt.LibDir, "asn", name, "source")
	}
	if src != nil && src.HasUse(config.UseGeoIP) {
		return filepath.Join(rt.LibDir, "geolocation", name+".source")
	}
	return filepath.Join(rt.BaseDir, name+".source")
}

func staticSourceMaterializationChanged(src *config.Source, sourcePath string) bool {
	if src == nil || len(src.Static) == 0 || sourcePath == "" {
		return false
	}
	want := strings.Join(src.Static, "\n") + "\n"
	got, err := os.ReadFile(sourcePath)
	if err != nil {
		return true
	}
	return string(got) != want
}

// sourceKind projects the product feed family into the smaller
// operator-facing taxonomy used by the admin runtime queues.
func sourceKind(src *config.Source) string {
	if src == nil {
		return "source"
	}
	switch src.Provenance {
	case config.ProvenanceSecondaryMerge:
		return "merge"
	case config.ProvenanceSecondaryRetention:
		return "retention"
	}
	switch {
	case src.HasUse(config.UseASN):
		return "asn"
	case src.HasUse(config.UseGeoIP):
		return "geolocation"
	case src.HasUse(config.UseBogons):
		return "bogon"
	}
	return "source"
}

// snapshotEntryForSource returns the cache entry for a source.
// Pre-unification the bash-era "split" output mode needed a
// merge step to pick the fresher of the _ip/_net children, but
// split is no longer supported — only ipset and netset — so this
// is now a straight lookup.
func snapshotEntryForSource(name string, _ *config.Source, index map[string]cache.Entry) cache.Entry {
	return index[name]
}
