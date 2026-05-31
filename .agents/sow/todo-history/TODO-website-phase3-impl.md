# Phase 3 — Feed Detail Page Luxury Redesign (Implementation Tracker)

## TL;DR

Surgical rebuild of the feed detail page in `pkg/web/static/{index.html,app.css,app.js}`
to match the spec in `TODO-website-phase3-design.md`. Consumes the insights API
being built in parallel (`TODO-insights.md`). Facts only; no editorial labels.

## Scope

- Feed detail page (`view === 'detail'` block in `index.html`) ONLY.
- Catalog/home/admin/header/footer are OFF-LIMITS.
- Backend is OFF-LIMITS.
- API shapes stay unchanged; only consumption changes.

## Critical rules (non-negotiable)

1. Time unit = last 500 recorded updates. Duration is derived.
2. Default percentile = p75.
3. No hedging or verdict words in any copy.
4. Empty state = silence.
5. Methodology link on every factual callout.

## Implementation checklist

- [x] Read both TODO files cover to cover
- [x] Read current index.html / app.css / app.js / server.go
- [x] Download Inter Display font (Bold/Regular/SemiBold woff2 in pkg/web/static/fonts/)
- [ ] Add new CSS custom properties + display utility classes
- [ ] Add @font-face for InterDisplay
- [ ] Add body tabular-nums rule
- [ ] Add CSS for hero rebuild (grid, title, tagline, status strip)
- [ ] Add CSS for vitals strip (5 cards with sparklines)
- [ ] Add CSS for insights top callout + in-context callouts
- [ ] Add CSS for composition grid (geo | asn + bogons full width)
- [ ] Add CSS for viz tabs framework (+ mobile select)
- [ ] Add CSS for behavior grid (2x2)
- [ ] Add CSS for comparison strip + uniqueness callout
- [ ] Add CSS for tech specs (6 groups)
- [ ] Add CSS for provenance
- [ ] Add CSS for download footer cards
- [ ] Add CSS for reveal scroll animation
- [ ] Rebuild detail page HTML in index.html (new order)
- [ ] Add insights loading + topInsights + sectionInsights + dedupe logic
- [ ] Add animateNumber helper
- [ ] Add scroll reveal observer helper
- [ ] Add tab state manager + localStorage persistence
- [ ] Add hero evolution chart renderer (uPlot filled area)
- [ ] Add sparkline renderer (uPlot)
- [ ] Add behavior grid: churn, freshness (with p75/p90/p100 marks), survival, cadence
- [ ] Add composition: geo choropleth (existing) + geo table (new tab)
- [ ] Add composition: asn table (existing) + asn bubble pack (new tab)
- [ ] Add composition: bogons 3-bucket + per-source breakdown
- [ ] Add comparison scroll strip cards + uniqueness % callout
- [ ] Add tech specs table renderer
- [ ] Add provenance section renderer
- [ ] Add download footer cards + list
- [ ] Load uPlot script in index.html
- [ ] `go build ./...` clean
- [ ] `go test ./...` clean
- [ ] Visual smoke test
- [ ] Commit with clean message

## Judgment calls / tradeoffs

- **Inter Display**: no variable woff2 in Inter v4.0 release; using Regular
  (400) + SemiBold (600) + Bold (700) static woff2s. Total ~325KB, 3 HTTP
  requests. Acceptable — loaded once and cached. Variable font would save
  ~50%.
- **Kaplan-Meier**: the retention.csv `past` data already has the shape
  needed for a survival curve (hours + IPs removed per bucket). The spec
  asks for a proper KM curve with censoring for currently-listed IPs. I
  will implement a simple survival curve from the past data (S(t) = 1 -
  cumulative fraction removed by t), which is the unweighted KM estimate
  when we don't need per-IP censoring data. Good enough for a fact display;
  documented below.
- **hero evolution chart**: uPlot is loaded but not yet used. I'll add a
  script tag and render the filled area chart with uPlot. The existing
  D3 evolution chart stays for the behavior grid's retention/freshness
  views (different data).
- **Sparklines in vital cards**: uPlot mini-mode.
- **Mobile collapse of tabs**: `<select>` dropdown under 640px.

## What will be stubbed (acceptable per task instructions)

- Geography sunburst (tab 2 beyond default+1)
- ASN radar (tab 3 beyond default+1)
- Bogons RFC range (tab 3 beyond default+1)
- Overlaps sankey / chord (extra tabs)
- Retention beeswarm (extra tab)
- Trends calendar heatmap / stream graph (extra tabs)

These will show a small "Coming in a future revision" placeholder with the
appropriate structure so the tab framework is exercised.

## Testing

- `go build ./...`
- `go test ./...`
- Visual smoke test against the running server (local build)

## Done when

- All items checked above
- Single commit landed
- No regressions in other views
- Report delivered
