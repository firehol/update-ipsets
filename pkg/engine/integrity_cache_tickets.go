package engine

func pipelineIntegritySnapshotFromTicket(opts IntegrityOptions, ticket LaneTicket) PipelineIntegrityCacheSnapshot {
	state := IntegrityCacheRefreshQueued
	if ticket.State == LaneWorkActive {
		state = IntegrityCacheRefreshRunning
	}
	return PipelineIntegrityCacheSnapshot{
		CacheState:      state,
		Running:         state == IntegrityCacheRefreshRunning,
		Queued:          state == IntegrityCacheRefreshQueued,
		Coalesced:       ticket.Coalesced,
		Ticket:          cloneLaneTicket(ticket),
		IncludeArchived: opts.IncludeArchived,
		EnableAll:       opts.EnableAll,
		WebDir:          opts.WebDir,
		Findings:        []IntegrityFinding{},
	}
}

func entityIntegritySnapshotFromTicket(ticket LaneTicket) EntityIntegrityCacheSnapshot {
	state := IntegrityCacheRefreshQueued
	if ticket.State == LaneWorkActive {
		state = IntegrityCacheRefreshRunning
	}
	return EntityIntegrityCacheSnapshot{
		CacheState: state,
		Running:    state == IntegrityCacheRefreshRunning,
		Queued:     state == IntegrityCacheRefreshQueued,
		Coalesced:  ticket.Coalesced,
		Ticket:     cloneLaneTicket(ticket),
		Findings:   []EntityIntegrityFinding{},
	}
}
