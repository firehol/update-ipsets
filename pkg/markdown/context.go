package markdown

import (
	"time"

	"github.com/firehol/update-ipsets/pkg/enrichment"
)

type FeedPageContext struct {
	Name             string
	Category         string
	Maintainer       string
	MaintainerURL    string
	License          string
	OfficialName     string
	ShortDescription string
	CurrentStatus    *enrichment.CurrentStatus
	Enrichment       *enrichment.Feed
	Info             string
	Source           string
	Format           string
	Output           string
	IPV              string
	Aggregation      int
	Frequency        int
	Hash             string
	Processor        string
	Downloader       string
	Entries          int
	EntriesMin       int
	EntriesMax       int
	IPs              uint64
	IPsMin           uint64
	IPsMax           uint64
	Started          int64
	Updated          int64
	Processed        int64
	Checked          int64
	HealthClass      string
	// HealthStatus is a short human-readable description of the
	// health-derived status when the class warrants a status panel
	// (archived, unmaintained, empty). Empty when the class does not
	// merit surfacing. The site renders the equivalent block via
	// SectionStatus; this field mirrors that text into the markdown.
	HealthStatus string
	AvgUpdateMins    int
	MinUpdateMins    int
	MaxUpdateMins    int

	RotationMedian     float64
	RotationP75        float64
	RotationSamples    int
	ChangeRatioMedian  float64
	ChangeRatioP75     float64
	ChangeRatioSamples int

	UsedFor          []string
	MergeIncluded    []MergeInput
	MergeSubtracted  []MergeInput
	DontRedistribute bool
	Hidden           bool

	Insights []InsightContext

	Critical *CriticalContext
	ASN      []ASNProviderContext
	GEO      []GEOProviderContext
	Bogons   []BogonProviderContext

	Activity *ActivityContext
	Cadence  []CadenceBin

	Retention *RetentionContext

	Comparison []CompareRowContext
}

func (c *FeedPageContext) DisplayFormat() string {
	if c == nil {
		return ""
	}
	if c.Format != "" {
		return c.Format
	}
	return c.Output
}

type MergeInput struct {
	Name        string
	Role        string
	Reason      string
	HealthClass string
	Enabled     bool
	HasFeedBody bool
}

type InsightContext struct {
	Code        string
	Section     string
	Headline    string
	Evidence    map[string]any
	Methodology string
}

type ActivityContext struct {
	Rows           []ActivityRow
	Resolution     string
	Rollup         bool
	RawChanges     int
	HistorySamples int
	LatestIPs      uint64
	WindowNote     string
	RollupNote     string
}

type ActivityRow struct {
	Timestamp time.Time
	IPs       float64
	Entries   float64
	Added     float64
	Removed   float64
	ChurnPct  float64
}

type CriticalContext struct {
	FeedIPs     uint64
	CriticalIPs uint64
	Percent     float64
	Complete    bool
	Tiers       []CriticalTierContext
	Providers   []CriticalProviderContext
	ASNContext  *CriticalASNContext
}

type CriticalTierContext struct {
	Tier        string
	CriticalIPs uint64
	Percent     float64
	Providers   int
}

type CriticalProviderContext struct {
	Name        string
	FeedIPs     uint64
	CriticalIPs uint64
	Percent     float64
}

type CriticalASNContext struct {
	Provider string
	FeedIPs  uint64
	IPs      uint64
	Percent  float64
	Matches  []CriticalASNMatch
}

type CriticalASNMatch struct {
	ASN     uint32
	Name    string
	Tier    string
	Role    string
	IPs     uint64
	Percent float64
}

type ASNProviderContext struct {
	Provider      string
	FeedIPs       uint64
	AttributedIPs uint64
	BogonIPs      uint64
	UnknownIPs    uint64
	TopASNs       CappedRow
}

type ASNEntry struct {
	ASN     uint32
	Name    string
	Count   uint64
	Percent float64
}

type GEOProviderContext struct {
	Provider     string
	TotalMapped  uint64
	TopCountries CappedRow
}

type CountryEntry struct {
	Code  string
	Name  string
	Value uint64
}

type BogonProviderContext struct {
	Provider string
	FeedIPs  uint64
	BogonIPs uint64
	Percent  float64
	Ranges   []BogonRange
}

type BogonRange struct {
	CIDR  string
	Name  string
	RFC   string
	Count uint64
}

type RetentionContext struct {
	CurrentRetention     float64
	PastRetention        float64
	CurrentRetentionDays float64
	PastRetentionDays    float64
	CurrentSeries        []RetentionBucket
	PastSeries           []RetentionBucket
}

type RetentionBucket struct {
	Day   int
	Label string
	Hours int
	IPs   uint64
}

type CompareRowContext struct {
	Name         string
	Category     string
	IPs          uint64
	Common       uint64
	ThisPercent  float64
	TheirPercent float64
	Related      bool
}

type CountryPageContext struct {
	Code            string
	Provider        string
	Totals          CountryTotals
	TopCategories   []CategorySummary
	TopMaintainers  []MaintainerSummary
	TopASNs         []CountryASN
	FeedsByCategory map[string][]FeedInEntity
}

type CountryTotals struct {
	Feeds       int
	IPs         uint64
	Categories  int
	Maintainers int
	ASNs        int
}

type CategorySummary struct {
	Category string
	Feeds    int
	IPs      uint64
}

type MaintainerSummary struct {
	Slug  string
	Name  string
	URL   string
	Feeds int
	IPs   uint64
}

type CountryASN struct {
	ASN   uint32
	Name  string
	Feeds int
	IPs   uint64
}

type FeedInEntity struct {
	Name     string
	Category string
	IPs      uint64
	Health   string
}

type ASNPageContext struct {
	ASN             uint32
	Name            string
	Description     string
	Provider        string
	Totals          ASNTotals
	TopCountries    []ASNCountry
	TopCategories   []CategorySummary
	TopMaintainers  []MaintainerSummary
	FeedsByCategory map[string][]FeedInEntity
}

type ASNTotals struct {
	Feeds       int
	IPs         uint64
	Categories  int
	Maintainers int
	Countries   int
}

type ASNCountry struct {
	Code  string
	Feeds int
	IPs   uint64
}

type MaintainerPageContext struct {
	Slug            string
	Name            string
	URL             string
	Totals          MaintainerTotals
	FeedsByCategory map[string][]FeedInEntity
}

type MaintainerTotals struct {
	Feeds      int
	IPs        uint64
	Categories int
}
