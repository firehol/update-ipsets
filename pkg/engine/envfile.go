package engine

import (
	"bufio"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const envFileName = ".update-ipsets.env"

// loadEnvFile reads a KEY=VALUE file from the HOME directory of the
// running process and sets the variables as environment variables.
// Lines starting with # are comments. Blank lines are skipped.
// Values may be optionally quoted with single or double quotes.
// This is called early in engine initialization so that URL templates
// like ${MAXMIND_LICENSE_KEY} get expanded correctly.
func loadEnvFile(logger *slog.Logger) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, envFileName)
	loadEnvFileFrom(path, logger)
}

func loadEnvFileFrom(path string, logger *slog.Logger) {
	file, err := openFilePathUnderRoot(filepath.Dir(path), path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("no env file found", "path", path)
		} else {
			logger.Warn("failed to open env file", "path", path, "error", err)
		}
		return
	}
	defer func() { _ = file.Close() }()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		// Strip optional quotes from value.
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		// Only set if not already set in the environment — explicit
		// env vars (e.g. from systemd) take precedence.
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				logger.Warn("failed to set env variable from env file", "path", path, "key", key, "error", err)
				continue
			}
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Warn("error reading env file", "path", path, "error", err)
	}
	if count > 0 {
		logger.Info("loaded environment variables from env file", "path", path, "count", count)
	}
}

// hasUnexpandedVars checks if a URL still contains ${...} patterns,
// indicating that a required environment variable was not set.
func hasUnexpandedVars(url string) (string, bool) {
	start := strings.Index(url, "${")
	if start < 0 {
		return "", false
	}
	end := strings.Index(url[start:], "}")
	if end < 0 {
		return "", false
	}
	// Extract the variable name (strip the default part after -)
	varExpr := url[start+2 : start+end]
	varName, _, _ := strings.Cut(varExpr, "-")
	return strings.TrimSpace(varName), true
}
