package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestCriticalInfrastructureProvidersExposeTypedMetadata(t *testing.T) {
	cfg := config.New()
	cfg.Sources["dns_core"] = &config.Source{
		Name:            "dns_core",
		Label:           "Core public DNS",
		Format:          "text",
		Use:             []string{config.UseCriticalInfrastructure},
		Critical:        testCriticalMetadata("hard", "public_dns_core"),
		Redistributable: boolPtr(false),
		Maintainer:      "Example",
		MaintainerURL:   "https://example.test",
	}
	eng := newEngineFixture(t, withConfig(cfg))

	providers := eng.CriticalInfrastructureProviders()
	if len(providers) != 1 {
		t.Fatalf("providers = %+v, want one provider", providers)
	}
	got := providers[0]
	if got.Name != "dns_core" || got.Tier != "hard" || got.Role != "public_dns_core" {
		t.Fatalf("provider metadata = %+v, want typed hard public_dns_core provider", got)
	}
	if got.Redistributable {
		t.Fatalf("provider Redistributable = true, want false from source metadata")
	}
}

func TestCriticalProviderSetIDIsIdentityOnly(t *testing.T) {
	// The provider-set identity captures the configured catalog and per-source
	// configuration fingerprint only. It MUST NOT depend on materialized cache
	// state (ContentHash, Entries, UniqueIPs, file paths, processed timestamps,
	// version counters) because those fluctuate during normal operation and
	// would cause runtime drift between artifacts and the runtime marker.
	cfg := configWithCriticalProviderForTest("dns_core")
	base := CriticalInfrastructureProviderSetIDForSnapshot(cfg)
	if base == "" {
		t.Fatalf("provider set ID is empty for a configured critical provider")
	}
	if got := CriticalInfrastructureProviderSetIDForSnapshot(cfg); got != base {
		t.Fatalf("provider set ID is non-deterministic across reads: got %q want %q", got, base)
	}

	// Label change -> identity changes.
	relabeled := configWithCriticalProviderForTest("dns_core")
	relabeled.Sources["dns_core"].Label = "Renamed provider"
	if got := CriticalInfrastructureProviderSetIDForSnapshot(relabeled); got == base {
		t.Fatalf("provider set ID did not change after public provider metadata changed: %q", got)
	}

	// Processor change -> identity changes via SourceConfig fingerprint.
	processorChanged := configWithCriticalProviderForTest("dns_core")
	processorChanged.Sources["dns_core"].Processor = []config.ProcessorStep{{Name: "passthrough"}}
	if got := CriticalInfrastructureProviderSetIDForSnapshot(processorChanged); got == base {
		t.Fatalf("provider set ID did not change after processing config changed: %q", got)
	}

	// Downloader change -> identity changes via SourceConfig fingerprint.
	downloaderChanged := configWithCriticalProviderForTest("dns_core")
	downloaderChanged.Sources["dns_core"].Downloader = "copyfile"
	downloaderChanged.Sources["dns_core"].DownloaderOptions = "/tmp/provider.txt"
	if got := CriticalInfrastructureProviderSetIDForSnapshot(downloaderChanged); got == base {
		t.Fatalf("provider set ID did not change after downloader config changed: %q", got)
	}
}

func TestCriticalProviderSetIDIncludesASNContext(t *testing.T) {
	base := configWithCriticalProviderForTest("dns_core")
	withContext := configWithCriticalProviderForTest("dns_core")
	withContext.CriticalASNContext = []config.CriticalASNContext{{
		ASN:           36459,
		Name:          "GitHub",
		Tier:          "contextual",
		Role:          "developer_platform",
		SourceQuality: "C",
		Rationale:     "test context",
	}}

	baseID := CriticalInfrastructureProviderSetIDForSnapshot(base)
	contextID := CriticalInfrastructureProviderSetIDForSnapshot(withContext)
	if contextID == baseID {
		t.Fatalf("provider set ID did not change after critical ASN context was added: %q", contextID)
	}
	withContext.CriticalASNContext[0].Rationale = "updated rationale"
	updatedContextID := CriticalInfrastructureProviderSetIDForSnapshot(withContext)
	if updatedContextID == contextID {
		t.Fatalf("provider set ID did not reflect ASN context metadata: %q", updatedContextID)
	}
}

func TestCriticalProviderSetIDIncludesASNProviderWhenASNContextEnabled(t *testing.T) {
	// Without ASN context configured, the ASN provider catalog does not
	// participate in the identity at all.
	cfg := configWithCriticalProviderForTest("dns_core")
	cfg.Sources["iptoasn"] = &config.Source{
		Name:   "iptoasn",
		Use:    []string{config.UseASN},
		Format: "iptoasn_combined_tsv",
		Hidden: true,
	}
	withoutContextID := CriticalInfrastructureProviderSetIDForSnapshot(cfg)
	if got := CriticalInfrastructureProviderSetIDForSnapshot(cfg); got != withoutContextID {
		t.Fatalf("provider set ID is non-deterministic without ASN context: got %q want %q", got, withoutContextID)
	}

	// With ASN context configured, the ASN provider config participates via
	// its SourceConfig fingerprint (label, format, downloader, processor).
	// Materialized content of the ASN provider (entries, unique IPs) must
	// not change the identity, since identity is catalog-only.
	cfg.CriticalASNContext = []config.CriticalASNContext{{
		ASN:           36459,
		Name:          "GitHub",
		Tier:          "contextual",
		Role:          "developer_platform",
		SourceQuality: "C",
		Rationale:     "test context",
	}}
	withContextID := CriticalInfrastructureProviderSetIDForSnapshot(cfg)
	if withContextID == withoutContextID {
		t.Fatalf("provider set ID did not change when ASN context was configured: %q", withContextID)
	}
	cfg.Sources["iptoasn"].Format = "different_format"
	if got := CriticalInfrastructureProviderSetIDForSnapshot(cfg); got == withContextID {
		t.Fatalf("provider set ID did not change after ASN provider config changed with ASN context enabled: %q", got)
	}
}

func TestCriticalInfrastructureTargetRequiresConfiguredComparableSource(t *testing.T) {
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", IPV: "ipv4", Output: "netset"}
	cfg.Sources["sample_v6"] = &config.Source{Name: "sample_v6", IPV: "ipv6", Output: "netset"}
	cfg.Sources["aws_context"] = &config.Source{Name: "aws_context", IPV: "ipv4", Output: "netset", Use: []string{config.UseProviderContext}}
	cfg.Sources["critical_dns"] = &config.Source{
		Name:     "critical_dns",
		IPV:      "ipv4",
		Output:   "netset",
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
	}
	eng := newEngineFixture(t, withConfig(cfg))

	if !eng.IsCriticalInfrastructureTarget("sample") {
		t.Fatalf("sample should be a critical-infrastructure comparison target")
	}
	for _, name := range []string{"sample_v6", "aws_context", "critical_dns", "missing"} {
		if eng.IsCriticalInfrastructureTarget(name) {
			t.Fatalf("%s should not be a critical-infrastructure comparison target", name)
		}
	}
}

func configWithCriticalProviderForTest(name string) *config.Config {
	cfg := config.New()
	cfg.Sources[name] = &config.Source{
		Name:     name,
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
	}
	return cfg
}

func TestWriteCriticalInfrastructureFilesDeduplicatesAndSkipsReferenceTargets(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60, IPV: "ipv4", Output: "netset"}
	cfg.Sources["hard_a"] = &config.Source{
		Name:      "hard_a",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical:  testCriticalMetadata("hard", "public_dns_core"),
	}
	cfg.Sources["hard_b"] = &config.Source{
		Name:      "hard_b",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical:  testCriticalMetadata("hard", "dns_root"),
	}
	cfg.Sources["soft_zero"] = &config.Source{
		Name:      "soft_zero",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical:  testCriticalMetadata("soft", "cdn_edge"),
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
	}))

	writeProcessedFeedForTest(t, eng, baseDir, "sample", "sample.netset", "10.0.0.0/28\n10.0.0.16/30\n")
	writeProcessedFeedForTest(t, eng, baseDir, "hard_a", "hard_a.netset", "10.0.0.0/30\n")

	datasets := &criticalDatasets{
		Providers: map[string]*criticalProviderSet{
			"hard_a": {
				Name: "hard_a",
				Meta: criticalProviderFromSource(cfg.Sources["hard_a"]),
				Set:  mustIPSet(t, "hard_a", iprange.Range{Lo: 0x0a000000, Hi: 0x0a000003}),
			},
			"hard_b": {
				Name: "hard_b",
				Meta: criticalProviderFromSource(cfg.Sources["hard_b"]),
				Set:  mustIPSet(t, "hard_b", iprange.Range{Lo: 0x0a000002, Hi: 0x0a000005}),
			},
			"soft_zero": {
				Name: "soft_zero",
				Meta: criticalProviderFromSource(cfg.Sources["soft_zero"]),
				Set:  mustIPSet(t, "soft_zero", iprange.Range{Lo: 0xc0000201, Hi: 0xc0000201}),
			},
		},
		Names:         []string{"hard_a", "hard_b", "soft_zero"},
		Configured:    []string{"hard_a", "hard_b", "soft_zero"},
		ProviderSetID: "test-provider-set",
	}

	if err := eng.writeCriticalInfrastructureFiles(t.Context(), datasets, []string{"hard_a"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	var aggregate criticalAggregateJSON
	readJSONForTest(t, filepath.Join(webDir, "sample_critical_infrastructure.json"), &aggregate)
	if aggregate.Feed != "sample" || aggregate.Family != "ipv4" {
		t.Fatalf("aggregate identity = %+v, want sample/ipv4", aggregate)
	}
	if !aggregate.Complete || aggregate.ProviderSetID == "" || len(aggregate.ConfiguredProviders) != 3 {
		t.Fatalf("aggregate provider completeness = %+v, want complete provider set metadata", aggregate)
	}
	if aggregate.FeedIPs != 20 {
		t.Fatalf("feed_ips = %d, want 20", aggregate.FeedIPs)
	}
	if aggregate.CriticalIPs != 6 {
		t.Fatalf("critical_ips = %d, want de-duplicated hard overlap 6", aggregate.CriticalIPs)
	}
	if len(aggregate.Providers) != 2 {
		t.Fatalf("aggregate providers = %+v, want only two positive providers", aggregate.Providers)
	}
	if len(aggregate.Tiers) != 1 || aggregate.Tiers[0].Tier != "hard" || aggregate.Tiers[0].CriticalIPs != 6 || aggregate.Tiers[0].Providers != 2 {
		t.Fatalf("tier summaries = %+v, want one hard summary with de-duplicated 6 IPs and 2 providers", aggregate.Tiers)
	}
	entry := eng.state.EntrySnapshot("sample")
	if entry == nil || len(entry.CriticalOverlapTiers) != 1 || entry.CriticalOverlapTiers[0] != "hard" {
		t.Fatalf("cached critical overlap tiers = %+v, want [hard]", entry)
	}

	var provider criticalProviderOverlapJSON
	readJSONForTest(t, filepath.Join(webDir, "sample_critical_soft_zero.json"), &provider)
	if provider.CriticalIPs != 0 {
		t.Fatalf("zero provider critical_ips = %d, want zero per-provider artifact", provider.CriticalIPs)
	}
	if _, err := os.Stat(filepath.Join(webDir, "hard_a_critical_infrastructure.json")); !os.IsNotExist(err) {
		t.Fatalf("reference provider target artifact err = %v, want missing self-overlap artifact", err)
	}
}

func TestWriteCriticalInfrastructureFilesAddsASNContext(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.CriticalASNContext = []config.CriticalASNContext{{
		ASN:           36459,
		Name:          "GitHub",
		Tier:          "contextual",
		Role:          "developer_platform",
		SourceQuality: "C",
		Rationale:     "GitHub ASN context is a fallback signal in tests.",
	}}
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60, IPV: "ipv4", Output: "netset"}
	cfg.Sources["critical_dns"] = &config.Source{
		Name:      "critical_dns",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical:  testCriticalMetadata("hard", "public_dns_core"),
	}
	cfg.Sources["iptoasn"] = &config.Source{
		Name:   "iptoasn",
		Use:    []string{config.UseASN},
		Format: "iptoasn_combined_tsv",
		Hidden: true,
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
	}))

	writeProcessedFeedForTest(t, eng, baseDir, "sample", "sample.netset", "10.0.0.0/30\n203.0.113.10\n")
	writeProcessedFeedForTest(t, eng, baseDir, "critical_dns", "critical_dns.netset", "203.0.113.10\n")
	asnPayload := asnFeedJSON{
		Provider:      "iptoasn",
		FeedIPs:       5,
		AttributedIPs: 2,
		UnknownIPs:    3,
		ByASN: []asnEntryJSON{{
			ASN:     36459,
			Name:    "GITHUB",
			Count:   2,
			Percent: 40,
		}},
	}
	data, err := jsonMarshalTabIndent(asnPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "sample_asn_iptoasn.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	datasets := &criticalDatasets{
		Providers: map[string]*criticalProviderSet{
			"critical_dns": {
				Name: "critical_dns",
				Meta: criticalProviderFromSource(cfg.Sources["critical_dns"]),
				Set:  mustIPSet(t, "critical_dns", iprange.Range{Lo: 0xcb00710a, Hi: 0xcb00710a}),
			},
		},
		Names:         []string{"critical_dns"},
		Configured:    []string{"critical_dns"},
		ProviderSetID: "test-provider-set",
	}
	if err := eng.writeCriticalInfrastructureFiles(t.Context(), datasets, []string{"sample"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	var aggregate criticalAggregateJSON
	readJSONForTest(t, filepath.Join(webDir, "sample_critical_infrastructure.json"), &aggregate)
	if aggregate.ASNContext == nil {
		t.Fatalf("asn_context missing from aggregate: %+v", aggregate)
	}
	if aggregate.ASNContext.Provider != "iptoasn" || aggregate.ASNContext.IPs != 2 {
		t.Fatalf("asn_context = %+v, want iptoasn with 2 IPs", aggregate.ASNContext)
	}
	if len(aggregate.ASNContext.Matches) != 1 || aggregate.ASNContext.Matches[0].ASN != 36459 {
		t.Fatalf("asn_context matches = %+v, want AS36459", aggregate.ASNContext.Matches)
	}
}

func TestCriticalInfrastructureAggregateRecordsMissingProviders(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60, IPV: "ipv4", Output: "netset"}
	cfg.Sources["loaded"] = &config.Source{
		Name:      "loaded",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical:  testCriticalMetadata("hard", "public_dns_core"),
	}
	cfg.Sources["missing"] = &config.Source{
		Name:      "missing",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical:  testCriticalMetadata("soft", "cdn_edge"),
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
	}))
	writeProcessedFeedForTest(t, eng, baseDir, "sample", "sample.netset", "10.0.0.1\n")
	writeProcessedFeedForTest(t, eng, baseDir, "loaded", "loaded.netset", "10.0.0.1\n")

	datasets, err := eng.loadCriticalInfrastructureSources(t.Context(), eng.CriticalInfrastructureProviderSetID())
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.writeCriticalInfrastructureFiles(t.Context(), datasets, []string{"sample"}, webDir, nil); err != nil {
		t.Fatal(err)
	}

	var aggregate criticalAggregateJSON
	readJSONForTest(t, filepath.Join(webDir, "sample_critical_infrastructure.json"), &aggregate)
	if aggregate.Complete {
		t.Fatalf("aggregate complete = true, want false when configured provider is missing")
	}
	if len(aggregate.MissingProviders) != 1 || aggregate.MissingProviders[0].Name != "missing" {
		t.Fatalf("missing providers = %+v, want missing provider recorded", aggregate.MissingProviders)
	}
}

func TestStaticConfigSourceProcessesFromYAML(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	cfgPath := filepath.Join(root, "config.yaml")
	content := `
runtime:
  base_dir: "` + baseDir + `"
  history_dir: "` + filepath.Join(root, "history") + `"
  lib_dir: "` + filepath.Join(root, "lib") + `"
  errors_dir: "` + filepath.Join(root, "errors") + `"
  web_dir: "` + webDir + `"
  cache_dir: "` + filepath.Join(root, "cache") + `"
  ipsets_apply: false
sources:
  static_reference:
    static:
      - 1.1.1.1
      - 8.8.8.0/31
    frequency: 0
    ipv: ipv4
    output: netset
    processor:
      - passthrough
    category: provider_infrastructure
    info: Static test reference feed.
    maintainer: test
    maintainer_url: https://example.test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"static_reference"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected failures: %#v", report.Failed)
	}
	raw, err := os.ReadFile(eng.sourcePath("static_reference"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != "1.1.1.1\n8.8.8.0/31\n" {
		t.Fatalf("raw static body = %q, want YAML lines materialized", got)
	}
	processed, err := os.ReadFile(filepath.Join(baseDir, "static_reference.netset"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(processed); !strings.Contains(got, "1.1.1.1") || !strings.Contains(got, "8.8.8.0/31") {
		t.Fatalf("processed static netset = %q, want configured ranges", got)
	}
}

func TestContentHashOnlyForCriticalInfrastructureSources(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	content := `
runtime:
  base_dir: "` + filepath.Join(root, "base") + `"
  history_dir: "` + filepath.Join(root, "history") + `"
  lib_dir: "` + filepath.Join(root, "lib") + `"
  errors_dir: "` + filepath.Join(root, "errors") + `"
  web_dir: "` + filepath.Join(root, "web") + `"
  cache_dir: "` + filepath.Join(root, "cache") + `"
  ipsets_apply: false
sources:
  critical_reference:
    static:
      - 1.1.1.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    category: provider_infrastructure
    info: Critical static test reference feed.
    maintainer: test
    maintainer_url: https://example.test
    enabled_by_all: true
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: C
      rationale: test critical provider
  ordinary_feed:
    static:
      - 192.0.2.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    category: attacks
    info: Ordinary static test feed.
    maintainer: test
    maintainer_url: https://example.test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"critical_reference", "ordinary_feed"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected failures: %#v", report.Failed)
	}
	if got := eng.state.Entry("critical_reference").ContentHash; got == "" {
		t.Fatalf("critical reference content_hash is empty")
	}
	wantHash := eng.state.Entry("critical_reference").ContentHash
	if got := eng.state.Entry("ordinary_feed").ContentHash; got != "" {
		t.Fatalf("ordinary feed content_hash = %q, want empty", got)
	}

	eng.state.Entry("critical_reference").ContentHash = "stale-hash-from-old-cache"
	if err := cache.Save(filepath.Join(root, "base", ".cache.json"), eng.state); err != nil {
		t.Fatal(err)
	}
	reloaded, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.state.Entry("critical_reference").ContentHash; got != wantHash {
		t.Fatalf("reloaded critical reference content_hash = %q, want %q", got, wantHash)
	}
}

func TestRunOnceGeneratesCriticalInfrastructureArtifactsFromConfigSources(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	content := `
runtime:
  base_dir: "` + filepath.Join(root, "base") + `"
  history_dir: "` + filepath.Join(root, "history") + `"
  lib_dir: "` + filepath.Join(root, "lib") + `"
  errors_dir: "` + filepath.Join(root, "errors") + `"
  web_dir: "` + filepath.Join(root, "web") + `"
  cache_dir: "` + filepath.Join(root, "cache") + `"
  ipsets_apply: false
sources:
  sample:
    static:
      - 1.1.1.1
      - 203.0.113.10
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    category: attacks
    info: Ordinary test feed.
    maintainer: test
    maintainer_url: https://example.test
  critical_reference:
    static:
      - 1.1.1.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    category: provider_infrastructure
    info: Critical static test reference feed.
    maintainer: test
    maintainer_url: https://example.test
    enabled_by_all: true
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: C
      rationale: test critical provider
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"critical_reference", "sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("unexpected failures: %#v", report.Failed)
	}

	var aggregate criticalAggregateJSON
	readJSONForTest(t, filepath.Join(eng.runtime.WebDir, "sample_critical_infrastructure.json"), &aggregate)
	if aggregate.ProviderSetID == "" || aggregate.ProviderSetID != eng.CriticalInfrastructureProviderSetID() {
		t.Fatalf("aggregate provider_set_id = %q, want current %q", aggregate.ProviderSetID, eng.CriticalInfrastructureProviderSetID())
	}
	if aggregate.CriticalIPs != 1 || aggregate.FeedIPs != 2 {
		t.Fatalf("aggregate counts = critical %d feed %d, want critical 1 feed 2", aggregate.CriticalIPs, aggregate.FeedIPs)
	}
	if len(aggregate.Providers) != 1 || aggregate.Providers[0].Provider.Name != "critical_reference" || aggregate.Providers[0].CriticalIPs != 1 {
		t.Fatalf("aggregate providers = %+v, want critical_reference overlap", aggregate.Providers)
	}

	var provider criticalProviderOverlapJSON
	readJSONForTest(t, filepath.Join(eng.runtime.WebDir, "sample_critical_critical_reference.json"), &provider)
	if provider.ProviderSetID != aggregate.ProviderSetID || provider.CriticalIPs != 1 || provider.FeedIPs != 2 {
		t.Fatalf("provider payload = %+v, want current provider_set_id and 1/2 overlap", provider)
	}
}

func TestPipelineRunWritesMarkerAndArtifactsWithSameProviderSetID(t *testing.T) {
	// Contract under test (SOW-0086 Acceptance A5):
	// Within a single pipeline run, every critical-overlap artifact and the
	// runtime provider_set_id marker MUST be stamped with the same identity
	// value, captured exactly once at plan time. Re-reading engine state
	// after the heavy phase or letting any concurrent mutation alter the
	// value between artifact write and marker write is a contract violation
	// that the admin integrity check would (correctly) report as a runtime
	// regression.
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	content := `
runtime:
  base_dir: "` + filepath.Join(root, "base") + `"
  history_dir: "` + filepath.Join(root, "history") + `"
  lib_dir: "` + filepath.Join(root, "lib") + `"
  errors_dir: "` + filepath.Join(root, "errors") + `"
  web_dir: "` + filepath.Join(root, "web") + `"
  cache_dir: "` + filepath.Join(root, "cache") + `"
  ipsets_apply: false
sources:
  sample:
    static:
      - 1.1.1.1
      - 203.0.113.10
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    category: attacks
    info: Ordinary test feed.
    maintainer: test
    maintainer_url: https://example.test
  critical_reference:
    static:
      - 1.1.1.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    category: provider_infrastructure
    info: Critical static test reference feed.
    maintainer: test
    maintainer_url: https://example.test
    enabled_by_all: true
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: C
      rationale: test critical provider
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{
		Selected:   []string{"critical_reference", "sample"},
		EnableAll:  true,
		Manual:     true,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	var aggregate criticalAggregateJSON
	readJSONForTest(t, filepath.Join(eng.runtime.WebDir, "sample_critical_infrastructure.json"), &aggregate)
	if aggregate.ProviderSetID == "" {
		t.Fatalf("aggregate provider_set_id is empty")
	}
	var provider criticalProviderOverlapJSON
	readJSONForTest(t, filepath.Join(eng.runtime.WebDir, "sample_critical_critical_reference.json"), &provider)

	markerPath := CriticalInfrastructureProviderSetMarkerPath(eng.runtime)
	markerBytes, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read provider_set_id marker: %v", err)
	}
	marker := strings.TrimSpace(string(markerBytes))

	if marker != aggregate.ProviderSetID {
		t.Fatalf("marker = %q, aggregate provider_set_id = %q, want equal", marker, aggregate.ProviderSetID)
	}
	if marker != provider.ProviderSetID {
		t.Fatalf("marker = %q, per-provider provider_set_id = %q, want equal", marker, provider.ProviderSetID)
	}
	if marker != CriticalInfrastructureProviderSetIDForSnapshot(eng.cfg) {
		t.Fatalf("marker = %q does not match identity recomputed from current config", marker)
	}

	// An admin integrity sweep right after the run must be clean for these
	// artifacts: the single-snapshot contract ensures both the aggregate
	// and the per-provider artifact carry the marker's ID.
	for _, finding := range eng.CheckIntegrity() {
		for _, file := range finding.MalformedFiles {
			if strings.Contains(file, "_critical_") {
				t.Fatalf("integrity flagged %s as malformed after a clean pipeline run: %+v", file, finding)
			}
		}
	}
}

func TestTargetFeedsForFanOutCriticalProviderUpdateForcesFullFanOut(t *testing.T) {
	cfg := newTestConfigForFanOut()
	cfg.Sources["critical_dns"] = &config.Source{
		Name:      "critical_dns",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseCriticalInfrastructure},
		Critical:  testCriticalMetadata("hard", "public_dns_core"),
	}

	got := targetFeedsForFanOut(cfg, []string{"critical_dns"}, []string{"feed_a", "feed_b"}, config.UseCriticalInfrastructure)
	if len(got) != 2 {
		t.Fatalf("got %v, want all output names when critical provider updates", got)
	}

	got = targetFeedsForFanOut(cfg, []string{"critical_dns"}, []string{"feed_a", "feed_b"}, config.UseGeoIP)
	if len(got) != 0 {
		t.Fatalf("got %v, want no geo fan-out for critical-provider-only update", got)
	}
}

func TestOnlyCriticalProviderSetChangedRunAllowsSchedulerReprocess(t *testing.T) {
	cfg := config.New()
	cfg.Sources["critical_dns"] = &config.Source{
		Name:     "critical_dns",
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
	}
	cfg.Sources["plain"] = &config.Source{Name: "plain"}
	eng := newEngineFixture(t, withConfig(cfg))

	if !eng.onlyCriticalProviderSetChangedRun(true, []string{"critical_dns"}, false, RunOptions{
		Selected:  []string{"critical_dns"},
		Reprocess: true,
	}) {
		t.Fatalf("critical provider scheduled reprocess should use critical-only heavy path")
	}
	if eng.onlyCriticalProviderSetChangedRun(true, []string{"plain"}, false, RunOptions{
		Selected:  []string{"plain"},
		Reprocess: true,
	}) {
		t.Fatalf("non-critical feed update should not use critical-only heavy path")
	}
	if eng.onlyCriticalProviderSetChangedRun(true, []string{"critical_dns"}, false, RunOptions{Reprocess: true}) {
		t.Fatalf("global reprocess should not use critical-only heavy path")
	}
}

func TestCriticalTargetNamesSkipsReferenceAndIPv6Targets(t *testing.T) {
	cfg := config.New()
	cfg.Sources["plain_v4"] = &config.Source{Name: "plain_v4", IPV: "ipv4", Output: "netset"}
	cfg.Sources["plain_v6"] = &config.Source{Name: "plain_v6", IPV: "ipv6", Output: "netset"}
	cfg.Sources["critical_ref"] = &config.Source{
		Name:     "critical_ref",
		IPV:      "ipv4",
		Output:   "netset",
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
	}

	got := criticalTargetNames(cfg, []string{"plain_v4", "plain_v6", "critical_ref"})
	if len(got) != 1 || got[0] != "plain_v4" {
		t.Fatalf("critical target names = %v, want only plain_v4", got)
	}
}

func TestMarkStaleCriticalInfrastructureArtifactDeletesRemovedProviders(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", IPV: "ipv4", Output: "netset"}
	cfg.Sources["data_shield"] = &config.Source{Name: "data_shield", IPV: "ipv4", Output: "netset"}
	cfg.Sources["data_shield_critical"] = &config.Source{Name: "data_shield_critical", IPV: "ipv4", Output: "netset"}
	cfg.Sources["orphan_critical_infrastructure"] = &config.Source{Name: "orphan_critical_infrastructure", IPV: "ipv4", Output: "netset"}
	cfg.Sources["orphan_critical_critical_dns"] = &config.Source{Name: "orphan_critical_critical_dns", IPV: "ipv4", Output: "netset"}
	cfg.Sources["critical_dns"] = &config.Source{
		Name:     "critical_dns",
		IPV:      "ipv4",
		Output:   "netset",
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = filepath.Join(root, "base")
		rt.WebDir = webDir
	}))
	if err := os.MkdirAll(eng.runtime.BaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeProcessedFeedForTest(t, eng, eng.runtime.BaseDir, "sample", "sample.netset", "10.0.0.1\n")
	writeProcessedFeedForTest(t, eng, eng.runtime.BaseDir, "data_shield", "data_shield.netset", "10.0.0.5\n")
	writeProcessedFeedForTest(t, eng, eng.runtime.BaseDir, "data_shield_critical", "data_shield_critical.netset", "10.0.0.2\n")
	writeProcessedFeedForTest(t, eng, eng.runtime.BaseDir, "orphan_critical_infrastructure", "orphan_critical_infrastructure.netset", "10.0.0.3\n")
	writeProcessedFeedForTest(t, eng, eng.runtime.BaseDir, "orphan_critical_critical_dns", "orphan_critical_critical_dns.netset", "10.0.0.4\n")

	keep := filepath.Join(webDir, "sample_critical_critical_dns.json")
	keepShorterFeedWithCriticalPrefixProvider := filepath.Join(webDir, "data_shield_critical_critical_dns.json")
	keepCriticalNamedFeed := filepath.Join(webDir, "data_shield_critical_critical_critical_dns.json")
	keepExactAggregateLookingFeed := filepath.Join(webDir, "orphan_critical_infrastructure.json")
	keepExactProviderLookingFeed := filepath.Join(webDir, "orphan_critical_critical_dns.json")
	keepCriticalNamedASN := filepath.Join(webDir, "data_shield_critical_asn_caida_prefix2as.json")
	keepCriticalNamedBogons := filepath.Join(webDir, "data_shield_critical_bogons_fullbogons.json")
	keepCriticalNamedGeo := filepath.Join(webDir, "data_shield_critical_dbip_country.json")
	keepCriticalNamedRetention := filepath.Join(webDir, "data_shield_critical_retention.json")
	removeProvider := filepath.Join(webDir, "sample_critical_removed_provider.json")
	removeCriticalNamedProvider := filepath.Join(webDir, "data_shield_critical_critical_removed_provider.json")
	removeFeed := filepath.Join(webDir, "old_feed_critical_infrastructure.json")
	for _, path := range []string{keep, keepShorterFeedWithCriticalPrefixProvider, keepCriticalNamedFeed, keepExactAggregateLookingFeed, keepExactProviderLookingFeed, keepCriticalNamedASN, keepCriticalNamedBogons, keepCriticalNamedGeo, keepCriticalNamedRetention, removeProvider, removeCriticalNamedProvider, removeFeed} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	batch, err := eng.newWebPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer batch.cleanup()
	if err := eng.markStaleCriticalInfrastructureArtifactDeletes(batch.stagedPublishBatch); err != nil {
		t.Fatal(err)
	}
	touched, err := batch.publish()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("current provider artifact was deleted: %v", err)
	}
	if _, err := os.Stat(keepCriticalNamedFeed); err != nil {
		t.Fatalf("current provider artifact for feed ending in _critical was deleted: %v", err)
	}
	if _, err := os.Stat(keepShorterFeedWithCriticalPrefixProvider); err != nil {
		t.Fatalf("current provider artifact for shorter feed with provider starting in critical_ was deleted: %v", err)
	}
	if _, err := os.Stat(keepExactAggregateLookingFeed); err != nil {
		t.Fatalf("exact public feed ending in _critical_infrastructure was deleted: %v", err)
	}
	if _, err := os.Stat(keepExactProviderLookingFeed); err != nil {
		t.Fatalf("exact public feed ending in provider artifact suffix was deleted: %v", err)
	}
	for _, path := range []string{keepCriticalNamedASN, keepCriticalNamedBogons, keepCriticalNamedGeo, keepCriticalNamedRetention} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("normal artifact for feed ending in _critical was deleted: %s: %v", path, err)
		}
	}
	for _, path := range []string{removeProvider, removeCriticalNamedProvider, removeFeed} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected stale artifact %q to be deleted, stat err = %v", path, err)
		}
		if !containsString(touched, path) {
			t.Fatalf("deleted artifact %q not reported for publish sync: %v", path, touched)
		}
	}
}

func TestMarkStaleCriticalInfrastructureArtifactDeletesAggregatesWhenNoProvidersRemain(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", IPV: "ipv4", Output: "netset"}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = filepath.Join(root, "base")
		rt.WebDir = webDir
	}))
	if err := os.MkdirAll(eng.runtime.BaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeProcessedFeedForTest(t, eng, eng.runtime.BaseDir, "sample", "sample.netset", "10.0.0.1\n")
	aggregate := filepath.Join(webDir, "sample_critical_infrastructure.json")
	provider := filepath.Join(webDir, "sample_critical_removed_provider.json")
	for _, path := range []string{aggregate, provider} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	batch, err := eng.newWebPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer batch.cleanup()
	if err := eng.markStaleCriticalInfrastructureArtifactDeletes(batch.stagedPublishBatch); err != nil {
		t.Fatal(err)
	}
	if _, err := batch.publish(); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{aggregate, provider} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected stale critical artifact %q to be deleted, stat err = %v", path, err)
		}
	}
}

func TestCleanupCriticalInfrastructureArtifactsIfUnconfigured(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", IPV: "ipv4", Output: "netset"}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = filepath.Join(root, "base")
		rt.LibDir = libDir
		rt.WebDir = webDir
	}))
	aggregate := filepath.Join(webDir, "sample_critical_infrastructure.json")
	provider := filepath.Join(webDir, "sample_critical_removed_provider.json")
	for _, path := range []string{aggregate, provider} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := CriticalInfrastructureProviderSetMarkerPath(eng.runtime)
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("old-provider-set\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := eng.CleanupCriticalInfrastructureArtifactsIfUnconfigured(); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{aggregate, provider, marker} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected stale critical artifact or marker %q to be removed, stat err = %v", path, err)
		}
	}
}

func TestCleanupStaleCriticalInfrastructureArtifactsRemovesNonComparableTargets(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["sample_v6"] = &config.Source{Name: "sample_v6", IPV: "ipv6", Output: "netset"}
	cfg.Sources["critical_dns"] = &config.Source{
		Name:     "critical_dns",
		IPV:      "ipv4",
		Output:   "netset",
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = filepath.Join(root, "base")
		rt.WebDir = webDir
	}))
	aggregate := filepath.Join(webDir, "sample_v6_critical_infrastructure.json")
	provider := filepath.Join(webDir, "sample_v6_critical_critical_dns.json")
	for _, path := range []string{aggregate, provider} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := eng.CleanupStaleCriticalInfrastructureArtifacts(); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{aggregate, provider} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected non-comparable critical artifact %q to be deleted, stat err = %v", path, err)
		}
	}
}

func TestReloadCleansCriticalInfrastructureArtifactsWhenProvidersRemoved(t *testing.T) {
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
    static:
      - 10.0.0.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor:
      - passthrough
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	aggregate := filepath.Join(eng.runtime.WebDir, "sample_critical_infrastructure.json")
	provider := filepath.Join(eng.runtime.WebDir, "sample_critical_removed_provider.json")
	if err := os.MkdirAll(eng.runtime.WebDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{aggregate, provider} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := CriticalInfrastructureProviderSetMarkerPath(eng.runtime)
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("old-provider-set\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{aggregate, provider, marker} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected stale critical artifact or marker %q to be removed on reload, stat err = %v", path, err)
		}
	}
}

func TestSignalSnapshotUsesCriticalInfrastructureAggregateArtifact(t *testing.T) {
	root := t.TempDir()
	stageDir := filepath.Join(root, "stage")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}

	eng := newEngineFixture(t, withConfig(configWithCriticalProviderForTest("critical_dns")), withRuntime(func(rt *Runtime) {
		rt.WebDir = webDir
		rt.LibDir = filepath.Join(root, "lib")
	}))
	eng.cfg.Sources["sample"] = &config.Source{Name: "sample", IPV: "ipv4", Output: "netset"}
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.Category = "attacks"
	entry.UniqueIPs = 1000

	body := []byte(fmt.Sprintf(`{"feed":"sample","family":"ipv4","feed_ips":1000,"critical_ips":25,"percent":2.5,"provider_set_id":%q,"tiers":[{"tier":"hard","critical_ips":1,"percent":0.1,"providers":1},{"tier":"soft","critical_ips":24,"percent":2.4,"providers":2}]}`, eng.CriticalInfrastructureProviderSetID()))
	if err := os.WriteFile(filepath.Join(stageDir, "sample_critical_infrastructure.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	snap, err := eng.buildSignalSnapshot("sample", stageDir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.InfraShare != 0.025 {
		t.Fatalf("InfraShare = %v, want 0.025 from critical aggregate artifact", snap.InfraShare)
	}
	if snap.InfraIPs != 25 {
		t.Fatalf("InfraIPs = %d, want 25 from critical aggregate artifact", snap.InfraIPs)
	}
	if len(snap.InfraTiers) != 2 || snap.InfraTiers[0].Tier != "hard" || snap.InfraTiers[0].IPs != 1 {
		t.Fatalf("InfraTiers = %+v, want hard/soft tier summaries from critical aggregate artifact", snap.InfraTiers)
	}

	stale := []byte(`{"feed":"sample","family":"ipv4","feed_ips":1000,"critical_ips":25,"percent":2.5,"provider_set_id":"stale"}`)
	if err := os.WriteFile(filepath.Join(stageDir, "sample_critical_infrastructure.json"), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	snap, err = eng.buildSignalSnapshot("sample", stageDir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.InfraShare != 0 {
		t.Fatalf("InfraShare = %v, want 0 from stale critical provider_set_id", snap.InfraShare)
	}
	if snap.InfraIPs != 0 || len(snap.InfraTiers) != 0 {
		t.Fatalf("Infra facts = %d/%+v, want zero from stale critical provider_set_id", snap.InfraIPs, snap.InfraTiers)
	}
}

func testCriticalMetadata(tier, role string) *config.CriticalMetadata {
	return &config.CriticalMetadata{
		Tier:          tier,
		Role:          role,
		SourceType:    "curated_static",
		SourceQuality: "A",
		Rationale:     "test rationale",
	}
}

func writeProcessedFeedForTest(t *testing.T, eng *Engine, baseDir, name, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(baseDir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := eng.state.Entry(name)
	entry.Name = name
	entry.File = file
	entry.Version = 1
}

func mustIPSet(t *testing.T, name string, ranges ...iprange.Range) *iprange.IPSet {
	t.Helper()
	set := iprange.New(name)
	for _, r := range ranges {
		if err := set.AddRange(r); err != nil {
			t.Fatal(err)
		}
	}
	set.Optimize()
	return set
}

func readJSONForTest(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal %s: %v\n%s", path, err, string(data))
	}
}
