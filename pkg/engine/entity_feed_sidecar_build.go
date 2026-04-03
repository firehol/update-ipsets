package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/config"
)

type feedEntitySidecarBuildResult struct {
	name    string
	sidecar *feedEntitySidecar
	err     error
}

func (e *Engine) buildFeedEntitySidecars(ctx context.Context, names []string, view entityOutputView, task *BackgroundTaskHandle) (map[string]*feedEntitySidecar, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	geoProvider := e.preferredGeoProvider()
	asnProvider := e.preferredASNProvider()
	geoPrepared := e.loadGeoProviderForLookup(geoProvider)
	asnDB := e.loadASNProviderForLookup(asnProvider)

	out := make(map[string]*feedEntitySidecar, len(names))
	if task != nil {
		task.Update("building feed sidecars", fmt.Sprintf("computing entity sidecars for %d feeds", len(names)), 0, len(names))
	}

	workers := e.runtime.BackgroundWorkers()
	if workers < 1 {
		workers = 1
	}
	buildCtx, cancel, results := e.startFeedEntitySidecarBuild(ctx, names, workers, view, geoProvider, asnProvider, geoPrepared, asnDB, func() (*latestSetCache, func()) {
		setCache := newLatestSetCache(e)
		return setCache, func() { setCache.CloseAll(e.logger) }
	})
	defer cancel()

	progress := 0
	var errs joinedErrorCollector
	for result := range results {
		if result.err != nil {
			if errs.add(result.err) {
				cancel()
			}
			continue
		}
		if result.sidecar != nil {
			out[result.name] = result.sidecar
		}
		progress++
		if task != nil {
			task.Update("building feed sidecars", fmt.Sprintf("computing entity sidecars for %d feeds", len(names)), progress, len(names))
		}
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	if err := contextErr(buildCtx); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *Engine) stageFeedEntitySidecarsFromLoadedProviders(ctx context.Context, geoProviders geoPreparedProviders, asnDBs asnDatasets, updatedNames []string, webStageDir string, entityBatch *stagedPublishBatch, setCache *latestSetCache) ([]string, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || e.cfg == nil || entityBatch == nil {
		return nil, nil
	}
	targetFeeds := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseGeoIP, config.UseASN, config.UseBogons)
	if len(targetFeeds) == 0 {
		return nil, nil
	}

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
	geoPrepared := geoProviders[geoProvider]
	asnDB := asnDBs[asnProvider]
	view := newEntityOutputView(e, webStageDir)
	if setCache == nil {
		setCache = newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
	}

	workers := e.runtime.HeavyPhaseWorkers()
	if workers < 1 {
		workers = 1
	}
	buildCtx, cancel, results := e.startFeedEntitySidecarBuild(ctx, targetFeeds, workers, view, geoProvider, asnProvider, geoPrepared, asnDB, func() (*latestSetCache, func()) {
		return setCache, nil
	})
	defer cancel()

	refreshTargets := make([]string, 0, len(targetFeeds))
	var errs joinedErrorCollector
	for result := range results {
		if result.err != nil {
			if errs.add(result.err) {
				cancel()
			}
			continue
		}
		if err := contextErr(buildCtx); err != nil {
			if !errs.hasErrors() {
				errs.add(err)
			}
			continue
		}
		changed, err := e.stageFeedEntitySidecarResult(result, geoProvider, asnProvider, geoRefPath, geoRefTime, asnRefPath, asnRefTime, webStageDir, entityBatch)
		if err != nil {
			wrapped := fmt.Errorf("stage feed entity sidecar %s: %w", result.name, err)
			if errs.add(wrapped) {
				cancel()
			}
			continue
		}
		if changed {
			refreshTargets = append(refreshTargets, result.name)
		}
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	if err := contextErr(buildCtx); err != nil {
		return nil, err
	}
	slices.Sort(refreshTargets)
	return refreshTargets, nil
}

func (e *Engine) startFeedEntitySidecarBuild(ctx context.Context, names []string, workers int, view entityOutputView, geoProvider, asnProvider string, geoPrepared *geoPreparedProvider, asnDB *asnloc.Database, setCacheForWorker func() (*latestSetCache, func())) (context.Context, context.CancelFunc, <-chan feedEntitySidecarBuildResult) {
	entries := e.state.SnapshotEntries()
	results := make(chan feedEntitySidecarBuildResult, len(names))
	ctx, cancel := context.WithCancel(ctx)
	jobs := make(chan string)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			setCache, closeSetCache := setCacheForWorker()
			if closeSetCache != nil {
				defer closeSetCache()
			}
			resolver := newEffectiveEntryResolver(e.cfg, entries)
			for {
				select {
				case <-ctx.Done():
					return
				case name, ok := <-jobs:
					if !ok {
						return
					}
					sidecar, err := e.buildSingleFeedEntitySidecar(name, view, resolver, geoProvider, asnProvider, geoPrepared, asnDB, setCache)
					if err != nil {
						sendFeedEntitySidecarBuildError(results, feedEntitySidecarBuildResult{name: name, err: fmt.Errorf("build feed entity sidecar %s: %w", name, err)})
						cancel()
						return
					}
					if !sendFeedEntitySidecarBuildResult(ctx, results, feedEntitySidecarBuildResult{name: name, sidecar: sidecar}) {
						return
					}
				}
			}
		})
	}
	closeResultsWhenFeedEntitySidecarBuildDone(ctx, names, jobs, results, &wg)
	return ctx, cancel, results
}

func closeResultsWhenFeedEntitySidecarBuildDone(ctx context.Context, names []string, jobs chan<- string, results chan<- feedEntitySidecarBuildResult, wg *sync.WaitGroup) {
	go func() {
	send:
		for _, name := range names {
			select {
			case <-ctx.Done():
				break send
			case jobs <- name:
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
}

func sendFeedEntitySidecarBuildResult(ctx context.Context, results chan<- feedEntitySidecarBuildResult, result feedEntitySidecarBuildResult) bool {
	select {
	case results <- result:
		return true
	case <-ctx.Done():
		return false
	}
}

func sendFeedEntitySidecarBuildError(results chan<- feedEntitySidecarBuildResult, result feedEntitySidecarBuildResult) {
	results <- result
}

func (e *Engine) stageFeedEntitySidecarResult(result feedEntitySidecarBuildResult, geoProvider, asnProvider, geoRefPath string, geoRefTime time.Time, asnRefPath string, asnRefTime time.Time, webStageDir string, entityBatch *stagedPublishBatch) (bool, error) {
	_, sidecarRefTime, err := e.entityFeedSidecarReferenceInOutputDir(result.name, webStageDir, geoProvider, asnProvider, geoRefPath, geoRefTime, asnRefPath, asnRefTime)
	if err != nil {
		return false, err
	}
	logicalTime := entityFeedSidecarReferenceMTime(result.sidecar, sidecarRefTime, e.feedProcessingTimestamp(result.name))
	current, err := e.loadCommittedFeedEntitySidecar(result.name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if reflect.DeepEqual(current, result.sidecar) {
		entityBatch.markDelete(e.entityFeedPendingRelPath(result.name))
		e.observeRunCounter("entity.sidecar_stage.unchanged_feed", 1, 0)
		if result.sidecar != nil {
			path := filepath.Join(e.entityFeedsDir(), result.name+".json")
			e.entityArtifactsMu.Lock()
			err := e.touchObservedFileAt(path, "entity.sidecar_stage.unchanged_feed_touch", logicalTime)
			e.entityArtifactsMu.Unlock()
			if err != nil && !os.IsNotExist(err) {
				return false, err
			}
		}
		return false, nil
	}
	if result.sidecar == nil {
		entityBatch.markDelete(e.entityFeedPendingRelPath(result.name))
		return true, nil
	}
	if err := writeJSONFileAt(filepath.Join(entityBatch.stageDir, e.entityFeedPendingRelPath(result.name)), result.sidecar, logicalTime); err != nil {
		return false, err
	}
	return true, nil
}
