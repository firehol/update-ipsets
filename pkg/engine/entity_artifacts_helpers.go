package engine

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

func sortedStringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func (e *Engine) loadFeedEntitySidecarMTimes() map[string]time.Time {
	return e.loadFeedEntitySidecarMTimesWithRuntime(e.Runtime())
}

func (e *Engine) loadFeedEntitySidecarMTimesWithRuntime(rt Runtime) map[string]time.Time {
	out := map[string]time.Time{}
	paths, err := sortedJSONFiles(entityFeedsDirForRuntime(rt))
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
	return e.stagePublicSitemapFilesWithSnapshot(e.operationSnapshot(), webBatch, generated)
}

func (e *Engine) stagePublicSitemapFilesWithSnapshot(snap operationSnapshot, webBatch *stagedPublishBatch, generated []output.GeneratedFile) ([]output.GeneratedFile, error) {
	if webBatch == nil {
		return generated, nil
	}
	siteBase := publicSiteBaseURLForRuntime(snap.runtime)
	files, err := e.writeSitemapFilesWithSnapshot(snap, webBatch.stageDir, siteBase, publicFeedURLPrefixForRuntime(snap.runtime, siteBase), e.publicOutputNamesForSnapshot(snap))
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		generated = append(generated, output.GeneratedFile{Path: filepath.Join(outputDirForRuntime(snap.runtime), name), Redistributable: true})
	}
	stale, err := staleSitemapShardNames(outputDirForRuntime(snap.runtime), files)
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
