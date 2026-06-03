package markdown

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/firehol/update-ipsets/internal/fileutil"
	"github.com/firehol/update-ipsets/pkg/enrichment"
)

type FeedArtifactReader struct {
	dir                  string
	preferredASNProvider string
	preferredGEOProvider string
}

type FeedArtifactReaderOption func(*FeedArtifactReader)

func WithPreferredASNProvider(provider string) FeedArtifactReaderOption {
	return func(r *FeedArtifactReader) {
		r.preferredASNProvider = strings.TrimSpace(provider)
	}
}

func WithPreferredGEOProvider(provider string) FeedArtifactReaderOption {
	return func(r *FeedArtifactReader) {
		r.preferredGEOProvider = strings.TrimSpace(provider)
	}
}

func NewFeedArtifactReader(dir string, opts ...FeedArtifactReaderOption) *FeedArtifactReader {
	r := &FeedArtifactReader{dir: dir}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *FeedArtifactReader) BuildFeedContext(name string) (*FeedPageContext, error) {
	ctx := &FeedPageContext{Name: name}

	meta, err := r.readFeedMetadata(name)
	if err != nil {
		return nil, fmt.Errorf("metadata for %s: %w", name, err)
	}
	r.populateFromMetadata(ctx, meta)

	if insights, err := r.readInsights(name); err == nil && insights != nil {
		for _, item := range insights.Items {
			ctx.Insights = append(ctx.Insights, InsightContext{
				Code:        item.Code,
				Section:     item.Section,
				Headline:    item.Headline,
				Evidence:    item.Evidence,
				Methodology: item.Methodology,
			})
		}
	} else if err != nil && !os.IsNotExist(err) {
		slog.Debug("markdown insights read failed", "feed", name, "error", err)
	}

	if critical, err := r.readCritical(name); err == nil && critical != nil {
		ctx.Critical = critical
	} else if err != nil && !os.IsNotExist(err) {
		slog.Debug("markdown critical read failed", "feed", name, "error", err)
	}

	if asnProviders, err := r.readASNProviders(name); err == nil {
		ctx.ASN = asnProviders
	} else if err != nil && !os.IsNotExist(err) {
		slog.Debug("markdown ASN read failed", "feed", name, "error", err)
	}

	if geoProviders, err := r.readGEOProviders(name, meta); err == nil {
		ctx.GEO = geoProviders
	} else if err != nil && !os.IsNotExist(err) {
		slog.Debug("markdown GEO read failed", "feed", name, "error", err)
	}

	if bogonProviders, err := r.readBogonProviders(name); err == nil {
		ctx.Bogons = bogonProviders
	} else if err != nil && !os.IsNotExist(err) {
		slog.Debug("markdown bogon read failed", "feed", name, "error", err)
	}

	r.populateBehavior(ctx, name)

	if retention, err := r.readRetention(name); err == nil && retention != nil {
		ctx.Retention = retention
	} else if err != nil && !os.IsNotExist(err) {
		slog.Debug("markdown retention read failed", "feed", name, "error", err)
	}

	if comparison, err := r.readComparison(name, ctx.IPs); err == nil {
		ctx.Comparison = comparison
	} else if err != nil && !os.IsNotExist(err) {
		slog.Debug("markdown comparison read failed", "feed", name, "error", err)
	}

	return ctx, nil
}

func (r *FeedArtifactReader) path(rel string) string {
	return filepath.Join(r.dir, rel)
}

func (r *FeedArtifactReader) readFile(rel string) ([]byte, error) {
	return fileutil.ReadFileUnderRoot(r.dir, rel)
}

func (r *FeedArtifactReader) readMatchedFile(path string) ([]byte, error) {
	rel, err := filepath.Rel(r.dir, path)
	if err != nil {
		return nil, err
	}
	return r.readFile(rel)
}

func (r *FeedArtifactReader) readFeedMetadata(name string) (map[string]any, error) {
	data, err := r.readFile(name + ".json")
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (r *FeedArtifactReader) populateFromMetadata(ctx *FeedPageContext, m map[string]any) {
	ctx.Category = strVal(m["category"])
	ctx.Maintainer = strVal(m["maintainer"])
	ctx.MaintainerURL = strVal(m["maintainer_url"])
	ctx.License = strVal(m["license"])
	ctx.OfficialName = strVal(m["official_name"])
	ctx.ShortDescription = strVal(m["short_description"])
	ctx.Info = strVal(m["info"])
	ctx.Source = strVal(m["source"])
	ctx.Format = strVal(m["format"])
	ctx.Output = strVal(m["output"])
	ctx.IPV = strVal(m["ipv"])
	ctx.Hash = strVal(m["hash"])
	ctx.Processor = strVal(m["processor"])
	ctx.Downloader = strVal(m["downloader"])
	ctx.Entries = intVal(m["entries"])
	ctx.EntriesMin = intVal(m["entries_min"])
	ctx.EntriesMax = intVal(m["entries_max"])
	ctx.IPs = uintVal(m["ips"])
	ctx.IPsMin = uintVal(m["ips_min"])
	ctx.IPsMax = uintVal(m["ips_max"])
	ctx.Aggregation = intVal(m["aggregation"])
	ctx.Frequency = intVal(m["frequency"])
	ctx.Started = int64Val(m["started"])
	ctx.Updated = int64Val(m["updated"])
	ctx.Processed = int64Val(m["processed"])
	ctx.Checked = int64Val(m["checked"])
	ctx.AvgUpdateMins = firstIntVal(m, "average_update", "avg_update_mins", "average_update_mins")
	ctx.MinUpdateMins = firstIntVal(m, "min_update", "min_update_mins")
	ctx.MaxUpdateMins = firstIntVal(m, "max_update", "max_update_mins")
	ctx.DontRedistribute = boolVal(m["dont_redistribute"])
	ctx.Hidden = boolVal(m["hidden"])
	if raw, ok := m["current_status"].(map[string]any); ok {
		ctx.CurrentStatus = decodeEnrichmentValue[enrichment.CurrentStatus](raw)
	}
	if raw, ok := m["enrichment"].(map[string]any); ok {
		ctx.Enrichment = decodeEnrichmentValue[enrichment.Feed](raw)
	}

	if h, ok := m["health"].(map[string]any); ok {
		ctx.HealthClass = strVal(h["class"])
		ctx.HealthStatus = healthStatusDescription(h)
	}

	ctx.RotationMedian = float64Val(m["rotation_median_pct"])
	ctx.RotationP75 = float64Val(m["rotation_p75_pct"])
	ctx.RotationSamples = intVal(m["rotation_samples"])
	ctx.ChangeRatioMedian = float64Val(m["change_ratio_median_pct"])
	ctx.ChangeRatioP75 = float64Val(m["change_ratio_p75_pct"])
	ctx.ChangeRatioSamples = intVal(m["change_ratio_samples"])

	if usedFor, ok := m["used_for"].([]any); ok {
		for _, u := range usedFor {
			if s, ok := u.(string); ok {
				ctx.UsedFor = append(ctx.UsedFor, s)
			}
		}
	}

	if mergeIncluded, ok := m["merge_included"].([]any); ok {
		for _, mi := range mergeIncluded {
			if entry, ok := mi.(map[string]any); ok {
				ctx.MergeIncluded = append(ctx.MergeIncluded, MergeInput{
					Name:        strVal(entry["name"]),
					Role:        strVal(entry["role"]),
					Reason:      strVal(entry["reason"]),
					HealthClass: strVal(entry["health_class"]),
					Enabled:     boolVal(entry["enabled"]),
					HasFeedBody: boolVal(entry["has_feed_body"]),
				})
			}
		}
	}

	if mergeSubtracted, ok := m["merge_subtracted"].([]any); ok {
		for _, mi := range mergeSubtracted {
			if entry, ok := mi.(map[string]any); ok {
				ctx.MergeSubtracted = append(ctx.MergeSubtracted, MergeInput{
					Name:        strVal(entry["name"]),
					Role:        strVal(entry["role"]),
					Reason:      strVal(entry["reason"]),
					HealthClass: strVal(entry["health_class"]),
					Enabled:     boolVal(entry["enabled"]),
					HasFeedBody: boolVal(entry["has_feed_body"]),
				})
			}
		}
	}
}

// healthStatusDescription mirrors the UI's feedHealthDescription for the
// three classes we surface as status panels (archived, unmaintained,
// empty). Other classes return an empty string so the template can omit
// the status block. Keep this in sync with `ui/src/lib/feed-health.ts`
// — both layers describe the same deterministic detection.
func healthStatusDescription(h map[string]any) string {
	if h == nil {
		return ""
	}
	class := strVal(h["class"])
	switch class {
	case "archived":
		archivalThreshold := intVal(h["archival_threshold_mins"])
		timeSinceFailure := intVal(h["time_since_failure_mins"])
		threshold := intVal(h["threshold_mins"])
		if archivalThreshold > 0 && timeSinceFailure > 0 && threshold > 0 && timeSinceFailure > threshold {
			return "Feed has remained unavailable for " +
				formatMinutes(timeSinceFailure-threshold) +
				" beyond the unavailable threshold; archival threshold " +
				formatMinutes(archivalThreshold) + "."
		}
		return "Feed has remained unavailable long enough to stop automatic retries."
	case "unmaintained":
		threshold := intVal(h["unmaintained_threshold_mins"])
		since := intVal(h["time_since_last_change_mins"])
		if threshold > 0 && since > 0 {
			return "No observed content change for " +
				formatMinutes(since) +
				"; unmaintained threshold " +
				formatMinutes(threshold) + "."
		}
		return "Feed is older than its observed maintenance cadence."
	case "empty":
		return "Download works, but the current feed has zero entries."
	}
	return ""
}

func formatMinutes(minutes int) string {
	if minutes <= 0 {
		return "—"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes < 60*24 {
		return fmt.Sprintf("%dh", (minutes+30)/60)
	}
	return fmt.Sprintf("%dd", (minutes+(60*12))/(60*24))
}

func decodeEnrichmentValue[T any](raw any) *T {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return &out
}
