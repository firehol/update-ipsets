package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEntityOutputViewDoesNotRetainDecodedPayloadsByDefault(t *testing.T) {
	t.Parallel()

	eng := newEngineFixture(t)
	writeHomeCountryPayload(t, eng.outputDir(), "alpha", "geolite2_country", []CountryValue{{Code: "US", Value: 7}})
	writeOutputViewASNPayload(t, eng.outputDir(), "alpha", "iptoasn", []topASNRow{{ASN: 13335, Name: "CLOUDFLARENET", Count: 11}})

	view := newEntityOutputViewWithRuntime(eng, eng.Runtime(), "")
	countryPayload, err := view.countryComparison("alpha", "geolite2_country")
	if err != nil {
		t.Fatalf("countryComparison: %v", err)
	}
	if countryPayload == nil || len(countryPayload.Countries) != 1 || countryPayload.Countries[0].Value != 7 {
		t.Fatalf("countryComparison payload = %+v; want one US value", countryPayload)
	}
	asnRows, err := view.topASNsWithError("alpha", "iptoasn")
	if err != nil {
		t.Fatalf("topASNsWithError: %v", err)
	}
	if len(asnRows) != 1 || asnRows[0].ASN != 13335 || asnRows[0].Count != 11 {
		t.Fatalf("ASN rows = %+v; want one 13335 row", asnRows)
	}
	if view.cache {
		t.Fatalf("default entityOutputView cache flag = true; want false")
	}
	if len(view.countryCache) != 0 || len(view.asnCache) != 0 {
		t.Fatalf("default entityOutputView retained cache entries: countries=%d asns=%d", len(view.countryCache), len(view.asnCache))
	}
}

func TestCachedEntityOutputViewRetainsOnlyWhenRequested(t *testing.T) {
	t.Parallel()

	eng := newEngineFixture(t)
	writeHomeCountryPayload(t, eng.outputDir(), "alpha", "geolite2_country", []CountryValue{{Code: "US", Value: 7}})
	writeOutputViewASNPayload(t, eng.outputDir(), "alpha", "iptoasn", []topASNRow{{ASN: 13335, Name: "CLOUDFLARENET", Count: 11}})

	view := newCachedEntityOutputViewWithRuntime(eng, eng.Runtime(), "")
	if _, err := view.countryComparison("alpha", "geolite2_country"); err != nil {
		t.Fatalf("countryComparison: %v", err)
	}
	if _, err := view.topASNsWithError("alpha", "iptoasn"); err != nil {
		t.Fatalf("topASNsWithError: %v", err)
	}
	if len(view.countryCache) != 1 || len(view.asnCache) != 1 {
		t.Fatalf("cached entityOutputView cache entries: countries=%d asns=%d; want 1 each", len(view.countryCache), len(view.asnCache))
	}

	writeHomeCountryPayload(t, eng.outputDir(), "alpha", "geolite2_country", []CountryValue{{Code: "US", Value: 99}})
	writeOutputViewASNPayload(t, eng.outputDir(), "alpha", "iptoasn", []topASNRow{{ASN: 13335, Name: "CLOUDFLARENET", Count: 99}})

	countryPayload, err := view.countryComparison("alpha", "geolite2_country")
	if err != nil {
		t.Fatalf("countryComparison after overwrite: %v", err)
	}
	if countryPayload == nil || len(countryPayload.Countries) != 1 || countryPayload.Countries[0].Value != 7 {
		t.Fatalf("cached countryComparison payload = %+v; want original value 7", countryPayload)
	}
	asnRows, err := view.topASNsWithError("alpha", "iptoasn")
	if err != nil {
		t.Fatalf("topASNsWithError after overwrite: %v", err)
	}
	if len(asnRows) != 1 || asnRows[0].Count != 11 {
		t.Fatalf("cached ASN rows = %+v; want original count 11", asnRows)
	}
}

func writeOutputViewASNPayload(t *testing.T, dir, feed, provider string, rows []topASNRow) {
	t.Helper()

	payload := struct {
		ByASN []struct {
			ASN   uint32 `json:"asn"`
			Name  string `json:"name"`
			Count uint64 `json:"count"`
		} `json:"by_asn"`
	}{}
	for _, row := range rows {
		payload.ByASN = append(payload.ByASN, struct {
			ASN   uint32 `json:"asn"`
			Name  string `json:"name"`
			Count uint64 `json:"count"`
		}{ASN: row.ASN, Name: row.Name, Count: row.Count})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, feed+"_asn_"+provider+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
