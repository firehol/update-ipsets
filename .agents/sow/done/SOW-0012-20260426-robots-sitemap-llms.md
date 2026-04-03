# SOW-0012 | 2026-04-26 | robots-sitemap-llms

## Status

completed
regression correction completed: robots discourages live/query crawling, and sitemap detail coverage includes public feeds, countries, ASNs, and maintainers

## Requirements

Given the public site should be discoverable and machine-readable, when this SOW is complete, then it must serve appropriate `robots.txt`, `sitemap.xml`, and `llms.txt` files.

Given generated pages and dynamic routes may be numerous, when the sitemap is built, then route inclusion rules must be explicit and efficient.

Given `llms.txt` is intended for AI agents, when it is added, then it must point to the canonical public docs, methodology, APIs, and feed surfaces without exposing admin or private paths.

Verification methods:

- HTTP tests must fetch `/robots.txt`, `/sitemap.xml`, and `/llms.txt` from the public handler.
- On-disk publication tests must verify `web/robots.txt`, `web/sitemap.xml`, and `web/llms.txt` are generated.
- Content checks must verify the public metadata files include public routes and do not mention `/admin` or `/api/v1/admin`.

## Analysis

Initial sources to consult:

- Public route map in `ui/src` and `pkg/web`.
- Existing static file serving.
- Public docs and methodology pages.
- Current generated/static assets.

Current known context:

- No release-specific robots/sitemap/llms work is tracked outside this SOW.
- `pkg/engine/output.go` already writes `sitemap.xml` and `robots.txt`, but it does not write `llms.txt`.
- The current sitemap uses `runtime.web_url` as its root. The default `runtime.web_url` is `https://iplists.firehol.org/ipsets/`, which is the feed-detail prefix, not the public site root.
- `pkg/web/server.go` explicitly serves `/all-ipsets.json`, `/sitemap.xml`, and `/robots.txt` from the published web output directory; `/llms.txt` is missing from that explicit route list.
- `pkg/web/server.go` treats root-level `.txt` files as published web artifacts in the SPA fallback, so `llms.txt` would be servable after generation, but adding it to the explicit list keeps the public metadata route table searchable and intentional.
- `pkg/web/server.go` excludes `index.json`, `all-ipsets.json`, `sitemap.xml`, and `robots.txt` from feed-scoped public-artifact checks; `llms.txt` must be added there so a future feed named `llms` cannot affect the root metadata file behavior.

External references consulted:

- Sitemaps.org says sitemap URLs must be absolute, UTF-8, and XML-escaped; `lastmod`, `changefreq`, and `priority` are optional.
- Google Search Central documents `Sitemap:` in `robots.txt` as an absolute URL directive supported by major search engines.
- llmstxt.org describes `/llms.txt` as a Markdown proposal for curated LLM-readable site context, not as a formal standards-body requirement.

## Implications and decisions

- `robots.txt` must not accidentally expose admin or private endpoints; it will allow public crawling and point only to the public sitemap.
- `sitemap.xml` route inclusion rule: include the public homepage, public index routes, the methodology index, and one feed-detail URL per public output feed. Do not enumerate admin routes, API routes, raw files, country details, ASN details, or maintainer details in this SOW.
- The public site base URL is `runtime.public_base_url` when configured. If it is empty, derive the site root from `runtime.web_url` by removing the `/ipsets` feed-detail prefix when present.
- `llms.txt` will be concise Markdown and will point to public pages, public APIs, methodology, and feed surfaces only.
- `llms.txt` must not claim that AI providers universally use the file. The evidence supports treating it as an emerging proposal/convention.

## Plan

Single-unit implementation, no chunking - reasoning: the change is atomic and confined to the published metadata generator, public metadata serving allowlist, tests, specs, and this SOW record.

## Execution log

2026-04-26T18:27:26+03:00 - analysis and plan

- Read SOW workflow/file-format rules and project coding/testing skills.
- Inspected `pkg/engine/output.go`, `pkg/engine/metadata.go`, `pkg/web/server.go`, public route definitions in `ui/src/App.tsx`, and existing web/engine tests.
- Verified external metadata-file conventions from Sitemaps.org, Google Search Central, and llmstxt.org.

2026-04-26T18:33:25+03:00 - single-unit implementation

- Updated `pkg/engine/output.go` to generate `sitemap.xml`, `robots.txt`, and `llms.txt` together.
- Fixed sitemap root derivation so default `runtime.web_url: https://iplists.firehol.org/ipsets/` produces the public site root `https://iplists.firehol.org`.
- Added `llms.txt` to generated-file tracking in `pkg/engine/metadata.go`.
- Added explicit `/llms.txt` serving and root artifact classification in `pkg/web/server.go`.
- Added engine helper tests and public handler tests in `pkg/engine/output_test.go` and `pkg/web/server_test.go`.
- Updated `README.md`, `.agents/sow/specs/website.md`, and `.agents/sow/specs/files-layout.md`.

Review notes:

- In-session code review checked the generated route set, URL normalization, public/private boundary, and artifact allowlists.
- No must-fix review findings remained after changing `llms.txt` example links so search/compose examples use fetchable query forms.

2026-04-26T18:37:00+03:00 - install and smoke validation

- Ran `./install.sh`.
- The install built the UI, refreshed embedded static assets, built the Go binary, installed `/opt/update-ipsets/bin/update-ipsets`, reloaded systemd, and restarted `update-ipsets`.
- Smoke checks passed for `http://localhost:18888/healthz`, `http://localhost:18888/api/v1/status`, `http://localhost:18888/api/v1/sets`, `http://localhost:18888/robots.txt`, `http://localhost:18888/sitemap.xml`, and `http://localhost:18888/llms.txt`.

## Validation

- [x] Acceptance criteria evidence
  - `go test ./pkg/engine ./pkg/web` passed.
  - `make test` passed.
  - `make build` passed.
  - `pkg/web/server_test.go` verifies `/robots.txt`, `/sitemap.xml`, and `/llms.txt` return `200`.
  - `pkg/web/server_test.go` verifies `web/robots.txt`, `web/sitemap.xml`, and `web/llms.txt` are generated on disk.
  - `pkg/web/server_test.go` verifies sitemap/robots/llms content does not include `/admin` or `/api/v1/admin`.
- [x] Real-use validation evidence
  - `go test ./pkg/web -run TestAPIEndpointsAndCORS -count=1` passed, exercising a scheduler-style feed run, generated web artifacts, and public HTTP handler reads for the affected routes.
  - `./install.sh` completed successfully and restarted the local `update-ipsets` service.
  - Installed-service smoke checks confirmed `/healthz`, `/api/v1/status`, `/api/v1/sets`, `/robots.txt`, `/sitemap.xml`, and `/llms.txt` respond on `http://localhost:18888`.
- [x] Cross-model reviewer findings (logged + addressed)
  - Low-risk SOW reviewed in-session. Finding addressed: the first `llms.txt` draft linked bare query-required API endpoints; it was changed to use concrete fetchable examples for IP search and compose.
  - External/cross-model assistants were not run because Costa did not ask to run them for this SOW.
- [x] Lessons extracted (or "none, reasoning: ...")
  - Lessons recorded below and reflected in `.agents/sow/specs/website.md` and `.agents/sow/specs/files-layout.md`.
- [x] Same-failure-at-other-scales check
  - `rg` confirmed the root public metadata allowlists now include `llms.txt` in both generated-file tracking and public serving.
  - `rg` confirmed the retired `writeSitemapAndRobots` helper no longer exists.
  - Existing hidden-feed sitemap test remains in `pkg/web/admin_unification_test.go`.

Additional validation note:

- `make lint` was attempted but failed on pre-existing unrelated `go vet` findings:
  - `pkg/cache/cache.go:91:70: call to time.Since is not deferred`
  - `pkg/config/config.go:535:71: call to time.Since is not deferred`
  - `pkg/engine/background_tasks.go:200:57: call to time.Since is not deferred`

## Outcome

SOW-0012 shipped in the working tree and was installed locally. The product now publishes and serves
`robots.txt`, `sitemap.xml`, and `llms.txt`; the sitemap uses the public site
root rather than the feed-detail prefix; `llms.txt` points only to public pages,
public APIs, methodology, and feed catalog surfaces; docs/specs define the
public metadata contract.

## Lessons extracted

- Public metadata files are part of the website contract, not incidental
  generated files. The route inclusion and privacy rules were added to
  `.agents/sow/specs/website.md`.
- `web/llms.txt` belongs to the published artifact layout alongside
  `web/sitemap.xml` and `web/robots.txt`. The filesystem contract was updated
  in `.agents/sow/specs/files-layout.md`.

## Regression

Detected on 2026-04-26 after Costa asked whether robots.txt prevents live
queries and whether sitemap.xml shows all feeds, maintainer pages, country
pages, and ASN pages.

Evidence:

- Installed `robots.txt` allows all paths and only points to the sitemap.
- Installed `sitemap.xml` contains all public feed pages but only index pages
  for countries, ASNs, and maintainers.
- The completed SOW explicitly recorded the wrong route inclusion rule:
  do not enumerate country detail, ASN detail, or maintainer detail pages.

Costa's decision:

- Reopen SOW-0012 as a regression.
- Fix `robots.txt` so crawler policy discourages live/query endpoint crawling.
- Replace the single sitemap with a sitemap index and shard files so the
  sitemap can include public feed pages, country detail pages, ASN detail
  pages, maintainer detail pages, and public index pages without approaching
  sitemap URL-count limits.

Regression fix scope:

- Update generator code and public serving allowlists.
- Update tests to verify robots disallows live/query endpoints and the sitemap
  index/shards include the expected public detail pages.
- Update website/files-layout specs and SOW validation.
- Install, smoke test, and commit the correction.

Regression implementation:

- Updated `robots.txt` generation to add `Disallow` rules for live/query
  endpoints: `/api/v1/search`, `/api/v1/query`, `/api/v1/compose`,
  `/api/v1/client-ip`, `/api/v1/sets/*/search`, and
  `/api/v1/ipsets/*/search`. These are crawler hints only, not security or
  rate limiting.
- Replaced the single sitemap URL set with a sitemap index plus route shards:
  `sitemap-pages.xml`, `sitemap-feeds.xml`, `sitemap-countries.xml`,
  `sitemap-maintainers.xml`, and `sitemap-asns-*.xml`.
- ASN shards use a 45,000-URL maximum so the site stays below the official
  50,000 URL-per-sitemap limit.
- Fixed the sitemap entity source of truth: country/ASN sitemap URLs now prefer
  the same published or staged entity index files that `/api/v1/countries` and
  `/api/v1/asns` serve, falling back to direct live aggregation only when those
  indexes are unavailable.
- Updated entity-index rebuild/repair paths to stage sitemap refreshes together
  with regenerated country/ASN indexes.
- Added stale root `sitemap-*.xml` shard cleanup for direct writes and staged
  delete marking for entity-index publishes.

Regression validation:

- `go test ./pkg/engine ./pkg/web` passed.
- `go test ./...` passed.
- `make build` passed.
- `make lint` still fails on pre-existing `go vet` findings unrelated to this
  SOW:
  - `pkg/config/config.go:535:71: call to time.Since is not deferred`
  - `pkg/cache/cache.go:91:70: call to time.Since is not deferred`
  - `pkg/engine/background_tasks.go:200:57: call to time.Since is not deferred`
- `./install.sh` passed and restarted `update-ipsets.service`.
- Installed smoke checks on `http://localhost:18888` confirmed:
  - `/robots.txt` contains the live/query `Disallow` rules and points to the
    public sitemap index.
  - `/sitemap.xml` is a sitemap index with feed, country, maintainer, and two
    ASN shard entries.
  - `/sitemap-feeds.xml` has 331 URLs, matching `/api/v1/sets`.
  - `/sitemap-countries.xml` has 243 URLs, matching `/api/v1/countries`.
  - `/sitemap-maintainers.xml` has 58 URLs, matching `/api/v1/maintainers`.
  - Final post-commit install smoke showed `/sitemap-asns-0001.xml` with
    45,000 URLs and `/sitemap-asns-0002.xml` with 3,171 URLs, totaling 48,171
    and matching `/api/v1/asns`.

Regression lessons:

- Public sitemap entity detail coverage must be generated from the public
  entity indexes, because those indexes are the public API source of truth.
- Entity-index rebuilds must also refresh sitemap shards; otherwise the API and
  sitemap can drift even when both are individually valid.

Project skill update:

- `.agents/skills/project-coding/SKILL.md` now records that public sitemap
  entity URLs must come from the same published/staged entity index artifacts
  used by the public API.
