package config

import "fmt"

const (
	GeoAnonymousSourceName = "anonymous"
	GeoSatelliteSourceName = "satellite"
)

func injectBuiltInSyntheticSources(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if cfg.Sources == nil {
		cfg.Sources = map[string]*Source{}
	}
	if len(geoSyntheticParents(cfg)) == 0 {
		return nil
	}
	for _, def := range builtInSyntheticSourceDefs(cfg) {
		existing := cfg.Sources[def.Name]
		if existing != nil {
			if existing.URL != def.URL {
				return fmt.Errorf("synthetic source %q collides with configured source", def.Name)
			}
			continue
		}
		cfg.Sources[def.Name] = def
		cfg.SourceOrder = append(cfg.SourceOrder, def.Name)
	}
	return nil
}

func builtInSyntheticSourceDefs(cfg *Config) []*Source {
	frequency := 5
	if cfg != nil && cfg.Runtime.ProcessingIntervalMinutes > 0 {
		frequency = cfg.Runtime.ProcessingIntervalMinutes
	}
	derived := geoSyntheticParents(cfg)
	if len(derived) == 0 {
		return nil
	}
	return []*Source{
		{
			Name:                    GeoAnonymousSourceName,
			Label:                   "Anonymous networks",
			URL:                     "internal://" + GeoAnonymousSourceName,
			Frequency:               frequency,
			IPV:                     "ipv4",
			Output:                  "netset",
			Processor:               []ProcessorStep{{Name: "passthrough"}},
			ProcessorRaw:            "cat",
			Hidden:                  true,
			AcceptEmpty:             true,
			EnabledByAll:            true,
			ExcludeFromUnmaintained: true,
			Info:                    "Synthetic hidden feed built from the anonymous-network buckets of the configured GeoIP providers.",
			Maintainer:              "FireHOL",
			MaintainerURL:           "https://iplists.firehol.org/",
			DerivedFrom:             derived,
			Provenance:              ProvenancePrimary,
		},
		{
			Name:                    GeoSatelliteSourceName,
			Label:                   "Satellite providers",
			URL:                     "internal://" + GeoSatelliteSourceName,
			Frequency:               frequency,
			IPV:                     "ipv4",
			Output:                  "netset",
			Processor:               []ProcessorStep{{Name: "passthrough"}},
			ProcessorRaw:            "cat",
			Hidden:                  true,
			AcceptEmpty:             true,
			EnabledByAll:            true,
			ExcludeFromUnmaintained: true,
			Info:                    "Synthetic hidden feed built from the satellite-provider buckets of the configured GeoIP providers.",
			Maintainer:              "FireHOL",
			MaintainerURL:           "https://iplists.firehol.org/",
			DerivedFrom:             derived,
			Provenance:              ProvenancePrimary,
		},
	}
}

func geoSyntheticParents(cfg *Config) []string {
	if cfg == nil || len(cfg.Sources) == 0 {
		return nil
	}
	parents := make([]string, 0, len(cfg.Sources))
	for _, name := range cfg.orderedSourceNames() {
		src := cfg.Sources[name]
		if src != nil && src.HasUse(UseGeoIP) {
			parents = append(parents, name)
		}
	}
	return parents
}
