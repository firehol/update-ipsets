// Package asnloc provides IP-to-ASN attribution from multiple provider
// formats (MaxMind GeoLite2 ASN MMDB, DB-IP Lite ASN MMDB, iptoasn.com TSV,
// and CAIDA RouteViews prefix2as).
//
// Each Database wraps one provider's loaded dataset behind a backend
// interface so the public range-walking methods (Lookup, Stats, CountFeed,
// CountFeedWithBogons) work uniformly across MMDB-backed and text-backed
// formats.
//
// New providers are added by writing a backend implementation and
// registering it in Open. The frontend presents the providers as tabs so
// users can compare and cross-validate; disagreement between providers is
// itself a useful signal.
package asnloc

import (
	"fmt"
	"net"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// Record is one provider's attribution for a single IP. ASN is zero when
// the database has no record for the queried address.
type Record struct {
	ASN  uint32
	Name string
}

// Network is the IPv4 range over which a single ASN attribution is
// constant in the database. Lo and Hi are inclusive uint32 boundaries.
// Range-based callers use this to advance past the entire constant
// region in one step instead of looking up every individual IP.
type Network struct {
	Lo uint32
	Hi uint32
}

// backend is the storage-format-specific lookup engine that powers a
// Database. Implementations are per provider format: one for MMDB files
// (MaxMind, DB-IP) and one for sorted in-memory range tables (iptoasn,
// CAIDA prefix2as).
//
// All methods must be safe for concurrent use because Database.Lookup is
// called from per-feed goroutines in the engine.
type backend interface {
	// lookup returns the ASN attribution for an IPv4 address and the
	// network range over which the attribution is constant. When the
	// backend has no record for the IP, the returned Record.ASN is 0
	// and the Network covers the largest contiguous unattributed region
	// containing the IP, so the caller can skip the entire gap in one
	// step.
	lookup(ipv4 uint32) (Record, Network, error)

	// stats returns the total number of distinct attribution networks
	// and the total number of IPv4 addresses they cover. Used to
	// populate the admin row for the provider.
	stats() (networks int, ipv4Covered uint64, err error)

	// close releases any underlying resources (file handles, mmaps).
	close() error
}

// Database is one ASN provider's loaded dataset. The underlying backend
// is selected by Open based on the provider type string. Lookups are
// thread-safe.
type Database struct {
	Provider string
	be       backend
}

// Open loads an ASN database file. The providerType selects the backend
// for the file's format; pass the same string used in the YAML config
// (e.g. "maxmind_geolite2_asn_mmdb", "iptoasn_combined_tsv",
// "caida_prefix2as", "dbip_asn_lite_mmdb").
//
// For MMDB-format providers, path is the .mmdb file. For text-format
// providers, path is the decompressed plain text file.
func Open(providerType, path string) (*Database, error) {
	switch providerType {
	case "maxmind_geolite2_asn_mmdb", "maxmind_asn_mmdb_tar_gz", "dbip_asn_lite_mmdb":
		be, err := openMMDBBackend(providerType, path)
		if err != nil {
			return nil, err
		}
		return &Database{Provider: providerType, be: be}, nil
	case "iptoasn_combined_tsv":
		be, err := loadIPToASNTSV(path)
		if err != nil {
			return nil, err
		}
		return &Database{Provider: providerType, be: be}, nil
	case "caida_prefix2as":
		be, err := loadCAIDAPrefix2AS(path)
		if err != nil {
			return nil, err
		}
		return &Database{Provider: providerType, be: be}, nil
	default:
		return nil, fmt.Errorf("unsupported ASN database type %q", providerType)
	}
}

// Close releases the underlying file handles or in-memory tables. Safe to
// call on nil.
func (d *Database) Close() error {
	if d == nil || d.be == nil {
		return nil
	}
	return d.be.close()
}

// Lookup returns the ASN attribution for an IPv4 address and the network
// range over which the attribution is constant. When the database has
// no record for the IP, Record.ASN is 0 but the Network bounds are
// still set so a range-walking caller can advance past the unknown
// region in one step.
func (d *Database) Lookup(ipv4 uint32) (Record, Network, error) {
	if d == nil || d.be == nil {
		return Record{}, Network{}, fmt.Errorf("nil database")
	}
	return d.be.lookup(ipv4)
}

// Stats walks every IPv4 network in the underlying database and returns
// the total number of distinct networks and the total number of IPv4
// addresses covered. Each ASN provider's database will report a slightly
// different total — disagreement between providers is itself useful.
func (d *Database) Stats() (networks int, ipv4Covered uint64, err error) {
	if d == nil || d.be == nil {
		return 0, 0, fmt.Errorf("nil database")
	}
	return d.be.stats()
}

// CountFeed walks every range in the input IPv4 source and returns the
// per-ASN IP counts. Walking is range-aware: we move forward by entire
// ASN boundaries rather than one IP at a time, so a feed with millions
// of expanded IPs is processed in time proportional to the number of
// distinct ASN regions, not the IP count.
//
// The argument is an iprange.RangeSource so callers can pass either an
// in-memory *iprange.IPSet or a file-backed iprange.FileSet without
// loading the whole feed into RAM.
//
// Two maps are returned. counts maps ASN number to the total number of
// IPs in the feed that fall within that ASN. names maps ASN number to
// the most recently seen organization name for that ASN. The zero ASN
// (unknown) is included in counts so callers can report it as "unknown"
// coverage.
func (d *Database) CountFeed(src iprange.RangeSource) (counts map[uint32]uint64, names map[uint32]string, err error) {
	counts, names, _, err = d.countFeedRanges(src.Iter())
	return counts, names, err
}

// CountFeedExcluding walks every range in src except ranges covered by exclude.
// It is the residual counting half of CountFeedWithBogons, exposed so callers
// comparing the same feed against multiple ASN providers can compute the
// provider-independent excluded count once and reuse it.
func (d *Database) CountFeedExcluding(src, exclude iprange.RangeSource) (counts map[uint32]uint64, names map[uint32]string, err error) {
	if exclude == nil {
		return d.CountFeed(src)
	}
	counts, names, _, err = d.countFeedRanges(iprange.ExcludeIter(src, exclude))
	return counts, names, err
}

// CountFeedWithBogons is like CountFeed but with a bogon-aware split.
// IPs in the feed that overlap with bogonSet are counted as bogon and
// never looked up in the ASN database (which would either return ASN 0
// or, worse, attribute private/reserved space to whatever happens to be
// listed in the database). The remainder is walked through the database
// exactly as in CountFeed.
//
// Returns:
//   - counts: map of ASN -> attributed IP count. ASN 0 is the residual
//     unknown bucket (IPs that are NOT bogon AND have no database record).
//   - names: most recently seen organization name per ASN.
//   - bogonCount: total IPs of the feed that fell into bogonSet.
//
// The invariant for callers is:
//
//	feed_ips == sum(counts where asn != 0) + counts[0] + bogonCount
//
// where the three RHS terms map to attributed_ips, unknown_ips, and
// bogon_ips in the JSON output.
//
// When bogonSet is nil, the result is identical to CountFeed and
// bogonCount is zero.
func (d *Database) CountFeedWithBogons(src iprange.RangeSource, bogonSet iprange.RangeSource) (counts map[uint32]uint64, names map[uint32]string, bogonCount uint64, err error) {
	if bogonSet == nil {
		counts, names, err = d.CountFeed(src)
		return counts, names, 0, err
	}
	// Bogon contribution: intersection of the feed with the bogon set.
	bogonCount = iprange.OverlapCountIter(src, bogonSet)
	// Database walk for the bogon-free residual of the feed.
	counts, names, err = d.CountFeedExcluding(src, bogonSet)
	return counts, names, bogonCount, err
}

// countFeedRanges walks the supplied range stream through the ASN
// database. It is the shared core of CountFeed and CountFeedWithBogons.
func (d *Database) countFeedRanges(seq func(yield func(iprange.Range) bool)) (counts map[uint32]uint64, names map[uint32]string, total uint64, err error) {
	counts = map[uint32]uint64{}
	names = map[uint32]string{}
	if d == nil || seq == nil {
		return counts, names, 0, nil
	}
	seq(func(r iprange.Range) bool {
		cur := r.Lo
		for {
			rec, network, lookupErr := d.Lookup(cur)
			if lookupErr != nil {
				err = lookupErr
				return false
			}
			// Defensive max(cur, ...) in case a backend ever returns a
			// malformed network whose Hi is below the current cursor;
			// without it the loop could run forever.
			end := max(cur, min(network.Hi, r.Hi))
			span := uint64(end-cur) + 1
			counts[rec.ASN] += span
			total += span
			if rec.ASN != 0 && rec.Name != "" {
				names[rec.ASN] = rec.Name
			}
			if end == ^uint32(0) || end >= r.Hi {
				break
			}
			cur = end + 1
		}
		return true
	})
	return counts, names, total, err
}

// uint32ToIP converts an IPv4 stored as a host-order uint32 into a net.IP.
func uint32ToIP(v uint32) net.IP {
	return net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// ipnetToBounds converts a net.IPNet returned by MMDB into uint32 lo/hi
// boundaries for IPv4. Returns false for IPv6-only networks.
func ipnetToBounds(n *net.IPNet) (lo, hi uint32, ok bool) {
	ip4 := n.IP.To4()
	if ip4 == nil {
		return 0, 0, false
	}
	lo = uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
	ones, bits := n.Mask.Size()
	if bits != 32 {
		// Mask in 16-byte form (IPv4-in-IPv6) — extract the trailing 32 bits.
		if bits == 128 && ones >= 96 {
			ones -= 96
		} else {
			return 0, 0, false
		}
	}
	if ones >= 32 {
		return lo, lo, true
	}
	hostBits := uint32(32 - ones)
	size := uint32(1)<<hostBits - 1
	return lo, lo + size, true
}
