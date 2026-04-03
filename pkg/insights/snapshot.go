package insights

// SignalSnapshot is the pre-assembled bundle of facts the engine passes
// to the insights Engine. Every field is derived from data the engine
// already produces (history.csv, retention.json, per-provider country
// and ASN JSON, comparison.json, cache entries). Rules only read this
// struct — they never touch the filesystem.
type SignalSnapshot struct {
	// Identification
	Name           string `json:"name"`
	Category       string `json:"category,omitempty"`
	TrackedSinceTS int64  `json:"tracked_since_ts,omitempty"`
	SnapshotTS     int64  `json:"snapshot_ts,omitempty"`

	// Size + churn time series. SizeSeries[i] is the unique IP count
	// after the i-th recorded update. ChurnSeries[i] holds the raw
	// add/remove/kept counts and the post-update size so rules can
	// derive churn = (added+removed)/size without re-scanning history.
	// Both arrays are clamped to the last WebChartsEntries points
	// (500 by default); rules must use the series length as N rather
	// than a time duration.
	SizeSeries  []SizePoint  `json:"size_series,omitempty"`
	ChurnSeries []ChurnPoint `json:"churn_series,omitempty"`

	// Currently-listed IP age histogram. AgeOfListed.Counts[i] is the
	// number of currently-listed IPs that have been listed for
	// approximately AgeOfListed.BucketsHours[i] hours. Built from the
	// engine's `current` retention series.
	AgeOfListed AgeHistogram `json:"age_of_listed"`

	// Removed-IP duration histogram. AgeOfRemoved.Counts[i] is the
	// number of IPs that were listed for approximately
	// AgeOfRemoved.BucketsHours[i] hours before being removed. Built
	// from the engine's `past` retention series.
	AgeOfRemoved AgeHistogram `json:"age_of_removed"`

	// Composition snapshot.
	TotalIPs     uint64         `json:"total_ips,omitempty"`
	TopCountries []CountryShare `json:"top_countries,omitempty"`
	TopASNs      []ASNShare     `json:"top_asns,omitempty"`
	BogonShare   float64        `json:"bogon_share,omitempty"`
	InfraIPs     uint64         `json:"infra_ips,omitempty"`
	InfraShare   float64        `json:"infra_share,omitempty"`
	InfraTiers   []InfraTier    `json:"infra_tiers,omitempty"`

	// Overlap facts. Overlaps holds pairwise results with every other
	// tracked feed; OverlapsByCat holds the maximum overlap share of
	// this feed's IPs seen in any feed of each other category, plus
	// the number of candidate feeds in each category so the
	// cross-category rule can apply its sample-size guard.
	Overlaps      []FeedOverlap           `json:"overlaps,omitempty"`
	OverlapsByCat map[string]CategoryStat `json:"overlaps_by_cat,omitempty"`

	// Operational health (used by freshness rules; not yet implemented
	// but populated for future rules without another engine change).
	LastUpdatedTS     int64 `json:"last_updated_ts,omitempty"`
	ConfiguredFreqMin int   `json:"configured_freq_min,omitempty"`
	DownloadFailures  int   `json:"download_failures,omitempty"`
	ClockSkewSeconds  int64 `json:"clock_skew_seconds,omitempty"`
}

// SizePoint is one sample in the size time series.
type SizePoint struct {
	TS   int64  `json:"ts"`
	Size uint64 `json:"size"`
}

// ChurnPoint is one sample in the churn time series. Size is the size
// AFTER this update was applied; churn is computed at rule time as
// (Added+Removed)/Size.
type ChurnPoint struct {
	TS      int64  `json:"ts"`
	Added   uint64 `json:"added"`
	Removed uint64 `json:"removed"`
	Kept    uint64 `json:"kept"`
	Size    uint64 `json:"size"`
}

// AgeHistogram is a bucketed age distribution. BucketsHours is sorted
// ascending and has the same length as Counts. Total is pre-computed
// as sum(Counts) so rules can apply sample-size guards without a
// secondary loop.
type AgeHistogram struct {
	BucketsHours []int    `json:"buckets_hours,omitempty"`
	Counts       []uint64 `json:"counts,omitempty"`
	Total        uint64   `json:"total,omitempty"`
}

// CountryShare is one row of the geographic composition, sorted by
// share descending when assembled.
type CountryShare struct {
	Code  string  `json:"code"`
	Name  string  `json:"name,omitempty"`
	IPs   uint64  `json:"ips"`
	Share float64 `json:"share"`
}

// ASNShare is one row of the ASN composition.
type ASNShare struct {
	Number  uint32  `json:"number"`
	Name    string  `json:"name,omitempty"`
	IPs     uint64  `json:"ips"`
	Share   float64 `json:"share"`
	IsBogon bool    `json:"is_bogon,omitempty"`
}

// InfraTier is one tier summary from the critical-infrastructure overlap
// artifact. Rules use it to distinguish hard operational dependencies from
// softer or policy-dependent provider ranges.
type InfraTier struct {
	Tier      string  `json:"tier"`
	IPs       uint64  `json:"ips"`
	Share     float64 `json:"share"`
	Providers int     `json:"providers"`
}

// FeedOverlap is one pairwise comparison row. OurShare is the fraction
// of THIS feed that also appears in the other feed; TheirShare is the
// fraction of the OTHER feed that appears in this one. OlderThanThis
// lets the subset_of rule require that the candidate superset was
// tracked before us (so a derivative can only fire toward the parent).
type FeedOverlap struct {
	Other         string  `json:"other"`
	Category      string  `json:"category,omitempty"`
	OurShare      float64 `json:"our_share"`
	TheirShare    float64 `json:"their_share"`
	OlderThanThis bool    `json:"older_than_this,omitempty"`
}

// CategoryStat aggregates cross-category overlap facts. MaxShare is the
// highest OurShare across every overlap with a feed in this category;
// FeedCount is the number of distinct feeds in that category that were
// compared (used for the sample-size guard on cross_category_overlap).
// ExampleFeed names one representative feed at the max share.
type CategoryStat struct {
	MaxShare    float64 `json:"max_share"`
	FeedCount   int     `json:"feed_count"`
	ExampleFeed string  `json:"example_feed,omitempty"`
}
