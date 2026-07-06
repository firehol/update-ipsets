package engine

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFullEntityArtifactWriteStagesGeneratedSidecarsWithoutRetainingMap(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	eng := newEngineFixture(t, withConfig(homeAggregateTestConfig()), withNow(func() time.Time { return now }))
	setHomeAggregateTestEntry(eng, "alpha", now)
	eng.state.Entry("alpha").File = "alpha.ipset"
	writeHomeCountryPayload(t, eng.outputDir(), "alpha", "geolite2_country", []CountryValue{{Code: "US", Value: 7}})
	writeOutputViewASNPayload(t, eng.outputDir(), "alpha", "iptoasn", []topASNRow{{ASN: 13335, Name: "CLOUDFLARENET", Count: 11}})

	webBatch, err := eng.newWebPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer webBatch.cleanup()
	entityBatch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatal(err)
	}
	defer entityBatch.cleanup()

	state, err := eng.newEntityArtifactWriteStateWithSnapshot(t.Context(), eng.operationSnapshot(), []string{"alpha"}, true, webBatch.stagedPublishBatch, entityBatch.stagedPublishBatch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.loadProviderReferences(); err != nil {
		t.Fatal(err)
	}
	if err := state.loadFeedSidecars(); err != nil {
		t.Fatal(err)
	}
	if len(state.newSidecars) != 0 {
		t.Fatalf("full rebuild retained %d generated sidecars in memory; want 0", len(state.newSidecars))
	}
	if _, ok := state.newSidecarNames["alpha"]; !ok {
		t.Fatalf("full rebuild did not record generated sidecar name alpha: %+v", state.newSidecarNames)
	}
	if _, ok := state.affectedCountries["US"]; !ok {
		t.Fatalf("full rebuild did not collect affected country US: %+v", state.affectedCountries)
	}
	if _, ok := state.affectedASNs[13335]; !ok {
		t.Fatalf("full rebuild did not collect affected ASN 13335: %+v", state.affectedASNs)
	}

	stagedPath := filepath.Join(entityBatch.stageDir, eng.entityFeedSidecarRelPath("alpha"))
	sidecar, err := eng.loadFeedEntitySidecar(stagedPath)
	if err != nil {
		t.Fatalf("load staged sidecar: %v", err)
	}
	if sidecar == nil || len(sidecar.Countries) != 1 || len(sidecar.ASNs) != 1 {
		t.Fatalf("staged sidecar = %+v; want country and ASN contributions", sidecar)
	}

	walked := map[string]struct{}{}
	if err := state.walkCurrentFeedSidecars(func(name string, sidecar *feedEntitySidecar) error {
		walked[name] = struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("walkCurrentFeedSidecars: %v", err)
	}
	if _, ok := walked["alpha"]; !ok {
		t.Fatalf("full rebuild current walker did not read staged sidecar: %+v", walked)
	}
}
