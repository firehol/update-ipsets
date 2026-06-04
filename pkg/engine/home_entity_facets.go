package engine

import (
	"sort"
	"strings"
)

type detailFacetAccumulator struct {
	categoryTotals   map[string]*detailCategoryAggregate
	maintainerTotals map[string]*detailMaintainerAggregate
}

func newDetailFacetAccumulator() detailFacetAccumulator {
	return detailFacetAccumulator{
		categoryTotals:   make(map[string]*detailCategoryAggregate),
		maintainerTotals: make(map[string]*detailMaintainerAggregate),
	}
}

func (a *detailFacetAccumulator) add(category, maintainer, maintainerURL string, attributedIPs uint64) {
	a.ensureMaps()
	categoryAgg := a.categoryTotals[category]
	if categoryAgg == nil {
		categoryAgg = &detailCategoryAggregate{}
		a.categoryTotals[category] = categoryAgg
	}
	categoryAgg.feedCount++
	categoryAgg.attributedIPs += attributedIPs

	maintainerName := strings.TrimSpace(maintainer)
	if maintainerName == "" {
		return
	}
	slug := maintainerSlugify(maintainerName)
	maintainerAgg := a.maintainerTotals[slug]
	if maintainerAgg == nil {
		maintainerAgg = &detailMaintainerAggregate{
			slug: slug,
			name: maintainerName,
			url:  maintainerURL,
		}
		a.maintainerTotals[slug] = maintainerAgg
	}
	if maintainerAgg.url == "" && maintainerURL != "" {
		maintainerAgg.url = maintainerURL
	}
	maintainerAgg.feedCount++
	maintainerAgg.attributedIPs += attributedIPs
}

func (a *detailFacetAccumulator) ensureMaps() {
	if a.categoryTotals == nil {
		a.categoryTotals = make(map[string]*detailCategoryAggregate)
	}
	if a.maintainerTotals == nil {
		a.maintainerTotals = make(map[string]*detailMaintainerAggregate)
	}
}

func (a detailFacetAccumulator) categoryCount() int {
	return len(a.categoryTotals)
}

func (a detailFacetAccumulator) maintainerCount() int {
	return len(a.maintainerTotals)
}

func (a detailFacetAccumulator) topCategories() []DetailCategorySummary {
	topCategories := make([]DetailCategorySummary, 0, len(a.categoryTotals))
	for category, agg := range a.categoryTotals {
		topCategories = append(topCategories, DetailCategorySummary{
			Category:      category,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topCategories, func(i, j int) bool {
		if topCategories[i].AttributedIPs != topCategories[j].AttributedIPs {
			return topCategories[i].AttributedIPs > topCategories[j].AttributedIPs
		}
		if topCategories[i].FeedCount != topCategories[j].FeedCount {
			return topCategories[i].FeedCount > topCategories[j].FeedCount
		}
		return topCategories[i].Category < topCategories[j].Category
	})
	return topCategories
}

func (a detailFacetAccumulator) topMaintainers() []DetailMaintainerSummary {
	topMaintainers := make([]DetailMaintainerSummary, 0, len(a.maintainerTotals))
	for _, agg := range a.maintainerTotals {
		topMaintainers = append(topMaintainers, DetailMaintainerSummary{
			Slug:          agg.slug,
			Name:          agg.name,
			URL:           agg.url,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topMaintainers, func(i, j int) bool {
		if topMaintainers[i].AttributedIPs != topMaintainers[j].AttributedIPs {
			return topMaintainers[i].AttributedIPs > topMaintainers[j].AttributedIPs
		}
		if topMaintainers[i].FeedCount != topMaintainers[j].FeedCount {
			return topMaintainers[i].FeedCount > topMaintainers[j].FeedCount
		}
		return topMaintainers[i].Name < topMaintainers[j].Name
	})
	return topMaintainers
}
