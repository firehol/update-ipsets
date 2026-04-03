package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

// testHandlerWithCatalog spins up an engine that has both a regular
// ipset source and a hidden synthetic source so the unification tests
// can exercise the hidden filter without needing the full FireHOL catalog.
func testHandlerWithCatalog(t *testing.T, opts Options) (*engine.Engine, http.Handler) {
	t.Helper()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n5.6.7.0/30\n"))
	}))
	t.Cleanup(sourceServer.Close)

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
  rfc_reserved:
    url: internal://rfc_reserved
    frequency: 0
    ipv: ipv4
    output: net
    processor:
      - remove_comments
    processor_raw: remove_comments
    category: unroutable
    info: synthetic baseline
    maintainer: FireHOL
    maintainer_url: https://iplists.firehol.org/
    hidden: true
    use: [bogons]
    format: rfc_reserved_baseline
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceServer.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
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
	return eng, newHandler(eng, opts, runner)
}

// TestAdminFeedsExposesRfcReservedSyntheticSource verifies that the
// hidden synthetic rfc_reserved source is visible in the admin feeds
// list (operators need to monitor it) even though it never appears in
// the public catalog.
func TestAdminFeedsExposesRfcReservedSyntheticSource(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")

	_, handler := testHandlerWithCatalog(t, Options{EnableAll: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/feeds", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin feeds status: got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)

	var feeds []map[string]any
	if err := json.Unmarshal(body, &feeds); err != nil {
		t.Fatalf("decode admin feeds: %v", err)
	}

	var rfc map[string]any
	for _, f := range feeds {
		if name, _ := f["name"].(string); name == "rfc_reserved" {
			rfc = f
			break
		}
	}
	if rfc == nil {
		t.Fatal("rfc_reserved source missing from admin feeds list")
	}
	if hidden, _ := rfc["hidden"].(bool); !hidden {
		t.Errorf("rfc_reserved should report hidden=true, got %v", rfc["hidden"])
	}
	if kind, _ := rfc["kind"].(string); kind != "bogon" {
		t.Errorf("rfc_reserved kind = %q, want %q", kind, "bogon")
	}
	uses, _ := rfc["uses"].([]any)
	if len(uses) != 1 || uses[0] != "bogons" {
		t.Errorf("rfc_reserved uses = %v, want [bogons]", uses)
	}
}

// TestPublicCatalogEndpointHidesRfcReserved verifies the public set
// catalog excludes hidden sources. The frontend never sees a
// rfc_reserved entry because the public Entry endpoint returns 404
// even when the source has been fully processed and a cache entry
// exists. The admin endpoints can still see it.
func TestPublicCatalogEndpointHidesRfcReserved(t *testing.T) {
	eng, handler := testHandlerWithCatalog(t, Options{EnableAll: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets/rfc_reserved", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("public set endpoint status = %d, want 404 for hidden source", rec.Code)
	}

	// /api/v1/sets returns the cache snapshot. Hidden sources must
	// be stripped from the list so the public catalog stays clean.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public sets list status = %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	var entries []map[string]any
	if err := json.Unmarshal(body, &entries); err != nil {
		t.Fatalf("decode public sets: %v", err)
	}
	for _, e := range entries {
		if name, _ := e["name"].(string); name == "rfc_reserved" {
			t.Fatal("rfc_reserved leaked into public sets list")
		}
	}

	indexJSON, err := os.ReadFile(filepath.Join(eng.Runtime().WebDir, "index.json"))
	if err != nil {
		t.Fatalf("read index.json: %v", err)
	}
	if strings.Contains(string(indexJSON), `"name": "rfc_reserved"`) {
		t.Fatal("rfc_reserved leaked into published index.json")
	}
	sitemapXML, err := os.ReadFile(filepath.Join(eng.Runtime().WebDir, "sitemap.xml"))
	if err != nil {
		t.Fatalf("read sitemap.xml: %v", err)
	}
	if strings.Contains(string(sitemapXML), "/rfc_reserved") {
		t.Fatal("rfc_reserved leaked into published sitemap.xml")
	}
	sitemapFiles, err := filepath.Glob(filepath.Join(eng.Runtime().WebDir, "sitemap*.xml"))
	if err != nil {
		t.Fatalf("glob sitemap files: %v", err)
	}
	for _, path := range sitemapFiles {
		sitemapXML, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(sitemapXML), "/rfc_reserved") {
			t.Fatalf("rfc_reserved leaked into published sitemap shard %s", path)
		}
	}

	if err := os.WriteFile(filepath.Join(eng.Runtime().WebDir, "rfc_reserved.json"), []byte(`{"name":"rfc_reserved"}`), 0o644); err != nil {
		t.Fatalf("seed stale hidden metadata: %v", err)
	}

	for _, path := range []string{
		"/api/v1/sets/rfc_reserved/history",
		"/rfc_reserved.json",
		"/rfc_reserved.netset",
		"/files/rfc_reserved.netset",
	} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404 for hidden public artifact, got %d", path, rec.Code)
		}
	}
}

func TestPublicCompareExcludesHiddenPeers(t *testing.T) {
	_, handler := testHandlerWithCatalog(t, Options{EnableAll: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/compare", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public compare status = %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Result().Body)
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode compare rows: %v", err)
	}
	for _, row := range rows {
		if name, _ := row["name"].(string); name == "rfc_reserved" {
			t.Fatal("hidden feed leaked into public compare rows")
		}
	}
}

func testHandlerWithProviderCatalog(t *testing.T, opts Options) (*engine.Engine, http.Handler) {
	t.Helper()

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
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: public sample feed
    maintainer: test
    maintainer_url: https://example.test
  geodb:
    url: https://example.test/geodb.csv
    frequency: 1440
    use: [geoip]
    format: dbip_country_csv
    category: geolocation
    info: geo database
    maintainer: Geo Maintainer
    maintainer_url: https://example.test/geo
  asndb:
    url: https://example.test/asn.tsv
    frequency: 1440
    use: [asn]
    format: iptoasn_combined_tsv
    category: asn
    info: asn database
    maintainer: ASN Maintainer
    maintainer_url: https://example.test/asn
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := scheduler.New(eng, true, nil)
	return eng, newHandler(eng, opts, runner)
}

func TestPublicCatalogExcludesProviderDatasets(t *testing.T) {
	eng, handler := testHandlerWithProviderCatalog(t, Options{EnableAll: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public sets list status = %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Result().Body)
	var entries []map[string]any
	if err := json.Unmarshal(body, &entries); err != nil {
		t.Fatalf("decode public sets: %v", err)
	}

	for _, e := range entries {
		name, _ := e["name"].(string)
		switch name {
		case "geodb", "asndb":
			t.Fatalf("provider dataset %q leaked into public sets list", name)
		}
	}

	for _, name := range []string{"geodb", "asndb"} {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/"+name, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404 for provider dataset public detail, got %d", name, rec.Code)
		}
	}

	for _, name := range []string{"geodb", "asndb"} {
		if err := os.WriteFile(filepath.Join(eng.Runtime().WebDir, name+".json"), []byte(`{"name":"`+name+`"}`), 0o644); err != nil {
			t.Fatalf("%s: seed stale provider metadata: %v", name, err)
		}
		req = httptest.NewRequest(http.MethodGet, "/"+name+".json", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404 for stale provider static metadata, got %d", name, rec.Code)
		}
	}
}

func testHandlerWithArtifactCatalog(t *testing.T, opts Options) (*engine.Engine, http.Handler) {
	t.Helper()

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
artifacts:
  dronebl:
    type: dronebl_buildzone
    frequency: 60
    info: dronebl shared source
    maintainer: DroneBL
    maintainer_url: https://dronebl.org
    rsync_url: rsync://example.test/dronebl/
sources:
  dronebl_auto_botnets:
    url: artifact://dronebl?parts=auto_botnets
    frequency: 0
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: child feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := scheduler.New(eng, true, nil)
	return eng, newHandler(eng, opts, runner)
}

func TestAdminArtifactsEndpointKeepsParentsOutOfFeedsList(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")

	_, handler := testHandlerWithArtifactCatalog(t, Options{EnableAll: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/artifacts", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin artifacts status: got %d", rec.Code)
	}

	var artifacts []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &artifacts); err != nil {
		t.Fatalf("decode admin artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %#v", artifacts)
	}
	if artifacts[0]["name"] != "dronebl" {
		t.Fatalf("unexpected artifact payload: %#v", artifacts[0])
	}
	children, _ := artifacts[0]["child_feeds"].([]any)
	if len(children) != 1 || children[0] != "dronebl_auto_botnets" {
		t.Fatalf("unexpected child feeds: %#v", artifacts[0]["child_feeds"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/feeds", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin feeds status: got %d", rec.Code)
	}

	var feeds []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &feeds); err != nil {
		t.Fatalf("decode admin feeds: %v", err)
	}
	for _, feed := range feeds {
		if name, _ := feed["name"].(string); name == "dronebl" {
			t.Fatal("artifact parent leaked into admin feeds list")
		}
	}
}

func TestAdminArtifactsEndpointShowsArtifactParentState(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
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
artifacts:
  dronebl:
    type: dronebl_buildzone
    frequency: 60
    info: dronebl shared source
    maintainer: DroneBL
    maintainer_url: https://dronebl.org
    rsync_url: rsync://example.test/dronebl/
sources:
  dronebl_auto_botnets:
    url: artifact://dronebl?parts=auto_botnets
    frequency: 0
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: child feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := cache.New()
	entry := st.Entry("dronebl")
	entry.CheckedDate = 1_700_000_000
	entry.SourceDate = 1_700_000_100
	entry.DownloadFailures = 4
	entry.LastStatus = "download_failed"
	entry.LastError = "artifact too large"
	if err := cache.Save(filepath.Join(baseDir, ".cache.json"), st); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := scheduler.New(eng, true, nil)
	handler := newHandler(eng, Options{EnableAll: true}, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/artifacts", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected admin artifacts status: got %d", rec.Code)
	}

	var artifacts []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &artifacts); err != nil {
		t.Fatalf("decode admin artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %#v", artifacts)
	}
	artifact := artifacts[0]
	if got := int64(artifact["last_check"].(float64)); got != 1_700_000_000 {
		t.Fatalf("last_check = %d, want 1700000000", got)
	}
	if got := int64(artifact["last_update"].(float64)); got != 1_700_000_100 {
		t.Fatalf("last_update = %d, want 1700000100", got)
	}
	if got := int(artifact["download_failures"].(float64)); got != 4 {
		t.Fatalf("download_failures = %d, want 4", got)
	}
	if got := artifact["scheduler_detail"].(string); got == "never checked" {
		t.Fatalf("scheduler_detail = %q, want persisted artifact state to be visible", got)
	}
}
