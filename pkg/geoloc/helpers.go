package geoloc

import (
	"archive/zip"
	"fmt"
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// openZipEntry opens the first zip entry whose base name matches exactly.
func openZipEntry(archive *zip.Reader, name string) (*zip.File, error) {
	for _, file := range archive.File {
		if filepath.Base(file.Name) == name {
			return file, nil
		}
	}
	return nil, fmt.Errorf("zip entry %q not found", name)
}

// openZipEntrySuffix opens the first zip entry whose name ends with suffix.
func openZipEntrySuffix(archive *zip.Reader, suffix string) (*zip.File, error) {
	for _, file := range archive.File {
		if strings.HasSuffix(file.Name, suffix) {
			return file, nil
		}
	}
	return nil, fmt.Errorf("zip entry with suffix %q not found", suffix)
}

func normalizeCountryCode(code string) string {
	code = strings.Trim(strings.ToUpper(code), `"`)
	switch code {
	case "", "-", "ZZ":
		return "COUNTRYLESS"
	default:
		return code
	}
}

func addToken(sets map[string]*iprange.IPSet, code, token string) error {
	token = strings.TrimSpace(strings.Trim(token, `"`))
	if token == "" {
		return nil
	}
	set := ensureSet(sets, code)
	switch {
	case strings.Contains(token, "/"):
		ip, network, err := net.ParseCIDR(token)
		if err != nil || ip.To4() == nil {
			return nil
		}
		start := ipv4ToUint32(network.IP.To4())
		end := start | ^maskToUint32(network.Mask)
		return set.AddRange(iprange.Range{Lo: start, Hi: end})
	case strings.Contains(token, "-"):
		left, right, ok := strings.Cut(strings.ReplaceAll(token, " ", ""), "-")
		if !ok {
			return nil
		}
		start, err := dottedIPv4(left)
		if err != nil {
			return nil
		}
		end, err := dottedIPv4(right)
		if err != nil {
			return nil
		}
		return addRange(sets, code, start, end)
	default:
		ip, err := dottedIPv4(token)
		if err != nil {
			return nil
		}
		return set.Add(ip, ip)
	}
}

func addRange(sets map[string]*iprange.IPSet, code string, start, end uint32) error {
	return ensureSet(sets, code).Add(start, end)
}

func mergeInto(sets map[string]*iprange.IPSet, code string, source *iprange.IPSet) error {
	target := ensureSet(sets, code)
	return target.Merge(source)
}

func ensureSet(sets map[string]*iprange.IPSet, code string) *iprange.IPSet {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		code = "UNKNOWN"
	}
	set := sets[code]
	if set == nil {
		set = iprange.New(code)
		sets[code] = set
	}
	return set
}

func numericIPv4(raw string) (uint32, bool) {
	raw = strings.Trim(raw, `"`)
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

func dottedIPv4(raw string) (uint32, error) {
	ip := net.ParseIP(strings.Trim(raw, `"`)).To4()
	if ip == nil {
		return 0, fmt.Errorf("invalid IPv4 %q", raw)
	}
	return ipv4ToUint32(ip), nil
}

func ipv4ToUint32(ip net.IP) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func maskToUint32(mask net.IPMask) uint32 {
	return uint32(mask[0])<<24 | uint32(mask[1])<<16 | uint32(mask[2])<<8 | uint32(mask[3])
}

// SortedCodes returns the country codes from a Dataset sorted alphabetically.
func SortedCodes(data *Dataset) []string {
	if data == nil {
		return nil
	}
	codes := make([]string, 0, len(data.Sets))
	for code := range data.Sets {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}
