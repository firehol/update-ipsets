package engine

import (
	"context"
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

const entityArtifactsVersion = "3"

type entityPublishBatch struct {
	*stagedPublishBatch
}

func (e *Engine) newEntityPublishBatch() (*entityPublishBatch, error) {
	batch, err := newStagedPublishBatch(e.entitiesDir(), "", ".update-ipsets-entities-*")
	if err != nil {
		return nil, err
	}
	return &entityPublishBatch{stagedPublishBatch: batch}, nil
}

func (e *Engine) entitiesDir() string {
	return filepath.Join(e.runtime.LibDir, "entities")
}

func (e *Engine) entityFeedsDir() string {
	return filepath.Join(e.entitiesDir(), "feeds")
}

func (e *Engine) entityFeedPendingDir() string {
	return filepath.Join(e.entitiesDir(), "feeds-pending")
}

func (e *Engine) entityCountriesDir() string {
	return filepath.Join(e.entitiesDir(), "countries")
}

func (e *Engine) entityASNsDir() string {
	return filepath.Join(e.entitiesDir(), "asns")
}

func (e *Engine) entityVersionPath() string {
	return filepath.Join(e.entitiesDir(), "version")
}

func (e *Engine) publicCountryIndexRelPath() string {
	return filepath.Join("countries", "index.json")
}

func (e *Engine) publicCountryDetailRelPath(code string) string {
	return filepath.Join("countries", strings.ToUpper(strings.TrimSpace(code))+".json")
}

func (e *Engine) publicASNIndexRelPath() string {
	return filepath.Join("asns", "index.json")
}

func (e *Engine) publicASNDetailRelPath(asn uint32) string {
	return filepath.Join("asns", strconv.FormatUint(uint64(asn), 10)+".json")
}

func (e *Engine) PublicCountryIndexPath() string {
	return filepath.Join(e.outputDir(), e.publicCountryIndexRelPath())
}

func (e *Engine) PublicCountryDetailPath(code string) string {
	return filepath.Join(e.outputDir(), e.publicCountryDetailRelPath(code))
}

func (e *Engine) PublicASNIndexPath() string {
	return filepath.Join(e.outputDir(), e.publicASNIndexRelPath())
}

func (e *Engine) PublicASNDetailPath(asn uint32) string {
	return filepath.Join(e.outputDir(), e.publicASNDetailRelPath(asn))
}

func (e *Engine) entityFeedSidecarRelPath(name string) string {
	return filepath.Join("feeds", name+".json")
}

func (e *Engine) entityFeedPendingRelPath(name string) string {
	return filepath.Join("feeds-pending", name+".json")
}

func (e *Engine) entityCountrySidecarRelPath(code string) string {
	return filepath.Join("countries", strings.ToUpper(strings.TrimSpace(code))+".json")
}

func (e *Engine) entityASNSidecarRelPath(asn uint32) string {
	return filepath.Join("asns", strconv.FormatUint(uint64(asn), 10)+".json")
}

func (e *Engine) entityArtifactsNeedBootstrapFast() bool {
	version, err := readFileInRoot(e.entitiesDir(), "version")
	if err != nil {
		return true
	}
	if strings.TrimSpace(string(version)) != entityArtifactsVersion {
		return true
	}
	required := []string{
		filepath.Join(e.outputDir(), e.publicCountryIndexRelPath()),
		filepath.Join(e.outputDir(), e.publicASNIndexRelPath()),
		e.PublicHomeAggregatesPath(),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			return true
		}
	}
	return false
}

func (e *Engine) RebuildEntityArtifacts() error {
	return e.RebuildEntityArtifactsWithTrigger(context.Background(), "background")
}

func (e *Engine) RebuildEntityArtifactsWithTrigger(ctx context.Context, trigger string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	return e.withBackgroundTask(
		"Entity artifacts rebuild",
		trigger,
		"planning",
		backgroundEntityTaskDetail("full", 0),
		0,
		0,
		func(task *BackgroundTaskHandle) error {
			return e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("full", 0), func() error {
				return e.rebuildEntityArtifactsFromLive(ctx, task)
			})
		},
	)
}

func (e *Engine) RefreshEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if len(feedNames) == 0 {
		return nil
	}
	if e.entityArtifactsNeedBootstrapFast() {
		return e.RebuildEntityArtifactsWithTrigger(ctx, "health_transition")
	}
	return e.withBackgroundTask(
		"Entity artifacts refresh",
		"health_transition",
		"scanning memberships",
		backgroundEntityTaskDetail("health", len(feedNames)),
		0,
		len(feedNames),
		func(task *BackgroundTaskHandle) error {
			return e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("health", len(feedNames)), func() error {
				return e.refreshEntityArtifactsForHealthTransitions(ctx, feedNames, task)
			})
		},
	)
}

func (e *Engine) refreshEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	affectedCountries := map[string]struct{}{}
	affectedASNs := map[uint32]struct{}{}
	for i, name := range feedNames {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if task != nil {
			task.Update("scanning memberships", backgroundEntityTaskDetail("health", len(feedNames)), i+1, len(feedNames))
		}
		sidecar, err := e.loadCommittedFeedEntitySidecar(name)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		for _, code := range sidecar.countryCodes() {
			affectedCountries[code] = struct{}{}
		}
		for _, asn := range sidecar.asnNumbers() {
			affectedASNs[asn] = struct{}{}
		}
	}
	if len(affectedCountries) == 0 && len(affectedASNs) == 0 {
		webBatch, err := e.newWebPublishBatch()
		if err != nil {
			return err
		}
		defer webBatch.cleanup()
		homeAggregate, err := e.stageHomeAggregates(ctx, webBatch.stageDir, "")
		if err != nil {
			return err
		}
		generated := []output.GeneratedFile{homeAggregate}
		if err := webBatch.applyGeneratedFileTimestamps(generated); err != nil {
			return err
		}
		published, err := webBatch.publish()
		if err != nil {
			return err
		}
		return e.syncGeneratedFiles(generated, published)
	}
	if task != nil {
		task.Update(
			"rebuilding final payloads",
			fmt.Sprintf("rewriting %d countries and %d ASNs after health transitions", len(affectedCountries), len(affectedASNs)),
			0,
			len(affectedCountries)+len(affectedASNs),
		)
	}

	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		return err
	}
	defer webBatch.cleanup()

	generated := make([]output.GeneratedFile, 0, len(affectedCountries)+len(affectedASNs))
	health := e.newFeedHealthClassifier()
	progress := 0
	total := len(affectedCountries) + len(affectedASNs)
	for _, code := range sortedStringSet(affectedCountries) {
		if err := contextErr(ctx); err != nil {
			return err
		}
		sidecarPath := filepath.Join(e.entityCountriesDir(), code+".json")
		sidecar, err := loadCountryDetailSidecar(sidecarPath)
		if err != nil {
			return err
		}
		sidecarInfo, err := os.Stat(sidecarPath)
		if err != nil {
			return err
		}
		rel := e.publicCountryDetailRelPath(code)
		payload := e.materializeCountryDetailWithHealth(sidecar, health)
		if err := writeJSONFile(filepath.Join(webBatch.stageDir, rel), payload); err != nil {
			return err
		}
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: sidecarInfo.ModTime().UTC(), Redistributable: true})
		if mdFile, _ := e.stageCountryMarkdown(code, payload, webBatch.stageDir); mdFile.Path != "" {
			mdFile.Timestamp = sidecarInfo.ModTime().UTC()
			generated = append(generated, mdFile)
		}
		progress++
		if task != nil {
			task.Update("rebuilding final payloads", fmt.Sprintf("rewriting %d countries and %d ASNs after health transitions", len(affectedCountries), len(affectedASNs)), progress, total)
		}
	}
	for _, asn := range sortedUint32Set(affectedASNs) {
		if err := contextErr(ctx); err != nil {
			return err
		}
		sidecarPath := filepath.Join(e.entityASNsDir(), strconv.FormatUint(uint64(asn), 10)+".json")
		sidecar, err := loadASNDetailSidecar(sidecarPath)
		if err != nil {
			return err
		}
		sidecarInfo, err := os.Stat(sidecarPath)
		if err != nil {
			return err
		}
		rel := e.publicASNDetailRelPath(asn)
		payload := e.materializeASNDetailWithHealth(sidecar, health)
		if err := writeJSONFile(filepath.Join(webBatch.stageDir, rel), payload); err != nil {
			return err
		}
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: sidecarInfo.ModTime().UTC(), Redistributable: true})
		if mdFile, _ := e.stageASNMarkdown(asn, payload, webBatch.stageDir); mdFile.Path != "" {
			mdFile.Timestamp = sidecarInfo.ModTime().UTC()
			generated = append(generated, mdFile)
		}
		progress++
		if task != nil {
			task.Update("rebuilding final payloads", fmt.Sprintf("rewriting %d countries and %d ASNs after health transitions", len(affectedCountries), len(affectedASNs)), progress, total)
		}
	}
	if task != nil {
		task.Update("publishing", "publishing refreshed entity artifacts", total, total)
	}
	homeAggregate, err := e.stageHomeAggregates(ctx, webBatch.stageDir, "")
	if err != nil {
		return err
	}
	generated = append(generated, homeAggregate)
	if err := webBatch.applyGeneratedFileTimestamps(generated); err != nil {
		return err
	}
	published, err := webBatch.publish()
	if err != nil {
		return err
	}
	return e.syncGeneratedFiles(generated, published)
}

func (e *Engine) rebuildEntityArtifactsFromLive(ctx context.Context, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
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
	generated, err := e.writeEntityArtifacts(ctx, nil, true, webBatch.stagedPublishBatch, entityBatch.stagedPublishBatch, task)
	if err != nil {
		return err
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if task != nil {
		task.Update("publishing", "publishing rebuilt entity artifacts", 1, 1)
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

func (e *Engine) rebuildEntityArtifactsForFeeds(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
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

	generated, err := e.writeEntityArtifacts(ctx, feedNames, false, webBatch.stagedPublishBatch, entityBatch.stagedPublishBatch, task)
	if err != nil {
		return err
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if task != nil {
		task.Update("publishing", "publishing repaired entity artifacts", 1, 1)
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

func (e *Engine) writeEntityArtifacts(ctx context.Context, updatedNames []string, full bool, webBatch *stagedPublishBatch, entityBatch *stagedPublishBatch, task *BackgroundTaskHandle) ([]output.GeneratedFile, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	if webBatch == nil {
		return nil, fmt.Errorf("web publish batch is required")
	}
	if entityBatch == nil {
		return nil, fmt.Errorf("entity publish batch is required")
	}
	targetFeeds := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseGeoIP, config.UseASN, config.UseBogons)
	if full {
		targetFeeds = e.publicOutputNames()
	}
	view := newEntityOutputView(e, "")
	geoProvider := e.preferredGeoProvider()
	asnProvider := e.preferredASNProvider()
	geoRefPath, geoRefTime, err := e.entityGeoProviderReference(geoProvider)
	if err != nil {
		return nil, err
	}
	asnRefPath, asnRefTime, err := e.entityASNProviderReference(asnProvider)
	if err != nil {
		return nil, err
	}
	liveSidecars, err := e.loadAllFeedEntitySidecars()
	if err != nil {
		return nil, err
	}
	feedTimes := e.loadFeedEntitySidecarMTimes()
	newSidecars, err := e.buildFeedEntitySidecars(ctx, targetFeeds, view, task)
	if err != nil {
		return nil, err
	}

	if full {
		for name := range liveSidecars {
			if _, ok := newSidecars[name]; !ok {
				entityBatch.markDelete(e.entityFeedSidecarRelPath(name))
			}
		}
		existingPendingSidecars, err := sortedJSONFiles(e.entityFeedPendingDir())
		if err != nil {
			return nil, err
		}
		for _, path := range existingPendingSidecars {
			name := strings.TrimSuffix(filepath.Base(path), ".json")
			entityBatch.markDelete(e.entityFeedPendingRelPath(name))
		}
	}
	changedFeeds := map[string]struct{}{}
	for _, name := range targetFeeds {
		sidecar := newSidecars[name]
		_, sidecarRefTime, err := e.entityFeedSidecarReferenceInOutputDir(name, e.outputDir(), geoProvider, asnProvider, geoRefPath, geoRefTime, asnRefPath, asnRefTime)
		if err != nil {
			return nil, err
		}
		logicalTime := entityFeedSidecarReferenceMTime(sidecar, sidecarRefTime, e.feedProcessingTimestamp(name))
		if !full && reflect.DeepEqual(liveSidecars[name], sidecar) {
			if sidecar != nil {
				path := filepath.Join(e.entityFeedsDir(), name+".json")
				if err := e.touchObservedFileAt(path, "entity.repair.feed_sidecar_touch", logicalTime); err != nil {
					return nil, err
				}
				feedTimes[name] = logicalTime
			}
			entityBatch.markDelete(e.entityFeedPendingRelPath(name))
			continue
		}
		changedFeeds[name] = struct{}{}
		if sidecar == nil {
			entityBatch.markDelete(e.entityFeedSidecarRelPath(name))
			delete(feedTimes, name)
			continue
		}
		if err := writeJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityFeedSidecarRelPath(name)), sidecar, logicalTime); err != nil {
			return nil, err
		}
		feedTimes[name] = logicalTime
	}

	allSidecars := liveSidecars
	if full {
		allSidecars = make(map[string]*feedEntitySidecar, len(newSidecars))
	}
	for _, name := range targetFeeds {
		if !full {
			if _, ok := changedFeeds[name]; !ok {
				continue
			}
		}
		if sidecar := newSidecars[name]; sidecar == nil {
			delete(allSidecars, name)
			continue
		}
		allSidecars[name] = newSidecars[name]
	}

	affectedCountries := map[string]struct{}{}
	affectedASNs := map[uint32]struct{}{}
	if full {
		for _, sidecar := range allSidecars {
			for _, code := range sidecar.countryCodes() {
				affectedCountries[code] = struct{}{}
			}
			for _, asn := range sidecar.asnNumbers() {
				affectedASNs[asn] = struct{}{}
			}
		}
	} else {
		for _, name := range targetFeeds {
			if _, ok := changedFeeds[name]; !ok {
				continue
			}
			if oldSidecar, ok := liveSidecars[name]; ok {
				for _, code := range oldSidecar.countryCodes() {
					affectedCountries[code] = struct{}{}
				}
				for _, asn := range oldSidecar.asnNumbers() {
					affectedASNs[asn] = struct{}{}
				}
			}
			if newSidecar, ok := newSidecars[name]; ok {
				for _, code := range newSidecar.countryCodes() {
					affectedCountries[code] = struct{}{}
				}
				for _, asn := range newSidecar.asnNumbers() {
					affectedASNs[asn] = struct{}{}
				}
			}
		}
	}

	if !full && len(affectedCountries) == 0 && len(affectedASNs) == 0 {
		if err := writeFileAtomic(filepath.Join(entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
			return nil, err
		}
		homeAggregate, err := e.stageHomeAggregates(ctx, webBatch.stageDir, "")
		if err != nil {
			return nil, err
		}
		return []output.GeneratedFile{homeAggregate}, nil
	}

	existingCountryFiles, err := sortedJSONFiles(e.entityCountriesDir())
	if err != nil {
		return nil, err
	}
	existingASNFiles, err := sortedJSONFiles(e.entityASNsDir())
	if err != nil {
		return nil, err
	}
	if full {
		for _, path := range existingCountryFiles {
			code := strings.TrimSuffix(filepath.Base(path), ".json")
			if _, ok := affectedCountries[strings.ToUpper(code)]; !ok {
				entityBatch.markDelete(e.entityCountrySidecarRelPath(code))
			}
		}
		for _, path := range existingASNFiles {
			raw := strings.TrimSuffix(filepath.Base(path), ".json")
			asn, err := strconv.ParseUint(raw, 10, 32)
			if err != nil {
				continue
			}
			if _, ok := affectedASNs[uint32(asn)]; !ok {
				entityBatch.markDelete(e.entityASNSidecarRelPath(uint32(asn)))
			}
		}
	}

	if task != nil {
		task.Update(
			"aggregating entity details",
			fmt.Sprintf("building %d country pages and %d ASN pages", len(affectedCountries), len(affectedASNs)),
			0,
			len(affectedCountries)+len(affectedASNs),
		)
	}
	countrySidecars, asnSidecars, err := e.buildSelectedEntityDetailSidecarsFromFeedSidecars(allSidecars, affectedCountries, affectedASNs, full)
	if err != nil {
		return nil, err
	}

	generated := make([]output.GeneratedFile, 0, len(affectedCountries)+len(affectedASNs)+2)
	health := e.newFeedHealthClassifier()
	progress := 0
	total := len(affectedCountries) + len(affectedASNs)
	for _, code := range sortedStringSet(affectedCountries) {
		sidecar := countrySidecars[code]
		if sidecar == nil {
			entityBatch.markDelete(e.entityCountrySidecarRelPath(code))
			webBatch.markDelete(e.publicCountryDetailRelPath(code))
			continue
		}
		logicalTime := countryDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
		if err := writeJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityCountrySidecarRelPath(code)), sidecar, logicalTime); err != nil {
			return nil, err
		}
		rel := e.publicCountryDetailRelPath(code)
		countryPayload := e.materializeCountryDetailWithHealth(sidecar, health)
		if err := writeJSONFile(filepath.Join(webBatch.stageDir, rel), countryPayload); err != nil {
			return nil, err
		}
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
		if mdFile, _ := e.stageCountryMarkdown(code, countryPayload, webBatch.stageDir); mdFile.Path != "" {
			mdFile.Timestamp = logicalTime
			generated = append(generated, mdFile)
		}
		progress++
		if task != nil {
			task.Update("aggregating entity details", fmt.Sprintf("building %d country pages and %d ASN pages", len(affectedCountries), len(affectedASNs)), progress, total)
		}
	}
	for _, asn := range sortedUint32Set(affectedASNs) {
		sidecar := asnSidecars[asn]
		if sidecar == nil {
			entityBatch.markDelete(e.entityASNSidecarRelPath(asn))
			webBatch.markDelete(e.publicASNDetailRelPath(asn))
			continue
		}
		logicalTime := asnDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
		if err := writeJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityASNSidecarRelPath(asn)), sidecar, logicalTime); err != nil {
			return nil, err
		}
		rel := e.publicASNDetailRelPath(asn)
		asnPayload := e.materializeASNDetailWithHealth(sidecar, health)
		if err := writeJSONFile(filepath.Join(webBatch.stageDir, rel), asnPayload); err != nil {
			return nil, err
		}
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
		if mdFile, _ := e.stageASNMarkdown(asn, asnPayload, webBatch.stageDir); mdFile.Path != "" {
			mdFile.Timestamp = logicalTime
			generated = append(generated, mdFile)
		}
		progress++
		if task != nil {
			task.Update("aggregating entity details", fmt.Sprintf("building %d country pages and %d ASN pages", len(affectedCountries), len(affectedASNs)), progress, total)
		}
	}

	if task != nil {
		task.Update("building indexes", "building country and ASN index payloads", 0, 2)
	}
	countryIndex := e.buildCountryIndexFromFeedSidecars(allSidecars)
	if err := writeJSONFile(filepath.Join(webBatch.stageDir, e.publicCountryIndexRelPath()), countryIndex); err != nil {
		return nil, err
	}
	generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), e.publicCountryIndexRelPath()), Redistributable: true})
	if task != nil {
		task.Update("building indexes", "building country and ASN index payloads", 1, 2)
	}

	asnIndex := e.buildASNIndexFromFeedSidecars(allSidecars)
	if err := writeJSONFile(filepath.Join(webBatch.stageDir, e.publicASNIndexRelPath()), asnIndex); err != nil {
		return nil, err
	}
	generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), e.publicASNIndexRelPath()), Redistributable: true})
	if task != nil {
		task.Update("building indexes", "building country and ASN index payloads", 2, 2)
	}

	if full {
		mdFiles, mdErr := e.writeMaintainerMarkdownFiles(webBatch.stageDir)
		if mdErr != nil {
			e.logger.Warn("maintainer markdown generation failed", "error", mdErr)
		} else {
			generated = append(generated, mdFiles...)
		}
	}

	generated, err = e.stagePublicSitemapFiles(webBatch, generated)
	if err != nil {
		return nil, err
	}
	homeAggregate, err := e.stageHomeAggregates(ctx, webBatch.stageDir, "")
	if err != nil {
		return nil, err
	}
	generated = append(generated, homeAggregate)

	if err := writeFileAtomic(filepath.Join(entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		return nil, err
	}
	return generated, nil
}

func sortedStringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func (e *Engine) loadFeedEntitySidecarMTimes() map[string]time.Time {
	out := map[string]time.Time{}
	paths, err := sortedJSONFiles(e.entityFeedsDir())
	if err != nil {
		return out
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		out[name] = info.ModTime().UTC()
	}
	return out
}

func feedEntitySidecarLogicalMTime(sidecar *feedEntitySidecar, fallback time.Time) time.Time {
	var latest int64
	if sidecar != nil && sidecar.LastChangeTS > latest {
		latest = sidecar.LastChangeTS
	}
	if latest > 0 {
		return time.Unix(latest, 0).UTC()
	}
	if fallback.IsZero() {
		return time.Time{}
	}
	return fallback.UTC()
}

func entityFeedSidecarReferenceMTime(sidecar *feedEntitySidecar, refTime, fallback time.Time) time.Time {
	logical := feedEntitySidecarLogicalMTime(sidecar, fallback)
	if refTime.After(logical) {
		return refTime.UTC()
	}
	return logical
}

func countryDetailLogicalMTime(sidecar *countryDetailSidecar, feedTimes map[string]time.Time, fallback time.Time) time.Time {
	latest := fallback.UTC()
	if sidecar != nil {
		for _, row := range sidecar.Feeds {
			if when := feedTimes[row.Name]; when.After(latest) {
				latest = when.UTC()
			}
			if row.LastChangeTS > 0 {
				when := time.Unix(row.LastChangeTS, 0).UTC()
				if when.After(latest) {
					latest = when
				}
			}
		}
	}
	return latest
}

func asnDetailLogicalMTime(sidecar *asnDetailSidecar, feedTimes map[string]time.Time, fallback time.Time) time.Time {
	latest := fallback.UTC()
	if sidecar != nil {
		for _, row := range sidecar.Feeds {
			if when := feedTimes[row.Name]; when.After(latest) {
				latest = when.UTC()
			}
			if row.LastChangeTS > 0 {
				when := time.Unix(row.LastChangeTS, 0).UTC()
				if when.After(latest) {
					latest = when
				}
			}
		}
	}
	return latest
}

func (e *Engine) stagePublicSitemapFiles(webBatch *stagedPublishBatch, generated []output.GeneratedFile) ([]output.GeneratedFile, error) {
	if webBatch == nil {
		return generated, nil
	}
	siteBase := e.publicSiteBaseURL()
	files, err := e.writeSitemapFiles(webBatch.stageDir, siteBase, e.publicFeedURLPrefix(siteBase), e.publicOutputNames())
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), name), Redistributable: true})
	}
	stale, err := staleSitemapShardNames(e.outputDir(), files)
	if err != nil {
		return nil, err
	}
	for _, name := range stale {
		webBatch.markDelete(name)
	}
	return generated, nil
}

func sortedUint32Set(values map[uint32]struct{}) []uint32 {
	out := make([]uint32, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
