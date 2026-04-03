package engine

import (
	"context"
	"fmt"
	"strings"
)

func (e *Engine) PublicCompose(ctx context.Context, include, exclude []string, format string) ([]byte, error) {
	if len(include) == 0 {
		return nil, fmt.Errorf("missing include sets")
	}
	canonicalInclude := make([]string, 0, len(include))
	for _, name := range include {
		canonical, err := e.canonicalPublicComposeSet(name)
		if err != nil {
			return nil, err
		}
		canonicalInclude = append(canonicalInclude, canonical)
	}
	canonicalExclude := make([]string, 0, len(exclude))
	for _, name := range exclude {
		canonical, err := e.canonicalPublicComposeSet(name)
		if err != nil {
			return nil, err
		}
		canonicalExclude = append(canonicalExclude, canonical)
	}
	return e.Compose(ctx, canonicalInclude, canonicalExclude, format)
}

func (e *Engine) canonicalPublicComposeSet(input string) (string, error) {
	requested := strings.TrimSpace(input)
	if requested == "" {
		return "", fmt.Errorf("empty set name")
	}
	name, ok := e.canonicalConfiguredName(requested)
	if !ok {
		return "", fmt.Errorf("unknown set %q", requested)
	}
	if !e.isPublicFeedName(name) {
		return "", fmt.Errorf("unknown set %q", requested)
	}
	if !e.isRedistributable(name) {
		return "", fmt.Errorf("set %q is not redistributable", name)
	}
	if !e.PublicRawFeedAllowed(name) {
		return "", fmt.Errorf("set %q is not available for raw feed access", name)
	}
	return name, nil
}

func (e *Engine) canonicalConfiguredName(requested string) (string, bool) {
	if e == nil || e.cfg == nil {
		return "", false
	}
	for name, src := range e.cfg.Sources {
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
	for name := range e.cfg.Merges {
		if name == requested {
			return name, true
		}
	}
	return "", false
}
