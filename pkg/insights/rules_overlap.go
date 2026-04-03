package insights

import "fmt"

func init() {
	catalog = append(catalog,
		ruleIndependent(),
		ruleSubsetOf(),
		ruleCrossCategoryOverlap(),
	)
}

// ruleIndependent (R04) fires when the feed has low overlap with every
// other feed we track. The condition is: max(OurShare) < 0.10 across at
// least 5 compared feeds. The headline reports the unique fraction
// 1 - max(OurShare) so the user sees "87% of this list's IPs do not
// appear in any other feed we track" rather than the inverse.
func ruleIndependent() Rule {
	return Rule{
		Code:    "independent",
		Name:    "Independent feed",
		Section: SectionOverview,
		MinSamples: func(s SignalSnapshot) bool {
			return len(s.Overlaps) >= 5 && s.TotalIPs >= 100
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			var maxShare float64
			for _, o := range s.Overlaps {
				if o.OurShare > maxShare {
					maxShare = o.OurShare
				}
			}
			if maxShare >= 0.10 {
				return Insight{}, false
			}
			unique := 1 - maxShare
			return Insight{
				Headline: fmt.Sprintf(
					"%s of this list's IPs do not appear in any other feed we track.",
					formatPercent(unique),
				),
				Evidence: map[string]any{
					"max_overlap_share": maxShare,
					"unique_share":      unique,
					"compared_feeds":    len(s.Overlaps),
				},
			}, true
		},
		Methodology: "/methodology/independent",
	}
}

// ruleSubsetOf (R15) fires when there exists an older feed whose
// overlap contains more than 95% of this feed's IPs. The "older than
// this" requirement prevents a brand-new feed that happens to include
// everything in an older specialist feed from being reported as a
// subset of the specialist.
func ruleSubsetOf() Rule {
	return Rule{
		Code:    "subset_of",
		Name:    "Subset of another feed",
		Section: SectionRelationships,
		MinSamples: func(s SignalSnapshot) bool {
			return s.TotalIPs >= 100 && len(s.Overlaps) >= 1
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			var best *FeedOverlap
			for i, o := range s.Overlaps {
				if !o.OlderThanThis {
					continue
				}
				if o.OurShare <= 0.95 {
					continue
				}
				if best == nil || o.OurShare > best.OurShare {
					best = &s.Overlaps[i]
				}
			}
			if best == nil {
				return Insight{}, false
			}
			category := best.Category
			if category == "" {
				category = "other"
			}
			return Insight{
				Headline: fmt.Sprintf(
					"%s of this list's IPs also appear in %s (%s).",
					formatPercent(best.OurShare), best.Other, category,
				),
				Evidence: map[string]any{
					"other":       best.Other,
					"category":    category,
					"our_share":   best.OurShare,
					"their_share": best.TheirShare,
				},
			}, true
		},
		Methodology: "/methodology/subset-of",
	}
}

// ruleCrossCategoryOverlap (R16) fires when a feed has significant
// overlap (>30% of this feed's IPs) with feeds in a different category
// than its own. The sample-size guard requires at least 3 feeds in the
// target category so a single specialist feed cannot single-handedly
// trigger the rule.
func ruleCrossCategoryOverlap() Rule {
	return Rule{
		Code:    "cross_category_overlap",
		Name:    "Cross-category overlap",
		Section: SectionRelationships,
		MinSamples: func(s SignalSnapshot) bool {
			return s.TotalIPs >= 100 && len(s.OverlapsByCat) > 0
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			var bestCat string
			var bestStat CategoryStat
			for cat, stat := range s.OverlapsByCat {
				if cat == "" || cat == s.Category {
					continue
				}
				if stat.FeedCount < 3 {
					continue
				}
				if stat.MaxShare <= 0.30 {
					continue
				}
				if stat.MaxShare > bestStat.MaxShare {
					bestStat = stat
					bestCat = cat
				}
			}
			if bestCat == "" {
				return Insight{}, false
			}
			own := s.Category
			if own == "" {
				own = "uncategorized"
			}
			return Insight{
				Headline: fmt.Sprintf(
					"Although categorized as %s, this list overlaps %s with %s feeds.",
					own, formatPercent(bestStat.MaxShare), bestCat,
				),
				Evidence: map[string]any{
					"own_category":   own,
					"other_category": bestCat,
					"max_share":      bestStat.MaxShare,
					"feed_count":     bestStat.FeedCount,
					"example_feed":   bestStat.ExampleFeed,
				},
			}, true
		},
		Methodology: "/methodology/cross-category-overlap",
	}
}
