package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

type entitySurgicalRefreshState struct {
	e     *Engine
	snap  operationSnapshot
	ctx   context.Context
	task  *BackgroundTaskHandle
	web   *webPublishBatch
	ent   *entityPublishBatch
	total int

	targetFeeds []string
	deltas      []feedEntityDelta
	presence    *entityArtifactFeedPresence
	allSidecars map[string]*feedEntitySidecar

	affectedCountries map[string]struct{}
	affectedASNs      map[uint32]struct{}
	countryUpdates    map[string]*countryDetailSidecar
	asnUpdates        map[uint32]*asnDetailSidecar
	feedTimes         map[string]time.Time
	health            *feedHealthClassifier
	generated         []output.GeneratedFile
	progress          int
}

func (e *Engine) refreshEntityArtifactsForFeedUpdates(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) error {
	return e.refreshEntityArtifactsForFeedUpdatesWithSnapshot(ctx, e.operationSnapshot(), feedNames, task)
}

func (e *Engine) refreshEntityArtifactsForFeedUpdatesWithSnapshot(ctx context.Context, snap operationSnapshot, feedNames []string, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("feeds", len(feedNames)), func() (*entityArtifactMutationPlan, error) {
		plan, err := e.stageRefreshEntityArtifactsForFeedUpdatesWithSnapshot(ctx, snap, feedNames, task)
		if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
			return e.stageRebuildEntityArtifactsFromLiveWithSnapshot(ctx, snap, task)
		}
		return plan, err
	})
}

func (e *Engine) stageRefreshEntityArtifactsForFeedUpdates(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	return e.stageRefreshEntityArtifactsForFeedUpdatesWithSnapshot(ctx, e.operationSnapshot(), feedNames, task)
}

func (e *Engine) stageRefreshEntityArtifactsForFeedUpdatesWithSnapshot(ctx context.Context, snap operationSnapshot, feedNames []string, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	if len(feedNames) == 0 {
		return nil, nil
	}
	state, err := e.newEntitySurgicalRefreshStateWithSnapshot(ctx, snap, feedNames, task)
	if err != nil {
		return nil, err
	}
	if len(state.targetFeeds) == 0 {
		state.cleanup()
		return nil, nil
	}
	if err := state.loadFeedDeltas(); err != nil {
		state.cleanup()
		return nil, err
	}
	if len(state.deltas) == 0 {
		return state.stageNoDeltaVersion()
	}
	state.startDetailPatch()
	if err := state.rebuildAffectedDetailsFromFeedSidecars(); err != nil {
		state.cleanup()
		return nil, err
	}
	if err := state.patchEntityIndexes(); err != nil {
		state.cleanup()
		return nil, err
	}
	return state.patchedArtifactsPlan()
}

func (e *Engine) newEntitySurgicalRefreshState(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) (*entitySurgicalRefreshState, error) {
	return e.newEntitySurgicalRefreshStateWithSnapshot(ctx, e.operationSnapshot(), feedNames, task)
}

func (e *Engine) newEntitySurgicalRefreshStateWithSnapshot(ctx context.Context, snap operationSnapshot, feedNames []string, task *BackgroundTaskHandle) (*entitySurgicalRefreshState, error) {
	webBatch, err := newWebPublishBatchForRuntime(snap.runtime)
	if err != nil {
		return nil, err
	}
	entityBatch, err := newEntityPublishBatchForRuntime(snap.runtime)
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}
	return &entitySurgicalRefreshState{
		e:                 e,
		snap:              snap,
		ctx:               ctx,
		task:              task,
		web:               webBatch,
		ent:               entityBatch,
		targetFeeds:       filterPublicOutputNamesForSnapshot(e, snap, feedNames),
		presence:          newEntityArtifactFeedPresenceWithRuntime(e, snap.runtime),
		affectedCountries: map[string]struct{}{},
		affectedASNs:      map[uint32]struct{}{},
	}, nil
}

func (s *entitySurgicalRefreshState) cleanup() {
	s.web.cleanup()
	s.ent.cleanup()
}

func (s *entitySurgicalRefreshState) loadFeedDeltas() error {
	s.e.observeRunCounter("entity.refresh.target_feeds", int64(len(s.targetFeeds)), 0)
	if s.task != nil {
		s.task.Update("loading feed deltas", s.feedDeltaDetail(), 0, len(s.targetFeeds))
	}
	s.deltas = make([]feedEntityDelta, 0, len(s.targetFeeds))
	for i, name := range s.targetFeeds {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		delta, err := s.e.buildFeedEntityDeltaWithSnapshot(name, s.snap, s.presence)
		if err != nil {
			if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
				s.e.observeRunCounter("entity.refresh.full_rebuild_fallback", 1, 0)
				return errEntitySurgicalNeedsFullRebuild
			}
			return err
		}
		if delta.old == nil && delta.new == nil {
			continue
		}
		if err := s.stageFeedDelta(name, delta); err != nil {
			return err
		}
		if s.task != nil {
			s.task.Update("loading feed deltas", s.feedDeltaDetail(), i+1, len(s.targetFeeds))
		}
	}
	return nil
}

func (s *entitySurgicalRefreshState) feedDeltaDetail() string {
	return fmt.Sprintf("loading committed and pending feed entity sidecars for %d feeds", len(s.targetFeeds))
}

func (s *entitySurgicalRefreshState) stageFeedDelta(name string, delta feedEntityDelta) error {
	s.deltas = append(s.deltas, delta)
	addChangedActorTargets(s.affectedCountries, s.affectedASNs, delta)
	if delta.new == nil {
		s.ent.markDelete(s.e.entityFeedSidecarRelPath(name))
	} else if err := s.e.writeObservedJSONFileAt(filepath.Join(s.ent.stageDir, s.e.entityFeedSidecarRelPath(name)), delta.new, delta.newMTime, "entity.refresh.feed_sidecar_write"); err != nil {
		return err
	}
	s.ent.markDelete(s.e.entityFeedPendingRelPath(name))
	return nil
}

func (s *entitySurgicalRefreshState) stageNoDeltaVersion() (*entityArtifactMutationPlan, error) {
	if err := contextErr(s.ctx); err != nil {
		s.cleanup()
		return nil, err
	}
	if err := s.stageFeedPresenceIndex(); err != nil {
		s.cleanup()
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(s.ent.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		s.cleanup()
		return nil, err
	}
	return &entityArtifactMutationPlan{
		web:            s.web,
		entity:         s.ent,
		publishStage:   "publishing",
		publishDetail:  "publishing refreshed entity artifacts",
		publishCurrent: 1,
		publishTotal:   1,
	}, nil
}

func (s *entitySurgicalRefreshState) startDetailPatch() {
	s.total = len(s.affectedCountries) + len(s.affectedASNs)
	if s.task != nil {
		s.task.Update("patching entity details", s.detailPatchDetail(), 0, s.total)
	}
	s.e.observeRunCounter("entity.refresh.affected_countries", int64(len(s.affectedCountries)), 0)
	s.e.observeRunCounter("entity.refresh.affected_asns", int64(len(s.affectedASNs)), 0)

	s.generated = make([]output.GeneratedFile, 0, s.total+2)
	s.countryUpdates = map[string]*countryDetailSidecar{}
	s.asnUpdates = map[uint32]*asnDetailSidecar{}
	s.feedTimes = s.e.loadFeedEntitySidecarMTimesWithRuntime(s.snap.runtime)
	for _, delta := range s.deltas {
		if delta.new == nil {
			delete(s.feedTimes, delta.name)
			continue
		}
		s.feedTimes[delta.name] = delta.newMTime
	}
	s.health = s.e.newFeedHealthClassifierForConfigPolicy(s.snap.cfg, s.snap.feedHealthPolicy, s.e.state.SnapshotEntries(), s.e.now().UTC())
}

func (s *entitySurgicalRefreshState) detailPatchDetail() string {
	return fmt.Sprintf("patching affected entity artifacts: %d countries and %d ASNs", len(s.affectedCountries), len(s.affectedASNs))
}

func (s *entitySurgicalRefreshState) loadMergedFeedSidecars() (map[string]*feedEntitySidecar, error) {
	if s.allSidecars != nil {
		return s.allSidecars, nil
	}
	sidecars, err := s.e.loadAllFeedEntitySidecarsWithRuntime(s.snap.runtime)
	if err != nil {
		return nil, err
	}
	for _, delta := range s.deltas {
		if delta.new == nil {
			delete(sidecars, delta.name)
			continue
		}
		sidecars[delta.name] = delta.new
	}
	s.allSidecars = sidecars
	return sidecars, nil
}

func (s *entitySurgicalRefreshState) rebuildAffectedDetailsFromFeedSidecars() error {
	sidecars, err := s.loadMergedFeedSidecars()
	if err != nil {
		return err
	}
	countrySidecars, asnSidecars, err := s.e.buildSelectedEntityDetailSidecarsFromFeedSidecars(sidecars, s.affectedCountries, s.affectedASNs, false)
	if err != nil {
		return err
	}
	for _, code := range sortedStringSet(s.affectedCountries) {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.stageRebuiltCountryDetail(code, countrySidecars[code]); err != nil {
			return err
		}
	}
	for _, asn := range sortedUint32Set(s.affectedASNs) {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.stageRebuiltASNDetail(asn, asnSidecars[asn]); err != nil {
			return err
		}
	}
	return nil
}

func (s *entitySurgicalRefreshState) stageRebuiltCountryDetail(code string, sidecar *countryDetailSidecar) error {
	s.countryUpdates[code] = sidecar
	if sidecar == nil {
		s.ent.markDelete(s.e.entityCountrySidecarRelPath(code))
		s.web.markDelete(s.e.publicCountryDetailRelPath(code))
		s.advanceDetailProgress()
		return nil
	}
	logicalTime := countryDetailLogicalMTime(sidecar, s.feedTimes, s.e.now().UTC())
	if err := s.e.writeObservedJSONFileAt(filepath.Join(s.ent.stageDir, s.e.entityCountrySidecarRelPath(code)), sidecar, logicalTime, "entity.refresh.country_sidecar_write"); err != nil {
		return err
	}
	rel := s.e.publicCountryDetailRelPath(code)
	materializeStart := time.Now()
	payload := s.e.materializeCountryDetailWithHealth(sidecar, s.health)
	s.e.observeRunOperation("entity.refresh.country_materialize", time.Since(materializeStart))
	if err := s.e.writeObservedJSONFileAt(filepath.Join(s.web.stageDir, rel), payload, logicalTime, "entity.refresh.country_public_write"); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snap.runtime), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := s.e.stageCountryMarkdownWithRuntime(s.snap.runtime, code, payload, s.web.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		s.generated = append(s.generated, mdFile)
	}
	s.advanceDetailProgress()
	return nil
}

func (s *entitySurgicalRefreshState) stageRebuiltASNDetail(asn uint32, sidecar *asnDetailSidecar) error {
	s.asnUpdates[asn] = sidecar
	if sidecar == nil {
		s.ent.markDelete(s.e.entityASNSidecarRelPath(asn))
		s.web.markDelete(s.e.publicASNDetailRelPath(asn))
		s.advanceDetailProgress()
		return nil
	}
	logicalTime := asnDetailLogicalMTime(sidecar, s.feedTimes, s.e.now().UTC())
	if err := s.e.writeObservedJSONFileAt(filepath.Join(s.ent.stageDir, s.e.entityASNSidecarRelPath(asn)), sidecar, logicalTime, "entity.refresh.asn_sidecar_write"); err != nil {
		return err
	}
	rel := s.e.publicASNDetailRelPath(asn)
	materializeStart := time.Now()
	payload := s.e.materializeASNDetailWithHealth(sidecar, s.health)
	s.e.observeRunOperation("entity.refresh.asn_materialize", time.Since(materializeStart))
	if err := s.e.writeObservedJSONFileAt(filepath.Join(s.web.stageDir, rel), payload, logicalTime, "entity.refresh.asn_public_write"); err != nil {
		return err
	}
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snap.runtime), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := s.e.stageASNMarkdownWithRuntime(s.snap.runtime, asn, payload, s.web.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		s.generated = append(s.generated, mdFile)
	}
	s.advanceDetailProgress()
	return nil
}

func (s *entitySurgicalRefreshState) advanceDetailProgress() {
	s.progress++
	if s.task != nil {
		s.task.Update("patching entity details", s.detailPatchDetail(), s.progress, s.total)
	}
}

func (s *entitySurgicalRefreshState) patchEntityIndexes() error {
	if err := contextErr(s.ctx); err != nil {
		return err
	}
	indexSteps := 0
	if len(s.countryUpdates) > 0 {
		indexSteps++
	}
	if len(s.asnUpdates) > 0 {
		indexSteps++
	}
	if indexSteps == 0 {
		return nil
	}
	indexProgress := 0
	if s.task != nil {
		s.task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", 0, indexSteps)
	}
	if len(s.countryUpdates) > 0 {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.e.patchCountryIndexWithSnapshot(s.snap, s.web, s.countryUpdates); err != nil {
			return err
		}
		s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snap.runtime), s.e.publicCountryIndexRelPath()), Redistributable: true})
		indexProgress++
		if s.task != nil {
			s.task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", indexProgress, indexSteps)
		}
	}
	if len(s.asnUpdates) > 0 {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.e.patchASNIndexWithSnapshot(s.snap, s.web, s.asnUpdates); err != nil {
			return err
		}
		s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(s.snap.runtime), s.e.publicASNIndexRelPath()), Redistributable: true})
		indexProgress++
		if s.task != nil {
			s.task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", indexProgress, indexSteps)
		}
	}
	return nil
}

func (s *entitySurgicalRefreshState) patchedArtifactsPlan() (*entityArtifactMutationPlan, error) {
	if err := s.stageFeedPresenceIndex(); err != nil {
		s.cleanup()
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(s.ent.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		s.cleanup()
		return nil, err
	}
	if err := contextErr(s.ctx); err != nil {
		s.cleanup()
		return nil, err
	}
	return &entityArtifactMutationPlan{
		web:            s.web,
		entity:         s.ent,
		generated:      s.generated,
		publishStage:   "publishing",
		publishDetail:  "publishing patched entity artifacts",
		publishCurrent: s.total,
		publishTotal:   s.total,
	}, nil
}

func (s *entitySurgicalRefreshState) stageFeedPresenceIndex() error {
	if err := contextErr(s.ctx); err != nil {
		return err
	}
	sidecars, err := s.loadMergedFeedSidecars()
	if err != nil {
		return err
	}
	if err := contextErr(s.ctx); err != nil {
		return err
	}
	return stageEntityFeedPresenceIndex(s.ent.stagedPublishBatch, entityFeedPresenceNamesFromSidecars(sidecars))
}
