package config

import (
	"strings"
	"testing"
)

func TestValidateArtifactContract(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func() *Config
		wantErr string
	}{
		{
			name: "valid artifact",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["dronebl"] = &Artifact{Type: ArtifactTypeDroneBLBuildzone, Frequency: 60, MaxDownloadSize: -1}
				return cfg
			},
		},
		{
			name: "artifact source collision",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["same"] = &Artifact{Type: ArtifactTypeDroneBLBuildzone, Frequency: 60}
				cfg.Sources["same"] = &Source{Name: "same", URL: "https://example.test/feed.txt", Frequency: 60, IPV: "ipv4", Output: "ipset"}
				return cfg
			},
			wantErr: "collides with source",
		},
		{
			name: "negative frequency",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["bad"] = &Artifact{Type: ArtifactTypeDroneBLBuildzone, Frequency: -1}
				return cfg
			},
			wantErr: "invalid frequency",
		},
		{
			name: "invalid max download size",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["bad"] = &Artifact{Type: ArtifactTypeDroneBLBuildzone, Frequency: 60, MaxDownloadSize: -2}
				return cfg
			},
			wantErr: "invalid max_download_size",
		},
		{
			name: "unknown type",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["bad"] = &Artifact{Type: "unknown", Frequency: 60}
				return cfg
			},
			wantErr: "unknown type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid artifact config, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q validation error, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateSourceURLContract(t *testing.T) {
	sourceWithURL := func(rawURL string, frequency int) *Source {
		return &Source{
			Name:      "feed",
			URL:       rawURL,
			Frequency: frequency,
			IPV:       "ipv4",
			Output:    "ipset",
		}
	}
	tests := []struct {
		name    string
		cfg     func() *Config
		wantErr string
	}{
		{
			name: "valid file url",
			cfg: func() *Config {
				cfg := New()
				cfg.Sources["feed"] = sourceWithURL("file:///tmp/feed.txt", 60)
				return cfg
			},
		},
		{
			name: "valid artifact-backed source",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["dronebl"] = &Artifact{Type: ArtifactTypeDroneBLBuildzone, Frequency: 60}
				cfg.Sources["feed"] = sourceWithURL("artifact://dronebl?parts=list.txt", 0)
				return cfg
			},
		},
		{
			name: "file url host rejected",
			cfg: func() *Config {
				cfg := New()
				cfg.Sources["feed"] = sourceWithURL("file://example.test/feed.txt", 60)
				return cfg
			},
			wantErr: "host component is not allowed",
		},
		{
			name: "file url requires absolute path",
			cfg: func() *Config {
				cfg := New()
				cfg.Sources["feed"] = sourceWithURL("file:relative.txt", 60)
				return cfg
			},
			wantErr: "absolute path required",
		},
		{
			name: "disallowed scheme",
			cfg: func() *Config {
				cfg := New()
				cfg.Sources["feed"] = sourceWithURL("ftp://example.test/feed.txt", 60)
				return cfg
			},
			wantErr: "disallowed url scheme",
		},
		{
			name: "malformed artifact url",
			cfg: func() *Config {
				cfg := New()
				cfg.Sources["feed"] = sourceWithURL("artifact://dronebl", 0)
				return cfg
			},
			wantErr: "missing parts= query",
		},
		{
			name: "unknown artifact",
			cfg: func() *Config {
				cfg := New()
				cfg.Sources["feed"] = sourceWithURL("artifact://missing?parts=list.txt", 0)
				return cfg
			},
			wantErr: "references unknown artifact",
		},
		{
			name: "artifact-backed source must be generated",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["dronebl"] = &Artifact{Type: ArtifactTypeDroneBLBuildzone, Frequency: 60}
				cfg.Sources["feed"] = sourceWithURL("artifact://dronebl?parts=list.txt", 60)
				return cfg
			},
			wantErr: "must have frequency 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid source URL config, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q validation error, got %v", tt.wantErr, err)
			}
		})
	}
}
