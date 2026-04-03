package insights

import "fmt"

func init() {
	catalog = append(catalog,
		ruleCountryConcentrated(),
		ruleCountryDiverse(),
		ruleSingleCountry(),
	)
}

// ruleCountryConcentrated (R05) fires when the top three countries
// collectively hold more than 70% of the list. Sample guard: at least
// 100 attributed IPs AND at least 3 countries with any share.
func ruleCountryConcentrated() Rule {
	return Rule{
		Code:    "country_concentrated",
		Name:    "Country concentration",
		Section: SectionComposition,
		MinSamples: func(s SignalSnapshot) bool {
			if s.TotalIPs < 100 {
				return false
			}
			return len(s.TopCountries) >= 3
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			top3 := s.TopCountries[:3]
			total := top3[0].Share + top3[1].Share + top3[2].Share
			if total <= 0.70 {
				return Insight{}, false
			}
			// The single_country rule handles the extreme case; don't
			// double up.
			if top3[0].Share > 0.95 {
				return Insight{}, false
			}
			c1 := countryLabel(top3[0])
			c2 := countryLabel(top3[1])
			c3 := countryLabel(top3[2])
			return Insight{
				Headline: fmt.Sprintf(
					"%s (%s), %s (%s) and %s (%s) account for %s of this list.",
					c1, formatPercent(top3[0].Share),
					c2, formatPercent(top3[1].Share),
					c3, formatPercent(top3[2].Share),
					formatPercent(total),
				),
				Evidence: map[string]any{
					"top1":      top3[0],
					"top2":      top3[1],
					"top3":      top3[2],
					"top3_sum":  total,
					"total_ips": s.TotalIPs,
				},
			}, true
		},
		Methodology: "/methodology/country-concentrated",
	}
}

// ruleCountryDiverse (R06) fires when no single country exceeds 5% AND
// the list spans at least 50 countries. That's the "globally diffuse"
// fingerprint of a large, distributed source population.
func ruleCountryDiverse() Rule {
	return Rule{
		Code:    "country_diverse",
		Name:    "Country diversity",
		Section: SectionComposition,
		MinSamples: func(s SignalSnapshot) bool {
			return s.TotalIPs >= 100
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			if len(s.TopCountries) < 50 {
				return Insight{}, false
			}
			for _, c := range s.TopCountries {
				if c.Share >= 0.05 {
					return Insight{}, false
				}
			}
			return Insight{
				Headline: fmt.Sprintf(
					"No single country exceeds 5%%; this list spans %d countries.",
					len(s.TopCountries),
				),
				Evidence: map[string]any{
					"n_countries": len(s.TopCountries),
					"total_ips":   s.TotalIPs,
					"top_share":   s.TopCountries[0].Share,
				},
			}, true
		},
		Methodology: "/methodology/country-diverse",
	}
}

// ruleSingleCountry (R07) fires when a single country holds more than
// 95% of the list. The extreme-concentration case: a country-specific
// feed, a single-ASN incident, or a narrow honeypot deployment.
func ruleSingleCountry() Rule {
	return Rule{
		Code:    "single_country",
		Name:    "Single country",
		Section: SectionComposition,
		MinSamples: func(s SignalSnapshot) bool {
			return s.TotalIPs >= 100 && len(s.TopCountries) >= 1
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			top := s.TopCountries[0]
			if top.Share <= 0.95 {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"%s alone holds %s of this list.",
					countryLabel(top), formatPercent(top.Share),
				),
				Evidence: map[string]any{
					"country":   top,
					"total_ips": s.TotalIPs,
				},
			}, true
		},
		Methodology: "/methodology/single-country",
	}
}

// countryLabel returns a display label for a CountryShare, preferring
// the country name when present and falling back to the ISO code. This
// keeps headlines readable across providers that do not ship full
// country names.
func countryLabel(c CountryShare) string {
	if c.Name != "" {
		return c.Name
	}
	return c.Code
}
