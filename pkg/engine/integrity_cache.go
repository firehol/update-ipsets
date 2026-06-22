package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrIntegrityCacheNotFresh = errors.New("integrity cache is not fresh")

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

func (e *Engine) EntityIntegrityCacheSnapshot() EntityIntegrityCacheSnapshot {
	e.entityIntegrityCacheMu.RLock()
	defer e.entityIntegrityCacheMu.RUnlock()
	return e.entityIntegrityCache.snapshotLocked()
}

func (e *Engine) IntegrityCacheStatus() (PipelineIntegrityCacheStatus, EntityIntegrityCacheStatus) {
	if e == nil {
		return PipelineIntegrityCacheStatus{CacheState: IntegrityCacheCold}, EntityIntegrityCacheStatus{CacheState: IntegrityCacheCold}
	}
	opts := e.normalizeIntegrityOptions(IntegrityOptions{})
	e.pipelineIntegrityCacheMu.RLock()
	pipeline := PipelineIntegrityCacheStatus{
		CacheState:      IntegrityCacheCold,
		IncludeArchived: opts.IncludeArchived,
		EnableAll:       opts.EnableAll,
		WebDir:          opts.WebDir,
	}
	if state := e.pipelineIntegrityCacheForReadLocked(opts); state != nil {
		pipeline = state.statusLocked()
	}
	e.pipelineIntegrityCacheMu.RUnlock()
	e.entityIntegrityCacheMu.RLock()
	entity := e.entityIntegrityCache.statusLocked()
	e.entityIntegrityCacheMu.RUnlock()
	return pipeline, entity
}

func (e *Engine) QueuePipelineIntegrityRefresh(ctx context.Context, opts IntegrityOptions, trigger string) (PipelineIntegrityCacheSnapshot, error) {
	if e == nil || e.engineLane == nil {
		return PipelineIntegrityCacheSnapshot{CacheState: IntegrityCacheCold, Findings: []IntegrityFinding{}}, nil
	}
	ctx = nonNilContext(ctx)
	opts = e.normalizeIntegrityOptions(opts)
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
	ticket, err := e.engineLane.Submit(ctx, work, func(laneCtx context.Context) error {
		e.setPipelineIntegrityRunning(opts, workID, work.Trigger)
		findings, scanErr := e.CheckIntegrityWithOptionsContext(laneCtx, opts)
		e.setPipelineIntegritySettled(opts, workID, findings, scanErr)
		return scanErr
	})
	if err != nil {
		return e.PipelineIntegrityCacheSnapshot(opts), err
	}
	e.setPipelineIntegrityQueued(opts, workID, ticket)
	return e.PipelineIntegrityCacheSnapshot(opts), nil
}

func (e *Engine) QueuePipelineIntegrityReprocess(ctx context.Context, opts IntegrityOptions, trigger string, fn func(context.Context, []IntegrityFinding) error) (LaneTicket, error) {
	if e == nil || e.engineLane == nil {
		return LaneTicket{}, ErrLaneShuttingDown
	}
	if fn == nil {
		return LaneTicket{}, errors.New("integrity reprocess requires callback")
	}
	ctx = nonNilContext(ctx)
	opts = e.normalizeIntegrityOptions(opts)
	workID := e.nextIntegrityWorkID("pipeline_reprocess")

	e.pipelineIntegrityCacheMu.RLock()
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
		return fn(laneCtx, cloneIntegrityFindings(findings))
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
	ticket, err := e.engineLane.Submit(ctx, work, func(laneCtx context.Context) error {
		e.setEntityIntegrityRunning(workID, work.Trigger)
		findings, _, scanErr := e.CheckEntityArtifactsIntegrity()
		if scanErr == nil {
			scanErr = contextErr(laneCtx)
		}
		e.setEntityIntegritySettled(workID, findings, scanErr)
		return scanErr
	})
	if err != nil {
		return e.EntityIntegrityCacheSnapshot(), err
	}
	e.setEntityIntegrityQueued(workID, ticket)
	return e.EntityIntegrityCacheSnapshot(), nil
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
		opts.WebDir = e.runtime.WebDir
	}
	return opts
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
	state := e.pipelineIntegrityCacheForUpdateLocked(opts)
	if ticket.Coalesced && strings.TrimSpace(ticket.ID) != "" {
		workID = ticket.ID
	}
	if state.workID == workID &&
		(state.state == IntegrityCacheRefreshRunning || state.state == IntegrityCacheFresh) {
		return
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
	if e != nil && e.now != nil {
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
	e.pipelineIntegrityCacheMu.Lock()
	defer e.pipelineIntegrityCacheMu.Unlock()
	state := e.pipelineIntegrityCacheForUpdateLocked(opts)
	now := time.Now().UTC()
	if e != nil && e.now != nil {
		now = e.now().UTC()
	}
	state.scope = opts
	state.workID = workID
	state.endedAt = now
	state.startupScanRunning = false
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
	state.findings = cloneIntegrityFindings(findings)
	state.ticket = nil
}

func (e *Engine) setEntityIntegrityQueued(workID string, ticket LaneTicket) {
	e.entityIntegrityCacheMu.Lock()
	defer e.entityIntegrityCacheMu.Unlock()
	if ticket.Coalesced && strings.TrimSpace(ticket.ID) != "" {
		workID = ticket.ID
	}
	if e.entityIntegrityCache.workID == workID &&
		(e.entityIntegrityCache.state == IntegrityCacheRefreshRunning || e.entityIntegrityCache.state == IntegrityCacheFresh) {
		return
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
	if e != nil && e.now != nil {
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
	if e != nil && e.now != nil {
		now = e.now().UTC()
	}
	e.entityIntegrityCache.workID = workID
	e.entityIntegrityCache.endedAt = now
	e.entityIntegrityCache.startupScanRunning = false
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
	e.entityIntegrityCache.ticket = nil
}

func (s pipelineIntegrityCacheState) snapshotLocked() PipelineIntegrityCacheSnapshot {
	state := s.state
	if state == "" {
		state = IntegrityCacheCold
	}
	return PipelineIntegrityCacheSnapshot{
		Generation:         s.generation,
		CacheState:         state,
		Running:            state == IntegrityCacheRefreshRunning,
		Queued:             state == IntegrityCacheRefreshQueued,
		Coalesced:          s.ticket != nil && s.ticket.Coalesced,
		Ticket:             cloneLaneTicketPtr(s.ticket),
		IncludeArchived:    s.scope.IncludeArchived,
		EnableAll:          s.scope.EnableAll,
		WebDir:             s.scope.WebDir,
		CheckedAt:          s.checkedAt,
		LastStarted:        s.startedAt,
		LastEnded:          s.endedAt,
		LastError:          s.lastError,
		StartupScanRunning: s.startupScanRunning,
		Findings:           cloneIntegrityFindings(s.findings),
	}
}

func (s entityIntegrityCacheState) snapshotLocked() EntityIntegrityCacheSnapshot {
	state := s.state
	if state == "" {
		state = IntegrityCacheCold
	}
	return EntityIntegrityCacheSnapshot{
		Generation:         s.generation,
		CacheState:         state,
		Running:            state == IntegrityCacheRefreshRunning,
		Queued:             state == IntegrityCacheRefreshQueued,
		Coalesced:          s.ticket != nil && s.ticket.Coalesced,
		Ticket:             cloneLaneTicketPtr(s.ticket),
		CheckedAt:          s.checkedAt,
		LastStarted:        s.startedAt,
		LastEnded:          s.endedAt,
		LastError:          s.lastError,
		StartupScanRunning: s.startupScanRunning,
		Findings:           cloneEntityIntegrityFindings(s.findings),
	}
}

func (s pipelineIntegrityCacheState) statusLocked() PipelineIntegrityCacheStatus {
	snap := s.snapshotLocked()
	return PipelineIntegrityCacheStatus{
		Generation:         snap.Generation,
		CacheState:         snap.CacheState,
		Running:            snap.Running,
		Queued:             snap.Queued,
		Coalesced:          snap.Coalesced,
		Ticket:             cloneLaneTicketPtr(snap.Ticket),
		IncludeArchived:    snap.IncludeArchived,
		EnableAll:          snap.EnableAll,
		WebDir:             snap.WebDir,
		CheckedAt:          snap.CheckedAt,
		LastStarted:        snap.LastStarted,
		LastEnded:          snap.LastEnded,
		LastError:          snap.LastError,
		StartupScanRunning: snap.StartupScanRunning,
		Count:              len(s.findings),
	}
}

func (s entityIntegrityCacheState) statusLocked() EntityIntegrityCacheStatus {
	snap := s.snapshotLocked()
	return EntityIntegrityCacheStatus{
		Generation:         snap.Generation,
		CacheState:         snap.CacheState,
		Running:            snap.Running,
		Queued:             snap.Queued,
		Coalesced:          snap.Coalesced,
		Ticket:             cloneLaneTicketPtr(snap.Ticket),
		CheckedAt:          snap.CheckedAt,
		LastStarted:        snap.LastStarted,
		LastEnded:          snap.LastEnded,
		LastError:          snap.LastError,
		StartupScanRunning: snap.StartupScanRunning,
		Count:              len(s.findings),
	}
}

func cloneLaneTicket(ticket LaneTicket) *LaneTicket {
	return &LaneTicket{
		ID:        ticket.ID,
		Kind:      ticket.Kind,
		Component: ticket.Component,
		Queued:    ticket.Queued,
		Coalesced: ticket.Coalesced,
		State:     ticket.State,
	}
}

func cloneLaneTicketPtr(ticket *LaneTicket) *LaneTicket {
	if ticket == nil {
		return nil
	}
	return cloneLaneTicket(*ticket)
}

func cloneIntegrityFindings(in []IntegrityFinding) []IntegrityFinding {
	if len(in) == 0 {
		return []IntegrityFinding{}
	}
	out := make([]IntegrityFinding, len(in))
	copy(out, in)
	for i := range out {
		out[i].MissingFiles = append([]string(nil), out[i].MissingFiles...)
		out[i].StaleFiles = append([]string(nil), out[i].StaleFiles...)
		out[i].MalformedFiles = append([]string(nil), out[i].MalformedFiles...)
		out[i].BlockedFeeds = append([]string(nil), out[i].BlockedFeeds...)
		out[i].RecoveryTargets = append([]string(nil), out[i].RecoveryTargets...)
	}
	return out
}

func cloneEntityIntegrityFindings(in []EntityIntegrityFinding) []EntityIntegrityFinding {
	if len(in) == 0 {
		return []EntityIntegrityFinding{}
	}
	out := make([]EntityIntegrityFinding, len(in))
	copy(out, in)
	return out
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
