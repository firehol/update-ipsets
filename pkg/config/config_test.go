package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessorStepRoundTrip(t *testing.T) {
	cfg := New()
	cfg.Sources["test"] = &Source{
		Name:       "test",
		Frequency:  10,
		IPV:        "ipv4",
		Output:     "ipset",
		Processor:  []ProcessorStep{{Name: "csv_column", Args: map[string]string{"index": "1"}}},
		Attributes: map[string]string{"license": "unknown"},
	}

	var buf bytes.Buffer
	if err := SaveYAML(&buf, cfg); err != nil {
		t.Fatal(err)
	}

	tmp := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadYAML(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Sources["test"].Processor[0].Name != "csv_column" {
		t.Fatalf("unexpected processor name: %#v", loaded.Sources["test"].Processor)
	}
}

func TestExtractLegacyScriptCounts(t *testing.T) {
	// Opt-in test against an external bash script. Set UPDATE_IPSETS_LEGACY_BASH
	// to the absolute path of a sbin/update-ipsets script (e.g. from a local
	// checkout of firehol/firehol). The repo itself does not depend on the
	// bash script being present anywhere.
	legacy := os.Getenv("UPDATE_IPSETS_LEGACY_BASH")
	if legacy == "" {
		t.Skip("set UPDATE_IPSETS_LEGACY_BASH to run this test")
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Skipf("UPDATE_IPSETS_LEGACY_BASH=%q not readable: %v", legacy, err)
	}

	cfg, err := ExtractLegacyScript(legacy, ExtractOptions{IncludeGeolocation: true})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(cfg.Sources), 169; got != want {
		t.Fatalf("unexpected source count: got %d want %d", got, want)
	}
	if got, want := len(cfg.Merges), 14; got != want {
		t.Fatalf("unexpected merge count: got %d want %d", got, want)
	}
	if len(cfg.Renames) == 0 {
		t.Fatal("expected renames to be extracted")
	}
	if len(cfg.Deleted) == 0 {
		t.Fatal("expected deleted ipsets to be extracted")
	}
	// After source unification the legacy bash extractor no longer
	// injects geolocation feeds — those live in the YAML catalog with
	// use:[geoip] and are sourced from there even when an operator
	// migrates from the legacy bash format.
}

func TestLoadDirectoryMergesSupplementalSources(t *testing.T) {
	dir := t.TempDir()
	first := []byte(`
sources:
  first:
    url: https://example.test/one.txt
    frequency: 10
    ipv: ipv4
    output: ip
`)
	second := []byte(`
sources:
  second:
    url: https://example.test/two.txt
    frequency: 20
    ipv: ipv4
    output: net
  subtract:
    url: https://example.test/subtract.txt
    frequency: 20
    ipv: ipv4
    output: net
merges:
  combined:
    label: Combined test merge
    frequency: 15
    ipv: ipv4
    output: ip
    use: [bogons]
    license: Test license
    attribution: Test attribution
    sources: [first, second]
    exclude: [subtract]
`)
	if err := os.WriteFile(filepath.Join(dir, "01-first.yaml"), first, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02-second.yaml"), second, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Sources["first"]; !ok {
		t.Fatal("expected first source to be loaded")
	}
	if _, ok := cfg.Sources["second"]; !ok {
		t.Fatal("expected second source to be loaded")
	}
	// After ExpandDerivatives runs during LoadYAML, merges are moved
	// into cfg.Sources as first-class entries with internal://merge
	// URLs. The legacy cfg.Merges map is empty post-expansion.
	combined, ok := cfg.Sources["combined"]
	if !ok {
		t.Fatal("expected combined merge to be loaded as a Source after expansion")
	}
	if !strings.HasPrefix(combined.URL, InternalMergeScheme) {
		t.Fatalf("combined merge should have internal://merge URL, got %q", combined.URL)
	}
	if combined.Frequency != 15 {
		t.Fatalf("combined merge should preserve frequency 15, got %d", combined.Frequency)
	}
	if combined.Label != "Combined test merge" {
		t.Fatalf("combined merge label = %q, want %q", combined.Label, "Combined test merge")
	}
	if got, want := combined.Use, []string{UseBogons}; !equalStrings(got, want) {
		t.Fatalf("combined merge use = %v, want %v", got, want)
	}
	if combined.License != "Test license" {
		t.Fatalf("combined merge license = %q, want Test license", combined.License)
	}
	if combined.Attribution != "Test attribution" {
		t.Fatalf("combined merge attribution = %q, want Test attribution", combined.Attribution)
	}
	if got, want := combined.MergeSources, []string{"first", "second"}; !equalStrings(got, want) {
		t.Fatalf("combined merge sources = %v, want %v", got, want)
	}
	if got, want := combined.MergeExclude, []string{"subtract"}; !equalStrings(got, want) {
		t.Fatalf("combined merge exclude = %v, want %v", got, want)
	}
	if got, want := combined.DerivedFrom, []string{"first", "second", "subtract"}; !equalStrings(got, want) {
		t.Fatalf("combined derived_from = %v, want %v", got, want)
	}
	inputs, exclude, err := ParseMergeURLParts(combined.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inputs, []string{"first", "second"}; !equalStrings(got, want) {
		t.Fatalf("merge URL inputs = %v, want %v", got, want)
	}
	if got, want := exclude, []string{"subtract"}; !equalStrings(got, want) {
		t.Fatalf("merge URL exclude = %v, want %v", got, want)
	}
}

func TestLoadDirectoryPropagatesCriticalMetadataOnMerge(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`
sources:
  left:
    url: https://example.test/left.txt
    frequency: 10
    ipv: ipv4
    output: netset
  right:
    url: https://example.test/right.txt
    frequency: 10
    ipv: ipv4
    output: netset
merges:
  critical_public_dns_core:
    frequency: 15
    ipv: ipv4
    output: netset
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: A
      rationale: Official public recursive DNS resolver addresses.
    sources: [left, right]
`)
	if err := os.WriteFile(filepath.Join(dir, "critical.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	src := cfg.Sources["critical_public_dns_core"]
	if src == nil {
		t.Fatal("expected expanded critical merge source")
	}
	if !src.HasUse(UseCriticalInfrastructure) {
		t.Fatalf("expanded merge use = %v, want critical_infrastructure", src.Use)
	}
	if src.Critical == nil {
		t.Fatal("expected critical metadata to propagate to expanded merge source")
	}
	if got, want := src.Critical.Tier, "hard"; got != want {
		t.Fatalf("critical tier = %q, want %q", got, want)
	}
	if got, want := src.Critical.Role, "public_dns_core"; got != want {
		t.Fatalf("critical role = %q, want %q", got, want)
	}
	if got, want := src.Critical.SourceType, "curated_static"; got != want {
		t.Fatalf("critical source_type = %q, want %q", got, want)
	}
	if got, want := src.Critical.SourceQuality, "A"; got != want {
		t.Fatalf("critical source_quality = %q, want %q", got, want)
	}
}

func TestMergeURLPartsRoundTripWithExclude(t *testing.T) {
	url := buildMergeURL([]string{"left", "right"}, []string{"subtract"})

	inputs, exclude, err := ParseMergeURLParts(url)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inputs, []string{"left", "right"}; !equalStrings(got, want) {
		t.Fatalf("merge URL inputs = %v, want %v", got, want)
	}
	if got, want := exclude, []string{"subtract"}; !equalStrings(got, want) {
		t.Fatalf("merge URL exclude = %v, want %v", got, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestLoadLegacyReadsRuntimeAssignments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-ipsets.conf")
	content := `
BASE_DIR=/tmp/firehol-base
RUN_PARENT_DIR=/tmp/firehol-run
update sample 30 0 ipv4 ip https://example.test/feed.txt passthrough tests "sample" local https://example.test
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadLegacy(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Runtime.BaseDir, "/tmp/firehol-base"; got != want {
		t.Fatalf("unexpected base dir: got %q want %q", got, want)
	}
	if got, want := cfg.Runtime.RunParentDir, "/tmp/firehol-run"; got != want {
		t.Fatalf("unexpected run parent dir: got %q want %q", got, want)
	}
	if _, ok := cfg.Sources["sample"]; !ok {
		t.Fatal("expected legacy source to be loaded")
	}
}

func TestValidateRejectsInvalidSourceValues(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{Name: "bad", Frequency: -1, IPV: "ipv5", Output: "bogus", URL: "http://["}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for invalid source values")
	}
}

func TestValidateStaticSourceBodyContract(t *testing.T) {
	t.Run("valid static source", func(t *testing.T) {
		cfg := New()
		cfg.Sources["static_reference"] = &Source{
			Name:      "static_reference",
			Static:    []string{"1.1.1.1", "8.8.8.0/31"},
			Frequency: 0,
			IPV:       "ipv4",
			Output:    "netset",
		}
		if err := Validate(cfg); err != nil {
			t.Fatalf("expected valid static source, got %v", err)
		}
	})

	t.Run("static excludes url", func(t *testing.T) {
		cfg := New()
		cfg.Sources["static_reference"] = &Source{
			Name:      "static_reference",
			URL:       "https://example.test/feed.txt",
			Static:    []string{"1.1.1.1"},
			Frequency: 60,
			IPV:       "ipv4",
			Output:    "netset",
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "both url and static") {
			t.Fatalf("expected url/static conflict error, got %v", err)
		}
	})

	t.Run("static line must be one physical line", func(t *testing.T) {
		cfg := New()
		cfg.Sources["static_reference"] = &Source{
			Name:      "static_reference",
			Static:    []string{"1.1.1.1\n8.8.8.8"},
			Frequency: 0,
			IPV:       "ipv4",
			Output:    "netset",
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "contains a newline") {
			t.Fatalf("expected multiline static entry error, got %v", err)
		}
	})

	t.Run("static line must be canonical whitespace", func(t *testing.T) {
		cfg := New()
		cfg.Sources["static_reference"] = &Source{
			Name:      "static_reference",
			Static:    []string{" 1.1.1.1"},
			Frequency: 0,
			IPV:       "ipv4",
			Output:    "netset",
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "leading or trailing whitespace") {
			t.Fatalf("expected static whitespace error, got %v", err)
		}
	})
}

func TestExpandDerivativesClearsStaticBodyOnRetentionVariant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	content := `
sources:
  static_reference:
    static:
      - 1.1.1.1
    history: [1440]
    frequency: 0
    ipv: ipv4
    output: netset
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	variant := cfg.Sources["static_reference_1d"]
	if variant == nil {
		t.Fatal("expected static_reference_1d retention variant")
	}
	if len(variant.Static) != 0 {
		t.Fatalf("retention variant static body = %v, want empty", variant.Static)
	}
	if variant.URL == "" || !strings.HasPrefix(variant.URL, "internal://retention_window") {
		t.Fatalf("retention variant url = %q, want internal retention URL", variant.URL)
	}
}

func TestValidateRejectsLegacyInfrastructureASNs(t *testing.T) {
	cfg := New()
	cfg.InfrastructureASNs = []InfrastructureASN{{
		ASN:      13335,
		Name:     "Cloudflare",
		Category: "cdn",
	}}

	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "infrastructure_asns is no longer supported") {
		t.Fatalf("Validate() error = %v, want unsupported infrastructure_asns error", err)
	}
}

func TestValidateCriticalASNContextContract(t *testing.T) {
	t.Run("valid contextual entry", func(t *testing.T) {
		cfg := New()
		cfg.CriticalASNContext = []CriticalASNContext{{
			ASN:           32934,
			Name:          "Meta Platforms",
			Tier:          "contextual",
			Role:          "social_platform",
			SourceQuality: "C",
			Rationale:     "Service-owned ASN with no public cloud product; use as contextual evidence only.",
		}}
		if err := Validate(cfg); err != nil {
			t.Fatalf("expected valid critical_asn_context, got %v", err)
		}
	})

	t.Run("rejects broad customer hosting ASN", func(t *testing.T) {
		cfg := New()
		cfg.CriticalASNContext = []CriticalASNContext{{
			ASN:           16509,
			Name:          "Amazon AWS",
			Tier:          "contextual",
			Role:          "cloud_customer_hosting",
			SourceQuality: "C",
			Rationale:     "AWS is broad customer hosting.",
		}}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "AS16509 is disallowed") {
			t.Fatalf("expected disallowed broad ASN error, got %v", err)
		}
	})
}

func TestValidateDefaultProviderContract(t *testing.T) {
	cfg := New()
	cfg.Sources["iptoasn"] = &Source{Name: "iptoasn", Use: []string{UseASN}, Format: "iptoasn_combined_tsv"}
	cfg.Sources["dbip_country"] = &Source{Name: "dbip_country", Use: []string{UseGeoIP}, Format: "dbip_country_csv"}
	cfg.Defaults.ASNProvider = "iptoasn"
	cfg.Defaults.GeoProvider = "dbip_country"
	if err := Validate(cfg); err != nil {
		t.Fatalf("valid default providers rejected: %v", err)
	}

	cfg.Defaults.ASNProvider = "missing"
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "defaults.asn_provider references unknown source") {
		t.Fatalf("expected unknown default ASN provider error, got %v", err)
	}

	cfg.Defaults.ASNProvider = "dbip_country"
	err = Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "without use:[asn]") {
		t.Fatalf("expected wrong-role default ASN provider error, got %v", err)
	}
}

func TestValidateCriticalMetadataContract(t *testing.T) {
	valid := CriticalMetadata{
		Tier:          "hard",
		Role:          "dns_root",
		SourceType:    "authoritative_root_hints",
		SourceQuality: "A",
		Rationale:     "IANA root hints are authoritative root DNS service data.",
	}

	t.Run("role requires metadata", func(t *testing.T) {
		cfg := New()
		cfg.Sources["infra"] = &Source{
			Name:      "infra",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "no critical metadata") {
			t.Fatalf("expected missing critical metadata error, got %v", err)
		}
	})

	t.Run("metadata requires role", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["infra"] = &Source{
			Name:      "infra",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "critical metadata but no use") {
			t.Fatalf("expected role mismatch error, got %v", err)
		}
	})

	t.Run("invalid enum", func(t *testing.T) {
		cfg := New()
		meta := valid
		meta.Tier = "urgent"
		cfg.Sources["infra"] = &Source{
			Name:      "infra",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "invalid critical tier") {
			t.Fatalf("expected invalid critical tier error, got %v", err)
		}
	})

	t.Run("valid source", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["infra"] = &Source{
			Name:      "infra",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		if err := Validate(cfg); err != nil {
			t.Fatalf("expected valid critical metadata, got %v", err)
		}
	})

	t.Run("reserved provider route name", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["providers"] = &Source{
			Name:      "providers",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "reserved critical infrastructure provider name") {
			t.Fatalf("expected reserved provider name error, got %v", err)
		}
	})

	t.Run("reserved aggregate provider artifact name", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["infrastructure"] = &Source{
			Name:      "infrastructure",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "reserved critical infrastructure provider name") {
			t.Fatalf("expected reserved provider artifact name error, got %v", err)
		}
	})

	t.Run("public feed cannot collide with generated aggregate artifact", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["sample"] = &Source{Name: "sample", Frequency: 1, IPV: "ipv4", Output: "netset"}
		cfg.Sources["sample_critical_infrastructure"] = &Source{Name: "sample_critical_infrastructure", Frequency: 1, IPV: "ipv4", Output: "netset"}
		cfg.Sources["critical_dns"] = &Source{
			Name:      "critical_dns",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "collides with generated critical infrastructure aggregate artifact") {
			t.Fatalf("expected generated aggregate collision error, got %v", err)
		}
	})

	t.Run("public feed cannot collide with generated provider artifact", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["sample"] = &Source{Name: "sample", Frequency: 1, IPV: "ipv4", Output: "netset"}
		cfg.Sources["sample_critical_critical_dns"] = &Source{Name: "sample_critical_critical_dns", Frequency: 1, IPV: "ipv4", Output: "netset"}
		cfg.Sources["critical_dns"] = &Source{
			Name:      "critical_dns",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "collides with generated critical infrastructure provider artifact") {
			t.Fatalf("expected generated provider collision error, got %v", err)
		}
	})

	t.Run("critical source is ipv4 only in v1", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["infra6"] = &Source{
			Name:      "infra6",
			Frequency: 1,
			IPV:       "ipv6",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "IPv4-only") {
			t.Fatalf("expected IPv4-only critical metadata error, got %v", err)
		}
	})

	t.Run("critical source cannot also be bogons", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["infra_bogon"] = &Source{
			Name:      "infra_bogon",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure, UseBogons},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "with use:[bogons]") {
			t.Fatalf("expected critical/bogons role conflict error, got %v", err)
		}
	})

	t.Run("critical source cannot also be provider context", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["infra_context"] = &Source{
			Name:      "infra_context",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure, UseProviderContext},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "with use:[provider_context]") {
			t.Fatalf("expected critical/provider_context role conflict error, got %v", err)
		}
	})

	t.Run("critical source cannot be provider database", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["infra_asn"] = &Source{
			Name:      "infra_asn",
			Frequency: 1,
			Use:       []string{UseCriticalInfrastructure, UseASN},
			Format:    "iptoasn_csv",
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "provider database role") {
			t.Fatalf("expected provider database role error, got %v", err)
		}
	})

	t.Run("critical static body validates IP syntax", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["critical_static"] = &Source{
			Name:      "critical_static",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Static:    []string{"1.1.1.1/33"},
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "invalid critical infrastructure IP/CIDR") {
			t.Fatalf("expected critical static IP/CIDR validation error, got %v", err)
		}
	})

	t.Run("critical source cannot define history windows", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["critical_static"] = &Source{
			Name:      "critical_static",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			History:   []int{1440},
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "with history windows") {
			t.Fatalf("expected critical history-window validation error, got %v", err)
		}
	})

	t.Run("valid cloud proxy plain text source", func(t *testing.T) {
		cfg := New()
		meta := valid
		meta.Role = "cloud_proxy"
		meta.SourceType = "authoritative_plain_text"
		cfg.Sources["zscaler_reference"] = &Source{
			Name:      "zscaler_reference",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure},
			Critical:  &meta,
		}
		if err := Validate(cfg); err != nil {
			t.Fatalf("expected cloud_proxy authoritative_plain_text metadata to validate, got %v", err)
		}
	})

	t.Run("critical merge cannot also be bogons", func(t *testing.T) {
		cfg := New()
		meta := valid
		cfg.Sources["a"] = &Source{Name: "a", Frequency: 1, IPV: "ipv4", Output: "netset"}
		cfg.Merges["infra_merge"] = &Merge{
			Sources:   []string{"a"},
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseCriticalInfrastructure, UseBogons},
			Critical:  &meta,
		}
		err := Validate(cfg)
		if err == nil || !strings.Contains(err.Error(), "with use:[bogons]") {
			t.Fatalf("expected critical/bogons merge role conflict error, got %v", err)
		}
	})

	t.Run("provider context source is an ordinary published ipset", func(t *testing.T) {
		cfg := New()
		cfg.Sources["cloud_context"] = &Source{
			Name:      "cloud_context",
			Frequency: 1,
			IPV:       "ipv4",
			Output:    "netset",
			Use:       []string{UseProviderContext},
		}
		if err := Validate(cfg); err != nil {
			t.Fatalf("expected valid provider_context source, got %v", err)
		}
	})
}

func TestLoadYAMLRejectsHistoryOnProviderWithoutFeedBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	content := `
sources:
  geo_db:
    url: https://example.test/geo.mmdb
    frequency: 1440
    use: [geoip]
    format: maxmind_country_csv
    history: [1440]
    info: provider
    maintainer: test
    maintainer_url: https://example.test
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadYAML(path)
	if err == nil {
		t.Fatal("expected history-on-provider expansion error")
	}
	if !strings.Contains(err.Error(), "does not produce a feed body") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFireholCatalogHasExpectedCatalogShape(t *testing.T) {
	cfg := loadCatalog(t)
	// Post-ExpandDerivatives and synthetic-source injection:
	// 341 plain sources + 67 retention variants + 13 merges +
	// 2 hidden synthetic GeoIP-derived feeds = 423.
	if got, want := len(cfg.Sources), 423; got != want {
		t.Fatalf("unexpected source count: got %d want %d", got, want)
	}
	// cfg.Merges is cleared by ExpandDerivatives; merges now live in
	// cfg.Sources as entries with internal://merge URLs. Count them
	// by URL scheme.
	mergeCount := 0
	for _, src := range cfg.Sources {
		if strings.HasPrefix(src.URL, InternalMergeScheme) {
			mergeCount++
		}
	}
	if mergeCount != 13 {
		t.Fatalf("expected 13 merge Sources, got %d", mergeCount)
	}
	if got := len(cfg.Merges); got != 0 {
		t.Fatalf("expected cfg.Merges empty after expansion, got %d", got)
	}

	// The 5 geoip databases live under cfg.Sources with use:[geoip]
	// and a known format. After unification the supported set is the
	// same — only the home of the entries changed.
	supportedGeolocation := map[string]struct{}{
		"dbip_country_csv":        {},
		"ip2location_country_zip": {},
		"ipdeny_country_tar_gz":   {},
		"ipip_country_zip":        {},
		"maxmind_country_csv":     {},
	}
	geoSources := cfg.SourcesWithUse(UseGeoIP)
	if got, want := len(geoSources), 5; got != want {
		t.Fatalf("unexpected geoip source count: got %d want %d", got, want)
	}
	for _, src := range geoSources {
		if _, ok := supportedGeolocation[src.Format]; !ok {
			t.Fatalf("geoip source %q has unsupported format %q", src.Name, src.Format)
		}
	}
}
