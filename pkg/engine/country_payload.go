package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// loadCountryComparisonPayload accepts both the current Go payload shape
// ({total_mapped,countries}) and the legacy bash payload shape (a raw
// []CountryValue array). The legacy format did not carry a de-duplicated
// total_mapped field, so the fallback computes it as the sum of the
// per-country values. This is an approximation when the source contains
// overlapping country ranges, but it is still far better than treating
// ten years of historical files as unreadable.
func loadCountryComparisonPayload(path string) (*CountryComparisonPayload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeCountryComparisonPayload(data)
}

func decodeCountryComparisonPayload(data []byte) (*CountryComparisonPayload, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		if _, ok := obj["countries"]; !ok {
			if _, ok := obj["total_mapped"]; !ok {
				return nil, fmt.Errorf("country payload object missing countries/total_mapped")
			}
		}
		var payload CountryComparisonPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		if payload.Countries == nil {
			payload.Countries = []CountryValue{}
		}
		return &payload, nil
	}

	var legacy []CountryValue
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	sort.Slice(legacy, func(i, j int) bool {
		return legacy[i].Code < legacy[j].Code
	})
	var totalMapped uint64
	for _, row := range legacy {
		totalMapped += row.Value
	}
	return &CountryComparisonPayload{
		TotalMapped: totalMapped,
		Countries:   legacy,
	}, nil
}
