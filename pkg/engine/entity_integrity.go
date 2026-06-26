package engine

import (
	"context"
	"strings"
	"time"
)

type EntityIntegrityFinding struct {
	Scope             string    `json:"scope"`
	Kind              string    `json:"kind"`
	Subject           string    `json:"subject,omitempty"`
	Feed              string    `json:"feed,omitempty"`
	Country           string    `json:"country,omitempty"`
	ASN               uint32    `json:"asn,omitempty"`
	Path              string    `json:"path,omitempty"`
	PathMTime         time.Time `json:"path_mtime,omitempty"`
	ReferencePath     string    `json:"reference_path,omitempty"`
	ReferenceMTime    time.Time `json:"reference_mtime,omitempty"`
	RepairAction      string    `json:"repair_action,omitempty"`
	Reason            string    `json:"reason"`
	AffectedCountries int       `json:"affected_countries,omitempty"`
	AffectedASNs      int       `json:"affected_asns,omitempty"`
}

type entityIntegrityPlan struct {
	full                 bool
	feedNames            map[string]struct{}
	countryCodes         map[string]struct{}
	asns                 map[uint32]struct{}
	rebuildCountryIndex  bool
	rebuildASNIndex      bool
	rebuildHomeAggregate bool
	healthFeeds          map[string]struct{}
}

type entityDependencyRef struct {
	path string
	when time.Time
}

type entityHealthCheck struct {
	feed         string
	healthClass  string
	transitionAt time.Time
	countries    []string
	asns         []uint32
}

const maxStartupEntityAutoRepairTargets = 1024

func (p *entityIntegrityPlan) markFull() {
	if p != nil {
		p.full = true
	}
}

func (p *entityIntegrityPlan) addFeed(name string) {
	if p == nil || strings.TrimSpace(name) == "" {
		return
	}
	if p.feedNames == nil {
		p.feedNames = map[string]struct{}{}
	}
	p.feedNames[name] = struct{}{}
}

func (p *entityIntegrityPlan) addCountry(code string) {
	if p == nil {
		return
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return
	}
	if p.countryCodes == nil {
		p.countryCodes = map[string]struct{}{}
	}
	p.countryCodes[code] = struct{}{}
}

func (p *entityIntegrityPlan) addASN(asn uint32) {
	if p == nil || asn == 0 {
		return
	}
	if p.asns == nil {
		p.asns = map[uint32]struct{}{}
	}
	p.asns[asn] = struct{}{}
}

func (p *entityIntegrityPlan) addHealthFeed(name string) {
	if p == nil || strings.TrimSpace(name) == "" {
		return
	}
	if p.healthFeeds == nil {
		p.healthFeeds = map[string]struct{}{}
	}
	p.healthFeeds[name] = struct{}{}
}

func (p *entityIntegrityPlan) addHomeAggregate() {
	if p != nil {
		p.rebuildHomeAggregate = true
	}
}

func (p entityIntegrityPlan) hasWork() bool {
	return p.full ||
		len(p.feedNames) > 0 ||
		len(p.countryCodes) > 0 ||
		len(p.asns) > 0 ||
		p.rebuildCountryIndex ||
		p.rebuildASNIndex ||
		p.rebuildHomeAggregate ||
		len(p.healthFeeds) > 0
}

func (p entityIntegrityPlan) sortedFeeds() []string {
	return sortedStringSet(p.feedNames)
}

func (p entityIntegrityPlan) sortedHealthFeeds() []string {
	return sortedStringSet(p.healthFeeds)
}

func (p entityIntegrityPlan) targetCount() int {
	count := len(p.feedNames) + len(p.countryCodes) + len(p.asns) + len(p.healthFeeds)
	if p.rebuildCountryIndex {
		count++
	}
	if p.rebuildASNIndex {
		count++
	}
	if p.rebuildHomeAggregate {
		count++
	}
	return count
}

func (p entityIntegrityPlan) shouldDeferStartupRepair() bool {
	if p.full {
		return false
	}
	return p.targetCount() > maxStartupEntityAutoRepairTargets
}

func (e *Engine) EnsureEntityArtifactsCurrent(ctx context.Context) error {
	return e.EnsureEntityArtifactsCurrentWithTrigger(ctx, "bootstrap")
}

func (e *Engine) EnsureEntityArtifactsCurrentWithTrigger(ctx context.Context, trigger string) error {
	if e == nil || e.engineLane == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if trigger == "" {
		trigger = "bootstrap"
	}
	return e.engineLane.Run(ctx, LaneWork{
		Kind:      LaneWorkEntityRepair,
		Component: LaneComponentEntityIntegrity,
		Name:      "entity.integrity.repair",
		Trigger:   trigger,
		Stage:     "scanning",
		Detail:    "checking entity artifact integrity",
	}, func(laneCtx context.Context) error {
		return e.ensureEntityArtifactsCurrentWithTriggerAdmitted(laneCtx, trigger)
	})
}

func (e *Engine) QueueEntityArtifactsEnsure(ctx context.Context, trigger string) (EntityArtifactQueueResult, error) {
	if e == nil || e.engineLane == nil {
		return EntityArtifactQueueResult{}, nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return EntityArtifactQueueResult{}, err
	}
	if trigger == "" {
		trigger = "bootstrap"
	}
	ticket, err := e.engineLane.Submit(ctx, LaneWork{
		Kind:          LaneWorkEntityRepair,
		Component:     LaneComponentEntityIntegrity,
		Name:          "entity.integrity.repair",
		Trigger:       trigger,
		Stage:         "scanning",
		Detail:        "checking entity artifact integrity",
		CoalescingKey: entityArtifactsEnsureCoalescingKey(trigger),
	}, func(laneCtx context.Context) error {
		return e.ensureEntityArtifactsCurrentWithTriggerAdmitted(laneCtx, trigger)
	})
	return entityArtifactQueueResult(ticket), err
}

func entityArtifactsEnsureCoalescingKey(trigger string) string {
	switch trigger {
	case "reload":
		return "entity:integrity:reload"
	case "startup", "bootstrap":
		return "entity:repair:startup"
	case "operator", "operator_rebuild", "admin_refresh":
		return "entity:repair:operator"
	default:
		return "entity:repair:background"
	}
}

func (e *Engine) ensureEntityArtifactsCurrentWithTriggerAdmitted(ctx context.Context, trigger string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	e.setEntityIntegrityRunning("entity_integrity:"+trigger, trigger)
	findings, plan, err := e.CheckEntityArtifactsIntegrityContext(ctx)
	if err != nil {
		e.StoreEntityIntegrityFindings(findings, err)
		return err
	}
	e.StoreEntityIntegrityFindings(findings, nil)
	if len(findings) == 0 || !plan.hasWork() {
		return nil
	}
	if trigger == "startup" && plan.shouldDeferStartupRepair() {
		e.observeRunCounter("entity.integrity_startup_repair_deferred", int64(plan.targetCount()), 0)
		e.mu.Lock()
		e.startupRepairDeferred = true
		e.startupRepairDeferredTargets = plan.targetCount()
		e.mu.Unlock()
		if e.logger != nil {
			e.logger.Warn("deferred broad startup entity artifact repair",
				"targets", plan.targetCount(),
				"findings", len(findings),
				"limit", maxStartupEntityAutoRepairTargets)
		}
		return nil
	}
	if err := e.repairEntityArtifactsWithPlanAdmitted(ctx, trigger, plan); err != nil {
		return err
	}
	e.MarkIntegrityCachesStale()
	return nil
}

func (e *Engine) CheckEntityArtifactsIntegrity() ([]EntityIntegrityFinding, entityIntegrityPlan, error) {
	return e.CheckEntityArtifactsIntegrityContext(context.Background())
}

func (e *Engine) CheckEntityArtifactsIntegrityContext(ctx context.Context) ([]EntityIntegrityFinding, entityIntegrityPlan, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, entityIntegrityPlan{}, err
	}
	scanner := newEntityIntegrityScanner(ctx, e)
	if e == nil || scanner.snapshot.cfg == nil {
		return nil, entityIntegrityPlan{}, nil
	}

	if err := scanner.run(); err != nil {
		return nil, scanner.plan, err
	}
	return scanner.findings, scanner.plan, nil
}
