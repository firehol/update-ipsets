package geoloc

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// Dataset holds parsed country-level IP sets from a geolocation provider.
type Dataset struct {
	Provider string
	Sets     map[string]*iprange.IPSet
}

// ParseFile parses a geolocation dataset from a file on disk. This avoids
// loading the entire file into memory -- zip archives are opened via the OS
// file handle, and gzip/tar/csv are streamed line-by-line.
func ParseFile(providerType, path string) (*Dataset, error) {
	var (
		dataset *Dataset
		err     error
	)
	switch providerType {
	case "ipdeny_country_tar_gz":
		dataset, err = parseIPDenyFile(path)
	case "ip2location_country_zip":
		dataset, err = parseIP2LocationFile(path)
	case "ipip_country_zip":
		dataset, err = parseIPIPFile(path)
	case "dbip_country_csv":
		dataset, err = parseDBIPFile(path)
	case "maxmind_country_csv":
		dataset, err = parseGeoLite2File(path)
	default:
		return nil, fmt.Errorf("unsupported geolocation provider type %q", providerType)
	}
	return datasetWithProviderType(providerType, dataset, err)
}

// Parse parses a geolocation dataset from in-memory data. Kept for
// backward compatibility and tests; prefer ParseFile for large archives.
func Parse(providerType string, data []byte) (*Dataset, error) {
	var (
		dataset *Dataset
		err     error
	)
	switch providerType {
	case "ipdeny_country_tar_gz":
		dataset, err = parseIPDenyBytes(data)
	case "ip2location_country_zip":
		dataset, err = parseIP2LocationBytes(data)
	case "ipip_country_zip":
		dataset, err = parseIPIPBytes(data)
	case "dbip_country_csv":
		dataset, err = parseDBIPBytes(data)
	case "maxmind_country_csv":
		dataset, err = parseGeoLite2Bytes(data)
	default:
		return nil, fmt.Errorf("unsupported geolocation provider type %q", providerType)
	}
	return datasetWithProviderType(providerType, dataset, err)
}

func datasetWithProviderType(providerType string, dataset *Dataset, err error) (*Dataset, error) {
	if err != nil || dataset == nil {
		return dataset, err
	}
	dataset.Provider = providerType
	return dataset, nil
}

// --- IPDeny (tar.gz) ----------------------------------------------------------

func parseIPDenyFile(path string) (*Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseIPDenyReader(f)
}

func parseIPDenyBytes(data []byte) (*Dataset, error) {
	return parseIPDenyReader(bytes.NewReader(data))
}

func parseIPDenyReader(r io.Reader) (*Dataset, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	sets := map[string]*iprange.IPSet{}
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg || !strings.HasSuffix(header.Name, ".zone") {
			continue
		}
		code := strings.ToUpper(strings.TrimSuffix(filepath.Base(header.Name), ".zone"))
		// Stream parse directly from the tar entry reader.
		set, err := iprange.ParseReader(context.Background(), code, tr, iprange.DefaultParseOptions())
		if err != nil {
			return nil, err
		}
		sets[code] = set
	}
	return &Dataset{Sets: sets}, nil
}

// --- IP2Location (zip with CSV) -----------------------------------------------

func parseIP2LocationFile(path string) (*Dataset, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	return parseIP2LocationZip(&zr.Reader)
}

func parseIP2LocationBytes(data []byte) (*Dataset, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return parseIP2LocationZip(zr)
}

func parseIP2LocationZip(archive *zip.Reader) (*Dataset, error) {
	zf, err := openZipEntry(archive, "IP2LOCATION-LITE-DB1.CSV")
	if err != nil {
		return nil, err
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	sets := map[string]*iprange.IPSet{}
	reader := csv.NewReader(rc)
	reader.FieldsPerRecord = -1
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) < 4 {
			continue
		}
		code := normalizeCountryCode(strings.TrimSpace(row[2]))
		start, ok1 := numericIPv4(strings.TrimSpace(row[0]))
		end, ok2 := numericIPv4(strings.TrimSpace(row[1]))
		if !ok1 || !ok2 {
			continue
		}
		if err := addRange(sets, code, start, end); err != nil {
			return nil, err
		}
	}
	return &Dataset{Sets: sets}, nil
}

// --- IPIP (zip with text) -----------------------------------------------------

func parseIPIPFile(path string) (*Dataset, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	return parseIPIPZip(&zr.Reader)
}

func parseIPIPBytes(data []byte) (*Dataset, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return parseIPIPZip(zr)
}

func parseIPIPZip(archive *zip.Reader) (*Dataset, error) {
	zf, err := openZipEntry(archive, "country.txt")
	if err != nil {
		return nil, err
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	sets := map[string]*iprange.IPSet{}
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.ReplaceAll(scanner.Text(), "\r", "")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		code := normalizeCountryCode(fields[len(fields)-1])
		if err := addToken(sets, code, fields[0]); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Dataset{Sets: sets}, nil
}

// --- DB-IP (gzip CSV) ---------------------------------------------------------

func parseDBIPFile(path string) (*Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseDBIPReader(f)
}

func parseDBIPBytes(data []byte) (*Dataset, error) {
	return parseDBIPReader(bytes.NewReader(data))
}

func parseDBIPReader(r io.Reader) (*Dataset, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	sets := map[string]*iprange.IPSet{}
	csvr := csv.NewReader(gz)
	csvr.FieldsPerRecord = -1
	for {
		row, err := csvr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) < 3 {
			continue
		}
		code := normalizeCountryCode(strings.TrimSpace(row[2]))
		start, err2 := dottedIPv4(strings.TrimSpace(row[0]))
		if err2 != nil {
			continue
		}
		end, err2 := dottedIPv4(strings.TrimSpace(row[1]))
		if err2 != nil {
			continue
		}
		if err := addRange(sets, code, start, end); err != nil {
			return nil, err
		}
	}
	return &Dataset{Sets: sets}, nil
}

// --- GeoLite2 (zip with two CSVs) --------------------------------------------

func parseGeoLite2File(path string) (*Dataset, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	return parseGeoLite2Zip(&zr.Reader)
}

func parseGeoLite2Bytes(data []byte) (*Dataset, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return parseGeoLite2Zip(zr)
}

func parseGeoLite2Zip(archive *zip.Reader) (*Dataset, error) {
	byID := map[string]*iprange.IPSet{}
	sets := map[string]*iprange.IPSet{}

	// Stream blocks CSV.
	blocksFile, err := openZipEntrySuffix(archive, "GeoLite2-Country-Blocks-IPv4.csv")
	if err != nil {
		return nil, err
	}
	blocksRC, err := blocksFile.Open()
	if err != nil {
		return nil, err
	}
	blocksCSV := csv.NewReader(blocksRC)
	blocksCSV.FieldsPerRecord = -1
	firstRow := true
	for {
		row, err := blocksCSV.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = blocksRC.Close()
			return nil, err
		}
		if firstRow {
			firstRow = false
			continue
		}
		if len(row) < 6 {
			continue
		}
		network := strings.TrimSpace(row[0])
		for _, id := range []string{row[1], row[2], row[3]} {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if err := addToken(byID, id, network); err != nil {
				_ = blocksRC.Close()
				return nil, err
			}
		}
		if strings.TrimSpace(row[4]) == "1" {
			if err := addToken(sets, "ANONYMOUS", network); err != nil {
				_ = blocksRC.Close()
				return nil, err
			}
		}
		if strings.TrimSpace(row[5]) == "1" {
			if err := addToken(sets, "SATELLITE", network); err != nil {
				_ = blocksRC.Close()
				return nil, err
			}
		}
	}
	_ = blocksRC.Close()

	// Stream locations CSV.
	locsFile, err := openZipEntrySuffix(archive, "GeoLite2-Country-Locations-en.csv")
	if err != nil {
		return nil, err
	}
	locsRC, err := locsFile.Open()
	if err != nil {
		return nil, err
	}
	locsCSV := csv.NewReader(locsRC)
	locsCSV.FieldsPerRecord = -1
	firstRow = true
	for {
		row, err := locsCSV.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = locsRC.Close()
			return nil, err
		}
		if firstRow {
			firstRow = false
			continue
		}
		if len(row) < 6 {
			continue
		}
		id := strings.TrimSpace(row[0])
		base := byID[id]
		if base == nil {
			continue
		}
		if code := normalizeCountryCode(strings.TrimSpace(row[4])); code != "" {
			if err := mergeInto(sets, code, base); err != nil {
				_ = locsRC.Close()
				return nil, err
			}
		}
		// Continent code (row[2]) is intentionally ignored: it would
		// double-count every IP under a continent pseudo-code, and
		// AF/AS/NA/SA collide with real ISO-3166 codes (Afghanistan,
		// American Samoa, Namibia, Saudi Arabia), corrupting them.
	}
	_ = locsRC.Close()

	return &Dataset{Sets: sets}, nil
}
