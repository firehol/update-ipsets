package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/output"
)

type entityArtifactWriteState struct {
	engine      *Engine
	ctx         context.Context
	full        bool
	webBatch    *stagedPublishBatch
	entityBatch *stagedPublishBatch
	task        *BackgroundTaskHandle

	targetFeeds []string
	view        entityOutputView

	geoProvider string
	asnProvider string
	geoRefPath  string
	asnRefPath  string
	geoRefTime  time.Time
	asnRefTime  time.Time

	liveSidecars map[string]*feedEntitySidecar
	newSidecars  map[string]*feedEntitySidecar
	allSidecars  map[string]*feedEntitySidecar
	feedTimes    map[string]time.Time
	changedFeeds map[string]struct{}

	affectedCountries map[string]struct{}
	affectedASNs      map[uint32]struct{}
	generated         []output.GeneratedFile
}

func (e *Engine) writeEntityArtifacts(ctx context.Context, updatedNames []string, full bool, webBatch *stagedPublishBatch, entityBatch *stagedPublishBatch, task *BackgroundTaskHandle) ([]output.GeneratedFile, error) {
	state, err := e.newEntityArtifactWriteState(ctx, updatedNames, full, webBatch, entityBatch, task)
	if err != nil {
		return nil, err
	}
	if err := state.loadProviderReferences(); err != nil {
		return nil, err
	}
	if err := state.loadFeedSidecars(); err != nil {
		return nil, err
	}
	if err := state.stageFeedSidecars(); err != nil {
		return nil, err
	}
	state.collectAffectedEntities()
	if state.hasNoAffectedEntities() {
		return state.stageNoAffectedArtifacts()
	}
	if err := state.markStaleDetailDeletesForFullRebuild(); err != nil {
		return nil, err
	}
	if err := state.writeSelectedEntityDetails(); err != nil {
		return nil, err
	}
	if err := state.stageIndexesAndSharedArtifacts(); err != nil {
		return nil, err
	}
	return state.generated, nil
}

func (e *Engine) newEntityArtifactWriteState(ctx context.Context, updatedNames []string, full bool, webBatch *stagedPublishBatch, entityBatch *stagedPublishBatch, task *BackgroundTaskHandle) (*entityArtifactWriteState, error) {
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
	return &entityArtifactWriteState{
		engine:            e,
		ctx:               ctx,
		full:              full,
		webBatch:          webBatch,
		entityBatch:       entityBatch,
		task:              task,
		targetFeeds:       targetFeeds,
		view:              newEntityOutputView(e, ""),
		changedFeeds:      map[string]struct{}{},
		affectedCountries: map[string]struct{}{},
		affectedASNs:      map[uint32]struct{}{},
	}, nil
}

func (s *entityArtifactWriteState) loadProviderReferences() error {
	e := s.engine
	s.geoProvider = e.preferredGeoProvider()
	s.asnProvider = e.preferredASNProvider()

	var err error
	s.geoRefPath, s.geoRefTime, err = e.entityGeoProviderReference(s.geoProvider)
	if err != nil {
		return err
	}
	s.asnRefPath, s.asnRefTime, err = e.entityASNProviderReference(s.asnProvider)
	return err
}

func (s *entityArtifactWriteState) loadFeedSidecars() error {
	e := s.engine
	var err error
	s.liveSidecars, err = e.loadAllFeedEntitySidecars()
	if err != nil {
		return err
	}
	s.feedTimes = e.loadFeedEntitySidecarMTimes()
	s.newSidecars, err = e.buildFeedEntitySidecars(s.ctx, s.targetFeeds, s.view, s.task)
	return err
}

func (s *entityArtifactWriteState) stageFeedSidecars() error {
	if s.full {
		if err := s.markStaleFeedSidecarDeletesForFullRebuild(); err != nil {
			return err
		}
	}
	for _, name := range s.targetFeeds {
		if err := s.stageFeedSidecar(name); err != nil {
			return err
		}
	}
	s.mergeAllSidecars()
	return nil
}

func (s *entityArtifactWriteState) markStaleFeedSidecarDeletesForFullRebuild() error {
	e := s.engine
	for name := range s.liveSidecars {
		if _, ok := s.newSidecars[name]; !ok {
			s.entityBatch.markDelete(e.entityFeedSidecarRelPath(name))
		}
	}
	existingPendingSidecars, err := sortedJSONFiles(e.entityFeedPendingDir())
	if err != nil {
		return err
	}
	for _, path := range existingPendingSidecars {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		s.entityBatch.markDelete(e.entityFeedPendingRelPath(name))
	}
	return nil
}

func (s *entityArtifactWriteState) stageFeedSidecar(name string) error {
	e := s.engine
	sidecar := s.newSidecars[name]
	_, sidecarRefTime, err := e.entityFeedSidecarReferenceInOutputDir(
		name,
		e.outputDir(),
		s.geoProvider,
		s.asnProvider,
		s.geoRefPath,
		s.geoRefTime,
		s.asnRefPath,
		s.asnRefTime,
	)
	if err != nil {
		return err
	}
	logicalTime := entityFeedSidecarReferenceMTime(sidecar, sidecarRefTime, e.feedProcessingTimestamp(name))
	if !s.full && reflect.DeepEqual(s.liveSidecars[name], sidecar) {
		if sidecar != nil {
			path := filepath.Join(e.entityFeedsDir(), name+".json")
			if err := e.touchObservedFileAt(path, "entity.repair.feed_sidecar_touch", logicalTime); err != nil {
				return err
			}
			s.feedTimes[name] = logicalTime
		}
		s.entityBatch.markDelete(e.entityFeedPendingRelPath(name))
		return nil
	}

	s.changedFeeds[name] = struct{}{}
	if sidecar == nil {
		s.entityBatch.markDelete(e.entityFeedSidecarRelPath(name))
		delete(s.feedTimes, name)
		return nil
	}
	if err := writeJSONFileAt(filepath.Join(s.entityBatch.stageDir, e.entityFeedSidecarRelPath(name)), sidecar, logicalTime); err != nil {
		return err
	}
	s.feedTimes[name] = logicalTime
	return nil
}

func (s *entityArtifactWriteState) mergeAllSidecars() {
	s.allSidecars = s.liveSidecars
	if s.full {
		s.allSidecars = make(map[string]*feedEntitySidecar, len(s.newSidecars))
	}
	for _, name := range s.targetFeeds {
		if !s.full {
			if _, ok := s.changedFeeds[name]; !ok {
				continue
			}
		}
		if sidecar := s.newSidecars[name]; sidecar == nil {
			delete(s.allSidecars, name)
			continue
		}
		s.allSidecars[name] = s.newSidecars[name]
	}
}

func (s *entityArtifactWriteState) collectAffectedEntities() {
	if s.full {
		for _, sidecar := range s.allSidecars {
			s.addAffectedSidecarEntities(sidecar)
		}
		return
	}
	for _, name := range s.targetFeeds {
		if _, ok := s.changedFeeds[name]; !ok {
			continue
		}
		if oldSidecar, ok := s.liveSidecars[name]; ok {
			s.addAffectedSidecarEntities(oldSidecar)
		}
		if newSidecar, ok := s.newSidecars[name]; ok {
			s.addAffectedSidecarEntities(newSidecar)
		}
	}
}

func (s *entityArtifactWriteState) addAffectedSidecarEntities(sidecar *feedEntitySidecar) {
	if sidecar == nil {
		return
	}
	for _, code := range sidecar.countryCodes() {
		s.affectedCountries[code] = struct{}{}
	}
	for _, asn := range sidecar.asnNumbers() {
		s.affectedASNs[asn] = struct{}{}
	}
}

func (s *entityArtifactWriteState) hasNoAffectedEntities() bool {
	return !s.full && len(s.affectedCountries) == 0 && len(s.affectedASNs) == 0
}

func (s *entityArtifactWriteState) stageNoAffectedArtifacts() ([]output.GeneratedFile, error) {
	e := s.engine
	if err := writeFileAtomic(filepath.Join(s.entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		return nil, err
	}
	homeAggregate, err := e.stageHomeAggregates(s.ctx, s.webBatch.stageDir, "")
	if err != nil {
		return nil, err
	}
	return []output.GeneratedFile{homeAggregate}, nil
}

func (s *entityArtifactWriteState) markStaleDetailDeletesForFullRebuild() error {
	e := s.engine
	existingCountryFiles, err := sortedJSONFiles(e.entityCountriesDir())
	if err != nil {
		return err
	}
	existingASNFiles, err := sortedJSONFiles(e.entityASNsDir())
	if err != nil {
		return err
	}
	if !s.full {
		return nil
	}
	s.markStaleCountryDeletesForFullRebuild(existingCountryFiles)
	s.markStaleASNDeletesForFullRebuild(existingASNFiles)
	return nil
}

func (s *entityArtifactWriteState) markStaleCountryDeletesForFullRebuild(existingCountryFiles []string) {
	e := s.engine
	for _, path := range existingCountryFiles {
		code := strings.TrimSuffix(filepath.Base(path), ".json")
		if _, ok := s.affectedCountries[strings.ToUpper(code)]; !ok {
			s.entityBatch.markDelete(e.entityCountrySidecarRelPath(code))
		}
	}
}

func (s *entityArtifactWriteState) markStaleASNDeletesForFullRebuild(existingASNFiles []string) {
	e := s.engine
	for _, path := range existingASNFiles {
		raw := strings.TrimSuffix(filepath.Base(path), ".json")
		asn, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			continue
		}
		if _, ok := s.affectedASNs[uint32(asn)]; !ok {
			s.entityBatch.markDelete(e.entityASNSidecarRelPath(uint32(asn)))
		}
	}
}

func (s *entityArtifactWriteState) writeSelectedEntityDetails() error {
	e := s.engine
	if s.task != nil {
		s.task.Update("aggregating entity details", s.entityDetailProgressDetail(), 0, s.entityDetailProgressTotal())
	}
	countrySidecars, asnSidecars, err := e.buildSelectedEntityDetailSidecarsFromFeedSidecars(s.allSidecars, s.affectedCountries, s.affectedASNs, s.full)
	if err != nil {
		return err
	}

	s.generated = make([]output.GeneratedFile, 0, len(s.affectedCountries)+len(s.affectedASNs)+2)
	health := e.newFeedHealthClassifier()
	progress := 0
	total := s.entityDetailProgressTotal()
	for _, code := range sortedStringSet(s.affectedCountries) {
		if err := s.writeCountryDetail(code, countrySidecars[code], health, &progress, total); err != nil {
			return err
		}
	}
	for _, asn := range sortedUint32Set(s.affectedASNs) {
		if err := s.writeASNDetail(asn, asnSidecars[asn], health, &progress, total); err != nil {
			return err
		}
	}
	return nil
}

func (s *entityArtifactWriteState) writeCountryDetail(code string, sidecar *countryDetailSidecar, health *feedHealthClassifier, progress *int, total int) error {
	e := s.engine
	if sidecar == nil {
		s.entityBatch.markDelete(e.entityCountrySidecarRelPath(code))
		s.webBatch.markDelete(e.publicCountryDetailRelPath(code))
		return nil
	}
	logicalTime := countryDetailLogicalMTime(sidecar, s.feedTimes, e.now().UTC())
	if err := writeJSONFileAt(filepath.Join(s.entityBatch.stageDir, e.entityCountrySidecarRelPath(code)), sidecar, logicalTime); err != nil {
		return err
	}
	rel := e.publicCountryDetailRelPath(code)
	countryPayload := e.materializeCountryDetailWithHealth(sidecar, health)
	if err := writeJSONFile(filepath.Join(s.webBatch.stageDir, rel), countryPayload); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageCountryMarkdown(code, countryPayload, s.webBatch.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		s.generated = append(s.generated, mdFile)
	}
	*progress = *progress + 1
	s.updateEntityDetailProgress(*progress, total)
	return nil
}

func (s *entityArtifactWriteState) writeASNDetail(asn uint32, sidecar *asnDetailSidecar, health *feedHealthClassifier, progress *int, total int) error {
	e := s.engine
	if sidecar == nil {
		s.entityBatch.markDelete(e.entityASNSidecarRelPath(asn))
		s.webBatch.markDelete(e.publicASNDetailRelPath(asn))
		return nil
	}
	logicalTime := asnDetailLogicalMTime(sidecar, s.feedTimes, e.now().UTC())
	if err := writeJSONFileAt(filepath.Join(s.entityBatch.stageDir, e.entityASNSidecarRelPath(asn)), sidecar, logicalTime); err != nil {
		return err
	}
	rel := e.publicASNDetailRelPath(asn)
	asnPayload := e.materializeASNDetailWithHealth(sidecar, health)
	if err := writeJSONFile(filepath.Join(s.webBatch.stageDir, rel), asnPayload); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageASNMarkdown(asn, asnPayload, s.webBatch.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		s.generated = append(s.generated, mdFile)
	}
	*progress = *progress + 1
	s.updateEntityDetailProgress(*progress, total)
	return nil
}

func (s *entityArtifactWriteState) entityDetailProgressTotal() int {
	return len(s.affectedCountries) + len(s.affectedASNs)
}

func (s *entityArtifactWriteState) entityDetailProgressDetail() string {
	return fmt.Sprintf("building %d country pages and %d ASN pages", len(s.affectedCountries), len(s.affectedASNs))
}

func (s *entityArtifactWriteState) updateEntityDetailProgress(progress, total int) {
	if s.task != nil {
		s.task.Update("aggregating entity details", s.entityDetailProgressDetail(), progress, total)
	}
}

func (s *entityArtifactWriteState) stageIndexesAndSharedArtifacts() error {
	if err := s.stageEntityIndexes(); err != nil {
		return err
	}
	s.stageMaintainerMarkdownForFullRebuild()
	if err := s.stageSitemapHomeAndVersion(); err != nil {
		return err
	}
	return nil
}

func (s *entityArtifactWriteState) stageEntityIndexes() error {
	e := s.engine
	if s.task != nil {
		s.task.Update("building indexes", "building country and ASN index payloads", 0, 2)
	}
	countryIndex := e.buildCountryIndexFromFeedSidecars(s.allSidecars)
	if err := writeJSONFile(filepath.Join(s.webBatch.stageDir, e.publicCountryIndexRelPath()), countryIndex); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), e.publicCountryIndexRelPath()), Redistributable: true})
	if s.task != nil {
		s.task.Update("building indexes", "building country and ASN index payloads", 1, 2)
	}

	asnIndex := e.buildASNIndexFromFeedSidecars(s.allSidecars)
	if err := writeJSONFile(filepath.Join(s.webBatch.stageDir, e.publicASNIndexRelPath()), asnIndex); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(e.outputDir(), e.publicASNIndexRelPath()), Redistributable: true})
	if s.task != nil {
		s.task.Update("building indexes", "building country and ASN index payloads", 2, 2)
	}
	return nil
}

func (s *entityArtifactWriteState) stageMaintainerMarkdownForFullRebuild() {
	if !s.full {
		return
	}
	mdFiles, mdErr := s.engine.writeMaintainerMarkdownFiles(s.webBatch.stageDir)
	if mdErr != nil {
		s.engine.logger.Warn("maintainer markdown generation failed", "error", mdErr)
		return
	}
	s.generated = append(s.generated, mdFiles...)
}

func (s *entityArtifactWriteState) stageSitemapHomeAndVersion() error {
	e := s.engine
	var err error
	s.generated, err = e.stagePublicSitemapFiles(s.webBatch, s.generated)
	if err != nil {
		return err
	}
	homeAggregate, err := e.stageHomeAggregates(s.ctx, s.webBatch.stageDir, "")
	if err != nil {
		return err
	}
	s.generated = append(s.generated, homeAggregate)

	return writeFileAtomic(filepath.Join(s.entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode)
}
