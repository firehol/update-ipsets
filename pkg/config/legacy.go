package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var runtimeAssignments = map[string]func(*RuntimeConfig, string) error{
	"BASE_DIR":                     func(r *RuntimeConfig, v string) error { r.BaseDir = v; return nil },
	"CONFIG_FILE":                  func(r *RuntimeConfig, v string) error { r.ConfigFile = v; return nil },
	"RUN_PARENT_DIR":               func(r *RuntimeConfig, v string) error { r.RunParentDir = v; return nil },
	"UPDATE_IPSETS_LOCK_FILE":      func(r *RuntimeConfig, v string) error { r.LockFile = v; return nil },
	"LOCK_FILE":                    func(r *RuntimeConfig, v string) error { r.LockFile = v; return nil },
	"CACHE_DIR":                    func(r *RuntimeConfig, v string) error { r.CacheDir = v; return nil },
	"LIB_DIR":                      func(r *RuntimeConfig, v string) error { r.LibDir = v; return nil },
	"ADMIN_SUPPLIED_IPSETS":        func(r *RuntimeConfig, v string) error { r.AdminSuppliedIPSets = v; return nil },
	"DISTRIBUTION_SUPPLIED_IPSETS": func(r *RuntimeConfig, v string) error { r.DistributionSuppliedIPSets = v; return nil },
	"USER_SUPPLIED_IPSETS":         func(r *RuntimeConfig, v string) error { r.UserSuppliedIPSets = v; return nil },
	"HISTORY_DIR":                  func(r *RuntimeConfig, v string) error { r.HistoryDir = v; return nil },
	"ERRORS_DIR":                   func(r *RuntimeConfig, v string) error { r.ErrorsDir = v; return nil },
	"TMP_DIR":                      func(r *RuntimeConfig, v string) error { r.TmpDir = v; return nil },
	"USER_AGENT":                   func(r *RuntimeConfig, v string) error { r.UserAgent = v; return nil },
	"WEB_DIR":                      func(r *RuntimeConfig, v string) error { r.WebDir = v; return nil },
	"WEB_OWNER":                    func(r *RuntimeConfig, v string) error { r.WebOwner = v; return nil },
	"WEB_URL":                      func(r *RuntimeConfig, v string) error { r.WebURL = v; return nil },
	"PUBLIC_BASE_URL":              func(r *RuntimeConfig, v string) error { r.PublicBaseURL = v; return nil },
	"WEB_DIR_FOR_IPSETS":           func(r *RuntimeConfig, v string) error { r.WebDirForIPSets = v; return nil },
	"LOCAL_COPY_URL":               func(r *RuntimeConfig, v string) error { r.LocalCopyURL = v; return nil },
	"GITHUB_CHANGES_URL":           func(r *RuntimeConfig, v string) error { r.GitHubChangesURL = v; return nil },
	"GITHUB_SETINFO":               func(r *RuntimeConfig, v string) error { r.GitHubSetInfo = v; return nil },
}

var legacyAssignmentRE = regexp.MustCompile(`^([A-Z0-9_]+)=(.*)$`)

func LoadLegacy(path string) (*Config, error) {
	cfg, err := ExtractLegacyScript(path, ExtractOptions{IncludeGeolocation: true})
	if err != nil {
		return nil, err
	}

	if err := mergeLegacyAssignments(path, &cfg.Runtime); err != nil {
		return nil, err
	}

	dir := cfg.Runtime.AdminSuppliedIPSets
	if dir == "" {
		dir = filepath.Dir(path)
	}
	if extra, err := loadLegacyDirectory(dir); err == nil {
		for name, src := range extra.Sources {
			cfg.Sources[name] = src
		}
		for name, merge := range extra.Merges {
			cfg.Merges[name] = merge
		}
		for oldName, newName := range extra.Renames {
			cfg.Renames[oldName] = newName
		}
		cfg.Deleted = append(cfg.Deleted, extra.Deleted...)
	}

	if err := injectBuiltInSyntheticSources(cfg); err != nil {
		return nil, err
	}
	return cfg, Validate(cfg)
}

func loadLegacyDirectory(dir string) (*Config, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	cfg := New()
	matches, err := filepath.Glob(filepath.Join(dir, "*.conf"))
	if err != nil {
		return nil, err
	}
	for _, match := range matches {
		extracted, err := ExtractLegacyScript(match, ExtractOptions{})
		if err != nil {
			return nil, err
		}
		for name, src := range extracted.Sources {
			cfg.Sources[name] = src
		}
		for name, merge := range extracted.Merges {
			cfg.Merges[name] = merge
		}
		for oldName, newName := range extracted.Renames {
			cfg.Renames[oldName] = newName
		}
		cfg.Deleted = append(cfg.Deleted, extracted.Deleted...)
	}
	return cfg, nil
}

func mergeLegacyAssignments(path string, runtime *RuntimeConfig) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := legacyAssignmentRE.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		key := match[1]
		handler := runtimeAssignments[key]
		if handler == nil {
			continue
		}
		value := strings.TrimSpace(match[2])
		value = strings.Trim(value, `"`)
		value = strings.Trim(value, `'`)
		if err := handler(runtime, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
