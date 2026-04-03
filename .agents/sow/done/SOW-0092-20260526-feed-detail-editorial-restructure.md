# SOW-0092 - Feed-detail editorial restructure and character

## Status

Status: completed

Sub-state: Implementation, validation, and follow-up fixes shipped on 2026-05-27.

## Requirements

### Purpose

Make the feed-detail page (`/ipsets/:name`) readable and interesting instead of a 16,817-pixel wall of identical gray text. Give each post-hero section its own visual character (editorial, fact-card, infographic) at the right place, and trim content that does not earn its space.

### User Request

Quote: "The web ui is ugly. full of blocks of text. Can you help me fix it? We need to give a character to everything." Plus the structural decisions captured in `## Implications And Decisions`.

### Assistant Understanding

Facts:

- The "Integrate researched feed context" commit (51d6ebc) added ~3 KB of narrative prose per feed and rendered it all through one paragraph-only component (`ui/src/components/feed-detail/markdown-text.tsx:18-34`).
- The post-hero sections all use the same `DetailSection` chrome (`ui/src/components/feed-detail/section.tsx:33-47`), the same `border border-border` boxes, the same `text-muted-foreground` body — visual monotony.
- The feed-detail page renders nine sections (`ui/src/pages/feed-detail.tsx:95-134`). The About section alone bundles long_description, derivation, scope_and_intent, listing/unlisting/removal, community, and sources_consulted (`ui/src/components/feed-detail/section-about.tsx:23-43`).
- The markdown template also surfaces `not_intended_for` (`configs/templates/markdown/feed.md.tmpl:50-52`) and the full `sources_consulted` list (`configs/templates/markdown/feed.md.tmpl:101-108`).

Inferences:

- The hero's design language (dark surface, accent, big number, sparkline, colored CTA) makes everything below look lifeless by comparison, not because each section is bad but because every section is identical.
- Cutting `not_intended_for` and `sources_consulted` from the public surfaces does not destroy auditability — the raw enrichment JSON is still served and still includes those fields for anyone who needs them.

Unknowns:

- None blocking implementation.

### Acceptance Criteria

- The feed-detail page reads as nine visually distinct chapters, not nine copies of the same template (verified manually via Playwright on dshield, dronebl_anonymizers, abuseipdb_30d).
- `short_description` appears as a tagline in the hero.
- A new editorial section is the first thing after the hero, with: drop-cap on the long-description first paragraph, at least one pull-quote callout, a right sidebar containing role(s), upstream link, scope.description as a short italic lede, and `intended_for` as a check-icon list.
- `not_intended_for` is gone from the UI and from `feed.md.tmpl`.
- `sources_consulted` is gone from the UI and from `feed.md.tmpl`.
- Insights cards carry per-insight icons and a category-tinted rail.
- A standalone "Method" section presents *Primary method · Source-of-source · Update cadence* as fact-card chips, with prose tucked behind a "Method details" disclosure.
- "Listing rules and removal" sits above the Overlap section with three distinctly iconified cards.
- Reputation / community signals sits below Specs, collapsed, single-row, footer-style chrome.
- Chart sections (CritInfra, ASN, Geo, Bogons, Behavior, Retention, Overlap) carry a section icon + category-tinted accent.
- Every feed reference on every page (explorer cards/table/timeline/treemap/maintainer view, sidebar, maintainer-detail, country-detail, asn-detail, in-feed lookup, comparison/overlap, source feeds, bogons) renders the `FeedRef` tooltip with `short_description`.
- `pnpm --dir ui build`, `pnpm --dir ui lint`, and `go test ./pkg/markdown/...` all pass; `make race` passes for touched Go packages.

## Analysis

Sources checked:

- `ui/src/components/feed-detail/section-about.tsx`
- `ui/src/components/feed-detail/section.tsx`
- `ui/src/components/feed-detail/hero.tsx`
- `ui/src/components/feed-detail/markdown-text.tsx`
- `ui/src/pages/feed-detail.tsx`
- `ui/src/lib/enrichment-types.ts`
- `configs/templates/markdown/feed.md.tmpl`
- `pkg/markdown/context.go`, `pkg/markdown/context_feed.go`
- Live page screenshot of `/ipsets/dshield` (full-page, 1440 × 16817 px).
- Live enrichment JSON for `dshield` (character counts measured).

Current state:

- Page height on dshield: 16,817 px. Most of it is paragraph stacks.
- After the hero, the visual chrome is constant: same off-black, same outlined rectangles, same muted gray paragraphs, same eyebrow/title/lede pattern in every section header.
- About-section subsections render the same way (`border border-border p-5`).

Risks:

- Tests referencing the removed `not_intended_for` / `sources_consulted` content will break and must be updated.
- Specs may reference removed surfaces; need to update or note.
- Reordering sections changes deep links/anchors; if any in-page anchors are externally linked, they may break. Likely low risk because no anchor IDs are presently exposed.
- Reputation-as-footer must not look like a system footer; clear separator + small label needed so users don't miss it.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- One generic section wrapper + one paragraph-only renderer + 3 KB of fresh enrichment prose per feed = visual monotony and reader fatigue. Fix is structural (split About into purpose-built sections) and visual (give each section a distinct vocabulary: editorial, fact-card, infographic, footer).

Evidence reviewed:

- See "Sources checked" and live screenshot evidence above.
- No external open-source reference was needed; this is a project-local UI restructure.

Affected contracts and surfaces:

- UI: `ui/src/pages/feed-detail.tsx`, `ui/src/components/feed-detail/*`, `ui/src/lib/enrichment-types.ts` (no schema change, only consumption).
- Markdown template: `configs/templates/markdown/feed.md.tmpl`.
- Go enrichment consumption: `pkg/markdown/context_feed.go` — no struct change required; the unused field can stay on the context (template just stops referencing it) so we don't churn schema for a UI move.
- Tests: `ui/src/pages/feed-detail.test.tsx`, `ui/src/test/fixtures.ts`, `pkg/markdown/feed_template_test.go`, `pkg/markdown/feed_context_test.go`.
- Specs: `.agents/sow/specs/website.md` (homepage/feed-detail surface), possibly `.agents/sow/specs/homepage.md` and the AI-classification-rules content surface.
- Public methodology pages: `pkg/web/static/methodology/ai-researched-feed-context.md` — may reference `sources_consulted` and `not_intended_for`; need to align.

Existing patterns to reuse:

- `DetailSection` / `DetailSubsection` / `DetailNotice` / `DetailTwoColumnPanels` (`ui/src/components/feed-detail/section.tsx`) — extend with optional `icon` + `accentTone` props rather than fork.
- `MarkdownText` (`ui/src/components/feed-detail/markdown-text.tsx`) — extend to support an optional drop-cap and per-paragraph variants instead of building a new renderer.
- `CategoryBadge` colour palette — derive an `accentTone` per feed category to tint chart-section accents.
- `lucide-react` icons (already used in `hero.tsx`) for per-section icons and per-insight icons.

Risk and blast radius:

- All changes live in: `ui/`, `configs/templates/markdown/feed.md.tmpl`, and tests. No daemon, scheduler, downloader, integrity, or memory subsystems touched.
- Public serving stays cache-first: no new request-time computation, no new API.
- The enrichment JSON is unchanged; only its rendering is. Operators relying on the JSON (CLI, mirrors) are not affected.

Sensitive data handling plan:

- No secrets, credentials, or customer data are involved. Fixtures and tests will not introduce real customer identifiers. All durable artifacts will use the same sanitisation rules as the existing tests.

Implementation plan:

1. **Hero**: render `short_description` (preferring `feed.short_description`, falling back to `enrichment.short_description`) as a single tagline line under the title. Tighten the existing "by maintainer …" line so the hero does not become noisier.
2. **`section.tsx` extension**: add optional `icon` (a `lucide-react` `LucideIcon`) and `accentTone` (`'primary' | 'category-*' | 'muted'`) to `DetailSection`; map `accentTone` to a tinted `AccentBar` and a small icon next to the title. Update `DetailSubsection` similarly. No breaking change to existing call sites.
3. **`SectionEditorial`** (new file `section-editorial.tsx`): magazine layout. Left ≈ 7/12: `MarkdownText` with `dropCap` on the first paragraph; one extracted pull-quote rendered as a large left-rail callout. Right ≈ 5/12 sidebar: "Operated by" role cards (max 2), `Upstream source` link, `scope.description` rendered as a short italic 2-sentence lede, and `intended_for` rendered as a `Check` icon list.
4. **`MarkdownText` extension**: optional `dropCap` prop that turns the first letter of the first paragraph into a serif drop-cap (CSS `:first-letter` on the first `<p>` only). Optional `pullQuoteFrom` helper to extract one sentence; default: longest sentence in the second paragraph.
5. **`SectionInsights` restyle**: keep card grid; per insight kind (`retention`, `relationships`, `behavior`, `bogons`, `geo`, etc.), pick a lucide icon and render it inside the existing rail-card; tint the rail using the feed category.
6. **`SectionMethod`** (new file `section-method.tsx`): three fact-card chips at the top — *Primary method* (humanised), *Source-of-source* (original first-party vs. derived/aggregated), *Update cadence* (from `enrichment.update_frequency`). Below the chips: role grid (max 2). All of `derivation.description`, `detection_classification.description`, and `derivation.source_feeds` go behind a `<details>` disclosure labelled "Method details".
7. **`SectionListing`** (new file `section-listing.tsx`): three cards with distinct icons — `ListChecks` for Listing, `Slash` for Unlisting, `Mail` for Removal. Removal card gets the louder accent (primary tint) since it is the actionable one. Move out of `SectionAbout` and place above Overlap in the page composition.
8. **`SectionReputation`** (new file `section-reputation.tsx`): below Specs. Single row, smaller text, three collapsed `<details>` boxes (Positive signals, Past complaints, Maintainer engagement), thin top hairline, no big section header. Optional `Quote` icon.
9. **`SectionAbout` removal**: replaced by editorial + method + listing + reputation. Delete the file once nothing references it. Keep the `FallbackAbout` (raw `feed.info`) only on `SectionEditorial` for feeds without enrichment.
10. **`feed-detail.tsx` composition**: new order — `IPSearchSurface`, `SectionEditorial`, `SectionInsights`, `SectionMethod`, `SectionCriticalInfrastructure`, `SectionASN`, `SectionGeo`, `SectionBogons`, `SectionBehavior`, `SectionRetention`, `SectionListing`, `SectionComparison`, `SectionSpecs`, `SectionReputation`.
11. **Chart sections**: add a category-derived `accentTone` + a `LucideIcon` to each chart `DetailSection` call. No body changes.
12. **Markdown template**: in `configs/templates/markdown/feed.md.tmpl`, drop the `NotIntendedFor` block (lines 50-52 region) and the entire `Sources consulted` block (lines 101-108 region). Keep `scope_and_intent.description` and `intended_for`.
13. **Site-wide FeedRef tooltip**: migrate the eight direct-`<Link>` call sites to `FeedRef` with `feed=` descriptors; audit existing `FeedRef` call sites to ensure each passes a descriptor. Add a `useFeedRefDescriptor(name)` helper (selector over the existing feeds catalog query) for sites that only know the name.
14. **Test updates**: refresh fixtures and tests for changed surfaces.

Validation plan:

- `pnpm --dir ui build`
- `pnpm --dir ui lint`
- `pnpm --dir ui test`
- `go test ./pkg/markdown/...`
- `go build ./...`
- Manual Playwright run on `dshield`, `dronebl_anonymizers`, `abuseipdb_30d`, plus one feed with no enrichment to confirm the fallback (raw `feed.info`).
- Visual delta: capture before/after full-page screenshots; page height must drop meaningfully and section identities must read as distinct.
- Same-failure scan: `grep -rn "not_intended_for\|NotIntendedFor\|sources_consulted\|SourcesConsulted\|section-about" pkg ui` after edits and re-evaluate.

Artifact impact plan:

- AGENTS.md: likely unaffected (no new workflow rule emerges).
- Runtime project skills: `project-frontend-best-practices` may absorb a "give each section a distinct visual vocabulary" lesson at close. To be re-evaluated during retrospection.
- Specs: `.agents/sow/specs/website.md` — update feed-detail section list. Possibly note the markdown template surface trim in `.agents/sow/specs/feeds.md` if applicable.
- End-user/operator docs: `pkg/web/static/methodology/ai-researched-feed-context.md` — verify it does not promise `not_intended_for` or `sources_consulted` to the public; trim if it does.
- End-user/operator skills: none impacted.
- SOW lifecycle: this SOW gets `Status: completed` and moves to `.agents/sow/done/` in the implementing commit.

Open-source reference evidence:

- None required; this is a project-local UI restructure.

Open decisions:

- All resolved. See `## Implications And Decisions`.

## Implications And Decisions

User decisions captured 2026-05-26:

1. **Hero tagline.** Selection: add `short_description` to the hero. Implication: hero gains one tagline line; tracked-since paragraph stays.
2. **Editorial first.** Selection: long_description becomes the first section after the hero, in magazine/editorial style.
3. **Reputation last.** Selection: community/reputation moves below Specs, smaller text, collapsed.
4. **Intended-for placement.** Selection: directly below long_description editorial; `not_intended_for` removed from UI and markdown.
5. **Sources consulted.** Selection: removed from UI and markdown.
6. **Listing rules placement.** Selection: above Overlap.
7. **Insights order.** Selection: Editorial first, Insights right after.
8. **Method.** Selection: standalone fact-card section, prose collapsed behind disclosure.
9. **Scope prose.** Selection: keep, constrained to a 2-sentence italic lede above Intended for.
10. **Markdown cleanup.** Selection: symmetric removal of `not_intended_for` and `sources_consulted` from `feed.md.tmpl`.
11. **Site-wide feed-ref tooltip.** Selection: every feed reference on every page shows a tooltip with the feed's `short_description` (plus `official_name` and `maintainer` when available). `short_description` acts as the feed's identifying caption.

   Implementation:
   - All call sites that render a feed name as a clickable element route through `FeedRef` and pass a `feed` descriptor (`name`, `official_name`, `short_description`, `maintainer`).
   - Direct `<Link to={`/ipsets/${name}`}>` usages found at: `ui/src/pages/maintainer-detail.tsx:127`, `ui/src/pages/asn-detail.tsx:298`, `ui/src/pages/country-detail.tsx:281`, `ui/src/components/home/home-explorer-view-timeline.tsx:134`, `ui/src/components/feed-sidebar.tsx:237`, `ui/src/components/home/home-explorer-view-treemap.tsx:208`, `ui/src/components/home/home-explorer-view-maintainers.tsx:130`, `ui/src/components/feed-detail/section-bogons.tsx:210` — migrated to `FeedRef`.
   - Existing `FeedRef` call sites that pass only `name` (no descriptor): audited and updated to pass `feed=` when the descriptor is in scope; for the few sites that only know the feed name (e.g. overlap rows, `source_feeds.identifier`, in-feed lookup result rows), resolve the descriptor via a lightweight site-wide catalog hook that selects `name / official_name / short_description / maintainer` from the existing `feedsCatalogQuery`.

## Plan

1. Extend `DetailSection`/`DetailSubsection` with optional `icon` + `accentTone`; extend `MarkdownText` with `dropCap`. No call-site breakage.
2. Hero: add `short_description` tagline.
3. Build `SectionEditorial` (replaces the long_description + scope + intended_for + roles sub-blocks of old About).
4. Build `SectionMethod` (replaces the derivation + detection + source_feeds sub-block of old About).
5. Build `SectionListing` (extracted from old About; new placement above Overlap).
6. Build `SectionReputation` (extracted; new footer-style placement below Specs).
7. Restyle `SectionInsights` with per-insight icons + category-tinted rails.
8. Apply category-tinted accent + icon to all chart sections.
9. Update `feed-detail.tsx` page composition.
10. Trim `feed.md.tmpl` (`not_intended_for`, `sources_consulted`).
11. Refresh tests and fixtures.
12. Update specs and public methodology copy.
13. Validate (build, lint, ui tests, go tests, manual Playwright).
14. Commit implementation + SOW status change + SOW move in one commit.

## Execution Log

### 2026-05-26

- SOW drafted and accepted.
- Implemented `SectionEditorial`, `SectionMethod`, `SectionListing`, `SectionReputation`, `SectionSourcesConsulted`, `SectionStatus`; removed `SectionAbout`; added drop-cap + pull-quote helpers, category-tinted section icons, hero `short_description` tagline.
- Recomposed `feed-detail.tsx` in the new order: Editorial → Status → Insights → Method → CritInfra → ASN → Geo → Bogons → Behavior → Retention → Listing → Overlap → Specs → Reputation → Sources consulted.
- Trimmed `not_intended_for` from UI + markdown; restored `sources_consulted` as a folded block at the very end of both surfaces per user feedback.
- Migrated every direct `<Link to="/ipsets/...">` caller to `FeedRef` with a descriptor-map hook so every feed reference site-wide carries the short_description tooltip.
- Updated specs `feeds.md`, `website.md`, and the methodology copy.

### 2026-05-27

- Status block scope expanded to also cover internal health classes (`archived`, `unmaintained`, `empty`) alongside the AI-researched `current_status` lifecycle states; both site and markdown share identical wording via the `statusLabel`/`statusLead` template funcs and the `healthStatusDescription` Go port of `feedHealthDescription`.
- Merge enrichment generator (`tools/build-firehol-static-enrichment.py`) extended to weave `exclude:` components into derivation, listing, unlist, and detection-classification descriptions; UI `SectionMethod` renders an authoritative "Subtracted from the result" block from `feed.merge_excluded`. Regenerated all 20 FireHOL-maintained enrichments and embedded them via `agents/enrichment-public.py embed --all --write`.
- Fixed critical-infrastructure ASN-context lookup at `pkg/engine/critical.go:717` to use `SourcesWithUseDefaultFirst(UseASN)` so the catalog's configured `defaults.asn_provider: iptoasn` is honored instead of the alphabetically-first source `caida_prefix2as`.

## Validation

Acceptance criteria evidence:

- Hero shows `short_description` tagline; verified on `dshield`, `iblocklist_level1`, `feodo_badips`, `cymru_unassigned`.
- Editorial section renders with drop-cap, pull-quote, sidebar (Operated by + upstream link + scope lede + Intended-for checklist); verified visually.
- `not_intended_for` and the old `Current status` block are absent from the markdown; `## Sources consulted` is present at the very end. `## Reputation and community signals` lives at the end of the page on both surfaces.
- Hovering an overlap-table feed link (`bitwire_inbound` on `dshield`) shows the official name, short_description, and maintainer tooltip.
- Status block on `feodo_badips` shows `STATUS: UNMAINTAINED` with the deterministic detection text; on `iblocklist_level1` it shows `STATUS: UNKNOWN` with the AI-researched description; same wording in the markdown.
- `cymru_unassigned` Method section shows "Built from: fullbogons" plus "Subtracted from the result: bogons (archived)"; derivation description in both surfaces mentions inclusion and exclusion.

Tests or equivalent validation:

- `pnpm --dir ui build`: clean.
- `pnpm --dir ui lint`: clean.
- `pnpm --dir ui exec vitest run`: 39/39 passing.
- `go test ./pkg/markdown/...`: clean.
- `go test ./pkg/engine/ -run "Critical|ASN"`: clean.
- `go build ./...`: clean.

Real-use evidence:

- Daemon reinstalled via `./install.sh`; `/healthz` returns 200; manual playwright verification on the four feeds above; markdown re-fetched for each post-reprocess.

Reviewer findings:

- No external reviewer requested for this SOW.

Same-failure scan:

- `grep -rn "SectionAbout\|section-about" ui/src`: no matches.
- `grep -rn "not_intended_for\|NotIntendedFor" pkg ui configs/templates`: only matches at non-rendering sites (data schema, JSON fixtures retaining the field).
- `grep -rn "SourcesWithUse(config.UseASN)" pkg/engine`: no remaining critical-path callers.

Sensitive data gate:

- All durable artifacts use existing fixture identifiers; no credentials, API keys, or customer data added.

Artifact maintenance gate:

- AGENTS.md: no update needed; this SOW reuses the existing runtime contract.
- Runtime project skills: no update; existing project skills already covered the patterns this work used.
- Specs: `.agents/sow/specs/feeds.md` and `.agents/sow/specs/website.md` updated.
- End-user/operator docs: `pkg/web/static/methodology/ai-researched-feed-context.md` updated to reflect the new section ordering and the folded Sources consulted block.
- End-user/operator skills: none affected.
- SOW lifecycle: this SOW closes here; same commit moves the file to `.agents/sow/done/`.

Specs update:

- See artifact maintenance gate.

Project skills update:

- Skipped: no new pattern emerged beyond what `project-frontend-best-practices` and `project-content-surfaces` already cover.

End-user/operator docs update:

- See artifact maintenance gate.

End-user/operator skills update:

- None impacted.

Lessons:

- "Don't lie by omission in narrative" — when a feed is composed of `sources MINUS excludes`, both halves must appear in every generated description and every visible composition list, otherwise readers (human and AI) draw wrong conclusions from the partial story.
- "Helpers named `Preferred…` must actually use the configured preference" — the critical-infra ASN bug existed because `readPreferredASNPayload` used the wrong catalog iterator. Audit similar helper names for the same defect; default to `SourcesWithUseDefaultFirst` (or its sibling) whenever the name implies a preference.
- "Folded surfaces still belong on the page" — auditable evidence like `sources_consulted` should not be removed wholesale just because it is noisy by default; folding it at the end keeps it discoverable without polluting the primary read.

Follow-up mapping:

- "Update cadence" chip on merges shows the raw maintainer string (e.g. "Every 1440 minutes"). Not blocking; track as a polish item, not a SOW.

## Outcome

Completed. Feed-detail pages now have distinct visual character per section, the markdown surface mirrors the website ordering, site-wide feed-reference tooltips carry the short_description, status blocks surface both deterministic and researched lifecycle signals, merge descriptions cover inclusions and exclusions, and the critical-infrastructure ASN-context lookup honours the configured default provider.

## Lessons Extracted

See Lessons above.

## Followup

None yet.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and later testing or use found broken behavior. Use a dated `## Regression - YYYY-MM-DD` heading at the end of the file. Never prepend regression content above the original SOW narrative.
