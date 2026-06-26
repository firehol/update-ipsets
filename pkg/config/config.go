package config

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/firehol/update-ipsets/pkg/enrichment"

	yaml "go.yaml.in/yaml/v3"
)

type Config struct {
	Runtime            RuntimeConfig                 `yaml:"runtime"`
	Defaults           DefaultProviders              `yaml:"defaults,omitempty"`
	Categories         map[string]CategoryDefinition `yaml:"categories,omitempty"`
	Artifacts          map[string]*Artifact          `yaml:"artifacts,omitempty"`
	Sources            map[string]*Source            `yaml:"sources,omitempty"`
	InfrastructureASNs []InfrastructureASN           `yaml:"infrastructure_asns,omitempty"`
	CriticalASNContext []CriticalASNContext          `yaml:"critical_asn_context,omitempty"`
	Merges             map[string]*Merge             `yaml:"merges,omitempty"`
	Renames            map[string]string             `yaml:"renames,omitempty"`
	Deleted            []string                      `yaml:"deleted,omitempty"`

	// SourceOrder records the order in which sources appeared in the
	// YAML file. Populated by the custom UnmarshalYAML on Config when
	// loading from disk. Engines that surface providers to users
	// (ASNProviders, BogonProviders, GeoProviders) iterate sources in
	// this order so the curator's YAML ordering is the single source
	// of truth for tab order. Empty when the Config was built
	// programmatically (e.g. tests) — callers fall back to alphabetical
	// in that case.
	SourceOrder []string `yaml:"-"`

	// ArtifactOrder records YAML insertion order for the top-level
	// artifacts: block, mirroring SourceOrder.
	ArtifactOrder []string `yaml:"-"`

	// RuntimeDefined records whether this YAML document explicitly had
	// a top-level runtime: block. Directory catalogs merge many small
	// fragments, so a fragment that only contains one source must not
	// overwrite the accumulated runtime defaults.
	RuntimeDefined bool `yaml:"-"`
}

// UnmarshalYAML decodes the Config and additionally records the
// insertion order of the `sources:` mapping. Without this hook the
// underlying map[string]*Source would lose YAML ordering completely
// (Go map iteration is randomised), and the public ASN/Geo/Bogon
// provider listings could not respect curator intent.
//
// The shadow is seeded from the current value of *c so that any
// defaults already populated by New()/DefaultRuntime() (most
// importantly the runtime block) are preserved when the YAML omits
// them — yaml.v3 only writes the keys it sees and leaves untouched
// fields at whatever they were before the call.
func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	type configShadow Config
	shadow := configShadow(*c)
	if err := node.Decode(&shadow); err != nil {
		return err
	}
	*c = Config(shadow)

	// Walk the document node to find the sources mapping and record
	// its key order. Documents are wrapped in a single content node;
	// mappings are alternating key/value pairs.
	mapNode := node
	if mapNode.Kind == yaml.DocumentNode && len(mapNode.Content) > 0 {
		mapNode = mapNode.Content[0]
	}
	if mapNode.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		key := mapNode.Content[i]
		val := mapNode.Content[i+1]
		switch {
		case key.Value == "runtime":
			c.RuntimeDefined = true
		case key.Value == "sources" && val.Kind == yaml.MappingNode:
			c.SourceOrder = make([]string, 0, len(val.Content)/2)
			for j := 0; j+1 < len(val.Content); j += 2 {
				c.SourceOrder = append(c.SourceOrder, val.Content[j].Value)
			}
		case key.Value == "artifacts" && val.Kind == yaml.MappingNode:
			c.ArtifactOrder = make([]string, 0, len(val.Content)/2)
			for j := 0; j+1 < len(val.Content); j += 2 {
				c.ArtifactOrder = append(c.ArtifactOrder, val.Content[j].Value)
			}
		}
	}
	return nil
}

// DefaultProviders selects the provider datasets used when the product needs
// one canonical ASN or geolocation answer. Provider source names are used here;
// labels remain source metadata.
type DefaultProviders struct {
	ASNProvider string `yaml:"asn_provider,omitempty"`
	GeoProvider string `yaml:"geo_provider,omitempty"`
}

type RuntimeConfig struct {
	BaseDir                       string `yaml:"base_dir,omitempty"`
	ConfigFile                    string `yaml:"config_file,omitempty"`
	RunParentDir                  string `yaml:"run_parent_dir,omitempty"`
	LockFile                      string `yaml:"lock_file,omitempty"`
	CacheDir                      string `yaml:"cache_dir,omitempty"`
	LibDir                        string `yaml:"lib_dir,omitempty"`
	AdminSuppliedIPSets           string `yaml:"admin_supplied_ipsets,omitempty"`
	DistributionSuppliedIPSets    string `yaml:"distribution_supplied_ipsets,omitempty"`
	UserSuppliedIPSets            string `yaml:"user_supplied_ipsets,omitempty"`
	HistoryDir                    string `yaml:"history_dir,omitempty"`
	ErrorsDir                     string `yaml:"errors_dir,omitempty"`
	TmpDir                        string `yaml:"tmp_dir,omitempty"`
	IPSetReduceFactor             int    `yaml:"ipset_reduce_factor,omitempty"`
	IPSetReduceEntries            int    `yaml:"ipset_reduce_entries,omitempty"`
	WebChartsEntries              int    `yaml:"web_charts_entries,omitempty"`
	WebArtifactCacheMaxEntries    int    `yaml:"web_artifact_cache_max_entries,omitempty"`
	WebArtifactCacheMaxBytes      int64  `yaml:"web_artifact_cache_max_bytes,omitempty"`
	WebArtifactCacheMaxFileBytes  int64  `yaml:"web_artifact_cache_max_file_bytes,omitempty"`
	PushToGit                     bool   `yaml:"push_to_git,omitempty"`
	PushToGitMerged               bool   `yaml:"push_to_git_merged,omitempty"`
	PushToGitCommitOptions        string `yaml:"push_to_git_commit_options,omitempty"`
	PushToGitPushOptions          string `yaml:"push_to_git_push_options,omitempty"`
	PushToGitWeb                  bool   `yaml:"push_to_git_web,omitempty"`
	PushToGitTimeout              int    `yaml:"push_to_git_timeout,omitempty"`
	MaxConnectTime                int    `yaml:"max_connect_time,omitempty"`
	UserAgent                     string `yaml:"user_agent,omitempty"`
	MaxDownloadTime               int    `yaml:"max_download_time,omitempty"`
	MaxDownloadSize               int64  `yaml:"max_download_size,omitempty"`
	ParallelDownloads             int    `yaml:"parallel_downloads,omitempty"`
	IgnoreRepeatingDownloadErrors int    `yaml:"ignore_repeating_download_errors,omitempty"`
	ParallelDNSQueries            int    `yaml:"parallel_dns_queries,omitempty"`
	WebDir                        string `yaml:"web_dir,omitempty"`
	WebOwner                      string `yaml:"web_owner,omitempty"`
	WebURL                        string `yaml:"web_url,omitempty"`
	PublicBaseURL                 string `yaml:"public_base_url,omitempty"`
	WebDirForIPSets               string `yaml:"web_dir_for_ipsets,omitempty"`
	LocalCopyURL                  string `yaml:"local_copy_url,omitempty"`
	GitHubChangesURL              string `yaml:"github_changes_url,omitempty"`
	GitHubSetInfo                 string `yaml:"github_setinfo,omitempty"`
	IPSetsApply                   bool   `yaml:"ipsets_apply,omitempty"`
	// Daemon resource controls — keep the daemon lightweight.
	MaxIngestWorkers          int  `yaml:"max_ingest_workers,omitempty"`
	MaxProcessingWorkers      int  `yaml:"max_processing_workers,omitempty"`
	MaxHeavyPhaseWorkers      int  `yaml:"max_heavy_phase_workers,omitempty"`
	MaxBackgroundWorkers      int  `yaml:"max_background_workers,omitempty"`
	MaxEngineLaneWorkers      int  `yaml:"max_engine_lane_workers,omitempty"`
	MinRunIntervalSeconds     int  `yaml:"min_run_interval_seconds,omitempty"`
	ProcessingIntervalMinutes int  `yaml:"processing_interval_minutes,omitempty"`
	SkipComparisonIfNoUpdates bool `yaml:"skip_comparison_if_no_updates,omitempty"`
	// Feed health thresholds used by the public/admin health
	// classifier. single_observation_grace_minutes delays
	// classification for feeds with only one observed version;
	// the default/category cadence thresholds drive the
	// ok/delayed/risky/unmaintained ladder after that.
	FeedHealthSingleObservationGraceMins int                                     `yaml:"feed_health_single_observation_grace_minutes,omitempty"`
	FeedHealthDefaultHealthyCadenceMins  int                                     `yaml:"feed_health_default_healthy_cadence_minutes,omitempty"`
	FeedHealthDefaultRiskyCadenceMins    int                                     `yaml:"feed_health_default_risky_cadence_minutes,omitempty"`
	FeedHealthArchivalThresholdMins      int                                     `yaml:"feed_health_archival_threshold_minutes,omitempty"`
	FeedHealthCategoryThresholds         map[string]FeedHealthCategoryThresholds `yaml:"feed_health_category_thresholds,omitempty"`
	TrustProxyHeaders                    bool                                    `yaml:"trust_proxy_headers,omitempty"`
	TrustCloudflareHeaders               bool                                    `yaml:"trust_cloudflare_headers,omitempty"`
}

// Source is the single unified representation of every feed the engine
// can download and process. The `use:` field selects the engine role:
// empty (or absent) is a plain ipset; "bogons", "asn", "geoip",
// "critical_infrastructure", and "provider_context" mark special roles.
// A single source can carry multiple roles where validation allows it.
//
// Fields carry meaning based on role:
//   - ipset role: IPV/Output/Processor/History/etc. apply; Format is
//     optional (defaults to text).
//   - asn and geoip roles: Format is required and selects the parser
//     (maxmind_asn_mmdb_tar_gz, dbip_country_csv, …). These sources do
//     NOT produce an `.ipset` file; they build an in-memory database.
//   - bogons role: the source participates in the bogon union used to
//     split the unknown bucket of the ASN breakdown. Only maintained
//     reference providers should carry this role; stale themed lists stay
//     plain ipsets even when their title says "bogons". Bogon sources are
//     also regular ipsets unless Hidden is true.
//   - critical_infrastructure role: the source is an exact reference feed
//     used to build per-feed operational-risk overlap artifacts.
//   - provider_context role: the source is a broad provider/customer-hosting
//     context feed. It publishes as a normal feed but is not warning truth.
type Source struct {
	Name string `yaml:"-"`
	// Label is the human-friendly display name surfaced on the public
	// website for ASN/Geo/Bogon provider tabs and tile labels. Optional;
	// when absent the bare source name is used. Treated as authoritative
	// — the frontend never substitutes its own labels.
	Label         string            `yaml:"label,omitempty"`
	URL           string            `yaml:"url,omitempty"`
	Static        []string          `yaml:"static,omitempty"`
	Frequency     int               `yaml:"frequency"`
	History       []int             `yaml:"history,omitempty"`
	IPV           string            `yaml:"ipv,omitempty"`
	Output        string            `yaml:"output,omitempty"`
	Processor     []ProcessorStep   `yaml:"processor,omitempty"`
	ProcessorRaw  string            `yaml:"processor_raw,omitempty"`
	Category      string            `yaml:"category,omitempty"`
	ProvenanceRaw string            `yaml:"provenance,omitempty"`
	Info          string            `yaml:"info,omitempty"`
	Maintainer    string            `yaml:"maintainer,omitempty"`
	MaintainerURL string            `yaml:"maintainer_url,omitempty"`
	Attributes    map[string]string `yaml:"attributes,omitempty"`
	Enrichment    *enrichment.Feed  `yaml:"enrichment,omitempty"`
	// Critical describes the tiered critical-infrastructure semantics for
	// sources that declare use:[critical_infrastructure]. It is typed instead of
	// stored in Attributes because tier/role/source mistakes drive public
	// warning behavior.
	Critical     *CriticalMetadata `yaml:"critical,omitempty"`
	EnabledByAll bool              `yaml:"enabled_by_all,omitempty"`
	// Redistributable is a tri-state: nil means "default true" — the
	// raw downloaded data may be republished in the public mirror.
	// An explicit false opts the source out for feeds whose license
	// forbids redistribution. An explicit true is allowed but
	// redundant. Always read this through IsRedistributable().
	Redistributable *bool `yaml:"redistributable,omitempty"`
	AcceptEmpty     bool  `yaml:"accept_empty,omitempty"`

	// Use selects the engine role(s) this source fills. Empty or absent
	// means "plain ipset". Known values are defined by the Use* constants
	// below. Multiple roles can be combined (e.g. [bogons] plus the
	// implicit ipset handling happens automatically).
	Use []string `yaml:"use,omitempty"`

	// Hidden excludes the source from the public catalog (all-ipsets.json)
	// and from the per-source page. The admin UI and scheduler still see
	// it. Used for synthetic sources like rfc_reserved that have no real
	// upstream to show users.
	Hidden bool `yaml:"hidden,omitempty"`

	// ExcludeFromUnmaintained suppresses age-based health states.
	// Empty and unavailable still apply. Used for feeds whose content
	// is legitimately static or changes too rarely for age-based
	// maintenance heuristics to be meaningful.
	ExcludeFromUnmaintained bool `yaml:"exclude_from_unmaintained,omitempty"`

	// Format identifies the wire format for non-default source handling
	// (asn, geoip, rfc_reserved_baseline, etc.). Artifact-backed child
	// feeds do not use Format; they use artifact:// URLs instead.
	// Required when Use includes asn or geoip.
	Format string `yaml:"format,omitempty"`

	// Downloader/DownloaderOptions override the default HTTP downloader.
	// Promoted from ASNFeed/GeolocationFeed so every source can use them.
	Downloader        string `yaml:"downloader,omitempty"`
	DownloaderOptions string `yaml:"downloader_options,omitempty"`

	// License and Attribution describe the legal terms of the data.
	// Surfaced on the public API so consumers can credit correctly and
	// tell at a glance which sources permit raw redistribution.
	License     string `yaml:"license,omitempty"`
	Attribution string `yaml:"attribution,omitempty"`

	// DerivedFrom lists the source names this source derives from. It
	// is populated exclusively by the config loader when it expands
	// curator-facing sugar (history windows declared via History,
	// merges declared in the top-level merges: block) into standalone
	// Source entries. Plain upstream-acquired sources leave this field
	// empty. Consumed by:
	//   - enable-state propagation,
	//   - admin/API lineage exposure,
	//   - cycle detection at load time.
	//
	// Curators do not write this field in YAML; the loader builds it.
	DerivedFrom []string `yaml:"-"`

	// MergeSources and MergeExclude preserve the signed composition of
	// merge-derived sources. DerivedFrom contains both lists for dependency
	// traversal; these fields tell the engine which parents add ranges and
	// which parents subtract ranges.
	MergeSources []string   `yaml:"-"`
	MergeExclude []string   `yaml:"-"`
	Provenance   Provenance `yaml:"-"`
	// HistoryWindowDays is populated only for history derivatives.
	// It records the loader's whole-day shortcut for the semantic
	// retention window of the derived feed. The downloader still
	// derives the exact history window from the internal URL when it
	// needs the authoritative duration.
	HistoryWindowDays int `yaml:"-"`

	// ArtifactParent is populated by the loader when URL uses the
	// artifact:// scheme. This is separate from DerivedFrom because
	// artifact parents are not feed sources and must not participate in
	// merge/retention dependency traversal.
	ArtifactParent string `yaml:"-"`
}

// Engine role markers used in Source.Use.
const (
	UseBogons                 = "bogons"
	UseASN                    = "asn"
	UseGeoIP                  = "geoip"
	UseCriticalInfrastructure = "critical_infrastructure"
	UseProviderContext        = "provider_context"
)

// HasUse reports whether the source declares the given engine role.
func (s *Source) HasUse(role string) bool {
	if s == nil {
		return false
	}
	for _, r := range s.Use {
		if r == role {
			return true
		}
	}
	return false
}

// IsRedistributable reports whether the raw downloaded data may be
// redistributed. Default is true — most public blocklists permit
// republication. An explicit false in the YAML opts the source out
// for feeds whose license forbids redistribution.
func (s *Source) IsRedistributable() bool {
	if s == nil {
		return false
	}
	if s.Redistributable == nil {
		return true
	}
	return *s.Redistributable
}

// boolPtr returns a pointer to b, used to populate Source.Redistributable
// from a literal bool. The redistributable field is *bool tri-state so a
// missing value means "default true".
func boolPtr(b bool) *bool { return &b }

// SourceByName returns the live Source entry for the given name, or
// nil if no such source exists. Used by the engine to look up
// config-time facts (license, attribution, …) that are not always
// stored in the runtime cache.Entry — fields populated by finalize.go
// only land in entries written after that change shipped, so callers
// must fall back to the live config to surface them on existing
// cached entries.
func (c *Config) SourceByName(name string) *Source {
	if c == nil || c.Sources == nil {
		return nil
	}
	return c.Sources[name]
}

// SourcesWithUse returns every source that declares the given engine
// role, in YAML insertion order when the config was loaded from a YAML
// file, or alphabetical order otherwise (programmatic configs in tests
// have no defined ordering). The returned slice is safe to iterate
// without further locking — it points at live config entries but the
// config is treated as read-only after load.
func (c *Config) SourcesWithUse(role string) []*Source {
	if c == nil || len(c.Sources) == 0 {
		return nil
	}
	names := c.orderedSourceNames()
	out := make([]*Source, 0, len(names))
	for _, name := range names {
		src := c.Sources[name]
		if src != nil && src.HasUse(role) {
			out = append(out, src)
		}
	}
	return out
}

// SourcesWithUseDefaultFirst returns sources for role with the configured
// default source moved to the front. The remaining providers preserve normal
// catalog order so the default is explicit without discarding curator ordering.
func (c *Config) SourcesWithUseDefaultFirst(role string) []*Source {
	sources := c.SourcesWithUse(role)
	preferred := c.DefaultProviderForRole(role)
	if preferred == "" || len(sources) < 2 {
		return sources
	}
	out := make([]*Source, 0, len(sources))
	for _, src := range sources {
		if src != nil && src.Name == preferred {
			out = append(out, src)
			break
		}
	}
	if len(out) == 0 {
		return sources
	}
	for _, src := range sources {
		if src == nil || src.Name == preferred {
			continue
		}
		out = append(out, src)
	}
	return out
}

// DefaultProviderForRole returns the configured default provider source name
// for roles that support a canonical provider selection.
func (c *Config) DefaultProviderForRole(role string) string {
	if c == nil {
		return ""
	}
	switch role {
	case UseASN:
		return strings.TrimSpace(c.Defaults.ASNProvider)
	case UseGeoIP:
		return strings.TrimSpace(c.Defaults.GeoProvider)
	default:
		return ""
	}
}

// orderedSourceNames returns every source name known to the config,
// honouring SourceOrder when present and falling back to alphabetical
// otherwise. Defensive against drift: any source missing from
// SourceOrder (added after load) is appended in alphabetical order so
// no source is silently dropped.
func (c *Config) orderedSourceNames() []string {
	if len(c.SourceOrder) == 0 {
		names := make([]string, 0, len(c.Sources))
		for name := range c.Sources {
			names = append(names, name)
		}
		slices.Sort(names)
		return names
	}
	seen := make(map[string]bool, len(c.SourceOrder))
	out := make([]string, 0, len(c.Sources))
	for _, name := range c.SourceOrder {
		if _, ok := c.Sources[name]; ok {
			out = append(out, name)
			seen[name] = true
		}
	}
	if len(seen) < len(c.Sources) {
		extra := make([]string, 0, len(c.Sources)-len(seen))
		for name := range c.Sources {
			if !seen[name] {
				extra = append(extra, name)
			}
		}
		slices.Sort(extra)
		out = append(out, extra...)
	}
	return out
}

// SortedSourceNamesWithUse returns the names of every source that
// declares the given engine role, in sorted order.
func (c *Config) SortedSourceNamesWithUse(role string) []string {
	if c == nil || len(c.Sources) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.Sources))
	for name, src := range c.Sources {
		if src != nil && src.HasUse(role) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

type Merge struct {
	Name          string           `yaml:"-"`
	Label         string           `yaml:"label,omitempty"`
	Frequency     int              `yaml:"frequency,omitempty"`
	History       []int            `yaml:"history,omitempty"`
	IPV           string           `yaml:"ipv"`
	Output        string           `yaml:"output"`
	Category      string           `yaml:"category,omitempty"`
	ProvenanceRaw string           `yaml:"provenance,omitempty"`
	Info          string           `yaml:"info,omitempty"`
	Maintainer    string           `yaml:"maintainer,omitempty"`
	MaintainerURL string           `yaml:"maintainer_url,omitempty"`
	License       string           `yaml:"license,omitempty"`
	Attribution   string           `yaml:"attribution,omitempty"`
	Enrichment    *enrichment.Feed `yaml:"enrichment,omitempty"`
	// Redistributable is a declared policy input for merge-derived sources.
	// During expansion the effective Source.Redistributable is the conservative
	// result of this flag and every transitive additive/subtractive parent.
	Redistributable *bool `yaml:"redistributable,omitempty"`
	// See Source.ExcludeFromUnmaintained. Expanded merge sources copy
	// this flag verbatim.
	ExcludeFromUnmaintained bool     `yaml:"exclude_from_unmaintained,omitempty"`
	Sources                 []string `yaml:"sources"`
	// Exclude lists subtractive inputs. The expanded merge is
	// union(Sources) minus union(Exclude).
	Exclude    []string          `yaml:"exclude,omitempty"`
	Use        []string          `yaml:"use,omitempty"`
	Critical   *CriticalMetadata `yaml:"critical,omitempty"`
	Provenance Provenance        `yaml:"-"`
}

// CriticalMetadata classifies a critical-infrastructure reference source.
// It is required when use:[critical_infrastructure] is present and is copied
// from merge declarations to their expanded Source entry.
type CriticalMetadata struct {
	Tier          string `yaml:"tier,omitempty"`
	Role          string `yaml:"role,omitempty"`
	SourceType    string `yaml:"source_type,omitempty"`
	SourceQuality string `yaml:"source_quality,omitempty"`
	Rationale     string `yaml:"rationale,omitempty"`
}

func cloneCriticalMetadata(in *CriticalMetadata) *CriticalMetadata {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

// CriticalASNContext defines the intentionally small secondary ASN signal used
// when an operator owns a service network but has no exact public IP feed.
// It is contextual evidence only; it does not replace reference-feed overlap.
type CriticalASNContext struct {
	ASN           uint32 `yaml:"asn"`
	Name          string `yaml:"name,omitempty"`
	Tier          string `yaml:"tier,omitempty"`
	Role          string `yaml:"role,omitempty"`
	SourceQuality string `yaml:"source_quality,omitempty"`
	Rationale     string `yaml:"rationale,omitempty"`
}

// InfrastructureASN is the legacy top-level infrastructure_asns entry shape.
// Supported configs reject non-empty infrastructure_asns; the struct remains so
// old YAML can be decoded and rejected with an explicit migration error.
type InfrastructureASN struct {
	ASN         uint32 `yaml:"asn"`
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Category    string `yaml:"category,omitempty"`
}

type ProcessorStep struct {
	Name string            `yaml:"name,omitempty"`
	Args map[string]string `yaml:"args,omitempty"`
}

func (p ProcessorStep) MarshalYAML() (any, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("processor step name is empty")
	}
	if len(p.Args) == 0 {
		return p.Name, nil
	}
	return map[string]any{p.Name: p.Args}, nil
}

func (p *ProcessorStep) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		p.Name = node.Value
		p.Args = nil
		return nil
	case yaml.MappingNode:
		if len(node.Content) != 2 {
			return fmt.Errorf("processor step mapping must have exactly one key")
		}
		p.Name = node.Content[0].Value
		args := map[string]string{}
		switch node.Content[1].Kind {
		case yaml.ScalarNode:
			args["value"] = node.Content[1].Value
		case yaml.MappingNode:
			for i := 0; i < len(node.Content[1].Content); i += 2 {
				args[node.Content[1].Content[i].Value] = node.Content[1].Content[i+1].Value
			}
		default:
			return fmt.Errorf("unsupported processor step value kind %d", node.Content[1].Kind)
		}
		p.Args = args
		return nil
	default:
		return fmt.Errorf("unsupported processor step node kind %d", node.Kind)
	}
}

func DefaultRuntime() RuntimeConfig {
	return RuntimeConfig{
		BaseDir:                              "/etc/firehol/ipsets",
		ConfigFile:                           "/etc/firehol/update-ipsets",
		RunParentDir:                         "/var/run",
		LockFile:                             "{run_parent_dir}/update-ipsets.lock",
		CacheDir:                             "/var/cache/update-ipsets",
		LibDir:                               "/var/lib/update-ipsets",
		AdminSuppliedIPSets:                  "/etc/firehol/ipsets.d",
		DistributionSuppliedIPSets:           "/usr/share/firehol/ipsets.d",
		UserSuppliedIPSets:                   "{HOME}/.update-ipsets/ipsets.d",
		HistoryDir:                           "{base_dir}/history",
		ErrorsDir:                            "{base_dir}/errors",
		TmpDir:                               "/tmp",
		IPSetReduceFactor:                    20,
		IPSetReduceEntries:                   65536,
		WebChartsEntries:                     500,
		WebArtifactCacheMaxEntries:           2048,
		WebArtifactCacheMaxBytes:             64 << 20,
		WebArtifactCacheMaxFileBytes:         8 << 20,
		PushToGit:                            false,
		PushToGitMerged:                      true,
		PushToGitCommitOptions:               "",
		PushToGitPushOptions:                 "",
		PushToGitWeb:                         false,
		PushToGitTimeout:                     600,
		MaxConnectTime:                       10,
		UserAgent:                            "FireHOL-Update-Ipsets/3.0 (linux-gnu) https://iplists.firehol.org/",
		MaxDownloadTime:                      300,
		ParallelDownloads:                    5,
		IgnoreRepeatingDownloadErrors:        10,
		ParallelDNSQueries:                   10,
		WebURL:                               "https://iplists.firehol.org/ipsets/",
		PublicBaseURL:                        "",
		LocalCopyURL:                         "https://iplists.firehol.org/files/",
		GitHubChangesURL:                     "https://github.com/firehol/blocklist-ipsets/commits/master/",
		GitHubSetInfo:                        "https://github.com/firehol/blocklist-ipsets/tree/master/",
		IPSetsApply:                          true,
		MaxProcessingWorkers:                 2,
		MaxHeavyPhaseWorkers:                 0,
		MaxBackgroundWorkers:                 1,
		MaxEngineLaneWorkers:                 1,
		MinRunIntervalSeconds:                30,
		ProcessingIntervalMinutes:            5,
		SkipComparisonIfNoUpdates:            true,
		FeedHealthSingleObservationGraceMins: 10 * 24 * 60,
		FeedHealthDefaultHealthyCadenceMins:  7 * 24 * 60,
		FeedHealthDefaultRiskyCadenceMins:    30 * 24 * 60,
		FeedHealthArchivalThresholdMins:      60 * 24 * 60,
		FeedHealthCategoryThresholds:         DefaultFeedHealthCategoryThresholds(),
	}
}

func New() *Config {
	return &Config{
		Runtime:    DefaultRuntime(),
		Defaults:   DefaultProviders{},
		Categories: map[string]CategoryDefinition{},
		Artifacts:  map[string]*Artifact{},
		Sources:    map[string]*Source{},
		Merges:     map[string]*Merge{},
		Renames:    map[string]string{},
	}
}

func finalizeLoadedConfig(cfg *Config) (*Config, error) {
	if err := normalizeCatalogMetadata(cfg); err != nil {
		return nil, err
	}
	normalizeFeedHealthThresholds(&cfg.Runtime)
	if err := validateMergeReferences(cfg); err != nil {
		return nil, err
	}
	// Expand curator-facing sugar (source.History windows, the
	// merges: block) into first-class Source entries. Internal URLs
	// remain a loader detail for those derived feeds, but the runtime
	// pipeline keys off provenance and derived-from metadata rather
	// than a special fetch path.
	if err := ExpandDerivatives(cfg); err != nil {
		return nil, err
	}
	if err := injectBuiltInSyntheticSources(cfg); err != nil {
		return nil, err
	}
	if err := normalizeArtifactBackedSources(cfg); err != nil {
		return nil, err
	}
	// Canonicalise Source.Output across the WHOLE expanded
	// catalog. This runs after ExpandDerivatives so merges (which
	// become Source entries with Output copied from Merge.Output)
	// also pick up the translation. See canonicalizeOutput for
	// the migration rules — the loader accepts the old "ip",
	// "net", "both" values as aliases so existing YAML deployments
	// keep loading without changes, but Validate rejects anything
	// else after this pass.
	for _, src := range cfg.Sources {
		if src == nil {
			continue
		}
		src.Output = canonicalizeOutput(src.Output)
	}
	return cfg, Validate(cfg)
}

// rejectLegacyTopLevelBlocks scans the raw YAML for the three blocks
// the source unification refactor removed (geolocation, asn, bogons)
// and returns a friendly migration error if any are present. The check
// is line-based to avoid a full second yaml.Unmarshal pass.
func rejectLegacyTopLevelBlocks(data []byte) error {
	// Top-level keys are at column 0 and end with ':'. We only fail
	// for the exact removed blocks so an inline source named e.g.
	// "asn_lookup" does not trip the check.
	deprecated := map[string]bool{
		"geolocation:": true,
		"asn:":         true,
		"bogons:":      true,
	}
	for _, line := range strings.Split(string(data), "\n") {
		// Skip indented lines and comments — we only care about
		// actual top-level keys.
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		trimmed := strings.TrimRight(line, " \t\r")
		if deprecated[trimmed] {
			return fmt.Errorf("config has removed top-level block %q; the geolocation, asn, and bogons blocks have been folded into sources with `use: [...]`. See docs/feeds/use-roles.md and docs/migration-from-bash.md", strings.TrimSuffix(trimmed, ":"))
		}
	}
	return nil
}

func SaveYAML(w io.Writer, cfg *Config) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}

func SortedSourceNames(cfg *Config) []string {
	names := make([]string, 0, len(cfg.Sources))
	for name := range cfg.Sources {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// SortedMergeNames returns the merge names in sorted order.
func SortedMergeNames(cfg *Config) []string {
	names := make([]string, 0, len(cfg.Merges))
	for name := range cfg.Merges {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (cfg *Config) Merge(other *Config) {
	if cfg == nil || other == nil {
		return
	}
	if cfg.Sources == nil {
		cfg.Sources = map[string]*Source{}
	}
	if cfg.Artifacts == nil {
		cfg.Artifacts = map[string]*Artifact{}
	}
	if cfg.Categories == nil {
		cfg.Categories = map[string]CategoryDefinition{}
	}
	if cfg.Merges == nil {
		cfg.Merges = map[string]*Merge{}
	}
	if cfg.Renames == nil {
		cfg.Renames = map[string]string{}
	}
	if other.RuntimeDefined {
		cfg.Runtime = other.Runtime
		cfg.RuntimeDefined = true
	}
	if other.Defaults.ASNProvider != "" {
		cfg.Defaults.ASNProvider = other.Defaults.ASNProvider
	}
	if other.Defaults.GeoProvider != "" {
		cfg.Defaults.GeoProvider = other.Defaults.GeoProvider
	}
	appendOrderedNames(&cfg.ArtifactOrder, cfg.Artifacts, other.ArtifactOrder, other.Artifacts)
	for name, artifact := range other.Artifacts {
		if artifact == nil {
			cfg.Artifacts[name] = nil
			continue
		}
		clone := *artifact
		clone.Name = name
		cfg.Artifacts[name] = &clone
	}
	appendOrderedNames(&cfg.SourceOrder, cfg.Sources, other.SourceOrder, other.Sources)
	for name, src := range other.Sources {
		if src == nil {
			cfg.Sources[name] = nil
			continue
		}
		clone := *src
		clone.Name = name
		clone.Enrichment = enrichment.Clone(src.Enrichment)
		cfg.Sources[name] = &clone
	}
	for name, category := range other.Categories {
		cfg.Categories[name] = category
	}
	cfg.InfrastructureASNs = append(cfg.InfrastructureASNs, other.InfrastructureASNs...)
	cfg.CriticalASNContext = append(cfg.CriticalASNContext, other.CriticalASNContext...)
	for name, merge := range other.Merges {
		if merge == nil {
			cfg.Merges[name] = nil
			continue
		}
		clone := *merge
		clone.Name = name
		clone.Enrichment = enrichment.Clone(merge.Enrichment)
		cfg.Merges[name] = &clone
	}
	for oldName, newName := range other.Renames {
		cfg.Renames[oldName] = newName
	}
	cfg.Deleted = append(cfg.Deleted, other.Deleted...)
}

func appendOrderedNames[T any](dst *[]string, existing map[string]T, preferred []string, incoming map[string]T) {
	if len(incoming) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(*dst)+len(existing))
	for _, name := range *dst {
		seen[name] = struct{}{}
	}
	for name := range existing {
		seen[name] = struct{}{}
	}
	ordered := make([]string, 0, len(incoming))
	localSeen := make(map[string]struct{}, len(incoming))
	for _, name := range preferred {
		if _, ok := incoming[name]; !ok {
			continue
		}
		if _, duplicate := localSeen[name]; duplicate {
			continue
		}
		ordered = append(ordered, name)
		localSeen[name] = struct{}{}
	}
	if len(localSeen) < len(incoming) {
		extra := make([]string, 0, len(incoming)-len(localSeen))
		for name := range incoming {
			if _, ok := localSeen[name]; ok {
				continue
			}
			extra = append(extra, name)
		}
		slices.Sort(extra)
		ordered = append(ordered, extra...)
	}
	for _, name := range ordered {
		if _, ok := seen[name]; ok {
			continue
		}
		*dst = append(*dst, name)
		seen[name] = struct{}{}
	}
}
