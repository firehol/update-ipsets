package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/output"
)

type feedEntityDelta struct {
	name     string
	old      *feedEntitySidecar
	new      *feedEntitySidecar
	oldMTime time.Time
	newMTime time.Time
	oldIndex feedEntitySidecarIndex
	newIndex feedEntitySidecarIndex
}

var errEntitySurgicalNeedsFullRebuild = errors.New("entity surgical refresh requires full rebuild")

func (e *Engine) RefreshEntityArtifactsForFeedUpdates(ctx context.Context, feedNames []string, trigger string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	feedNames = uniqueNonEmptyStrings(feedNames)
	if len(feedNames) == 0 {
		return nil
	}
	if e.preferredGeoProvider() == "" && e.preferredASNProvider() == "" {
		return nil
	}
	if trigger == "" {
		trigger = "feed_update"
	}
	if e.entityArtifactsNeedBootstrapFast() {
		return e.RebuildEntityArtifactsWithTrigger(ctx, trigger)
	}
	return e.withBackgroundTask(
		"Entity artifacts refresh",
		trigger,
		"planning",
		backgroundEntityTaskDetail("feeds", len(feedNames)),
		0,
		len(feedNames),
		func(task *BackgroundTaskHandle) error {
			return e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("feeds", len(feedNames)), func() error {
				return e.refreshEntityArtifactsForFeedUpdates(ctx, feedNames, task)
			})
		},
	)
}

func (e *Engine) refreshEntityArtifactsForFeedUpdates(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	if len(feedNames) == 0 {
		return nil
	}
	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		return err
	}
	defer webBatch.cleanup()
	entityBatch, err := e.newEntityPublishBatch()
	if err != nil {
		return err
	}
	defer entityBatch.cleanup()

	targetFeeds := filterPublicOutputNames(e, feedNames)
	if len(targetFeeds) == 0 {
		return nil
	}
	e.observeRunCounter("entity.refresh.target_feeds", int64(len(targetFeeds)), 0)
	if task != nil {
		task.Update("loading feed deltas", fmt.Sprintf("loading committed and pending feed entity sidecars for %d feeds", len(targetFeeds)), 0, len(targetFeeds))
	}

	deltas := make([]feedEntityDelta, 0, len(targetFeeds))
	affectedCountries := map[string]struct{}{}
	affectedASNs := map[uint32]struct{}{}
	for i, name := range targetFeeds {
		if err := contextErr(ctx); err != nil {
			return err
		}
		delta, err := e.buildFeedEntityDelta(name)
		if err != nil {
			if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
				e.observeRunCounter("entity.refresh.full_rebuild_fallback", 1, 0)
				return e.rebuildEntityArtifactsFromLive(ctx, task)
			}
			return err
		}
		if delta.old == nil && delta.new == nil {
			continue
		}
		deltas = append(deltas, delta)
		addChangedActorTargets(affectedCountries, affectedASNs, delta)
		if delta.new == nil {
			entityBatch.markDelete(e.entityFeedSidecarRelPath(name))
		} else if err := e.writeObservedJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityFeedSidecarRelPath(name)), delta.new, delta.newMTime, "entity.refresh.feed_sidecar_write"); err != nil {
			return err
		}
		entityBatch.markDelete(e.entityFeedPendingRelPath(name))
		if task != nil {
			task.Update("loading feed deltas", fmt.Sprintf("loading committed and pending feed entity sidecars for %d feeds", len(targetFeeds)), i+1, len(targetFeeds))
		}
	}

	if len(deltas) == 0 {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if err := writeFileAtomic(filepath.Join(entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
			return err
		}
		if _, err := entityBatch.publish(); err != nil {
			return err
		}
		return nil
	}

	if task != nil {
		task.Update(
			"patching entity details",
			fmt.Sprintf("patching affected entity artifacts: %d countries and %d ASNs", len(affectedCountries), len(affectedASNs)),
			0,
			len(affectedCountries)+len(affectedASNs),
		)
	}
	e.observeRunCounter("entity.refresh.affected_countries", int64(len(affectedCountries)), 0)
	e.observeRunCounter("entity.refresh.affected_asns", int64(len(affectedASNs)), 0)

	generated := make([]output.GeneratedFile, 0, len(affectedCountries)+len(affectedASNs)+2)
	countryUpdates := map[string]*countryDetailSidecar{}
	asnUpdates := map[uint32]*asnDetailSidecar{}
	feedTimes := e.loadFeedEntitySidecarMTimes()
	for _, delta := range deltas {
		if delta.new == nil {
			delete(feedTimes, delta.name)
			continue
		}
		feedTimes[delta.name] = delta.newMTime
	}
	health := e.newFeedHealthClassifier()
	progress := 0
	total := len(affectedCountries) + len(affectedASNs)
	for _, code := range sortedStringSet(affectedCountries) {
		if err := contextErr(ctx); err != nil {
			return err
		}
		sidecar, changed, err := e.patchCountrySidecarForFeedDeltas(code, deltas)
		if err != nil {
			if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
				return e.rebuildEntityArtifactsFromLive(ctx, task)
			}
			return err
		}
		if !changed {
			if sidecar != nil {
				logicalTime := countryDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
				privatePath := filepath.Join(e.entityCountriesDir(), strings.ToUpper(strings.TrimSpace(code))+".json")
				publicPath := e.PublicCountryDetailPath(code)
				if entityDetailFilesExist(privatePath, publicPath) {
					if err := e.touchObservedFileAt(privatePath, "entity.refresh.country_sidecar_touch", logicalTime); err != nil {
						return err
					}
					if err := e.touchObservedFileAt(publicPath, "entity.refresh.country_public_touch", logicalTime); err != nil {
						return err
					}
				}
			}
			e.observeRunCounter("entity.refresh.country_unchanged", 1, 0)
			progress++
			if task != nil {
				task.Update("patching entity details", fmt.Sprintf("patching affected entity artifacts: %d countries and %d ASNs", len(affectedCountries), len(affectedASNs)), progress, total)
			}
			continue
		}
		countryUpdates[code] = sidecar
		if sidecar == nil {
			entityBatch.markDelete(e.entityCountrySidecarRelPath(code))
			webBatch.markDelete(e.publicCountryDetailRelPath(code))
		} else {
			logicalTime := countryDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
			if err := e.writeObservedJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityCountrySidecarRelPath(code)), sidecar, logicalTime, "entity.refresh.country_sidecar_write"); err != nil {
				return err
			}
			rel := e.publicCountryDetailRelPath(code)
			materializeStart := time.Now()
			payload := e.materializeCountryDetailWithHealth(sidecar, health)
			e.observeRunOperation("entity.refresh.country_materialize", time.Since(materializeStart))
			if err := e.writeObservedJSONFileAt(filepath.Join(webBatch.stageDir, rel), payload, logicalTime, "entity.refresh.country_public_write"); err != nil {
				return err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
			if mdFile, _ := e.stageCountryMarkdown(code, payload, webBatch.stageDir); mdFile.Path != "" {
				mdFile.Timestamp = logicalTime
				generated = append(generated, mdFile)
			}
		}
		progress++
		if task != nil {
			task.Update("patching entity details", fmt.Sprintf("patching affected entity artifacts: %d countries and %d ASNs", len(affectedCountries), len(affectedASNs)), progress, total)
		}
	}
	for _, asn := range sortedUint32Set(affectedASNs) {
		if err := contextErr(ctx); err != nil {
			return err
		}
		sidecar, changed, err := e.patchASNSidecarForFeedDeltas(asn, deltas)
		if err != nil {
			if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
				return e.rebuildEntityArtifactsFromLive(ctx, task)
			}
			return err
		}
		if !changed {
			if sidecar != nil {
				logicalTime := asnDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
				privatePath := filepath.Join(e.entityASNsDir(), strconv.FormatUint(uint64(asn), 10)+".json")
				publicPath := e.PublicASNDetailPath(asn)
				if entityDetailFilesExist(privatePath, publicPath) {
					if err := e.touchObservedFileAt(privatePath, "entity.refresh.asn_sidecar_touch", logicalTime); err != nil {
						return err
					}
					if err := e.touchObservedFileAt(publicPath, "entity.refresh.asn_public_touch", logicalTime); err != nil {
						return err
					}
				}
			}
			e.observeRunCounter("entity.refresh.asn_unchanged", 1, 0)
			progress++
			if task != nil {
				task.Update("patching entity details", fmt.Sprintf("patching affected entity artifacts: %d countries and %d ASNs", len(affectedCountries), len(affectedASNs)), progress, total)
			}
			continue
		}
		asnUpdates[asn] = sidecar
		if sidecar == nil {
			entityBatch.markDelete(e.entityASNSidecarRelPath(asn))
			webBatch.markDelete(e.publicASNDetailRelPath(asn))
		} else {
			logicalTime := asnDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
			if err := e.writeObservedJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityASNSidecarRelPath(asn)), sidecar, logicalTime, "entity.refresh.asn_sidecar_write"); err != nil {
				return err
			}
			rel := e.publicASNDetailRelPath(asn)
			materializeStart := time.Now()
			payload := e.materializeASNDetailWithHealth(sidecar, health)
			e.observeRunOperation("entity.refresh.asn_materialize", time.Since(materializeStart))
			if err := e.writeObservedJSONFileAt(filepath.Join(webBatch.stageDir, rel), payload, logicalTime, "entity.refresh.asn_public_write"); err != nil {
				return err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
			if mdFile, _ := e.stageASNMarkdown(asn, payload, webBatch.stageDir); mdFile.Path != "" {
				mdFile.Timestamp = logicalTime
				generated = append(generated, mdFile)
			}
		}
		progress++
		if task != nil {
			task.Update("patching entity details", fmt.Sprintf("patching affected entity artifacts: %d countries and %d ASNs", len(affectedCountries), len(affectedASNs)), progress, total)
		}
	}
	indexSteps := 0
	if len(countryUpdates) > 0 {
		indexSteps++
	}
	if len(asnUpdates) > 0 {
		indexSteps++
	}
	if indexSteps > 0 {
		indexProgress := 0
		if task != nil {
			task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", 0, indexSteps)
		}
		if len(countryUpdates) > 0 {
			if err := e.patchCountryIndex(webBatch, countryUpdates); err != nil {
				return err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), e.publicCountryIndexRelPath()), Redistributable: true})
			indexProgress++
			if task != nil {
				task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", indexProgress, indexSteps)
			}
		}
		if len(asnUpdates) > 0 {
			if err := e.patchASNIndex(webBatch, asnUpdates); err != nil {
				return err
			}
			generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), e.publicASNIndexRelPath()), Redistributable: true})
			indexProgress++
			if task != nil {
				task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", indexProgress, indexSteps)
			}
		}
	}

	if err := writeFileAtomic(filepath.Join(entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		return err
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if task != nil {
		task.Update("publishing", "publishing patched entity artifacts", total, total)
	}
	if _, err := entityBatch.publish(); err != nil {
		return err
	}
	if err := webBatch.applyGeneratedFileTimestamps(generated); err != nil {
		return err
	}
	published, err := webBatch.publish()
	if err != nil {
		return err
	}
	return e.syncGeneratedFiles(generated, published)
}

func (e *Engine) buildFeedEntityDelta(name string) (feedEntityDelta, error) {
	delta := feedEntityDelta{name: name}

	oldPath := filepath.Join(e.entityFeedsDir(), name+".json")
	oldSidecar, err := e.loadFeedEntitySidecar(oldPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return delta, err
		}
		found, scanErr := e.entityArtifactsContainFeed(name)
		if scanErr != nil {
			return delta, scanErr
		}
		if found {
			return delta, errEntitySurgicalNeedsFullRebuild
		}
	} else {
		if oldSidecar.legacy {
			return delta, errEntitySurgicalNeedsFullRebuild
		}
		delta.old = oldSidecar
		if info, statErr := os.Stat(oldPath); statErr == nil {
			delta.oldMTime = info.ModTime().UTC()
		}
		delta.oldIndex = indexFeedEntitySidecar(oldSidecar)
	}

	newPath := filepath.Join(e.entityFeedPendingDir(), name+".json")
	newSidecar, err := e.loadFeedEntitySidecar(newPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return delta, fmt.Errorf("%w: pending feed sidecar for %s is unreadable: %w", errEntitySurgicalNeedsFullRebuild, name, err)
		}
	} else {
		if newSidecar.legacy {
			return delta, fmt.Errorf("%w: pending feed sidecar for %s uses legacy membership-only format", errEntitySurgicalNeedsFullRebuild, name)
		}
		delta.new = newSidecar
		if info, statErr := os.Stat(newPath); statErr == nil {
			delta.newMTime = info.ModTime().UTC()
		}
		delta.newIndex = indexFeedEntitySidecar(newSidecar)
	}
	return delta, nil
}

func (e *Engine) patchCountrySidecarForFeedDeltas(code string, deltas []feedEntityDelta) (*countryDetailSidecar, bool, error) {
	patchStart := time.Now()
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, false, nil
	}
	defer func() {
		e.observeRunOperation("entity.refresh.country_patch", time.Since(patchStart))
	}()
	path := filepath.Join(e.entityCountriesDir(), code+".json")
	start := time.Now()
	sidecar, err := loadCountryDetailSidecar(path)
	var original *countryDetailSidecar
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, false, err
		}
		sidecar = e.emptyCountryDetailSidecar(code)
	} else {
		original = sidecar
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.refresh.country_sidecar_read", 1, bytes)
		e.observeRunOperation("entity.refresh.country_sidecar_read", time.Since(start))
	}
	feeds := removeCountryFeedRows(sidecar.Feeds, deltas)
	asnTotals := countryASNAggregatesFromSidecar(sidecar)
	for _, delta := range deltas {
		if contribution, ok := delta.old.countryActorContribution(code, delta.oldIndex); ok {
			if err := applyCountryJointRows(asnTotals, contribution.asns, -1); err != nil {
				return nil, false, fmt.Errorf("subtract old %s contribution from country %s: %w", delta.name, code, err)
			}
		}
		if contribution, ok := delta.new.countryActorContribution(code, delta.newIndex); ok {
			feeds = append(feeds, contribution.feed)
			if err := applyCountryJointRows(asnTotals, contribution.asns, 1); err != nil {
				return nil, false, fmt.Errorf("add new %s contribution to country %s: %w", delta.name, code, err)
			}
		}
	}
	updated := e.rebuildCountrySidecarFromParts(code, sidecar, feeds, asnTotals)
	return updated, !reflect.DeepEqual(original, updated), nil
}

func (e *Engine) patchASNSidecarForFeedDeltas(asn uint32, deltas []feedEntityDelta) (*asnDetailSidecar, bool, error) {
	patchStart := time.Now()
	if asn == 0 {
		return nil, false, nil
	}
	defer func() {
		e.observeRunOperation("entity.refresh.asn_patch", time.Since(patchStart))
	}()
	path := filepath.Join(e.entityASNsDir(), strconv.FormatUint(uint64(asn), 10)+".json")
	start := time.Now()
	sidecar, err := loadASNDetailSidecar(path)
	var original *asnDetailSidecar
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, false, err
		}
		sidecar = e.emptyASNDetailSidecar(asn)
	} else {
		original = sidecar
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.refresh.asn_sidecar_read", 1, bytes)
		e.observeRunOperation("entity.refresh.asn_sidecar_read", time.Since(start))
	}
	feeds := removeASNFeedRows(sidecar.Feeds, deltas)
	countryTotals := asnCountryAggregatesFromSidecar(sidecar)
	for _, delta := range deltas {
		if contribution, ok := delta.old.asnActorContribution(asn, delta.oldIndex); ok {
			if err := applyASNCountryRows(countryTotals, contribution.countries, -1); err != nil {
				return nil, false, fmt.Errorf("subtract old %s contribution from ASN %d: %w", delta.name, asn, err)
			}
		}
		if contribution, ok := delta.new.asnActorContribution(asn, delta.newIndex); ok {
			feeds = append(feeds, contribution.feed)
			if sidecar.Name == "" && contribution.name != "" {
				sidecar.Name = contribution.name
			}
			if err := applyASNCountryRows(countryTotals, contribution.countries, 1); err != nil {
				return nil, false, fmt.Errorf("add new %s contribution to ASN %d: %w", delta.name, asn, err)
			}
		}
	}
	updated := e.rebuildASNSidecarFromParts(asn, sidecar, feeds, countryTotals)
	return updated, !reflect.DeepEqual(original, updated), nil
}

func addChangedActorTargets(countries map[string]struct{}, asns map[uint32]struct{}, delta feedEntityDelta) {
	countryCandidates := map[string]struct{}{}
	for _, code := range delta.old.countryCodes() {
		countryCandidates[code] = struct{}{}
	}
	for _, code := range delta.new.countryCodes() {
		countryCandidates[code] = struct{}{}
	}
	for code := range countryCandidates {
		oldContribution, oldOK := delta.old.countryActorContribution(code, delta.oldIndex)
		newContribution, newOK := delta.new.countryActorContribution(code, delta.newIndex)
		if oldOK != newOK || !reflect.DeepEqual(oldContribution, newContribution) {
			countries[code] = struct{}{}
		}
	}

	asnCandidates := map[uint32]struct{}{}
	for _, asn := range delta.old.asnNumbers() {
		asnCandidates[asn] = struct{}{}
	}
	for _, asn := range delta.new.asnNumbers() {
		asnCandidates[asn] = struct{}{}
	}
	for asn := range asnCandidates {
		oldContribution, oldOK := delta.old.asnActorContribution(asn, delta.oldIndex)
		newContribution, newOK := delta.new.asnActorContribution(asn, delta.newIndex)
		if oldOK != newOK || !reflect.DeepEqual(oldContribution, newContribution) {
			asns[asn] = struct{}{}
		}
	}
}

func (e *Engine) entityArtifactsContainFeed(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, nil
	}
	countryFiles, err := sortedJSONFiles(e.entityCountriesDir())
	if err != nil {
		return false, err
	}
	e.observeRunCounter("entity.repair_feed_scan.country_files", int64(len(countryFiles)), 0)
	for _, path := range countryFiles {
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.repair_feed_scan.country_sidecar_read", 1, bytes)
		sidecar, err := loadCountryDetailSidecar(path)
		if err != nil {
			return false, err
		}
		for _, row := range sidecar.Feeds {
			if row.Name == name {
				return true, nil
			}
		}
	}
	asnFiles, err := sortedJSONFiles(e.entityASNsDir())
	if err != nil {
		return false, err
	}
	e.observeRunCounter("entity.repair_feed_scan.asn_files", int64(len(asnFiles)), 0)
	for _, path := range asnFiles {
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.repair_feed_scan.asn_sidecar_read", 1, bytes)
		sidecar, err := loadASNDetailSidecar(path)
		if err != nil {
			return false, err
		}
		for _, row := range sidecar.Feeds {
			if row.Name == name {
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *Engine) writeObservedJSONFile(path string, value any, metric string) error {
	start := time.Now()
	data, err := jsonMarshalTabIndent(value)
	if err != nil {
		return err
	}
	body := append(data, '\n')
	if err := writeFileAtomicNoSync(path, body, generatedFileMode); err != nil {
		return err
	}
	e.observeRunCounter(metric, 1, int64(len(body)))
	e.observeRunOperation(metric, time.Since(start))
	return nil
}

func (e *Engine) writeObservedJSONFileAt(path string, value any, mod time.Time, metric string) error {
	if err := e.writeObservedJSONFile(path, value, metric); err != nil {
		return err
	}
	if mod.IsZero() {
		return nil
	}
	return os.Chtimes(path, mod.UTC(), mod.UTC())
}

func (e *Engine) touchObservedFileAt(path string, metric string, mod time.Time) error {
	start := time.Now()
	if mod.IsZero() {
		mod = time.Now()
	}
	mod = mod.UTC()
	if err := os.Chtimes(path, mod, mod); err != nil {
		return err
	}
	e.observeRunCounter(metric, 1, 0)
	e.observeRunOperation(metric, time.Since(start))
	return nil
}

func entityDetailFilesExist(privatePath, publicPath string) bool {
	if _, err := os.Stat(privatePath); err != nil {
		return false
	}
	if _, err := os.Stat(publicPath); err != nil {
		return false
	}
	return true
}

func (e *Engine) emptyCountryDetailSidecar(code string) *countryDetailSidecar {
	geoProvider := e.preferredGeoProvider()
	asnProvider := e.preferredASNProvider()
	return &countryDetailSidecar{
		Code: strings.ToUpper(strings.TrimSpace(code)),
		Provider: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(e.lookupSource(geoProvider)),
		},
		ASNProvider: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(e.lookupSource(asnProvider)),
		},
	}
}

func (e *Engine) emptyASNDetailSidecar(asn uint32) *asnDetailSidecar {
	asnProvider := e.preferredASNProvider()
	geoProvider := e.preferredGeoProvider()
	return &asnDetailSidecar{
		ASN: asn,
		Provider: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(e.lookupSource(asnProvider)),
		},
		GeoProvider: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(e.lookupSource(geoProvider)),
		},
	}
}

func (e *Engine) rebuildCountrySidecarFromParts(code string, base *countryDetailSidecar, feeds []countryDetailFeedBase, asnTotals map[uint32]*countryDetailASNAggregate) *countryDetailSidecar {
	if len(feeds) == 0 {
		return nil
	}
	if base == nil {
		base = e.emptyCountryDetailSidecar(code)
	}
	builder := newCountryDetailBuilder(code)
	for _, row := range feeds {
		builder.addFeed(row, sourceMaintainerURL(e.lookupSource(row.Name)))
	}
	builder.asnTotals = asnTotals
	return builder.build(base.Provider, base.ASNProvider)
}

func (e *Engine) rebuildASNSidecarFromParts(asn uint32, base *asnDetailSidecar, feeds []asnDetailFeedBase, countryTotals map[string]*asnDetailCountryAggregate) *asnDetailSidecar {
	if len(feeds) == 0 {
		return nil
	}
	if base == nil {
		base = e.emptyASNDetailSidecar(asn)
	}
	builder := newASNDetailBuilder(asn)
	builder.name = base.Name
	builder.description = base.Description
	for _, row := range feeds {
		builder.addFeed(row, sourceMaintainerURL(e.lookupSource(row.Name)), builder.name)
	}
	builder.countryTotals = countryTotals
	builder.distributionCounts = make(map[string]uint64, len(countryTotals))
	for code, agg := range countryTotals {
		if agg == nil || agg.attributedIPs == 0 {
			continue
		}
		builder.distributionCounts[code] = agg.attributedIPs
		builder.totalMapped += agg.attributedIPs
	}
	return builder.build(base.Provider, base.GeoProvider)
}

func (e *Engine) patchCountryIndex(webBatch *webPublishBatch, updates map[string]*countryDetailSidecar) error {
	payload := e.emptyCountryIndexPayload()
	start := time.Now()
	data, err := readFileInRoot(e.outputDir(), e.publicCountryIndexRelPath())
	if err == nil {
		e.observeRunCounter("entity.refresh.country_index_read", 1, int64(len(data)))
		e.observeRunOperation("entity.refresh.country_index_read", time.Since(start))
		if err := json.Unmarshal(data, payload); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	rows := make(map[string]CountryIndexEntry, len(payload.Countries)+len(updates))
	for _, row := range payload.Countries {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code != "" {
			row.Code = code
			rows[code] = row
		}
	}
	for code, sidecar := range updates {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		if sidecar == nil {
			delete(rows, code)
			continue
		}
		rows[code] = CountryIndexEntry{
			Code:          code,
			FeedCount:     sidecar.Totals.FeedsMatching,
			AttributedIPs: sidecar.Totals.AttributedIPsInFeed,
		}
	}
	payload.Provider = e.emptyCountryIndexPayload().Provider
	payload.Countries = make([]CountryIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.Countries = append(payload.Countries, row)
	}
	sort.Slice(payload.Countries, func(i, j int) bool {
		if payload.Countries[i].FeedCount != payload.Countries[j].FeedCount {
			return payload.Countries[i].FeedCount > payload.Countries[j].FeedCount
		}
		if payload.Countries[i].AttributedIPs != payload.Countries[j].AttributedIPs {
			return payload.Countries[i].AttributedIPs > payload.Countries[j].AttributedIPs
		}
		return payload.Countries[i].Code < payload.Countries[j].Code
	})
	return e.writeObservedJSONFile(filepath.Join(webBatch.stageDir, e.publicCountryIndexRelPath()), payload, "entity.refresh.country_index_write")
}

func (e *Engine) patchASNIndex(webBatch *webPublishBatch, updates map[uint32]*asnDetailSidecar) error {
	payload := e.emptyASNIndexPayload()
	start := time.Now()
	data, err := readFileInRoot(e.outputDir(), e.publicASNIndexRelPath())
	if err == nil {
		e.observeRunCounter("entity.refresh.asn_index_read", 1, int64(len(data)))
		e.observeRunOperation("entity.refresh.asn_index_read", time.Since(start))
		if err := json.Unmarshal(data, payload); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	rows := make(map[uint32]ASNIndexEntry, len(payload.ASNs)+len(updates))
	for _, row := range payload.ASNs {
		if row.ASN != 0 {
			rows[row.ASN] = row
		}
	}
	for asn, sidecar := range updates {
		if asn == 0 {
			continue
		}
		if sidecar == nil {
			delete(rows, asn)
			continue
		}
		rows[asn] = ASNIndexEntry{
			ASN:           asn,
			Name:          sidecar.Name,
			FeedCount:     sidecar.Totals.FeedsMatching,
			AttributedIPs: sidecar.Totals.AttributedIPs,
		}
	}
	payload.Provider = e.emptyASNIndexPayload().Provider
	payload.ASNs = make([]ASNIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.ASNs = append(payload.ASNs, row)
	}
	sort.Slice(payload.ASNs, func(i, j int) bool {
		if payload.ASNs[i].FeedCount != payload.ASNs[j].FeedCount {
			return payload.ASNs[i].FeedCount > payload.ASNs[j].FeedCount
		}
		if payload.ASNs[i].AttributedIPs != payload.ASNs[j].AttributedIPs {
			return payload.ASNs[i].AttributedIPs > payload.ASNs[j].AttributedIPs
		}
		return payload.ASNs[i].ASN < payload.ASNs[j].ASN
	})
	return e.writeObservedJSONFile(filepath.Join(webBatch.stageDir, e.publicASNIndexRelPath()), payload, "entity.refresh.asn_index_write")
}

func (e *Engine) emptyCountryIndexPayload() *CountryIndexPayload {
	provider := e.preferredGeoProvider()
	return &CountryIndexPayload{
		Provider: HomeSummaryProvider{
			Name:  provider,
			Label: providerDisplayLabel(e.lookupSource(provider)),
		},
	}
}

func (e *Engine) emptyASNIndexPayload() *ASNIndexPayload {
	provider := e.preferredASNProvider()
	return &ASNIndexPayload{
		Provider: HomeSummaryProvider{
			Name:  provider,
			Label: providerDisplayLabel(e.lookupSource(provider)),
		},
	}
}

func removeCountryFeedRows(rows []countryDetailFeedBase, deltas []feedEntityDelta) []countryDetailFeedBase {
	remove := make(map[string]struct{}, len(deltas))
	for _, delta := range deltas {
		remove[delta.name] = struct{}{}
	}
	out := make([]countryDetailFeedBase, 0, len(rows))
	for _, row := range rows {
		if _, ok := remove[row.Name]; ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

func removeASNFeedRows(rows []asnDetailFeedBase, deltas []feedEntityDelta) []asnDetailFeedBase {
	remove := make(map[string]struct{}, len(deltas))
	for _, delta := range deltas {
		remove[delta.name] = struct{}{}
	}
	out := make([]asnDetailFeedBase, 0, len(rows))
	for _, row := range rows {
		if _, ok := remove[row.Name]; ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

func countryASNAggregatesFromSidecar(sidecar *countryDetailSidecar) map[uint32]*countryDetailASNAggregate {
	out := map[uint32]*countryDetailASNAggregate{}
	if sidecar == nil {
		return out
	}
	for _, row := range sidecar.TopASNs {
		if row.ASN == 0 || row.AttributedIPs == 0 || row.FeedCount <= 0 {
			continue
		}
		out[row.ASN] = &countryDetailASNAggregate{
			name:          row.Name,
			feedCount:     row.FeedCount,
			attributedIPs: row.AttributedIPs,
		}
	}
	return out
}

func asnCountryAggregatesFromSidecar(sidecar *asnDetailSidecar) map[string]*asnDetailCountryAggregate {
	out := map[string]*asnDetailCountryAggregate{}
	if sidecar == nil {
		return out
	}
	for _, row := range sidecar.TopCountries {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" || row.AttributedIPs == 0 || row.FeedCount <= 0 {
			continue
		}
		out[code] = &asnDetailCountryAggregate{
			feedCount:     row.FeedCount,
			attributedIPs: row.AttributedIPs,
		}
	}
	return out
}

func applyCountryJointRows(totals map[uint32]*countryDetailASNAggregate, rows []feedEntityJointASN, sign int) error {
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if row.ASN == 0 || row.Count == 0 {
			continue
		}
		agg := totals[row.ASN]
		if sign < 0 {
			if agg == nil || agg.feedCount <= 0 || agg.attributedIPs < row.Count {
				return fmt.Errorf("%w: aggregate underflow for ASN %d", errEntitySurgicalNeedsFullRebuild, row.ASN)
			}
			agg.feedCount--
			agg.attributedIPs -= row.Count
			if agg.feedCount == 0 || agg.attributedIPs == 0 {
				delete(totals, row.ASN)
			}
			continue
		}
		if agg == nil {
			agg = &countryDetailASNAggregate{}
			totals[row.ASN] = agg
		}
		if agg.name == "" && row.Name != "" {
			agg.name = row.Name
		}
		agg.feedCount++
		agg.attributedIPs += row.Count
	}
	return nil
}

func applyASNCountryRows(totals map[string]*asnDetailCountryAggregate, rows []asnCountryDeltaRow, sign int) error {
	for _, row := range rows {
		code := strings.ToUpper(strings.TrimSpace(row.code))
		if code == "" || row.count == 0 {
			continue
		}
		agg := totals[code]
		if sign < 0 {
			if agg == nil || agg.feedCount <= 0 || agg.attributedIPs < row.count {
				return fmt.Errorf("%w: aggregate underflow for country %s", errEntitySurgicalNeedsFullRebuild, code)
			}
			agg.feedCount--
			agg.attributedIPs -= row.count
			if agg.feedCount == 0 || agg.attributedIPs == 0 {
				delete(totals, code)
			}
			continue
		}
		if agg == nil {
			agg = &asnDetailCountryAggregate{}
			totals[code] = agg
		}
		agg.feedCount++
		agg.attributedIPs += row.count
	}
	return nil
}

type asnCountryDeltaRow struct {
	code  string
	count uint64
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func filterPublicOutputNames(e *Engine, values []string) []string {
	if e == nil {
		return nil
	}
	allowed := stringExactSet(e.publicOutputNames())
	out := make([]string, 0, len(values))
	for _, value := range uniqueNonEmptyStrings(values) {
		if _, ok := allowed[value]; ok {
			out = append(out, value)
		}
	}
	return out
}

func stringExactSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func sourceMaintainerURL(src *config.Source) string {
	if src == nil {
		return ""
	}
	return src.MaintainerURL
}
