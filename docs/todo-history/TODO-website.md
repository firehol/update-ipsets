# Website — Presentation Redesign (Facts-Only, ASN Intelligence, Luxury Product Pages)

> **Supersedes** the previous Alpine/Tailwind/D3 redesign TODO. The framework work from that TODO is already done (Alpine + D3 + embedded SPA + tabs + charts + catalog + admin + Disqus). This TODO is the **next chapter**: cleaning up illusions, adding ASN intelligence, and elevating the presentation to a premium product-page experience — **while being strictly factual**.

## TL;DR

- **Philosophy shift**: present facts, never editorial labels. "Last source update 14 days ago, expected every 30 minutes" not "Stale". Users are smart; we help them connect dots, not conclude for them.
- **Delete phantom fields** (License/Grade/Protection/IntendedUse/FalsePositives/Poisoning/Services) — 202 of 204 sources have no values for them. Remove from Go, JSON, UI, tests.
- **Add ASN intelligence**: compute IP→ASN at ingestion time, multi-provider (MaxMind GeoLite2-ASN first, ip2asn + db-ip later), tabbed UI like geo maps. Identify "infrastructure ASNs" (Cloudflare, Google, Microsoft, Apple, GitHub, AWS, etc.) from a YAML whitelist and count per-feed overlap as a bare fact.
- **Methodology pages**: every computed number gets a Markdown page documenting the exact formula, inputs, source code reference, worked example.
- **Luxury product-page redesign**: each feed becomes a premium "product page" — hero with cinematic evolution chart, vitals strip, composition section, behavior grid, comparison, tech specs table, provenance, description, discussion. Apple/Linear/Vercel visual language.
- **Homepage adds per-category tables** ranking feeds by infrastructure overlap (and other factual metrics).

## Relevant memory (must-read for any agent touching this)

- [Show facts, not labels](.claude/projects/-home-costa-src-firehol-update-ipsets/memory/feedback_facts_not_labels.md)
- [Methodology transparency](.claude/projects/-home-costa-src-firehol-update-ipsets/memory/feedback_methodology_transparency.md)
- [No phantom schema fields](.claude/projects/-home-costa-src-firehol-update-ipsets/memory/feedback_no_phantom_fields.md)

---

## Current state analysis (facts from the code)

### What already exists (don't re-do)
- Alpine.js + D3 SPA, embedded in Go via `//go:embed` in `pkg/web/static/`
- `pkg/web/server.go` serves `/ipsets/{name}` SPA routes, `/api/v1/*` JSON endpoints
- Charts rendered in `pkg/web/static/app.js`: evolution, retention, freshness, geo choropleth, overlap table, sankey, network graph
- Geo provider tabs already implemented with `activeGeoProvider` state (keys: `geolite2`, `ipdeny`, `ip2location`) — `app.js:40-42,154,1130`
- Admin page with feeds table, scheduler, system info
- Disqus integration with preserved thread IDs
- Dark-first theme with recently reworked navy palette
- Homepage globe (globe.gl) in hero
- 204 feeds across 9 categories; `all-ipsets.json` / `<name>.json` / `<name>_history.csv` / `<name>_comparison.json` / `<name>_{provider}_country.json` / `<name>_retention.json` are all produced by the engine

### Phantom fields — actual population status (measured, not assumed)
Fields in `cache.Entry` populated in `pkg/engine/finalize.go:99-110` from `src.Attributes`, defaulting to `"unknown"`:

| Field | Sources that set it | Status |
|---|---|---|
| `License` | 1 source (`ip2location`) | Illusion for 203/204 |
| `Grade` | 0 sources | Illusion for 204/204 |
| `Protection` | 1 source (`darklist_de`) | Illusion for 203/204 |
| `IntendedUse` | 1 source (`darklist_de`) | Illusion for 203/204 |
| `FalsePositives` | 0 sources | Illusion for 204/204 |
| `Poisoning` | 0 sources | Illusion for 204/204 |
| `Services` | 1 source (`darklist_de`) | Illusion for 203/204 |

Verdict: delete. Re-add later if/when Costa decides to curate these for top feeds.

### What the presentation currently lacks
- Tech specs table (we have 47+ facts per feed but no clean single-view spec sheet)
- ASN breakdown (geo yes, ASN no)
- Infrastructure ASN detection (doesn't exist)
- Methodology pages (nothing documents how numbers are computed)
- Chart headlines computed from data (charts have generic text, not 1-line factual summaries)
- "From the maintainer" labeling on the description (our voice mixes with theirs)
- Per-category homepage tables (only flat catalog exists)
- Premium typographic hierarchy (current type scale is functional, not editorial)
- Scroll motion (no reveal-on-scroll, no number ticking, no chart entrance animation)
- Cinematic hero visual per feed (hero is text-only)

---

## Decisions locked by Costa

| # | Decision | Choice |
|---|----------|--------|
| 1 | Delete phantom fields entirely | **Yes** — remove from `cache.Entry`, `setMetadata`, `adminFeedDetail`, legacy loader, tests, frontend, JSON output |
| 2 | ASN database — first provider | **MaxMind GeoLite2-ASN** (we already have credentials from geo work) |
| 3 | ASN providers — multi-provider from day 1 | **Yes** — config supports a list; UI shows one tab per provider (same pattern as geo). Planned additions: ip2asn (Team Cymru), db-ip, more later. Cross-provider disagreement is itself a truth signal, shown honestly. |
| 4 | Infrastructure ASN whitelist location | **Main YAML config** alongside sources, merges, geolocation. Versioned with the repo, PR-able. |
| 5 | Methodology pages format | **Markdown files**, rendered to HTML (server-side via `goldmark`) |
| 6 | Infrastructure overlap visibility | **Per-feed page + global feed list + homepage tables grouped per category** (attacks, abuse, malware, anonymizers, reputation, organizations, unroutable, spam, geolocation) |
| 7 | ASN processing timing | **At ingestion time**, produces `<name>_asn_<provider>.json` per feed (same pattern as `<name>_<provider>_country.json`) |
| 8 | Replace "Trust & transparency" section | **With a "Provenance" section** (source URL, processors, downloader, first seen, commit history). Facts only. |
| 9 | Presentation aesthetic | **Luxury e-commerce product page** — Apple/Linear/Vercel reference. Hero → vitals → composition → behavior → comparison → tech specs → provenance → description → discussion. |
| 10 | Editorial tagging of feeds | **Forbidden**. No "narrow/broad", no "production-ready", no "best for…", no maintainer grades. Only raw facts. |
| 11 | Methodology for every computed number | **Required**. Each metric on the page links to its methodology page. |
| P1 | Hero visual on feed detail pages | **All-time evolution area chart** (cinematic, unique signature per feed, doesn't dilute the homepage globe identity) |
| P2 | Young feeds with insufficient data | **Show sections with "Full analysis available after N days of tracking" indicators** alongside whatever partial data exists. Honest, premium, never looks broken. |
| P3 | Category color palette strength | **Each category gets a distinct muted jewel tone** — attacks=red, abuse=amber, malware=purple, anonymizers=cyan, reputation=emerald, organizations=slate, spam=rose, unroutable=stone, geolocation=teal. Validate against color-blind accessibility before launch. |
| P4 | Download CTA prominence | **Big primary button in the hero** — "Download list" for redistributable feeds, "View metadata" for non-redistributable |
| P5 | Share buttons | **None** — URL copy is enough. No widgets, no tracking. |
| P6 | Hero background treatment | **Static dark gradient** — quiet, lets data carry the visual weight |
| P7 | PDF dossier export | **Yes, phase 4** — once HTML is clean, mostly CSS work. Real value for compliance/audit users. |
| P8 | Schema cleanup depth | **Delete entirely** — phantom fields gone from `cache.Entry`, `setMetadata`, `adminFeedDetail`, legacy loader, tests, frontend, JSON output |

---

## Implied decisions (proceeding unless Costa vetoes)

- **Methodology page location**: `pkg/web/static/methodology/*.md` embedded, rendered server-side via `goldmark` (tiny Go dep, CommonMark-compliant). Served at `/methodology/<slug>`.
- **ASN config schema**: new top-level `asn:` map in YAML, each entry with `name`, `type` (e.g. `maxmind_geolite2_asn_mmdb`), `url`, `frequency`. Mirrors the existing `geolocation:` config shape.
- **Infrastructure ASN config schema**: new top-level `infrastructure_asns:` list in YAML, each with `asn`, `name`, `description`, `category` (cdn / hyperscaler / hosting / dev-platform), `added` date.
- **Per-feed ASN output file**: `<name>_asn_<provider>.json` with shape `[{asn: int, name: string, count: int, percent: float, is_infrastructure: bool}]` sorted desc by count.
- **Per-feed infrastructure summary**: computed and embedded in the feed's main `<name>.json` as an `infrastructure` object `{total_infra_ips, total_infra_asns, by_asn: [{asn, name, count}]}` — so the detail page can show the callout without another fetch.
- **Homepage category tables**: new API endpoint `/api/v1/categories/<category>/ranked?by=infra_overlap` (and other sort keys), or compute client-side from `all-ipsets.json` + per-category precomputed stats file `categories.json`.
- **MaxMind MMDB reader**: `github.com/oschwald/maxminddb-golang` — well-maintained, small, official.
- **Markdown renderer**: `github.com/yuin/goldmark` — CommonMark-compliant, small, standard.
- **Chart headline computation**: done client-side in `app.js` from the data already fetched; no new API needed.
- **Tech specs table**: pure HTML/CSS, reads from existing `<name>.json` metadata — no new backend work.
- **Delete phantom fields**: remove from `pkg/cache/cache.go` Entry, `pkg/cache/legacy.go` loader, `pkg/engine/finalize.go` populator, `pkg/engine/output.go` setMetadata, `pkg/web/admin.go` adminFeedDetail, `pkg/cache/cache_test.go` tests, and any frontend display in `pkg/web/static/*`.
- **Motion budget**: use IntersectionObserver for scroll reveals, RAF for number ticking, D3's built-in transitions for chart entry. No animation library.
- **Typography**: add Inter Display for headlines (CDN or self-host via `/static/fonts/`), keep Inter for body, add JetBrains Mono for technical data. `font-variant-numeric: tabular-nums` globally on numeric contexts.

---

## Plan

### Phase 1 — Truth cleanup (the foundation)

**Goal**: remove illusions, establish facts-only discipline, prepare the code for the new sections.

1. **Delete phantom fields**
   - `pkg/cache/cache.go`: remove `License`, `Grade`, `Protection`, `IntendedUse`, `FalsePositives`, `Poisoning`, `Services` from `Entry` struct
   - `pkg/cache/legacy.go:269-281`: remove loader code
   - `pkg/engine/finalize.go:99-110`: remove populator code
   - `pkg/engine/output.go` `setMetadata`: remove fields 52-58
   - `pkg/web/admin.go` `adminFeedDetail`: remove fields 85-90
   - `pkg/cache/cache_test.go:384-390`: update test
   - `pkg/web/static/index.html` + `app.js`: remove any display
   - Verify: `go test ./...` passes, frontend renders without the fields

2. **Delete existing source attributes that fed those fields** in `configs/firehol.yaml` (darklist_de, ip2location) — they're no longer read by anything

3. **Add "From the maintainer" label** to the description section (clear separation of our voice from theirs) — already done for collapse, just add the label

4. **Compute and display chart headlines** in `app.js`:
   - Evolution: "`{current}` IPs today. `{years}`-year range: `{min}` — `{max}`."
   - Freshness: "Oldest entry added `{days}` days ago. Median age: `{median}`."
   - Retention: "`{pct}`% of IPs are removed within `{hours}` hours of being added."
   - Geo: "Top concentration: `{c1} {p1}%`, `{c2} {p2}%`, `{c3} {p3}%`."
   - All computed from already-fetched data. Each has a methodology link placeholder.

5. **Methodology pages framework**
   - Add `goldmark` dep
   - `pkg/web/static/methodology/*.md` (embedded)
   - Server route `/methodology/<slug>` renders Markdown → HTML with site shell
   - Add initial 6 pages: update freshness, update cadence, reliability, clock skew, retention half-life, pairwise overlap
   - Each page: plain-English definition, formula, data source, worked example, code reference (`file:line`)

### Phase 2 — ASN intelligence (the highest-impact addition)

**Goal**: make the truth about "which feeds contain infrastructure IPs" visible everywhere.

1. **Add ASN config schema** in `pkg/config/config.go`:
   - `ASNProviders map[string]*ASNProvider` with `Name`, `Type`, `URL`, `Frequency`
   - Add to YAML parser + default config

2. **Add infrastructure ASN config schema**:
   - `InfrastructureASNs []InfrastructureASN` with `ASN`, `Name`, `Description`, `Category`, `Added`
   - Seed with Cloudflare, Google, Microsoft (Azure + corp), Apple, GitHub, AWS, Akamai, Fastly, Meta, Linode, DigitalOcean, OVH, Hetzner, CloudFront, Vercel, Netlify, Heroku — ~20-50 entries

3. **Add ASN database download + ingestion**:
   - New `pkg/asn/` package
   - MMDB reader via `oschwald/maxminddb-golang`
   - Downloaded to cache dir on its own schedule
   - Load into memory once per run

4. **Compute ASN breakdown per feed at processing time**:
   - In `pkg/engine/geoloc.go` (or new `pkg/engine/asn.go`), iterate feed IPs, look up ASN per provider
   - Produce `<name>_asn_<provider>.json` → `[{asn, name, count, percent, is_infrastructure}]` sorted desc
   - Update `<name>.json` to include `infrastructure` summary object

5. **Add ASN API endpoint** (mirror geo):
   - `GET /api/v1/sets/<name>/asn/<provider>` returns the ASN array
   - `GET /api/v1/sets/<name>/asn` returns provider list

6. **Frontend: ASN section with tabbed providers**
   - New section in feed detail page, beside or below geo map
   - Tab switcher for providers (uses same `provider-tab` CSS class as geo tabs)
   - Table of top 20 ASNs: number, org name, IP count, percent, infrastructure badge
   - Infrastructure callout panel above the table (bare counts, methodology link)

7. **Homepage per-category tables**
   - New `categories.json` precomputed index
   - Homepage renders each category as a sortable table (size, last update, cadence, infra overlap, uniqueness)
   - Default sort: size desc
   - Click row → feed detail page

8. **Global feed list**: add an "Infrastructure overlap" column (sortable)

9. **Methodology pages**: add "How we identify infrastructure ASNs" (renders the YAML whitelist directly), "How we compute ASN breakdown", "How we handle cross-provider disagreement"

### Phase 3 — Luxury product-page redesign (the visible transformation)

**Goal**: make each feed detail page feel like a premium product page.

1. **Typography system**
   - Load Inter Display + JetBrains Mono (self-hosted in `/static/fonts/` or via CSS font loading API)
   - Redefine scale: hero 96-120px, section heads 48-64px, body 16-18px, stat numbers 48-72px
   - `font-variant-numeric: tabular-nums` globally where numbers appear
   - Line-heights: generous for hero, tight for spec tables

2. **Color system refinement**
   - Deepen dark mode base (`#0a0e17` bg, `#171d2b` surface)
   - Add category accent colors as CSS custom properties — one `--accent` per category, applied to category pill, hero gradient, active-tab underline, chart stroke
   - Light mode companion

3. **Hero section rebuild** (pending P1 decision)
   - Full viewport height on desktop, sensible on mobile
   - Left: category pill → feed name → 1-line factual tagline → maintainer row ("by X · tracking since YYYY · N years") → primary CTA + secondary actions
   - Right: cinematic visual (evolution area chart or globe, per P1)
   - Bottom: live status strip (last update pulse, next check, reliability, clock skew, errors) — thin bar

4. **Vitals strip** — 4-6 large stat cards with sparklines as background texture, numbers tick up on first view, each with methodology link

5. **Composition section** — geo map (left) + ASN tabs (right) + infrastructure callout below

6. **Behavior grid** — existing 4 charts restyled, each with computed headline and methodology link, scroll-triggered entry animations

7. **Comparison section** — horizontal scroll strip of related feeds + overlap table + uniqueness fact

8. **Tech specs table** — new section, exhaustive two-column layout grouped into: Identification / Data / Updates / Access / Processing / Maintainer

9. **Provenance section** — source URL, processor pipeline, downloader, first seen, commit history link — all facts

10. **Description section** — labeled "From the maintainer", the `Info` content (already has collapse)

11. **Download / Access footer** — big card with every consumable artifact (raw list, JSON, history CSV, comparison JSON, API docs)

12. **Scroll motion system**
    - IntersectionObserver-based fade+slide reveals per section
    - Number tick-up on first view (requestAnimationFrame, 600-1000ms)
    - D3 chart entry animations
    - Respects `prefers-reduced-motion`

### Phase 4 — Polish & optional extras

1. **More ASN providers**: add ip2asn (Team Cymru), db-ip — each a new entry in the ASN config
2. **Cross-provider disagreement display**: if providers disagree on an IP's ASN, show both
3. **Maintainer pages** (factual): `/maintainers/<name>` listing all their feeds, our tracking history, inter-feed overlap pattern, processor footprint — no grades, no editorial
4. **Cross-feed infrastructure ranking** on its own page: all feeds ranked by infrastructure overlap %, sortable, filterable by category
5. **Category landing pages**: `/categories/<name>` with full per-category table, category-level stats
6. **PDF dossier export** (pending P7) — feed detail printable to PDF for compliance users
7. **More methodology pages** as additional metrics come online

---

## Testing requirements

- `go test ./...` passes after phantom field deletion
- Existing `pkg/web/feature_test.go` still passes (SPA served from embed)
- New test: `pkg/asn/` package — MMDB parsing, IP lookup, infrastructure overlap computation
- New test: `pkg/engine/asn.go` — ingestion produces correct `<name>_asn_<provider>.json`
- New test: methodology page rendering (Markdown → HTML with correct shell)
- New test: API endpoints for ASN data
- Visual test: each section renders correctly in dark + light themes
- Visual test: chart headlines match expected format for real feeds
- Visual test: scroll reveals work, respect `prefers-reduced-motion`
- Responsive: hero, vitals, composition, tech specs all readable on mobile/tablet/desktop
- Cross-browser: Chrome, Firefox, Safari, Edge
- Sanity: verify `all-ipsets.json`, `<name>.json`, existing geo/retention JSON still produced correctly (nothing regressed)
- Deploy to d1 workflow unchanged

## Documentation updates required

- **Methodology pages** (phase 1 & 2): every computed metric gets a page
- **README.md**: document the new presentation philosophy ("facts only"), ASN ingestion, infrastructure ASN concept, how to contribute to the whitelist
- **CLAUDE.md / AGENTS.md**: add a note about the facts-only design principle for future agents
- **Update `configs/firehol.yaml`**:
  - Remove the 4 source attribute entries that fed phantom fields
  - Add new top-level `asn:` map (initially just MaxMind GeoLite2-ASN)
  - Add new top-level `infrastructure_asns:` list
- **`docs/` (if we have one)**: design rationale, category color palette, typography system

---

## What this TODO does NOT cover (separate tracks)

- UI rewrite to Vue 3 + PrimeVue (separate decision in main TODO.md)
- Admin interface changes (Admin UI is fine as-is; the facts-only rule applies there too if we ever redesign it)
- Backend performance work
- Memory/core-out-of-core work (separate TODO)
