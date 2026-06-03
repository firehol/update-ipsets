package engine

import (
	"strings"
	"testing"
)

func TestEntityDetailBuildersRejectInvalidIdentifiers(t *testing.T) {
	eng, _, _ := newDetailEngineForTest(t)

	if _, err := eng.CountryDetail(" \t "); err == nil || !strings.Contains(err.Error(), "country code is required") {
		t.Fatalf("CountryDetail blank error = %v, want country code validation", err)
	}
	if _, err := eng.ASNDetail(0); err == nil || !strings.Contains(err.Error(), "asn must be a positive integer") {
		t.Fatalf("ASNDetail zero error = %v, want ASN validation", err)
	}
}

func TestEntityDetailBuildersReturnEmptyPayloadsWhenNoFeedsMatch(t *testing.T) {
	eng, _, _ := newDetailEngineForTest(t)

	country, err := eng.CountryDetail("zz")
	if err != nil {
		t.Fatal(err)
	}
	if country.Code != "ZZ" {
		t.Fatalf("country code = %q, want ZZ", country.Code)
	}
	if country.Provider.Name != "dbip_country" {
		t.Fatalf("country provider = %q, want dbip_country", country.Provider.Name)
	}
	if country.ASNProvider.Name != "iptoasn" {
		t.Fatalf("country ASN provider = %q, want iptoasn", country.ASNProvider.Name)
	}
	if country.Totals.FeedsMatching != 0 || len(country.Feeds) != 0 || len(country.TopASNs) != 0 {
		t.Fatalf("country detail is not empty: totals=%+v feeds=%d top_asns=%d", country.Totals, len(country.Feeds), len(country.TopASNs))
	}

	asn, err := eng.ASNDetail(64512)
	if err != nil {
		t.Fatal(err)
	}
	if asn.ASN != 64512 {
		t.Fatalf("ASN = %d, want 64512", asn.ASN)
	}
	if asn.Provider.Name != "iptoasn" {
		t.Fatalf("ASN provider = %q, want iptoasn", asn.Provider.Name)
	}
	if asn.GeoProvider.Name != "dbip_country" {
		t.Fatalf("ASN geo provider = %q, want dbip_country", asn.GeoProvider.Name)
	}
	if asn.Totals.FeedsMatching != 0 || len(asn.Feeds) != 0 || len(asn.TopCountries) != 0 || asn.CountryDistribution != nil {
		t.Fatalf("ASN detail is not empty: totals=%+v feeds=%d top_countries=%d distribution=%+v", asn.Totals, len(asn.Feeds), len(asn.TopCountries), asn.CountryDistribution)
	}
}
