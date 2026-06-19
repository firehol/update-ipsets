package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
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

func (e *Engine) buildFeedEntityDelta(name string) (feedEntityDelta, error) {
	return e.buildFeedEntityDeltaWithPresence(name, nil)
}

func (e *Engine) buildFeedEntityDeltaWithPresence(name string, presence *entityArtifactFeedPresence) (feedEntityDelta, error) {
	delta := feedEntityDelta{name: name}

	oldPath := filepath.Join(e.entityFeedsDir(), name+".json")
	oldSidecar, err := e.loadFeedEntitySidecar(oldPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return delta, err
		}
		found, scanErr := e.entityArtifactsContainFeedWithPresence(name, presence)
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
