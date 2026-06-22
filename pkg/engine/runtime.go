package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

type Runtime struct {
	ConfigPath                    string
	BaseDir                       string
	RunParentDir                  string
	LockFile                      string
	CacheDir                      string
	LibDir                        string
	AdminSuppliedIPSets           string
	DistributionSuppliedIPSets    string
	UserSuppliedIPSets            string
	HistoryDir                    string
	ErrorsDir                     string
	TmpDir                        string
	WebDir                        string
	WebOwner                      string
	WebDirForIPSets               string
	WebURL                        string
	PublicBaseURL                 string
	LocalCopyURL                  string
	GitHubChangesURL              string
	GitHubSetInfo                 string
	UserAgent                     string
	MaxConnectTime                time.Duration
	MaxDownloadTime               time.Duration
	MaxDownloadSize               int64
	ParallelDownloads             int
	IgnoreRepeatingDownloadErrors int
	ParallelDNSQueries            int
	IPSetsApply                   bool
	PushToGit                     bool
	PushToGitMerged               bool
	PushToGitCommitOptions        string
	PushToGitPushOptions          string
	PushToGitWeb                  bool
	WebChartsEntries              int
	WebArtifactCacheMaxEntries    int
	WebArtifactCacheMaxBytes      int64
	WebArtifactCacheMaxFileBytes  int64
	MaxIngestWorkers              int
	MaxProcessingWorkers          int
	MaxHeavyPhaseWorkers          int
	MaxBackgroundWorkers          int
	MaxEngineLaneWorkers          int
	MinRunIntervalSeconds         int
	ProcessingIntervalMinutes     int
	SkipComparisonIfNoUpdates     bool
	TrustProxyHeaders             bool
	TrustCloudflareHeaders        bool
}

type runtimePathContext struct {
	userMode      bool
	vars          map[string]string
	runParentDir  string
	baseDir       string
	cacheTemplate string
	libTemplate   string
}

func resolveConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	candidates := []string{
		"configs/firehol",
		"/opt/update-ipsets/etc/config",
		"/etc/firehol/update-ipsets",
		"/etc/firehol/update-ipsets.conf",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no config path provided and no default config found")
}

func resolveRuntime(cfg *config.Config, now time.Time) (Runtime, error) {
	if cfg == nil {
		return Runtime{}, fmt.Errorf("nil config")
	}

	pathCtx := resolveRuntimePathContext(cfg.Runtime, config.DefaultRuntime(), now)
	r := runtimeFromConfig(cfg.Runtime, pathCtx, now)
	applyRuntimeDefaults(&r, pathCtx)
	return r, nil
}

func resolveRuntimePathContext(rt, defaults config.RuntimeConfig, now time.Time) runtimePathContext {
	home, _ := os.UserHomeDir()
	userMode := os.Geteuid() != 0
	runParentTemplate := runtimeTemplateForMode(rt.RunParentDir, defaults.RunParentDir, filepath.Join(home, ".update-ipsets", "run"), userMode)
	runParentDir := expandTemplate(runParentTemplate, map[string]string{"HOME": home}, now)
	if runParentDir == "" {
		runParentDir = "/var/run"
	}
	vars := map[string]string{
		"HOME":           home,
		"run_parent_dir": runParentDir,
		"RUN_PARENT_DIR": runParentDir,
	}
	baseTemplate := runtimeTemplateForMode(rt.BaseDir, defaults.BaseDir, filepath.Join(home, ".update-ipsets", "ipsets"), userMode)
	baseDir := expandTemplate(baseTemplate, vars, now)
	vars["base_dir"] = baseDir
	vars["BASE_DIR"] = baseDir

	return runtimePathContext{
		userMode:      userMode,
		vars:          vars,
		runParentDir:  runParentDir,
		baseDir:       baseDir,
		cacheTemplate: runtimeTemplateForMode(rt.CacheDir, defaults.CacheDir, filepath.Join(home, ".cache", "update-ipsets"), userMode),
		libTemplate:   runtimeTemplateForMode(rt.LibDir, defaults.LibDir, filepath.Join(home, ".local", "share", "update-ipsets"), userMode),
	}
}

func runtimeTemplateForMode(configured, defaultValue, userModeValue string, userMode bool) string {
	if userMode && (configured == "" || configured == defaultValue) {
		return userModeValue
	}
	return configured
}

func runtimeFromConfig(rt config.RuntimeConfig, pathCtx runtimePathContext, now time.Time) Runtime {
	vars := pathCtx.vars
	r := Runtime{
		BaseDir:                       pathCtx.baseDir,
		RunParentDir:                  pathCtx.runParentDir,
		LockFile:                      expandTemplate(rt.LockFile, vars, now),
		CacheDir:                      expandTemplate(pathCtx.cacheTemplate, vars, now),
		LibDir:                        expandTemplate(pathCtx.libTemplate, vars, now),
		AdminSuppliedIPSets:           expandTemplate(rt.AdminSuppliedIPSets, vars, now),
		DistributionSuppliedIPSets:    expandTemplate(rt.DistributionSuppliedIPSets, vars, now),
		UserSuppliedIPSets:            expandTemplate(rt.UserSuppliedIPSets, vars, now),
		HistoryDir:                    expandTemplate(rt.HistoryDir, vars, now),
		ErrorsDir:                     expandTemplate(rt.ErrorsDir, vars, now),
		TmpDir:                        expandTemplate(rt.TmpDir, vars, now),
		WebDir:                        expandTemplate(rt.WebDir, vars, now),
		WebOwner:                      expandTemplate(rt.WebOwner, vars, now),
		WebDirForIPSets:               expandTemplate(rt.WebDirForIPSets, vars, now),
		WebURL:                        expandTemplate(rt.WebURL, vars, now),
		PublicBaseURL:                 expandTemplate(rt.PublicBaseURL, vars, now),
		LocalCopyURL:                  expandTemplate(rt.LocalCopyURL, vars, now),
		GitHubChangesURL:              expandTemplate(rt.GitHubChangesURL, vars, now),
		GitHubSetInfo:                 expandTemplate(rt.GitHubSetInfo, vars, now),
		UserAgent:                     expandTemplate(rt.UserAgent, vars, now),
		MaxConnectTime:                time.Duration(rt.MaxConnectTime) * time.Second,
		MaxDownloadTime:               time.Duration(rt.MaxDownloadTime) * time.Second,
		MaxDownloadSize:               rt.MaxDownloadSize,
		ParallelDownloads:             rt.ParallelDownloads,
		IgnoreRepeatingDownloadErrors: rt.IgnoreRepeatingDownloadErrors,
		ParallelDNSQueries:            rt.ParallelDNSQueries,
		IPSetsApply:                   rt.IPSetsApply && !pathCtx.userMode,
		PushToGit:                     rt.PushToGit,
		PushToGitMerged:               rt.PushToGitMerged,
		PushToGitCommitOptions:        rt.PushToGitCommitOptions,
		PushToGitPushOptions:          rt.PushToGitPushOptions,
		PushToGitWeb:                  rt.PushToGitWeb,
		WebChartsEntries:              rt.WebChartsEntries,
		WebArtifactCacheMaxEntries:    rt.WebArtifactCacheMaxEntries,
		WebArtifactCacheMaxBytes:      rt.WebArtifactCacheMaxBytes,
		WebArtifactCacheMaxFileBytes:  rt.WebArtifactCacheMaxFileBytes,
		MaxIngestWorkers:              rt.MaxIngestWorkers,
		MaxProcessingWorkers:          rt.MaxProcessingWorkers,
		MaxHeavyPhaseWorkers:          rt.MaxHeavyPhaseWorkers,
		MaxBackgroundWorkers:          rt.MaxBackgroundWorkers,
		MaxEngineLaneWorkers:          rt.MaxEngineLaneWorkers,
		MinRunIntervalSeconds:         rt.MinRunIntervalSeconds,
		ProcessingIntervalMinutes:     rt.ProcessingIntervalMinutes,
		SkipComparisonIfNoUpdates:     rt.SkipComparisonIfNoUpdates,
		TrustProxyHeaders:             rt.TrustProxyHeaders,
		TrustCloudflareHeaders:        rt.TrustCloudflareHeaders,
	}
	return r
}

func applyRuntimeDefaults(r *Runtime, pathCtx runtimePathContext) {
	applyRuntimePathDefaults(r, pathCtx)
	applyRuntimeDownloadDefaults(r)
	applyRuntimeWorkerDefaults(r)
	applyRuntimeIngestWorkerCeiling(r)
}

func applyRuntimePathDefaults(r *Runtime, pathCtx runtimePathContext) {
	if r.CacheDir == "" {
		r.CacheDir = pathCtx.baseDir
	}
	if r.LockFile == "" {
		r.LockFile = filepath.Join(pathCtx.runParentDir, "update-ipsets.lock")
	}
	if r.LibDir == "" {
		r.LibDir = filepath.Join(pathCtx.baseDir, "lib")
	}
	if r.HistoryDir == "" {
		r.HistoryDir = filepath.Join(pathCtx.baseDir, "history")
	}
	if r.ErrorsDir == "" {
		r.ErrorsDir = filepath.Join(pathCtx.baseDir, "errors")
	}
	if r.TmpDir == "" {
		r.TmpDir = os.TempDir()
	}
}

func applyRuntimeDownloadDefaults(r *Runtime) {
	if r.MaxConnectTime <= 0 {
		r.MaxConnectTime = 10 * time.Second
	}
	if r.MaxDownloadTime <= 0 {
		r.MaxDownloadTime = 300 * time.Second
	}
	if r.IgnoreRepeatingDownloadErrors <= 0 {
		r.IgnoreRepeatingDownloadErrors = 10
	}
	if r.ParallelDownloads <= 0 {
		r.ParallelDownloads = 5
	}
	if r.ParallelDNSQueries <= 0 {
		r.ParallelDNSQueries = 10
	}
}

func applyRuntimeWorkerDefaults(r *Runtime) {
	if r.MaxProcessingWorkers <= 0 {
		r.MaxProcessingWorkers = 2
	}
	if r.MaxHeavyPhaseWorkers <= 0 {
		r.MaxHeavyPhaseWorkers = autoHeavyPhaseWorkers(r.MaxProcessingWorkers)
	}
	if r.MaxBackgroundWorkers <= 0 {
		r.MaxBackgroundWorkers = 1
	}
	if r.MaxEngineLaneWorkers <= 0 {
		r.MaxEngineLaneWorkers = 1
	}
	if r.MinRunIntervalSeconds <= 0 {
		r.MinRunIntervalSeconds = 30
	}
	if r.ProcessingIntervalMinutes <= 0 {
		r.ProcessingIntervalMinutes = 10
	}
}

func applyRuntimeIngestWorkerCeiling(r *Runtime) {
	if r.MaxIngestWorkers <= 0 {
		return
	}
	r.ParallelDownloads = clampRuntimeWorkers(r.ParallelDownloads, r.MaxIngestWorkers)
	r.ParallelDNSQueries = clampRuntimeWorkers(r.ParallelDNSQueries, r.MaxIngestWorkers)
	r.MaxProcessingWorkers = clampRuntimeWorkers(r.MaxProcessingWorkers, r.MaxIngestWorkers)
	r.MaxHeavyPhaseWorkers = clampRuntimeWorkers(r.MaxHeavyPhaseWorkers, r.MaxIngestWorkers)
	r.MaxBackgroundWorkers = clampRuntimeWorkers(r.MaxBackgroundWorkers, r.MaxIngestWorkers)
	r.MaxEngineLaneWorkers = clampRuntimeWorkers(r.MaxEngineLaneWorkers, r.MaxIngestWorkers)
}

func clampRuntimeWorkers(value, ceiling int) int {
	if value < 1 {
		value = 1
	}
	if ceiling > 0 && value > ceiling {
		return ceiling
	}
	return value
}

func (e *Engine) ApplyRuntimeOverrides(webDir, filesDir string) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	e.runtimeOverrideWebDir = strings.TrimSpace(webDir)
	e.runtimeOverrideFilesDir = strings.TrimSpace(filesDir)
	e.applyRuntimeOverridesLocked()
	e.mu.Unlock()
	return e.ensureDirectories()
}

func (e *Engine) applyRuntimeOverridesLocked() {
	if e == nil {
		return
	}
	if e.runtimeOverrideWebDir != "" {
		e.runtime.WebDir = e.runtimeOverrideWebDir
		if e.cfg != nil {
			e.cfg.Runtime.WebDir = e.runtimeOverrideWebDir
		}
	}
	if e.runtimeOverrideFilesDir != "" {
		e.runtime.WebDirForIPSets = e.runtimeOverrideFilesDir
		if e.cfg != nil {
			e.cfg.Runtime.WebDirForIPSets = e.runtimeOverrideFilesDir
		}
	}
}

func autoHeavyPhaseWorkers(processingWorkers int) int {
	auto := goruntime.NumCPU()
	if auto < 1 {
		auto = 1
	}
	if auto > 8 {
		auto = 8
	}
	if processingWorkers > auto {
		return processingWorkers
	}
	return auto
}

func (r Runtime) HeavyPhaseWorkers() int {
	if r.MaxHeavyPhaseWorkers > 0 {
		return r.MaxHeavyPhaseWorkers
	}
	return autoHeavyPhaseWorkers(r.MaxProcessingWorkers)
}

func (r Runtime) BackgroundWorkers() int {
	if r.MaxBackgroundWorkers > 0 {
		return r.MaxBackgroundWorkers
	}
	return 1
}

func (r Runtime) EngineLaneWorkers() int {
	if r.MaxEngineLaneWorkers > 0 {
		return r.MaxEngineLaneWorkers
	}
	return 1
}

func expandTemplate(input string, vars map[string]string, now time.Time) string {
	if input == "" {
		return ""
	}
	out := expandShellStyle(input, vars)
	out = os.Expand(out, func(key string) string {
		if value, ok := vars[key]; ok {
			return value
		}
		return os.Getenv(key)
	})
	replacer := strings.NewReplacer(
		"{HOME}", vars["HOME"],
		"{run_parent_dir}", vars["run_parent_dir"],
		"{RUN_PARENT_DIR}", vars["RUN_PARENT_DIR"],
		"{base_dir}", vars["base_dir"],
		"{BASE_DIR}", vars["BASE_DIR"],
		"{YYYY}", now.UTC().Format("2006"),
		"{YY}", now.UTC().Format("06"),
		"{MM}", now.UTC().Format("01"),
		"{DD}", now.UTC().Format("02"),
	)
	return replacer.Replace(out)
}

var shellExpansionRE = regexp.MustCompile(`\$\{([^{}]+)\}`)

func expandShellStyle(input string, vars map[string]string) string {
	out := input
	for {
		changed := false
		out = shellExpansionRE.ReplaceAllStringFunc(out, func(match string) string {
			content := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
			key, def, hasDef := strings.Cut(content, "-")
			key = strings.TrimSpace(key)
			value := vars[key]
			if value == "" {
				value = os.Getenv(key)
			}
			if value == "" && hasDef {
				value = def
			}
			if value != match {
				changed = true
			}
			return value
		})
		if !changed {
			break
		}
	}
	return out
}
