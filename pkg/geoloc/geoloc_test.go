package geoloc_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/firehol/update-ipsets/pkg/geoloc"
)

func TestParseIPDeny(t *testing.T) {
	payload := buildIPDenyArchive(t)
	data, err := geoloc.Parse("ipdeny_country_tar_gz", payload)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sets["US"].UniqueCount() != 256 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseIPDenyFile(t *testing.T) {
	payload := buildIPDenyArchive(t)
	path := writeTempFile(t, "ipdeny.tar.gz", payload)
	data, err := geoloc.ParseFile("ipdeny_country_tar_gz", path)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sets["US"].UniqueCount() != 256 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseIP2Location(t *testing.T) {
	payload := buildIP2LocationArchive(t)
	data, err := geoloc.Parse("ip2location_country_zip", payload)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sets["US"].UniqueCount() != 2 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseIP2LocationFile(t *testing.T) {
	payload := buildIP2LocationArchive(t)
	path := writeTempFile(t, "ip2location.zip", payload)
	data, err := geoloc.ParseFile("ip2location_country_zip", path)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sets["US"].UniqueCount() != 2 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseIPIP(t *testing.T) {
	payload := buildIPIPArchive(t)
	data, err := geoloc.Parse("ipip_country_zip", payload)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sets["US"].UniqueCount() != 256 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseIPIPFile(t *testing.T) {
	payload := buildIPIPArchive(t)
	path := writeTempFile(t, "ipip.zip", payload)
	data, err := geoloc.ParseFile("ipip_country_zip", path)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sets["US"].UniqueCount() != 256 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseDBIP(t *testing.T) {
	payload := buildDBIPArchive(t)
	data, err := geoloc.Parse("dbip_country_csv", payload)
	if err != nil {
		t.Fatal(err)
	}
	if data.Provider != "dbip_country_csv" {
		t.Fatalf("provider = %q, want format name", data.Provider)
	}
	if data.Sets["US"].UniqueCount() != 2 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseDBIPFile(t *testing.T) {
	payload := buildDBIPArchive(t)
	path := writeTempFile(t, "dbip.csv.gz", payload)
	data, err := geoloc.ParseFile("dbip_country_csv", path)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sets["US"].UniqueCount() != 2 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
}

func TestParseGeoLite2(t *testing.T) {
	payload := buildGeoLite2Archive(t)
	data, err := geoloc.Parse("maxmind_country_csv", payload)
	if err != nil {
		t.Fatal(err)
	}
	assertGeoLite2(t, data)
}

func TestParseGeoLite2File(t *testing.T) {
	payload := buildGeoLite2Archive(t)
	path := writeTempFile(t, "geolite2.zip", payload)
	data, err := geoloc.ParseFile("maxmind_country_csv", path)
	if err != nil {
		t.Fatal(err)
	}
	assertGeoLite2(t, data)
}

// --- archive builders --------------------------------------------------------

func buildIPDenyArchive(t *testing.T) []byte {
	t.Helper()
	var payload bytes.Buffer
	gw := gzip.NewWriter(&payload)
	tw := tar.NewWriter(gw)
	body := []byte("1.2.3.0/24\n")
	if err := tw.WriteHeader(&tar.Header{Name: "us.zone", Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}

func buildIP2LocationArchive(t *testing.T) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	file, err := zw.Create("IP2LOCATION-LITE-DB1.CSV")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("\"16777216\",\"16777217\",\"US\",\"United States\"\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func buildIPIPArchive(t *testing.T) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	file, err := zw.Create("country.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("1.2.3.0/24\tUS\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func buildDBIPArchive(t *testing.T) []byte {
	t.Helper()
	var payload bytes.Buffer
	gw := gzip.NewWriter(&payload)
	_, _ = gw.Write([]byte("1.2.3.0,1.2.3.1,US\n"))
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}

func buildGeoLite2Archive(t *testing.T) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	blocks, err := zw.Create("GeoLite2-Country-Blocks-IPv4.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = blocks.Write([]byte("network,geoname_id,registered_country_geoname_id,represented_country_geoname_id,is_anonymous_proxy,is_satellite_provider\n1.2.3.0/24,1,,,0,0\n5.6.7.0/24,,,,1,1\n"))
	locations, err := zw.Create("GeoLite2-Country-Locations-en.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = locations.Write([]byte("geoname_id,locale_code,continent_code,continent_name,country_iso_code,country_name\n1,en,NA,North America,US,United States\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

// --- assertions --------------------------------------------------------------

func assertGeoLite2(t *testing.T, data *geoloc.Dataset) {
	t.Helper()
	if data.Sets["US"].UniqueCount() != 256 {
		t.Fatalf("unexpected US set size: %d", data.Sets["US"].UniqueCount())
	}
	if _, ok := data.Sets["NA"]; ok {
		t.Fatalf("continent code NA must not appear in country dataset (collides with Namibia)")
	}
	if data.Sets["ANONYMOUS"].UniqueCount() != 256 {
		t.Fatalf("unexpected ANONYMOUS set size: %d", data.Sets["ANONYMOUS"].UniqueCount())
	}
	if data.Sets["SATELLITE"].UniqueCount() != 256 {
		t.Fatalf("unexpected SATELLITE set size: %d", data.Sets["SATELLITE"].UniqueCount())
	}
}

// --- helpers -----------------------------------------------------------------

func writeTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
