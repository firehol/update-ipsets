package config

func normalizeCatalogMetadata(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	for name, src := range cfg.Sources {
		if src == nil {
			continue
		}
		src.Name = name
		p, err := NormalizeProvenance(src.ProvenanceRaw)
		if err != nil {
			return err
		}
		src.Provenance = p
	}
	for name, merge := range cfg.Merges {
		if merge == nil {
			continue
		}
		merge.Name = name
		p, err := NormalizeProvenance(merge.ProvenanceRaw)
		if err != nil {
			return err
		}
		merge.Provenance = p
	}
	return nil
}
