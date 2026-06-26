package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (e *Engine) patchCountryIndex(webBatch *webPublishBatch, updates map[string]*countryDetailSidecar) error {
	return e.patchCountryIndexWithSnapshot(e.operationSnapshot(), webBatch, updates)
}

func (e *Engine) patchCountryIndexWithSnapshot(snap operationSnapshot, webBatch *webPublishBatch, updates map[string]*countryDetailSidecar) error {
	payload := e.emptyCountryIndexPayloadWithSnapshot(snap)
	start := time.Now()
	outDir := outputDirForRuntime(snap.runtime)
	data, err := readFileInRoot(outDir, e.publicCountryIndexRelPath())
	if err == nil {
		e.observeRunCounter("entity.refresh.country_index_read", 1, int64(len(data)))
		e.observeRunOperation("entity.refresh.country_index_read", time.Since(start))
		if err := json.Unmarshal(data, payload); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	rows := make(map[string]CountryIndexEntry, len(payload.Countries)+len(updates))
	for _, row := range payload.Countries {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code != "" {
			row.Code = code
			rows[code] = row
		}
	}
	for code, sidecar := range updates {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		if sidecar == nil {
			delete(rows, code)
			continue
		}
		rows[code] = CountryIndexEntry{
			Code:          code,
			FeedCount:     sidecar.Totals.FeedsMatching,
			AttributedIPs: sidecar.Totals.AttributedIPsInFeed,
		}
	}
	payload.Provider = e.emptyCountryIndexPayloadWithSnapshot(snap).Provider
	payload.Countries = make([]CountryIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.Countries = append(payload.Countries, row)
	}
	sort.Slice(payload.Countries, func(i, j int) bool {
		if payload.Countries[i].FeedCount != payload.Countries[j].FeedCount {
			return payload.Countries[i].FeedCount > payload.Countries[j].FeedCount
		}
		if payload.Countries[i].AttributedIPs != payload.Countries[j].AttributedIPs {
			return payload.Countries[i].AttributedIPs > payload.Countries[j].AttributedIPs
		}
		return payload.Countries[i].Code < payload.Countries[j].Code
	})
	return e.writeObservedJSONFile(filepath.Join(webBatch.stageDir, e.publicCountryIndexRelPath()), payload, "entity.refresh.country_index_write")
}

func (e *Engine) patchASNIndex(webBatch *webPublishBatch, updates map[uint32]*asnDetailSidecar) error {
	return e.patchASNIndexWithSnapshot(e.operationSnapshot(), webBatch, updates)
}

func (e *Engine) patchASNIndexWithSnapshot(snap operationSnapshot, webBatch *webPublishBatch, updates map[uint32]*asnDetailSidecar) error {
	payload := e.emptyASNIndexPayloadWithSnapshot(snap)
	start := time.Now()
	outDir := outputDirForRuntime(snap.runtime)
	data, err := readFileInRoot(outDir, e.publicASNIndexRelPath())
	if err == nil {
		e.observeRunCounter("entity.refresh.asn_index_read", 1, int64(len(data)))
		e.observeRunOperation("entity.refresh.asn_index_read", time.Since(start))
		if err := json.Unmarshal(data, payload); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	rows := make(map[uint32]ASNIndexEntry, len(payload.ASNs)+len(updates))
	for _, row := range payload.ASNs {
		if row.ASN != 0 {
			rows[row.ASN] = row
		}
	}
	for asn, sidecar := range updates {
		if asn == 0 {
			continue
		}
		if sidecar == nil {
			delete(rows, asn)
			continue
		}
		rows[asn] = ASNIndexEntry{
			ASN:           asn,
			Name:          sidecar.Name,
			FeedCount:     sidecar.Totals.FeedsMatching,
			AttributedIPs: sidecar.Totals.AttributedIPs,
		}
	}
	payload.Provider = e.emptyASNIndexPayloadWithSnapshot(snap).Provider
	payload.ASNs = make([]ASNIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.ASNs = append(payload.ASNs, row)
	}
	sort.Slice(payload.ASNs, func(i, j int) bool {
		if payload.ASNs[i].FeedCount != payload.ASNs[j].FeedCount {
			return payload.ASNs[i].FeedCount > payload.ASNs[j].FeedCount
		}
		if payload.ASNs[i].AttributedIPs != payload.ASNs[j].AttributedIPs {
			return payload.ASNs[i].AttributedIPs > payload.ASNs[j].AttributedIPs
		}
		return payload.ASNs[i].ASN < payload.ASNs[j].ASN
	})
	return e.writeObservedJSONFile(filepath.Join(webBatch.stageDir, e.publicASNIndexRelPath()), payload, "entity.refresh.asn_index_write")
}

func (e *Engine) emptyCountryIndexPayload() *CountryIndexPayload {
	return e.emptyCountryIndexPayloadWithSnapshot(e.operationSnapshot())
}

func (e *Engine) emptyCountryIndexPayloadWithSnapshot(snap operationSnapshot) *CountryIndexPayload {
	provider := preferredGeoProviderForConfig(snap.cfg)
	return &CountryIndexPayload{
		Provider: HomeSummaryProvider{
			Name:  provider,
			Label: providerDisplayLabel(lookupSourceForConfig(snap.cfg, provider)),
		},
	}
}

func (e *Engine) emptyASNIndexPayload() *ASNIndexPayload {
	return e.emptyASNIndexPayloadWithSnapshot(e.operationSnapshot())
}

func (e *Engine) emptyASNIndexPayloadWithSnapshot(snap operationSnapshot) *ASNIndexPayload {
	provider := preferredASNProviderForConfig(snap.cfg)
	return &ASNIndexPayload{
		Provider: HomeSummaryProvider{
			Name:  provider,
			Label: providerDisplayLabel(lookupSourceForConfig(snap.cfg, provider)),
		},
	}
}
