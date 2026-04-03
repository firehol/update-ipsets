package markdown

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (r *FeedArtifactReader) readInsights(name string) (*insightsPayload, error) {
	data, err := os.ReadFile(r.path(name + "_insights.json"))
	if err != nil {
		return nil, err
	}
	var payload insightsPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

type insightsPayload struct {
	Items []struct {
		Code        string         `json:"code"`
		Section     string         `json:"section"`
		Headline    string         `json:"headline"`
		Evidence    map[string]any `json:"evidence"`
		Methodology string         `json:"methodology"`
	} `json:"items"`
}

func (r *FeedArtifactReader) readCritical(name string) (*CriticalContext, error) {
	data, err := os.ReadFile(r.path(name + "_critical_infrastructure.json"))
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	ctx := &CriticalContext{
		FeedIPs:     uintVal(raw["feed_ips"]),
		CriticalIPs: uintVal(raw["critical_ips"]),
		Percent:     float64Val(raw["percent"]),
		Complete:    boolVal(raw["complete"]),
	}

	if tiers, ok := raw["tiers"].([]any); ok {
		for _, t := range tiers {
			if m, ok := t.(map[string]any); ok {
				ctx.Tiers = append(ctx.Tiers, CriticalTierContext{
					Tier:        strVal(m["tier"]),
					CriticalIPs: uintVal(m["critical_ips"]),
					Percent:     float64Val(m["percent"]),
					Providers:   intVal(m["providers"]),
				})
			}
		}
	}

	if providers, ok := raw["providers"].([]any); ok {
		for _, p := range providers {
			if m, ok := p.(map[string]any); ok {
				ctx.Providers = append(ctx.Providers, CriticalProviderContext{
					Name:        criticalProviderDisplayName(m["provider"]),
					FeedIPs:     uintVal(m["feed_ips"]),
					CriticalIPs: uintVal(m["critical_ips"]),
					Percent:     float64Val(m["percent"]),
				})
			}
		}
	}

	if asnCtx, ok := raw["asn_context"].(map[string]any); ok {
		ctx.ASNContext = &CriticalASNContext{
			Provider: strVal(asnCtx["provider"]),
			FeedIPs:  uintVal(asnCtx["feed_ips"]),
			IPs:      uintVal(asnCtx["ips"]),
			Percent:  float64Val(asnCtx["percent"]),
		}
		if matches, ok := asnCtx["matches"].([]any); ok {
			for _, m := range matches {
				if mm, ok := m.(map[string]any); ok {
					ctx.ASNContext.Matches = append(ctx.ASNContext.Matches, CriticalASNMatch{
						ASN:     uint32Val(mm["asn"]),
						Name:    strVal(mm["name"]),
						Tier:    strVal(mm["tier"]),
						Role:    strVal(mm["role"]),
						IPs:     uintVal(mm["ips"]),
						Percent: float64Val(mm["percent"]),
					})
				}
			}
		}
	}

	return ctx, nil
}

func criticalProviderDisplayName(v any) string {
	if provider, ok := v.(map[string]any); ok {
		for _, field := range []string{"label", "name", "maintainer"} {
			if value := strings.TrimSpace(strVal(provider[field])); value != "" {
				return value
			}
		}
		return "unknown provider"
	}
	return strVal(v)
}

func (r *FeedArtifactReader) readASNProviders(name string) ([]ASNProviderContext, error) {
	if r.preferredASNProvider != "" {
		pctx, err := r.readASNProviderFile(r.path(fmt.Sprintf("%s_asn_%s.json", name, r.preferredASNProvider)))
		if err != nil {
			return nil, err
		}
		return []ASNProviderContext{pctx}, nil
	}

	pattern := name + "_asn_*.json"
	matches, err := filepath.Glob(r.path(pattern))
	if err != nil || len(matches) == 0 {
		return nil, err
	}

	var result []ASNProviderContext
	for _, p := range matches {
		pctx, err := r.readASNProviderFile(p)
		if err != nil {
			continue
		}
		result = append(result, pctx)
	}
	return result, nil
}

func (r *FeedArtifactReader) readASNProviderFile(path string) (ASNProviderContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ASNProviderContext{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return ASNProviderContext{}, err
	}

	pctx := ASNProviderContext{
		Provider:      strVal(raw["provider"]),
		FeedIPs:       uintVal(raw["feed_ips"]),
		AttributedIPs: uintVal(raw["attributed_ips"]),
		BogonIPs:      uintVal(raw["bogon_ips"]),
		UnknownIPs:    uintVal(raw["unknown_ips"]),
	}

	if byASN, ok := raw["by_asn"].([]any); ok {
		entries := make([]CappedEntry, 0, len(byASN))
		for _, a := range byASN {
			if m, ok := a.(map[string]any); ok {
				name := fmt.Sprintf("AS%d %s", uint32Val(m["asn"]), strVal(m["name"]))
				entries = append(entries, CappedEntry{
					Name:  name,
					Value: uintVal(m["count"]),
				})
			}
		}
		pctx.TopASNs = TopN(entries, 50)
	}

	return pctx, nil
}

func (r *FeedArtifactReader) readGEOProviders(name string, meta map[string]any) ([]GEOProviderContext, error) {
	geoMap, ok := meta["geo"].(map[string]any)
	if !ok {
		return nil, nil
	}

	var result []GEOProviderContext
	providers := make([]string, 0, len(geoMap))
	if r.preferredGEOProvider != "" {
		providers = append(providers, r.preferredGEOProvider)
	} else {
		for provider := range geoMap {
			providers = append(providers, provider)
		}
		slices.Sort(providers)
	}

	for _, provider := range providers {
		filename, ok := geoMap[provider]
		if !ok {
			continue
		}
		fname, ok := filename.(string)
		if !ok {
			continue
		}
		data, err := os.ReadFile(r.path(fname))
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		pctx := GEOProviderContext{
			Provider:    provider,
			TotalMapped: uintVal(raw["total_mapped"]),
		}

		if countries, ok := raw["countries"].([]any); ok {
			entries := make([]CappedEntry, 0, len(countries))
			for _, c := range countries {
				if m, ok := c.(map[string]any); ok {
					label := fmt.Sprintf("%s (%s)", strVal(m["name"]), strVal(m["code"]))
					entries = append(entries, CappedEntry{
						Name:  label,
						Value: uintVal(m["value"]),
					})
				}
			}
			pctx.TopCountries = TopN(entries, 50)
		}

		result = append(result, pctx)
	}
	return result, nil
}

func (r *FeedArtifactReader) readBogonProviders(name string) ([]BogonProviderContext, error) {
	pattern := name + "_bogons_*.json"
	matches, err := filepath.Glob(r.path(pattern))
	if err != nil || len(matches) == 0 {
		return nil, err
	}

	var result []BogonProviderContext
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		pctx := BogonProviderContext{
			Provider: strVal(raw["provider"]),
			FeedIPs:  uintVal(raw["feed_ips"]),
			BogonIPs: uintVal(raw["bogon_ips"]),
			Percent:  float64Val(raw["percent"]),
		}

		if byRange, ok := raw["by_range"].([]any); ok {
			for _, br := range byRange {
				if m, ok := br.(map[string]any); ok {
					pctx.Ranges = append(pctx.Ranges, BogonRange{
						CIDR:  strVal(m["cidr"]),
						Name:  strVal(m["name"]),
						RFC:   strVal(m["rfc"]),
						Count: uintVal(m["count"]),
					})
				}
			}
		}

		result = append(result, pctx)
	}
	return result, nil
}
