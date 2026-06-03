package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

type entitySurgicalRefreshState struct {
	e     *Engine
	ctx   context.Context
	task  *BackgroundTaskHandle
	web   *webPublishBatch
	ent   *entityPublishBatch
	total int

	targetFeeds []string
	deltas      []feedEntityDelta

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
	ctx = nonNilContext(ctx)
	if len(feedNames) == 0 {
		return nil
	}
	state, err := e.newEntitySurgicalRefreshState(ctx, feedNames, task)
	if err != nil {
		return err
	}
	defer state.cleanup()
	if len(state.targetFeeds) == 0 {
		return nil
	}
	if err := state.loadFeedDeltas(); err != nil {
		return err
	}
	if len(state.deltas) == 0 {
		return state.publishNoDeltaVersion()
	}
	state.startDetailPatch()
	if err := state.patchCountryDetails(); err != nil {
		return err
	}
	if err := state.patchASNDetails(); err != nil {
		return err
	}
	if err := state.patchEntityIndexes(); err != nil {
		return err
	}
	return state.publishPatchedArtifacts()
}

func (e *Engine) newEntitySurgicalRefreshState(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) (*entitySurgicalRefreshState, error) {
	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		return nil, err
	}
	entityBatch, err := e.newEntityPublishBatch()
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}
	return &entitySurgicalRefreshState{
		e:                 e,
		ctx:               ctx,
		task:              task,
		web:               webBatch,
		ent:               entityBatch,
		targetFeeds:       filterPublicOutputNames(e, feedNames),
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
		delta, err := s.e.buildFeedEntityDelta(name)
		if err != nil {
			if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
				s.e.observeRunCounter("entity.refresh.full_rebuild_fallback", 1, 0)
				return s.e.rebuildEntityArtifactsFromLive(s.ctx, s.task)
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

func (s *entitySurgicalRefreshState) publishNoDeltaVersion() error {
	if err := contextErr(s.ctx); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(s.ent.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		return err
	}
	_, err := s.ent.publish()
	return err
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
	s.feedTimes = s.e.loadFeedEntitySidecarMTimes()
	for _, delta := range s.deltas {
		if delta.new == nil {
			delete(s.feedTimes, delta.name)
			continue
		}
		s.feedTimes[delta.name] = delta.newMTime
	}
	s.health = s.e.newFeedHealthClassifier()
}

func (s *entitySurgicalRefreshState) detailPatchDetail() string {
	return fmt.Sprintf("patching affected entity artifacts: %d countries and %d ASNs", len(s.affectedCountries), len(s.affectedASNs))
}

func (s *entitySurgicalRefreshState) patchCountryDetails() error {
	for _, code := range sortedStringSet(s.affectedCountries) {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.patchCountryDetail(code); err != nil {
			return err
		}
	}
	return nil
}

func (s *entitySurgicalRefreshState) patchCountryDetail(code string) error {
	sidecar, changed, err := s.e.patchCountrySidecarForFeedDeltas(code, s.deltas)
	if err != nil {
		if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
			return s.e.rebuildEntityArtifactsFromLive(s.ctx, s.task)
		}
		return err
	}
	if !changed {
		if err := s.touchUnchangedCountryDetail(code, sidecar); err != nil {
			return err
		}
		s.e.observeRunCounter("entity.refresh.country_unchanged", 1, 0)
		s.advanceDetailProgress()
		return nil
	}
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
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(s.e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := s.e.stageCountryMarkdown(code, payload, s.web.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		s.generated = append(s.generated, mdFile)
	}
	s.advanceDetailProgress()
	return nil
}

func (s *entitySurgicalRefreshState) touchUnchangedCountryDetail(code string, sidecar *countryDetailSidecar) error {
	if sidecar == nil {
		return nil
	}
	logicalTime := countryDetailLogicalMTime(sidecar, s.feedTimes, s.e.now().UTC())
	privatePath := filepath.Join(s.e.entityCountriesDir(), strings.ToUpper(strings.TrimSpace(code))+".json")
	publicPath := s.e.PublicCountryDetailPath(code)
	if !entityDetailFilesExist(privatePath, publicPath) {
		return nil
	}
	if err := s.e.touchObservedFileAt(privatePath, "entity.refresh.country_sidecar_touch", logicalTime); err != nil {
		return err
	}
	return s.e.touchObservedFileAt(publicPath, "entity.refresh.country_public_touch", logicalTime)
}

func (s *entitySurgicalRefreshState) patchASNDetails() error {
	for _, asn := range sortedUint32Set(s.affectedASNs) {
		if err := contextErr(s.ctx); err != nil {
			return err
		}
		if err := s.patchASNDetail(asn); err != nil {
			return err
		}
	}
	return nil
}

func (s *entitySurgicalRefreshState) patchASNDetail(asn uint32) error {
	sidecar, changed, err := s.e.patchASNSidecarForFeedDeltas(asn, s.deltas)
	if err != nil {
		if errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
			return s.e.rebuildEntityArtifactsFromLive(s.ctx, s.task)
		}
		return err
	}
	if !changed {
		if err := s.touchUnchangedASNDetail(asn, sidecar); err != nil {
			return err
		}
		s.e.observeRunCounter("entity.refresh.asn_unchanged", 1, 0)
		s.advanceDetailProgress()
		return nil
	}
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
	s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(s.e.outputDir(), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := s.e.stageASNMarkdown(asn, payload, s.web.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		s.generated = append(s.generated, mdFile)
	}
	s.advanceDetailProgress()
	return nil
}

func (s *entitySurgicalRefreshState) touchUnchangedASNDetail(asn uint32, sidecar *asnDetailSidecar) error {
	if sidecar == nil {
		return nil
	}
	logicalTime := asnDetailLogicalMTime(sidecar, s.feedTimes, s.e.now().UTC())
	privatePath := filepath.Join(s.e.entityASNsDir(), strconv.FormatUint(uint64(asn), 10)+".json")
	publicPath := s.e.PublicASNDetailPath(asn)
	if !entityDetailFilesExist(privatePath, publicPath) {
		return nil
	}
	if err := s.e.touchObservedFileAt(privatePath, "entity.refresh.asn_sidecar_touch", logicalTime); err != nil {
		return err
	}
	return s.e.touchObservedFileAt(publicPath, "entity.refresh.asn_public_touch", logicalTime)
}

func (s *entitySurgicalRefreshState) advanceDetailProgress() {
	s.progress++
	if s.task != nil {
		s.task.Update("patching entity details", s.detailPatchDetail(), s.progress, s.total)
	}
}

func (s *entitySurgicalRefreshState) patchEntityIndexes() error {
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
		if err := s.e.patchCountryIndex(s.web, s.countryUpdates); err != nil {
			return err
		}
		s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(s.e.outputDir(), s.e.publicCountryIndexRelPath()), Redistributable: true})
		indexProgress++
		if s.task != nil {
			s.task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", indexProgress, indexSteps)
		}
	}
	if len(s.asnUpdates) > 0 {
		if err := s.e.patchASNIndex(s.web, s.asnUpdates); err != nil {
			return err
		}
		s.generated = append(s.generated, output.GeneratedFile{Path: filepath.Join(s.e.outputDir(), s.e.publicASNIndexRelPath()), Redistributable: true})
		indexProgress++
		if s.task != nil {
			s.task.Update("patching entity indexes", "updating country and ASN indexes from patched entities", indexProgress, indexSteps)
		}
	}
	return nil
}

func (s *entitySurgicalRefreshState) publishPatchedArtifacts() error {
	if err := writeFileAtomic(filepath.Join(s.ent.stageDir, "version"), []byte(entityArtifactsVersion+"\n"), generatedFileMode); err != nil {
		return err
	}
	if err := contextErr(s.ctx); err != nil {
		return err
	}
	if s.task != nil {
		s.task.Update("publishing", "publishing patched entity artifacts", s.total, s.total)
	}
	if _, err := s.ent.publish(); err != nil {
		return err
	}
	if err := s.web.applyGeneratedFileTimestamps(s.generated); err != nil {
		return err
	}
	published, err := s.web.publish()
	if err != nil {
		return err
	}
	return s.e.syncGeneratedFiles(s.generated, published)
}
