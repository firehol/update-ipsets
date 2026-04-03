package config

import "fmt"

type Provenance string

const (
	ProvenancePrimary            Provenance = "primary"
	ProvenanceSecondaryUpstream  Provenance = "secondary_upstream"
	ProvenanceSecondaryMerge     Provenance = "secondary_merge"
	ProvenanceSecondaryRetention Provenance = "secondary_retention"
)

func (p Provenance) Valid() bool {
	switch p {
	case ProvenancePrimary,
		ProvenanceSecondaryUpstream,
		ProvenanceSecondaryMerge,
		ProvenanceSecondaryRetention:
		return true
	default:
		return false
	}
}

func NormalizeProvenance(raw string) (Provenance, error) {
	if raw == "" {
		return ProvenancePrimary, nil
	}
	p := Provenance(raw)
	if !p.Valid() {
		return "", fmt.Errorf("invalid provenance %q", raw)
	}
	return p, nil
}
