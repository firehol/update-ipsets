package web

import (
	"path/filepath"
	"testing"
)

func TestBuildFeedManifestRequiresConfiguredProviderFanOutArtifacts(t *testing.T) {
	eng, _ := testHandlerWithProviderCatalog(t, Options{EnableAll: true})
	cfg := eng.Config()
	src := cfg.Sources["sample"]
	if src == nil {
		t.Fatal("sample source missing")
	}

	resp := buildFeedManifest("sample", src, cfg, eng.Runtime(), eng)

	assertManifestRequiredFile(t, resp, "geo", "geodb", "sample_geodb.json")
	assertManifestRequiredFile(t, resp, "asn", "asndb", "sample_asn_asndb.json")
}

func assertManifestRequiredFile(t *testing.T, resp ManifestResponse, kind, provider, suffix string) {
	t.Helper()

	for _, file := range resp.Files {
		if file.Kind != kind || file.Provider != provider {
			continue
		}
		if !file.Required {
			t.Fatalf("%s/%s required = false, want true", kind, provider)
		}
		if got, want := filepath.Base(file.Rel), suffix; got != want {
			t.Fatalf("%s/%s rel basename = %q, want %q", kind, provider, got, want)
		}
		if got, want := filepath.Base(file.Path), suffix; got != want {
			t.Fatalf("%s/%s path basename = %q, want %q", kind, provider, got, want)
		}
		return
	}
	t.Fatalf("manifest missing %s/%s provider artifact", kind, provider)
}
