package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type CountryValue struct {
	Code  string `json:"code"`
	Name  string `json:"name,omitempty"`
	Value uint64 `json:"value"`
}

func isPublicFeedSource(src *config.Source) bool {
	if src == nil || src.Hidden {
		return false
	}
	if src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP) {
		return false
	}
	return true
}

func (e *Engine) isPublicFeedName(name string) bool {
	if e == nil {
		return false
	}
	return isPublicFeedNameForConfig(e.Config(), name)
}

func isPublicFeedNameForConfig(cfg *config.Config, name string) bool {
	if cfg == nil || name == "" {
		return false
	}
	if !configuredNamesForConfig(cfg)[name] {
		return false
	}
	return isPublicFeedSource(lookupSourceForConfig(cfg, name))
}

func (e *Engine) IsPublicFeedName(name string) bool {
	return e.isPublicFeedName(name)
}

func (e *Engine) IsRedistributable(name string) bool {
	return e.isRedistributable(name)
}

func (e *Engine) PublicRawFeedAllowed(name string) bool {
	_, ok := e.PublicRawFeedFile(name)
	return ok
}

func (e *Engine) PublicRawFeedFile(name string) (string, bool) {
	return e.publicRawFeedFileWithSnapshot(e.operationSnapshot(), name)
}

func (e *Engine) publicRawFeedFileWithSnapshot(snap operationSnapshot, name string) (string, bool) {
	if e == nil {
		return "", false
	}
	if !isPublicFeedNameForConfig(snap.cfg, name) {
		return "", false
	}
	if !isRedistributableForConfig(snap.cfg, name) {
		return "", false
	}
	entry := e.EntrySnapshot(name)
	if entry == nil {
		return "", false
	}
	src := lookupSourceForConfig(snap.cfg, name)
	if feedhealth.Classify(entry, src, snap.feedHealthPolicy, e.now().UTC()).Class == feedhealth.ClassArchived {
		return "", false
	}
	if !rawFeedFileMatches(name, entry.File) {
		return "", false
	}
	return entry.File, true
}

func (e *Engine) Entry(name string) (*cache.Entry, error) {
	// Hidden sources and provider-role supporting datasets are still
	// tracked by the cache but never appear on the normal public feed
	// API. They are visible only to admin endpoints and to the
	// provider-scoped public views that explicitly model them as
	// supporting datasets rather than public feeds.
	if !e.isPublicFeedName(name) {
		return nil, fmt.Errorf("unknown set %q", name)
	}
	snap := e.EntrySnapshot(name)
	if snap == nil {
		return nil, fmt.Errorf("unknown set %q", name)
	}
	return snap, nil
}

func (e *Engine) SetData(name string) ([]byte, *cache.Entry, error) {
	snap := e.operationSnapshot()
	if !isPublicFeedNameForConfig(snap.cfg, name) {
		return nil, nil, fmt.Errorf("unknown set %q", name)
	}
	entry := e.EntrySnapshot(name)
	if entry == nil {
		return nil, nil, fmt.Errorf("unknown set %q", name)
	}
	if !isRedistributableForConfig(snap.cfg, name) {
		return nil, nil, fmt.Errorf("set %q is not redistributable", name)
	}
	src := lookupSourceForConfig(snap.cfg, name)
	if feedhealth.Classify(entry, src, snap.feedHealthPolicy, e.now().UTC()).Class == feedhealth.ClassArchived {
		return nil, nil, fmt.Errorf("set %q is archived and raw feed access is disabled", name)
	}
	if entry.File == "" {
		return nil, nil, fmt.Errorf("set %q has no materialized file", name)
	}
	if !rawFeedFileMatches(name, entry.File) {
		return nil, nil, fmt.Errorf("set %q has unexpected materialized file %q", name, entry.File)
	}
	if _, ok := safeRuntimeFilePath(snap.runtime.BaseDir, entry.File); !ok {
		return nil, nil, fmt.Errorf("set %q has unsafe materialized file %q", name, entry.File)
	}
	data, err := readFileInRoot(snap.runtime.BaseDir, entry.File)
	if err != nil {
		return nil, nil, err
	}
	return data, entry, nil
}

func rawFeedFileMatches(name, file string) bool {
	return file == name+".ipset" || file == name+".netset"
}

func safeRuntimeFilePath(baseDir, rel string) (string, bool) {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(rel) == "" || filepath.IsAbs(rel) {
		return "", false
	}
	cleanBase := filepath.Clean(baseDir)
	resolved := filepath.Clean(filepath.Join(cleanBase, rel))
	if resolved == cleanBase || strings.HasPrefix(resolved, cleanBase+string(os.PathSeparator)) {
		return resolved, true
	}
	return "", false
}

// ASNProvider describes one configured ASN database source as exposed
// to the public API. The frontend uses this to render the provider tabs.
//
// Label is the human-friendly display name pulled from the source's
// YAML `label:` field; when empty the frontend falls back to Name.
// License and Attribution surface the licensing constraints of the
// underlying data so the frontend can render the required attribution
// notice and so users can tell at a glance which sources permit raw
// redistribution. Redistributable mirrors the YAML flag of the same
// name; when false, only derived statistics (the per-feed JSON outputs)
// are published — the raw downloaded archive stays local.
//
// The Type field carries the source's Format value (the wire format
// identifier) so the frontend can recognise specific data shapes when
// it has to.
type ASNProvider struct {
	Name            string `json:"name"`
	Label           string `json:"label,omitempty"`
	Type            string `json:"type"`
	Info            string `json:"info,omitempty"`
	License         string `json:"license,omitempty"`
	Attribution     string `json:"attribution,omitempty"`
	Redistributable bool   `json:"redistributable"`
	Maintainer      string `json:"maintainer,omitempty"`
	MaintainerURL   string `json:"maintainer_url,omitempty"`
}

// BogonProvider describes one configured bogon source as exposed to
// the public API. The frontend uses this to render the provider tabs
// in the bogon section.
//
// Authoritative is true for the source carrying the RFC reserved
// baseline (format == RFCReservedFormat) — the IETF-defined ranges
// are not subject to maintainer drift, so the frontend uses that one
// source as the "authoritative" subsection and treats every other
// bogon source as a third-party cross-reference. The flag is the
// only way the frontend can distinguish the two without hardcoding
// either a source name or a format identifier of its own.
type BogonProvider struct {
	Name          string `json:"name"`
	Label         string `json:"label,omitempty"`
	Type          string `json:"type"`
	Feed          string `json:"feed,omitempty"`
	Info          string `json:"info,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	MaintainerURL string `json:"maintainer_url,omitempty"`
	Authoritative bool   `json:"authoritative,omitempty"`
}

// GeoProvider describes one configured geolocation database source as
// exposed to the public API. Mirrors ASNProvider/BogonProvider so the
// frontend can iterate the three provider lists with the same logic.
type GeoProvider struct {
	Name            string `json:"name"`
	Label           string `json:"label,omitempty"`
	Type            string `json:"type"`
	Info            string `json:"info,omitempty"`
	License         string `json:"license,omitempty"`
	Attribution     string `json:"attribution,omitempty"`
	Redistributable bool   `json:"redistributable"`
	Maintainer      string `json:"maintainer,omitempty"`
	MaintainerURL   string `json:"maintainer_url,omitempty"`
}

// BogonProviders returns the list of configured bogon sources in YAML
// declaration order. Hidden sources ARE included here because they
// represent reference data that other feeds compare against — the
// rfc_reserved synthetic baseline is the canonical example. Hiding it
// from the bogon provider tabs would defeat its purpose. The Hidden
// flag affects per-source pages, not comparison tabs.
func (e *Engine) BogonProviders() []BogonProvider {
	if e == nil {
		return nil
	}
	cfg := e.Config()
	if cfg == nil {
		return nil
	}
	sources := cfg.SourcesWithUse(config.UseBogons)
	out := make([]BogonProvider, 0, len(sources))
	for _, src := range sources {
		out = append(out, BogonProvider{
			Name:          src.Name,
			Label:         src.Label,
			Type:          src.Format,
			Info:          src.Info,
			Maintainer:    src.Maintainer,
			MaintainerURL: src.MaintainerURL,
			Authoritative: src.Format == RFCReservedFormat,
		})
	}
	return out
}

// ASNProviders returns configured ASN sources with the explicit default source
// first, followed by the remaining sources in catalog order. Hidden sources are
// included because ASN tabs represent reference data, not navigable feed pages.
func (e *Engine) ASNProviders() []ASNProvider {
	if e == nil {
		return nil
	}
	cfg := e.Config()
	if cfg == nil {
		return nil
	}
	sources := cfg.SourcesWithUseDefaultFirst(config.UseASN)
	out := make([]ASNProvider, 0, len(sources))
	for _, src := range sources {
		out = append(out, ASNProvider{
			Name:            src.Name,
			Label:           src.Label,
			Type:            src.Format,
			Info:            src.Info,
			License:         src.License,
			Attribution:     src.Attribution,
			Redistributable: src.IsRedistributable(),
			Maintainer:      src.Maintainer,
			MaintainerURL:   src.MaintainerURL,
		})
	}
	return out
}

// GeoProviders returns configured geolocation sources with the explicit default
// source first, followed by the remaining sources in catalog order.
func (e *Engine) GeoProviders() []GeoProvider {
	if e == nil {
		return nil
	}
	cfg := e.Config()
	if cfg == nil {
		return nil
	}
	sources := cfg.SourcesWithUseDefaultFirst(config.UseGeoIP)
	out := make([]GeoProvider, 0, len(sources))
	for _, src := range sources {
		out = append(out, GeoProvider{
			Name:            src.Name,
			Label:           src.Label,
			Type:            src.Format,
			Info:            src.Info,
			License:         src.License,
			Attribution:     src.Attribution,
			Redistributable: src.IsRedistributable(),
			Maintainer:      src.Maintainer,
			MaintainerURL:   src.MaintainerURL,
		})
	}
	return out
}

// CountryComparisonPayload is the serialised shape of
// <feed>_<provider>.json. The Countries array is sorted by code;
// TotalMapped is the feed's intersection with the provider's union
// of all country sets (de-duplicated, unlike summing per-country
// values which can over-count when the parser has overlapping ranges).
type CountryComparisonPayload struct {
	TotalMapped uint64         `json:"total_mapped"`
	Countries   []CountryValue `json:"countries"`
}

// CountryComparison loads the per-feed-per-provider country payload
// for the given provider. The provider argument is the full source
// name (e.g. "geolite2_country") — the frontend learns it from the
// /api/v1/sets/{name}/countries index endpoint and the URL path mirror
// is exact, no suffix manipulation.
func (e *Engine) CountryComparison(name, provider string) (*CountryComparisonPayload, error) {
	dir := ""
	if e != nil {
		dir = e.outputDir()
	}
	return e.CountryComparisonInDir(name, provider, dir)
}

// CountryComparisonInDir loads the per-feed-per-provider country payload
// from a specific published artifact directory. Public web handlers use
// this when Options.WebDir overrides the engine runtime directory.
func (e *Engine) CountryComparisonInDir(name, provider, dir string) (*CountryComparisonPayload, error) {
	if dir == "" && e != nil {
		dir = e.outputDir()
	}
	if dir == "" {
		return nil, fmt.Errorf("engine is not configured")
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.json", name, provider))
	start := time.Now()
	payload, err := loadCountryComparisonPayload(path)
	if err == nil {
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("engine.country_comparison_json_read", 1, bytes)
		e.observeRunOperation("engine.country_comparison_json_load", time.Since(start))
	}
	return payload, err
}

type limitedWriter struct {
	w        *bytes.Buffer
	limit    int
	exceeded bool
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.exceeded {
		return 0, fmt.Errorf("compose result too large (%d bytes > %d bytes limit)", lw.w.Len()+len(p), lw.limit)
	}
	if lw.w.Len()+len(p) > lw.limit {
		lw.exceeded = true
		return 0, fmt.Errorf("compose result too large (%d bytes > %d bytes limit)", lw.w.Len()+len(p), lw.limit)
	}
	return lw.w.Write(p)
}

type serverError struct {
	err error
}

func (e *serverError) Error() string { return e.err.Error() }
func (e *serverError) Unwrap() error { return e.err }

func wrapServerError(err error) error {
	if err == nil {
		return nil
	}
	return &serverError{err: err}
}

// IsServerError reports whether err originated from a server-side failure
// (I/O, mmap, file close) rather than a bad client request.
func IsServerError(err error) bool {
	var se *serverError
	return errors.As(err, &se)
}

const (
	composeMaxInclude = 20
	composeMaxExclude = 20
	composeMaxOutput  = 32 * 1024 * 1024 // 32 MiB
)

func (e *Engine) Compose(ctx context.Context, include, exclude []string, format string) ([]byte, error) {
	if len(include) == 0 {
		return nil, fmt.Errorf("missing include sets")
	}
	if len(include) > composeMaxInclude {
		return nil, fmt.Errorf("too many include sets (%d > %d)", len(include), composeMaxInclude)
	}
	if len(exclude) > composeMaxExclude {
		return nil, fmt.Errorf("too many exclude sets (%d > %d)", len(exclude), composeMaxExclude)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	includeSrcs := make([]*closableSource, 0, len(include))
	for _, name := range include {
		src, err := e.openLatestSet(ctx, name)
		if err != nil {
			_ = closeClosableSources(includeSrcs)
			return nil, err
		}
		includeSrcs = append(includeSrcs, src)
	}

	rangeSrcs := make([]iprange.RangeSource, len(includeSrcs))
	for i, src := range includeSrcs {
		rangeSrcs[i] = src.RangeSource
	}
	result, err := iprange.UnionSourcesContext(ctx, "composed", rangeSrcs...)
	if err != nil {
		_ = closeClosableSources(includeSrcs)
		return nil, wrapServerError(err)
	}

	for i, s := range includeSrcs {
		if ioErr := checkFileSetErr(s.RangeSource, include[i], e.logger); ioErr != nil {
			_ = closeClosableSources(includeSrcs)
			return nil, wrapServerError(fmt.Errorf("compose include %s: %w", include[i], ioErr))
		}
	}

	if err := closeClosableSources(includeSrcs); err != nil {
		return nil, wrapServerError(fmt.Errorf("close compose include sources: %w", err))
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	for _, name := range exclude {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		exclSrc, err := e.openLatestSet(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("compose exclude %s: %w", name, err)
		}
		result, err = iprange.ExcludeSourcesContext(ctx, "composed", result, exclSrc.RangeSource)
		if err != nil {
			_ = exclSrc.Close()
			return nil, wrapServerError(err)
		}
		if ioErr := checkFileSetErr(exclSrc.RangeSource, name, e.logger); ioErr != nil {
			_ = exclSrc.Close()
			return nil, wrapServerError(fmt.Errorf("compose exclude %s: %w", name, ioErr))
		}
		if err := exclSrc.Close(); err != nil {
			return nil, wrapServerError(fmt.Errorf("close compose exclude %s: %w", name, err))
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	opts := iprange.DefaultPrintOptions()
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "cidr", "net", "nets":
		opts.Format = iprange.PrintCIDR
	case "range", "ranges":
		opts.Format = iprange.PrintRanges
	case "single", "ip", "ips":
		opts.Format = iprange.PrintSingleIPs
	default:
		return nil, fmt.Errorf("unsupported compose format %q", format)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, limit: composeMaxOutput}
	if err := result.Write(lw, opts); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
