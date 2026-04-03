package config

import "testing"

func TestValidateRejectsNegativeWebArtifactCacheControls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RuntimeConfig)
	}{
		{
			name: "entries",
			mutate: func(rt *RuntimeConfig) {
				rt.WebArtifactCacheMaxEntries = -1
			},
		},
		{
			name: "bytes",
			mutate: func(rt *RuntimeConfig) {
				rt.WebArtifactCacheMaxBytes = -1
			},
		},
		{
			name: "file bytes",
			mutate: func(rt *RuntimeConfig) {
				rt.WebArtifactCacheMaxFileBytes = -1
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := New()
			tc.mutate(&cfg.Runtime)
			if err := Validate(cfg); err == nil {
				t.Fatal("expected validation error for negative web artifact cache control")
			}
		})
	}
}
