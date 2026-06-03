package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"
)

func (e *Engine) materializeCountryDetail(sidecar *countryDetailSidecar) *CountryDetailPayload {
	return e.materializeCountryDetailWithHealth(sidecar, e.newFeedHealthClassifier())
}

func (e *Engine) materializeCountryDetailWithHealth(sidecar *countryDetailSidecar, health *feedHealthClassifier) *CountryDetailPayload {
	if sidecar == nil {
		return &CountryDetailPayload{}
	}
	feeds := make([]CountryDetailFeed, 0, len(sidecar.Feeds))
	grouped := map[string][]CountryDetailFeed{}
	for _, base := range sidecar.Feeds {
		row := CountryDetailFeed{
			Name:          base.Name,
			Category:      base.Category,
			Provenance:    base.Provenance,
			Maintainer:    base.Maintainer,
			AttributedIPs: base.AttributedIPs,
			UniqueIPs:     base.UniqueIPs,
			HealthClass:   health.class(base.Name),
			LastChangeTS:  base.LastChangeTS,
		}
		feeds = append(feeds, row)
		grouped[row.Category] = append(grouped[row.Category], row)
	}
	for category, rows := range grouped {
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].AttributedIPs != rows[j].AttributedIPs {
				return rows[i].AttributedIPs > rows[j].AttributedIPs
			}
			return rows[i].Name < rows[j].Name
		})
		grouped[category] = rows
	}
	return &CountryDetailPayload{
		Code:            sidecar.Code,
		Provider:        sidecar.Provider,
		ASNProvider:     sidecar.ASNProvider,
		Totals:          sidecar.Totals,
		Feeds:           feeds,
		FeedsByCategory: grouped,
		TopCategories:   sidecar.TopCategories,
		TopMaintainers:  sidecar.TopMaintainers,
		TopASNs:         sidecar.TopASNs,
	}
}

func (e *Engine) materializeASNDetail(sidecar *asnDetailSidecar) *ASNDetailPayload {
	return e.materializeASNDetailWithHealth(sidecar, e.newFeedHealthClassifier())
}

func (e *Engine) materializeASNDetailWithHealth(sidecar *asnDetailSidecar, health *feedHealthClassifier) *ASNDetailPayload {
	if sidecar == nil {
		return &ASNDetailPayload{}
	}
	feeds := make([]ASNDetailFeed, 0, len(sidecar.Feeds))
	grouped := map[string][]ASNDetailFeed{}
	for _, base := range sidecar.Feeds {
		row := ASNDetailFeed{
			Name:          base.Name,
			Category:      base.Category,
			Provenance:    base.Provenance,
			Maintainer:    base.Maintainer,
			AttributedIPs: base.AttributedIPs,
			UniqueIPs:     base.UniqueIPs,
			HealthClass:   health.class(base.Name),
			LastChangeTS:  base.LastChangeTS,
		}
		feeds = append(feeds, row)
		grouped[row.Category] = append(grouped[row.Category], row)
	}
	for category, rows := range grouped {
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].AttributedIPs != rows[j].AttributedIPs {
				return rows[i].AttributedIPs > rows[j].AttributedIPs
			}
			return rows[i].Name < rows[j].Name
		})
		grouped[category] = rows
	}
	return &ASNDetailPayload{
		ASN:                 sidecar.ASN,
		Name:                sidecar.Name,
		Description:         sidecar.Description,
		Provider:            sidecar.Provider,
		GeoProvider:         sidecar.GeoProvider,
		Totals:              sidecar.Totals,
		Feeds:               feeds,
		FeedsByCategory:     grouped,
		TopCategories:       sidecar.TopCategories,
		TopMaintainers:      sidecar.TopMaintainers,
		TopCountries:        sidecar.TopCountries,
		CountryDistribution: sidecar.CountryDistribution,
	}
}

func loadCountryDetailSidecar(path string) (*countryDetailSidecar, error) {
	data, err := readFilePathUnderRoot(filepath.Dir(path), path)
	if err != nil {
		return nil, err
	}
	var sidecar countryDetailSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, err
	}
	return &sidecar, nil
}

func loadASNDetailSidecar(path string) (*asnDetailSidecar, error) {
	data, err := readFilePathUnderRoot(filepath.Dir(path), path)
	if err != nil {
		return nil, err
	}
	var sidecar asnDetailSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, err
	}
	return &sidecar, nil
}

func writeJSONFile(path string, value any) error {
	data, err := jsonMarshalTabIndent(value)
	if err != nil {
		return err
	}
	return writeFileAtomicNoSync(path, append(data, '\n'), generatedFileMode)
}

func writeJSONFileAt(path string, value any, mod time.Time) error {
	if err := writeJSONFile(path, value); err != nil {
		return err
	}
	if mod.IsZero() {
		return nil
	}
	return os.Chtimes(path, mod.UTC(), mod.UTC())
}

func sortedJSONFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		out = append(out, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(out)
	return out, nil
}
