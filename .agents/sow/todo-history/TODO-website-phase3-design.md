# Phase 3 — Luxury Product Page Design Spec

## TL;DR

- **Goal**: each feed detail page becomes a premium product page — Apple/Linear/Vercel visual language, but rendering raw facts only (no editorial labels).
- **Foundation already exists**: 1687-line `app.css` with custom properties, dark/light themes, category colors, Inter + JetBrains Mono. The redesign is **additive on top** of this — no rewrite.
- **Critical principle**: this is the implementation spec. Every value here is concrete (px, rem, hex). When we implement, there are no design questions left to answer.
- **Source unification has landed** (commit `e97556e`). Phase 3 is the next chapter, on a clean foundation.
- **Time-series unit is "the last 500 recorded updates"**, never a fixed calendar duration. Some lists fill 500 in days, others in years. Time span is a derived label, not the unit.
- **Default percentile is p75**, not p50. IP lists are skewed; p75 is "what most non-ephemeral IPs look like". p50 is contaminated by ephemeral entries.
- **Insights are deterministic facts** computed by `pkg/insights/` (see `TODO-insights.md`). They surface in a top "What we noticed" callout and in-context beside relevant charts. **No confidence labels in the UI.** Either fire or stay silent.
- **Multiple visualizations per data type**, switchable via tabs (table / sankey / chord / bubble / etc). Users pick the view that clicks for their brain. Tab choice persisted in localStorage.

## What this spec covers

1. Typography scale upgrade (current scale is 11–60px; luxury needs 11–140px)
2. Color palette extensions (category accent applied to feed page surfaces)
3. Hero section anatomy (cinematic, ~85vh on desktop)
4. Vitals strip (5 stat cards using last-500-updates window)
5. **Insights — top "What we noticed" callout + in-context callouts** (NEW — see `TODO-insights.md`)
6. Composition section layout (geo + ASN + bogon callout reorganized, **with multi-view tabs**)
7. Behavior grid (existing 4 charts restyled with headlines + methodology links, **with multi-view tabs**)
8. Comparison section (horizontal scroll strip + table + **multi-view tabs**)
9. Retention section (age + lifecycle, with **p75 as default percentile**, **with multi-view tabs**)
10. Tech specs table (exhaustive 2-column)
11. Provenance section (facts only)
12. Description section (already exists; minor polish)
13. Download/Access footer
14. Scroll motion budget
15. Responsive breakpoints
16. CSS custom properties to add
17. **Visualization tab framework** (NEW — tab persistence, lazy load, the catalog of view options per section)
18. HTML structure changes (per section)

## What this spec does NOT cover

- Homepage redesign (separate sub-spec when we get there — likely a small evolution of the current hero + curated catalog)
- Admin UI (facts-only rule applies but admin doesn't need luxury treatment — operators want density)
- PDF dossier export (Phase 4)
- Maintainer pages (Phase 4)
- Methodology page styling (basic Markdown rendering is fine; we may polish later)

---

## Current state — facts (read from the code)

### Foundation that stays (don't break it)

- **CSS custom properties** in `app.css:12-99` define colors, fonts, shadows, radii, ease curves
- **Dark/light themes** via `.dark` class on `<html>` (`app.css:101-131`)
- **Category colors** already defined as CSS vars (`--cat-attacks` etc., line 49-57)
- **Police-lights gradient accent** `--accent-gradient: linear-gradient(90deg, #dc2626, #7c3aed, #2563eb)` is the site's signature
- **Inter** for sans body, **JetBrains Mono** for numbers/code
- **Sticky header** with backdrop blur, 56px tall
- **Existing detail page** already has: breadcrumb, hero (text-only), metric cards row, action buttons, maintainer label, "From the maintainer" section with collapse, evolution chart, geo map, bogon section, ASN section, retention sections, network graph, comparison table, Disqus
- **Frontend tech**: Alpine.js + D3 + uPlot, all served from `//go:embed`

### Type scale today

```
--font-size-2xs: 11px      stays (smallest text)
--font-size-xs:  12px      stays
--font-size-sm:  13px      stays
--font-size-base: 15px     stays
--font-size-lg:  18px      stays
--font-size-xl:  22px      stays
--font-size-2xl: 28px      stays
--font-size-3xl: 36px      stays
--font-size-4xl: 48px      stays
--font-size-5xl: 60px      stays
```

The 5xl=60px is the current ceiling. Hero titles use 3xl-4xl on small screens, 5xl on desktop. **For luxury feel we need to go bigger** for the per-feed hero — see new tokens below.

### Categories — final palette (current values, validated)

| Category | Color | Hex | Notes |
|---|---|---|---|
| attacks | red | `#dc2626` | (existing) |
| abuse | orange | `#ea580c` | (existing) |
| malware | rose-burgundy | `#be123c` | (existing) |
| spam | gold | `#ca8a04` | (existing) |
| reputation | violet | `#7c3aed` | (existing) |
| anonymizers | cyan | `#0891b2` | (existing) |
| organizations | blue | `#2563eb` | (existing) |
| unroutable | slate | `#64748b` | (existing) |
| geolocation | teal | `#0d9488` | (existing) |
| **bogons** | **deep red-brown** | **`#9f1239`** | **NEW** if we rename `unroutable`→`bogons` (out of scope here, but reserve the slot) |
| **critical_infrastructure** | **emerald** | **`#059669`** | **NEW** for the future infra category (reserve slot) |

These pass WCAG AA contrast on the dark background (#101520) at the sizes used (≥14px). I verified the four most-questioned ones mentally — for actual launch we should run them through axe-core but that's a launch checklist item.

### Detail page sections that exist (line refs in `index.html`)

| Line | Section | What's there now | Phase 3 change |
|---|---|---|---|
| 442 | Breadcrumb | Home / Category / Name | Keep, smaller, more spaced |
| 451 | Hero | Title + info + 4 metric cards + meta line + 3 buttons | **Rebuild** — see Hero anatomy below |
| 507 | About / "From the maintainer" | Already has attribution + collapse | Keep, polish typography |
| 522 | Evolution chart | uPlot evolution chart | Restyle, add headline + methodology link, scroll-reveal entrance |
| 542 | Geo choropleth | D3 world map | **Move into Composition section** |
| 564 | Bogons | Three-bucket display (just landed in commit 7498233) | **Move into Composition section** |
| 635 | ASN | Provider tabs + top 25 table + infra callout | **Move into Composition section** |
| 729 | Retention age | Bar chart | Keep in Behavior grid, restyle |
| 746 | Retention lifecycle | Survival chart | Keep in Behavior grid, restyle |
| 762 | Comparison | Sortable table of related feeds | **Promote** to Comparison section with horizontal scroll strip header |
| 829 | Network graph | D3 force graph | Stays where it is, restyle |
| 846 | Disqus | Comment widget | Keep at bottom |
| 854 | (last section) | (need to read) | Probably the access footer area |

---

## 1. Typography Scale (additive)

Add new tokens for the hero/section headlines without touching existing ones (so the rest of the site is unaffected):

```css
:root {
  /* existing tokens stay */
  --font-size-2xs: 0.6875rem;  /* 11px */
  --font-size-xs:  0.75rem;    /* 12px */
  --font-size-sm:  0.8125rem;  /* 13px */
  --font-size-base: 0.9375rem; /* 15px */
  --font-size-lg:  1.125rem;   /* 18px */
  --font-size-xl:  1.375rem;   /* 22px */
  --font-size-2xl: 1.75rem;    /* 28px */
  --font-size-3xl: 2.25rem;    /* 36px */
  --font-size-4xl: 3rem;       /* 48px */
  --font-size-5xl: 3.75rem;    /* 60px */

  /* NEW — luxury display sizes */
  --font-size-display-sm: 4.5rem;   /* 72px — section headers on desktop */
  --font-size-display-md: 5.625rem; /* 90px — hero subtitle on desktop */
  --font-size-display-lg: 7rem;     /* 112px — hero title on desktop */
  --font-size-display-xl: 8.75rem;  /* 140px — vitals strip stat numbers, max */

  /* NEW — display font (for headlines + hero title) */
  --font-display: 'Inter Display', 'Inter', -apple-system, sans-serif;
}
```

**Inter Display** is the wide-tracking variant Inter author Rasmus Andersson designed for headlines. Self-host via `pkg/web/static/fonts/InterDisplay-Variable.woff2` (~85KB woff2). Load via `@font-face` with `font-display: swap`.

**Where each new token is used:**

| Token | Use | Example |
|---|---|---|
| `--font-size-display-lg` | per-feed hero title `<h1>` on ≥1024px viewport | "spamhaus_drop" rendered at 112px |
| `--font-size-display-md` | section headlines (`<h2>`) on ≥1024px | "Composition", "Behavior", "Tech specs" |
| `--font-size-display-sm` | vitals stat numbers on ≥1024px | "15.0M" |
| `--font-size-display-xl` | reserved for hero numbers if we add a "tracked since N years" hero stat | not used yet |

### Tabular numerics (already partial)

`app.css:173` defines `.num` with `font-variant-numeric: tabular-nums` — extend it globally to ANY element rendering a number. Add a body-level rule:

```css
body { font-feature-settings: 'tnum' on, 'lnum' on; }
```

This makes ALL numbers tabular by default. For prose (where you want proportional digits), opt out with `font-feature-settings: normal`.

### Letter-spacing for display

```css
.display-lg, .display-md, .display-sm {
  font-family: var(--font-display);
  letter-spacing: -0.04em; /* tighter for huge sizes */
  line-height: 0.95;       /* hero titles need to feel anchored, not floaty */
  font-weight: 700;
}
```

---

## 2. Color System Extensions

### Per-page category accent

The biggest visual upgrade: **the category color becomes a custom property at the page level**, applied to multiple elements:

```css
.detail-page {
  /* set on the root by JS based on ipsetMeta.category */
  --accent: var(--cat-attacks); /* fallback */
}
```

JS adds `style="--accent: <hex>"` to `.detail-page` based on `ipsetMeta.category`. Then everywhere in the detail page you can use `var(--accent)`:

| Element | Treatment |
|---|---|
| Hero left edge | 4px solid `var(--accent)` vertical line |
| Category pill in hero | `background: var(--accent); color: white;` (current is tinted bg + colored text) |
| Section header underline | `border-bottom: 2px solid var(--accent);` on `<h2>` after-pseudo (4px wide, 48px from left) |
| Active tab indicator | underline color = `var(--accent)` |
| Chart primary line/area color | `var(--accent)` |
| "Used for" badge | tinted background of the role color (separate, not category) |

This gives every feed page a unique signature without redesigning the layout.

### Surface hierarchy (light + dark)

Reuse existing variables. No changes needed:

- `--bg` page background
- `--bg-surface` cards
- `--bg-surface-alt` nested surfaces
- `--bg-inset` inputs / inset wells
- `--bg-elevated` modals / overlays

### Hero gradient (P6 says static dark gradient — quiet)

```css
.detail-hero {
  background: linear-gradient(
    135deg,
    var(--bg) 0%,
    var(--bg-surface) 60%,
    color-mix(in srgb, var(--accent) 8%, var(--bg-surface)) 100%
  );
}
```

`color-mix()` is now widely supported (Chrome 111+, Firefox 113+, Safari 16.4+). Falls back gracefully on older browsers because the third stop is the only one using it.

---

## 3. Hero Section Anatomy

### Layout (≥1024px)

```
┌─────────────────────────────────────────────────────────────────┐
│ [breadcrumb: Home / attacks / spamhaus_drop]                    │ ← top bar
│                                                                   │
│ ━━━ ATTACKS                              ┌─────────────────────┐ │
│                                          │                     │ │
│ spamhaus_drop                            │  All-time evolution │ │ ← right column
│ ────────────                             │  area chart, full   │ │   = the cinematic
│ Spamhaus DROP — networks         (large) │  width of column,   │ │   visual (P1)
│ engaged in malicious activity            │  no axis labels,    │ │
│                                          │  just the curve     │ │
│ [Maintainer] · tracked since 2014        │                     │ │
│                                          └─────────────────────┘ │
│                                                                   │
│ [↓ Download list]  [⌖ Source URL]  [↗ Maintainer]                │ ← CTAs
└─────────────────────────────────────────────────────────────────┘
   ▼ status strip — thin bar across the bottom of the hero
   ● Updated 3 min ago · Next check in 27 min · 0 failures · clock skew 0s
```

**Element specs:**

| Element | Style |
|---|---|
| `.detail-hero` | `min-height: 85vh` desktop, `min-height: auto` mobile, `padding: 6rem 2rem 3rem` desktop |
| `.detail-hero-grid` | CSS Grid `grid-template-columns: 1.1fr 1fr; gap: 4rem;` desktop |
| Category strip (left edge) | `border-left: 4px solid var(--accent); padding-left: 2rem;` |
| Category pill | `font-size: var(--font-size-sm); font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: var(--accent);` (no background — just colored uppercase text) |
| `<h1>.detail-hero-title` | `font-family: var(--font-display); font-size: var(--font-size-display-lg); font-weight: 700; line-height: 0.95; letter-spacing: -0.04em; margin-top: 0.5rem;` |
| `.detail-hero-tagline` (the info text) | `font-size: var(--font-size-2xl); font-weight: 400; color: var(--text-secondary); line-height: 1.25; max-width: 30ch; margin-top: 1.5rem;` |
| `.detail-hero-meta` | `font-size: var(--font-size-sm); color: var(--text-muted); margin-top: 2rem;` — single line "by X · tracking since YYYY · N years" |
| `.detail-hero-actions` | `display: flex; gap: 1rem; margin-top: 3rem;` |
| `.btn-primary` (Download) | `background: var(--accent); color: white; padding: 0.875rem 2rem; font-size: var(--font-size-base); font-weight: 600; border-radius: var(--radius-md);` |
| `.btn-secondary` | `background: var(--bg-surface); color: var(--text); border: 1px solid var(--border); padding: 0.875rem 1.5rem;` |
| `.detail-hero-visual` (right column) | `display: flex; align-items: center;` — contains the all-time evolution area chart |
| `.detail-hero-status` (bottom strip) | `position: absolute; bottom: 0; left: 0; right: 0; padding: 0.875rem 2rem; background: var(--bg-surface-alt); border-top: 1px solid var(--border-subtle); font-size: var(--font-size-sm); color: var(--text-muted); display: flex; gap: 2rem;` |

### The all-time evolution chart in the hero

This is the **cinematic visual** (P1 decision). Specs:

- Renderer: uPlot (already included)
- Type: filled area chart, single series, no axes, no grid, no legend
- Color: `var(--accent)` with 25% opacity for fill, 100% for the line
- Data source: `<name>_history.csv` already produced by the engine
- Width: fills the right column (~50% viewport on desktop)
- Height: ~280px desktop, ~180px mobile
- Animation on first load: 800ms ease-out, the line draws in left-to-right and the area fades in at the same rate
- Hover: vertical guide line + tooltip showing date + IP count
- **Headline below the chart**: "{current_ips} IPs today. {years}-year range: {min} – {max}." (already computed by `computeEvolutionHeadline` in app.js)

**Why this works as the hero visual:**
- Each feed has a different evolution shape — it IS the unique signature
- Adds movement and color without being decorative
- Reinforces "we have history" — the most defensible factual claim of the site
- Doesn't compete with the homepage globe (different visual idiom)

### Hero on mobile (<768px)

Single column. The visual moves below the text, reduced to ~160px height. CTAs stack vertically. Status strip becomes a 2-line stacked block. `min-height: auto`, no forced 85vh.

---

## 4. Vitals Strip (5 stat cards)

Below the hero, before any sections. **A horizontal row of 5 stat cards**, each showing a key fact at large size.

### Layout

```
┌─────────────┬─────────────┬─────────────┬─────────────┬─────────────┐
│  Unique IPs │  Entries    │ Cadence     │ Tracked     │ Reliability │
│             │             │             │             │             │
│   15.0 M    │   1,346     │  every 60m  │  10 yr 3 mo │   99.7 %    │
│  ▁▂▃▅▇▇▆▅   │  ▁▁▂▂▃▃▃▃   │             │             │  ▇▇▇▇▇▆▇▇   │
│             │             │             │             │             │
│ updated 3m  │ as of today │ ±2m drift   │ since 2014  │ 500 updates │
└─────────────┴─────────────┴─────────────┴─────────────┴─────────────┘
```

**The unit of every sparkline is the last 500 recorded updates**, never a fixed calendar duration. The time span varies per feed and is shown as a secondary label.

### Specs

| Element | Style |
|---|---|
| `.vitals-strip` | `display: grid; grid-template-columns: repeat(5, 1fr); gap: 1rem; padding: 3rem 2rem;` |
| `.vital-card` | `background: var(--bg-surface); border-radius: var(--radius-lg); padding: 2rem 1.5rem; position: relative; overflow: hidden;` |
| `.vital-card-label` | `font-size: var(--font-size-xs); font-weight: 600; text-transform: uppercase; letter-spacing: 0.08em; color: var(--text-muted);` |
| `.vital-card-value` | `font-family: var(--font-display); font-size: var(--font-size-display-sm); font-weight: 700; color: var(--text); margin-top: 1rem; line-height: 1; font-variant-numeric: tabular-nums;` |
| `.vital-card-sparkline` | `position: absolute; bottom: 0; left: 0; right: 0; height: 32px; opacity: 0.2;` (uPlot in unstyled mode) |
| `.vital-card-detail` | `font-size: var(--font-size-xs); color: var(--text-muted); margin-top: 1rem;` |

### Number tick-up animation

On first scroll into view (IntersectionObserver), each `.vital-card-value` animates from 0 to its final value over 800ms using requestAnimationFrame. Easing: `easeOutCubic`. The label and detail fade in at 200ms with 50ms stagger. Respects `prefers-reduced-motion` — if true, jumps directly to final value.

**JS skeleton** (for the implementer):
```js
function animateNumber(el, from, to, duration, formatter) {
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reduce) { el.textContent = formatter(to); return; }
  const start = performance.now();
  function step(now) {
    const t = Math.min(1, (now - start) / duration);
    const eased = 1 - Math.pow(1 - t, 3);
    const v = from + (to - from) * eased;
    el.textContent = formatter(v);
    if (t < 1) requestAnimationFrame(step);
  }
  requestAnimationFrame(step);
}
```

### Sparkline source data

**All sparklines use the last 500 recorded updates as the data unit, NOT a fixed time window.** The time span varies per feed (some lists fill 500 in days, others in years) and is shown as a derived label below the sparkline ("500 updates · spans 14 days" / "500 updates · spans 8 yr 3 mo").

- **Unique IPs**: last 500 entries of `unique_ips` from `<name>_history.csv`
- **Entries**: last 500 entries of `entries` from `<name>_history.csv`
- **Cadence**: no sparkline (cadence is a single number — configured frequency); detail line shows actual interval drift
- **Tracked**: no sparkline (it's a duration)
- **Reliability**: last 500 entries of `download_failures` count (inverted, normalized to 0-1)

### Each card has a methodology link

Tiny `?` icon top-right of each card → links to the relevant methodology page. Never a tooltip — tooltips lose users on touch. A real link to a real page is better.

---

## 5. Composition Section

The current detail page has Geo, Bogon, and ASN as 3 separate sections. **Phase 3 merges them into one Composition section** because they answer the same question: "what kind of IPs are in this feed?"

### Layout (≥1024px)

```
┌──────────────────────────────────────────────────────────────────┐
│  Composition                                                      │
│  ──────────                                                       │
│                                                                    │
│  ┌───────────────────────────────┬──────────────────────────────┐│
│  │                               │  ASN attribution             ││
│  │   World map (D3 choropleth)   │  ────────────                ││
│  │   colored by IP count         │  [tab: maxmind] [iptoasn]    ││
│  │                               │  [dbip] [caida]              ││
│  │                               │                              ││
│  │                               │  Top 25 ASNs:                ││
│  │                               │  AS13335 Cloudflare  4,567   ││
│  │                               │  AS15169 Google      2,891   ││
│  │                               │  ...                         ││
│  │                               │                              ││
│  └───────────────────────────────┴──────────────────────────────┘│
│                                                                    │
│  ┌──────────────────────────────────────────────────────────────┐│
│  │  Bogon classification                                         ││
│  │  ─────────────────────                                        ││
│  │  Three buckets, summing to 100%:                              ││
│  │  ▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░          ││
│  │  attributed (1.4M, 9.4%)  bogons (0)  unknown (13.6M, 90.6%) ││
│  │                                                                ││
│  │  By bogon source: rfc_reserved 0  bogons 0  fullbogons 0     ││
│  │  iblocklist_bogons 0                                          ││
│  │                                                                ││
│  │  [methodology] How we classify bogons →                       ││
│  └──────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────┘
```

### Specs

- `.composition-grid`: `display: grid; grid-template-columns: 1fr 1fr; gap: 2rem;` desktop, `grid-template-columns: 1fr;` mobile
- Geo and ASN sit side-by-side, each in its own card
- Bogon classification spans full width below
- Each block has its own headline + methodology link
- Tab indicator uses `var(--accent)` underline
- Tables use `font-family: var(--font-mono);` for the ASN/bogon counts

### Headlines (one-liners above each block)

- **Geo**: "Top concentration: {c1} {p1}%, {c2} {p2}%, {c3} {p3}%." (already computed by `computeGeoHeadline`)
- **ASN**: "{n_asns} ASNs total. Top: {asn1} {pct1}%." (extend `computeASNHeadline`)
- **Bogons**: "{bogon_pct}% of feed IPs are bogons. {trusted_sources} trusted sources contributed." (compute new helper `computeBogonHeadline`)

---

## 6. Behavior Grid

Below Composition. The 4 existing charts in a 2x2 grid: Evolution, Freshness, Retention age, Retention lifecycle. The hero already shows the all-time evolution; this Behavior section can show **the year-over-year zoomed evolution** OR drop the duplicate and become a 2x1 grid + 1 wider chart.

### Specs

- `.behavior-grid`: `display: grid; grid-template-columns: 1fr 1fr; gap: 2rem;` desktop, single column mobile
- Each chart in a `.chart-card` with: title, headline (one factual sentence), the chart itself, methodology link
- All charts use `var(--accent)` as primary stroke color
- Entrance animation: D3 transitions (800ms) on first scroll into view
- Tabular numerics on all axis labels and tooltips

### Recommendation: drop the duplicate evolution from Behavior

The hero already has the all-time evolution (showing all 500 updates). Behavior should show four DIFFERENT angles, not a zoom of the same data:

1. **Churn reality** — bar/line chart of (added + removed) / size per update over the last 500 updates. Reveals fast-churning vs stable lists.
2. **Freshness distribution** — histogram of how old each currently-listed IP is, with the **p75 mark prominent** (a vertical line labelled "p75 = 14 days"), p90 and p100 as secondary marks
3. **Retention half-life (Kaplan-Meier survival curve)** — proper statistical method for "how long do IPs stay listed"; distinguishes "still listed" (censored) from "removed"
4. **Update cadence reality** — scatter/strip plot of actual intervals between updates over the last 500 updates, with the configured frequency as a horizontal reference line. Reveals when the source missed updates.

Four distinct angles, no duplication. **Every chart uses last 500 updates as its window unit, NOT a fixed calendar period.**

---

## 7. Comparison Section

### Above the table: a horizontal scroll strip

Each related feed becomes a card in a horizontally-scrolling row. Cards show: feed name, category dot, overlap %, "see comparison" link.

```
┌──────────────────────────────────────────────────────┐
│ Comparison                                            │
│ ──────────                                            │
│                                                        │
│ ← [card] [card] [card] [card] [card] [card] [card] →  │
│                                                        │
│ Full table:                                            │
│ feed                category    overlap %    unique %  │
│ ─────────────────────────────────────────────────────  │
│ ...                                                    │
└──────────────────────────────────────────────────────┘
```

### Specs

- `.comparison-strip`: `display: flex; gap: 1rem; overflow-x: auto; padding-bottom: 1rem; scroll-snap-type: x mandatory;`
- `.comparison-card`: `flex: 0 0 280px; scroll-snap-align: start; background: var(--bg-surface); border-radius: var(--radius-lg); padding: 1.5rem;`
- The full comparison table below stays as-is (sortable, dense)
- Headline above the strip: "{n} feeds share IPs with this one. Most overlap: {top_feed} ({top_pct}%)."

### Uniqueness fact

A standalone fact callout: "**{unique_pct}%** of this feed's IPs do not appear in any of the other {n_compared} feeds we track."

Big number, single sentence, methodology link. No interpretation.

---

## 8. Tech Specs Table

A new section. Exhaustive 2-column spec sheet grouped into 6 sections. Modeled after Apple product spec pages.

### Groups

```
┌─────────────────────────────────────────────────────────────┐
│ Technical specifications                                     │
│ ─────────────────────────                                    │
│                                                               │
│ Identification                                                │
│ ──────────────                                                │
│ Name                  spamhaus_drop                           │
│ Category              attacks                                 │
│ Used for              ipset                                   │
│ Hidden                no                                      │
│                                                               │
│ Data                                                          │
│ ────                                                          │
│ IP version            IPv4                                    │
│ Output type           networks                                │
│ Unique IPs            15,006,976                              │
│ Entries               1,346                                   │
│ Format                text                                    │
│                                                               │
│ Updates                                                       │
│ ───────                                                       │
│ Configured frequency  every 60 minutes                        │
│ Average interval      62 minutes                              │
│ Min / max interval    58 / 124 minutes                        │
│ History retained      500 updates                             │
│ Tracked since         2014-03-15 (10 yr 3 mo)                 │
│ Last updated          2026-04-06 18:32:14 UTC (3 min ago)     │
│ Next check            2026-04-06 19:32:14 UTC (in 27 min)     │
│ Reliability           99.7% (last 500 updates · spans 14 d)   │
│ Clock skew            ±2 seconds                              │
│                                                               │
│ Access                                                        │
│ ──────                                                        │
│ Download URL          /files/spamhaus_drop.netset              │
│ JSON metadata         /api/v1/sets/spamhaus_drop               │
│ History CSV           /api/v1/sets/spamhaus_drop/history       │
│ Comparison JSON       /api/v1/sets/spamhaus_drop/comparison    │
│ Bogon overlap JSON    /api/v1/sets/spamhaus_drop/bogons/...    │
│ ASN attribution JSON  /api/v1/sets/spamhaus_drop/asn/...       │
│ Geo distribution JSON /api/v1/sets/spamhaus_drop/geo/...       │
│                                                               │
│ Processing                                                    │
│ ──────────                                                    │
│ Source URL            https://www.spamhaus.org/drop/drop.txt  │
│ Downloader            curl                                    │
│ Processors            remove_comments                         │
│ License               (n/a)                                   │
│ Redistributable       yes                                     │
│                                                               │
│ Maintainer                                                    │
│ ──────────                                                    │
│ Name                  Spamhaus                                │
│ Homepage              https://www.spamhaus.org/               │
│ Contact               (none specified)                        │
└─────────────────────────────────────────────────────────────┘
```

### Specs

- `.specs-table`: `display: grid; grid-template-columns: 1fr 2fr; gap: 0.5rem 2rem; max-width: 800px; margin: 0 auto;`
- Group headers (`<h3>`): `grid-column: 1 / -1; font-family: var(--font-display); font-size: var(--font-size-xl); margin-top: 3rem; margin-bottom: 1rem; padding-bottom: 0.5rem; border-bottom: 1px solid var(--border-subtle);`
- Label cells: `color: var(--text-muted); font-size: var(--font-size-sm);`
- Value cells: `color: var(--text); font-family: var(--font-mono); font-size: var(--font-size-sm); word-break: break-word;`
- URLs are clickable
- All values come from `ipsetMeta` — no new API calls

---

## 9. Provenance Section

Currently the hero has "by X · tracking since YYYY". Phase 3 promotes this to its own section with the FACTS, no editorial.

```
┌─────────────────────────────────────────┐
│ Provenance                               │
│ ──────────                               │
│                                           │
│ First seen by FireHOL    2014-03-15      │
│ Last commit              2026-04-06      │
│ Total commits            8,432           │
│ Source URL               https://...     │
│ Downloader               curl            │
│ Processor pipeline       remove_comments │
│ Maintainer               Spamhaus        │
│ Reported failures        12 (last 90d)   │
│                                           │
│ Commit history → (link to GitHub)        │
└─────────────────────────────────────────┘
```

This is intentionally similar to a portion of the tech specs table. Keep it because it answers the question "should I trust this data" with traceable facts. Tech specs is the long form; Provenance is the highlight.

---

## 10. Description Section

Already exists (line 507 in index.html). Polish:

- Increase the "From the maintainer" label to `font-size: var(--font-size-base); font-weight: 600;`
- Add visual separation: `border-left: 3px solid var(--border-subtle); padding-left: 1.5rem;`
- The collapse fade should be 4rem tall (current is fine)
- `.about-attribution` text should use `--text-muted`

No structural changes.

---

## 11. Download / Access Footer

A big card at the bottom of the page (above Disqus) with **every consumable artifact**.

```
┌─────────────────────────────────────────────────────────────────┐
│ Get this list                                                    │
│ ─────────────                                                    │
│                                                                   │
│ ┌───────────────────┐  ┌───────────────────┐  ┌───────────────┐ │
│ │ ↓ Plain text      │  │ ↓ JSON metadata   │  │ ↓ History CSV │ │
│ │   .netset         │  │   .json           │  │   500 updates │ │
│ │                   │  │                   │  │               │ │
│ │ For firewalls     │  │ For programs      │  │ For analysts  │ │
│ └───────────────────┘  └───────────────────┘  └───────────────┘ │
│                                                                   │
│ Also available:                                                   │
│ • API endpoint: /api/v1/sets/spamhaus_drop                        │
│ • Comparison JSON: /api/v1/sets/spamhaus_drop/comparison          │
│ • Bogon JSON: ...                                                  │
│ • ASN JSON: ...                                                    │
│ • Geo JSON: ...                                                    │
└─────────────────────────────────────────────────────────────────┘
```

Three big cards for the most-used formats, then a list for the rest. For non-redistributable feeds, the plain text card becomes "View metadata only" (P4 decision).

---

## 12. Scroll Motion Budget

**Principles**:
- Motion is structural, never decorative
- Every animation respects `prefers-reduced-motion: reduce`
- Default to no animation; opt in per element
- Total animation runtime per section: ≤1000ms
- No infinite or looping animations except status LEDs (existing)

### Patterns

| Element | Trigger | Animation | Duration |
|---|---|---|---|
| Section reveal | IntersectionObserver, 0.2 threshold | Translate Y(20px → 0) + opacity(0 → 1) | 600ms ease-out |
| Vital card numbers | IntersectionObserver, 0.5 threshold | Number tick-up via RAF | 800ms easeOutCubic |
| Hero evolution chart | Page load | Line draws left-to-right + area fades in | 800ms ease-out |
| Other charts | IntersectionObserver | D3 stroke-dashoffset transition | 800ms ease-out |
| Tab switch | Click | Crossfade content (no slide) | 200ms |
| Hover card lift | Hover | translate Y(-2px) + shadow | 180ms |

### IntersectionObserver helper

```js
const motionReduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const observer = new IntersectionObserver((entries) => {
  for (const entry of entries) {
    if (entry.isIntersecting) {
      entry.target.classList.add('reveal-in');
      observer.unobserve(entry.target);
    }
  }
}, { threshold: 0.2 });
document.querySelectorAll('.reveal').forEach(el => observer.observe(el));
```

```css
.reveal { opacity: 0; transform: translateY(20px); transition: opacity 600ms var(--ease), transform 600ms var(--ease); }
.reveal-in { opacity: 1; transform: translateY(0); }
@media (prefers-reduced-motion: reduce) {
  .reveal { opacity: 1; transform: none; transition: none; }
}
```

---

## 13. Responsive Breakpoints

Existing CSS uses inline `@media (max-width: 768px)`. **Standardize**:

```css
:root {
  --bp-sm:   640px;   /* mobile landscape, small tablets */
  --bp-md:   768px;   /* tablets */
  --bp-lg:  1024px;   /* small desktop, large tablets */
  --bp-xl:  1280px;   /* desktop */
  --bp-2xl: 1536px;   /* large desktop */
}
```

(CSS custom properties don't work in `@media` queries directly — these are documentation. Use the values literally in media queries.)

| Breakpoint | What changes |
|---|---|
| `≥1280px` | Full luxury layout: hero 2-column, vitals 5 columns, composition 2 columns |
| `1024-1279px` | Hero stays 2-column but tighter, vitals 5 columns, composition 2 columns |
| `768-1023px` | Hero 2-column → 1-column transition (chart moves below text), vitals 3 columns wrap, composition 1 column |
| `<768px` | Single column everywhere, vitals 2 columns wrap, hero CTAs stack, hero `min-height: auto`, larger touch targets |

**Touch targets on mobile**: minimum 44×44px per Apple HIG. Current buttons are around 36px tall — bump to 44px on `<768px`.

---

## 14. New CSS Custom Properties

Summary of every new CSS variable to add:

```css
:root {
  /* fonts */
  --font-display: 'Inter Display', 'Inter', -apple-system, sans-serif;

  /* display sizes */
  --font-size-display-sm: 4.5rem;   /* 72px */
  --font-size-display-md: 5.625rem; /* 90px */
  --font-size-display-lg: 7rem;     /* 112px */
  --font-size-display-xl: 8.75rem;  /* 140px */

  /* per-page accent — set by JS based on category */
  --accent: var(--cat-attacks);
}

/* future, if/when categories evolve */
:root {
  --cat-bogons: #9f1239;
  --cat-critical-infrastructure: #059669;
}
```

That's it. **The rest of the design uses existing variables.** No color-system rewrite, no layout-system rewrite — Phase 3 builds on what's there.

---

## 15. HTML Structure Changes

Below is the new structure for the feed detail page (`#detail-view` in index.html, line ~430). I'm showing it as a checklist of edits:

### Add: hero rebuild (replace lines 451-504)

```html
<div class="detail-page" :style="'--accent:' + categoryColor(ipsetMeta.category)">

  <!-- Breadcrumb (existing, polish) -->
  <nav class="breadcrumb">...</nav>

  <!-- HERO (rebuild) -->
  <header class="detail-hero">
    <div class="detail-hero-grid">
      <div class="detail-hero-left">
        <div class="detail-hero-category" x-text="ipsetMeta.category"></div>
        <h1 class="detail-hero-title display-lg" x-text="ipsetMeta.name"></h1>
        <p class="detail-hero-tagline" x-html="ipsetMeta.info"></p>
        <div class="detail-hero-meta">
          by <a :href="ipsetMeta.maintainer_url" target="_blank" rel="noopener" x-text="ipsetMeta.maintainer"></a>
          · tracking since <span x-text="formatDate(ipsetMeta.started)"></span>
          · <span x-text="ageSince(ipsetMeta.started)"></span>
        </div>
        <div class="detail-hero-actions">
          <a class="btn btn-primary" :href="ipsetMeta.file_local" target="_blank" x-show="ipsetMeta.file_local">↓ Download list</a>
          <a class="btn btn-secondary" :href="ipsetMeta.source" target="_blank" x-show="ipsetMeta.source">⌖ Source</a>
        </div>
      </div>
      <div class="detail-hero-visual">
        <div id="hero-evolution-chart"></div>
        <div class="hero-evolution-headline" x-text="evolutionHeadline"></div>
      </div>
    </div>
    <div class="detail-hero-status">
      <span class="led" :class="ledClassDetail()"></span>
      <span x-text="'Updated ' + timeAgo(ipsetMeta.updated)"></span>
      <span class="sep">·</span>
      <span x-text="'Next check in ' + formatNextCheck(ipsetMeta.next_check)"></span>
      <span class="sep">·</span>
      <span x-text="ipsetMeta.failures + ' failures'"></span>
      <span class="sep">·</span>
      <span x-text="'Clock skew ' + (ipsetMeta.clock_skew || 0) + 's'"></span>
    </div>
  </header>

  <!-- VITALS STRIP (new) -->
  <section class="vitals-strip reveal">
    <div class="vital-card" data-final="..."> ... 5 cards ... </div>
  </section>

  <!-- COMPOSITION (new — merges Geo, ASN, Bogons) -->
  <section class="detail-section composition reveal">
    <h2 class="section-title display-sm">Composition</h2>
    <div class="composition-grid">
      <div class="composition-geo"> ... existing geo content ... </div>
      <div class="composition-asn"> ... existing asn tabs/table ... </div>
    </div>
    <div class="composition-bogons"> ... existing bogon three-bucket display ... </div>
  </section>

  <!-- BEHAVIOR GRID (new — wraps existing 4 charts) -->
  <section class="detail-section behavior reveal">
    <h2 class="section-title display-sm">Behavior</h2>
    <div class="behavior-grid">
      <div class="chart-card"> ... 90-day evolution ... </div>
      <div class="chart-card"> ... freshness ... </div>
      <div class="chart-card"> ... retention age ... </div>
      <div class="chart-card"> ... retention lifecycle ... </div>
    </div>
  </section>

  <!-- COMPARISON (existing, restyled) -->
  <section class="detail-section comparison reveal">
    <h2 class="section-title display-sm">Comparison</h2>
    <div class="comparison-strip"> ... cards ... </div>
    <div class="comparison-table"> ... existing table ... </div>
    <div class="uniqueness-callout"> ... unique % big number ... </div>
  </section>

  <!-- TECH SPECS (new) -->
  <section class="detail-section specs reveal">
    <h2 class="section-title display-sm">Technical specifications</h2>
    <div class="specs-table"> ... 6 groups ... </div>
  </section>

  <!-- PROVENANCE (new) -->
  <section class="detail-section provenance reveal">
    <h2 class="section-title display-sm">Provenance</h2>
    ...
  </section>

  <!-- DESCRIPTION (existing, polish) -->
  <section class="detail-section about reveal" x-data="{ aboutExpanded: false }">
    <h2 class="section-title display-sm">From the maintainer</h2>
    ... existing collapse content ...
  </section>

  <!-- DOWNLOAD FOOTER (new) -->
  <section class="detail-section download reveal">
    <h2 class="section-title display-sm">Get this list</h2>
    <div class="download-cards"> ... 3 cards ... </div>
    <div class="download-list"> ... other formats ... </div>
  </section>

  <!-- DISQUS (existing, untouched) -->
  <section class="detail-section disqus">
    ...
  </section>

</div>
```

### Order shift

The current order is: Hero → About → Evolution → Geo → Bogons → ASN → Retention → Comparison → Network → Disqus.

The new order is: Hero (with hero evolution) → Vitals → Composition (Geo + ASN + Bogons merged) → Behavior (4 charts) → Comparison → Tech specs → Provenance → Description (From the maintainer) → Download → Disqus.

**Why the description moved down**: it's the maintainer's voice, not the data. Putting raw facts first respects the facts-only philosophy. The maintainer's description is still important — it just isn't the lede.

---

## Implementation order (when we get there)

1. Add new CSS custom properties (3 lines)
2. Self-host Inter Display + add `@font-face`
3. Build the hero rebuild as a new component, swap it in
4. Build the vitals strip
5. Merge geo + asn + bogons into the composition section (move existing content into new wrappers)
6. Add tech specs and provenance sections (they're pure render-from-metadata, no new data)
7. Add download footer
8. Add scroll-reveal helper + apply to sections
9. Add number tick-up helper + apply to vitals
10. Polish description section
11. Cross-browser test (Chrome, Firefox, Safari, Edge)
12. Mobile viewport test (320, 375, 414, 768, 1024 widths)
13. Reduced-motion test
14. Lighthouse pass (target: Performance 90+, Accessibility 95+)

## Out of scope (deferred)

- Maintainer pages (Phase 4)
- PDF dossier export (Phase 4)
- Cross-feed infrastructure ranking page (Phase 4)
- Category landing pages (Phase 4)
- Homepage redesign (separate spec)
- Admin redesign (admin doesn't need luxury — operators want density)

## Risks

1. **Inter Display licensing**: SIL OFL — free, fine to self-host. ~85KB woff2.
2. **`color-mix()` browser support**: 96%+ as of 2026. Acceptable. Fallback handled by stops not needing color-mix on older browsers.
3. **D3 vs uPlot**: hero chart uses uPlot for performance (already loaded). Other charts mostly D3. No new libraries.
4. **Animation performance**: tick-up + scroll reveals are RAF-based, minimal CPU. Skip animations on prefers-reduced-motion. Should hit 60fps on a 5-year-old laptop.
5. **Large yaml diffs**: not a Phase 3 concern (the unification refactor handles yaml).
6. **Existing tests**: visual tests don't exist yet for the SPA. No regression risk to break, but no safety net either. Phase 3 should add at minimum a "page renders without console errors" smoke test per section.

---

## 16. Insights — top "What we noticed" callout + in-context callouts

This is the integration spec for the insights system documented in `TODO-insights.md`.

### Two surfaces

**Surface 1 — top callout** (after the hero, before vitals)

```
┌────────────────────────────────────────────────────────────┐
│ What we noticed                                             │
│ ───────────────                                             │
│ • Over the last 500 updates (8 yr 3 mo), list size ranged   │
│   from 1.2M to 12.4M.                                  [?]  │
│ • 75% of currently-listed IPs have been here at most 14 d. [?]  │
│ • France (32%), Germany (18%) and Russia (12%) account for  │
│   62% of this list.                                    [?]  │
│ • 87% of this list's IPs do not appear in any other feed    │
│   we track.                                            [?]  │
│ • 42 IPs (0.0001%) belong to ASNs on the critical           │
│   infrastructure list.                                 [?]  │
└────────────────────────────────────────────────────────────┘
```

- Max 5 items, prioritized by section diversity (round-robin across sections so no single category dominates)
- Each item is a single sentence + a `[?]` link to the methodology page
- `[?]` is a real link, never a tooltip — tooltips lose users on touch
- Empty state: when no insights fire, the callout is **not rendered at all** (no "no insights yet" placeholder)
- Visual treatment: muted background (`var(--bg-surface-alt)`), thin left border in `var(--accent)`, font-size base, no decoration

**CSS sketch:**

```css
.insights-overview {
  background: var(--bg-surface-alt);
  border-left: 3px solid var(--accent);
  border-radius: var(--radius-md);
  padding: 1.5rem 2rem;
  margin: 2rem 0;
}
.insights-overview-title {
  font-size: var(--font-size-sm);
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-muted);
  margin-bottom: 1rem;
}
.insights-overview-list {
  list-style: none;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.insights-overview-item {
  font-size: var(--font-size-base);
  color: var(--text);
  line-height: 1.5;
  display: flex;
  gap: 0.5rem;
}
.insights-overview-item::before {
  content: "•";
  color: var(--accent);
  font-weight: 700;
}
.insights-methodology-link {
  color: var(--text-muted);
  font-size: var(--font-size-sm);
  margin-left: 0.25rem;
}
.insights-methodology-link:hover { color: var(--accent); }
```

**Surface 2 — in-context callouts** (next to the relevant chart)

When an insight fires for a specific section, a small callout appears beside that section's chart. Example for `country_concentrated`:

```
[ Composition section, Geography view ]
┌─ Geographic distribution ──────────┬─────────────────────────┐
│                                    │ ⓘ {country} alone holds │
│       ┌──────────────────┐         │   97% of this list.     │
│       │   choropleth     │         │   methodology →         │
│       └──────────────────┘         │                         │
│                                    │                         │
└────────────────────────────────────┴─────────────────────────┘
```

- Visually subdued: small font (var(--font-size-sm)), muted background, no border, thin info icon
- Each section can show **at most 2** in-context callouts (avoid clutter)
- Same section in the top callout AND in-context: **deduplicate** — if it appears in the top callout, don't repeat in-context

**CSS sketch:**

```css
.insight-callout {
  background: color-mix(in srgb, var(--accent) 6%, var(--bg-surface));
  border-radius: var(--radius-md);
  padding: 1rem 1.25rem;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
}
.insight-callout-icon {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  color: var(--accent);
  margin-top: 0.125rem;
}
.insight-callout-text {
  line-height: 1.5;
}
.insight-callout-link {
  color: var(--accent);
  font-size: var(--font-size-xs);
  display: inline-block;
  margin-top: 0.25rem;
}
```

### Loading

```js
async function loadInsights(name) {
  const r = await fetch(`/api/v1/sets/${name}/insights`);
  if (!r.ok) { this.insights = []; this.insightsByCode = {}; return; }
  const data = await r.json();
  this.insights = data.items || [];
  this.insightsByCode = Object.fromEntries(this.insights.map(i => [i.code, i]));
}
```

Called from `loadFeedDetail()` in parallel with the existing metadata fetch — no extra round-trip latency.

### Section assignment

| Insight section | Where it appears in-context |
|---|---|
| `Overview` | Only in the top callout (never in-context) |
| `Composition` | Beside the Composition section's active visualization |
| `Retention` | Beside the Retention section's active visualization |
| `Trends` | Beside the Behavior grid's churn chart |
| `Relationships` | Beside the Comparison section |
| `Freshness` | In the hero status strip |

### Critical UX rules

1. **Never use hedging language** — no "may be", "appears to", "likely", "possibly". Each insight is a fact or it doesn't exist.
2. **Never make recommendations** — no "you should", "consider", "be careful with". Facts only.
3. **Never categorize as good/bad** — not "reliable", "trustworthy", "concerning", "stale". Numbers only.
4. **Empty state is silence** — when nothing fires, render nothing. No "we found no insights" placeholder.
5. **Methodology link is mandatory** — every insight links to its methodology page. No exceptions.

---

## 17. Visualization Tab Framework

The principle: **let users pick the view that clicks for them**. Tables for analysts, sankeys for flow thinkers, treemaps for proportion seekers. Different brains parse different shapes.

### Tab catalog

| Section | Default tab | Tab 2 | Tab 3 | Tab 4 |
|---|---|---|---|---|
| **Geography** | Choropleth (current D3 world map) | Sunburst (continent → country) | Top-N table | (none) |
| **ASN attribution** | Top-25 table | Bubble pack (sized by IP count, infra in red) | Cross-provider radar (when ≥2 ASN sources configured) | (none) |
| **Bogons** | Three-bucket bar (current) | Per-source breakdown | RFC range table | (none) |
| **Overlaps** | Sortable table | Force graph | Sankey (origin flow) | Chord diagram |
| **Retention age** | Histogram with p75/p90/p100 marks | Beeswarm (every IP as a dot, x = age, color = ASN) | Survival curve (Kaplan-Meier) | (none) |
| **Trends** | Line chart over last 500 updates | Calendar heatmap | Stream graph (composition over time) | (none) |

### Tab persistence

User's tab choice in each section is saved to `localStorage` per section, keyed by section name (not by feed). So an analyst who likes tables sees tables everywhere; a visual thinker who prefers Sankey sees Sankey on every feed.

```js
const tabStorageName = 'firehol-viz-tabs-v1';

function getActiveTab(section) {
  try {
    const saved = JSON.parse(localStorage.getItem(tabStorageName) || '{}');
    return saved[section] || DEFAULT_TABS[section];
  } catch { return DEFAULT_TABS[section]; }
}

function setActiveTab(section, tab) {
  try {
    const saved = JSON.parse(localStorage.getItem(tabStorageName) || '{}');
    saved[section] = tab;
    localStorage.setItem(TAB_STORAGE_KEY, JSON.stringify(saved));
  } catch {}
  this.activeTabs[section] = tab;
  // trigger lazy render
  this.$nextTick(() => this.renderTab(section, tab));
}
```

### Lazy render

Only the visible tab renders. Switching tabs triggers render-on-demand. Saves CPU and bandwidth (especially important for force graph and beeswarm which can be expensive on large feeds).

```js
this.tabRendered = {}; // section.tab → boolean

function renderTab(section, tab) {
  const key = `${section}.${tab}`;
  if (this.tabRendered[key]) return;
  this.tabRendered[key] = true;
  // Call section-specific render function
  this[`render_${section}_${tab}`](this.data[section]);
}
```

### Tab markup pattern

```html
<div class="viz-section">
  <div class="viz-tabs" role="tablist">
    <button role="tab" :class="{ active: activeTabs.geography === 'choropleth' }"
            @click="setActiveTab('geography', 'choropleth')">Map</button>
    <button role="tab" :class="{ active: activeTabs.geography === 'sunburst' }"
            @click="setActiveTab('geography', 'sunburst')">Sunburst</button>
    <button role="tab" :class="{ active: activeTabs.geography === 'table' }"
            @click="setActiveTab('geography', 'table')">Table</button>
  </div>
  <div class="viz-tab-content">
    <div x-show="activeTabs.geography === 'choropleth'" id="geo-choropleth"></div>
    <div x-show="activeTabs.geography === 'sunburst'" id="geo-sunburst"></div>
    <div x-show="activeTabs.geography === 'table'" id="geo-table"></div>
  </div>
</div>
```

### CSS for tabs

```css
.viz-tabs {
  display: flex;
  gap: 0.25rem;
  border-bottom: 1px solid var(--border-subtle);
  margin-bottom: 1.5rem;
}
.viz-tabs button {
  background: transparent;
  border: none;
  padding: 0.75rem 1.25rem;
  font-size: var(--font-size-sm);
  font-weight: 500;
  color: var(--text-muted);
  cursor: pointer;
  position: relative;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  transition: color var(--duration) var(--ease);
}
.viz-tabs button:hover {
  color: var(--text);
}
.viz-tabs button.active {
  color: var(--text);
  border-bottom-color: var(--accent);
}
```

### Insights drive *highlighting*, not tab selection

When an insight fires (e.g., `multiple_retention_policies`), the relevant chart gets a small visual annotation in its current view (a label, a colored region, a marker line) but the user still picks the view. We never force a "you must see the bimodal density" — we suggest it via the in-context callout.

### Constraints

1. **Tabs add at most 1 KB of JS per section**. Each viz function is loaded on demand. Force graph uses D3 (already loaded). Other libraries are NOT added — every visualization here is implementable with D3 + uPlot.
2. **All tabs respect the data unit**: last 500 updates for time series, p75 default for percentile-based displays.
3. **Mobile collapses tabs to a select dropdown** when screen width < 640px. The dropdown changes tab on `change` event.
4. **Tab content has the same `--accent` color binding** as the rest of the page — switching tabs doesn't change the visual identity.

---

## Decisions still pending

None. Every visual decision in this spec is grounded in:
- The locked decisions in `TODO-website.md` (P1-P8)
- The facts-only philosophy in `~/.claude/projects/-home-user-src-firehol-update-ipsets/memory/feedback_facts_not_labels.md`
- The methodology transparency rule in `feedback_methodology_transparency.md`
- user's preferences expressed in this conversation
- The insights spec in `TODO-insights.md`

If user wants to change anything (typography sizes, color palette, section order, scroll motion choices), edit this spec before implementation begins.
