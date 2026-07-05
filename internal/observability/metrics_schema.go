package observability

import "strconv"

func metricDescriptors() []metricDescriptor {
	return []metricDescriptor{
		counterMetric("api.recalculation.requests", labelAPI("api.surface"), labelAPIAction(), labelAPIResult()),
		counterMetric("api.recalculation.targets", labelAPI("api.surface"), labelAPIAction(), labelAPIResult()),
		counterMetric("background.tasks", labelBackgroundComponent(), labelResult()),
		durationMetric("background.worker.wait.duration_ms", "ms", labelBackgroundComponent()),
		counterMetric("background.worker.long_running", labelBackgroundComponent()),
		gaugeMetric("background.workers.active"),
		counterMetric("background.workers.attach_duplicate"),
		counterMetric("background.workers.finalization_panic"),
		gaugeMetric("background.workers.limit"),
		durationMetric("config.load.duration_ms", "ms", labelConfigResult()),
		counterMetric("config.loads", labelConfigResult()),
		counterMetric("daemon.goroutine.panics", labelDaemonGoroutine()),
		gaugeMetric("daemon.up"),
		counterMetric("daemon.watchdog.diagnostics"),
		counterMetric("download.errors", labelDownloadStatus()),
		counterMetric("download.fetch.bytes", labelDownloadStatus()),
		durationMetric("download.fetch.duration_ms", "ms", labelDownloadStatus()),
		counterMetric("download.fetches", labelDownloadStatus()),
		gaugeMetric("engine.phase.current", labelEnginePhase()),
		durationMetric("engine.phase.duration_ms", "ms", labelEnginePhase()),
		counterMetric("engine.lane.diagnostics.panics"),
		durationMetric("engine.run.duration_ms", "ms", labelRunReason(), labelRunStatus()),
		gaugeMetric("engine.running"),
		counterMetric("engine.runs", labelRunReason(), labelRunStatus()),
		counterMetric("file.write_atomic", labelFileSync()),
		counterMetric("file.write_atomic.bytes", labelFileSync()),
		durationMetric("file.write_atomic.duration_ms", "ms", labelFileSync()),
		gaugeMetric("feed.catalog.entries"),
		gaugeMetric("feed.catalog.errors"),
		gaugeMetric("feed.catalog.feeds"),
		gaugeMetric("feed.catalog.unique_ips"),
		durationMetric("http.server.request.duration", "s", labelHTTPRoute(), labelHTTPMethod(), labelHTTPStatus()),
		durationMetric("integrity.check.duration_ms", "ms", labelIntegrityKind(), labelIntegrityResult()),
		counterMetric("integrity.checks", labelIntegrityKind(), labelIntegrityResult()),
		gaugeMetric("integrity.findings", labelIntegrityKind()),
		counterMetric("integrity.recovery.targets", labelIntegrityKind(), labelIntegrityAction()),
		durationMetric("processor.run.duration_ms", "ms", labelProcessorMode(), labelProcessorStatus()),
		counterMetric("processor.runs", labelProcessorMode(), labelProcessorStatus()),
		durationMetric("processor.temp.write.duration_ms", "ms", labelProcessorTempKind()),
		counterMetric("processor.temp.writes", labelProcessorTempKind()),
		durationMetric("runtime.cache.operation.duration_ms", "ms", labelCacheOperation(), labelCacheResult()),
		counterMetric("runtime.cache.operations", labelCacheOperation(), labelCacheResult()),
		gaugeMetric("runtime.go.goroutines"),
		gaugeMetric("runtime.go.heap.alloc.bytes"),
		gaugeMetric("runtime.go.heap.sys.bytes"),
		gaugeMetric("runtime.go.heap.inuse.bytes"),
		gaugeMetric("runtime.go.heap.released.bytes"),
		gaugeMetric("runtime.go.heap.objects"),
		gaugeMetric("runtime.go.stack.inuse.bytes"),
		gaugeMetric("runtime.go.sys.bytes"),
		gaugeMetric("runtime.go.gc.count"),
		gaugeMetric("runtime.go.gc.pause.total.ms"),
		gaugeMetric("runtime.go.mem.limit.bytes"),
		gaugeMetric("runtime.process.cpu.user.ms"),
		gaugeMetric("runtime.process.cpu.system.ms"),
		gaugeMetric("runtime.process.cpu.total.ms"),
		counterMetric("scheduler.action.admission_failures"),
		durationMetric("scheduler.batch.duration_ms", "ms", labelSchedulerQueue()),
		gaugeMetric("scheduler.batch.items", labelSchedulerQueue()),
		counterMetric("scheduler.queue.admissions", labelSchedulerQueue(), labelSchedulerResult()),
		gaugeMetric("scheduler.queue.depth", labelSchedulerQueue()),
		counterMetric("scheduler.recovered_panics", labelSchedulerComponent()),
		counterMetric("scheduler.work.completed", labelSchedulerQueue()),
		counterMetric("scheduler.work.started", labelSchedulerQueue()),
		counterMetric("systemd.notify.failures"),
		counterMetric("telemetry.logs.dropped"),
		counterMetric("telemetry.metrics.unknown"),
		counterMetric("telemetry.traces.dropped"),
		gaugeMetric("web.artifact.cache.bytes"),
		gaugeMetric("web.artifact.cache.entries"),
		counterMetric("web.artifact.cache.evictions", labelCacheReason()),
		counterMetric("web.artifact.cache.lookups", labelCacheResult()),
	}
}

func counterMetric(name string, labels ...labelDescriptor) metricDescriptor {
	return metricDescriptor{name: name, kind: metricCounter, labels: labels}
}

func gaugeMetric(name string, labels ...labelDescriptor) metricDescriptor {
	return metricDescriptor{name: name, kind: metricGauge, labels: labels}
}

func durationMetric(name, unit string, labels ...labelDescriptor) metricDescriptor {
	return metricDescriptor{name: name, kind: metricDuration, unit: unit, labels: labels}
}

func newLabel(key string, values ...string) labelDescriptor {
	if len(values) == 0 {
		values = []string{"other"}
	}
	hasOther := false
	for _, value := range values {
		if value == "other" {
			hasOther = true
			break
		}
	}
	if !hasOther {
		values = append(values, "other")
	}
	out := labelDescriptor{
		key:        key,
		values:     append([]string(nil), values...),
		stringVals: make(map[string]uint16, len(values)),
		intVals:    make(map[int64]uint16, len(values)),
		boolVals:   make(map[bool]uint16, 2),
	}
	for i, value := range out.values {
		idx := uint16(i)
		out.stringVals[value] = idx
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			out.intVals[parsed] = idx
		}
		switch value {
		case "true":
			out.boolVals[true] = idx
		case "false":
			out.boolVals[false] = idx
		}
	}
	return out
}

func labelAPI(key string) labelDescriptor {
	return newLabel(key, "admin", "public")
}

func labelAPIAction() labelDescriptor {
	return newLabel("api.action",
		"artifact_recheck",
		"compose",
		"entity_integrity_refresh",
		"entity_rebuild",
		"feed_recheck",
		"feed_reprocess",
		"feed_search",
		"integrity_refresh",
		"integrity_reprocess",
		"run_due",
		"run_recheck",
		"run_reprocess",
		"search",
	)
}

func labelAPIResult() labelDescriptor {
	return newLabel("api.result", "clean", "conflict", "error", "in_progress", "ok", "rejected", "scheduled")
}

func labelBackgroundComponent() labelDescriptor {
	return newLabel("background.component",
		"engine_run",
		"entity_artifacts",
		"entity_artifacts_health",
		"integrity",
		"cleanup",
	)
}

func labelResult() labelDescriptor {
	return newLabel("background.result", "started", "completed", "failed", "unknown")
}

func labelConfigResult() labelDescriptor {
	return newLabel("config.result", "ok", "error", "unknown")
}

func labelDaemonGoroutine() labelDescriptor {
	return newLabel("daemon.goroutine",
		"sighup_loop",
		"sighup_reload",
		"runtime_stats_sampler",
		"startup_integrity_recovery",
		"startup_entity_artifacts",
		"startup_critical_infrastructure_cleanup",
		"shutdown_watcher",
		"systemd_stopping",
		"systemd_ready",
		"watchdog",
		"watchdog_self_health",
		"unknown",
	)
}

func labelDownloadStatus() labelDescriptor {
	return newLabel("download.status", "ok", "error", "not_modified", "same", "skipped", "failed")
}

func labelFileSync() labelDescriptor {
	return newLabel("file.sync", "true", "false")
}

func labelEnginePhase() labelDescriptor {
	return newLabel("engine.phase",
		"preflight",
		"sources",
		"geoip",
		"bogons",
		"asn",
		"critical_infrastructure",
		"entities",
		"metadata",
		"insights",
		"publish",
	)
}

func labelRunReason() labelDescriptor {
	return newLabel("run.reason",
		"scheduled_due",
		"manual_run",
		"manual_recheck",
		"manual_reprocess",
		"startup_integrity_reprocess",
		"integrity_reprocess",
		"dependency_update",
		"provider_defaults_update",
	)
}

func labelRunStatus() labelDescriptor {
	return newLabel("run.status", "ok", "error")
}

func labelHTTPRoute() labelDescriptor {
	return newLabel("http.route",
		"/",
		"/*",
		"/admin",
		"/admin/*",
		"/api/v1/status",
		"/api/v1/*",
		"/api/v1/admin/artifacts",
		"/api/v1/admin/artifacts/{name}",
		"/api/v1/admin/artifacts/{name}/{action}",
		"/api/v1/admin/artifacts/{name}/disable",
		"/api/v1/admin/artifacts/{name}/enable",
		"/api/v1/admin/artifacts/{name}/manifest",
		"/api/v1/admin/artifacts/{name}/recheck",
		"/api/v1/admin/artifacts/{name}/reprocess",
		"/api/v1/admin/feeds",
		"/api/v1/admin/feeds/{name}",
		"/api/v1/admin/feeds/{name}/{action}",
		"/api/v1/admin/feeds/{name}/disable",
		"/api/v1/admin/feeds/{name}/enable",
		"/api/v1/admin/feeds/{name}/manifest",
		"/api/v1/admin/feeds/{name}/recheck",
		"/api/v1/admin/feeds/{name}/reprocess",
		"/api/v1/admin/integrity",
		"/api/v1/admin/integrity/entities",
		"/api/v1/admin/integrity/entities/rebuild",
		"/api/v1/admin/integrity/entities/refresh",
		"/api/v1/admin/integrity/refresh",
		"/api/v1/admin/integrity/reprocess",
		"/api/v1/admin/run",
		"/api/v1/admin/schedule",
		"/api/v1/admin/status",
		"/api/v1/asns",
		"/api/v1/asns/{asn}",
		"/api/v1/categories",
		"/api/v1/client-ip",
		"/api/v1/compose",
		"/api/v1/countries",
		"/api/v1/countries/{code}",
		"/api/v1/home/globe",
		"/api/v1/home/summary",
		"/api/v1/ipsets",
		"/api/v1/ipsets/{name}",
		"/api/v1/ipsets/{name}/{action}",
		"/api/v1/ipsets/{name}/asn",
		"/api/v1/ipsets/{name}/asn/{provider}",
		"/api/v1/ipsets/{name}/bogons",
		"/api/v1/ipsets/{name}/bogons/{provider}",
		"/api/v1/ipsets/{name}/changesets",
		"/api/v1/ipsets/{name}/comparison",
		"/api/v1/ipsets/{name}/countries",
		"/api/v1/ipsets/{name}/countries/{provider}",
		"/api/v1/ipsets/{name}/data",
		"/api/v1/ipsets/{name}/history",
		"/api/v1/ipsets/{name}/infrastructure",
		"/api/v1/ipsets/{name}/infrastructure/{provider}",
		"/api/v1/ipsets/{name}/infrastructure/providers",
		"/api/v1/ipsets/{name}/insights",
		"/api/v1/ipsets/{name}/retention",
		"/api/v1/ipsets/{name}/search",
		"/api/v1/maintainers",
		"/api/v1/maintainers/{slug}",
		"/api/v1/methodology",
		"/api/v1/methodology/{slug}",
		"/api/v1/query",
		"/api/v1/search",
		"/api/v1/sets",
		"/api/v1/sets/{name}",
		"/api/v1/sets/{name}/{action}",
		"/api/v1/sets/{name}/asn",
		"/api/v1/sets/{name}/asn/{provider}",
		"/api/v1/sets/{name}/bogons",
		"/api/v1/sets/{name}/bogons/{provider}",
		"/api/v1/sets/{name}/changesets",
		"/api/v1/sets/{name}/comparison",
		"/api/v1/sets/{name}/countries",
		"/api/v1/sets/{name}/countries/{provider}",
		"/api/v1/sets/{name}/data",
		"/api/v1/sets/{name}/history",
		"/api/v1/sets/{name}/infrastructure",
		"/api/v1/sets/{name}/infrastructure/{provider}",
		"/api/v1/sets/{name}/infrastructure/providers",
		"/api/v1/sets/{name}/insights",
		"/api/v1/sets/{name}/retention",
		"/api/v1/sets/{name}/search",
		"/asns/{asn}",
		"/countries/{code}",
		"/files/{name}",
		"/healthz",
		"/ipsets/{name}",
		"/maintainers/{slug}",
		"/methodology/{slug}",
		"/metrics",
		"/mcp",
		"/static/*",
		"/world/*",
	)
}

func labelHTTPMethod() labelDescriptor {
	return newLabel("http.request.method", "GET", "HEAD", "OPTIONS", "POST")
}

func labelHTTPStatus() labelDescriptor {
	return newLabel("http.response.status_code", "200", "204", "304", "400", "401", "403", "404", "405", "409", "500", "503")
}

func labelIntegrityKind() labelDescriptor {
	return newLabel("integrity.kind", "artifact", "entity", "pipeline", "all", "unknown")
}

func labelIntegrityResult() labelDescriptor {
	return newLabel("integrity.result", "clean", "error", "findings", "issues", "ok", "in_progress", "scheduled", "unknown")
}

func labelIntegrityAction() labelDescriptor {
	return newLabel("integrity.action", "refresh", "rebuild", "recheck", "reprocess", "repair", "unknown")
}

func labelProcessorMode() labelDescriptor {
	return newLabel("processor.mode", "memory", "stream")
}

func labelProcessorStatus() labelDescriptor {
	return newLabel("processor.status", "ok", "error")
}

func labelProcessorTempKind() labelDescriptor {
	return newLabel("processor.temp.kind", "bytes", "copy", "stream")
}

func labelCacheOperation() labelDescriptor {
	return newLabel("cache.operation", "get", "set", "delete", "refresh", "load", "save", "unknown")
}

func labelCacheResult() labelDescriptor {
	return newLabel("cache.result", "hit", "miss", "ok", "error", "stale", "oversize", "unknown")
}

func labelCacheReason() labelDescriptor {
	return newLabel("cache.reason", "capacity", "expired", "manual", "reload", "max_bytes", "max_entries")
}

func labelSchedulerQueue() labelDescriptor {
	return newLabel("scheduler.queue", "download", "processing")
}

func labelSchedulerResult() labelDescriptor {
	return newLabel("scheduler.result", "queued", "deferred", "requeued")
}

func labelSchedulerComponent() labelDescriptor {
	return newLabel("scheduler.component",
		"action",
		"download_worker",
		"fetch_loop",
		"processing_batch",
		"processing_loop",
		"unknown",
	)
}
