package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

type EntityIntegrityFinding struct {
	Scope             string    `json:"scope"`
	Kind              string    `json:"kind"`
	Subject           string    `json:"subject,omitempty"`
	Feed              string    `json:"feed,omitempty"`
	Country           string    `json:"country,omitempty"`
	ASN               uint32    `json:"asn,omitempty"`
	Path              string    `json:"path,omitempty"`
	PathMTime         time.Time `json:"path_mtime,omitempty"`
	ReferencePath     string    `json:"reference_path,omitempty"`
	ReferenceMTime    time.Time `json:"reference_mtime,omitempty"`
	RepairAction      string    `json:"repair_action,omitempty"`
	Reason            string    `json:"reason"`
	AffectedCountries int       `json:"affected_countries,omitempty"`
	AffectedASNs      int       `json:"affected_asns,omitempty"`
}

type entityIntegrityPlan struct {
	full                 bool
	feedNames            map[string]struct{}
	countryCodes         map[string]struct{}
	asns                 map[uint32]struct{}
	rebuildCountryIndex  bool
	rebuildASNIndex      bool
	rebuildHomeAggregate bool
	healthFeeds          map[string]struct{}
}

type entityDependencyRef struct {
	path string
	when time.Time
}

type entityHealthCheck struct {
	feed         string
	healthClass  string
	transitionAt time.Time
	countries    []string
	asns         []uint32
}

const maxStartupEntityAutoRepairTargets = 1024

func (p *entityIntegrityPlan) markFull() {
	if p != nil {
		p.full = true
	}
}

func (p *entityIntegrityPlan) addFeed(name string) {
	if p == nil || strings.TrimSpace(name) == "" {
		return
	}
	if p.feedNames == nil {
		p.feedNames = map[string]struct{}{}
	}
	p.feedNames[name] = struct{}{}
}

func (p *entityIntegrityPlan) addCountry(code string) {
	if p == nil {
		return
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return
	}
	if p.countryCodes == nil {
		p.countryCodes = map[string]struct{}{}
	}
	p.countryCodes[code] = struct{}{}
}

func (p *entityIntegrityPlan) addASN(asn uint32) {
	if p == nil || asn == 0 {
		return
	}
	if p.asns == nil {
		p.asns = map[uint32]struct{}{}
	}
	p.asns[asn] = struct{}{}
}

func (p *entityIntegrityPlan) addHealthFeed(name string) {
	if p == nil || strings.TrimSpace(name) == "" {
		return
	}
	if p.healthFeeds == nil {
		p.healthFeeds = map[string]struct{}{}
	}
	p.healthFeeds[name] = struct{}{}
}

func (p *entityIntegrityPlan) addHomeAggregate() {
	if p != nil {
		p.rebuildHomeAggregate = true
	}
}

func (p entityIntegrityPlan) hasWork() bool {
	return p.full ||
		len(p.feedNames) > 0 ||
		len(p.countryCodes) > 0 ||
		len(p.asns) > 0 ||
		p.rebuildCountryIndex ||
		p.rebuildASNIndex ||
		p.rebuildHomeAggregate ||
		len(p.healthFeeds) > 0
}

func (p entityIntegrityPlan) sortedFeeds() []string {
	return sortedStringSet(p.feedNames)
}

func (p entityIntegrityPlan) sortedHealthFeeds() []string {
	return sortedStringSet(p.healthFeeds)
}

func (p entityIntegrityPlan) targetCount() int {
	count := len(p.feedNames) + len(p.countryCodes) + len(p.asns) + len(p.healthFeeds)
	if p.rebuildCountryIndex {
		count++
	}
	if p.rebuildASNIndex {
		count++
	}
	if p.rebuildHomeAggregate {
		count++
	}
	return count
}

func (p entityIntegrityPlan) shouldDeferStartupRepair() bool {
	if p.full {
		return false
	}
	return p.targetCount() > maxStartupEntityAutoRepairTargets
}

func (e *Engine) EnsureEntityArtifactsCurrent(ctx context.Context) error {
	return e.EnsureEntityArtifactsCurrentWithTrigger(ctx, "bootstrap")
}

func (e *Engine) EnsureEntityArtifactsCurrentWithTrigger(ctx context.Context, trigger string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	findings, plan, err := e.CheckEntityArtifactsIntegrity()
	if err != nil {
		return err
	}
	if len(findings) == 0 || !plan.hasWork() {
		return nil
	}
	if trigger == "startup" && plan.shouldDeferStartupRepair() {
		e.observeRunCounter("entity.integrity_startup_repair_deferred", int64(plan.targetCount()), 0)
		e.mu.Lock()
		e.startupRepairDeferred = true
		e.startupRepairDeferredTargets = plan.targetCount()
		e.mu.Unlock()
		if e.logger != nil {
			e.logger.Warn("deferred broad startup entity artifact repair",
				"targets", plan.targetCount(),
				"findings", len(findings),
				"limit", maxStartupEntityAutoRepairTargets)
		}
		return nil
	}
	return e.repairEntityArtifactsWithPlan(ctx, trigger, plan)
}

func (e *Engine) CheckEntityArtifactsIntegrity() ([]EntityIntegrityFinding, entityIntegrityPlan, error) {
	if e == nil || e.cfg == nil {
		return nil, entityIntegrityPlan{}, nil
	}

	findings := make([]EntityIntegrityFinding, 0)
	var plan entityIntegrityPlan

	versionPath := e.entityVersionPath()
	versionInfo, err := os.Stat(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			plan.markFull()
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "global",
				Kind:         "version_missing",
				Subject:      "entity_artifacts",
				Path:         versionPath,
				RepairAction: "full_rebuild",
				Reason:       "entity artifact version marker is missing",
			})
			return findings, plan, nil
		}
		return nil, plan, err
	}
	versionData, err := os.ReadFile(versionPath)
	if err != nil {
		return nil, plan, err
	}
	if strings.TrimSpace(string(versionData)) != entityArtifactsVersion {
		plan.markFull()
		findings = append(findings, EntityIntegrityFinding{
			Scope:        "global",
			Kind:         "version_mismatch",
			Subject:      "entity_artifacts",
			Path:         versionPath,
			PathMTime:    versionInfo.ModTime().UTC(),
			RepairAction: "full_rebuild",
			Reason:       fmt.Sprintf("entity artifact version is stale; want %s", entityArtifactsVersion),
		})
		return findings, plan, nil
	}

	configPath, configTime, err := e.latestEntityConfigInputTime()
	if err != nil {
		return nil, plan, err
	}
	if !configTime.IsZero() && versionInfo.ModTime().UTC().Before(configTime) {
		plan.markFull()
		findings = append(findings, EntityIntegrityFinding{
			Scope:          "global",
			Kind:           "config_newer",
			Subject:        "entity_artifacts",
			Path:           versionPath,
			PathMTime:      versionInfo.ModTime().UTC(),
			ReferencePath:  configPath,
			ReferenceMTime: configTime,
			RepairAction:   "full_rebuild",
			Reason:         "entity artifacts are older than the loaded catalog inputs",
		})
		return findings, plan, nil
	}

	geoProvider := e.preferredGeoProvider()
	asnProvider := e.preferredASNProvider()
	geoRefPath, geoRefTime, err := e.entityGeoProviderReference(geoProvider)
	if err != nil {
		return nil, plan, err
	}
	asnRefPath, asnRefTime, err := e.entityASNProviderReference(asnProvider)
	if err != nil {
		return nil, plan, err
	}

	countryRefs := map[string]entityDependencyRef{}
	asnRefs := map[uint32]entityDependencyRef{}
	countryPublicHealth := map[string]map[string]string{}
	asnPublicHealth := map[uint32]map[string]string{}
	healthChecks := make([]entityHealthCheck, 0, 32)
	health := e.newFeedHealthClassifier()

	for _, name := range e.publicOutputNames() {
		if strings.TrimSpace(name) == "" {
			continue
		}
		sidecarPath := filepath.Join(e.entityFeedsDir(), name+".json")
		sidecarRefPath, sidecarRefTime, err := e.entityFeedSidecarReference(name, geoProvider, asnProvider, geoRefPath, geoRefTime, asnRefPath, asnRefTime)
		if err != nil {
			return nil, plan, err
		}
		if sidecarRefTime.IsZero() {
			continue
		}

		sidecarInfo, err := os.Stat(sidecarPath)
		if err != nil {
			if os.IsNotExist(err) {
				expected, expectedErr := e.feedEntityInputsHaveContributions(name, geoProvider, asnProvider)
				if expectedErr != nil {
					return nil, plan, expectedErr
				}
				if !expected {
					continue
				}
				findings = append(findings, EntityIntegrityFinding{
					Scope:          "feed",
					Kind:           "feed_sidecar_missing",
					Subject:        name,
					Feed:           name,
					Path:           sidecarPath,
					ReferencePath:  sidecarRefPath,
					ReferenceMTime: sidecarRefTime,
					RepairAction:   "refresh_feed",
					Reason:         "feed entity sidecar is missing",
				})
				plan.addFeed(name)
				continue
			}
			return nil, plan, err
		}
		sidecarMTime := sidecarInfo.ModTime().UTC()
		sidecar, err := e.loadFeedEntitySidecar(sidecarPath)
		if err != nil {
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "feed",
				Kind:         "feed_sidecar_malformed",
				Subject:      name,
				Feed:         name,
				Path:         sidecarPath,
				PathMTime:    sidecarMTime,
				RepairAction: "refresh_feed",
				Reason:       "feed entity sidecar is unreadable",
			})
			plan.addFeed(name)
			continue
		}
		if sidecarMTime.Before(sidecarRefTime) && !e.feedEntitySidecarCoversReference(name, sidecar, sidecarRefPath, sidecarRefTime) {
			findings = append(findings, EntityIntegrityFinding{
				Scope:          "feed",
				Kind:           "feed_sidecar_stale",
				Subject:        name,
				Feed:           name,
				Path:           sidecarPath,
				PathMTime:      sidecarMTime,
				ReferencePath:  sidecarRefPath,
				ReferenceMTime: sidecarRefTime,
				RepairAction:   "refresh_feed",
				Reason:         "feed entity sidecar is older than its local inputs",
			})
			plan.addFeed(name)
			continue
		}

		for _, code := range sidecar.countryCodes() {
			normalized := strings.ToUpper(strings.TrimSpace(code))
			ref := countryRefs[normalized]
			mergeEntityDependencyRef(&ref, sidecarPath, sidecarMTime)
			countryRefs[normalized] = ref
		}
		for _, asn := range sidecar.asnNumbers() {
			ref := asnRefs[asn]
			mergeEntityDependencyRef(&ref, sidecarPath, sidecarMTime)
			asnRefs[asn] = ref
		}

		if transitionAt := e.entityFeedHealthTransitionTime(name, health); !transitionAt.IsZero() {
			healthClass := health.class(name)
			if strings.TrimSpace(healthClass) == "" {
				continue
			}
			healthChecks = append(healthChecks, entityHealthCheck{
				feed:         name,
				healthClass:  healthClass,
				transitionAt: transitionAt,
				countries:    append([]string(nil), sidecar.countryCodes()...),
				asns:         append([]uint32(nil), sidecar.asnNumbers()...),
			})
		}
	}

	for code, ref := range countryRefs {
		sidecarPath := filepath.Join(e.entityCountriesDir(), code+".json")
		sidecarInfo, err := os.Stat(sidecarPath)
		if err != nil {
			if os.IsNotExist(err) {
				findings = append(findings, EntityIntegrityFinding{
					Scope:          "country",
					Kind:           "detail_sidecar_missing",
					Subject:        code,
					Country:        code,
					Path:           sidecarPath,
					ReferencePath:  ref.path,
					ReferenceMTime: ref.when,
					RepairAction:   "refresh_entity",
					Reason:         "country detail sidecar is missing",
				})
				plan.addCountry(code)
				continue
			}
			return nil, plan, err
		}
		sidecarMTime := sidecarInfo.ModTime().UTC()
		if _, err := loadCountryDetailSidecar(sidecarPath); err != nil {
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "country",
				Kind:         "detail_sidecar_malformed",
				Subject:      code,
				Country:      code,
				Path:         sidecarPath,
				PathMTime:    sidecarMTime,
				RepairAction: "refresh_entity",
				Reason:       "country detail sidecar is unreadable",
			})
			plan.addCountry(code)
			continue
		}
		publicPath := e.PublicCountryDetailPath(code)
		publicInfo, err := os.Stat(publicPath)
		if err != nil {
			if os.IsNotExist(err) {
				findings = append(findings, EntityIntegrityFinding{
					Scope:          "country",
					Kind:           "detail_public_missing",
					Subject:        code,
					Country:        code,
					Path:           publicPath,
					ReferencePath:  sidecarPath,
					ReferenceMTime: sidecarMTime,
					RepairAction:   "refresh_entity",
					Reason:         "country public JSON is missing",
				})
				plan.addCountry(code)
				continue
			}
			return nil, plan, err
		}
		publicMTime := publicInfo.ModTime().UTC()
		healths, err := loadCountryDetailPublicHealth(publicPath)
		if err != nil {
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "country",
				Kind:         "detail_public_malformed",
				Subject:      code,
				Country:      code,
				Path:         publicPath,
				PathMTime:    publicMTime,
				RepairAction: "refresh_entity",
				Reason:       "country public JSON is unreadable",
			})
			plan.addCountry(code)
			continue
		}
		countryPublicHealth[code] = healths
		if publicMTime.Before(sidecarMTime) {
			findings = append(findings, EntityIntegrityFinding{
				Scope:          "country",
				Kind:           "detail_public_stale",
				Subject:        code,
				Country:        code,
				Path:           publicPath,
				PathMTime:      publicMTime,
				ReferencePath:  sidecarPath,
				ReferenceMTime: sidecarMTime,
				RepairAction:   "refresh_entity",
				Reason:         "country public JSON is older than its private sidecar",
			})
			plan.addCountry(code)
		}
	}

	for asn, ref := range asnRefs {
		subject := strconv.FormatUint(uint64(asn), 10)
		sidecarPath := filepath.Join(e.entityASNsDir(), subject+".json")
		sidecarInfo, err := os.Stat(sidecarPath)
		if err != nil {
			if os.IsNotExist(err) {
				findings = append(findings, EntityIntegrityFinding{
					Scope:          "asn",
					Kind:           "detail_sidecar_missing",
					Subject:        subject,
					ASN:            asn,
					Path:           sidecarPath,
					ReferencePath:  ref.path,
					ReferenceMTime: ref.when,
					RepairAction:   "refresh_entity",
					Reason:         "ASN detail sidecar is missing",
				})
				plan.addASN(asn)
				continue
			}
			return nil, plan, err
		}
		sidecarMTime := sidecarInfo.ModTime().UTC()
		if _, err := loadASNDetailSidecar(sidecarPath); err != nil {
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "asn",
				Kind:         "detail_sidecar_malformed",
				Subject:      subject,
				ASN:          asn,
				Path:         sidecarPath,
				PathMTime:    sidecarMTime,
				RepairAction: "refresh_entity",
				Reason:       "ASN detail sidecar is unreadable",
			})
			plan.addASN(asn)
			continue
		}
		publicPath := e.PublicASNDetailPath(asn)
		publicInfo, err := os.Stat(publicPath)
		if err != nil {
			if os.IsNotExist(err) {
				findings = append(findings, EntityIntegrityFinding{
					Scope:          "asn",
					Kind:           "detail_public_missing",
					Subject:        subject,
					ASN:            asn,
					Path:           publicPath,
					ReferencePath:  sidecarPath,
					ReferenceMTime: sidecarMTime,
					RepairAction:   "refresh_entity",
					Reason:         "ASN public JSON is missing",
				})
				plan.addASN(asn)
				continue
			}
			return nil, plan, err
		}
		publicMTime := publicInfo.ModTime().UTC()
		healths, err := loadASNDetailPublicHealth(publicPath)
		if err != nil {
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "asn",
				Kind:         "detail_public_malformed",
				Subject:      subject,
				ASN:          asn,
				Path:         publicPath,
				PathMTime:    publicMTime,
				RepairAction: "refresh_entity",
				Reason:       "ASN public JSON is unreadable",
			})
			plan.addASN(asn)
			continue
		}
		asnPublicHealth[asn] = healths
		if publicMTime.Before(sidecarMTime) {
			findings = append(findings, EntityIntegrityFinding{
				Scope:          "asn",
				Kind:           "detail_public_stale",
				Subject:        subject,
				ASN:            asn,
				Path:           publicPath,
				PathMTime:      publicMTime,
				ReferencePath:  sidecarPath,
				ReferenceMTime: sidecarMTime,
				RepairAction:   "refresh_entity",
				Reason:         "ASN public JSON is older than its private sidecar",
			})
			plan.addASN(asn)
		}
	}

	countryIndexPath := e.PublicCountryIndexPath()
	if _, err := os.Stat(countryIndexPath); err != nil {
		if os.IsNotExist(err) {
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "index",
				Kind:         "country_index_missing",
				Subject:      "countries",
				Path:         countryIndexPath,
				RepairAction: "refresh_index",
				Reason:       "country index JSON is missing",
			})
			plan.rebuildCountryIndex = true
		} else {
			return nil, plan, err
		}
	}

	asnIndexPath := e.PublicASNIndexPath()
	if _, err := os.Stat(asnIndexPath); err != nil {
		if os.IsNotExist(err) {
			findings = append(findings, EntityIntegrityFinding{
				Scope:        "index",
				Kind:         "asn_index_missing",
				Subject:      "asns",
				Path:         asnIndexPath,
				RepairAction: "refresh_index",
				Reason:       "ASN index JSON is missing",
			})
			plan.rebuildASNIndex = true
		} else {
			return nil, plan, err
		}
	}

	if err := e.checkHomeAggregatesIntegrity(&findings, &plan, health); err != nil {
		return nil, plan, err
	}

	for _, check := range healthChecks {
		staleCountries := 0
		for _, code := range check.countries {
			normalized := strings.ToUpper(strings.TrimSpace(code))
			if _, pending := plan.countryCodes[normalized]; pending {
				continue
			}
			healths, ok := countryPublicHealth[normalized]
			if ok && healths[check.feed] != check.healthClass {
				staleCountries++
			}
		}
		staleASNs := 0
		for _, asn := range check.asns {
			if _, pending := plan.asns[asn]; pending {
				continue
			}
			healths, ok := asnPublicHealth[asn]
			if ok && healths[check.feed] != check.healthClass {
				staleASNs++
			}
		}
		if staleCountries == 0 && staleASNs == 0 {
			continue
		}
		findings = append(findings, EntityIntegrityFinding{
			Scope:             "feed",
			Kind:              "detail_health_stale",
			Subject:           check.feed,
			Feed:              check.feed,
			ReferenceMTime:    check.transitionAt,
			RepairAction:      "refresh_health",
			Reason:            "entity public payloads are older than the feed's current health transition",
			AffectedCountries: staleCountries,
			AffectedASNs:      staleASNs,
		})
		plan.addHealthFeed(check.feed)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Scope != findings[j].Scope {
			return findings[i].Scope < findings[j].Scope
		}
		if findings[i].Subject != findings[j].Subject {
			return findings[i].Subject < findings[j].Subject
		}
		return findings[i].Kind < findings[j].Kind
	})
	return findings, plan, nil
}

func (e *Engine) entityFeedSidecarReference(name, geoProvider, asnProvider string, geoPath string, geoTime time.Time, asnPath string, asnTime time.Time) (string, time.Time, error) {
	return e.entityFeedSidecarReferenceInOutputDir(name, e.outputDir(), geoProvider, asnProvider, geoPath, geoTime, asnPath, asnTime)
}

func (e *Engine) entityFeedSidecarReferenceInOutputDir(name, outDir, geoProvider, asnProvider string, geoPath string, geoTime time.Time, asnPath string, asnTime time.Time) (string, time.Time, error) {
	var ref entityDependencyRef
	if geoProvider != "" {
		path, when, err := e.entityFeedCountryPayloadReferenceInOutputDir(outDir, name, geoProvider)
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, when)
	}
	if asnProvider != "" {
		path, when, err := e.entityFeedASNPayloadReferenceInOutputDir(outDir, name, asnProvider)
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, when)
	}
	latestPath, latestTime, err := e.entityLatestSetReference(name)
	if err != nil {
		return "", time.Time{}, err
	}
	mergeEntityDependencyRef(&ref, latestPath, latestTime)
	mergeEntityDependencyRef(&ref, geoPath, geoTime)
	mergeEntityDependencyRef(&ref, asnPath, asnTime)
	return ref.path, ref.when, nil
}

func (e *Engine) feedEntitySidecarCoversReference(name string, sidecar *feedEntitySidecar, refPath string, refTime time.Time) bool {
	if sidecar == nil || sidecar.LastChangeTS <= 0 || refTime.IsZero() {
		return false
	}
	refPath = filepath.Clean(strings.TrimSpace(refPath))
	if refPath == "" {
		return false
	}
	candidates := []string{
		filepath.Join(e.runtime.LibDir, name, "latest"),
		filepath.Join(e.runtime.LibDir, name, "latest.set"),
		latestFeedBodyPath(e.feedBodyPath(name)),
	}
	var isLatestSetRef bool
	for _, candidate := range candidates {
		if refPath == filepath.Clean(candidate) {
			isLatestSetRef = true
			break
		}
	}
	if !isLatestSetRef {
		return false
	}
	sidecarSourceTime := time.Unix(sidecar.LastChangeTS, 0).UTC()
	return !sidecarSourceTime.Before(refTime.UTC().Truncate(time.Second))
}

func (e *Engine) feedEntityInputsHaveContributions(name, geoProvider, asnProvider string) (bool, error) {
	view := newEntityOutputView(e, "")
	if geoProvider != "" {
		payload, err := view.countryComparison(name, geoProvider)
		if err != nil {
			if !os.IsNotExist(err) {
				return false, err
			}
		} else {
			for _, country := range payload.Countries {
				if strings.TrimSpace(country.Code) != "" && country.Value > 0 {
					return true, nil
				}
			}
		}
	}
	if asnProvider != "" {
		data, err := readFirstExisting(asnCandidatePaths(e.outputDir(), name, asnProvider))
		if err != nil {
			if !os.IsNotExist(err) {
				return false, err
			}
			return false, nil
		}
		var payload struct {
			ByASN []struct {
				ASN   uint32 `json:"asn"`
				Count uint64 `json:"count"`
			} `json:"by_asn"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return false, err
		}
		for _, row := range payload.ByASN {
			if row.ASN != 0 && row.Count > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *Engine) entityFeedCountryPayloadReferenceInOutputDir(outDir, name, provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	if strings.TrimSpace(outDir) == "" {
		outDir = e.outputDir()
	}
	path := filepath.Join(outDir, name+"_"+provider+".json")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityFeedASNPayloadReferenceInOutputDir(outDir, name, provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	if strings.TrimSpace(outDir) == "" {
		outDir = e.outputDir()
	}
	path := filepath.Join(outDir, name+"_asn_"+provider+".json")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityGeoProviderReference(provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	path := filepath.Join(e.runtime.LibDir, "geolocation", provider+".source")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityASNProviderReference(provider string) (string, time.Time, error) {
	if provider == "" {
		return "", time.Time{}, nil
	}
	src := e.lookupSource(provider)
	if src == nil {
		return "", time.Time{}, nil
	}
	spec, ok := lookupFormat(src.Format)
	if !ok {
		return "", time.Time{}, nil
	}
	path := filepath.Join(e.runtime.LibDir, "asn", provider, spec.dataFile)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) entityLatestSetReference(name string) (string, time.Time, error) {
	for _, filename := range []string{"latest", "latest.set"} {
		path := filepath.Join(e.runtime.LibDir, name, filename)
		info, err := os.Stat(path)
		if err == nil {
			return path, info.ModTime().UTC(), nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", time.Time{}, err
		}
	}
	path := latestFeedBodyPath(e.feedBodyPath(name))
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	return path, info.ModTime().UTC(), nil
}

func (e *Engine) latestEntityConfigInputTime() (string, time.Time, error) {
	var ref entityDependencyRef
	if e.runtime.ConfigPath != "" {
		info, err := os.Stat(e.runtime.ConfigPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", time.Time{}, err
			}
		} else {
			mergeEntityDependencyRef(&ref, e.runtime.ConfigPath, info.ModTime().UTC())
		}
	}
	for _, dir := range []string{e.runtime.DistributionSuppliedIPSets, e.runtime.AdminSuppliedIPSets, e.runtime.UserSuppliedIPSets} {
		path, when, err := newestConfigFragmentTime(dir)
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, when)
	}
	return ref.path, ref.when, nil
}

func newestConfigFragmentTime(dir string) (string, time.Time, error) {
	if strings.TrimSpace(dir) == "" {
		return "", time.Time{}, nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", time.Time{}, nil
		}
		return "", time.Time{}, err
	}
	if !info.IsDir() {
		return "", time.Time{}, fmt.Errorf("%s is not a directory", dir)
	}
	var ref entityDependencyRef
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", time.Time{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".yaml", ".yml", ".conf":
		default:
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return "", time.Time{}, err
		}
		mergeEntityDependencyRef(&ref, path, info.ModTime().UTC())
	}
	return ref.path, ref.when, nil
}

func (e *Engine) entityFeedHealthTransitionTime(name string, health *feedHealthClassifier) time.Time {
	if e == nil || e.cfg == nil {
		return time.Time{}
	}
	snap, ok := health.snapshot(name)
	if !ok {
		return time.Time{}
	}
	now := e.now().UTC()
	switch snap.Class {
	case feedhealth.ClassDelayed:
		return transitionAfterLastChange(snap.LastChangeAt, snap.EffectiveHealthyGapMins, now)
	case feedhealth.ClassRisky:
		return transitionAfterLastChange(snap.LastChangeAt, snap.RiskyCadenceMins, now)
	case feedhealth.ClassUnmaintained:
		return transitionAfterLastChange(snap.LastChangeAt, snap.UnmaintainedThresholdMins, now)
	case feedhealth.ClassUnavailable:
		return entityUnavailableTransitionTime(snap, now)
	case feedhealth.ClassArchived:
		return entityArchivedTransitionTime(snap, now)
	default:
		return time.Time{}
	}
}

func transitionAfterLastChange(lastChangeAt int64, minutes int, now time.Time) time.Time {
	if lastChangeAt <= 0 || minutes <= 0 {
		return time.Time{}
	}
	transition := time.Unix(lastChangeAt, 0).UTC().Add(time.Duration(minutes) * time.Minute)
	if transition.After(now) {
		return time.Time{}
	}
	return transition
}

func entityUnavailableTransitionTime(snap feedhealth.Snapshot, now time.Time) time.Time {
	threshold := entityUnavailableThresholdMins(snap)
	if threshold <= 0 {
		return time.Time{}
	}
	candidates := make([]time.Time, 0, 2)
	if snap.FailureStartedAt > 0 {
		candidates = append(candidates, time.Unix(snap.FailureStartedAt, 0).UTC().Add(time.Duration(threshold)*time.Minute))
	}
	if snap.LastChangeAt > 0 {
		candidates = append(candidates, time.Unix(snap.LastChangeAt, 0).UTC().Add(time.Duration(threshold)*time.Minute))
	}
	return earliestPastTime(candidates, now)
}

func entityArchivedTransitionTime(snap feedhealth.Snapshot, now time.Time) time.Time {
	threshold := entityUnavailableThresholdMins(snap)
	if threshold <= 0 || snap.ArchivalThresholdMins <= 0 {
		return time.Time{}
	}
	candidates := make([]time.Time, 0, 2)
	if snap.FailureStartedAt > 0 {
		candidates = append(candidates, time.Unix(snap.FailureStartedAt, 0).UTC().Add(time.Duration(threshold+snap.ArchivalThresholdMins)*time.Minute))
	}
	if snap.LastChangeAt > 0 {
		candidates = append(candidates, time.Unix(snap.LastChangeAt, 0).UTC().Add(time.Duration(threshold+snap.ArchivalThresholdMins)*time.Minute))
	}
	return earliestPastTime(candidates, now)
}

func earliestPastTime(candidates []time.Time, now time.Time) time.Time {
	var earliest time.Time
	for _, candidate := range candidates {
		if candidate.IsZero() || candidate.After(now) {
			continue
		}
		if earliest.IsZero() || candidate.Before(earliest) {
			earliest = candidate
		}
	}
	return earliest
}

func entityUnavailableThresholdMins(snap feedhealth.Snapshot) int {
	threshold := snap.UnmaintainedThresholdMins
	if snap.ObservedUpdates <= 1 && snap.SingleObservationGraceMins > threshold {
		threshold = snap.SingleObservationGraceMins
	}
	return threshold
}

func mergeEntityDependencyRef(dst *entityDependencyRef, path string, when time.Time) {
	if dst == nil || when.IsZero() {
		return
	}
	when = when.UTC()
	if dst.when.IsZero() || when.After(dst.when) {
		dst.path = path
		dst.when = when
	}
}

func loadCountryDetailPublicHealth(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload CountryDetailPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	healths := make(map[string]string, len(payload.Feeds))
	for _, row := range payload.Feeds {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		healths[row.Name] = row.HealthClass
	}
	return healths, nil
}

func loadASNDetailPublicHealth(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload ASNDetailPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	healths := make(map[string]string, len(payload.Feeds))
	for _, row := range payload.Feeds {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		healths[row.Name] = row.HealthClass
	}
	return healths, nil
}
