package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/fileutil"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/iprange"
	"github.com/firehol/update-ipsets/pkg/kernel"
	"github.com/firehol/update-ipsets/pkg/output"
)

var (
	aggregationDayHourSuffixRE = regexp.MustCompile(`_(\d+)d(\d+)h$`)
	aggregationDaySuffixRE     = regexp.MustCompile(`_(\d+)d$`)
	aggregationHourSuffixRE    = regexp.MustCompile(`_(\d+)h$`)
)

// expandURL expands environment variable templates in a URL.
// Returns an empty string if the URL still contains unexpanded ${...}
// variables after expansion (i.e. a required API key is not set).
func (e *Engine) expandURL(url string) string {
	expanded := expandTemplate(url, map[string]string{
		"HOME":     os.Getenv("HOME"),
		"base_dir": e.runtime.BaseDir,
		"BASE_DIR": e.runtime.BaseDir,
	}, e.now().UTC())
	if varName, unexpanded := hasUnexpandedVars(expanded); unexpanded {
		e.logger.Warn("URL has missing environment variable, skipping download", "var", varName, "url_template", url)
		return ""
	}
	return expanded
}

func (e *Engine) isEnabled(name string, opts RunOptions) bool {
	return EffectiveSourceEnabledForRun(
		e.cfg,
		e.runtime,
		name,
		opts.EnableAll,
		opts.Manual || isSelected(name, opts.Selected),
	)
}

func (e *Engine) sourcePath(name string) string {
	return sourcePathForRuntime(e.runtime, name)
}

func processingPath(path string) string {
	return path + ".processing"
}

func stagedPath(path string) string {
	return path + ".new"
}

func pendingTempPath(path string) string {
	return path + ".tmp"
}

func preferStagedPath(path string) string {
	stage := stagedPath(path)
	if fileExists(stage) {
		return stage
	}
	return path
}

func (e *Engine) providerArchivePath(name string, src *config.Source) string {
	if src != nil && src.HasUse(config.UseASN) {
		return filepath.Join(e.runtime.LibDir, "asn", name, "source")
	}
	return filepath.Join(e.runtime.LibDir, "geolocation", name+".source")
}

func (e *Engine) finalPath(name, output string) string {
	switch canonicalOutputFamily(output) {
	case "ipset":
		return filepath.Join(e.runtime.BaseDir, name+".ipset")
	default:
		return filepath.Join(e.runtime.BaseDir, name+".netset")
	}
}

func (e *Engine) feedBodyPath(name string) string {
	if e == nil {
		return ""
	}
	if e.cfg == nil {
		return filepath.Join(e.runtime.BaseDir, name+".ipset")
	}
	src := e.cfg.Sources[name]
	if src == nil {
		return filepath.Join(e.runtime.BaseDir, name+".ipset")
	}
	return e.finalPath(name, src.Output)
}

func (e *Engine) FeedBodyPath(name string) string {
	return e.feedBodyPath(name)
}

func latestFeedBodyPath(path string) string {
	if p, ok := existingLatestFeedBodyPath(path); ok {
		return p
	}
	return path
}

func existingLatestFeedBodyPath(path string) (string, bool) {
	if fileExists(stagedPath(path)) {
		return stagedPath(path), true
	}
	if fileExists(processingPath(path)) {
		return processingPath(path), true
	}
	if fileExists(path) {
		return path, true
	}
	return "", false
}

func claimProcessingFeedBody(path string) (string, error) {
	procPath := processingPath(path)
	if fileExists(procPath) {
		return procPath, nil
	}
	stagePath := stagedPath(path)
	if fileExists(stagePath) {
		_ = os.Remove(procPath)
		if err := os.Rename(stagePath, procPath); err != nil {
			return "", err
		}
		return procPath, nil
	}
	if fileExists(path) {
		return path, nil
	}
	return "", os.ErrNotExist
}

func (e *Engine) applyRenamesAndDeletes() error {
	for oldName, newName := range e.cfg.Renames {
		if err := e.renameIPSet(oldName, newName); err != nil {
			return err
		}
	}
	for _, name := range e.cfg.Deleted {
		if err := e.deleteIPSet(name); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) renameIPSet(oldName, newName string) error {
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	for _, suffix := range []string{".source", ".ipset", ".ipset.new", ".ipset.processing", ".netset", ".netset.new", ".netset.processing", ".split", ".setinfo"} {
		oldPath := filepath.Join(e.runtime.BaseDir, oldName+suffix)
		newPath := filepath.Join(e.runtime.BaseDir, newName+suffix)
		if err := renameIfPresent(oldPath, newPath); err != nil {
			return err
		}
	}
	for _, dir := range []string{e.runtime.HistoryDir, e.runtime.LibDir} {
		oldPath := filepath.Join(dir, oldName)
		newPath := filepath.Join(dir, newName)
		if fileExists(oldPath) && !fileExists(newPath) {
			if err := os.Rename(oldPath, newPath); err != nil {
				return err
			}
		}
	}
	for _, suffix := range e.publicArtifactSuffixes() {
		oldPath := filepath.Join(e.outputDir(), oldName+suffix)
		newPath := filepath.Join(e.outputDir(), newName+suffix)
		if err := renameIfPresent(oldPath, newPath); err != nil {
			return err
		}
	}
	e.state.RenameEntry(oldName, newName)
	return nil
}

func (e *Engine) deleteIPSet(name string) error {
	if name == "" {
		return nil
	}
	for _, suffix := range []string{".source", ".ipset", ".ipset.new", ".ipset.processing", ".netset", ".netset.new", ".netset.processing", ".split", ".setinfo"} {
		if err := os.Remove(filepath.Join(e.runtime.BaseDir, name+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			e.logger.Warn("failed to remove ipset file during cleanup", "source", name, "suffix", suffix, "error", err)
		}
	}
	for _, suffix := range e.publicArtifactSuffixes() {
		if err := os.Remove(filepath.Join(e.outputDir(), name+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			e.logger.Warn("failed to remove ipset web file during cleanup", "source", name, "suffix", suffix, "error", err)
		}
	}
	if err := os.RemoveAll(filepath.Join(e.runtime.HistoryDir, name)); err != nil {
		e.logger.Warn("failed to remove ipset history dir during cleanup", "source", name, "error", err)
	}
	if err := os.RemoveAll(filepath.Join(e.runtime.LibDir, name)); err != nil {
		e.logger.Warn("failed to remove ipset lib dir during cleanup", "source", name, "error", err)
	}
	e.state.Remove(name)
	return nil
}

func renameIfPresent(oldPath, newPath string) error {
	if !fileExists(oldPath) || fileExists(newPath) {
		return nil
	}
	return os.Rename(oldPath, newPath)
}

func (e *Engine) publicArtifactSuffixes() []string {
	suffixes := []string{
		"_comparison.json",
		"_history.csv",
		"_changesets.csv",
		"_retention.json",
		"retention.json",
		"_insights.json",
		"_critical_infrastructure.json",
		".json",
		".html",
	}
	if e.cfg != nil {
		for _, src := range e.cfg.SourcesWithUse(config.UseGeoIP) {
			suffixes = append(suffixes, "_"+src.Name+".json")
		}
		for _, src := range e.cfg.SourcesWithUse(config.UseASN) {
			suffixes = append(suffixes, "_asn_"+src.Name+".json")
		}
		for _, src := range e.cfg.SourcesWithUse(config.UseBogons) {
			suffixes = append(suffixes, "_bogons_"+src.Name+".json")
		}
		for _, src := range e.cfg.SourcesWithUse(config.UseCriticalInfrastructure) {
			suffixes = append(suffixes, "_critical_"+src.Name+".json")
		}
	}
	slices.Sort(suffixes)
	out := suffixes[:0]
	var prev string
	for i, suffix := range suffixes {
		if i == 0 || suffix != prev {
			out = append(out, suffix)
			prev = suffix
		}
	}
	return out
}

func (e *Engine) applyKernelSet(name, hash, bodyPath string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	body, err := os.Open(bodyPath)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	lines, err := nonCommentLineStrings(body)
	if err != nil {
		return err
	}
	_, err = kernel.ApplyIfLoaded(name, hash, lines)
	if err != nil {
		return fmt.Errorf("kernel ipset apply failed: %w", err)
	}
	return nil
}

func processorSteps(src *config.Source) []config.ProcessorStep {
	if len(src.Processor) > 0 {
		return append([]config.ProcessorStep(nil), src.Processor...)
	}
	if src.ProcessorRaw != "" {
		return []config.ProcessorStep{{Name: src.ProcessorRaw}}
	}
	return []config.ProcessorStep{{Name: "passthrough"}}
}

func publicURL(src *config.Source) string {
	if src.Attributes["public_url"] != "" {
		return src.Attributes["public_url"]
	}
	return src.URL
}

func (e *Engine) isRedistributable(name string) bool {
	src := e.cfg.Sources[name]
	if src == nil {
		return true
	}
	// Derivatives (merges, retention variants) inherit the AND of
	// their own explicit flag and every transitive parent's flag.
	// Retention variants pick up the parent's pointer at expansion
	// time so the explicit-flag check handles them; merges were
	// created with the default (nil ⇒ true) so this loop is the
	// only thing that makes them non-redistributable when any
	// input is.
	if !src.IsRedistributable() {
		return false
	}
	if len(src.DerivedFrom) > 0 {
		for _, parent := range src.DerivedFrom {
			if !e.isRedistributable(parent) {
				return false
			}
		}
	}
	return true
}

func filterGeneratedFiles(baseDir string, files []output.GeneratedFile) []output.GeneratedFile {
	filtered := make([]output.GeneratedFile, 0, len(files))
	for _, file := range files {
		if rel, err := filepath.Rel(baseDir, file.Path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func historyLabel(minutes int) string {
	if minutes >= 24*60 {
		days := minutes / (24 * 60)
		rem := minutes - days*(24*60)
		if rem == 0 {
			return fmt.Sprintf("_%dd", days)
		}
		return fmt.Sprintf("_%dd%dh", days, rem/60)
	}
	return fmt.Sprintf("_%dh", minutes/60)
}

// hashForOutput maps the canonical Source.Output value to the
// kernel ipset "hash:" type string. "ipset" → "ip" (hash:ip),
// "netset" → "net" (hash:net). Used when rendering the .ipset /
// .netset file header, which is where the operator sees the
// ipset type displayed.
func hashForOutput(output string) string {
	if canonicalOutputFamily(output) == "ipset" {
		return "ip"
	}
	return "net"
}

func ipSetContentHash(set *iprange.IPSet) string {
	if set == nil {
		return ""
	}
	set.Optimize()
	hash, _ := rangeSourceContentHash(set)
	return hash
}

func ipSetContentHashIfNeeded(set *iprange.IPSet, needed bool) string {
	if !needed {
		return ""
	}
	return ipSetContentHash(set)
}

func rangeSourceContentHash(src iprange.RangeSource) (string, bool) {
	if src == nil {
		return "", false
	}
	h := sha256.New()
	var buf [8]byte
	for r := range src.Iter() {
		binary.BigEndian.PutUint32(buf[0:4], r.Lo)
		binary.BigEndian.PutUint32(buf[4:8], r.Hi)
		_, _ = h.Write(buf[:])
	}
	type errChecker interface {
		Err() error
	}
	if ec, ok := src.(errChecker); ok && ec.Err() != nil {
		return "", false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func canonicalOutputFamily(output string) string {
	switch strings.TrimSpace(strings.ToLower(output)) {
	case "ip", "ips", "ipset":
		return "ipset"
	case "net", "nets", "both", "all", "netset":
		return "netset"
	default:
		return output
	}
}

// aggregationMinutesFromName extracts the aggregation window (in minutes)
// from a history variant suffix like "_1d", "_7d", "_30d", "_12h", "_1d12h".
// Returns 0 for base ipsets (no history suffix).
func aggregationMinutesFromName(name string) int {
	// Match patterns like _<n>d, _<n>h, _<n>d<n>h at the end of name.
	if m := aggregationDayHourSuffixRE.FindStringSubmatch(name); m != nil {
		days, _ := strconv.Atoi(m[1])
		hours, _ := strconv.Atoi(m[2])
		return days*24*60 + hours*60
	}
	if m := aggregationDaySuffixRE.FindStringSubmatch(name); m != nil {
		days, _ := strconv.Atoi(m[1])
		return days * 24 * 60
	}
	if m := aggregationHourSuffixRE.FindStringSubmatch(name); m != nil {
		hours, _ := strconv.Atoi(m[1])
		return hours * 60
	}
	return 0
}

func wrapInfo(info string) []string {
	info = strings.ReplaceAll(info, "](", "] (")
	words := strings.Fields(info)
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, len(words)/8+1)
	var current strings.Builder
	for _, word := range words {
		if current.Len() > 0 && current.Len()+1+len(word) > 60 {
			lines = append(lines, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

// minutesText converts a duration in minutes to the same textual
// format produced by the bash mins_to_text function. Non-zero
// values include a trailing space (matching the bash output).
func minutesText(minutes int) string {
	if minutes <= 0 {
		return "none"
	}
	var parts []string
	days := minutes / (24 * 60)
	minutes -= days * 24 * 60
	hours := minutes / 60
	minutes -= hours * 60

	switch days {
	case 0:
	case 1:
		parts = append(parts, "1 day")
	default:
		parts = append(parts, fmt.Sprintf("%d days", days))
	}
	switch hours {
	case 0:
	case 1:
		parts = append(parts, "1 hour")
	default:
		parts = append(parts, fmt.Sprintf("%d hours", hours))
	}
	switch minutes {
	case 0:
	case 1:
		parts = append(parts, "1 min")
	default:
		parts = append(parts, fmt.Sprintf("%d mins", minutes))
	}
	if len(parts) == 0 {
		return "none"
	}
	// Bash mins_to_text always prints a trailing space after each part.
	return strings.Join(parts, " ") + " "
}

// moveDownloadedBody atomically moves a downloader result's temp file to dst.
// Falls back to copy if rename fails (cross-device). Cleans up the source on success.
func moveDownloadedBody(result *downloader.Result, dst string) error {
	if result == nil || result.BodyPath == "" {
		return fmt.Errorf("no body file to move")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	// Try rename first (same filesystem).
	if err := os.Rename(result.BodyPath, dst); err == nil {
		result.BodyPath = ""
		return nil
	}
	// Cross-device: copy + remove.
	src, err := os.Open(result.BodyPath)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-mv-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	if err := os.Remove(result.BodyPath); err != nil {
		return err
	}
	result.BodyPath = ""
	return nil
}

// stageDownloadedBody writes a completed download into {dst}.new via an
// explicit {dst}.tmp hop so restart recovery can distinguish incomplete
// writes from fully-downloaded, not-yet-committed artifacts.
func stageDownloadedBody(result *downloader.Result, dst string) error {
	if result == nil || result.BodyPath == "" {
		return fmt.Errorf("no body file to stage")
	}
	tmpPath := pendingTempPath(dst)
	stagePath := stagedPath(dst)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	_ = os.Remove(tmpPath)
	_ = os.Remove(stagePath)
	if err := moveDownloadedBody(result, tmpPath); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, stagePath); err != nil {
		return err
	}
	return nil
}

func promoteStagedFile(dst string) error {
	stagePath := stagedPath(dst)
	if !fileExists(stagePath) {
		return nil
	}
	_ = os.Remove(dst)
	return os.Rename(stagePath, dst)
}

// writeFileAtomic delegates to the shared fileutil package.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	return fileutil.WriteAtomic(path, data, mode)
}

func writeFileAtomicNoSync(path string, data []byte, mode os.FileMode) error {
	return fileutil.WriteAtomicNoSync(path, data, mode)
}

func writeFileAtomicAt(path string, data []byte, mode os.FileMode, mod time.Time) error {
	if err := writeFileAtomic(path, data, mode); err != nil {
		return err
	}
	if mod.IsZero() {
		return nil
	}
	return os.Chtimes(path, mod.UTC(), mod.UTC())
}

func touchFileAt(path string, mod time.Time) error {
	if mod.IsZero() {
		mod = time.Now()
	}
	if !fileExists(path) {
		if err := writeFileAtomic(path, nil, 0o600); err != nil {
			return err
		}
	}
	return os.Chtimes(path, mod, mod)
}

func writeBinaryPath(path string, set *iprange.IPSet, mod time.Time) error {
	var buf bytes.Buffer
	if err := iprange.WriteBinary(&buf, set); err != nil {
		return err
	}
	if err := writeFileAtomic(path, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return touchFileAt(path, mod)
}

func (e *Engine) feedProcessingTimestamp(name string) time.Time {
	if e != nil {
		if e.state != nil {
			if entry := e.EntrySnapshot(name); entry != nil && entry.ProcessedDate > 0 {
				return time.Unix(entry.ProcessedDate, 0).UTC()
			}
		}
		if e.now != nil {
			return e.now().UTC()
		}
	}
	return time.Now().UTC()
}

func ensureCSVHeader(path, header string) error {
	if fileExists(path) {
		return nil
	}
	return writeFileAtomic(path, []byte(header), 0o600)
}

func appendCSV(path, header, line string) error {
	if err := ensureCSVHeader(path, header); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if _, err = io.WriteString(file, line); err != nil {
		return err
	}
	return file.Sync()
}

// fileExists delegates to the shared fileutil package.
func fileExists(path string) bool {
	return fileutil.Exists(path)
}

func isSelected(name string, selected []string) bool {
	for _, entry := range selected {
		if entry == name {
			return true
		}
	}
	return false
}

// targetFeedsForFanOut returns the list of feed names a fan-out computation
// should process. Role arguments identify which provider roles affect the
// caller's artifacts. The rule is:
//
//   - If updatedNames is empty (e.g. an initial run with no source
//     updates) return every output name.
//   - If any name in updatedNames is itself a configured provider for one of
//     the caller's roles, the provider has fresh data so EVERY output feed
//     must be re-compared against it. Return all output names regardless of
//     which other feeds updated.
//   - If any name in updatedNames is the underlying feed of a
//     bogon feed_reference provider, the provider's data has
//     effectively updated, so force the same full fan-out — every
//     output feed needs to be re-compared against the new bogon
//     union.
//   - Otherwise, intersect updatedNames with the set of valid output
//     names so we only re-compare the feeds that actually changed.
//
// This is the single source of truth for the geo/asn/bogon/critical fan-out
// selection. Without the provider-aware branch, a run where ONLY a provider
// updates (and no source feeds) would compute nothing: the provider's fresh
// data would be loaded and discarded. The role filter is deliberately
// caller-specific so a critical-reference update does not also force all geo,
// ASN, bogon, and entity sidecar artifacts to rebuild.
func targetFeedsForFanOut(cfg *config.Config, updatedNames []string, outputNames []string, roles ...string) []string {
	if len(updatedNames) == 0 {
		return outputNames
	}
	if len(roles) == 0 {
		roles = []string{config.UseASN, config.UseGeoIP, config.UseBogons, config.UseCriticalInfrastructure}
	}
	providerUpdated := false
	// Build the set of source names that participate in any of the
	// caller's engine roles whose comparison files fan out to every feed.
	// Updating any of these sources requires regenerating every per-feed
	// comparison file because the reference data they all depend on has
	// changed.
	var roleSources map[string]struct{}
	if cfg != nil {
		for _, role := range roles {
			for _, src := range cfg.SourcesWithUse(role) {
				if roleSources == nil {
					roleSources = map[string]struct{}{}
				}
				roleSources[src.Name] = struct{}{}
			}
		}
	}
	for _, name := range updatedNames {
		if cfg == nil {
			break
		}
		if _, ok := roleSources[name]; ok {
			providerUpdated = true
			break
		}
	}
	if providerUpdated {
		return outputNames
	}
	allSet := make(map[string]struct{}, len(outputNames))
	for _, name := range outputNames {
		allSet[name] = struct{}{}
	}
	out := make([]string, 0, len(updatedNames))
	for _, name := range updatedNames {
		if _, ok := allSet[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

func (e *Engine) perFeedPublicationNames(updatedNames []string, opts RunOptions) []string {
	outputNames := e.publicOutputNames()
	if opts.Reprocess && len(opts.Selected) == 0 {
		return outputNames
	}
	if len(updatedNames) == 0 {
		return nil
	}
	outputSet := make(map[string]struct{}, len(outputNames))
	for _, name := range outputNames {
		outputSet[name] = struct{}{}
	}
	out := make([]string, 0, len(updatedNames))
	seen := make(map[string]struct{}, len(updatedNames))
	for _, name := range updatedNames {
		if _, ok := outputSet[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// applyEntryStatsUpdate updates the running min/max bounds, version
// counter, and initial cadence stats on a cache entry after a successful
// update. Entries and UniqueIPs must already be populated on the entry
// before this is called. frequency is the configured check frequency
// used to seed cadence stats on the first successful update; pass 0
// for feed kinds that do not have a scheduled cadence (e.g. merges,
// which rebuild opportunistically whenever an input changes) and the
// cadence fields will be left alone.
//
// This is the single source of truth for post-update bookkeeping and is
// called from every processing path (sources, merges, geolocation
// providers, ASN providers) so every kind of feed gets the same
// treatment.
func applyEntryStatsUpdate(entry *cache.Entry, frequency int) {
	entry.RecordStatsUpdate(frequency)
}
