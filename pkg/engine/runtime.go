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
	MaxProcessingWorkers          int
	MaxHeavyPhaseWorkers          int
	MaxBackgroundWorkers          int
	MinRunIntervalSeconds         int
	ProcessingIntervalMinutes     int
	SkipComparisonIfNoUpdates     bool
	TrustProxyHeaders            bool
	TrustCloudflareHeaders       bool
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

	home, _ := os.UserHomeDir()
	defaults := config.DefaultRuntime()
	userMode := os.Geteuid() != 0
	runParentTemplate := cfg.Runtime.RunParentDir
	if userMode && (runParentTemplate == "" || runParentTemplate == defaults.RunParentDir) {
		runParentTemplate = filepath.Join(home, ".update-ipsets", "run")
	}
	runParentDir := expandTemplate(runParentTemplate, map[string]string{"HOME": home}, now)
	if runParentDir == "" {
		runParentDir = "/var/run"
	}
	vars := map[string]string{
		"HOME":           home,
		"run_parent_dir": runParentDir,
		"RUN_PARENT_DIR": runParentDir,
	}
	baseTemplate := cfg.Runtime.BaseDir
	if userMode && (baseTemplate == "" || baseTemplate == defaults.BaseDir) {
		baseTemplate = filepath.Join(home, ".update-ipsets", "ipsets")
	}
	baseDir := expandTemplate(baseTemplate, vars, now)
	vars["base_dir"] = baseDir
	vars["BASE_DIR"] = baseDir

	cacheTemplate := cfg.Runtime.CacheDir
	if userMode && (cacheTemplate == "" || cacheTemplate == defaults.CacheDir) {
		cacheTemplate = filepath.Join(home, ".cache", "update-ipsets")
	}
	libTemplate := cfg.Runtime.LibDir
	if userMode && (libTemplate == "" || libTemplate == defaults.LibDir) {
		libTemplate = filepath.Join(home, ".local", "share", "update-ipsets")
	}

	r := Runtime{
		BaseDir:                       baseDir,
		RunParentDir:                  runParentDir,
		LockFile:                      expandTemplate(cfg.Runtime.LockFile, vars, now),
		CacheDir:                      expandTemplate(cacheTemplate, vars, now),
		LibDir:                        expandTemplate(libTemplate, vars, now),
		AdminSuppliedIPSets:           expandTemplate(cfg.Runtime.AdminSuppliedIPSets, vars, now),
		DistributionSuppliedIPSets:    expandTemplate(cfg.Runtime.DistributionSuppliedIPSets, vars, now),
		UserSuppliedIPSets:            expandTemplate(cfg.Runtime.UserSuppliedIPSets, vars, now),
		HistoryDir:                    expandTemplate(cfg.Runtime.HistoryDir, vars, now),
		ErrorsDir:                     expandTemplate(cfg.Runtime.ErrorsDir, vars, now),
		TmpDir:                        expandTemplate(cfg.Runtime.TmpDir, vars, now),
		WebDir:                        expandTemplate(cfg.Runtime.WebDir, vars, now),
		WebOwner:                      expandTemplate(cfg.Runtime.WebOwner, vars, now),
		WebDirForIPSets:               expandTemplate(cfg.Runtime.WebDirForIPSets, vars, now),
		WebURL:                        expandTemplate(cfg.Runtime.WebURL, vars, now),
		PublicBaseURL:                 expandTemplate(cfg.Runtime.PublicBaseURL, vars, now),
		LocalCopyURL:                  expandTemplate(cfg.Runtime.LocalCopyURL, vars, now),
		GitHubChangesURL:              expandTemplate(cfg.Runtime.GitHubChangesURL, vars, now),
		GitHubSetInfo:                 expandTemplate(cfg.Runtime.GitHubSetInfo, vars, now),
		UserAgent:                     expandTemplate(cfg.Runtime.UserAgent, vars, now),
		MaxConnectTime:                time.Duration(cfg.Runtime.MaxConnectTime) * time.Second,
		MaxDownloadTime:               time.Duration(cfg.Runtime.MaxDownloadTime) * time.Second,
		MaxDownloadSize:               cfg.Runtime.MaxDownloadSize,
		ParallelDownloads:             cfg.Runtime.ParallelDownloads,
		IgnoreRepeatingDownloadErrors: cfg.Runtime.IgnoreRepeatingDownloadErrors,
		ParallelDNSQueries:            cfg.Runtime.ParallelDNSQueries,
		IPSetsApply:                   cfg.Runtime.IPSetsApply && !userMode,
		PushToGit:                     cfg.Runtime.PushToGit,
		PushToGitMerged:               cfg.Runtime.PushToGitMerged,
		PushToGitCommitOptions:        cfg.Runtime.PushToGitCommitOptions,
		PushToGitPushOptions:          cfg.Runtime.PushToGitPushOptions,
		PushToGitWeb:                  cfg.Runtime.PushToGitWeb,
		WebChartsEntries:              cfg.Runtime.WebChartsEntries,
		WebArtifactCacheMaxEntries:    cfg.Runtime.WebArtifactCacheMaxEntries,
		WebArtifactCacheMaxBytes:      cfg.Runtime.WebArtifactCacheMaxBytes,
		WebArtifactCacheMaxFileBytes:  cfg.Runtime.WebArtifactCacheMaxFileBytes,
		MaxProcessingWorkers:          cfg.Runtime.MaxProcessingWorkers,
		MaxHeavyPhaseWorkers:          cfg.Runtime.MaxHeavyPhaseWorkers,
		MaxBackgroundWorkers:          cfg.Runtime.MaxBackgroundWorkers,
		MinRunIntervalSeconds:         cfg.Runtime.MinRunIntervalSeconds,
		ProcessingIntervalMinutes:     cfg.Runtime.ProcessingIntervalMinutes,
		SkipComparisonIfNoUpdates:     cfg.Runtime.SkipComparisonIfNoUpdates,
		TrustProxyHeaders:            cfg.Runtime.TrustProxyHeaders,
		TrustCloudflareHeaders:       cfg.Runtime.TrustCloudflareHeaders,
	}
	if r.CacheDir == "" {
		r.CacheDir = baseDir
	}
	if r.LockFile == "" {
		r.LockFile = filepath.Join(runParentDir, "update-ipsets.lock")
	}
	if r.LibDir == "" {
		r.LibDir = filepath.Join(baseDir, "lib")
	}
	if r.HistoryDir == "" {
		r.HistoryDir = filepath.Join(baseDir, "history")
	}
	if r.ErrorsDir == "" {
		r.ErrorsDir = filepath.Join(baseDir, "errors")
	}
	if r.TmpDir == "" {
		r.TmpDir = os.TempDir()
	}
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
	if r.MaxProcessingWorkers <= 0 {
		r.MaxProcessingWorkers = 2
	}
	if r.MaxHeavyPhaseWorkers <= 0 {
		r.MaxHeavyPhaseWorkers = autoHeavyPhaseWorkers(r.MaxProcessingWorkers)
	}
	if r.MaxBackgroundWorkers <= 0 {
		r.MaxBackgroundWorkers = 1
	}
	if r.MinRunIntervalSeconds <= 0 {
		r.MinRunIntervalSeconds = 30
	}
	if r.ProcessingIntervalMinutes <= 0 {
		r.ProcessingIntervalMinutes = 10
	}
	return r, nil
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
