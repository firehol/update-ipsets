package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

func (e *Engine) rewriteSelectedEntityArtifacts(ctx context.Context, countries map[string]struct{}, asns map[uint32]struct{}, rebuildCountryIndex, rebuildASNIndex bool, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("integrity", len(countries)+len(asns)), func() (*entityArtifactMutationPlan, error) {
		return e.stageRewriteSelectedEntityArtifacts(ctx, countries, asns, rebuildCountryIndex, rebuildASNIndex, task)
	})
}

func (e *Engine) stageRewriteSelectedEntityArtifacts(ctx context.Context, countries map[string]struct{}, asns map[uint32]struct{}, rebuildCountryIndex, rebuildASNIndex bool, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	snap := e.operationSnapshot()
	if len(countries) == 0 && len(asns) == 0 && !rebuildCountryIndex && !rebuildASNIndex {
		return nil, nil
	}
	webBatch, err := newWebPublishBatchForRuntime(snap.runtime)
	if err != nil {
		return nil, err
	}
	entityBatch, err := newEntityPublishBatchForRuntime(snap.runtime)
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}

	allSidecars, err := e.loadAllFeedEntitySidecarsWithRuntime(snap.runtime)
	if err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	generated := make([]output.GeneratedFile, 0, len(countries)+len(asns)+2)
	generated, err = e.rewriteSelectedEntityDetails(ctx, snap, webBatch, entityBatch, allSidecars, countries, asns, generated, task)
	if err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	generated, err = e.rewriteSelectedEntityIndexes(ctx, snap, webBatch, allSidecars, rebuildCountryIndex, rebuildASNIndex, generated, task)
	if err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	homeAggregate, err := e.stageHomeAggregatesWithSnapshot(ctx, snap, webBatch.stageDir, "")
	if err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	generated = append(generated, homeAggregate)

	return &entityArtifactMutationPlan{
		web:            webBatch,
		entity:         entityBatch,
		generated:      generated,
		publishStage:   "publishing",
		publishDetail:  "publishing repaired entity artifacts",
		publishCurrent: 1,
		publishTotal:   1,
	}, nil
}

func (e *Engine) rewriteSelectedEntityDetails(ctx context.Context, snap operationSnapshot, webBatch *webPublishBatch, entityBatch *entityPublishBatch, allSidecars map[string]*feedEntitySidecar, countries map[string]struct{}, asns map[uint32]struct{}, generated []output.GeneratedFile, task *BackgroundTaskHandle) ([]output.GeneratedFile, error) {
	if len(countries) == 0 && len(asns) == 0 {
		return generated, nil
	}
	total := len(countries) + len(asns)
	if task != nil {
		task.Update("repairing entity details", selectedEntityRepairDetail(countries, asns), 0, total)
	}
	countrySidecars, asnSidecars, err := e.buildSelectedEntityDetailSidecarsFromFeedSidecars(allSidecars, countries, asns, false)
	if err != nil {
		return generated, err
	}
	feedTimes := e.loadFeedEntitySidecarMTimesWithRuntime(snap.runtime)
	health := e.newFeedHealthClassifierForConfigPolicy(snap.cfg, snap.feedHealthPolicy, e.state.SnapshotEntries(), e.now().UTC())
	progress := 0
	generated, progress, err = e.rewriteSelectedCountryDetails(ctx, snap, webBatch, entityBatch, countrySidecars, countries, feedTimes, health, generated, progress, total, task)
	if err != nil {
		return generated, err
	}
	return e.rewriteSelectedASNDetails(ctx, snap, webBatch, entityBatch, asnSidecars, asns, feedTimes, health, generated, progress, total, task)
}

func (e *Engine) rewriteSelectedCountryDetails(ctx context.Context, snap operationSnapshot, webBatch *webPublishBatch, entityBatch *entityPublishBatch, sidecars map[string]*countryDetailSidecar, countries map[string]struct{}, feedTimes map[string]time.Time, health *feedHealthClassifier, generated []output.GeneratedFile, progress, total int, task *BackgroundTaskHandle) ([]output.GeneratedFile, int, error) {
	for _, code := range sortedStringSet(countries) {
		if err := contextErr(ctx); err != nil {
			return generated, progress, err
		}
		sidecar := sidecars[code]
		if sidecar == nil {
			entityBatch.markDelete(e.entityCountrySidecarRelPath(code))
			webBatch.markDelete(e.publicCountryDetailRelPath(code))
		} else if err := e.stageSelectedCountryDetail(snap, webBatch, entityBatch, code, sidecar, feedTimes, health, &generated); err != nil {
			return generated, progress, err
		}
		progress++
		if task != nil {
			task.Update("repairing entity details", selectedEntityRepairDetail(countries, nil), progress, total)
		}
	}
	return generated, progress, nil
}

func (e *Engine) stageSelectedCountryDetail(snap operationSnapshot, webBatch *webPublishBatch, entityBatch *entityPublishBatch, code string, sidecar *countryDetailSidecar, feedTimes map[string]time.Time, health *feedHealthClassifier, generated *[]output.GeneratedFile) error {
	privatePath := filepath.Join(entityCountriesDirForRuntime(snap.runtime), strings.ToUpper(strings.TrimSpace(code))+".json")
	publicPath := publicCountryDetailPathForRuntime(snap.runtime, code)
	logicalTime := countryDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
	if current, err := loadCountryDetailSidecar(privatePath); err == nil && reflect.DeepEqual(current, sidecar) && entityDetailFilesExist(privatePath, publicPath) {
		e.observeRunCounter("entity.repair.country_unchanged", 1, 0)
		entityBatch.markTouch(e.entityCountrySidecarRelPath(code), logicalTime)
		webBatch.markTouch(e.publicCountryDetailRelPath(code), logicalTime)
		e.observeRunCounter("entity.repair.country_sidecar_touch", 1, 0)
		e.observeRunCounter("entity.repair.country_public_touch", 1, 0)
		return nil
	}
	if err := e.writeObservedJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityCountrySidecarRelPath(code)), sidecar, logicalTime, "entity.repair.country_sidecar_write"); err != nil {
		return err
	}
	rel := e.publicCountryDetailRelPath(code)
	countryPayload := e.materializeCountryDetailWithHealth(sidecar, health)
	if err := e.writeObservedJSONFile(filepath.Join(webBatch.stageDir, rel), countryPayload, "entity.repair.country_public_write"); err != nil {
		return err
	}
	*generated = append(*generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(snap.runtime), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageCountryMarkdownWithRuntime(snap.runtime, code, countryPayload, webBatch.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		*generated = append(*generated, mdFile)
	}
	return nil
}

func (e *Engine) rewriteSelectedASNDetails(ctx context.Context, snap operationSnapshot, webBatch *webPublishBatch, entityBatch *entityPublishBatch, sidecars map[uint32]*asnDetailSidecar, asns map[uint32]struct{}, feedTimes map[string]time.Time, health *feedHealthClassifier, generated []output.GeneratedFile, progress, total int, task *BackgroundTaskHandle) ([]output.GeneratedFile, error) {
	for _, asn := range sortedUint32Set(asns) {
		if err := contextErr(ctx); err != nil {
			return generated, err
		}
		sidecar := sidecars[asn]
		if sidecar == nil {
			entityBatch.markDelete(e.entityASNSidecarRelPath(asn))
			webBatch.markDelete(e.publicASNDetailRelPath(asn))
		} else if err := e.stageSelectedASNDetail(snap, webBatch, entityBatch, asn, sidecar, feedTimes, health, &generated); err != nil {
			return generated, err
		}
		progress++
		if task != nil {
			task.Update("repairing entity details", selectedEntityRepairDetail(nil, asns), progress, total)
		}
	}
	return generated, nil
}

func (e *Engine) stageSelectedASNDetail(snap operationSnapshot, webBatch *webPublishBatch, entityBatch *entityPublishBatch, asn uint32, sidecar *asnDetailSidecar, feedTimes map[string]time.Time, health *feedHealthClassifier, generated *[]output.GeneratedFile) error {
	privatePath := filepath.Join(entityASNsDirForRuntime(snap.runtime), strconv.FormatUint(uint64(asn), 10)+".json")
	publicPath := publicASNDetailPathForRuntime(snap.runtime, asn)
	logicalTime := asnDetailLogicalMTime(sidecar, feedTimes, e.now().UTC())
	if current, err := loadASNDetailSidecar(privatePath); err == nil && reflect.DeepEqual(current, sidecar) && entityDetailFilesExist(privatePath, publicPath) {
		e.observeRunCounter("entity.repair.asn_unchanged", 1, 0)
		entityBatch.markTouch(e.entityASNSidecarRelPath(asn), logicalTime)
		webBatch.markTouch(e.publicASNDetailRelPath(asn), logicalTime)
		e.observeRunCounter("entity.repair.asn_sidecar_touch", 1, 0)
		e.observeRunCounter("entity.repair.asn_public_touch", 1, 0)
		return nil
	}
	if err := e.writeObservedJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityASNSidecarRelPath(asn)), sidecar, logicalTime, "entity.repair.asn_sidecar_write"); err != nil {
		return err
	}
	rel := e.publicASNDetailRelPath(asn)
	asnPayload := e.materializeASNDetailWithHealth(sidecar, health)
	if err := e.writeObservedJSONFile(filepath.Join(webBatch.stageDir, rel), asnPayload, "entity.repair.asn_public_write"); err != nil {
		return err
	}
	*generated = append(*generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(snap.runtime), rel), Timestamp: logicalTime, Redistributable: true})
	if mdFile, _ := e.stageASNMarkdownWithRuntime(snap.runtime, asn, asnPayload, webBatch.stageDir); mdFile.Path != "" {
		mdFile.Timestamp = logicalTime
		*generated = append(*generated, mdFile)
	}
	return nil
}

func (e *Engine) rewriteSelectedEntityIndexes(ctx context.Context, snap operationSnapshot, webBatch *webPublishBatch, allSidecars map[string]*feedEntitySidecar, rebuildCountryIndex, rebuildASNIndex bool, generated []output.GeneratedFile, task *BackgroundTaskHandle) ([]output.GeneratedFile, error) {
	indexSteps := 0
	if rebuildCountryIndex {
		indexSteps++
	}
	if rebuildASNIndex {
		indexSteps++
	}
	if indexSteps == 0 {
		return generated, nil
	}
	if task != nil {
		task.Update("repairing entity indexes", "rewriting country and ASN index payloads", 0, indexSteps)
	}
	progress := 0
	if rebuildCountryIndex {
		if err := contextErr(ctx); err != nil {
			return generated, err
		}
		rel := e.publicCountryIndexRelPath()
		if err := writeEntityJSONFile(filepath.Join(webBatch.stageDir, rel), e.buildCountryIndexFromFeedSidecarsWithSnapshot(snap, allSidecars)); err != nil {
			return generated, err
		}
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(snap.runtime), rel), Redistributable: true})
		progress++
	}
	if rebuildASNIndex {
		if err := contextErr(ctx); err != nil {
			return generated, err
		}
		rel := e.publicASNIndexRelPath()
		if err := writeEntityJSONFile(filepath.Join(webBatch.stageDir, rel), e.buildASNIndexFromFeedSidecarsWithSnapshot(snap, allSidecars)); err != nil {
			return generated, err
		}
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(snap.runtime), rel), Redistributable: true})
		progress++
	}
	if task != nil {
		task.Update("repairing entity indexes", "rewriting country and ASN index payloads", progress, indexSteps)
	}
	return e.stagePublicSitemapFilesWithSnapshot(snap, webBatch.stagedPublishBatch, generated)
}

func (e *Engine) rewriteHomeAggregate(ctx context.Context, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("integrity", 1), func() (*entityArtifactMutationPlan, error) {
		return e.stageRewriteHomeAggregate(ctx, task)
	})
}

func (e *Engine) stageRewriteHomeAggregate(ctx context.Context, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	snap := e.operationSnapshot()
	webBatch, err := newWebPublishBatchForRuntime(snap.runtime)
	if err != nil {
		return nil, err
	}
	if task != nil {
		task.Update("repairing home aggregate", "rewriting homepage aggregate artifact", 0, 1)
	}
	homeAggregate, err := e.stageHomeAggregatesWithSnapshot(ctx, snap, webBatch.stageDir, "")
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}
	generated := []output.GeneratedFile{homeAggregate}
	return &entityArtifactMutationPlan{
		web:            webBatch,
		generated:      generated,
		publishStage:   "publishing",
		publishDetail:  "publishing homepage aggregate artifact",
		publishCurrent: 1,
		publishTotal:   1,
	}, nil
}

func selectedEntityRepairDetail(countries map[string]struct{}, asns map[uint32]struct{}) string {
	return fmt.Sprintf("rewriting %d countries and %d ASNs from current private sidecars", len(countries), len(asns))
}
