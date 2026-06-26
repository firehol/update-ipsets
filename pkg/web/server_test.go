package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestServeRunServersReturnsErrorWhenServerGoroutinePanics(t *testing.T) {
	oldServe := serveRunServer
	serveRunServer = func(namedServer, string, string) error {
		panic("forced server panic")
	}
	t.Cleanup(func() {
		serveRunServer = oldServe
	})

	ctx, cancel := context.WithCancel(context.Background())
	err := serveRunServers([]namedServer{{
		name:   "admin",
		addr:   "127.0.0.1:0",
		server: &http.Server{},
	}}, "", "", cancel)
	if err == nil || !strings.Contains(err.Error(), "forced server panic") {
		t.Fatalf("serveRunServers error = %v, want recovered panic", err)
	}
	if ctx.Err() == nil {
		t.Fatal("serveRunServers did not call cancel after server panic")
	}
}

func TestAPIEndpointsAndCORS(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n5.6.7.0/30\n"))
	}))
	defer sourceServer.Close()

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  sample:
    url: %q
    frequency: 1
    history: [60]
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{EnableAll: true, Manual: true, CleanupOld: true}); err != nil {
		t.Fatal(err)
	}

	runner := scheduler.New(eng, true, nil)
	handler := newHandler(eng, Options{EnableAll: true}, runner)
	server := newWebHTTPTestServer(t, handler)

	for _, name := range []string{"sitemap.xml", "sitemap-pages.xml", "sitemap-feeds.xml", "sitemap-countries.xml", "sitemap-maintainers.xml", "robots.txt", "llms.txt"} {
		if _, err := os.Stat(filepath.Join(eng.Runtime().WebDir, name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}

	var body []byte
	status, headers, _ := server.do(t, http.MethodOptions, "/api/v1/status", nil)
	if status != http.StatusNoContent {
		t.Fatalf("unexpected OPTIONS status: got %d", status)
	}
	if got := headers.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("missing CORS header, got %q", got)
	}

	var ipsets []struct {
		Name string `json:"name"`
	}
	status, _ = server.getJSON(t, "/api/v1/ipsets", &ipsets)
	if status != http.StatusOK {
		t.Fatalf("unexpected ipsets status: got %d", status)
	}
	if !containsPublicFeedName(ipsets, "sample") {
		t.Fatalf("unexpected ipsets body: %+v", ipsets)
	}

	var searchPayload struct {
		Matches []struct {
			Name string `json:"name"`
		} `json:"matches"`
	}
	status, _ = server.getJSON(t, "/api/v1/search?ip=5.6.7.2&details=true", &searchPayload)
	if status != http.StatusOK {
		t.Fatalf("unexpected search status: got %d", status)
	}
	if !containsSearchMatchName(searchPayload.Matches, "sample") {
		t.Fatalf("unexpected search body: %+v", searchPayload)
	}

	status, _, body = server.get(t, "/api/v1/compose?include=sample&format=single")
	if status != http.StatusOK {
		t.Fatalf("unexpected compose status: got %d", status)
	}
	if !strings.Contains(string(body), "1.2.3.4") {
		t.Fatalf("unexpected compose body: %s", body)
	}

	status, _, _ = server.do(t, http.MethodGet, "/admin/", func(req *http.Request) {
		req.SetBasicAuth("admin", "secret")
	})
	if status != http.StatusOK {
		t.Fatalf("unexpected admin slash status: got %d", status)
	}

	status, _, body = server.get(t, "/sitemap.xml")
	if status != http.StatusOK {
		t.Fatalf("unexpected sitemap status: got %d", status)
	}
	sitemap := string(body)
	if !strings.Contains(sitemap, "<sitemapindex") {
		t.Fatalf("expected sitemap index, got %s", sitemap)
	}
	for _, want := range []string{
		"<loc>https://iplists.firehol.org/sitemap-pages.xml</loc>",
		"<loc>https://iplists.firehol.org/sitemap-feeds.xml</loc>",
		"<loc>https://iplists.firehol.org/sitemap-maintainers.xml</loc>",
	} {
		if !strings.Contains(sitemap, want) {
			t.Fatalf("sitemap index missing %q: %s", want, sitemap)
		}
	}
	if strings.Contains(sitemap, "/admin") || strings.Contains(sitemap, "/api/v1/admin") {
		t.Fatalf("sitemap exposed admin path: %s", sitemap)
	}

	status, _, body = server.get(t, "/sitemap-pages.xml")
	if status != http.StatusOK {
		t.Fatalf("unexpected sitemap pages status: got %d", status)
	}
	pagesSitemap := string(body)
	if !strings.Contains(pagesSitemap, "<loc>https://iplists.firehol.org</loc>") {
		t.Fatalf("expected public site root in pages sitemap, got %s", pagesSitemap)
	}

	status, _, body = server.get(t, "/sitemap-feeds.xml")
	if status != http.StatusOK {
		t.Fatalf("unexpected sitemap feeds status: got %d", status)
	}
	feedsSitemap := string(body)
	if !strings.Contains(feedsSitemap, "<loc>https://iplists.firehol.org/ipsets/sample</loc>") {
		t.Fatalf("expected sample URL in feeds sitemap, got %s", feedsSitemap)
	}

	status, _, body = server.get(t, "/sitemap-maintainers.xml")
	if status != http.StatusOK {
		t.Fatalf("unexpected sitemap maintainers status: got %d", status)
	}
	maintainersSitemap := string(body)
	if !strings.Contains(maintainersSitemap, "<loc>https://iplists.firehol.org/maintainers/test</loc>") {
		t.Fatalf("expected maintainer URL in maintainers sitemap, got %s", maintainersSitemap)
	}

	status, _, body = server.get(t, "/robots.txt")
	if status != http.StatusOK {
		t.Fatalf("unexpected robots status: got %d", status)
	}
	robots := string(body)
	if !strings.Contains(robots, "User-agent: *\n") || !strings.Contains(robots, "Allow: /\n") {
		t.Fatalf("unexpected robots body: %s", robots)
	}
	for _, want := range []string{
		"Disallow: /api/v1/search\n",
		"Disallow: /api/v1/query\n",
		"Disallow: /api/v1/compose\n",
		"Disallow: /api/v1/client-ip\n",
		"Disallow: /api/v1/sets/*/search\n",
		"Disallow: /api/v1/ipsets/*/search\n",
	} {
		if !strings.Contains(robots, want) {
			t.Fatalf("robots missing %q: %s", want, robots)
		}
	}
	if !strings.Contains(robots, "Sitemap: https://iplists.firehol.org/sitemap.xml\n") {
		t.Fatalf("robots missing public sitemap URL: %s", robots)
	}
	if strings.Contains(robots, "/admin") || strings.Contains(robots, "/api/v1/admin") {
		t.Fatalf("robots exposed admin path: %s", robots)
	}

	status, _, body = server.get(t, "/llms.txt")
	if status != http.StatusOK {
		t.Fatalf("unexpected llms status: got %d", status)
	}
	llms := string(body)
	for _, want := range []string{
		"# FireHOL IP Lists",
		"https://iplists.firehol.org/methodology",
		"https://iplists.firehol.org/api/v1/sets",
		"https://iplists.firehol.org/all-ipsets.json",
		"https://iplists.firehol.org/ipsets/sample",
	} {
		if !strings.Contains(llms, want) {
			t.Fatalf("llms.txt missing %q: %s", want, llms)
		}
	}
	if strings.Contains(llms, "/admin") || strings.Contains(llms, "/api/v1/admin") {
		t.Fatalf("llms.txt exposed admin path: %s", llms)
	}

	// Insights endpoint serves the JSON file written by the engine
	// in the heavy block. The fixture run above produces an insights
	// file (possibly empty) for every output feed; the endpoint must
	// return it with application/json content type.
	status, headers, body = server.get(t, "/api/v1/sets/sample/insights")
	if status != http.StatusOK {
		t.Fatalf("unexpected insights status: got %d", status)
	}
	if ct := headers.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected insights content type: %q", ct)
	}
	var insightsPayload struct {
		Items []any `json:"items"`
	}
	decodeTestJSONInto(t, body, &insightsPayload)
	if insightsPayload.Items == nil {
		t.Fatalf("insights body missing items field: %s", body)
	}

	// A non-existent feed must 404 (the Entry() check fires before the
	// file lookup).
	status, _, _ = server.get(t, "/api/v1/sets/does_not_exist/insights")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown feed, got %d", status)
	}

	status, _, _ = server.get(t, "/api/v1/does-not-exist")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown API route, got %d", status)
	}

	status, _, body = server.get(t, "/world/countries-110m.json")
	if status != http.StatusOK {
		t.Fatalf("expected 200 for embedded world topology, got %d", status)
	}
	if len(body) == 0 {
		t.Fatalf("expected non-empty world topology body")
	}
	var topology struct {
		Type string `json:"type"`
	}
	decodeTestJSONInto(t, body, &topology)
	if topology.Type != "Topology" {
		t.Fatalf("world topology type = %q, want Topology", topology.Type)
	}

	status, _, _ = server.get(t, "/missing-artifact.json")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for missing root-level artifact, got %d", status)
	}
}

func containsPublicFeedName(feeds []struct {
	Name string `json:"name"`
}, name string) bool {
	for _, feed := range feeds {
		if feed.Name == name {
			return true
		}
	}
	return false
}

func containsSearchMatchName(matches []struct {
	Name string `json:"name"`
}, name string) bool {
	for _, match := range matches {
		if match.Name == name {
			return true
		}
	}
	return false
}

func TestTopLevelArtifactsAreServedFromConfiguredWebDir(t *testing.T) {
	eng, handler := testHandler(t, Options{EnableAll: true})
	const want = "User-agent: *\nDisallow: /custom-only\n"
	if err := os.WriteFile(filepath.Join(eng.Runtime().WebDir, "robots.txt"), []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}

	server := newWebHTTPTestServer(t, handler)
	status, _, body := server.get(t, "/robots.txt")
	if status != http.StatusOK {
		t.Fatalf("robots status = %d, want 200 body=%s", status, body)
	}
	if string(body) != want {
		t.Fatalf("robots body = %q, want configured WebDir body %q", body, want)
	}
}
