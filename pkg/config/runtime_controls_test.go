package config

import "testing"

func TestValidateRejectsNegativeRuntimeResourceControls(t *testing.T) {
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
			name: "parallel downloads",
			mutate: func(rt *RuntimeConfig) {
				rt.ParallelDownloads = -1
			},
		},
		{
			name: "download error suppression",
			mutate: func(rt *RuntimeConfig) {
				rt.IgnoreRepeatingDownloadErrors = -1
			},
		},
		{
			name: "parallel dns queries",
			mutate: func(rt *RuntimeConfig) {
				rt.ParallelDNSQueries = -1
			},
		},
		{
			name: "ingest workers",
			mutate: func(rt *RuntimeConfig) {
				rt.MaxIngestWorkers = -1
			},
		},
		{
			name: "processing workers",
			mutate: func(rt *RuntimeConfig) {
				rt.MaxProcessingWorkers = -1
			},
		},
		{
			name: "heavy phase workers",
			mutate: func(rt *RuntimeConfig) {
				rt.MaxHeavyPhaseWorkers = -1
			},
		},
		{
			name: "background workers",
			mutate: func(rt *RuntimeConfig) {
				rt.MaxBackgroundWorkers = -1
			},
		},
		{
			name: "minimum run interval",
			mutate: func(rt *RuntimeConfig) {
				rt.MinRunIntervalSeconds = -1
			},
		},
		{
			name: "processing interval",
			mutate: func(rt *RuntimeConfig) {
				rt.ProcessingIntervalMinutes = -1
			},
		},
		{
			name: "push to git timeout",
			mutate: func(rt *RuntimeConfig) {
				rt.PushToGitTimeout = -1
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
				t.Fatal("expected validation error for negative runtime resource control")
			}
		})
	}
}

func TestValidateAcceptsZeroRuntimeResourceControls(t *testing.T) {
	cfg := New()
	cfg.Runtime.WebArtifactCacheMaxEntries = 0
	cfg.Runtime.WebArtifactCacheMaxBytes = 0
	cfg.Runtime.WebArtifactCacheMaxFileBytes = 0
	cfg.Runtime.ParallelDownloads = 0
	cfg.Runtime.IgnoreRepeatingDownloadErrors = 0
	cfg.Runtime.ParallelDNSQueries = 0
	cfg.Runtime.MaxIngestWorkers = 0
	cfg.Runtime.MaxProcessingWorkers = 0
	cfg.Runtime.MaxHeavyPhaseWorkers = 0
	cfg.Runtime.MaxBackgroundWorkers = 0
	cfg.Runtime.MinRunIntervalSeconds = 0
	cfg.Runtime.ProcessingIntervalMinutes = 0
	cfg.Runtime.PushToGitTimeout = 0
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() with zero runtime resource controls error = %v", err)
	}
}
