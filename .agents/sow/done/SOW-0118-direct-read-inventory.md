# SOW-0118 Direct Read Inventory

Generated: 2026-06-26

Purpose: initial inventory for runtime reload snapshot work. This is a maintainer/SOW artifact, not public documentation.

Command:

```bash
rg -n '\b[a-zA-Z_][a-zA-Z0-9_]*\.(cfg|runtime|downloads|geoProviders|ledgerCache|retentionMaxWindow|asnLookupCache)\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'
```

Classification legend:

- `safe-locked`: current code copies or reads while holding `e.mu` and is expected to remain a safe helper.
- `reload-or-accessor`: current code is reload, startup, override, or accessor logic and must remain locked or be proved startup-only.
- `unsafe-convert`: current code is treated as unsafe until converted to an operation/request snapshot or proved safe with file/line evidence.

| Class | Location | Code |
|---|---|---|
| `unsafe-convert` | `pkg/web/admin_manifest_builder.go:122` | `	for _, provider := range b.cfg.SourcesWithUse(config.UseGeoIP) {` |
| `unsafe-convert` | `pkg/web/admin_manifest_builder.go:125` | `	for _, provider := range b.cfg.SourcesWithUse(config.UseASN) {` |
| `unsafe-convert` | `pkg/web/admin_manifest_builder.go:128` | `	for _, provider := range b.cfg.SourcesWithUse(config.UseBogons) {` |
| `reload-or-accessor` | `pkg/engine/engine.go:209` | `	if e.cfg == nil {` |
| `reload-or-accessor` | `pkg/engine/engine.go:212` | `	m := make(map[string]time.Duration, len(e.cfg.Sources))` |
| `reload-or-accessor` | `pkg/engine/engine.go:213` | `	for _, src := range e.cfg.Sources {` |
| `reload-or-accessor` | `pkg/engine/engine.go:227` | `	e.retentionMaxWindow = m` |
| `reload-or-accessor` | `pkg/engine/engine.go:236` | `	e.runtime.PushToGit = enabled` |
| `reload-or-accessor` | `pkg/engine/engine.go:287` | `	previousWebDir := e.runtime.WebDir` |
| `reload-or-accessor` | `pkg/engine/engine.go:288` | `	e.cfg = cfg` |
| `reload-or-accessor` | `pkg/engine/engine.go:289` | `	e.runtime = rt` |
| `reload-or-accessor` | `pkg/engine/engine.go:291` | `	effectiveRuntime = e.runtime` |
| `reload-or-accessor` | `pkg/engine/engine.go:293` | `	e.downloads = downloader.New(effectiveRuntime.MaxConnectTime, effectiveRuntime.MaxDownloadTime)` |
| `reload-or-accessor` | `pkg/engine/engine.go:298` | `	e.geoProviders = newGeoProviderCache()` |
| `reload-or-accessor` | `pkg/engine/engine.go:299` | `	if e.asnLookupCache == nil {` |
| `reload-or-accessor` | `pkg/engine/engine.go:300` | `		e.asnLookupCache = newASNDatabaseCache()` |
| `reload-or-accessor` | `pkg/engine/engine.go:302` | `		staleASNLookups = e.asnLookupCache.retireAll()` |
| `reload-or-accessor` | `pkg/engine/engine.go:304` | `	e.ledgerCache = newRuntimeLedgerCache()` |
| `reload-or-accessor` | `pkg/engine/engine.go:379` | `	return e.cfg` |
| `reload-or-accessor` | `pkg/engine/engine.go:388` | `	return e.runtime` |
| `reload-or-accessor` | `pkg/engine/engine.go:397` | `	return e.cfg, e.runtime` |
| `unsafe-convert` | `pkg/engine/home_aggregates.go:83` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_aggregates.go:119` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_aggregates.go:126` | `	policy := feedhealth.PolicyFromRuntime(e.cfg.Runtime)` |
| `unsafe-convert` | `pkg/engine/home_aggregates.go:139` | `		if !homeSummaryEligible(e.cfg, src, nil) {` |
| `unsafe-convert` | `pkg/engine/home_aggregates.go:311` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/asn_url_resolver.go:84` | `	req.Header.Set("User-Agent", e.runtime.UserAgent)` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_build.go:49` | `	workers := e.runtime.BackgroundWorkers()` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_build.go:90` | `	if e == nil \|\| e.cfg == nil \|\| entityBatch == nil {` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_build.go:93` | `	targetFeeds := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseGeoIP, config.UseASN, config.UseBogons)` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_build.go:116` | `	workers := e.runtime.HeavyPhaseWorkers()` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_build.go:177` | `			resolver := newEffectiveEntryResolver(e.cfg, entries)` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:210` | `	if e == nil \|\| e.cfg == nil \|\| e.state == nil {` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:221` | `		resolver: newEffectiveEntryResolver(e.cfg, entries),` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:222` | `		policy:   feedhealth.PolicyFromRuntime(e.cfg.Runtime),` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:236` | `	if c == nil \|\| c.e == nil \|\| c.e.cfg == nil \|\| c.resolver == nil {` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:287` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:314` | `		if !detailSurfaceEligible(e.cfg, src) {` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:362` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:390` | `		if !detailSurfaceEligible(e.cfg, src) {` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:438` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_entity_builders.go:455` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/retention_update.go:158` | `	dir := filepath.Join(e.runtime.LibDir, name)` |
| `unsafe-convert` | `pkg/engine/retention_update.go:211` | `		if err := normalizeChangesetLedgerHeader(e.runtime.LibDir, filepath.Join(name, "changesets.csv")); err != nil {` |
| `unsafe-convert` | `pkg/engine/retention_update.go:390` | `		oldSource, err := openRetentionCohortSet(ctx, candidate.baseName, e.runtime.LibDir, filepath.Join(name, "new", candidate.baseName), candidate.path)` |
| `safe-locked` | `pkg/engine/runtime_ledger_cache.go:37` | `	rt := e.runtime` |
| `safe-locked` | `pkg/engine/runtime_ledger_cache.go:38` | `	ledger := e.ledgerCache` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:20` | `	artifact := e.cfg.ArtifactByName(name)` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:36` | `	return e.runtime.MaxDownloadSize` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:54` | `		Timeout:  e.runtime.MaxDownloadTime,` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:62` | `	result, err := e.downloads.Fetch(ctx, downloader.Request{` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:67` | `		TmpDir:          e.runtime.TmpDir,` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:141` | `	artifact := e.cfg.ArtifactByName(artifactName)` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:181` | `		Timeout:       e.runtime.MaxDownloadTime,` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:199` | `		src := e.cfg.Sources[childName]` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:208` | `		result, err := e.downloads.Fetch(ctx, downloader.Request{` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:214` | `			TmpDir:          e.runtime.TmpDir,` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:232` | `	children := e.cfg.ArtifactChildren(artifactName)` |
| `unsafe-convert` | `pkg/engine/artifact_stage.go:235` | `		if src == nil \|\| !EffectiveSourceEnabled(e.cfg, e.runtime, src.Name, enableAll) {` |
| `unsafe-convert` | `pkg/engine/merge_inputs.go:48` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/merge_inputs.go:55` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/merge_inputs.go:59` | `	for _, name := range config.SortedSourceNames(e.cfg) {` |
| `unsafe-convert` | `pkg/engine/merge_inputs.go:60` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/merge_inputs.go:70` | `	if e == nil \|\| e.cfg == nil \|\| src == nil {` |
| `unsafe-convert` | `pkg/engine/merge_inputs.go:100` | `		row.Enabled = EffectiveSourceEnabledForRun(e.cfg, e.runtime, parent, enableAll, false)` |
| `unsafe-convert` | `pkg/engine/geoloc.go:16` | `	geoSources := e.cfg.SourcesWithUse(config.UseGeoIP)` |
| `unsafe-convert` | `pkg/engine/geoloc.go:21` | `	sourceDir := filepath.Join(e.runtime.LibDir, "geolocation")` |
| `unsafe-convert` | `pkg/engine/geoloc.go:74` | `			prepared, err := e.geoProviders.LoadOrParse(name, src.Format, processingPath)` |
| `unsafe-convert` | `pkg/engine/geoloc.go:127` | `	targetNames := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseGeoIP)` |
| `unsafe-convert` | `pkg/engine/geoloc.go:145` | `	numWorkers := e.runtime.HeavyPhaseWorkers()` |
| `unsafe-convert` | `pkg/engine/public_series.go:74` | `	if err := normalizeChangesetLedgerHeader(e.runtime.LibDir, filepath.Join(name, "changesets.csv")); err != nil {` |
| `unsafe-convert` | `pkg/engine/public_series.go:105` | `	data, err := readFileInRoot(e.runtime.LibDir, filepath.Join(name, "retention.json"))` |
| `unsafe-convert` | `pkg/engine/integrity.go:132` | `	if e == nil \|\| e.cfg == nil \|\| src == nil {` |
| `unsafe-convert` | `pkg/engine/integrity.go:149` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/integrity.go:172` | `		provider := e.cfg.Sources[artifact.Provider]` |
| `unsafe-convert` | `pkg/engine/integrity.go:205` | `	if e == nil \|\| e.cfg == nil \|\| src == nil \|\| src.Provenance != config.ProvenanceSecondaryRetention \|\| len(src.DerivedFrom) == 0 {` |
| `unsafe-convert` | `pkg/engine/integrity.go:218` | `		snapshots, err := readHistorySnapshots(filepath.Join(e.runtime.HistoryDir, parent))` |
| `unsafe-convert` | `pkg/engine/integrity.go:260` | `	if e == nil \|\| e.cfg == nil \|\| entry == nil \|\| src == nil {` |
| `unsafe-convert` | `pkg/engine/integrity.go:266` | `	health := feedhealth.Classify(entry, src, feedhealth.PolicyFromRuntime(e.cfg.Runtime), e.now().UTC())` |
| `unsafe-convert` | `pkg/engine/integrity.go:293` | `	if e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/integrity.go:296` | `	for _, p := range e.cfg.SourcesWithUse(config.UseGeoIP) {` |
| `unsafe-convert` | `pkg/engine/integrity.go:304` | `	for _, p := range e.cfg.SourcesWithUse(config.UseASN) {` |
| `unsafe-convert` | `pkg/engine/integrity.go:312` | `	for _, p := range e.cfg.SourcesWithUse(config.UseBogons) {` |
| `unsafe-convert` | `pkg/engine/integrity.go:320` | `	criticalProviders := e.cfg.SourcesWithUse(config.UseCriticalInfrastructure)` |
| `unsafe-convert` | `pkg/engine/integrity.go:321` | `	if len(criticalProviders) > 0 && !isCriticalInfrastructureOutputName(e.cfg, name) && isCriticalInfrastructureComparableName(e.cfg, name) {` |
| `unsafe-convert` | `pkg/engine/integrity.go:374` | `	path, ok := safeRuntimeFilePath(e.runtime.BaseDir, entry.File)` |
| `unsafe-convert` | `pkg/engine/home_globe.go:41` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/insights.go:73` | `	targetNames := insightTargetNames(e.cfg, updatedNames, e.publicOutputNames(), outDir, e.outputDir())` |
| `unsafe-convert` | `pkg/engine/insights.go:199` | `	data, err := readFileInRoot(e.runtime.LibDir, filepath.Join(name, "retention.json"))` |
| `unsafe-convert` | `pkg/engine/insights.go:384` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/insights.go:388` | `	for _, src := range e.cfg.SourcesWithUse(config.UseBogons) {` |
| `unsafe-convert` | `pkg/engine/web_ipsets.go:18` | `	if e.runtime.WebDirForIPSets == "" \|\| !dirExists(e.runtime.WebDirForIPSets) {` |
| `unsafe-convert` | `pkg/engine/web_ipsets.go:48` | `		if _, ok := safeRuntimeFilePath(e.runtime.BaseDir, entry.File); !ok {` |
| `unsafe-convert` | `pkg/engine/web_ipsets.go:52` | `		dst := filepath.Join(e.runtime.WebDirForIPSets, entry.File)` |
| `unsafe-convert` | `pkg/engine/web_ipsets.go:53` | `		mod, err := copyFileViaNewContext(ctx, e.runtime.BaseDir, entry.File, dst, e.runtime.WebOwner)` |
| `unsafe-convert` | `pkg/engine/output_comparison.go:503` | `	relatedness := newComparisonRelatedness(e.cfg)` |
| `unsafe-convert` | `pkg/engine/output_comparison.go:557` | `	family := leafAncestors(r.cfg, name)` |
| `unsafe-convert` | `pkg/engine/output_comparison.go:614` | `	if e != nil && e.runtime.WebOwner != "" {` |
| `unsafe-convert` | `pkg/engine/helpers.go:40` | `		"base_dir": e.runtime.BaseDir,` |
| `unsafe-convert` | `pkg/engine/helpers.go:41` | `		"BASE_DIR": e.runtime.BaseDir,` |
| `unsafe-convert` | `pkg/engine/helpers.go:52` | `		e.cfg,` |
| `unsafe-convert` | `pkg/engine/helpers.go:53` | `		e.runtime,` |
| `unsafe-convert` | `pkg/engine/helpers.go:61` | `	return sourcePathForRuntime(e.runtime, name)` |
| `unsafe-convert` | `pkg/engine/helpers.go:167` | `	for oldName, newName := range e.cfg.Renames {` |
| `unsafe-convert` | `pkg/engine/helpers.go:172` | `	for _, name := range e.cfg.Deleted {` |
| `unsafe-convert` | `pkg/engine/helpers.go:185` | `		oldPath := filepath.Join(e.runtime.BaseDir, oldName+suffix)` |
| `unsafe-convert` | `pkg/engine/helpers.go:186` | `		newPath := filepath.Join(e.runtime.BaseDir, newName+suffix)` |
| `unsafe-convert` | `pkg/engine/helpers.go:191` | `	for _, dir := range []string{e.runtime.HistoryDir, e.runtime.LibDir} {` |
| `unsafe-convert` | `pkg/engine/helpers.go:216` | `		if err := os.Remove(filepath.Join(e.runtime.BaseDir, name+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {` |
| `unsafe-convert` | `pkg/engine/helpers.go:225` | `	if err := os.RemoveAll(filepath.Join(e.runtime.HistoryDir, name)); err != nil {` |
| `unsafe-convert` | `pkg/engine/helpers.go:228` | `	if err := os.RemoveAll(filepath.Join(e.runtime.LibDir, name)); err != nil {` |
| `unsafe-convert` | `pkg/engine/helpers.go:254` | `	if e.cfg != nil {` |
| `unsafe-convert` | `pkg/engine/helpers.go:255` | `		for _, src := range e.cfg.SourcesWithUse(config.UseGeoIP) {` |
| `unsafe-convert` | `pkg/engine/helpers.go:258` | `		for _, src := range e.cfg.SourcesWithUse(config.UseASN) {` |
| `unsafe-convert` | `pkg/engine/helpers.go:261` | `		for _, src := range e.cfg.SourcesWithUse(config.UseBogons) {` |
| `unsafe-convert` | `pkg/engine/helpers.go:264` | `		for _, src := range e.cfg.SourcesWithUse(config.UseCriticalInfrastructure) {` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_single.go:43` | `	if !detailSurfaceEligible(e.cfg, src) {` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_single.go:59` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/entity_feed_sidecar_single.go:63` | `		resolver = newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries())` |
| `unsafe-convert` | `pkg/engine/asn.go:38` | `	asnSources := e.cfg.SourcesWithUse(config.UseASN)` |
| `unsafe-convert` | `pkg/engine/asn.go:43` | `	asnDir := filepath.Join(e.runtime.LibDir, "asn")` |
| `unsafe-convert` | `pkg/engine/asn.go:130` | `	targetNames := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseASN, config.UseBogons)` |
| `unsafe-convert` | `pkg/engine/asn.go:149` | `	numWorkers := e.runtime.HeavyPhaseWorkers()` |
| `unsafe-convert` | `pkg/engine/home_aggregate_integrity.go:11` | `	if e == nil \|\| e.cfg == nil \|\| findings == nil \|\| plan == nil {` |
| `unsafe-convert` | `pkg/engine/home_aggregate_integrity.go:73` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_aggregate_integrity.go:87` | `		if !homeSummaryEligible(e.cfg, src, nil) {` |
| `unsafe-convert` | `pkg/engine/public_url.go:9` | `	if base := normalizeAbsolutePublicURL(e.runtime.PublicBaseURL); base != "" {` |
| `unsafe-convert` | `pkg/engine/public_url.go:12` | `	return derivePublicSiteBaseFromWebURL(e.runtime.WebURL)` |
| `unsafe-convert` | `pkg/engine/public_url.go:16` | `	if prefix := normalizeAbsolutePublicURL(e.runtime.WebURL); prefix != "" {` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:48` | `		results = make(map[string]*sourceResult, len(e.cfg.Sources))` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:72` | `			src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:94` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:142` | `	currentCriticalProviderSetID := CriticalInfrastructureProviderSetIDForSnapshot(e.cfg)` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:147` | `	markerPath := CriticalInfrastructureProviderSetMarkerPath(e.runtime)` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:148` | `	criticalProviderSetChanged := markerPath != "" && readCriticalInfrastructureProviderSetMarker(e.runtime) != currentCriticalProviderSetID` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:165` | `		e.runtime.SkipComparisonIfNoUpdates &&` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:211` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/run_pipeline.go:316` | `		report.EntityRefreshTargets = targetFeedsForFanOut(e.cfg, plan.fanOutUpdated, e.publicOutputNames(), config.UseGeoIP, config.UseASN, config.UseBogons)` |
| `unsafe-convert` | `pkg/engine/legacy_failure_bootstrap.go:64` | `	if e == nil \|\| e.runtime.BaseDir == "" {` |
| `unsafe-convert` | `pkg/engine/legacy_failure_bootstrap.go:67` | `	root := filepath.Dir(e.runtime.BaseDir)` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:22` | `	if e == nil \|\| e.cfg == nil \|\| len(findings) == 0 {` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:45` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:48` | `	findingSource := e.cfg.Sources[finding.Feed]` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:80` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:83` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:94` | `		if e.cfg.ArtifactByName(src.ArtifactParent) != nil {` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:106` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/integrity_recovery.go:109` | `	src := e.cfg.Sources[name]` |
| `reload-or-accessor` | `pkg/engine/runtime.go:294` | `	rt = e.runtime` |
| `reload-or-accessor` | `pkg/engine/runtime.go:304` | `		e.runtime.WebDir = e.runtimeOverrideWebDir` |
| `reload-or-accessor` | `pkg/engine/runtime.go:305` | `		if e.cfg != nil {` |
| `reload-or-accessor` | `pkg/engine/runtime.go:306` | `			e.cfg.Runtime.WebDir = e.runtimeOverrideWebDir` |
| `reload-or-accessor` | `pkg/engine/runtime.go:310` | `		e.runtime.WebDirForIPSets = e.runtimeOverrideFilesDir` |
| `reload-or-accessor` | `pkg/engine/runtime.go:311` | `		if e.cfg != nil {` |
| `reload-or-accessor` | `pkg/engine/runtime.go:312` | `			e.cfg.Runtime.WebDirForIPSets = e.runtimeOverrideFilesDir` |
| `unsafe-convert` | `pkg/engine/run.go:255` | `	workers := e.runtime.MaxProcessingWorkers` |
| `unsafe-convert` | `pkg/engine/run.go:303` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/run.go:308` | `		names = append(names, config.SortedSourceNames(e.cfg)...)` |
| `unsafe-convert` | `pkg/engine/run.go:334` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/run.go:349` | `		lenA := len(e.cfg.Sources[a].DerivedFrom)` |
| `unsafe-convert` | `pkg/engine/run.go:350` | `		lenB := len(e.cfg.Sources[b].DerivedFrom)` |
| `safe-locked` | `pkg/engine/scheduler_config_snapshot.go:36` | `	if e.cfg == nil {` |
| `safe-locked` | `pkg/engine/scheduler_config_snapshot.go:39` | `	if len(e.cfg.Sources) > 0 {` |
| `safe-locked` | `pkg/engine/scheduler_config_snapshot.go:42` | `		for name, src := range e.cfg.Sources {` |
| `safe-locked` | `pkg/engine/scheduler_config_snapshot.go:54` | `	if len(e.cfg.Artifacts) > 0 {` |
| `safe-locked` | `pkg/engine/scheduler_config_snapshot.go:55` | `		snapshot.Artifacts = make(map[string]struct{}, len(e.cfg.Artifacts))` |
| `safe-locked` | `pkg/engine/scheduler_config_snapshot.go:56` | `		for name, artifact := range e.cfg.Artifacts {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:32` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:48` | `	if e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:51` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:56` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:59` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:64` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:67` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:72` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:83` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:108` | `	for _, name := range config.SortedSourceNames(e.cfg) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:109` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:148` | `			src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:177` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:191` | `	if e == nil \|\| e.cfg == nil \|\| e.cfg.Sources[name] == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:198` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:204` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:216` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:219` | `	src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:238` | `	if e.cfg.ArtifactByName(src.ArtifactParent) != nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:249` | `	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:264` | `		result, err = e.downloads.Fetch(ctx, downloader.Request{` |
| `unsafe-convert` | `pkg/engine/download_stage.go:268` | `			UserAgent:         e.runtime.UserAgent,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:269` | `			MaxConnectTime:    e.runtime.MaxConnectTime,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:270` | `			MaxDownloadTime:   e.runtime.MaxDownloadTime,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:276` | `			MaxDownloadSize:   e.runtime.MaxDownloadSize,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:277` | `			TmpDir:            e.runtime.TmpDir,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:310` | `	tmpDir := e.runtime.TmpDir` |
| `unsafe-convert` | `pkg/engine/download_stage.go:461` | `	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:484` | `	result, err := e.downloads.Fetch(ctx, downloader.Request{` |
| `unsafe-convert` | `pkg/engine/download_stage.go:488` | `		UserAgent:         e.runtime.UserAgent,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:489` | `		MaxConnectTime:    e.runtime.MaxConnectTime,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:490` | `		MaxDownloadTime:   e.runtime.MaxDownloadTime,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:494` | `		MaxDownloadSize:   e.runtime.MaxDownloadSize,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:495` | `		TmpDir:            e.runtime.TmpDir,` |
| `unsafe-convert` | `pkg/engine/download_stage.go:510` | `	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:555` | `	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:589` | `	if !EffectiveSourceEnabledForRun(e.cfg, e.runtime, name, enableAll, force) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:776` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:779` | `	targets := make([]string, 0, len(e.cfg.Sources))` |
| `unsafe-convert` | `pkg/engine/download_stage.go:780` | `	for _, name := range config.SortedSourceNames(e.cfg) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:781` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:788` | `		if !EffectiveSourceEnabled(e.cfg, e.runtime, name, enableAll) {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:800` | `	if e == nil \|\| e.cfg == nil \|\| decision == nil {` |
| `unsafe-convert` | `pkg/engine/download_stage.go:803` | `	dependents := e.cfg.Dependents()[parent]` |
| `unsafe-convert` | `pkg/engine/download_stage.go:812` | `		src := e.cfg.Sources[dep]` |
| `unsafe-convert` | `pkg/engine/effective_entry.go:111` | `	if r == nil \|\| r.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/effective_entry.go:114` | `	return r.cfg.SourceByName(name)` |
| `safe-locked` | `pkg/engine/status_snapshot.go:32` | `	maxIngestWorkers := e.runtime.MaxIngestWorkers` |
| `safe-locked` | `pkg/engine/status_snapshot.go:33` | `	parallelDownloads := e.runtime.ParallelDownloads` |
| `safe-locked` | `pkg/engine/status_snapshot.go:34` | `	parallelDNSQueries := e.runtime.ParallelDNSQueries` |
| `safe-locked` | `pkg/engine/status_snapshot.go:35` | `	maxProcessingWorkers := e.runtime.MaxProcessingWorkers` |
| `safe-locked` | `pkg/engine/status_snapshot.go:36` | `	maxHeavyPhaseWorkers := e.runtime.HeavyPhaseWorkers()` |
| `safe-locked` | `pkg/engine/status_snapshot.go:37` | `	maxBackgroundWorkers := e.runtime.BackgroundWorkers()` |
| `safe-locked` | `pkg/engine/status_snapshot.go:38` | `	maxEngineLaneWorkers := e.runtime.EngineLaneWorkers()` |
| `safe-locked` | `pkg/engine/status_snapshot.go:39` | `	sourceCount := len(e.cfg.Sources)` |
| `safe-locked` | `pkg/engine/status_snapshot.go:40` | `	mergeCount := mergeCountForConfig(e.cfg)` |
| `safe-locked` | `pkg/engine/status_snapshot.go:121` | `	configPath := e.runtime.ConfigPath` |
| `safe-locked` | `pkg/engine/status_snapshot.go:122` | `	baseDir := e.runtime.BaseDir` |
| `safe-locked` | `pkg/engine/status_snapshot.go:123` | `	maxIngestWorkers := e.runtime.MaxIngestWorkers` |
| `safe-locked` | `pkg/engine/status_snapshot.go:124` | `	parallelDownloads := e.runtime.ParallelDownloads` |
| `safe-locked` | `pkg/engine/status_snapshot.go:125` | `	parallelDNSQueries := e.runtime.ParallelDNSQueries` |
| `safe-locked` | `pkg/engine/status_snapshot.go:126` | `	maxProcessingWorkers := e.runtime.MaxProcessingWorkers` |
| `safe-locked` | `pkg/engine/status_snapshot.go:127` | `	maxHeavyPhaseWorkers := e.runtime.HeavyPhaseWorkers()` |
| `safe-locked` | `pkg/engine/status_snapshot.go:128` | `	maxBackgroundWorkers := e.runtime.BackgroundWorkers()` |
| `safe-locked` | `pkg/engine/status_snapshot.go:129` | `	maxEngineLaneWorkers := e.runtime.EngineLaneWorkers()` |
| `safe-locked` | `pkg/engine/status_snapshot.go:130` | `	sourceCount := len(e.cfg.Sources)` |
| `safe-locked` | `pkg/engine/status_snapshot.go:131` | `	mergeCount := mergeCountForConfig(e.cfg)` |
| `unsafe-convert` | `pkg/engine/output_comparison_pair_ledger.go:94` | `	if e == nil \|\| e.runtime.CacheDir == "" {` |
| `unsafe-convert` | `pkg/engine/output_comparison_pair_ledger.go:97` | `	return filepath.Join(e.runtime.CacheDir, comparisonPairLedgerFileName)` |
| `unsafe-convert` | `pkg/engine/output_comparison_pair_ledger.go:101` | `	if e == nil \|\| e.runtime.CacheDir == "" {` |
| `unsafe-convert` | `pkg/engine/output_comparison_pair_ledger.go:104` | `	return filepath.Join(e.runtime.CacheDir, comparisonPairLegacyLedgerFileName)` |
| `unsafe-convert` | `pkg/engine/entity_artifacts.go:30` | `	return filepath.Join(e.runtime.LibDir, "entities")` |
| `unsafe-convert` | `pkg/engine/critical.go:393` | `	sources := e.cfg.SourcesWithUse(config.UseCriticalInfrastructure)` |
| `unsafe-convert` | `pkg/engine/critical.go:457` | `	if len(e.cfg.CriticalASNContext) > 0 {` |
| `unsafe-convert` | `pkg/engine/critical.go:460` | `	targetNames := criticalTargetNames(e.cfg, targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), roles...))` |
| `unsafe-convert` | `pkg/engine/critical.go:467` | `	numWorkers := e.runtime.HeavyPhaseWorkers()` |
| `unsafe-convert` | `pkg/engine/critical.go:543` | `	if e == nil \|\| e.cfg == nil \|\| len(e.cfg.CriticalASNContext) == 0 {` |
| `unsafe-convert` | `pkg/engine/critical.go:553` | `	contextByASN := make(map[uint32]config.CriticalASNContext, len(e.cfg.CriticalASNContext))` |
| `unsafe-convert` | `pkg/engine/critical.go:554` | `	for _, entry := range e.cfg.CriticalASNContext {` |
| `unsafe-convert` | `pkg/engine/critical.go:608` | `	for _, provider := range e.cfg.SourcesWithUseDefaultFirst(config.UseASN) {` |
| `unsafe-convert` | `pkg/engine/public_compose.go:54` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/public_compose.go:57` | `	for name, src := range e.cfg.Sources {` |
| `unsafe-convert` | `pkg/engine/public_compose.go:68` | `	for name := range e.cfg.Merges {` |
| `unsafe-convert` | `pkg/engine/bootstrap_entries.go:36` | `	if e == nil \|\| e.cfg == nil \|\| e.state == nil {` |
| `unsafe-convert` | `pkg/engine/bootstrap_entries.go:41` | `	for _, name := range config.SortedArtifactNames(e.cfg) {` |
| `unsafe-convert` | `pkg/engine/bootstrap_entries.go:45` | `		artifact := e.cfg.ArtifactByName(name)` |
| `unsafe-convert` | `pkg/engine/bootstrap_entries.go:56` | `	for _, name := range config.SortedSourceNames(e.cfg) {` |
| `unsafe-convert` | `pkg/engine/bootstrap_entries.go:60` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/artifact_paths.go:32` | `	return e != nil && e.cfg != nil && e.cfg.ArtifactByName(name) != nil` |
| `unsafe-convert` | `pkg/engine/artifact_paths.go:36` | `	return artifactRootDirForRuntime(e.runtime, name)` |
| `unsafe-convert` | `pkg/engine/artifact_paths.go:40` | `	return sourceEnablePathForRuntime(e.runtime, name)` |
| `unsafe-convert` | `pkg/engine/artifact_paths.go:44` | `	return artifactEnablePathForRuntime(e.runtime, name)` |
| `unsafe-convert` | `pkg/engine/artifact_paths.go:48` | `	return artifactSourcePathForRuntime(e.runtime, name)` |
| `unsafe-convert` | `pkg/engine/artifact_paths.go:52` | `	return artifactExtractDirForRuntime(e.runtime, name)` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:64` | `		viewResolver:   newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries()),` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:70` | `		baseGit:        output.HasGitDir(e.runtime.BaseDir),` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:161` | `			Path:            filepath.Join(r.e.runtime.BaseDir, viewEntry.File),` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:182` | `		setInfoPath := filepath.Join(r.e.runtime.BaseDir, name+".setinfo")` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:189` | `		r.addGenerated(filepath.Join(r.e.runtime.BaseDir, viewEntry.File), time.Unix(viewEntry.SourceDate, 0).UTC(), redistributable)` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:252` | `	if err := output.WriteREADME(r.e.runtime.BaseDir, r.setInfo); err != nil {` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:256` | `	if err := output.WriteGitIgnore(r.e.runtime.BaseDir, r.generated); err != nil {` |
| `unsafe-convert` | `pkg/engine/metadata_write.go:260` | `	if err := output.WriteTimestampScript(r.e.runtime.BaseDir, r.timestampFiles); err != nil {` |
| `unsafe-convert` | `pkg/engine/entity_detail_selection.go:24` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:23` | `	return downloader.PrepareCanonicalFeedBody(ctx, name, src.Output, inputPath, processorSteps(src), e.runtime.TmpDir, e.runtime.ParallelDNSQueries)` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:288` | `	if e == nil \|\| e.retentionMaxWindow == nil {` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:291` | `	if window, ok := e.retentionMaxWindow[parent]; !ok \|\| window <= 0 {` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:294` | `	dir := filepath.Join(e.runtime.HistoryDir, parent)` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:331` | `	if e == nil \|\| e.retentionMaxWindow == nil {` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:334` | `	window, ok := e.retentionMaxWindow[parent]` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:338` | `	dir := filepath.Join(e.runtime.HistoryDir, parent)` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:389` | `	dir := filepath.Join(e.runtime.HistoryDir, parent)` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:406` | `	parentSet, err := parseFeedBodyFile(ctx, parent, parentPath, e.runtime.ParallelDNSQueries)` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:473` | `	set, err := parseFeedBodyReader(ctx, src.Name, io.MultiReader(readers...), e.runtime.ParallelDNSQueries)` |
| `unsafe-convert` | `pkg/engine/feed_body_stage.go:483` | `		excludeSet, err := parseFeedBodyReader(ctx, src.Name+"_exclude", io.MultiReader(excludeReaders...), e.runtime.ParallelDNSQueries)` |
| `unsafe-convert` | `pkg/engine/unique_share.go:36` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/unique_share.go:59` | `	familyCache := make(map[string]map[string]bool, len(e.cfg.Sources))` |
| `unsafe-convert` | `pkg/engine/unique_share.go:64` | `		family := leafAncestors(e.cfg, name)` |
| `unsafe-convert` | `pkg/engine/entity_artifacts_write.go:82` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/entity_artifacts_write.go:92` | `	targetFeeds := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseGeoIP, config.UseASN, config.UseBogons)` |
| `unsafe-convert` | `pkg/engine/home_detail.go:153` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_detail.go:160` | `	policy := feedhealth.PolicyFromRuntime(e.cfg.Runtime)` |
| `unsafe-convert` | `pkg/engine/home_detail.go:171` | `		if !homeSummaryEligible(e.cfg, src, nil) {` |
| `unsafe-convert` | `pkg/engine/home_detail.go:253` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/home_detail.go:257` | `	policy := feedhealth.PolicyFromRuntime(e.cfg.Runtime)` |
| `unsafe-convert` | `pkg/engine/home_detail.go:272` | `		if !homeSummaryEligible(e.cfg, src, nil) {` |
| `unsafe-convert` | `pkg/engine/markdown.go:18` | `	dir := filepath.Join(e.runtime.ConfigPath, markdownTemplatesSubdir)` |
| `unsafe-convert` | `pkg/engine/markdown.go:116` | `		if !homeSummaryEligible(e.cfg, src, nil) {` |
| `unsafe-convert` | `pkg/engine/home_summary.go:93` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/history_stats.go:16` | `	if e.runtime.LibDir == "" {` |
| `unsafe-convert` | `pkg/engine/history_stats.go:19` | `	points := parseHistoryCSVInRoot(e.runtime.LibDir, filepath.Join(name, "history.csv"), name)` |
| `unsafe-convert` | `pkg/engine/download_recovery.go:30` | `	for _, name := range config.SortedArtifactNames(e.cfg) {` |
| `unsafe-convert` | `pkg/engine/finalize.go:22` | `	if e.runtime.IPSetsApply {` |
| `unsafe-convert` | `pkg/engine/finalize.go:25` | `			if e.runtime.ErrorsDir != "" {` |
| `unsafe-convert` | `pkg/engine/finalize.go:26` | `				if writeErr := writeFeedBodyAtomic(filepath.Join(e.runtime.ErrorsDir, filepath.Base(path)), nil, bodyPath, time.Time{}); writeErr != nil {` |
| `unsafe-convert` | `pkg/engine/finalize.go:39` | `	latestDir := filepath.Join(e.runtime.LibDir, name)` |
| `unsafe-convert` | `pkg/engine/process.go:22` | `	return ensureDirectoriesForRuntime(e.runtime)` |
| `unsafe-convert` | `pkg/engine/process.go:123` | `	parseOpts.DNSThreads = e.runtime.ParallelDNSQueries` |
| `unsafe-convert` | `pkg/engine/integrity_check.go:78` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/integrity_check.go:83` | `		webDir = e.runtime.WebDir` |
| `unsafe-convert` | `pkg/engine/integrity_check.go:85` | `	baseDir := e.runtime.BaseDir` |
| `unsafe-convert` | `pkg/engine/integrity_check.go:94` | `		resolver: newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries()),` |
| `unsafe-convert` | `pkg/engine/integrity_check.go:100` | `	for _, name := range config.SortedSourceNames(c.e.cfg) {` |
| `unsafe-convert` | `pkg/engine/integrity_check.go:120` | `	src := c.e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/entity_integrity.go:258` | `	if e == nil \|\| e.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/critical_feed_writer.go:170` | `		Family:              criticalFeedFamily(w.e.cfg, w.name),` |
| `unsafe-convert` | `pkg/engine/integrity_payloads.go:87` | `	if eng == nil \|\| eng.cfg == nil {` |
| `unsafe-convert` | `pkg/engine/integrity_payloads.go:117` | `	for _, provider := range eng.cfg.SourcesWithUse(config.UseGeoIP) {` |
| `unsafe-convert` | `pkg/engine/integrity_payloads.go:126` | `	for _, provider := range eng.cfg.SourcesWithUse(config.UseASN) {` |
| `unsafe-convert` | `pkg/engine/integrity_payloads.go:135` | `	for _, provider := range eng.cfg.SourcesWithUse(config.UseBogons) {` |
| `unsafe-convert` | `pkg/engine/integrity_payloads.go:147` | `	for _, provider := range eng.cfg.SourcesWithUse(config.UseCriticalInfrastructure) {` |
| `safe-locked` | `pkg/engine/ip_context.go:335` | `	cfg := e.cfg` |
| `safe-locked` | `pkg/engine/ip_context.go:336` | `	rt := e.runtime` |
| `safe-locked` | `pkg/engine/ip_context.go:337` | `	geoProviders := e.geoProviders` |
| `safe-locked` | `pkg/engine/ip_context.go:338` | `	asnLookupCache := e.asnLookupCache` |
| `unsafe-convert` | `pkg/engine/metadata.go:153` | `	fmt.Fprintf(&buf, "# available at:\n#\n#  %s%s\n#\n", e.runtime.WebURL, name)` |
| `unsafe-convert` | `pkg/engine/metadata.go:168` | `			name, e.runtime.WebURL, name, entry.Info, entry.IPV, entry.Hash, quantity, freq, entry.PublicURL)` |
| `unsafe-convert` | `pkg/engine/metadata.go:171` | `		name, e.runtime.WebURL, name, entry.Info, entry.IPV, entry.Hash, quantity, freq)` |
| `unsafe-convert` | `pkg/engine/entry_timestamp_sanitize.go:17` | `	if e == nil \|\| e.cfg == nil \|\| e.state == nil {` |
| `unsafe-convert` | `pkg/engine/entry_timestamp_sanitize.go:22` | `	for _, name := range config.SortedSourceNames(e.cfg) {` |
| `unsafe-convert` | `pkg/engine/entry_timestamp_sanitize.go:23` | `		src := e.cfg.Sources[name]` |
| `unsafe-convert` | `pkg/engine/bogons.go:65` | `	bogonSources := e.cfg.SourcesWithUse(config.UseBogons)` |
| `unsafe-convert` | `pkg/engine/bogons.go:180` | `	targetNames := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseBogons)` |
| `unsafe-convert` | `pkg/engine/bogons.go:208` | `	numWorkers := e.runtime.HeavyPhaseWorkers()` |
| `unsafe-convert` | `pkg/engine/home_entity_detail_live.go:100` | `		if !detailSurfaceEligible(e.cfg, src) {` |
| `unsafe-convert` | `pkg/engine/home_entity_detail_live.go:169` | `		if !detailSurfaceEligible(e.cfg, src) {` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:53` | `		filepath.Join(e.runtime.LibDir, name, "latest"),` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:54` | `		filepath.Join(e.runtime.LibDir, name, "latest.set"),` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:111` | `	path := filepath.Join(e.runtime.LibDir, "geolocation", provider+".source")` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:134` | `	path := filepath.Join(e.runtime.LibDir, "asn", provider, spec.dataFile)` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:147` | `		path := filepath.Join(e.runtime.LibDir, name, filename)` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:169` | `	if e.runtime.ConfigPath != "" {` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:170` | `		info, err := os.Stat(e.runtime.ConfigPath)` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:176` | `			mergeEntityDependencyRef(&ref, e.runtime.ConfigPath, info.ModTime().UTC())` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:179` | `	for _, dir := range []string{e.runtime.DistributionSuppliedIPSets, e.runtime.AdminSuppliedIPSets, e.runtime.UserSuppliedIPSets} {` |
| `unsafe-convert` | `pkg/engine/entity_integrity_refs.go:228` | `	if e == nil \|\| e.cfg == nil {` |

## Post-Implementation Reconciliation - 2026-06-26

Post-change command:

```bash
rg -n '\b(e|eng|r\.e|r\.eng|c\.e|w\.e|b\.e)\.(cfg|runtime|downloads|geoProviders|ledgerCache|retentionMaxWindow|asnLookupCache)\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'
```

Remaining direct reload-swapped-field rows:

| Class | Locations | Status |
|---|---|---|
| Engine reload/lifecycle | `pkg/engine/engine.go:209,212,213,227,236,287,288,289,291,293,298,299,300,302,304,379,388,397` | Expected direct reads/writes inside engine construction, reload publication, lock-protected accessors, and lifecycle setters. |
| Operation snapshot copy | `pkg/engine/runtime_snapshot.go:30,31,32,33,34,35,36,37,47` | Expected locked snapshot capture; this is the SOW-0118 boundary. |
| Runtime override | `pkg/engine/runtime.go:294,304,307` | Expected lock-protected runtime override. This path no longer mutates `e.cfg.Runtime`. |
| Runtime ledger snapshot | `pkg/engine/runtime_ledger_cache.go:37,38` | Expected short lock-protected local copy of runtime and ledger cache before filesystem work. |
| Scheduler config snapshot | `pkg/engine/scheduler_config_snapshot.go:35` | Expected lock-protected config snapshot helper. |
| IP lookup context snapshot | `pkg/engine/ip_context.go:335,336,337,338` | Expected lock-protected request snapshot of config, runtime, provider cache, and ASN lookup cache. |
| Status snapshots | `pkg/engine/status_snapshot.go:32-40,121-131` | Expected short lock-protected status snapshots for admin/watchdog visibility. |

Post-change `e.state` assignment command:

```bash
rg -n '\b[a-zA-Z_][a-zA-Z0-9_]*\.state\s*=' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'
```

Result:

- No `e.state =` engine state pointer reassignment was found.
- Matches are work-lane item state transitions and entity-integrity cache state
  transitions. They do not replace the engine state pointer captured by active
  work.

Post-change nested-lock scan result:

- A function-level scan for functions containing both `e.mu.Lock()` /
  `e.mu.RLock()` and `operationSnapshot()` found only
  `pkg/engine/runtime_snapshot.go:23`, which is the snapshot helper itself.
- No current caller captures an operation snapshot while already holding the
  broad engine mutex.
