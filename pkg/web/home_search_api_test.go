package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/iprange"
	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestFeedScopedSearchEndpoint(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	libDir := filepath.Join(root, "lib")
	webDir := filepath.Join(root, "web")
	cfgPath := filepath.Join(root, "config.yaml")
	modified := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()
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
    frequency: 60
    history: [60]
    ipv: ipv4
    output: ip
    category: intrusion
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), libDir, filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}
	olderSeen := modified.Add(-30 * time.Minute)
	writeHistorySnapshotForTest(t, filepath.Join(root, "history"), "sample", modified.Add(-15*time.Minute), "1.2.3.4")
	writeRetentionCohortForTest(t, filepath.Join(root, "lib"), "sample", olderSeen, "1.2.3.4")
	eng, err = engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := newHandler(eng, Options{EnableAll: true}, scheduler.New(eng, true, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/search?ip=1.2.3.4", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Scope        string `json:"scope"`
		SearchedFeed string `json:"searched_feed"`
		Matches      []struct {
			Name      string `json:"name"`
			FirstSeen int64  `json:"first_seen"`
			LastSeen  int64  `json:"last_seen"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, rec.Body.String())
	}
	if payload.Scope != "feed" || payload.SearchedFeed != "sample" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if len(payload.Matches) != 1 || payload.Matches[0].Name != "sample" {
		t.Fatalf("unexpected matches: %+v", payload.Matches)
	}
	if payload.Matches[0].FirstSeen != olderSeen.Unix() || payload.Matches[0].LastSeen != modified.Unix() {
		t.Fatalf("unexpected timing payload: %+v", payload.Matches[0])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/sets/sample/search?ip=1.2.3.4&details=true", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected detailed status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var detailedPayload struct {
		Scope        string `json:"scope"`
		SearchedFeed string `json:"searched_feed"`
		Matches      []struct {
			Name string `json:"name"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detailedPayload); err != nil {
		t.Fatalf("invalid detailed JSON: %v body=%s", err, rec.Body.String())
	}
	if detailedPayload.Scope != "feed" || detailedPayload.SearchedFeed != "sample" {
		t.Fatalf("unexpected detailed payload: %+v", detailedPayload)
	}
	if len(detailedPayload.Matches) != 1 || detailedPayload.Matches[0].Name != "sample" {
		t.Fatalf("unexpected detailed matches: %+v", detailedPayload.Matches)
	}
}

func writeHistorySnapshotForTest(t *testing.T, historyDir, parent string, ts time.Time, cidrs ...string) string {
	t.Helper()

	dir := filepath.Join(historyDir, parent)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(cidrs, "\n") + "\n"
	set, err := iprange.ParseReader(t.Context(), parent, strings.NewReader(text), iprange.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse snapshot cidrs: %v", err)
	}
	set.Optimize()
	var buf bytes.Buffer
	if err := iprange.WriteBinary(&buf, set); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, strconv.FormatInt(ts.Unix(), 10)+".set")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeRetentionCohortForTest(t *testing.T, libDir, feed string, ts time.Time, cidrs ...string) string {
	t.Helper()

	dir := filepath.Join(libDir, feed, "new")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(cidrs, "\n") + "\n"
	set, err := iprange.ParseReader(t.Context(), feed, strings.NewReader(text), iprange.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse retention cohort cidrs: %v", err)
	}
	set.Optimize()
	path := filepath.Join(dir, strconv.FormatInt(ts.Unix(), 10))
	var buf bytes.Buffer
	if err := iprange.WriteBinary(&buf, set); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
	if err := writeRetentionCohortIndexForTest(filepath.Join(libDir, feed, "retention_cohorts.csv"), dir); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeRetentionCohortIndexForTest(indexPath, cohortDir string) error {
	entries, err := os.ReadDir(cohortDir)
	if err != nil {
		return err
	}
	lines := []string{"date_added,ips"}
	keys := make([]int64, 0, len(entries))
	counts := make(map[int64]uint64, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		addedAt, err := strconv.ParseInt(strings.TrimSuffix(entry.Name(), ".set"), 10, 64)
		if err != nil || addedAt <= 0 {
			continue
		}
		filePath := filepath.Join(cohortDir, entry.Name())
		fs, err := iprange.OpenFileSet(filePath)
		if err != nil {
			return err
		}
		counts[addedAt] = fs.UniqueIPs()
		_ = fs.Close()
		keys = append(keys, addedAt)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, addedAt := range keys {
		lines = append(lines, fmt.Sprintf("%d,%d", addedAt, counts[addedAt]))
	}
	return os.WriteFile(indexPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func TestHomeGlobeEndpoint(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	runtimeWebDir := filepath.Join(root, "runtime-web")
	servedWebDir := filepath.Join(root, "served-web")
	cfgPath := filepath.Join(root, "config.yaml")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
  feed_health_single_observation_grace_minutes: 60
  feed_health_default_healthy_cadence_minutes: 60
  feed_health_default_risky_cadence_minutes: 120
  feed_health_category_thresholds:
    intrusion:
      healthy_cadence_minutes: 60
      risky_cadence_minutes: 120
categories:
  intrusion:
    label: Intrusion
    description: test
sources:
  sample:
    url: %q
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    maintainer: test
    maintainer_url: https://example.test
  geolite2_country:
    url: https://example.test/geo.csv
    frequency: 1440
    use: [geoip]
    format: maxmind_country_csv
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), runtimeWebDir, filepath.Join(root, "cache"), server.URL)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtimeWebDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(servedWebDir, 0o755); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		Selected:   []string{"sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(servedWebDir, "sample_geolite2_country.json"), []byte(`{"total_mapped":10,"countries":[{"code":"US","value":10}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(servedWebDir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(servedWebDir, "home", "aggregates.json"), []byte(`{
		"version": 1,
		"providers": {"geo": {"name": "geolite2_country", "label": "GeoLite2 Country"}, "asn": {}},
		"categories": [{
			"category": "intrusion",
			"eligible_feeds": 1,
			"contributing_feeds": 1,
			"unique_ips": 1,
			"countries": [{"code": "US", "feed_count": 1, "attributed_ips": 10}]
		}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(eng, Options{EnableAll: true, WebDir: servedWebDir}, scheduler.New(eng, true, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/home/globe?categories=intrusion", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		EligibleFeeds int `json:"eligible_feeds"`
		Countries     []struct {
			Code      string `json:"code"`
			FeedCount int    `json:"feed_count"`
		} `json:"countries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, rec.Body.String())
	}
	if payload.EligibleFeeds != 1 {
		t.Fatalf("eligible feeds = %d, want 1", payload.EligibleFeeds)
	}
	if len(payload.Countries) != 1 || payload.Countries[0].Code != "US" || payload.Countries[0].FeedCount != 1 {
		t.Fatalf("unexpected countries payload: %+v", payload.Countries)
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/api/v1/home/summary?categories=intrusion", nil)
	summaryRec := httptest.NewRecorder()
	handler.ServeHTTP(summaryRec, summaryReq)
	if summaryRec.Code != http.StatusOK {
		t.Fatalf("unexpected summary status: got %d body=%s", summaryRec.Code, summaryRec.Body.String())
	}
	var summaryPayload struct {
		TopCountries []struct {
			Code      string `json:"code"`
			FeedCount int    `json:"feed_count"`
		} `json:"top_countries"`
	}
	if err := json.Unmarshal(summaryRec.Body.Bytes(), &summaryPayload); err != nil {
		t.Fatalf("invalid summary JSON: %v body=%s", err, summaryRec.Body.String())
	}
	if len(summaryPayload.TopCountries) != 1 || summaryPayload.TopCountries[0].Code != "US" || summaryPayload.TopCountries[0].FeedCount != 1 {
		t.Fatalf("unexpected summary countries payload: %+v", summaryPayload.TopCountries)
	}
}

func TestHomeEndpointsReturnNotReadyWhenAggregateMissing(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
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
categories:
  intrusion:
    label: Intrusion
    description: test
sources:
  sample:
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(eng, Options{EnableAll: true, WebDir: webDir}, scheduler.New(eng, true, nil))

	for _, path := range []string{
		"/api/v1/home/summary",
		"/api/v1/home/globe?categories=intrusion",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d body=%s, want 503", path, rec.Code, rec.Body.String())
		}
	}
}

func TestHomeEndpointsReturnNotReadyWhenAggregateMalformed(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
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
categories:
  intrusion:
    label: Intrusion
    description: test
sources:
  sample:
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ip
    category: intrusion
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	aggregatePath := filepath.Join(webDir, "home", "aggregates.json")
	if err := os.MkdirAll(filepath.Dir(aggregatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aggregatePath, []byte(`{"version":`), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(eng, Options{EnableAll: true, WebDir: webDir}, scheduler.New(eng, true, nil))

	for _, path := range []string{
		"/api/v1/home/summary",
		"/api/v1/home/globe?categories=intrusion",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d body=%s, want 503", path, rec.Code, rec.Body.String())
		}
	}
}
