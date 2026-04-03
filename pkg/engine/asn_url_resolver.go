package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var caidaCreationLogHTTPClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: caidaCreationLogTransport(),
}

func caidaCreationLogTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConnsPerHost:   2,
			ResponseHeaderTimeout: 15 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
		}
	}
	transport := base.Clone()
	transport.MaxIdleConnsPerHost = 2
	transport.ResponseHeaderTimeout = 15 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	return transport
}

// resolveASNDownloadURL turns the YAML-configured URL for an ASN provider
// into the concrete URL the downloader should fetch.
//
// For most provider types this is a no-op pass-through; the URL in the
// config (after template expansion for variables like {YYYY}-{MM}) is
// already the file we want.
//
// For CAIDA prefix2as, the daily filename is not predictable from the
// current date alone (it follows BGP table availability and lags by ~12
// to 24 hours). Instead of guessing today's filename, we fetch the
// pfx2as creation log from a sibling URL and pick the latest entry.
// This keeps the YAML config stable across days while always pointing
// at a real file.
func (e *Engine) resolveASNDownloadURL(ctx context.Context, providerType, configuredURL string) (string, error) {
	switch providerType {
	case "caida_prefix2as":
		return e.resolveCAIDAPrefix2ASURL(ctx, configuredURL)
	default:
		return configuredURL, nil
	}
}

// resolveCAIDAPrefix2ASURL fetches the CAIDA pfx2as-creation.log from
// the directory two levels above the configured URL and returns the
// last entry mapped to a full https URL.
//
// The configured URL must point at a file under
// https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/
// (any month is fine — the function only uses the host and base path).
//
// The creation log format is three tab-separated columns per line:
// seqnum, unix timestamp, relative path. The last line is the most
// recent file. Empty lines and lines starting with '#' are ignored.
func (e *Engine) resolveCAIDAPrefix2ASURL(ctx context.Context, configuredURL string) (string, error) {
	const baseDir = "https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/"
	logURL := baseDir + "pfx2as-creation.log"

	if !strings.HasPrefix(configuredURL, baseDir) {
		// Honor any non-default URL the operator may have set (e.g. a
		// mirror) — in that case we cannot use the central creation
		// log, so we fall through and use the URL as configured.
		return configuredURL, nil
	}

	httpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(httpCtx, http.MethodGet, logURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request for %s: %w", logURL, err)
	}
	req.Header.Set("User-Agent", e.runtime.UserAgent)
	resp, err := caidaCreationLogHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", logURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: status %d", logURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", logURL, err)
	}

	latestPath := lastCAIDACreationLogEntry(string(body))
	if latestPath == "" {
		return "", fmt.Errorf("creation log %s has no usable entries", logURL)
	}
	return baseDir + latestPath, nil
}

// lastCAIDACreationLogEntry returns the relative path field of the last
// non-empty, non-comment line in a CAIDA pfx2as creation log. The line
// format is "seqnum \t timestamp \t YYYY/MM/routeviews-rv2-...pfx2as.gz".
// Returns an empty string if no usable line is found.
func lastCAIDACreationLogEntry(body string) string {
	var latest string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		latest = fields[len(fields)-1]
	}
	return latest
}
