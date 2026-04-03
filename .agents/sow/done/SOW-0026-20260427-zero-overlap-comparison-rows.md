# SOW-0026 | 2026-04-27 | zero-overlap-comparison-rows

## Status

completed — public comparison artifacts now contain only non-zero overlaps;
stale explicit zero rows are removed during comparison writes and flagged by
integrity.

## Requirements

### Purpose

Explain why the public overlap list for `bitwire_inbound` includes rows with
zero shared IPs, and identify whether this is expected product behavior or a
bug that needs a code change.

### User request quoted verbatim

> Check the overlaps of http://costa-desktop:18888/ipsets/bitwire_inbound
> Why there are entries with zero overlap in this list?

> It seems that overlaps keeps the non-overlapping. Does this also happen to
> countries and ASNs. If this happens widely, we are just producing noise.

### Assistant understanding

- Stated: Costa observed zero-overlap rows on the `bitwire_inbound` public
  feed page.
- Stated: Costa wants the cause, not a guess.
- Inferred: the relevant evidence is the served comparison artifact/API, the
  pairwise comparison writer, and the UI table filtering behavior.

### Acceptance criteria

1. Confirm whether zero rows come from the backend artifact or only the UI.
2. Identify the code path that creates or preserves zero-overlap rows.
3. Classify the zero rows for `bitwire_inbound` by empty peers vs non-empty
   non-overlapping peers.
4. Present fix options with evidence and risks before implementation.
5. Check whether country and ASN attribution payloads have the same zero-row
   behavior.

## Analysis

### Evidence

- `curl -fsS http://costa-desktop:18888/api/v1/sets/bitwire_inbound` showed
  `ips: 22715820`, so the target feed is not empty.
- `curl -fsS http://costa-desktop:18888/api/v1/sets/bitwire_inbound/compare`
  returned 385 rows, including 33 rows where `common == 0`.
- The static artifact `http://costa-desktop:18888/bitwire_inbound_comparison.json`
  returned the same counts, proving the zeros are in the generated artifact,
  not introduced by the React table.
- Of the 33 zero rows, 24 have peer `ips == 0`; 9 have peer `ips > 0`.
- `pkg/web/server.go` serves `/api/v1/sets/{name}/compare` directly from
  `{name}_comparison.json`; it does not recompute or filter this route.
- `pkg/engine/output.go` appends `CompareRow{common: 0}` when pairwise
  comparison is skipped because one side is empty, the min/max ranges cannot
  overlap, or the prefix bitmap says no overlap is possible.
- `ui/src/components/feed-detail/section-comparison.tsx` maps every comparison
  row into `displayRows` and passes all rows to `DataTable`; the comment says
  "no filtering".

### Current facts

- The zero-overlap rows are deliberate output from the backend writer today.
- For `bitwire_inbound`, most zero rows are empty peer feeds; examples include
  `anonymous`, `blueliv_crimeserver_last_1d`, `botvrij_src`, and `satellite`.
- The non-empty zero rows are feeds that the backend concluded cannot overlap
  with `bitwire_inbound` without doing the expensive full overlap count.
  Examples include `iblocklist_org_activision`, `iblocklist_org_ncsoft`, and
  `misp_openai_gptbot`.
- The artifact writer emits those zero rows so both sides can have a current
  pairwise fact after incremental comparison updates. That is freshness-friendly
  but noisy for an "overlap" list.
- Country attribution does not show the same behavior. Code evidence:
  `pkg/engine/geo_provider_cache.go` skips countries with `count == 0` before
  returning `CountryValue` rows. Live evidence for `bitwire_inbound`: all five
  country providers returned `zero_rows: 0`. Local artifact census:
  2,970 country provider files, 131,413 country rows, 0 zero rows.
- ASN attribution does not show the same behavior. Code evidence:
  `pkg/engine/asn.go` keeps unknown ASN 0 in `unknown_ips`, skips ASN 0 from
  `by_asn`, and infrastructure rows explicitly skip `count == 0`. Live evidence
  for `bitwire_inbound`: all four ASN providers returned `zero_rows: 0`. Local
  artifact census: 1,556 ASN provider files, 1,851,420 ASN rows, 0 zero rows.
- Pairwise feed comparison is where the noise is widespread. Local artifact
  census: 646 `_comparison.json` files, 166,822 rows, 99,824 rows with
  `common == 0`, and 386 files containing at least one zero-overlap row.

### Working conclusion

The current pairwise feed comparison behavior is internally consistent but
product-hostile: the section is titled "Overlap" and "Where else these IPs
appear", so rows with `common: 0` do not belong in the default overlap list.
The problem does not appear in country or ASN attribution rows.

The earlier "keep zero rows for audit" argument is weak. The rows do not carry
run timestamp, skip reason, source artifact version, or recomputation evidence;
they only say `common: 0`. That is not a useful public audit record.

If the system needs operational evidence, the right place is run telemetry,
admin-only counters, and logs such as `metadata.comparison_pair_skipped_empty`,
`metadata.comparison_pair_skipped_range`, and
`metadata.comparison_pair_skipped_prefix`, not public comparison rows.

## Implications and decisions

### Decision 1: Where to remove zero-overlap rows from the public list

Evidence:

- Backend comparison artifacts currently contain zero rows from
  `pkg/engine/output.go`.
- The public UI intentionally passes every row to the table in
  `ui/src/components/feed-detail/section-comparison.tsx`.
- Methodology says "A feed with zero IPs overlaps everything by zero", but it
  does not require showing zero rows in the default overlap table.

Options:

A. Filter `common == 0` rows in the UI default table.
- Pros: smallest change; preserves raw artifact/audit data; fixes visible
  confusion immediately.
- Cons: export from the default UI would omit zero rows unless a separate
  toggle is added.
- Implications: API stays unchanged; artifact freshness logic stays unchanged.
- Risks: users wanting a complete pairwise matrix from the UI lose visibility
  unless we add an "include zero" control.

B. Stop writing zero rows into comparison artifacts.
- Pros: artifact name and payload match "overlaps"; smaller JSON files.
- Cons: changes backend contract; incremental merge logic must also remove old
  zero rows or stale zeros stay forever.
- Implications: any consumer expecting a complete pairwise record loses
  explicit "known no overlap" facts.
- Risks: higher regression risk in comparison freshness/merge code.

C. Keep zeros in artifacts and add a UI toggle: default hides `common == 0`,
   optional "Show zero-overlap rows" reveals them.
- Pros: best product behavior and keeps complete audit data available.
- Cons: slightly more UI work and tests.
- Implications: default page means actual overlaps; raw/all mode remains
  complete.
- Risks: needs careful wording so "overlap" and "all comparisons" are not
  confused.

Recommendation: B, with a cleanup guard. Public comparison artifacts should
contain actual overlaps only (`common > 0`). The writer should not emit zero
rows, and incremental merge logic must remove any existing stale zero rows so
old artifacts do not keep the noise forever. Operational skip counts stay in
telemetry/admin evidence, not public JSON.

User decision:

- 2026-04-27: Costa approved the minimal public artifact principle:
  public data should be absolutely necessary for the work, with no excess data;
  valueless facts are waste and should not be present.
- Implementation decision: use option B. Public comparison artifacts contain
  only `common > 0` rows; zero overlap is represented by row absence.

## Plan

1. Change backend comparison merge semantics so public artifacts keep only
   `common > 0` rows.
2. Keep fresh zero comparison results internally long enough to delete stale
   positive rows during incremental artifact merge.
3. Add integrity validation so old public comparison artifacts containing
   `common: 0` are flagged malformed and normal repair/reprocess regenerates
   them.
4. Update UI/API comments and public methodology text so the visible contract
   says "non-zero overlaps", not "all pairwise comparisons".
5. Update specs and project skills with the minimal public artifact rule.
6. Validate with focused Go tests, UI build/lint if touched, full backend gates,
   install, reprocess, and live artifact smoke.

## Execution log

- Investigation: confirmed the noise is in pairwise comparison artifacts, not
  country or ASN attribution artifacts.
- Code: changed comparison row merge semantics to drop existing zero rows and
  use fresh zero rows as deletion signals for stale peer rows.
- Code: added semantic integrity validation for comparison artifacts containing
  explicit zero-overlap rows.
- Tests: added unit/integrity coverage for zero-row deletion and stale artifact
  detection.
- Specs/skills: updated operating, processing, website, integrity, coding,
  testing, and reviewing memory with the minimal public artifact rule.
- Install/reprocess: rebuilt and restarted the development service, then ran a
  full reprocess so the served comparison corpus was regenerated/sanitized.

## Validation

- Runtime API evidence collected from the live development service.
- Code evidence collected from `pkg/web/server.go`, `pkg/engine/output.go`, and
  `ui/src/components/feed-detail/section-comparison.tsx`.
- Focused tests passed:
  - `go test ./pkg/engine -run 'TestMergeCompareRowsDropsAndDeletesZeroOverlapRows|TestValidateComparisonPayloadRejectsZeroOverlapRows|TestWriteComparisonFilesRemovesStaleZeroOverlapRows|TestSanitizeComparisonArtifactsStagesUntouchedLiveZeroRows|TestCheckIntegrityFlagsZeroOverlapComparisonRows' -count=1`
- Full validation passed:
  - `make test`
  - `make lint`
  - `make build`
  - `make race`
  - `pnpm --dir ui lint`
  - `pnpm --dir ui build`
  - `git diff --check`
  - `./install.sh`
- Live service validation:
  - `systemctl is-active update-ipsets` returned `active`
  - `/healthz` returned `ok`
  - `/api/v1/status` returned `source_count: 397`, `merge_count: 15`,
    `running: false`, `last_error: null`
  - `/api/v1/admin/integrity` returned `status: clean`, `count: 0`
  - `/api/v1/sets/bitwire_inbound/compare` returned 352 rows and
    `zero_common: 0`
  - local scan of `/opt/update-ipsets/web/*_comparison.json` found 646 files,
    66,998 rows, `zeros: 0`, `filesWithZero: 0`, `malformed: 0`
- Reviewer evidence:
  - N/A: no external/subagent reviewer was run because Costa did not ask for
    agents on this follow-up, and local tests plus live corpus validation cover
    the changed contract directly.

## Outcome

Public feed comparison artifacts now encode "no overlap" as absence, not as an
explicit row with `common: 0`.

The writer still uses fresh zero results internally as deletion signals during
incremental merge, so if a pair used to overlap and now does not, the stale
positive row is removed. Every heavy comparison run also sanitizes existing
served comparison artifacts so untouched old files do not keep historical
zero-row noise.

## Followup

- Validation found comparison files in `/opt/update-ipsets/web` for names whose
  current `/api/v1/sets/{name}` route returns 404, for example `anonymous` and
  `cleantalk_new_1d`. This SOW removed valueless zero rows from those files but
  did not delete stale/orphan public artifacts. A separate cleanup decision is
  needed because deletion policy and compatibility impact are broader than the
  zero-overlap-row bug.

## Lessons extracted

- Public data minimality is a product contract, not a micro-optimization. If a
  public row has no user-facing value, it should not exist.
- Integrity should catch stale public artifacts that violate the current
  minimality contract, not only malformed JSON.
- Specs and project skills now require public artifacts to avoid valueless
  explicit empty facts; comparison artifacts specifically require `common > 0`.
