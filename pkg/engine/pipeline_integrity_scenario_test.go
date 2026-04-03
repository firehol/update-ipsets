package engine

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type pipelineIntegrityScenario struct {
	t       *testing.T
	eng     *Engine
	now     time.Time
	entries map[string]map[string]struct{}
	geoRows []string
	asnRows []string
	server  *httptest.Server
}

type pipelineIntegrityStep struct {
	Name               string
	Delta              time.Duration
	Feed               string
	Add                []string
	Remove             []string
	Selected           []string
	AllowEmptySelected bool
	NoRecheck          bool
	Reprocess          bool
	DirectRun          bool
	SetGeoRows         []string
	SetASNRows         []string
	BeforeRun          func(*pipelineIntegrityScenario)
	AfterRun           func(*pipelineIntegrityScenario, *Report)
	ExpectPresent      map[string][]string
	ExpectAbsent       map[string][]string
}

func TestPipelineIntegrityScenarioCoreBranches(t *testing.T) {
	scenario := newPipelineIntegrityScenario(t)

	steps := []pipelineIntegrityStep{
		{
			Name:     "initial publish ordinary feeds and providers",
			Delta:    time.Minute,
			Selected: scenario.sourceNamesExcept("merged"),
			ExpectPresent: map[string][]string{
				"sample": {"1.1.1.1", "10.0.0.1"},
				"peer":   {"10.0.0.1"},
			},
			AfterRun: func(s *pipelineIntegrityScenario, _ *Report) {
				s.assertCountryMapped("sample", "dbip_country", "US", 1)
				s.assertASNCount("sample", "iptoasn", 13335, 1)
				s.assertCriticalOverlap("sample", 1)
			},
		},
		{
			Name:     "initial merge compose after parents",
			Delta:    time.Minute,
			Selected: []string{"merged"},
			ExpectPresent: map[string][]string{
				"merged": {"10.0.0.1"},
			},
			ExpectAbsent: map[string][]string{
				"merged": {"1.1.1.1"},
			},
		},
		{
			Name:  "ordinary feed content update",
			Delta: time.Minute,
			Feed:  "sample",
			Add:   []string{"10.0.0.2"},
			ExpectPresent: map[string][]string{
				"sample": {"10.0.0.1", "10.0.0.2"},
			},
		},
		{
			Name:  "same-body forced recheck",
			Delta: time.Minute,
			Feed:  "sample",
			AfterRun: func(_ *pipelineIntegrityScenario, report *Report) {
				assertReportContains(t, report.Updated, "sample", "forced same-body recheck should reprocess sample")
			},
		},
		{
			Name:      "same-body unforced check skips processing",
			Delta:     time.Minute,
			Feed:      "sample",
			NoRecheck: true,
			AfterRun: func(_ *pipelineIntegrityScenario, report *Report) {
				if len(report.Updated) != 0 {
					t.Fatalf("unforced same-body check updated %v, want no updates", report.Updated)
				}
			},
		},
		{
			Name:       "geolocation provider update",
			Delta:      time.Minute,
			Feed:       "dbip_country",
			SetGeoRows: []string{"1.1.1.0,1.1.1.255,AU", "10.0.0.0,10.0.0.255,CA"},
			AfterRun: func(s *pipelineIntegrityScenario, report *Report) {
				assertReportContains(t, report.Updated, "sample", "geo provider update should reprocess sample")
				s.assertCountryMapped("sample", "dbip_country", "CA", 2)
			},
		},
		{
			Name:       "ASN provider update",
			Delta:      time.Minute,
			Feed:       "iptoasn",
			SetASNRows: []string{"1.1.1.0\t1.1.1.255\t13335\tAU\tCLOUDFLARENET", "10.0.0.0\t10.0.0.255\t64500\tCA\tEXAMPLE-NET"},
			AfterRun: func(s *pipelineIntegrityScenario, report *Report) {
				assertReportContains(t, report.Updated, "sample", "ASN provider update should reprocess sample")
				s.assertASNCount("sample", "iptoasn", 64500, 2)
			},
		},
		{
			Name:  "merge exclude update",
			Delta: time.Minute,
			Feed:  "subtract",
			Add:   []string{"10.0.0.1"},
			AfterRun: func(s *pipelineIntegrityScenario, _ *Report) {
				s.applyStep(pipelineIntegrityStep{
					Name:     "recompose merge after exclude update",
					Delta:    time.Minute,
					Selected: []string{"merged"},
					ExpectAbsent: map[string][]string{
						"merged": {"1.1.1.1", "10.0.0.1"},
					},
				})
			},
		},
		{
			Name:      "scoped reprocess",
			Delta:     time.Minute,
			Feed:      "sample",
			Reprocess: true,
			AfterRun: func(_ *pipelineIntegrityScenario, report *Report) {
				assertReportContains(t, report.Updated, "sample", "scoped reprocess should update selected feed")
			},
		},
		{
			Name:               "global reprocess from committed inputs",
			Delta:              time.Minute,
			DirectRun:          true,
			Reprocess:          true,
			AllowEmptySelected: true,
			BeforeRun: func(s *pipelineIntegrityScenario) {
				s.writeTextFeedBody("anonymous", "")
				s.writeTextFeedBody("satellite", "")
			},
			AfterRun: func(_ *pipelineIntegrityScenario, report *Report) {
				assertReportContains(t, report.Updated, "sample", "global reprocess should update sample")
				assertReportContains(t, report.Updated, "peer", "global reprocess should update peer")
				assertReportContains(t, report.Updated, "merged", "global reprocess should update merge")
			},
		},
		{
			Name:      "critical provider-set marker repair branch",
			Delta:     time.Minute,
			Selected:  []string{"__no_source__"},
			DirectRun: true,
			NoRecheck: true,
			BeforeRun: func(s *pipelineIntegrityScenario) {
				path := CriticalInfrastructureProviderSetMarkerPath(s.eng.runtime)
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Fatalf("remove critical provider-set marker: %v", err)
				}
			},
			AfterRun: func(_ *pipelineIntegrityScenario, report *Report) {
				if len(report.Failed) != 0 {
					t.Fatalf("critical marker repair failed feeds: %v", report.Failed)
				}
			},
		},
	}

	for _, step := range steps {
		scenario.applyStep(step)
	}
}

func TestPipelineIntegrityScenarioBogonUpdateRefreshesEntitySidecars(t *testing.T) {
	scenario := newPipelineIntegrityScenario(t)

	steps := []pipelineIntegrityStep{
		{
			Name:     "initial publish",
			Delta:    time.Minute,
			Selected: scenario.sourceNamesExcept("merged"),
			ExpectPresent: map[string][]string{
				"sample": {"1.1.1.1", "10.0.0.1"},
			},
		},
		{
			Name:   "remove attributed IP",
			Delta:  time.Minute,
			Feed:   "sample",
			Remove: []string{"1.1.1.1"},
			ExpectPresent: map[string][]string{
				"sample": {"10.0.0.1"},
			},
			ExpectAbsent: map[string][]string{
				"sample": {"1.1.1.1"},
			},
		},
		{
			Name:  "bogon provider update touches stale unchanged sidecar",
			Delta: time.Minute,
			Feed:  "rfc_reserved",
			Add:   []string{"10.0.0.0/8"},
			BeforeRun: func(s *pipelineIntegrityScenario) {
				// Reproduce the production shape: a byte-identical feed
				// sidecar survived older than the provider-derived ASN
				// payload it must cover.
				s.backdateFeedSidecar("sample", s.now.Add(-24*time.Hour))
			},
		},
	}

	for _, step := range steps {
		scenario.applyStep(step)
	}
	scenario.assertASNUnknownSplit("sample", "iptoasn", 1, 0)
}

func newPipelineIntegrityScenario(t *testing.T) *pipelineIntegrityScenario {
	t.Helper()

	root := t.TempDir()
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	scenario := &pipelineIntegrityScenario{
		t:   t,
		now: start,
		entries: map[string]map[string]struct{}{
			"sample": {
				"1.1.1.1":  {},
				"10.0.0.1": {},
			},
			"peer": {
				"10.0.0.1": {},
			},
			"subtract": {
				"1.1.1.1": {},
			},
			"rfc_reserved": {
				"203.0.113.0/24": {},
			},
			"critical_dns": {
				"10.0.0.1": {},
			},
		},
		geoRows: []string{
			"1.1.1.0,1.1.1.255,AU",
			"10.0.0.0,10.0.0.255,US",
		},
		asnRows: []string{
			"1.1.1.0\t1.1.1.255\t13335\tAU\tCLOUDFLARENET",
		},
	}
	scenario.server = httptest.NewServer(http.HandlerFunc(scenario.serveHTTP))
	t.Cleanup(scenario.server.Close)

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
  feed_health_single_observation_grace_minutes: 100000
  feed_health_default_healthy_cadence_minutes: 100000
  feed_health_default_risky_cadence_minutes: 100001
sources:
  sample:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
  peer:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: peer feed
    maintainer: test
    maintainer_url: https://example.test
  subtract:
    url: %q
    frequency: 1
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: subtract feed
    maintainer: test
    maintainer_url: https://example.test
  rfc_reserved:
    url: %q
    frequency: 1
    hidden: true
    use: [bogons]
    ipv: ipv4
    output: netset
    processor:
      - passthrough
    category: infrastructure
    info: reserved address space
    maintainer: test
  critical_dns:
    url: %q
    frequency: 1
    hidden: true
    use: [critical_infrastructure]
    ipv: ipv4
    output: netset
    processor:
      - passthrough
    category: infrastructure
    info: critical dns
    maintainer: test
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: A
      rationale: Test critical DNS fixture.
  dbip_country:
    url: %q
    frequency: 1
    hidden: true
    use: [geoip]
    format: dbip_country_csv
    label: DB-IP Country
  iptoasn:
    url: %q
    frequency: 1
    hidden: true
    use: [asn]
    format: iptoasn_combined_tsv
    label: IPtoASN
merges:
  merged:
    label: Test merged feed
    frequency: 1
    ipv: ipv4
    output: ip
    category: attacks
    info: merged test feed
    maintainer: test
    sources: [sample, peer]
    exclude: [subtract]
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), scenario.server.URL+"/sample", scenario.server.URL+"/peer", scenario.server.URL+"/subtract", scenario.server.URL+"/rfc_reserved", scenario.server.URL+"/critical_dns", scenario.server.URL+"/dbip_country", scenario.server.URL+"/iptoasn")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	configTime := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	if err := os.Chtimes(cfgPath, configTime, configTime); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.now = func() time.Time { return scenario.now }
	scenario.eng = eng
	return scenario
}

func (s *pipelineIntegrityScenario) applyStep(step pipelineIntegrityStep) {
	s.t.Helper()

	if step.Delta > 0 {
		s.now = s.now.Add(step.Delta)
	}
	if step.Feed != "" {
		entries := s.entriesFor(step.Feed)
		for _, cidr := range step.Add {
			entries[cidr] = struct{}{}
		}
		for _, cidr := range step.Remove {
			delete(entries, cidr)
		}
	}
	if step.SetGeoRows != nil {
		s.geoRows = append([]string(nil), step.SetGeoRows...)
	}
	if step.SetASNRows != nil {
		s.asnRows = append([]string(nil), step.SetASNRows...)
	}
	if step.BeforeRun != nil {
		step.BeforeRun(s)
	}

	selected := append([]string(nil), step.Selected...)
	if len(selected) == 0 && !step.AllowEmptySelected {
		selected = []string{step.Feed}
	}
	opts := RunOptions{
		Selected:   selected,
		EnableAll:  true,
		Manual:     true,
		Recheck:    !step.NoRecheck,
		Reprocess:  step.Reprocess,
		CleanupOld: true,
	}
	var (
		report *Report
		err    error
	)
	if step.DirectRun {
		report, err = s.eng.RunOnce(s.t.Context(), opts)
	} else {
		report, err = runSchedulerStyleOnce(s.t, s.eng, opts)
	}
	if err != nil {
		s.t.Fatalf("run scheduler-style step %+v: %v", step, err)
	}
	s.settleEntityRefresh(report)
	s.assertCleanIntegrity()
	s.assertGeneratedArtifactMTimeInvariant()
	s.assertExpectedEntries(step.ExpectPresent, true)
	s.assertExpectedEntries(step.ExpectAbsent, false)
	if step.AfterRun != nil {
		step.AfterRun(s, report)
		s.assertCleanIntegrity()
		s.assertGeneratedArtifactMTimeInvariant()
	}
}

func (s *pipelineIntegrityScenario) settleEntityRefresh(report *Report) {
	s.t.Helper()

	targets := append([]string(nil), report.EntityRefreshTargets...)
	if len(targets) == 0 {
		targets = append(targets, report.Updated...)
	}
	if len(targets) == 0 {
		return
	}
	slices.Sort(targets)
	if err := s.eng.RefreshEntityArtifactsForFeedUpdates(s.t.Context(), targets, "pipeline_integrity_scenario"); err != nil {
		s.t.Fatalf("settle entity refresh for %v: %v", targets, err)
	}
}

func (s *pipelineIntegrityScenario) assertCleanIntegrity() {
	s.t.Helper()

	if findings := s.eng.CheckIntegrityWithOptions(IntegrityOptions{EnableAll: true}); len(findings) > 0 {
		s.t.Fatalf("feed-output integrity findings after step: %+v", findings)
	}
	findings, plan, err := s.eng.CheckEntityArtifactsIntegrity()
	if err != nil {
		s.t.Fatal(err)
	}
	if len(findings) > 0 || plan.hasWork() {
		s.t.Fatalf("entity integrity findings after step: findings=%+v plan=%+v", findings, plan)
	}
}

func (s *pipelineIntegrityScenario) assertGeneratedArtifactMTimeInvariant() {
	s.t.Helper()

	for name, entry := range s.eng.state.SnapshotEntries() {
		if entry.ProcessedDate <= 0 || !s.eng.isPublicFeedName(name) || !s.eng.hasMaterializedLatestSetFile(name) {
			continue
		}
		processedAt := time.Unix(entry.ProcessedDate, 0).UTC()
		for _, artifact := range s.eng.expectedSecondaryArtifacts(name) {
			path := filepath.Join(s.eng.runtime.WebDir, artifact.RelPath)
			info, err := os.Stat(path)
			if err != nil {
				s.t.Fatalf("stat generated artifact %s for %s: %v", artifact.RelPath, name, err)
			}
			if info.ModTime().UTC().Before(processedAt) {
				s.t.Fatalf("generated artifact %s for %s has mtime %s before processed timestamp %s", artifact.RelPath, name, info.ModTime().UTC(), processedAt)
			}
		}
	}
}

func (s *pipelineIntegrityScenario) backdateFeedSidecar(feed string, when time.Time) {
	s.t.Helper()

	path := filepath.Join(s.eng.entityFeedsDir(), feed+".json")
	if err := os.Chtimes(path, when, when); err != nil {
		s.t.Fatalf("backdate feed sidecar %s: %v", feed, err)
	}
}

func (s *pipelineIntegrityScenario) writeTextFeedBody(feed, body string) {
	s.t.Helper()

	path := s.eng.feedBodyPath(feed)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.t.Fatalf("mkdir for %s: %v", path, err)
	}
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		s.t.Fatalf("write feed body %s: %v", feed, err)
	}
	if err := os.Chtimes(path, s.now, s.now); err != nil {
		s.t.Fatalf("touch feed body %s: %v", feed, err)
	}
}

func (s *pipelineIntegrityScenario) assertExpectedEntries(expected map[string][]string, wantPresent bool) {
	s.t.Helper()

	for feed, ips := range expected {
		set, err := s.eng.openLatestSet(s.t.Context(), feed)
		if err != nil {
			s.t.Fatalf("open latest set for %s: %v", feed, err)
		}
		defer func() { _ = set.Close() }()
		for _, ip := range ips {
			parsed, err := iprange.ParseIPv4Token(ip)
			if err != nil {
				s.t.Fatalf("parse expected IP %q: %v", ip, err)
			}
			if got := set.Contains(parsed); got != wantPresent {
				s.t.Fatalf("feed %s contains %s = %v, want %v", feed, ip, got, wantPresent)
			}
		}
	}
}

func (s *pipelineIntegrityScenario) assertASNUnknownSplit(feed, provider string, wantBogon, wantUnknown uint64) {
	s.t.Helper()

	var payload asnFeedJSON
	loadJSONForTest(s.t, filepath.Join(s.eng.runtime.WebDir, fmt.Sprintf("%s_asn_%s.json", feed, provider)), &payload)
	if payload.BogonIPs != wantBogon || payload.UnknownIPs != wantUnknown {
		s.t.Fatalf("ASN unknown split for %s/%s bogon=%d unknown=%d, want bogon=%d unknown=%d", feed, provider, payload.BogonIPs, payload.UnknownIPs, wantBogon, wantUnknown)
	}
}

func (s *pipelineIntegrityScenario) assertCountryMapped(feed, provider, country string, want uint64) {
	s.t.Helper()

	var payload CountryComparisonPayload
	loadJSONForTest(s.t, filepath.Join(s.eng.runtime.WebDir, fmt.Sprintf("%s_%s.json", feed, provider)), &payload)
	for _, row := range payload.Countries {
		if strings.EqualFold(row.Code, country) {
			if row.Value != want {
				s.t.Fatalf("country mapping %s/%s/%s = %d, want %d", feed, provider, country, row.Value, want)
			}
			return
		}
	}
	if want != 0 {
		s.t.Fatalf("country mapping %s/%s missing %s, want %d", feed, provider, country, want)
	}
}

func (s *pipelineIntegrityScenario) assertASNCount(feed, provider string, asn uint32, want uint64) {
	s.t.Helper()

	var payload asnFeedJSON
	loadJSONForTest(s.t, filepath.Join(s.eng.runtime.WebDir, fmt.Sprintf("%s_asn_%s.json", feed, provider)), &payload)
	for _, row := range payload.ByASN {
		if row.ASN == asn {
			if row.Count != want {
				s.t.Fatalf("ASN count %s/%s/AS%d = %d, want %d", feed, provider, asn, row.Count, want)
			}
			return
		}
	}
	if want != 0 {
		s.t.Fatalf("ASN count %s/%s missing AS%d, want %d", feed, provider, asn, want)
	}
}

func (s *pipelineIntegrityScenario) assertCriticalOverlap(feed string, want uint64) {
	s.t.Helper()

	var payload criticalAggregateJSON
	loadJSONForTest(s.t, filepath.Join(s.eng.runtime.WebDir, feed+"_critical_infrastructure.json"), &payload)
	if payload.CriticalIPs != want {
		s.t.Fatalf("critical overlap %s = %d, want %d", feed, payload.CriticalIPs, want)
	}
}

func (s *pipelineIntegrityScenario) sourceNamesExcept(excluded ...string) []string {
	s.t.Helper()

	exclude := map[string]struct{}{}
	for _, name := range excluded {
		exclude[name] = struct{}{}
	}
	out := make([]string, 0, len(s.eng.Config().Sources))
	for _, name := range config.SortedSourceNames(s.eng.Config()) {
		if _, ok := exclude[name]; ok {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (s *pipelineIntegrityScenario) entriesFor(name string) map[string]struct{} {
	entries := s.entries[name]
	if entries == nil {
		entries = map[string]struct{}{}
		s.entries[name] = entries
	}
	return entries
}

func (s *pipelineIntegrityScenario) serveHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	w.Header().Set("Last-Modified", s.now.Format(http.TimeFormat))

	switch name {
	case "dbip_country":
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(gzipPayload(s.t, strings.Join(s.geoRows, "\n")+"\n"))
	case "iptoasn":
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(gzipPayload(s.t, strings.Join(s.asnRows, "\n")+"\n"))
	default:
		entries := make([]string, 0, len(s.entries[name]))
		for cidr := range s.entries[name] {
			entries = append(entries, cidr)
		}
		slices.Sort(entries)
		_, _ = fmt.Fprintln(w, strings.Join(entries, "\n"))
	}
}

func gzipPayload(t *testing.T, body string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertReportContains(t *testing.T, values []string, want, context string) {
	t.Helper()

	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%s: %v does not contain %q", context, values, want)
}
