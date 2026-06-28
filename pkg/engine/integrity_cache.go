package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrIntegrityCacheNotFresh = errors.New("integrity cache is not fresh")
	ErrIntegrityCacheBusy     = errors.New("integrity cache is busy")
)

var (
	integrityRefreshHookMu            sync.Mutex
	pipelineIntegrityAfterRunningHook func()
	entityIntegrityAfterRunningHook   func()
)

func setPipelineIntegrityAfterRunningHookForTest(fn func()) func() {
	integrityRefreshHookMu.Lock()
	old := pipelineIntegrityAfterRunningHook
	pipelineIntegrityAfterRunningHook = fn
	integrityRefreshHookMu.Unlock()
	return func() {
		integrityRefreshHookMu.Lock()
		pipelineIntegrityAfterRunningHook = old
		integrityRefreshHookMu.Unlock()
	}
}

func setEntityIntegrityAfterRunningHookForTest(fn func()) func() {
	integrityRefreshHookMu.Lock()
	old := entityIntegrityAfterRunningHook
	entityIntegrityAfterRunningHook = fn
	integrityRefreshHookMu.Unlock()
	return func() {
		integrityRefreshHookMu.Lock()
		entityIntegrityAfterRunningHook = old
		integrityRefreshHookMu.Unlock()
	}
}

func pipelineIntegrityAfterRunningHookForTest() func() {
	integrityRefreshHookMu.Lock()
	defer integrityRefreshHookMu.Unlock()
	return pipelineIntegrityAfterRunningHook
}

func entityIntegrityAfterRunningHookForTest() func() {
	integrityRefreshHookMu.Lock()
	defer integrityRefreshHookMu.Unlock()
	return entityIntegrityAfterRunningHook
}

type IntegrityCacheState string

const (
	IntegrityCacheCold           IntegrityCacheState = "cold"
	IntegrityCacheFresh          IntegrityCacheState = "fresh"
	IntegrityCacheStale          IntegrityCacheState = "stale"
	IntegrityCacheRefreshQueued  IntegrityCacheState = "refresh_queued"
	IntegrityCacheRefreshRunning IntegrityCacheState = "refresh_running"
)

type PipelineIntegrityCacheSnapshot struct {
	Generation         uint64              `json:"generation"`
	CacheState         IntegrityCacheState `json:"cache_state"`
	Running            bool                `json:"running,omitempty"`
	Queued             bool                `json:"queued,omitempty"`
	Coalesced          bool                `json:"coalesced,omitempty"`
	Ticket             *LaneTicket         `json:"ticket,omitempty"`
	IncludeArchived    bool                `json:"include_archived,omitempty"`
	EnableAll          bool                `json:"enable_all,omitempty"`
	WebDir             string              `json:"web_dir,omitempty"`
	CheckedAt          time.Time           `json:"checked_at,omitempty"`
	LastStarted        time.Time           `json:"last_started,omitempty"`
	LastEnded          time.Time           `json:"last_ended,omitempty"`
	LastError          string              `json:"last_error,omitempty"`
	StartupScanRunning bool                `json:"startup_scan_running,omitempty"`
	Findings           []IntegrityFinding  `json:"findings"`
}

type EntityIntegrityCacheSnapshot struct {
	Generation         uint64                   `json:"generation"`
	CacheState         IntegrityCacheState      `json:"cache_state"`
	Running            bool                     `json:"running,omitempty"`
	Queued             bool                     `json:"queued,omitempty"`
	Coalesced          bool                     `json:"coalesced,omitempty"`
	Ticket             *LaneTicket              `json:"ticket,omitempty"`
	CheckedAt          time.Time                `json:"checked_at,omitempty"`
	LastStarted        time.Time                `json:"last_started,omitempty"`
	LastEnded          time.Time                `json:"last_ended,omitempty"`
	LastError          string                   `json:"last_error,omitempty"`
	StartupScanRunning bool                     `json:"startup_scan_running,omitempty"`
	Findings           []EntityIntegrityFinding `json:"findings"`
}

type PipelineIntegrityCacheStatus struct {
	Generation         uint64              `json:"generation"`
	CacheState         IntegrityCacheState `json:"cache_state"`
	Running            bool                `json:"running,omitempty"`
	Queued             bool                `json:"queued,omitempty"`
	Coalesced          bool                `json:"coalesced,omitempty"`
	Ticket             *LaneTicket         `json:"ticket,omitempty"`
	IncludeArchived    bool                `json:"include_archived,omitempty"`
	EnableAll          bool                `json:"enable_all,omitempty"`
	WebDir             string              `json:"web_dir,omitempty"`
	CheckedAt          time.Time           `json:"checked_at,omitempty"`
	LastStarted        time.Time           `json:"last_started,omitempty"`
	LastEnded          time.Time           `json:"last_ended,omitempty"`
	LastError          string              `json:"last_error,omitempty"`
	StartupScanRunning bool                `json:"startup_scan_running,omitempty"`
	Count              int                 `json:"count"`
}

type EntityIntegrityCacheStatus struct {
	Generation         uint64              `json:"generation"`
	CacheState         IntegrityCacheState `json:"cache_state"`
	Running            bool                `json:"running,omitempty"`
	Queued             bool                `json:"queued,omitempty"`
	Coalesced          bool                `json:"coalesced,omitempty"`
	Ticket             *LaneTicket         `json:"ticket,omitempty"`
	CheckedAt          time.Time           `json:"checked_at,omitempty"`
	LastStarted        time.Time           `json:"last_started,omitempty"`
	LastEnded          time.Time           `json:"last_ended,omitempty"`
	LastError          string              `json:"last_error,omitempty"`
	StartupScanRunning bool                `json:"startup_scan_running,omitempty"`
	Count              int                 `json:"count"`
}

type pipelineIntegrityCacheState struct {
	generation         uint64
	scope              IntegrityOptions
	state              IntegrityCacheState
	workID             string
	ticket             *LaneTicket
	startedAt          time.Time
	endedAt            time.Time
	checkedAt          time.Time
	lastError          string
	startupScanRunning bool
	findings           []IntegrityFinding
}

type pipelineIntegrityCacheKey struct {
	includeArchived bool
	enableAll       bool
	webDir          string
}

type entityIntegrityCacheState struct {
	generation         uint64
	state              IntegrityCacheState
	workID             string
	ticket             *LaneTicket
	startedAt          time.Time
	endedAt            time.Time
	checkedAt          time.Time
	lastError          string
	startupScanRunning bool
	findings           []EntityIntegrityFinding
}

func (e *Engine) PipelineIntegrityCacheSnapshot(opts IntegrityOptions) PipelineIntegrityCacheSnapshot {
	opts = e.normalizeIntegrityOptions(opts)
	e.pipelineIntegrityCacheMu.RLock()
	defer e.pipelineIntegrityCacheMu.RUnlock()
	state := e.pipelineIntegrityCacheForReadLocked(opts)
	if state == nil {
		return PipelineIntegrityCacheSnapshot{
			CacheState:      IntegrityCacheCold,
			IncludeArchived: opts.IncludeArchived,
			EnableAll:       opts.EnableAll,
			WebDir:          opts.WebDir,
			Findings:        []IntegrityFinding{},
		}
	}
	return state.snapshotLocked()
}

func (e *Engine) TryPipelineIntegrityCacheSnapshot(opts IntegrityOptions) (PipelineIntegrityCacheSnapshot, bool) {
	opts, ok := e.tryNormalizeIntegrityOptions(opts)
	if !ok {
		return PipelineIntegrityCacheSnapshot{CacheState: IntegrityCacheCold, Findings: []IntegrityFinding{}}, false
	}
	if !e.pipelineIntegrityCacheMu.TryRLock() {
		return PipelineIntegrityCacheSnapshot{
			CacheState:      IntegrityCacheRefreshRunning,
			Running:         true,
			IncludeArchived: opts.IncludeArchived,
			EnableAll:       opts.EnableAll,
			WebDir:          opts.WebDir,
			Findings:        []IntegrityFinding{},
		}, false
	}
	defer e.pipelineIntegrityCacheMu.RUnlock()
	state := e.pipelineIntegrityCacheForReadLocked(opts)
	if state == nil {
		return PipelineIntegrityCacheSnapshot{
			CacheState:      IntegrityCacheCold,
			IncludeArchived: opts.IncludeArchived,
			EnableAll:       opts.EnableAll,
			WebDir:          opts.WebDir,
			Findings:        []IntegrityFinding{},
		}, true
	}
	return state.snapshotLocked(), true
}

func (e *Engine) EntityIntegrityCacheSnapshot() EntityIntegrityCacheSnapshot {
	e.entityIntegrityCacheMu.RLock()
	defer e.entityIntegrityCacheMu.RUnlock()
	return e.entityIntegrityCache.snapshotLocked()
}

func (e *Engine) TryEntityIntegrityCacheSnapshot() (EntityIntegrityCacheSnapshot, bool) {
	if e == nil {
		return EntityIntegrityCacheSnapshot{CacheState: IntegrityCacheCold, Findings: []EntityIntegrityFinding{}}, true
	}
	if !e.entityIntegrityCacheMu.TryRLock() {
		return EntityIntegrityCacheSnapshot{
			CacheState: IntegrityCacheRefreshRunning,
			Running:    true,
			Findings:   []EntityIntegrityFinding{},
		}, false
	}
	defer e.entityIntegrityCacheMu.RUnlock()
	return e.entityIntegrityCache.snapshotLocked(), true
}

func (e *Engine) IntegrityCacheStatus() (PipelineIntegrityCacheStatus, EntityIntegrityCacheStatus) {
	if e == nil {
		return PipelineIntegrityCacheStatus{CacheState: IntegrityCacheCold}, EntityIntegrityCacheStatus{CacheState: IntegrityCacheCold}
	}
	opts := e.normalizeIntegrityOptions(IntegrityOptions{})
	return e.integrityCacheStatusForNormalizedOptions(opts)
}

func (e *Engine) integrityCacheStatusForNormalizedOptions(opts IntegrityOptions) (PipelineIntegrityCacheStatus, EntityIntegrityCacheStatus) {
	pipeline, entity, _ := e.integrityCacheStatusForNormalizedOptionsBestEffort(opts, true)
	return pipeline, entity
}

func (e *Engine) tryIntegrityCacheStatusForNormalizedOptions(opts IntegrityOptions) (PipelineIntegrityCacheStatus, EntityIntegrityCacheStatus, bool) {
	return e.integrityCacheStatusForNormalizedOptionsBestEffort(opts, false)
}

func (e *Engine) integrityCacheStatusForNormalizedOptionsBestEffort(opts IntegrityOptions, wait bool) (PipelineIntegrityCacheStatus, EntityIntegrityCacheStatus, bool) {
	if e == nil {
		return PipelineIntegrityCacheStatus{CacheState: IntegrityCacheCold}, EntityIntegrityCacheStatus{CacheState: IntegrityCacheCold}, true
	}
	ok := true
	pipeline := PipelineIntegrityCacheStatus{
		CacheState:      IntegrityCacheCold,
		IncludeArchived: opts.IncludeArchived,
		EnableAll:       opts.EnableAll,
		WebDir:          opts.WebDir,
	}
	if wait {
		e.pipelineIntegrityCacheMu.RLock()
		if state := e.pipelineIntegrityCacheForReadLocked(opts); state != nil {
			pipeline = state.statusLocked()
		}
		e.pipelineIntegrityCacheMu.RUnlock()
	} else {
		if e.pipelineIntegrityCacheMu.TryRLock() {
			if state := e.pipelineIntegrityCacheForReadLocked(opts); state != nil {
				pipeline = state.statusLocked()
			}
			e.pipelineIntegrityCacheMu.RUnlock()
		} else {
			pipeline.CacheState = IntegrityCacheRefreshRunning
			pipeline.Running = true
			ok = false
		}
	}

	entity := EntityIntegrityCacheStatus{CacheState: IntegrityCacheCold}
	if wait {
		e.entityIntegrityCacheMu.RLock()
		entity = e.entityIntegrityCache.statusLocked()
		e.entityIntegrityCacheMu.RUnlock()
	} else {
		if e.entityIntegrityCacheMu.TryRLock() {
			entity = e.entityIntegrityCache.statusLocked()
			e.entityIntegrityCacheMu.RUnlock()
		} else {
			entity.CacheState = IntegrityCacheRefreshRunning
			entity.Running = true
			ok = false
		}
	}
	return pipeline, entity, ok
}

func (e *Engine) QueuePipelineIntegrityRefresh(ctx context.Context, opts IntegrityOptions, trigger string) (PipelineIntegrityCacheSnapshot, error) {
	if e == nil || e.engineLane == nil {
		return PipelineIntegrityCacheSnapshot{CacheState: IntegrityCacheCold, Findings: []IntegrityFinding{}}, nil
	}
	ctx = nonNilContext(ctx)
	var ok bool
	opts, ok = e.tryNormalizeIntegrityOptions(opts)
	if !ok {
		return PipelineIntegrityCacheSnapshot{CacheState: IntegrityCacheRefreshRunning, Running: true, Findings: []IntegrityFinding{}}, ErrIntegrityCacheBusy
	}
	workID := e.nextIntegrityWorkID("pipeline")
	work := LaneWork{
		ID:            workID,
		Kind:          LaneWorkIntegrityRefresh,
		Component:     LaneComponentPipelineIntegrity,
		Name:          "pipeline.integrity.refresh",
		Trigger:       defaultString(trigger, "admin_refresh"),
		Stage:         "scanning",
		Detail:        "checking pipeline artifact integrity",
		CoalescingKey: pipelineIntegrityCoalescingKey(opts),
	}
	ticket, err := e.engineLane.Submit(ctx, work, func(laneCtx context.Context) (scanErr error) {
		e.setPipelineIntegrityRunning(opts, workID, work.Trigger)
		var findings []IntegrityFinding
		defer func() {
			if recovered := recover(); recovered != nil {
				scanErr = fmt.Errorf("%w: pipeline integrity refresh panicked: %v", ErrLanePanic, recovered)
			}
			e.setPipelineIntegritySettled(opts, workID, findings, scanErr)
		}()
		if hook := pipelineIntegrityAfterRunningHookForTest(); hook != nil {
			hook()
		}
		findings, scanErr = e.CheckIntegrityWithOptionsContext(laneCtx, opts)
		return scanErr
	})
	if err != nil {
		snap, _ := e.TryPipelineIntegrityCacheSnapshot(opts)
		return snap, err
	}
	if !e.trySetPipelineIntegrityQueued(opts, workID, ticket) {
		return pipelineIntegritySnapshotFromTicket(opts, ticket), nil
	}
	if snap, ok := e.TryPipelineIntegrityCacheSnapshot(opts); ok {
		return snap, nil
	}
	return pipelineIntegritySnapshotFromTicket(opts, ticket), nil
}

func (e *Engine) QueuePipelineIntegrityReprocess(ctx context.Context, opts IntegrityOptions, trigger string, fn func(context.Context, []IntegrityFinding) error) (LaneTicket, error) {
	if e == nil || e.engineLane == nil {
		return LaneTicket{}, ErrLaneShuttingDown
	}
	if fn == nil {
		return LaneTicket{}, errors.New("integrity reprocess requires callback")
	}
	ctx = nonNilContext(ctx)
	var ok bool
	opts, ok = e.tryNormalizeIntegrityOptions(opts)
	if !ok {
		return LaneTicket{}, ErrIntegrityCacheBusy
	}
	workID := e.nextIntegrityWorkID("pipeline_reprocess")

	if !e.pipelineIntegrityCacheMu.TryRLock() {
		return LaneTicket{}, ErrIntegrityCacheBusy
	}
	state := e.pipelineIntegrityCacheForReadLocked(opts)
	if state == nil || state.state != IntegrityCacheFresh {
		e.pipelineIntegrityCacheMu.RUnlock()
		return LaneTicket{}, ErrIntegrityCacheNotFresh
	}
	findings := cloneIntegrityFindings(state.findings)
	e.pipelineIntegrityCacheMu.RUnlock()

	work := LaneWork{
		ID:            workID,
		Kind:          LaneWorkIntegrityReprocess,
		Component:     LaneComponentPipelineIntegrity,
		Name:          "pipeline.integrity.reprocess",
		Trigger:       defaultString(trigger, "admin_reprocess"),
		Stage:         "queueing",
		Detail:        "queueing integrity recovery work",
		CoalescingKey: pipelineIntegrityReprocessCoalescingKey(opts),
	}
	return e.engineLane.Submit(ctx, work, func(laneCtx context.Context) error {
		if err := contextErr(laneCtx); err != nil {
			return err
		}
		return fn(laneCtx, e.integrityFindingsWithRecoveryPlanIfMissing(findings))
	})
}

func (e *Engine) QueueEntityIntegrityRefresh(ctx context.Context, trigger string) (EntityIntegrityCacheSnapshot, error) {
	if e == nil || e.engineLane == nil {
		return EntityIntegrityCacheSnapshot{CacheState: IntegrityCacheCold, Findings: []EntityIntegrityFinding{}}, nil
	}
	ctx = nonNilContext(ctx)
	workID := e.nextIntegrityWorkID("entity")
	work := LaneWork{
		ID:            workID,
		Kind:          LaneWorkIntegrityRefresh,
		Component:     LaneComponentEntityIntegrity,
		Name:          "entity.integrity.refresh",
		Trigger:       defaultString(trigger, "admin_refresh"),
		Stage:         "scanning",
		Detail:        "checking entity artifact integrity",
		CoalescingKey: "integrity:entity:refresh",
	}
	ticket, err := e.engineLane.Submit(ctx, work, func(laneCtx context.Context) (scanErr error) {
		e.setEntityIntegrityRunning(workID, work.Trigger)
		var findings []EntityIntegrityFinding
		defer func() {
			if recovered := recover(); recovered != nil {
				scanErr = fmt.Errorf("%w: entity integrity refresh panicked: %v", ErrLanePanic, recovered)
			}
			e.setEntityIntegritySettled(workID, findings, scanErr)
		}()
		if hook := entityIntegrityAfterRunningHookForTest(); hook != nil {
			hook()
		}
		findings, _, scanErr = e.CheckEntityArtifactsIntegrityContext(laneCtx)
		if scanErr == nil {
			scanErr = contextErr(laneCtx)
		}
		return scanErr
	})
	if err != nil {
		snap, _ := e.TryEntityIntegrityCacheSnapshot()
		return snap, err
	}
	if !e.trySetEntityIntegrityQueued(workID, ticket) {
		return entityIntegritySnapshotFromTicket(ticket), nil
	}
	if snap, ok := e.TryEntityIntegrityCacheSnapshot(); ok {
		return snap, nil
	}
	return entityIntegritySnapshotFromTicket(ticket), nil
}

func (e *Engine) StorePipelineIntegrityFindings(opts IntegrityOptions, findings []IntegrityFinding, err error) {
	opts = e.normalizeIntegrityOptions(opts)
	e.setPipelineIntegritySettled(opts, "", findings, err)
}

func (e *Engine) StoreEntityIntegrityFindings(findings []EntityIntegrityFinding, err error) {
	e.setEntityIntegritySettled("", findings, err)
}

func (e *Engine) MarkPipelineIntegrityStartupScanRunning(opts IntegrityOptions) {
	if e == nil {
		return
	}
	opts = e.normalizeIntegrityOptions(opts)
	e.pipelineIntegrityCacheMu.Lock()
	defer e.pipelineIntegrityCacheMu.Unlock()
	state := e.pipelineIntegrityCacheForUpdateLocked(opts)
	state.scope = opts
	state.state = IntegrityCacheRefreshRunning
	state.startupScanRunning = true
	state.startedAt = time.Now().UTC()
	if e.now != nil {
		state.startedAt = e.now().UTC()
	}
	state.lastError = ""
}

func (e *Engine) MarkIntegrityCachesStale() {
	if e == nil {
		return
	}
	e.pipelineIntegrityCacheMu.Lock()
	for _, state := range e.pipelineIntegrityCaches {
		if state.state == IntegrityCacheFresh {
			state.state = IntegrityCacheStale
		}
	}
	e.pipelineIntegrityCacheMu.Unlock()

	e.entityIntegrityCacheMu.Lock()
	if e.entityIntegrityCache.state == IntegrityCacheFresh {
		e.entityIntegrityCache.state = IntegrityCacheStale
	}
	e.entityIntegrityCacheMu.Unlock()
}

func (e *Engine) normalizeIntegrityOptions(opts IntegrityOptions) IntegrityOptions {
	if e == nil {
		return opts
	}
	opts.WebDir = strings.TrimSpace(opts.WebDir)
	if opts.WebDir == "" {
		opts.WebDir = e.Runtime().WebDir
	}
	return opts
}

func (e *Engine) tryNormalizeIntegrityOptions(opts IntegrityOptions) (IntegrityOptions, bool) {
	if e == nil {
		return opts, true
	}
	opts.WebDir = strings.TrimSpace(opts.WebDir)
	if opts.WebDir != "" {
		return opts, true
	}
	_, rt, ok := e.TryConfigRuntimeSnapshot()
	if !ok {
		return opts, false
	}
	opts.WebDir = rt.WebDir
	return opts, true
}

func (e *Engine) pipelineIntegrityCacheForReadLocked(opts IntegrityOptions) *pipelineIntegrityCacheState {
	if e == nil || len(e.pipelineIntegrityCaches) == 0 {
		return nil
	}
	return e.pipelineIntegrityCaches[integrityCacheKey(opts)]
}

func (e *Engine) pipelineIntegrityCacheForUpdateLocked(opts IntegrityOptions) *pipelineIntegrityCacheState {
	if e.pipelineIntegrityCaches == nil {
		e.pipelineIntegrityCaches = make(map[pipelineIntegrityCacheKey]*pipelineIntegrityCacheState)
	}
	key := integrityCacheKey(opts)
	state := e.pipelineIntegrityCaches[key]
	if state == nil {
		state = &pipelineIntegrityCacheState{scope: opts}
		e.pipelineIntegrityCaches[key] = state
	}
	return state
}

func integrityCacheKey(opts IntegrityOptions) pipelineIntegrityCacheKey {
	return pipelineIntegrityCacheKey{
		includeArchived: opts.IncludeArchived,
		enableAll:       opts.EnableAll,
		webDir:          strings.TrimSpace(opts.WebDir),
	}
}

func pipelineIntegrityCoalescingKey(opts IntegrityOptions) string {
	return fmt.Sprintf("integrity:pipeline:include_archived=%t:enable_all=%t:web_dir=%s", opts.IncludeArchived, opts.EnableAll, integrityWebDirKeyScope(opts.WebDir))
}

func pipelineIntegrityReprocessCoalescingKey(opts IntegrityOptions) string {
	return fmt.Sprintf("integrity:pipeline:reprocess:include_archived=%t:enable_all=%t:web_dir=%s", opts.IncludeArchived, opts.EnableAll, integrityWebDirKeyScope(opts.WebDir))
}

func integrityWebDirKeyScope(webDir string) string {
	webDir = strings.TrimSpace(webDir)
	if webDir == "" {
		return "default"
	}
	sum := sha256.Sum256([]byte(webDir))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func (e *Engine) nextIntegrityWorkID(scope string) string {
	now := time.Now().UTC().UnixNano()
	if e != nil && e.now != nil {
		now = e.now().UTC().UnixNano()
	}
	return fmt.Sprintf("integrity_refresh:%s:%d", scope, now)
}

func (e *Engine) setPipelineIntegrityQueued(opts IntegrityOptions, workID string, ticket LaneTicket) {
	e.pipelineIntegrityCacheMu.Lock()
	defer e.pipelineIntegrityCacheMu.Unlock()
	e.setPipelineIntegrityQueuedLocked(opts, workID, ticket)
}

func (e *Engine) trySetPipelineIntegrityQueued(opts IntegrityOptions, workID string, ticket LaneTicket) bool {
	if !e.pipelineIntegrityCacheMu.TryLock() {
		return false
	}
	defer e.pipelineIntegrityCacheMu.Unlock()
	e.setPipelineIntegrityQueuedLocked(opts, workID, ticket)
	return true
}

func (e *Engine) setPipelineIntegrityQueuedLocked(opts IntegrityOptions, workID string, ticket LaneTicket) {
	state := e.pipelineIntegrityCacheForUpdateLocked(opts)
	if ticket.Coalesced && strings.TrimSpace(ticket.ID) != "" {
		workID = ticket.ID
	}
	if state.workID == workID {
		switch state.state {
		case IntegrityCacheRefreshQueued, IntegrityCacheRefreshRunning, IntegrityCacheFresh:
			return
		case IntegrityCacheCold, IntegrityCacheStale:
			if !state.endedAt.IsZero() {
				return
			}
		}
	}
	cacheState := IntegrityCacheRefreshQueued
	if ticket.State == LaneWorkActive {
		cacheState = IntegrityCacheRefreshRunning
	}
	state.scope = opts
	state.state = cacheState
	state.workID = workID
	state.ticket = cloneLaneTicket(ticket)
	state.lastError = ""
}

func (e *Engine) setPipelineIntegrityRunning(opts IntegrityOptions, workID, trigger string) {
	e.pipelineIntegrityCacheMu.Lock()
	defer e.pipelineIntegrityCacheMu.Unlock()
	state := e.pipelineIntegrityCacheForUpdateLocked(opts)
	now := time.Now().UTC()
	if e.now != nil {
		now = e.now().UTC()
	}
	state.scope = opts
	state.state = IntegrityCacheRefreshRunning
	state.workID = workID
	state.startedAt = now
	if trigger == "startup" {
		state.startupScanRunning = true
	}
	state.lastError = ""
}

func (e *Engine) setPipelineIntegritySettled(opts IntegrityOptions, workID string, findings []IntegrityFinding, err error) {
	settledFindings := cloneIntegrityFindings(findings)
	if err == nil {
		settledFindings = e.integrityFindingsWithRecoveryPlan(settledFindings)
	}
	e.pipelineIntegrityCacheMu.Lock()
	defer e.pipelineIntegrityCacheMu.Unlock()
	state := e.pipelineIntegrityCacheForUpdateLocked(opts)
	now := time.Now().UTC()
	if e.now != nil {
		now = e.now().UTC()
	}
	state.scope = opts
	state.workID = workID
	state.endedAt = now
	state.startupScanRunning = false
	state.ticket = nil
	if err != nil {
		state.lastError = err.Error()
		if len(state.findings) == 0 {
			state.state = IntegrityCacheCold
		} else {
			state.state = IntegrityCacheStale
		}
		return
	}
	state.state = IntegrityCacheFresh
	state.generation++
	state.checkedAt = now
	state.lastError = ""
	state.findings = settledFindings
}

func (e *Engine) setEntityIntegrityQueued(workID string, ticket LaneTicket) {
	e.entityIntegrityCacheMu.Lock()
	defer e.entityIntegrityCacheMu.Unlock()
	e.setEntityIntegrityQueuedLocked(workID, ticket)
}

func (e *Engine) trySetEntityIntegrityQueued(workID string, ticket LaneTicket) bool {
	if !e.entityIntegrityCacheMu.TryLock() {
		return false
	}
	defer e.entityIntegrityCacheMu.Unlock()
	e.setEntityIntegrityQueuedLocked(workID, ticket)
	return true
}

func (e *Engine) setEntityIntegrityQueuedLocked(workID string, ticket LaneTicket) {
	if ticket.Coalesced && strings.TrimSpace(ticket.ID) != "" {
		workID = ticket.ID
	}
	if e.entityIntegrityCache.workID == workID {
		switch e.entityIntegrityCache.state {
		case IntegrityCacheRefreshQueued, IntegrityCacheRefreshRunning, IntegrityCacheFresh:
			return
		case IntegrityCacheCold, IntegrityCacheStale:
			if !e.entityIntegrityCache.endedAt.IsZero() {
				return
			}
		}
	}
	state := IntegrityCacheRefreshQueued
	if ticket.State == LaneWorkActive {
		state = IntegrityCacheRefreshRunning
	}
	e.entityIntegrityCache.state = state
	e.entityIntegrityCache.workID = workID
	e.entityIntegrityCache.ticket = cloneLaneTicket(ticket)
	e.entityIntegrityCache.lastError = ""
}

func (e *Engine) setEntityIntegrityRunning(workID, trigger string) {
	e.entityIntegrityCacheMu.Lock()
	defer e.entityIntegrityCacheMu.Unlock()
	now := time.Now().UTC()
	if e.now != nil {
		now = e.now().UTC()
	}
	e.entityIntegrityCache.state = IntegrityCacheRefreshRunning
	e.entityIntegrityCache.workID = workID
	e.entityIntegrityCache.startedAt = now
	if trigger == "startup" {
		e.entityIntegrityCache.startupScanRunning = true
	}
	e.entityIntegrityCache.lastError = ""
}

func (e *Engine) setEntityIntegritySettled(workID string, findings []EntityIntegrityFinding, err error) {
	e.entityIntegrityCacheMu.Lock()
	defer e.entityIntegrityCacheMu.Unlock()
	now := time.Now().UTC()
	if e.now != nil {
		now = e.now().UTC()
	}
	e.entityIntegrityCache.workID = workID
	e.entityIntegrityCache.endedAt = now
	e.entityIntegrityCache.startupScanRunning = false
	e.entityIntegrityCache.ticket = nil
	if err != nil {
		e.entityIntegrityCache.lastError = err.Error()
		if len(e.entityIntegrityCache.findings) == 0 {
			e.entityIntegrityCache.state = IntegrityCacheCold
		} else {
			e.entityIntegrityCache.state = IntegrityCacheStale
		}
		return
	}
	e.entityIntegrityCache.state = IntegrityCacheFresh
	e.entityIntegrityCache.generation++
	e.entityIntegrityCache.checkedAt = now
	e.entityIntegrityCache.lastError = ""
	e.entityIntegrityCache.findings = cloneEntityIntegrityFindings(findings)
}
