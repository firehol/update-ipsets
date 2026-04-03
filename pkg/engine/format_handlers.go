package engine

// formatRole identifies which engine role a format handler serves.
// Format names are unique per role; this is informational only.
type formatRole int

const (
	formatRoleASN formatRole = iota
	formatRoleGeoIP
	formatRoleIPSet
)

// formatSpec describes how a single wire format is downloaded and
// parsed. It is the registry entry the engine looks up by source.Format
// when processing asn/geoip sources. ipset formats are not registered
// here — they go through the existing processor pipeline.
//
// Fields:
//
//   - role: which engine role this format belongs to (asn vs geoip)
//   - dataFile: name of the parsed/extracted file under lib_dir/<source>/
//   - extract: takes the freshly downloaded archive at archivePath and
//     writes the parsed/extracted file to dstPath. May be nil for
//     formats that need no extraction (the downloaded file IS the
//     parsed file).
type formatSpec struct {
	role     formatRole
	dataFile string
	extract  func(archivePath, dstPath string) error
}

// formatRegistry is populated by init() functions in the per-format
// files. Looking up an unknown format returns ok=false and the engine
// surfaces a clear error pointing operators at the YAML.
var formatRegistry = map[string]formatSpec{}

// registerFormat adds a format spec to the registry. Duplicate
// registrations panic — they are programmer errors.
func registerFormat(name string, spec formatSpec) {
	if _, exists := formatRegistry[name]; exists {
		panic("duplicate format registration: " + name)
	}
	formatRegistry[name] = spec
}

// lookupFormat returns the spec for a registered format name.
func lookupFormat(name string) (formatSpec, bool) {
	spec, ok := formatRegistry[name]
	return spec, ok
}

// init registers every format the engine knows how to handle. New
// formats add themselves here in one place so the registry is
// discoverable from a single file.
func init() {
	// ASN formats
	registerFormat("maxmind_asn_mmdb_tar_gz", formatSpec{
		role:     formatRoleASN,
		dataFile: "database.mmdb",
		extract:  extractMMDBFromArchive,
	})
	registerFormat("dbip_asn_lite_mmdb", formatSpec{
		role:     formatRoleASN,
		dataFile: "database.mmdb",
		extract:  decompressGzipToFile,
	})
	registerFormat("iptoasn_combined_tsv", formatSpec{
		role:     formatRoleASN,
		dataFile: "database.tsv",
		extract:  decompressGzipToFile,
	})
	registerFormat("caida_prefix2as", formatSpec{
		role:     formatRoleASN,
		dataFile: "database.txt",
		extract:  decompressGzipToFile,
	})

	// GeoIP formats: the parser reads the downloaded file in place,
	// so dataFile is the source file itself and extract is nil.
	registerFormat("dbip_country_csv", formatSpec{
		role:     formatRoleGeoIP,
		dataFile: "source",
	})
	registerFormat("maxmind_country_csv", formatSpec{
		role:     formatRoleGeoIP,
		dataFile: "source",
	})
	registerFormat("ip2location_country_zip", formatSpec{
		role:     formatRoleGeoIP,
		dataFile: "source",
	})
	registerFormat("ipdeny_country_tar_gz", formatSpec{
		role:     formatRoleGeoIP,
		dataFile: "source",
	})
	registerFormat("ipip_country_zip", formatSpec{
		role:     formatRoleGeoIP,
		dataFile: "source",
	})
}
