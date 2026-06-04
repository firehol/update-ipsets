package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIndexFeedEntityJointSidecarProvidesConstantTimePatchRows(t *testing.T) {
	sidecar := &feedEntitySidecar{
		Feed: "sample",
		Countries: []feedEntityCountryContribution{
			{
				Code:          "us",
				AttributedIPs: 30,
				ASNs: []feedEntityJointASN{
					{ASN: 13335, Name: "Cloudflare", Count: 10},
					{ASN: 15169, Name: "Google", Count: 20},
				},
			},
			{
				Code:          "DE",
				AttributedIPs: 5,
				ASNs: []feedEntityJointASN{
					{ASN: 13335, Name: "Cloudflare", Count: 5},
					{ASN: 0, Count: 99},
				},
			},
		},
		ASNs: []feedEntityASNContribution{
			{ASN: 13335, Name: "Cloudflare", AttributedIPs: 15},
			{ASN: 15169, Name: "Google", AttributedIPs: 20},
		},
	}

	index := indexFeedEntitySidecar(sidecar)
	gotCountry, ok := index.countryContribution("US")
	if !ok {
		t.Fatal("expected US country contribution")
	}
	if got, want := gotCountry.ASNs, sidecar.Countries[0].ASNs; !slices.Equal(got, want[:2]) {
		t.Fatalf("unexpected US country rows: got %#v want %#v", got, want[:2])
	}

	gotASNRows := index.asnCountries(13335)
	wantASNRows := []asnCountryDeltaRow{{code: "US", count: 10}, {code: "DE", count: 5}}
	if !slices.Equal(gotASNRows, wantASNRows) {
		t.Fatalf("unexpected ASN rows: got %#v want %#v", gotASNRows, wantASNRows)
	}
}

func TestFeedEntitySidecarTargetDetection(t *testing.T) {
	sidecar := &feedEntitySidecar{
		Countries: []feedEntityCountryContribution{
			{Code: " us ", AttributedIPs: 10},
			{Code: "", AttributedIPs: 1},
			{Code: "DE", AttributedIPs: 5},
		},
		ASNs: []feedEntityASNContribution{
			{ASN: 0, AttributedIPs: 1},
			{ASN: 13335, AttributedIPs: 15},
			{ASN: 15169, AttributedIPs: 20},
		},
	}

	countryTargets := map[string]struct{}{"US": {}}
	if !feedEntitySidecarHasCountries(sidecar, countryTargets) {
		t.Fatal("expected country target detection to match normalized US code")
	}
	if feedEntitySidecarHasCountries(sidecar, map[string]struct{}{"FR": {}}) {
		t.Fatal("unexpected country target match")
	}
	if feedEntitySidecarHasCountries(nil, countryTargets) {
		t.Fatal("nil sidecar should not match country targets")
	}

	asnTargets := map[uint32]struct{}{13335: {}}
	if !feedEntitySidecarHasASNs(sidecar, asnTargets) {
		t.Fatal("expected ASN target detection to match 13335")
	}
	if feedEntitySidecarHasASNs(sidecar, map[uint32]struct{}{64512: {}}) {
		t.Fatal("unexpected ASN target match")
	}
	if feedEntitySidecarHasASNs(nil, asnTargets) {
		t.Fatal("nil sidecar should not match ASN targets")
	}
}

func TestLoadFeedEntitySidecarAcceptsLegacyMembershipArrays(t *testing.T) {
	eng := newEngineFixture(t)
	path := filepath.Join(eng.entityFeedsDir(), "sample.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
		"feed": "sample",
		"countries": ["us", "DE", ""],
		"asns": [13335, 15169, 0]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	sidecar, err := eng.loadCommittedFeedEntitySidecar("sample")
	if err != nil {
		t.Fatal(err)
	}
	if !sidecar.legacy {
		t.Fatal("expected legacy sidecar marker")
	}
	if got, want := sidecar.countryCodes(), []string{"DE", "US"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected country codes: got %#v want %#v", got, want)
	}
	if got, want := sidecar.asnNumbers(), []uint32{13335, 15169}; !slices.Equal(got, want) {
		t.Fatalf("unexpected ASNs: got %#v want %#v", got, want)
	}
}

func TestBuildFeedEntityDeltaFallsBackForLegacyCommittedSidecar(t *testing.T) {
	eng := newEngineFixture(t)
	committedPath := filepath.Join(eng.entityFeedsDir(), "sample.json")
	pendingPath := filepath.Join(eng.entityFeedPendingDir(), "sample.json")
	for _, path := range []string{committedPath, pendingPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(committedPath, []byte(`{
		"feed": "sample",
		"countries": ["US"],
		"asns": [13335]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pendingPath, []byte(`{
		"feed": "sample",
		"countries": [{"code":"US","attributed_ips":10}],
		"asns": [{"asn":13335,"attributed_ips":10}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := eng.buildFeedEntityDelta("sample")
	if !errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
		t.Fatalf("expected full rebuild fallback, got %v", err)
	}
}

func TestBuildFeedEntityDeltaPreservesUnreadablePendingSidecarError(t *testing.T) {
	eng := newEngineFixture(t)
	pendingPath := filepath.Join(eng.entityFeedPendingDir(), "sample.json")
	if err := os.MkdirAll(filepath.Dir(pendingPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pendingPath, []byte(`{"feed":`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := eng.buildFeedEntityDelta("sample")
	if !errors.Is(err, errEntitySurgicalNeedsFullRebuild) {
		t.Fatalf("expected full rebuild fallback, got %v", err)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected JSON syntax error in chain, got %v", err)
	}
}
