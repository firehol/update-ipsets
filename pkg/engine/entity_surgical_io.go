package engine

import (
	"errors"
	"os"
	"strings"
	"time"
)

func (e *Engine) entityArtifactsContainFeedWithPresence(name string, presence *entityArtifactFeedPresence) (bool, error) {
	if presence != nil {
		return presence.contains(name)
	}
	return newEntityArtifactFeedPresence(e).contains(name)
}

type entityArtifactFeedPresence struct {
	e          *Engine
	complete   bool
	triedIndex bool
	names      map[string]struct{}
}

func newEntityArtifactFeedPresence(e *Engine) *entityArtifactFeedPresence {
	return &entityArtifactFeedPresence{e: e}
}

func (p *entityArtifactFeedPresence) contains(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, nil
	}
	if p == nil || p.e == nil {
		return false, nil
	}
	if p.complete {
		_, ok := p.names[name]
		return ok, nil
	}
	if !p.triedIndex {
		p.triedIndex = true
		names, bytes, err := p.e.loadEntityFeedPresenceIndex()
		if err == nil {
			p.names = names
			p.complete = true
			p.e.observeRunCounter("entity.feed_presence_index_read", 1, bytes)
			_, ok := p.names[name]
			return ok, nil
		}
		if errors.Is(err, errEntityFeedPresenceIndexMiss) {
			p.e.observeRunCounter("entity.feed_presence_index_missing", 1, 0)
		} else {
			p.e.observeRunCounter("entity.feed_presence_index_ignored", 1, bytes)
		}
	}
	return p.scan(name)
}

func (p *entityArtifactFeedPresence) scan(target string) (bool, error) {
	e := p.e
	seen := map[string]struct{}{}
	countryFiles, err := sortedJSONFiles(e.entityCountriesDir())
	if err != nil {
		return false, err
	}
	e.observeRunCounter("entity.repair_feed_scan.country_files", int64(len(countryFiles)), 0)
	for _, path := range countryFiles {
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.repair_feed_scan.country_sidecar_read", 1, bytes)
		sidecar, err := loadCountryDetailSidecar(path)
		if err != nil {
			return false, err
		}
		for _, row := range sidecar.Feeds {
			seen[row.Name] = struct{}{}
			if row.Name == target {
				return true, nil
			}
		}
	}
	asnFiles, err := sortedJSONFiles(e.entityASNsDir())
	if err != nil {
		return false, err
	}
	e.observeRunCounter("entity.repair_feed_scan.asn_files", int64(len(asnFiles)), 0)
	for _, path := range asnFiles {
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.repair_feed_scan.asn_sidecar_read", 1, bytes)
		sidecar, err := loadASNDetailSidecar(path)
		if err != nil {
			return false, err
		}
		for _, row := range sidecar.Feeds {
			seen[row.Name] = struct{}{}
			if row.Name == target {
				return true, nil
			}
		}
	}
	p.names = seen
	p.complete = true
	return false, nil
}

func (e *Engine) writeObservedJSONFile(path string, value any, metric string) error {
	start := time.Now()
	data, err := jsonMarshalTabIndent(value)
	if err != nil {
		return err
	}
	body := append(data, '\n')
	if err := writeFileAtomicNoSync(path, body, generatedFileMode); err != nil {
		return err
	}
	e.observeRunCounter(metric, 1, int64(len(body)))
	e.observeRunOperation(metric, time.Since(start))
	return nil
}

func (e *Engine) writeObservedJSONFileAt(path string, value any, mod time.Time, metric string) error {
	if err := e.writeObservedJSONFile(path, value, metric); err != nil {
		return err
	}
	if mod.IsZero() {
		return nil
	}
	return os.Chtimes(path, mod.UTC(), mod.UTC())
}

func entityDetailFilesExist(privatePath, publicPath string) bool {
	if _, err := os.Stat(privatePath); err != nil {
		return false
	}
	if _, err := os.Stat(publicPath); err != nil {
		return false
	}
	return true
}

func (e *Engine) emptyCountryDetailSidecar(code string) *countryDetailSidecar {
	geoProvider := e.preferredGeoProvider()
	asnProvider := e.preferredASNProvider()
	return &countryDetailSidecar{
		Code: strings.ToUpper(strings.TrimSpace(code)),
		Provider: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(e.lookupSource(geoProvider)),
		},
		ASNProvider: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(e.lookupSource(asnProvider)),
		},
	}
}

func (e *Engine) emptyASNDetailSidecar(asn uint32) *asnDetailSidecar {
	asnProvider := e.preferredASNProvider()
	geoProvider := e.preferredGeoProvider()
	return &asnDetailSidecar{
		ASN: asn,
		Provider: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(e.lookupSource(asnProvider)),
		},
		GeoProvider: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(e.lookupSource(geoProvider)),
		},
	}
}
