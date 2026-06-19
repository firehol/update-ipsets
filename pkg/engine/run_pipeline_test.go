package engine

import (
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestBuildPipelineRunPlan(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T) *Engine
		report *Report
		opts   RunOptions
		assert func(t *testing.T, plan pipelineRunPlan)
	}{
		{
			name: "no updates does not publish",
			setup: func(t *testing.T) *Engine {
				return engineForRunPlanTest(t, map[string]*config.Source{
					"sample": runPlanPlainSource("sample"),
				}, Runtime{SkipComparisonIfNoUpdates: true})
			},
			report: &Report{},
			assert: func(t *testing.T, plan pipelineRunPlan) {
				if plan.shouldPublish {
					t.Fatal("no-update run should not publish")
				}
				if !plan.skipHeavy {
					t.Fatal("no-update run with skip_comparison_if_no_updates should skip heavy phases")
				}
			},
		},
		{
			name: "no updates without skip comparison flag still does not publish",
			setup: func(t *testing.T) *Engine {
				return engineForRunPlanTest(t, map[string]*config.Source{
					"sample": runPlanPlainSource("sample"),
				}, Runtime{SkipComparisonIfNoUpdates: false})
			},
			report: &Report{},
			assert: func(t *testing.T, plan pipelineRunPlan) {
				if plan.shouldPublish {
					t.Fatal("no-update run without an independent repair reason should not publish")
				}
			},
		},
		{
			name: "manual reprocess with no updates still publishes",
			setup: func(t *testing.T) *Engine {
				return engineForRunPlanTest(t, map[string]*config.Source{
					"sample": runPlanPlainSource("sample"),
				}, Runtime{SkipComparisonIfNoUpdates: true})
			},
			report: &Report{},
			opts:   RunOptions{Selected: []string{"sample"}, Reprocess: true, Manual: true},
			assert: func(t *testing.T, plan pipelineRunPlan) {
				if !plan.shouldPublish {
					t.Fatal("manual reprocess should publish even when the selected feed does not report an update")
				}
				if plan.skipHeavy {
					t.Fatal("manual reprocess should force heavy phases")
				}
			},
		},
		{
			name: "database selection forces heavy phase",
			setup: func(t *testing.T) *Engine {
				return engineForRunPlanTest(t, map[string]*config.Source{
					"sample":       runPlanPlainSource("sample"),
					"dbip_country": {Name: "dbip_country", Use: []string{config.UseGeoIP}},
				}, Runtime{SkipComparisonIfNoUpdates: true})
			},
			report: &Report{},
			opts:   RunOptions{Selected: []string{"dbip_country"}},
			assert: func(t *testing.T, plan pipelineRunPlan) {
				if !plan.databaseSelected {
					t.Fatal("selected geolocation provider should be classified as database-selected")
				}
				if !plan.shouldPublish {
					t.Fatal("selected database provider should publish even without ordinary feed updates")
				}
				if plan.skipHeavy {
					t.Fatal("selected database provider should force heavy phases")
				}
			},
		},
		{
			name: "critical provider-set drift uses critical-only fan-out",
			setup: func(t *testing.T) *Engine {
				return engineForRunPlanTest(t, map[string]*config.Source{
					"sample":       runPlanPlainSource("sample"),
					"critical_dns": runPlanCriticalSource("critical_dns"),
				}, Runtime{LibDir: t.TempDir(), SkipComparisonIfNoUpdates: true})
			},
			report: &Report{Updated: []string{"critical_dns"}},
			opts:   RunOptions{Selected: []string{"critical_dns"}},
			assert: func(t *testing.T, plan pipelineRunPlan) {
				if !plan.criticalProviderSetChanged {
					t.Fatal("missing critical provider-set marker should be detected as drift")
				}
				if !plan.onlyCriticalProviderSet {
					t.Fatal("critical-only update should avoid unrelated geo/asn/bogon/entity fan-out")
				}
				if plan.criticalFanOutUpdated != nil {
					t.Fatalf("critical provider drift should force full critical fan-out, got %v", plan.criticalFanOutUpdated)
				}
			},
		},
		{
			name: "provider default drift forces global fan-out",
			setup: func(t *testing.T) *Engine {
				eng := engineForRunPlanTest(t, map[string]*config.Source{
					"sample":       runPlanPlainSource("sample"),
					"dbip_country": {Name: "dbip_country", Use: []string{config.UseGeoIP}, Label: "DB-IP", Format: "dbip_country_csv"},
				}, Runtime{LibDir: t.TempDir(), SkipComparisonIfNoUpdates: true})
				eng.cfg.Defaults.GeoProvider = "dbip_country"
				return eng
			},
			report: &Report{Updated: []string{"sample"}},
			opts:   RunOptions{Selected: []string{"sample"}},
			assert: func(t *testing.T, plan pipelineRunPlan) {
				if !plan.providerDefaultsChanged {
					t.Fatal("missing provider-default marker should be detected as drift")
				}
				if plan.fanOutUpdated != nil {
					t.Fatalf("provider-default drift should force global heavy fan-out, got %v", plan.fanOutUpdated)
				}
				if plan.criticalFanOutUpdated != nil {
					t.Fatalf("provider-default drift should force global critical fan-out, got %v", plan.criticalFanOutUpdated)
				}
				if plan.insightUpdated != nil {
					t.Fatalf("provider-default drift should force global insight fan-out, got %v", plan.insightUpdated)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.setup(t).buildPipelineRunPlan(tc.report, tc.opts)
			tc.assert(t, plan)
		})
	}
}

func engineForRunPlanTest(t *testing.T, sources map[string]*config.Source, rt Runtime) *Engine {
	t.Helper()
	cfg := &config.Config{Sources: sources}
	for name, src := range cfg.Sources {
		if src != nil && src.Name == "" {
			src.Name = name
		}
	}
	return newEngineFixture(t, withConfig(cfg), withRuntime(func(runtime *Runtime) {
		*runtime = rt
	}))
}

func runPlanPlainSource(name string) *config.Source {
	return &config.Source{
		Name:     name,
		IPV:      "ipv4",
		Output:   "ip",
		Category: "attacks",
	}
}

func runPlanCriticalSource(name string) *config.Source {
	return &config.Source{
		Name:     name,
		Use:      []string{config.UseCriticalInfrastructure},
		Critical: testCriticalMetadata("hard", "public_dns_core"),
		Hidden:   true,
		IPV:      "ipv4",
		Output:   "netset",
		Category: "infrastructure",
	}
}
