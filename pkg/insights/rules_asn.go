package insights

import "fmt"

func init() {
	catalog = append(catalog,
		ruleBogonPresent(),
		ruleInfrastructurePresent(),
	)
}

// ruleBogonPresent (R08) fires when any fraction of the feed is in
// bogon ranges. The headline reports both the absolute count and the
// share, so 42 IPs on a 40M feed still produces an observable number
// instead of rounding to 0%.
func ruleBogonPresent() Rule {
	return Rule{
		Code:    "bogon_present",
		Name:    "Bogon ranges present",
		Section: SectionComposition,
		MinSamples: func(s SignalSnapshot) bool {
			return s.TotalIPs >= 100
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			if s.BogonShare <= 0 {
				return Insight{}, false
			}
			nBogon := uint64(float64(s.TotalIPs)*s.BogonShare + 0.5)
			if nBogon == 0 {
				return Insight{}, false
			}
			return Insight{
				Headline: fmt.Sprintf(
					"%s IPs (%s) are in bogon ranges.",
					formatCount(nBogon), formatPercent(s.BogonShare),
				),
				Evidence: map[string]any{
					"bogon_ips":   nBogon,
					"bogon_share": s.BogonShare,
					"total_ips":   s.TotalIPs,
				},
			}, true
		},
		Methodology: "/methodology/bogon-present",
	}
}

// ruleInfrastructurePresent (R09) fires when any fraction of the feed overlaps
// configured critical-infrastructure reference feeds. Every hit is noteworthy
// because the reference feeds describe operator-sensitive infrastructure, not
// threat indicators.
func ruleInfrastructurePresent() Rule {
	return Rule{
		Code:    "infrastructure_present",
		Name:    "Critical infrastructure overlap present",
		Section: SectionComposition,
		MinSamples: func(s SignalSnapshot) bool {
			return s.TotalIPs > 0
		},
		Compute: func(s SignalSnapshot) (Insight, bool) {
			facts := infrastructureFacts(s)
			if facts.TotalIPs == 0 {
				return Insight{}, false
			}
			if facts.HardIPs > 0 {
				return Insight{
					Headline: fmt.Sprintf(
						"%s hard-tier IPs (%s) overlap critical infrastructure reference feeds.",
						formatCount(facts.HardIPs), formatPercent(facts.HardShare),
					),
					Evidence: map[string]any{
						"tier":              "hard",
						"infra_ips":         facts.HardIPs,
						"infra_share":       facts.HardShare,
						"hard_providers":    facts.HardProviders,
						"total_infra_ips":   facts.TotalIPs,
						"total_infra_share": facts.TotalShare,
						"total_ips":         s.TotalIPs,
					},
				}, true
			}
			if s.TotalIPs < 100 {
				return Insight{}, false
			}
			if facts.TotalIPs < 10 && facts.TotalShare < 0.0001 {
				return Insight{}, false
			}
			label := facts.nonHardLabel()
			if label == "" {
				label = "critical infrastructure"
			}
			return Insight{
				Headline: fmt.Sprintf(
					"%s %s IPs (%s) overlap reference feeds; review with service context.",
					formatCount(facts.TotalIPs), label, formatPercent(facts.TotalShare),
				),
				Evidence: map[string]any{
					"tier":                 label,
					"infra_ips":            facts.TotalIPs,
					"infra_share":          facts.TotalShare,
					"soft_ips":             facts.SoftIPs,
					"soft_providers":       facts.SoftProviders,
					"contextual_ips":       facts.ContextualIPs,
					"contextual_providers": facts.ContextualProviders,
					"total_ips":            s.TotalIPs,
				},
			}, true
		},
		Methodology: "/methodology/infrastructure-present",
	}
}

type infrastructureSummary struct {
	TotalIPs            uint64
	TotalShare          float64
	HardIPs             uint64
	HardShare           float64
	HardProviders       int
	SoftIPs             uint64
	SoftProviders       int
	ContextualIPs       uint64
	ContextualProviders int
}

func infrastructureFacts(s SignalSnapshot) infrastructureSummary {
	out := infrastructureSummary{TotalIPs: s.InfraIPs, TotalShare: s.InfraShare}
	var tierIPs uint64
	for _, tier := range s.InfraTiers {
		switch tier.Tier {
		case "hard":
			out.HardIPs = tier.IPs
			out.HardShare = tier.Share
			out.HardProviders = tier.Providers
		case "soft":
			out.SoftIPs = tier.IPs
			out.SoftProviders = tier.Providers
		case "contextual":
			out.ContextualIPs = tier.IPs
			out.ContextualProviders = tier.Providers
		}
		tierIPs += tier.IPs
	}
	if out.TotalIPs == 0 {
		out.TotalIPs = tierIPs
	}
	if out.TotalIPs == 0 && s.InfraShare > 0 && s.TotalIPs > 0 {
		out.TotalIPs = uint64(float64(s.TotalIPs)*s.InfraShare + 0.5)
	}
	if out.TotalShare == 0 && s.TotalIPs > 0 && out.TotalIPs > 0 {
		out.TotalShare = float64(out.TotalIPs) / float64(s.TotalIPs)
	}
	return out
}

func (s infrastructureSummary) nonHardLabel() string {
	switch {
	case s.SoftIPs > 0 && s.ContextualIPs > 0:
		return "soft/contextual"
	case s.SoftIPs > 0:
		return "soft-tier"
	case s.ContextualIPs > 0:
		return "contextual"
	default:
		return ""
	}
}
