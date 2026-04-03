# Field population gaps — HIGH + MEDIUM fixes

## TL;DR

Four external AI reviewers (qwen, kimi, minimax, plus one failed glm) audited `cache.Entry` field population across all four feed kinds. They converged on a set of gaps left unfixed by commit `b3a8073`. This TODO captures the **HIGH and MEDIUM** severity gaps that are worth fixing now as a single commit. LOW severity items are deferred.

## Status

- [x] H1 — Merge early-return paths never touch `cache.Entry`
- [x] H2 — Geo/ASN fetch-error branches only log
- [x] M1 — Missing env var path doesn't set status (geo/asn)
- [x] M2 — Unexpected downloader status only increments counter (geo/asn)
- [x] M3 — `StartedDate` never set for geo/asn
- [x] M4 — `ClockSkewSeconds` never computed for geo/asn
- [x] Build passes (`go build ./...`)
- [x] Tests pass (`go test ./...`)
- [x] Commit created — `c8d5d03` "Persist merge and geo/asn outcomes to the cache entry"

**Assigned to**: background agent (general-purpose, worktree isolation)
**Status**: pending spawn

## Background

This is Phase 2 of the field population cleanup. Phase 1 (commit `b3a8073`) added `applyEntryStatsUpdate` helper, fixed Entries/UniqueIPs gaps, added LastStatus/LastError handling for file-missing / parse-failed / open-failed / stale / re-extract-failed branches.

The gaps below were found by multi-model audit and verified against HEAD by hand.

## Gaps to fix

### H1 — Merge early-return paths never touch `cache.Entry`

**Severity**: HIGH

**Problem**: `processMerge` in `pkg/engine/process.go:453-581` returns `(status, msg, err)` tuples from multiple early-exit branches. The caller in `pkg/engine/run.go:155-165` writes these to `report.Statuses[name]` and `report.Messages[name]` (run report) but **never updates the persistent `cache.Entry.LastStatus` / `LastError`**. The admin API reads from the cache entry, so it shows the merge's previous successful run indefinitely, regardless of what's happening now.

**Early-exit locations** (verified in current HEAD):
- `process.go:455` — nil merge
- `process.go:459` — disabled (`!e.isEnabled(name, opts)`)
- `process.go:465` — `touchFileAt` failed
- `process.go:494` — no files to merge
- `process.go:499` — source files not updated
- `process.go:507, 519, 525, 537, 545, 548, 552, 555` — merge streaming I/O failures

**Fix pattern**:
Before each `return "status", "msg", err`, set the entry fields:
```go
entry := e.state.Entry(name)
entry.LastStatus = "<status>"
entry.LastError = "<msg>" // empty string for success-like skips (disabled, not_due, same)
entry.CheckedDate = e.now().UTC().Unix()
```

To keep this DRY, consider introducing a small helper in `pkg/engine/process.go`:
```go
// recordMergeOutcome writes the merge's final status to the persistent
// cache entry so the admin API reflects the latest run. Every early-exit
// path in processMerge should call this before returning.
func (e *Engine) recordMergeOutcome(name, status, msg string) {
    entry := e.state.Entry(name)
    entry.LastStatus = status
    if isErrorStatus(status) {
        entry.LastError = msg
    } else {
        entry.LastError = ""
    }
    entry.CheckedDate = e.now().UTC().Unix()
}
```

Then call `e.recordMergeOutcome(name, "disabled", "")` before `return "skipped", "disabled", nil`, etc.

**Reference**: Sources handle this in `applyFetchOutcome` / `processConcreteSource` where every return path sets `LastStatus` / `LastError` before returning.

### H2 — Geo/ASN fetch-error branches only log

**Severity**: HIGH

**Problem**: When `e.downloads.Fetch()` returns `err != nil` (network error, timeout, DNS failure), the code only logs. No `DownloadFailures++`, no `LastStatus`, no `LastError`. The entry keeps its previous state.

**Locations**:
- `pkg/engine/geoloc.go:79-81` — the `if err != nil { e.logger.Error(...) }` branch
- `pkg/engine/asn.go:110-112` — the equivalent `if err != nil` branch

**Fix pattern**:
```go
if err != nil {
    e.logger.Error("geolocation download failed", "name", name, "error", err)
    entry.DownloadFailures++
    entry.LastStatus = "download_failed"
    entry.LastError = err.Error()
} else {
    // existing switch on result.Status
}
```

**Reference**: `pkg/engine/process.go` `applyFetchOutcome` does this for sources. Look at how `outcome.Err != nil` is handled there for the authoritative pattern.

### M1 — Missing env var path doesn't set status

**Severity**: MEDIUM

**Problem**: When a URL template references an env var that's unset, `expandedURL == ""` and the code sets `needDownload = false` and continues. No `LastStatus` / `LastError` update. The admin row shows whatever status was set on the previous run.

**Locations**:
- `pkg/engine/geoloc.go:60-63`
- `pkg/engine/asn.go` — equivalent branch (grep for `expandedURL == "" && feed.URL != ""`)

**Fix pattern**:
```go
if expandedURL == "" && feed.URL != "" {
    e.logger.Warn("...", "url_template", feed.URL)
    needDownload = false
    entry.LastStatus = "missing_env"
    entry.LastError = "URL template references an unset environment variable: " + feed.URL
}
```

### M2 — Unexpected downloader status only increments counter

**Severity**: MEDIUM

**Problem**: In the switch over `result.Status` inside geo/asn, the `default:` branch only increments `DownloadFailures` and logs. No `LastStatus` / `LastError`.

**Locations**:
- `pkg/engine/geoloc.go:97-101`
- `pkg/engine/asn.go:144-147`

**Fix pattern**:
```go
default:
    result.CleanUp()
    entry.DownloadFailures++
    entry.LastStatus = "download_failed"
    entry.LastError = result.Message
    e.logger.Warn("... fetch finished with non-update status", ...)
```

### M3 — `StartedDate` never set for geo/asn

**Severity**: MEDIUM

**Problem**: Sources set `StartedDate` in `finalize.go:63-64` on the first successful run (`if entry.StartedDate == 0 { entry.StartedDate = entry.ProcessedDate }`). Geo and ASN never touch this field, so operators cannot see when a provider was first loaded.

**Locations**:
- `pkg/engine/geoloc.go` — after `entry.ProcessedDate = e.now().UTC().Unix()` line (around line 124)
- `pkg/engine/asn.go` — after the equivalent `entry.ProcessedDate = ...` line (around line 171)

**Fix pattern**:
```go
entry.ProcessedDate = e.now().UTC().Unix()
if entry.StartedDate == 0 {
    entry.StartedDate = entry.ProcessedDate
}
```

**Reference**: `pkg/engine/finalize.go:63-64`.

### M4 — `ClockSkewSeconds` never computed for geo/asn

**Severity**: MEDIUM

**Problem**: Sources compute clock skew in `finalize.go:84-88` by comparing the source mtime to the local clock. Geo and ASN have an analogous timestamp (`archiveTime` — the downloaded archive's `ModifiedTime`) but never compute skew against it.

**Locations**:
- `pkg/engine/geoloc.go` — after the successful processing block, use `archiveTime` as the reference
- `pkg/engine/asn.go` — same, use `archiveTime`

**Fix pattern**:
```go
now := e.now().UTC()
if archiveTime.After(now) {
    entry.ClockSkewSeconds = int64(archiveTime.Sub(now).Seconds())
} else {
    entry.ClockSkewSeconds = 0
}
```

**Reference**: `pkg/engine/finalize.go:84-88`.

## Constraints

- **Single commit** covering all 6 gaps. Use a clear commit message that lists each gap and cites the corresponding file:line.
- **Do not install, do not restart any daemon.** Production deployment is Costa's call, not the agent's.
- **Do not modify files outside `pkg/engine/`** except for tests if strictly needed. No config changes. No frontend changes.
- **Run `go build ./...` and `go test ./...` and confirm both pass** before committing.
- **Never use `git add -A`.** Add only the specific files changed.
- **Never mention the agent, Claude, or any AI product** in the commit message or code comments.
- **No emojis anywhere** in commit messages or code.

## Testing requirements

- `go build ./...` — must pass with no warnings
- `go test ./...` — all packages green
- Manual inspection of the diff to verify every early-return in `processMerge` now sets the entry before returning
- Manual inspection of geo/asn `if err != nil` and `default:` branches to verify all fields are set

## Documentation updates

None — the field semantics are already documented in comments on the struct fields.

## Out of scope (LOW severity, deferred)

- L1: `CheckedDate` for URL-less sources — rarely triggered, cosmetic
- L2: Merge cadence stats seeded with `frequency=1` — 14 rows, display only
- L3: `Downloader` / `DownloaderOptions` fields unused for geo/asn — config structs don't have these fields; would require adding them first

## Follow-up

After the commit lands, Costa will install + restart + trigger a rebuild to validate in production. That's a separate step that only Costa authorizes.
