package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

func (e *Engine) PublicCompose(ctx context.Context, include, exclude []string, format string) ([]byte, error) {
	if len(include) == 0 {
		return nil, fmt.Errorf("missing include sets")
	}
	snap := e.operationSnapshot()
	canonicalInclude := make([]string, 0, len(include))
	for _, name := range include {
		canonical, err := e.canonicalPublicComposeSetWithSnapshot(snap, name)
		if err != nil {
			return nil, err
		}
		canonicalInclude = append(canonicalInclude, canonical)
	}
	canonicalExclude := make([]string, 0, len(exclude))
	for _, name := range exclude {
		canonical, err := e.canonicalPublicComposeSetWithSnapshot(snap, name)
		if err != nil {
			return nil, err
		}
		canonicalExclude = append(canonicalExclude, canonical)
	}
	return e.Compose(ctx, canonicalInclude, canonicalExclude, format)
}

func (e *Engine) canonicalPublicComposeSet(input string) (string, error) {
	return e.canonicalPublicComposeSetWithSnapshot(e.operationSnapshot(), input)
}

func (e *Engine) canonicalPublicComposeSetWithSnapshot(snap operationSnapshot, input string) (string, error) {
	requested := strings.TrimSpace(input)
	if requested == "" {
		return "", fmt.Errorf("empty set name")
	}
	name, ok := e.canonicalConfiguredNameWithSnapshot(snap, requested)
	if !ok {
		return "", fmt.Errorf("unknown set %q", requested)
	}
	if !isPublicFeedNameForConfig(snap.cfg, name) {
		return "", fmt.Errorf("unknown set %q", requested)
	}
	if !isRedistributableForConfig(snap.cfg, name) {
		return "", fmt.Errorf("set %q is not redistributable", name)
	}
	entry := e.EntrySnapshot(name)
	if entry == nil {
		return "", fmt.Errorf("set %q is not available for raw feed access", name)
	}
	src := lookupSourceForConfig(snap.cfg, name)
	if feedhealth.Classify(entry, src, snap.feedHealthPolicy, e.now().UTC()).Class == feedhealth.ClassArchived {
		return "", fmt.Errorf("set %q is not available for raw feed access", name)
	}
	return name, nil
}

func (e *Engine) canonicalConfiguredName(requested string) (string, bool) {
	return e.canonicalConfiguredNameWithSnapshot(e.operationSnapshot(), requested)
}

func (e *Engine) canonicalConfiguredNameWithSnapshot(snap operationSnapshot, requested string) (string, bool) {
	if e == nil || snap.cfg == nil {
		return "", false
	}
	for name, src := range snap.cfg.Sources {
		if name == requested {
			return name, true
		}
		for _, minutes := range src.History {
			historyName := name + historyLabel(minutes)
			if historyName == requested {
				return historyName, true
			}
		}
	}
	for name := range snap.cfg.Merges {
		if name == requested {
			return name, true
		}
	}
	return "", false
}
