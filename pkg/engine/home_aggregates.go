package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/output"
)

const homeAggregatesVersion = 1

var ErrHomeAggregatesNotReady = errors.New("homepage aggregate artifact is not ready")

type homeAggregatesPayload struct {
	Version     int                     `json:"version"`
	GeneratedAt int64                   `json:"generated_at"`
	Providers   HomeSummaryProviders    `json:"providers"`
	Categories  []homeCategoryAggregate `json:"categories"`
}

type homeCategoryAggregate struct {
	Category          string                  `json:"category"`
	EligibleFeeds     int                     `json:"eligible_feeds"`
	ContributingFeeds int                     `json:"contributing_feeds"`
	UniqueIPs         uint64                  `json:"unique_ips"`
	Countries         []HomeSummaryCountry    `json:"countries,omitempty"`
	ASNs              []HomeSummaryASN        `json:"asns,omitempty"`
	Maintainers       []HomeSummaryMaintainer `json:"maintainers,omitempty"`
}

type homeMutableCategoryAggregate struct {
	category          string
	eligibleFeeds     int
	contributingFeeds int
	uniqueIPs         uint64
	countries         map[string]*homeCountryAggregate
	asns              map[uint32]*homeASNAggregate
	maintainers       map[string]*homeMaintainerAggregate
}

type homeCountryAggregate struct {
	feedCount     int
	attributedIPs uint64
}

type homeASNAggregate struct {
	name          string
	feedCount     int
	attributedIPs uint64
}

type homeMaintainerAggregate struct {
	slug              string
	name              string
	url               string
	feedCount         int
	uniqueIPs         uint64
	categoryBreakdown map[string]int
}

func (e *Engine) publicHomeAggregatesRelPath() string {
	return filepath.Join("home", "aggregates.json")
}

func (e *Engine) PublicHomeAggregatesPath() string {
	return filepath.Join(e.outputDir(), e.publicHomeAggregatesRelPath())
}

func (e *Engine) stageHomeAggregates(ctx context.Context, stageDir, inputDir string) (output.GeneratedFile, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return output.GeneratedFile{}, err
	}
	if e == nil || e.cfg == nil {
		return output.GeneratedFile{}, fmt.Errorf("engine is not configured")
	}
	if stageDir == "" {
		return output.GeneratedFile{}, fmt.Errorf("homepage aggregate stage directory is required")
	}
	if inputDir == "" {
		inputDir = e.outputDir()
	}

	started := time.Now()
	payload, err := e.buildHomeAggregatesInDir(ctx, inputDir)
	if err != nil {
		e.observeRunOperation("metadata.write_home_aggregates", time.Since(started))
		return output.GeneratedFile{}, err
	}
	rel := e.publicHomeAggregatesRelPath()
	path := filepath.Join(stageDir, rel)
	if err := writeJSONFile(path, payload); err != nil {
		e.observeRunOperation("metadata.write_home_aggregates", time.Since(started))
		return output.GeneratedFile{}, err
	}
	_, refTime, _ := e.homeAggregatesReference()
	timestamp := e.now().UTC()
	if refTime.After(timestamp) {
		timestamp = refTime
	}
	e.observeRunOperation("metadata.write_home_aggregates", time.Since(started))
	return output.GeneratedFile{
		Path:            filepath.Join(e.outputDir(), rel),
		Timestamp:       timestamp,
		Redistributable: true,
	}, nil
}

func (e *Engine) buildHomeAggregatesInDir(ctx context.Context, inputDir string) (*homeAggregatesPayload, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	geoProvider := e.preferredGeoProvider()
	asnProvider := e.preferredASNProvider()
	geoSrc := e.lookupSource(geoProvider)
	asnSrc := e.lookupSource(asnProvider)
	policy := feedhealth.PolicyFromRuntime(e.cfg.Runtime)
	now := e.now().UTC()
	view := newEntityOutputView(e, inputDir)

	categories := map[string]*homeMutableCategoryAggregate{}
	for _, entry := range e.EntriesSnapshot() {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		src := e.lookupSource(entry.Name)
		if !homeSummaryEligible(e.cfg, src, nil) {
			continue
		}
		health := feedhealth.Classify(&entry, src, policy, now)
		if !homeGlobeHealthEligible(health.Class) {
			continue
		}
		category := strings.TrimSpace(src.Category)
		if category == "" {
			continue
		}
		agg := categories[category]
		if agg == nil {
			agg = newHomeMutableCategoryAggregate(category)
			categories[category] = agg
		}
		agg.eligibleFeeds++
		agg.uniqueIPs += entry.UniqueIPs
		agg.addMaintainer(entry.Maintainer, entry.MaintainerURL, category, entry.UniqueIPs)

		contributed := false
		if geoProvider != "" {
			payload, err := view.countryComparison(entry.Name, geoProvider)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("read homepage country aggregate input for %s/%s: %w", entry.Name, geoProvider, err)
			}
			if err == nil && payload != nil {
				seen := map[string]struct{}{}
				for _, country := range payload.Countries {
					code := strings.ToUpper(strings.TrimSpace(country.Code))
					if code == "" {
						continue
					}
					agg.addCountry(code, country.Value, seen)
				}
				if len(payload.Countries) > 0 {
					contributed = true
				}
			}
		}
		if asnProvider != "" {
			asnPayload, err := view.topASNsWithError(entry.Name, asnProvider)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("read homepage ASN aggregate input for %s/%s: %w", entry.Name, asnProvider, err)
			}
			seen := map[uint32]struct{}{}
			for _, row := range asnPayload {
				if row.ASN == 0 {
					continue
				}
				agg.addASN(row.ASN, row.Name, row.Count, seen)
			}
			if len(asnPayload) > 0 {
				contributed = true
			}
		}
		if contributed {
			agg.contributingFeeds++
		}
	}

	payload := &homeAggregatesPayload{
		Version:     homeAggregatesVersion,
		GeneratedAt: now.Unix(),
		Providers: HomeSummaryProviders{
			Geo: HomeSummaryProvider{
				Name:  geoProvider,
				Label: providerDisplayLabel(geoSrc),
			},
			ASN: HomeSummaryProvider{
				Name:  asnProvider,
				Label: providerDisplayLabel(asnSrc),
			},
		},
		Categories: make([]homeCategoryAggregate, 0, len(categories)),
	}
	for _, category := range sortedHomeCategoryKeys(categories) {
		payload.Categories = append(payload.Categories, categories[category].snapshot())
	}
	return payload, nil
}

func newHomeMutableCategoryAggregate(category string) *homeMutableCategoryAggregate {
	return &homeMutableCategoryAggregate{
		category:    category,
		countries:   map[string]*homeCountryAggregate{},
		asns:        map[uint32]*homeASNAggregate{},
		maintainers: map[string]*homeMaintainerAggregate{},
	}
}

func (a *homeMutableCategoryAggregate) addCountry(code string, attributed uint64, seen map[string]struct{}) {
	current := a.countries[code]
	if current == nil {
		current = &homeCountryAggregate{}
		a.countries[code] = current
	}
	current.attributedIPs += attributed
	if _, ok := seen[code]; !ok {
		current.feedCount++
		seen[code] = struct{}{}
	}
}

func (a *homeMutableCategoryAggregate) addASN(asn uint32, name string, attributed uint64, seen map[uint32]struct{}) {
	current := a.asns[asn]
	if current == nil {
		current = &homeASNAggregate{name: name}
		a.asns[asn] = current
	}
	if current.name == "" && name != "" {
		current.name = name
	}
	current.attributedIPs += attributed
	if _, ok := seen[asn]; !ok {
		current.feedCount++
		seen[asn] = struct{}{}
	}
}

func (a *homeMutableCategoryAggregate) addMaintainer(name, url, category string, uniqueIPs uint64) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	slug := maintainerSlugify(name)
	current := a.maintainers[slug]
	if current == nil {
		current = &homeMaintainerAggregate{
			slug:              slug,
			name:              name,
			url:               url,
			categoryBreakdown: map[string]int{},
		}
		a.maintainers[slug] = current
	}
	if current.url == "" && url != "" {
		current.url = url
	}
	current.feedCount++
	current.uniqueIPs += uniqueIPs
	current.categoryBreakdown[category]++
}

func (a *homeMutableCategoryAggregate) snapshot() homeCategoryAggregate {
	return homeCategoryAggregate{
		Category:          a.category,
		EligibleFeeds:     a.eligibleFeeds,
		ContributingFeeds: a.contributingFeeds,
		UniqueIPs:         a.uniqueIPs,
		Countries:         sortedHomeCountries(a.countries),
		ASNs:              sortedHomeASNs(a.asns),
		Maintainers:       sortedHomeMaintainers(a.maintainers),
	}
}

func sortedHomeCategoryKeys(categories map[string]*homeMutableCategoryAggregate) []string {
	out := make([]string, 0, len(categories))
	for category := range categories {
		out = append(out, category)
	}
	slices.Sort(out)
	return out
}

func (e *Engine) loadHomeAggregatesInDir(outputDir string) (*homeAggregatesPayload, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	if outputDir == "" {
		outputDir = e.outputDir()
	}
	started := time.Now()
	payload, size, err := readHomeAggregatesFile(outputDir, e.publicHomeAggregatesRelPath())
	if err != nil {
		return nil, err
	}
	e.observeRunCounter("http.home_aggregates.read", 1, int64(size))
	e.observeRunOperation("http.home_aggregates.read", time.Since(started))
	return payload, nil
}

func readHomeAggregatesFile(rootDir, rel string) (*homeAggregatesPayload, int, error) {
	data, err := readFileInRoot(rootDir, rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, ErrHomeAggregatesNotReady
		}
		return nil, 0, fmt.Errorf("read homepage aggregate artifact: %w", err)
	}
	var payload homeAggregatesPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, len(data), fmt.Errorf("%w: decode homepage aggregate artifact: %w", ErrHomeAggregatesNotReady, err)
	}
	if payload.Version != homeAggregatesVersion {
		return nil, len(data), fmt.Errorf("%w: unsupported version %d", ErrHomeAggregatesNotReady, payload.Version)
	}
	return &payload, len(data), nil
}

func (p *homeAggregatesPayload) selectedCategories(filter map[string]struct{}) []homeCategoryAggregate {
	if p == nil {
		return nil
	}
	selected := make([]homeCategoryAggregate, 0, len(p.Categories))
	for _, category := range p.Categories {
		if len(filter) > 0 {
			if _, ok := filter[category.Category]; !ok {
				continue
			}
		}
		selected = append(selected, category)
	}
	return selected
}

func composeHomeSummaryFromAggregates(payload *homeAggregatesPayload, filter map[string]struct{}, limit int) *HomeSummaryPayload {
	selected := payload.selectedCategories(filter)
	countries := map[string]*homeCountryAggregate{}
	asns := map[uint32]*homeASNAggregate{}
	maintainers := map[string]*homeMaintainerAggregate{}
	categorySet := map[string]struct{}{}

	eligibleFeeds := 0
	contributingFeeds := 0
	var totalUniqueIPs uint64
	for _, category := range selected {
		categorySet[category.Category] = struct{}{}
		eligibleFeeds += category.EligibleFeeds
		contributingFeeds += category.ContributingFeeds
		totalUniqueIPs += category.UniqueIPs
		for _, row := range category.Countries {
			current := countries[row.Code]
			if current == nil {
				current = &homeCountryAggregate{}
				countries[row.Code] = current
			}
			current.feedCount += row.FeedCount
			current.attributedIPs += row.AttributedIPs
		}
		for _, row := range category.ASNs {
			current := asns[row.ASN]
			if current == nil {
				current = &homeASNAggregate{name: row.Name}
				asns[row.ASN] = current
			}
			if current.name == "" && row.Name != "" {
				current.name = row.Name
			}
			current.feedCount += row.FeedCount
			current.attributedIPs += row.AttributedIPs
		}
		for _, row := range category.Maintainers {
			current := maintainers[row.Slug]
			if current == nil {
				current = &homeMaintainerAggregate{
					slug:              row.Slug,
					name:              row.Name,
					url:               row.URL,
					categoryBreakdown: map[string]int{},
				}
				maintainers[row.Slug] = current
			}
			if current.url == "" && row.URL != "" {
				current.url = row.URL
			}
			current.feedCount += row.FeedCount
			current.uniqueIPs += row.UniqueIPs
			for category, count := range row.CategoryBreakdown {
				current.categoryBreakdown[category] += count
			}
		}
	}

	topCountries := sortedHomeCountries(countries)
	if len(topCountries) > limit {
		topCountries = topCountries[:limit]
	}
	topASNs := sortedHomeASNs(asns)
	if len(topASNs) > limit {
		topASNs = topASNs[:limit]
	}
	topMaintainers := sortedHomeMaintainers(maintainers)
	if len(topMaintainers) > limit {
		topMaintainers = topMaintainers[:limit]
	}
	return &HomeSummaryPayload{
		Categories:        sortedFilterKeys(filter),
		EligibleFeeds:     eligibleFeeds,
		ContributingFeeds: contributingFeeds,
		Totals: HomeSummaryTotals{
			Feeds:      eligibleFeeds,
			UniqueIPs:  totalUniqueIPs,
			Categories: len(categorySet),
		},
		Providers:      payload.Providers,
		TopCountries:   topCountries,
		TopASNs:        topASNs,
		TopMaintainers: topMaintainers,
	}
}

func composeHomeGlobeFromAggregates(payload *homeAggregatesPayload, filter map[string]struct{}) *HomeGlobePayload {
	selected := payload.selectedCategories(filter)
	countries := map[string]*homeCountryAggregate{}
	eligibleFeeds := 0
	contributingFeeds := 0
	for _, category := range selected {
		eligibleFeeds += category.EligibleFeeds
		contributingFeeds += category.ContributingFeeds
		for _, row := range category.Countries {
			current := countries[row.Code]
			if current == nil {
				current = &homeCountryAggregate{}
				countries[row.Code] = current
			}
			current.feedCount += row.FeedCount
			current.attributedIPs += row.AttributedIPs
		}
	}
	return &HomeGlobePayload{
		Provider:          payload.Providers.Geo.Name,
		ProviderLabel:     payload.Providers.Geo.Label,
		Categories:        sortedFilterKeys(filter),
		EligibleFeeds:     eligibleFeeds,
		ContributingFeeds: contributingFeeds,
		Countries:         homeCountriesForGlobe(countries),
	}
}

func sortedHomeCountries(countries map[string]*homeCountryAggregate) []HomeSummaryCountry {
	out := make([]HomeSummaryCountry, 0, len(countries))
	for code, agg := range countries {
		out = append(out, HomeSummaryCountry{
			Code:          code,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FeedCount != out[j].FeedCount {
			return out[i].FeedCount > out[j].FeedCount
		}
		if out[i].AttributedIPs != out[j].AttributedIPs {
			return out[i].AttributedIPs > out[j].AttributedIPs
		}
		return out[i].Code < out[j].Code
	})
	return out
}

func homeCountriesForGlobe(countries map[string]*homeCountryAggregate) []HomeGlobeCountry {
	summaryRows := sortedHomeCountries(countries)
	out := make([]HomeGlobeCountry, 0, len(summaryRows))
	for _, row := range summaryRows {
		out = append(out, HomeGlobeCountry(row))
	}
	return out
}

func sortedHomeASNs(asns map[uint32]*homeASNAggregate) []HomeSummaryASN {
	out := make([]HomeSummaryASN, 0, len(asns))
	for asn, agg := range asns {
		out = append(out, HomeSummaryASN{
			ASN:           asn,
			Name:          agg.name,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FeedCount != out[j].FeedCount {
			return out[i].FeedCount > out[j].FeedCount
		}
		if out[i].AttributedIPs != out[j].AttributedIPs {
			return out[i].AttributedIPs > out[j].AttributedIPs
		}
		return out[i].ASN < out[j].ASN
	})
	return out
}

func sortedHomeMaintainers(maintainers map[string]*homeMaintainerAggregate) []HomeSummaryMaintainer {
	out := make([]HomeSummaryMaintainer, 0, len(maintainers))
	for _, agg := range maintainers {
		out = append(out, HomeSummaryMaintainer{
			Slug:              agg.slug,
			Name:              agg.name,
			URL:               agg.url,
			FeedCount:         agg.feedCount,
			UniqueIPs:         agg.uniqueIPs,
			CategoryBreakdown: cloneStringIntMap(agg.categoryBreakdown),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FeedCount != out[j].FeedCount {
			return out[i].FeedCount > out[j].FeedCount
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func cloneStringIntMap(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
