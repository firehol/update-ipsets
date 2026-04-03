package asnloc

import (
	"fmt"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

// mmdbBackend serves lookups out of a MaxMind-style MMDB file. Used by
// both the MaxMind GeoLite2-ASN database and DB-IP Lite ASN, which share
// the same field schema (autonomous_system_number,
// autonomous_system_organization).
type mmdbBackend struct {
	reader *maxminddb.Reader
	decode mmdbDecoderFunc
}

type mmdbDecoderFunc func(reader *maxminddb.Reader, ip net.IP) (Record, *net.IPNet, error)

func openMMDBBackend(providerType, path string) (*mmdbBackend, error) {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ASN database %q: %w", path, err)
	}
	decode, err := mmdbDecoderFor(providerType)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	return &mmdbBackend{reader: reader, decode: decode}, nil
}

func (b *mmdbBackend) lookup(ipv4 uint32) (Record, Network, error) {
	ip := uint32ToIP(ipv4)
	rec, ipnet, err := b.decode(b.reader, ip)
	if err != nil {
		return Record{}, Network{}, err
	}
	if ipnet == nil {
		// MMDB miss with no enclosing network — advance one IP at a time.
		return rec, Network{Lo: ipv4, Hi: ipv4}, nil
	}
	lo, hi, ok := ipnetToBounds(ipnet)
	if !ok {
		// Non-IPv4 network returned (shouldn't happen for IPv4 lookups).
		return rec, Network{Lo: ipv4, Hi: ipv4}, nil
	}
	return rec, Network{Lo: lo, Hi: hi}, nil
}

func (b *mmdbBackend) stats() (networks int, ipv4Covered uint64, err error) {
	if b == nil || b.reader == nil {
		return 0, 0, fmt.Errorf("nil mmdb reader")
	}
	iter := b.reader.Networks()
	var raw struct{}
	for iter.Next() {
		ipnet, nErr := iter.Network(&raw)
		if nErr != nil {
			err = nErr
			return
		}
		lo, hi, ok := ipnetToBounds(ipnet)
		if !ok {
			// IPv6-only network — not counted in IPv4 coverage but
			// still counted as a network for total network count.
			networks++
			continue
		}
		networks++
		ipv4Covered += uint64(hi-lo) + 1
	}
	if iterErr := iter.Err(); iterErr != nil {
		err = iterErr
	}
	return
}

func (b *mmdbBackend) close() error {
	if b == nil || b.reader == nil {
		return nil
	}
	return b.reader.Close()
}

// mmdbDecoderFor returns the per-record decoder for an MMDB-format
// provider type. MaxMind GeoLite2-ASN and DB-IP Lite ASN both expose the
// same two fields so they share decodeMaxMindASN.
func mmdbDecoderFor(providerType string) (mmdbDecoderFunc, error) {
	switch providerType {
	case "maxmind_geolite2_asn_mmdb", "maxmind_asn_mmdb_tar_gz", "dbip_asn_lite_mmdb":
		return decodeMaxMindASN, nil
	default:
		return nil, fmt.Errorf("unsupported MMDB ASN database type %q", providerType)
	}
}

// maxmindASNRecord matches the record schema of MaxMind's GeoLite2-ASN
// MMDB file (and DB-IP Lite ASN, which uses the same field names).
// Field tags are documented at:
// https://dev.maxmind.com/geoip/docs/databases/asn
type maxmindASNRecord struct {
	AutonomousSystemNumber       uint32 `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

func decodeMaxMindASN(r *maxminddb.Reader, ip net.IP) (Record, *net.IPNet, error) {
	var rec maxmindASNRecord
	network, _, err := r.LookupNetwork(ip, &rec)
	if err != nil {
		return Record{}, network, err
	}
	return Record{
		ASN:  rec.AutonomousSystemNumber,
		Name: rec.AutonomousSystemOrganization,
	}, network, nil
}
