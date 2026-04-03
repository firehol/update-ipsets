package engine

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

// RFCReservedSourceName is the canonical name of the synthetic source
// that carries the hardcoded RFC reserved baseline. It matches the YAML
// entry under `sources:` and is the target of the `internal://` URL
// registered at package init. Exported so the config validator and
// engine can refer to it without duplicating the string literal.
const RFCReservedSourceName = "rfc_reserved"

// RFCReservedFormat is the `format:` identifier in the YAML for any
// source that provides an RFC reserved baseline. The engine dispatches
// on this value in a couple of places (synthetic-source build path,
// per-range breakdown emitter). Exported so those call sites can use
// the constant instead of re-typing the string literal and drifting
// out of sync with the YAML if we ever rename the format.
const RFCReservedFormat = "rfc_reserved_baseline"

// RFCReservedBytes returns the hardcoded RFC reserved baseline encoded
// as a plain text file: one "CIDR # label (rfc)" line per entry plus
// a short header comment. The same bytes are registered with the
// downloader under internal://rfc_reserved so the synthetic source
// flows through the normal download/cache/parse pipeline.
func RFCReservedBytes() []byte {
	var buf bytes.Buffer
	buf.WriteString("# RFC reserved IPv4 ranges — hardcoded baseline\n")
	buf.WriteString("# Maintained by FireHOL; edits require a citation to an RFC.\n")
	for _, entry := range rfcReservedBogons {
		fmt.Fprintf(&buf, "%s # %s (%s)\n", entry.CIDR, entry.Name, entry.RFC)
	}
	return buf.Bytes()
}

// registerInternalDownloads wires the process-wide downloader registry
// to the synthetic sources the engine supports. Called exactly once at
// package init so the engine does not have to remember to do it.
func init() {
	downloader.RegisterInternal(RFCReservedSourceName, func(_ string) ([]byte, error) {
		return RFCReservedBytes(), nil
	})
}

// rfcReservedEntry is one row in the hardcoded RFC reserved baseline
// list. CIDR is the IPv4 prefix in dotted-quad notation, Name is a
// short human-readable label, and RFC names the document that defines
// the reservation. The slice below is the only authoritative list and
// is consumed by buildRFCReservedSet to materialize an *iprange.IPSet.
type rfcReservedEntry struct {
	CIDR string
	Name string
	RFC  string
}

// rfcReservedBogons is the hardcoded baseline list of RFC-reserved
// IPv4 ranges. These are always included in the bogon classification
// regardless of what external bogon feeds are configured, so even if
// every external provider fails, RFC 1918 private space and the
// other reserved ranges are still correctly identified as bogus.
//
// The list is intentionally curated and excludes deprecated edge cases
// that would require explanation. New entries must cite an RFC.
var rfcReservedBogons = []rfcReservedEntry{
	{CIDR: "0.0.0.0/8", Name: "Current network", RFC: "RFC 1122 section 3.2.1.3"},
	{CIDR: "10.0.0.0/8", Name: "RFC 1918 private (10/8)", RFC: "RFC 1918"},
	{CIDR: "100.64.0.0/10", Name: "Carrier-grade NAT", RFC: "RFC 6598"},
	{CIDR: "127.0.0.0/8", Name: "Loopback", RFC: "RFC 1122 section 3.2.1.3"},
	{CIDR: "169.254.0.0/16", Name: "Link-local", RFC: "RFC 3927"},
	{CIDR: "172.16.0.0/12", Name: "RFC 1918 private (172.16/12)", RFC: "RFC 1918"},
	{CIDR: "192.0.0.0/24", Name: "IETF protocol assignments", RFC: "RFC 6890"},
	{CIDR: "192.0.2.0/24", Name: "TEST-NET-1", RFC: "RFC 5737"},
	{CIDR: "192.88.99.0/24", Name: "6to4 relay anycast (deprecated)", RFC: "RFC 7526"},
	{CIDR: "192.168.0.0/16", Name: "RFC 1918 private (192.168/16)", RFC: "RFC 1918"},
	{CIDR: "198.18.0.0/15", Name: "Network benchmarking", RFC: "RFC 2544"},
	{CIDR: "198.51.100.0/24", Name: "TEST-NET-2", RFC: "RFC 5737"},
	{CIDR: "203.0.113.0/24", Name: "TEST-NET-3", RFC: "RFC 5737"},
	{CIDR: "224.0.0.0/4", Name: "IPv4 multicast", RFC: "RFC 5771"},
	{CIDR: "240.0.0.0/4", Name: "Reserved for future use", RFC: "RFC 1112"},
}

// rfcReservedRange is one parsed entry with the lo/hi boundaries
// already computed. Used to populate per-range counts in the bogon
// JSON output for the rfc_reserved provider, which is the only
// provider where individual range labels are known.
type rfcReservedRange struct {
	Lo   uint32
	Hi   uint32
	Name string
	CIDR string
	RFC  string
}

// rfcReservedRanges is the lazily-built parsed form of rfcReservedBogons.
// It is computed once on first call to getRFCReservedRanges and is safe
// to share read-only across goroutines after that.
var (
	rfcReservedRangesCache []rfcReservedRange
	rfcReservedSetCache    *iprange.IPSet
)

// getRFCReservedRanges returns the parsed RFC reserved baseline ranges.
// Computed once and cached.
func getRFCReservedRanges() ([]rfcReservedRange, error) {
	if rfcReservedRangesCache != nil {
		return rfcReservedRangesCache, nil
	}
	out := make([]rfcReservedRange, 0, len(rfcReservedBogons))
	for _, entry := range rfcReservedBogons {
		lo, hi, err := parseCIDRBounds(entry.CIDR)
		if err != nil {
			return nil, fmt.Errorf("rfc reserved %q: %w", entry.CIDR, err)
		}
		out = append(out, rfcReservedRange{
			Lo:   lo,
			Hi:   hi,
			Name: entry.Name,
			CIDR: entry.CIDR,
			RFC:  entry.RFC,
		})
	}
	rfcReservedRangesCache = out
	return out, nil
}

// buildRFCReservedSet returns an *iprange.IPSet containing all the
// hardcoded RFC reserved baseline ranges. Computed once and cached.
func buildRFCReservedSet() (*iprange.IPSet, error) {
	if rfcReservedSetCache != nil {
		return rfcReservedSetCache, nil
	}
	ranges, err := getRFCReservedRanges()
	if err != nil {
		return nil, err
	}
	set := iprange.New(RFCReservedSourceName)
	for _, r := range ranges {
		if err := set.AddRange(iprange.Range{Lo: r.Lo, Hi: r.Hi}); err != nil {
			return nil, fmt.Errorf("rfc reserved add range %s: %w", r.CIDR, err)
		}
	}
	set.Optimize()
	rfcReservedSetCache = set
	return set, nil
}

// parseCIDRBounds converts a "1.2.3.0/24" string into inclusive uint32
// lo/hi boundaries. Wraps the iprange parsers so callers do not need
// to import them just to materialize a single CIDR.
func parseCIDRBounds(cidr string) (uint32, uint32, error) {
	parts := strings.SplitN(cidr, "/", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("not a CIDR: %q", cidr)
	}
	addr, err := iprange.ParseIPv4Token(parts[0])
	if err != nil {
		return 0, 0, err
	}
	prefix, err := iprange.ParsePrefix(parts[1])
	if err != nil {
		return 0, 0, err
	}
	lo := iprange.Network(addr, prefix)
	hi := iprange.Broadcast(lo, prefix)
	return lo, hi, nil
}
