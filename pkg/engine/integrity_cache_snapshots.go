package engine

import "strings"

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
