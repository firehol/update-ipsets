package insights

// Test helpers shared across rule tests. Each test builds a snapshot
// starting from buildSnap() (which is a valid snapshot with enough
// samples to pass every guard but no rule-firing values) and applies
// modifier methods for the specific fact under test. This keeps the
// individual tests focused on the math rather than the skeleton.

// snap is a builder wrapper around SignalSnapshot so tests read as:
//
//	buildSnap().
//	    withSizeSeries(60, steady(1_000_000)).
//	    withChurnSeries(60, zeroChurn(1_000_000)).
//	    toSnap()
//
// The builder returns *snap so chained calls mutate in place.
type snap struct {
	s SignalSnapshot
}

// buildSnap returns a default valid snapshot whose values do not trip
// any rule thresholds but whose sample sizes satisfy every guard. Use
// this as a neutral baseline and override only the fields a specific
// test cares about.
func buildSnap() *snap {
	b := &snap{
		s: SignalSnapshot{
			Name:       "test_feed",
			Category:   "attacks",
			SnapshotTS: 1_700_000_000,
			TotalIPs:   1_000_000,
		},
	}
	// Size series: steady 1M for 60 updates (no range => R01 silent).
	for i := 0; i < 60; i++ {
		ts := int64(1_700_000_000 + i*3600)
		b.s.SizeSeries = append(b.s.SizeSeries, SizePoint{TS: ts, Size: 1_000_000})
		b.s.ChurnSeries = append(b.s.ChurnSeries, ChurnPoint{
			TS:      ts,
			Added:   100_000, // churn = 0.2 (middle of the band: silent for R13/R14)
			Removed: 100_000,
			Kept:    800_000,
			Size:    1_000_000,
		})
	}
	// Age-of-listed: 200 IPs with a modest 24h tail.
	b.s.AgeOfListed = AgeHistogram{
		BucketsHours: []int{1, 6, 24},
		Counts:       []uint64{100, 60, 40},
		Total:        200,
	}
	// Age-of-removed: 2000 IPs concentrated in the 24h bucket with a
	// mild spread to neighbouring buckets. p50 and p90 land close
	// together so multiple_retention_policies does NOT fire on the
	// baseline; tests that want to fire it supply a bimodal histogram.
	b.s.AgeOfRemoved = AgeHistogram{
		BucketsHours: []int{12, 24, 36},
		Counts:       []uint64{600, 900, 500},
		Total:        2000,
	}
	// Top countries: 4 countries, each 20-30%, so no rule trips.
	b.s.TopCountries = []CountryShare{
		{Code: "US", Name: "United States", IPs: 300_000, Share: 0.30},
		{Code: "DE", Name: "Germany", IPs: 250_000, Share: 0.25},
		{Code: "FR", Name: "France", IPs: 250_000, Share: 0.25},
		{Code: "BR", Name: "Brazil", IPs: 200_000, Share: 0.20},
	}
	// Overlaps: 5 other feeds at modest 15% sharing — R04 silent
	// (needs <10%), R15 silent (needs >95%).
	b.s.Overlaps = []FeedOverlap{
		{Other: "other_a", Category: "attacks", OurShare: 0.15, TheirShare: 0.10},
		{Other: "other_b", Category: "attacks", OurShare: 0.12, TheirShare: 0.08},
		{Other: "other_c", Category: "attacks", OurShare: 0.14, TheirShare: 0.09},
		{Other: "other_d", Category: "malware", OurShare: 0.10, TheirShare: 0.05},
		{Other: "other_e", Category: "malware", OurShare: 0.11, TheirShare: 0.06},
	}
	b.s.OverlapsByCat = map[string]CategoryStat{
		"attacks": {MaxShare: 0.15, FeedCount: 3, ExampleFeed: "other_a"},
		"malware": {MaxShare: 0.11, FeedCount: 2, ExampleFeed: "other_e"},
	}
	return b
}

// toSnap returns the underlying SignalSnapshot.
func (b *snap) toSnap() SignalSnapshot { return b.s }

// withSizeSeries replaces the size series with the supplied points.
func (b *snap) withSizeSeries(points []SizePoint) *snap {
	b.s.SizeSeries = points
	return b
}

// withChurnSeries replaces the churn series with the supplied points.
func (b *snap) withChurnSeries(points []ChurnPoint) *snap {
	b.s.ChurnSeries = points
	return b
}

// withAgeOfListed replaces the currently-listed age distribution.
func (b *snap) withAgeOfListed(h AgeHistogram) *snap {
	b.s.AgeOfListed = h
	return b
}

// withAgeOfRemoved replaces the removed-IP age distribution.
func (b *snap) withAgeOfRemoved(h AgeHistogram) *snap {
	b.s.AgeOfRemoved = h
	return b
}

// withTopCountries replaces the top countries list.
func (b *snap) withTopCountries(list []CountryShare) *snap {
	b.s.TopCountries = list
	return b
}

// withBogonShare sets the bogon share.
func (b *snap) withBogonShare(share float64) *snap {
	b.s.BogonShare = share
	return b
}

// withInfraShare sets the infrastructure share.
func (b *snap) withInfraShare(share float64) *snap {
	b.s.InfraShare = share
	return b
}

// withInfraIPs sets the aggregate infrastructure IP count.
func (b *snap) withInfraIPs(ips uint64) *snap {
	b.s.InfraIPs = ips
	return b
}

// withInfraTiers replaces the tiered infrastructure summaries.
func (b *snap) withInfraTiers(tiers []InfraTier) *snap {
	b.s.InfraTiers = tiers
	return b
}

// withOverlaps replaces the pairwise overlaps list.
func (b *snap) withOverlaps(list []FeedOverlap) *snap {
	b.s.Overlaps = list
	return b
}

// withOverlapsByCat replaces the cross-category overlap stats.
func (b *snap) withOverlapsByCat(m map[string]CategoryStat) *snap {
	b.s.OverlapsByCat = m
	return b
}

// withCategory sets the feed's own category.
func (b *snap) withCategory(c string) *snap {
	b.s.Category = c
	return b
}

// withTotalIPs sets the feed's total IP count.
func (b *snap) withTotalIPs(n uint64) *snap {
	b.s.TotalIPs = n
	return b
}

// insightCodes returns the stable code of every insight in the input
// slice. Tests use this to assert which rules fired or did not fire.
func insightCodes(list []Insight) []string {
	out := make([]string, 0, len(list))
	for _, ins := range list {
		out = append(out, ins.Code)
	}
	return out
}

// containsCode reports whether list contains the given rule code.
func containsCode(list []Insight, code string) bool {
	for _, ins := range list {
		if ins.Code == code {
			return true
		}
	}
	return false
}

// uniformAgeHistogram builds a histogram where every hour bucket from
// 1 to maxHours has the same count. Used by negative tests to prove a
// distribution with no bimodal split does not fire the policy rules.
func uniformAgeHistogram(maxHours int, totalIPs uint64) AgeHistogram {
	if maxHours <= 0 {
		return AgeHistogram{}
	}
	perBucket := totalIPs / uint64(maxHours)
	if perBucket == 0 {
		perBucket = 1
	}
	h := AgeHistogram{}
	var sum uint64
	for i := 1; i <= maxHours; i++ {
		h.BucketsHours = append(h.BucketsHours, i)
		h.Counts = append(h.Counts, perBucket)
		sum += perBucket
	}
	h.Total = sum
	return h
}

// bimodalAgeHistogram builds a histogram with two clearly separated
// modes: a fast mode at fastHours and a slow mode at slowHours. The
// helper is used by the multiple_retention_policies positive tests.
func bimodalAgeHistogram(fastHours, slowHours int, fastIPs, slowIPs uint64) AgeHistogram {
	return AgeHistogram{
		BucketsHours: []int{fastHours, slowHours},
		Counts:       []uint64{fastIPs, slowIPs},
		Total:        fastIPs + slowIPs,
	}
}

// steadySizeSeries builds n size points all at the same size. Used by
// negative tests for the size_variation rule.
func steadySizeSeries(n int, size uint64) []SizePoint {
	out := make([]SizePoint, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, SizePoint{TS: int64(1_700_000_000 + i*3600), Size: size})
	}
	return out
}

// rangingSizeSeries builds n size points that swing between minSize
// and maxSize in alternating fashion so min/median/max are stable.
func rangingSizeSeries(n int, minSize, maxSize uint64) []SizePoint {
	out := make([]SizePoint, 0, n)
	for i := 0; i < n; i++ {
		size := minSize
		if i%2 == 0 {
			size = maxSize
		}
		out = append(out, SizePoint{TS: int64(1_700_000_000 + i*3600), Size: size})
	}
	return out
}

// steadyChurnSeries builds n churn points with the given size, added,
// and removed counts each.
func steadyChurnSeries(n int, size, added, removed uint64) []ChurnPoint {
	out := make([]ChurnPoint, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ChurnPoint{
			TS:      int64(1_700_000_000 + i*3600),
			Added:   added,
			Removed: removed,
			Kept:    size - removed,
			Size:    size,
		})
	}
	return out
}
