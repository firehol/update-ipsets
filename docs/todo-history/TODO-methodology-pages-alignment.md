## Purpose

Align every methodology page under `pkg/web/static/methodology/` with the current product specs and the live implementation, so operators and public users read explanations that match the actual system behavior and terminology.

## TL;DR

- User asked to update all methodology pages so they reflect both the current specs and the actual implementation.
- Scope is the full methodology set under `pkg/web/static/methodology/`.
- Work must identify stale claims, terminology drift, outdated implementation references, and missing behavior notes introduced by recent spec/implementation changes.

## Analysis

Facts gathered so far:

- Methodology pages currently present:
  - `pkg/web/static/methodology/asn-attribution.md`
  - `pkg/web/static/methodology/bogon-classification.md`
  - `pkg/web/static/methodology/bogon-present.md`
  - `pkg/web/static/methodology/change-rate.md`
  - `pkg/web/static/methodology/churn-high.md`
  - `pkg/web/static/methodology/churn-low.md`
  - `pkg/web/static/methodology/country-concentrated.md`
  - `pkg/web/static/methodology/country-diverse.md`
  - `pkg/web/static/methodology/cross-category-overlap.md`
  - `pkg/web/static/methodology/currently-listed-age-p100.md`
  - `pkg/web/static/methodology/currently-listed-age-p75.md`
  - `pkg/web/static/methodology/data-freshness.md`
  - `pkg/web/static/methodology/evolution.md`
  - `pkg/web/static/methodology/feed-health.md`
  - `pkg/web/static/methodology/geographic-distribution.md`
  - `pkg/web/static/methodology/independent.md`
  - `pkg/web/static/methodology/infrastructure-asns.md`
  - `pkg/web/static/methodology/infrastructure-present.md`
  - `pkg/web/static/methodology/ip-retention.md`
  - `pkg/web/static/methodology/multiple-retention-policies.md`
  - `pkg/web/static/methodology/pairwise-overlap.md`
  - `pkg/web/static/methodology/permanent-bans.md`
  - `pkg/web/static/methodology/removed-age-p75.md`
  - `pkg/web/static/methodology/single-country.md`
  - `pkg/web/static/methodology/size-variation.md`
  - `pkg/web/static/methodology/subset-of.md`
  - `pkg/web/static/methodology/unique-share.md`
  - `pkg/web/static/methodology/update-cadence.md`

- Relevant normative specs for this work:
  - `specs/feeds.md`
  - `specs/downloader.md`
  - `specs/processing-engine.md`
  - `specs/pipeline.md`
  - `specs/homepage.md`
  - `specs/website.md`
  - `specs/admin-ui.md`
  - `specs/integrity.md`

- Already confirmed mismatch:
  - `pkg/web/static/methodology/update-cadence.md` still says cadence is exponential and updated in `pkg/engine/finalize.go`.
  - Live implementation computes cadence from the full history ledger as an arithmetic mean in `pkg/engine/runtime_ledger_cache.go`.

- Additional confirmed mismatches from the audit:
  - multiple methodology pages still reference the retired static frontend implementation (`pkg/web/static/app.js`, `pkg/web/static/index.html`)
  - several pages still describe old UI wording instead of the current React feed-detail sections
  - `pkg/insights/rules_age.go` publishes the methodology slug `/methodology/observation-wall`, but no corresponding Markdown page currently exists under `pkg/web/static/methodology/`
  - several methodology pages talk as if public metrics are derived on the client, while the current product computes and publishes them from backend-owned artifacts or backend classifiers
  - `feed-health.md` was missing `archived` and did not document the archival threshold or the unavailable-to-archived transition
  - geography methodology still documented the old `<feed>_<provider>_country.json` layout, while the current engine writes `<feed>_<provider>.json` with `total_mapped` plus `countries`
  - `unique-share.md` still referenced the retired `historical` state in the independent-peer definition
  - `currently-listed-age-p75.md` and `currently-listed-age-p100.md` did not document the live suppression behavior when the observation-wall rule fires
  - `removed-age-p75.md` did not mention the short-observation-window wording branch in the live rule

Likely risk areas to verify page by page:

- Old terminology from pre-rewrite behavior
- Stale references to code files/functions that no longer own the behavior
- Explanations that no longer match downloader vs engine responsibilities
- Missing mention of canonical feed bodies and ledger-backed calculations
- Health-state explanations missing `archived`
- Pages that describe outputs as if derived from the wrong timestamp or wrong source of truth

## Decisions

- No user decisions currently pending.
- Normative source of truth for behavior is the current `specs/*.md` set plus the implementation where the specs are already explicit.

## Plan

1. Read every methodology page and map it to the relevant spec and code path.
2. Identify all mismatches: stale claims, wrong terminology, wrong ownership, wrong formulas, wrong file references.
3. Add any missing methodology page required by a live metric or insight slug.
4. Update every affected methodology page to match the current specs and live implementation.
5. Run a consistency sweep for terminology across all methodology pages.
6. Verify there are no remaining stale references with targeted searches.

Status:

- Completed the page-by-page rewrite for all audited drift points.
- Added the missing `pkg/web/static/methodology/observation-wall.md`.
- Added a regression test so insight methodology slugs must exist as embedded pages.
- Completed stale-reference and missing-slug verification.

## Implied Decisions

- Methodology pages should explain real behavior in user-facing language, but when they mention implementation details they must point to the current owning subsystem/file.
- Where the implementation intentionally has caveats or fallback behavior, the methodology should state that clearly instead of presenting idealized behavior.
- If a methodology topic is governed by a backend classifier or ledger, the page should say so explicitly and avoid implying the UI derives it independently.
- Missing methodology coverage for a live published insight should be treated as a documentation gap and fixed in the same pass.

## Testing Requirements

- Search-based verification that outdated file/function references are removed or corrected.
- Manual review that each methodology page matches the current spec terminology.
- Spot-check high-risk pages against implementation:
  - `update-cadence.md`
  - `feed-health.md`
  - `data-freshness.md`
  - retention/history-related pages
  - missing `observation-wall` methodology coverage
- Run focused tests for methodology serving and insight docs coverage.

Verification completed:

- `go test ./pkg/web -run TestMethodology`
- `go test ./pkg/insights`
- `go test ./pkg/web`
- repository search confirmed no remaining methodology references to retired `pkg/web/static/app.js`, retired `pkg/web/static/index.html`, the old `_country.json` path, the retired `historical` feed-state wording, or the old exponential-cadence claim

## Documentation Updates Required

- Update all affected files under `pkg/web/static/methodology/`.
- Added `pkg/web/static/methodology/observation-wall.md`.
- Updated `pkg/web/methodology_test.go` to enforce presence of shipped insight methodology pages.
- Update specs only if the audit reveals methodology topics that are not covered clearly enough by the current spec set.
