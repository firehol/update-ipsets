package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	snapshot    operationSnapshot
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

	liveSidecars     map[string]*feedEntitySidecar
	liveSidecarNames map[string]struct{}
	newSidecars      map[string]*feedEntitySidecar
	newSidecarNames  map[string]struct{}
	feedTimes        map[string]time.Time
	changedFeeds     map[string]struct{}

	affectedCountries map[string]struct{}
	affectedASNs      map[uint32]struct{}
	generated         []output.GeneratedFile
}

func (e *Engine) writeEntityArtifacts(ctx context.Context, updatedNames []string, full bool, webBatch *stagedPublishBatch, entityBatch *stagedPublishBatch, task *BackgroundTaskHandle) ([]output.GeneratedFile, error) {
	return e.writeEntityArtifactsWithSnapshot(ctx, e.operationSnapshot(), updatedNames, full, webBatch, entityBatch, task)
}

func (e *Engine) writeEntityArtifactsWithSnapshot(ctx context.Context, snap operationSnapshot, updatedNames []string, full bool, webBatch *stagedPublishBatch, entityBatch *stagedPublishBatch, task *BackgroundTaskHandle) ([]output.GeneratedFile, error) {
	state, err := e.newEntityArtifactWriteStateWithSnapshot(ctx, snap, updatedNames, full, webBatch, entityBatch, task)
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
	if err := state.collectAffectedEntities(); err != nil {
		return nil, err
	}
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
	return e.newEntityArtifactWriteStateWithSnapshot(ctx, e.operationSnapshot(), updatedNames, full, webBatch, entityBatch, task)
}

func (e *Engine) newEntityArtifactWriteStateWithSnapshot(ctx context.Context, snap operationSnapshot, updatedNames []string, full bool, webBatch *stagedPublishBatch, entityBatch *stagedPublishBatch, task *BackgroundTaskHandle) (*entityArtifactWriteState, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || snap.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	if webBatch == nil {
		return nil, fmt.Errorf("web publish batch is required")
	}
	if entityBatch == nil {
		return nil, fmt.Errorf("entity publish batch is required")
	}

	targetFeeds := targetFeedsForFanOut(snap.cfg, updatedNames, e.publicOutputNamesForSnapshot(snap), config.UseGeoIP, config.UseASN, config.UseBogons)
	if full {
		targetFeeds = e.publicOutputNamesForSnapshot(snap)
	}
	return &entityArtifactWriteState{
		engine:            e,
		snapshot:          snap,
		ctx:               ctx,
		full:              full,
		webBatch:          webBatch,
		entityBatch:       entityBatch,
		task:              task,
		targetFeeds:       targetFeeds,
		view:              newEntityOutputViewWithRuntime(e, snap.runtime, ""),
		changedFeeds:      map[string]struct{}{},
		affectedCountries: map[string]struct{}{},
		affectedASNs:      map[uint32]struct{}{},
	}, nil
}

func (s *entityArtifactWriteState) loadProviderReferences() error {
	e := s.engine
	s.geoProvider = preferredGeoProviderForConfig(s.snapshot.cfg)
	s.asnProvider = preferredASNProviderForConfig(s.snapshot.cfg)

	var err error
	s.geoRefPath, s.geoRefTime, err = e.entityGeoProviderReferenceWithSnapshot(s.snapshot, s.geoProvider)
	if err != nil {
		return err
	}
	s.asnRefPath, s.asnRefTime, err = e.entityASNProviderReferenceWithSnapshot(s.snapshot, s.asnProvider)
	return err
}

func (s *entityArtifactWriteState) loadFeedSidecars() error {
	e := s.engine
	var err error
	if s.full {
		s.liveSidecarNames, err = committedFeedEntitySidecarNamesWithRuntime(s.snapshot.runtime)
		if err != nil {
			return err
		}
	} else {
		s.liveSidecars = make(map[string]*feedEntitySidecar, len(s.targetFeeds))
		for _, name := range s.targetFeeds {
			sidecar, loadErr := e.loadCommittedFeedEntitySidecarWithRuntime(s.snapshot.runtime, name)
			if loadErr != nil {
				if errors.Is(loadErr, os.ErrNotExist) {
					continue
				}
				return loadErr
			}
			s.liveSidecars[name] = sidecar
		}
	}
	s.feedTimes = e.loadFeedEntitySidecarMTimesWithRuntime(s.snapshot.runtime)
	if s.full {
		return s.buildAndStageFullFeedSidecars()
	}
	s.newSidecars, err = e.buildFeedEntitySidecarsWithSnapshot(s.ctx, s.snapshot, s.targetFeeds, s.view, s.task)
	return err
}

func (s *entityArtifactWriteState) buildAndStageFullFeedSidecars() error {
	s.newSidecarNames = make(map[string]struct{}, len(s.targetFeeds))
	return s.engine.visitBuiltFeedEntitySidecarsWithSnapshot(s.ctx, s.snapshot, s.targetFeeds, s.view, s.task, func(name string, sidecar *feedEntitySidecar) error {
		if sidecar == nil {
			return nil
		}
		s.newSidecarNames[name] = struct{}{}
		if err := s.stageFeedSidecar(name, sidecar); err != nil {
			return err
		}
		s.addAffectedSidecarEntities(sidecar)
		return nil
	})
}

func (s *entityArtifactWriteState) stageFeedSidecars() error {
	if s.full {
		return s.markStaleFeedSidecarDeletesForFullRebuild()
	}
	for _, name := range s.targetFeeds {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.stageFeedSidecar(name, s.newSidecars[name]); err != nil {
			return err
		}
	}
	return nil
}

func (s *entityArtifactWriteState) markStaleFeedSidecarDeletesForFullRebuild() error {
	e := s.engine
	for name := range s.liveSidecarNames {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if _, ok := s.newSidecarNames[name]; !ok {
			s.entityBatch.markDelete(e.entityFeedSidecarRelPath(name))
		}
	}
	existingPendingSidecars, err := sortedJSONFiles(entityFeedPendingDirForRuntime(s.snapshot.runtime))
	if err != nil {
		return err
	}
	for _, path := range existingPendingSidecars {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		s.entityBatch.markDelete(e.entityFeedPendingRelPath(name))
	}
	return nil
}

func (s *entityArtifactWriteState) stageFeedSidecar(name string, sidecar *feedEntitySidecar) error {
	e := s.engine
	_, sidecarRefTime, err := e.entityFeedSidecarReferenceInOutputDirWithSnapshot(
		s.snapshot,
		name,
		outputDirForRuntime(s.snapshot.runtime),
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
			s.entityBatch.markTouch(e.entityFeedSidecarRelPath(name), logicalTime)
			e.observeRunCounter("entity.repair.feed_sidecar_touch", 1, 0)
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
	if err := writeEntityJSONFileAt(filepath.Join(s.entityBatch.stageDir, e.entityFeedSidecarRelPath(name)), sidecar, logicalTime); err != nil {
		return err
	}
	s.feedTimes[name] = logicalTime
	return nil
}

func (s *entityArtifactWriteState) currentSidecarReplacements() map[string]*feedEntitySidecar {
	if s.full {
		return s.newSidecars
	}
	replacements := make(map[string]*feedEntitySidecar, len(s.changedFeeds))
	for _, name := range s.targetFeeds {
		if _, ok := s.changedFeeds[name]; !ok {
			continue
		}
		replacements[name] = s.newSidecars[name]
	}
	return replacements
}

func (s *entityArtifactWriteState) walkCurrentFeedSidecars(visit feedEntitySidecarVisitFunc) error {
	if s.full {
		return s.engine.walkFeedEntitySidecarsInDir(s.ctx, filepath.Join(s.entityBatch.stageDir, "feeds"), visit)
	}
	return s.engine.walkMergedFeedEntitySidecarsWithRuntime(s.ctx, s.snapshot.runtime, s.currentSidecarReplacements(), s.full, visit)
}

func (s *entityArtifactWriteState) collectAffectedEntities() error {
	if s.full {
		return nil
	}
	for _, name := range s.targetFeeds {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
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
	return nil
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
	if err := stageEntityFeedPresenceIndexFromWalker(s.ctx, s.entityBatch, s.walkCurrentFeedSidecars); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(s.entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		return nil, err
	}
	homeAggregate, err := e.stageHomeAggregatesWithSnapshot(s.ctx, s.snapshot, s.webBatch.stageDir, "")
	if err != nil {
		return nil, err
	}
	return []output.GeneratedFile{homeAggregate}, nil
}

func (s *entityArtifactWriteState) markStaleDetailDeletesForFullRebuild() error {
	existingCountryFiles, err := sortedJSONFiles(entityCountriesDirForRuntime(s.snapshot.runtime))
	if err != nil {
		return err
	}
	existingASNFiles, err := sortedJSONFiles(entityASNsDirForRuntime(s.snapshot.runtime))
	if err != nil {
		return err
	}
	if !s.full {
		return nil
	}
	if err := s.markStaleCountryDeletesForFullRebuild(existingCountryFiles); err != nil {
		return err
	}
	return s.markStaleASNDeletesForFullRebuild(existingASNFiles)
}

func (s *entityArtifactWriteState) markStaleCountryDeletesForFullRebuild(existingCountryFiles []string) error {
	e := s.engine
	for _, path := range existingCountryFiles {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		code := strings.TrimSuffix(filepath.Base(path), ".json")
		if _, ok := s.affectedCountries[strings.ToUpper(code)]; !ok {
			s.entityBatch.markDelete(e.entityCountrySidecarRelPath(code))
		}
	}
	return nil
}

func (s *entityArtifactWriteState) markStaleASNDeletesForFullRebuild(existingASNFiles []string) error {
	e := s.engine
	for _, path := range existingASNFiles {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		raw := strings.TrimSuffix(filepath.Base(path), ".json")
		asn, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			continue
		}
		if _, ok := s.affectedASNs[uint32(asn)]; !ok {
			s.entityBatch.markDelete(e.entityASNSidecarRelPath(uint32(asn)))
		}
	}
	return nil
}

func (s *entityArtifactWriteState) writeSelectedEntityDetails() error {
	e := s.engine
	if s.task != nil {
		s.task.Update("aggregating entity details", s.entityDetailProgressDetail(), 0, s.entityDetailProgressTotal())
	}
	countrySidecars, asnSidecars, err := e.buildSelectedEntityDetailSidecarsFromFeedSidecarWalker(s.ctx, s.snapshot, s.affectedCountries, s.affectedASNs, s.full, s.walkCurrentFeedSidecars)
	if err != nil {
		return err
	}

	s.generated = make([]output.GeneratedFile, 0, len(s.affectedCountries)+len(s.affectedASNs)+2)
	health := e.newFeedHealthClassifierForConfigPolicy(s.snapshot.cfg, s.snapshot.feedHealthPolicy, e.state.SnapshotEntries(), e.now().UTC())
	progress := 0
	total := s.entityDetailProgressTotal()
	for _, code := range sortedStringSet(s.affectedCountries) {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.writeCountryDetail(code, countrySidecars[code], health, &progress, total); err != nil {
			return err
		}
	}
	for _, asn := range sortedUint32Set(s.affectedASNs) {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
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
	if err := writeEntityJSONFileAt(filepath.Join(s.entityBatch.stageDir, e.entityCountrySidecarRelPath(code)), sidecar, logicalTime); err != nil {
		return err
	}
	rel := e.publicCountryDetailRelPath(code)
	countryPayload := e.materializeCountryDetailWithHealth(sidecar, health)
	if err := writeEntityJSONFile(filepath.Join(s.webBatch.stageDir, rel), countryPayload); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snapshot.runtime), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageCountryMarkdownWithRuntime(s.snapshot.runtime, code, countryPayload, s.webBatch.stageDir); mdFile.Path != "" {
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
	if err := writeEntityJSONFileAt(filepath.Join(s.entityBatch.stageDir, e.entityASNSidecarRelPath(asn)), sidecar, logicalTime); err != nil {
		return err
	}
	rel := e.publicASNDetailRelPath(asn)
	asnPayload := e.materializeASNDetailWithHealth(sidecar, health)
	if err := writeEntityJSONFile(filepath.Join(s.webBatch.stageDir, rel), asnPayload); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snapshot.runtime), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageASNMarkdownWithRuntime(s.snapshot.runtime, asn, asnPayload, s.webBatch.stageDir); mdFile.Path != "" {
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
	countryIndex, asnIndex, err := e.buildEntityIndexesFromFeedSidecarWalkerWithSnapshot(s.ctx, s.snapshot, s.walkCurrentFeedSidecars, true, true)
	if err != nil {
		return err
	}
	if err := writeEntityJSONFile(filepath.Join(s.webBatch.stageDir, e.publicCountryIndexRelPath()), countryIndex); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snapshot.runtime), e.publicCountryIndexRelPath()), Redistributable: true})
	if s.task != nil {
		s.task.Update("building indexes", "building country and ASN index payloads", 1, 2)
	}

	if err := writeEntityJSONFile(filepath.Join(s.webBatch.stageDir, e.publicASNIndexRelPath()), asnIndex); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snapshot.runtime), e.publicASNIndexRelPath()), Redistributable: true})
	if s.task != nil {
		s.task.Update("building indexes", "building country and ASN index payloads", 2, 2)
	}
	return nil
}

func (s *entityArtifactWriteState) stageMaintainerMarkdownForFullRebuild() {
	if !s.full {
		return
	}
	mdFiles, mdErr := s.engine.writeMaintainerMarkdownFilesWithSnapshot(s.snapshot, s.webBatch.stageDir)
	if mdErr != nil {
		s.engine.logger.Warn("maintainer markdown generation failed", "error", mdErr)
		return
	}
	s.generated = append(s.generated, mdFiles...)
}

func (s *entityArtifactWriteState) stageSitemapHomeAndVersion() error {
	e := s.engine
	var err error
	s.generated, err = e.stagePublicSitemapFilesWithSnapshot(s.snapshot, s.webBatch, s.generated)
	if err != nil {
		return err
	}
	homeAggregate, err := e.stageHomeAggregatesWithSnapshot(s.ctx, s.snapshot, s.webBatch.stageDir, "")
	if err != nil {
		return err
	}
	s.generated = append(s.generated, homeAggregate)
	if err := stageEntityFeedPresenceIndexFromWalker(s.ctx, s.entityBatch, s.walkCurrentFeedSidecars); err != nil {
		return err
	}

	return writeFileAtomic(filepath.Join(s.entityBatch.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode)
}
