package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestStreamingSelectedEntityDetailsMatchMapBuilder(t *testing.T) {
	t.Parallel()

	eng := newEngineFixture(t)
	snap := eng.operationSnapshot()
	sidecars := sampleFeedEntitySidecarsForStreamTests()
	countries := map[string]struct{}{"US": {}}
	asns := map[uint32]struct{}{13335: {}}

	mapCountries, mapASNs, err := eng.buildSelectedEntityDetailSidecarsFromFeedSidecars(sidecars, countries, asns, false)
	if err != nil {
		t.Fatalf("map detail build: %v", err)
	}
	streamCountries, streamASNs, err := eng.buildSelectedEntityDetailSidecarsFromFeedSidecarWalker(t.Context(), snap, countries, asns, false, func(visit feedEntitySidecarVisitFunc) error {
		return walkFeedEntitySidecarMap(t.Context(), sidecars, visit)
	})
	if err != nil {
		t.Fatalf("stream detail build: %v", err)
	}

	if !reflect.DeepEqual(streamCountries, mapCountries) {
		t.Fatalf("stream country sidecars differ\nstream=%#v\nmap=%#v", streamCountries, mapCountries)
	}
	if !reflect.DeepEqual(streamASNs, mapASNs) {
		t.Fatalf("stream ASN sidecars differ\nstream=%#v\nmap=%#v", streamASNs, mapASNs)
	}
}

func TestStreamingEntityIndexesMatchMapBuilders(t *testing.T) {
	t.Parallel()

	eng := newEngineFixture(t)
	snap := eng.operationSnapshot()
	sidecars := sampleFeedEntitySidecarsForStreamTests()

	mapCountryIndex := eng.buildCountryIndexFromFeedSidecarsWithSnapshot(snap, sidecars)
	mapASNIndex := eng.buildASNIndexFromFeedSidecarsWithSnapshot(snap, sidecars)
	streamCountryIndex, streamASNIndex, err := eng.buildEntityIndexesFromFeedSidecarWalkerWithSnapshot(t.Context(), snap, func(visit feedEntitySidecarVisitFunc) error {
		return walkFeedEntitySidecarMap(t.Context(), sidecars, visit)
	}, true, true)
	if err != nil {
		t.Fatalf("stream index build: %v", err)
	}

	if !reflect.DeepEqual(streamCountryIndex, mapCountryIndex) {
		t.Fatalf("stream country index differs\nstream=%#v\nmap=%#v", streamCountryIndex, mapCountryIndex)
	}
	if !reflect.DeepEqual(streamASNIndex, mapASNIndex) {
		t.Fatalf("stream ASN index differs\nstream=%#v\nmap=%#v", streamASNIndex, mapASNIndex)
	}
}

func TestStreamingFeedPresenceNamesMatchMapBuilder(t *testing.T) {
	t.Parallel()

	sidecars := sampleFeedEntitySidecarsForStreamTests()
	mapNames := entityFeedPresenceNamesFromSidecars(sidecars)
	streamNames, err := entityFeedPresenceNamesFromSidecarWalker(t.Context(), func(visit feedEntitySidecarVisitFunc) error {
		return walkFeedEntitySidecarMap(t.Context(), sidecars, visit)
	})
	if err != nil {
		t.Fatalf("stream presence names: %v", err)
	}

	if !slices.Equal(streamNames, mapNames) {
		t.Fatalf("stream presence names = %v, want %v", streamNames, mapNames)
	}
}

func TestMergedSidecarWalkerUsesReplacementWithoutDecodingCommittedFile(t *testing.T) {
	t.Parallel()

	eng := newEngineFixture(t)
	rt := eng.Runtime()
	dir := entityFeedsDirForRuntime(rt)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir sidecar dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write invalid replaced sidecar: %v", err)
	}
	if err := writeTestFeedEntitySidecar(filepath.Join(dir, "beta.json"), sampleFeedEntitySidecarsForStreamTests()["beta"]); err != nil {
		t.Fatalf("write beta sidecar: %v", err)
	}

	replacements := map[string]*feedEntitySidecar{
		"alpha": sampleFeedEntitySidecarsForStreamTests()["alpha"],
		"beta":  nil,
		"gamma": sampleFeedEntitySidecarsForStreamTests()["gamma"],
	}
	visited := []string{}
	err := eng.walkMergedFeedEntitySidecarsWithRuntime(t.Context(), rt, replacements, false, func(name string, _ *feedEntitySidecar) error {
		visited = append(visited, name)
		return nil
	})
	if err != nil {
		t.Fatalf("walk merged sidecars: %v", err)
	}

	if want := []string{"alpha", "gamma"}; !slices.Equal(visited, want) {
		t.Fatalf("visited sidecars = %v, want %v", visited, want)
	}
}

func sampleFeedEntitySidecarsForStreamTests() map[string]*feedEntitySidecar {
	return map[string]*feedEntitySidecar{
		"alpha": {
			Feed:         "alpha",
			Category:     "malware",
			Maintainer:   "maint-a",
			UniqueIPs:    100,
			LastChangeTS: 1000,
			Countries: []feedEntityCountryContribution{
				{Code: "US", AttributedIPs: 40, ASNs: []feedEntityJointASN{{ASN: 13335, Name: "Cloudflare", Count: 40}}},
			},
			ASNs: []feedEntityASNContribution{
				{ASN: 13335, Name: "Cloudflare", AttributedIPs: 40},
			},
		},
		"beta": {
			Feed:         "beta",
			Category:     "scanner",
			Maintainer:   "maint-b",
			UniqueIPs:    80,
			LastChangeTS: 2000,
			Countries: []feedEntityCountryContribution{
				{Code: "DE", AttributedIPs: 30, ASNs: []feedEntityJointASN{{ASN: 15169, Name: "Google", Count: 30}}},
			},
			ASNs: []feedEntityASNContribution{
				{ASN: 15169, Name: "Google", AttributedIPs: 30},
			},
		},
		"gamma": {
			Feed:         "gamma",
			Category:     "malware",
			Maintainer:   "maint-a",
			UniqueIPs:    120,
			LastChangeTS: 3000,
			Countries: []feedEntityCountryContribution{
				{Code: "US", AttributedIPs: 60, ASNs: []feedEntityJointASN{{ASN: 13335, Name: "Cloudflare", Count: 60}}},
				{Code: "FR", AttributedIPs: 20, ASNs: []feedEntityJointASN{{ASN: 13335, Name: "Cloudflare", Count: 20}}},
			},
			ASNs: []feedEntityASNContribution{
				{ASN: 13335, Name: "Cloudflare", AttributedIPs: 80},
			},
		},
	}
}

func writeTestFeedEntitySidecar(path string, sidecar *feedEntitySidecar) error {
	data, err := json.Marshal(sidecar)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
