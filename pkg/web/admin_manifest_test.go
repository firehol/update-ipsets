package web

import (
	"path/filepath"
	"strings"
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

func TestBuildFeedManifestUsesProviderSourceForDatabaseFeeds(t *testing.T) {
	eng, _ := testHandlerWithProviderCatalog(t, Options{EnableAll: true})
	cfg := eng.Config()

	cases := []struct {
		name       string
		wantSuffix string
	}{
		{name: "geodb", wantSuffix: "/geolocation/geodb.source"},
		{name: "asndb", wantSuffix: "/asn/asndb/source"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			src := cfg.Sources[tc.name]
			if src == nil {
				t.Fatalf("%s source missing", tc.name)
			}
			resp := buildFeedManifest(tc.name, src, cfg, eng.Runtime(), eng)
			providerSource := requireManifestKind(t, resp, "provider_source")
			if !providerSource.Required {
				t.Fatalf("provider_source required = false, want true")
			}
			if !strings.HasSuffix(filepath.ToSlash(providerSource.Path), tc.wantSuffix) {
				t.Fatalf("provider_source path = %q, want suffix %q", providerSource.Path, tc.wantSuffix)
			}
			for _, forbidden := range []string{"canonical", "metadata", "binary"} {
				if file, ok := manifestKind(resp, forbidden); ok {
					t.Fatalf("database manifest unexpectedly includes %s file: %+v", forbidden, file)
				}
			}
		})
	}
}

func requireManifestKind(t *testing.T, resp ManifestResponse, kind string) ManifestFile {
	t.Helper()
	file, ok := manifestKind(resp, kind)
	if !ok {
		t.Fatalf("manifest missing %s file", kind)
	}
	return file
}

func manifestKind(resp ManifestResponse, kind string) (ManifestFile, bool) {
	for _, file := range resp.Files {
		if file.Kind == kind {
			return file, true
		}
	}
	return ManifestFile{}, false
}
