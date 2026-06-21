package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSurgicalRefreshStagesUnchangedCountryDetailTouchWithoutMutatingLiveFile(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	logical := now.Add(time.Hour)
	oldMTime := now.Add(-time.Hour)

	webBatch, err := eng.newWebPublishBatch()
	if err != nil {
		t.Fatalf("newWebPublishBatch() error = %v", err)
	}
	defer webBatch.cleanup()
	entityBatch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatalf("newEntityPublishBatch() error = %v", err)
	}
	defer entityBatch.cleanup()

	sidecar := &countryDetailSidecar{
		Code: "US",
		Feeds: []countryDetailFeedBase{{
			Name:         "alpha",
			LastChangeTS: logical.Unix(),
		}},
	}
	privatePath := filepath.Join(libDir, "entities", "countries", "US.json")
	publicPath := filepath.Join(webDir, "countries", "US.json")
	writeJSONFileAtForTest(t, privatePath, sidecar, oldMTime)
	writeJSONFileAtForTest(t, publicPath, eng.materializeCountryDetail(sidecar), oldMTime)

	state := &entitySurgicalRefreshState{
		e:         eng,
		ctx:       t.Context(),
		web:       webBatch,
		ent:       entityBatch,
		feedTimes: map[string]time.Time{"alpha": logical},
	}
	if err := state.touchUnchangedCountryDetail("US", sidecar); err != nil {
		t.Fatalf("touchUnchangedCountryDetail() error = %v", err)
	}
	assertFileMTimeForTest(t, privatePath, oldMTime)
	assertFileMTimeForTest(t, publicPath, oldMTime)

	if _, err := entityBatch.publishContext(t.Context()); err != nil {
		t.Fatalf("entity publish error = %v", err)
	}
	if _, err := webBatch.publishContext(t.Context()); err != nil {
		t.Fatalf("web publish error = %v", err)
	}
	assertFileMTimeForTest(t, privatePath, logical)
	assertFileMTimeForTest(t, publicPath, logical)
}

func TestSurgicalRefreshStagesUnchangedASNDetailTouchWithoutMutatingLiveFile(t *testing.T) {
	eng, webDir, libDir := newDetailEngineForTest(t)
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	eng.now = func() time.Time { return now }
	logical := now.Add(time.Hour)
	oldMTime := now.Add(-time.Hour)

	webBatch, err := eng.newWebPublishBatch()
	if err != nil {
		t.Fatalf("newWebPublishBatch() error = %v", err)
	}
	defer webBatch.cleanup()
	entityBatch, err := eng.newEntityPublishBatch()
	if err != nil {
		t.Fatalf("newEntityPublishBatch() error = %v", err)
	}
	defer entityBatch.cleanup()

	sidecar := &asnDetailSidecar{
		ASN: 13335,
		Feeds: []asnDetailFeedBase{{
			Name:         "alpha",
			LastChangeTS: logical.Unix(),
		}},
	}
	privatePath := filepath.Join(libDir, "entities", "asns", "13335.json")
	publicPath := filepath.Join(webDir, "asns", "13335.json")
	writeJSONFileAtForTest(t, privatePath, sidecar, oldMTime)
	writeJSONFileAtForTest(t, publicPath, eng.materializeASNDetail(sidecar), oldMTime)

	state := &entitySurgicalRefreshState{
		e:         eng,
		ctx:       t.Context(),
		web:       webBatch,
		ent:       entityBatch,
		feedTimes: map[string]time.Time{"alpha": logical},
	}
	if err := state.touchUnchangedASNDetail(13335, sidecar); err != nil {
		t.Fatalf("touchUnchangedASNDetail() error = %v", err)
	}
	assertFileMTimeForTest(t, privatePath, oldMTime)
	assertFileMTimeForTest(t, publicPath, oldMTime)

	if _, err := entityBatch.publishContext(t.Context()); err != nil {
		t.Fatalf("entity publish error = %v", err)
	}
	if _, err := webBatch.publishContext(t.Context()); err != nil {
		t.Fatalf("web publish error = %v", err)
	}
	assertFileMTimeForTest(t, privatePath, logical)
	assertFileMTimeForTest(t, publicPath, logical)
}

func writeJSONFileAtForTest(t *testing.T, path string, value any, mod time.Time) {
	t.Helper()
	if err := writeJSONFileAt(path, value, mod); err != nil {
		t.Fatalf("writeJSONFileAt(%q) error = %v", path, err)
	}
}

func assertFileMTimeForTest(t *testing.T, path string, want time.Time) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.ModTime().UTC(); !got.Equal(want.UTC()) {
		t.Fatalf("%s mtime = %s, want %s", path, got, want.UTC())
	}
}
