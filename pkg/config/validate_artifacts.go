package config

import "fmt"

func validateArtifacts(cfg *Config) error {
	for name, artifact := range cfg.Artifacts {
		if err := validateArtifact(cfg, name, artifact); err != nil {
			return err
		}
	}
	return nil
}

func validateArtifact(cfg *Config, name string, artifact *Artifact) error {
	if artifact == nil {
		return fmt.Errorf("artifact %q is nil", name)
	}
	if !validFeedName(name) {
		return fmt.Errorf("artifact name %q contains invalid characters (path separators, commas, control chars, or non-ASCII)", name)
	}
	if _, ok := cfg.Sources[name]; ok {
		return fmt.Errorf("artifact %q collides with source %q", name, name)
	}
	if artifact.Frequency < 0 {
		return fmt.Errorf("artifact %q has invalid frequency %d", name, artifact.Frequency)
	}
	if artifact.MaxDownloadSize < -1 {
		return fmt.Errorf("artifact %q has invalid max_download_size %d", name, artifact.MaxDownloadSize)
	}
	if _, ok := validArtifactTypes[artifact.Type]; !ok {
		return fmt.Errorf("artifact %q has unknown type %q", name, artifact.Type)
	}
	return nil
}
